import { Outlet, useNavigate } from "react-router-dom";
import { useTheme } from "next-themes";
import { LogOut, Moon, Sun } from "lucide-react";

import { Button } from "@/components/ui/button";
import { SessionExpiredDialog } from "@/features/auth/components/SessionExpiredDialog";
import { useAuthStore } from "@/features/auth/store/auth.store";

/**
 * Estructura comun de las pantallas autenticadas.
 *
 * Una cabecera y el contenido. Sin barra lateral ni navegacion: la aplicacion
 * tiene una sola pantalla de trabajo y un menu con un unico destino seria
 * mobiliario inutil.
 */
export function AppLayout() {
  const navigate = useNavigate();
  const { resolvedTheme, setTheme } = useTheme();

  const username = useAuthStore((state) => state.username);
  const logout = useAuthStore((state) => state.logout);

  const handleLogout = async () => {
    await logout();
    navigate("/login", { replace: true });
  };

  return (
    <div className="bg-muted/20 min-h-svh">
      <header className="bg-background/80 sticky top-0 z-10 border-b backdrop-blur">
        <div className="mx-auto flex max-w-7xl items-center gap-4 px-6 py-3">
          <div className="min-w-0">
            <h1 className="truncate text-sm font-semibold">Factorización QR</h1>
            <p className="text-muted-foreground truncate text-xs">
              API en Go + API en Node · reto técnico
            </p>
          </div>

          <div className="ml-auto flex items-center gap-2">
            {username ? (
              <span className="text-muted-foreground hidden text-xs sm:inline">{username}</span>
            ) : null}

            <Button
              variant="ghost"
              size="icon"
              onClick={() => setTheme(resolvedTheme === "dark" ? "light" : "dark")}
              aria-label="Cambiar tema"
            >
              {resolvedTheme === "dark" ? (
                <Sun className="size-4" />
              ) : (
                <Moon className="size-4" />
              )}
            </Button>

            <Button variant="outline" size="sm" onClick={() => void handleLogout()}>
              <LogOut className="size-4" />
              <span className="hidden sm:inline">Salir</span>
            </Button>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-7xl p-6">
        <Outlet />
      </main>

      {/* Vive en el layout para que el aviso pueda aparecer sobre cualquier
          pantalla autenticada, sin que cada pagina tenga que montarlo. */}
      <SessionExpiredDialog />
    </div>
  );
}
