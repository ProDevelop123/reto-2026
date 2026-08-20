import { Navigate, Outlet, useLocation } from "react-router-dom";

import { useAuthStore } from "@/features/auth/store/auth.store";

/**
 * Guarda de rutas privadas.
 *
 * Patron heredado del sistema de referencia, sin la capa de permisos: este
 * proyecto no tiene RBAC, solo sesion iniciada o no.
 */
export function ProtectedLayout() {
  const location = useLocation();
  const hydrated = useAuthStore((state) => state.hydrated);
  const accessToken = useAuthStore((state) => state.accessToken);

  // Hay que esperar a que zustand restaure el estado persistido. Sin esta
  // espera, recargar la pagina redirigiria al login durante un instante aunque
  // la sesion siga siendo valida: el token todavia no se ha leido de
  // localStorage y el guarda lo interpreta como "no autenticado".
  if (!hydrated) {
    return (
      <div className="text-muted-foreground flex min-h-svh items-center justify-center text-sm">
        Cargando…
      </div>
    );
  }

  // Se comprueba la PRESENCIA del token, no su vigencia. Un token caducado no
  // se expulsa aqui: el interceptor lo renovara de forma transparente en la
  // primera peticion. Echar al usuario por un token vencido cuando la cookie de
  // refresco sigue viva seria cerrarle la sesion sin necesidad.
  if (!accessToken) {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />;
  }

  return <Outlet />;
}
