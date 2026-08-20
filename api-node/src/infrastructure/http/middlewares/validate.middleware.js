import { z } from 'zod';

import { AppError } from '../../../domain/errors.js';

/**
 * Fabrica de middlewares de validacion a partir de un esquema Zod.
 *
 * Sustituye `req.body` por el valor ya parseado, de modo que los controladores
 * trabajan siempre con datos tipados y con los valores por defecto aplicados.
 *
 * @param {import('zod').ZodType} schema
 * @returns {import('express').RequestHandler}
 */
export function validateBody(schema) {
  return (req, _res, next) => {
    const result = schema.safeParse(req.body);

    if (!result.success) {
      // `treeifyError` produce un arbol que refleja la forma del payload, lo
      // que resulta mucho mas util que una lista plana cuando el error esta en
      // una fila concreta de una matriz concreta.
      return next(
        AppError.validation('El cuerpo de la peticion no es valido.', z.treeifyError(result.error)),
      );
    }

    req.body = result.data;
    return next();
  };
}
