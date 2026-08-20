import axios, {
  AxiosError,
  type AxiosInstance,
  type InternalAxiosRequestConfig,
} from "axios";

/**
 * Cliente HTTP de la aplicacion.
 *
 * Toda peticion al backend pasa por aqui. Concentra dos preocupaciones
 * transversales que, dispersas por los componentes, se olvidarian tarde o
 * temprano en alguna llamada nueva: adjuntar el token de acceso y renovarlo
 * cuando caduca.
 */

/** Marca una peticion que ya fue reintentada, para no entrar en bucle. */
type RetriableRequest = InternalAxiosRequestConfig & { _retried?: boolean };

export const api: AxiosInstance = axios.create({
  baseURL: import.meta.env.VITE_API_URL ?? "http://localhost:8080",

  // Imprescindible para que el navegador envie la cookie HttpOnly del refresh
  // token. Sin esto el login funcionaria y la renovacion fallaria siempre, con
  // un sintoma —"sesion caducada a los 15 minutos"— que no apunta a su causa.
  withCredentials: true,

  headers: {
    "Content-Type": "application/json",

    // Cabecera de verificacion de origen que exige la API en las rutas cuya
    // credencial es la cookie (refresh y logout).
    //
    // Un formulario HTML o una etiqueta img de un sitio ajeno no pueden anadir
    // cabeceras propias, asi que no pueden falsificar estas peticiones. Anadir
    // una convierte la peticion en "no simple" para CORS, lo que obliga al
    // navegador a hacer antes una consulta preliminar que la politica de
    // origenes rechazara si la web no esta autorizada.
    //
    // Es necesaria porque en produccion la cookie usa SameSite=None: el
    // frontend y la API viven en dominios distintos y sin ese valor la cookie
    // no viajaria, pero a cambio se pierde la proteccion que SameSite=Lax
    // ofrece de forma gratuita en local.
    "X-Refresh-Request": "1",
  },
});

/**
 * Devuelve el token de acceso vigente.
 *
 * Se resuelve mediante una funcion inyectada en lugar de importar el store
 * directamente para romper el ciclo de dependencias: el store necesita a `api`
 * para poder refrescar, y `api` necesita al store para leer el token.
 */
let getAccessToken: () => string | null = () => null;

/** Renueva la sesion. La implementa el store de autenticacion. */
let refreshSession: () => Promise<string | null> = async () => null;

/** Se invoca cuando la sesion ya no puede recuperarse. */
let onSessionLost: () => void = () => {};

/** Conecta el cliente con el store de autenticacion. Se llama una sola vez. */
export function configureApi(handlers: {
  getAccessToken: () => string | null;
  refreshSession: () => Promise<string | null>;
  onSessionLost: () => void;
}) {
  getAccessToken = handlers.getAccessToken;
  refreshSession = handlers.refreshSession;
  onSessionLost = handlers.onSessionLost;
}

// --- Interceptor de peticion ------------------------------------------------

api.interceptors.request.use((config) => {
  const token = getAccessToken();

  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }

  return config;
});

// --- Interceptor de respuesta -----------------------------------------------

/**
 * Renovacion en curso, compartida por todas las peticiones que fallen a la vez.
 *
 * Sin esta guarda, tres peticiones simultaneas que reciban 401 lanzarian tres
 * renovaciones. Y como el backend ROTA el refresh token en cada canje, la
 * segunda llegaria con un token ya consumido: el servidor lo interpretaria como
 * una reutilizacion —la firma de un token robado— y revocaria la familia
 * entera, cerrando la sesion del usuario legitimo.
 *
 * Compartir una unica promesa convierte N renovaciones en una sola.
 */
let refreshInFlight: Promise<string | null> | null = null;

function refreshOnce(): Promise<string | null> {
  refreshInFlight ??= refreshSession().finally(() => {
    refreshInFlight = null;
  });

  return refreshInFlight;
}

/** Rutas cuyo 401 NO debe disparar una renovacion. */
const AUTH_PATHS = ["/api/v1/auth/login", "/api/v1/auth/refresh", "/api/v1/auth/logout"];

function isAuthEndpoint(url: string | undefined): boolean {
  return !!url && AUTH_PATHS.some((path) => url.includes(path));
}

api.interceptors.response.use(
  (response) => response,

  async (error: AxiosError) => {
    const request = error.config as RetriableRequest | undefined;

    // Solo se intenta renovar ante un 401, una unica vez por peticion y nunca
    // sobre las rutas de autenticacion: un 401 del propio login significa
    // "credenciales incorrectas", y renovar ahi no tiene sentido.
    const shouldRefresh =
      error.response?.status === 401 &&
      request !== undefined &&
      !request._retried &&
      !isAuthEndpoint(request.url);

    if (!shouldRefresh) {
      return Promise.reject(error);
    }

    request._retried = true;

    const token = await refreshOnce();

    if (!token) {
      onSessionLost();
      return Promise.reject(error);
    }

    // Se reintenta la peticion original con el token nuevo. Para quien llamo,
    // la renovacion es invisible: recibe la respuesta que esperaba.
    request.headers.Authorization = `Bearer ${token}`;
    return api(request);
  },
);

// --- Traduccion de errores --------------------------------------------------

/**
 * Forma del error que devuelven ambas APIs del sistema.
 *
 * El sobre es identico en Go y en Node, asi que un unico traductor sirve para
 * todo el backend.
 */
export interface ApiErrorBody {
  code: string;
  message: string;
  details?: unknown;
}

export interface ApiError {
  status: number;
  code: string;
  message: string;
  details?: unknown;
}

/**
 * Convierte cualquier fallo en una forma uniforme y presentable.
 *
 * Se prefiere el mensaje que envia el backend: esta redactado para el usuario y
 * es coherente entre servicios. Los mensajes de axios ("Request failed with
 * status code 422") no dicen nada util a quien esta usando la aplicacion.
 */
export function toApiError(error: unknown): ApiError {
  if (axios.isAxiosError(error)) {
    const body = error.response?.data as { error?: ApiErrorBody } | undefined;

    if (body?.error) {
      return {
        status: error.response?.status ?? 0,
        code: body.error.code,
        message: body.error.message,
        details: body.error.details,
      };
    }

    // Sin respuesta hay dos causas posibles y conviene distinguirlas: el
    // servidor no esta levantado, o la peticion fue bloqueada por CORS.
    if (!error.response) {
      return {
        status: 0,
        code: "NETWORK_ERROR",
        message:
          "No se pudo contactar con la API. Comprueba que el backend este " +
          "levantado y que este origen figure en su lista de CORS.",
      };
    }

    return {
      status: error.response.status,
      code: "HTTP_ERROR",
      message: error.message,
    };
  }

  return {
    status: 0,
    code: "UNKNOWN_ERROR",
    message: error instanceof Error ? error.message : "Ha ocurrido un error inesperado.",
  };
}
