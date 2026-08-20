/**
 * Error de aplicacion con codigo HTTP asociado.
 *
 * Permite que cualquier capa lance un error semantico (`AppError.badRequest(...)`)
 * y que el middleware de errores lo traduzca a una respuesta HTTP coherente,
 * sin que los controladores tengan que construir respuestas de error a mano.
 *
 * Los errores que NO son instancia de AppError se consideran fallos no
 * previstos y el middleware los reporta como 500 sin filtrar detalles internos
 * al cliente.
 */
export class AppError extends Error {
  /**
   * @param {number} status Codigo HTTP.
   * @param {string} code Codigo estable, legible por maquina (ej. VALIDATION_ERROR).
   * @param {string} message Mensaje legible por humanos.
   * @param {unknown} [details] Informacion adicional segura de exponer.
   */
  constructor(status, code, message, details) {
    super(message);
    this.name = 'AppError';
    this.status = status;
    this.code = code;
    this.details = details;
    Error.captureStackTrace(this, AppError);
  }

  static badRequest(message, details) {
    return new AppError(400, 'BAD_REQUEST', message, details);
  }

  static validation(message, details) {
    return new AppError(422, 'VALIDATION_ERROR', message, details);
  }

  static unauthorized(message, details) {
    return new AppError(401, 'UNAUTHORIZED', message, details);
  }

  static notFound(message, details) {
    return new AppError(404, 'NOT_FOUND', message, details);
  }

  static internal(message, details) {
    return new AppError(500, 'INTERNAL_ERROR', message, details);
  }

  /** Serializacion segura para el cuerpo de la respuesta HTTP. */
  toJSON() {
    return {
      code: this.code,
      message: this.message,
      ...(this.details !== undefined ? { details: this.details } : {}),
    };
  }
}
