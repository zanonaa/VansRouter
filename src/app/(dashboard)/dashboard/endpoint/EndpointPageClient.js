"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import Box from "@mui/material/Box";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import IconButton from "@mui/material/IconButton";
import MuiButton from "@mui/material/Button";
import { Card, Button, Input, Modal, CardSkeleton, Toggle, ConfirmModal } from "@/shared/components";
import { useCopyToClipboard } from "@/shared/hooks/useCopyToClipboard";
import { getCurrentLocale, onLocaleChange } from "@/i18n/runtime";
import {
  WENYAN_LOCALES,
  TUNNEL_BENEFITS,
  TUNNEL_PING_INTERVAL_MS,
  TUNNEL_PING_MAX_MS,
  STATUS_POLL_FAST_MS,
  REACHABLE_MISS_THRESHOLD,
  CLIENT_PING_FAST_MS,
  CAVEMAN_LEVELS,
  PONYTAIL_LEVELS,
} from "./endpointConstants";
import { clientPingUrl, clientPingAny } from "./endpointPing";
import EndpointRow from "./components/EndpointRow";
import StatusAlert from "./components/StatusAlert";
import Tooltip from "./components/Tooltip";
import SecurityWarning from "./components/SecurityWarning";
import { buildProviderList } from "@/shared/utils/aclProviderList";

function MsIcon({ children, sx, className = "" }) {
  return (
    <Icon className={`material-symbols-outlined ${className}`.trim()} baseClassName="" sx={sx}>
      {children}
    </Icon>
  );
}

function Spinner({ size = 14 }) {
  return <MsIcon sx={{ fontSize: size, animation: "mui-spinner-rotate 1s linear infinite", "@keyframes mui-spinner-rotate": { to: { transform: "rotate(360deg)" } } }}>progress_activity</MsIcon>;
}

function SectionTitle({ icon, children, sx }) {
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 1, ...sx }}>
      <MsIcon sx={{ color: "primary.main", fontSize: 24 }}>{icon}</MsIcon>
      <Typography sx={{ fontSize: 18, fontWeight: 600 }}>{children}</Typography>
    </Box>
  );
}

function SubtleLink({ href, children, sx }) {
  return (
    <Box
      component="a"
      href={href}
      target="_blank"
      rel="noreferrer"
      sx={{ fontSize: 12, fontWeight: 400, color: "primary.main", textDecoration: "underline", "&:hover": { opacity: 0.8 }, ...sx }}
    >
      {children}
    </Box>
  );
}

// Inline status banner matching the original Tailwind tones.
function Banner({ tone = "neutral", children, sx }) {
  return (
    <Box
      sx={(theme) => ({
        flex: 1,
        display: "flex",
        alignItems: "center",
        gap: 1,
        px: 1.5,
        py: 0.75,
        borderRadius: "6px",
        border: 1,
        fontSize: 14,
        minWidth: 0,
        ...(tone === "neutral" && {
          borderColor: theme.palette.divider,
          bgcolor: "var(--color-surface-2)",
          color: theme.palette.text.secondary,
        }),
        ...(tone === "warn" && {
          borderColor: theme.palette.mode === "dark" ? "#92400e" : "#fcd34d",
          bgcolor: "rgba(245,158,11,0.05)",
          color: theme.palette.mode === "dark" ? "#fbbf24" : "#d97706",
        }),
        ...(tone === "error" && {
          borderColor: theme.palette.mode === "dark" ? "#991b1b" : "#fca5a5",
          bgcolor: "rgba(239,68,68,0.05)",
          color: theme.palette.mode === "dark" ? "#f87171" : "#dc2626",
        }),
        ...(typeof sx === "function" ? sx(theme) : sx),
      })}
    >
      {children}
    </Box>
  );
}

const LABEL_CHIP = (active) => ({
  fontSize: 12,
  fontFamily: "ui-monospace, monospace",
  px: 0.75,
  py: 0.25,
  borderRadius: "4px",
  flexShrink: 0,
  minWidth: 88,
  textAlign: "center",
  bgcolor: active ? "rgba(229,106,74,0.10)" : "var(--color-surface-2)",
  color: active ? "primary.main" : "text.secondary",
});

const POWER_BUTTON_SX = { color: "#dc2626", "&:hover": { bgcolor: "rgba(239,68,68,0.10)" } };
const COPY_BUTTON_SX = { color: "text.secondary", "&:hover": { color: "primary.main", bgcolor: "var(--color-surface-2)" } };

export default function APIPageClient() {
  const [keys, setKeys] = useState([]);
  const [loading, setLoading] = useState(true);
  const [showAddModal, setShowAddModal] = useState(false);
  const [newKeyName, setNewKeyName] = useState("");
  const [createdKey, setCreatedKey] = useState(null);
  const [confirmState, setConfirmState] = useState(null);

  // ACL Edit Modal state
  const [editingKey, setEditingKey] = useState(null); // key object being edited
  const [editName, setEditName] = useState("");
  const [editKinds, setEditKinds] = useState([]); // selected kinds
  const [editProviders, setEditProviders] = useState([]); // selected provider IDs
  const [editCombos, setEditCombos] = useState([]); // selected combo names
  const [editKindsAll, setEditKindsAll] = useState(true); // null = all
  const [editProvidersAll, setEditProvidersAll] = useState(true);
  const [editCombosAll, setEditCombosAll] = useState(true);
  const [editSaving, setEditSaving] = useState(false);
  const [providerList, setProviderList] = useState([]);
  const [aliasMap, setAliasMap] = useState({}); // alias → provider ID
  const [comboList, setComboList] = useState([]);

  const [requireApiKey, setRequireApiKey] = useState(false);
  const [allowRemoteNoApiKey, setAllowRemoteNoApiKey] = useState(false);
  const [requireLogin, setRequireLogin] = useState(true);
  const [hasPassword, setHasPassword] = useState(true);
  const [tunnelDashboardAccess, setTunnelDashboardAccess] = useState(false);
  const [rtkEnabled, setRtkEnabledState] = useState(true);
  const [headroomEnabled, setHeadroomEnabled] = useState(false);
  const [headroomUrl, setHeadroomUrl] = useState("http://localhost:8787");
  const [headroomCompressUserMessages, setHeadroomCompressUserMessages] = useState(false);
  const [headroomStatus, setHeadroomStatus] = useState({ installed: false, running: false, python: null, loading: true });
  const [showHeadroomInstallModal, setShowHeadroomInstallModal] = useState(false);
  const [headroomActionLoading, setHeadroomActionLoading] = useState(false);
  const [headroomActionError, setHeadroomActionError] = useState("");
  const [cavemanEnabled, setCavemanEnabled] = useState(false);
  const [cavemanLevel, setCavemanLevel] = useState("full");
  const [ponytailEnabled, setPonytailEnabled] = useState(false);
  const [ponytailLevel, setPonytailLevel] = useState("full");
  const [locale, setLocale] = useState("en");

  // Cloudflare Tunnel state
  const [tunnelChecking, setTunnelChecking] = useState(true);
  const [tunnelEnabled, setTunnelEnabled] = useState(false);
  const [tunnelReachable, setTunnelReachable] = useState(false);
  const [tunnelUrl, setTunnelUrl] = useState("");
  const [tunnelPublicUrl, setTunnelPublicUrl] = useState("");
  const [tunnelLoading, setTunnelLoading] = useState(false);
  const [tunnelProgress, setTunnelProgress] = useState("");
  const [tunnelStatus, setTunnelStatus] = useState(null);
  const [showEnableTunnelModal, setShowEnableTunnelModal] = useState(false);
  const [showDisableTunnelModal, setShowDisableTunnelModal] = useState(false);

  // Tailscale state
  const [tsEnabled, setTsEnabled] = useState(false);
  const [tsReachable, setTsReachable] = useState(false);
  const [tsUrl, setTsUrl] = useState("");
  const [tsLoading, setTsLoading] = useState(false);
  const [tsProgress, setTsProgress] = useState("");
  const [tsStatus, setTsStatus] = useState(null);
  const [tsAuthUrl, setTsAuthUrl] = useState("");
  const [tsAuthLabel, setTsAuthLabel] = useState("");
  const [tsInstalled, setTsInstalled] = useState(null); // null=checking, true/false
  const [tsInstalling, setTsInstalling] = useState(false);
  const [tsInstallLog, setTsInstallLog] = useState([]);
  const [tsSudoPassword, setTsSudoPassword] = useState("");
  const [tsConnecting, setTsConnecting] = useState(false);
  const [showTsModal, setShowTsModal] = useState(false);
  const [showDisableTsModal, setShowDisableTsModal] = useState(false);
  const tsLogRef = useRef(null);

  // Debounce reachable=false: server may briefly return false during background refresh.
  // Only flip UI to "reconnecting" after N consecutive misses to avoid spinner flicker.
  const tunnelMissRef = useRef(0);
  const tsMissRef = useRef(0);
  // Browser-side reachable cache (independent of backend DNS quirks)
  const tunnelClientReachableRef = useRef(false);
  const tsClientReachableRef = useRef(false);
  // Track whether reachable=true was ever observed in this session.
  // Distinguishes "Checking..." (initial cold cache) from "Reconnecting..." (lost connection).
  const tunnelEverReachableRef = useRef(false);
  const tsEverReachableRef = useRef(false);
  const [tunnelEverReachable, setTunnelEverReachable] = useState(false);
  const [tsEverReachable, setTsEverReachable] = useState(false);

  // API key visibility toggle state
  const [visibleKeys, setVisibleKeys] = useState(new Set());

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/immutability --
     The useEffects below intentionally run once on mount (locale detection,
     bootstrap fetch, scroll-into-view, status poll) or sync external state
     changes back into the UI (caveman level reset when locale changes). The
     handler functions they call (syncTunnelStatus, loadSettings, patchSetting,
     fetchData) are declared further down in this 2000-line file, which is
     a static-analysis limitation, not a runtime bug — closures capture the
     declarations correctly when the effects run. */

  // Client-side local/remote detection (UI hint only, not a security gate)
  const [isRemoteHost, setIsRemoteHost] = useState(false);
  useEffect(() => {
    if (typeof window !== "undefined")
      // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time client-only detection on mount.
      setIsRemoteHost(!["localhost", "127.0.0.1", "::1"].includes(window.location.hostname));
  }, []);

  // Track app UI locale to gate wenyan caveman levels
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time locale sync on mount; effect is intentional, not a derived value.
    setLocale(getCurrentLocale());
    return onLocaleChange(() => setLocale(getCurrentLocale()));
  }, []);

  const isWenyanLocale = WENYAN_LOCALES.includes(locale);
  const visibleCavemanLevels = isWenyanLocale
    ? CAVEMAN_LEVELS
    : CAVEMAN_LEVELS.filter((lvl) => !lvl.wenyan);

  // Reset wenyan level to "ultra" when leaving a Chinese locale
  // eslint-disable-next-line react-hooks/immutability -- patchSetting is declared further down in the file; captured by closure at runtime.
  useEffect(() => {
    const current = CAVEMAN_LEVELS.find((lvl) => lvl.id === cavemanLevel);
    if (current?.wenyan && !isWenyanLocale) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- syncs external UI locale change back into local storage; effect is intentional.
      setCavemanLevel("ultra");
      patchSetting({ cavemanLevel: "ultra" });
    }
  }, [isWenyanLocale, cavemanLevel]);

  const { copied, copy } = useCopyToClipboard();

  // Security gate: block remote exposure while dashboard uses default password or login is off.
  const isLoginUnsafe = !requireLogin || !hasPassword;
  const unsafeReason = !requireLogin
    ? "Enable \"Require login\" and set a custom password before activating the tunnel."
    : "Change the default dashboard password before activating the tunnel.";

  // Auto-scroll install log
  useEffect(() => {
    if (tsLogRef.current) tsLogRef.current.scrollTop = tsLogRef.current.scrollHeight;
  }, [tsInstallLog]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- initial bootstrap fetch; the function declarations below are intentionally hoisted below the JSX (no derived state pattern fits a one-shot init).
    fetchData();
    // eslint-disable-next-line react-hooks/set-state-in-effect -- initial bootstrap fetch.
    loadSettings();
  }, []);

  // Status poll: only while degraded (not yet reachable). Stop once healthy to avoid spam.
  // Visibility re-check: refresh once when tab becomes visible.
  // eslint-disable-next-line react-hooks/immutability -- syncTunnelStatus is declared further down; captured by closure at runtime.
  useEffect(() => {
    const anyEnabled = tunnelEnabled || tsEnabled;
    if (!anyEnabled) return;
    const tunnelHealthy = !tunnelEnabled || tunnelReachable;
    const tsHealthy = !tsEnabled || tsReachable;
    const allHealthy = tunnelHealthy && tsHealthy;
    const onVisible = () => { if (!document.hidden) syncTunnelStatus(); };
    document.addEventListener("visibilitychange", onVisible);
    if (allHealthy) return () => document.removeEventListener("visibilitychange", onVisible);
    const timer = setInterval(() => { if (!document.hidden) syncTunnelStatus(); }, STATUS_POLL_FAST_MS);
    return () => {
      clearInterval(timer);
      document.removeEventListener("visibilitychange", onVisible);
    };
  // eslint-disable-next-line react-hooks/immutability -- syncTunnelStatus is declared further down in the file; capturing via closure works fine at runtime but trips the static analysis.
  }, [tunnelEnabled, tsEnabled, tunnelReachable, tsReachable]);

  // Browser-side periodic ping: probes tunnel/tailscale URLs directly so UI stays
  // "reachable" even when backend DNS (1.1.1.1) hiccups on *.ts.net or *.trycloudflare.com.
  // Adaptive: slow when healthy, fast when degraded; pause when tab hidden.
  useEffect(() => {
    const probeBoth = async () => {
      if (document.hidden) return;
      if (tunnelEnabled && (tunnelUrl || tunnelPublicUrl)) {
        const ok = await clientPingAny(tunnelPublicUrl, tunnelUrl);
        tunnelClientReachableRef.current = ok;
        if (ok) { tunnelMissRef.current = 0; setTunnelReachable(true); if (!tunnelEverReachableRef.current) { tunnelEverReachableRef.current = true; setTunnelEverReachable(true); } }
        else { tunnelMissRef.current += 1; if (tunnelMissRef.current >= REACHABLE_MISS_THRESHOLD) setTunnelReachable(false); }
      } else {
        tunnelClientReachableRef.current = false;
      }
      if (tsEnabled && tsUrl) {
        const ok = await clientPingUrl(tsUrl);
        tsClientReachableRef.current = ok;
        if (ok) { tsMissRef.current = 0; setTsReachable(true); if (!tsEverReachableRef.current) { tsEverReachableRef.current = true; setTsEverReachable(true); } }
        else { tsMissRef.current += 1; if (tsMissRef.current >= REACHABLE_MISS_THRESHOLD) setTsReachable(false); }
      } else {
        tsClientReachableRef.current = false;
      }
    };
    const anyEnabled = (tunnelEnabled && (tunnelUrl || tunnelPublicUrl)) || (tsEnabled && tsUrl);
    if (!anyEnabled) return;
    probeBoth();
    const tunnelHealthy = !tunnelEnabled || tunnelReachable;
    const tsHealthy = !tsEnabled || tsReachable;
    if (tunnelHealthy && tsHealthy) return;
    const id = setInterval(probeBoth, CLIENT_PING_FAST_MS);
    return () => clearInterval(id);
  }, [tunnelEnabled, tunnelUrl, tunnelPublicUrl, tsEnabled, tsUrl, tunnelReachable, tsReachable]);

  // Client-side reachable only (server no longer probes; watchdog handles backend health).
  // Miss-debounce: only flip to false after N consecutive misses.
  const updateReachable = useCallback((_unused, clientRef, missRef, setter, everRef, everSetter) => {
    const reachable = clientRef.current;
    if (reachable) {
      missRef.current = 0;
      setter(true);
      if (!everRef.current) {
        everRef.current = true;
        everSetter(true);
      }
    } else {
      missRef.current += 1;
      if (missRef.current >= REACHABLE_MISS_THRESHOLD) setter(false);
    }
  }, []);

  // Trust user intent (settingsEnabled): UI stays "enabled" while watchdog restarts process
  const syncTunnelStatus = async () => {
    try {
      const statusRes = await fetch("/api/tunnel/status", { cache: "no-store" });
      if (!statusRes.ok) return;
      const data = await statusRes.json();
      const tEnabled = data.tunnel?.settingsEnabled ?? data.tunnel?.enabled ?? false;
      const tUrl = data.tunnel?.tunnelUrl || "";
      setTunnelUrl(tUrl);
      setTunnelPublicUrl(data.tunnel?.publicUrl || "");
      setTunnelEnabled(tEnabled);
      updateReachable(null, tunnelClientReachableRef, tunnelMissRef, setTunnelReachable, tunnelEverReachableRef, setTunnelEverReachable);

      const tsEn = data.tailscale?.settingsEnabled ?? data.tailscale?.enabled ?? false;
      const tsUrlVal = data.tailscale?.tunnelUrl || "";
      setTsUrl(tsUrlVal);
      setTsEnabled(tsEn);
      updateReachable(null, tsClientReachableRef, tsMissRef, setTsReachable, tsEverReachableRef, setTsEverReachable);
    } catch { /* ignore poll errors */ }
  };

  const loadSettings = async () => {
    setTunnelChecking(true);
    try {
      const [settingsRes, statusRes] = await Promise.all([
        fetch("/api/settings"),
        fetch("/api/tunnel/status", { cache: "no-store" })
      ]);
      if (settingsRes.ok) {
        const data = await settingsRes.json();
        setRequireApiKey(data.requireApiKey || false);
        setAllowRemoteNoApiKey(data.allowRemoteNoApiKey || false);
        setRequireLogin(data.requireLogin !== false);
        setHasPassword(data.hasPassword || false);
        setTunnelDashboardAccess(data.tunnelDashboardAccess || false);
        setRtkEnabledState(data.rtkEnabled !== false);
        setHeadroomEnabled(!!data.headroomEnabled);
        setHeadroomUrl(data.headroomUrl || "http://localhost:8787");
        setHeadroomCompressUserMessages(!!data.headroomCompressUserMessages);
        refreshHeadroomStatus();
        setCavemanEnabled(!!data.cavemanEnabled);
        setCavemanLevel(data.cavemanLevel || "full");
        setPonytailEnabled(!!data.ponytailEnabled);
        setPonytailLevel(data.ponytailLevel || "full");
      }
      if (statusRes.ok) {
        const data = await statusRes.json();
        const tEnabled = data.tunnel?.settingsEnabled ?? data.tunnel?.enabled ?? false;
        const tUrl = data.tunnel?.tunnelUrl || "";
        setTunnelUrl(tUrl);
        setTunnelPublicUrl(data.tunnel?.publicUrl || "");
        setTunnelEnabled(tEnabled);
        updateReachable(null, tunnelClientReachableRef, tunnelMissRef, setTunnelReachable, tunnelEverReachableRef, setTunnelEverReachable);

        const tsEn = data.tailscale?.settingsEnabled ?? data.tailscale?.enabled ?? false;
        const tsUrlVal = data.tailscale?.tunnelUrl || "";
        setTsUrl(tsUrlVal);
        setTsEnabled(tsEn);
        updateReachable(null, tsClientReachableRef, tsMissRef, setTsReachable, tsEverReachableRef, setTsEverReachable);
      }
    } catch (error) {
      console.log("Error loading settings:", error);
    } finally {
      setTunnelChecking(false);
    }
  };

  const handleTunnelDashboardAccess = async (value) => {
    try {
      const res = await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ tunnelDashboardAccess: value }),
      });
      if (res.ok) setTunnelDashboardAccess(value);
    } catch (error) {
      console.log("Error updating tunnelDashboardAccess:", error);
    }
  };

  const handleRequireApiKey = async (value) => {
    try {
      const res = await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ requireApiKey: value }),
      });
      if (res.ok) setRequireApiKey(value);
    } catch (error) {
      console.log("Error updating requireApiKey:", error);
    }
  };

  const handleAllowRemoteNoApiKey = async (value) => {
    try {
      const res = await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ allowRemoteNoApiKey: value }),
      });
      if (res.ok) setAllowRemoteNoApiKey(value);
    } catch (error) {
      console.log("Error updating allowRemoteNoApiKey:", error);
    }
  };

  const handleRtkEnabled = async (value) => {
    try {
      const res = await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ rtkEnabled: value }),
      });
      if (res.ok) setRtkEnabledState(value);
    } catch (error) {
      console.log("Error updating rtkEnabled:", error);
    }
  };

  const patchSetting = async (patch) => {
    try {
      await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(patch),
      });
    } catch (error) {
      console.log("Error updating setting:", error);
    }
  };

  const handleCavemanEnabled = (value) => {
    setCavemanEnabled(value);
    patchSetting({ cavemanEnabled: value });
  };

  const handleHeadroomEnabled = (value) => {
    const nextUrl = headroomUrl.trim() || "http://localhost:8787";
    setHeadroomUrl(nextUrl);
    setHeadroomEnabled(value);
    patchSetting({ headroomEnabled: value, headroomUrl: nextUrl });
  };

  const handleHeadroomUrlBlur = async () => {
    const next = headroomUrl.trim() || "http://localhost:8787";
    setHeadroomUrl(next);
    await patchSetting({ headroomUrl: next });
    refreshHeadroomStatus();
  };

  const handleHeadroomCompressUserMessages = (value) => {
    setHeadroomCompressUserMessages(value);
    patchSetting({ headroomCompressUserMessages: value });
  };

  const refreshHeadroomStatus = useCallback(async () => {
    setHeadroomStatus((s) => ({ ...s, loading: true }));
    try {
      const res = await fetch("/api/headroom/status", { headers: { "Cache-Control": "no-store" } });
      const data = await res.json();
      setHeadroomStatus({ ...data, loading: false });
    } catch {
      setHeadroomStatus({ installed: false, running: false, python: null, loading: false });
    }
  }, []);

  const handleHeadroomStart = useCallback(async () => {
    setHeadroomActionError("");
    setHeadroomActionLoading(true);
    try {
      const res = await fetch("/api/headroom/start", { method: "POST" });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || "Failed to start proxy");
      await refreshHeadroomStatus();
    } catch (e) {
      setHeadroomActionError(e.message);
    } finally {
      setHeadroomActionLoading(false);
    }
  }, [refreshHeadroomStatus]);

  const handleHeadroomStop = useCallback(async () => {
    setHeadroomActionLoading(true);
    try {
      await fetch("/api/headroom/stop", { method: "POST" });
      await refreshHeadroomStatus();
    } finally {
      setHeadroomActionLoading(false);
    }
  }, [refreshHeadroomStatus]);

  const handleCavemanLevel = (level) => {
    setCavemanLevel(level);
    patchSetting({ cavemanLevel: level });
  };

  const handlePonytailEnabled = (value) => {
    setPonytailEnabled(value);
    patchSetting({ ponytailEnabled: value });
  };

  const handlePonytailLevel = (level) => {
    setPonytailLevel(level);
    patchSetting({ ponytailLevel: level });
  };

  // ── ACL Edit Key handlers ──────────────────────────────────────────
  const ALL_KINDS = ["llm", "embedding", "image", "video", "tts", "stt", "webSearch", "webFetch"];

  // Provider list for the ACL dialog is built by src/shared/utils/aclProviderList.js
  // (connections + nodes + registered noAuth/free providers). Auth-requiring
  // providers with zero connections are not shown.

  const handleOpenEditKey = async (key) => {
    setEditingKey(key);
    setEditName(key.name || "");
    const ap = key.allowedProviders;
    const ac = key.allowedCombos;
    const ak = key.allowedKinds;
    setEditProvidersAll(!ap);
    setEditCombosAll(!ac);
    setEditKindsAll(!ak);
    setEditCombos(ac || []);
    setEditKinds(ak || []);

    // Resolve stored ACL provider values to provider IDs in our list
    // Stored values can be: full provider ID, prefix (e.g. "tr"), or alias (e.g. "oc", "qd", "kc")
    if (ap && providerList.length > 0) {
      const matched = new Set();
      for (const stored of ap) {
        // Try direct match with provider ID
        const direct = providerList.find((p) => p.id === stored);
        if (direct) { matched.add(direct.id); continue; }
        // Try match by alias (e.g. "oc" → "opencode", "qd" → "qoder", "kc" → "kilocode")
        const byAlias = providerList.find((p) => p.alias === stored);
        if (byAlias) { matched.add(byAlias.id); continue; }
        // Try match by prefix (e.g. "tr" → TokenRouter)
        const byPrefix = providerList.find((p) => p.prefix === stored);
        if (byPrefix) { matched.add(byPrefix.id); continue; }
        // Keep as-is (unknown reference — may be a deleted provider)
        matched.add(stored);
      }
      setEditProviders([...matched]);
    } else {
      setEditProviders([]);
    }
  };

  const handleSaveEditKey = async () => {
    if (!editingKey) return;
    setEditSaving(true);
    try {
      const body = {
        name: editName.trim() || editingKey.name || "",
        allowedProviders: editProvidersAll ? null : editProviders,
        allowedCombos: editCombosAll ? null : editCombos,
        allowedKinds: editKindsAll ? null : editKinds,
      };
      const res = await fetch(`/api/keys/${editingKey.id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      if (res.ok) {
        const data = await res.json();
        setKeys((prev) => prev.map((k) => (k.id === editingKey.id ? { ...k, ...data.key } : k)));
        setEditingKey(null);
      }
    } catch (error) {
      console.log("Error saving key ACL:", error);
    } finally {
      setEditSaving(false);
    }
  };

  const toggleEditProvider = (id) => {
    setEditProviders((prev) => prev.includes(id) ? prev.filter((p) => p !== id) : [...prev, id]);
  };

  const toggleEditCombo = (name) => {
    setEditCombos((prev) => prev.includes(name) ? prev.filter((c) => c !== name) : [...prev, name]);
  };

  const toggleEditKind = (kind) => {
    setEditKinds((prev) => prev.includes(kind) ? prev.filter((k) => k !== kind) : [...prev, kind]);
  };

  const fetchData = async () => {
    try {
      const [keysRes, providersRes, combosRes, nodesRes] = await Promise.all([
        fetch("/api/keys", { cache: "no-store" }),
        fetch("/api/providers", { cache: "no-store" }),
        fetch("/api/combos", { cache: "no-store" }),
        fetch("/api/provider-nodes", { cache: "no-store" }),
      ]);

      const keysData = await keysRes.json();
      if (keysRes.ok) {
        setKeys(keysData.keys || []);
      }
      let connections = [];
      let nodes = [];
      let registered = [];
      if (providersRes.ok) {
        const pData = await providersRes.json();
        connections = pData.connections || [];
        if (pData.aliasMap) setAliasMap(pData.aliasMap);
        registered = pData.providers || [];
      }
      if (nodesRes.ok) {
        const nData = await nodesRes.json();
        nodes = nData.nodes || [];
      }
      setProviderList(buildProviderList(connections, nodes, registered));
      if (combosRes.ok) {
        const cData = await combosRes.json();
        setComboList(cData.combos || []);
      }
    } catch (error) {
      console.log("Error fetching data:", error);
    } finally {
      setLoading(false);
    }
  };

  // u2500u2500u2500 Cloudflare Tunnel handlers
  // Ping tunnel health until reachable. Race multiple URLs (shortlink + direct) — 1 OK is enough.
  const pingTunnelHealth = async (...urls) => {
    setTunnelLoading(true);
    setTunnelProgress("Waiting for tunnel ready...");
    const targets = urls.filter(Boolean).map((u) => `${u}/api/health`);
    const start = Date.now();
    while (Date.now() - start < TUNNEL_PING_MAX_MS) {
      await new Promise((r) => setTimeout(r, TUNNEL_PING_INTERVAL_MS));
      const ok = await Promise.any(targets.map(async (h) => {
        const p = await fetch(h, { mode: "cors", cache: "no-store" });
        if (p.ok) return true;
        throw new Error("not ready");
      })).catch(() => false);
      if (ok) {
        setTunnelEnabled(true);
        setTunnelLoading(false);
        setTunnelProgress("");
        return true;
      }
      // Every 5 pings (~10s), check if backend process still alive
      if ((Date.now() - start) % 10000 < TUNNEL_PING_INTERVAL_MS) {
        try {
          const statusRes = await fetch("/api/tunnel/status");
          if (statusRes.ok) {
            const status = await statusRes.json();
            if (!status.tunnel?.enabled) {
              setTunnelStatus({ type: "error", message: "Tunnel process stopped unexpectedly." });
              setTunnelLoading(false);
              setTunnelProgress("");
              return false;
            }
          }
        } catch { /* ignore */ }
      }
    }
    setTunnelStatus({ type: "error", message: "Tunnel created but not reachable. Please try again." });
    setTunnelLoading(false);
    setTunnelProgress("");
    return false;
  };

  const handleEnableTunnel = async () => {
    setShowEnableTunnelModal(false);
    setTunnelLoading(true);
    setTunnelStatus(null);
    setTunnelProgress("Creating tunnel...");

    // Poll download progress while enable request is pending
    let polling = true;
    const pollProgress = async () => {
      while (polling) {
        try {
          const r = await fetch("/api/tunnel/status");
          if (r.ok) {
            const s = await r.json();
            if (s.download?.downloading) {
              setTunnelProgress(`Downloading cloudflared... ${s.download.progress}%`);
            } else if (polling) {
              setTunnelProgress("Creating tunnel...");
            }
          }
        } catch { /* ignore */ }
        await new Promise((r) => setTimeout(r, 1000));
      }
    };
    pollProgress();

    try {
      const res = await fetch("/api/tunnel/enable", { method: "POST" });
      polling = false;
      const data = await res.json();
      if (!res.ok) {
        setTunnelStatus({ type: "error", message: data.error || "Failed to enable tunnel" });
        return;
      }

      const url = data.tunnelUrl;
      if (!url) {
        setTunnelStatus({ type: "error", message: "No tunnel URL returned" });
        return;
      }

      setTunnelUrl(url);
      setTunnelPublicUrl(data.publicUrl || "");
      await pingTunnelHealth(data.publicUrl, url);
    } catch (error) {
      setTunnelStatus({ type: "error", message: error.message });
    } finally {
      polling = false;
      setTunnelLoading(false);
      setTunnelProgress("");
    }
  };

  const handleDisableTunnel = async () => {
    setTunnelLoading(true);
    setTunnelStatus(null);
    try {
      const res = await fetch("/api/tunnel/disable", { method: "POST" });
      const data = await res.json();
      if (res.ok) {
        setTunnelEnabled(false);
        setTunnelUrl("");
        setShowDisableTunnelModal(false);
        setTunnelStatus({ type: "success", message: "Tunnel disabled" });
      } else {
        setTunnelStatus({ type: "error", message: data.error || "Failed to disable tunnel" });
      }
    } catch (error) {
      setTunnelStatus({ type: "error", message: error.message });
    } finally {
      setTunnelLoading(false);
    }
  };

  // u2500u2500u2500 Tailscale handlers
  const checkTailscaleInstalled = async () => {
    setTsInstalled(null);
    try {
      const res = await fetch("/api/tunnel/tailscale-check");
      if (res.ok) {
        const data = await res.json();
        setTsInstalled(data.installed);
        return data;
      }
    } catch { /* ignore */ }
    setTsInstalled(false);
    return { installed: false };
  };

  const handleInstallTailscale = async () => {
    setTsInstalling(true);
    setTsStatus(null);
    setTsInstallLog([]);
    try {
      const res = await fetch("/api/tunnel/tailscale-install", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ sudoPassword: tsSudoPassword }),
      });
      setTsSudoPassword("");

      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const parts = buffer.split("\n\n");
        buffer = parts.pop() || "";
        for (const part of parts) {
          const lines = part.split("\n");
          let event = "progress";
          let data = null;
          for (const line of lines) {
            if (line.startsWith("event: ")) event = line.slice(7).trim();
            if (line.startsWith("data: ")) {
              try { data = JSON.parse(line.slice(6)); } catch { /* skip */ }
            }
          }
          if (!data) continue;
          if (event === "progress") {
            setTsInstallLog((prev) => [...prev.slice(-50), data.message]);
          } else if (event === "done") {
            setTsInstalled(true);
            setTsInstalling(false);
            setShowTsModal(false);
            handleConnectTailscale();
            return;
          } else if (event === "error") {
            setTsStatus({ type: "error", message: data.error || "Install failed" });
          }
        }
      }
    } catch (e) {
      setTsStatus({ type: "error", message: e.message });
    } finally {
      setTsInstalling(false);
    }
  };

  // Ping Tailscale health until reachable
  const pingTsHealth = async (url) => {
    setTsProgress("Waiting for Tailscale ready...");
    const healthUrl = `${url}/api/health`;
    const start = Date.now();
    while (Date.now() - start < TUNNEL_PING_MAX_MS) {
      await new Promise((r) => setTimeout(r, TUNNEL_PING_INTERVAL_MS));
      try {
        const ping = await fetch(healthUrl, { mode: "no-cors", cache: "no-store" });
        if (ping.ok || ping.type === "opaque") return true;
      } catch { /* not ready yet */ }
    }
    return false;
  };

  // Show inline login button instead of auto-opening popup (browsers block popups
  // opened after async work because the user gesture is lost).
  const requestUserAuth = (url, label) => {
    setTsAuthUrl(url);
    setTsAuthLabel(label);
  };

  const clearUserAuth = () => {
    setTsAuthUrl("");
    setTsAuthLabel("");
  };

  const handleConnectTailscale = async () => {
    setShowTsModal(false);
    setTsConnecting(true);
    setTsLoading(true);
    setTsStatus(null);
    setTsProgress("Connecting...");
    clearUserAuth();
    try {
      const res = await fetch("/api/tunnel/tailscale-enable", { method: "POST" });
      const data = await res.json();

      if (res.ok && data.success) {
        setTsUrl(data.tunnelUrl || "");
        const reachable = await pingTsHealth(data.tunnelUrl);
        setTsEnabled(true);
        setTsStatus(reachable ? null : { type: "warning", message: "Connected but not reachable yet." });
        return;
      }

      if (data.needsLogin && data.authUrl) {
        requestUserAuth(data.authUrl, "Open Login Page");
        setTsProgress("Login required — click \"Open Login Page\" to continue");
        for (let i = 0; i < 40; i++) {
          await new Promise((r) => setTimeout(r, 3000));
          try {
            const r2 = await fetch("/api/tunnel/tailscale-check");
            if (r2.ok) {
              const check = await r2.json();
              if (check.loggedIn) {
                clearUserAuth();
                setTsProgress("Starting funnel...");
                const res2 = await fetch("/api/tunnel/tailscale-enable", { method: "POST" });
                const data2 = await res2.json();
                if (res2.ok && data2.success) {
                  setTsUrl(data2.tunnelUrl || "");
                  const ok2 = await pingTsHealth(data2.tunnelUrl);
                  setTsEnabled(true);
                  setTsStatus(ok2 ? null : { type: "warning", message: "Connected but not reachable yet." });
                } else if (data2.funnelNotEnabled && data2.enableUrl) {
                  await pollFunnelEnable(data2.enableUrl);
                } else {
                  setTsStatus({ type: "error", message: data2.error || "Failed to start funnel" });
                }
                return;
              }
            }
          } catch { /* retry */ }
        }
        clearUserAuth();
        setTsStatus({ type: "error", message: "Login timed out. Please try again." });
        return;
      }

      if (data.funnelNotEnabled && data.enableUrl) {
        await pollFunnelEnable(data.enableUrl);
        return;
      }

      setTsStatus({ type: "error", message: data.error || "Failed to connect" });
    } catch (error) {
      setTsStatus({ type: "error", message: error.message });
    } finally {
      setTsLoading(false);
      setTsConnecting(false);
      setTsProgress("");
      clearUserAuth();
    }
  };

  const pollFunnelEnable = async (enableUrl) => {
    requestUserAuth(enableUrl, "Open Funnel Settings");
    setTsProgress("Click \"Open Funnel Settings\" to enable Funnel...");
    for (let i = 0; i < 40; i++) {
      await new Promise((r) => setTimeout(r, 3000));
      try {
        const res = await fetch("/api/tunnel/tailscale-enable", { method: "POST" });
        const data = await res.json();
        if (res.ok && data.success) {
          clearUserAuth();
          setTsUrl(data.tunnelUrl || "");
          const ok3 = await pingTsHealth(data.tunnelUrl);
          setTsEnabled(true);
          setTsStatus(ok3 ? null : { type: "warning", message: "Connected but not reachable yet." });
          return;
        }
        if (data.funnelNotEnabled) continue;
        if (data.error) {
          clearUserAuth();
          setTsStatus({ type: "error", message: data.error });
          return;
        }
      } catch { /* retry */ }
    }
    clearUserAuth();
    setTsStatus({ type: "error", message: "Timed out waiting for Funnel to be enabled." });
  };

  const handleDisableTailscale = async () => {
    setTsLoading(true);
    setTsStatus(null);
    try {
      const res = await fetch("/api/tunnel/tailscale-disable", { method: "POST" });
      const data = await res.json();
      if (res.ok) {
        setTsEnabled(false);
        setTsUrl("");
        setShowDisableTsModal(false);
        setTsStatus({ type: "success", message: "Tailscale disabled" });
      } else {
        setTsStatus({ type: "error", message: data.error || "Failed to disable Tailscale" });
      }
    } catch (e) {
      setTsStatus({ type: "error", message: e.message });
    } finally {
      setTsLoading(false);
    }
  };

  const handleOpenTsModal = async () => {
    setTsStatus(null);
    setTsInstallLog([]);
    const data = await checkTailscaleInstalled();
    if (data?.installed && data?.hasCachedPassword) {
      handleConnectTailscale();
    } else {
      setShowTsModal(true);
    }
  };

  const handleCreateKey = async () => {
    if (!newKeyName.trim()) return;

    try {
      const res = await fetch("/api/keys", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: newKeyName }),
      });
      const data = await res.json();

      if (res.ok) {
        setCreatedKey(data.key);
        await fetchData();
        setNewKeyName("");
        setShowAddModal(false);
      }
    } catch (error) {
      console.log("Error creating key:", error);
    }
  };

  const handleDeleteKey = async (id) => {
    setConfirmState({
      title: "Delete API Key",
      message: "Delete this API key?",
      onConfirm: async () => {
        setConfirmState(null);
        try {
          const res = await fetch(`/api/keys/${id}`, { method: "DELETE" });
          if (res.ok) {
            setKeys(keys.filter((k) => k.id !== id));
            setVisibleKeys(prev => {
              const next = new Set(prev);
              next.delete(id);
              return next;
            });
          }
        } catch (error) {
          console.log("Error deleting key:", error);
        }
      }
    });
  };

  const handleToggleKey = async (id, isActive) => {
    try {
      const res = await fetch(`/api/keys/${id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ isActive }),
      });
      if (res.ok) {
        setKeys(prev => prev.map(k => k.id === id ? { ...k, isActive } : k));
      }
    } catch (error) {
      console.log("Error toggling key:", error);
    }
  };

  const maskKey = (fullKey) => {
    if (!fullKey || fullKey.length <= 10) return fullKey || "";
    return fullKey.slice(0, 6) + "•".repeat(fullKey.length - 10) + fullKey.slice(-4);
  };

  const toggleKeyVisibility = (keyId) => {
    setVisibleKeys(prev => {
      const next = new Set(prev);
      if (next.has(keyId)) next.delete(keyId);
      else next.add(keyId);
      return next;
    });
  };

  const [baseUrl, setBaseUrl] = useState("/v1");

  // Hydration fix: Only access window on client side
  useEffect(() => {
    if (typeof window !== "undefined") {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time client-only hydration; this is the canonical useEffect+setState pattern, not a derived value.
      setBaseUrl(`${window.location.origin}/v1`);
    }
  }, []);

  if (loading) {
    return (
      <Stack spacing={4}>
        <CardSkeleton />
        <CardSkeleton />
      </Stack>
    );
  }

  const currentEndpoint = baseUrl;
  const headroomRunning = !!headroomStatus.running;
  const headroomLocalUrl = headroomStatus.localUrl !== false;
  const headroomCanStart = !!headroomStatus.canStart;
  const headroomManaged = headroomLocalUrl && !!headroomStatus.managedPid;
  const headroomStatusLabel = headroomStatus.loading
    ? "Checking…"
    : headroomRunning
      ? "Running"
      : headroomLocalUrl && !headroomStatus.installed
        ? "Not installed"
        : headroomLocalUrl
          ? "Proxy off"
          : "Unreachable";

  return (
    <Stack spacing={4}>
      {/* Endpoint Card */}
      <Card>
        <SectionTitle icon="api" sx={{ mb: 2 }}>API Endpoint</SectionTitle>

        {/* Endpoint rows */}
        <Stack spacing={1}>
          {/* Local */}
          <EndpointRow
            label="Local"
            url={currentEndpoint}
            copyId="local_url"
            copied={copied}
            onCopy={copy}
          />
          {/* Cloudflare Tunnel */}
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <Box component="span" sx={LABEL_CHIP(tunnelEnabled)}>Tunnel</Box>
            {tunnelEnabled && !tunnelLoading && tunnelReachable ? (
              <>
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Input value={`${tunnelPublicUrl || tunnelUrl}/v1`} readOnly className="" />
                </Box>
                <IconButton
                  onClick={() => copy(`${tunnelPublicUrl || tunnelUrl}/v1`, "tunnel_url")}
                  sx={COPY_BUTTON_SX}
                  aria-label="Copy tunnel URL"
                >
                  <MsIcon sx={{ fontSize: 18 }}>{copied === "tunnel_url" ? "check" : "content_copy"}</MsIcon>
                </IconButton>
                <IconButton onClick={() => setShowDisableTunnelModal(true)} title="Disable Tunnel" sx={POWER_BUTTON_SX}>
                  <MsIcon sx={{ fontSize: 18 }}>power_settings_new</MsIcon>
                </IconButton>
              </>
            ) : tunnelEnabled && !tunnelLoading && !tunnelReachable ? (
              <>
                <Banner tone="warn">
                  <Spinner />
                  {tunnelEverReachable ? "Tunnel reconnecting..." : "Tunnel checking..."}
                </Banner>
                <IconButton onClick={() => setShowDisableTunnelModal(true)} title="Disable Tunnel" sx={POWER_BUTTON_SX}>
                  <MsIcon sx={{ fontSize: 18 }}>power_settings_new</MsIcon>
                </IconButton>
              </>
            ) : tunnelLoading ? (
              <>
                <Banner tone="neutral">
                  <Spinner />
                  {tunnelProgress || "Creating tunnel..."}
                </Banner>
                <IconButton
                  onClick={() => { setTunnelLoading(false); setTunnelProgress(""); }}
                  title="Stop"
                  sx={POWER_BUTTON_SX}
                >
                  <MsIcon sx={{ fontSize: 18 }}>power_settings_new</MsIcon>
                </IconButton>
              </>
            ) : tunnelStatus?.type === "error" ? (
              <>
                <Banner tone="error">
                  <MsIcon sx={{ fontSize: 14 }}>error</MsIcon>
                  {tunnelStatus.message}
                </Banner>
                <Button size="sm" icon="cloud_upload" onClick={() => setShowEnableTunnelModal(true)}>Enable</Button>
              </>
            ) : tunnelChecking ? (
              <>
                <Banner tone="neutral">
                  <Spinner />
                  Checking...
                </Banner>
                <IconButton
                  onClick={() => setTunnelChecking(false)}
                  title="Stop"
                  sx={POWER_BUTTON_SX}
                >
                  <MsIcon sx={{ fontSize: 18 }}>power_settings_new</MsIcon>
                </IconButton>
              </>
            ) : (
              <Button
                size="sm"
                icon="cloud_upload"
                onClick={() => {
                  if (isLoginUnsafe) {
                    setTunnelStatus({ type: "error", message: `Security required: ${unsafeReason}` });
                    return;
                  }
                  if (!requireApiKey) {
                    setTunnelStatus({ type: "error", message: "Security required: Enable \"Require API key\" before activating the tunnel." });
                    return;
                  }
                  setShowEnableTunnelModal(true);
                }}
              >
                Enable
              </Button>
            )}
          </Box>
          {/* Tailscale */}
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <Box component="span" sx={LABEL_CHIP(tsEnabled)}>Tailscale</Box>
            {tsEnabled && !tsLoading && tsReachable ? (
              <>
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Input value={`${tsUrl}/v1`} readOnly className="" />
                </Box>
                <IconButton
                  onClick={() => copy(`${tsUrl}/v1`, "ts_url")}
                  sx={COPY_BUTTON_SX}
                  aria-label="Copy Tailscale URL"
                >
                  <MsIcon sx={{ fontSize: 18 }}>{copied === "ts_url" ? "check" : "content_copy"}</MsIcon>
                </IconButton>
                <IconButton onClick={() => setShowDisableTsModal(true)} title="Disable Tailscale" sx={POWER_BUTTON_SX}>
                  <MsIcon sx={{ fontSize: 18 }}>power_settings_new</MsIcon>
                </IconButton>
              </>
            ) : tsEnabled && !tsLoading && !tsReachable ? (
              <>
                <Banner tone="warn">
                  <Spinner />
                  {tsEverReachable ? "Tailscale reconnecting..." : "Tailscale checking..."}
                </Banner>
                <IconButton onClick={() => setShowDisableTsModal(true)} title="Disable Tailscale" sx={POWER_BUTTON_SX}>
                  <MsIcon sx={{ fontSize: 18 }}>power_settings_new</MsIcon>
                </IconButton>
              </>
            ) : (tsLoading || tsConnecting) ? (
              <>
                <Banner tone="neutral">
                  <Spinner />
                  {tsProgress || "Connecting..."}
                </Banner>
                {tsAuthUrl && (
                  <Button
                    size="sm"
                    icon="open_in_new"
                    onClick={() => window.open(tsAuthUrl, "tailscale_auth", "width=600,height=700,noopener,noreferrer")}
                  >
                    {tsAuthLabel || "Open"}
                  </Button>
                )}
                <IconButton
                  onClick={() => { setTsLoading(false); setTsConnecting(false); setTsProgress(""); clearUserAuth(); }}
                  title="Stop"
                  sx={POWER_BUTTON_SX}
                >
                  <MsIcon sx={{ fontSize: 18 }}>power_settings_new</MsIcon>
                </IconButton>
              </>
            ) : tsStatus?.type === "error" ? (
              <>
                <Banner tone="error">
                  <MsIcon sx={{ fontSize: 14 }}>error</MsIcon>
                  {tsStatus.message}
                </Banner>
                <Button size="sm" icon="vpn_lock" onClick={handleOpenTsModal}>Enable</Button>
              </>
            ) : (
              <MuiButton
                size="small"
                variant="contained"
                disableElevation
                onClick={() => {
                  if (isLoginUnsafe) {
                    setTsStatus({ type: "error", message: `Security required: ${unsafeReason}` });
                    return;
                  }
                  handleOpenTsModal();
                }}
                startIcon={<MsIcon sx={{ fontSize: 18 }}>vpn_lock</MsIcon>}
                sx={{
                  background: "linear-gradient(to right, #6366f1, #a855f7)",
                  color: "#fff",
                  textTransform: "none",
                  fontWeight: 600,
                  "&:hover": { background: "linear-gradient(to right, #4f46e5, #9333ea)" },
                }}
              >
                Enable
              </MuiButton>
            )}
          </Box>
        </Stack>

        {/* Pre-enable security gate banner */}
        {isLoginUnsafe && !tunnelEnabled && !tsEnabled && (
          <Box sx={{ mt: 2 }}>
            <SecurityWarning
              message={unsafeReason}
              action={{ label: "Open settings", href: "/dashboard/profile" }}
            />
          </Box>
        )}

        {/* Security warnings when tunnel or tailscale is active */}
        {(tunnelEnabled || tsEnabled) && (
          <Stack spacing={1} sx={{ mt: 2 }}>
            {!requireApiKey && (
              <SecurityWarning
                message="Require API key is disabled — your endpoint is publicly accessible without authentication."
                action={{ label: "Enable", href: "#require-api-key" }}
              />
            )}
            {(!requireLogin || !hasPassword) && (
              <SecurityWarning
                message={
                  !requireLogin
                    ? "Require login is disabled — anyone can access your dashboard via tunnel."
                    : "Dashboard is using the new-installation default — change it in Profile settings before exposing access."
                }
                action={{
                  label: !requireLogin ? "Enable" : "Change password",
                  href: "/dashboard/profile",
                }}
              />
            )}
          </Stack>
        )}

        {/* Tunnel dashboard access option */}
        {(tunnelEnabled || tsEnabled) && (
          <Box sx={{ mt: 2, pt: 2, borderTop: 1, borderColor: "divider", display: "flex", alignItems: "center", gap: 1.5 }}>
            <Toggle
              checked={tunnelDashboardAccess}
              onChange={() => handleTunnelDashboardAccess(!tunnelDashboardAccess)}
            />
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
              <Typography sx={{ fontWeight: 500, fontSize: 14 }}>Allow dashboard access via tunnel</Typography>
              <Tooltip text="When enabled, the dashboard can be accessed through your tunnel or Tailscale URL (login still required). When disabled, dashboard access via tunnel/Tailscale is completely blocked." />
            </Box>
          </Box>
        )}
      </Card>

      {/* Token Saver (RTK + Caveman + Ponytail + Headroom) */}
      <Card id="rtk">
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", mb: 1 }}>
          <SectionTitle icon="bolt">Token Saver</SectionTitle>
        </Box>
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", pt: 1, pb: 2, borderBottom: 1, borderColor: "divider", gap: 2 }}>
          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Typography sx={{ fontWeight: 500 }}>
              Compress tool output{" "}
              <SubtleLink href="https://github.com/rtk-ai/rtk">(RTK)</SubtleLink>
            </Typography>
            <Typography sx={{ fontSize: 14, color: "text.secondary" }}>
              git/grep/ls/tree/logs → 60-90% fewer input tokens
            </Typography>
          </Box>
          <Toggle
            checked={rtkEnabled}
            onChange={() => handleRtkEnabled(!rtkEnabled)}
          />
        </Box>
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", py: 2, borderBottom: 1, borderColor: "divider", gap: 2, flexWrap: "wrap" }}>
          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
              <Typography sx={{ fontWeight: 500 }}>
                Compress context{" "}
                <SubtleLink href="https://github.com/chopratejas/headroom">(Headroom)</SubtleLink>
              </Typography>
              <Box
                component="span"
                sx={{
                  fontSize: 12,
                  px: 1,
                  py: 0.25,
                  borderRadius: "4px",
                  bgcolor: headroomRunning ? "rgba(16,185,129,0.15)" : "rgba(245,158,11,0.15)",
                  color: headroomRunning ? "#10b981" : "#f59e0b",
                }}
              >
                {headroomStatusLabel}
              </Box>
              <Box
                component="button"
                type="button"
                onClick={() => setShowHeadroomInstallModal(true)}
                sx={{
                  fontSize: 12,
                  color: "primary.main",
                  textDecoration: "underline",
                  background: "none",
                  border: 0,
                  p: 0,
                  cursor: "pointer",
                  "&:hover": { opacity: 0.8 },
                }}
              >
                {headroomRunning ? "Manage" : "Setup"}
              </Box>
            </Box>
            <Typography sx={{ fontSize: 14, color: "text.secondary", mt: 0.5 }}>
              Compress prompts via /v1/compress before routing to the model
            </Typography>
          </Box>
          <Toggle
            checked={headroomEnabled}
            onChange={() => handleHeadroomEnabled(!headroomEnabled)}
          />
        </Box>
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", pt: 2, gap: 2, flexWrap: "wrap" }}>
          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Typography sx={{ fontWeight: 500 }}>
              Compress LLM output{" "}
              <SubtleLink href="https://github.com/JuliusBrussee/caveman">(Caveman)</SubtleLink>
            </Typography>
            <Typography sx={{ fontSize: 14, color: "text.secondary" }}>
              Terse-style system prompt → ~65% fewer output tokens (up to 87%)
            </Typography>
          </Box>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, shrink: 0 }}>
            {cavemanEnabled && (
              <Box sx={{ display: "flex", flexDirection: "column", alignItems: "flex-end", gap: 0.5 }}>
                <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
                  {visibleCavemanLevels.map((lvl) => (
                    <Button
                      key={lvl.id}
                      size="sm"
                      variant={cavemanLevel === lvl.id ? "primary" : "secondary"}
                      onClick={() => handleCavemanLevel(lvl.id)}
                      title={lvl.desc}
                    >
                      {lvl.label}
                    </Button>
                  ))}
                </Box>
                <Typography sx={{ fontSize: 12, color: "primary.main" }}>
                  {CAVEMAN_LEVELS.find((lvl) => lvl.id === cavemanLevel)?.desc}
                </Typography>
              </Box>
            )}
            <Toggle
              checked={cavemanEnabled}
              onChange={() => handleCavemanEnabled(!cavemanEnabled)}
            />
          </Box>
        </Box>
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", pt: 2, mt: 2, borderTop: 1, borderColor: "divider", gap: 2, flexWrap: "wrap" }}>
          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Typography sx={{ fontWeight: 500 }}>
              Lazy senior dev{" "}
              <SubtleLink href="https://github.com/DietrichGebert/ponytail">(Ponytail)</SubtleLink>
            </Typography>
            <Typography sx={{ fontSize: 14, color: "text.secondary" }}>
              Bias the model toward minimal code: YAGNI, reuse stdlib, deletion over addition
            </Typography>
          </Box>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, shrink: 0 }}>
            {ponytailEnabled && (
              <Box sx={{ display: "flex", flexDirection: "column", alignItems: "flex-end", gap: 0.5 }}>
                <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
                  {PONYTAIL_LEVELS.map((lvl) => (
                    <Button
                      key={lvl.id}
                      size="sm"
                      variant={ponytailLevel === lvl.id ? "primary" : "secondary"}
                      onClick={() => handlePonytailLevel(lvl.id)}
                      title={lvl.desc}
                    >
                      {lvl.label}
                    </Button>
                  ))}
                </Box>
                <Typography sx={{ fontSize: 12, color: "primary.main" }}>
                  {PONYTAIL_LEVELS.find((lvl) => lvl.id === ponytailLevel)?.desc}
                </Typography>
              </Box>
            )}
            <Toggle
              checked={ponytailEnabled}
              onChange={() => handlePonytailEnabled(!ponytailEnabled)}
            />
          </Box>
        </Box>
      </Card>

      {/* API Keys */}
      <Card id="require-api-key">
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", mb: 2 }}>
          <SectionTitle icon="vpn_key">API Keys</SectionTitle>
          <Button icon="add" onClick={() => setShowAddModal(true)}>
            Create Key
          </Button>
        </Box>

        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", pb: 2, mb: 2, borderBottom: 1, borderColor: "divider" }}>
          <Box>
            <Typography sx={{ fontWeight: 500 }}>Require API key</Typography>
            <Typography sx={{ fontSize: 14, color: "text.secondary" }}>
              Requests without a valid key will be rejected
            </Typography>
          </Box>
          <Toggle
            checked={requireApiKey}
            onChange={() => handleRequireApiKey(!requireApiKey)}
          />
        </Box>

        {isRemoteHost && !requireApiKey && (
          <Box sx={{ mb: 2, mt: -1 }}>
            <SecurityWarning message="Endpoint is exposed without an API key." />
          </Box>
        )}

        {!requireApiKey && isRemoteHost && (
          <Box
            sx={(theme) => ({
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              p: 2,
              borderRadius: "12px",
              bgcolor: theme.palette.mode === "dark" ? "rgba(38,38,38,0.5)" : "rgba(255,255,255,0.5)",
              border: 1,
              borderColor: theme.palette.mode === "dark" ? "rgba(51,51,51,0.3)" : "rgba(229,231,235,0.3)",
              mb: 2,
            })}
          >
            <Box>
              <Typography sx={{ fontSize: 14, fontWeight: 500, color: "text.primary" }}>
                Allow Remote Access Without API Key
              </Typography>
              <Typography sx={{ fontSize: 14, color: "text.secondary" }}>
                Let non-loopback requests reach /v1/* without a key (use with caution)
              </Typography>
            </Box>
            <Toggle
              checked={allowRemoteNoApiKey}
              onChange={() => handleAllowRemoteNoApiKey(!allowRemoteNoApiKey)}
            />
          </Box>
        )}

        {!requireApiKey && allowRemoteNoApiKey && (
          <Box sx={{ mb: 2, mt: -1 }}>
            <SecurityWarning message="Remote access without an API key is enabled — anyone who can reach this endpoint can use your providers." />
          </Box>
        )}

        {keys.length === 0 ? (
          <Box sx={{ textAlign: "center", py: 6 }}>
            <Box sx={{ display: "inline-flex", alignItems: "center", justifyContent: "center", width: 64, height: 64, borderRadius: "50%", bgcolor: "rgba(229,106,74,0.10)", color: "primary.main", mb: 2 }}>
              <MsIcon sx={{ fontSize: 32 }}>vpn_key</MsIcon>
            </Box>
            <Typography sx={{ color: "text.primary", fontWeight: 500, mb: 0.5 }}>No API keys yet</Typography>
            <Typography sx={{ fontSize: 14, color: "text.secondary", mb: 2 }}>Create your first API key to get started</Typography>
            <Button icon="add" onClick={() => setShowAddModal(true)}>
              Create Key
            </Button>
          </Box>
        ) : (
          <Box sx={{ display: "flex", flexDirection: "column" }}>
            {keys.map((key) => (
              <Box
                key={key.id}
                sx={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  py: 1.5,
                  borderBottom: 1,
                  borderColor: "rgba(0,0,0,0.03)",
                  "&:last-of-type": { borderBottom: 0 },
                  opacity: key.isActive === false ? 0.6 : 1,
                  "&:hover .key-actions": { opacity: 1 },
                }}
              >
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Typography sx={{ fontSize: 14, fontWeight: 500 }}>{key.name}</Typography>
                  <Box sx={{ display: "flex", alignItems: "center", gap: 1, mt: 0.5 }}>
                    <Box component="code" sx={{ fontSize: 12, color: "text.secondary", fontFamily: "ui-monospace, monospace" }}>
                      {visibleKeys.has(key.id) ? key.key : maskKey(key.key)}
                    </Box>
                    <IconButton
                      onClick={() => toggleKeyVisibility(key.id)}
                      title={visibleKeys.has(key.id) ? "Hide key" : "Show key"}
                      sx={COPY_BUTTON_SX}
                    >
                      <MsIcon sx={{ fontSize: 14 }}>
                        {visibleKeys.has(key.id) ? "visibility_off" : "visibility"}
                      </MsIcon>
                    </IconButton>
                    <IconButton
                      onClick={() => copy(key.key, key.id)}
                      sx={COPY_BUTTON_SX}
                    >
                      <MsIcon sx={{ fontSize: 14 }}>
                        {copied === key.id ? "check" : "content_copy"}
                      </MsIcon>
                    </IconButton>
                  </Box>
                  <Typography sx={{ fontSize: 12, color: "text.secondary", mt: 0.5 }}>
                    Created {new Date(key.createdAt).toLocaleDateString()}
                  </Typography>
                  {key.isActive === false && (
                    <Typography sx={{ fontSize: 12, color: "#f97316", mt: 0.5 }}>Paused</Typography>
                  )}
                  {/* ACL badges */}
                  {(key.allowedProviders || key.allowedCombos || key.allowedKinds) && (
                    <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.5, mt: 0.5 }}>
                      {key.allowedProviders && (
                        <Box
                          component="span"
                          title={key.allowedProviders.join(", ")}
                          sx={{ fontSize: 10, px: 0.75, py: 0.25, borderRadius: "4px", bgcolor: "rgba(59,130,246,0.10)", color: "#3b82f6" }}
                        >
                          {key.allowedProviders.length === 0
                            ? "No providers"
                            : key.allowedProviders.map((stored) => {
                                // Try providerList first (has connections)
                                const p = providerList.find((pp) => pp.id === stored || pp.alias === stored || pp.prefix === stored);
                                if (p) return p.displayName;
                                // Try aliasMap for providers without connections
                                const resolved = aliasMap[stored];
                                if (resolved) return resolved;
                                return stored;
                              }).join(", ")}
                        </Box>
                      )}
                      {key.allowedCombos && (
                        <Box component="span" sx={{ fontSize: 10, px: 0.75, py: 0.25, borderRadius: "4px", bgcolor: "rgba(168,85,247,0.10)", color: "#a855f7" }}>
                          {key.allowedCombos.length === 0 ? "No combos" : key.allowedCombos.join(", ")}
                        </Box>
                      )}
                      {key.allowedKinds && (
                        <Box component="span" sx={{ fontSize: 10, px: 0.75, py: 0.25, borderRadius: "4px", bgcolor: "rgba(34,197,94,0.10)", color: "#22c55e" }}>
                          {key.allowedKinds.length === 0 ? "No kinds" : key.allowedKinds.join(", ")}
                        </Box>
                      )}
                    </Box>
                  )}
                </Box>
                <Box className="key-actions" sx={{ display: "flex", alignItems: "center", gap: 1, opacity: { xs: 1, sm: 0 }, transition: "opacity 150ms" }}>
                  {/* Edit ACL button */}
                  <IconButton
                    onClick={() => handleOpenEditKey(key)}
                    title="Edit access control"
                    sx={{ color: "primary.main", "&:hover": { bgcolor: "rgba(229,106,74,0.10)" } }}
                  >
                    <MsIcon sx={{ fontSize: 18 }}>tune</MsIcon>
                  </IconButton>
                  <Toggle
                    size="sm"
                    checked={key.isActive ?? true}
                    onChange={(checked) => {
                      if (key.isActive && !checked) {
                        setConfirmState({
                          title: "Pause API Key",
                          message: `Pause API key "${key.name}"?\n\nThis key will stop working immediately but can be resumed later.`,
                          onConfirm: async () => {
                            setConfirmState(null);
                            handleToggleKey(key.id, checked);
                          }
                        });
                      } else {
                        handleToggleKey(key.id, checked);
                      }
                    }}
                    title={key.isActive ? "Pause key" : "Resume key"}
                  />
                  <IconButton
                    onClick={() => handleDeleteKey(key.id)}
                    sx={{ color: "#dc2626", "&:hover": { bgcolor: "rgba(239,68,68,0.10)" } }}
                  >
                    <MsIcon sx={{ fontSize: 18 }}>delete</MsIcon>
                  </IconButton>
                </Box>
              </Box>
            ))}
          </Box>
        )}
      </Card>

      {/* Add Key Modal */}
      <Modal
        isOpen={showAddModal}
        title="Create API Key"
        onClose={() => {
          setShowAddModal(false);
          setNewKeyName("");
        }}
      >
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Input
            label="Key Name"
            value={newKeyName}
            onChange={(e) => setNewKeyName(e.target.value)}
            placeholder="Production Key"
          />
          <Box sx={{ display: "flex", gap: 1 }}>
            <Button onClick={handleCreateKey} fullWidth disabled={!newKeyName.trim()}>
              Create
            </Button>
            <Button
              onClick={() => {
                setShowAddModal(false);
                setNewKeyName("");
              }}
              variant="ghost"
              fullWidth
            >
              Cancel
            </Button>
          </Box>
        </Box>
      </Modal>

      {/* Created Key Modal */}
      <Modal
        isOpen={!!createdKey}
        title="API Key Created"
        onClose={() => setCreatedKey(null)}
      >
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Box
            sx={(theme) => ({
              bgcolor: theme.palette.mode === "dark" ? "rgba(113,63,18,0.20)" : "#fefce8",
              border: 1,
              borderColor: theme.palette.mode === "dark" ? "#854d0e" : "#fef08a",
              borderRadius: "8px",
              p: 2,
            })}
          >
            <Typography sx={(theme) => ({ fontSize: 14, color: theme.palette.mode === "dark" ? "#fef08a" : "#854d0e", mb: 0.5, fontWeight: 500 })}>
              Save this key now!
            </Typography>
            <Typography sx={(theme) => ({ fontSize: 14, color: theme.palette.mode === "dark" ? "#fde047" : "#a16207" })}>
              This is the only time you will see this key. Store it securely.
            </Typography>
          </Box>
          <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Input
                value={createdKey || ""}
                readOnly
                className=""
              />
            </Box>
            <Button
              variant="secondary"
              icon={copied === "created_key" ? "check" : "content_copy"}
              onClick={() => copy(createdKey, "created_key")}
            >
              {copied === "created_key" ? "Copied!" : "Copy"}
            </Button>
          </Box>
          <Button onClick={() => setCreatedKey(null)} fullWidth>
            Done
          </Button>
        </Box>
      </Modal>

      {/* Edit ACL Modal */}
      <Modal
        isOpen={!!editingKey}
        title={`Edit Access: ${editingKey?.name || ""}`}
        onClose={() => !editSaving && setEditingKey(null)}
      >
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
          {/* Name */}
          <Box>
            <Typography component="label" sx={{ display: "block", fontSize: 14, fontWeight: 500, mb: 0.5 }}>Key Name</Typography>
            <Input value={editName} onChange={(e) => setEditName(e.target.value)} placeholder="Key name" />
          </Box>

          {/* Service Kinds */}
          <Box>
            <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", mb: 1 }}>
              <Typography component="label" sx={{ fontSize: 14, fontWeight: 500 }}>Service Kinds</Typography>
              <Box component="label" sx={{ display: "flex", alignItems: "center", gap: 0.75, fontSize: 12, cursor: "pointer" }}>
                <input type="checkbox" checked={editKindsAll} onChange={(e) => setEditKindsAll(e.target.checked)} />
                <Box component="span" sx={{ color: "text.secondary" }}>All allowed</Box>
              </Box>
            </Box>
            {!editKindsAll && (
              <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
                {ALL_KINDS.map((kind) => {
                  const active = editKinds.includes(kind);
                  return (
                    <Box
                      key={kind}
                      component="label"
                      sx={{
                        display: "flex",
                        alignItems: "center",
                        gap: 0.75,
                        px: 1.5,
                        py: 0.75,
                        borderRadius: "8px",
                        border: 1,
                        fontSize: 12,
                        cursor: "pointer",
                        transition: "border-color 150ms, background-color 150ms",
                        borderColor: active ? "primary.main" : "var(--color-border-subtle)",
                        bgcolor: active ? "rgba(229,106,74,0.10)" : "transparent",
                        color: active ? "primary.main" : "text.secondary",
                        "&:hover": { borderColor: "rgba(229,106,74,0.4)" },
                      }}
                    >
                      <input type="checkbox" hidden checked={active} onChange={() => toggleEditKind(kind)} />
                      {kind}
                    </Box>
                  );
                })}
              </Box>
            )}
            {editKindsAll && <Typography sx={{ fontSize: 12, color: "text.secondary" }}>This key can access all service kinds.</Typography>}
          </Box>

          {/* Providers */}
          <Box>
            <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", mb: 1 }}>
              <Typography component="label" sx={{ fontSize: 14, fontWeight: 500 }}>Providers</Typography>
              <Box component="label" sx={{ display: "flex", alignItems: "center", gap: 0.75, fontSize: 12, cursor: "pointer" }}>
                <input type="checkbox" checked={editProvidersAll} onChange={(e) => setEditProvidersAll(e.target.checked)} />
                <Box component="span" sx={{ color: "text.secondary" }}>All allowed</Box>
              </Box>
            </Box>
            {!editProvidersAll && (
              <>
                <Box sx={{ maxHeight: 240, overflowY: "auto", border: 1, borderColor: "var(--color-border-subtle)", borderRadius: "8px", p: 1, "& > * + *": { mt: 0.5 } }}>
                  {providerList.length === 0 ? (
                    <Typography sx={{ fontSize: 12, color: "text.secondary", p: 1 }}>No providers configured.</Typography>
                  ) : (
                    providerList.map((p) => {
                      const checked = editProviders.includes(p.id);
                      return (
                        <Box
                          key={p.id}
                          component="label"
                          sx={{
                            display: "flex",
                            alignItems: "center",
                            gap: 1,
                            px: 1,
                            py: 1,
                            borderRadius: "4px",
                            fontSize: 12,
                            cursor: "pointer",
                            transition: "background-color 150ms",
                            bgcolor: checked ? "rgba(229,106,74,0.10)" : "transparent",
                            color: checked ? "primary.main" : "inherit",
                            "&:hover": { bgcolor: checked ? "rgba(229,106,74,0.10)" : "var(--color-surface-2)" },
                          }}
                        >
                          <input type="checkbox" checked={checked} onChange={() => toggleEditProvider(p.id)} />
                          <Box sx={{ flex: 1, minWidth: 0 }}>
                            <Box component="span" sx={{ fontWeight: 500 }}>{p.displayName}</Box>
                            {p.prefix && (
                              <Box component="span" sx={{ ml: 0.75, fontSize: 10, px: 0.5, py: 0.25, borderRadius: "4px", bgcolor: "var(--color-surface-2)", color: "text.secondary", fontFamily: "ui-monospace, monospace" }}>{p.prefix}</Box>
                            )}
                          </Box>
                          <Box component="span" sx={{ color: "text.secondary", fontSize: 10, flexShrink: 0 }}>{p.count} conn</Box>
                        </Box>
                      );
                    })
                  )}
                </Box>
                {editProviders.length === 0 && (
                  <Box sx={{ fontSize: 12, color: "#f59e0b", bgcolor: "rgba(245,158,11,0.10)", border: 1, borderColor: "rgba(245,158,11,0.20)", borderRadius: "4px", px: 1, py: 0.5, mt: 0.75, display: "flex", alignItems: "center", gap: 0.75 }}>
                    <MsIcon sx={{ fontSize: 14 }}>warning</MsIcon>
                    <Box component="span">Warning: Zero providers selected. This key will have NO access to any providers (Deny All).</Box>
                  </Box>
                )}
              </>
            )}
            {editProvidersAll && <Typography sx={{ fontSize: 12, color: "text.secondary" }}>This key can access all providers ({providerList.length}).</Typography>}
          </Box>

          {/* Combos */}
          <Box>
            <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", mb: 1 }}>
              <Typography component="label" sx={{ fontSize: 14, fontWeight: 500 }}>Combos</Typography>
              <Box component="label" sx={{ display: "flex", alignItems: "center", gap: 0.75, fontSize: 12, cursor: "pointer" }}>
                <input type="checkbox" checked={editCombosAll} onChange={(e) => setEditCombosAll(e.target.checked)} />
                <Box component="span" sx={{ color: "text.secondary" }}>All allowed</Box>
              </Box>
            </Box>
            {!editCombosAll && (
              <Box sx={{ maxHeight: 160, overflowY: "auto", border: 1, borderColor: "var(--color-border-subtle)", borderRadius: "8px", p: 1, "& > * + *": { mt: 0.5 } }}>
                {comboList.length === 0 ? (
                  <Typography sx={{ fontSize: 12, color: "text.secondary", p: 1 }}>No combos configured.</Typography>
                ) : (
                  comboList.map((combo) => {
                    const name = combo.name || combo.id;
                    const checked = editCombos.includes(name);
                    return (
                      <Box
                        key={name}
                        component="label"
                        sx={{
                          display: "flex",
                          alignItems: "center",
                          gap: 1,
                          px: 1,
                          py: 0.75,
                          borderRadius: "4px",
                          fontSize: 12,
                          cursor: "pointer",
                          transition: "background-color 150ms",
                          bgcolor: checked ? "rgba(229,106,74,0.10)" : "transparent",
                          color: checked ? "primary.main" : "inherit",
                          "&:hover": { bgcolor: checked ? "rgba(229,106,74,0.10)" : "var(--color-surface-2)" },
                        }}
                      >
                        <input type="checkbox" checked={checked} onChange={() => toggleEditCombo(name)} />
                        <Box component="span">{name}</Box>
                        <Box component="span" sx={{ color: "text.secondary", ml: "auto", fontSize: 10 }}>{Array.isArray(combo.models) ? combo.models.length : 0} models</Box>
                      </Box>
                    );
                  })
                )}
              </Box>
            )}
            {editCombosAll && <Typography sx={{ fontSize: 12, color: "text.secondary" }}>This key can access all combos.</Typography>}
          </Box>

          {/* ACL info */}
          <Box sx={{ fontSize: 12, color: "text.secondary", bgcolor: "var(--color-surface-2)", borderRadius: "8px", p: 1.5, border: 1, borderColor: "var(--color-border-subtle)" }}>
            <Typography sx={{ fontWeight: 500, mb: 0.5 }}>How it works:</Typography>
            <Box component="ul" sx={{ listStyle: "disc", pl: 2, "& li": { mb: 0.25 } }}>
              <li><strong>All allowed</strong> = unrestricted (default). No ACL filtering.</li>
              <li><strong>Unchecked + empty</strong> = deny everything of that type.</li>
              <li><strong>Checked items</strong> = only those items are accessible.</li>
            </Box>
          </Box>

          {/* Actions */}
          <Box sx={{ display: "flex", gap: 1.5 }}>
            <Button onClick={() => setEditingKey(null)} variant="ghost" fullWidth disabled={editSaving}>
              Cancel
            </Button>
            <Button onClick={handleSaveEditKey} fullWidth disabled={editSaving}>
              {editSaving ? "Saving..." : "Save ACL"}
            </Button>
          </Box>
        </Box>
      </Modal>

      {/* Enable Tunnel Modal */}
      <Modal
        isOpen={showEnableTunnelModal}
        title="Enable Tunnel"
        onClose={() => setShowEnableTunnelModal(false)}
      >
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Box sx={{ bgcolor: "var(--color-surface-2)", border: 1, borderColor: "var(--color-border-subtle)", borderRadius: "8px", p: 2 }}>
            <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1.5 }}>
              <MsIcon sx={{ color: "primary.main" }}>cloud_upload</MsIcon>
              <Box>
                <Typography sx={{ fontSize: 14, color: "text.primary", fontWeight: 500, mb: 0.5 }}>
                  Cloudflare Tunnel
                </Typography>
                <Typography sx={{ fontSize: 14, color: "text.secondary" }}>
                  Expose your local 9Router to the internet. No port forwarding, no static IP needed. Share endpoint URL with your team or use it in Cursor, Cline, and other AI tools from anywhere.
                </Typography>
              </Box>
            </Box>
          </Box>

          <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 1.5 }}>
            {TUNNEL_BENEFITS.map((benefit) => (
              <Box key={benefit.title} sx={{ display: "flex", flexDirection: "column", alignItems: "center", textAlign: "center", p: 1.5, borderRadius: "8px", bgcolor: "var(--color-bg-alt)" }}>
                <MsIcon sx={{ fontSize: 20, color: "primary.main", mb: 0.5 }}>{benefit.icon}</MsIcon>
                <Typography sx={{ fontSize: 12, fontWeight: 600 }}>{benefit.title}</Typography>
                <Typography sx={{ fontSize: 12, color: "text.secondary" }}>{benefit.desc}</Typography>
              </Box>
            ))}
          </Box>

          <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
            Requires outbound port 7844 (TCP/UDP). Connection may take 10-30s.
          </Typography>

          <Box sx={{ display: "flex", gap: 1 }}>
            <Button onClick={handleEnableTunnel} fullWidth>
              Start Tunnel
            </Button>
            <Button onClick={() => setShowEnableTunnelModal(false)} variant="ghost" fullWidth>Cancel</Button>
          </Box>
        </Box>
      </Modal>

      {/* Disable Cloudflare Tunnel Modal */}
      <Modal
        isOpen={showDisableTunnelModal}
        title="Disable Tunnel"
        onClose={() => !tunnelLoading && setShowDisableTunnelModal(false)}
      >
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Typography sx={{ fontSize: 14, color: "text.secondary" }}>The Cloudflare tunnel will be disconnected. Remote access via tunnel URL will stop working.</Typography>
          <Box sx={{ display: "flex", gap: 1 }}>
            <Button onClick={handleDisableTunnel} fullWidth disabled={tunnelLoading} variant="danger">
              {tunnelLoading ? "Disabling..." : "Disable"}
            </Button>
            <Button onClick={() => setShowDisableTunnelModal(false)} variant="ghost" fullWidth disabled={tunnelLoading}>Cancel</Button>
          </Box>
        </Box>
      </Modal>

      {/* Tailscale Modal */}
      <Modal
        isOpen={showTsModal}
        title="Tailscale Funnel"
        onClose={() => { if (!tsInstalling) { setShowTsModal(false); setTsSudoPassword(""); setTsStatus(null); } }}
      >
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          {/* Checking state */}
          {tsInstalled === null && (
            <Typography sx={{ fontSize: 14, color: "text.secondary", display: "flex", alignItems: "center", gap: 1 }}>
              <Spinner />
              Checking...
            </Typography>
          )}

          {/* Not installed */}
          {tsInstalled === false && !tsInstalling && (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
              <Typography sx={{ fontSize: 14, color: "text.secondary" }}>Tailscale is not installed. Install it to enable Funnel.</Typography>
              <Box sx={{ display: "flex", gap: 1 }}>
                <Button onClick={handleInstallTailscale} fullWidth>
                  Install Tailscale
                </Button>
                <Button onClick={() => setShowTsModal(false)} variant="ghost" fullWidth>Cancel</Button>
              </Box>
            </Box>
          )}

          {/* Installing with progress log */}
          {tsInstalling && (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1, fontSize: 14, color: "text.secondary" }}>
                <Spinner />
                Installing Tailscale...
              </Box>
              {tsInstallLog.length > 0 && (
                <Box
                  ref={tsLogRef}
                  sx={(theme) => ({
                    bgcolor: theme.palette.mode === "dark" ? "rgba(255,255,255,0.05)" : "rgba(0,0,0,0.05)",
                    borderRadius: "4px",
                    p: 1,
                    maxHeight: 160,
                    overflowY: "auto",
                    fontFamily: "ui-monospace, monospace",
                    fontSize: 12,
                    color: "text.secondary",
                  })}
                >
                  {tsInstallLog.map((line, i) => (
                    <Box key={i}>{line}</Box>
                  ))}
                </Box>
              )}
            </Box>
          )}

          {/* Installed: show Connect button */}
          {tsInstalled === true && !tsInstalling && (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1, fontSize: 14, color: "#16a34a" }}>
                <MsIcon sx={{ fontSize: 16 }}>check_circle</MsIcon>
                Tailscale installed
              </Box>
              <Box sx={{ display: "flex", gap: 1 }}>
                <Button
                  onClick={() => handleConnectTailscale()}
                  fullWidth
                >
                  Connect
                </Button>
                <Button onClick={() => setShowTsModal(false)} variant="ghost" fullWidth>Cancel</Button>
              </Box>
            </Box>
          )}

          {tsStatus && <StatusAlert status={tsStatus} />}
        </Box>
      </Modal>

      {/* Disable Tailscale Modal */}
      <Modal
        isOpen={showDisableTsModal}
        title="Disable Tailscale"
        onClose={() => !tsLoading && setShowDisableTsModal(false)}
      >
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Typography sx={{ fontSize: 14, color: "text.secondary" }}>Tailscale Funnel will be stopped. Remote access via Tailscale URL will stop working.</Typography>
          <Box sx={{ display: "flex", gap: 1 }}>
            <Button onClick={handleDisableTailscale} fullWidth disabled={tsLoading} variant="danger">
              {tsLoading ? "Disabling..." : "Disable"}
            </Button>
            <Button onClick={() => setShowDisableTsModal(false)} variant="ghost" fullWidth disabled={tsLoading}>Cancel</Button>
          </Box>
        </Box>
      </Modal>

      {/* Headroom Install Guide Modal */}
      <Modal
        isOpen={showHeadroomInstallModal}
        title={headroomRunning ? "Headroom" : "Setup Headroom"}
        onClose={() => setShowHeadroomInstallModal(false)}
      >
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", fontSize: 14 }}>
            <Box component="span">Status</Box>
            <Box component="span" sx={{ color: headroomRunning ? "#10b981" : "#f59e0b" }}>
              {headroomStatusLabel}
            </Box>
          </Box>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
            <Typography sx={{ fontSize: 14, fontWeight: 500 }}>Proxy URL</Typography>
            <Input
              value={headroomUrl}
              onChange={(e) => setHeadroomUrl(e.target.value)}
              onBlur={handleHeadroomUrlBlur}
              placeholder="http://localhost:8787"
            />
            <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
              Use a local proxy for Start/Stop, or an external Docker sidecar like http://headroom:8787.
            </Typography>
          </Box>
          {headroomManaged ? (
            <Button onClick={handleHeadroomStop} variant="ghost" fullWidth disabled={headroomActionLoading}>
              {headroomActionLoading ? "Stopping…" : "Stop Headroom"}
            </Button>
          ) : headroomRunning ? (
            <Typography sx={{ fontSize: 14, color: "#10b981" }}>Headroom proxy is reachable. You can enable the token saver.</Typography>
          ) : headroomCanStart ? (
            <Button onClick={handleHeadroomStart} fullWidth disabled={headroomActionLoading}>
              {headroomActionLoading ? "Starting…" : "Start Headroom"}
            </Button>
          ) : !headroomLocalUrl ? (
            <Typography sx={{ fontSize: 14, color: "#f59e0b" }}>Start Headroom separately at the configured URL, then recheck.</Typography>
          ) : !headroomStatus.python ? (
            <Typography sx={{ fontSize: 14, color: "#f59e0b" }}>Python ≥ 3.10 required for local managed mode. Install Python first, or use an external proxy URL.</Typography>
          ) : (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
              <Typography sx={{ fontSize: 14, fontWeight: 500 }}>Install then click Start:</Typography>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Box
                  component="pre"
                  sx={(theme) => ({
                    flex: 1,
                    borderRadius: "4px",
                    bgcolor: theme.palette.mode === "dark" ? "rgba(255,255,255,0.05)" : "rgba(0,0,0,0.05)",
                    p: 1,
                    fontSize: 12,
                    fontFamily: "ui-monospace, monospace",
                    overflowX: "auto",
                    m: 0,
                  })}
                >{`pip install "headroom-ai[proxy]"`}</Box>
                <Button size="sm" variant="ghost" onClick={() => copy(`pip install "headroom-ai[proxy]"`)}>
                  {copied ? "Copied" : "Copy"}
                </Button>
              </Box>
            </Box>
          )}
          {headroomActionError && (
            <Typography sx={{ fontSize: 14, color: "#f59e0b" }}>{headroomActionError}</Typography>
          )}
          <Box sx={{ display: "flex", gap: 1 }}>
            <Button onClick={() => refreshHeadroomStatus()} variant="ghost" fullWidth>Recheck</Button>
            <Button onClick={() => setShowHeadroomInstallModal(false)} fullWidth>Done</Button>
          </Box>
        </Box>
      </Modal>

      {/* Confirm Modal */}
      <ConfirmModal
        isOpen={!!confirmState}
        onClose={() => setConfirmState(null)}
        onConfirm={confirmState?.onConfirm}
        title={confirmState?.title || "Confirm"}
        message={confirmState?.message}
        variant="danger"
      />
    </Stack>
  );
}
