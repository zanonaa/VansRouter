// /masuk sem SSR: consulta /api/auth/status e repassa initialAuth ao MasukClient,
// com estado de carregamento (hasPassword null) e fallback silencioso — mesma
// semântica do try/catch do page.js original.
import { useEffect, useState } from "react";
import MasukClient from "@/app/masuk/MasukClient";

const FALLBACK = {
  hasPassword: null,
  authMode: "password",
  oidcConfigured: false,
  oidcLoginLabel: "Masuk dengan OIDC",
  requireLogin: true,
};

export default function MasukRoute() {
  const [initialAuth, setInitialAuth] = useState(FALLBACK);

  useEffect(() => {
    let active = true;
    fetch("/api/auth/status", { headers: { Accept: "application/json" } })
      .then((res) => (res.ok ? res.json() : null))
      .then((data) => {
        if (!active || !data) return;
        setInitialAuth({
          hasPassword: data.hasPassword === true,
          authMode: data.authMode || "password",
          oidcConfigured: data.oidcConfigured === true,
          oidcLoginLabel: (data.oidcLoginLabel || "").trim() || "Sign in with OIDC",
          requireLogin: data.requireLogin !== false,
        });
      })
      .catch(() => {});
    return () => {
      active = false;
    };
  }, []);

  return <MasukClient initialAuth={initialAuth} />;
}
