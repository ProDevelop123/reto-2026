import cors from 'cors';
import express from 'express';
import helmet from 'helmet';
import pinoHttp from 'pino-http';

import { config } from './config/index.js';
import { errorHandler, notFoundHandler } from './infrastructure/http/middlewares/error-handler.middleware.js';
import { apiRouter } from './infrastructure/http/routes/index.js';
import { logger } from './shared/logger.js';

/**
 * Construye la aplicacion Express.
 *
 * Se separa de `server.js` a proposito: esta funcion devuelve la app sin
 * ponerla a escuchar, lo que permite que los tests de integracion la monten en
 * memoria con supertest, sin abrir puertos ni gestionar ciclo de vida.
 *
 * @returns {import('express').Express}
 */
export function createApp() {
  const app = express();

  // La API se despliega detras del balanceador de la plataforma cloud. Sin
  // esto, la IP del cliente que se registra en los logs seria la del proxy.
  app.set('trust proxy', true);

  // Cabeceras de seguridad por defecto. Se desactiva CSP porque esta API solo
  // sirve JSON: no hay documento HTML al que aplicarla.
  app.use(helmet({ contentSecurityPolicy: false }));

  // CORS solo si hay origenes declarados. Esta API es un servicio interno que
  // en la arquitectura del reto solo consume la API en Go, servidor a servidor,
  // donde el navegador no interviene y CORS es irrelevante.
  if (config.corsOrigins.length > 0) {
    app.use(
      cors({
        origin: config.corsOrigins,
        methods: ['GET', 'POST', 'OPTIONS'],
        allowedHeaders: ['Content-Type', 'Authorization'],
        credentials: true,
        maxAge: 86_400,
      }),
    );
  }

  // Se censuran las cabeceras que transportan credenciales antes de que lleguen
  // al log. Sin esto, `pino-http` volcaria el JWT completo en cada peticion y
  // cualquiera con acceso a los logs dispondria de tokens validos.
  app.use(
    pinoHttp({
      logger,
      autoLogging: config.env !== 'test',
      redact: {
        paths: [
          'req.headers.authorization',
          'req.headers.cookie',
          'res.headers["set-cookie"]',
        ],
        censor: '[REDACTED]',
      },
    }),
  );

  // Limite de tamano del cuerpo: una matriz grande es legitima, pero un payload
  // ilimitado es un vector de agotamiento de memoria.
  app.use(express.json({ limit: '5mb' }));

  app.use(apiRouter);

  // El 404 y el manejador de errores van al final: cualquier ruta que no haya
  // sido atendida hasta aqui no existe.
  app.use(notFoundHandler);
  app.use(errorHandler);

  return app;
}
