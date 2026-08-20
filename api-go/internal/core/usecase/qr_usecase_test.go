package usecase_test

import (
	"context"
	"errors"
	"testing"

	"api-go/internal/core/domain"
	"api-go/internal/core/usecase"
	"api-go/internal/matrix"
	"api-go/pkg/apperror"
)

// fakeStatistics es un doble en memoria del puerto StatisticsProvider.
//
// Aqui se ve el beneficio concreto de la arquitectura hexagonal: todo el caso
// de uso —factorizacion incluida— se ejercita sin levantar la API de Node, sin
// abrir un socket y sin depender de la red. Estos tests corren en microsegundos
// y no pueden fallar de forma intermitente.
type fakeStatistics struct {
	// received guarda la ultima peticion, para poder afirmar QUE se envio al
	// servicio remoto y no solo que la llamada ocurrio.
	received domain.StatisticsRequest
	calls    int
	result   domain.StatisticsResult
	err      error
}

func (f *fakeStatistics) Analyze(
	_ context.Context,
	request domain.StatisticsRequest,
) (domain.StatisticsResult, error) {
	f.calls++
	f.received = request

	if f.err != nil {
		return domain.StatisticsResult{}, f.err
	}

	return f.result, nil
}

func TestFactorizeSendsQAndRToStatisticsProvider(t *testing.T) {
	statistics := &fakeStatistics{}
	useCase := usecase.NewQRUseCase(statistics)

	input := domain.FactorizationInput{
		Matrix: matrix.Matrix{{1, 2}, {3, 4}, {5, 6}},
		Mode:   matrix.ModeReduced,
	}

	result, err := useCase.Factorize(context.Background(), input)
	if err != nil {
		t.Fatalf("Factorize devolvio error: %v", err)
	}

	if statistics.calls != 1 {
		t.Errorf("se llamo %d veces al proveedor, se esperaba 1", statistics.calls)
	}

	if len(statistics.received.Matrices) != 2 {
		t.Fatalf("se enviaron %d matrices, se esperaban 2", len(statistics.received.Matrices))
	}

	// Los nombres importan: son los que permiten al cliente saber cual de las
	// matrices resulto ser diagonal.
	if got := statistics.received.Matrices[0].Name; got != "Q" {
		t.Errorf("la primera matriz se llama %q, se esperaba \"Q\"", got)
	}
	if got := statistics.received.Matrices[1].Name; got != "R" {
		t.Errorf("la segunda matriz se llama %q, se esperaba \"R\"", got)
	}

	// Se comprueba que lo enviado es realmente la factorizacion y no la matriz
	// original: A = Q*R con las matrices que viajaron al proveedor.
	product, err := statistics.received.Matrices[0].Data.Mul(statistics.received.Matrices[1].Data)
	if err != nil {
		t.Fatalf("Q*R fallo: %v", err)
	}
	residual, err := matrix.MaxAbsDiff(input.Matrix, product)
	if err != nil {
		t.Fatalf("MaxAbsDiff fallo: %v", err)
	}
	if residual > 1e-10 {
		t.Errorf("las matrices enviadas no reconstruyen A, residuo %g", residual)
	}

	if result.Original.Name != "A" {
		t.Errorf("la matriz original se llama %q, se esperaba \"A\"", result.Original.Name)
	}
	if result.Mode != matrix.ModeReduced {
		t.Errorf("el modo devuelto es %q, se esperaba %q", result.Mode, matrix.ModeReduced)
	}
}

func TestFactorizePropagatesTolerance(t *testing.T) {
	statistics := &fakeStatistics{}
	useCase := usecase.NewQRUseCase(statistics)

	tolerance := 0.25

	_, err := useCase.Factorize(context.Background(), domain.FactorizationInput{
		Matrix:    matrix.Matrix{{1, 0}, {0, 1}},
		Mode:      matrix.ModeReduced,
		Tolerance: &tolerance,
	})
	if err != nil {
		t.Fatalf("Factorize devolvio error: %v", err)
	}

	if statistics.received.Tolerance == nil {
		t.Fatal("la tolerancia no se propago al proveedor")
	}
	if *statistics.received.Tolerance != tolerance {
		t.Errorf("tolerancia %g, se esperaba %g", *statistics.received.Tolerance, tolerance)
	}
}

func TestFactorizePropagatesZeroTolerance(t *testing.T) {
	statistics := &fakeStatistics{}
	useCase := usecase.NewQRUseCase(statistics)

	// Cero significa "comparacion exacta" y es un valor legitimo. Se comprueba
	// que sobrevive al viaje y no se confunde con "no indicado", que es la
	// razon por la que el campo es un puntero.
	zero := 0.0

	_, err := useCase.Factorize(context.Background(), domain.FactorizationInput{
		Matrix:    matrix.Matrix{{1, 0}, {0, 1}},
		Mode:      matrix.ModeReduced,
		Tolerance: &zero,
	})
	if err != nil {
		t.Fatalf("Factorize devolvio error: %v", err)
	}

	if statistics.received.Tolerance == nil {
		t.Fatal("la tolerancia cero se perdio por el camino")
	}
	if *statistics.received.Tolerance != 0 {
		t.Errorf("tolerancia %g, se esperaba 0", *statistics.received.Tolerance)
	}
}

func TestFactorizeRejectsInvalidMatrixBeforeCallingProvider(t *testing.T) {
	statistics := &fakeStatistics{}
	useCase := usecase.NewQRUseCase(statistics)

	_, err := useCase.Factorize(context.Background(), domain.FactorizationInput{
		Matrix: matrix.Matrix{{1, 2}, {3}},
		Mode:   matrix.ModeReduced,
	})

	if err == nil {
		t.Fatal("se esperaba un error de validacion")
	}

	var appErr *apperror.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("el error no es un AppError: %v", err)
	}
	if appErr.Status != 422 {
		t.Errorf("estado %d, se esperaba 422", appErr.Status)
	}

	// No se debe molestar al servicio remoto con una peticion que ya se sabe
	// invalida.
	if statistics.calls != 0 {
		t.Errorf("se llamo al proveedor %d veces pese a la entrada invalida", statistics.calls)
	}
}

func TestFactorizePropagatesProviderError(t *testing.T) {
	// El adaptador ya clasifica el fallo remoto con su codigo (502 o 504). El
	// caso de uso debe propagarlo tal cual, sin degradarlo a un 500 generico
	// que perderia esa informacion.
	upstream := apperror.Upstream("El servicio de estadisticas fallo.")

	statistics := &fakeStatistics{err: upstream}
	useCase := usecase.NewQRUseCase(statistics)

	_, err := useCase.Factorize(context.Background(), domain.FactorizationInput{
		Matrix: matrix.Matrix{{1, 2}, {3, 4}},
		Mode:   matrix.ModeReduced,
	})

	var appErr *apperror.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("el error no es un AppError: %v", err)
	}
	if appErr.Status != 502 {
		t.Errorf("estado %d, se esperaba 502", appErr.Status)
	}
	if appErr.Code != apperror.CodeUpstream {
		t.Errorf("codigo %q, se esperaba %q", appErr.Code, apperror.CodeUpstream)
	}
}

func TestFactorizeReturnsStatisticsFromProvider(t *testing.T) {
	statistics := &fakeStatistics{
		result: domain.StatisticsResult{
			Global: domain.GlobalStatistics{
				Matrices:         2,
				Max:              5,
				Min:              -7,
				Sum:              12,
				Average:          1.5,
				IsAnyDiagonal:    true,
				DiagonalMatrices: []string{"Q"},
			},
			Tolerance: 1e-9,
		},
	}

	useCase := usecase.NewQRUseCase(statistics)

	result, err := useCase.Factorize(context.Background(), domain.FactorizationInput{
		Matrix: matrix.Matrix{{1, 2}, {3, 4}},
		Mode:   matrix.ModeReduced,
	})
	if err != nil {
		t.Fatalf("Factorize devolvio error: %v", err)
	}

	if !result.Statistics.Global.IsAnyDiagonal {
		t.Error("las estadisticas del proveedor no llegaron al resultado")
	}
	if result.Statistics.Global.Max != 5 {
		t.Errorf("max %g, se esperaba 5", result.Statistics.Global.Max)
	}
}

func TestFactorizeCountsReflectors(t *testing.T) {
	statistics := &fakeStatistics{}
	useCase := usecase.NewQRUseCase(statistics)

	cases := []struct {
		name   string
		input  matrix.Matrix
		expect int
		why    string
	}{
		{
			name:   "rango completo",
			input:  matrix.Matrix{{1, 2}, {3, 4}, {5, 6}},
			expect: 2,
			why:    "una reflexion por columna",
		},
		{
			name:  "deficiente de rango sin ceros exactos",
			input: matrix.Matrix{{1, 2}, {2, 4}, {3, 6}},
			// La segunda columna es el doble de la primera, pero tras la primera
			// reflexion la subcolumna residual queda en el orden de 1e-16 y no en
			// cero exacto, asi que se aplica la segunda reflexion igualmente.
			// Detectar el rango de forma fiable exigiria pivoteo por columnas.
			expect: 2,
			why:    "el residuo no es cero exacto, se refleja de todas formas",
		},
		{
			name:  "columna exactamente nula",
			input: matrix.Matrix{{0, 2}, {0, 4}, {0, 6}},
			// Aqui la primera subcolumna SI es cero exacto, de modo que su
			// reflexion se omite. Es el unico caso en el que el contador baja.
			expect: 1,
			why:    "la reflexion de una columna nula se omite",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := useCase.Factorize(context.Background(), domain.FactorizationInput{
				Matrix: tc.input,
				Mode:   matrix.ModeReduced,
			})
			if err != nil {
				t.Fatalf("Factorize devolvio error: %v", err)
			}

			if result.Reflectors != tc.expect {
				t.Errorf("reflexiones %d, se esperaban %d (%s)",
					result.Reflectors, tc.expect, tc.why)
			}
		})
	}
}
