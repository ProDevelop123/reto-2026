package middleware

import (
	"errors"

	"github.com/gofiber/fiber/v3"
)

// asFiberError comprueba si el error proviene del propio Fiber.
//
// Se aisla en su propia funcion para dejar claro el orden de comprobacion en
// ErrorHandler: primero los errores del framework, despues los de la
// aplicacion. Invertirlo haria que un 404 de ruta desconocida se reportara
// como error interno.
func asFiberError(err error, target **fiber.Error) bool {
	return errors.As(err, target)
}
