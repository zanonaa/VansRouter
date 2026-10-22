// Error boundary raiz da SPA (papel de app/global-error.js).
// Reutiliza o mesmo visual; reset recarrega a rota (divergência documentada).
import { useEffect } from "react";
import { useLocation, useNavigate, useRouteError, isRouteErrorResponse } from "react-router-dom";

export default function RootError() {
  const error = useRouteError();
  const navigate = useNavigate();
  const location = useLocation();
  useEffect(() => {
    console.error(error);
  }, [error]);

  if (isRouteErrorResponse(error) && error.status === 404) {
    return (
      <main className="flex min-h-screen flex-col items-center justify-center gap-4 bg-bg px-6 text-text-main">
        <h1 className="text-4xl font-bold">404</h1>
        <p className="text-sm text-text-muted">This page could not be found.</p>
        <button
          type="button"
          onClick={() => navigate("/")}
          className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-primary-hover"
        >
          Return Home
        </button>
      </main>
    );
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-bg px-6 py-16 text-text-main">
      <section className="w-full max-w-lg rounded-lg border border-border bg-surface p-6 shadow-soft">
        <div className="mb-4 flex h-11 w-11 items-center justify-center rounded-lg bg-red-500/10 text-red-600 dark:text-red-400">
          <span className="material-symbols-outlined text-[24px]">emergency_home</span>
        </div>
        <h1 className="text-xl font-semibold">VansAI could not load</h1>
        <p className="mt-2 text-sm leading-6 text-text-muted">
          A critical error occurred while rendering the application. Reload the page to try again.
        </p>
        {error?.message ? (
          <pre className="mt-4 max-h-36 overflow-auto rounded-md border border-border bg-surface-2 p-3 text-xs text-text-muted">
            {error.message}
          </pre>
        ) : null}
        <div className="mt-6 flex flex-wrap gap-3">
          <button
            type="button"
            onClick={() => navigate(location.pathname + location.search, { replace: true })}
            className="inline-flex items-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-primary-hover"
          >
            <span className="material-symbols-outlined text-[18px]">refresh</span>
            Try again
          </button>
          <button
            type="button"
            onClick={() => navigate("/")}
            className="inline-flex items-center gap-2 rounded-md border border-border px-4 py-2 text-sm font-medium text-text-main transition-colors hover:bg-surface-2"
          >
            <span className="material-symbols-outlined text-[18px]">home</span>
            Back to home
          </button>
        </div>
      </section>
    </main>
  );
}
