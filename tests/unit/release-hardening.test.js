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
    expect(workflow).not.toMatch(/run:\s*pnpm run build/);
    expect(read("Dockerfile")).toContain("RUN pnpm run build");
    expect(JSON.parse(read("package.json")).scripts.build).toBe("vite build");
    expect(workflow).toContain("platforms: linux/amd64,linux/arm64");
    expect(workflow).toContain("push:");
    expect(workflow).not.toContain("windows-latest");
    expect(workflow).not.toContain("macos-latest");
  });

  it("keeps Docker dependency installation locked and the Go build checksum-verified", () => {
    const dockerfile = read("Dockerfile");
    expect(dockerfile).toContain("pnpm install --frozen-lockfile");
    expect(dockerfile).toContain("@sha256:");
    expect(dockerfile).toContain("go mod download");
    expect(dockerfile).toContain("COPY backend/go.mod backend/go.sum ./");
  });

  it("keeps the Tailwind PostCSS pipeline inside the SPA build stage", () => {
    // Regressão real (Dev 2026-10-23): sem postcss.config.mjs no contexto do
    // estágio Vite, o @import "tailwindcss" vira CSS estático e a UI sobe sem
    // utilitárias. O build precisa copiar o config e provar que gerou classes.
    const dockerfile = read("Dockerfile");
    expect(dockerfile).toContain("COPY index.html vite.config.js postcss.config.mjs ./");
    expect(dockerfile).toContain("grep -q '\\.flex{'");
    expect(dockerfile).toContain("grep -q '\\.grid{'");
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

  it("keeps runtime scripts free of Next entries and documents the app port", () => {
    const packageJson = JSON.parse(read("package.json"));
    const scripts = Object.values(packageJson.scripts || {}).join(" ");
    expect(scripts).not.toContain("next ");
    expect(scripts).not.toMatch(/next-(dev|build|start)/);
    expect(read("docker-compose.yml")).toContain("20128:20128");
    expect(read("Dockerfile")).toContain("ENV PORT=20128");
  });

  it("keeps landing onboarding on Docker and the web dashboard", () => {
    const getStarted = read("src/app/landing/components/GetStarted.js");
    const navigation = read("src/app/landing/components/Navigation.js");
    expect(getStarted).toContain("docker run");
    expect(getStarted).toContain("ghcr.io/zanonaa/vansrouter:1.2.0");
    expect(getStarted).toContain("/masuk");
    expect(navigation).toContain("VansRouter");
  });
});
