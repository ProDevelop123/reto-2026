package usecase_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"api-go/internal/config"
	"api-go/internal/core/domain"
	"api-go/internal/core/usecase"
	appjwt "api-go/internal/infrastructure/jwt"
	"api-go/internal/infrastructure/persistence/memory"
	"api-go/pkg/apperror"
)

// --- Dobles -------------------------------------------------------------

// fakeClock permite adelantar el tiempo a voluntad.
//
// Sin el, comprobar la expiracion de un token exigiria dormir de verdad: el
// test seria lento y, peor, intermitente. Con el reloj inyectado, "han pasado
// ocho dias" es una linea de codigo instantanea y determinista.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// sequentialIDs produce identificadores predecibles para hacer las
// aserciones legibles.
type sequentialIDs struct {
	mu      sync.Mutex
	counter int
}

func (g *sequentialIDs) NewID() (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.counter++
	return fmt.Sprintf("id-%d", g.counter), nil
}

// --- Utilidades ---------------------------------------------------------

// newTestAuth monta el caso de uso con el servicio JWT REAL sobre un par de
// claves efimero.
//
// Se usa el servicio real y no un doble porque la firma y la verificacion son
// justo lo que interesa comprobar: un doble que devolviera cadenas inventadas
// no probaria nada sobre la seguridad del sistema.
func newTestAuth(t *testing.T) (*usecase.AuthUseCase, *memory.SessionRepository, *fakeClock) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("no se pudo generar la clave RSA: %v", err)
	}

	privatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: mustMarshalPKCS8(t, key),
	})
	publicPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: mustMarshalPKIX(t, &key.PublicKey),
	})

	clock := newFakeClock()

	tokens, err := appjwt.New(config.JWTConfig{
		PrivateKeyPEM: privatePEM,
		PublicKeyPEM:  publicPEM,
		Issuer:        "reto-2026-api-go",
		Audience:      "reto-2026",
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    7 * 24 * time.Hour,
	}, clock)
	if err != nil {
		t.Fatalf("no se pudo construir el servicio JWT: %v", err)
	}

	sessions := memory.NewSessionRepository()

	useCase := usecase.NewAuthUseCase(
		domain.Credentials{Username: "admin", Password: "secreto"},
		tokens, sessions, clock, &sequentialIDs{},
	)

	return useCase, sessions, clock
}

func mustMarshalPKCS8(t *testing.T, key *rsa.PrivateKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("no se pudo serializar la clave privada: %v", err)
	}
	return der
}

func mustMarshalPKIX(t *testing.T, key *rsa.PublicKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		t.Fatalf("no se pudo serializar la clave publica: %v", err)
	}
	return der
}

func assertStatus(t *testing.T, err error, want int) *apperror.AppError {
	t.Helper()

	var appErr *apperror.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("el error no es un AppError: %v", err)
	}
	if appErr.Status != want {
		t.Errorf("estado %d, se esperaba %d (%v)", appErr.Status, want, err)
	}

	return appErr
}

// --- Login --------------------------------------------------------------

func TestLoginWithValidCredentials(t *testing.T) {
	auth, sessions, _ := newTestAuth(t)

	pair, err := auth.Login(context.Background(),
		domain.Credentials{Username: "admin", Password: "secreto"})
	if err != nil {
		t.Fatalf("Login devolvio error: %v", err)
	}

	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Error("el login no devolvio ambos tokens")
	}
	if pair.TokenType != "Bearer" {
		t.Errorf("tipo de token %q, se esperaba \"Bearer\"", pair.TokenType)
	}
	if !pair.RefreshExpiresAt.After(pair.AccessExpiresAt) {
		t.Error("el refresh token deberia caducar despues que el de acceso")
	}
	if sessions.Len() != 1 {
		t.Errorf("hay %d sesiones registradas, se esperaba 1", sessions.Len())
	}
}

func TestLoginRejectsBadCredentials(t *testing.T) {
	cases := []struct {
		name        string
		credentials domain.Credentials
	}{
		{"contrasena incorrecta", domain.Credentials{Username: "admin", Password: "otra"}},
		{"usuario inexistente", domain.Credentials{Username: "otro", Password: "secreto"}},
		{"ambos incorrectos", domain.Credentials{Username: "x", Password: "y"}},
		{"vacios", domain.Credentials{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auth, sessions, _ := newTestAuth(t)

			_, err := auth.Login(context.Background(), tc.credentials)

			appErr := assertStatus(t, err, 401)

			// El mensaje debe ser identico para usuario inexistente y para
			// contrasena incorrecta: distinguirlos permitiria enumerar usuarios.
			if appErr.Message != "Usuario o contrasena incorrectos." {
				t.Errorf("mensaje %q: no debe revelar cual de los dos campos fallo", appErr.Message)
			}
			if sessions.Len() != 0 {
				t.Error("no debe registrarse ninguna sesion tras un login fallido")
			}
		})
	}
}

// --- Refresco y rotacion -------------------------------------------------

func TestRefreshRotatesTheToken(t *testing.T) {
	auth, _, clock := newTestAuth(t)

	first, err := auth.Login(context.Background(),
		domain.Credentials{Username: "admin", Password: "secreto"})
	if err != nil {
		t.Fatalf("Login devolvio error: %v", err)
	}

	// Se avanza el reloj para que el token nuevo tenga distinta marca temporal
	// y no pueda coincidir por casualidad con el anterior.
	clock.Advance(time.Minute)

	second, err := auth.Refresh(context.Background(), first.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh devolvio error: %v", err)
	}

	if second.RefreshToken == first.RefreshToken {
		t.Error("el refresh token no rotó: se devolvio el mismo")
	}
	if second.AccessToken == first.AccessToken {
		t.Error("el access token no se renovo")
	}
}

func TestRefreshRejectsAlreadyUsedTokenAndRevokesTheFamily(t *testing.T) {
	auth, sessions, clock := newTestAuth(t)

	first, err := auth.Login(context.Background(),
		domain.Credentials{Username: "admin", Password: "secreto"})
	if err != nil {
		t.Fatalf("Login devolvio error: %v", err)
	}

	clock.Advance(time.Minute)

	second, err := auth.Refresh(context.Background(), first.RefreshToken)
	if err != nil {
		t.Fatalf("primer refresco: %v", err)
	}

	// Se reutiliza el token ya canjeado: es la firma de un token robado, porque
	// el legitimo ya lo uso.
	_, err = auth.Refresh(context.Background(), first.RefreshToken)

	appErr := assertStatus(t, err, 401)
	if !errors.Is(appErr, domain.ErrSessionReused) {
		t.Errorf("el error no identifica la reutilizacion: %v", err)
	}

	// La familia entera queda revocada: tambien el token que si era legitimo.
	// Es deliberado: no se puede saber cual de las dos copias es del atacante,
	// asi que se invalidan ambas y el usuario vuelve a autenticarse.
	if _, err := auth.Refresh(context.Background(), second.RefreshToken); err == nil {
		t.Error("el token legitimo deberia haber quedado revocado con su familia")
	}

	if sessions.Len() != 0 {
		t.Errorf("quedan %d sesiones, la familia deberia haberse eliminado", sessions.Len())
	}
}

func TestRefreshRejectsExpiredToken(t *testing.T) {
	auth, _, clock := newTestAuth(t)

	pair, err := auth.Login(context.Background(),
		domain.Credentials{Username: "admin", Password: "secreto"})
	if err != nil {
		t.Fatalf("Login devolvio error: %v", err)
	}

	// Ocho dias, mas alla del TTL de siete: instantaneo gracias al reloj falso.
	clock.Advance(8 * 24 * time.Hour)

	_, err = auth.Refresh(context.Background(), pair.RefreshToken)

	appErr := assertStatus(t, err, 401)
	if appErr.Details == nil {
		t.Error("el error deberia indicar el motivo de la expiracion")
	}
}

func TestRefreshRejectsAccessTokenUsedAsRefresh(t *testing.T) {
	auth, _, _ := newTestAuth(t)

	pair, err := auth.Login(context.Background(),
		domain.Credentials{Username: "admin", Password: "secreto"})
	if err != nil {
		t.Fatalf("Login devolvio error: %v", err)
	}

	// Ambos tokens estan firmados con la misma clave, asi que la verificacion
	// criptografica del token de acceso es correcta. Lo que lo rechaza es el
	// tipo declarado en los claims.
	_, err = auth.Refresh(context.Background(), pair.AccessToken)

	appErr := assertStatus(t, err, 401)
	details, ok := appErr.Details.(map[string]string)
	if !ok || details["reason"] != "wrong_token_type" {
		t.Errorf("motivo %v, se esperaba wrong_token_type", appErr.Details)
	}
}

func TestRefreshRejectsGarbageToken(t *testing.T) {
	auth, _, _ := newTestAuth(t)

	_, err := auth.Refresh(context.Background(), "esto.no.es-un-jwt")

	assertStatus(t, err, 401)
}

func TestRefreshRejectsTokenSignedByAnotherKey(t *testing.T) {
	// Un token con la estructura correcta pero firmado por otra autoridad. Es
	// la comprobacion de que se verifica la FIRMA y no solo el formato.
	other, _, _ := newTestAuth(t)
	auth, _, _ := newTestAuth(t)

	foreign, err := other.Login(context.Background(),
		domain.Credentials{Username: "admin", Password: "secreto"})
	if err != nil {
		t.Fatalf("Login devolvio error: %v", err)
	}

	_, err = auth.Refresh(context.Background(), foreign.RefreshToken)

	assertStatus(t, err, 401)
}

// --- Cierre de sesion ----------------------------------------------------

func TestLogoutRevokesTheSession(t *testing.T) {
	auth, sessions, _ := newTestAuth(t)

	pair, err := auth.Login(context.Background(),
		domain.Credentials{Username: "admin", Password: "secreto"})
	if err != nil {
		t.Fatalf("Login devolvio error: %v", err)
	}

	if err := auth.Logout(context.Background(), pair.RefreshToken); err != nil {
		t.Fatalf("Logout devolvio error: %v", err)
	}

	if sessions.Len() != 0 {
		t.Errorf("quedan %d sesiones tras cerrar sesion", sessions.Len())
	}

	// Este es el motivo de existir del almacen de sesiones: sin el, el JWT
	// seguiria siendo criptograficamente valido hasta su expiracion.
	if _, err := auth.Refresh(context.Background(), pair.RefreshToken); err == nil {
		t.Error("el token revocado deberia dejar de servir para refrescar")
	}
}

func TestLogoutIsIdempotentWithInvalidToken(t *testing.T) {
	auth, _, _ := newTestAuth(t)

	// Cerrar sesion con un token ilegible no es un error: el efecto deseado ya
	// se cumple y devolver un fallo dejaria al cliente sin saber si puede
	// limpiar su estado local.
	if err := auth.Logout(context.Background(), "no-es-un-token"); err != nil {
		t.Errorf("Logout deberia ser idempotente, devolvio: %v", err)
	}
}

// --- Concurrencia --------------------------------------------------------

func TestConcurrentLoginsAreSafe(t *testing.T) {
	auth, sessions, _ := newTestAuth(t)

	const logins = 50
	var wg sync.WaitGroup
	errs := make(chan error, logins)

	// Se ejecuta con -race para detectar accesos concurrentes al mapa de
	// sesiones. Un servidor HTTP atiende cada peticion en su propia goroutine,
	// asi que el almacen tiene que ser seguro por construccion, no por suerte.
	for i := 0; i < logins; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := auth.Login(context.Background(),
				domain.Credentials{Username: "admin", Password: "secreto"}); err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("login concurrente fallo: %v", err)
	}

	if sessions.Len() != logins {
		t.Errorf("hay %d sesiones, se esperaban %d", sessions.Len(), logins)
	}
}
