package matrix

import "math"

// Mode selecciona la variante de factorizacion QR que se desea obtener.
type Mode string

const (
	// ModeReduced produce la factorizacion QR reducida (tambien llamada "thin"):
	// para una matriz A de m x n con k = min(m, n), devuelve Q de m x k con
	// columnas ortonormales y R de k x n triangular superior.
	//
	// Es el modo por defecto porque es el que se usa en la practica: contiene
	// toda la informacion necesaria para reconstruir A y para resolver minimos
	// cuadrados, sin arrastrar las m-k columnas de Q que solo aportan una base
	// del complemento ortogonal. Para una matriz de 1000 x 3, la Q completa
	// seria de 1000 x 1000 (un millon de valores) frente a 3000.
	ModeReduced Mode = "reduced"

	// ModeComplete produce la factorizacion QR completa: Q de m x m ortogonal
	// y R de m x n triangular superior. Se ofrece porque es la definicion
	// formal de la descomposicion y permite comprobar la ortogonalidad de Q en
	// ambos sentidos (Q^T*Q = I y Q*Q^T = I).
	ModeComplete Mode = "complete"
)

// Factorization es el resultado de descomponer una matriz A como A = Q * R.
type Factorization struct {
	// Q tiene columnas ortonormales: Q^T * Q = I.
	Q Matrix
	// R es triangular superior: sus elementos por debajo de la diagonal son
	// exactamente cero.
	R Matrix
	// Mode indica la variante con la que se calculo la factorizacion.
	Mode Mode
	// Reflectors es el numero de reflexiones de Householder que se aplicaron.
	//
	// Normalmente vale min(m, n). Solo baja de ahi cuando una subcolumna es
	// EXACTAMENTE nula, como ocurre con una columna de ceros.
	//
	// No es un estimador del rango, y conviene no confundirlo con uno. Sobre
	// una matriz deficiente de rango pero sin ceros exactos, la subcolumna
	// residual tras la reflexion anterior queda en el orden de 1e-16 en vez de
	// en cero, de modo que se aplica una reflexion mas sobre lo que es ruido de
	// redondeo. Es el mismo comportamiento de LAPACK y no afecta a la validez
	// del resultado: el elemento correspondiente de la diagonal de R sale
	// despreciable. Determinar el rango numerico de forma fiable exige QR con
	// pivoteo por columnas, que es un algoritmo distinto y no el que se
	// implementa aqui.
	Reflectors int
}

// Decompose calcula la factorizacion QR de a mediante reflexiones de
// Householder y devuelve Q y R tales que a = Q * R.
//
// # Por que Householder
//
// Existen tres caminos habituales hacia la QR:
//
//   - Gram-Schmidt clasico: intuitivo, pero numericamente inestable. Cuando las
//     columnas de A estan casi alineadas, la ortogonalidad de Q se degrada de
//     forma severa por cancelacion catastrofica.
//   - Rotaciones de Givens: estable y la mejor opcion cuando la matriz es
//     dispersa o casi triangular, porque anula un solo elemento por rotacion.
//     Para una matriz densa hace mas trabajo del necesario.
//   - Reflexiones de Householder: anula una columna entera por reflexion, es
//     incondicionalmente estable y es lo que usan LAPACK y las bibliotecas de
//     referencia para matrices densas.
//
// Se elige Householder por ser el estandar para el caso de este servicio:
// matrices densas de dimensiones arbitrarias.
//
// # El algoritmo
//
// En cada columna j se construye un vector unitario v y con el la reflexion
// H = I - 2*v*v^T, que es simetrica y ortogonal. H se elige de modo que refleje
// la subcolumna j sobre el eje e1, anulando todos sus elementos por debajo de
// la diagonal. Tras k reflexiones, R = H_k * ... * H_1 * A es triangular
// superior, y como cada H es su propia inversa, A = H_1 * ... * H_k * R, de
// donde Q = H_1 * ... * H_k.
//
// Las reflexiones nunca se materializan como matrices m x m: aplicar
// H = I - 2*v*v^T a una matriz M equivale a M - 2*v*(v^T*M), que es un producto
// matriz-vector mas una actualizacion de rango uno. El coste pasa de O(m^2)
// por reflexion a O(m*n), y no se reserva memoria adicional.
//
// # Estabilidad
//
// El signo de la reflexion se escoge para alejar el vector de su reflejo en
// lugar de acercarlo. Si se eligiera el signo contrario y el vector ya
// estuviera casi alineado con e1, la resta v = x - alfa*e1 sufriria cancelacion
// catastrofica: se restarian dos cantidades casi iguales y el resultado tendria
// muy pocos digitos significativos. Es el detalle que hace estable al
// algoritmo, y omitirlo es el error clasico al implementarlo.
//
// La matriz de entrada no se modifica: se trabaja sobre una copia.
func Decompose(a Matrix, mode Mode) (Factorization, error) {
	if err := Validate(a); err != nil {
		return Factorization{}, err
	}

	rows, cols := a.Rows(), a.Cols()

	// Numero de reflexiones posibles. Mas alla de min(m, n) no queda ninguna
	// subcolumna que anular.
	steps := rows
	if cols < steps {
		steps = cols
	}

	r := a.Clone()      // Acumula H_k * ... * H_1 * A hasta volverse triangular.
	q := Identity(rows) // Acumula H_1 * ... * H_k.

	// Espacio de trabajo reutilizado por todas las reflexiones: el vector v y
	// el vector de proyecciones. Reservarlos una sola vez evita que el
	// recolector de basura trabaje dentro del bucle principal.
	v := make([]float64, rows)
	projections := make([]float64, max(rows, cols))

	applied := 0

	for j := 0; j < steps; j++ {
		length := rows - j // Longitud de la subcolumna que se va a anular.

		// Norma euclidea de la subcolumna A[j:m, j], calculada con escalado
		// para no desbordar ni subdesbordar con valores extremos.
		norm := columnNorm(r, j)

		// Si la subcolumna ya es nula no hay nada que reflejar. Ocurre con
		// matrices deficientes de rango; saltar el paso mantiene R triangular
		// y evita dividir por cero.
		if norm == 0 {
			continue
		}

		// alfa es el valor que ocupara la diagonal de R en esta columna. Se le
		// da el signo OPUESTO al primer elemento de la subcolumna para que la
		// resta siguiente sume magnitudes en lugar de cancelarlas.
		alpha := -math.Copysign(norm, r[j][j])

		// v = x - alfa*e1, es decir, la subcolumna con su primer elemento
		// desplazado. El resto de componentes se copia tal cual.
		v = v[:length]
		v[0] = r[j][j] - alpha
		for i := 1; i < length; i++ {
			v[i] = r[j+i][j]
		}

		// Se normaliza v para que H = I - 2*v*v^T sea una reflexion, evitando
		// tener que arrastrar el factor 2/(v^T*v) en cada aplicacion.
		vNorm := vectorNorm(v)
		if vNorm == 0 {
			// La subcolumna ya estaba en la direccion de e1 con el signo
			// correcto: la reflexion seria la identidad.
			continue
		}
		for i := range v {
			v[i] /= vNorm
		}

		applyLeft(r, v, j, projections)
		applyRight(q, v, j, projections)

		applied++
	}

	// Los elementos por debajo de la diagonal de R son cero por construccion,
	// pero en aritmetica de punto flotante quedan como residuos del orden de
	// 1e-17. Se anulan de forma explicita: no son valores calculados sino
	// ceros estructurales, y devolver ruido de redondeo en su lugar seria
	// enganoso para quien consuma el resultado.
	zeroBelowDiagonal(r)

	if mode == ModeReduced {
		q, r = reduce(q, r, steps)
	}

	return Factorization{Q: q, R: r, Mode: mode, Reflectors: applied}, nil
}

// applyLeft aplica la reflexion H = I - 2*v*v^T por la izquierda a la
// submatriz m[j:, j:], es decir, calcula m = H*m.
//
// Se implementa como m -= 2*v*(v^T*m), evitando construir H.
func applyLeft(m Matrix, v []float64, j int, projections []float64) {
	rows, cols := m.Rows(), m.Cols()

	// projections[c] = v^T * m[j:, c] para cada columna afectada.
	proj := projections[:cols]
	for c := j; c < cols; c++ {
		proj[c] = 0
	}
	for i := j; i < rows; i++ {
		factor := v[i-j]
		if factor == 0 {
			continue
		}
		row := m[i]
		for c := j; c < cols; c++ {
			proj[c] += factor * row[c]
		}
	}

	// Actualizacion de rango uno: m[i][c] -= 2 * v[i] * projections[c].
	for i := j; i < rows; i++ {
		factor := 2 * v[i-j]
		if factor == 0 {
			continue
		}
		row := m[i]
		for c := j; c < cols; c++ {
			row[c] -= factor * proj[c]
		}
	}
}

// applyRight aplica la reflexion por la derecha a las columnas j: de m, es
// decir, calcula m = m*H.
//
// Se usa para acumular Q = H_1 * ... * H_k paso a paso.
func applyRight(m Matrix, v []float64, j int, projections []float64) {
	rows, cols := m.Rows(), m.Cols()

	// projections[i] = m[i, j:] * v para cada fila.
	proj := projections[:rows]

	for i := 0; i < rows; i++ {
		row := m[i]
		sum := 0.0
		for c := j; c < cols; c++ {
			sum += row[c] * v[c-j]
		}
		proj[i] = sum
	}

	for i := 0; i < rows; i++ {
		factor := 2 * proj[i]
		if factor == 0 {
			continue
		}
		row := m[i]
		for c := j; c < cols; c++ {
			row[c] -= factor * v[c-j]
		}
	}
}

// reduce recorta la factorizacion completa a su forma reducida, quedandose con
// las primeras k columnas de Q y las primeras k filas de R.
//
// Las columnas descartadas de Q generan el complemento ortogonal del espacio
// columna de A, y las filas descartadas de R son nulas, de modo que el producto
// Q*R no cambia.
func reduce(q, r Matrix, k int) (Matrix, Matrix) {
	reducedQ := New(q.Rows(), k)
	for i := 0; i < q.Rows(); i++ {
		copy(reducedQ[i], q[i][:k])
	}

	reducedR := New(k, r.Cols())
	for i := 0; i < k; i++ {
		copy(reducedR[i], r[i])
	}

	return reducedQ, reducedR
}

// zeroBelowDiagonal anula explicitamente el triangulo estrictamente inferior.
func zeroBelowDiagonal(m Matrix) {
	for i := 1; i < m.Rows(); i++ {
		limit := i
		if c := m.Cols(); limit > c {
			limit = c
		}
		row := m[i]
		for j := 0; j < limit; j++ {
			row[j] = 0
		}
	}
}

// columnNorm devuelve la norma euclidea de la subcolumna m[j:, j].
func columnNorm(m Matrix, j int) float64 {
	scale, sumSquares := 0.0, 1.0

	// Algoritmo de escalado dinamico: se normaliza sobre la marcha por el mayor
	// valor visto hasta el momento. Elevar al cuadrado directamente desbordaria
	// con valores del orden de 1e200 y subdesbordaria a cero con 1e-200, aunque
	// la norma resultante fuese perfectamente representable.
	for i := j; i < m.Rows(); i++ {
		value := math.Abs(m[i][j])
		if value == 0 {
			continue
		}

		if value > scale {
			ratio := scale / value
			sumSquares = 1 + sumSquares*ratio*ratio
			scale = value
		} else {
			ratio := value / scale
			sumSquares += ratio * ratio
		}
	}

	if scale == 0 {
		return 0
	}

	return scale * math.Sqrt(sumSquares)
}

// vectorNorm devuelve la norma euclidea de un vector, con el mismo escalado
// dinamico que columnNorm.
func vectorNorm(v []float64) float64 {
	scale, sumSquares := 0.0, 1.0

	for _, raw := range v {
		value := math.Abs(raw)
		if value == 0 {
			continue
		}

		if value > scale {
			ratio := scale / value
			sumSquares = 1 + sumSquares*ratio*ratio
			scale = value
		} else {
			ratio := value / scale
			sumSquares += ratio * ratio
		}
	}

	if scale == 0 {
		return 0
	}

	return scale * math.Sqrt(sumSquares)
}
