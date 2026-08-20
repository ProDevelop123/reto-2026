// Package usecase contiene la logica de aplicacion: orquesta el dominio y los
// puertos para cumplir cada caso de uso del servicio.
//
// Ningun fichero de este paquete importa Fiber, HTTP ni configuracion. Solo
// conoce el dominio y las interfaces declaradas en port, lo que permite
// testear cada caso de uso al completo con dobles en memoria.
package usecase

import (
	"context"
	"fmt"

	"api-go/internal/core/domain"
	"api-go/internal/core/port"
	"api-go/internal/matrix"
	"api-go/pkg/apperror"
)

// Nombres con los que viajan las matrices resultantes por el sistema. Se
// declaran como constantes para que el caso de uso, el adaptador y el frontend
// compartan una unica fuente de verdad.
const (
	matrixNameQ = "Q"
	matrixNameR = "R"
	matrixNameA = "A"
)

// QRUseCase implementa el caso de uso principal del servicio: factorizar una
// matriz y enriquecer el resultado con estadisticas.
//
// Depende de la INTERFAZ StatisticsProvider, no de un cliente HTTP concreto.
// Esa es la inversion de dependencias en la practica.
type QRUseCase struct {
	statistics port.StatisticsProvider
}

// NewQRUseCase construye el caso de uso con sus dependencias.
func NewQRUseCase(statistics port.StatisticsProvider) *QRUseCase {
	return &QRUseCase{statistics: statistics}
}

// Factorize descompone la matriz de entrada como A = Q*R y devuelve la
// descomposicion junto con las estadisticas calculadas sobre Q y R.
//
// El flujo completo del reto ocurre aqui:
//
//  1. Se factoriza A localmente mediante reflexiones de Householder.
//  2. Se envian Q y R al servicio de estadisticas a traves del puerto.
//  3. Se compone la respuesta con ambas cosas.
//
// Si el servicio de estadisticas falla, el caso de uso falla. Se considero
// degradar con elegancia —devolver Q y R con las estadisticas vacias y un
// aviso— y se descarto: el contrato publicado promete estadisticas, y devolver
// un 200 con la mitad del resultado obliga a todos los clientes a comprobar si
// llegaron. Un 502 dice con precision que la peticion era correcta y que quien
// fallo fue una dependencia nuestra, lo que ademas hace razonable reintentar.
func (uc *QRUseCase) Factorize(
	ctx context.Context,
	input domain.FactorizationInput,
) (domain.FactorizationResult, error) {
	factorization, err := matrix.Decompose(input.Matrix, input.Mode)
	if err != nil {
		// Los errores de matrix son de validacion de la entrada del usuario, no
		// fallos del servicio, de ahi el 422.
		return domain.FactorizationResult{},
			apperror.Validation(fmt.Sprintf("No se pudo factorizar la matriz: %v", err)).
				WithCause(err)
	}

	q := domain.NamedMatrix{Name: matrixNameQ, Data: factorization.Q}
	r := domain.NamedMatrix{Name: matrixNameR, Data: factorization.R}

	statistics, err := uc.statistics.Analyze(ctx, domain.StatisticsRequest{
		Matrices:  []domain.NamedMatrix{q, r},
		Tolerance: input.Tolerance,
	})
	if err != nil {
		// El adaptador ya devuelve un AppError con el codigo adecuado (502 o
		// 504). Se propaga tal cual para no perder esa precision.
		return domain.FactorizationResult{}, err
	}

	return domain.FactorizationResult{
		Original:   domain.NamedMatrix{Name: matrixNameA, Data: input.Matrix},
		Q:          q,
		R:          r,
		Mode:       factorization.Mode,
		Reflectors: factorization.Reflectors,
		Statistics: statistics,
	}, nil
}
