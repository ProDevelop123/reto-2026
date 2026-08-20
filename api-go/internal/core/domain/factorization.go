// Package domain define los conceptos del negocio del servicio y su
// vocabulario.
//
// No importa nada de infraestructura: ni HTTP, ni Fiber, ni JSON, ni
// configuracion. Los tipos de este paquete describen QUE hace el sistema, no
// COMO se transporta ni donde se ejecuta.
package domain

import "api-go/internal/matrix"

// NamedMatrix es una matriz acompanada de una etiqueta que permite seguirle la
// pista a lo largo del sistema.
//
// El nombre importa porque el resultado atraviesa dos servicios: sin el, la
// respuesta de estadisticas diria "alguna matriz es diagonal" sin poder decir
// cual, y el frontend no podria emparejar cada bloque de metricas con su matriz.
type NamedMatrix struct {
	Name string
	Data matrix.Matrix
}

// FactorizationInput es la peticion de factorizar una matriz.
type FactorizationInput struct {
	// Matrix es la matriz original A que se desea descomponer.
	Matrix matrix.Matrix
	// Mode selecciona la variante reducida o completa de la descomposicion.
	Mode matrix.Mode
	// Tolerance es el umbral opcional con el que el servicio de estadisticas
	// decidira si una matriz es diagonal. Es un puntero porque cero es un
	// valor legitimo ("comparacion exacta") y hay que poder distinguirlo de
	// "no se ha indicado ninguno".
	Tolerance *float64
}

// FactorizationResult reune todo lo que produce el flujo completo: la
// descomposicion calculada aqui y las estadisticas calculadas por el servicio
// de Node.
//
// Se devuelven juntas de forma deliberada. El enunciado admitiria devolver solo
// las estadisticas, pero entonces el trabajo algebraico quedaria invisible y el
// frontend necesitaria una segunda llamada para mostrar Q y R. Un unico viaje
// que devuelve todo el contexto es mas util y demuestra la integracion entre
// ambos servicios de forma tangible.
type FactorizationResult struct {
	Original   NamedMatrix
	Q          NamedMatrix
	R          NamedMatrix
	Mode       matrix.Mode
	Reflectors int
	Statistics StatisticsResult
}
