import { config } from '../../../config/index.js';
import { AppError } from '../../../domain/errors.js';
import { verifyAccessToken } from '../../jwt/jwt.verifier.js';

const BEARER_PREFIX = 'Bearer ';

/**
 * Middleware de autenticacion: exige un JWT valido en `Authorization: Bearer <token>`.
 *
 * Deja los claims verificados en `req.auth` para que las capas siguientes
 * puedan usarlos sin volver a decodificar el token.
 */
export function authMiddleware(req, _res, next) {
  if (!config.jwt.enabled) {
    // Modo de desarrollo local explicitamente habilitado por configuracion.
    // `validateConfig` impide que este camino exista en produccion.
    req.auth = { sub: 'anonymous', tokenType: 'access', bypassed: true };
    return next();
  }

  const header = req.get('authorization');

  if (!header) {
    return next(
      AppError.unauthorized('Falta la cabecera Authorization.', { reason: 'missing_header' }),
    );
  }

  if (!header.startsWith(BEARER_PREFIX)) {
    return next(
      AppError.unauthorized('Formato invalido, se espera "Bearer <token>".', {
        reason: 'malformed_header',
      }),
    );
  }

  try {
    req.auth = verifyAccessToken(header.slice(BEARER_PREFIX.length).trim());
    return next();
  } catch (error) {
    return next(error);
  }
}
