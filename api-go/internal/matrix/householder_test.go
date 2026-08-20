package matrix

import (
	"errors"
	"math"
	"math/rand"
	"testing"
)

// tolerance es el residuo maximo admitido al comprobar identidades sobre
// flotantes de doble precision. El epsilon de la maquina ronda 2.2e-16; el
// margen elegido deja holgura para el error acumulado sin llegar a ocultar un
// algoritmo defectuoso, que fallaria por varios ordenes de magnitud.
const tolerance = 1e-10

// assertReconstructsOriginal comprueba la identidad fundamental A = Q*R.
//
// Es la unica prueba que realmente importa: si se cumple con un residuo del
// orden del epsilon de la maquina, la factorizacion es correcta, sea cual sea
// la forma concreta de Q y R.
func assertReconstructsOriginal(t *testing.T, a Matrix, f Factorization) {
	t.Helper()

	product, err := f.Q.Mul(f.R)
	if err != nil {
		t.Fatalf("no se pudo multiplicar Q por R: %v", err)
	}

	residual, err := MaxAbsDiff(a, product)
	if err != nil {
		t.Fatalf("Q*R no tiene las dimensiones de A: %v", err)
	}

	if residual > tolerance {
		t.Errorf("A != Q*R, residuo maximo %g (tolerancia %g)", residual, tolerance)
	}
}

// assertOrthonormalColumns comprueba que Q^T*Q = I, es decir, que las columnas
// de Q son ortogonales entre si y de norma unidad.
//
// Esta es la propiedad que distingue una implementacion estable de una
// ingenua: Gram-Schmidt clasico tambien satisface A = Q*R, pero pierde la
// ortogonalidad de Q cuando la matriz esta mal condicionada.
func assertOrthonormalColumns(t *testing.T, q Matrix) {
	t.Helper()

	gram, err := q.Transpose().Mul(q)
	if err != nil {
		t.Fatalf("no se pudo calcular Q^T*Q: %v", err)
	}

	residual, err := MaxAbsDiff(gram, Identity(q.Cols()))
	if err != nil {
		t.Fatalf("Q^T*Q no es cuadrada del orden esperado: %v", err)
	}

	if residual > tolerance {
		t.Errorf("Q^T*Q != I, residuo maximo %g (tolerancia %g)", residual, tolerance)
	}
}

// assertUpperTriangular comprueba que R no tiene elementos por debajo de la
// diagonal, y que son cero EXACTO y no residuos de redondeo.
func assertUpperTriangular(t *testing.T, r Matrix) {
	t.Helper()

	for i := 1; i < r.Rows(); i++ {
		for j := 0; j < i && j < r.Cols(); j++ {
			if r[i][j] != 0 {
				t.Errorf("R[%d][%d] = %g, se esperaba cero exacto", i, j, r[i][j])
			}
		}
	}
}

func TestDecomposeKnownMatrix(t *testing.T) {
	// Ejemplo clasico de matriz rectangular 3x2 de rango completo.
	a := Matrix{{1, 2}, {3, 4}, {5, 6}}

	f, err := Decompose(a, ModeReduced)
	if err != nil {
		t.Fatalf("Decompose devolvio error: %v", err)
	}

	if f.Q.Rows() != 3 || f.Q.Cols() != 2 {
		t.Errorf("Q es %dx%d, se esperaba 3x2", f.Q.Rows(), f.Q.Cols())
	}
	if f.R.Rows() != 2 || f.R.Cols() != 2 {
		t.Errorf("R es %dx%d, se esperaba 2x2", f.R.Rows(), f.R.Cols())
	}

	assertReconstructsOriginal(t, a, f)
	assertOrthonormalColumns(t, f.Q)
	assertUpperTriangular(t, f.R)

	// La convencion de signo de Householder deja la diagonal de R con el signo
	// opuesto al pivote original. Se comprueba para fijar el comportamiento:
	// la factorizacion QR solo es unica salvo signos, y conviene que el
	// resultado sea reproducible entre ejecuciones.
	if f.R[0][0] >= 0 {
		t.Errorf("R[0][0] = %g, se esperaba negativo por la convencion de signo", f.R[0][0])
	}
}

func TestDecomposeShapes(t *testing.T) {
	shapes := []struct {
		name       string
		rows, cols int
	}{
		{"vertical alta", 6, 2},
		{"vertical estrecha", 10, 1},
		{"cuadrada pequena", 3, 3},
		{"cuadrada mediana", 12, 12},
		{"horizontal ancha", 2, 6},
		{"horizontal muy ancha", 1, 8},
		{"escalar", 1, 1},
		{"casi cuadrada", 7, 6},
	}

	for _, shape := range shapes {
		t.Run(shape.name, func(t *testing.T) {
			// Semilla fija: un fallo debe poder reproducirse exactamente.
			a := randomMatrix(shape.rows, shape.cols, rand.New(rand.NewSource(42)))

			for _, mode := range []Mode{ModeReduced, ModeComplete} {
				f, err := Decompose(a, mode)
				if err != nil {
					t.Fatalf("modo %s: %v", mode, err)
				}

				assertReconstructsOriginal(t, a, f)
				assertOrthonormalColumns(t, f.Q)
				assertUpperTriangular(t, f.R)

				k := min(shape.rows, shape.cols)
				wantQCols, wantRRows := k, k
				if mode == ModeComplete {
					wantQCols, wantRRows = shape.rows, shape.rows
				}

				if f.Q.Rows() != shape.rows || f.Q.Cols() != wantQCols {
					t.Errorf("modo %s: Q es %dx%d, se esperaba %dx%d",
						mode, f.Q.Rows(), f.Q.Cols(), shape.rows, wantQCols)
				}
				if f.R.Rows() != wantRRows || f.R.Cols() != shape.cols {
					t.Errorf("modo %s: R es %dx%d, se esperaba %dx%d",
						mode, f.R.Rows(), f.R.Cols(), wantRRows, shape.cols)
				}
			}
		})
	}
}

func TestDecomposeCompleteModeGivesFullyOrthogonalQ(t *testing.T) {
	a := Matrix{{1, 2}, {3, 4}, {5, 6}}

	f, err := Decompose(a, ModeComplete)
	if err != nil {
		t.Fatalf("Decompose devolvio error: %v", err)
	}

	// En el modo completo Q es cuadrada y ortogonal, de modo que ademas de
	// Q^T*Q = I se cumple Q*Q^T = I. En el modo reducido la segunda identidad
	// NO se cumple: Q solo tiene columnas ortonormales, no es ortogonal.
	product, err := f.Q.Mul(f.Q.Transpose())
	if err != nil {
		t.Fatalf("no se pudo calcular Q*Q^T: %v", err)
	}

	residual, err := MaxAbsDiff(product, Identity(3))
	if err != nil {
		t.Fatalf("Q*Q^T no es 3x3: %v", err)
	}
	if residual > tolerance {
		t.Errorf("Q*Q^T != I, residuo maximo %g", residual)
	}
}

func TestDecomposeRankDeficient(t *testing.T) {
	// La segunda columna es exactamente el doble de la primera, asi que la
	// matriz tiene rango 1. La factorizacion sigue existiendo: R queda con un
	// cero en la diagonal. Un algoritmo que no contemple este caso divide por
	// cero y devuelve NaN.
	a := Matrix{{1, 2}, {2, 4}, {3, 6}}

	f, err := Decompose(a, ModeReduced)
	if err != nil {
		t.Fatalf("Decompose devolvio error: %v", err)
	}

	assertReconstructsOriginal(t, a, f)
	assertOrthonormalColumns(t, f.Q)
	assertUpperTriangular(t, f.R)

	if math.Abs(f.R[1][1]) > tolerance {
		t.Errorf("R[1][1] = %g, se esperaba cero por la deficiencia de rango", f.R[1][1])
	}
}

func TestDecomposeZeroMatrix(t *testing.T) {
	// Ninguna subcolumna tiene norma no nula, asi que no se aplica ninguna
	// reflexion y Q debe quedar como la identidad recortada.
	a := New(3, 2)

	f, err := Decompose(a, ModeReduced)
	if err != nil {
		t.Fatalf("Decompose devolvio error: %v", err)
	}

	if f.Reflectors != 0 {
		t.Errorf("se aplicaron %d reflexiones sobre la matriz nula, se esperaban 0", f.Reflectors)
	}

	assertReconstructsOriginal(t, a, f)
	assertOrthonormalColumns(t, f.Q)

	for i := range f.R {
		for j := range f.R[i] {
			if f.R[i][j] != 0 {
				t.Errorf("R[%d][%d] = %g, se esperaba cero", i, j, f.R[i][j])
			}
		}
	}
}

func TestDecomposeAlreadyUpperTriangular(t *testing.T) {
	// Una matriz que ya es triangular superior con diagonal positiva obliga al
	// algoritmo a reflejar de todas formas (por la convencion de signo), y el
	// resultado debe seguir reconstruyendo A.
	a := Matrix{{3, 4}, {0, 5}}

	f, err := Decompose(a, ModeReduced)
	if err != nil {
		t.Fatalf("Decompose devolvio error: %v", err)
	}

	assertReconstructsOriginal(t, a, f)
	assertOrthonormalColumns(t, f.Q)
	assertUpperTriangular(t, f.R)
}

func TestDecomposeIllConditioned(t *testing.T) {
	// Matriz de Lauchli: sus columnas son casi identicas entre si. Es el
	// contraejemplo clasico con el que Gram-Schmidt clasico pierde por
	// completo la ortogonalidad de Q, mientras que Householder la conserva
	// hasta el epsilon de la maquina.
	const eps = 1e-8
	a := Matrix{
		{1, 1, 1},
		{eps, 0, 0},
		{0, eps, 0},
		{0, 0, eps},
	}

	f, err := Decompose(a, ModeReduced)
	if err != nil {
		t.Fatalf("Decompose devolvio error: %v", err)
	}

	assertReconstructsOriginal(t, a, f)
	assertOrthonormalColumns(t, f.Q)
}

func TestDecomposeHandlesExtremeMagnitudes(t *testing.T) {
	// Valores tan grandes que elevarlos al cuadrado desbordaria a infinito, y
	// tan pequenos que subdesbordarian a cero. El calculo escalado de la norma
	// es lo que permite factorizar estas matrices.
	cases := map[string]Matrix{
		"muy grandes":  {{1e200, 2e200}, {3e200, 4e200}},
		"muy pequenos": {{1e-200, 2e-200}, {3e-200, 4e-200}},
	}

	for name, a := range cases {
		t.Run(name, func(t *testing.T) {
			f, err := Decompose(a, ModeReduced)
			if err != nil {
				t.Fatalf("Decompose devolvio error: %v", err)
			}

			for i := range f.R {
				for j := range f.R[i] {
					if math.IsNaN(f.R[i][j]) || math.IsInf(f.R[i][j], 0) {
						t.Fatalf("R[%d][%d] = %g, se desbordo el calculo", i, j, f.R[i][j])
					}
				}
			}

			assertOrthonormalColumns(t, f.Q)
		})
	}
}

func TestDecomposeDoesNotMutateInput(t *testing.T) {
	a := Matrix{{1, 2}, {3, 4}, {5, 6}}
	original := a.Clone()

	if _, err := Decompose(a, ModeReduced); err != nil {
		t.Fatalf("Decompose devolvio error: %v", err)
	}

	residual, err := MaxAbsDiff(a, original)
	if err != nil {
		t.Fatalf("MaxAbsDiff fallo: %v", err)
	}
	if residual != 0 {
		t.Errorf("la matriz de entrada fue modificada, diferencia maxima %g", residual)
	}
}

func TestDecomposeIsDeterministic(t *testing.T) {
	a := randomMatrix(8, 5, rand.New(rand.NewSource(7)))

	first, err := Decompose(a, ModeReduced)
	if err != nil {
		t.Fatalf("primera llamada: %v", err)
	}
	second, err := Decompose(a, ModeReduced)
	if err != nil {
		t.Fatalf("segunda llamada: %v", err)
	}

	// Bit a bit: no hay concurrencia ni recorridos de mapa dentro del
	// algoritmo, asi que dos ejecuciones deben coincidir exactamente.
	for _, pair := range []struct{ x, y Matrix }{{first.Q, second.Q}, {first.R, second.R}} {
		residual, err := MaxAbsDiff(pair.x, pair.y)
		if err != nil {
			t.Fatalf("MaxAbsDiff fallo: %v", err)
		}
		if residual != 0 {
			t.Errorf("dos ejecuciones difieren en %g", residual)
		}
	}
}

func TestDecomposeRejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name  string
		input Matrix
		want  error
	}{
		{"sin filas", Matrix{}, ErrEmpty},
		{"fila vacia", Matrix{{}}, ErrEmpty},
		{"no rectangular", Matrix{{1, 2}, {3}}, ErrNotRectangular},
		{"contiene NaN", Matrix{{1, math.NaN()}, {3, 4}}, ErrNotFinite},
		{"contiene infinito", Matrix{{1, 2}, {math.Inf(1), 4}}, ErrNotFinite},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decompose(tc.input, ModeReduced)
			if !errors.Is(err, tc.want) {
				t.Errorf("error = %v, se esperaba que envolviera %v", err, tc.want)
			}
		})
	}
}

// randomMatrix genera una matriz con valores en [-10, 10) a partir de una
// fuente de aleatoriedad explicita, para que los tests sean reproducibles.
func randomMatrix(rows, cols int, rng *rand.Rand) Matrix {
	m := New(rows, cols)
	for i := range m {
		for j := range m[i] {
			m[i][j] = rng.Float64()*20 - 10
		}
	}
	return m
}

func BenchmarkDecompose(b *testing.B) {
	sizes := []struct {
		name       string
		rows, cols int
	}{
		{"50x50", 50, 50},
		{"200x50", 200, 50},
		{"200x200", 200, 200},
	}

	for _, size := range sizes {
		a := randomMatrix(size.rows, size.cols, rand.New(rand.NewSource(1)))

		b.Run(size.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := Decompose(a, ModeReduced); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
