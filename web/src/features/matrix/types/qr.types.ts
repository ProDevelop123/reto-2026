/**
 * Tipos del contrato de POST /api/v1/qr.
 *
 * Reflejan exactamente lo que publica la API en Go. Se declaran a mano en lugar
 * de generarlos porque el contrato es pequeno y estable, y tenerlo escrito aqui
 * sirve tambien de documentacion para quien lea el frontend.
 */

/** Matriz con sus dimensiones, tal como la devuelve la API. */
export interface MatrixPayload {
  name: string;
  rows: number;
  columns: number;
  data: number[][];
}

/** Metricas de una matriz concreta. */
export interface MatrixStatistics {
  name: string;
  rows: number;
  columns: number;
  count: number;
  max: number;
  min: number;
  sum: number;
  average: number;
  isSquare: boolean;
  isDiagonal: boolean;
}

/** Metricas agregadas: las cinco que exige el enunciado del reto. */
export interface GlobalStatistics {
  matrices: number;
  count: number;
  max: number;
  min: number;
  sum: number;
  average: number;
  isAnyDiagonal: boolean;
  diagonalMatrices: string[];
}

export interface Statistics {
  global: GlobalStatistics;
  perMatrix: MatrixStatistics[];
}

/** Variante de la factorizacion. */
export type FactorizationMode = "reduced" | "complete";

export interface FactorizeRequest {
  matrix: number[][];
  mode?: FactorizationMode;
  tolerance?: number;
}

export interface FactorizeData {
  matrix: MatrixPayload;
  q: MatrixPayload;
  r: MatrixPayload;
  statistics: Statistics;
}

export interface FactorizeMetadata {
  mode: string;
  reflectors: number;
  tolerance: number;
  durationMs: number;
  computedAt: string;
}

export interface FactorizeResponse {
  success: boolean;
  data: FactorizeData;
  metadata: FactorizeMetadata;
}
