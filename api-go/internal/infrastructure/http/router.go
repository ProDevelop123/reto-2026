// Package http monta la aplicacion Fiber y define las rutas del servicio.
package http

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"go.uber.org/zap"

	"api-go/internal/config"
	"api-go/internal/infrastructure/http/handler"
	appmw "api-go/internal/infrastructure/http/middleware"
	appjwt "api-go/internal/infrastructure/jwt"
)

// Handlers agrupa los manejadores que el router necesita.
//
// Se pasan ya construidos desde el punto de entrada: el router decide QUE ruta
// atiende cada handler, no como se construye ninguno de ellos.
type Handlers struct {
	QR     *handler.QRHandler
	Auth   *handler.AuthHandler
	Health *handler.HealthHandler
}

// NewApp construye la aplicacion Fiber con sus middlewares y rutas.
func NewApp(
	cfg config.Config,
	handlers Handlers,
	tokens *appjwt.Service,
	logger *zap.Logger,
) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName: cfg.App.Name,
		// Un unico manejador de errores para todo el servicio: los handlers
		// devuelven el error y aqui se convierte en respuesta.
		ErrorHandler: appmw.ErrorHandler(logger),
		// Una matriz grande es una peticion legitima; un cuerpo ilimitado es un
		// vector de agotamiento de memoria.
		BodyLimit:    cfg.App.BodyLimit,
		ReadTimeout:  cfg.App.ReadTimeout,
		WriteTimeout: cfg.App.WriteTimeout,
		// No se anuncia el framework ni su version: es informacion gratuita
		// para quien busque vulnerabilidades conocidas.
		ServerHeader: "",
	})

	// recover convierte un panico en un error 500 en lugar de derribar el
	// proceso entero. Sin el, un unico fallo imprevisto en un handler tumbaria
	// el servicio para todos los usuarios.
	app.Use(recover.New())

	// Un identificador por peticion permite correlacionar en los logs todas las
	// lineas de un mismo flujo, incluida la llamada al servicio de estadisticas.
	app.Use(requestid.New())

	app.Use(cors.New(cors.Config{
		// Origenes explicitos, nunca comodin: la especificacion prohibe
		// combinar "*" con credenciales, y el frontend necesita credenciales
		// para que viaje la cookie de refresco.
		AllowOrigins: cfg.CORS.AllowOrigins,
		AllowMethods: []string{fiber.MethodGet, fiber.MethodPost, fiber.MethodOptions},
		AllowHeaders: []string{
			fiber.HeaderContentType,
			fiber.HeaderAuthorization,
			// Cabecera de verificacion de origen. Declararla aqui es lo que
			// permite que la consulta preliminar la apruebe para los origenes
			// autorizados, y solo para ellos.
			appmw.CSRFHeaderName,
		},
		AllowCredentials: true,
		MaxAge:           int(12 * 60 * 60),
	}))

	app.Use(appmw.RequestLogger(logger))

	// Sonda de vida sin autenticar: la consultan Docker y el balanceador de la
	// plataforma, que no disponen de token.
	app.Get("/health", handlers.Health.Health)

	api := app.Group("/api/v1")

	// EL ORDEN DE REGISTRO A PARTIR DE AQUI ES SIGNIFICATIVO.
	//
	// Fiber resuelve las rutas en orden de declaracion, de modo que las rutas
	// publicas deben declararse ANTES del middleware de autenticacion. Toda
	// ruta publica nueva va en este bloque; cualquier cosa declarada despues
	// del grupo protegido quedara automaticamente detras del middleware.

	// Rutas de autenticacion. Publicas por necesidad: son el punto donde se
	// obtienen las credenciales que protegen al resto.
	auth := api.Group("/auth")

	// El login se autentica con lo que viaja en el cuerpo, no con una cookie,
	// asi que no es susceptible de CSRF: un atacante que pudiera enviar
	// credenciales validas no necesitaria enganar a nadie.
	auth.Post("/login", handlers.Auth.Login)

	// Refresh y logout SI se autentican con la cookie, de modo que el navegador
	// la adjunta venga la peticion de donde venga. Exigir una cabecera propia
	// obliga a pasar antes por la consulta preliminar de CORS, que rechaza los
	// origenes no autorizados. Ver csrf.go.
	auth.Post("/refresh", appmw.RequireCSRFHeader(), handlers.Auth.Refresh)
	auth.Post("/logout", appmw.RequireCSRFHeader(), handlers.Auth.Logout)

	// Rutas protegidas. El middleware se aplica al GRUPO y no ruta por ruta, de
	// modo que una ruta nueva nace protegida por omision: olvidar anadir el
	// middleware deja de ser un fallo de seguridad posible, que es el error mas
	// facil de cometer y el mas caro de detectar.
	//
	// Efecto secundario asumido: como el middleware cubre todo el prefijo
	// /api/v1, una ruta inexistente bajo el responde 401 en lugar de 404. Es
	// deliberado —no se revela que rutas existen a quien no se ha
	// autenticado—; fuera de ese prefijo el 404 funciona con normalidad.
	protected := api.Group("", appmw.Auth(tokens))
	protected.Post("/qr", handlers.QR.Factorize)

	return app
}
