import fs from 'node:fs';
import path from 'node:path';

import { DEFAULT_TOLERANCE } from '../domain/statistics.js';

/** Lee una variable de entorno aplicando un valor por defecto. */
function env(key, fallback) {
  const value = process.env[key];
  return value === undefined || value === '' ? fallback : value;
}

/** Lee una variable de entorno numerica, validando que sea un numero finito. */
function envNumber(key, fallback) {
  const raw = process.env[key];
  if (raw === undefined || raw === '') return fallback;

  const parsed = Number(raw);
  if (!Number.isFinite(parsed)) {
    throw new Error(`La variable de entorno ${key} debe ser numerica, se recibio "${raw}"`);
  }
  return parsed;
}

/** Lee una variable de entorno booleana ("true"/"false"). */
function envBoolean(key, fallback) {
  const raw = process.env[key];
  if (raw === undefined || raw === '') return fallback;
  return raw.toLowerCase() === 'true';
}

/** Lee una lista separada por comas, descartando entradas vacias. */
function envList(key, fallback = []) {
  const raw = process.env[key];
  if (raw === undefined || raw === '') return fallback;
  return raw.split(',').map((item) => item.trim()).filter(Boolean);
}

const nodeEnv = env('NODE_ENV', 'development');

export const config = {
  env: nodeEnv,
  isProduction: nodeEnv === 'production',
  port: envNumber('PORT', 3001),
  logLevel: env('LOG_LEVEL', nodeEnv === 'production' ? 'info' : 'debug'),

  /**
   * Tolerancia por defecto para la deteccion de matriz diagonal. Cada peticion
   * puede sobreescribirla, pero el valor de entorno permite ajustarla a nivel
   * de despliegue sin tocar los clientes.
   */
  diagonalTolerance: envNumber('DIAGONAL_TOLERANCE', DEFAULT_TOLERANCE),

  /**
   * Origenes permitidos por CORS.
   *
   * Esta API es un servicio interno: en la arquitectura del reto solo la
   * consume la API en Go. Por eso la lista viene vacia por defecto y CORS
   * queda deshabilitado, en lugar de abrir un comodin "*".
   */
  corsOrigins: envList('CORS_ORIGINS', []),

  jwt: {
    /**
     * Permite desactivar la verificacion de JWT en desarrollo local para poder
     * probar el endpoint con curl sin emitir un token. Nunca debe activarse en
     * produccion: el arranque falla si se intenta (ver validateConfig).
     */
    enabled: envBoolean('JWT_ENABLED', true),

    /**
     * Solo se admite RS256 (firma asimetrica). Esta API es un verificador puro:
     * conoce unicamente la clave publica y por tanto es incapaz de emitir
     * tokens. Si un atacante comprometiera este servicio, no obtendria material
     * de firma. Fijar el algoritmo de forma explicita ademas cierra el ataque
     * clasico de confusion de algoritmo (cambiar "alg" a HS256 o a "none").
     */
    algorithms: ['RS256'],
    issuer: env('JWT_ISSUER', 'reto-2026-api-go'),
    audience: env('JWT_AUDIENCE', 'reto-2026'),

    /**
     * La clave publica se puede entregar de dos formas:
     *  - JWT_PUBLIC_KEY: el PEM completo o su codificacion base64, pensado para
     *    plataformas serverless donde montar ficheros es incomodo (Cloud Run).
     *  - JWT_PUBLIC_KEY_PATH: ruta a un fichero .pem, que es lo comodo en
     *    docker-compose montando un volumen.
     * La primera tiene prioridad sobre la segunda.
     */
    publicKeyInline: env('JWT_PUBLIC_KEY', ''),
    publicKeyPath: env('JWT_PUBLIC_KEY_PATH', path.resolve(process.cwd(), '../keys/public.pem')),
  },
};

let cachedPublicKey = null;

/**
 * Resuelve la clave publica de verificacion.
 *
 * La resolucion es perezosa y cacheada: asi el modulo de configuracion se puede
 * importar en tests sin exigir que exista un fichero de claves, y en produccion
 * el fichero se lee una unica vez y no en cada peticion.
 *
 * @returns {string} PEM de la clave publica.
 */
export function getJwtPublicKey() {
  if (cachedPublicKey) return cachedPublicKey;

  const { publicKeyInline, publicKeyPath } = config.jwt;

  if (publicKeyInline) {
    // Se acepta el PEM tal cual o codificado en base64, porque muchos gestores
    // de secretos no conservan bien los saltos de linea.
    cachedPublicKey = publicKeyInline.includes('BEGIN')
      ? publicKeyInline
      : Buffer.from(publicKeyInline, 'base64').toString('utf8');
    return cachedPublicKey;
  }

  try {
    cachedPublicKey = fs.readFileSync(publicKeyPath, 'utf8');
  } catch (cause) {
    throw new Error(
      `No se pudo leer la clave publica JWT en "${publicKeyPath}". ` +
        'Defina JWT_PUBLIC_KEY o JWT_PUBLIC_KEY_PATH.',
      { cause },
    );
  }

  return cachedPublicKey;
}

/** Limpia la clave cacheada. Se usa en los tests para reconfigurar el entorno. */
export function resetJwtPublicKeyCache() {
  cachedPublicKey = null;
}

/**
 * Valida invariantes de configuracion antes de aceptar trafico.
 *
 * Se prefiere fallar en el arranque a arrancar en un estado inseguro: un
 * contenedor que no levanta es un problema visible, uno que sirve peticiones
 * sin autenticacion no lo es.
 */
export function validateConfig() {
  if (config.isProduction && !config.jwt.enabled) {
    throw new Error('JWT_ENABLED=false no esta permitido en produccion.');
  }

  if (config.jwt.enabled) {
    getJwtPublicKey();
  }

  if (config.diagonalTolerance < 0) {
    throw new Error('DIAGONAL_TOLERANCE no puede ser negativa.');
  }
}
