#!/usr/bin/env sh
#
# Genera el par de claves RSA que sostiene la autenticacion del proyecto.
#
# La API en Go firma los tokens con la clave PRIVADA; la API en Node y
# cualquier otro consumidor solo reciben la clave PUBLICA para verificarlos.
# Esa asimetria es deliberada: el servicio de estadisticas es incapaz de emitir
# tokens, de modo que comprometerlo no permite suplantar a nadie. Con un
# secreto compartido (HS256) ambos servicios podrian firmar.
#
# Las claves NO se versionan. Cada entorno genera las suyas y en produccion se
# inyectan desde el gestor de secretos de la plataforma.
#
# Uso:  sh scripts/generate-keys.sh [directorio_destino]

set -eu

KEYS_DIR="${1:-keys}"

if ! command -v openssl >/dev/null 2>&1; then
  echo "Error: se requiere openssl en el PATH." >&2
  exit 1
fi

mkdir -p "$KEYS_DIR"

if [ -f "$KEYS_DIR/private.pem" ]; then
  echo "Ya existen claves en $KEYS_DIR. Borrelas manualmente si desea regenerarlas."
  exit 0
fi

# 2048 bits es el minimo recomendado hoy para RSA y suficiente para tokens de
# vida corta. El formato PKCS#8 es el que espera golang-jwt sin conversiones.
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$KEYS_DIR/private.pem" 2>/dev/null
openssl rsa -in "$KEYS_DIR/private.pem" -pubout -out "$KEYS_DIR/public.pem" 2>/dev/null

# La clave privada solo debe ser legible por su propietario.
chmod 600 "$KEYS_DIR/private.pem" 2>/dev/null || true
chmod 644 "$KEYS_DIR/public.pem" 2>/dev/null || true

echo "Claves generadas en $KEYS_DIR/"
echo "  private.pem  -> firma de tokens (solo la API en Go). NO versionar."
echo "  public.pem   -> verificacion de tokens (API en Node, frontend)."
