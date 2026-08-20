// Package matrix implementa las operaciones de algebra lineal del servicio:
// el tipo Matrix y la factorizacion QR por reflexiones de Householder.
//
// El paquete es deliberadamente puro: no importa nada fuera de la biblioteca
// estandar, no conoce HTTP ni configuracion, y no registra logs. Esa pureza es
// lo que permite verificar el algoritmo con tests matematicos y reutilizarlo
// en cualquier contexto.
package matrix

import (
	"errors"
	"fmt"
	"math"
)

// Errores de validacion del paquete. Se exponen como valores centinela para
// que las capas superiores puedan distinguirlos con errors.Is y traducirlos al
// codigo HTTP adecuado, en lugar de comparar cadenas de texto.
var (
	// ErrEmpty indica que la matriz no tiene filas o no tiene columnas.
	ErrEmpty = errors.New("la matriz no puede estar vacia")

	// ErrNotRectangular indica que las filas no tienen todas la misma longitud.
	ErrNotRectangular = errors.New("la matriz no es rectangular")

	// ErrNotFinite indica la presencia de NaN o de infinito. Un solo valor no
	// finito contamina toda la factorizacion en silencio, asi que se rechaza
	// en la entrada en lugar de propagarlo al resultado.
	ErrNotFinite = errors.New("la matriz contiene valores no finitos")

	// ErrDimensionMismatch indica dimensiones incompatibles en un producto.
	ErrDimensionMismatch = errors.New("dimensiones incompatibles")
)

// Matrix representa una matriz densa como una rebanada de filas.
//
// Se elige esta representacion, en lugar de una rebanada plana con salto de
// fila, porque coincide exactamente con el formato JSON de entrada y salida
// (array de arrays) y evita conversiones en el borde HTTP. Una representacion
// plana tendria mejor localidad de cache, pero para las dimensiones de este
// servicio la diferencia es despreciable frente al coste de serializar JSON,
// y la claridad del codigo pesa mas.
type Matrix [][]float64

// New crea una matriz de ceros de las dimensiones indicadas.
//
// Reserva el almacenamiento en un unico bloque contiguo y reparte sub-rebanadas
// sobre el: se hace una sola asignacion en vez de una por fila, y las filas
// quedan contiguas en memoria, lo que mejora la localidad al recorrerlas.
func New(rows, cols int) Matrix {
	backing := make([]float64, rows*cols)
	m := make(Matrix, rows)

	for i := range m {
		m[i] = backing[i*cols : (i+1)*cols : (i+1)*cols]
	}

	return m
}

// Identity crea la matriz identidad de orden n.
func Identity(n int) Matrix {
	m := New(n, n)
	for i := 0; i < n; i++ {
		m[i][i] = 1
	}
	return m
}

// Rows devuelve el numero de filas.
func (m Matrix) Rows() int { return len(m) }

// Cols devuelve el numero de columnas. Presupone una matriz ya validada.
func (m Matrix) Cols() int {
	if len(m) == 0 {
		return 0
	}
	return len(m[0])
}

// Clone devuelve una copia independiente de la matriz.
//
// Se usa para no mutar la matriz de entrada del usuario durante la
// factorizacion: quien llama sigue siendo dueno de sus datos.
func (m Matrix) Clone() Matrix {
	clone := New(m.Rows(), m.Cols())
	for i, row := range m {
		copy(clone[i], row)
	}
	return clone
}

// Transpose devuelve la matriz traspuesta.
func (m Matrix) Transpose() Matrix {
	rows, cols := m.Rows(), m.Cols()
	t := New(cols, rows)

	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			t[j][i] = m[i][j]
		}
	}

	return t
}

// Mul devuelve el producto m * other.
//
// El bucle se ordena i-k-j en lugar del clasico i-j-k para recorrer ambas
// matrices por filas: asi se accede a `other[k]` de forma secuencial y se
// aprovecha la linea de cache, en vez de saltar por columnas.
func (m Matrix) Mul(other Matrix) (Matrix, error) {
	if m.Cols() != other.Rows() {
		return nil, fmt.Errorf("%w: %dx%d por %dx%d",
			ErrDimensionMismatch, m.Rows(), m.Cols(), other.Rows(), other.Cols())
	}

	rows, inner, cols := m.Rows(), m.Cols(), other.Cols()
	result := New(rows, cols)

	for i := 0; i < rows; i++ {
		resultRow := result[i]
		for k := 0; k < inner; k++ {
			factor := m[i][k]
			if factor == 0 {
				continue
			}
			otherRow := other[k]
			for j := 0; j < cols; j++ {
				resultRow[j] += factor * otherRow[j]
			}
		}
	}

	return result, nil
}

// MaxAbsDiff devuelve la mayor diferencia absoluta elemento a elemento entre
// dos matrices de las mismas dimensiones.
//
// Es la metrica con la que los tests comprueban las identidades de la
// factorizacion (A = Q*R y Q^T*Q = I): comparar flotantes por igualdad exacta
// no tendria sentido, lo que interesa es acotar el residuo.
func MaxAbsDiff(a, b Matrix) (float64, error) {
	if a.Rows() != b.Rows() || a.Cols() != b.Cols() {
		return 0, fmt.Errorf("%w: %dx%d frente a %dx%d",
			ErrDimensionMismatch, a.Rows(), a.Cols(), b.Rows(), b.Cols())
	}

	worst := 0.0
	for i := range a {
		for j := range a[i] {
			if d := math.Abs(a[i][j] - b[i][j]); d > worst {
				worst = d
			}
		}
	}

	return worst, nil
}

// Validate comprueba que los datos forman una matriz rectangular no vacia de
// valores finitos.
//
// Se ejecuta antes de factorizar porque el algoritmo de Householder presupone
// esas condiciones: una fila corta provocaria un panico por indice fuera de
// rango y un NaN se propagaria a todo el resultado sin sintomas visibles.
func Validate(m Matrix) error {
	if len(m) == 0 {
		return fmt.Errorf("%w: no tiene filas", ErrEmpty)
	}

	cols := len(m[0])
	if cols == 0 {
		return fmt.Errorf("%w: no tiene columnas", ErrEmpty)
	}

	for i, row := range m {
		if len(row) != cols {
			return fmt.Errorf("%w: la fila %d tiene %d elemento(s) y se esperaban %d",
				ErrNotRectangular, i, len(row), cols)
		}

		for j, value := range row {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return fmt.Errorf("%w: la posicion [%d][%d] no es un numero finito",
					ErrNotFinite, i, j)
			}
		}
	}

	return nil
}
