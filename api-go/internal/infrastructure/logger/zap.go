// Package logger construye el registrador estructurado del servicio.
package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New construye el logger segun el entorno.
//
// En produccion se emite JSON por stdout, que es lo que esperan los
// recolectores de logs de las plataformas de contenedores (Cloud Run entre
// ellas): escribir a fichero dentro de un contenedor efimero seria escribir a
// un disco que desaparece con el.
//
// En desarrollo se usa salida con colores y niveles legibles en terminal.
func New(env, level string) (*zap.Logger, error) {
	parsedLevel, err := zapcore.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("nivel de log %q invalido: %w", level, err)
	}

	var cfg zap.Config
	if env == "production" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	cfg.Level = zap.NewAtomicLevelAt(parsedLevel)
	cfg.EncoderConfig.TimeKey = "timestamp"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := cfg.Build()
	if err != nil {
		return nil, fmt.Errorf("no se pudo construir el logger: %w", err)
	}

	return logger, nil
}
