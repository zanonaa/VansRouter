"use client";

import { useState, useEffect, useCallback, useRef, useMemo } from "react";
import { useSearchParams, useRouter, usePathname } from "next/navigation";
import {
  Box,
  Button,
  Dialog,
  DialogContent,
  DialogTitle,
  IconButton,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Typography,
} from "@mui/material";
import ProviderIcon from "@/shared/components/ProviderIcon";
import QuotaTable from "./QuotaTable";
import Toggle from "@/shared/components/Toggle";
import Tooltip from "@/shared/components/Tooltip";
import {
  parseQuotaData,
  calculatePercentage,
  getConnectionLabel,
  getConnectionQuotaRemaining,
  sortVisibleConnections,
  buildLoadingState,
  filterQuotaStateByConnections,
  getConnectionsEmptyMessage,
  getPageSizeLabel,
  getConnectionsPaginationSummary,
  getSafePagination,
  getSafeTotals,
  shouldResetPage,
  getPaginationPageValue,
  getProviderOptions,
  reconcileConnectionsPage,
  getQuotaCache,
  setQuotaCache,
  getModelOptionsForProvider,
  isMultiModelProvider,
  isCodexUnavailable401,
  getQuotaModelKey,
  filterQuotasByModel,
  QUOTA_CACHE_KEY,
  REFRESH_INTERVAL_MS,
  CLAUDE_REFRESH_INTERVAL_MS,
  DEPLETED_QUOTA_THRESHOLD,
  AUTO_REFRESH_STORAGE_KEY,
  CONNECTIONS_PAGE_SIZE,
  ACCOUNT_PAGE_SIZE_OPTIONS,
  ACCOUNT_PAGE_SIZE_MAX,
  ACCOUNT_FILTER_OPTIONS,
  QUOTA_SORT_OPTIONS,
} from "./utils";
import Card from "@/shared/components/Card";
import { ConfirmModal, EditConnectionModal } from "@/shared/components";
import { USAGE_SUPPORTED_PROVIDERS } from "@/shared/constants/providers";
import { useCopyToClipboard } from "@/shared/hooks/useCopyToClipboard";

// Maps the stored providerSpecificData.authMethod to a human label for Kiro.
// Values come from the Kiro connect flows: builder-id/idc (device code),
// google/github (social), imported (refresh-token paste), api_key (headless).
const KIRO_METHOD_LABELS = {
  "builder-id": "AWS Builder ID",
  idc: "IAM Identity Center",
  google: "Google",
  github: "GitHub",
  imported: "Imported Token",
  api_key: "API Key",
};

const AUTO_PING_SETTINGS_KEYS = {
  claude: "claudeAutoPing",
  codex: "codexAutoPing",
};

const AUTO_PING_TOOLTIPS = {
  claude: "When your 5h quota runs out, auto-sends a request the moment it resets so a new window starts right away.",
  codex: "Auto-starts the next 5h Codex window after reset by sending a tiny gpt-5.5 request. Consumes a small amount of quota.",
};

const borderSubtle = (theme) =>
  theme.palette.mode === "dark" ? "rgba(255, 255, 255, 0.1)" : "rgba(0, 0, 0, 0.1)";
const borderFaint = (theme) =>
  theme.palette.mode === "dark" ? "rgba(255, 255, 255, 0.05)" : "rgba(0, 0, 0, 0.05)";
const bgSubtle = (theme) =>
  theme.palette.mode === "dark" ? "rgba(255, 255, 255, 0.03)" : "rgba(0, 0, 0, 0.02)";
const bgHover = (theme) =>
  theme.palette.mode === "dark" ? "rgba(255, 255, 255, 0.1)" : "rgba(0, 0, 0, 0.05)";

const controlSx = {
  height: 32,
  borderRadius: 2,
  border: 1,
  borderColor: borderSubtle,
  bgcolor: bgSubtle,
  color: "text.primary",
  px: 1,
  fontSize: 12,
  outline: "none",
  transition: "background-color 150ms ease",
  cursor: "pointer",
  "&:hover": { bgcolor: bgHover },
  "& option": { bgcolor: "background.paper", color: "text.primary" },
};

const compactButtonSx = {
  height: 32,
  minWidth: 0,
  px: 1,
  gap: 0.5,
  fontSize: 12,
  borderColor: borderSubtle,
  color: "text.primary",
  "&:hover": { bgcolor: bgHover, borderColor: borderSubtle },
  "&.Mui-disabled": { opacity: 0.5 },
};

const rowIconSx = {
  width: 32,
  height: 32,
  borderRadius: 2,
  color: "text.secondary",
  "&:hover": { bgcolor: bgHover },
  "&.Mui-disabled": { opacity: 0.5 },
};

function kiroMethodLabel(conn) {
  const m = conn.providerSpecificData?.authMethod;
  if (m && KIRO_METHOD_LABELS[m]) return KIRO_METHOD_LABELS[m];
  return conn.authType === "api_key" ? "API Key" : "OAuth";
}

function getConnectionSecondaryLabel(connection) {
  if (connection.name?.trim() && connection.email?.trim() && connection.name.trim() !== connection.email.trim()) {
    return connection.email.trim();
  }

  if (connection.name?.trim() && connection.displayName?.trim() && connection.name.trim() !== connection.displayName.trim()) {
    return connection.displayName.trim();
  }

  return null;
}

// Region is stored for builder-id/idc/api_key flows; social and imported flows
// omit it, so fall back to the region segment of the profileArn
// (arn:aws:codewhisperer:<region>:...).
function kiroRegion(conn) {
  const r = conn.providerSpecificData?.region;
  if (r) return r;
  const arn = conn.providerSpecificData?.profileArn;
  const seg = typeof arn === "string" ? arn.split(":")[3] : "";
  return seg || "";
}

function getCodexResetCreditCount(quota) {
  const value = quota?.raw?.resetCredits?.availableCount;
  const count = typeof value === "number" ? value : Number(value);
  return Number.isFinite(count) ? Math.max(0, count) : 0;
}

const CLAUDE_RESET_LIMIT_NAMES = {
  five_hour: "session",
  seven_day: "weekly",
  seven_day_overage_included: "weekly",
  seven_day_opus: "Opus weekly",
  seven_day_sonnet: "Sonnet weekly",
};

function formatClaudeResetClears(clears) {
  const names = [...new Set((clears || []).map((c) => CLAUDE_RESET_LIMIT_NAMES[c]).filter(Boolean))];
  return names.length ? `${names.join(" + ")} limits` : "limits";
}

function claudeGrantStatus(grant) {
  if (grant.resetsLeft <= 0) return "used";
  if (grant.paused) return "paused";
  if (grant.endsAt && new Date(grant.endsAt).getTime() <= Date.now()) return "expired";
  if (grant.usableNow) return "usable now";
  if (grant.startsAt && new Date(grant.startsAt).getTime() > Date.now()) return "not started";
  return grant.useRequiresLimit ? "at limit only" : "unavailable";
}

function formatCreditDate(value) {
  if (!value) return "N/A";
  const date = new Date(value);
  if (!Number.isFinite(date.getTime())) return "N/A";
  return date.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

function formatTimeRemaining(value) {
  if (!value) return "N/A";
  const diffMs = new Date(value).getTime() - Date.now();
  if (!Number.isFinite(diffMs)) return "N/A";
  if (diffMs <= 0) return "Expired";
  const totalHours = Math.ceil(diffMs / (60 * 60 * 1000));
  const days = Math.floor(totalHours / 24);
  const hours = totalHours % 24;
  return days > 0 ? `${days}d ${hours}h` : `${hours}h`;
}

export default function ProviderLimits() {
  const { copied, copy } = useCopyToClipboard();
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  const [connections, setConnections] = useState([]);
  const [quotaData, setQuotaData] = useState({});
  const [loading, setLoading] = useState({});
  const [errors, setErrors] = useState({});
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [autoPingMaps, setAutoPingMaps] = useState({ claude: {}, codex: {} });
  const [lastUpdated, setLastUpdated] = useState(null);
  const [hasHydratedAutoRefresh, setHasHydratedAutoRefresh] = useState(false);
  const [refreshingAll, setRefreshingAll] = useState(false);
  const [countdown, setCountdown] = useState(60);
  const [connectionsLoading, setConnectionsLoading] = useState(true);
  const [deletingId, setDeletingId] = useState(null);
  const [togglingId, setTogglingId] = useState(null);
  const [resettingLimitId, setResettingLimitId] = useState(null);
  const [resetConfirmState, setResetConfirmState] = useState(null);
  const [resetCreditsState, setResetCreditsState] = useState(null);
  const [showEditModal, setShowEditModal] = useState(false);
  const [selectedConnection, setSelectedConnection] = useState(null);
  const [proxyPools, setProxyPools] = useState([]);
  // Initialize providerFilter from URL ?provider= param so the page
  // can be bookmarked / deep-linked to a specific provider (#4217).
  const [providerFilter, _setProviderFilter] = useState(
    () => searchParams?.get("provider") || "all"
  );
  // Wrapper: keeps URL in sync with the selected provider so the view can be
  // bookmarked. Replaces the URL without adding to browser history.
  const setProviderFilter = useCallback((value) => {
    _setProviderFilter(value);
    try {
      const params = new URLSearchParams(searchParams?.toString() || "");
      if (value === "all") {
        params.delete("provider");
      } else {
        params.set("provider", value);
      }
      const newUrl = params.toString() ? `${pathname}?${params.toString()}` : pathname;
      router.replace(newUrl, { scroll: false });
    } catch { /* non-fatal: URL sync is best-effort */ }
  }, [pathname, router, searchParams]);
  const [providerOptions, setProviderOptions] = useState([]);
  const [modelFilter, setModelFilter] = useState("all");
  const [accountFilter, setAccountFilter] = useState("all");
  const [quotaSortMode, setQuotaSortMode] = useState("default");
  const [expiringFirst, setExpiringFirst] = useState(false);
  const [providerMenuOpen, setProviderMenuOpen] = useState(false);
  const [bulkToggling, setBulkToggling] = useState(false);
  const [bulkDeleting, setBulkDeleting] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(CONNECTIONS_PAGE_SIZE);
  const [customPageSizeInput, setCustomPageSizeInput] = useState(
    String(CONNECTIONS_PAGE_SIZE),
  );
  const [pagination, setPagination] = useState({
    page: 1,
    pageSize: CONNECTIONS_PAGE_SIZE,
    total: 0,
    totalPages: 1,
  });
  const [totals, setTotals] = useState({
    eligibleConnections: 0,
    providerFilteredConnections: 0,
  });

  const intervalRef = useRef(null);
  const countdownRef = useRef(null);
  const tickCountRef = useRef(0);

  const fetchConnections = useCallback(
    async (targetPage = page) => {
      try {
        const params = new URLSearchParams({
          page: String(targetPage),
          pageSize: String(pageSize),
          accountStatus: accountFilter,
          sort: "priority",
        });

        if (providerFilter !== "all") {
          params.set("provider", providerFilter);
        }

        const response = await fetch(
          `/api/providers/client?${params.toString()}`,
        );
        if (!response.ok) throw new Error("Failed to fetch connections");

        const data = await response.json();
        const connectionList = data.connections || [];
        const nextPagination = getSafePagination(data.pagination, pageSize);
        const nextTotals = getSafeTotals(data.totals, connectionList.length);

        setConnections(connectionList);
        setProviderOptions(getProviderOptions(data.providerOptions));
        setPagination(nextPagination);
        setTotals(nextTotals);
        setPage(getPaginationPageValue(data.pagination, targetPage));
        return connectionList;
      } catch (error) {
        console.error("Error fetching connections:", error);
        setConnections([]);
        setProviderOptions([]);
        setPagination({ page: 1, pageSize, total: 0, totalPages: 1 });
        setTotals({ eligibleConnections: 0, providerFilteredConnections: 0 });
        return [];
      }
    },
    [accountFilter, page, pageSize, providerFilter],
  );

  // Fetch quota for a specific connection
  const fetchQuota = useCallback(async (connectionId, provider, { force = false } = {}) => {
    setLoading((prev) => ({ ...prev, [connectionId]: true }));
    setErrors((prev) => ({ ...prev, [connectionId]: null }));

    try {
      console.log(
        `[ProviderLimits] Fetching quota for ${provider} (${connectionId})`,
      );
      const response = await fetch(`/api/usage/${connectionId}${force ? "?force=1" : ""}`);

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        const errorMsg = errorData.error || response.statusText;

        // Handle different error types gracefully
        if (response.status === 404) {
          // Connection not found - skip silently
          console.warn(
            `[ProviderLimits] Connection not found for ${provider}, skipping`,
          );
          return;
        }

        if (response.status === 401) {
          // Auth error - show message instead of throwing
          console.warn(
            `[ProviderLimits] Auth error for ${provider}:`,
            errorMsg,
          );
          const quotaEntry = {
            quotas: [],
            message: errorMsg,
          };
          setQuotaData((prev) => ({
            ...prev,
            [connectionId]: quotaEntry,
          }));
          setQuotaCache(connectionId, quotaEntry);
          return;
        }

        throw new Error(`HTTP ${response.status}: ${errorMsg}`);
      }

      const data = await response.json();
      console.log(`[ProviderLimits] Got quota for ${provider}:`, data);

      // Parse quota data using provider-specific parser
      const parsedQuotas = parseQuotaData(provider, data);

      const quotaEntry = {
        quotas: parsedQuotas,
        plan: data.plan || null,
        message: data.message || null,
        raw: data,
      };

      setQuotaData((prev) => ({
        ...prev,
        [connectionId]: quotaEntry,
      }));
      setQuotaCache(connectionId, quotaEntry);
    } catch (error) {
      console.error(
        `[ProviderLimits] Error fetching quota for ${provider} (${connectionId}):`,
        error,
      );
      setErrors((prev) => ({
        ...prev,
        [connectionId]: error.message || "Failed to fetch quota",
      }));
    } finally {
      setLoading((prev) => ({ ...prev, [connectionId]: false }));
    }
  }, []);

  // Refresh quota for a specific provider
  const refreshProvider = useCallback(
    async (connectionId, provider) => {
      await fetchQuota(connectionId, provider, { force: true });
      setLastUpdated(new Date());
    },
    [fetchQuota],
  );

  const handleResetCodexLimit = useCallback(
    async (connectionId, provider) => {
      if ((provider !== "codex" && provider !== "claude") || resettingLimitId) return;

      setResettingLimitId(connectionId);
      setErrors((prev) => ({ ...prev, [connectionId]: null }));

      try {
        const response = provider === "claude"
          ? await fetch(`/api/usage/${connectionId}/claude-reset`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ grantId: quotaData[connectionId]?.raw?.resetCredits?.nextGrantId }),
          })
          : await fetch(`/api/usage/${connectionId}/codex-reset-credits`, { method: "POST" });
        const result = await response.json().catch(() => ({}));

        if (!response.ok) {
          throw new Error(result.message || result.error || result.code || "Failed to reset limit");
        }

        await fetchQuota(connectionId, provider, { force: true });
        setLastUpdated(new Date());
      } catch (error) {
        setErrors((prev) => ({ ...prev, [connectionId]: error.message || "Failed to reset limit" }));
      } finally {
        setResettingLimitId(null);
      }
    },
    [fetchQuota, resettingLimitId, quotaData],
  );

  // Claude grants already arrive with the usage read; no extra fetch
  const handleViewClaudeResets = useCallback((connection, resetCredits) => {
    setResetCreditsState({ connection, loading: false, error: null, data: { kind: "claude", ...resetCredits } });
  }, []);

  const handleViewCodexResetCredits = useCallback(async (connection) => {
    setResetCreditsState({ connection, loading: true, error: null, data: null });
    try {
      const response = await fetch(`/api/usage/${connection.id}/codex-reset-credits`, { cache: "no-store" });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) {
        throw new Error(result.error || result.message || "Failed to load Codex reset credits");
      }
      const credits = Array.isArray(result.credits) ? [...result.credits] : [];
      credits.sort((a, b) => {
        const aTime = a.expiresAt ? new Date(a.expiresAt).getTime() : Number.POSITIVE_INFINITY;
        const bTime = b.expiresAt ? new Date(b.expiresAt).getTime() : Number.POSITIVE_INFINITY;
        return aTime - bTime;
      });
      setResetCreditsState({ connection, loading: false, error: null, data: { ...result, credits } });
    } catch (error) {
      setResetCreditsState({ connection, loading: false, error: error.message || "Failed to load Codex reset credits", data: null });
    }
  }, []);

  const handleDeleteConnection = useCallback(
    async (id) => {
      if (!confirm("Delete this connection?")) return;
      setDeletingId(id);
      try {
        const res = await fetch(`/api/providers/${id}`, { method: "DELETE" });
        if (res.ok) {
          setQuotaData((prev) => {
            const next = { ...prev };
            delete next[id];
            return next;
          });
          setLoading((prev) => {
            const next = { ...prev };
            delete next[id];
            return next;
          });
          setErrors((prev) => {
            const next = { ...prev };
            delete next[id];
            return next;
          });

          if (typeof window !== "undefined") {
            try {
              const cache = getQuotaCache();
              if (cache[id]) {
                delete cache[id];
                window.localStorage.setItem(
                  QUOTA_CACHE_KEY,
                  JSON.stringify(cache),
                );
              }
            } catch (e) {
              console.error("Error deleting cache entry:", e);
            }
          }

          await reconcileConnectionsPage(fetchConnections, page);
        }
      } catch (error) {
        console.error("Error deleting connection:", error);
      } finally {
        setDeletingId(null);
      }
    },
    [fetchConnections, page],
  );

  const handleToggleConnectionActive = useCallback(
    async (id, isActive) => {
      setTogglingId(id);
      try {
        const res = await fetch(`/api/providers/${id}`, {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ isActive }),
        });
        if (res.ok) {
          setQuotaData((prev) => {
            const next = { ...prev };
            return next;
          });
          await reconcileConnectionsPage(fetchConnections, page);
        }
      } catch (error) {
        console.error("Error updating connection status:", error);
      } finally {
        setTogglingId(null);
      }
    },
    [fetchConnections, page],
  );

  const handleUpdateConnection = useCallback(
    async (formData) => {
      if (!selectedConnection?.id) return;
      const connectionId = selectedConnection.id;
      const provider = selectedConnection.provider;
      try {
        const res = await fetch(`/api/providers/${connectionId}`, {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(formData),
        });
        if (res.ok) {
          await fetchConnections();
          setShowEditModal(false);
          setSelectedConnection(null);
          if (USAGE_SUPPORTED_PROVIDERS.includes(provider)) {
            await fetchQuota(connectionId, provider);
          }
        }
      } catch (error) {
        console.error("Error saving connection:", error);
      }
    },
    [selectedConnection, fetchConnections, fetchQuota],
  );

  useEffect(() => {
    let cancelled = false;
    fetch("/api/proxy-pools?isActive=true", { cache: "no-store" })
      .then((res) => res.json())
      .then((data) => {
        if (!cancelled && data?.proxyPools) {
          setProxyPools(data.proxyPools);
        }
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, []);

  const refreshAll = useCallback(async (force = false) => {
    if (refreshingAll) return;

    setRefreshingAll(true);
    setCountdown(60);

    // Throttle Claude: poll its quota every Nth auto-tick (manual force bypasses)
    const tick = (tickCountRef.current += 1);
    const claudeEvery = Math.round(CLAUDE_REFRESH_INTERVAL_MS / REFRESH_INTERVAL_MS);
    const shouldFetch = (conn) =>
      force || conn.provider !== "claude" || tick % claudeEvery === 0;

    try {
      const visibleConnections = await fetchConnections(page);

      setLoading(buildLoadingState(visibleConnections));
      setErrors((prev) =>
        filterQuotaStateByConnections(prev, visibleConnections),
      );
      setQuotaData((prev) =>
        filterQuotaStateByConnections(prev, visibleConnections),
      );

      await Promise.all(
        visibleConnections
          .filter(shouldFetch)
          .map((conn) => fetchQuota(conn.id, conn.provider)),
      );

      setLastUpdated(new Date());
    } catch (error) {
      console.error("Error refreshing all providers:", error);
    } finally {
      setRefreshingAll(false);
    }
  }, [refreshingAll, fetchConnections, fetchQuota, page]);

  useEffect(() => {
    const initializeData = async () => {
      setConnectionsLoading(true);
      const visibleConnections = await fetchConnections(page);
      setConnectionsLoading(false);

      // Always fetch fresh quota on mount, no cache display
      setLoading(buildLoadingState(visibleConnections));
      setErrors((prev) =>
        filterQuotaStateByConnections(prev, visibleConnections),
      );
      setQuotaData((prev) =>
        filterQuotaStateByConnections(prev, visibleConnections),
      );

      await Promise.all(
        visibleConnections.map((conn) => fetchQuota(conn.id, conn.provider)),
      );
      setLastUpdated(new Date());
    };

    initializeData();
  }, [fetchConnections, fetchQuota, page]);

  useEffect(() => {
    if (typeof window === "undefined") return;
    const stored = window.localStorage.getItem(AUTO_REFRESH_STORAGE_KEY);
    setAutoRefresh(stored === null ? true : stored === "true");
    setHasHydratedAutoRefresh(true);
  }, []);

  // Persist auto-refresh preference
  useEffect(() => {
    if (typeof window === "undefined" || !hasHydratedAutoRefresh) return;
    window.localStorage.setItem(AUTO_REFRESH_STORAGE_KEY, String(autoRefresh));
  }, [autoRefresh, hasHydratedAutoRefresh]);

  // Load auto-ping per-connection maps
  useEffect(() => {
    fetch("/api/settings", { cache: "no-store" })
      .then((r) => (r.ok ? r.json() : {}))
      .then((s) => setAutoPingMaps({
        claude: s?.claudeAutoPing?.connections || {},
        codex: s?.codexAutoPing?.connections || {},
      }))
      .catch(() => {});
  }, []);

  const toggleAutoPing = useCallback(async (connectionId, provider, on) => {
    const settingsKey = AUTO_PING_SETTINGS_KEYS[provider];
    if (!settingsKey) return;

    const previous = autoPingMaps;
    const nextProviderMap = { ...(autoPingMaps[provider] || {}), [connectionId]: on };
    const nextMaps = { ...autoPingMaps, [provider]: nextProviderMap };
    setAutoPingMaps(nextMaps);
    try {
      const r = await fetch("/api/settings", { cache: "no-store" });
      const s = r.ok ? await r.json() : {};
      const cfg = { ...(s[settingsKey] || {}), connections: nextProviderMap };
      await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ [settingsKey]: cfg }),
      });
    } catch {
      setAutoPingMaps(previous);
    }
  }, [autoPingMaps]);

  // Auto-refresh interval
  useEffect(() => {
    if (!hasHydratedAutoRefresh || !autoRefresh) {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
        intervalRef.current = null;
      }
      if (countdownRef.current) {
        clearInterval(countdownRef.current);
        countdownRef.current = null;
      }
      return;
    }

    // Main refresh interval
    intervalRef.current = setInterval(() => {
      refreshAll();
    }, REFRESH_INTERVAL_MS);

    // Countdown interval
    countdownRef.current = setInterval(() => {
      setCountdown((prev) => {
        if (prev <= 1) return 60;
        return prev - 1;
      });
    }, 1000);

    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
      if (countdownRef.current) clearInterval(countdownRef.current);
    };
  }, [autoRefresh, refreshAll, hasHydratedAutoRefresh]);

  // Pause auto-refresh when tab is hidden (Page Visibility API)
  useEffect(() => {
    const handleVisibilityChange = () => {
      if (document.hidden) {
        if (intervalRef.current) {
          clearInterval(intervalRef.current);
          intervalRef.current = null;
        }
        if (countdownRef.current) {
          clearInterval(countdownRef.current);
          countdownRef.current = null;
        }
      } else if (autoRefresh && hasHydratedAutoRefresh) {
        // Resume auto-refresh when tab becomes visible
        intervalRef.current = setInterval(() => refreshAll(), REFRESH_INTERVAL_MS);
        countdownRef.current = setInterval(() => {
          setCountdown((prev) => (prev <= 1 ? 60 : prev - 1));
        }, 1000);
      }
    };

    document.addEventListener("visibilitychange", handleVisibilityChange);
    return () => {
      document.removeEventListener("visibilitychange", handleVisibilityChange);
    };
  }, [autoRefresh, refreshAll, hasHydratedAutoRefresh]);

  // Reset per-model filter when the provider filter changes or no longer supports it.
  useEffect(() => {
    setModelFilter("all");
  }, [providerFilter]);

  const modelOptions = useMemo(
    () => getModelOptionsForProvider(providerFilter),
    [providerFilter],
  );

  const sortedConnections = useMemo(
    () =>
      sortVisibleConnections(
        connections,
        quotaData,
        expiringFirst,
        providerFilter,
        quotaSortMode,
      ),
    [connections, quotaData, expiringFirst, providerFilter, quotaSortMode],
  );

  // Connection is depleted when any quota entry hit the threshold
  const isConnectionDepleted = (conn) => {
    const quotas = quotaData[conn.id]?.quotas;
    if (!quotas?.length) return false;
    return quotas.some((q) => {
      if (!q.total || q.total <= 0) return false;
      return calculatePercentage(q.used, q.total) <= DEPLETED_QUOTA_THRESHOLD;
    });
  };

  const bulkSetActive = useCallback(
    async (targetIds, isActive) => {
      if (!targetIds.length || bulkToggling) return;
      setBulkToggling(true);
      try {
        await Promise.all(
          targetIds.map((id) =>
            fetch(`/api/providers/${id}`, {
              method: "PUT",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ isActive }),
            }),
          ),
        );
        await reconcileConnectionsPage(fetchConnections, page);
      } catch (error) {
        console.error("Error bulk toggling connections:", error);
      } finally {
        setBulkToggling(false);
      }
    },
    [bulkToggling, fetchConnections, page],
  );

  const handleDisableDepleted = () => {
    const ids = sortedConnections
      .filter((c) => (c.isActive ?? true) && isConnectionDepleted(c))
      .map((c) => c.id);
    bulkSetActive(ids, false);
  };

  const handleEnableAvailable = () => {
    const ids = sortedConnections
      .filter((c) => !(c.isActive ?? true) && !isConnectionDepleted(c))
      .map((c) => c.id);
    bulkSetActive(ids, true);
  };

  const codexUnavailableIds = useMemo(() => {
    return sortedConnections
      .filter((c) => isCodexUnavailable401(c, quotaData[c.id], errors[c.id]))
      .map((c) => c.id);
  }, [sortedConnections, quotaData, errors]);

  const handleBulkDeleteCodexUnavailable = useCallback(async () => {
    if (!codexUnavailableIds.length || bulkDeleting) return;

    if (
      !confirm(
        `Delete ${codexUnavailableIds.length} Codex connection(s) with "Usage API temporarily unavailable (401)"?`,
      )
    ) {
      return;
    }

    setBulkDeleting(true);
    try {
      await Promise.all(
        codexUnavailableIds.map((id) =>
          fetch(`/api/providers/${id}`, { method: "DELETE" }),
        ),
      );

      setQuotaData((prev) => {
        const next = { ...prev };
        for (const id of codexUnavailableIds) delete next[id];
        return next;
      });
      setLoading((prev) => {
        const next = { ...prev };
        for (const id of codexUnavailableIds) delete next[id];
        return next;
      });
      setErrors((prev) => {
        const next = { ...prev };
        for (const id of codexUnavailableIds) delete next[id];
        return next;
      });

      if (typeof window !== "undefined") {
        try {
          const cache = getQuotaCache();
          let changed = false;
          for (const id of codexUnavailableIds) {
            if (cache[id]) {
              delete cache[id];
              changed = true;
            }
          }
          if (changed) {
            window.localStorage.setItem(
              QUOTA_CACHE_KEY,
              JSON.stringify(cache),
            );
          }
        } catch (e) {
          console.error("Error deleting cache entries:", e);
        }
      }

      await reconcileConnectionsPage(fetchConnections, page);
    } catch (error) {
      console.error("Error bulk deleting codex unavailable connections:", error);
    } finally {
      setBulkDeleting(false);
    }
  }, [codexUnavailableIds, bulkDeleting, fetchConnections, page]);

  const selectedProviderLabel =
    providerFilter === "all" ? "All providers" : providerFilter;
  const hasEligibleConnections = totals.eligibleConnections > 0;
  const hasVisibleConnections = sortedConnections.length > 0;
  const emptyState = getConnectionsEmptyMessage(
    totals,
    providerFilter,
    accountFilter,
  );
  const connectionsPageSummary = getConnectionsPaginationSummary(pagination);
  const isCustomPageSize = !ACCOUNT_PAGE_SIZE_OPTIONS.includes(pageSize);
  const pageSizeLabel = getPageSizeLabel(pageSize, isCustomPageSize);

  if (!connectionsLoading && !hasEligibleConnections) {
    return (
      <Card padding="lg">
        <Box sx={{ textAlign: "center", py: 6 }}>
          <Box
            component="span"
            className="material-symbols-outlined"
            sx={{ fontSize: 64, color: "text.secondary", opacity: 0.2 }}
          >
            cloud_off
          </Box>
          <Typography component="h3" sx={{ mt: 2, fontSize: 18, fontWeight: 600, color: "text.primary" }}>
            No Providers Connected
          </Typography>
          <Typography sx={{ mt: 1, fontSize: 14, color: "text.secondary", maxWidth: 448, mx: "auto" }}>
            Connect to providers with OAuth to track your API quota limits and
            usage.
          </Typography>
        </Box>
      </Card>
    );
  }

  if (!connectionsLoading && !hasVisibleConnections) {
    return (
      <Card padding="lg">
        <Box sx={{ textAlign: "center", py: 6 }}>
          <Box
            component="span"
            className="material-symbols-outlined"
            sx={{ fontSize: 64, color: "text.secondary", opacity: 0.2 }}
          >
            {emptyState.icon}
          </Box>
          <Typography component="h3" sx={{ mt: 2, fontSize: 18, fontWeight: 600, color: "text.primary" }}>
            {emptyState.title}
          </Typography>
          <Typography sx={{ mt: 1, fontSize: 14, color: "text.secondary", maxWidth: 448, mx: "auto" }}>
            {emptyState.description}
          </Typography>
        </Box>
      </Card>
    );
  }

  const providerMenuContainerSx = {
    position: "absolute",
    left: 0,
    zIndex: 40,
    mt: 0.75,
    width: { xs: 208, sm: 240 },
    overflow: "hidden",
    borderRadius: 3,
    border: 1,
    borderColor: borderSubtle,
    bgcolor: (theme) =>
      theme.palette.mode === "dark" ? "rgba(38, 38, 38, 0.95)" : "rgba(255, 255, 255, 0.95)",
    p: 0.5,
    boxShadow: "0 10px 15px -3px rgba(0, 0, 0, 0.1)",
    backdropFilter: "blur(8px)",
  };

  const menuItemSx = (selected) => ({
    display: "flex",
    width: "100%",
    alignItems: "center",
    gap: 1,
    borderRadius: 2,
    px: 1.25,
    py: 0.75,
    textAlign: "left",
    fontSize: 12,
    border: 0,
    cursor: "pointer",
    bgcolor: selected ? "rgba(229, 106, 74, 0.1)" : "transparent",
    color: selected ? "primary.main" : "text.primary",
    transition: "background-color 150ms ease",
    "&:hover": {
      bgcolor: selected ? "rgba(229, 106, 74, 0.14)" : bgHover,
    },
  });

  const resetCreditsOpen = Boolean(resetCreditsState);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      {/* Header Controls */}
      <Box
        sx={{
          display: "flex",
          flexDirection: { xs: "column", sm: "row" },
          alignItems: { sm: "center" },
          justifyContent: { sm: "flex-end" },
          gap: 2,
        }}
      >
        <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 0.75 }}>
          <Box sx={{ position: "relative" }}>
            <Button
              type="button"
              variant="outlined"
              onClick={() => setProviderMenuOpen((prev) => !prev)}
              aria-haspopup="menu"
              aria-expanded={providerMenuOpen}
              title="Filter quota providers"
              sx={{
                ...compactButtonSx,
                height: 32,
                justifyContent: "space-between",
                gap: 0.5,
                bgcolor: bgSubtle,
              }}
            >
              <Box component="span" sx={{ display: "flex", minWidth: 0, alignItems: "center", gap: 0.75 }}>
                {providerFilter === "all" ? (
                  <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 14, color: "text.secondary" }}>
                    apps
                  </Box>
                ) : (
                  <ProviderIcon
                    src={`/providers/${providerFilter}.webp`}
                    alt={providerFilter}
                    size={18}
                    fallbackText={providerFilter.slice(0, 2).toUpperCase()}
                  />
                )}
                <Box
                  component="span"
                  sx={{
                    display: { xs: "none", lg: "inline" },
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                    textTransform: "capitalize",
                  }}
                >
                  {selectedProviderLabel}
                </Box>
              </Box>
              <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 14, color: "text.secondary" }}>
                expand_more
              </Box>
            </Button>

            {providerMenuOpen && (
              <>
                <Box
                  component="button"
                  type="button"
                  aria-label="Close provider filter"
                  onClick={() => setProviderMenuOpen(false)}
                  sx={{
                    position: "fixed",
                    inset: 0,
                    zIndex: 30,
                    bgcolor: "transparent",
                    border: 0,
                    p: 0,
                    cursor: "default",
                  }}
                />
                <Box sx={providerMenuContainerSx}>
                  <Box
                    component="button"
                    type="button"
                    onClick={() => {
                      if (shouldResetPage(providerFilter, "all")) {
                        setPage(1);
                      }
                      setProviderFilter("all");
                      setProviderMenuOpen(false);
                    }}
                    sx={menuItemSx(providerFilter === "all")}
                  >
                    <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 16 }}>
                      apps
                    </Box>
                    <Box component="span" sx={{ fontWeight: 500 }}>All providers</Box>
                    {providerFilter === "all" && (
                      <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 16, ml: "auto" }}>
                        check
                      </Box>
                    )}
                  </Box>
                  <Box sx={{ my: 0.5, height: "1px", bgcolor: borderSubtle }} />
                  <Box sx={{ maxHeight: 240, overflowY: "auto", pr: 0.25 }}>
                    {providerOptions.map((provider) => (
                      <Box
                        component="button"
                        key={provider}
                        type="button"
                        onClick={() => {
                          if (shouldResetPage(providerFilter, provider)) {
                            setPage(1);
                          }
                          setProviderFilter(provider);
                          setProviderMenuOpen(false);
                        }}
                        sx={menuItemSx(providerFilter === provider)}
                      >
                        <ProviderIcon
                          src={`/providers/${provider}.webp`}
                          alt={provider}
                          size={18}
                          fallbackText={provider.slice(0, 2).toUpperCase()}
                        />
                        <Box component="span" sx={{ fontWeight: 500, textTransform: "capitalize" }}>
                          {provider}
                        </Box>
                        {providerFilter === provider && (
                          <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 16, ml: "auto" }}>
                            check
                          </Box>
                        )}
                      </Box>
                    ))}
                  </Box>
                </Box>
              </>
            )}
          </Box>
          <Box
            component="select"
            value={accountFilter}
            onChange={(event) => {
              const nextValue = event.target.value;
              if (shouldResetPage(accountFilter, nextValue)) {
                setPage(1);
              }
              setAccountFilter(nextValue);
            }}
            aria-label="Filter accounts by status"
            sx={controlSx}
          >
            {ACCOUNT_FILTER_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </Box>

          {providerFilter === "codex" && (
            <Box
              component="select"
              value={quotaSortMode}
              onChange={(event) => setQuotaSortMode(event.target.value)}
              aria-label="Sort Codex quotas by remaining"
              sx={controlSx}
            >
              {QUOTA_SORT_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </Box>
          )}

          {isMultiModelProvider(providerFilter) && modelOptions.length > 0 && (
            <Box
              component="select"
              value={modelFilter}
              onChange={(event) => setModelFilter(event.target.value)}
              aria-label="Filter quotas by model"
              sx={controlSx}
            >
              <option value="all">All models</option>
              {modelOptions.map((model) => (
                <option key={model.id} value={model.id}>
                  {model.name}
                </option>
              ))}
            </Box>
          )}

          <Button
            type="button"
            variant="outlined"
            onClick={() => setExpiringFirst((prev) => !prev)}
            aria-pressed={expiringFirst}
            title="Sort accounts by earliest quota reset time"
            sx={{
              ...compactButtonSx,
              flexShrink: 0,
              ...(expiringFirst
                ? {
                    borderColor: "rgba(245, 158, 11, 0.4)",
                    bgcolor: "rgba(245, 158, 11, 0.1)",
                    color: "#f59e0b",
                    "&:hover": { bgcolor: "rgba(245, 158, 11, 0.16)", borderColor: "rgba(245, 158, 11, 0.4)" },
                  }
                : {}),
            }}
          >
            <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 14 }}>
              hourglass_top
            </Box>
            <Box component="span" sx={{ display: { xs: "none", sm: "inline" } }}>Expiring first</Box>
          </Button>

          {/* Bulk: disable depleted */}
          <Button
            type="button"
            variant="outlined"
            onClick={handleDisableDepleted}
            disabled={bulkToggling}
            title="Disable connections with depleted quota on the current page"
            sx={{
              ...compactButtonSx,
              flexShrink: 0,
              borderColor: "rgba(239, 68, 68, 0.3)",
              color: "#ef4444",
              "&:hover": { bgcolor: "rgba(239, 68, 68, 0.1)", borderColor: "rgba(239, 68, 68, 0.3)" },
            }}
          >
            <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 14 }}>block</Box>
            <Box component="span" sx={{ display: { xs: "none", sm: "inline" } }}>Turn off Empty</Box>
          </Button>

          {/* Bulk: enable available */}
          <Button
            type="button"
            variant="outlined"
            onClick={handleEnableAvailable}
            disabled={bulkToggling}
            title="Enable connections that still have quota on the current page"
            sx={{
              ...compactButtonSx,
              flexShrink: 0,
              borderColor: "rgba(16, 185, 129, 0.3)",
              color: "#10b981",
              "&:hover": { bgcolor: "rgba(16, 185, 129, 0.1)", borderColor: "rgba(16, 185, 129, 0.3)" },
            }}
          >
            <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 14 }}>
              check_circle
            </Box>
            <Box component="span" sx={{ display: { xs: "none", sm: "inline" } }}>Turn on Available</Box>
          </Button>

          {/* Bulk: delete Codex 401 unavailable */}
          {providerFilter === "codex" && (
            <Button
              type="button"
              variant="outlined"
              onClick={handleBulkDeleteCodexUnavailable}
              disabled={bulkDeleting || codexUnavailableIds.length === 0}
              title={
                codexUnavailableIds.length > 0
                  ? `Delete ${codexUnavailableIds.length} Codex connection(s) with "Usage API temporarily unavailable (401)"`
                  : "No Codex 401 unavailable connections on current page"
              }
              sx={{
                ...compactButtonSx,
                flexShrink: 0,
                borderColor: "rgba(239, 68, 68, 0.4)",
                bgcolor: "rgba(239, 68, 68, 0.1)",
                color: (theme) => (theme.palette.mode === "dark" ? "#f87171" : "#dc2626"),
                "&:hover": { bgcolor: "rgba(239, 68, 68, 0.2)", borderColor: "rgba(239, 68, 68, 0.4)" },
                "&.Mui-disabled": { opacity: 0.4 },
              }}
            >
              <Box
                component="span"
                className="material-symbols-outlined"
                sx={{ fontSize: 14, animation: bulkDeleting ? "spin 1s linear infinite" : "none" }}
              >
                {bulkDeleting ? "progress_activity" : "delete_sweep"}
              </Box>
              <Box component="span" sx={{ display: { xs: "none", sm: "inline" } }}>
                Delete 401 ({codexUnavailableIds.length})
              </Box>
              <Box component="span" sx={{ display: { xs: "inline", sm: "none" } }}>
                401 ({codexUnavailableIds.length})
              </Box>
            </Button>
          )}

          {/* Auto-refresh toggle */}
          <Button
            variant="outlined"
            onClick={() => setAutoRefresh((prev) => !prev)}
            title={autoRefresh ? "Disable auto-refresh" : "Enable auto-refresh"}
            sx={{ ...compactButtonSx, flexShrink: 0, bgcolor: "transparent" }}
          >
            <Box
              component="span"
              className="material-symbols-outlined"
              sx={{ fontSize: 14, color: autoRefresh ? "primary.main" : "text.secondary" }}
            >
              {autoRefresh ? "toggle_on" : "toggle_off"}
            </Box>
            <Box component="span" sx={{ display: { xs: "none", sm: "inline" }, color: "text.primary" }}>
              Auto-refresh
            </Box>
            {autoRefresh && (
              <Box
                component="span"
                sx={{ fontSize: 10, color: "text.secondary", fontVariantNumeric: "tabular-nums" }}
              >
                ({countdown}s)
              </Box>
            )}
          </Button>

          {/* Refresh all button */}
          <Button
            type="button"
            variant="outlined"
            onClick={() => refreshAll(true)}
            disabled={refreshingAll}
            title="Refresh all"
            sx={{ ...compactButtonSx, flexShrink: 0, bgcolor: "transparent", px: 1 }}
          >
            <Box
              component="span"
              className="material-symbols-outlined"
              sx={{ fontSize: 14, animation: refreshingAll ? "spin 1s linear infinite" : "none" }}
            >
              refresh
            </Box>
          </Button>
        </Box>
      </Box>

      {/* Provider cards: 2 columns, compact */}
      {expiringFirst && (
        <Box
          sx={{
            borderRadius: 3,
            border: 1,
            borderColor: "rgba(245, 158, 11, 0.2)",
            bgcolor: "rgba(245, 158, 11, 0.1)",
            px: 1.5,
            py: 1,
            fontSize: 12,
            color: (theme) => (theme.palette.mode === "dark" ? "#fcd34d" : "#b45309"),
          }}
        >
          Expiring-first currently reorders accounts inside the current page.
          Cross-page ordering still follows backend pagination.
        </Box>
      )}

      <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", md: "repeat(2, minmax(0, 1fr))" }, gap: 1.5 }}>
        {sortedConnections.map((conn) => {
          const quota = quotaData[conn.id];
          const isLoading = loading[conn.id];
          const error = errors[conn.id];

          // Use table layout for all providers
          const isInactive = conn.isActive === false;
          const isCodex = conn.provider === "codex";
          const claudeReset = conn.provider === "claude" ? quota?.raw?.resetCredits : null;
          const resetLabel = isCodex ? "Codex reset credit" : "Claude limit reset";
          const resetCreditCount = getCodexResetCreditCount(quota);
          const isResettingLimit = resettingLimitId === conn.id;
          const rowBusy = deletingId === conn.id || togglingId === conn.id || isResettingLimit;

          return (
            <Card
              key={conn.id}
              padding="none"
              sx={{ minWidth: 0, opacity: isInactive ? 0.6 : 1 }}
            >
              <Box sx={{ px: 1.5, py: 1, borderBottom: 1, borderColor: borderSubtle }}>
                <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
                  <Box sx={{ display: "flex", alignItems: "center", gap: 1, minWidth: 0 }}>
                    <Box sx={{ width: 32, height: 32, flexShrink: 0, borderRadius: 1, display: "flex", alignItems: "center", justifyContent: "center", overflow: "hidden" }}>
                      <ProviderIcon
                        src={`/providers/${conn.provider}.webp`}
                        alt={conn.provider}
                        size={32}
                        fallbackText={
                          conn.provider?.slice(0, 2).toUpperCase() || "PR"
                        }
                      />
                    </Box>
                    <Box sx={{ minWidth: 0 }}>
                      <Typography
                        component="h3"
                        noWrap
                        sx={{ fontSize: 14, fontWeight: 600, color: "text.primary", textTransform: "capitalize" }}
                      >
                        {conn.provider}
                      </Typography>
                      {getConnectionLabel(conn) ? (
                        <Typography noWrap sx={{ fontSize: 12, color: "text.secondary" }}>
                          {getConnectionLabel(conn)}
                        </Typography>
                      ) : null}
                      {getConnectionSecondaryLabel(conn) ? (
                        <Typography noWrap sx={{ fontSize: 11, color: "text.secondary", opacity: 0.8 }}>
                          {getConnectionSecondaryLabel(conn)}
                        </Typography>
                      ) : null}
                      {conn.provider === "kiro" && (
                        <Box sx={{ mt: 0.5, display: "flex", flexWrap: "wrap", alignItems: "center", gap: 0.5 }}>
                          <Box
                            component="span"
                            sx={{
                              borderRadius: 999,
                              bgcolor: "rgba(229, 106, 74, 0.1)",
                              px: 1,
                              py: 0.25,
                              fontSize: 10,
                              fontWeight: 600,
                              color: (theme) => (theme.palette.mode === "dark" ? "#f0a68f" : "#cc5236"),
                            }}
                          >
                            {kiroMethodLabel(conn)}
                          </Box>
                          {kiroRegion(conn) && (
                            <Box
                              component="span"
                              sx={{
                                borderRadius: 999,
                                bgcolor: "rgba(59, 130, 246, 0.1)",
                                px: 1,
                                py: 0.25,
                                fontSize: 10,
                                fontWeight: 600,
                                color: (theme) => (theme.palette.mode === "dark" ? "#60a5fa" : "#2563eb"),
                              }}
                            >
                              {kiroRegion(conn)}
                            </Box>
                          )}
                          <Box
                            component="span"
                            sx={{
                              borderRadius: 999,
                              px: 1,
                              py: 0.25,
                              fontSize: 10,
                              fontWeight: 600,
                              ...(isInactive
                                ? {
                                    bgcolor: (theme) => (theme.palette.mode === "dark" ? "#303030" : "#f4f4f5"),
                                    color: "text.secondary",
                                  }
                                : conn.testStatus === "active" || conn.testStatus === "success"
                                  ? {
                                      bgcolor: "rgba(34, 197, 94, 0.1)",
                                      color: (theme) => (theme.palette.mode === "dark" ? "#4ade80" : "#16a34a"),
                                    }
                                  : conn.testStatus === "error" || conn.testStatus === "expired" || conn.testStatus === "unavailable"
                                    ? {
                                        bgcolor: "rgba(239, 68, 68, 0.1)",
                                        color: (theme) => (theme.palette.mode === "dark" ? "#f87171" : "#dc2626"),
                                      }
                                    : {
                                        bgcolor: (theme) => (theme.palette.mode === "dark" ? "#303030" : "#f4f4f5"),
                                        color: "text.secondary",
                                      }),
                            }}
                          >
                            {isInactive ? "disabled" : conn.testStatus || "unknown"}
                          </Box>
                          {conn.providerSpecificData?.profileArn && (
                            <Box
                              component="button"
                              type="button"
                              onClick={() => copy(conn.providerSpecificData.profileArn, conn.id)}
                              title={conn.providerSpecificData.profileArn}
                              sx={{
                                display: "inline-flex",
                                maxWidth: "100%",
                                alignItems: "center",
                                gap: 0.5,
                                borderRadius: 999,
                                border: 1,
                                borderColor: borderFaint,
                                bgcolor: "transparent",
                                px: 1,
                                py: 0.25,
                                fontSize: 10,
                                color: "text.secondary",
                                cursor: "pointer",
                                transition: "color 150ms ease",
                                "&:hover": { color: "primary.main" },
                              }}
                            >
                              <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 12 }}>
                                {copied === conn.id ? "check" : "content_copy"}
                              </Box>
                              <Box component="code" sx={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontFamily: "monospace" }}>
                                {conn.providerSpecificData.profileArn}
                              </Box>
                            </Box>
                          )}
                        </Box>
                      )}
                    </Box>
                  </Box>

                  <Box sx={{ display: "flex", alignItems: "center", gap: 0.5, flexShrink: 0 }}>
                    {(isCodex || claudeReset) && (
                      <>
                        <Tooltip
                          text={
                            resetCreditCount > 0
                              ? claudeReset
                                ? `Use your reset now (${resetCreditCount} left, use by ${formatCreditDate(claudeReset.expiresAt)}) · refills ${formatClaudeResetClears(claudeReset.clears)}`
                                : `Use one ${resetLabel}. Available: ${resetCreditCount}`
                              : `No ${resetLabel}s available`
                          }
                        >
                          <Button
                            type="button"
                            variant="outlined"
                            onClick={() => setResetConfirmState({ connection: conn, resetCreditCount })}
                            disabled={resetCreditCount <= 0 || isLoading || rowBusy}
                            aria-label={
                              resetCreditCount > 0
                                ? `Use one ${resetLabel}. ${resetCreditCount} available.`
                                : `No ${resetLabel}s available`
                            }
                            sx={{
                              height: 32,
                              minWidth: 40,
                              px: 1,
                              gap: 0.5,
                              fontSize: 11,
                              fontWeight: 500,
                              fontVariantNumeric: "tabular-nums",
                              borderRadius: 2,
                              "&.Mui-disabled": { opacity: 0.6 },
                              ...(resetCreditCount > 0
                                ? {
                                    borderColor: "rgba(229, 106, 74, 0.3)",
                                    bgcolor: "rgba(229, 106, 74, 0.05)",
                                    color: "primary.main",
                                    "&:hover": { bgcolor: "rgba(229, 106, 74, 0.1)", borderColor: "rgba(229, 106, 74, 0.3)" },
                                  }
                                : {
                                    borderColor: borderSubtle,
                                    bgcolor: bgSubtle,
                                    color: "text.secondary",
                                  }),
                            }}
                          >
                            <Box
                              component="span"
                              className="material-symbols-outlined"
                              sx={{ fontSize: 15, animation: isResettingLimit ? "spin 1s linear infinite" : "none" }}
                            >
                              {isResettingLimit ? "progress_activity" : "restart_alt"}
                            </Box>
                            <Box component="span">{resetCreditCount}</Box>
                          </Button>
                        </Tooltip>
                        <Tooltip text={isCodex ? "View Codex reset credit expiry" : "View Claude Code reset expiry"}>
                          <IconButton
                            type="button"
                            onClick={() => (isCodex ? handleViewCodexResetCredits(conn) : handleViewClaudeResets(conn, claudeReset))}
                            disabled={isLoading || rowBusy}
                            aria-label={isCodex ? "View Codex reset credit expiry" : "View Claude Code reset expiry"}
                            sx={rowIconSx}
                          >
                            <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 17 }}>schedule</Box>
                          </IconButton>
                        </Tooltip>
                      </>
                    )}
                    {AUTO_PING_SETTINGS_KEYS[conn.provider] && conn.authType === "oauth" && (
                      <Tooltip text={AUTO_PING_TOOLTIPS[conn.provider]}>
                        <IconButton
                          type="button"
                          onClick={() => toggleAutoPing(conn.id, conn.provider, !(autoPingMaps[conn.provider]?.[conn.id] === true))}
                          aria-label="Toggle auto-ping"
                          sx={{
                            ...rowIconSx,
                            color: autoPingMaps[conn.provider]?.[conn.id] === true ? "primary.main" : "text.secondary",
                          }}
                        >
                          <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 18 }}>bolt</Box>
                        </IconButton>
                      </Tooltip>
                    )}
                    <Tooltip text="Refresh quota">
                      <IconButton
                        type="button"
                        onClick={() => refreshProvider(conn.id, conn.provider)}
                        disabled={isLoading || rowBusy}
                        aria-label="Refresh quota"
                        sx={rowIconSx}
                      >
                        <Box
                          component="span"
                          className="material-symbols-outlined"
                          sx={{ fontSize: 18, color: "text.secondary", animation: isLoading ? "spin 1s linear infinite" : "none" }}
                        >
                          refresh
                        </Box>
                      </IconButton>
                    </Tooltip>
                    <Tooltip text="Edit connection">
                      <IconButton
                        type="button"
                        onClick={() => {
                          setSelectedConnection(conn);
                          setShowEditModal(true);
                        }}
                        disabled={rowBusy}
                        aria-label="Edit connection"
                        sx={{
                          ...rowIconSx,
                          "&:hover": { bgcolor: bgHover, color: "primary.main" },
                        }}
                      >
                        <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 18 }}>
                          edit
                        </Box>
                      </IconButton>
                    </Tooltip>
                    <Tooltip text="Delete connection">
                      <IconButton
                        type="button"
                        onClick={() => handleDeleteConnection(conn.id)}
                        disabled={rowBusy}
                        aria-label="Delete connection"
                        sx={{
                          ...rowIconSx,
                          color: "#ef4444",
                          "&:hover": { bgcolor: "rgba(239, 68, 68, 0.1)" },
                        }}
                      >
                        <Box
                          component="span"
                          className="material-symbols-outlined"
                          sx={{ fontSize: 18, animation: deletingId === conn.id ? "pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite" : "none" }}
                        >
                          delete
                        </Box>
                      </IconButton>
                    </Tooltip>
                    <Box
                      sx={{ display: "inline-flex", alignItems: "center", pl: 0.25 }}
                      title={
                        (conn.isActive ?? true)
                          ? "Disable connection"
                          : "Enable connection"
                      }
                    >
                      <Toggle
                        size="sm"
                        checked={conn.isActive ?? true}
                        disabled={rowBusy}
                        onChange={(nextActive) =>
                          handleToggleConnectionActive(conn.id, nextActive)
                        }
                      />
                    </Box>
                  </Box>
                </Box>
              </Box>

              <Box sx={{ px: 1, py: 0.75 }}>
                {isLoading ? (
                  <Box sx={{ textAlign: "center", py: 2.5, color: "text.secondary" }}>
                    <Box
                      component="span"
                      className="material-symbols-outlined"
                      sx={{ fontSize: 28, animation: "spin 1s linear infinite" }}
                    >
                      progress_activity
                    </Box>
                  </Box>
                ) : error ? (
                  <Box sx={{ textAlign: "center", py: 2.5 }}>
                    <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 28, color: "#ef4444" }}>
                      error
                    </Box>
                    <Typography sx={{ mt: 0.75, fontSize: 12, color: "text.secondary" }}>{error}</Typography>
                  </Box>
                ) : quota?.message ? (
                  <Box sx={{ textAlign: "center", py: 2.5 }}>
                    <Typography sx={{ fontSize: 12, color: "text.secondary" }}>{quota.message}</Typography>
                  </Box>
                ) : (
                  <QuotaTable
                    quotas={filterQuotasByModel(quota?.quotas, modelFilter)}
                    compact
                    sortMode="default"
                    showSortLabel={
                      conn.provider === "codex" && quotaSortMode !== "default"
                    }
                  />
                )}
              </Box>
            </Card>
          );
        })}
      </Box>

      <Box
        sx={{
          borderRadius: 3,
          border: 1,
          borderColor: borderSubtle,
          bgcolor: bgSubtle,
          px: 1.5,
          py: 1,
        }}
      >
        <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
          <Typography sx={{ fontSize: 12, color: "text.secondary" }}>{connectionsPageSummary}</Typography>
          <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 1 }}>
            <Box
              component="select"
              value={isCustomPageSize ? "custom" : String(pageSize)}
              onChange={(event) => {
                const nextValue = event.target.value;
                if (nextValue === "custom") return;
                const nextPageSize = Number.parseInt(nextValue, 10);
                if (Number.isFinite(nextPageSize)) {
                  setPage(1);
                  setPageSize(nextPageSize);
                  setCustomPageSizeInput(String(nextPageSize));
                }
              }}
              aria-label="Accounts per page"
              sx={controlSx}
            >
              {ACCOUNT_PAGE_SIZE_OPTIONS.map((option) => (
                <option key={option} value={String(option)}>
                  {option} / page
                </option>
              ))}
              <option value="custom">Custom</option>
            </Box>
            <Box
              component="input"
              type="number"
              min="1"
              max={String(ACCOUNT_PAGE_SIZE_MAX)}
              inputMode="numeric"
              value={customPageSizeInput}
              onChange={(event) => setCustomPageSizeInput(event.target.value)}
              onBlur={() => {
                const parsedValue = Number.parseInt(customPageSizeInput, 10);
                if (!Number.isFinite(parsedValue)) {
                  setCustomPageSizeInput(String(pageSize));
                  return;
                }
                const nextPageSize = Math.min(ACCOUNT_PAGE_SIZE_MAX, Math.max(1, parsedValue));
                setPage(1);
                setPageSize(nextPageSize);
                setCustomPageSizeInput(String(nextPageSize));
              }}
              onKeyDown={(event) => {
                if (event.key !== "Enter") return;
                const parsedValue = Number.parseInt(customPageSizeInput, 10);
                if (!Number.isFinite(parsedValue)) {
                  setCustomPageSizeInput(String(pageSize));
                  return;
                }
                const nextPageSize = Math.min(ACCOUNT_PAGE_SIZE_MAX, Math.max(1, parsedValue));
                setPage(1);
                setPageSize(nextPageSize);
                setCustomPageSizeInput(String(nextPageSize));
              }}
              aria-label="Custom accounts per page"
              placeholder="Custom"
              sx={{ ...controlSx, width: 80 }}
            />
            <Typography sx={{ fontSize: 12, color: "text.secondary" }}>Page {pagination.page} / {pagination.totalPages}</Typography>
          </Box>
          <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
            <Button
              type="button"
              variant="outlined"
              onClick={() => setPage(1)}
              disabled={
                pagination.page <= 1 || connectionsLoading || refreshingAll
              }
              sx={{ ...compactButtonSx, px: 1.5 }}
            >
              First Page
            </Button>
            <IconButton
              type="button"
              onClick={() =>
                setPage((currentPage) => Math.max(1, currentPage - 1))
              }
              disabled={
                pagination.page <= 1 || connectionsLoading || refreshingAll
              }
              aria-label="Previous accounts page"
              sx={{ ...rowIconSx, border: 1, borderColor: borderSubtle, color: "text.primary" }}
            >
              <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 16 }}>
                chevron_left
              </Box>
            </IconButton>
            <IconButton
              type="button"
              onClick={() =>
                setPage((currentPage) =>
                  Math.min(pagination.totalPages, currentPage + 1),
                )
              }
              disabled={
                pagination.page >= pagination.totalPages ||
                connectionsLoading ||
                refreshingAll
              }
              aria-label="Next accounts page"
              sx={{ ...rowIconSx, border: 1, borderColor: borderSubtle, color: "text.primary" }}
            >
              <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 16 }}>
                chevron_right
              </Box>
            </IconButton>
            <Button
              type="button"
              variant="outlined"
              onClick={() => setPage(pagination.totalPages)}
              disabled={
                pagination.page >= pagination.totalPages ||
                connectionsLoading ||
                refreshingAll
              }
              sx={{ ...compactButtonSx, px: 1.5 }}
            >
              Last Page
            </Button>
          </Box>
        </Box>
      </Box>

      <ConfirmModal
        isOpen={Boolean(resetConfirmState)}
        onClose={() => {
          if (!resettingLimitId) setResetConfirmState(null);
        }}
        onConfirm={async () => {
          const connection = resetConfirmState?.connection;
          if (!connection) return;
          await handleResetCodexLimit(connection.id, connection.provider);
          setResetConfirmState(null);
        }}
        title={resetConfirmState?.connection?.provider === "claude" ? "Reset Claude limits?" : "Reset Codex limit?"}
        message={resetConfirmState?.connection?.provider === "claude"
          ? `Refills your ${formatClaudeResetClears(quotaData[resetConfirmState.connection.id]?.raw?.resetCredits?.clears)} now for ${getConnectionLabel(resetConfirmState.connection) || "this account"} · your weekly reset day stays ${formatCreditDate(quotaData[resetConfirmState.connection.id]?.raw?.resetCredits?.weeklyResetsAt)}. This cannot be undone. Resets left: ${resetConfirmState.resetCreditCount ?? 0}.`
          : `Use 1 Codex reset credit for ${getConnectionLabel(resetConfirmState?.connection || {}) || "this account"}. This cannot be undone. Remaining credits: ${resetConfirmState?.resetCreditCount ?? 0}.`}
        confirmText="Reset limit"
        cancelText="Cancel"
        variant="danger"
        loading={Boolean(resettingLimitId)}
      />

      <Dialog
        open={resetCreditsOpen}
        onClose={() => setResetCreditsState(null)}
        maxWidth="md"
        fullWidth
      >
        {resetCreditsState && (
          <>
            <DialogTitle
              sx={{
                display: "flex",
                alignItems: "flex-start",
                justifyContent: "space-between",
                gap: 1.5,
                borderBottom: 1,
                borderColor: borderSubtle,
                bgcolor: (theme) =>
                  theme.palette.mode === "dark" ? "rgba(255, 255, 255, 0.04)" : "rgba(0, 0, 0, 0.03)",
                px: 2,
                py: 1.5,
              }}
            >
              <Box sx={{ minWidth: 0 }}>
                <Typography sx={{ fontSize: 16, fontWeight: 600, color: "text.primary" }}>
                  {resetCreditsState.data?.kind === "claude" ? "Claude Code Limit Resets" : "Codex Reset Credit Expiry"}
                </Typography>
                <Typography noWrap sx={{ mt: 0.25, fontSize: 12, color: "text.secondary" }}>
                  {getConnectionLabel(resetCreditsState.connection) || (resetCreditsState.data?.kind === "claude" ? "Claude account" : "Codex account")}
                </Typography>
              </Box>
              <IconButton
                type="button"
                onClick={() => setResetCreditsState(null)}
                aria-label="Close reset credit expiry modal"
                sx={{
                  width: 32,
                  height: 32,
                  borderRadius: 2,
                  color: "text.secondary",
                  "&:hover": { bgcolor: bgHover, color: "text.primary" },
                }}
              >
                <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 18 }}>close</Box>
              </IconButton>
            </DialogTitle>

            <DialogContent sx={{ p: 2, bgcolor: "background.paper" }}>
              <Box sx={{ maxHeight: "70vh", overflow: "auto" }}>
                {resetCreditsState.loading ? (
                  <Box sx={{ display: "flex", alignItems: "center", justifyContent: "center", gap: 1, py: 5, fontSize: 14, color: "text.secondary" }}>
                    <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 20, animation: "spin 1s linear infinite" }}>progress_activity</Box>
                    Loading reset credits...
                  </Box>
                ) : resetCreditsState.error ? (
                  <Box
                    sx={{
                      borderRadius: 3,
                      border: 1,
                      borderColor: "rgba(239, 68, 68, 0.2)",
                      bgcolor: "rgba(239, 68, 68, 0.1)",
                      px: 1.5,
                      py: 1,
                      fontSize: 14,
                      color: (theme) => (theme.palette.mode === "dark" ? "#fca5a5" : "#dc2626"),
                    }}
                  >
                    {resetCreditsState.error}
                  </Box>
                ) : resetCreditsState.data?.kind === "claude" && resetCreditsState.data.grants?.length ? (
                  <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
                    <Box
                      sx={{
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "space-between",
                        borderRadius: 3,
                        border: 1,
                        borderColor: borderSubtle,
                        bgcolor: bgSubtle,
                        px: 1.5,
                        py: 1,
                        fontSize: 12,
                        color: "text.secondary",
                      }}
                    >
                      <Box component="span">{resetCreditsState.data.availableCount ?? 0} reset{resetCreditsState.data.availableCount === 1 ? "" : "s"} left</Box>
                      <Box component="span">Weekly reset day: {formatCreditDate(resetCreditsState.data.weeklyResetsAt)}</Box>
                    </Box>
                    <Box sx={{ overflowX: "auto", borderRadius: 3, border: 1, borderColor: borderSubtle }}>
                      <Table size="small" sx={{ minWidth: 560 }}>
                        <TableHead>
                          <TableRow>
                            {["Reset", "Left", "Refills", "Use By", "Remaining"].map((heading) => (
                              <TableCell
                                key={heading}
                                sx={{
                                  px: 1.5,
                                  py: 1,
                                  fontSize: 12,
                                  fontWeight: 500,
                                  textTransform: "uppercase",
                                  letterSpacing: "0.05em",
                                  color: "text.secondary",
                                  bgcolor: (theme) =>
                                    theme.palette.mode === "dark" ? "rgba(255, 255, 255, 0.04)" : "rgba(0, 0, 0, 0.03)",
                                  borderBottom: 1,
                                  borderColor: borderFaint,
                                }}
                              >
                                {heading}
                              </TableCell>
                            ))}
                          </TableRow>
                        </TableHead>
                        <TableBody>
                          {(resetCreditsState.data.grants || []).map((grant) => (
                            <TableRow key={grant.id}>
                              <TableCell sx={{ px: 1.5, py: 1, borderTop: 1, borderColor: borderFaint }}>
                                <Box sx={{ color: "text.primary" }}>{grant.label || grant.id}</Box>
                                <Box
                                  component="span"
                                  sx={{
                                    mt: 0.5,
                                    display: "inline-block",
                                    borderRadius: 999,
                                    bgcolor: "rgba(229, 106, 74, 0.1)",
                                    px: 1,
                                    py: 0.25,
                                    fontSize: 12,
                                    fontWeight: 500,
                                    color: "primary.main",
                                  }}
                                >
                                  {claudeGrantStatus(grant)}
                                </Box>
                              </TableCell>
                              <TableCell sx={{ px: 1.5, py: 1, whiteSpace: "nowrap", fontWeight: 500, fontVariantNumeric: "tabular-nums", color: "text.primary", borderTop: 1, borderColor: borderFaint }}>{grant.resetsLeft} / {grant.resetsTotal}</TableCell>
                              <TableCell sx={{ px: 1.5, py: 1, color: "text.secondary", borderTop: 1, borderColor: borderFaint }}>{formatClaudeResetClears(grant.clears)}</TableCell>
                              <TableCell sx={{ px: 1.5, py: 1, color: "text.primary", borderTop: 1, borderColor: borderFaint }}>{formatCreditDate(grant.endsAt)}</TableCell>
                              <TableCell sx={{ px: 1.5, py: 1, whiteSpace: "nowrap", fontWeight: 500, color: "text.primary", borderTop: 1, borderColor: borderFaint }}>{formatTimeRemaining(grant.endsAt)}</TableCell>
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    </Box>
                  </Box>
                ) : resetCreditsState.data?.credits?.length ? (
                  <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
                    <Box
                      sx={{
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "space-between",
                        borderRadius: 3,
                        border: 1,
                        borderColor: borderSubtle,
                        bgcolor: bgSubtle,
                        px: 1.5,
                        py: 1,
                        fontSize: 12,
                        color: "text.secondary",
                      }}
                    >
                      <Box component="span">{resetCreditsState.data.credits.length} reset credit{resetCreditsState.data.credits.length === 1 ? "" : "s"}</Box>
                      <Box component="span">{resetCreditsState.data.availableCount ?? 0} available</Box>
                    </Box>
                    <Box sx={{ overflowX: "auto", borderRadius: 3, border: 1, borderColor: borderSubtle }}>
                      <Table size="small" sx={{ minWidth: 560 }}>
                        <TableHead>
                          <TableRow>
                            {["Status", "Granted At", "Expires At", "Remaining"].map((heading) => (
                              <TableCell
                                key={heading}
                                sx={{
                                  px: 1.5,
                                  py: 1,
                                  fontSize: 12,
                                  fontWeight: 500,
                                  textTransform: "uppercase",
                                  letterSpacing: "0.05em",
                                  color: "text.secondary",
                                  bgcolor: (theme) =>
                                    theme.palette.mode === "dark" ? "rgba(255, 255, 255, 0.04)" : "rgba(0, 0, 0, 0.03)",
                                  borderBottom: 1,
                                  borderColor: borderFaint,
                                }}
                              >
                                {heading}
                              </TableCell>
                            ))}
                          </TableRow>
                        </TableHead>
                        <TableBody>
                          {resetCreditsState.data.credits.map((credit, index) => (
                            <TableRow key={`${credit.status}-${credit.expiresAt || index}`}>
                              <TableCell sx={{ px: 1.5, py: 1, borderTop: 1, borderColor: borderFaint }}>
                                <Box
                                  component="span"
                                  sx={{
                                    borderRadius: 999,
                                    bgcolor: "rgba(229, 106, 74, 0.1)",
                                    px: 1,
                                    py: 0.25,
                                    fontSize: 12,
                                    fontWeight: 500,
                                    color: "primary.main",
                                  }}
                                >
                                  {credit.status || "unknown"}
                                </Box>
                              </TableCell>
                              <TableCell sx={{ px: 1.5, py: 1, color: "text.secondary", borderTop: 1, borderColor: borderFaint }}>{formatCreditDate(credit.grantedAt)}</TableCell>
                              <TableCell sx={{ px: 1.5, py: 1, color: "text.primary", borderTop: 1, borderColor: borderFaint }}>{formatCreditDate(credit.expiresAt)}</TableCell>
                              <TableCell sx={{ px: 1.5, py: 1, fontWeight: 500, color: "text.primary", borderTop: 1, borderColor: borderFaint }}>{formatTimeRemaining(credit.expiresAt)}</TableCell>
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    </Box>
                  </Box>
                ) : (
                  <Box
                    sx={{
                      borderRadius: 3,
                      border: 1,
                      borderColor: borderSubtle,
                      bgcolor: bgSubtle,
                      px: 1.5,
                      py: 4,
                      textAlign: "center",
                      fontSize: 14,
                      color: "text.secondary",
                    }}
                  >
                    {resetCreditsState.data?.kind === "claude" ? "No limit resets available for this account." : "No reset credit details returned for this account."}
                  </Box>
                )}
              </Box>
            </DialogContent>
          </>
        )}
      </Dialog>

      <EditConnectionModal
        isOpen={showEditModal}
        connection={selectedConnection}
        proxyPools={proxyPools}
        onSave={handleUpdateConnection}
        onClose={() => {
          setShowEditModal(false);
          setSelectedConnection(null);
        }}
      />
    </Box>
  );
}
