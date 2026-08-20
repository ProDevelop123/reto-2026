package usecase

import (
	"context"
	"crypto/subtle"
	"errors"

	"api-go/internal/core/domain"
	"api-go/internal/core/port"
	"api-go/pkg/apperror"
)

// AuthUseCase implementa el login, el refresco y el cierre de sesion.
//
// El almacen de usuarios se reduce a un par de credenciales estaticas: el reto
// no pide gestion de usuarios y montar una base de datos para validar un
// usuario y una contrasena anadiria infraestructura sin demostrar nada nuevo.
// Lo que si se implementa con rigor es todo lo demas: firma asimetrica,
// rotacion de tokens de refresco, deteccion de reutilizacion y revocacion.
type AuthUseCase struct {
	credentials domain.Credentials
	tokens      port.TokenIssuer
	sessions    port.SessionRepository
	clock       port.Clock
	ids         port.IDGenerator
}

// NewAuthUseCase construye el caso de uso con sus dependencias.
func NewAuthUseCase(
	credentials domain.Credentials,
	tokens port.TokenIssuer,
	sessions port.SessionRepository,
	clock port.Clock,
	ids port.IDGenerator,
) *AuthUseCase {
	return &AuthUseCase{
		credentials: credentials,
		tokens:      tokens,
		sessions:    sessions,
		clock:       clock,
		ids:         ids,
	}
}

// Login valida las credenciales y abre una sesion nueva.
func (uc *AuthUseCase) Login(
	ctx context.Context,
	credentials domain.Credentials,
) (domain.TokenPair, error) {
	if !uc.credentialsMatch(credentials) {
		return domain.TokenPair{},
			apperror.Unauthorized("Usuario o contrasena incorrectos.").
				WithCause(domain.ErrInvalidCredentials)
	}

	// Cada login inicia una familia de tokens nueva. Los sucesivos refrescos
	// crean tokens dentro de esa misma familia, lo que permite invalidar de
	// golpe toda la cadena si mas adelante se detecta un robo.
	familyID, err := uc.ids.NewID()
	if err != nil {
		return domain.TokenPair{}, apperror.Internal("No se pudo iniciar la sesion.").WithCause(err)
	}

	return uc.issuePair(ctx, credentials.Username, familyID)
}

// Refresh canjea un token de refresco por un par nuevo.
//
// Se aplica ROTACION: el token presentado queda marcado como usado y se emite
// uno nuevo. Un token de refresco es de vida larga y viaja en cada renovacion,
// asi que su ventana de exposicion es amplia; rotarlo reduce a un solo uso el
// valor de una copia robada.
//
// Ademas se detecta la REUTILIZACION. Si llega un token ya canjeado, hay dos
// copias en circulacion y una de ellas es de un atacante. Como no se puede
// saber cual, se revoca la familia entera: el usuario legitimo tendra que
// volver a autenticarse, que es preferible a dejar viva una sesion robada.
func (uc *AuthUseCase) Refresh(ctx context.Context, refreshToken string) (domain.TokenPair, error) {
	claims, err := uc.tokens.ParseRefresh(refreshToken)
	if err != nil {
		return domain.TokenPair{}, err
	}

	session, err := uc.sessions.Find(ctx, claims.SessionID)
	if err != nil {
		if errors.Is(err, domain.ErrSessionNotFound) {
			// La firma es valida pero la sesion no existe: o se cerro sesion, o
			// se revoco la familia, o el servicio se reinicio (el almacen es
			// volatil). En cualquier caso, hay que volver a autenticarse.
			return domain.TokenPair{},
				apperror.Unauthorized("La sesion ya no es valida.").WithCause(err)
		}
		return domain.TokenPair{}, apperror.Internal("No se pudo recuperar la sesion.").WithCause(err)
	}

	now := uc.clock.Now()

	if session.IsUsed() {
		// Reutilizacion detectada: se corta toda la cadena.
		if revokeErr := uc.sessions.RevokeFamily(ctx, session.FamilyID); revokeErr != nil {
			return domain.TokenPair{},
				apperror.Internal("No se pudo revocar la sesion comprometida.").WithCause(revokeErr)
		}
		return domain.TokenPair{},
			apperror.Unauthorized("Se ha detectado una reutilizacion del token; la sesion fue revocada.").
				WithCause(domain.ErrSessionReused)
	}

	if session.IsExpired(now) {
		return domain.TokenPair{},
			apperror.Unauthorized("La sesion ha expirado.").WithCause(domain.ErrSessionExpired)
	}

	if err := uc.sessions.MarkUsed(ctx, session.ID, now); err != nil {
		return domain.TokenPair{}, apperror.Internal("No se pudo rotar la sesion.").WithCause(err)
	}

	return uc.issuePair(ctx, session.Username, session.FamilyID)
}

// Logout revoca la familia completa a la que pertenece el token presentado.
//
// Es la razon de ser del almacen de sesiones: sin el, un JWT autocontenido
// seguiria siendo valido hasta su expiracion por mucho que el usuario cerrara
// sesion.
func (uc *AuthUseCase) Logout(ctx context.Context, refreshToken string) error {
	claims, err := uc.tokens.ParseRefresh(refreshToken)
	if err != nil {
		// Cerrar sesion con un token ilegible no es un error que merezca
		// reportarse: el efecto deseado —que la sesion no siga viva— ya se
		// cumple. Se responde con exito para que el cliente limpie su estado.
		return nil
	}

	if err := uc.sessions.RevokeFamily(ctx, claims.FamilyID); err != nil {
		return apperror.Internal("No se pudo cerrar la sesion.").WithCause(err)
	}

	return nil
}

// issuePair emite un par de tokens y registra la sesion correspondiente.
func (uc *AuthUseCase) issuePair(
	ctx context.Context,
	username, familyID string,
) (domain.TokenPair, error) {
	sessionID, err := uc.ids.NewID()
	if err != nil {
		return domain.TokenPair{}, apperror.Internal("No se pudo emitir la sesion.").WithCause(err)
	}

	accessToken, accessExpiry, err := uc.tokens.IssueAccess(username)
	if err != nil {
		return domain.TokenPair{}, apperror.Internal("No se pudo emitir el token de acceso.").WithCause(err)
	}

	refreshToken, refreshExpiry, err := uc.tokens.IssueRefresh(username, sessionID, familyID)
	if err != nil {
		return domain.TokenPair{}, apperror.Internal("No se pudo emitir el token de refresco.").WithCause(err)
	}

	session := domain.Session{
		ID:        sessionID,
		FamilyID:  familyID,
		Username:  username,
		IssuedAt:  uc.clock.Now(),
		ExpiresAt: refreshExpiry,
	}

	if err := uc.sessions.Save(ctx, session); err != nil {
		return domain.TokenPair{}, apperror.Internal("No se pudo registrar la sesion.").WithCause(err)
	}

	return domain.TokenPair{
		AccessToken:      accessToken,
		AccessExpiresAt:  accessExpiry,
		RefreshToken:     refreshToken,
		RefreshExpiresAt: refreshExpiry,
		TokenType:        "Bearer",
	}, nil
}

// credentialsMatch compara las credenciales en tiempo constante.
//
// Una comparacion de cadenas normal aborta en el primer byte distinto, lo que
// filtra por el tiempo de respuesta cuantos caracteres iniciales son correctos
// y permite reconstruir la contrasena caracter a caracter. Aunque aqui las
// credenciales sean estaticas, escribir la comparacion insegura seria un mal
// ejemplo y un fallo real si el almacen se sustituyera por uno de verdad.
//
// Se comparan SIEMPRE ambos campos, sin cortocircuitar cuando el usuario ya no
// coincide, para que el tiempo de respuesta no revele si el usuario existe.
func (uc *AuthUseCase) credentialsMatch(candidate domain.Credentials) bool {
	usernameMatch := subtle.ConstantTimeCompare(
		[]byte(candidate.Username), []byte(uc.credentials.Username))
	passwordMatch := subtle.ConstantTimeCompare(
		[]byte(candidate.Password), []byte(uc.credentials.Password))

	return usernameMatch == 1 && passwordMatch == 1
}
