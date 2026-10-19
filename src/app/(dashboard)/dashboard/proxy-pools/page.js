"use client";

import { useCallback, useEffect, useMemo, useState, useRef } from "react";
import { Badge, Button, Card, CardSkeleton, Input, Modal, Toggle, ConfirmModal } from "@/shared/components";
import { useNotificationStore } from "@/store/notificationStore";
import { countBatchResults, dedupeProxyEntries, runProxyPoolBatch } from "./batchOperations.js";
import { PUBLIC_PROXY_SOURCES } from "@/shared/constants/publicProxySources.js";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import { alpha } from "@mui/material/styles";

function MsIcon({ children, sx, ...props }) {
  return <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18, ...sx }} {...props}>{children}</Icon>;
}

const menuButtonSx = { display: "flex", width: "100%", alignItems: "center", gap: 1, borderRadius: 2, px: 1.5, py: 1, fontSize: 14, color: "text.primary", border: 0, bgcolor: "transparent", cursor: "pointer", textAlign: "left", transition: "background-color 150ms ease", "&:hover": { bgcolor: "action.hover" } };
const selectSx = { width: "100%", px: 1, py: 1, bgcolor: "background.paper", borderRadius: 1, fontSize: 14, color: "text.primary", border: 1, borderColor: "divider", outline: "none", "&:focus": { borderColor: "primary.main" } };

function parseProxyLine(line) {
  const trimmed = line.trim();
  if (!trimmed) return null;
  if (trimmed.includes("://")) {
    const parsed = new URL(trimmed);
    const hostLabel = parsed.port ? `${parsed.hostname}:${parsed.port}` : parsed.hostname;
    return { proxyUrl: parsed.toString(), name: `Imported ${hostLabel}` };
  }
  const parts = trimmed.split(":");
  if (parts.length === 4) {
    const [host, port, username, password] = parts;
    if (!host || !port || !username || !password) throw new Error("Invalid host:port:user:pass format");
    const proxyUrl = `http://${encodeURIComponent(username)}:${encodeURIComponent(password)}@${host}:${port}`;
    const parsed = new URL(proxyUrl);
    return { proxyUrl: parsed.toString(), name: `Imported ${host}:${port}` };
  }
  throw new Error("Unsupported format");
}

function getStatusVariant(status) {
  if (status === "active") return "success";
  if (status === "error") return "error";
  return "default";
}

function formatDateTime(value) {
  if (!value) return "Never";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "Never";
  return date.toLocaleString();
}

function normalizeFormData(data = {}) {
  return {
    name: data.name || "",
    proxyUrl: data.proxyUrl || "",
    noProxy: data.noProxy || "",
    isActive: data.isActive !== false,
    strictProxy: data.strictProxy === true,
  };
}

const VERCEL_TOKEN_HINT = <>Token is used once for deployment and not stored. <Box component="a" href="https://vercel.com/account/tokens" target="_blank" rel="noopener noreferrer" sx={{ color: "primary.main", "&:hover": { textDecoration: "underline" } }}>Get token →</Box></>;
const CF_TOKEN_HINT = <>Requires &quot;Workers Scripts: Edit&quot; permission. <Box component="a" href="https://dash.cloudflare.com/profile/api-tokens" target="_blank" rel="noopener noreferrer" sx={{ color: "primary.main", "&:hover": { textDecoration: "underline" } }}>Get token →</Box></>;

function PoolFormModal({ isOpen, editingProxyPool, formData, saving, onClose, onSave, onChange }) {
  return (
    <Modal isOpen={isOpen} title={editingProxyPool ? "Edit Proxy Pool" : "Add Proxy Pool"} onClose={onClose}>
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Input label="Name" value={formData.name} onChange={(e) => onChange("name", e.target.value)} placeholder="Office Proxy" />
        <Input label="Proxy URL" value={formData.proxyUrl} onChange={(e) => onChange("proxyUrl", e.target.value)} placeholder="http://127.0.0.1:7897" />
        <Input label="No Proxy" value={formData.noProxy} onChange={(e) => onChange("noProxy", e.target.value)} placeholder="localhost,127.0.0.1,.internal" hint="Comma-separated hosts/domains to bypass proxy" />
        <ToggleField label="Active" description="Inactive pools are ignored by runtime resolution." checked={formData.isActive} onChange={() => onChange("isActive", !formData.isActive)} disabled={saving} />
        <ToggleField label="Strict Proxy" description="Fail request if proxy is unreachable instead of falling back to direct." checked={formData.strictProxy} onChange={() => onChange("strictProxy", !formData.strictProxy)} disabled={saving} />
        <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)" }, gap: 1 }}>
          <Button fullWidth onClick={onSave} disabled={!formData.name.trim() || !formData.proxyUrl.trim() || saving}>{saving ? "Saving..." : "Save"}</Button>
          <Button fullWidth variant="ghost" onClick={onClose} disabled={saving}>Cancel</Button>
        </Box>
      </Box>
    </Modal>
  );
}

function ToggleField({ label, description, checked, onChange, disabled }) {
  return (
    <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1.5, alignItems: { sm: "center" }, justifyContent: { sm: "space-between" }, borderRadius: 2, border: 1, borderColor: "divider", p: 1.5 }}>
      <Box><Typography sx={{ fontWeight: 500, fontSize: 14 }}>{label}</Typography><Typography sx={{ fontSize: 12, color: "text.secondary" }}>{description}</Typography></Box>
      <Toggle checked={checked === true} onChange={onChange} disabled={disabled} />
    </Box>
  );
}

function BatchImportModal({ isOpen, text, importing, onChange, onImport, onClose }) {
  return (
    <Modal isOpen={isOpen} title="Batch Import Proxies" onClose={onClose}>
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Box>
          <Typography component="label" htmlFor="proxy-batch-import" sx={{ display: "block", mb: 0.5, fontSize: 14, fontWeight: 500, color: "text.primary" }}>Paste Proxy List (One per line)</Typography>
          <Box component="textarea" id="proxy-batch-import" value={text} onChange={(e) => onChange(e.target.value)} placeholder={"http://user:pass@127.0.0.1:7897\n127.0.0.1:7897:user:pass"} sx={{ width: "100%", minHeight: 180, py: 1, px: 1.5, fontSize: 14, color: "text.primary", bgcolor: "background.paper", border: 1, borderColor: "divider", borderRadius: 1, outline: "none", transition: "border-color 150ms ease", "&:focus": { borderColor: "primary.main" } }} />
          <Typography sx={{ fontSize: 12, color: "text.secondary", mt: 0.5 }}>Supported formats: protocol://user:pass@host:port, host:port:user:pass</Typography>
        </Box>
        <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)" }, gap: 1 }}>
          <Button fullWidth onClick={onImport} disabled={!text.trim() || importing}>{importing ? "Importing..." : "Import"}</Button>
          <Button fullWidth variant="ghost" onClick={onClose} disabled={importing}>Cancel</Button>
        </Box>
      </Box>
    </Modal>
  );
}

const PUBLIC_IMPORT_SELECT_SX = selectSx;

function PublicImportModal({ isOpen, form, importing, onChange, onImport, onClose }) {
  const source = PUBLIC_PROXY_SOURCES[form.source] || PUBLIC_PROXY_SOURCES.freeproxydb;
  const vpsLabSocks = form.source === "vpslab" && form.protocol === "socks5";
  const anonymityDisabled = !source.supportsAnonymity || (vpsLabSocks && !source.anonymityProtocols?.includes("socks5"));

  return (
    <Modal isOpen={isOpen} title="Import from Public Lists" onClose={onClose}>
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Box>
          <Typography component="label" htmlFor="public-proxy-source" sx={{ display: "block", mb: 0.5, fontSize: 14, fontWeight: 500, color: "text.primary" }}>Source</Typography>
          <Box
            component="select"
            id="public-proxy-source"
            value={form.source}
            onChange={(e) => onChange("source", e.target.value)}
            sx={PUBLIC_IMPORT_SELECT_SX}
            disabled={importing}
          >
            {Object.values(PUBLIC_PROXY_SOURCES).map((entry) => (
              <option key={entry.id} value={entry.id}>{entry.label}</option>
            ))}
          </Box>
          <Typography sx={{ fontSize: 12, color: "text.secondary", mt: 0.5 }}>{source.note}</Typography>
        </Box>
        <Box sx={{ display: "grid", gridTemplateColumns: "repeat(2, 1fr)", gap: 1.5 }}>
          <Box>
            <Typography component="label" htmlFor="public-proxy-count" sx={{ display: "block", mb: 0.5, fontSize: 14, fontWeight: 500, color: "text.primary" }}>Count</Typography>
            <Input
              id="public-proxy-count"
              type="number"
              value={form.count}
              onChange={(e) => onChange("count", e.target.value)}
              placeholder={String(source.defaultCount)}
              hint={`${source.minCount}-${source.maxCount}`}
              disabled={importing}
            />
          </Box>
          <Box>
            <Typography component="label" htmlFor="public-proxy-protocol" sx={{ display: "block", mb: 0.5, fontSize: 14, fontWeight: 500, color: "text.primary" }}>Protocol</Typography>
            <Box
              component="select"
              id="public-proxy-protocol"
              value={form.protocol}
              onChange={(e) => onChange("protocol", e.target.value)}
              sx={PUBLIC_IMPORT_SELECT_SX}
              disabled={importing}
            >
              {source.protocols.map((protocol) => (
                <option key={protocol} value={protocol}>{protocol.toUpperCase()}</option>
              ))}
            </Box>
          </Box>
        </Box>
        {source.supportsCountry ? (
          <Input
            label="Country"
            value={form.country}
            onChange={(e) => onChange("country", e.target.value)}
            placeholder="US,CA"
            hint="Optional country codes, comma-separated"
            disabled={importing}
          />
        ) : null}
        <Box>
          <Typography component="label" htmlFor="public-proxy-anonymity" sx={{ display: "block", mb: 0.5, fontSize: 14, fontWeight: 500, color: "text.primary" }}>Anonymity</Typography>
          <Box
            component="select"
            id="public-proxy-anonymity"
            value={form.anonymity}
            onChange={(e) => onChange("anonymity", e.target.value)}
            sx={PUBLIC_IMPORT_SELECT_SX}
            disabled={importing || anonymityDisabled}
          >
            <option value="">any</option>
            <option value="elite">elite</option>
            <option value="anonymous">anonymous</option>
            <option value="transparent">transparent</option>
          </Box>
          {anonymityDisabled ? <Typography sx={{ fontSize: 12, color: "text.secondary", mt: 0.5 }}>VPSLab socks5 list has no anonymity cuts.</Typography> : null}
        </Box>
        <ToggleField
          label="HTTPS only"
          description="Only proxies verified as HTTPS-capable."
          checked={form.httpsOnly === true}
          onChange={() => onChange("httpsOnly", !form.httpsOnly)}
          disabled={importing || (form.source === "vpslab" && form.protocol === "socks5")}
        />
        {source.supportsMaxSpeed ? (
          <Input
            label="Max speed (s)"
            value={form.maxSpeed}
            onChange={(e) => onChange("maxSpeed", e.target.value)}
            placeholder="10"
            hint="Optional upper bound on response time in seconds"
            type="number"
            disabled={importing}
          />
        ) : null}
        {source.supportsOrderBy ? (
          <Box>
            <Typography component="label" htmlFor="public-proxy-order" sx={{ display: "block", mb: 0.5, fontSize: 14, fontWeight: 500, color: "text.primary" }}>Order by</Typography>
            <Box
              component="select"
              id="public-proxy-order"
              value={form.orderBy}
              onChange={(e) => onChange("orderBy", e.target.value)}
              sx={PUBLIC_IMPORT_SELECT_SX}
              disabled={importing}
            >
              {source.orderByOptions.map((option) => (
                <option key={option.value} value={option.value}>{option.label}</option>
              ))}
            </Box>
            <Typography sx={{ fontSize: 12, color: "text.secondary", mt: 0.5 }}>Fastest = lowest measured latency; unmeasured proxies sort last.</Typography>
          </Box>
        ) : null}
        <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
          Imported pools start active. Free proxies churn fast — run Health Check after importing and disable the dead ones.
        </Typography>
        <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)" }, gap: 1 }}>
          <Button fullWidth onClick={onImport} disabled={importing}>{importing ? "Importing..." : "Import"}</Button>
          <Button fullWidth variant="ghost" onClick={onClose} disabled={importing}>Cancel</Button>
        </Box>
      </Box>
    </Modal>
  );
}

function DeploymentModal({ isOpen, title, onClose, children }) {
  return <Modal isOpen={isOpen} title={title} onClose={onClose}>{children}</Modal>;
}

function PoolRow({ pool, selected, testing, onSelect, onToggle, onTest, onEdit, onDelete }) {
  return (
    <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1.5, py: 1.5, alignItems: { sm: "center" }, justifyContent: { sm: "space-between" } }}>
      <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1.5, minWidth: 0, flex: 1 }}>
        <Box component="input" type="checkbox" checked={selected} onChange={onSelect} aria-label={`Select proxy ${pool.name || pool.id}`} sx={{ mt: 0.5, width: 16, height: 16, flexShrink: 0, accentColor: "primary.main" }} />
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
            <Typography sx={{ minWidth: 0, maxWidth: { xs: "100%", sm: 288 }, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontSize: 14, fontWeight: 500 }}>{pool.name}</Typography>
            <Badge variant={getStatusVariant(pool.testStatus)} size="sm" dot>{pool.testStatus || "unknown"}</Badge>
            <Badge variant={pool.isActive ? "success" : "default"} size="sm">{pool.isActive ? "active" : "inactive"}</Badge>
            {pool.type === "vercel" && <Badge variant="default" size="sm">vercel relay</Badge>}
            {pool.type === "cloudflare" && <Badge variant="default" size="sm">cloudflare relay</Badge>}
            <Badge variant="default" size="sm">{pool.boundConnectionCount || 0} bound</Badge>
          </Box>
          <Typography sx={{ fontSize: 12, color: "text.secondary", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", mt: 0.5 }}>{pool.proxyUrl}</Typography>
          {pool.noProxy ? <Typography sx={{ fontSize: 12, color: "text.secondary", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>No proxy: {pool.noProxy}</Typography> : null}
          <Typography sx={{ fontSize: 11, color: "text.secondary", mt: 0.5 }}>Last tested: {formatDateTime(pool.lastTestedAt)}{pool.lastError ? ` · ${pool.lastError}` : ""}</Typography>
        </Box>
      </Box>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "flex-end", gap: 0.5 }}>
        <Toggle size="sm" checked={pool.isActive === true} onChange={onToggle} title={pool.isActive ? "Disable" : "Enable"} />
        <Box component="button" type="button" onClick={onTest} sx={{ p: 1, borderRadius: 1, border: 0, bgcolor: "transparent", color: "text.secondary", cursor: testing ? "not-allowed" : "pointer", "&:hover": { bgcolor: "action.hover", color: "primary.main" } }} title="Test proxy" disabled={testing}>
          <MsIcon sx={{ fontSize: 18, ...(testing ? { animation: "spin 1s linear infinite" } : {}) }}>{testing ? "progress_activity" : "science"}</MsIcon>
        </Box>
        <Box component="button" type="button" onClick={onEdit} sx={{ p: 1, borderRadius: 1, border: 0, bgcolor: "transparent", color: "text.secondary", cursor: "pointer", "&:hover": { bgcolor: "action.hover", color: "primary.main" } }} title="Edit"><MsIcon sx={{ fontSize: 18 }}>edit</MsIcon></Box>
        <Box component="button" type="button" onClick={onDelete} sx={{ p: 1, borderRadius: 1, border: 0, bgcolor: "transparent", color: "error.main", cursor: "pointer", "&:hover": { bgcolor: (theme) => alpha(theme.palette.error.main, 0.1) } }} title="Delete"><MsIcon sx={{ fontSize: 18 }}>delete</MsIcon></Box>
      </Box>
    </Box>
  );
}

export default function ProxyPoolsPage() {
  const [proxyPools, setProxyPools] = useState([]);
  const [loading, setLoading] = useState(true);
  const [showFormModal, setShowFormModal] = useState(false);
  const [showBatchImportModal, setShowBatchImportModal] = useState(false);
  const [showVercelModal, setShowVercelModal] = useState(false);
  const [showCloudflareModal, setShowCloudflareModal] = useState(false);
  const [showDenoModal, setShowDenoModal] = useState(false);
  const [showRelayMenu, setShowRelayMenu] = useState(false);
  const [showAddMenu, setShowAddMenu] = useState(false);
  const [showPublicImportModal, setShowPublicImportModal] = useState(false);
  const [publicImportForm, setPublicImportForm] = useState({
    source: "freeproxydb",
    count: PUBLIC_PROXY_SOURCES.freeproxydb.defaultCount,
    protocol: "http",
    country: "",
    anonymity: "",
    httpsOnly: false,
    maxSpeed: "",
    orderBy: "success",
  });
  const [publicImporting, setPublicImporting] = useState(false);
  const [editingProxyPool, setEditingProxyPool] = useState(null);
  const [formData, setFormData] = useState(() => normalizeFormData());
  const [batchImportText, setBatchImportText] = useState("");
  const [vercelForm, setVercelForm] = useState({ vercelToken: "", projectName: "vercel-relay" });
  const [cloudflareForm, setCloudflareForm] = useState({ accountId: "", apiToken: "", projectName: "cloudflare-relay" });
  const [denoForm, setDenoForm] = useState({ denoToken: "", orgDomain: "", projectName: "" });
  const [saving, setSaving] = useState(false);
  const [importing, setImporting] = useState(false);
  const [deploying, setDeploying] = useState(false);
  const [testingId, setTestingId] = useState(null);
  const [selectedIds, setSelectedIds] = useState([]);
  const [healthChecking, setHealthChecking] = useState(false);
  const [healthProgress, setHealthProgress] = useState({ current: 0, total: 0 });
  const [bulkBusy, setBulkBusy] = useState(false);
  const [batchProgress, setBatchProgress] = useState({ label: "", current: 0, total: 0 });
  const [confirmState, setConfirmState] = useState(null);
  const relayMenuRef = useRef(null);
  const addMenuRef = useRef(null);
  const notify = useNotificationStore();

  useEffect(() => {
    const handleClickOutside = (e) => {
      if (relayMenuRef.current && !relayMenuRef.current.contains(e.target)) {
        setShowRelayMenu(false);
      }
      if (addMenuRef.current && !addMenuRef.current.contains(e.target)) {
        setShowAddMenu(false);
      }
    };
    if (showRelayMenu || showAddMenu) {
      document.addEventListener("mousedown", handleClickOutside);
    }
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [showRelayMenu, showAddMenu]);

  const fetchProxyPools = useCallback(async () => {
    try {
      const res = await fetch("/api/proxy-pools?includeUsage=true", { cache: "no-store" });
      const data = await res.json();
      if (res.ok) {
        setProxyPools(data.proxyPools || []);
      }
    } catch (error) {
      console.log("Error fetching proxy pools:", error);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- bootstrap fetch.
    fetchProxyPools();
  }, [fetchProxyPools]);

  const resetForm = () => {
    setEditingProxyPool(null);
    setFormData(normalizeFormData());
  };

  const openCreateModal = () => {
    resetForm();
    setShowFormModal(true);
  };

  const openEditModal = (proxyPool) => {
    setEditingProxyPool(proxyPool);
    setFormData(normalizeFormData(proxyPool));
    setShowFormModal(true);
  };

  const closeFormModal = () => {
    setShowFormModal(false);
    resetForm();
  };

  const handleSave = async () => {
    const payload = {
      name: formData.name.trim(),
      proxyUrl: formData.proxyUrl.trim(),
      noProxy: formData.noProxy.trim(),
      isActive: formData.isActive === true,
      strictProxy: formData.strictProxy === true,
    };

    if (!payload.name || !payload.proxyUrl) return;

    setSaving(true);
    try {
      const isEdit = !!editingProxyPool;
      const res = await fetch(isEdit ? `/api/proxy-pools/${editingProxyPool.id}` : "/api/proxy-pools", {
        method: isEdit ? "PUT" : "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });

      if (res.ok) {
        await fetchProxyPools();
        closeFormModal();
        notify.success(editingProxyPool ? "Proxy pool updated" : "Proxy pool created");
      } else {
        const data = await res.json();
        notify.error(data.error || "Failed to save proxy pool");
      }
    } catch (error) {
      console.log("Error saving proxy pool:", error);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (proxyPool) => {
    setConfirmState({
      title: "Delete Proxy Pool",
      message: `Delete proxy pool "${proxyPool.name}"?`,
      onConfirm: async () => {
        setConfirmState(null);
        try {
          const res = await fetch(`/api/proxy-pools/${proxyPool.id}`, { method: "DELETE" });
          if (res.ok) {
            setProxyPools((prev) => prev.filter((item) => item.id !== proxyPool.id));
            notify.success("Proxy pool deleted");
            return;
          }

          const data = await res.json();
          if (res.status === 409) {
            notify.warning(`Cannot delete: ${data.boundConnectionCount || 0} connection(s) are still using this pool.`);
          } else {
            notify.error(data.error || "Failed to delete proxy pool");
          }
        } catch (error) {
          console.log("Error deleting proxy pool:", error);
          notify.error("Failed to delete proxy pool");
        }
      }
    });
  };

  const handleTest = async (proxyPoolId) => {
    setTestingId(proxyPoolId);
    try {
      const res = await fetch(`/api/proxy-pools/${proxyPoolId}/test`, { method: "POST" });
      const data = await res.json();

      if (!res.ok) {
        notify.error(data.error || "Failed to test proxy");
        return;
      }

      await fetchProxyPools();
      notify.success(data.ok ? "Proxy test passed" : "Proxy test failed");
    } catch (error) {
      console.log("Error testing proxy pool:", error);
      notify.error("Failed to test proxy");
    } finally {
      setTestingId(null);
    }
  };

  const handleToggleActive = async (pool) => {
    const next = !pool.isActive;
    setProxyPools((prev) => prev.map((p) => p.id === pool.id ? { ...p, isActive: next } : p));
    try {
      const res = await fetch(`/api/proxy-pools/${pool.id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ isActive: next }),
      });
      if (!res.ok) {
        setProxyPools((prev) => prev.map((p) => p.id === pool.id ? { ...p, isActive: pool.isActive } : p));
        notify.error("Failed to update active state");
      }
    } catch (error) {
      console.log("Error toggling active:", error);
      setProxyPools((prev) => prev.map((p) => p.id === pool.id ? { ...p, isActive: pool.isActive } : p));
    }
  };

  const validSelectedIds = selectedIds.filter((id) => proxyPools.some((p) => p.id === id));
  const allSelected = proxyPools.length > 0 && validSelectedIds.length === proxyPools.length;
  const toggleSelect = (id) => setSelectedIds((prev) => prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]);
  const toggleSelectAll = () => setSelectedIds(allSelected ? [] : proxyPools.map((p) => p.id));
  const clearSelection = () => setSelectedIds([]);

  const bulkSetActive = async (isActive) => {
    const targets = selectedIds.length > 0 ? selectedIds : proxyPools.map((p) => p.id);
    if (targets.length === 0) return;
    setBulkBusy(true);
    setBatchProgress({ label: isActive ? "Activating" : "Deactivating", current: 0, total: targets.length });
    try {
      const results = await runProxyPoolBatch(targets, async (id) => {
        const res = await fetch(`/api/proxy-pools/${id}`, {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ isActive }),
        });
        return res.ok ? "ok" : "fail";
      }, (current) => setBatchProgress((prev) => ({ ...prev, current })));
      const { ok = 0, fail = 0 } = countBatchResults(results);
      const failed = fail;
      await fetchProxyPools();
      notify.success(`${isActive ? "Activated" : "Deactivated"} ${ok}${failed ? `, failed ${failed}` : ""}`);
    } finally {
      setBatchProgress({ label: "", current: 0, total: 0 });
      setBulkBusy(false);
    }
  };

  const bulkDelete = async () => {
    if (selectedIds.length === 0) return;
    setConfirmState({
      title: "Delete Proxy Pools",
      message: `Delete ${selectedIds.length} proxy pool(s)?`,
      onConfirm: async () => {
        setConfirmState(null);
        setBulkBusy(true);
        const targets = [...selectedIds];
        setBatchProgress({ label: "Deleting", current: 0, total: targets.length });
        try {
          const results = await runProxyPoolBatch(targets, async (id) => {
            const res = await fetch(`/api/proxy-pools/${id}`, { method: "DELETE" });
            if (res.ok) return "ok";
            if (res.status === 409) return "blocked";
            return "fail";
          }, (current) => setBatchProgress((prev) => ({ ...prev, current })));
          const { ok = 0, blocked = 0, fail: failed = 0 } = countBatchResults(results);
          await fetchProxyPools();
          clearSelection();
          notify.success(`Deleted ${ok}${blocked ? `, ${blocked} bound` : ""}${failed ? `, ${failed} failed` : ""}`);
        } finally {
          setBatchProgress({ label: "", current: 0, total: 0 });
          setBulkBusy(false);
        }
      }
    });
  };

  const handleHealthCheck = async () => {
    const targets = selectedIds.length > 0
      ? proxyPools.filter((p) => selectedIds.includes(p.id))
      : proxyPools;
    if (targets.length === 0) return;
    setHealthChecking(true);
    setHealthProgress({ current: 0, total: targets.length });
    let alive = 0; const deadIds = [];
    let done = 0;
    const CONCURRENCY = 10;
    const queue = [...targets];

    const worker = async () => {
      while (queue.length > 0) {
        const pool = queue.shift();
        if (!pool) break;
        try {
          const res = await fetch(`/api/proxy-pools/${pool.id}/test`, { method: "POST" });
          const data = await res.json();
          if (res.ok && data.ok) alive += 1; else deadIds.push(pool.id);
        } catch {
          deadIds.push(pool.id);
        } finally {
          done += 1;
          setHealthProgress({ current: done, total: targets.length });
        }
      }
    };

    await Promise.all(Array.from({ length: Math.min(CONCURRENCY, targets.length) }, worker));
    await fetchProxyPools();
    setHealthChecking(false);
    setHealthProgress({ current: 0, total: 0 });

    if (deadIds.length > 0) {
      setConfirmState({
        title: "Disable Dead Proxies",
        message: `Alive: ${alive}, Dead: ${deadIds.length}.\n\nDisable ${deadIds.length} dead proxies?`,
        onConfirm: async () => {
          setConfirmState(null);
          setBulkBusy(true);
          try {
            await Promise.all(deadIds.map(id =>
              fetch(`/api/proxy-pools/${id}`, {
                method: "PUT",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ isActive: false }),
              }).catch(() => {})
            ));
            await fetchProxyPools();
            notify.success(`Disabled ${deadIds.length} dead proxies`);
          } finally {
            setBulkBusy(false);
          }
        }
      });
    } else {
      notify.success(`Health check done. Alive: ${alive}, Dead: ${deadIds.length}`);
    }
  };

  const openBatchImportModal = () => {
    setBatchImportText("");
    setShowBatchImportModal(true);
  };

  const closeBatchImportModal = () => {
    if (importing) return;
    setShowBatchImportModal(false);
  };

  const openPublicImportModal = () => {
    setPublicImportForm({
      source: "freeproxydb",
      count: PUBLIC_PROXY_SOURCES.freeproxydb.defaultCount,
      protocol: "http",
      country: "",
      anonymity: "",
      httpsOnly: false,
      maxSpeed: "",
      orderBy: "success",
    });
    setShowPublicImportModal(true);
  };

  const closePublicImportModal = () => {
    if (publicImporting) return;
    setShowPublicImportModal(false);
  };

  const handlePublicImportField = (field, value) => {
    setPublicImportForm((prev) => {
      if (field === "source") {
        const source = PUBLIC_PROXY_SOURCES[value] || prev.source;
        return {
          source: value,
          count: source.defaultCount,
          protocol: "http",
          country: source.supportsCountry ? prev.country : "",
          anonymity: "",
          httpsOnly: false,
          maxSpeed: source.supportsMaxSpeed ? prev.maxSpeed : "",
          orderBy: "success",
        };
      }
      return { ...prev, [field]: value };
    });
  };

  const handlePublicImport = async () => {
    setPublicImporting(true);
    try {
      const res = await fetch("/api/proxy-pools/import-public", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(publicImportForm),
      });
      const data = await res.json();
      if (res.ok) {
        await fetchProxyPools();
        setShowPublicImportModal(false);
        if (data.imported === 0 && data.reason) {
          notify.warning(data.reason);
        } else {
          notify.success(
            `Public import: created ${data.imported}, skipped ${data.duplicates} duplicates${data.failed ? `, failed ${data.failed}` : ""}`
          );
        }
      } else {
        notify.error(data.error || "Public import failed");
      }
    } catch (error) {
      console.log("Error importing public proxies:", error);
      notify.error("Public import failed");
    } finally {
      setPublicImporting(false);
    }
  };

  const openVercelModal = () => {
    setVercelForm({ vercelToken: "", projectName: "vercel-relay" });
    setShowVercelModal(true);
  };

  const closeVercelModal = () => {
    if (deploying) return;
    setShowVercelModal(false);
  };

  const openCloudflareModal = () => {
    setCloudflareForm({ accountId: "", apiToken: "", projectName: "cloudflare-relay" });
    setShowCloudflareModal(true);
  };

  const closeCloudflareModal = () => {
    if (deploying) return;
    setShowCloudflareModal(false);
  };

  const openDenoModal = () => {
    setDenoForm({ denoToken: "", orgDomain: "", projectName: "" });
    setShowDenoModal(true);
  };

  const closeDenoModal = () => {
    if (deploying) return;
    setShowDenoModal(false);
  };

  const handleVercelDeploy = async () => {
    if (!vercelForm.vercelToken.trim()) return;
    setDeploying(true);
    try {
      const res = await fetch("/api/proxy-pools/vercel-deploy", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(vercelForm),
      });
      const data = await res.json();
      if (res.ok) {
        await fetchProxyPools();
        closeVercelModal();
        notify.success(`Deployed: ${data.deployUrl}`);
      } else {
        notify.error(data.error || "Deploy failed");
      }
    } catch (error) {
      console.log("Error deploying Vercel relay:", error);
      notify.error("Deploy failed");
    } finally {
      setDeploying(false);
    }
  };

  const handleCloudflareDeploy = async () => {
    if (!cloudflareForm.accountId.trim() || !cloudflareForm.apiToken.trim()) return;
    setDeploying(true);
    try {
      const res = await fetch("/api/proxy-pools/cloudflare-deploy", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(cloudflareForm),
      });
      const data = await res.json();
      if (res.ok) {
        await fetchProxyPools();
        closeCloudflareModal();
        notify.success(`Deployed: ${data.deployUrl}`);
      } else {
        notify.error(data.error || "Deploy failed");
      }
    } catch (error) {
      console.log("Error deploying Cloudflare relay:", error);
      notify.error("Deploy failed");
    } finally {
      setDeploying(false);
    }
  };

  const handleDenoDeploy = async () => {
    if (!denoForm.denoToken.trim()) return;
    setDeploying(true);
    try {
      const res = await fetch("/api/proxy-pools/deno-deploy", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(denoForm),
      });
      const data = await res.json();
      if (res.ok) {
        await fetchProxyPools();
        closeDenoModal();
        notify.success(`Deployed: ${data.deployUrl}`);
      } else {
        notify.error(data.error || "Deploy failed");
      }
    } catch (error) {
      console.log("Error deploying Deno relay:", error);
      notify.error("Deploy failed");
    } finally {
      setDeploying(false);
    }
  };

  const handleBatchImport = async () => {
    const lines = batchImportText
      .split(/\r?\n/)
      .flatMap((line) => { const t = line.trim(); return t ? [t] : []; });

    if (lines.length === 0) {
      notify.warning("Please paste at least one proxy line.");
      return;
    }

    const parsedEntries = [];
    const invalidLines = [];

    lines.forEach((line, index) => {
      try {
        const parsed = parseProxyLine(line);
        if (parsed) {
          parsedEntries.push({
            ...parsed,
            lineNumber: index + 1,
          });
        }
      } catch (error) {
        invalidLines.push(`Line ${index + 1}: ${error.message}`);
      }
    });

    if (invalidLines.length > 0) {
      notify.error(`Invalid proxy format:\n${invalidLines.join("\n")}`);
      return;
    }

    setImporting(true);
    try {
      const existingKeys = new Set(
        proxyPools.map((pool) => `${(pool.proxyUrl || "").trim()}|||${(pool.noProxy || "").trim()}`)
      );

      let created = 0;
      let skipped = 0;
      let failed = 0;

      const { accepted: toCreate, skipped: duplicateCount } = dedupeProxyEntries(parsedEntries, existingKeys);
      skipped += duplicateCount;

      setBatchProgress({ label: "Importing", current: 0, total: toCreate.length });
      const results = await runProxyPoolBatch(toCreate, async (entry) => {
        const res = await fetch("/api/proxy-pools", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            name: entry.name,
            proxyUrl: entry.proxyUrl,
            noProxy: "",
            isActive: true,
          }),
        });
        return res.ok ? "ok" : "fail";
      }, (current) => setBatchProgress((prev) => ({ ...prev, current })));
      const counts = countBatchResults(results);
      created = counts.ok || 0;
      failed = counts.fail || 0;

      await fetchProxyPools();
      setShowBatchImportModal(false);
      notify.success(`Batch import completed: Created ${created}, Skipped ${skipped}, Failed ${failed}`);
    } catch (error) {
      console.log("Error batch importing proxies:", error);
      notify.error("Batch import failed");
    } finally {
      setBatchProgress({ label: "", current: 0, total: 0 });
      setImporting(false);
    }
  };

  const activeCount = useMemo(
    () => proxyPools.filter((pool) => pool.isActive === true).length,
    [proxyPools]
  );

  if (loading) {
    return (
      <Box sx={{ mx: "auto", display: "flex", width: "100%", maxWidth: 1024, flexDirection: "column", gap: { xs: 2, sm: 3 }, px: { xs: 0.5, sm: 0 } }}>
        <CardSkeleton />
        <CardSkeleton />
      </Box>
    );
  }

  return (
    <Box sx={{ mx: "auto", display: "flex", width: "100%", maxWidth: 1024, flexDirection: "column", gap: { xs: 2, sm: 3 }, px: { xs: 0.5, sm: 0 } }}>
      <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1.5, alignItems: { sm: "flex-start" }, justifyContent: { sm: "space-between" } }}>
        <Box sx={{ minWidth: 0 }}>
          <Typography component="h1" sx={{ fontSize: { xs: 20, sm: 24 }, fontWeight: 600 }}>Proxy Pools</Typography>
        </Box>

        <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "repeat(3, auto)" }, gap: 1, alignItems: { sm: "center" } }}>
          <Box sx={{ position: "relative" }} ref={relayMenuRef}>
            <Button
              size="sm"
              variant="secondary"
              icon="rocket_launch"
              onClick={() => setShowRelayMenu(!showRelayMenu)}
            >
              Deploy Relay
              <MsIcon sx={{ ml: 0.5, fontSize: 18 }}>{showRelayMenu ? "expand_less" : "expand_more"}</MsIcon>
            </Button>

            {showRelayMenu && (
              <Box sx={{ position: "absolute", left: { xs: 0, sm: "auto" }, right: { sm: 0 }, top: "100%", zIndex: 50, mt: 0.5, width: 192, borderRadius: 3, border: 1, borderColor: "divider", bgcolor: "background.paper", p: 0.5, boxShadow: 6 }}>
                <Box component="button" type="button"
                  onClick={() => {
                    openCloudflareModal();
                    setShowRelayMenu(false);
                  }}
                  sx={menuButtonSx}
                >
                  <MsIcon sx={{ fontSize: 20, color: "warning.main" }}>cloud</MsIcon>
                  Cloudflare Relay
                </Box>
                <Box component="button" type="button"
                  onClick={() => {
                    openVercelModal();
                    setShowRelayMenu(false);
                  }}
                  sx={menuButtonSx}
                >
                  <MsIcon sx={{ fontSize: 20, color: "info.main" }}>cloud_upload</MsIcon>
                  Vercel Relay
                </Box>
                <Box component="button" type="button"
                  onClick={() => {
                    openDenoModal();
                    setShowRelayMenu(false);
                  }}
                  sx={menuButtonSx}
                >
                  <MsIcon sx={{ fontSize: 20, color: "success.main" }}>terminal</MsIcon>
                  Deno Relay
                </Box>
              </Box>
            )}
          </Box>

          <Button size="sm" variant="secondary" icon="upload" onClick={openBatchImportModal}>
            Batch Import
          </Button>
          <Box sx={{ position: "relative" }} ref={addMenuRef}>
            <Button
              size="sm"
              icon="add"
              onClick={() => setShowAddMenu(!showAddMenu)}
            >
              Add Proxy Pool
              <MsIcon sx={{ ml: 0.5, fontSize: 18 }}>{showAddMenu ? "expand_less" : "expand_more"}</MsIcon>
            </Button>

            {showAddMenu && (
              <Box sx={{ position: "absolute", left: { xs: 0, sm: "auto" }, right: { sm: 0 }, top: "100%", zIndex: 50, mt: 0.5, width: 224, borderRadius: 3, border: 1, borderColor: "divider", bgcolor: "background.paper", p: 0.5, boxShadow: 6 }}>
                <Box component="button" type="button"
                  onClick={() => {
                    openCreateModal();
                    setShowAddMenu(false);
                  }}
                  sx={menuButtonSx}
                >
                  <MsIcon sx={{ fontSize: 20, color: "primary.main" }}>edit_note</MsIcon>
                  Manual Entry
                </Box>
                <Box component="button" type="button"
                  onClick={() => {
                    openPublicImportModal();
                    setShowAddMenu(false);
                  }}
                  sx={menuButtonSx}
                >
                  <MsIcon sx={{ fontSize: 20, color: "success.main" }}>public</MsIcon>
                  Import from Public Lists
                </Box>
              </Box>
            )}
          </Box>
        </Box>
      </Box>

      <Card>
        <Box sx={{ mb: 2, display: "flex", flexWrap: "wrap", alignItems: "center", gap: 1 }}>
          {proxyPools.length > 0 && (
            <Box component="label" sx={{ display: "flex", alignItems: "center", gap: 0.75, fontSize: 12, color: "text.secondary", cursor: "pointer" }}>
              <Box component="input" type="checkbox" checked={allSelected} onChange={toggleSelectAll} sx={{ width: 16, height: 16, accentColor: "primary.main" }} />
              {allSelected ? "Unselect all" : "Select all"}
            </Box>
          )}
          <Badge variant="default">Total: {proxyPools.length}</Badge>
          <Badge variant="success">Active: {activeCount}</Badge>
        </Box>

        {(selectedIds.length > 0 || healthChecking || batchProgress.total > 0) && (
          <Box sx={{ mb: 2, display: "flex", flexWrap: "wrap", alignItems: "center", gap: 1, borderRadius: 2, border: 1, borderColor: (theme) => alpha(theme.palette.primary.main, 0.3), bgcolor: (theme) => alpha(theme.palette.primary.main, 0.05), px: 1.5, py: 1 }}>
            <MsIcon sx={{ fontSize: 18, color: "primary.main" }}>checklist</MsIcon>
            <Typography sx={{ fontSize: 12, fontWeight: 500, color: "primary.main" }}>
              {batchProgress.total > 0
                ? `${batchProgress.label} ${batchProgress.current}/${batchProgress.total}`
                : selectedIds.length > 0 ? `${selectedIds.length} selected` : "All pools"}
            </Typography>
            {batchProgress.total > 0 && (
              <Box sx={{ width: "100%", flexBasis: "100%" }} role="progressbar" aria-valuemin="0" aria-valuemax={batchProgress.total} aria-valuenow={batchProgress.current} aria-label={`${batchProgress.label} progress`}>
                <Box sx={{ height: 6, overflow: "hidden", borderRadius: 999, bgcolor: "action.selected" }}>
                  <Box sx={{ width: `${(batchProgress.current / batchProgress.total) * 100}%`, height: "100%", borderRadius: 999, bgcolor: "primary.main", transition: "width 150ms ease" }} />
                </Box>
              </Box>
            )}
            <Box sx={{ ml: "auto", display: "flex", flexWrap: "wrap", alignItems: "center", gap: 1 }}>
              <Button
                size="sm"
                icon={healthChecking ? "progress_activity" : "health_and_safety"}
                onClick={handleHealthCheck}
                disabled={healthChecking || bulkBusy || proxyPools.length === 0}
              >
                {healthChecking ? `Checking ${healthProgress.current}/${healthProgress.total}` : "Health Check"}
              </Button>
              {selectedIds.length > 0 && (
                <>
                  <Button size="sm" variant="secondary" icon="toggle_on" onClick={() => bulkSetActive(true)} disabled={bulkBusy || healthChecking}>
                    Activate
                  </Button>
                  <Button size="sm" variant="secondary" icon="toggle_off" onClick={() => bulkSetActive(false)} disabled={bulkBusy || healthChecking}>
                    Deactivate
                  </Button>
                  <Button size="sm" variant="secondary" icon="delete" onClick={bulkDelete} disabled={bulkBusy || healthChecking}>
                    Delete
                  </Button>
                  <Button size="sm" variant="ghost" onClick={clearSelection} disabled={bulkBusy || healthChecking}>
                    Clear
                  </Button>
                </>
              )}
            </Box>
          </Box>
        )}

        {proxyPools.length === 0 ? (
          <Box sx={{ textAlign: "center", py: 5 }}>
            <Typography sx={{ color: "text.primary", fontWeight: 500, mb: 0.5 }}>No proxy pool entries yet</Typography>
            <Typography sx={{ fontSize: 14, color: "text.secondary", mb: 2 }}>Create a proxy pool entry, then assign it to connections.</Typography>
            <Button icon="add" onClick={openCreateModal}>Add Proxy Pool</Button>
          </Box>
        ) : (
          <Box sx={{ display: "flex", flexDirection: "column", "& > * + *": { borderTop: "1px solid", borderColor: "divider" } }}>
            {proxyPools.map((pool) => (
              <PoolRow
                key={pool.id}
                pool={pool}
                selected={selectedIds.includes(pool.id)}
                testing={testingId === pool.id}
                onSelect={() => toggleSelect(pool.id)}
                onToggle={() => handleToggleActive(pool)}
                onTest={() => handleTest(pool.id)}
                onEdit={() => openEditModal(pool)}
                onDelete={() => handleDelete(pool)}
              />
            ))}
          </Box>
        )}
      </Card>

      <BatchImportModal
        isOpen={showBatchImportModal}
        text={batchImportText}
        importing={importing}
        onChange={setBatchImportText}
        onImport={handleBatchImport}
        onClose={closeBatchImportModal}
      />

      <PublicImportModal
        isOpen={showPublicImportModal}
        form={publicImportForm}
        importing={publicImporting}
        onChange={handlePublicImportField}
        onImport={handlePublicImport}
        onClose={closePublicImportModal}
      />

      <DeploymentModal isOpen={showVercelModal} title="Deploy Vercel Relay" onClose={closeVercelModal}>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Box sx={{ borderRadius: 2, bgcolor: (theme) => alpha(theme.palette.info.main, 0.05), border: 1, borderColor: (theme) => alpha(theme.palette.info.main, 0.1), p: 1.5, display: "flex", flexDirection: "column", gap: 0.75 }}>
            <Typography sx={{ fontSize: 14, color: "text.primary", fontWeight: 500 }}>What is Vercel Relay?</Typography>
            <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
              Deploys an edge relay function to Vercel. All AI provider requests will be forwarded through Vercel&apos;s edge network, masking your real IP from providers.
            </Typography>
            <Box component="ul" sx={{ m: 0, pl: 2, fontSize: 12, color: "text.secondary", "& li + li": { mt: 0.5 } }}>
              <li>Your IP is replaced by Vercel&apos;s dynamic edge IPs (hundreds of IPs across 20+ global regions)</li>
              <li>Vercel serves millions of apps — providers can&apos;t block Vercel IPs without affecting legitimate traffic</li>
              <li>Free tier: 100GB bandwidth/month, 500K edge invocations</li>
              <li>Deploy multiple relays on different accounts for more IP diversity</li>
            </Box>
          </Box>
          <Input
            label="Vercel API Token"
            value={vercelForm.vercelToken}
            onChange={(e) => setVercelForm((prev) => ({ ...prev, vercelToken: e.target.value }))}
            placeholder="your-vercel-api-token"
            hint={VERCEL_TOKEN_HINT}
            type="password"
          />
          <Input
            label="Project Name"
            value={vercelForm.projectName}
            onChange={(e) => setVercelForm((prev) => ({ ...prev, projectName: e.target.value }))}
            placeholder="my-relay"
            hint="Unique name for your Vercel project. Leave empty for auto-generated name."
          />
          <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)" }, gap: 1 }}>
            <Button
              fullWidth
              onClick={handleVercelDeploy}
              disabled={!vercelForm.vercelToken.trim() || deploying}
            >
              {deploying ? "Deploying... (may take ~1 min)" : "Deploy"}
            </Button>
            <Button fullWidth variant="ghost" onClick={closeVercelModal} disabled={deploying}>
              Cancel
            </Button>
          </Box>
        </Box>
      </DeploymentModal>

      <DeploymentModal isOpen={showCloudflareModal} title="Deploy Cloudflare Relay" onClose={closeCloudflareModal}>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Box sx={{ borderRadius: 2, bgcolor: (theme) => alpha(theme.palette.warning.main, 0.05), border: 1, borderColor: (theme) => alpha(theme.palette.warning.main, 0.1), p: 1.5, display: "flex", flexDirection: "column", gap: 0.75 }}>
            <Typography sx={{ fontSize: 14, color: "text.primary", fontWeight: 500 }}>What is Cloudflare Relay?</Typography>
            <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
              Deploys a Cloudflare Worker as a proxy relay. All AI provider requests will be forwarded through Cloudflare&apos;s global edge network.
            </Typography>
            <Box component="ul" sx={{ m: 0, pl: 2, fontSize: 12, color: "text.secondary", "& li + li": { mt: 0.5 } }}>
              <li>High performance global routing and IP masking via Cloudflare Workers</li>
              <li>Free tier: 100,000 requests per day</li>
              <li>Requires Cloudflare Account ID and a Workers API Token (Edit Workers permission)</li>
            </Box>
            <Box sx={{ mt: 1, pt: 1, borderTop: 1, borderColor: (theme) => alpha(theme.palette.warning.main, 0.1), fontSize: 12, color: "text.secondary" }}>
              <Typography sx={{ fontSize: 12, fontWeight: 500, color: "text.primary", mb: 0.5 }}>How to generate your API Token:</Typography>
              <Box component="ol" sx={{ m: 0, pl: 2, "& li + li": { mt: 0.5 } }}>
                <li>Go to <b>My Profile</b> → <b>API Tokens</b> → <b>Create Token</b></li>
                <li>Scroll down to <b>Custom Token</b> and click <b>Get started</b></li>
                <li>Under <b>Permissions</b>: Account | Workers Scripts | Edit</li>
                <li>Under <b>Account Resources</b>: Include | Account | <i>Your Account Name</i></li>
                <li>Click <b>Continue to summary</b> → <b>Create Token</b></li>
              </Box>
            </Box>
          </Box>
          <Input
            label="Account ID"
            value={cloudflareForm.accountId}
            onChange={(e) => setCloudflareForm((prev) => ({ ...prev, accountId: e.target.value }))}
            placeholder="your-cloudflare-account-id"
            hint="Found on the right side of the Cloudflare dashboard overview page."
          />
          <Input
            label="API Token"
            value={cloudflareForm.apiToken}
            onChange={(e) => setCloudflareForm((prev) => ({ ...prev, apiToken: e.target.value }))}
            placeholder="your-cloudflare-api-token"
            hint={CF_TOKEN_HINT}
            type="password"
          />
          <Input
            label="Worker Name"
            value={cloudflareForm.projectName}
            onChange={(e) => setCloudflareForm((prev) => ({ ...prev, projectName: e.target.value }))}
            placeholder="my-relay"
            hint="Unique name for your Cloudflare Worker. Leave empty for auto-generated name."
          />
          <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)" }, gap: 1 }}>
            <Button
              fullWidth
              onClick={handleCloudflareDeploy}
              disabled={!cloudflareForm.accountId.trim() || !cloudflareForm.apiToken.trim() || deploying}
            >
              {deploying ? "Deploying..." : "Deploy Worker"}
            </Button>
            <Button fullWidth variant="ghost" onClick={closeCloudflareModal} disabled={deploying}>
              Cancel
            </Button>
          </Box>
        </Box>
      </DeploymentModal>

      <DeploymentModal isOpen={showDenoModal} title="Deploy Deno Relay" onClose={closeDenoModal}>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Box sx={{ borderRadius: 2, bgcolor: "action.hover", border: 1, borderColor: "divider", p: 1.5, display: "flex", flexDirection: "column", gap: 0.75 }}>
            <Typography sx={{ fontSize: 14, color: "text.primary", fontWeight: 500 }}>What is Deno Relay?</Typography>
            <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
              Deploys a relay worker to Deno Deploy&apos;s global edge network. All AI provider requests are forwarded through Deno&apos;s edge, masking your real IP.
            </Typography>
            <Box component="ul" sx={{ m: 0, pl: 2, fontSize: 12, color: "text.secondary", "& li + li": { mt: 0.5 } }}>
              <li>Deno Deploy v2 runs on a high-performance global edge network</li>
              <li>Free tier: 1M requests & 100GiB outbound traffic per month</li>
              <li>No per-request CPU time limits (unlike Vercel/Cloudflare)</li>
              <li>Support up to 20 active apps & 50 custom domains</li>
              <li>Deploy multiple relays for maximum IP diversity</li>
            </Box>
            <Box sx={{ mt: 1, pt: 1, borderTop: 1, borderColor: "divider", fontSize: 12, color: "text.secondary" }}>
              <Typography sx={{ fontSize: 12, fontWeight: 500, color: "text.primary", mb: 0.5 }}>How to generate API token:</Typography>
              <Box component="ol" sx={{ m: 0, pl: 2, "& li + li": { mt: 0.5 } }}>
                <li>Go to <b>console.deno.com</b></li>
                <li>Select your <b>Organization</b> → <b>Settings</b> → <b>Organization Tokens</b></li>
                <li>Create a <b>Organization Token</b> (prefix <b>ddo_</b>)</li>
              </Box>
            </Box>
          </Box>
          <Input
            label="Deno Deploy API Token"
            value={denoForm.denoToken}
            onChange={(e) => setDenoForm((prev) => ({ ...prev, denoToken: e.target.value }))}
            placeholder="ddo_xxxxxxxxxxxxxxxx"
            hint="Token is used once for deployment, not stored. Found in Organization Settings."
            type="password"
          />
          <Input
            label="Organization Domain"
            value={denoForm.orgDomain}
            onChange={(e) => setDenoForm((prev) => ({ ...prev, orgDomain: e.target.value }))}
            placeholder="your-org.deno.net"
            hint="Organization's default domain. Your relay URL will be in the format: https://my-relay.your-org.deno.net"
          />
          <Input
            label="App Name"
            value={denoForm.projectName}
            onChange={(e) => setDenoForm((prev) => ({ ...prev, projectName: e.target.value }))}
            placeholder="deno-relay"
            hint="Unique app name. Leave empty for auto-generated name."
          />
          <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)" }, gap: 1 }}>
            <Button
              fullWidth
              onClick={handleDenoDeploy}
              disabled={!denoForm.denoToken.trim() || !denoForm.orgDomain.trim() || deploying}
            >
              {deploying ? "Deploying..." : "Deploy Relay"}
            </Button>
            <Button fullWidth variant="ghost" onClick={closeDenoModal} disabled={deploying}>
              Cancel
            </Button>
          </Box>
        </Box>
      </DeploymentModal>

      <PoolFormModal
        isOpen={showFormModal}
        editingProxyPool={editingProxyPool}
        formData={formData}
        saving={saving}
        onClose={closeFormModal}
        onSave={handleSave}
        onChange={(field, value) => setFormData((prev) => ({ ...prev, [field]: value }))}
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
    </Box>
  );
}
