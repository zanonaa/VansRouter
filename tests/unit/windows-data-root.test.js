import fs from "node:fs";
import { describe, expect, it } from "vitest";

// Data roots used by the web gateway and its upgrade/runtime helpers.
const DATA_ROOT_FILES = [
  "src/lib/dataDir.js",
  "src/lib/appUpdater.js",
  "src/lib/updater/updater.js",
  "scripts/runtime-paths.cjs",
  "src/shared/utils/apiKey.js",
];

describe("Windows data root", () => {
  it("uses one directory name everywhere, never the stale VansRoute brand", () => {
    const offenders = DATA_ROOT_FILES.filter((file) => {
      const source = fs.readFileSync(file, "utf8");
      return /APPDATA[^\n]*VansRoute|"VansRoute"\s*\)/.test(source);
    });
    expect(offenders).toEqual([]);
  });

  it("keeps the app package name used by the updater", () => {
    const source = fs.readFileSync("src/lib/updater/updater.js", "utf8");
    expect(source).toContain("vansrouter");
  });

  it("rejects a Unix DATA_DIR on Windows wherever the data root is resolved", () => {
    const resolvers = [
      "src/lib/dataDir.js",
      "src/shared/utils/apiKey.js",
    ];
    for (const file of resolvers) {
      const source = fs.readFileSync(file, "utf8");
      expect(source.includes("win32"), `${file} should branch on win32`).toBe(true);
      expect(source.includes("/^\\//"), `${file} should reject a leading-slash DATA_DIR`).toBe(true);
    }
  });
});
