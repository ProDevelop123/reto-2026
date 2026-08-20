import { config, validateConfig } from './config/index.js';
import { createApp } from './app.js';
import { logger } from './shared/logger.js';

/**
 * Punto de entrada del proceso.
 *
 * Responsabilidades: validar la configuracion antes de aceptar trafico, poner
 * la aplicacion a escuchar y cerrar de forma ordenada.
 */

try {
  validateConfig();
} catch (error) {
  logger.error({ err: error }, 'Configuracion invalida, el servicio no puede arrancar');
  process.exit(1);
}

const server = createApp().listen(config.port, () => {
  logger.info(
    { port: config.port, env: config.env, jwt: config.jwt.enabled },
    'API de estadisticas escuchando',
  );
});

/**
 * Cierre ordenado.
 *
 * Al recibir la senal de parada se dejan de aceptar conexiones nuevas y se
 * espera a que terminen las peticiones en curso. Sin esto, un redespliegue
 * cortaria peticiones a medio responder. El temporizador de gracia evita que
 * una conexion colgada bloquee el apagado indefinidamente.
 */
function shutdown(signal) {
  logger.info({ signal }, 'Senal de parada recibida, cerrando servidor');

  const forceExit = setTimeout(() => {
    logger.error('Cierre ordenado agotado, forzando salida');
    process.exit(1);
  }, 10_000);
  forceExit.unref();

  server.close((error) => {
    if (error) {
      logger.error({ err: error }, 'Error durante el cierre');
      process.exit(1);
    }
    logger.info('Servidor cerrado correctamente');
    process.exit(0);
  });
}

process.on('SIGTERM', () => shutdown('SIGTERM'));
process.on('SIGINT', () => shutdown('SIGINT'));
