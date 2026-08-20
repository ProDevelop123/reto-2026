import { api } from "@/shared/lib/api";

/**
 * Llamadas al servicio de autenticacion de la API en Go.
 *
 * El refresh token NUNCA aparece aqui: viaja en una cookie HttpOnly que el
 * navegador adjunta y recibe por su cuenta. El codigo de la aplicacion no puede
 * leerlo, que es precisamente lo que impide que un XSS lo robe.
 */

/** Respuesta de login y de refresco. */
export interface TokenResponse {
  accessToken: string;
  tokenType: string;
  expiresIn: number;
  expiresAt: string;
  username: string;
}

interface SuccessEnvelope<T> {
  success: boolean;
  data: T;
}

export async function login(username: string, password: string): Promise<TokenResponse> {
  const { data } = await api.post<SuccessEnvelope<TokenResponse>>("/api/v1/auth/login", {
    username,
    password,
  });

  return data.data;
}

export async function refresh(): Promise<TokenResponse> {
  // Sin cuerpo: la credencial es la cookie, no un dato de la peticion.
  const { data } = await api.post<SuccessEnvelope<TokenResponse>>("/api/v1/auth/refresh", {});

  return data.data;
}

export async function logout(): Promise<void> {
  // Se ignora el resultado a proposito. El backend responde 200 aunque no haya
  // cookie o el token sea ilegible, y aunque la llamada fallara por red, el
  // cliente debe limpiar su estado igualmente: dejar al usuario "dentro"
  // porque el servidor no contesto seria el peor comportamiento posible.
  await api.post("/api/v1/auth/logout", {}).catch(() => undefined);
}
