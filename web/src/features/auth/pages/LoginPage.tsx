import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { AlertCircle, LogIn } from "lucide-react";
import { z } from "zod";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toApiError } from "@/shared/lib/api";

import { useAuthStore } from "../store/auth.store";

/**
 * Validación del formulario.
 *
 * Se limita a comprobar que los campos vienen rellenos. Cualquier regla
 * adicional —longitud mínima, formato— filtraría información sobre las
 * credenciales válidas antes incluso de enviarlas.
 */
const schema = z.object({
  username: z.string().min(1, "El usuario es obligatorio."),
  password: z.string().min(1, "La contraseña es obligatoria."),
});

type FormValues = z.infer<typeof schema>;

export function LoginPage() {
  const navigate = useNavigate();
  const login = useAuthStore((state) => state.login);
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { username: "", password: "" },
  });

  const onSubmit = async (values: FormValues) => {
    setError(null);

    try {
      await login(values.username, values.password);
      navigate("/", { replace: true });
    } catch (caught) {
      setError(toApiError(caught).message);
    }
  };

  return (
    <div className="bg-muted/30 flex min-h-svh items-center justify-center p-6">
      <Card className="w-full max-w-sm">
        <CardHeader className="space-y-1">
          <CardTitle className="text-xl">Factorización QR</CardTitle>
          <p className="text-muted-foreground text-sm">
            Inicia sesión para acceder al sistema.
          </p>
        </CardHeader>

        <CardContent>
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4" noValidate>
            <div className="space-y-1.5">
              <Label htmlFor="username">Usuario</Label>
              <Input
                id="username"
                autoComplete="username"
                autoFocus
                aria-invalid={!!errors.username}
                {...register("username")}
              />
              {errors.username ? (
                <p className="text-destructive text-xs">{errors.username.message}</p>
              ) : null}
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="password">Contraseña</Label>
              <Input
                id="password"
                type="password"
                autoComplete="current-password"
                aria-invalid={!!errors.password}
                {...register("password")}
              />
              {errors.password ? (
                <p className="text-destructive text-xs">{errors.password.message}</p>
              ) : null}
            </div>

            {error ? (
              <div className="border-destructive/40 bg-destructive/5 text-destructive flex items-start gap-2 rounded-md border p-3 text-sm">
                <AlertCircle className="mt-0.5 size-4 shrink-0" />
                <p>{error}</p>
              </div>
            ) : null}

            <Button type="submit" className="w-full" disabled={isSubmitting}>
              <LogIn className="size-4" />
              {isSubmitting ? "Entrando…" : "Entrar"}
            </Button>
          </form>

          {/*
            Credenciales visibles a propósito.

            Son estáticas y de demostración, definidas en el docker-compose. El
            sistema no gestiona usuarios: lo que el reto pide demostrar es la
            emisión y verificación de JWT, no un CRUD de cuentas. Mostrarlas
            ahorra a quien evalúa tener que buscarlas en el README.
          */}
          <div className="text-muted-foreground mt-6 rounded-md border border-dashed p-3 text-xs">
            <p className="mb-1 font-medium">Credenciales de demostración</p>
            <p className="tabular">
              usuario <span className="text-foreground font-mono">admin</span> · contraseña{" "}
              <span className="text-foreground font-mono">Reto2026.Demo</span>
            </p>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
