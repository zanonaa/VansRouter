import fs from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { classifyInspectResult } from "../../scripts/inspect-image-digest.cjs";

const root = path.resolve(import.meta.dirname, "../..");
const read = (file) => fs.readFileSync(path.join(root, file), "utf8");

describe("release hardening contracts", () => {
  it("does not treat registry inspection errors as missing tags", () => {
    expect(classifyInspectResult({
      status: 1,
      stderr: "ERROR: ghcr.io/example/app:missing: not found",
    })).toEqual({ kind: "missing" });
    expect(() => classifyInspectResult({
      status: 1,
      stderr: "failed to connect to registry: connection refused",
    })).toThrow(/inspection failed/);
    expect(() => classifyInspectResult({
      status: 1,
      stderr: "requested access to the resource is denied",
    })).toThrow(/inspection failed/);
    expect(classifyInspectResult({
      status: 0,
      stdout: JSON.stringify(`sha256:${"a".repeat(64)}`),
    })).toEqual({ kind: "found", digest: `sha256:${"a".repeat(64)}` });
  });

  it("keeps the web gateway CI focused on tests, build, and Linux multi-arch Docker", () => {
    const workflow = read(".github/workflows/ci.yml");
    expect(workflow).toContain("pnpm test");
    expect(workflow).toContain("pnpm run build");
    expect(workflow).toContain("platforms: linux/amd64,linux/arm64");
    expect(workflow).toContain("push:");
    expect(workflow).not.toContain("windows-latest");
    expect(workflow).not.toContain("macos-latest");
  });

  it("keeps Docker dependency installation locked and externally downloaded binaries verified", () => {
    const dockerfile = read("Dockerfile");
    expect(dockerfile).toContain("pnpm install --frozen-lockfile");
    expect(dockerfile).toContain("@sha256:");
    expect(dockerfile).toContain("npm_config_build_from_source=true");
    expect(dockerfile).toContain("linuxmusl-${bs_arch}.tar.gz");
    expect(dockerfile).toContain("9b14618f3d9aa9b70daade965bca892f324b063e1cc7532b030d7138a8f9d070");
    expect(dockerfile).toContain("b5e92ec2637bb578cc12b8b9b94877a82c3ef5e862ccc72d420706aab978bb9a");
  });

  it("requires an explicit VansRouter version while preserving 9router storage", () => {
    const compose = read("docker-compose.yml");
    expect(compose).toContain("VANSROUTER_VERSION");
    expect(compose).not.toContain("vansrouter:latest");
    expect(compose).toContain("9router-data:/app/data");
    expect(compose).toContain("vansrouter-data:/migration-data:ro");
    expect(read(".env.example")).toMatch(/VANSROUTER_VERSION=\d+\.\d+\.\d+/);
  });

  it("keeps executable shell entrypoints LF across Windows checkouts", () => {
    const attributes = read(".gitattributes");
    expect(attributes).toMatch(/\*\.sh text eol=lf/);
    expect(attributes).toMatch(/docker\/entrypoint\.sh text eol=lf/);
  });

  it("uses the documented application port for the production start script", () => {
    const packageJson = JSON.parse(read("package.json"));
    expect(packageJson.scripts.start).toContain("--port 20128");
    expect(packageJson.scripts["preflight:upgrade"]).toContain("preflight-upgrade.cjs");
  });

  it("packages the production secret validator with the web gateway", () => {
    expect(read("Dockerfile")).toContain("COPY runtime-secrets.cjs");
    expect(read("scripts/build.js")).toContain("runtime-secrets.cjs");
  });

  it("keeps the native database lockfile in the Docker build context", () => {
    const gitignore = read(".gitignore");
    expect(fs.existsSync(path.join(root, "docker/native-deps/package-lock.json"))).toBe(true);
    expect(gitignore).toContain("!docker/native-deps/package-lock.json");
    expect(read("scripts/smoke-container.cjs")).toContain("VANROUTER_SKIP_UPDATE_CHECK");
  });

  it("keeps landing onboarding on Docker and the web dashboard", () => {
    const getStarted = read("src/app/landing/components/GetStarted.js");
    const navigation = read("src/app/landing/components/Navigation.js");
    expect(getStarted).toContain("docker run");
    expect(getStarted).toContain("ghcr.io/zanonaa/vansrouter:1.2.0");
    expect(getStarted).toContain("/masuk");
    expect(getStarted).not.toContain("npx vansrouter");
    expect(navigation).toContain("VansRouter");
  });
});
