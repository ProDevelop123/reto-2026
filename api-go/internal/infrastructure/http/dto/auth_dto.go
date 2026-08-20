package dto

import (
	"strings"
	"time"

	"api-go/internal/core/domain"
	"api-go/pkg/apperror"
)

// LoginRequest es el cuerpo de POST /api/v1/auth/login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ToDomain valida las credenciales recibidas.
//
// Solo se comprueba que esten presentes. Cualquier validacion adicional aqui
// —longitud minima, formato— revelaria informacion sobre las credenciales
// validas antes incluso de comprobarlas.
func (r LoginRequest) ToDomain() (domain.Credentials, error) {
	username := strings.TrimSpace(r.Username)

	if username == "" || r.Password == "" {
		return domain.Credentials{},
			apperror.Validation("Los campos \"username\" y \"password\" son obligatorios.")
	}

	return domain.Credentials{Username: username, Password: r.Password}, nil
}

// TokenResponse es la respuesta de login y de refresco.
//
// El refresh token NO aparece en el cuerpo: viaja exclusivamente en una cookie
// HttpOnly. Si se devolviera aqui, el frontend tendria que guardarlo en algun
// sitio accesible desde JavaScript y un XSS bastaria para robarlo. En la cookie
// es inalcanzable para el codigo de la pagina.
type TokenResponse struct {
	AccessToken string `json:"accessToken"`
	TokenType   string `json:"tokenType"`
	// ExpiresIn son segundos hasta la expiracion. Se envia ademas del instante
	// absoluto porque libera al cliente de depender de que su reloj este
	// sincronizado con el del servidor.
	ExpiresIn int       `json:"expiresIn"`
	ExpiresAt time.Time `json:"expiresAt"`
	Username  string    `json:"username"`
}

// NewTokenResponse traduce el par de tokens al formato publicado.
func NewTokenResponse(pair domain.TokenPair, username string, now time.Time) TokenResponse {
	return TokenResponse{
		AccessToken: pair.AccessToken,
		TokenType:   pair.TokenType,
		ExpiresIn:   int(pair.AccessExpiresAt.Sub(now).Seconds()),
		ExpiresAt:   pair.AccessExpiresAt,
		Username:    username,
	}
}
