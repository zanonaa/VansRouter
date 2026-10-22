// Fase 4A — mapa de rotas da SPA espelhando o App Router.
// Caminhos e parâmetros dinâmicos são idênticos aos arquivos page.js; páginas são
// carregadas em lazy para preservar o code-splitting por rota do Next.
import { lazy } from "react";
import { Outlet, useNavigate, useRouteError, isRouteErrorResponse } from "react-router-dom";
import { ThemeProvider } from "@/shared/components";
import { RuntimeI18nProvider } from "@/i18n/RuntimeI18nProvider";
import { DashboardLayout } from "@/shared/components";
import DashboardError from "@/app/(dashboard)/error.js";
import RootError from "./RootError.jsx";
import MasukRoute from "./MasukRoute.jsx";
import MediaKindRoute from "./MediaKindRoute.jsx";
import EndpointRoute from "./EndpointRoute.jsx";

const landing = lazy(() => import("@/app/landing/page.js"));
const login = lazy(() => import("@/app/login/page.js"));
const callback = lazy(() => import("@/app/callback/page.js"));
const dashboardHome = lazy(() => import("@/app/(dashboard)/dashboard/page.js"));
const basicChat = lazy(() => import("@/app/(dashboard)/dashboard/basic-chat/page.js"));
const combos = lazy(() => import("@/app/(dashboard)/dashboard/combos/page.js"));
const consoleLog = lazy(() => import("@/app/(dashboard)/dashboard/console-log/page.js"));
const mediaWeb = lazy(() => import("@/app/(dashboard)/dashboard/media-providers/web/page.js"));
const mediaKindItem = lazy(() => import("@/app/(dashboard)/dashboard/media-providers/[kind]/[id]/page.js"));
const mediaCombo = lazy(() => import("@/app/(dashboard)/dashboard/media-providers/combo/[id]/page.js"));
const profile = lazy(() => import("@/app/(dashboard)/dashboard/profile/page.js"));
const providers = lazy(() => import("@/app/(dashboard)/dashboard/providers/page.js"));
const providersNew = lazy(() => import("@/app/(dashboard)/dashboard/providers/new/page.js"));
const providerItem = lazy(() => import("@/app/(dashboard)/dashboard/providers/[id]/page.js"));
const proxyFitness = lazy(() => import("@/app/(dashboard)/dashboard/proxy-fitness/page.js"));
const proxyPools = lazy(() => import("@/app/(dashboard)/dashboard/proxy-pools/page.js"));
const pxpipe = lazy(() => import("@/app/(dashboard)/dashboard/pxpipe/page.js"));
const quota = lazy(() => import("@/app/(dashboard)/dashboard/quota/page.js"));
const tokenSaver = lazy(() => import("@/app/(dashboard)/dashboard/token-saver/page.js"));
const translator = lazy(() => import("@/app/(dashboard)/dashboard/translator/page.js"));
const usage = lazy(() => import("@/app/(dashboard)/dashboard/usage/page.js"));
const pricing = lazy(() => import("@/app/dashboard/settings/pricing/page.js"));

function RootLayout() {
  return (
    <RuntimeI18nProvider>
      <ThemeProvider>
        <Outlet />
      </ThemeProvider>
    </RuntimeI18nProvider>
  );
}

// O error boundary do grupo (dashboard) espera {error, reset}; reset do App Router
// re-renderiza o segmento — aqui revalidamos a rota (divergência documentada).
function DashboardErrorRoute() {
  const error = useRouteError();
  const navigate = useNavigate();
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
      { index: true, element: <landing /> },
      { path: "login", element: <login /> },
      { path: "masuk", element: <MasukRoute /> },
      { path: "callback", element: <callback /> },
      // /dashboard/settings/pricing fica fora do grupo (dashboard) e, como no
      // App Router, NÃO herda DashboardLayout.
      { path: "dashboard/settings/pricing", element: <pricing /> },
      {
        path: "dashboard",
        element: <DashboardLayout />,
        errorElement: <DashboardErrorRoute />,
        children: [
          { index: true, element: <dashboardHome /> },
          { path: "basic-chat", element: <basicChat /> },
          { path: "combos", element: <combos /> },
          { path: "console-log", element: <consoleLog /> },
          { path: "endpoint", element: <EndpointRoute /> },
          { path: "media-providers/web", element: <mediaWeb /> },
          { path: "media-providers/combo/:id", element: <mediaCombo /> },
          { path: "media-providers/:kind/:id", element: <mediaKindItem /> },
          { path: "media-providers/:kind", element: <MediaKindRoute /> },
          { path: "profile", element: <profile /> },
          { path: "providers/new", element: <providersNew /> },
          { path: "providers/:id", element: <providerItem /> },
          { path: "providers", element: <providers /> },
          { path: "proxy-fitness", element: <proxyFitness /> },
          { path: "proxy-pools", element: <proxyPools /> },
          { path: "pxpipe", element: <pxpipe /> },
          { path: "quota", element: <quota /> },
          { path: "token-saver", element: <tokenSaver /> },
          { path: "translator", element: <translator /> },
          { path: "usage", element: <usage /> },
        ],
      },
    ],
  },
];
