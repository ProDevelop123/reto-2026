import { useNavigate } from "react-router-dom";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

import { useAuthStore } from "../store/auth.store";

/**
 * Aviso de sesion caducada.
 *
 * Se muestra cuando la renovacion automatica falla y no queda forma de
 * recuperar la sesion. Redirigir en silencio al login dejaria a quien lo usa
 * preguntandose que hizo mal y si perdio su trabajo; explicarlo cuesta un
 * dialogo.
 *
 * El refresco falla por tres motivos, y en los tres la salida es la misma:
 * la cookie caduco, se cerro sesion en otra pestana, o el backend detecto una
 * reutilizacion del token de refresco y revoco la familia entera por seguridad.
 */
export function SessionExpiredDialog() {
  const navigate = useNavigate();
  const sessionExpired = useAuthStore((state) => state.sessionExpired);
  const dismiss = useAuthStore((state) => state.dismissSessionExpired);

  const handleConfirm = () => {
    dismiss();
    navigate("/login", { replace: true });
  };

  return (
    <Dialog open={sessionExpired} onOpenChange={(open) => !open && handleConfirm()}>
      <DialogContent showCloseButton={false}>
        <DialogHeader>
          <DialogTitle>La sesión ha caducado</DialogTitle>
          <DialogDescription>
            No se pudo renovar la sesión automáticamente. Vuelve a iniciar sesión para
            continuar.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button onClick={handleConfirm}>Ir al inicio de sesión</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
