// Package apperror define el error de aplicacion que atraviesa todas las capas
// del servicio.
//
// La idea es que cualquier capa pueda lanzar un error semantico
// (apperror.BadRequest(...)) y que un unico manejador en el borde HTTP lo
// traduzca a una respuesta con el codigo y el formato correctos. Ninguna capa
// interna necesita conocer codigos HTTP ni construir respuestas de error.
//
// El conjunto de codigos coincide deliberadamente con el de la API en Node,
// de modo que ambos servicios producen errores con la misma forma pese a estar
// escritos en lenguajes distintos: el contrato es del sistema, no del framework.
package apperror

import (
	"errors"
	"fmt"
	"net/http"
)

// Code identifica la clase de error de forma estable y legible por maquina.
// El cliente puede ramificar sobre el sin depender del texto del mensaje, que
// esta pensado para humanos y puede cambiar.
type Code string

const (
	CodeBadRequest   Code = "BAD_REQUEST"
	CodeValidation   Code = "VALIDATION_ERROR"
	CodeUnauthorized Code = "UNAUTHORIZED"
	CodeForbidden    Code = "FORBIDDEN"
	CodeNotFound     Code = "NOT_FOUND"
	CodeUpstream     Code = "UPSTREAM_ERROR"
	CodeTimeout      Code = "UPSTREAM_TIMEOUT"
	CodeInternal     Code = "INTERNAL_ERROR"
)

// AppError es un error con codigo HTTP y detalles seguros de exponer.
//
// El campo cause conserva el error original para el log, pero NUNCA se
// serializa hacia el cliente: puede contener rutas de fichero, nombres de host
// internos o mensajes de la biblioteca estandar que no deben salir del sistema.
type AppError struct {
	Status  int
	Code    Code
	Message string
	Details any
	cause   error
}

// Error implementa la interfaz error.
func (e *AppError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap permite que errors.Is y errors.As atraviesen el AppError hasta la
// causa original.
func (e *AppError) Unwrap() error { return e.cause }

// WithDetails devuelve una copia del error con detalles adicionales.
// Se devuelve una copia en lugar de mutar para que los errores centinela
// declarados a nivel de paquete no puedan contaminarse entre peticiones.
func (e *AppError) WithDetails(details any) *AppError {
	clone := *e
	clone.Details = details
	return &clone
}

// WithCause devuelve una copia del error asociada a una causa subyacente.
func (e *AppError) WithCause(cause error) *AppError {
	clone := *e
	clone.cause = cause
	return &clone
}

// New construye un AppError.
func New(status int, code Code, message string) *AppError {
	return &AppError{Status: status, Code: code, Message: message}
}

// BadRequest indica que la peticion esta mal formada (400).
func BadRequest(message string) *AppError {
	return New(http.StatusBadRequest, CodeBadRequest, message)
}

// Validation indica que la peticion es sintacticamente valida pero
// semanticamente incorrecta (422).
func Validation(message string) *AppError {
	return New(http.StatusUnprocessableEntity, CodeValidation, message)
}

// Unauthorized indica falta de credenciales validas (401).
func Unauthorized(message string) *AppError {
	return New(http.StatusUnauthorized, CodeUnauthorized, message)
}

// NotFound indica que el recurso no existe (404).
func NotFound(message string) *AppError {
	return New(http.StatusNotFound, CodeNotFound, message)
}

// Upstream indica que un servicio del que dependemos ha fallado (502).
//
// Se distingue de Internal a proposito: 502 le dice al cliente que su peticion
// era correcta y que el fallo esta en una dependencia nuestra, lo que hace
// razonable reintentar. Un 500 no transmite esa informacion.
func Upstream(message string) *AppError {
	return New(http.StatusBadGateway, CodeUpstream, message)
}

// Timeout indica que una dependencia no respondio a tiempo (504).
func Timeout(message string) *AppError {
	return New(http.StatusGatewayTimeout, CodeTimeout, message)
}

// Internal indica un fallo no previsto (500).
func Internal(message string) *AppError {
	return New(http.StatusInternalServerError, CodeInternal, message)
}

// From extrae el AppError de una cadena de errores.
//
// Si el error no proviene de este paquete se envuelve como error interno: se
// prefiere un 500 generico a filtrar al cliente el mensaje de un error
// inesperado, que puede exponer detalles de la implementacion.
func From(err error) *AppError {
	if err == nil {
		return nil
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}

	return Internal("Ha ocurrido un error interno.").WithCause(err)
}
