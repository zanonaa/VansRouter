const fs = require("node:fs");
const path = require("node:path");
const { resolveRuntimePaths } = require("./scripts/runtime-paths.cjs");

const root = __dirname;
// Keep PM2, the deployer, and the application on the same persistent data root.
// An explicit DATA_DIR remains authoritative; the default preserves ~/.9router.
const { dataDir, releaseRoot, currentLink } = resolveRuntimePaths();
// Fase 4B: o servidor de produção é o binário Go do release (com a SPA em
// SPA_DIR); custom-server.js/Next foram removidos.
const defaultReleaseServer = path.join(currentLink, "vansrouter");
const configuredReleaseServer = process.env.RELEASE_SERVER;
const releaseServer = process.env.CURRENT_LINK
  ? defaultReleaseServer
  : configuredReleaseServer && fs.existsSync(configuredReleaseServer)
    ? configuredReleaseServer
    : defaultReleaseServer;

module.exports = {
  apps: [{
    name: process.env.PM2_APP_NAME || "9router",
    cwd: root,
    script: releaseServer,
    interpreter: "none",
    exec_mode: "fork",
    instances: 1,
    env: {
      NODE_ENV: process.env.NODE_ENV || "production",
      PORT: process.env.PORT || "20128",
      DATA_DIR: dataDir,
      SPA_DIR: process.env.SPA_DIR || path.join(currentLink, "spa"),
      RELEASE_ROOT: releaseRoot,
      CURRENT_LINK: currentLink,
      RELEASE_SERVER: releaseServer,
      RELEASE_BUILD_ID: process.env.RELEASE_BUILD_ID || "",
    },
  }],
};
