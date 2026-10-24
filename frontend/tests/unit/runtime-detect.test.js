import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";

// Mock node:fs so the filesystem probes in runtime.js are deterministic
// regardless of the host environment (test runners are often systemd
// hosts where /run/systemd/system exists, which would skew isSystemd).
vi.mock("node:fs", async () => {
  const actual = await vi.importActual("node:fs");
  return {
    ...actual,
    existsSync: vi.fn(() => false),
    readFileSync: vi.fn(() => ""),
  };
});

import { existsSync, readFileSync } from "node:fs";
import { detectRuntime, updateAndRestartCommand } from "@/shared/utils/runtime";

const ORIGINAL_ENV = { ...process.env };

const DETECTION_VARS = [
  "PM2_HOME_PATH",
  "PM2_JSON_PROCESSING",
  "pm2_env",
  "INVOCATION_ID",
  "STY",
  "TMUX",
];

function wipeDetectionVars() {
  for (const k of DETECTION_VARS) delete process.env[k];
}

function setEnv(patch) {
  // Wipe known detection vars so leftover values from CI / other tests
  // don't make results non-deterministic.
  wipeDetectionVars();
  Object.assign(process.env, patch);
}

// Configure fs mocks for each test. By default, NOTHING exists on the
// filesystem — that lets each test opt in to docker / systemd markers
// by setting them explicitly.
function setFiles(map) {
  // map: { "/run/systemd/system": true, "/.dockerenv": true, ... }
  existsSync.mockImplementation((p) => Boolean(map[p]));
}

afterEach(() => {
  // Restore captured env, THEN immediately wipe detection vars. This is
  // critical because ORIGINAL_ENV may contain the host's INVOCATION_ID
  // (test runners are usually systemd hosts), which would make isSystemd()
  // return true and beat docker/tmux/screen in every test that doesn't
  // explicitly setEnv({INVOCATION_ID: ''}).
  process.env = { ...ORIGINAL_ENV };
  wipeDetectionVars();
  existsSync.mockReset();
  existsSync.mockImplementation(() => false);
  readFileSync.mockReset();
  readFileSync.mockImplementation(() => "");
});

describe("detectRuntime", () => {
  beforeEach(() => {
    setFiles({}); // no filesystem markers by default
  });

  it("returns 'pm2' when PM2_HOME_PATH is set", () => {
    setEnv({ PM2_HOME_PATH: "/home/user/.pm2" });
    expect(detectRuntime()).toBe("pm2");
  });

  it("returns 'pm2' when PM2_JSON_PROCESSING is set", () => {
    setEnv({ PM2_JSON_PROCESSING: "1" });
    expect(detectRuntime()).toBe("pm2");
  });

  it("returns 'pm2' when pm2_env is set (older PM2 fallback)", () => {
    setEnv({ pm2_env: '{"name":"vansrouter"}' });
    expect(detectRuntime()).toBe("pm2");
  });

  it("returns 'systemd' when INVOCATION_ID is set", () => {
    setEnv({ INVOCATION_ID: "abc-123-def" });
    expect(detectRuntime()).toBe("systemd");
  });

  it("returns 'direct' when /run/systemd/system exists without service env markers (interactive shell)", () => {
    setFiles({ "/run/systemd/system": true });
    expect(detectRuntime()).toBe("direct");
  });

  it("returns 'tmux' when TMUX is set", () => {
    setEnv({ TMUX: "/tmp/tmux-1000/default,12345,0" });
    expect(detectRuntime()).toBe("tmux");
  });

  it("returns 'screen' when STY is set (no TMUX)", () => {
    setEnv({ STY: "12345.pts-0.host" });
    expect(detectRuntime()).toBe("screen");
  });

  it("returns 'docker' when /.dockerenv exists", () => {
    setFiles({ "/.dockerenv": true });
    expect(detectRuntime()).toBe("docker");
  });

  it("returns 'docker' when /proc/1/cgroup mentions docker (no /.dockerenv)", () => {
    readFileSync.mockImplementation((p) =>
      p === "/proc/1/cgroup"
        ? "12:devices:/docker/abc123...\n11:cpu:/docker/abc123..."
        : "",
    );
    expect(detectRuntime()).toBe("docker");
  });

  it("returns 'direct' when nothing is set", () => {
    setEnv({});
    setFiles({});
    expect(detectRuntime()).toBe("direct");
  });

  it("prefers pm2 over everything else (defensive priority order)", () => {
    setEnv({
      PM2_HOME_PATH: "/x",
      TMUX: "/y",
      STY: "x",
      INVOCATION_ID: "id",
    });
    setFiles({ "/run/systemd/system": true, "/.dockerenv": true });
    expect(detectRuntime()).toBe("pm2");
  });

  it("prefers systemd over tmux/screen/docker when no pm2", () => {
    setEnv({ INVOCATION_ID: "abc-123-def", TMUX: "/y", STY: "x" });
    setFiles({ "/.dockerenv": true });
    expect(detectRuntime()).toBe("systemd");
  });

  it("prefers tmux over screen when both are set (tmux sets STY too)", () => {
    setEnv({ TMUX: "/y", STY: "x" });
    setFiles({});
    expect(detectRuntime()).toBe("tmux");
  });

  it("prefers screen over docker when neither pm2/systemd/tmux is set", () => {
    setEnv({ STY: "x" });
    setFiles({ "/.dockerenv": true });
    expect(detectRuntime()).toBe("screen");
  });
});

describe("updateAndRestartCommand", () => {
  const PKG = "vansrouter";

  beforeEach(() => {
    process.env = { ...ORIGINAL_ENV };
  });

  it("returns pm2 restart command when runtime is pm2", () => {
    const cmd = updateAndRestartCommand("pm2", PKG);
    expect(cmd).toContain("npm i -g vansrouter@latest");
    expect(cmd).toContain("pm2 restart vansrouter");
  });

  it("uses pm2 process name from pm2_env when available", () => {
    setEnv({ pm2_env: '{"name":"my-custom-pm2-app"}' });
    const cmd = updateAndRestartCommand("pm2", PKG);
    expect(cmd).toContain("pm2 restart my-custom-pm2-app");
  });

  it("falls back to default 'vansrouter' name when pm2_env has no name", () => {
    setEnv({ pm2_env: '{"other":"value"}' });
    const cmd = updateAndRestartCommand("pm2", PKG);
    expect(cmd).toContain("pm2 restart vansrouter");
  });

  it("returns systemd restart command when runtime is systemd", () => {
    const cmd = updateAndRestartCommand("systemd", PKG);
    expect(cmd).toContain("npm i -g vansrouter@latest");
    expect(cmd).toContain("sudo systemctl restart");
  });

  it("returns screen re-launch command when runtime is screen", () => {
    const cmd = updateAndRestartCommand("screen", PKG);
    expect(cmd).toContain("screen -dmS vansrouter");
  });

  it("returns tmux re-launch command when runtime is tmux", () => {
    const cmd = updateAndRestartCommand("tmux", PKG);
    expect(cmd).toContain("tmux new-session -d -s vansrouter");
  });

  it("returns docker hint when runtime is docker (host must pull new image)", () => {
    const cmd = updateAndRestartCommand("docker", PKG);
    expect(cmd).toContain("Docker");
    expect(cmd).toContain("Watchtower");
  });

  it("returns null for 'direct' runtime (user must restart manually)", () => {
    expect(updateAndRestartCommand("direct", PKG)).toBeNull();
  });

  it("returns null for unknown runtime values", () => {
    expect(updateAndRestartCommand("mystery", PKG)).toBeNull();
  });

  it("includes the provided package name in the install command", () => {
    const cmd = updateAndRestartCommand("pm2", "my-other-pkg");
    expect(cmd).toContain("npm i -g my-other-pkg@latest");
  });
});
