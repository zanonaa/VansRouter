// cli-chat-proxy rejects older identities with HTTP 426. Keep this on a
// current @xai-official/grok release (1.0.44 as of 2026-10-01; minimum 1.0.13).
// Overridable per deployment (read once at import): the gate moves with the
// upstream release, not with our credentials.
//   GROK_CLI_VERSION=1.4.2
// A malformed value is ignored with a warning rather than sent upstream.
const DEFAULT_GROK_CLI_VERSION = "1.0.44";
const VERSION_RE = /^\d+(?:\.\d+){1,3}$/;
const requestedGrokCliVersion = (process.env.GROK_CLI_VERSION || "").trim();
if (requestedGrokCliVersion && !VERSION_RE.test(requestedGrokCliVersion)) {
  console.warn(`[grok-cli] ignoring GROK_CLI_VERSION="${requestedGrokCliVersion}" (expected e.g. 1.4.2)`);
}
export const GROK_CLI_VERSION = VERSION_RE.test(requestedGrokCliVersion)
  ? requestedGrokCliVersion
  : DEFAULT_GROK_CLI_VERSION;
export const GROK_CLI_MODEL = "grok-build";
export const GROK_CLI_BASE_URL = "https://cli-chat-proxy.grok.com/v1";
export const GROK_CLI_CLIENT_IDENTIFIER = "grok-shell";
export const GROK_CLI_USER_AGENT = `grok-shell/${GROK_CLI_VERSION} (linux; x86_64)`;

// OAuth device-code endpoints (auth.x.ai) are reached with the pager-prefixed
// User-Agent rather than the inference one. Shape is part of the fingerprint —
// only the version tracks GROK_CLI_VERSION.
export const GROK_CLI_PAGER_USER_AGENT = `grok-pager/${GROK_CLI_VERSION} grok-shell/${GROK_CLI_VERSION} (linux; x86_64)`;

export function supportsGrokCliReasoningEffort(model) {
  // ponytail: unknown models omit effort until live metadata reaches dispatch.
  return /^grok-4\.5(?:$|-)/.test(String(model || ""));
}
