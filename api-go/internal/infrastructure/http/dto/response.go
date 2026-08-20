// Package dto define las estructuras de entrada y salida del borde HTTP.
//
// Se mantienen separadas de los tipos del dominio a proposito: el JSON que se
// publica es un contrato con terceros que debe poder evolucionar de forma
// independiente del modelo interno. Sin esta separacion, renombrar un campo
// del dominio romperia silenciosamente a todos los clientes.
package dto

import "api-go/pkg/apperror"

// SuccessResponse es el sobre comun a todas las respuestas correctas.
//
// Tanto esta API como la de Node emplean exactamente el mismo sobre pese a
// estar escritas en lenguajes distintos: el contrato pertenece al sistema, no
// al framework. El frontend puede escribir un unico interceptor que sepa
// desenvolver las respuestas de ambos servicios.
type SuccessResponse struct {
	Success bool `json:"success"`
	Data    any  `json:"data"`
	// Metadata transporta informacion sobre el calculo (tiempos, parametros
	// aplicados) sin mezclarla con el resultado en si.
	Metadata any `json:"metadata,omitempty"`
}

// ErrorResponse es el sobre comun a todas las respuestas de error.
type ErrorResponse struct {
	Success bool      `json:"success"`
	Error   ErrorBody `json:"error"`
}

// ErrorBody describe el fallo.
type ErrorBody struct {
	// Code es estable y legible por maquina: el cliente puede ramificar sobre
	// el sin depender del texto del mensaje, pensado para humanos.
	Code apperror.Code `json:"code"`
	// Message es la descripcion legible.
	Message string `json:"message"`
	// Details es informacion adicional segura de exponer. Nunca contiene la
	// causa interna del error.
	Details any `json:"details,omitempty"`
}

// Success construye una respuesta correcta.
func Success(data any) SuccessResponse {
	return SuccessResponse{Success: true, Data: data}
}

// SuccessWithMetadata construye una respuesta correcta con metadatos.
func SuccessWithMetadata(data, metadata any) SuccessResponse {
	return SuccessResponse{Success: true, Data: data, Metadata: metadata}
}

// FromAppError construye la respuesta de error a partir de un AppError.
func FromAppError(err *apperror.AppError) ErrorResponse {
	return ErrorResponse{
		Success: false,
		Error: ErrorBody{
			Code:    err.Code,
			Message: err.Message,
			Details: err.Details,
		},
	}
}
