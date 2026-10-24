import { describe, expect, it } from "vitest";
import fs from "node:fs";
import path from "node:path";
import { stripThinkingSuffix, supportsKiroThinkingLevels } from "../../src/catalogs/modelNames.js";

describe("frontend metadata boundary", () => {
  it("only normalizes names for catalog lookup", () => {
    expect(stripThinkingSuffix("gpt-6.1-sol(high)")).toBe("gpt-6.1-sol");
    expect(stripThinkingSuffix("plain-model")).toBe("plain-model");
  });
  it("preserves Kiro picker availability without importing request builders", () => {
    for (const model of ["claude-opus-4.6", "claude-sonnet-5", "gpt-5.6"]) expect(supportsKiroThinkingLevels(model)).toBe(true);
    for (const model of ["claude-sonnet-4.5", "claude-opus-4-20250514", "unknown"]) expect(supportsKiroThinkingLevels(model)).toBe(false);
  });
  it("never imports test references or legacy request transformers in application source", () => {
    const root = path.resolve(import.meta.dirname, "../../src");
    function walk(dir) {
      for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const file = path.join(dir, entry.name);
        if (entry.isDirectory()) walk(file);
        else if (/\.[cm]?jsx?$/.test(file)) {
          const text = fs.readFileSync(file, "utf8");
          expect(text, file).not.toMatch(/(?:from\s*|import\s*\()["'][^"']*(?:tests\/reference|thinkingUnified|kiroConstants|modelMetadata)/);
        }
      }
    }
    walk(root);
  });
});
