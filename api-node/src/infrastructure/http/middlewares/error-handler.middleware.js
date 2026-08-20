import { AppError } from '../../../domain/errors.js';
import { logger } from '../../../shared/logger.js';

/**
 * Middleware terminal de errores.
 *
 * Unico punto donde un error se convierte en respuesta HTTP. Distingue dos
 * casos:
 *  - `AppError`: error previsto por la aplicacion. Su codigo y detalles se
 *    exponen al cliente porque fueron redactados para ser expuestos.
 *  - Cualquier otro error: fallo no previsto. Se registra completo en el log,
 *    pero al cliente solo se le devuelve un 500 generico, para no filtrar
 *    rutas de fichero, versiones de dependencias ni trazas de pila.
 *
 * Express 5 encamina aqui automaticamente los rechazos de handlers async, por
 * lo que no hace falta envolver los controladores en un `asyncHandler`.
 */
// eslint-disable-next-line no-unused-vars -- Express identifica el handler de errores por su aridad de 4.
export function errorHandler(error, req, res, _next) {
  if (error instanceof AppError) {
    if (error.status >= 500) {
      logger.error({ err: error, path: req.path }, error.message);
    } else {
      logger.warn({ code: error.code, path: req.path }, error.message);
    }

    return res.status(error.status).json({ success: false, error: error.toJSON() });
  }

  logger.error({ err: error, path: req.path }, 'Error no controlado');

  return res.status(500).json({
    success: false,
    error: { code: 'INTERNAL_ERROR', message: 'Ha ocurrido un error interno.' },
  });
}

/** Captura cualquier ruta no registrada y la convierte en un 404 con el formato estandar. */
export function notFoundHandler(req, _res, next) {
  next(AppError.notFound(`La ruta ${req.method} ${req.originalUrl} no existe.`));
}
