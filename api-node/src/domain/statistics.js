/**
 * Dominio: calculo de estadisticas sobre matrices.
 *
 * Este modulo es logica pura: no conoce HTTP, ni Express, ni de donde vienen
 * las matrices. Recibe arrays de numeros y devuelve objetos planos. Esa
 * independencia es lo que permite testearlo sin levantar el servidor.
 *
 * Decision de diseno: no se usa ninguna libreria matematica externa. Las cinco
 * metricas exigidas por el reto (maximo, minimo, promedio, suma y deteccion de
 * matriz diagonal) se resuelven en UNA sola pasada O(m*n) sobre la matriz, sin
 * asignar arrays intermedios (nada de flatten). Traer una dependencia pesada
 * para esto costaria tamano de bundle y tiempo de arranque sin aportar nada.
 */

/**
 * Tolerancia por defecto para considerar que un valor flotante "es cero".
 *
 * Las matrices que llegan aqui provienen de una factorizacion QR, donde los
 * ceros teoricos (por ejemplo, el triangulo inferior de R) en la practica son
 * residuos del orden de 1e-16. Comparar contra cero exacto haria que ninguna
 * matriz real se detectara como diagonal.
 */
export const DEFAULT_TOLERANCE = 1e-9;

/**
 * Acumulador de suma compensada (algoritmo de Neumaier, una mejora sobre
 * Kahan que ademas es correcta cuando el siguiente termino es mayor en
 * magnitud que el acumulado).
 *
 * Motivacion: los valores de Q suelen ser del orden de 1e-1 mientras que los
 * de R pueden ser varios ordenes de magnitud mayores. Una suma ingenua con
 * `total += valor` acumula error de redondeo que despues se propaga al
 * promedio. La compensacion cuesta un par de operaciones por elemento y
 * mantiene la precision cercana a la de doble precision exacta.
 */
class CompensatedSum {
  #sum = 0;
  #compensation = 0;

  add(value) {
    const next = this.#sum + value;

    // Se recupera el error de redondeo perdido en la suma anterior,
    // distinguiendo cual de los dos operandos es mayor en magnitud.
    if (Math.abs(this.#sum) >= Math.abs(value)) {
      this.#compensation += this.#sum - next + value;
    } else {
      this.#compensation += value - next + this.#sum;
    }

    this.#sum = next;
  }

  get value() {
    return this.#sum + this.#compensation;
  }
}

/**
 * Determina si una matriz es diagonal: todos sus elementos fuera de la
 * diagonal principal son cero (dentro de la tolerancia dada).
 *
 * Se adopta la definicion general `a[i][j] == 0 para todo i != j`, que admite
 * matrices rectangulares diagonales (las mismas que aparecen, por ejemplo,
 * como matriz sigma en una descomposicion SVD). No se exige que los elementos
 * de la diagonal sean distintos de cero, de modo que la matriz nula se
 * considera diagonal, en linea con la definicion estandar del algebra lineal.
 *
 * Esta funcion se expone por separado por claridad y para poder testearla de
 * forma aislada, pero el calculo real ocurre dentro de la pasada unica de
 * `computeMatrixStatistics` para no recorrer la matriz dos veces.
 *
 * @param {number[][]} data Matriz como array de filas.
 * @param {number} [tolerance] Umbral absoluto bajo el cual un valor se toma como cero.
 * @returns {boolean}
 */
export function isDiagonal(data, tolerance = DEFAULT_TOLERANCE) {
  for (let i = 0; i < data.length; i += 1) {
    const row = data[i];
    for (let j = 0; j < row.length; j += 1) {
      if (i !== j && Math.abs(row[j]) > tolerance) {
        return false;
      }
    }
  }
  return true;
}

/**
 * Calcula todas las estadisticas de una sola matriz en una unica pasada.
 *
 * @param {number[][]} data Matriz no vacia como array de filas de igual longitud.
 * @param {object} [options]
 * @param {number} [options.tolerance] Umbral de cero para la deteccion de diagonal.
 * @returns {{
 *   rows: number, columns: number, count: number,
 *   max: number, min: number, sum: number, average: number,
 *   isSquare: boolean, isDiagonal: boolean
 * }}
 */
export function computeMatrixStatistics(data, { tolerance = DEFAULT_TOLERANCE } = {}) {
  const rows = data.length;
  const columns = rows > 0 ? data[0].length : 0;

  let max = -Infinity;
  let min = Infinity;
  let diagonal = true;
  const sum = new CompensatedSum();

  for (let i = 0; i < rows; i += 1) {
    const row = data[i];

    for (let j = 0; j < columns; j += 1) {
      const value = row[j];

      if (value > max) max = value;
      if (value < min) min = value;
      sum.add(value);

      // Se evalua la condicion de diagonal en la misma pasada. No se corta el
      // bucle al primer fallo porque el resto de metricas necesita recorrer
      // todos los elementos de todas formas.
      if (diagonal && i !== j && Math.abs(value) > tolerance) {
        diagonal = false;
      }
    }
  }

  const count = rows * columns;
  const total = sum.value;

  return {
    rows,
    columns,
    count,
    max,
    min,
    sum: total,
    average: total / count,
    isSquare: rows === columns,
    isDiagonal: diagonal,
  };
}

/**
 * Agrega las estadisticas de un conjunto de matrices.
 *
 * Devuelve dos niveles de resultado:
 *  - `perMatrix`: las metricas de cada matriz por separado, lo que permite al
 *    consumidor saber, por ejemplo, que la matriz R es la diagonal y no la Q.
 *  - `global`: las metricas sobre el total de valores de todas las matrices,
 *    que es lo que pide literalmente el enunciado ("el valor maximo encontrado
 *    en las matrices").
 *
 * El campo `global.isAnyDiagonal` responde al requisito "verificar si alguna
 * matriz es diagonal"; `global.diagonalMatrices` indica ademas cuales.
 *
 * La agregacion global se calcula a partir de los resultados parciales en vez
 * de recorrer los datos otra vez: el maximo global es el maximo de los maximos
 * y la suma global es la suma de las sumas, de modo que el coste total sigue
 * siendo una sola pasada sobre los datos.
 *
 * @param {Array<{name?: string, data: number[][]}>} matrices Coleccion no vacia.
 * @param {object} [options]
 * @param {number} [options.tolerance]
 * @returns {{ global: object, perMatrix: object[] }}
 */
export function computeAggregatedStatistics(matrices, { tolerance = DEFAULT_TOLERANCE } = {}) {
  const perMatrix = matrices.map((matrix, index) => ({
    name: matrix.name ?? `matrix_${index}`,
    ...computeMatrixStatistics(matrix.data, { tolerance }),
  }));

  let max = -Infinity;
  let min = Infinity;
  let count = 0;
  const sum = new CompensatedSum();
  const diagonalMatrices = [];

  for (const stats of perMatrix) {
    if (stats.max > max) max = stats.max;
    if (stats.min < min) min = stats.min;
    sum.add(stats.sum);
    count += stats.count;

    if (stats.isDiagonal) diagonalMatrices.push(stats.name);
  }

  const total = sum.value;

  return {
    global: {
      matrices: perMatrix.length,
      count,
      max,
      min,
      sum: total,
      average: total / count,
      isAnyDiagonal: diagonalMatrices.length > 0,
      diagonalMatrices,
    },
    perMatrix,
  };
}
