import { createBrowserRouter, Navigate } from "react-router-dom";

import { LoginPage } from "@/features/auth/pages/LoginPage";
import { WorkspacePage } from "@/features/matrix/pages/WorkspacePage";

import { AppLayout } from "./layouts/AppLayout";
import { ProtectedLayout } from "./layouts/ProtectedLayout";

/**
 * Rutas de la aplicacion.
 *
 * Tres en total, frente a las mas de cincuenta del sistema de referencia del
 * que se reutiliza el patron. La estructura es la misma —guarda de sesion,
 * layout, pagina— pero ajustada a lo que esta aplicacion necesita de verdad.
 */
export const router = createBrowserRouter([
  {
    path: "/login",
    element: <LoginPage />,
  },
  {
    path: "/",
    element: <ProtectedLayout />,
    children: [
      {
        element: <AppLayout />,
        children: [{ index: true, element: <WorkspacePage /> }],
      },
    ],
  },
  // Cualquier ruta desconocida vuelve al inicio. Una pagina de error dedicada
  // no aporta nada en una aplicacion con una sola pantalla.
  { path: "*", element: <Navigate to="/" replace /> },
]);
