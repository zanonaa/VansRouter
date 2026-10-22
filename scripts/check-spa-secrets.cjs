#!/usr/bin/env node
// Fase 5 — verificação de que nenhum segredo/configuração de ambiente vaza no
// bundle estático da SPA. O Vite substitui process.env por valores vazios
// (exceto NEXT_PUBLIC_* documentadas); este script varre o dist em busca de
// chaves sensíveis do .env.example e de padrões de credencial embutidos.
const fs = require("node:fs");
const path = require("node:path");

const root = path.resolve(__dirname, "..");
const dist = path.join(root, "dist");

function fail(message) {
  console.error(`FAIL segredos no bundle: ${message}`);
  process.exit(1);
}

if (!fs.existsSync(path.join(dist, "index.html"))) {
  fail("dist/index.html ausente — rode pnpm build antes desta verificação");
}

// Chaves sensíveis do template de ambiente (valor presente = vazamento).
// Placeholders do próprio template são permitidos: a landing exibe comandos
// de exemplo com INITIAL_PASSWORD='replace-with-a-strong-password'.
const envExample = fs.readFileSync(path.join(root, ".env.example"), "utf8");
const placeholders = new Map();
const secretKeys = [...envExample.matchAll(/^([A-Z][A-Z0-9_]+)=(.*)$/gm)]
  .map((m) => {
    placeholders.set(m[1], m[2].trim());
    return m[1];
  })
  .filter((key) => !key.startsWith("NEXT_PUBLIC_"));

const files = [];
(function walk(dir) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) walk(full);
    else if (/\.(js|css|html)$/.test(entry.name)) files.push(full);
  }
})(dist);

const offenders = [];
for (const file of files) {
  const content = fs.readFileSync(file, "utf8");
  for (const key of secretKeys) {
    const matches = content.match(new RegExp(`${key}\\s*[:=]\\s*["']([^"']{6,})`, "g")) || [];
    for (const match of matches) {
      const value = (match.match(/["']([^"']+)["']/) || [])[1] || "";
      if (value && value !== placeholders.get(key)) {
        offenders.push(`${path.relative(dist, file)}: ${key} com valor "${value.slice(0, 12)}..."`);
      }
    }
  }
  if (/["'](sk-[A-Za-z0-9]{20,}|GOCSPX-[A-Za-z0-9_-]{20,})["']/.test(content)) {
    offenders.push(`${path.relative(dist, file)}: padrão de credencial (sk-/GOCSPX)`);
  }
}

if (offenders.length > 0) {
  for (const offender of offenders) console.error(`  ${offender}`);
  fail(`${offenders.length} ocorrências`);
}
console.log(`OK: nenhum segredo em ${files.length} arquivos do bundle (chaves: ${secretKeys.length})`);
