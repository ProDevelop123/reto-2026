import { describe, expect, it } from '@jest/globals';

import {
  DEFAULT_TOLERANCE,
  computeAggregatedStatistics,
  computeMatrixStatistics,
  isDiagonal,
} from '../../src/domain/statistics.js';

describe('computeMatrixStatistics', () => {
  it('calcula maximo, minimo, suma y promedio de una matriz rectangular', () => {
    const stats = computeMatrixStatistics([
      [1, 2, 3],
      [4, 5, 6],
    ]);

    expect(stats).toMatchObject({
      rows: 2,
      columns: 3,
      count: 6,
      max: 6,
      min: 1,
      sum: 21,
      average: 3.5,
      isSquare: false,
    });
  });

  it('maneja valores negativos y decimales', () => {
    const stats = computeMatrixStatistics([
      [-2.5, 0],
      [1.25, -10],
    ]);

    expect(stats.max).toBe(1.25);
    expect(stats.min).toBe(-10);
    expect(stats.sum).toBeCloseTo(-11.25, 10);
    expect(stats.average).toBeCloseTo(-2.8125, 10);
  });

  it('trata correctamente una matriz de un solo elemento', () => {
    const stats = computeMatrixStatistics([[7]]);

    expect(stats).toMatchObject({ max: 7, min: 7, sum: 7, average: 7, count: 1, isSquare: true });
  });

  it('suma sin perder precision cuando hay magnitudes muy dispares', () => {
    // Una suma ingenua con `+=` da 0 aqui: al sumar 1 a 1e16 el 1 se pierde por
    // redondeo y al restar 1e16 no queda nada. La suma compensada lo conserva.
    const stats = computeMatrixStatistics([
      [1e16, 1],
      [-1e16, 0],
    ]);

    expect(stats.sum).toBe(1);
    expect(stats.average).toBe(0.25);
  });
});

describe('isDiagonal', () => {
  it('reconoce una matriz diagonal exacta', () => {
    expect(isDiagonal([
      [3, 0],
      [0, 5],
    ])).toBe(true);
  });

  it('rechaza una matriz con elementos fuera de la diagonal', () => {
    expect(isDiagonal([
      [3, 1],
      [0, 5],
    ])).toBe(false);
  });

  it('considera diagonal la matriz nula, segun la definicion estandar', () => {
    expect(isDiagonal([
      [0, 0],
      [0, 0],
    ])).toBe(true);
  });

  it('admite ceros en la diagonal principal', () => {
    expect(isDiagonal([
      [0, 0],
      [0, 4],
    ])).toBe(true);
  });

  it('admite matrices rectangulares diagonales', () => {
    expect(isDiagonal([
      [2, 0],
      [0, 3],
      [0, 0],
    ])).toBe(true);
  });

  it('trata como cero los residuos numericos dentro de la tolerancia', () => {
    // Caso realista: el triangulo inferior de una R producida por QR no es cero
    // exacto sino del orden del epsilon de la maquina.
    const matrix = [
      [4, 0],
      [1e-16, 9],
    ];

    expect(isDiagonal(matrix, DEFAULT_TOLERANCE)).toBe(true);
    expect(isDiagonal(matrix, 0)).toBe(false);
  });

  it('respeta una tolerancia personalizada mas estricta', () => {
    const matrix = [
      [4, 1e-6],
      [0, 9],
    ];

    expect(isDiagonal(matrix, 1e-3)).toBe(true);
    expect(isDiagonal(matrix, 1e-9)).toBe(false);
  });
});

describe('computeAggregatedStatistics', () => {
  const identity = { name: 'Q', data: [[1, 0], [0, 1]] };
  const upper = { name: 'R', data: [[3, 4], [0, 5]] };

  it('agrega las metricas globales sobre todas las matrices', () => {
    const { global } = computeAggregatedStatistics([identity, upper]);

    expect(global).toMatchObject({
      matrices: 2,
      count: 8,
      max: 5,
      min: 0,
      sum: 14,
      average: 1.75,
    });
  });

  it('indica que alguna matriz es diagonal y cual', () => {
    const { global } = computeAggregatedStatistics([identity, upper]);

    expect(global.isAnyDiagonal).toBe(true);
    expect(global.diagonalMatrices).toEqual(['Q']);
  });

  it('reporta que ninguna es diagonal cuando no lo son', () => {
    const { global } = computeAggregatedStatistics([upper, { name: 'S', data: [[1, 2], [3, 4]] }]);

    expect(global.isAnyDiagonal).toBe(false);
    expect(global.diagonalMatrices).toEqual([]);
  });

  it('devuelve tambien las metricas de cada matriz por separado', () => {
    const { perMatrix } = computeAggregatedStatistics([identity, upper]);

    expect(perMatrix).toHaveLength(2);
    expect(perMatrix[0]).toMatchObject({ name: 'Q', sum: 2, isDiagonal: true });
    expect(perMatrix[1]).toMatchObject({ name: 'R', sum: 12, isDiagonal: false });
  });

  it('asigna un nombre por posicion a las matrices anonimas', () => {
    const { perMatrix } = computeAggregatedStatistics([{ data: [[1]] }, { data: [[2]] }]);

    expect(perMatrix.map((m) => m.name)).toEqual(['matrix_0', 'matrix_1']);
  });

  it('propaga la tolerancia a cada matriz', () => {
    const almostDiagonal = { name: 'A', data: [[1, 1e-6], [0, 2]] };

    expect(computeAggregatedStatistics([almostDiagonal], { tolerance: 1e-3 }).global.isAnyDiagonal)
      .toBe(true);
    expect(computeAggregatedStatistics([almostDiagonal], { tolerance: 1e-9 }).global.isAnyDiagonal)
      .toBe(false);
  });

  it('funciona con una sola matriz', () => {
    const { global, perMatrix } = computeAggregatedStatistics([upper]);

    expect(perMatrix).toHaveLength(1);
    expect(global).toMatchObject({ matrices: 1, max: 5, min: 0, sum: 12, average: 3 });
  });
});
