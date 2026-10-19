"use client";

import { useState, useEffect, useMemo, useCallback, useRef } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Table from "@mui/material/Table";
import TableHead from "@mui/material/TableHead";
import TableBody from "@mui/material/TableBody";
import TableRow from "@mui/material/TableRow";
import TableCell from "@mui/material/TableCell";
import { FREE_PROVIDERS, AI_PROVIDERS } from "@/shared/constants/providers";

// Keep providers without serviceKinds (default LLM) or with "llm" in serviceKinds
function isLLMProvider(id) {
  const p = AI_PROVIDERS[id];
  if (!p?.serviceKinds) return true;
  return p.serviceKinds.includes("llm");
}
import Badge from "./Badge";
import Card from "./Card";
import OverviewCards from "@/app/(dashboard)/dashboard/usage/components/OverviewCards";
import UsageTable, { fmt, fmtTime } from "@/app/(dashboard)/dashboard/usage/components/UsageTable";
import dynamic from "next/dynamic";

// Panel-sized placeholder for a single topology column, shared with
// topologySkeleton so the two cannot drift (see its comment below).
const topologyPanelSx = {
  height: { xs: 320, sm: 480 },
  width: "100%",
  animation: "pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite",
  borderRadius: "8px",
  border: "1px solid var(--color-border)",
  bgcolor: "rgba(244,244,245,0.5)",
};
const topologyPanelSkeleton = <Box sx={topologyPanelSx} aria-hidden="true" />;

// ponytail: two independent guards, both needed. `loading:` (below) keeps the
// skeleton up for the whole chunk fetch, so the area never collapses mid-fetch;
// the parent gates keep ~721KB of recharts + reactflow off the critical path so
// the page is interactive first. Drop either one and you regress one half:
// without `loading:` the chart area goes 0px mid-fetch, without a gate the
// heavy chunks land ~300ms earlier and TBT climbs back to ~3.5s.
//
// The two gates are distinct on purpose:
//   idleReadyForTopology — requestIdleCallback. The reactflow topology sits at
//     y≈718, partly inside the 823px viewport, so a visibility gate would fire
//     immediately and change nothing. Idle already keeps its 229KB off first paint.
//   chartsInView — IntersectionObserver on the first recharts chart (y≈1624,
//     far below the fold). Idle fires almost immediately when the main thread
//     frees up, which does NOT keep the 482KB of recharts off the critical path;
//     visibility does, because it cannot fire until the user scrolls.

const ProviderTopology = dynamic(() => import("@/app/(dashboard)/dashboard/usage/components/ProviderTopology"), { ssr: false, loading: () => topologyPanelSkeleton });
const UsageChart = dynamic(() => import("@/app/(dashboard)/dashboard/usage/components/UsageChart"), { ssr: false, loading: () => chartSkeleton });
const ProviderBarChart = dynamic(() => import("@/app/(dashboard)/dashboard/usage/components/ProviderBarChart"), { ssr: false, loading: () => chartSkeleton });
const TopModelsChart = dynamic(() => import("@/app/(dashboard)/dashboard/usage/components/TopModelsChart"), { ssr: false, loading: () => chartSkeleton });

// Skeleton placeholders sized to match the final content so the layout does
// not shift (CLS) when data arrives. Keep dimensions in sync with the real
// components they replace.
const overviewSkeleton = (
  <Box sx={{ display: "grid", minWidth: 0, gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr", md: "repeat(4, 1fr)" }, gap: { xs: 1.5, sm: 2 } }}>
    {Array.from({ length: 4 }).map((_, i) => (
      <Box key={i} sx={{ height: 96, width: "100%", animation: "pulse 2s cubic-bezier(0.4,0,0.6,1) infinite", borderRadius: "8px", border: "1px solid var(--color-border)", bgcolor: "rgba(244,244,245,0.5)" }} aria-hidden="true" />
    ))}
  </Box>
);

const topologySkeleton = (
  <Box sx={{ display: "grid", minWidth: 0, gridTemplateColumns: { xs: "1fr", lg: "minmax(0,2fr) minmax(280px,1fr)" }, gap: 1 }}>
    {topologyPanelSkeleton}
    {topologyPanelSkeleton}
  </Box>
);

// Must match the vertical footprint of UsageChart's Card (p-3/p-4 + toggle row
// + gap-3 + 220px ResponsiveContainer) so the chartsInView gate swap does not
// shift the table below. ProviderBarChart/TopModelsChart are shorter (~252px) but
// share this placeholder, so their skeletons are 300px tall while loading.
const chartSkeleton = (
  <Box sx={{ minHeight: 300, width: "100%", animation: "pulse 2s cubic-bezier(0.4,0,0.6,1) infinite", borderRadius: "8px", border: "1px solid var(--color-border)", bgcolor: "rgba(244,244,245,0.5)" }} aria-hidden="true" />
);

const tableSkeleton = (
  <Box sx={{ height: 256, width: "100%", animation: "pulse 2s cubic-bezier(0.4,0,0.6,1) infinite", borderRadius: "8px", border: "1px solid var(--color-border)", bgcolor: "rgba(244,244,245,0.5)" }} aria-hidden="true" />
);

function timeAgo(timestamp) {
  const diff = Math.floor((Date.now() - new Date(timestamp)) / 1000);
  if (diff < 60) return `${diff}s ago`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return `${Math.floor(diff / 86400)}d ago`;
}

// Auto-update time display every second without re-rendering parent
function TimeAgo({ timestamp }) {
  const [, setTick] = useState(0);
  
  useEffect(() => {
    const timer = setInterval(() => setTick(t => t + 1), 1000);
    return () => clearInterval(timer);
  }, []);
  
  return <>{timeAgo(timestamp)}</>;
}

const EMPTY_REQUESTS = [];

// Fires once when the element first intersects the viewport (plus rootMargin),
// then disconnects — a below-fold element only ever needs the one transition.
// ponytail: no re-arm on scroll-out; if an "unload when hidden" need appears,
// keep the observer and toggle instead.
function useInView(rootMargin = "200px") {
  const ref = useRef(null);
  const [inView, setInView] = useState(false);
  useEffect(() => {
    const el = ref.current;
    if (!el || typeof IntersectionObserver === "undefined") {
      setInView(true);
      return;
    }
    const io = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          setInView(true);
          io.disconnect();
        }
      },
      { rootMargin },
    );
    io.observe(el);
    return () => io.disconnect();
  }, [rootMargin]);
  return [ref, inView];
}

function RecentRequests({ requests = EMPTY_REQUESTS }) {
  return (
    <Card padding="sm" sx={{ display: "flex", minWidth: 0, flexDirection: "column", overflow: "hidden", height: 480 }}>
      {/* Header */}
      <Box sx={{ px: 0.5, py: 1, borderBottom: "1px solid var(--color-border)", flexShrink: 0 }}>
        <Box component="span" sx={{ fontSize: 12, fontWeight: 600, color: "text.secondary", textTransform: "uppercase", letterSpacing: "0.05em" }}>Recent Requests</Box>
      </Box>

      {!requests.length ? (
        <Box sx={{ flex: 1, display: "flex", alignItems: "center", justifyContent: "center", color: "text.secondary", fontSize: 14 }}>No requests yet.</Box>
      ) : (
        <Box sx={{ flex: 1, overflowY: "auto" }}>
          <Table sx={{ width: "100%", minWidth: 300, fontSize: 12 }}>
            <TableHead sx={{ position: "sticky", top: 0, bgcolor: "background.default", zIndex: 10 }}>
              <TableRow>
                <TableCell sx={{ py: 0.75, width: 8, borderBottom: "1px solid var(--color-border)" }}><span style={{ position: "absolute", width: 1, height: 1, padding: 0, margin: -1, overflow: "hidden", clip: "rect(0,0,0,0)", whiteSpace: "nowrap", border: 0 }}>Status</span></TableCell>
                <TableCell sx={{ py: 0.75, textAlign: "left", fontWeight: 600, color: "text.secondary", borderBottom: "1px solid var(--color-border)" }}>Model</TableCell>
                <TableCell sx={{ py: 0.75, textAlign: "right", fontWeight: 600, color: "text.secondary", whiteSpace: "nowrap", borderBottom: "1px solid var(--color-border)" }}>In / Out</TableCell>
                <TableCell sx={{ py: 0.75, textAlign: "right", fontWeight: 600, color: "text.secondary", borderBottom: "1px solid var(--color-border)" }}>When</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {requests.map((r, i) => {
                const ok = !r.status || r.status === "ok" || r.status === "success";
                return (
                  <TableRow key={`${r.timestamp}-${r.model}-${i}`} hover>
                    <TableCell sx={{ py: 0.75 }}>
                      <Box component="span" aria-label={ok ? "Success" : "Error"} sx={{ display: "block", width: 6, height: 6, borderRadius: "50%", bgcolor: ok ? "success.main" : "error.main" }} />
                    </TableCell>
                    <TableCell sx={{ py: 0.75, fontFamily: "ui-monospace, monospace", maxWidth: 120, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={r.model}>{r.model}</TableCell>
                    <TableCell sx={{ py: 0.75, textAlign: "right", whiteSpace: "nowrap" }}>
                      <Box component="span" sx={{ color: "primary.main" }}>{fmt(r.promptTokens)}↑</Box>
                      {" "}
                      <Box component="span" sx={{ color: "success.main" }}>{fmt(r.completionTokens)}↓</Box>
                    </TableCell>
                    <TableCell sx={{ py: 0.75, textAlign: "right", color: "text.secondary", whiteSpace: "nowrap" }}><TimeAgo timestamp={r.timestamp} /></TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </Box>
      )}
    </Card>
  );
}

function sortData(dataMap, pendingMap = {}, sortBy, sortOrder) {
  return Object.entries(dataMap || {})
    .map(([key, data]) => {
      const totalTokens = (data.promptTokens || 0) + (data.completionTokens || 0);
      const totalCost = data.cost || 0;
      // ponytail: cost split is a token-share allocation of the (rate-accurate)
      // server total, not a per-rate recompute. cached is a subset of prompt, so
      // peel it out of the input share. Upgrade to a stored per-component cost
      // breakdown if exact cached-rate cost display is needed.
      const cachedTokens = data.cachedTokens || 0;
      const nonCachedInput = Math.max(0, (data.promptTokens || 0) - cachedTokens);
      const inputCost = totalTokens > 0 ? nonCachedInput * (totalCost / totalTokens) : 0;
      const cachedCost = totalTokens > 0 ? cachedTokens * (totalCost / totalTokens) : 0;
      const outputCost = totalTokens > 0 ? (data.completionTokens || 0) * (totalCost / totalTokens) : 0;
      return { ...data, key, totalTokens, totalCost, inputCost, cachedCost, outputCost, pending: pendingMap[key] || 0 };
    })
    .sort((a, b) => {
      let valA = a[sortBy];
      let valB = b[sortBy];
      if (typeof valA === "string") valA = valA.toLowerCase();
      if (typeof valB === "string") valB = valB.toLowerCase();
      if (valA < valB) return sortOrder === "asc" ? -1 : 1;
      if (valA > valB) return sortOrder === "asc" ? 1 : -1;
      return 0;
    });
}

function getGroupKey(item, keyField) {
  switch (keyField) {
    case "rawModel": return item.rawModel || "Unknown Model";
    case "accountName": return item.accountName || `Account ${item.connectionId?.slice(0, 8)}...` || "Unknown Account";
    case "keyName": return item.keyName || "Unknown Key";
    case "endpoint": return item.endpoint || "Unknown Endpoint";
    default: return item[keyField] || "Unknown";
  }
}

function groupDataByKey(data, keyField) {
  if (!Array.isArray(data)) return [];
  const groups = {};
  data.forEach((item) => {
    const gk = getGroupKey(item, keyField);
    if (!groups[gk]) {
      groups[gk] = {
        groupKey: gk,
        summary: {
          requests: 0, promptTokens: 0, completionTokens: 0, cachedTokens: 0,
          totalTokens: 0, cost: 0, inputCost: 0, cachedCost: 0, outputCost: 0,
          lastUsed: null, pending: 0,
          provider: null, rawModel: null, keyName: null, endpoint: null
        },
        items: [],
      };
    }
    const s = groups[gk].summary;
    s.requests += item.requests || 0;
    s.promptTokens += item.promptTokens || 0;
    s.completionTokens += item.completionTokens || 0;
    s.cachedTokens += item.cachedTokens || 0;
    s.totalTokens += item.totalTokens || 0;
    s.cost += item.cost || 0;
    s.inputCost += item.inputCost || 0;
    s.cachedCost += item.cachedCost || 0;
    s.outputCost += item.outputCost || 0;
    s.pending += item.pending || 0;
    if (item.lastUsed && (!s.lastUsed || new Date(item.lastUsed) > new Date(s.lastUsed))) {
      s.lastUsed = item.lastUsed;
    }

    const trackUnique = (field) => {
      if (s[field] === null) {
        s[field] = item[field] || undefined;
      } else if (s[field] !== undefined && s[field] !== item[field]) {
        s[field] = undefined;
      }
    };
    trackUnique("provider");
    trackUnique("rawModel");
    trackUnique("keyName");
    trackUnique("endpoint");

    groups[gk].items.push(item);
  });
  return Object.values(groups);
}

const MODEL_COLUMNS = [
  { field: "rawModel", label: "Model" },
  { field: "provider", label: "Provider" },
  { field: "requests", label: "Requests", align: "right" },
  { field: "lastUsed", label: "Last Used", align: "right" },
];

const ACCOUNT_COLUMNS = [
  { field: "accountName", label: "Account" },
  { field: "rawModel", label: "Model" },
  { field: "provider", label: "Provider" },
  { field: "requests", label: "Requests", align: "right" },
  { field: "lastUsed", label: "Last Used", align: "right" },
];

const API_KEY_COLUMNS = [
  { field: "keyName", label: "API Key Name" },
  { field: "rawModel", label: "Model" },
  { field: "provider", label: "Provider" },
  { field: "requests", label: "Requests", align: "right" },
  { field: "lastUsed", label: "Last Used", align: "right" },
];

const ENDPOINT_COLUMNS = [
  { field: "endpoint", label: "Endpoint" },
  { field: "rawModel", label: "Model" },
  { field: "provider", label: "Provider" },
  { field: "requests", label: "Requests", align: "right" },
  { field: "lastUsed", label: "Last Used", align: "right" },
];

const TABLE_OPTIONS = [
  { value: "model", label: "Usage by Model" },
  { value: "account", label: "Usage by Account" },
  { value: "apiKey", label: "Usage by API Key" },
  { value: "endpoint", label: "Usage by Endpoint" },
];

const PERIODS = [
  { value: "today", label: "Today" },
  { value: "24h", label: "24h" },
  { value: "7d", label: "7D" },
  { value: "30d", label: "30D" },
  { value: "60d", label: "60D" },
  { value: "all", label: "All" },
];

const periodBtnSx = (active) => ({
  borderRadius: "6px",
  px: 1.5,
  py: 0.5,
  fontSize: 14,
  fontWeight: 500,
  transition: "background-color 150ms, color 150ms",
  border: 0,
  cursor: "pointer",
  bgcolor: active ? "primary.main" : "transparent",
  color: active ? "#fff" : "text.secondary",
  boxShadow: active ? "0 1px 2px rgba(0,0,0,0.05)" : "none",
  "&:hover": { bgcolor: active ? "primary.main" : "var(--color-surface-3)", color: active ? "#fff" : "text.primary" },
});

function PeriodSelector({ period, setPeriod, fetching }) {
  return (
    <Box sx={{ display: "flex", width: "100%", alignItems: "center", gap: 1, flexDirection: { xs: "row", sm: "row" }, alignSelf: { sm: "flex-end" }, width: { sm: "auto" } }}>
      <Box sx={{ display: "grid", flex: 1, gridTemplateColumns: "repeat(6, 1fr)", alignItems: "center", gap: 0.5, borderRadius: "8px", border: "1px solid var(--color-border)", bgcolor: "var(--color-surface-2)", p: 0.5, ...({ "@media (min-width:600px)": { display: "flex", flex: "none" } }) }}>
        {PERIODS.map((p) => (
          <Box
            component="button"
            type="button"
            key={p.value}
            onClick={() => setPeriod(p.value)}
            disabled={fetching}
            sx={periodBtnSx(period === p.value)}
          >
            {p.label}
          </Box>
        ))}
      </Box>
      {fetching && (
        <span className="material-symbols-outlined" style={{ animation: "spin 1s linear infinite",  fontSize: 16, color: "var(--color-text-muted)" }}>progress_activity</span>
      )}
    </Box>
  );
}


export default function UsageStats({ period: periodProp, setPeriod: setPeriodProp, hidePeriodSelector = false } = {}) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const sortBy = searchParams.get("sortBy") || "rawModel";
  const sortOrder = searchParams.get("sortOrder") || "asc";
  const [stats, setStats] = useState(null);
  const [loadState, setLoadState] = useState({ loading: true, fetching: false });
  const loading = loadState.loading;
  const fetching = loadState.fetching;
  const [tableView, setTableView] = useState("model");
  const [viewMode, setViewMode] = useState("costs");
  const [providers, setProviders] = useState([]);
  const [idleReadyForTopology, setIdleReadyForTopology] = useState(false);
  const [chartsRef, chartsInView] = useInView("200px");
  const [periodLocal, setPeriodLocal] = useState("today");
  const isInitialLoad = useRef(true);
  const hasLoadedStats = useRef(false);
  const period = periodProp ?? periodLocal;
  const setPeriod = setPeriodProp ?? setPeriodLocal;
  // Hold the reactflow topology (~229KB) until the page is idle, so the first
  // input is not competing with its parse. It sits partly in view, so this is
  // the only gate that can help it — see the note above the dynamic() imports.
  useEffect(() => {
    if (typeof window === "undefined") return;
    if ("requestIdleCallback" in window) {
      const id = window.requestIdleCallback(() => setIdleReadyForTopology(true), { timeout: 2000 });
      return () => window.cancelIdleCallback(id);
    }
    const timer = setTimeout(() => setIdleReadyForTopology(true), 200);
    return () => clearTimeout(timer);
  }, []);
  // Fetch connected providers once, deduplicate by provider type
  // Always include noAuth free providers (e.g. opencode) regardless of connections
  useEffect(() => {
    const controller = new AbortController();
    Promise.all([
      fetch("/api/providers", { signal: controller.signal }).then((r) => r.ok ? r.json() : null),
      fetch("/api/provider-nodes", { signal: controller.signal }).then((r) => r.ok ? r.json() : null),
    ])
      .then(([d, nodesData]) => {
        if (controller.signal.aborted) return;
        // Build node name lookup for custom providers
        const nodeNameMap = {};
        for (const node of (nodesData?.nodes || [])) {
          nodeNameMap[node.id] = node.name;
        }
        const seen = new Set();
        const unique = (d?.connections || []).reduce((acc, c) => {
          if (c.isActive === false || !isLLMProvider(c.provider) || AI_PROVIDERS[c.provider]?.hidden || seen.has(c.provider)) return acc;
          seen.add(c.provider);
          acc.push({ ...c, nodeName: nodeNameMap[c.provider] || null });
          return acc;
        }, []);
        const noAuthProviders = Object.values(FREE_PROVIDERS).reduce((acc, p) => {
          if (p.noAuth && !p.hidden && !seen.has(p.id) && isLLMProvider(p.id)) acc.push({ provider: p.id, name: p.name });
          return acc;
        }, []);
        setProviders([...unique, ...noAuthProviders]);
      })
      .catch(() => {});
    return () => controller.abort();
  }, []);
  // Fetch filtered stats via REST when period changes
  useEffect(() => {
    // First load: show full spinner; subsequent: show subtle fetching indicator
    if (isInitialLoad.current) {
      isInitialLoad.current = false;
      setLoadState({ loading: true, fetching: false });
    } else {
      setLoadState({ loading: false, fetching: true });
    }

    const controller = new AbortController();
    fetch(`/api/usage/stats?period=${period}`, { signal: controller.signal })
      .then((r) => r.ok ? r.json() : null)
      .then((data) => {
        if (controller.signal.aborted) return;
        if (data) {
          hasLoadedStats.current = true;
          setStats((prev) => ({ ...prev, ...data }));
        }
      })
      .catch(() => {})
      .finally(() => {
        if (!controller.signal.aborted) setLoadState({ loading: false, fetching: false });
      });
    return () => controller.abort();
  }, [period]);
  // SSE connection - real-time updates for activeRequests + recentRequests only
  useEffect(() => {
    const es = new EventSource("/api/usage/stream");

    es.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data);
        // Always merge only real-time fields, never overwrite full stats from REST
        setStats((prev) => {
          if (!prev) return prev;
          return {
            ...prev,
            activeRequests: data.activeRequests,
            recentRequests: data.recentRequests,
            errorProvider: data.errorProvider,
            pending: data.pending,
          };
        });
        if (hasLoadedStats.current) setLoadState(prev => ({ ...prev, loading: false }));
      } catch (err) {
        console.error("[SSE CLIENT] parse error:", err);
      }
    };

    es.onerror = () => setLoadState(prev => ({ ...prev, loading: false }));

    return () => es.close();
  }, []);

  const toggleSort = useCallback((tableType, field) => {
    const params = new URLSearchParams(searchParams.toString());
    if (params.get("sortBy") === field) {
      params.set("sortOrder", params.get("sortOrder") === "asc" ? "desc" : "asc");
    } else {
      params.set("sortBy", field);
      params.set("sortOrder", "asc");
    }
    router.replace(`?${params.toString()}`, { scroll: false });
  }, [searchParams, router]);

  // Compute active table data
  const activeTableConfig = useMemo(() => {
    if (!stats) return null;
    switch (tableView) {
      case "model": {
        const pendingMap = stats.pending?.byModel || {};
        return {
          columns: MODEL_COLUMNS,
          groupedData: groupDataByKey(sortData(stats.byModel, pendingMap, sortBy, sortOrder), "rawModel"),
          storageKey: "usage-stats:expanded-models",
          emptyMessage: "No usage recorded yet.",
          renderSummaryCells: (group) => (
            <>
              <TableCell sx={{ px: 3, py: 1.5 }}>
                {group.summary.provider ? (
                  <Badge variant={group.summary.pending > 0 ? "primary" : "neutral"} size="sm">{group.summary.provider}</Badge>
                ) : (
                  <Box component="span" sx={{ color: "text.secondary" }}>—</Box>
                )}
              </TableCell>
              <TableCell sx={{ px: 3, py: 1.5, textAlign: "right" }}>{fmt(group.summary.requests)}</TableCell>
              <TableCell sx={{ px: 3, py: 1.5, textAlign: "right", color: "text.secondary", whiteSpace: "nowrap" }}>{fmtTime(group.summary.lastUsed)}</TableCell>
            </>
          ),
          renderDetailCells: (item) => (
            <>
              <TableCell sx={{ px: 3, py: 1.5, fontWeight: 500, transition: "color 150ms", color: item.pending > 0 ? "primary.main" : undefined }}>{item.rawModel}</TableCell>
              <TableCell sx={{ px: 3, py: 1.5 }}><Badge variant={item.pending > 0 ? "primary" : "neutral"} size="sm">{item.provider}</Badge></TableCell>
              <TableCell sx={{ px: 3, py: 1.5, textAlign: "right" }}>{fmt(item.requests)}</TableCell>
              <TableCell sx={{ px: 3, py: 1.5, textAlign: "right", color: "text.secondary", whiteSpace: "nowrap" }}>{fmtTime(item.lastUsed)}</TableCell>
            </>
          ),
        };
      }
      case "account": {
        const pendingMap = {};
        if (stats?.pending?.byAccount) {
          Object.entries(stats.byAccount || {}).forEach(([accountKey, data]) => {
            const connPending = stats.pending.byAccount[data.connectionId];
            if (connPending) {
              const modelKey = data.provider ? `${data.rawModel} (${data.provider})` : data.rawModel;
              pendingMap[accountKey] = connPending[modelKey] || 0;
            }
          });
        }
        return {
          columns: ACCOUNT_COLUMNS,
          groupedData: groupDataByKey(sortData(stats.byAccount, pendingMap, sortBy, sortOrder), "accountName"),
          storageKey: "usage-stats:expanded-accounts",
          emptyMessage: "No account-specific usage recorded yet.",
          renderSummaryCells: (group) => (
            <>
              <TableCell sx={{ px: 3, py: 1.5 }}>
                {group.summary.rawModel ? (
                  <Box component="span" sx={{ fontWeight: 500, color: "text.primary" }}>{group.summary.rawModel}</Box>
                ) : (
                  <Box component="span" sx={{ color: "text.secondary" }}>—</Box>
                )}
              </TableCell>
              <TableCell sx={{ px: 3, py: 1.5 }}>
                {group.summary.provider ? (
                  <Badge variant={group.summary.pending > 0 ? "primary" : "neutral"} size="sm">{group.summary.provider}</Badge>
                ) : (
                  <Box component="span" sx={{ color: "text.secondary" }}>—</Box>
                )}
              </TableCell>
              <TableCell sx={{ px: 3, py: 1.5, textAlign: "right" }}>{fmt(group.summary.requests)}</TableCell>
              <TableCell sx={{ px: 3, py: 1.5, textAlign: "right", color: "text.secondary", whiteSpace: "nowrap" }}>{fmtTime(group.summary.lastUsed)}</TableCell>
            </>
          ),
          renderDetailCells: (item) => (
            <>
              <TableCell sx={{ px: 3, py: 1.5, fontWeight: 500, transition: "color 150ms", color: item.pending > 0 ? "primary.main" : undefined }}>{item.accountName || `Account ${item.connectionId?.slice(0, 8)}...`}</TableCell>
              <TableCell sx={{ px: 3, py: 1.5, fontWeight: 500, transition: "color 150ms", color: item.pending > 0 ? "primary.main" : undefined }}>{item.rawModel}</TableCell>
              <TableCell sx={{ px: 3, py: 1.5 }}><Badge variant={item.pending > 0 ? "primary" : "neutral"} size="sm">{item.provider}</Badge></TableCell>
              <TableCell sx={{ px: 3, py: 1.5, textAlign: "right" }}>{fmt(item.requests)}</TableCell>
              <TableCell sx={{ px: 3, py: 1.5, textAlign: "right", color: "text.secondary", whiteSpace: "nowrap" }}>{fmtTime(item.lastUsed)}</TableCell>
            </>
          ),
        };
      }
      case "apiKey": {
        return {
          columns: API_KEY_COLUMNS,
          groupedData: groupDataByKey(sortData(stats.byApiKey, {}, sortBy, sortOrder), "keyName"),
          storageKey: "usage-stats:expanded-apikeys",
          emptyMessage: "No API key usage recorded yet.",
          renderSummaryCells: (group) => (
            <>
              <TableCell sx={{ px: 3, py: 1.5 }}>
                {group.summary.rawModel ? (
                  <Box component="span" sx={{ fontWeight: 500, color: "text.primary" }}>{group.summary.rawModel}</Box>
                ) : (
                  <Box component="span" sx={{ color: "text.secondary" }}>—</Box>
                )}
              </TableCell>
              <TableCell sx={{ px: 3, py: 1.5 }}>
                {group.summary.provider ? (
                  <Badge variant="neutral" size="sm">{group.summary.provider}</Badge>
                ) : (
                  <Box component="span" sx={{ color: "text.secondary" }}>—</Box>
                )}
              </TableCell>
              <TableCell sx={{ px: 3, py: 1.5, textAlign: "right" }}>{fmt(group.summary.requests)}</TableCell>
              <TableCell sx={{ px: 3, py: 1.5, textAlign: "right", color: "text.secondary", whiteSpace: "nowrap" }}>{fmtTime(group.summary.lastUsed)}</TableCell>
            </>
          ),
          renderDetailCells: (item) => (
            <>
              <TableCell sx={{ px: 3, py: 1.5, fontWeight: 500 }}>{item.keyName}</TableCell>
              <TableCell sx={{ px: 3, py: 1.5 }}>{item.rawModel}</TableCell>
              <TableCell sx={{ px: 3, py: 1.5 }}><Badge variant="neutral" size="sm">{item.provider}</Badge></TableCell>
              <TableCell sx={{ px: 3, py: 1.5, textAlign: "right" }}>{fmt(item.requests)}</TableCell>
              <TableCell sx={{ px: 3, py: 1.5, textAlign: "right", color: "text.secondary", whiteSpace: "nowrap" }}>{fmtTime(item.lastUsed)}</TableCell>
            </>
          ),
        };
      }
      case "endpoint":
      default: {
        return {
          columns: ENDPOINT_COLUMNS,
          groupedData: groupDataByKey(sortData(stats.byEndpoint, {}, sortBy, sortOrder), "endpoint"),
          storageKey: "usage-stats:expanded-endpoints",
          emptyMessage: "No endpoint usage recorded yet.",
          renderSummaryCells: (group) => (
            <>
              <TableCell sx={{ px: 3, py: 1.5 }}>
                {group.summary.rawModel ? (
                  <Box component="span" sx={{ fontWeight: 500, color: "text.primary" }}>{group.summary.rawModel}</Box>
                ) : (
                  <Box component="span" sx={{ color: "text.secondary" }}>—</Box>
                )}
              </TableCell>
              <TableCell sx={{ px: 3, py: 1.5 }}>
                {group.summary.provider ? (
                  <Badge variant="neutral" size="sm">{group.summary.provider}</Badge>
                ) : (
                  <Box component="span" sx={{ color: "text.secondary" }}>—</Box>
                )}
              </TableCell>
              <TableCell sx={{ px: 3, py: 1.5, textAlign: "right" }}>{fmt(group.summary.requests)}</TableCell>
              <TableCell sx={{ px: 3, py: 1.5, textAlign: "right", color: "text.secondary", whiteSpace: "nowrap" }}>{fmtTime(group.summary.lastUsed)}</TableCell>
            </>
          ),
          renderDetailCells: (item) => (
            <>
              <TableCell sx={{ px: 3, py: 1.5, fontWeight: 500, fontFamily: "ui-monospace, monospace", fontSize: 14 }}>{item.endpoint}</TableCell>
              <TableCell sx={{ px: 3, py: 1.5 }}>{item.rawModel}</TableCell>
              <TableCell sx={{ px: 3, py: 1.5 }}><Badge variant="neutral" size="sm">{item.provider}</Badge></TableCell>
              <TableCell sx={{ px: 3, py: 1.5, textAlign: "right" }}>{fmt(item.requests)}</TableCell>
              <TableCell sx={{ px: 3, py: 1.5, textAlign: "right", color: "text.secondary", whiteSpace: "nowrap" }}>{fmtTime(item.lastUsed)}</TableCell>
            </>
          ),
        };
      }
    }
  }, [stats, tableView, sortBy, sortOrder]);

  if (!stats && !loading) return <Box sx={{ color: "text.secondary" }}>Failed to load usage statistics.</Box>;

  return (
    <Box sx={{ display: "flex", minWidth: 0, flexDirection: "column", gap: 3 }}>
      {/* Period selector (hidden when controlled by parent) */}
      {!hidePeriodSelector && <PeriodSelector period={period} setPeriod={setPeriod} fetching={fetching} />}

      {/* Overview cards */}
      {loading ? overviewSkeleton : <OverviewCards stats={stats} />}

      {/* Provider topology + Recent Requests */}
      {loading || !idleReadyForTopology ? topologySkeleton : (
        <Box sx={{ display: "grid", minWidth: 0, gridTemplateColumns: { xs: "1fr", lg: "minmax(0,2fr) minmax(280px,1fr)" }, alignItems: "stretch", gap: 1 }}>
          <ProviderTopology
            providers={providers}
            activeRequests={stats.activeRequests || []}
            lastProvider={stats.recentRequests?.[0]?.provider || ""}
            errorProvider={stats.errorProvider || ""}
          />
          <RecentRequests requests={stats.recentRequests || []} />
        </Box>
      )}

      {/* Token / Cost chart - sync period. `chartsRef` marks the first recharts
          region; the visibility gate replaces the old idle gate for the three
          recharts charts (UsageChart + the byProvider/byModel pair below).
          `chartSkeleton` is sized to UsageChart's full height so this swap does
          not move the table. */}
      <Box ref={chartsRef}>
        {loading || !chartsInView ? chartSkeleton : <UsageChart period={period} />}
      </Box>

      {/* Provider and model breakdown charts */}
      {!loading && chartsInView && (stats.byProvider || stats.byModel) && (
        <Box sx={{ display: "grid", minWidth: 0, gridTemplateColumns: { xs: "1fr", lg: "1fr 1fr" }, gap: 1 }}>
          <ProviderBarChart byProvider={stats.byProvider} />
          <TopModelsChart byModel={stats.byModel} />
        </Box>
      )}

      {/* Table with dropdown selector */}
      <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
        <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1, alignItems: { sm: "center" }, justifyContent: { sm: "space-between" } }}>
          <Box
            component="select"
            value={tableView}
            onChange={(e) => setTableView(e.target.value)}
            sx={{ width: { xs: "100%", sm: "auto" }, borderRadius: "8px", border: "1px solid var(--color-border)", bgcolor: "var(--color-surface)", px: 1.5, py: 0.75, fontSize: 14, fontWeight: 500, color: "text.primary", outline: "none", "&:focus": { boxShadow: "0 0 0 2px rgba(229,106,74,0.5)" } }}
            style={{ colorScheme: "auto" }}
          >
            {TABLE_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>{opt.label}</option>
            ))}
          </Box>
          <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", alignItems: "center", gap: 0.5, borderRadius: "8px", border: "1px solid var(--color-border)", bgcolor: "var(--color-surface-2)", p: 0.5, ...({ "@media (min-width:600px)": { display: "flex" } }) }}>
            <Box component="button" type="button" onClick={() => setViewMode("costs")} sx={periodBtnSx(viewMode === "costs")}>
              Costs
            </Box>
            <Box component="button" type="button" onClick={() => setViewMode("tokens")} sx={periodBtnSx(viewMode === "tokens")}>
              Tokens
            </Box>
          </Box>
        </Box>
        {loading ? tableSkeleton : activeTableConfig && (
          <UsageTable
            title=""
            columns={activeTableConfig.columns}
            groupedData={activeTableConfig.groupedData}
            tableType={tableView}
            sortBy={sortBy}
            sortOrder={sortOrder}
            onToggleSort={toggleSort}
            viewMode={viewMode}
            storageKey={activeTableConfig.storageKey}
            renderSummaryCells={activeTableConfig.renderSummaryCells}
            renderDetailCells={activeTableConfig.renderDetailCells}
            emptyMessage={activeTableConfig.emptyMessage}
          />
        )}
      </Box>
    </Box>
  );
}
