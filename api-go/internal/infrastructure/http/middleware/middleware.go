// Package middleware contiene los middlewares HTTP propios del servicio.
package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"

	"api-go/internal/infrastructure/client"
	"api-go/internal/infrastructure/http/dto"
	appjwt "api-go/internal/infrastructure/jwt"
	"api-go/pkg/apperror"
)

// Claves con las que se publican los datos de autenticacion en el contexto de
// la peticion, para que los handlers posteriores los consulten.
const (
	// LocalUsername es el usuario autenticado.
	LocalUsername = "username"
	// LocalAccessToken es el token en bruto, necesario para propagarlo al
	// servicio de estadisticas.
	LocalAccessToken = "accessToken"
)

const bearerPrefix = "Bearer "

// Auth exige un token de acceso valido en la cabecera Authorization.
func Auth(tokens *appjwt.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		header := c.Get(fiber.HeaderAuthorization)

		if header == "" {
			return apperror.Unauthorized("Falta la cabecera Authorization.").
				WithDetails(map[string]string{"reason": "missing_header"})
		}

		if !strings.HasPrefix(header, bearerPrefix) {
			return apperror.Unauthorized("Formato invalido, se espera \"Bearer <token>\".").
				WithDetails(map[string]string{"reason": "malformed_header"})
		}

		raw := strings.TrimSpace(strings.TrimPrefix(header, bearerPrefix))

		claims, err := tokens.ParseAccess(raw)
		if err != nil {
			return err
		}

		// Fiber reutiliza el contexto de la peticion entre peticiones, asi que
		// las cadenas obtenidas de el solo son validas mientras dura el
		// handler. Se copian con strings.Clone antes de guardarlas para que no
		// puedan quedar apuntando a los datos de otra peticion.
		c.Locals(LocalUsername, strings.Clone(claims.Subject))
		c.Locals(LocalAccessToken, strings.Clone(raw))

		// El token se adjunta al contexto estandar para que el adaptador del
		// servicio de estadisticas pueda propagarlo aguas arriba, manteniendo
		// la identidad del usuario en la llamada interna.
		c.SetContext(client.WithAccessToken(c.Context(), raw))

		return c.Next()
	}
}

// ErrorHandler traduce cualquier error devuelto por un handler a la respuesta
// HTTP correspondiente.
//
// Es el unico punto del servicio donde un error se convierte en respuesta. Los
// handlers se limitan a devolver el error y no construyen respuestas de fallo,
// lo que garantiza que todos los errores tengan exactamente la misma forma.
func ErrorHandler(logger *zap.Logger) fiber.ErrorHandler {
	return func(c fiber.Ctx, err error) error {
		// Fiber senala con *fiber.Error sus propios fallos (404 de ruta
		// desconocida, 413 de cuerpo excesivo). Se traducen al formato del
		// servicio para que el cliente no reciba dos formas de error distintas.
		var fiberErr *fiber.Error
		if ok := asFiberError(err, &fiberErr); ok {
			appErr := apperror.New(fiberErr.Code, codeForStatus(fiberErr.Code), fiberErr.Message)
			return respond(c, logger, appErr)
		}

		return respond(c, logger, apperror.From(err))
	}
}

// respond registra el error y escribe la respuesta.
func respond(c fiber.Ctx, logger *zap.Logger, appErr *apperror.AppError) error {
	fields := []zap.Field{
		zap.String("code", string(appErr.Code)),
		zap.Int("status", appErr.Status),
		zap.String("method", c.Method()),
		zap.String("path", c.Path()),
	}

	// La causa interna se registra pero nunca se serializa hacia el cliente:
	// puede contener rutas de fichero, nombres de host internos o mensajes de
	// la biblioteca estandar que no deben salir del sistema.
	if cause := appErr.Unwrap(); cause != nil {
		fields = append(fields, zap.NamedError("cause", cause))
	}

	if appErr.Status >= 500 {
		logger.Error(appErr.Message, fields...)
	} else {
		logger.Warn(appErr.Message, fields...)
	}

	return c.Status(appErr.Status).JSON(dto.FromAppError(appErr))
}

// codeForStatus asigna un codigo de aplicacion a los errores que genera Fiber.
func codeForStatus(status int) apperror.Code {
	switch {
	case status == fiber.StatusNotFound:
		return apperror.CodeNotFound
	case status == fiber.StatusUnauthorized:
		return apperror.CodeUnauthorized
	case status >= 500:
		return apperror.CodeInternal
	default:
		return apperror.CodeBadRequest
	}
}
