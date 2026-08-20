// Package memory contiene los adaptadores de persistencia en memoria.
package memory

import (
	"context"
	"sync"
	"time"

	"api-go/internal/core/domain"
)

// SessionRepository guarda las sesiones en memoria del proceso.
// Implementa port.SessionRepository.
//
// Es una decision consciente y acotada: el reto no exige persistencia entre
// reinicios ni compartir estado entre instancias, y montar una base de datos
// para almacenar refresh tokens anadiria infraestructura sin demostrar nada
// que no quede ya demostrado.
//
// Lo importante es que la decision es REVERSIBLE sin tocar el nucleo: para un
// despliegue con varias instancias basta escribir un adaptador contra Redis
// que implemente la misma interfaz. Esa sustituibilidad es precisamente lo que
// justifica la arquitectura hexagonal en este servicio.
//
// Limitaciones asumidas y documentadas:
//   - Un reinicio invalida todas las sesiones. Los usuarios vuelven a
//     autenticarse; no se pierde ningun dato de negocio.
//   - Con varias replicas, el refresco solo funcionaria contra la instancia
//     que emitio el token.
type SessionRepository struct {
	// RWMutex y no Mutex: las lecturas (Find) son mucho mas frecuentes que las
	// escrituras y pueden ocurrir en paralelo sin bloquearse entre si.
	mu       sync.RWMutex
	sessions map[string]domain.Session
}

// NewSessionRepository construye el almacen.
func NewSessionRepository() *SessionRepository {
	return &SessionRepository{sessions: make(map[string]domain.Session)}
}

// Save registra una sesion.
func (r *SessionRepository) Save(_ context.Context, session domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sessions[session.ID] = session
	return nil
}

// Find recupera una sesion por su identificador.
func (r *SessionRepository) Find(_ context.Context, id string) (domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	session, ok := r.sessions[id]
	if !ok {
		return domain.Session{}, domain.ErrSessionNotFound
	}

	return session, nil
}

// MarkUsed marca la sesion como canjeada sin eliminarla.
//
// Conservarla es imprescindible: si se borrara al rotar, la reutilizacion de
// un token robado seria indistinguible de un token inexistente y se perderia
// la senal de que hay una copia en circulacion.
func (r *SessionRepository) MarkUsed(_ context.Context, id string, usedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.sessions[id]
	if !ok {
		return domain.ErrSessionNotFound
	}

	session.UsedAt = &usedAt
	r.sessions[id] = session

	return nil
}

// RevokeFamily elimina todas las sesiones de una misma familia.
func (r *SessionRepository) RevokeFamily(_ context.Context, familyID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, session := range r.sessions {
		if session.FamilyID == familyID {
			delete(r.sessions, id)
		}
	}

	return nil
}

// PurgeExpired elimina las sesiones caducadas y devuelve cuantas se borraron.
//
// Sin esta limpieza el mapa creceria indefinidamente: cada login y cada
// rotacion anaden una entrada que ya no sirve para nada una vez expirada, y en
// un proceso de larga vida eso es una fuga de memoria.
func (r *SessionRepository) PurgeExpired(now time.Time) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	removed := 0
	for id, session := range r.sessions {
		if session.IsExpired(now) {
			delete(r.sessions, id)
			removed++
		}
	}

	return removed
}

// Len devuelve el numero de sesiones almacenadas. Util para tests y metricas.
func (r *SessionRepository) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.sessions)
}
