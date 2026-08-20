import pino from 'pino';

import { config } from '../config/index.js';

/**
 * Logger estructurado de la aplicacion.
 *
 * En desarrollo se usa `pino-pretty` para salida legible en consola; en
 * produccion se emite JSON por stdout, que es lo que esperan los recolectores
 * de logs de las plataformas de contenedores (Cloud Run, entre otras).
 *
 * Se silencia durante los tests para no ensuciar la salida de Jest.
 */
export const logger = pino({
  level: config.logLevel,
  enabled: config.env !== 'test',
  ...(config.env === 'development'
    ? {
        transport: {
          target: 'pino-pretty',
          options: { colorize: true, translateTime: 'HH:MM:ss', ignore: 'pid,hostname' },
        },
      }
    : {}),
});
