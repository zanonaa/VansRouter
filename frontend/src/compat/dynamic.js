// Compatibilidade next/dynamic (Fase 4A): lazy + Suspense com fallback.
import { lazy, Suspense } from "react";

export default function dynamic(loader, options = {}) {
  const LazyComponent = lazy(() =>
    loader().then((mod) => ({ default: mod.default ?? mod }))
  );
  const fallback =
    typeof options.loading === "function" ? options.loading() : options.loading;
  function DynamicWrapper(props) {
    return (
      <Suspense fallback={fallback ?? null}>
        <LazyComponent {...props} />
      </Suspense>
    );
  }
  return DynamicWrapper;
}
