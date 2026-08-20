// Command api es el punto de entrada de la API en Go del reto.
//
// Recibe una matriz rectangular, calcula su factorizacion QR mediante
// reflexiones de Householder, envia las matrices resultantes a la API de
// estadisticas en Node y devuelve todo el resultado en una sola respuesta.
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"api-go/internal/config"
	"api-go/internal/core/domain"
	"api-go/internal/core/usecase"
	"api-go/internal/infrastructure/client"
	apphttp "api-go/internal/infrastructure/http"
	"api-go/internal/infrastructure/http/handler"
	appjwt "api-go/internal/infrastructure/jwt"
	"api-go/internal/infrastructure/logger"
	"api-go/internal/infrastructure/persistence/memory"
	"api-go/internal/infrastructure/system"
)

// version identifica la build. Se sobreescribe en el enlazado con
// -ldflags "-X main.version=..." para que la imagen publicada declare que
// contiene exactamente.
var version = "dev"

// sessionPurgeInterval es cada cuanto se limpian las sesiones caducadas del
// almacen en memoria. Sin esta limpieza el mapa creceria indefinidamente.
const sessionPurgeInterval = time.Hour

func main() {
	// El mismo binario sirve la API y sabe sondearla. Ver healthcheck.go: la
	// imagen final es distroless y no dispone de shell ni de curl con los que
	// comprobar el estado del servicio.
	if len(os.Args) > 1 && os.Args[1] == healthcheckCommand {
		os.Exit(runHealthcheck())
	}

	if err := run(); err != nil {
		// Se usa el log estandar porque un fallo de arranque puede ocurrir
		// antes de que exista el logger estructurado.
		log.Fatalf("el servicio no pudo arrancar: %v", err)
	}
}

// run concentra el arranque y devuelve error en lugar de terminar el proceso.
//
// Separarlo de main permite que todas las llamadas a defer se ejecuten: un
// log.Fatal dentro de main las saltaria, dejando el logger sin vaciar.
func run() error {
	// El fichero .env es una comodidad de desarrollo. En contenedores y en la
	// nube las variables llegan del entorno, asi que su ausencia no es un error.
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	appLogger, err := logger.New(cfg.App.Env, cfg.App.LogLevel)
	if err != nil {
		return err
	}
	defer func() { _ = appLogger.Sync() }()

	// --- Composicion de dependencias -------------------------------------
	//
	// Este es el unico lugar del servicio donde se decide QUE implementacion
	// concreta cumple cada puerto. El nucleo solo conoce las interfaces, de
	// modo que sustituir el almacen en memoria por Redis, o el cliente HTTP
	// por uno de gRPC, se reduce a cambiar una linea aqui.

	clock := system.NewClock()
	ids := system.NewIDGenerator()

	tokens, err := appjwt.New(cfg.JWT, clock)
	if err != nil {
		return err
	}

	sessions := memory.NewSessionRepository()
	statistics := client.New(cfg.Statistics)

	qrUseCase := usecase.NewQRUseCase(statistics)
	authUseCase := usecase.NewAuthUseCase(
		domain.Credentials{Username: cfg.Auth.Username, Password: cfg.Auth.Password},
		tokens, sessions, clock, ids,
	)

	app := apphttp.NewApp(cfg, apphttp.Handlers{
		QR:     handler.NewQRHandler(qrUseCase, clock),
		Auth:   handler.NewAuthHandler(authUseCase, cfg.Cookie, clock),
		Health: handler.NewHealthHandler(cfg.App.Name, version, clock),
	}, tokens, appLogger)

	// --- Ciclo de vida ----------------------------------------------------

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go purgeExpiredSessions(ctx, sessions, clock, appLogger)

	serverErrors := make(chan error, 1)

	go func() {
		appLogger.Info("API en Go escuchando",
			zap.String("port", cfg.App.Port),
			zap.String("env", cfg.App.Env),
			zap.String("version", version),
			zap.String("statisticsApi", cfg.Statistics.BaseURL),
		)

		if err := app.Listen(":"+cfg.App.Port, listenConfig(cfg.App)); err != nil {
			serverErrors <- err
		}
	}()

	select {
	case err := <-serverErrors:
		return err

	case <-ctx.Done():
		// Cierre ordenado: se dejan de aceptar conexiones nuevas y se espera a
		// que terminen las peticiones en curso. Sin esto, un redespliegue
		// cortaria peticiones a medio responder.
		appLogger.Info("senal de parada recibida, cerrando el servidor")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := app.ShutdownWithContext(shutdownCtx); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				appLogger.Warn("el cierre ordenado agoto su plazo, se fuerza la salida")
				return nil
			}
			return err
		}

		appLogger.Info("servidor cerrado correctamente")
		return nil
	}
}

// purgeExpiredSessions limpia periodicamente las sesiones caducadas.
//
// Se ejecuta en su propia goroutine y termina cuando el contexto se cancela,
// de modo que no impide el cierre ordenado del proceso.
func purgeExpiredSessions(
	ctx context.Context,
	sessions *memory.SessionRepository,
	clock system.Clock,
	appLogger *zap.Logger,
) {
	ticker := time.NewTicker(sessionPurgeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if removed := sessions.PurgeExpired(clock.Now()); removed > 0 {
				appLogger.Debug("sesiones caducadas eliminadas", zap.Int("count", removed))
			}
		}
	}
}

// listenConfig configura la escucha del servidor.
//
// El banner de arranque de Fiber es util en una terminal de desarrollo, pero
// en produccion contamina los logs estructurados con texto decorativo que el
// recolector no sabe interpretar.
func listenConfig(cfg config.AppConfig) fiber.ListenConfig {
	return fiber.ListenConfig{DisableStartupMessage: cfg.IsProduction()}
}
