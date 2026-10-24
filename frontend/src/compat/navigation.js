// Compatibilidade next/navigation sobre react-router (Fase 4A).
// refresh() do Next reexecuta Server Components; na SPA a semântica equivalente
// mais próxima é revalidar a rota atual (documentada como divergência no lote).
import { useLocation, useNavigate, useParams, useSearchParams as useRouterSearchParams } from "react-router-dom";

export { useParams };

// next/navigation: useSearchParams() devolve o URLSearchParams (somente leitura).
// react-router devolve a tupla [params, setter]; expor a tupla fazia as páginas
// migradas quebrarem em runtime ("searchParams.get is not a function" na Usage).
export function useSearchParams() {
  return useRouterSearchParams()[0];
}

export function usePathname() {
  return useLocation().pathname;
}

export function useRouter() {
  const navigate = useNavigate();
  // refresh() do Next reexecuta Server Components da rota atual. Em SPA, a
  // semântica equivalente é revalidar a rota corrente — mas chamadas como
  // push(url) seguido de refresh() precisam ler a URL DEPOIS da navegação
  // pendente, senão o refresh cancelaria o push (bug observado no login).
  const revalidate = () => {
    setTimeout(() => {
      const { pathname, search, hash } = window.location;
      navigate(pathname + search + hash, { replace: true });
    }, 0);
  };
  return {
    push: (url) => navigate(url),
    replace: (url) => navigate(url, { replace: true }),
    back: () => navigate(-1),
    forward: () => navigate(1),
    refresh: revalidate,
    prefetch: () => {},
  };
}

// next/navigation notFound() sinaliza 404 para o error boundary mais próximo.
// O react-router só reconhece ErrorResponse (isRouteErrorResponse exige
// `internal`); lançar uma Response crua durante o render cai no boundary de
// erro genérico — foi o que aconteceu na tela de mídia com id inexistente.
export function notFound() {
  throw { status: 404, statusText: "Not Found", internal: true, data: null };
}
