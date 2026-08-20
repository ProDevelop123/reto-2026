// Package config centraliza la lectura y validacion de la configuracion del
// servicio.
//
// Toda la configuracion entra por variables de entorno, sin valores sensibles
// codificados en el binario. Se valida en el arranque y el proceso aborta si
// algo es incoherente: un contenedor que no levanta es un problema visible,
// uno que sirve peticiones en un estado inseguro no lo es.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config agrupa toda la configuracion del servicio.
type Config struct {
	App        AppConfig
	Statistics StatisticsConfig
	JWT        JWTConfig
	Auth       AuthConfig
	CORS       CORSConfig
	Cookie     CookieConfig
}

// AppConfig cubre los parametros generales del proceso.
type AppConfig struct {
	Name         string
	Env          string
	Port         string
	LogLevel     string
	BodyLimit    int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// IsProduction indica si el servicio corre en un entorno productivo.
func (a AppConfig) IsProduction() bool { return a.Env == "production" }

// StatisticsConfig describe como alcanzar la API de estadisticas en Node.
type StatisticsConfig struct {
	BaseURL string
	// Timeout acota cada INTENTO individual. Sin el, una dependencia lenta
	// agotaria las goroutines del servidor y arrastraria a esta API en su caida.
	Timeout time.Duration

	// TotalTimeout acota la operacion COMPLETA, reintentos incluidos.
	//
	// No es redundante con Timeout: sin un presupuesto global, el tiempo maximo
	// de espera es Timeout x (MaxRetries+1) mas las esperas entre reintentos, de
	// modo que subir los reintentos alarga en silencio la peor latencia que ve
	// el cliente. Se detecto probando en contenedores: cuando el servicio remoto
	// esta caido, su nombre deja de resolverse en el DNS interno y cada intento
	// consume el timeout completo esperando a la resolucion, en lugar de fallar
	// al instante como ocurre en local con una conexion rechazada.
	TotalTimeout time.Duration
	// MaxRetries limita los reintentos ante fallos transitorios. Solo se
	// reintenta lo que puede tener exito al repetirse (errores de red y 5xx);
	// un 4xx significa que la peticion es incorrecta y repetirla es inutil.
	MaxRetries int
	// RetryBackoff es la espera base entre reintentos, que crece de forma
	// exponencial para no castigar a un servicio que ya esta sufriendo.
	RetryBackoff time.Duration
}

// JWTConfig describe la emision y verificacion de tokens.
type JWTConfig struct {
	// PrivateKey firma los tokens. Solo este servicio la posee.
	PrivateKeyPEM []byte
	// PublicKey los verifica. Se comparte con la API en Node y con el frontend.
	PublicKeyPEM []byte

	Issuer     string
	Audience   string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

// AuthConfig contiene las credenciales estaticas del login simulado.
//
// El reto no exige un almacen de usuarios, y montar una base de datos solo
// para validar un par de credenciales anadiria infraestructura sin demostrar
// nada nuevo. Se opta por credenciales de entorno, dejando claro que el punto
// a demostrar es la emision y verificacion de JWT, no la gestion de usuarios.
type AuthConfig struct {
	Username string
	Password string
}

// CORSConfig describe la politica de origen cruzado.
type CORSConfig struct {
	// AllowOrigins debe enumerar los origenes de forma explicita. El comodin
	// "*" es incompatible con AllowCredentials segun la especificacion, y el
	// frontend necesita credenciales para enviar la cookie de refresco.
	AllowOrigins []string
}

// CookieConfig describe la cookie que transporta el refresh token.
//
// El refresh token viaja en una cookie HttpOnly y nunca es accesible desde
// JavaScript, de modo que un XSS en el frontend no puede robarlo. El access
// token, en cambio, vive solo en memoria del cliente y es de vida corta.
type CookieConfig struct {
	Name     string
	Path     string
	Domain   string
	Secure   bool
	HTTPOnly bool
	// SameSite condiciona por completo el despliegue: con el frontend en
	// Vercel y las APIs en GCP, la peticion de refresco es de origen cruzado y
	// el navegador solo enviara la cookie si vale "None". Y "None" exige
	// Secure, que a su vez exige HTTPS.
	SameSite string
}

// Load lee la configuracion del entorno y la valida.
func Load() (Config, error) {
	cfg := Config{
		App: AppConfig{
			Name:         env("APP_NAME", "reto-2026-api-go"),
			Env:          env("APP_ENV", "development"),
			Port:         env("PORT", "8080"),
			LogLevel:     env("LOG_LEVEL", "debug"),
			BodyLimit:    envInt("BODY_LIMIT_BYTES", 5*1024*1024),
			ReadTimeout:  envDuration("READ_TIMEOUT", 15*time.Second),
			WriteTimeout: envDuration("WRITE_TIMEOUT", 30*time.Second),
		},
		Statistics: StatisticsConfig{
			BaseURL:      env("STATISTICS_API_URL", "http://localhost:3001"),
			Timeout:      envDuration("STATISTICS_TIMEOUT", 3*time.Second),
			TotalTimeout: envDuration("STATISTICS_TOTAL_TIMEOUT", 8*time.Second),
			MaxRetries:   envInt("STATISTICS_MAX_RETRIES", 2),
			RetryBackoff: envDuration("STATISTICS_RETRY_BACKOFF", 100*time.Millisecond),
		},
		JWT: JWTConfig{
			Issuer:     env("JWT_ISSUER", "reto-2026-api-go"),
			Audience:   env("JWT_AUDIENCE", "reto-2026"),
			AccessTTL:  envDuration("JWT_ACCESS_TTL", 15*time.Minute),
			RefreshTTL: envDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
		},
		Auth: AuthConfig{
			Username: env("AUTH_USERNAME", "admin"),
			Password: env("AUTH_PASSWORD", "admin123"),
		},
		CORS: CORSConfig{
			AllowOrigins: envList("CORS_ORIGINS", []string{"http://localhost:5173"}),
		},
		Cookie: CookieConfig{
			Name:     env("REFRESH_COOKIE_NAME", "refresh_token"),
			Path:     env("REFRESH_COOKIE_PATH", "/api/v1/auth"),
			Domain:   env("REFRESH_COOKIE_DOMAIN", ""),
			Secure:   envBool("REFRESH_COOKIE_SECURE", isProduction()),
			HTTPOnly: true,
			SameSite: env("REFRESH_COOKIE_SAMESITE", defaultSameSite()),
		},
	}

	privateKey, err := readKey("JWT_PRIVATE_KEY", "JWT_PRIVATE_KEY_PATH", "../keys/private.pem")
	if err != nil {
		return Config{}, fmt.Errorf("clave privada JWT: %w", err)
	}
	cfg.JWT.PrivateKeyPEM = privateKey

	publicKey, err := readKey("JWT_PUBLIC_KEY", "JWT_PUBLIC_KEY_PATH", "../keys/public.pem")
	if err != nil {
		return Config{}, fmt.Errorf("clave publica JWT: %w", err)
	}
	cfg.JWT.PublicKeyPEM = publicKey

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// validate comprueba las invariantes que hacen segura y coherente la
// configuracion antes de aceptar trafico.
func (c Config) validate() error {
	if c.Statistics.BaseURL == "" {
		return errors.New("STATISTICS_API_URL es obligatoria")
	}

	// Un presupuesto global menor que el de un solo intento cortaria la primera
	// llamada antes de que pudiera completarse, dejando el servicio inutil.
	if c.Statistics.TotalTimeout < c.Statistics.Timeout {
		return errors.New(
			"STATISTICS_TOTAL_TIMEOUT no puede ser menor que STATISTICS_TIMEOUT")
	}

	if !c.App.IsProduction() {
		return nil
	}

	// A partir de aqui, comprobaciones exclusivas de produccion.
	if c.Auth.Password == "admin123" {
		return errors.New("AUTH_PASSWORD conserva el valor por defecto en produccion")
	}

	// SameSite=None sin Secure hace que el navegador descarte la cookie sin
	// avisar: el login pareceria funcionar y el refresco fallaria siempre.
	if strings.EqualFold(c.Cookie.SameSite, "none") && !c.Cookie.Secure {
		return errors.New("REFRESH_COOKIE_SAMESITE=None exige REFRESH_COOKIE_SECURE=true")
	}

	for _, origin := range c.CORS.AllowOrigins {
		if origin == "*" {
			return errors.New(
				"CORS_ORIGINS no admite el comodin: es incompatible con el envio de credenciales")
		}
	}

	return nil
}

// defaultSameSite elige la politica de cookie segun el entorno.
//
// En produccion el frontend (Vercel) y la API (GCP) estan en dominios
// distintos, por lo que la peticion de refresco es de origen cruzado y solo
// "None" permite que la cookie viaje. En desarrollo, con todo en localhost,
// "Lax" es suficiente y ademas funciona sin HTTPS.
func defaultSameSite() string {
	if isProduction() {
		return "None"
	}
	return "Lax"
}

func isProduction() bool { return env("APP_ENV", "development") == "production" }

// readKey resuelve una clave PEM desde una variable inline o desde un fichero.
//
// La variante inline existe para plataformas serverless como Cloud Run, donde
// montar ficheros es incomodo y los secretos se inyectan como variables de
// entorno. Se acepta ademas codificada en base64 porque muchos gestores de
// secretos no conservan los saltos de linea del PEM.
func readKey(inlineVar, pathVar, defaultPath string) ([]byte, error) {
	if inline := os.Getenv(inlineVar); inline != "" {
		if strings.Contains(inline, "BEGIN") {
			return []byte(inline), nil
		}
		decoded, err := decodeBase64(inline)
		if err != nil {
			return nil, fmt.Errorf("%s no es un PEM ni base64 valido: %w", inlineVar, err)
		}
		return decoded, nil
	}

	path := env(pathVar, defaultPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer %q (defina %s o %s): %w",
			path, inlineVar, pathVar, err)
	}

	return data, nil
}

// decodeBase64 acepta tanto la codificacion estandar como la variante segura
// para URL, con y sin relleno: distintos gestores de secretos usan variantes
// distintas y fallar por el tipo de relleno seria un fallo de despliegue
// desconcertante.
func decodeBase64(value string) ([]byte, error) {
	value = strings.TrimSpace(value)

	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}

	var lastErr error
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(value)
		if err == nil {
			return decoded, nil
		}
		lastErr = err
	}

	return nil, lastErr
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(env(key, ""))
	if err != nil {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value, err := strconv.ParseBool(env(key, ""))
	if err != nil {
		return fallback
	}
	return value
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(env(key, ""))
	if err != nil {
		return fallback
	}
	return value
}

func envList(key string, fallback []string) []string {
	raw := env(key, "")
	if raw == "" {
		return fallback
	}

	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}
