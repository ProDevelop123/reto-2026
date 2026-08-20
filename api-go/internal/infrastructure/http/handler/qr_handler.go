// Package handler contiene los manejadores HTTP del servicio.
//
// Los handlers son deliberadamente finos: adaptan entre el mundo HTTP y los
// casos de uso, y nada mas. No contienen logica de negocio ni construyen
// respuestas de error, que es responsabilidad del ErrorHandler central.
package handler

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"api-go/internal/core/port"
	"api-go/internal/core/usecase"
	"api-go/internal/infrastructure/http/dto"
	"api-go/pkg/apperror"
)

// QRHandler expone el caso de uso de factorizacion.
type QRHandler struct {
	useCase *usecase.QRUseCase
	clock   port.Clock
}

// NewQRHandler construye el handler.
func NewQRHandler(useCase *usecase.QRUseCase, clock port.Clock) *QRHandler {
	return &QRHandler{useCase: useCase, clock: clock}
}

// Factorize atiende POST /api/v1/qr.
//
// Es el endpoint principal del reto: recibe la matriz original, calcula su
// factorizacion QR, envia Q y R al servicio de estadisticas en Node y devuelve
// todo junto en una sola respuesta.
func (h *QRHandler) Factorize(c fiber.Ctx) error {
	var request dto.FactorizeRequest

	if err := c.Bind().JSON(&request); err != nil {
		// Un cuerpo ilegible es un 400: la peticion no llega siquiera a poder
		// interpretarse, a diferencia de un 422, donde se entiende pero es
		// semanticamente incorrecta.
		return apperror.BadRequest("El cuerpo de la peticion no es JSON valido.").WithCause(err)
	}

	input, err := request.ToDomain()
	if err != nil {
		return err
	}

	started := h.clock.Now()

	result, err := h.useCase.Factorize(c.Context(), input)
	if err != nil {
		return err
	}

	elapsed := h.clock.Now().Sub(started)

	return c.Status(fiber.StatusOK).JSON(dto.SuccessWithMetadata(
		dto.NewFactorizeResponse(result),
		dto.NewFactorizeMetadata(result, elapsed),
	))
}

// HealthHandler responde a las sondas de vida.
type HealthHandler struct {
	appName   string
	version   string
	startedAt time.Time
	clock     port.Clock
}

// NewHealthHandler construye el handler de salud.
func NewHealthHandler(appName, version string, clock port.Clock) *HealthHandler {
	return &HealthHandler{
		appName:   appName,
		version:   version,
		startedAt: clock.Now(),
		clock:     clock,
	}
}

// Health atiende GET /health.
//
// No requiere autenticacion: la consultan el healthcheck de Docker y el
// balanceador de la plataforma cloud, que no disponen de token. Exigirlo haria
// que el contenedor se reportara siempre como no sano. No expone informacion
// sensible.
func (h *HealthHandler) Health(c fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(dto.Success(fiber.Map{
		"status":        "ok",
		"service":       h.appName,
		"version":       h.version,
		"uptimeSeconds": int(h.clock.Now().Sub(h.startedAt).Seconds()),
	}))
}
