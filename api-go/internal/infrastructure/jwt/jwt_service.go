// Package jwt implementa la emision y verificacion de tokens con firma
// asimetrica RS256.
//
// Este servicio es el UNICO del sistema que posee la clave privada y, por
// tanto, el unico capaz de emitir tokens. La API en Node recibe solo la clave
// publica: puede verificar firmas pero no producirlas, de modo que
// comprometerla no permite suplantar a nadie. Con un secreto compartido
// (HS256) ambos servicios podrian firmar y esa garantia desapareceria.
package jwt

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"api-go/internal/config"
	"api-go/internal/core/domain"
	"api-go/internal/core/port"
	"api-go/pkg/apperror"
)

// Tipos de token. Access y refresh se firman con la misma clave, asi que la
// verificacion criptografica no los distingue; el tipo declarado en los claims
// es lo que impide usar un refresh token de vida larga como token de acceso.
const (
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
)

// signingMethod fija el algoritmo de firma.
//
// Verificar con un algoritmo fijado, en lugar de confiar en la cabecera "alg"
// del propio token, cierra el ataque clasico de confusion de algoritmo: un
// atacante que cambie "alg" a "none", o a HS256 usando la clave publica como
// secreto, obtendria tokens validos si el verificador aceptara cualquier
// algoritmo.
var signingMethod = jwt.SigningMethodRS256

// Claims son los datos que transportan los tokens de este servicio.
type Claims struct {
	jwt.RegisteredClaims
	// TokenType distingue "access" de "refresh".
	TokenType string `json:"tokenType"`
	// FamilyID agrupa los refresh tokens descendientes de un mismo login.
	// Vacio en los tokens de acceso.
	FamilyID string `json:"familyId,omitempty"`
}

// Service emite y verifica tokens. Implementa port.TokenIssuer.
type Service struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	issuer     string
	audience   string
	accessTTL  time.Duration
	refreshTTL time.Duration
	clock      port.Clock
}

// New construye el servicio a partir de las claves en formato PEM.
//
// Las claves se parsean una sola vez, en el arranque: hacerlo en cada peticion
// seria un coste innecesario, y un PEM invalido debe impedir que el servicio
// arranque, no fallar en la primera peticion de un usuario.
func New(cfg config.JWTConfig, clock port.Clock) (*Service, error) {
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(cfg.PrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("clave privada RSA invalida: %w", err)
	}

	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(cfg.PublicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("clave publica RSA invalida: %w", err)
	}

	// Si las claves no forman pareja, todo token emitido por este servicio
	// seria rechazado por quien lo verifique. Detectarlo en el arranque evita
	// una depuracion desconcertante en produccion.
	if !privateKey.PublicKey.Equal(publicKey) {
		return nil, errors.New("la clave publica no corresponde a la clave privada")
	}

	return &Service{
		privateKey: privateKey,
		publicKey:  publicKey,
		issuer:     cfg.Issuer,
		audience:   cfg.Audience,
		accessTTL:  cfg.AccessTTL,
		refreshTTL: cfg.RefreshTTL,
		clock:      clock,
	}, nil
}

// IssueAccess emite un token de acceso de vida corta.
func (s *Service) IssueAccess(username string) (string, time.Time, error) {
	now := s.clock.Now()
	expiresAt := now.Add(s.accessTTL)

	token, err := s.sign(Claims{
		RegisteredClaims: s.registeredClaims(username, now, expiresAt),
		TokenType:        tokenTypeAccess,
	})
	if err != nil {
		return "", time.Time{}, err
	}

	return token, expiresAt, nil
}

// IssueRefresh emite un token de refresco ligado a una sesion y a su familia.
func (s *Service) IssueRefresh(username, sessionID, familyID string) (string, time.Time, error) {
	now := s.clock.Now()
	expiresAt := now.Add(s.refreshTTL)

	claims := Claims{
		RegisteredClaims: s.registeredClaims(username, now, expiresAt),
		TokenType:        tokenTypeRefresh,
		FamilyID:         familyID,
	}
	// El identificador de sesion viaja como "jti", que es la claim estandar
	// para identificar un token de forma unica y es justo lo que necesita el
	// almacen de sesiones para poder revocarlo.
	claims.ID = sessionID

	token, err := s.sign(claims)
	if err != nil {
		return "", time.Time{}, err
	}

	return token, expiresAt, nil
}

// ParseRefresh verifica un token de refresco y traduce sus claims al
// vocabulario del dominio.
func (s *Service) ParseRefresh(token string) (domain.RefreshClaims, error) {
	claims, err := s.parse(token, tokenTypeRefresh)
	if err != nil {
		return domain.RefreshClaims{}, err
	}

	if claims.ID == "" || claims.FamilyID == "" {
		return domain.RefreshClaims{},
			apperror.Unauthorized("El token de refresco esta incompleto.")
	}

	return domain.RefreshClaims{
		SessionID: claims.ID,
		FamilyID:  claims.FamilyID,
		Username:  claims.Subject,
		ExpiresAt: claims.ExpiresAt.Time,
	}, nil
}

// ParseAccess verifica un token de acceso. Lo usa el middleware de
// autenticacion.
func (s *Service) ParseAccess(token string) (*Claims, error) {
	return s.parse(token, tokenTypeAccess)
}

// registeredClaims construye las claims estandar comunes a todos los tokens.
func (s *Service) registeredClaims(username string, now, expiresAt time.Time) jwt.RegisteredClaims {
	return jwt.RegisteredClaims{
		Subject:   username,
		Issuer:    s.issuer,
		Audience:  jwt.ClaimStrings{s.audience},
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}
}

func (s *Service) sign(claims Claims) (string, error) {
	signed, err := jwt.NewWithClaims(signingMethod, claims).SignedString(s.privateKey)
	if err != nil {
		return "", fmt.Errorf("no se pudo firmar el token: %w", err)
	}
	return signed, nil
}

// parse verifica firma, emisor, audiencia, vigencia y tipo de un token.
func (s *Service) parse(raw, expectedType string) (*Claims, error) {
	claims := &Claims{}

	_, err := jwt.ParseWithClaims(raw, claims,
		func(*jwt.Token) (any, error) { return s.publicKey, nil },
		jwt.WithValidMethods([]string{signingMethod.Alg()}),
		jwt.WithIssuer(s.issuer),
		jwt.WithAudience(s.audience),
		jwt.WithExpirationRequired(),
		// El reloj inyectado permite a los tests simular la expiracion sin
		// esperar de verdad.
		jwt.WithTimeFunc(s.clock.Now),
	)
	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenExpired):
			return nil, apperror.Unauthorized("El token ha expirado.").
				WithDetails(map[string]string{"reason": "token_expired"}).
				WithCause(err)
		default:
			return nil, apperror.Unauthorized("Token invalido.").
				WithDetails(map[string]string{"reason": "token_invalid"}).
				WithCause(err)
		}
	}

	if claims.TokenType != expectedType {
		return nil, apperror.Unauthorized(fmt.Sprintf("Se requiere un token de tipo %q.", expectedType)).
			WithDetails(map[string]string{"reason": "wrong_token_type"})
	}

	return claims, nil
}
