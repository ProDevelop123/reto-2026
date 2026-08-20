import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import App from "./App";
import "./index.css";

// El store de autenticacion se importa por su efecto de modulo: al cargarse
// conecta el cliente HTTP con las funciones de token y renovacion. Sin esta
// importacion, el interceptor no sabria de donde sacar el token.
import "./features/auth/store/auth.store";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
