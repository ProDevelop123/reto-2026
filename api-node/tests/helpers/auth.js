import { generateKeyPairSync } from 'node:crypto';

import jwt from 'jsonwebtoken';

/**
 * Utilidades de autenticacion para los tests.
 *
 * Se genera un par de claves RSA efimero en memoria en lugar de leer las claves
 * reales del repositorio: los tests quedan asi autocontenidos, reproducibles y
 * sin depender de ningun fichero ni secreto del entorno.
 */

const { privateKey, publicKey } = generateKeyPairSync('rsa', {
  modulusLength: 2048,
  publicKeyEncoding: { type: 'spki', format: 'pem' },
  privateKeyEncoding: { type: 'pkcs8', format: 'pem' },
});

export const TEST_ISSUER = 'reto-2026-api-go';
export const TEST_AUDIENCE = 'reto-2026';

export const testPublicKey = publicKey;

/**
 * Firma un token de prueba.
 *
 * @param {object} [overrides] Claims a sobreescribir (p. ej. tokenType o exp).
 * @param {object} [options] Opciones de firma a sobreescribir.
 */
export function signTestToken(overrides = {}, options = {}) {
  return jwt.sign(
    { sub: 'tester', tokenType: 'access', ...overrides },
    privateKey,
    {
      algorithm: 'RS256',
      issuer: TEST_ISSUER,
      audience: TEST_AUDIENCE,
      expiresIn: '5m',
      ...options,
    },
  );
}

/**
 * Firma un token con OTRA clave privada distinta de la del servicio.
 * Sirve para comprobar que una firma valida pero de emisor desconocido se
 * rechaza: verificar el formato del token no basta, hay que verificar la firma.
 */
export function signTokenWithForeignKey() {
  const foreign = generateKeyPairSync('rsa', {
    modulusLength: 2048,
    privateKeyEncoding: { type: 'pkcs8', format: 'pem' },
    publicKeyEncoding: { type: 'spki', format: 'pem' },
  });

  return jwt.sign({ sub: 'attacker', tokenType: 'access' }, foreign.privateKey, {
    algorithm: 'RS256',
    issuer: TEST_ISSUER,
    audience: TEST_AUDIENCE,
    expiresIn: '5m',
  });
}
