import { create } from "zustand";
import { persist } from "zustand/middleware";

import { configureApi } from "@/shared/lib/api";

import * as authService from "../services/auth.service";
import { TokenService } from "../services/token.service";

/**
 * Estado de sesion del cliente.
 *
 * Version reducida del store del sistema de referencia: se han eliminado
 * permisos, tenant, sucursal, empresa y usuarios guardados, porque este
 * proyecto no tiene ninguno de esos conceptos. Queda lo imprescindible.
 */
interface AuthState {
  /**
   * Token de acceso.
   *
   * Se persiste en localStorage a conciencia y con una contrapartida asumida:
   * un XSS podria leerlo. A cambio, recargar la pagina no obliga a volver a
   * autenticarse. El riesgo esta acotado porque su vida es de 15 minutos y
   * porque el refresh token —la credencial de verdad, la de vida larga— vive en
   * una cookie HttpOnly inaccesible desde JavaScript.
   */
  accessToken: string | null;
  username: string | null;

  /** Indica que la sesion expiro y no pudo renovarse. Dispara el aviso. */
  sessionExpired: boolean;

  /** Marca si zustand ya restauro el estado persistido. */
  hydrated: boolean;

  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  refreshSession: () => Promise<string | null>;
  isAuthenticated: () => boolean;
  dismissSessionExpired: () => void;
  setHydrated: () => void;
}

const STORAGE_KEY = "reto-2026-auth";

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      accessToken: null,
      username: null,
      sessionExpired: false,
      hydrated: false,

      login: async (username, password) => {
        const tokens = await authService.login(username, password);

        set({
          accessToken: tokens.accessToken,
          username: tokens.username,
          sessionExpired: false,
        });
      },

      logout: async () => {
        await authService.logout();
        set({ accessToken: null, username: null, sessionExpired: false });
      },

      /**
       * Renueva la sesion usando la cookie de refresco.
       *
       * Devuelve el token nuevo, o null si la sesion ya no puede recuperarse.
       * Lo invoca el interceptor de `api`, que garantiza que solo haya una
       * renovacion en vuelo aunque varias peticiones fallen a la vez.
       */
      refreshSession: async () => {
        try {
          const tokens = await authService.refresh();

          set({
            accessToken: tokens.accessToken,
            username: tokens.username || get().username,
            sessionExpired: false,
          });

          return tokens.accessToken;
        } catch {
          // El refresco falla cuando la cookie expiro, cuando se cerro sesion,
          // o cuando el backend detecto una reutilizacion del token y revoco la
          // familia. En los tres casos hay que volver a autenticarse.
          set({ accessToken: null, sessionExpired: true });
          return null;
        }
      },

      isAuthenticated: () => {
        const token = get().accessToken;
        return token !== null && !TokenService.isExpired(token);
      },

      dismissSessionExpired: () => set({ sessionExpired: false }),

      setHydrated: () => set({ hydrated: true }),
    }),
    {
      name: STORAGE_KEY,
      // `sessionExpired` y `hydrated` son estado de la sesion actual del
      // navegador: persistirlos haria que la aplicacion arrancara mostrando un
      // aviso de caducidad heredado de la visita anterior.
      partialize: (state) => ({
        accessToken: state.accessToken,
        username: state.username,
      }),
      onRehydrateStorage: () => (state) => state?.setHydrated(),
    },
  ),
);

/**
 * Conecta el cliente HTTP con el store.
 *
 * La inyeccion rompe el ciclo de dependencias entre ambos modulos: `api`
 * necesita leer el token y saber renovar; el store necesita `api` para hacer
 * las llamadas. Se invoca una sola vez, al cargar el modulo.
 */
configureApi({
  getAccessToken: () => useAuthStore.getState().accessToken,
  refreshSession: () => useAuthStore.getState().refreshSession(),
  onSessionLost: () => useAuthStore.setState({ accessToken: null, sessionExpired: true }),
});
