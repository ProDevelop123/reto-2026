// Package port declara las interfaces que el nucleo necesita del exterior.
//
// Aqui esta el motivo por el que este servicio usa arquitectura hexagonal y la
// API de Node no: esta API DEPENDE de servicios externos. Declarando esas
// dependencias como interfaces que pertenecen al nucleo, la direccion de la
// dependencia se invierte: la infraestructura implementa lo que el nucleo
// define, y no al reves.
//
// La consecuencia practica es concreta y verificable: el caso de uso de
// factorizacion se testea al completo con un doble en memoria, sin levantar la
// API de Node ni abrir un socket. Si manana el servicio de estadisticas se
// consumiera por gRPC o por una cola de mensajes, cambiaria el adaptador y el
// nucleo no se tocaria.
package port

import (
	"context"
	"time"

	"api-go/internal/core/domain"
)

// StatisticsProvider calcula estadisticas sobre un conjunto de matrices.
//
// El nucleo no sabe —ni debe saber— que detras hay una API en Node hablando
// HTTP. Solo sabe que existe algo capaz de analizar matrices.
type StatisticsProvider interface {
	Analyze(ctx context.Context, request domain.StatisticsRequest) (domain.StatisticsResult, error)
}

// SessionRepository almacena los refresh tokens vigentes.
//
// La implementacion incluida guarda las sesiones en memoria, lo que basta para
// este servicio: no hay requisito de persistencia entre reinicios ni de
// compartir estado entre instancias. Sustituirla por Redis para un despliegue
// multi-instancia significa escribir un adaptador nuevo; esta interfaz y todo
// el nucleo permanecen intactos.
type SessionRepository interface {
	// Save registra una sesion nueva.
	Save(ctx context.Context, session domain.Session) error

	// Find recupera una sesion por su identificador. Devuelve ErrSessionNotFound
	// si no existe.
	Find(ctx context.Context, id string) (domain.Session, error)

	// MarkUsed marca la sesion como canjeada, sin borrarla. Conservarla es lo
	// que permite despues detectar su reutilizacion.
	MarkUsed(ctx context.Context, id string, usedAt time.Time) error

	// RevokeFamily elimina todas las sesiones de una misma familia. Se invoca
	// al cerrar sesion y al detectar la reutilizacion de un token ya canjeado.
	RevokeFamily(ctx context.Context, familyID string) error
}

// TokenIssuer emite y verifica los tokens del sistema.
//
// El nucleo no debe saber si por debajo hay JWT firmados con RSA, tokens
// opacos contra un almacen o cualquier otra cosa. Solo necesita poder emitir
// un token de acceso, uno de refresco, y leer los datos de este ultimo.
type TokenIssuer interface {
	// IssueAccess emite un token de acceso de vida corta.
	IssueAccess(username string) (string, time.Time, error)

	// IssueRefresh emite un token de refresco ligado a una sesion y a su familia.
	IssueRefresh(username, sessionID, familyID string) (string, time.Time, error)

	// ParseRefresh verifica un token de refresco y devuelve sus datos.
	ParseRefresh(token string) (domain.RefreshClaims, error)
}

// Clock abstrae el paso del tiempo.
//
// Existe para que los tests puedan comprobar la expiracion de tokens sin
// esperar de verdad: un test que dependa del reloj real seria lento y, peor
// aun, intermitente.
type Clock interface {
	Now() time.Time
}

// IDGenerator produce identificadores unicos e impredecibles para los tokens.
//
// Se abstrae por la misma razon que el reloj: permite fijar los identificadores
// en los tests y hacer las aserciones deterministas.
type IDGenerator interface {
	NewID() (string, error)
}
