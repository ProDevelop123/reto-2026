/**
 * Verificacion independiente de la factorizacion, en el navegador.
 *
 * Este modulo NO confia en el backend: recalcula por su cuenta las dos
 * identidades que definen una factorizacion QR y mide cuanto se desvia el
 * resultado recibido.
 *
 *   1. A = Q x R          la identidad fundamental. Si se cumple, la
 *                         descomposicion reconstruye la matriz original.
 *   2. Q^T x Q = I        las columnas de Q son ortonormales. Es la propiedad
 *                         que distingue una implementacion estable de una
 *                         ingenua: Gram-Schmidt clasico tambien satisface la
 *                         primera identidad, pero pierde esta segunda cuando la
 *                         matriz esta mal condicionada.
 *
 * Que la comprobacion ocurra en el cliente convierte la interfaz en una prueba:
 * los residuos que se muestran no los calcula quien produjo el resultado.
 */

/** Producto de matrices. Devuelve null si las dimensiones no encajan. */
export function multiply(a: number[][], b: number[][]): number[][] | null {
  const rows = a.length;
  const inner = b.length;

  if (rows === 0 || inner === 0 || a[0].length !== inner) return null;

  const cols = b[0].length;
  const result: number[][] = Array.from({ length: rows }, () => new Array<number>(cols).fill(0));

  // Bucle i-k-j en lugar del clasico i-j-k: recorre ambas matrices por filas y
  // aprovecha mejor la cache que saltar por columnas.
  for (let i = 0; i < rows; i += 1) {
    for (let k = 0; k < inner; k += 1) {
      const factor = a[i][k];
      if (factor === 0) continue;

      for (let j = 0; j < cols; j += 1) {
        result[i][j] += factor * b[k][j];
      }
    }
  }

  return result;
}

/** Traspuesta de una matriz. */
export function transpose(m: number[][]): number[][] {
  if (m.length === 0) return [];

  return m[0].map((_, j) => m.map((row) => row[j]));
}

/** Matriz identidad de orden n. */
export function identity(n: number): number[][] {
  return Array.from({ length: n }, (_, i) =>
    Array.from({ length: n }, (_, j) => (i === j ? 1 : 0)),
  );
}

/**
 * Mayor diferencia absoluta elemento a elemento.
 *
 * Se usa la norma del maximo y no la de Frobenius porque acota el PEOR
 * elemento: una norma que promedia podria diluir un unico valor muy erroneo
 * entre cientos de valores correctos.
 */
export function maxAbsDifference(a: number[][], b: number[][]): number | null {
  if (a.length !== b.length) return null;

  let worst = 0;

  for (let i = 0; i < a.length; i += 1) {
    if (a[i].length !== b[i].length) return null;

    for (let j = 0; j < a[i].length; j += 1) {
      const diff = Math.abs(a[i][j] - b[i][j]);
      if (diff > worst) worst = diff;
    }
  }

  return worst;
}

/** Resultado de una comprobacion individual. */
export interface Check {
  /** Identidad comprobada, en notacion matematica. */
  label: string;
  /** Que significa que se cumpla, en lenguaje llano. */
  meaning: string;
  /** Residuo medido, o null si no se pudo calcular. */
  residual: number | null;
  /** Si el residuo esta dentro de lo esperable en doble precision. */
  passed: boolean;
}

/**
 * Umbral de aceptacion.
 *
 * El epsilon de la maquina en doble precision ronda 2.2e-16. El error de una
 * factorizacion crece con el tamano y el condicionamiento de la matriz, asi que
 * se deja un margen amplio: lo que se quiere detectar es un algoritmo
 * equivocado, que fallaria por muchos ordenes de magnitud, no la acumulacion
 * normal de redondeo.
 */
const RESIDUAL_TOLERANCE = 1e-9;

/**
 * Comprueba las dos identidades de la factorizacion.
 *
 * @param a Matriz original.
 * @param q Factor ortogonal.
 * @param r Factor triangular superior.
 */
export function verifyFactorization(a: number[][], q: number[][], r: number[][]): Check[] {
  const checks: Check[] = [];

  // --- A = Q x R ---
  const product = multiply(q, r);
  const reconstruction = product ? maxAbsDifference(a, product) : null;

  checks.push({
    label: "‖A − Q·R‖",
    meaning: "La factorizacion reconstruye la matriz original",
    residual: reconstruction,
    passed: reconstruction !== null && reconstruction <= RESIDUAL_TOLERANCE,
  });

  // --- Q^T x Q = I ---
  const gram = multiply(transpose(q), q);
  const orthogonality = gram ? maxAbsDifference(gram, identity(gram.length)) : null;

  checks.push({
    label: "‖QᵀQ − I‖",
    meaning: "Las columnas de Q son ortonormales",
    residual: orthogonality,
    passed: orthogonality !== null && orthogonality <= RESIDUAL_TOLERANCE,
  });

  return checks;
}
