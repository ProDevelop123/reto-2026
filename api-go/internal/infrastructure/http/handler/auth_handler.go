package handler

import (
	"time"

	"strings"

	"github.com/gofiber/fiber/v3"

	"api-go/internal/config"
	"api-go/internal/core/domain"
	"api-go/internal/core/port"
	"api-go/internal/core/usecase"
	"api-go/internal/infrastructure/http/dto"
	"api-go/pkg/apperror"
)

// AuthHandler expone el login, el refresco y el cierre de sesion.
type AuthHandler struct {
	useCase *usecase.AuthUseCase
	cookie  config.CookieConfig
	clock   port.Clock
}

// NewAuthHandler construye el handler.
func NewAuthHandler(
	useCase *usecase.AuthUseCase,
	cookie config.CookieConfig,
	clock port.Clock,
) *AuthHandler {
	return &AuthHandler{useCase: useCase, cookie: cookie, clock: clock}
}

// Login atiende POST /api/v1/auth/login.
func (h *AuthHandler) Login(c fiber.Ctx) error {
	var request dto.LoginRequest

	if err := c.Bind().JSON(&request); err != nil {
		return apperror.BadRequest("El cuerpo de la peticion no es JSON valido.").WithCause(err)
	}

	credentials, err := request.ToDomain()
	if err != nil {
		return err
	}

	pair, err := h.useCase.Login(c.Context(), credentials)
	if err != nil {
		return err
	}

	h.setRefreshCookie(c, pair)

	return c.Status(fiber.StatusOK).JSON(dto.Success(
		dto.NewTokenResponse(pair, credentials.Username, h.clock.Now()),
	))
}

// Refresh atiende POST /api/v1/auth/refresh.
//
// El token de refresco se lee de la cookie, no del cuerpo: es la contrapartida
// de emitirlo en una cookie HttpOnly. El frontend no puede leerlo ni enviarlo
// explicitamente, y no lo necesita: el navegador la adjunta solo.
func (h *AuthHandler) Refresh(c fiber.Ctx) error {
	token := c.Cookies(h.cookie.Name)

	if token == "" {
		return apperror.Unauthorized("No se recibio el token de refresco.").
			WithDetails(map[string]string{"reason": "missing_refresh_cookie"})
	}

	pair, err := h.useCase.Refresh(c.Context(), strings.Clone(token))
	if err != nil {
		// Ante un refresco fallido se borra la cookie. Conservar una cookie
		// invalida haria que el cliente reintentara en bucle contra un token
		// que ya nunca sera aceptado.
		h.clearRefreshCookie(c)
		return err
	}

	h.setRefreshCookie(c, pair)

	username, _ := c.Locals("username").(string)

	return c.Status(fiber.StatusOK).JSON(dto.Success(
		dto.NewTokenResponse(pair, username, h.clock.Now()),
	))
}

// Logout atiende POST /api/v1/auth/logout.
//
// Responde 200 aunque no haya cookie o el token sea ilegible: el efecto
// deseado —que la sesion no siga viva— ya se cumple, y devolver un error
// dejaria al cliente sin saber si puede limpiar su estado local.
func (h *AuthHandler) Logout(c fiber.Ctx) error {
	if token := c.Cookies(h.cookie.Name); token != "" {
		if err := h.useCase.Logout(c.Context(), strings.Clone(token)); err != nil {
			return err
		}
	}

	h.clearRefreshCookie(c)

	return c.Status(fiber.StatusOK).JSON(dto.Success(fiber.Map{"message": "Sesion cerrada."}))
}

// setRefreshCookie emite la cookie con el token de refresco.
func (h *AuthHandler) setRefreshCookie(c fiber.Ctx, pair domain.TokenPair) {
	c.Cookie(&fiber.Cookie{
		Name:  h.cookie.Name,
		Value: pair.RefreshToken,
		// El Path acota la cookie a las rutas de autenticacion: no se envia en
		// las peticiones a /api/v1/qr, que no la necesitan. Menos exposicion.
		Path:   h.cookie.Path,
		Domain: h.cookie.Domain,
		// HttpOnly la vuelve inaccesible desde JavaScript, de modo que un XSS
		// en el frontend no puede robar el token de refresco.
		HTTPOnly: h.cookie.HTTPOnly,
		// Secure exige HTTPS. Es obligatorio cuando SameSite es None.
		Secure: h.cookie.Secure,
		// Con el frontend en Vercel y la API en GCP, la peticion de refresco es
		// de origen cruzado y el navegador solo enviara la cookie si vale None.
		SameSite: h.cookie.SameSite,
		Expires:  pair.RefreshExpiresAt,
	})
}

// clearRefreshCookie invalida la cookie en el navegador.
//
// Los atributos deben coincidir exactamente con los de emision —name, path y
// domain—; si difieren, el navegador crea una cookie distinta y la original
// sobrevive.
func (h *AuthHandler) clearRefreshCookie(c fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     h.cookie.Name,
		Value:    "",
		Path:     h.cookie.Path,
		Domain:   h.cookie.Domain,
		HTTPOnly: h.cookie.HTTPOnly,
		Secure:   h.cookie.Secure,
		SameSite: h.cookie.SameSite,
		// Una fecha en el pasado es la forma estandar de pedir al navegador que
		// borre la cookie.
		Expires: h.clock.Now().Add(-time.Hour),
		MaxAge:  -1,
	})
}
