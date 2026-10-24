import { describe, expect, it } from "vitest";
import { readFileSync, existsSync } from "node:fs";
import path from "node:path";

const root = path.resolve(import.meta.dirname, "../..");
const read = file => readFileSync(path.join(root, file), "utf8");
describe("VansGo brand", () => {
  it("uses local supplied logo assets and the new product name", () => {
    expect(read("src/shared/constants/config.js")).toContain('name: "VansGo"');
    expect(read("index.html")).toContain("VansGo — Gateway de IA");
    expect(read("index.html")).toContain('/favicon.png');
    for (const file of ["public/vansgo-logo.webp", "public/favicon.png", "public/icons/vansgo-192.png", "public/icons/vansgo-512.png"]) expect(existsSync(path.join(root, file))).toBe(true);
    for (const file of ["src/shared/components/Sidebar.js", "src/app/masuk/MasukClient.js", "src/app/landing/components/Navigation.js"]) expect(read(file)).toContain('/vansgo-logo.webp');
  });
  it("defines Go cyan and navy theme colors without changing storage identity", () => {
    const css = read("src/app/globals.css");
    expect(css).toContain("#007d9c");
    expect(css).toContain("#5dc9e2");
    expect(css).toContain("#061b37");
    expect(read("../docker-compose.yml")).toContain("9router-data:/app/data");
  });
});
