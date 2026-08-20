/**
 * Matrices de ejemplo.
 *
 * No son decorativas: cada una ejercita una propiedad concreta del backend y
 * permite comprobarla en un clic, sin teclear una sola celda.
 */
export interface Preset {
  id: string;
  label: string;
  /** Que demuestra este ejemplo. Se muestra al usuario. */
  description: string;
  matrix: number[][];
}

export const PRESETS: Preset[] = [
  {
    id: "classic",
    label: "Clásica 3×3",
    description:
      "Ejemplo canónico de los libros de álgebra lineal. R reproduce los valores de " +
      "referencia [[-14,-21,14],[0,-175,70],[0,0,35]] con una desviación de ~1e-14: en " +
      "aritmética de punto flotante no son enteros exactos, aunque lo parezcan al mostrarlos.",
    matrix: [
      [12, -51, 4],
      [6, 167, -68],
      [-4, 24, -41],
    ],
  },
  {
    id: "rectangular",
    label: "Rectangular 3×2",
    description:
      "Matriz no cuadrada, que es el caso que pide explícitamente el enunciado. " +
      "En modo reducido, Q queda de 3×2 y R de 2×2.",
    matrix: [
      [1, 2],
      [3, 4],
      [5, 6],
    ],
  },
  {
    id: "diagonal",
    label: "Diagonal",
    description:
      "Activa la quinta métrica del enunciado: isAnyDiagonal pasa a true. " +
      "Con esta entrada, tanto Q como R resultan diagonales.",
    matrix: [
      [3, 0],
      [0, 5],
    ],
  },
  {
    id: "rank-deficient",
    label: "Rango deficiente",
    description:
      "La segunda columna es el doble de la primera, de modo que la matriz tiene rango 1. " +
      "R[1][1] sale ≈ 0 sin que el algoritmo divida por cero: es el caso límite que rompe " +
      "las implementaciones ingenuas.",
    matrix: [
      [1, 2],
      [2, 4],
      [3, 6],
    ],
  },
  {
    id: "ill-conditioned",
    label: "Mal condicionada",
    description:
      "Matriz de Läuchli: sus columnas son casi idénticas. Es el contraejemplo con el que " +
      "Gram-Schmidt clásico pierde la ortogonalidad por completo (‖QᵀQ−I‖ ≈ 0.5), mientras " +
      "que Householder la conserva en el epsilon de la máquina. Míralo en el panel de verificación.",
    matrix: [
      [1, 1, 1],
      [1e-8, 0, 0],
      [0, 1e-8, 0],
      [0, 0, 1e-8],
    ],
  },
];

/** Ejemplo cargado al entrar, para que la primera factorización cueste un clic. */
export const DEFAULT_PRESET_ID = "classic";

/**
 * Límite del grid de la interfaz.
 *
 * La API admite hasta 512×512, pero renderizar esa cuadrícula serían 262.144
 * campos de entrada y el navegador se bloquearía. El techo es de la interfaz,
 * no del backend, y se indica en pantalla para que no se confunda una cosa con
 * la otra.
 */
export const MAX_GRID_DIMENSION = 12;

/** Límite real de la API, mostrado como referencia. */
export const API_MAX_DIMENSION = 512;

/**
 * Genera una matriz con valores aleatorios.
 *
 * Enteros y no decimales a propósito: mantienen las celdas estrechas y la
 * cuadrícula legible. Con decimales de quince dígitos, el grid se volvería
 * ilegible y la demostración perdería su gracia.
 */
export function randomMatrix(rows: number, columns: number, magnitude = 20): number[][] {
  return Array.from({ length: rows }, () =>
    Array.from({ length: columns }, () =>
      Math.floor(Math.random() * (2 * magnitude + 1)) - magnitude,
    ),
  );
}

/**
 * Redimensiona una matriz conservando los valores existentes.
 *
 * Cambiar el número de filas o columnas no debe borrar lo que el usuario ya
 * escribió: las celdas que siguen existiendo mantienen su valor y las nuevas
 * entran a cero.
 */
export function resizeMatrix(matrix: number[][], rows: number, columns: number): number[][] {
  return Array.from({ length: rows }, (_, i) =>
    Array.from({ length: columns }, (_, j) => matrix[i]?.[j] ?? 0),
  );
}
