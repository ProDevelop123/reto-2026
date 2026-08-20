import path from "node:path";

import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    // Alias heredado del sistema de diseno existente, para que los componentes
    // de shadcn se copien sin tener que reescribir sus importaciones.
    alias: { "@": path.resolve(__dirname, "./src") },
  },
  server: {
    // Puerto fijo: es el origen que la API en Go declara en CORS_ORIGINS. Si
    // Vite eligiera otro al estar ocupado, el navegador bloquearia las
    // peticiones y el fallo seria dificil de diagnosticar.
    port: 5173,
    strictPort: true,
  },
  build: { outDir: "dist" },
});
