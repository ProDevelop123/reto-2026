import { beforeAll, describe, expect, it } from '@jest/globals';

let analyzeMatrices;
let config;

beforeAll(async () => {
  // La configuracion lee el entorno al importarse, asi que se fija antes.
  process.env.DIAGONAL_TOLERANCE = '1e-9';

  ({ analyzeMatrices } = await import('../../src/application/analyze-matrices.usecase.js'));
  ({ config } = await import('../../src/config/index.js'));
});

describe('analyzeMatrices', () => {
  const qr = [
    { name: 'Q', data: [[1, 0], [0, 1]] },
    { name: 'R', data: [[3, 4], [0, 5]] },
  ];

  it('devuelve estadisticas globales y por matriz', () => {
    const { statistics } = analyzeMatrices({ matrices: qr });

    expect(statistics.global.max).toBe(5);
    expect(statistics.perMatrix).toHaveLength(2);
  });

  it('usa la tolerancia configurada cuando la peticion no indica una', () => {
    const { metadata } = analyzeMatrices({ matrices: qr });

    expect(metadata.tolerance).toBe(config.diagonalTolerance);
  });

  it('da prioridad a la tolerancia enviada en la peticion', () => {
    const { metadata } = analyzeMatrices({ matrices: qr, tolerance: 0.5 });

    expect(metadata.tolerance).toBe(0.5);
  });

  it('respeta una tolerancia de cero en lugar de sustituirla por el valor por defecto', () => {
    // Comprobacion del uso del operador de fusion nula frente al OR logico:
    // cero es un valor legitimo que significa "comparacion exacta", no "sin valor".
    const { metadata } = analyzeMatrices({ matrices: qr, tolerance: 0 });

    expect(metadata.tolerance).toBe(0);
  });

  it('informa cuantas matrices y cuantos valores se analizaron', () => {
    const { metadata } = analyzeMatrices({ matrices: qr });

    expect(metadata).toMatchObject({ analyzedMatrices: 2, analyzedValues: 8 });
    expect(new Date(metadata.computedAt).toString()).not.toBe('Invalid Date');
  });
});
