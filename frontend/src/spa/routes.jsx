// Fase 4A — mapa de rotas da SPA espelhando o App Router.
// Caminhos e parâmetros dinâmicos são idênticos aos arquivos page.js; páginas são
// carregadas em lazy para preservar o code-splitting por rota do Next.
import { lazy, Suspense } from "react";
import { Outlet, useNavigate, useRouteError, isRouteErrorResponse, Navigate } from "react-router-dom";
import { ThemeProvider } from "@/shared/components";
import { RuntimeI18nProvider } from "@/i18n/RuntimeI18nProvider";
import { DashboardLayout } from "@/shared/components";
import DashboardError from "@/app/(dashboard)/error.js";
import RootError from "./RootError.jsx";
import MasukRoute from "./MasukRoute.jsx";
import MediaKindRoute from "./MediaKindRoute.jsx";
import EndpointRoute from "./EndpointRoute.jsx";
import { AuthGuard, RootRedirect } from "./AuthGuard.jsx";

const LandingPage = lazy(() => import("@/app/landing/page.js"));
const CallbackPage = lazy(() => import("@/app/callback/page.js"));
const DashboardHomePage = lazy(() => import("@/app/(dashboard)/dashboard/page.js"));
const BasicChatPage = lazy(() => import("@/app/(dashboard)/dashboard/basic-chat/page.js"));
const CombosPage = lazy(() => import("@/app/(dashboard)/dashboard/combos/page.js"));
const ConsoleLogPage = lazy(() => import("@/app/(dashboard)/dashboard/console-log/page.js"));
const MediaWebPage = lazy(() => import("@/app/(dashboard)/dashboard/media-providers/web/page.js"));
const MediaKindItemPage = lazy(() => import("@/app/(dashboard)/dashboard/media-providers/[kind]/[id]/page.js"));
const MediaComboPage = lazy(() => import("@/app/(dashboard)/dashboard/media-providers/combo/[id]/page.js"));
const ProfilePage = lazy(() => import("@/app/(dashboard)/dashboard/profile/page.js"));
const ProvidersPage = lazy(() => import("@/app/(dashboard)/dashboard/providers/page.js"));
const ProvidersNewPage = lazy(() => import("@/app/(dashboard)/dashboard/providers/new/page.js"));
const ProviderItemPage = lazy(() => import("@/app/(dashboard)/dashboard/providers/[id]/page.js"));
const ProxyFitnessPage = lazy(() => import("@/app/(dashboard)/dashboard/proxy-fitness/page.js"));
const ProxyPoolsPage = lazy(() => import("@/app/(dashboard)/dashboard/proxy-pools/page.js"));
const PxpipePage = lazy(() => import("@/app/(dashboard)/dashboard/pxpipe/page.js"));
const QuotaPage = lazy(() => import("@/app/(dashboard)/dashboard/quota/page.js"));
const TokenSaverPage = lazy(() => import("@/app/(dashboard)/dashboard/token-saver/page.js"));
const TranslatorPage = lazy(() => import("@/app/(dashboard)/dashboard/translator/page.js"));
const UsagePage = lazy(() => import("@/app/(dashboard)/dashboard/usage/page.js"));
const PricingPage = lazy(() => import("@/app/dashboard/settings/pricing/page.js"));

function RootLayout() {
  return (
    <RuntimeI18nProvider>
      <ThemeProvider>
        {/* As páginas de rota são React.lazy; sem um boundary o primeiro
            acesso a uma rota lazy suspende a raiz e a tela fica em branco. */}
        <Suspense fallback={null}>
          <Outlet />
        </Suspense>
      </ThemeProvider>
    </RuntimeI18nProvider>
  );
}

// O error boundary do grupo (dashboard) espera {error, reset}; reset do App Router
// re-renderiza o segmento — aqui revalidamos a rota (divergência documentada).
function DashboardErrorRoute() {
  const error = useRouteError();
  const navigate = useNavigate();
  // notFound() do App Router vira uma Response 404; o Next renderiza a página
  // 404, não o boundary de erro do dashboard (entidade inexistente, ex. mídia).
  if (isRouteErrorResponse(error) && error.status === 404) {
    return <RootError />;
  }
  return (
    <DashboardError
      error={error}
      reset={() => navigate(window.location.pathname + window.location.search, { replace: true })}
    />
  );
}

export const routes = [
  {
    path: "/",
    element: <RootLayout />,
    errorElement: <RootError />,
    children: [
      // "/" do guard original: apenas redireciona (autenticado → /dashboard,
      // senão /masuk). A landing continua em /landing, como no App Router.
      { index: true, element: <RootRedirect /> },
      { path: "landing", element: <LandingPage /> },
      // O guard original redireciona /login → /masuk sem renderizar a página.
      { path: "login", element: <Navigate to="/masuk" replace /> },
      { path: "masuk", element: <MasukRoute /> },
      { path: "callback", element: <CallbackPage /> },
      // /dashboard/settings/pricing fica fora do grupo (dashboard) e, como no
      // App Router, NÃO herda DashboardLayout.
      { path: "dashboard/settings/pricing", element: <PricingPage /> },
      {
        path: "dashboard",
        element: (
          <AuthGuard>
            {/* DashboardLayout é o layout do App Router e renderiza
                {children}: a rota precisa passar o <Outlet/> explicitamente,
                senão o miolo de todas as telas do dashboard fica vazio. */}
            <DashboardLayout>
              <Suspense fallback={null}>
                <Outlet />
              </Suspense>
            </DashboardLayout>
          </AuthGuard>
        ),
        errorElement: <DashboardErrorRoute />,
        children: [
          { index: true, element: <DashboardHomePage /> },
          { path: "basic-chat", element: <BasicChatPage /> },
          { path: "combos", element: <CombosPage /> },
          { path: "console-log", element: <ConsoleLogPage /> },
          { path: "endpoint", element: <EndpointRoute /> },
          { path: "media-providers/web", element: <MediaWebPage /> },
          { path: "media-providers/combo/:id", element: <MediaComboPage /> },
          { path: "media-providers/:kind/:id", element: <MediaKindItemPage /> },
          { path: "media-providers/:kind", element: <MediaKindRoute /> },
          { path: "profile", element: <ProfilePage /> },
          { path: "providers/new", element: <ProvidersNewPage /> },
          { path: "providers/:id", element: <ProviderItemPage /> },
          { path: "providers", element: <ProvidersPage /> },
          { path: "proxy-fitness", element: <ProxyFitnessPage /> },
          { path: "proxy-pools", element: <ProxyPoolsPage /> },
          { path: "pxpipe", element: <PxpipePage /> },
          { path: "quota", element: <QuotaPage /> },
          { path: "token-saver", element: <TokenSaverPage /> },
          { path: "translator", element: <TranslatorPage /> },
          { path: "usage", element: <UsagePage /> },
        ],
      },
    ],
  },
];
