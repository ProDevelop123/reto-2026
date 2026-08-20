package middleware

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"go.uber.org/zap"
)

// RequestLogger registra cada peticion atendida en formato estructurado.
//
// Se escribe a medida en lugar de usar el logger que trae Fiber para emitir
// JSON con zap, que es lo que esperan los recolectores de logs de las
// plataformas de contenedores. El logger por defecto produce texto pensado
// para leerse en una terminal.
//
// Nunca se registra el cuerpo de la peticion ni la cabecera Authorization: el
// cuerpo puede contener matrices de cientos de miles de valores y la cabecera
// contiene un token valido, que en un log es una credencial filtrada.
func RequestLogger(logger *zap.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		started := time.Now()

		err := c.Next()

		fields := []zap.Field{
			zap.String("requestId", requestid.FromContext(c)),
			zap.String("method", c.Method()),
			zap.String("path", c.Path()),
			zap.Int("status", c.Response().StatusCode()),
			zap.Duration("latency", time.Since(started)),
			zap.String("ip", c.IP()),
		}

		// El usuario autenticado se anade cuando existe, para poder atribuir la
		// peticion sin necesidad de descifrar el token desde el log.
		if username, ok := c.Locals(LocalUsername).(string); ok && username != "" {
			fields = append(fields, zap.String("username", username))
		}

		logger.Info("peticion HTTP", fields...)

		return err
	}
}
