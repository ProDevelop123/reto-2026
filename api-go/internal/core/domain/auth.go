package domain

import "time"

// Credentials son las credenciales presentadas en el login.
type Credentials struct {
	Username string
	Password string
}

// TokenPair es el resultado de una autenticacion correcta.
//
// Los dos tokens tienen ciclos de vida y canales de transporte distintos por
// razones de seguridad:
//
//   - AccessToken es de vida corta y viaja en la cabecera Authorization. Vive
//     solo en memoria del cliente, nunca en almacenamiento persistente.
//   - RefreshToken es de vida larga y viaja en una cookie HttpOnly, inaccesible
//     desde JavaScript, de modo que un XSS en el frontend no puede robarlo.
type TokenPair struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	TokenType        string
}

// Session representa un refresh token emitido y todavia vigente.
//
// Mantener registro de las sesiones es lo que permite revocar: un JWT es
// autocontenido y, sin un registro contra el que contrastarlo, seguiria siendo
// valido hasta su expiracion aunque el usuario cerrara sesion.
type Session struct {
	// ID es el identificador unico del token (el claim jti).
	ID string
	// FamilyID agrupa todos los tokens descendientes de un mismo login. La
	// rotacion crea un token nuevo en cada refresco, y la familia permite
	// invalidarlos todos de golpe si se detecta una reutilizacion.
	FamilyID  string
	Username  string
	IssuedAt  time.Time
	ExpiresAt time.Time
	// UsedAt marca el momento en que el token se canjeo por otro. Un token ya
	// canjeado que vuelve a presentarse es la senal de que fue robado.
	UsedAt *time.Time
}

// RefreshClaims son los datos que transporta un token de refresco ya
// verificado.
//
// Es el vocabulario del dominio, no el de la biblioteca de JWT: el nucleo
// razona sobre sesiones y familias, no sobre claims registradas.
type RefreshClaims struct {
	SessionID string
	FamilyID  string
	Username  string
	ExpiresAt time.Time
}

// IsExpired indica si la sesion ha caducado.
func (s Session) IsExpired(now time.Time) bool { return now.After(s.ExpiresAt) }

// IsUsed indica si el token ya fue canjeado por otro.
func (s Session) IsUsed() bool { return s.UsedAt != nil }
