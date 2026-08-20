/**
 * Configuracion de Jest para ESM nativo.
 *
 * El proyecto usa modulos ES ("type": "module"), por lo que no se define
 * ningun transform: Node ejecuta el codigo tal cual, sin Babel de por medio.
 * Eso exige arrancar Jest con --experimental-vm-modules, que es lo que hacen
 * los scripts de npm.
 */
export default {
  testEnvironment: 'node',
  transform: {},
  testMatch: ['**/tests/**/*.test.js'],
  collectCoverageFrom: ['src/**/*.js', '!src/server.js'],
  coverageThreshold: {
    global: { statements: 80, branches: 75, functions: 80, lines: 80 },
  },
  verbose: true,
};
