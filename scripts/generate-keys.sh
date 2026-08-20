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

if [ -f "$KEYS_DIR/private.pem" ] && [ -f "$KEYS_DIR/public/public.pem" ]; then
  echo "Ya existen claves en $KEYS_DIR. Borrelas manualmente si desea regenerarlas."
  exit 0
fi

# 2048 bits es el minimo recomendado hoy para RSA y suficiente para tokens de
# vida corta. El formato PKCS#8 es el que espera golang-jwt sin conversiones.
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$KEYS_DIR/private.pem" 2>/dev/null
openssl rsa -in "$KEYS_DIR/private.pem" -pubout -out "$KEYS_DIR/public.pem" 2>/dev/null

# Copia de la clave publica en su propio directorio.
#
# Es lo que docker-compose monta en la API de estadisticas: al montar un
# directorio que contiene UNICAMENTE la clave publica, ese contenedor no tiene
# forma de alcanzar la privada. La separacion entre firmar y verificar queda
# impuesta por la infraestructura y no solo por convencion del codigo.
mkdir -p "$KEYS_DIR/public"
cp "$KEYS_DIR/public.pem" "$KEYS_DIR/public/public.pem"

# Permisos de lectura para todos, y no 600 como seria habitual en una clave
# privada.
#
# Es necesario porque los contenedores se ejecutan con un usuario sin
# privilegios (UID 65532) distinto del que ejecuta este script: con 600, la API
# en Go no podria leer la clave que necesita para firmar y no arrancaria.
#
# El riesgo es aceptable y esta acotado: es un par generado localmente para
# desarrollo, no se versiona y no sale de la maquina. En un despliegue real la
# clave se inyecta desde el gestor de secretos de la plataforma y nunca toca el
# sistema de ficheros.
chmod 644 "$KEYS_DIR/private.pem" "$KEYS_DIR/public.pem" "$KEYS_DIR/public/public.pem" 2>/dev/null || true

echo "Claves generadas en $KEYS_DIR/"
echo "  private.pem  -> firma de tokens (solo la API en Go). NO versionar."
echo "  public.pem   -> verificacion de tokens."
echo "  public/      -> lo que se monta en la API de estadisticas."
