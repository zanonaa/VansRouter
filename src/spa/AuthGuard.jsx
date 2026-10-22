// Guard de navegação da SPA: protege /dashboard* e decide redirects de /, /login
// e /masuk com a mesma semântica do dashboardGuard original (decisão no backend).
import { useEffect, useState } from "react";
import { Navigate, useLocation } from "react-router-dom";
import { AUTH_ALLOWED, AUTH_CHECKING, AUTH_DENIED, decideAuthRoute, fetchAuthStatus } from "./authGuard.js";

function AuthCheckingScreen() {
  return (
    <main className="flex min-h-screen items-center justify-center bg-bg text-text-muted">
      <span className="material-symbols-outlined animate-spin text-[28px]">progress_activity</span>
    </main>
  );
}

export function AuthGuard({ children }) {
  const [state, setState] = useState(AUTH_CHECKING);
  const location = useLocation();
  useEffect(() => {
    let active = true;
    fetchAuthStatus().then((status) => {
      if (active) setState(decideAuthRoute(status));
    });
    return () => {
      active = false;
    };
  }, [location.pathname]);
  if (state === AUTH_CHECKING) return <AuthCheckingScreen />;
  if (state === AUTH_ALLOWED) return children;
  return <Navigate to="/masuk" replace />;
}

// "/" nunca renderiza: redireciona para /dashboard autenticado, senão /masuk.
export function RootRedirect() {
  const [state, setState] = useState(AUTH_CHECKING);
  useEffect(() => {
    let active = true;
    fetchAuthStatus().then((status) => {
      if (active) setState(decideAuthRoute(status));
    });
    return () => {
      active = false;
    };
  }, []);
  if (state === AUTH_CHECKING) return <AuthCheckingScreen />;
  return <Navigate to={state === AUTH_ALLOWED ? "/dashboard" : "/masuk"} replace />;
}
