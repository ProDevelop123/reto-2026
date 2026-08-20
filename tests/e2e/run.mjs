/**
 * Prueba de integracion end-to-end del sistema completo.
 *
 * Ejercita el stack tal como lo veria un cliente real: por HTTP, contra los
 * contenedores en ejecucion, sin dobles ni mocks. Comprueba el flujo completo
 * —autenticacion, factorizacion, orquestacion entre servicios y manejo de
 * errores— y verifica la correccion matematica del resultado recalculandola por
 * su cuenta.
 *
 * Se escribe sin dependencias externas, usando solo el fetch nativo de Node,
 * para que pueda ejecutarse en cualquier entorno con Node 18 o superior sin
 * instalar nada.
 *
 * Uso:
 *   docker compose up -d --build
 *   node tests/e2e/run.mjs
 *
 * Variables de entorno:
 *   API_URL        (por defecto http://127.0.0.1:8080)
 *   AUTH_USERNAME  (por defecto admin)
 *   AUTH_PASSWORD  (por defecto Reto2026.Demo)
 */

const API = process.env.API_URL ?? "http://127.0.0.1:8080";
const USERNAME = process.env.AUTH_USERNAME ?? "admin";
const PASSWORD = process.env.AUTH_PASSWORD ?? "Reto2026.Demo";

/**
 * Cabecera de verificacion de origen.
 *
 * La exigen las rutas cuya credencial es la cookie. Ver la explicacion completa
 * en api-go/internal/infrastructure/http/middleware/csrf.go.
 */
const CSRF = { "X-Refresh-Request": "1" };

// --- Utilidades de asercion -------------------------------------------------

let passed = 0;
let failed = 0;

function check(name, condition, detail = "") {
  if (condition) {
    passed += 1;
    console.log(`  ✓ ${name}${detail ? `  ${detail}` : ""}`);
  } else {
    failed += 1;
    console.error(`  ✗ ${name}${detail ? `  ${detail}` : ""}`);
  }
}

function section(title) {
  console.log(`\n${title}`);
}

async function request(path, { method = "POST", token, cookie, body, headers = {} } = {}) {
  const response = await fetch(`${API}${path}`, {
    method,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(cookie ? { Cookie: cookie } : {}),
      ...headers,
    },
    ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
  });

  const text = await response.text();
  let parsed = null;
  try {
    parsed = JSON.parse(text);
  } catch {
    // Respuesta sin cuerpo JSON; se deja en null y lo comprueba quien llama.
  }

  const setCookie = response.headers.getSetCookie?.() ?? [];

  return {
    status: response.status,
    body: parsed,
    cookie: setCookie.length > 0 ? setCookie[0].split(";")[0] : null,
    rawCookie: setCookie[0] ?? "",
  };
}

// --- Algebra lineal, para verificar el resultado de forma independiente -----

function multiply(a, b) {
  return a.map((row) =>
    b[0].map((_, j) => row.reduce((sum, value, k) => sum + value * b[k][j], 0)),
  );
}

function transpose(m) {
  return m[0].map((_, j) => m.map((row) => row[j]));
}

function maxAbsDifference(a, b) {
  return Math.max(...a.flatMap((row, i) => row.map((v, j) => Math.abs(v - b[i][j]))));
}

function identity(n) {
  return Array.from({ length: n }, (_, i) =>
    Array.from({ length: n }, (_, j) => (i === j ? 1 : 0)),
  );
}

/** Umbral de residuo admitido. Amplio frente al epsilon de la maquina (2.2e-16),
 *  porque lo que se quiere detectar es un algoritmo equivocado, que fallaria por
 *  varios ordenes de magnitud, no la acumulacion normal de redondeo. */
const RESIDUAL_TOLERANCE = 1e-10;

// --- Espera de arranque -----------------------------------------------------

async function waitForApi(timeoutMs = 60_000) {
  const deadline = Date.now() + timeoutMs;

  while (Date.now() < deadline) {
    try {
      const response = await fetch(`${API}/health`);
      if (response.ok) return true;
    } catch {
      // El servicio todavia no acepta conexiones; se reintenta.
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }

  return false;
}

// --- Prueba -----------------------------------------------------------------

async function main() {
  console.log(`Prueba end-to-end contra ${API}\n`);

  if (!(await waitForApi())) {
    console.error(`No se pudo contactar con ${API}. ¿Esta levantado el docker compose?`);
    process.exit(1);
  }

  // === Salud ===
  section("Salud");
  {
    const response = await fetch(`${API}/health`);
    const body = await response.json();
    check("GET /health responde sin autenticar", response.status === 200);
    check("informa del servicio", body?.data?.service !== undefined, body?.data?.service ?? "");
  }

  // === Autenticacion ===
  section("Autenticacion");

  {
    const res = await request("/api/v1/qr", { body: { matrix: [[1, 2], [3, 4]] } });
    check("sin token se rechaza el endpoint protegido", res.status === 401);
    check(
      "el motivo del rechazo es explicito",
      res.body?.error?.details?.reason === "missing_header",
      res.body?.error?.details?.reason ?? "",
    );
  }

  {
    const res = await request("/api/v1/auth/login", {
      body: { username: USERNAME, password: "contrasena-incorrecta" },
    });
    check("credenciales incorrectas se rechazan", res.status === 401);
    // El mensaje no debe permitir distinguir "usuario inexistente" de
    // "contrasena incorrecta": hacerlo permitiria enumerar usuarios.
    check(
      "el mensaje no revela cual campo fallo",
      res.body?.error?.message === "Usuario o contrasena incorrectos.",
      res.body?.error?.message ?? "",
    );
  }

  const session = await request("/api/v1/auth/login", {
    body: { username: USERNAME, password: PASSWORD },
  });

  check("el login devuelve 200", session.status === 200);
  check("entrega un token de acceso", typeof session.body?.data?.accessToken === "string");
  check(
    "el refresh token NO viaja en el cuerpo",
    session.body?.data?.refreshToken === undefined,
  );
  check("el refresh token viaja en cookie HttpOnly", session.rawCookie.includes("HttpOnly"));
  check(
    "la cookie se limita a las rutas de autenticacion",
    session.rawCookie.includes("path=/api/v1/auth"),
  );

  const token = session.body?.data?.accessToken;
  if (!token) {
    console.error("\nSin token no se puede continuar.");
    process.exit(1);
  }

  // === Pipeline completo ===
  section("Pipeline de factorizacion");

  const matrix = [
    [12, -51, 4],
    [6, 167, -68],
    [-4, 24, -41],
  ];

  const qr = await request("/api/v1/qr", { token, body: { matrix } });

  check("POST /api/v1/qr responde 200", qr.status === 200);

  const data = qr.body?.data;

  check("devuelve la matriz original", data?.matrix?.name === "A");
  check("devuelve Q", data?.q?.name === "Q");
  check("devuelve R", data?.r?.name === "R");

  // La presencia de estadisticas demuestra que la API en Go llamo de verdad al
  // servicio en Node y compuso ambos resultados: es la prueba de la
  // orquestacion entre servicios.
  check("incluye las estadisticas del servicio en Node", data?.statistics?.global !== undefined);

  for (const metric of ["max", "min", "average", "sum", "isAnyDiagonal"]) {
    check(`la metrica "${metric}" esta presente`, data?.statistics?.global?.[metric] !== undefined);
  }

  section("Correccion matematica");
  {
    const reconstruction = maxAbsDifference(matrix, multiply(data.q.data, data.r.data));
    check(
      "A = Q·R",
      reconstruction <= RESIDUAL_TOLERANCE,
      `residuo ${reconstruction.toExponential(2)}`,
    );

    const gram = multiply(transpose(data.q.data), data.q.data);
    const orthogonality = maxAbsDifference(gram, identity(gram.length));
    check(
      "Qᵀ·Q = I",
      orthogonality <= RESIDUAL_TOLERANCE,
      `residuo ${orthogonality.toExponential(2)}`,
    );

    const belowDiagonal = data.r.data.flatMap((row, i) => row.filter((_, j) => j < i));
    check(
      "R es triangular superior con ceros exactos",
      belowDiagonal.every((value) => value === 0),
    );
  }

  section("Deteccion de matriz diagonal");
  {
    const res = await request("/api/v1/qr", { token, body: { matrix: [[3, 0], [0, 5]] } });
    check("una matriz diagonal activa el indicador", res.body?.data?.statistics?.global?.isAnyDiagonal === true);
    check(
      "identifica cuales lo son",
      Array.isArray(res.body?.data?.statistics?.global?.diagonalMatrices),
      JSON.stringify(res.body?.data?.statistics?.global?.diagonalMatrices ?? []),
    );
  }

  section("Validacion de entrada");
  {
    const cases = [
      ["matriz no rectangular", { matrix: [[1, 2], [3]] }, 422],
      ["matriz vacia", { matrix: [] }, 422],
      ["modo inexistente", { matrix: [[1, 2], [3, 4]], mode: "oblicuo" }, 422],
      ["tolerancia negativa", { matrix: [[1, 2], [3, 4]], tolerance: -1 }, 422],
    ];

    for (const [name, body, expected] of cases) {
      const res = await request("/api/v1/qr", { token, body });
      check(name, res.status === expected, `HTTP ${res.status}`);
    }
  }

  section("Rotacion de la sesion");
  {
    const first = await request("/api/v1/auth/refresh", {
      cookie: session.cookie,
      headers: CSRF,
    });
    check("el refresco funciona", first.status === 200);
    check("la cookie rota", first.cookie !== null && first.cookie !== session.cookie);

    // Reutilizar un token ya canjeado es la firma de un robo: el legitimo ya lo
    // uso, asi que quien lo presenta ahora tiene una copia.
    const reused = await request("/api/v1/auth/refresh", {
      cookie: session.cookie,
      headers: CSRF,
    });
    check("se detecta la reutilizacion del token", reused.status === 401);

    // Y la familia entera queda revocada, tambien el token legitimo: no se
    // puede saber cual de las dos copias es del atacante.
    const legitimate = await request("/api/v1/auth/refresh", {
      cookie: first.cookie,
      headers: CSRF,
    });
    check("la familia completa queda revocada", legitimate.status === 401);
  }

  section("Proteccion contra CSRF");
  {
    const fresh = await request("/api/v1/auth/login", {
      body: { username: USERNAME, password: PASSWORD },
    });

    const withoutHeader = await request("/api/v1/auth/refresh", { cookie: fresh.cookie });
    check("el refresco sin la cabecera se rechaza", withoutHeader.status === 401);
    check(
      "el motivo es explicito",
      withoutHeader.body?.error?.details?.reason === "missing_csrf_header",
      withoutHeader.body?.error?.details?.reason ?? "",
    );

    const withHeader = await request("/api/v1/auth/refresh", {
      cookie: fresh.cookie,
      headers: CSRF,
    });
    check("con la cabecera se acepta", withHeader.status === 200);
  }

  section("Aislamiento de red");
  {
    // La API de estadisticas es un servicio interno: el compose no publica su
    // puerto, de modo que desde fuera de la red de contenedores no existe.
    let reachable = false;
    try {
      const response = await fetch("http://127.0.0.1:3001/health", {
        signal: AbortSignal.timeout(2000),
      });
      reachable = response.ok;
    } catch {
      reachable = false;
    }

    check("la API de estadisticas NO es alcanzable desde el host", !reachable);
  }

  // --- Resultado ---
  console.log(`\n${passed} correctas, ${failed} fallidas`);
  process.exit(failed > 0 ? 1 : 0);
}

main().catch((error) => {
  console.error("\nLa prueba fallo con una excepcion:", error);
  process.exit(1);
});
