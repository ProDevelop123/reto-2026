import jwt from 'jsonwebtoken';

import { config, getJwtPublicKey } from '../../config/index.js';
import { AppError } from '../../domain/errors.js';

/**
 * Verificador de tokens de acceso.
 *
 * Esta API solo VERIFICA; nunca emite. La emision es responsabilidad exclusiva
 * de la API en Go, que es la unica que posee la clave privada.
 */

/** Tipo de token que este servicio acepta en la cabecera Authorization. */
const ACCESS_TOKEN_TYPE = 'access';

/**
 * Verifica la firma y los claims de un token de acceso.
 *
 * @param {string} token JWT en formato compacto.
 * @returns {import('jsonwebtoken').JwtPayload} Claims verificados.
 * @throws {AppError} 401 si el token es invalido, esta expirado o no es de acceso.
 */
export function verifyAccessToken(token) {
  let claims;

  try {
    claims = jwt.verify(token, getJwtPublicKey(), {
      algorithms: config.jwt.algorithms,
      issuer: config.jwt.issuer,
      audience: config.jwt.audience,
    });
  } catch (cause) {
    if (cause instanceof jwt.TokenExpiredError) {
      throw AppError.unauthorized('El token ha expirado.', { reason: 'token_expired' });
    }
    throw AppError.unauthorized('Token invalido.', { reason: 'token_invalid' });
  }

  // Un refresh token esta firmado con la misma clave, asi que la verificacion
  // criptografica lo daria por bueno. Comprobar el tipo impide usar un refresh
  // token (de vida larga) como si fuera un token de acceso.
  if (claims.tokenType !== ACCESS_TOKEN_TYPE) {
    throw AppError.unauthorized('Se requiere un token de acceso.', {
      reason: 'wrong_token_type',
    });
  }

  return claims;
}
