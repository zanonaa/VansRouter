// Guard de páginas da SPA (Fase 4A) — papel do dashboardGuard para navegação.
// A decisão de autenticação vem SEMPRE do backend via /api/auth/check
// (cookie auth_token), que responde 200 autenticado ou 401; nada de sessão
// ou segredo é avaliado no navegador.
export const AUTH_CHECKING = "checking";
export const AUTH_ALLOWED = "allowed";
export const AUTH_DENIED = "denied";

export function decideAuthRoute(checkStatus) {
  if (checkStatus === 200) return AUTH_ALLOWED;
  if (checkStatus === 401 || checkStatus === 403) return AUTH_DENIED;
  // Falha de rede/servidor: sem verificação não há acesso (deny-by-default,
  // mesma postura do guard server-side; o usuário pode recarregar).
  return AUTH_DENIED;
}

export function decideRootRedirect(authenticated) {
  return authenticated ? "/dashboard" : "/masuk";
}

// Página "/" do guard original: nunca renderiza conteúdo, apenas redireciona
// conforme autenticação. A landing vive em /landing, como no App Router.
export async function fetchAuthStatus() {
  try {
    const res = await fetch("/api/auth/check", { headers: { Accept: "application/json" } });
    return res.status;
  } catch {
    return 0;
  }
}
