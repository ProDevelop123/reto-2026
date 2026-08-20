import { beforeAll, describe, expect, it } from '@jest/globals';
import request from 'supertest';

import {
  TEST_AUDIENCE,
  TEST_ISSUER,
  signTestToken,
  signTokenWithForeignKey,
  testPublicKey,
} from '../helpers/auth.js';

/**
 * Tests de integracion de la API.
 *
 * Montan la aplicacion Express completa en memoria (sin abrir puerto) y la
 * ejercitan con peticiones HTTP reales a traves de supertest, de modo que se
 * verifica la cadena entera: enrutado, autenticacion, validacion, caso de uso,
 * dominio y formato de respuesta.
 */

let app;
let token;

beforeAll(async () => {
  // La clave publica de prueba se inyecta por entorno, igual que se hara en
  // produccion sobre Cloud Run. Debe fijarse antes de importar la configuracion.
  process.env.NODE_ENV = 'test';
  process.env.JWT_ENABLED = 'true';
  process.env.JWT_PUBLIC_KEY = testPublicKey;
  process.env.JWT_ISSUER = TEST_ISSUER;
  process.env.JWT_AUDIENCE = TEST_AUDIENCE;

  const { createApp } = await import('../../src/app.js');
  app = createApp();
  token = signTestToken();
});

const validBody = {
  matrices: [
    { name: 'Q', data: [[1, 0], [0, 1]] },
    { name: 'R', data: [[3, 4], [0, 5]] },
  ],
};

describe('GET /health', () => {
  it('responde sin necesidad de autenticacion', async () => {
    const res = await request(app).get('/health');

    expect(res.status).toBe(200);
    expect(res.body.data.status).toBe('ok');
  });
});

describe('POST /api/v1/statistics - autenticacion', () => {
  it('rechaza la peticion sin cabecera Authorization', async () => {
    const res = await request(app).post('/api/v1/statistics').send(validBody);

    expect(res.status).toBe(401);
    expect(res.body.error.details.reason).toBe('missing_header');
  });

  it('rechaza una cabecera que no sigue el formato Bearer', async () => {
    const res = await request(app)
      .post('/api/v1/statistics')
      .set('Authorization', token)
      .send(validBody);

    expect(res.status).toBe(401);
    expect(res.body.error.details.reason).toBe('malformed_header');
  });

  it('rechaza un token firmado con una clave desconocida', async () => {
    const res = await request(app)
      .post('/api/v1/statistics')
      .set('Authorization', 'Bearer ' + signTokenWithForeignKey())
      .send(validBody);

    expect(res.status).toBe(401);
    expect(res.body.error.details.reason).toBe('token_invalid');
  });

  it('rechaza un token expirado', async () => {
    const expired = signTestToken({}, { expiresIn: '-1s' });

    const res = await request(app)
      .post('/api/v1/statistics')
      .set('Authorization', 'Bearer ' + expired)
      .send(validBody);

    expect(res.status).toBe(401);
    expect(res.body.error.details.reason).toBe('token_expired');
  });

  it('rechaza un refresh token usado como token de acceso', async () => {
    const refresh = signTestToken({ tokenType: 'refresh' });

    const res = await request(app)
      .post('/api/v1/statistics')
      .set('Authorization', 'Bearer ' + refresh)
      .send(validBody);

    expect(res.status).toBe(401);
    expect(res.body.error.details.reason).toBe('wrong_token_type');
  });

  it('rechaza un token con un emisor distinto', async () => {
    const otherIssuer = signTestToken({}, { issuer: 'otro-emisor' });

    const res = await request(app)
      .post('/api/v1/statistics')
      .set('Authorization', 'Bearer ' + otherIssuer)
      .send(validBody);

    expect(res.status).toBe(401);
  });
});

describe('POST /api/v1/statistics - validacion', () => {
  const post = (body) =>
    request(app).post('/api/v1/statistics').set('Authorization', 'Bearer ' + token).send(body);

  it('rechaza un cuerpo sin matrices', async () => {
    const res = await post({});

    expect(res.status).toBe(422);
    expect(res.body.error.code).toBe('VALIDATION_ERROR');
  });

  it('rechaza una lista de matrices vacia', async () => {
    const res = await post({ matrices: [] });

    expect(res.status).toBe(422);
  });

  it('rechaza una matriz no rectangular indicando la fila culpable', async () => {
    const res = await post({ matrices: [{ name: 'A', data: [[1, 2], [3]] }] });

    expect(res.status).toBe(422);
    expect(JSON.stringify(res.body.error.details)).toContain('no es rectangular');
  });

  it('rechaza valores no numericos', async () => {
    const res = await post({ matrices: [{ data: [[1, 'dos']] }] });

    expect(res.status).toBe(422);
  });

  it('rechaza valores no finitos, que JSON serializa como null', async () => {
    const res = await post({ matrices: [{ data: [[1, null]] }] });

    expect(res.status).toBe(422);
  });

  it('rechaza una tolerancia negativa', async () => {
    const res = await post({ ...validBody, tolerance: -1 });

    expect(res.status).toBe(422);
  });
});

describe('POST /api/v1/statistics - calculo', () => {
  const post = (body) =>
    request(app).post('/api/v1/statistics').set('Authorization', 'Bearer ' + token).send(body);

  it('devuelve las cinco metricas exigidas por el reto', async () => {
    const res = await post(validBody);

    expect(res.status).toBe(200);
    expect(res.body.success).toBe(true);
    expect(res.body.data.global).toMatchObject({
      max: 5,
      min: 0,
      sum: 14,
      average: 1.75,
      isAnyDiagonal: true,
    });
    expect(res.body.data.global.diagonalMatrices).toEqual(['Q']);
  });

  it('incluye el desglose por matriz conservando los nombres recibidos', async () => {
    const res = await post(validBody);

    expect(res.body.data.perMatrix.map((m) => m.name)).toEqual(['Q', 'R']);
    expect(res.body.data.perMatrix[1].isDiagonal).toBe(false);
  });

  it('devuelve la tolerancia efectivamente aplicada en los metadatos', async () => {
    const res = await post({ ...validBody, tolerance: 0.01 });

    expect(res.body.metadata.tolerance).toBe(0.01);
  });

  it('procesa una matriz rectangular con valores decimales y negativos', async () => {
    const res = await post({
      matrices: [{ name: 'A', data: [[-1.5, 2.25, 0], [4, -8, 0.5]] }],
    });

    expect(res.status).toBe(200);
    expect(res.body.data.global.max).toBe(4);
    expect(res.body.data.global.min).toBe(-8);
    expect(res.body.data.global.sum).toBeCloseTo(-2.75, 10);
  });
});

describe('rutas inexistentes', () => {
  it('devuelve un 404 con el formato de error estandar', async () => {
    const res = await request(app).get('/no-existe');

    expect(res.status).toBe(404);
    expect(res.body).toMatchObject({ success: false, error: { code: 'NOT_FOUND' } });
  });
});
