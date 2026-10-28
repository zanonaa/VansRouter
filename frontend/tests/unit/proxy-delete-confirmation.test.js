import fs from "node:fs";
import { expect, it } from "vitest";

it("deletes directly and asks for confirmation only for bound pools", () => {
  const source = fs.readFileSync(new URL("../../src/app/(dashboard)/dashboard/proxy-pools/page.js", import.meta.url), "utf8");
  const single = source.split("const handleDelete =")[1].split("const handleTest =")[0];
  const bulk = source.split("const bulkDelete =")[1].split("const handleHealthCheck =")[0];
  for (const handler of [single, bulk]) {
    expect(handler).not.toContain("setConfirmState");
    expect(handler).toContain('method: "DELETE"');
    expect(handler).toContain("confirmUnlinkAndDelete");
  }
  expect(source).toContain('confirmText: "Excluir tudo"');
});
