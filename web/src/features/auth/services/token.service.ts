import { jwtDecode } from "jwt-decode";

/**
 * Claims que emite la API en Go.
 *
 * Es un subconjunto deliberado: solo se declara lo que el cliente usa. La
 * verificacion de la FIRMA ocurre en el servidor, que es el unico que puede
 * hacerla; aqui el token se lee unicamente para saber cuando caduca y a quien
 * pertenece. Ninguna decision de seguridad se toma con estos datos.
 */
export interface TokenPayload {
  sub: string;
  tokenType: string;
  exp: number;
  iat: number;
  iss: string;
  aud: string | string[];
}

/**
 * Margen con el que se considera que un token esta a punto de caducar.
 *
 * Se declara fuera del objeto porque `isAboutToExpire` lo usa como valor por
 * defecto de un parametro: referenciar el propio objeto dentro de su
 * inicializador impediria a TypeScript inferir su tipo.
 */
export const EXPIRATION_BUFFER_MS = 60_000;

/**
 * Utilidades de lectura del token de acceso.
 *
 * Adaptado del servicio equivalente del sistema de diseno de referencia,
 * eliminando la extraccion de permisos: este proyecto no tiene RBAC.
 */
export const TokenService = {
  decode(token: string): TokenPayload | null {
    try {
      return jwtDecode<TokenPayload>(token);
    } catch {
      // Un token ilegible se trata como inexistente. Propagar la excepcion
      // obligaria a cada punto de uso a envolverla en un try.
      return null;
    }
  },

  /** Instante de expiracion en milisegundos, o null si no se puede determinar. */
  getExpiration(token: string): number | null {
    const decoded = this.decode(token);
    return decoded?.exp ? decoded.exp * 1000 : null;
  },

  isExpired(token: string): boolean {
    const expiration = this.getExpiration(token);
    // Sin expiracion legible se asume caducado: es la suposicion segura.
    return expiration === null || Date.now() > expiration;
  },

  /**
   * Indica si el token caduca dentro del margen.
   *
   * Permite renovar de forma preventiva en lugar de esperar al primer 401, lo
   * que evita que el usuario perciba una peticion fallida.
   */
  isAboutToExpire(token: string, bufferMs = EXPIRATION_BUFFER_MS): boolean {
    const expiration = this.getExpiration(token);
    return expiration === null || Date.now() > expiration - bufferMs;
  },

  /** Segundos que restan hasta la expiracion, nunca negativos. */
  secondsUntilExpiration(token: string): number {
    const expiration = this.getExpiration(token);
    if (expiration === null) return 0;
    return Math.max(0, Math.floor((expiration - Date.now()) / 1000));
  },

  getUsername(token: string): string {
    return this.decode(token)?.sub ?? "";
  },
};
