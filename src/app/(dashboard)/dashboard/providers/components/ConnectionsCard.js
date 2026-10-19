"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import TextField from "@mui/material/TextField";
import { getStatusVariant as getConnectionStatusVariant } from "@/shared/utils/connectionStatus";
import { Card, Badge, Button, Modal, Select, Toggle, EditConnectionModal, ConfirmModal } from "@/shared/components";

const hoverBg = (t) => (t.palette.mode === "dark" ? "rgba(255,255,255,0.02)" : "rgba(0,0,0,0.02)");
const controlHoverBg = (t) => (t.palette.mode === "dark" ? "rgba(255,255,255,0.05)" : "rgba(0,0,0,0.05)");

// ── CooldownTimer ──────────────────────────────────────────────
function CooldownTimer({ until }) {
  const [remaining, setRemaining] = useState("");

  useEffect(() => {
    const update = () => {
      const diff = new Date(until).getTime() - Date.now();
      if (diff <= 0) { setRemaining(""); return; }
      const s = Math.floor(diff / 1000);
      if (s < 60) setRemaining(`${s}s`);
      else if (s < 3600) setRemaining(`${Math.floor(s / 60)}m ${s % 60}s`);
      else setRemaining(`${Math.floor(s / 3600)}h ${Math.floor((s % 3600) / 60)}m`);
    };
    update();
    const t = setInterval(update, 1000);
    return () => clearInterval(t);
  }, [until]);

  if (!remaining) return null;
  return <Box component="span" sx={{ fontSize: 12, color: "#f97316", fontFamily: "monospace" }}>⏱ {remaining}</Box>;
}

const arrowButtonSx = (disabled) => ({
  p: 0.25,
  border: 0,
  borderRadius: "4px",
  bgcolor: "transparent",
  cursor: disabled ? "not-allowed" : "pointer",
  color: disabled
    ? (t) => (t.palette.mode === "dark" ? "rgba(156,163,175,0.3)" : "rgba(107,114,128,0.3)")
    : "text.secondary",
  transition: "background-color 150ms ease, color 150ms ease",
  "&:hover": disabled ? {} : { bgcolor: "var(--color-sidebar)", color: "primary.main" },
});

const actionButtonSx = (active) => ({
  display: "flex",
  flexDirection: "column",
  alignItems: "center",
  px: 1,
  py: 0.5,
  border: 0,
  borderRadius: "4px",
  bgcolor: "transparent",
  cursor: "pointer",
  transition: "background-color 150ms ease, color 150ms ease",
  color: active ? "primary.main" : "text.secondary",
  "&:hover": { bgcolor: controlHoverBg, color: "primary.main" },
});

// ── ConnectionRow ──────────────────────────────────────────────
function ConnectionRow({ connection, proxyPools, isOAuth, isFirst, isLast, onMoveUp, onMoveDown, onToggleActive, onUpdateProxy, onEdit, onDelete }) {
  const [showProxyDropdown, setShowProxyDropdown] = useState(false);
  const [updatingProxy, setUpdatingProxy] = useState(false);
  const [isCooldown, setIsCooldown] = useState(false);
  const proxyDropdownRef = useRef(null);

  const proxyPoolMap = new Map((proxyPools || []).map((p) => [p.id, p]));
  const boundProxyPoolId = connection.providerSpecificData?.proxyPoolId || null;
  const boundProxyPool = boundProxyPoolId ? proxyPoolMap.get(boundProxyPoolId) : null;
  const hasLegacyProxy = connection.providerSpecificData?.connectionProxyEnabled === true && !!connection.providerSpecificData?.connectionProxyUrl;
  const hasAnyProxy = !!boundProxyPoolId || hasLegacyProxy;

  const proxyDisplayText = boundProxyPool
    ? `Pool: ${boundProxyPool.name}`
    : boundProxyPoolId ? `Pool: ${boundProxyPoolId} (inactive/missing)`
    : hasLegacyProxy ? `Legacy: ${connection.providerSpecificData?.connectionProxyUrl}` : "";

  let maskedProxyUrl = "";
  const rawProxyUrl = boundProxyPool?.proxyUrl || connection.providerSpecificData?.connectionProxyUrl;
  if (rawProxyUrl) {
    try {
      const p = new URL(rawProxyUrl);
      maskedProxyUrl = `${p.protocol}//${p.hostname}${p.port ? `:${p.port}` : ""}`;
    } catch { maskedProxyUrl = rawProxyUrl; }
  }

  const noProxyText = boundProxyPool?.noProxy || connection.providerSpecificData?.connectionNoProxy || "";
  const proxyBadgeVariant = boundProxyPool?.isActive === true ? "success" : (boundProxyPoolId || hasLegacyProxy) ? "error" : "default";

  const modelLockUntil = Object.entries(connection)
    .filter(([k]) => k.startsWith("modelLock_"))
    .map(([, v]) => v).filter(Boolean).sort()[0] || null;

  useEffect(() => {
    let t = null;
    const check = () => {
      const until = Object.entries(connection)
        .filter(([k]) => k.startsWith("modelLock_"))
        .map(([, v]) => v).filter(v => v && new Date(v).getTime() > Date.now()).sort()[0] || null;
      setIsCooldown(!!until);
    };
    check();
    if (modelLockUntil) {
      t = setInterval(check, 1000);
    }
    return () => { if (t) clearInterval(t); };
  }, [modelLockUntil, connection]);

  useEffect(() => {
    if (!showProxyDropdown) return;
    const handler = (e) => {
      if (proxyDropdownRef.current && !proxyDropdownRef.current.contains(e.target))
        setShowProxyDropdown(false);
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [showProxyDropdown]);

  const effectiveStatus = connection.testStatus === "unavailable" && !isCooldown ? "active" : connection.testStatus;

  const getStatusVariant = () => getConnectionStatusVariant(connection.isActive, effectiveStatus);

  const displayName = isOAuth
    ? connection.name || connection.email || connection.displayName || "OAuth Account"
    : connection.name;

  const handleSelectProxy = async (poolId) => {
    setUpdatingProxy(true);
    try { await onUpdateProxy(poolId === "__none__" ? null : poolId); }
    finally { setUpdatingProxy(false); setShowProxyDropdown(false); }
  };

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: { xs: "column", sm: "row" },
        gap: 1.5,
        p: 1,
        borderRadius: "8px",
        opacity: connection.isActive === false ? 0.6 : 1,
        transition: "background-color 150ms ease",
        "&:hover": { bgcolor: hoverBg },
        alignItems: { sm: "center" },
        justifyContent: { sm: "space-between" },
      }}
    >
      <Box sx={{ display: "flex", width: "100%", minWidth: 0, flex: 1, alignItems: { xs: "flex-start", sm: "center" }, gap: 1.5 }}>
        <Box sx={{ display: "flex", flexDirection: "column" }}>
          <Box component="button" onClick={onMoveUp} disabled={isFirst} sx={arrowButtonSx(isFirst)}>
            <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 14 }}>keyboard_arrow_up</Icon>
          </Box>
          <Box component="button" onClick={onMoveDown} disabled={isLast} sx={arrowButtonSx(isLast)}>
            <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 14 }}>keyboard_arrow_down</Icon>
          </Box>
        </Box>
        <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 16, color: "text.secondary" }}>{isOAuth ? "lock" : "key"}</Icon>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography sx={{ fontSize: 14, fontWeight: 500, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{displayName}</Typography>
          <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 1, mt: 0.5 }}>
            <Badge variant={getStatusVariant()} size="sm" dot>
              {connection.isActive === false ? "disabled" : (effectiveStatus || "Unknown")}
            </Badge>
            {hasAnyProxy && <Badge variant={proxyBadgeVariant} size="sm">Proxy</Badge>}
            {isCooldown && connection.isActive !== false && <CooldownTimer until={modelLockUntil} />}
            {connection.lastError && connection.isActive !== false && (
              <Box component="span" sx={{ fontSize: 12, color: "error.main", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", maxWidth: 300 }} title={connection.lastError}>{connection.lastError}</Box>
            )}
            <Box component="span" sx={{ fontSize: 12, color: "text.secondary" }}>#{connection.priority}</Box>
          </Box>
          {hasAnyProxy && (
            <Box sx={{ mt: 0.5, display: "flex", flexWrap: "wrap", alignItems: "center", gap: 1 }}>
              <Box component="span" sx={{ fontSize: 11, color: "text.secondary", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", maxWidth: 420 }} title={proxyDisplayText}>{proxyDisplayText}</Box>
              {maskedProxyUrl && <Box component="code" sx={{ fontSize: 10, fontFamily: "monospace", bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(255,255,255,0.05)" : "rgba(0,0,0,0.05)"), px: 0.5, py: 0.25, borderRadius: "4px", color: "text.secondary" }}>{maskedProxyUrl}</Box>}
              {noProxyText && <Box component="span" sx={{ fontSize: 11, color: "text.secondary", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", maxWidth: 320 }} title={noProxyText}>no_proxy: {noProxyText}</Box>}
            </Box>
          )}
        </Box>
      </Box>
      <Box sx={{ display: "flex", width: { xs: "100%", sm: "auto" }, flexWrap: "wrap", alignItems: "center", justifyContent: { xs: "space-between", sm: "flex-end" }, gap: 1 }}>
        <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.5 }}>
          {(proxyPools || []).length > 0 && (
            <Box sx={{ position: "relative" }} ref={proxyDropdownRef}>
              <Box
                component="button"
                onClick={() => setShowProxyDropdown((v) => !v)}
                sx={actionButtonSx(hasAnyProxy)}
                disabled={updatingProxy}
              >
                <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18 }}>{updatingProxy ? "progress_activity" : "lan"}</Icon>
                <Box component="span" sx={{ fontSize: 10, lineHeight: 1.25 }}>Proxy</Box>
              </Box>
              {showProxyDropdown && (
                <Box sx={{ position: "absolute", right: 0, top: "100%", mt: 0.5, zIndex: 50, bgcolor: "background.default", border: 1, borderColor: "divider", borderRadius: "8px", boxShadow: "0 10px 15px -3px rgba(0,0,0,0.1)", py: 0.5, minWidth: 160 }}>
                  <Box component="button" onClick={() => handleSelectProxy("__none__")} sx={{ width: "100%", textAlign: "left", px: 1.5, py: 0.75, fontSize: 14, border: 0, bgcolor: "transparent", cursor: "pointer", color: !boundProxyPoolId ? "primary.main" : "text.primary", fontWeight: !boundProxyPoolId ? 500 : 400, "&:hover": { bgcolor: controlHoverBg } }}>None</Box>
                  {(proxyPools || []).map((pool) => (
                    <Box component="button" key={pool.id} onClick={() => handleSelectProxy(pool.id)} sx={{ width: "100%", textAlign: "left", px: 1.5, py: 0.75, fontSize: 14, border: 0, bgcolor: "transparent", cursor: "pointer", color: boundProxyPoolId === pool.id ? "primary.main" : "text.primary", fontWeight: boundProxyPoolId === pool.id ? 500 : 400, "&:hover": { bgcolor: controlHoverBg } }}>{pool.name}</Box>
                  ))}
                </Box>
              )}
            </Box>
          )}
          <Box component="button" onClick={onEdit} sx={actionButtonSx(false)}>
            <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18 }}>edit</Icon>
            <Box component="span" sx={{ fontSize: 10, lineHeight: 1.25 }}>Edit</Box>
          </Box>
          <Box component="button" onClick={onDelete} sx={{ display: "flex", flexDirection: "column", alignItems: "center", px: 1, py: 0.5, border: 0, borderRadius: "4px", bgcolor: "transparent", cursor: "pointer", color: "#ef4444", transition: "background-color 150ms ease", "&:hover": { bgcolor: "rgba(239,68,68,0.1)" } }}>
            <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18 }}>delete</Icon>
            <Box component="span" sx={{ fontSize: 10, lineHeight: 1.25 }}>Delete</Box>
          </Box>
        </Box>
        <Toggle size="sm" checked={connection.isActive ?? true} onChange={onToggleActive} title={(connection.isActive ?? true) ? "Disable" : "Enable"} />
      </Box>
    </Box>
  );
}

// ── AddApiKeyModal ─────────────────────────────────────────────
function AddApiKeyModal({ isOpen, provider, providerName, proxyPools, onSave, onClose }) {
  const NONE = "__none__";
  const [formData, setFormData] = useState({ name: "", apiKey: "", priority: 1, proxyPoolId: NONE });
  const [validating, setValidating] = useState(false);
  const [validationResult, setValidationResult] = useState(null);
  const [saving, setSaving] = useState(false);

  const handleValidate = async () => {
    setValidating(true);
    try {
      const res = await fetch("/api/providers/validate", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ provider, apiKey: formData.apiKey }),
      });
      const data = await res.json();
      setValidationResult(data.valid ? "success" : "failed");
    } catch { setValidationResult("failed"); }
    finally { setValidating(false); }
  };

  const handleSubmit = async () => {
    if (!provider || !formData.apiKey) return;
    setSaving(true);
    try {
      let isValid = false;
      try {
        setValidating(true); setValidationResult(null);
        const res = await fetch("/api/providers/validate", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ provider, apiKey: formData.apiKey }),
        });
        const data = await res.json();
        isValid = !!data.valid;
        setValidationResult(isValid ? "success" : "failed");
      } catch { setValidationResult("failed"); }
      finally { setValidating(false); }
      await onSave({
        name: formData.name,
        apiKey: formData.apiKey,
        priority: formData.priority,
        proxyPoolId: formData.proxyPoolId === NONE ? null : formData.proxyPoolId,
        testStatus: isValid ? "active" : "unknown",
      });
    } finally { setSaving(false); }
  };

  if (!provider) return null;

  return (
    <Modal isOpen={isOpen} title={`Add ${providerName || provider} API Key`} onClose={onClose}>
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <TextField
          label="Name"
          size="small"
          fullWidth
          value={formData.name}
          onChange={(e) => setFormData({ ...formData, name: e.target.value })}
          placeholder="Production Key"
        />
        <Box sx={{ display: "flex", gap: 1 }}>
          <TextField
            label="API Key"
            type="password"
            size="small"
            fullWidth
            sx={{ flex: 1 }}
            value={formData.apiKey}
            onChange={(e) => setFormData({ ...formData, apiKey: e.target.value })}
          />
          <Box sx={{ pt: 3 }}>
            <Button onClick={handleValidate} disabled={!formData.apiKey || validating || saving} variant="secondary">
              {validating ? "Checking..." : "Check"}
            </Button>
          </Box>
        </Box>
        {validationResult && (
          <Badge variant={validationResult === "success" ? "success" : "error"}>
            {validationResult === "success" ? "Valid" : "Invalid"}
          </Badge>
        )}
        <TextField
          label="Priority"
          type="number"
          size="small"
          fullWidth
          value={formData.priority}
          onChange={(e) => setFormData({ ...formData, priority: Number.parseInt(e.target.value) || 1 })}
        />
        <Select label="Proxy Pool" value={formData.proxyPoolId} onChange={(e) => setFormData({ ...formData, proxyPoolId: e.target.value })}
          options={[{ value: NONE, label: "None" }, ...(proxyPools || []).map((p) => ({ value: p.id, label: p.name }))]} />
        <Box sx={{ display: "flex", gap: 1 }}>
          <Button onClick={handleSubmit} fullWidth disabled={!formData.name || !formData.apiKey || saving}>
            {saving ? "Saving..." : "Save"}
          </Button>
          <Button onClick={onClose} variant="ghost" fullWidth>Cancel</Button>
        </Box>
      </Box>
    </Modal>
  );
}


// ── ConnectionsCard ────────────────────────────────────────────
// Self-contained card: fetches, displays and manages all connections for a provider.
export default function ConnectionsCard({ providerId, isOAuth }) {
  const [connections, setConnections] = useState([]);
  const [proxyPools, setProxyPools] = useState([]);
  const [loading, setLoading] = useState(true);
  const [showAddModal, setShowAddModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [selectedConnection, setSelectedConnection] = useState(null);
  const [providerStrategy, setProviderStrategy] = useState(null);
  const [providerStickyLimit, setProviderStickyLimit] = useState("1");
  const [confirmState, setConfirmState] = useState(null);

  const fetch_ = useCallback(async () => {
    try {
      const [connRes, proxyRes, settingsRes] = await Promise.all([
        fetch("/api/providers", { cache: "no-store" }),
        fetch("/api/proxy-pools?isActive=true", { cache: "no-store" }),
        fetch("/api/settings", { cache: "no-store" }),
      ]);
      const connData = await connRes.json();
      const proxyData = await proxyRes.json();
      const settingsData = settingsRes.ok ? await settingsRes.json() : {};
      if (connRes.ok) setConnections((connData.connections || []).filter((c) => c.provider === providerId));
      if (proxyRes.ok) setProxyPools(proxyData.proxyPools || []);
      const override = (settingsData.providerStrategies || {})[providerId] || {};
      setProviderStrategy(override.fallbackStrategy || null);
      setProviderStickyLimit(override.stickyRoundRobinLimit != null ? String(override.stickyRoundRobinLimit) : "1");
    } catch (e) { console.log("ConnectionsCard fetch error:", e); }
    finally { setLoading(false); }
  }, [providerId]);

  useEffect(() => { /* eslint-disable-next-line react-hooks/set-state-in-effect -- bootstrap fetch on mount and when providerId changes. */
    fetch_(); }, [fetch_]);

  const saveStrategy = async (strategy, stickyLimit) => {
    try {
      const res = await fetch("/api/settings", { cache: "no-store" });
      const data = res.ok ? await res.json() : {};
      const current = data.providerStrategies || {};
      const override = {};
      if (strategy) override.fallbackStrategy = strategy;
      if (strategy === "round-robin" && stickyLimit !== "") override.stickyRoundRobinLimit = Number(stickyLimit) || 3;
      const updated = { ...current };
      if (Object.keys(override).length === 0) delete updated[providerId];
      else updated[providerId] = override;
      await fetch("/api/settings", { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ providerStrategies: updated }) });
    } catch (e) { console.log("saveStrategy error:", e); }
  };

  const handleSwapPriority = async (i1, i2) => {
    const next = [...connections];
    [next[i1], next[i2]] = [next[i2], next[i1]];
    setConnections(next);
    try {
      await Promise.all([
        fetch(`/api/providers/${next[i1].id}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ priority: i1 }) }),
        fetch(`/api/providers/${next[i2].id}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ priority: i2 }) }),
      ]);
    } catch { await fetch_(); }
  };

  const handleDelete = async (id) => {
    setConfirmState({
      title: "Delete Connection",
      message: "Delete this connection?",
      onConfirm: async () => {
        setConfirmState(null);
        try {
          const res = await fetch(`/api/providers/${id}`, { method: "DELETE" });
          if (res.ok) setConnections((prev) => prev.filter((c) => c.id !== id));
        } catch (e) { console.log("delete error:", e); }
      }
    });
  };

  const handleToggleActive = async (id, isActive) => {
    try {
      const res = await fetch(`/api/providers/${id}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ isActive }) });
      if (res.ok) setConnections((prev) => prev.map((c) => c.id === id ? { ...c, isActive } : c));
    } catch (e) { console.log("toggle error:", e); }
  };

  const handleUpdateProxy = async (connId, proxyPoolId) => {
    try {
      const res = await fetch(`/api/providers/${connId}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ proxyPoolId: proxyPoolId || null }) });
      if (res.ok) setConnections((prev) => prev.map((c) => c.id === connId ? { ...c, providerSpecificData: { ...c.providerSpecificData, proxyPoolId: proxyPoolId || null } } : c));
    } catch (e) { console.log("proxy error:", e); }
  };

  const handleSaveApiKey = async (formData) => {
    try {
      const res = await fetch("/api/providers", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ provider: providerId, ...formData }) });
      if (res.ok) { await fetch_(); setShowAddModal(false); }
    } catch (e) { console.log("save apikey error:", e); }
  };

  const handleUpdateConnection = async (formData) => {
    try {
      const res = await fetch(`/api/providers/${selectedConnection.id}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(formData) });
      if (res.ok) { await fetch_(); setShowEditModal(false); }
    } catch (e) { console.log("update connection error:", e); }
  };

  if (loading) return <Card><Box sx={{ height: 80, animation: "pulse 2s cubic-bezier(0.4,0,0.6,1) infinite", bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(255,255,255,0.05)" : "rgba(0,0,0,0.05)"), borderRadius: "8px" }} /></Card>;

  return (
    <>
      <Card>
        <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, alignItems: { sm: "center" }, justifyContent: { sm: "space-between" }, gap: 1.5, mb: 2 }}>
          <Typography component="h2" sx={{ fontSize: 18, fontWeight: 600 }}>Connections</Typography>
          <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 1 }}>
            <Box component="span" sx={{ fontSize: 12, color: "text.secondary", fontWeight: 500 }}>Round Robin</Box>
            <Toggle
              checked={providerStrategy === "round-robin"}
              onChange={(enabled) => {
                const strategy = enabled ? "round-robin" : null;
                setProviderStrategy(strategy);
                if (enabled && !providerStickyLimit) setProviderStickyLimit("1");
                saveStrategy(strategy, enabled ? (providerStickyLimit || "1") : providerStickyLimit);
              }}
            />
            {providerStrategy === "round-robin" && (
              <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 0.75 }}>
                <Box component="span" sx={{ fontSize: 12, color: "text.secondary" }}>Sticky:</Box>
                <TextField
                  type="number"
                  size="small"
                  value={providerStickyLimit}
                  onChange={(e) => { setProviderStickyLimit(e.target.value); saveStrategy("round-robin", e.target.value); }}
                  sx={{ width: 64, "& .MuiInputBase-input": { fontSize: 12, py: 0.5 } }}
                />
              </Box>
            )}
          </Box>
        </Box>

        {connections.length === 0 ? (
          <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, alignItems: { sm: "center" }, justifyContent: { sm: "space-between" }, gap: 1.5 }}>
            <Typography sx={{ fontSize: 14, color: "text.secondary" }}>No connections yet</Typography>
            <Button size="sm" icon="add" onClick={() => setShowAddModal(true)}>Add Connection</Button>
          </Box>
        ) : (
          <>
            <Box sx={{ display: "flex", flexDirection: "column", "& > * + *": { borderTop: 1, borderColor: "divider" } }}>
              {connections.map((conn, idx) => (
                <ConnectionRow
                  key={conn.id}
                  connection={conn}
                  proxyPools={proxyPools}
                  isOAuth={isOAuth}
                  isFirst={idx === 0}
                  isLast={idx === connections.length - 1}
                  onMoveUp={() => handleSwapPriority(idx, idx - 1)}
                  onMoveDown={() => handleSwapPriority(idx, idx + 1)}
                  onToggleActive={(isActive) => handleToggleActive(conn.id, isActive)}
                  onUpdateProxy={(poolId) => handleUpdateProxy(conn.id, poolId)}
                  onEdit={() => { setSelectedConnection(conn); setShowEditModal(true); }}
                  onDelete={() => handleDelete(conn.id)}
                />
              ))}
            </Box>
            <Box sx={{ mt: 2, display: "flex", justifyContent: { xs: "stretch", sm: "flex-start" } }}>
              <Button size="sm" icon="add" onClick={() => setShowAddModal(true)}>Add</Button>
            </Box>
          </>
        )}
      </Card>

      <AddApiKeyModal
        isOpen={showAddModal}
        provider={providerId}
        proxyPools={proxyPools}
        onSave={handleSaveApiKey}
        onClose={() => setShowAddModal(false)}
      />
      <EditConnectionModal
        isOpen={showEditModal}
        connection={selectedConnection}
        proxyPools={proxyPools}
        onSave={handleUpdateConnection}
        onClose={() => setShowEditModal(false)}
      />

      {/* Confirm Modal */}
      <ConfirmModal
        isOpen={!!confirmState}
        onClose={() => setConfirmState(null)}
        onConfirm={confirmState?.onConfirm}
        title={confirmState?.title || "Confirm"}
        message={confirmState?.message}
        variant="danger"
      />
    </>
  );
}
