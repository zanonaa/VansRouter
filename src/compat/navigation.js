// Compatibilidade next/navigation sobre react-router (Fase 4A).
// refresh() do Next reexecuta Server Components; na SPA a semântica equivalente
// mais próxima é revalidar a rota atual (documentada como divergência no lote).
import { useLocation, useNavigate, useParams, useSearchParams } from "react-router-dom";

export { useParams, useSearchParams };

export function usePathname() {
  return useLocation().pathname;
}

export function useRouter() {
  const navigate = useNavigate();
  const location = useLocation();
  return {
    push: (url) => navigate(url),
    replace: (url) => navigate(url, { replace: true }),
    back: () => navigate(-1),
    forward: () => navigate(1),
    refresh: () =>
      navigate(location.pathname + location.search + location.hash, { replace: true }),
    prefetch: () => {},
  };
}

// next/navigation notFound() sinaliza 404 para o error boundary mais próximo;
// no data router do react-router a convenção é lançar uma Response 404.
export function notFound() {
  throw new Response(null, { status: 404, statusText: "Not Found" });
}
