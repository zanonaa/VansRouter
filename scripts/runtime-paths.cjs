#!/usr/bin/env node

const os = require("node:os");
const path = require("node:path");
const { spawnSync } = require("node:child_process");

const appName = process.env.PM2_APP_NAME || "9router";

function defaultDataDir({ env = process.env, home = os.homedir() } = {}) {
  if (env.DATA_DIR) return path.resolve(env.DATA_DIR);
  if (process.platform === "win32") {
    return path.join(env.APPDATA || path.join(home, "AppData", "Roaming"), "9router");
  }
  return path.join(home, ".9router");
}

function resolveRuntimePaths({
  env = process.env,
  home = os.homedir(),
  dataDir = null,
  releaseRoot = null,
  currentLink = null,
} = {}) {
  const resolvedDataDir = path.resolve(dataDir || defaultDataDir({ env, home }));
  const resolvedReleaseRoot = path.resolve(
    releaseRoot || env.RELEASE_ROOT || path.join(resolvedDataDir, "releases"),
  );
  const resolvedCurrentLink = path.resolve(
    currentLink || env.CURRENT_LINK || path.join(path.dirname(resolvedReleaseRoot), "current"),
  );
  return {
    dataDir: resolvedDataDir,
    releaseRoot: resolvedReleaseRoot,
    currentLink: resolvedCurrentLink,
  };
}

// Fase 4B: leitura de estado do PM2 migrada de scripts/deploy-atomic.cjs
// (removido junto com o runtime Next). O servidor de produção é o binário Go
// do release; o estado PM2 continua relevante para paths em atualizações.
function readPm2App({ failOnError = false, run = spawnSync } = {}) {
  const result = run("pm2", ["jlist"], { encoding: "utf8" });
  if (result.error) {
    if (failOnError) throw new Error(`Unable to query PM2 state: ${result.error.message}`);
    return null;
  }
  if (result.status !== 0) {
    if (failOnError) throw new Error(`Unable to query PM2 state (exit ${result.status}): ${result.stderr || "unknown error"}`);
    return null;
  }
  try {
    const apps = JSON.parse(result.stdout || "[]");
    return apps.find((entry) => entry.name === appName) || null;
  } catch (error) {
    if (failOnError) throw new Error(`PM2 returned invalid application state: ${error.message}`);
    return null;
  }
}

function readPm2Environment({ failOnError = false, run = spawnSync } = {}) {
  return readPm2App({ failOnError, run })?.pm2_env?.env || {};
}

module.exports = { defaultDataDir, resolveRuntimePaths, readPm2App, readPm2Environment };
