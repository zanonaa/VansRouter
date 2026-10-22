// Lógica pura do guard de páginas da SPA (Fase 4A): mesma semântica de
// decisão do dashboardGuard original, com a autenticação decidida pelo
// backend (/api/auth/check) e nunca no navegador.
import { describe, expect, it } from "vitest";
import {
  AUTH_ALLOWED,
  AUTH_DENIED,
  decideAuthRoute,
  decideRootRedirect,
} from "../../src/spa/authGuard.js";

describe("spa auth guard", () => {
  it("autoriza apenas com resposta 200 do backend", () => {
    expect(decideAuthRoute(200)).toBe(AUTH_ALLOWED);
  });

  it("nega com 401/403 (sessão ausente ou inválida)", () => {
    expect(decideAuthRoute(401)).toBe(AUTH_DENIED);
    expect(decideAuthRoute(403)).toBe(AUTH_DENIED);
  });

  it("nega por padrão quando a verificação falha (rede/servidor)", () => {
    // Deny-by-default do guard server-side: sem verificação não há acesso.
    expect(decideAuthRoute(0)).toBe(AUTH_DENIED);
    expect(decideAuthRoute(500)).toBe(AUTH_DENIED);
  });

  it("raiz redireciona autenticado para /dashboard e não autenticado para /masuk", () => {
    expect(decideRootRedirect(true)).toBe("/dashboard");
    expect(decideRootRedirect(false)).toBe("/masuk");
  });
});
