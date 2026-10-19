"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { Card, Button, Input, Modal, CardSkeleton, Toggle, ConfirmModal } from "@/shared/components";
import { useCopyToClipboard } from "@/shared/hooks/useCopyToClipboard";
import { getCurrentLocale, onLocaleChange } from "@/i18n/runtime";
import {
  WENYAN_LOCALES,
  CAVEMAN_LEVELS,
  PONYTAIL_LEVELS,
} from "./endpointConstants";
import EndpointRow from "./components/EndpointRow";
import StatusAlert from "./components/StatusAlert";
import Tooltip from "./components/Tooltip";
import { buildProviderList } from "@/shared/utils/aclProviderList";
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
  const [requireLogin, setRequireLogin] = useState(true);
  const [hasPassword, setHasPassword] = useState(true);
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

  // API key visibility toggle state
  const [visibleKeys, setVisibleKeys] = useState(new Set());

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/immutability --
     Effects intentionally sync locale/bootstrap/external state. */

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

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- initial bootstrap fetch; the function declarations below are intentionally hoisted below the JSX (no derived state pattern fits a one-shot init).
    fetchData();
    // eslint-disable-next-line react-hooks/set-state-in-effect -- initial bootstrap fetch.
    loadSettings();
  }, []);

  const loadSettings = async () => {
    try {
      const settingsRes = await fetch("/api/settings");
      if (!settingsRes.ok) return;
      const data = await settingsRes.json();
      setRequireApiKey(data.requireApiKey || false);
      setRequireLogin(data.requireLogin !== false);
      setHasPassword(data.hasPassword || false);
      setRtkEnabledState(data.rtkEnabled !== false);
      setHeadroomEnabled(!!data.headroomEnabled);
      setHeadroomUrl(data.headroomUrl || "http://localhost:8787");
      setHeadroomCompressUserMessages(!!data.headroomCompressUserMessages);
      refreshHeadroomStatus();
      setCavemanEnabled(!!data.cavemanEnabled);
      setCavemanLevel(data.cavemanLevel || "full");
      setPonytailEnabled(!!data.ponytailEnabled);
      setPonytailLevel(data.ponytailLevel || "full");
    } catch (error) {
      console.log("Error loading settings:", error);
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
      <div className="flex flex-col gap-8">
        <CardSkeleton />
        <CardSkeleton />
      </div>
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
    <div className="flex flex-col gap-8">
      {/* Endpoint Card */}
      <Card>
        <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
          <span className="material-symbols-outlined text-primary">api</span>
          API Endpoint
        </h2>

        {/* Endpoint rows */}
        <div className="flex flex-col gap-2">
          {/* Local */}
          <EndpointRow
            label="Local"
            url={currentEndpoint}
            copyId="local_url"
            copied={copied}
            onCopy={copy}
          />
        </div>

      </Card>

      {/* API Keys */}
      <Card id="require-api-key">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold flex items-center gap-2">
            <span className="material-symbols-outlined text-primary">vpn_key</span>
            API Keys
          </h2>
          <Button icon="add" onClick={() => setShowAddModal(true)}>
            Create Key
          </Button>
        </div>

        <div className="flex items-center justify-between pb-4 mb-4 border-b border-border">
          <div>
            <p className="font-medium">Require API key</p>
            <p className="text-sm text-text-muted">
              Requests without a valid key will be rejected
            </p>
          </div>
          <Toggle
            checked={requireApiKey}
            onChange={() => handleRequireApiKey(!requireApiKey)}
          />
        </div>

        {keys.length === 0 ? (
          <div className="text-center py-12">
            <div className="inline-flex items-center justify-center w-16 h-16 rounded-full bg-primary/10 text-primary mb-4">
              <span className="material-symbols-outlined text-[32px]">vpn_key</span>
            </div>
            <p className="text-text-main font-medium mb-1">No API keys yet</p>
            <p className="text-sm text-text-muted mb-4">Create your first API key to get started</p>
            <Button icon="add" onClick={() => setShowAddModal(true)}>
              Create Key
            </Button>
          </div>
        ) : (
          <div className="flex flex-col">
            {keys.map((key) => (
              <div
                key={key.id}
                className={`group flex items-center justify-between py-3 border-b border-black/[0.03] dark:border-white/[0.03] last:border-b-0 ${key.isActive === false ? "opacity-60" : ""}`}
              >
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium">{key.name}</p>
                  <div className="flex items-center gap-2 mt-1">
                    <code className="text-xs text-text-muted font-mono">
                      {visibleKeys.has(key.id) ? key.key : maskKey(key.key)}
                    </code>
                    <button
                      onClick={() => toggleKeyVisibility(key.id)}
                      className="p-1 hover:bg-black/5 dark:hover:bg-white/5 rounded text-text-muted hover:text-primary opacity-100 sm:opacity-0 sm:group-hover:opacity-100 transition-all"
                      title={visibleKeys.has(key.id) ? "Hide key" : "Show key"}
                    >
                      <span className="material-symbols-outlined text-[14px]">
                        {visibleKeys.has(key.id) ? "visibility_off" : "visibility"}
                      </span>
                    </button>
                    <button
                      onClick={() => copy(key.key, key.id)}
                      className="p-1 hover:bg-black/5 dark:hover:bg-white/5 rounded text-text-muted hover:text-primary opacity-100 sm:opacity-0 sm:group-hover:opacity-100 transition-all"
                    >
                      <span className="material-symbols-outlined text-[14px]">
                        {copied === key.id ? "check" : "content_copy"}
                      </span>
                    </button>
                  </div>
                  <p className="text-xs text-text-muted mt-1">
                    Created {new Date(key.createdAt).toLocaleDateString()}
                  </p>
                   {key.isActive === false && (
                    <p className="text-xs text-orange-500 mt-1">Paused</p>
                  )}
                   {/* ACL badges */}
                  {(key.allowedProviders || key.allowedCombos || key.allowedKinds) && (
                    <div className="flex flex-wrap gap-1 mt-1">
                      {key.allowedProviders && (
                        <span className="text-[10px] px-1.5 py-0.5 rounded bg-blue-500/10 text-blue-500 dark:bg-blue-500/20" title={key.allowedProviders.join(", ")}>
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
                        </span>
                      )}
                      {key.allowedCombos && (
                        <span className="text-[10px] px-1.5 py-0.5 rounded bg-purple-500/10 text-purple-500 dark:bg-purple-500/20">
                          {key.allowedCombos.length === 0 ? "No combos" : key.allowedCombos.join(", ")}
                        </span>
                      )}
                      {key.allowedKinds && (
                        <span className="text-[10px] px-1.5 py-0.5 rounded bg-green-500/10 text-green-500 dark:bg-green-500/20">
                          {key.allowedKinds.length === 0 ? "No kinds" : key.allowedKinds.join(", ")}
                        </span>
                      )}
                    </div>
                  )}
                </div>
                <div className="flex items-center gap-2">
                  {/* Edit ACL button */}
                  <button
                    onClick={() => handleOpenEditKey(key)}
                    className="p-2 hover:bg-primary/10 rounded text-primary opacity-100 sm:opacity-0 sm:group-hover:opacity-100 transition-all"
                    title="Edit access control"
                  >
                    <span className="material-symbols-outlined text-[18px]">tune</span>
                  </button>
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
                  <button
                    onClick={() => handleDeleteKey(key.id)}
                    className="p-2 hover:bg-red-500/10 rounded text-red-500 opacity-100 sm:opacity-0 sm:group-hover:opacity-100 transition-all"
                  >
                    <span className="material-symbols-outlined text-[18px]">delete</span>
                  </button>
                </div>
              </div>
            ))}
          </div>
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
        <div className="flex flex-col gap-4">
          <Input
            label="Key Name"
            value={newKeyName}
            onChange={(e) => setNewKeyName(e.target.value)}
            placeholder="Production Key"
          />
          <div className="flex gap-2">
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
          </div>
        </div>
      </Modal>

      {/* Created Key Modal */}
      <Modal
        isOpen={!!createdKey}
        title="API Key Created"
        onClose={() => setCreatedKey(null)}
      >
        <div className="flex flex-col gap-4">
          <div className="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4">
            <p className="text-sm text-yellow-800 dark:text-yellow-200 mb-2 font-medium">
              Save this key now!
            </p>
            <p className="text-sm text-yellow-700 dark:text-yellow-300">
              This is the only time you will see this key. Store it securely.
            </p>
          </div>
          <div className="flex gap-2">
            <Input
              value={createdKey || ""}
              readOnly
              className="flex-1 font-mono text-sm"
            />
            <Button
              variant="secondary"
              icon={copied === "created_key" ? "check" : "content_copy"}
              onClick={() => copy(createdKey, "created_key")}
            >
              {copied === "created_key" ? "Copied!" : "Copy"}
            </Button>
          </div>
          <Button onClick={() => setCreatedKey(null)} fullWidth>
            Done
          </Button>
        </div>
      </Modal>

      {/* Edit ACL Modal */}
      <Modal
        isOpen={!!editingKey}
        title={`Edit Access: ${editingKey?.name || ""}`}
        onClose={() => !editSaving && setEditingKey(null)}
      >
        <div className="flex flex-col gap-5">
          {/* Name */}
          <div>
            <label className="block text-sm font-medium mb-1">Key Name</label>
            <Input value={editName} onChange={(e) => setEditName(e.target.value)} placeholder="Key name" />
          </div>

          {/* Service Kinds */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <label className="text-sm font-medium">Service Kinds</label>
              <label className="flex items-center gap-1.5 text-xs cursor-pointer">
                <input type="checkbox" checked={editKindsAll} onChange={(e) => setEditKindsAll(e.target.checked)} />
                <span className="text-text-muted">All allowed</span>
              </label>
            </div>
            {!editKindsAll && (
              <div className="flex flex-wrap gap-2">
                {ALL_KINDS.map((kind) => (
                  <label key={kind} className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg border text-xs cursor-pointer transition-colors ${editKinds.includes(kind) ? "border-primary bg-primary/10 text-primary" : "border-border-subtle text-text-muted hover:border-primary/40"}`}>
                    <input type="checkbox" className="hidden" checked={editKinds.includes(kind)} onChange={() => toggleEditKind(kind)} />
                    {kind}
                  </label>
                ))}
              </div>
            )}
            {editKindsAll && <p className="text-xs text-text-muted">This key can access all service kinds.</p>}
          </div>

          {/* Providers */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <label className="text-sm font-medium">Providers</label>
              <label className="flex items-center gap-1.5 text-xs cursor-pointer">
                <input type="checkbox" checked={editProvidersAll} onChange={(e) => setEditProvidersAll(e.target.checked)} />
                <span className="text-text-muted">All allowed</span>
              </label>
            </div>
            {!editProvidersAll && (
              <>
                <div className="max-h-60 overflow-y-auto border border-border-subtle rounded-lg p-2 space-y-1">
                  {providerList.length === 0 ? (
                    <p className="text-xs text-text-muted p-2">No providers configured.</p>
                  ) : (
                    providerList.map((p) => {
                      const checked = editProviders.includes(p.id);
                      return (
                        <label key={p.id} className={`flex items-center gap-2 px-2 py-2 rounded text-xs cursor-pointer transition-colors ${checked ? "bg-primary/10 text-primary" : "hover:bg-surface-2"}`}>
                          <input type="checkbox" checked={checked} onChange={() => toggleEditProvider(p.id)} className="rounded" />
                          <div className="flex-1 min-w-0">
                            <span className="font-medium">{p.displayName}</span>
                            {p.prefix && (
                              <span className="ml-1.5 text-[10px] px-1 py-0.5 rounded bg-surface-2 text-text-muted font-mono">{p.prefix}</span>
                            )}
                          </div>
                          <span className="text-text-muted text-[10px] shrink-0">{p.count} conn</span>
                        </label>
                      );
                    })
                  )}
                </div>
                {editProviders.length === 0 && (
                  <p className="text-xs text-warning bg-warning/10 border border-warning/20 rounded px-2 py-1 mt-1.5 flex items-center gap-1.5">
                    <span className="material-symbols-outlined text-[14px]">warning</span>
                    <span>Warning: Zero providers selected. This key will have NO access to any providers (Deny All).</span>
                  </p>
                )}
              </>
            )}
            {editProvidersAll && <p className="text-xs text-text-muted">This key can access all providers ({providerList.length}).</p>}
          </div>

          {/* Combos */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <label className="text-sm font-medium">Combos</label>
              <label className="flex items-center gap-1.5 text-xs cursor-pointer">
                <input type="checkbox" checked={editCombosAll} onChange={(e) => setEditCombosAll(e.target.checked)} />
                <span className="text-text-muted">All allowed</span>
              </label>
            </div>
            {!editCombosAll && (
              <div className="max-h-40 overflow-y-auto border border-border-subtle rounded-lg p-2 space-y-1">
                {comboList.length === 0 ? (
                  <p className="text-xs text-text-muted p-2">No combos configured.</p>
                ) : (
                  comboList.map((combo) => {
                    const name = combo.name || combo.id;
                    const checked = editCombos.includes(name);
                    return (
                      <label key={name} className={`flex items-center gap-2 px-2 py-1.5 rounded text-xs cursor-pointer transition-colors ${checked ? "bg-primary/10 text-primary" : "hover:bg-surface-2"}`}>
                        <input type="checkbox" checked={checked} onChange={() => toggleEditCombo(name)} className="rounded" />
                        <span>{name}</span>
                        <span className="text-text-muted ml-auto text-[10px]">{Array.isArray(combo.models) ? combo.models.length : 0} models</span>
                      </label>
                    );
                  })
                )}
              </div>
            )}
            {editCombosAll && <p className="text-xs text-text-muted">This key can access all combos.</p>}
          </div>

          {/* ACL info */}
          <div className="text-xs text-text-muted bg-surface-2 rounded-lg p-3 border border-border-subtle">
            <p className="font-medium mb-1">How it works:</p>
            <ul className="list-disc pl-4 space-y-0.5">
              <li><strong>All allowed</strong> = unrestricted (default). No ACL filtering.</li>
              <li><strong>Unchecked + empty</strong> = deny everything of that type.</li>
              <li><strong>Checked items</strong> = only those items are accessible.</li>
            </ul>
          </div>

          {/* Actions */}
          <div className="flex gap-3">
            <Button onClick={() => setEditingKey(null)} variant="ghost" fullWidth disabled={editSaving}>
              Cancel
            </Button>
            <Button onClick={handleSaveEditKey} fullWidth disabled={editSaving}>
              {editSaving ? "Saving..." : "Save ACL"}
            </Button>
          </div>
        </div>
      </Modal>

      {/* Headroom Install Guide Modal */}
      <Modal
        isOpen={showHeadroomInstallModal}
        title={headroomRunning ? "Headroom" : "Setup Headroom"}
        onClose={() => setShowHeadroomInstallModal(false)}
      >
        <div className="flex flex-col gap-4">
          <div className="flex items-center justify-between text-sm">
            <span>Status</span>
            <span className={headroomRunning ? "text-success" : "text-warning"}>
              {headroomStatusLabel}
            </span>
          </div>
          <div className="flex flex-col gap-1">
            <p className="text-sm font-medium">Proxy URL</p>
            <Input
              value={headroomUrl}
              onChange={(e) => setHeadroomUrl(e.target.value)}
              onBlur={handleHeadroomUrlBlur}
              placeholder="http://localhost:8787"
              className="font-mono text-sm"
            />
            <p className="text-xs text-text-muted">
              Use a local proxy for Start/Stop, or an external Docker sidecar like http://headroom:8787.
            </p>
          </div>
          {headroomManaged ? (
            <Button onClick={handleHeadroomStop} variant="ghost" fullWidth disabled={headroomActionLoading}>
              {headroomActionLoading ? "Stopping…" : "Stop Headroom"}
            </Button>
          ) : headroomRunning ? (
            <p className="text-sm text-success">Headroom proxy is reachable. You can enable the token saver.</p>
          ) : headroomCanStart ? (
            <Button onClick={handleHeadroomStart} fullWidth disabled={headroomActionLoading}>
              {headroomActionLoading ? "Starting…" : "Start Headroom"}
            </Button>
          ) : !headroomLocalUrl ? (
            <p className="text-sm text-warning">Start Headroom separately at the configured URL, then recheck.</p>
          ) : !headroomStatus.python ? (
            <p className="text-sm text-warning">Python ≥ 3.10 required for local managed mode. Install Python first, or use an external proxy URL.</p>
          ) : (
            <div className="flex flex-col gap-1">
              <p className="text-sm font-medium">Install then click Start:</p>
              <div className="flex items-center gap-2">
                <pre className="flex-1 rounded bg-black/5 dark:bg-white/5 p-2 text-xs font-mono overflow-x-auto">{`pip install "headroom-ai[proxy]"`}</pre>
                <Button size="sm" variant="ghost" onClick={() => copy(`pip install "headroom-ai[proxy]"`)}>
                  {copied ? "Copied" : "Copy"}
                </Button>
              </div>
            </div>
          )}
          {headroomActionError && (
            <p className="text-sm text-warning">{headroomActionError}</p>
          )}
          <div className="flex gap-2">
            <Button onClick={() => refreshHeadroomStatus()} variant="ghost" fullWidth>Recheck</Button>
            <Button onClick={() => setShowHeadroomInstallModal(false)} fullWidth>Done</Button>
          </div>
        </div>
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
    </div>
  );
}


