import { cleanupProviderConnections, getSettings } from "@/lib/localDb";

process.setMaxListeners(20);

const STARTUP_DEFER_MS = 3000;
const singleton = global.__appSingleton ??= { initialized: false };

/** Initialize the web gateway's background services (non-CLI). */
export async function initializeApp() {
  if (singleton.initialized) return;
  singleton.initialized = true;

  setTimeout(() => {
    runStartupTasks().catch((error) => {
      console.error("[InitApp] deferred startup failed:", error.message);
    });
  }, STARTUP_DEFER_MS);
}

async function runStartupTasks() {
  await cleanupProviderConnections();
  const settings = await getSettings();
  if (hasQuotaAutoPingEnabled(settings)) {
    import("@/shared/services/quotaAutoPing")
      .then(({ startQuotaAutoPing }) => startQuotaAutoPing())
      .catch((error) => console.log("[AutoPing] scheduler start failed:", error.message));
  }
}

function hasQuotaAutoPingEnabled(settings) {
  return [settings?.claudeAutoPing, settings?.codexAutoPing]
    .some((config) => Object.values(config?.connections || {}).some(Boolean));
}

export default initializeApp;
