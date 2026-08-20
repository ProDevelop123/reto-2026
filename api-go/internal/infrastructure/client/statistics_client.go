// Package client contiene los adaptadores hacia servicios externos.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"api-go/internal/config"
	"api-go/internal/core/domain"
	"api-go/internal/matrix"
	"api-go/pkg/apperror"
)

// StatisticsClient consume la API de estadisticas en Node por HTTP.
// Implementa port.StatisticsProvider.
//
// Este tipo es el ADAPTADOR: es el unico lugar del servicio que sabe que las
// estadisticas las calcula un proceso remoto, que se habla JSON con el y cual
// es la forma exacta de ese JSON. El caso de uso solo conoce la interfaz.
type StatisticsClient struct {
	baseURL      string
	httpClient   *http.Client
	maxRetries   int
	retryBackoff time.Duration
	totalTimeout time.Duration
}

// Rutas del servicio remoto.
const statisticsPath = "/api/v1/statistics"

// dialTimeout acota el establecimiento de la conexion, resolucion de nombre
// incluida.
//
// No se expone como variable de entorno porque no depende del despliegue: el
// servicio remoto siempre esta en la misma red y conectar con el toma
// milisegundos. Un valor generoso frente a esa escala, pero muy inferior al
// timeout de la peticion, basta para separar "el destino no esta" de "el
// destino tarda en responder".
const dialTimeout = 1500 * time.Millisecond

// New construye el cliente.
func New(cfg config.StatisticsConfig) *StatisticsClient {
	return &StatisticsClient{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		httpClient: &http.Client{
			// El timeout cubre toda la operacion, conexion incluida. Sin el,
			// una dependencia colgada retendria goroutines indefinidamente y
			// acabaria arrastrando a este servicio en su caida.
			Timeout: cfg.Timeout,
			Transport: &http.Transport{
				// Timeout de CONEXION, mas corto que el de la peticion completa.
				//
				// Distingue dos fallos que conviene tratar distinto: "no consigo
				// conectar" y "conecte pero la respuesta tarda". Establecer una
				// conexion dentro de la misma red toma milisegundos, asi que si
				// no se logra en este plazo es que el destino no esta. Sin esta
				// separacion, un servicio caido consumiria el timeout completo en
				// cada intento: dentro de una red de contenedores su nombre deja
				// de resolverse y la espera se va integra a la resolucion DNS.
				DialContext: (&net.Dialer{
					Timeout:   dialTimeout,
					KeepAlive: 30 * time.Second,
				}).DialContext,

				// Reutilizar conexiones evita repetir el saludo TCP en cada
				// peticion. Los valores por defecto de Go limitan a 2 las
				// conexiones ociosas por host, insuficiente cuando todo el
				// trafico va a un unico destino.
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		maxRetries:   cfg.MaxRetries,
		retryBackoff: cfg.RetryBackoff,
		totalTimeout: cfg.TotalTimeout,
	}
}

// Analyze envia las matrices al servicio de estadisticas y traduce su
// respuesta al vocabulario del dominio.
func (c *StatisticsClient) Analyze(
	ctx context.Context,
	request domain.StatisticsRequest,
) (domain.StatisticsResult, error) {
	payload, err := json.Marshal(toWireRequest(request))
	if err != nil {
		return domain.StatisticsResult{},
			apperror.Internal("No se pudo serializar la peticion de estadisticas.").WithCause(err)
	}

	// Presupuesto de tiempo para la operacion COMPLETA, reintentos incluidos.
	//
	// El timeout del cliente HTTP acota cada intento por separado, de modo que
	// sin este techo global la peor espera seria Timeout x (MaxRetries+1) mas
	// las pausas entre reintentos. Con la configuracion por defecto eso son 9
	// segundos de latencia para un cliente que solo va a recibir un 502.
	//
	// El caso que lo hace evidente es el servicio remoto caido dentro de una red
	// de contenedores: su nombre desaparece del DNS interno y cada intento se
	// consume esperando a una resolucion que nunca llega, en lugar de fallar al
	// instante como haria una conexion rechazada en local.
	ctx, cancel := context.WithTimeout(ctx, c.totalTimeout)
	defer cancel()

	response, err := c.postWithRetries(ctx, payload)
	if err != nil {
		return domain.StatisticsResult{}, err
	}

	return toDomainResult(response), nil
}

// postWithRetries ejecuta la peticion reintentando los fallos transitorios.
//
// Solo se reintenta lo que puede tener exito al repetirse: errores de red y
// respuestas 5xx. Un 4xx significa que la peticion es incorrecta y repetirla
// solo malgastaria tiempo y carga sobre el servicio remoto.
//
// La espera crece de forma exponencial para no castigar a un servicio que ya
// esta sufriendo, y respeta la cancelacion del contexto: si el cliente original
// se marcho, no tiene sentido seguir reintentando por el.
func (c *StatisticsClient) postWithRetries(
	ctx context.Context,
	payload []byte,
) (statisticsResponse, error) {
	var lastErr error
	backoff := c.retryBackoff

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return statisticsResponse{}, contextError(ctx.Err())
			case <-time.After(backoff):
				backoff *= 2
			}
		}

		response, retryable, err := c.post(ctx, payload)
		if err == nil {
			return response, nil
		}

		lastErr = err
		if !retryable {
			return statisticsResponse{}, err
		}
	}

	return statisticsResponse{}, lastErr
}

// post ejecuta un unico intento e indica si el fallo justifica reintentar.
func (c *StatisticsClient) post(
	ctx context.Context,
	payload []byte,
) (statisticsResponse, bool, error) {
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL+statisticsPath, bytes.NewReader(payload))
	if err != nil {
		return statisticsResponse{}, false,
			apperror.Internal("No se pudo construir la peticion de estadisticas.").WithCause(err)
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	// El token del usuario se propaga al servicio remoto para que la llamada
	// interna mantenga la identidad de quien la origino. La API de Node exige
	// autenticacion tambien en las llamadas de servicio a servicio: no se
	// asume que la red interna sea de confianza.
	if token, ok := ctx.Value(accessTokenKey).(string); ok && token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		// Hay que distinguir dos vencimientos que producen el mismo error.
		//
		// El timeout POR INTENTO del cliente HTTP tambien satisface
		// errors.Is(err, context.DeadlineExceeded), asi que comprobar solo eso
		// haria que una respuesta lenta se diera por perdida sin reintentar.
		// Consultar el estado del contexto padre resuelve la ambiguedad: si ya
		// vencio, el que se agoto es el presupuesto GLOBAL y reintentar seria
		// inutil; si sigue vivo, lo que vencio fue este intento y queda margen
		// para otro.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return statisticsResponse{}, false, contextError(ctxErr)
		}

		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return statisticsResponse{}, true,
				apperror.Timeout("El servicio de estadisticas no respondio a tiempo.").WithCause(err)
		}

		return statisticsResponse{}, true,
			apperror.Upstream("No se pudo contactar con el servicio de estadisticas.").WithCause(err)
	}
	defer response.Body.Close()

	// Se acota la lectura del cuerpo: un servicio remoto comprometido o
	// defectuoso podria responder con un flujo interminable y agotar la memoria.
	body, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return statisticsResponse{}, true,
			apperror.Upstream("No se pudo leer la respuesta del servicio de estadisticas.").WithCause(err)
	}

	if response.StatusCode != http.StatusOK {
		retryable := response.StatusCode >= 500
		return statisticsResponse{}, retryable, upstreamError(response.StatusCode, body)
	}

	var decoded statisticsResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return statisticsResponse{}, false,
			apperror.Upstream("La respuesta del servicio de estadisticas no es valida.").WithCause(err)
	}

	if !decoded.Success {
		return statisticsResponse{}, false,
			apperror.Upstream("El servicio de estadisticas reporto un fallo.")
	}

	return decoded, false, nil
}

// upstreamError traduce un codigo de estado remoto a un error de aplicacion.
//
// Se distingue con cuidado de quien es la culpa. Un 401 del servicio remoto no
// significa que el usuario no este autenticado: significa que ESTE servicio no
// se autentico correctamente frente a su dependencia, lo que es un fallo de
// configuracion nuestro y debe reportarse como tal.
func upstreamError(status int, body []byte) error {
	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &payload)

	detail := payload.Error.Message
	if detail == "" {
		detail = fmt.Sprintf("codigo de estado %d", status)
	}

	if status >= 500 {
		return apperror.Upstream("El servicio de estadisticas fallo.").
			WithDetails(map[string]any{"upstreamStatus": status, "upstreamMessage": detail})
	}

	return apperror.Upstream("El servicio de estadisticas rechazo la peticion.").
		WithDetails(map[string]any{"upstreamStatus": status, "upstreamMessage": detail})
}

// contextError traduce la cancelacion del contexto a un error de aplicacion.
func contextError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return apperror.Timeout("Se agoto el tiempo de espera del servicio de estadisticas.").
			WithCause(err)
	}
	return apperror.Upstream("La peticion fue cancelada.").WithCause(err)
}

// --- Traduccion entre el dominio y el formato de cable -------------------
//
// Estos tipos describen el JSON que entiende la API de Node. Viven aqui, en el
// adaptador, y no en el dominio: si manana ese contrato cambiara, solo habria
// que tocar este fichero.

type wireMatrix struct {
	Name string        `json:"name"`
	Data matrix.Matrix `json:"data"`
}

type statisticsRequestBody struct {
	Matrices  []wireMatrix `json:"matrices"`
	Tolerance *float64     `json:"tolerance,omitempty"`
}

type statisticsResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Global    wireGlobalStatistics   `json:"global"`
		PerMatrix []wireMatrixStatistics `json:"perMatrix"`
	} `json:"data"`
	Metadata struct {
		Tolerance  float64   `json:"tolerance"`
		ComputedAt time.Time `json:"computedAt"`
	} `json:"metadata"`
}

type wireGlobalStatistics struct {
	Matrices         int      `json:"matrices"`
	Count            int      `json:"count"`
	Max              float64  `json:"max"`
	Min              float64  `json:"min"`
	Sum              float64  `json:"sum"`
	Average          float64  `json:"average"`
	IsAnyDiagonal    bool     `json:"isAnyDiagonal"`
	DiagonalMatrices []string `json:"diagonalMatrices"`
}

type wireMatrixStatistics struct {
	Name       string  `json:"name"`
	Rows       int     `json:"rows"`
	Columns    int     `json:"columns"`
	Count      int     `json:"count"`
	Max        float64 `json:"max"`
	Min        float64 `json:"min"`
	Sum        float64 `json:"sum"`
	Average    float64 `json:"average"`
	IsSquare   bool    `json:"isSquare"`
	IsDiagonal bool    `json:"isDiagonal"`
}

func toWireRequest(request domain.StatisticsRequest) statisticsRequestBody {
	matrices := make([]wireMatrix, 0, len(request.Matrices))
	for _, m := range request.Matrices {
		matrices = append(matrices, wireMatrix{Name: m.Name, Data: m.Data})
	}

	return statisticsRequestBody{Matrices: matrices, Tolerance: request.Tolerance}
}

func toDomainResult(response statisticsResponse) domain.StatisticsResult {
	perMatrix := make([]domain.MatrixStatistics, 0, len(response.Data.PerMatrix))
	for _, m := range response.Data.PerMatrix {
		perMatrix = append(perMatrix, domain.MatrixStatistics(m))
	}

	return domain.StatisticsResult{
		Global:     domain.GlobalStatistics(response.Data.Global),
		PerMatrix:  perMatrix,
		Tolerance:  response.Metadata.Tolerance,
		ComputedAt: response.Metadata.ComputedAt,
	}
}
