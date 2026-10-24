// /masuk sem SSR: consulta a autenticação e o status de login.
// Autenticado → /dashboard (mesma regra do dashboardGuard); caso contrário
// carrega /api/auth/status e repassa initialAuth ao MasukClient com estado
// de carregamento, na semântica do try/catch do page.js original.
import { useEffect, useState } from "react";
import { Navigate } from "react-router-dom";
import MasukClient from "@/app/masuk/MasukClient";
import { AUTH_ALLOWED, AUTH_CHECKING, AUTH_DENIED, decideAuthRoute, fetchAuthStatus } from "./authGuard.js";

const FALLBACK = {
  hasPassword: null,
  authMode: "password",
  oidcConfigured: false,
  oidcLoginLabel: "Masuk dengan OIDC",
  requireLogin: true,
};

export default function MasukRoute() {
  const [auth, setAuth] = useState(AUTH_CHECKING);
  const [initialAuth, setInitialAuth] = useState(FALLBACK);

  useEffect(() => {
    let active = true;
    fetchAuthStatus().then(async (status) => {
      if (!active) return;
      const decision = decideAuthRoute(status);
      setAuth(decision);
      if (decision !== AUTH_DENIED) return;
      try {
        const res = await fetch("/api/auth/status", { headers: { Accept: "application/json" } });
        if (!res.ok) return;
        const data = await res.json();
        if (!active || !data) return;
        setInitialAuth({
          hasPassword: data.hasPassword === true,
          authMode: data.authMode || "password",
          oidcConfigured: data.oidcConfigured === true,
          oidcLoginLabel: (data.oidcLoginLabel || "").trim() || "Sign in with OIDC",
          requireLogin: data.requireLogin !== false,
        });
      } catch {}
    });
    return () => {
      active = false;
    };
  }, []);

  if (auth === AUTH_CHECKING) {
    return (
      <main className="flex min-h-screen items-center justify-center bg-bg text-text-muted">
        <span className="material-symbols-outlined animate-spin text-[28px]">progress_activity</span>
      </main>
    );
  }
  if (auth === AUTH_ALLOWED) return <Navigate to="/dashboard" replace />;
  return <MasukClient initialAuth={initialAuth} />;
}
