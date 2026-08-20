import { config } from '../config/index.js';
import { computeAggregatedStatistics } from '../domain/statistics.js';

/**
 * Caso de uso: analizar un conjunto de matrices y devolver sus estadisticas.
 *
 * Su unica responsabilidad es orquestar: resolver la tolerancia efectiva,
 * delegar el calculo en el dominio y enriquecer el resultado con los metadatos
 * que el consumidor necesita para interpretarlo. No contiene aritmetica ni
 * conoce Express.
 *
 * Que la tolerancia se resuelva aqui y no en el dominio es deliberado: el
 * dominio no debe leer configuracion de entorno, o dejaria de ser puro y
 * testeable de forma aislada.
 *
 * @param {{ matrices: Array<{name?: string, data: number[][]}>, tolerance?: number }} input
 * @returns {{ statistics: object, metadata: object }}
 */
export function analyzeMatrices({ matrices, tolerance }) {
  const effectiveTolerance = tolerance ?? config.diagonalTolerance;

  const { global, perMatrix } = computeAggregatedStatistics(matrices, {
    tolerance: effectiveTolerance,
  });

  return {
    statistics: { global, perMatrix },
    metadata: {
      // Se devuelve la tolerancia realmente aplicada, no la solicitada: el
      // resultado de `isDiagonal` no es interpretable sin conocer el umbral.
      tolerance: effectiveTolerance,
      analyzedMatrices: perMatrix.length,
      analyzedValues: global.count,
      computedAt: new Date().toISOString(),
    },
  };
}
