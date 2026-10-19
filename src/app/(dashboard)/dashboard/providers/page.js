"use client";

import { useState, useEffect, useMemo, useCallback } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import IconButton from "@mui/material/IconButton";
import Dialog from "@mui/material/Dialog";
import DialogTitle from "@mui/material/DialogTitle";
import DialogContent from "@mui/material/DialogContent";
import {
  Card,
  CardSkeleton,
  Badge,
  Button,
  Toggle,
} from "@/shared/components";
import ProviderIcon from "@/shared/components/ProviderIcon";
import { getProviderIconSrc } from "@/shared/utils/providerIcon";
import { getProviderAuthTypes } from "@/shared/utils/providerAuth";
import { OAUTH_PROVIDERS, APIKEY_PROVIDERS } from "@/shared/constants/config";
import {
  FREE_PROVIDERS,
  FREE_TIER_PROVIDERS,
  WEB_COOKIE_PROVIDERS,
  OPENAI_COMPATIBLE_PREFIX,
  ANTHROPIC_COMPATIBLE_PREFIX,
} from "@/shared/constants/providers";
import Link from "next/link";
import { getErrorCode, getRelativeTime } from "@/shared/utils";
import { useNotificationStore } from "@/store/notificationStore";
import { useHeaderSearchStore } from "@/store/headerSearchStore";
import ModelAvailabilityBadge from "./components/ModelAvailabilityBadge";
import AddCompatibleModal from "./components/AddCompatibleModal";
import { useCircuitBreakers } from "@/shared/hooks/useCircuitBreakers";
import CircuitBreakerBadge from "./components/CircuitBreakerBadge";

function getStatusDisplay(connected, error, errorCode) {
  const parts = [];
  if (connected > 0) {
    parts.push(
      <Badge key="connected" variant="success" size="sm" dot>
        {connected} Connected
      </Badge>,
    );
  }
  if (error > 0) {
    const errText = errorCode
      ? `${error} Error (${errorCode})`
      : `${error} Error`;
    parts.push(
      <Badge key="error" variant="error" size="sm" dot>
        {errText}
      </Badge>,
    );
  }
  if (parts.length === 0) {
    return <Box component="span" sx={{ color: "text.secondary" }}>No connections</Box>;
  }
  return parts;
}

function getConnectionErrorTag(connection) {
  if (!connection) return null;

  const explicitType = connection.lastErrorType;
  if (explicitType === "runtime_error") return "RUNTIME";
  if (
    explicitType === "upstream_auth_error" ||
    explicitType === "auth_missing" ||
    explicitType === "token_refresh_failed" ||
    explicitType === "token_expired"
  )
    return "AUTH";
  if (explicitType === "upstream_rate_limited") return "429";
  if (explicitType === "upstream_unavailable") return "5XX";
  if (explicitType === "network_error") return "NET";

  const numericCode = Number(connection.errorCode);
  if (Number.isFinite(numericCode) && numericCode >= 400)
    return String(numericCode);

  const fromMessage = getErrorCode(connection.lastError);
  if (fromMessage === "401" || fromMessage === "403") return "AUTH";
  if (fromMessage && fromMessage !== "ERR") return fromMessage;

  const msg = (connection.lastError || "").toLowerCase();
  if (
    msg.includes("runtime") ||
    msg.includes("not runnable") ||
    msg.includes("not installed")
  )
    return "RUNTIME";
  if (
    msg.includes("invalid api key") ||
    msg.includes("token invalid") ||
    msg.includes("revoked") ||
    msg.includes("unauthorized")
  )
    return "AUTH";

  return "ERR";
}

const APIKEY_INITIAL_VISIBLE = 20;

const cardGridSx = {
  display: "grid",
  gridTemplateColumns: {
    xs: "repeat(1, minmax(0,1fr))",
    sm: "repeat(2, minmax(0,1fr))",
    lg: "repeat(3, minmax(0,1fr))",
    xl: "repeat(4, minmax(0,1fr))",
  },
  gap: { xs: 1.5, sm: 2 },
};

const sectionSx = { display: "flex", flexDirection: "column", gap: 2 };

const sectionHeaderSx = {
  display: "flex",
  flexDirection: { xs: "column", sm: "row" },
  alignItems: { sm: "center" },
  justifyContent: { sm: "space-between" },
  gap: 1.5,
};

const sectionTitleSx = {
  fontSize: { xs: 18, sm: 20 },
  fontWeight: 600,
  display: "flex",
  alignItems: "center",
  gap: 1,
  lineHeight: 1.25,
  m: 0,
};

const testButtonSx = (active, disabled) => ({
  display: "flex",
  width: { xs: "100%", sm: "auto" },
  alignItems: "center",
  justifyContent: "center",
  gap: 0.75,
  borderRadius: "8px",
  border: 1,
  px: 1.5,
  py: { xs: 1, sm: 0.75 },
  fontSize: 12,
  fontWeight: 500,
  cursor: disabled ? "not-allowed" : "pointer",
  transition: "background-color 150ms ease, border-color 150ms ease, color 150ms ease",
  bgcolor: active ? "rgba(229,106,74,0.2)" : "background.default",
  borderColor: active ? "rgba(229,106,74,0.4)" : "divider",
  color: active ? "primary.main" : "text.secondary",
  animation: active ? "pulse 2s cubic-bezier(0.4,0,0.6,1) infinite" : "none",
  "&:hover": active || disabled ? {} : { color: "text.primary", borderColor: "rgba(229,106,74,0.4)" },
});

export default function ProvidersPage() {
  const [connections, setConnections] = useState([]);
  const [providerNodes, setProviderNodes] = useState([]);
  const [loading, setLoading] = useState(true);
  const [showAllApikey, setShowAllApikey] = useState(false);
  const [showAddCompatibleModal, setShowAddCompatibleModal] = useState(false);
  const [showAddAnthropicCompatibleModal, setShowAddAnthropicCompatibleModal] =
    useState(false);
  const [testingMode, setTestingMode] = useState(null);
  const [testResults, setTestResults] = useState(null);
  const notify = useNotificationStore();
  const searchQuery = useHeaderSearchStore((s) => s.query);
  const registerSearch = useHeaderSearchStore((s) => s.register);
  const unregisterSearch = useHeaderSearchStore((s) => s.unregister);
  const { getCircuitBreakerForProvider, resetCircuitBreaker } = useCircuitBreakers();

  useEffect(() => {
    registerSearch("Search providers...");
    return () => unregisterSearch();
  }, [registerSearch, unregisterSearch]);

  const matchSearch = (name) => {
    if (!searchQuery.trim()) return true;
    if (!name) return false;
    return name.toLowerCase().includes(searchQuery.trim().toLowerCase());
  };

  const sortByPriority = (entries, authType) =>
    [...entries].sort(([ka, a], [kb, b]) => {
      const pa = a.priority ?? 999;
      const pb = b.priority ?? 999;
      if (pa !== pb) return pa - pb;
      const sa = getProviderStats(ka, authType);
      const sb = getProviderStats(kb, authType);
      const ca = sa.connected > 0 ? 1 : 0;
      const cb = sb.connected > 0 ? 1 : 0;
      if (ca !== cb) return cb - ca;
      return (a.name || "").localeCompare(b.name || "");
    });

  const sortItemsByPriority = (items, authType) =>
    [...items].sort((a, b) => {
      const pa = a.priority ?? 999;
      const pb = b.priority ?? 999;
      if (pa !== pb) return pa - pb;
      const sa = getProviderStats(a.id, authType);
      const sb = getProviderStats(b.id, authType);
      const ca = sa.connected > 0 ? 1 : 0;
      const cb = sb.connected > 0 ? 1 : 0;
      if (ca !== cb) return cb - ca;
      return (a.name || "").localeCompare(b.name || "");
    });

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [connectionsRes, nodesRes] = await Promise.all([
          fetch("/api/providers", { cache: "no-store" }),
          fetch("/api/provider-nodes", { cache: "no-store" }),
        ]);

        const connectionsData = await connectionsRes.json();
        const nodesData = await nodesRes.json();
        if (connectionsRes.ok)
          setConnections(connectionsData.connections || []);
        if (nodesRes.ok) setProviderNodes(nodesData.nodes || []);
      } catch (error) {
        console.log("Error fetching data:", error);
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, []);

  const getProviderStats = useCallback((providerId, authType) => {
    const authTypes = Array.isArray(authType) ? authType : [authType];
    const providerConnections = connections.filter(
      (c) => c.provider === providerId && authTypes.includes(c.authType),
    );

    const getEffectiveStatus = (conn) => {
      const isCooldown = Object.entries(conn).some(
        ([k, v]) =>
          k.startsWith("modelLock_") && v && new Date(v).getTime() > Date.now(),
      );
      return conn.testStatus === "unavailable" && !isCooldown
        ? "active"
        : conn.testStatus;
    };

    const connected = providerConnections.filter((c) => {
      const status = getEffectiveStatus(c);
      return status === "active" || status === "success";
    }).length;

    const errorConns = providerConnections.filter((c) => {
      const status = getEffectiveStatus(c);
      return (
        status === "error" || status === "expired" || status === "unavailable"
      );
    });

    const error = errorConns.length;
    const total = providerConnections.length;
    const allDisabled =
      total > 0 && providerConnections.every((c) => c.isActive === false);

    const latestError = errorConns.sort(
      (a, b) => new Date(b.lastErrorAt || 0) - new Date(a.lastErrorAt || 0),
    )[0];
    const errorCode = latestError ? getConnectionErrorTag(latestError) : null;
    const errorTime = latestError?.lastErrorAt
      ? getRelativeTime(latestError.lastErrorAt)
      : null;

    return { connected, error, total, errorCode, errorTime, allDisabled };
  }, [connections]);

  // Toggle all connections for a provider on/off. authType may be a single
  // string or an array (kiro counts oauth + api_key/apikey together).
  const handleToggleProvider = async (providerId, authType, newActive) => {
    const authTypes = Array.isArray(authType) ? authType : [authType];
    const matches = (c) =>
      c.provider === providerId && authTypes.includes(c.authType);
    const providerConns = connections.filter(matches);
    setConnections((prev) =>
      prev.map((c) => (matches(c) ? { ...c, isActive: newActive } : c)),
    );
    await Promise.allSettled(
      providerConns.map((c) =>
        fetch(`/api/providers/${c.id}`, {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ isActive: newActive }),
        }),
      ),
    );
  };

  const handleBatchTest = async (mode, providerId = null) => {
    if (testingMode) return;
    setTestingMode(mode === "provider" ? providerId : mode);
    setTestResults(null);
    try {
      const res = await fetch("/api/providers/test-batch", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ mode, providerId }),
      });
      const data = await res.json();
      setTestResults(data);
      if (data.summary) {
        const { passed, failed, total } = data.summary;
        if (failed === 0) notify.success(`All ${total} tests passed`);
        else notify.warning(`${passed}/${total} passed, ${failed} failed`);
      }
    } catch (error) {
      setTestResults({ error: "Test request failed" });
      notify.error("Provider test failed");
    } finally {
      setTestingMode(null);
    }
  };

  const compatibleProviders = useMemo(() => {
    return providerNodes
      .filter((node) => node.type === "openai-compatible")
      .map((node) => ({
        id: node.id,
        name: node.name || "OpenAI Compatible",
        color: "#10A37F",
        textIcon: "OC",
        apiType: node.apiType,
      }))
      .filter((p) => matchSearch(p.name));
  }, [providerNodes, matchSearch]);

  const anthropicCompatibleProviders = useMemo(() => {
    return providerNodes
      .filter((node) => node.type === "anthropic-compatible")
      .map((node) => ({
        id: node.id,
        name: node.name || "Anthropic Compatible",
        color: "#D97757",
        textIcon: "AC",
      }))
      .filter((p) => matchSearch(p.name));
  }, [providerNodes, matchSearch]);

  const oauthEntries = useMemo(() => {
    return sortByPriority(
      Object.entries(OAUTH_PROVIDERS).filter(([, info]) => !info.hidden && matchSearch(info.name)),
      "oauth",
    );
  }, [matchSearch]);

  const freeEntries = useMemo(() => {
    return Object.entries(FREE_PROVIDERS)
      .filter(([, info]) => !info.hidden && matchSearch(info.name))
      .sort(([, a], [, b]) => (b.noAuth ? 1 : 0) - (a.noAuth ? 1 : 0));
  }, [matchSearch]);

  // Free Tier cards may be OAuth-only (e.g. Kimchi) or dual-auth. Use the
  // registry auth modes instead of assuming every card is API-key-only.
  const freeTierEntries = useMemo(() => {
    return Object.entries(FREE_TIER_PROVIDERS)
      .filter(
        ([, info]) =>
          !info.hidden &&
          matchSearch(info.name) &&
          (info.serviceKinds ?? ["llm"]).includes("llm"),
      )
      .sort(([ka, a], [kb, b]) => {
        const pa = a.priority ?? 999;
        const pb = b.priority ?? 999;
        if (pa !== pb) return pa - pb;
        const noAuthDiff = (b.noAuth ? 1 : 0) - (a.noAuth ? 1 : 0);
        if (noAuthDiff !== 0) return noAuthDiff;
        const ca = getProviderStats(ka, getProviderAuthTypes(a, ka)).connected > 0 ? 0 : 1;
        const cb = getProviderStats(kb, getProviderAuthTypes(b, kb)).connected > 0 ? 0 : 1;
        if (ca !== cb) return ca - cb;
        return (a.name || "").localeCompare(b.name || "");
      });
  }, [matchSearch, getProviderStats]);

  // API Key: connected providers first, then alphabetical by name
  const apikeyEntries = useMemo(() => {
    return Object.entries(APIKEY_PROVIDERS)
      .filter(
        ([, info]) =>
          !info.hidden &&
          (info.serviceKinds ?? ["llm"]).includes("llm") &&
          matchSearch(info.name),
      )
      .sort(([ka, a], [kb, b]) => {
        const ca = getProviderStats(ka, "apikey").total > 0 ? 0 : 1;
        const cb = getProviderStats(kb, "apikey").total > 0 ? 0 : 1;
        if (ca !== cb) return ca - cb;
        return (a.name || "").localeCompare(b.name || "");
      });
  }, [matchSearch, getProviderStats]);

  const webCookieEntries = useMemo(() => {
    return Object.entries(WEB_COOKIE_PROVIDERS)
      .filter(
        ([, info]) =>
          !info.hidden &&
          matchSearch(info.name),
      )
      .sort(([ka, a], [kb, b]) => {
        const ca = getProviderStats(ka, "cookie").total > 0 ? 0 : 1;
        const cb = getProviderStats(kb, "cookie").total > 0 ? 0 : 1;
        if (ca !== cb) return ca - cb;
        return (a.name || "").localeCompare(b.name || "");
      });
  }, [matchSearch, getProviderStats]);
  const isApikeySearching = !!searchQuery.trim();
  const visibleApikeyEntries =
    isApikeySearching || showAllApikey
      ? apikeyEntries
      : apikeyEntries.slice(0, APIKEY_INITIAL_VISIBLE);
  const hiddenApikeyCount = apikeyEntries.length - APIKEY_INITIAL_VISIBLE;

  if (loading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 4 }}>
        <CardSkeleton />
        <CardSkeleton />
      </Box>
    );
  }

  const hasAnyResult =
    oauthEntries.length > 0 ||
    freeEntries.length > 0 ||
    freeTierEntries.length > 0 ||
    apikeyEntries.length > 0 ||
    webCookieEntries.length > 0 ||
    compatibleProviders.length > 0 ||
    anthropicCompatibleProviders.length > 0;

  return (
    <Box sx={{ display: "flex", minWidth: 0, flexDirection: "column", gap: 3, px: { xs: 0.5, sm: 0 } }}>
      {!hasAnyResult && (
        <Box sx={{ textAlign: "center", py: 4, border: "1px dashed", borderColor: "divider", borderRadius: "14px" }}>
          <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 32, color: "text.secondary", mb: 1 }}>
            search_off
          </Icon>
          <Typography sx={{ color: "text.secondary", fontSize: 14 }}>No providers match your search</Typography>
        </Box>
      )}

      {/* Custom Providers (OpenAI/Anthropic Compatible) — dynamic */}
      <Box sx={sectionSx}>
        <Box sx={sectionHeaderSx}>
          <Typography component="h2" sx={sectionTitleSx}>
            Custom Providers (OpenAI/Anthropic Compatible){" "}
          </Typography>
          <Box sx={{ display: { xs: "grid", sm: "flex" }, gridTemplateColumns: "1fr", gap: 1, width: { sm: "auto" }, "& .MuiButton-root": { width: { xs: "100%", sm: "auto" } } }}>
            <Button
              size="sm"
              icon="add"
              onClick={() => setShowAddAnthropicCompatibleModal(true)}
            >
              Add Anthropic Compatible
            </Button>
            <Button
              size="sm"
              variant="secondary"
              icon="add"
              onClick={() => setShowAddCompatibleModal(true)}
            >
              Add OpenAI Compatible
            </Button>
          </Box>
        </Box>
        {compatibleProviders.length === 0 &&
        anthropicCompatibleProviders.length === 0 ? (
          <Box sx={{ display: "flex", alignItems: "center", justifyContent: "center", gap: 1, py: 1, border: "1px dashed", borderColor: "divider", borderRadius: "14px", color: "text.secondary", fontSize: 14 }}>
            <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18 }}>extension</Icon>
            <Box component="span">No custom providers — use buttons above to add OpenAI/Anthropic compatible endpoints</Box>
          </Box>
        ) : (
          <Box sx={cardGridSx}>
            {[...compatibleProviders, ...anthropicCompatibleProviders].map(
              (info) => (
                <ApiKeyProviderCard
                  key={info.id}
                  providerId={info.id}
                  provider={info}
                  stats={getProviderStats(info.id, "apikey")}
                  authType="compatible"
                  circuitBreaker={getCircuitBreakerForProvider(info.id)}
                  onResetCircuit={resetCircuitBreaker}
                  onToggle={(active) =>
                    handleToggleProvider(info.id, "apikey", active)
                  }
                />
              ),
            )}
          </Box>
        )}
      </Box>

      {/* OAuth Providers */}
      {oauthEntries.length > 0 && (
      <Box sx={sectionSx}>
        <Box sx={sectionHeaderSx}>
          <Typography component="h2" sx={sectionTitleSx}>
            OAuth Providers
          </Typography>
          <Box sx={{ display: "flex", width: { xs: "100%", sm: "auto" }, flexDirection: { xs: "column", sm: "row" }, alignItems: { sm: "center" }, gap: 1 }}>
            <ModelAvailabilityBadge />
            <Box
              component="button"
              onClick={() => handleBatchTest("oauth")}
              disabled={!!testingMode}
              sx={testButtonSx(testingMode === "oauth", !!testingMode)}
              title="Test all OAuth connections"
              aria-label="Test all OAuth connections"
            >
              <Icon
                className="material-symbols-outlined"
                baseClassName=""
                sx={{ fontSize: 14, animation: testingMode === "oauth" ? "spin 1s linear infinite" : "none" }}
              >
                play_arrow
              </Icon>
              {testingMode === "oauth" ? "Testing..." : "Test All"}
            </Box>
          </Box>
        </Box>
        <Box sx={cardGridSx}>
          {oauthEntries.map(([key, info]) => (
            <ProviderCard
              key={key}
              providerId={key}
              provider={info}
              stats={getProviderStats(key, "oauth")}
              authType="oauth"
              circuitBreaker={getCircuitBreakerForProvider(key)}
              onResetCircuit={resetCircuitBreaker}
              onToggle={(active) => handleToggleProvider(key, "oauth", active)}
            />
          ))}
        </Box>
      </Box>
      )}

      {/* Free Tier Providers */}
      {(freeEntries.length > 0 || freeTierEntries.length > 0) && (
      <Box sx={sectionSx}>
        <Box sx={sectionHeaderSx}>
          <Typography component="h2" sx={sectionTitleSx}>
            Free Tier Providers
          </Typography>
          <Box
            component="button"
            onClick={() => handleBatchTest("free")}
            disabled={!!testingMode}
            sx={testButtonSx(testingMode === "free", !!testingMode)}
            title="Test all Free connections"
            aria-label="Test all Free provider connections"
          >
            <Icon
              className="material-symbols-outlined"
              baseClassName=""
              sx={{ fontSize: 14, animation: testingMode === "free" ? "spin 1s linear infinite" : "none" }}
            >
              play_arrow
            </Icon>
            {testingMode === "free" ? "Testing..." : "Test All"}
          </Box>
        </Box>
        <Box sx={cardGridSx}>
          {freeEntries.map(([key, info]) => {
            // Kiro accepts both OAuth and api-key connections; count/toggle both
            // so the card total matches the provider detail page (#kiro-apikey).
            // Kiro's headless api-key flow persists authType "api_key" (underscore),
            // while generic apikey providers use "apikey" — include both spellings.
            const freeAuthTypes =
              key === "kiro" ? ["oauth", "apikey", "api_key"] : "oauth";
            return (
              <ProviderCard
                key={key}
                providerId={key}
                provider={info}
                stats={getProviderStats(key, freeAuthTypes)}
                authType="free"
                circuitBreaker={getCircuitBreakerForProvider(key)}
                onResetCircuit={resetCircuitBreaker}
                onToggle={(active) =>
                  handleToggleProvider(key, freeAuthTypes, active)
                }
              />
            );
          })}
          {freeTierEntries.map(([key, info]) => {
            const freeAuthTypes = getProviderAuthTypes(info, key);
            return (
              <ApiKeyProviderCard
                key={key}
                providerId={key}
                provider={info}
                stats={getProviderStats(key, freeAuthTypes)}
                authType={Array.isArray(freeAuthTypes) ? (freeAuthTypes[0] ?? "apikey") : freeAuthTypes}
                onToggle={(active) => handleToggleProvider(key, freeAuthTypes, active)}
              />
            );
          })}
        </Box>
      </Box>
      )}

      {/* API Key Providers — fixed list */}
      {apikeyEntries.length > 0 && (
      <Box sx={sectionSx}>
        <Box sx={sectionHeaderSx}>
          <Typography component="h2" sx={sectionTitleSx}>
            API Key Providers{" "}
          </Typography>
          <Box
            component="button"
            onClick={() => handleBatchTest("apikey")}
            disabled={!!testingMode}
            sx={testButtonSx(testingMode === "apikey", !!testingMode)}
            title="Test all API Key connections"
            aria-label="Test all API Key connections"
          >
            <Icon
              className="material-symbols-outlined"
              baseClassName=""
              sx={{ fontSize: 14, animation: testingMode === "apikey" ? "spin 1s linear infinite" : "none" }}
            >
              play_arrow
            </Icon>
            {testingMode === "apikey" ? "Testing..." : "Test All"}
          </Box>
        </Box>
        <Box sx={cardGridSx}>
          {visibleApikeyEntries.map(([key, info]) => (
            <ApiKeyProviderCard
              key={key}
              providerId={key}
              provider={info}
              stats={getProviderStats(key, "apikey")}
              authType="apikey"
              onToggle={(active) => handleToggleProvider(key, "apikey", active)}
            />
          ))}
        </Box>
        {!isApikeySearching && !showAllApikey && hiddenApikeyCount > 0 && (
          <Box
            component="button"
            onClick={() => setShowAllApikey(true)}
            sx={{ display: "flex", width: "100%", alignItems: "center", justifyContent: "center", gap: 0.75, borderRadius: "8px", border: "1px dashed", borderColor: "rgba(229,106,74,0.4)", px: 1.5, py: 1.25, fontSize: 14, fontWeight: 500, color: "primary.main", bgcolor: "transparent", cursor: "pointer", transition: "border-color 150ms ease, background-color 150ms ease", "&:hover": { borderColor: "primary.main", bgcolor: "rgba(229,106,74,0.05)" } }}
          >
            <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 16 }}>expand_more</Icon>
            Show all {apikeyEntries.length} providers
          </Box>
        )}
      </Box>
      )}

      {/* Web Cookie Providers — use browser subscription cookie instead of API key */}
      {webCookieEntries.length > 0 && (
      <Box sx={sectionSx}>
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
          <Typography component="h2" sx={sectionTitleSx}>
            Web Cookie Providers{" "}
          </Typography>
        </Box>
        <Box sx={cardGridSx}>
          {webCookieEntries.map(([key, info]) => (
            <ApiKeyProviderCard
              key={key}
              providerId={key}
              provider={info}
              stats={getProviderStats(key, "cookie")}
              authType="cookie"
              onToggle={(active) => handleToggleProvider(key, "cookie", active)}
            />
          ))}
        </Box>
      </Box>
      )}

      <AddCompatibleModal
        variant="openai"
        isOpen={showAddCompatibleModal}
        onClose={() => setShowAddCompatibleModal(false)}
        onCreated={(node) => {
          setProviderNodes((prev) => [...prev, node]);
          setShowAddCompatibleModal(false);
        }}
      />
      <AddCompatibleModal
        variant="anthropic"
        isOpen={showAddAnthropicCompatibleModal}
        onClose={() => setShowAddAnthropicCompatibleModal(false)}
        onCreated={(node) => {
          setProviderNodes((prev) => [...prev, node]);
          setShowAddAnthropicCompatibleModal(false);
        }}
      />

      {/* Test Results Modal */}
      {testResults && (
        <Dialog
          open
          onClose={() => setTestResults(null)}
          maxWidth="sm"
          fullWidth
          slotProps={{
            paper: {
              sx: {
                bgcolor: "background.paper",
                border: 1,
                borderColor: "divider",
                borderRadius: "12px",
                maxHeight: { xs: "86vh", sm: "80vh" },
                backgroundImage: "none",
              },
            },
          }}
        >
          <DialogTitle sx={{ position: "sticky", top: 0, zIndex: 10, display: "flex", alignItems: "center", justifyContent: "space-between", px: 2.5, py: 1.5, borderBottom: 1, borderColor: "divider", bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(38,38,38,0.95)" : "rgba(255,255,255,0.95)"), backdropFilter: "blur(4px)" }}>
            <Box component="h3" sx={{ fontWeight: 600, fontSize: 16 }}>Test Results</Box>
            <IconButton
              onClick={() => setTestResults(null)}
              aria-label="Close test results"
              sx={{ color: "text.secondary", p: 0.5, "&:hover": { bgcolor: "background.default", color: "text.primary" } }}
            >
              <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18 }}>close</Icon>
            </IconButton>
          </DialogTitle>
          <DialogContent sx={{ p: 2.5 }}>
            <ProviderTestResultsView results={testResults} />
          </DialogContent>
        </Dialog>
      )}
    </Box>
  );
}

function ProviderCard({ providerId, provider, stats, authType, onToggle, circuitBreaker, onResetCircuit }) {
  const { connected, error, errorCode, errorTime, allDisabled } = stats;
  const isNoAuth = !!provider.noAuth;

  const dotColors = {
    free: "bg-green-500",
    oauth: "bg-blue-500",
    apikey: "bg-amber-500",
    compatible: "bg-orange-500",
  };
  const dotLabels = {
    free: "Free",
    oauth: "OAuth",
    apikey: "API Key",
    compatible: "Compatible",
  };

  return (
    <Box
      component={Link}
      href={`/dashboard/providers/${providerId}`}
      sx={{
        minWidth: 0,
        display: "block",
        "&:hover .pc-toggle": { opacity: 1 },
      }}
    >
      <Card
        padding="xs"
        sx={{
          height: "100%",
          transition: "background-color 150ms ease",
          cursor: "pointer",
          opacity: allDisabled ? 0.5 : 1,
          "&:hover": { bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(255,255,255,0.01)" : "rgba(0,0,0,0.01)") },
        }}
      >
        <Box sx={{ display: "flex", minWidth: 0, alignItems: "center", justifyContent: "space-between", gap: 1.5 }}>
          <Box sx={{ display: "flex", minWidth: 0, alignItems: "center", gap: 1.5 }}>
            <Box
              sx={{
                width: 32,
                height: 32,
                flexShrink: 0,
                borderRadius: "8px",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                backgroundColor: `${provider.color?.length > 7 ? provider.color : provider.color + "15"}`,
              }}
            >
              <ProviderIcon
                src={`/providers/${provider.id}.webp`}
                alt={provider.name}
                size={30}
                fallbackText={
                  provider.textIcon || provider.id.slice(0, 2).toUpperCase()
                }
                fallbackColor={provider.color}
              />
            </Box>
            <Box sx={{ minWidth: 0 }}>
              <Typography component="h3" sx={{ fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{provider.name}</Typography>
              <Box sx={{ display: "flex", minWidth: 0, alignItems: "center", gap: 0.75, fontSize: 12, flexWrap: "wrap" }}>
                {allDisabled ? (
                  <Badge variant="default" size="sm">
                    <Box component="span" sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
                      <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 12 }}>
                        pause_circle
                      </Icon>
                      Disabled
                    </Box>
                  </Badge>
                ) : isNoAuth ? (
                  <Badge variant="success" size="sm" dot>Ready</Badge>
                ) : (
                  <>
                    {getStatusDisplay(connected, error, errorCode)}
                    {circuitBreaker && (
                      <CircuitBreakerBadge status={circuitBreaker} onReset={() => onResetCircuit(circuitBreaker?.name || providerId)} />
                    )}
                    {errorTime && (
                      <Box component="span" sx={{ color: "text.secondary" }}>{errorTime}</Box>
                    )}
                  </>
                )}
              </Box>
            </Box>
          </Box>
          <Box sx={{ display: "flex", flexShrink: 0, alignItems: "center", gap: 1 }}>
            {stats.total > 0 && (
              <Box
                className="pc-toggle"
                sx={{ opacity: { xs: 1, sm: 0 }, transition: "opacity 150ms ease" }}
                onClick={(e) => {
                  e.preventDefault();
                  e.stopPropagation();
                  onToggle(!allDisabled ? false : true);
                }}
              >
                <Toggle
                  size="sm"
                  checked={!allDisabled}
                  onChange={() => {}}
                  title={allDisabled ? "Enable provider" : "Disable provider"}
                />
              </Box>
            )}
          </Box>
        </Box>
      </Card>
    </Box>
  );
}



function ApiKeyProviderCard({
  providerId,
  provider,
  stats,
  authType,
  onToggle,
  circuitBreaker,
  onResetCircuit,
}) {
  const { connected, error, errorCode, errorTime, allDisabled } = stats;
  const isCompatible = providerId.startsWith(OPENAI_COMPATIBLE_PREFIX);
  const isAnthropicCompatible = providerId.startsWith(
    ANTHROPIC_COMPATIBLE_PREFIX,
  );

  const dotColors = {
    free: "bg-green-500",
    oauth: "bg-blue-500",
    apikey: "bg-amber-500",
    compatible: "bg-orange-500",
  };
  const dotLabels = {
    free: "Free",
    oauth: "OAuth",
    apikey: "API Key",
    compatible: "Compatible",
  };

  const getIconPath = () => {
    if (isCompatible && provider.apiType)
      return provider.apiType === "responses"
        ? "/providers/oai-r.webp"
        : "/providers/oai-cc.webp";
    if (isAnthropicCompatible) return "/providers/anthropic-m.webp";
    return getProviderIconSrc(provider.id);
  };

  return (
    <Box
      component={Link}
      href={`/dashboard/providers/${providerId}`}
      sx={{
        minWidth: 0,
        display: "block",
        "&:hover .pc-toggle": { opacity: 1 },
      }}
    >
      <Card
        padding="xs"
        sx={{
          height: "100%",
          transition: "background-color 150ms ease",
          cursor: "pointer",
          opacity: allDisabled ? 0.5 : 1,
          "&:hover": { bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(255,255,255,0.01)" : "rgba(0,0,0,0.01)") },
        }}
      >
        <Box sx={{ display: "flex", minWidth: 0, alignItems: "center", justifyContent: "space-between", gap: 1.5 }}>
          <Box sx={{ display: "flex", minWidth: 0, alignItems: "center", gap: 1.5 }}>
            <Box
              sx={{
                width: 32,
                height: 32,
                flexShrink: 0,
                borderRadius: "8px",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                backgroundColor: `${provider.color?.length > 7 ? provider.color : provider.color + "15"}`,
              }}
            >
              <ProviderIcon
                src={getIconPath()}
                alt={provider.name}
                size={30}
                fallbackText={
                  provider.textIcon || provider.id.slice(0, 2).toUpperCase()
                }
                fallbackColor={provider.color}
              />
            </Box>
            <Box sx={{ minWidth: 0 }}>
              <Typography component="h3" sx={{ fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{provider.name}</Typography>
              <Box sx={{ display: "flex", minWidth: 0, alignItems: "center", gap: 0.75, fontSize: 12, flexWrap: "wrap" }}>
                {allDisabled ? (
                  <Badge variant="default" size="sm">
                    <Box component="span" sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
                      <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 12 }}>
                        pause_circle
                      </Icon>
                      Disabled
                    </Box>
                  </Badge>
                ) : (
                  <>
                    {getStatusDisplay(connected, error, errorCode)}
                    {isCompatible && (
                      <Badge variant="default" size="sm">
                        {provider.apiType === "responses"
                          ? "Responses"
                          : "Chat"}
                      </Badge>
                    )}
                    {isAnthropicCompatible && (
                      <Badge variant="default" size="sm">
                        Messages
                      </Badge>
                    )}
                    {errorTime && (
                      <Box component="span" sx={{ color: "text.secondary" }}>{errorTime}</Box>
                    )}
                  </>
                )}
              </Box>
            </Box>
          </Box>
          <Box sx={{ display: "flex", flexShrink: 0, alignItems: "center", gap: 1 }}>
            {stats.total > 0 && (
              <Box
                className="pc-toggle"
                sx={{ opacity: { xs: 1, sm: 0 }, transition: "opacity 150ms ease" }}
                onClick={(e) => {
                  e.preventDefault();
                  e.stopPropagation();
                  onToggle(!allDisabled ? false : true);
                }}
              >
                <Toggle
                  size="sm"
                  checked={!allDisabled}
                  onChange={() => {}}
                  title={allDisabled ? "Enable provider" : "Disable provider"}
                />
              </Box>
            )}
          </Box>
        </Box>
      </Card>
    </Box>
  );
}



function ProviderTestResultsView({ results }) {
  if (results.error && !results.results) {
    return (
      <Box sx={{ textAlign: "center", py: 3 }}>
        <Icon className="material-symbols-outlined" baseClassName="" sx={{ color: "#ef4444", fontSize: 32, mb: 1, display: "block", mx: "auto" }}>
          error
        </Icon>
        <Typography sx={{ fontSize: 14, color: "#f87171" }}>{results.error}</Typography>
      </Box>
    );
  }

  const { summary, mode } = results;
  const items = results.results || [];
  const modeLabel =
    {
      oauth: "OAuth",
      free: "Free",
      apikey: "API Key",
      provider: "Provider",
      all: "All",
    }[mode] || mode;

  return (
    <Box sx={{ display: "flex", minWidth: 0, flexDirection: "column", gap: 1.5 }}>
      {summary && (
        <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: { xs: 1, sm: 1.5 }, fontSize: 12, mb: 0.5 }}>
          <Box component="span" sx={{ color: "text.secondary" }}>{modeLabel} Test</Box>
          <Box component="span" sx={{ px: 1, py: 0.25, borderRadius: "4px", bgcolor: "rgba(16,185,129,0.15)", color: "#34d399", fontWeight: 500 }}>
            {summary.passed} passed
          </Box>
          {summary.failed > 0 && (
            <Box component="span" sx={{ px: 1, py: 0.25, borderRadius: "4px", bgcolor: "rgba(239,68,68,0.15)", color: "#f87171", fontWeight: 500 }}>
              {summary.failed} failed
            </Box>
          )}
          <Box component="span" sx={{ color: "text.secondary", sm: { ml: "auto" } }}>
            {summary.total} tested
          </Box>
        </Box>
      )}
      {items.map((r, i) => (
        <Box
          key={r.connectionId || i}
          sx={{
            display: "flex",
            minWidth: 0,
            flexWrap: "wrap",
            alignItems: "center",
            gap: 1,
            borderRadius: "8px",
            bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(255,255,255,0.03)" : "rgba(0,0,0,0.03)"),
            px: 1.5,
            py: 1,
            fontSize: 12,
            flexWrap: { sm: "nowrap" },
          }}
        >
          <Icon
            className="material-symbols-outlined"
            baseClassName=""
            sx={{ fontSize: 16, color: r.valid ? "#10b981" : "#ef4444" }}
          >
            {r.valid ? "check_circle" : "error"}
          </Icon>
          <Box sx={{ minWidth: 0, flex: "1 1 160px" }}>
            <Box component="span" sx={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontWeight: 500, sm: { display: "inline" } }}>
              {r.connectionName}
            </Box>
            <Box component="span" sx={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", color: "text.secondary", sm: { display: "inline", ml: 0.75 } }}>
              ({r.provider})
            </Box>
          </Box>
          {r.latencyMs !== undefined && (
            <Box component="span" sx={{ flexShrink: 0, color: "text.secondary", fontFamily: "monospace", fontVariantNumeric: "tabular-nums" }}>
              {r.latencyMs}ms
            </Box>
          )}
          <Box
            component="span"
            sx={{
              flexShrink: 0,
              fontSize: 10,
              textTransform: "uppercase",
              fontWeight: 700,
              px: 0.75,
              py: 0.25,
              borderRadius: "4px",
              bgcolor: r.valid ? "rgba(16,185,129,0.15)" : "rgba(239,68,68,0.15)",
              color: r.valid ? "#34d399" : "#f87171",
            }}
          >
            {r.valid ? "OK" : r.diagnosis?.type || "ERROR"}
          </Box>
        </Box>
      ))}
      {items.length === 0 && (
        <Box sx={{ textAlign: "center", py: 2, color: "text.secondary", fontSize: 14 }}>
          No active connections found for this group.
        </Box>
      )}
    </Box>
  );
}
