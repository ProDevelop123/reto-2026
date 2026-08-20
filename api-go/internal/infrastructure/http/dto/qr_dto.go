package dto

import (
	"fmt"
	"math"
	"time"

	"api-go/internal/core/domain"
	"api-go/internal/matrix"
	"api-go/pkg/apperror"
)

// Limites de la peticion.
//
// Existen porque la factorizacion tiene coste O(m*n*min(m,n)) y la respuesta
// crece con el cuadrado de las dimensiones: sin cota, una sola peticion podria
// mantener ocupado un nucleo durante minutos y devolver cientos de megabytes.
const (
	maxRows = 512
	maxCols = 512
)

// FactorizeRequest es el cuerpo de POST /api/v1/qr.
type FactorizeRequest struct {
	// Matrix es la matriz original, como array de arrays de numeros. Es
	// exactamente el formato que pide el enunciado.
	Matrix [][]float64 `json:"matrix"`

	// Mode selecciona la variante de la factorizacion: "reduced" (por defecto)
	// o "complete".
	Mode string `json:"mode,omitempty"`

	// Tolerance es el umbral opcional con el que el servicio de estadisticas
	// decidira si una matriz es diagonal. Puntero porque cero es un valor
	// legitimo —comparacion exacta— y debe distinguirse de "no indicado".
	Tolerance *float64 `json:"tolerance,omitempty"`
}

// ToDomain valida la peticion y la traduce al vocabulario del dominio.
//
// La validacion vive aqui, en el borde, para que el nucleo pueda dar por
// buenos los datos que recibe. El paquete matrix vuelve a validar por su
// cuenta porque es una biblioteca independiente que no puede confiar en que
// alguien haya comprobado sus precondiciones; esta capa se ocupa ademas de las
// reglas propias de la API, como los limites de tamano.
func (r FactorizeRequest) ToDomain() (domain.FactorizationInput, error) {
	if len(r.Matrix) == 0 {
		return domain.FactorizationInput{},
			apperror.Validation("El campo \"matrix\" es obligatorio y no puede estar vacio.")
	}

	if len(r.Matrix) > maxRows {
		return domain.FactorizationInput{},
			apperror.Validation(fmt.Sprintf(
				"La matriz tiene %d filas y el maximo admitido es %d.", len(r.Matrix), maxRows))
	}

	columns := len(r.Matrix[0])
	if columns == 0 {
		return domain.FactorizationInput{},
			apperror.Validation("Las filas de la matriz no pueden estar vacias.")
	}

	if columns > maxCols {
		return domain.FactorizationInput{},
			apperror.Validation(fmt.Sprintf(
				"La matriz tiene %d columnas y el maximo admitido es %d.", columns, maxCols))
	}

	for i, row := range r.Matrix {
		if len(row) != columns {
			return domain.FactorizationInput{},
				apperror.Validation("La matriz no es rectangular.").
					WithDetails(map[string]any{
						"row":      i,
						"expected": columns,
						"received": len(row),
					})
		}

		for j, value := range row {
			// JSON no puede representar NaN ni Infinity, pero si valores tan
			// grandes que al parsearse a float64 se convierten en infinito.
			// Un solo valor no finito contamina toda la factorizacion.
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return domain.FactorizationInput{},
					apperror.Validation("La matriz contiene valores no finitos.").
						WithDetails(map[string]any{"row": i, "column": j})
			}
		}
	}

	mode, err := parseMode(r.Mode)
	if err != nil {
		return domain.FactorizationInput{}, err
	}

	if r.Tolerance != nil && (*r.Tolerance < 0 || math.IsNaN(*r.Tolerance)) {
		return domain.FactorizationInput{},
			apperror.Validation("El campo \"tolerance\" debe ser un numero no negativo.")
	}

	return domain.FactorizationInput{
		Matrix:    matrix.Matrix(r.Matrix),
		Mode:      mode,
		Tolerance: r.Tolerance,
	}, nil
}

// parseMode traduce el modo recibido, aplicando el valor por defecto.
func parseMode(raw string) (matrix.Mode, error) {
	switch raw {
	case "":
		// La variante reducida es la util en la practica y evita devolver la
		// parte de Q que solo describe el complemento ortogonal: para una
		// matriz de 512x3, la Q completa serian 262144 valores frente a 1536.
		return matrix.ModeReduced, nil
	case string(matrix.ModeReduced):
		return matrix.ModeReduced, nil
	case string(matrix.ModeComplete):
		return matrix.ModeComplete, nil
	default:
		return "", apperror.Validation(fmt.Sprintf(
			"El modo %q no existe. Valores admitidos: %q, %q.",
			raw, matrix.ModeReduced, matrix.ModeComplete))
	}
}

// --- Respuesta -----------------------------------------------------------

// MatrixPayload es una matriz con sus dimensiones.
//
// Las dimensiones se envian explicitamente aunque sean deducibles del array:
// evitan que el cliente tenga que calcularlas y hacen la respuesta legible por
// si sola al inspeccionarla.
type MatrixPayload struct {
	Name    string      `json:"name"`
	Rows    int         `json:"rows"`
	Columns int         `json:"columns"`
	Data    [][]float64 `json:"data"`
}

// FactorizeResponse es el resultado completo del pipeline.
type FactorizeResponse struct {
	Matrix     MatrixPayload     `json:"matrix"`
	Q          MatrixPayload     `json:"q"`
	R          MatrixPayload     `json:"r"`
	Statistics StatisticsPayload `json:"statistics"`
}

// StatisticsPayload reexpone las estadisticas calculadas por la API de Node.
type StatisticsPayload struct {
	Global    GlobalStatisticsPayload   `json:"global"`
	PerMatrix []MatrixStatisticsPayload `json:"perMatrix"`
}

// GlobalStatisticsPayload son las metricas agregadas: las cinco que pide el
// enunciado.
type GlobalStatisticsPayload struct {
	Matrices         int      `json:"matrices"`
	Count            int      `json:"count"`
	Max              float64  `json:"max"`
	Min              float64  `json:"min"`
	Sum              float64  `json:"sum"`
	Average          float64  `json:"average"`
	IsAnyDiagonal    bool     `json:"isAnyDiagonal"`
	DiagonalMatrices []string `json:"diagonalMatrices"`
}

// MatrixStatisticsPayload son las metricas de una matriz concreta.
type MatrixStatisticsPayload struct {
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

// FactorizeMetadata acompana al resultado con informacion sobre el calculo.
type FactorizeMetadata struct {
	Mode string `json:"mode"`
	// Reflectors es el numero de reflexiones de Householder aplicadas.
	// Normalmente min(filas, columnas); solo baja cuando una columna es
	// exactamente nula. No es un estimador del rango: eso exigiria pivoteo por
	// columnas.
	Reflectors int `json:"reflectors"`
	// Tolerance es el umbral que el servicio de estadisticas aplico realmente:
	// sin el, el valor de isDiagonal no es interpretable.
	Tolerance float64 `json:"tolerance"`
	// DurationMs mide el pipeline completo, factorizacion y llamada remota
	// incluidas.
	DurationMs float64   `json:"durationMs"`
	ComputedAt time.Time `json:"computedAt"`
}

// NewFactorizeResponse traduce el resultado del dominio al formato publicado.
func NewFactorizeResponse(result domain.FactorizationResult) FactorizeResponse {
	perMatrix := make([]MatrixStatisticsPayload, 0, len(result.Statistics.PerMatrix))
	for _, m := range result.Statistics.PerMatrix {
		perMatrix = append(perMatrix, MatrixStatisticsPayload(m))
	}

	return FactorizeResponse{
		Matrix: newMatrixPayload(result.Original),
		Q:      newMatrixPayload(result.Q),
		R:      newMatrixPayload(result.R),
		Statistics: StatisticsPayload{
			Global:    GlobalStatisticsPayload(result.Statistics.Global),
			PerMatrix: perMatrix,
		},
	}
}

// NewFactorizeMetadata construye los metadatos del calculo.
func NewFactorizeMetadata(result domain.FactorizationResult, duration time.Duration) FactorizeMetadata {
	return FactorizeMetadata{
		Mode:       string(result.Mode),
		Reflectors: result.Reflectors,
		Tolerance:  result.Statistics.Tolerance,
		DurationMs: float64(duration.Microseconds()) / 1000,
		ComputedAt: result.Statistics.ComputedAt,
	}
}

func newMatrixPayload(m domain.NamedMatrix) MatrixPayload {
	return MatrixPayload{
		Name:    m.Name,
		Rows:    m.Data.Rows(),
		Columns: m.Data.Cols(),
		Data:    m.Data,
	}
}
