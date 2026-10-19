"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import Image from "next/image";
import { Box, Checkbox, Typography } from "@mui/material";
import { getProviderIconSrc, markProviderIconMissing } from "@/shared/utils/providerIcon";
import ProviderIcon from "@/shared/components/ProviderIcon";
import ModelImportModal from "@/shared/components/ModelImportModal.js";
import { importSelectedModels } from "@/shared/utils/importSelectedModels.js";
import { deleteSelectedModels } from "@/shared/utils/deleteSelectedModels.js";
import { Card, Button, Badge, Input, Modal, CardSkeleton, OAuthModal, KiroOAuthWrapper, CursorAuthModal, ZedAuthModal, IFlowCookieModal, GitLabAuthModal, Toggle, Select, EditConnectionModal, NoAuthProxyCard, ConfirmModal, Pagination } from "@/shared/components";
import { CONNECTIONS_PER_PAGE, computeConnectionPagination } from "./connectionsPagination";
import { OAUTH_PROVIDERS, APIKEY_PROVIDERS, FREE_PROVIDERS, FREE_TIER_PROVIDERS, WEB_COOKIE_PROVIDERS, getProviderAlias, isOpenAICompatibleProvider, isAnthropicCompatibleProvider, AI_PROVIDERS } from "@/shared/constants/providers";
import { getModelsByProviderId, getModelKind } from "@/shared/constants/models";
import { getThinkingLevels } from "open-sse/providers/thinkingLevels.js";
import { useCopyToClipboard } from "@/shared/hooks/useCopyToClipboard";
import { useModelCaps } from "@/shared/hooks/useModelCaps";
import { translate } from "@/i18n/runtime";
import { fetchSuggestedModels } from "@/shared/utils/providerModelsFetcher";
import { getProviderCustomModelRows } from "@/shared/utils/providerCustomModels";
import ModelRow from "./ModelRow";
import PassthroughModelsSection from "./PassthroughModelsSection";
import CompatibleModelsSection from "./CompatibleModelsSection";
import ConnectionRow from "./ConnectionRow";
import AddApiKeyModal from "./AddApiKeyModal";
import EditCompatibleNodeModal from "./EditCompatibleNodeModal";
import AddCustomModelModal from "./AddCustomModelModal";
import BulkImportCodexModal from "./BulkImportCodexModal";
import BulkImportGrokCliModal from "./BulkImportGrokCliModal";
import CustomConfigCard from "./CustomConfigCard";

const ONE_BY_ONE_DELAY_MS = 1000;

const AUTO_PING_SETTINGS_KEYS = {
  claude: "claudeAutoPing",
  codex: "codexAutoPing",
};

const dividerFaint = (theme) =>
  theme.palette.mode === "dark" ? "rgba(255, 255, 255, 0.03)" : "rgba(0, 0, 0, 0.03)";
const dividerSoft = (theme) =>
  theme.palette.mode === "dark" ? "rgba(255, 255, 255, 0.1)" : "rgba(0, 0, 0, 0.1)";
const panelSubtle = (theme) =>
  theme.palette.mode === "dark" ? "rgba(255, 255, 255, 0.03)" : "rgba(0, 0, 0, 0.02)";
const hoverSubtle = (theme) =>
  theme.palette.mode === "dark" ? "rgba(255, 255, 255, 0.05)" : "rgba(0, 0, 0, 0.05)";

const proxyOptionSx = {
  display: "flex",
  alignItems: "center",
  gap: 1.5,
  borderRadius: 2,
  px: 1.5,
  py: 1,
  textAlign: "left",
  border: 0,
  bgcolor: "transparent",
  cursor: "pointer",
  transition: "background-color 150ms ease",
  "&:hover": { bgcolor: "action.hover" },
  "&:disabled": { cursor: "not-allowed", opacity: 0.5 },
};

const modelChipSx = {
  display: "flex",
  alignItems: "center",
  gap: 0.5,
  borderRadius: 2,
  border: 1,
  borderColor: dividerSoft,
  px: 1.25,
  py: 0.75,
  fontSize: 12,
  bgcolor: "transparent",
  color: "text.secondary",
  cursor: "pointer",
  transition: "color 150ms ease, border-color 150ms ease, background-color 150ms ease",
  "&:hover": {
    color: "primary.main",
    borderColor: "rgba(229, 106, 74, 0.4)",
    bgcolor: "rgba(229, 106, 74, 0.05)",
  },
};

const modelChipDashedSx = { ...modelChipSx, borderStyle: "dashed" };

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export default function ProviderDetailPage() {
  const params = useParams();
  const router = useRouter();
  const providerId = params.id;
  const { getCaps } = useModelCaps();
  const [connections, setConnections] = useState([]);
  const [loading, setLoading] = useState(true);
  const [providerNode, setProviderNode] = useState(null);
  const [proxyPools, setProxyPools] = useState([]);
  const [showOAuthModal, setShowOAuthModal] = useState(false);
  const [showIFlowCookieModal, setShowIFlowCookieModal] = useState(false);
  const [showAddApiKeyModal, setShowAddApiKeyModal] = useState(false);
  const [addConnectionError, setAddConnectionError] = useState("");
  const [showBulkImportCodex, setShowBulkImportCodex] = useState(false);
  const [showBulkImportGrokCli, setShowBulkImportGrokCli] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [showEditNodeModal, setShowEditNodeModal] = useState(false);
  const [showBulkProxyModal, setShowBulkProxyModal] = useState(false);
  const [selectedConnection, setSelectedConnection] = useState(null);
  const [modelAliases, setModelAliases] = useState({});
  const [customModels, setCustomModels] = useState([]);
  const [headerImgError, setHeaderImgError] = useState(false);
  const [modelTestResults, setModelTestResults] = useState({});
  const [modelsTestError, setModelsTestError] = useState("");
  const [testingModelIds, setTestingModelIds] = useState(() => new Set());
  const [showAddCustomModel, setShowAddCustomModel] = useState(false);
  const [selectedConnectionIds, setSelectedConnectionIds] = useState([]);
  const [connectionPage, setConnectionPage] = useState(1);
  const [bulkProxyPoolId, setBulkProxyPoolId] = useState("__none__");
  const [bulkUpdatingProxy, setBulkUpdatingProxy] = useState(false);
  const [providerStrategy, setProviderStrategy] = useState(null);
  const [providerStickyLimit, setProviderStickyLimit] = useState("");
  const [strictModelAssignment, setStrictModelAssignment] = useState(false);
  const [thinkingMode, setThinkingMode] = useState("auto");
  const [autoPing, setAutoPing] = useState({ enabled: false, connections: {} });
  const [suggestedModels, setSuggestedModels] = useState([]);
  const [importPreview, setImportPreview] = useState(null);
  const [selectedModelIds, setSelectedModelIds] = useState([]);
  const [bulkDeleteBusy, setBulkDeleteBusy] = useState(false);
  const [showImportMenu, setShowImportMenu] = useState(false);
  const [liveModels, setLiveModels] = useState([]);
  const [kiloFreeModels, setKiloFreeModels] = useState([]);
  const [disabledModelIds, setDisabledModelIds] = useState([]);
  const [confirmState, setConfirmState] = useState(null);
  const [noticeState, setNoticeState] = useState(null);
  const [showAgRiskModal, setShowAgRiskModal] = useState(false);
  const [oneByOneRunning, setOneByOneRunning] = useState(false);
  const [oneByOneStopping, setOneByOneStopping] = useState(false);
  const [oneByOneCurrentConnectionId, setOneByOneCurrentConnectionId] = useState(null);
  const [oneByOneResults, setOneByOneResults] = useState({});
  const [oneByOneSummary, setOneByOneSummary] = useState(null);
  const stopOneByOneRef = useRef(false);
  const importMenuRef = useRef(null);
  const [importingModels, setImportingModels] = useState(false);
  const { copied, copy } = useCopyToClipboard();

  const AG_RISK_STORAGE_KEY = "ag_risk_confirmed";

  const openOAuthConnection = () => {
    setShowOAuthModal(true);
  };

  const triggerOAuthConnection = () => {
    if (providerId === "antigravity" && typeof window !== "undefined") {
      const confirmed = window.localStorage.getItem(AG_RISK_STORAGE_KEY) === "true";
      if (!confirmed) {
        setShowAgRiskModal(true);
        return;
      }
    }
    if (isOAuth) {
      openOAuthConnection();
      return;
    }
    setAddConnectionError("");
    setShowAddApiKeyModal(true);
  };

  const triggerApiKeyConnection = () => {
    setAddConnectionError("");
    setShowAddApiKeyModal(true);
  };

  const triggerAddConnection = () => {
    if (isOAuth) {
      triggerOAuthConnection();
      return;
    }
    triggerApiKeyConnection();
  };

  const handleAgRiskConfirm = () => {
    if (typeof window !== "undefined") {
      window.localStorage.setItem(AG_RISK_STORAGE_KEY, "true");
    }
    setShowAgRiskModal(false);
    if (isOAuth) {
      openOAuthConnection();
      return;
    }
    triggerApiKeyConnection();
  };

  const providerInfo = providerNode
    ? {
        id: providerNode.id,
        name: providerNode.name || (providerNode.type === "anthropic-compatible" ? "Anthropic Compatible" : "OpenAI Compatible"),
        color: providerNode.type === "anthropic-compatible" ? "#D97757" : "#10A37F",
        textIcon: providerNode.type === "anthropic-compatible" ? "AC" : "OC",
        apiType: providerNode.apiType,
        baseUrl: providerNode.baseUrl,
        type: providerNode.type,
      }
    : (OAUTH_PROVIDERS[providerId] || APIKEY_PROVIDERS[providerId] || FREE_PROVIDERS[providerId] || FREE_TIER_PROVIDERS[providerId] || WEB_COOKIE_PROVIDERS[providerId]);
  const authModes = providerInfo?.authModes || [];
  const isOAuth = !!OAUTH_PROVIDERS[providerId] || !!FREE_PROVIDERS[providerId] || authModes.includes("oauth");
  const supportsApiKeyAuth = !!APIKEY_PROVIDERS[providerId] || authModes.includes("apikey");
  const isFreeNoAuth = !!FREE_PROVIDERS[providerId]?.noAuth;
  const staticModels = getModelsByProviderId(providerId);
  const models = providerId === "cursor" && liveModels.length > 0
    ? liveModels
    : staticModels;
  const providerAlias = getProviderAlias(providerId);

  const isOpenAICompatible = isOpenAICompatibleProvider(providerId);
  const isAnthropicCompatible = isAnthropicCompatibleProvider(providerId);
  const isCompatible = isOpenAICompatible || isAnthropicCompatible;
  const hasDualAuthModes = !isCompatible && isOAuth && supportsApiKeyAuth;
  const oauthConnectionLabel =
    providerId === "xai" ? "Grok Build OAuth"
    : providerId === "grok-cli" ? "Grok CLI Device Login"
    : providerId === "kimi" ? "Kimi Coding OAuth"
    : "OAuth";
  const apiKeyConnectionLabel =
    providerId === "xai" ? "xAI API Key"
    : providerId === "kimi" ? "Kimi API Key"
    : providerId === "qoder" ? "PAT"
    : "API Key";
  // Resolve suffix "(level)" for a model when a thinking level is picked and the model supports it.
  const resolveThinkingSuffix = (modelId) => {
    if (!thinkingMode || thinkingMode === "auto") return null;
    const levels = getThinkingLevels(providerId, modelId);
    return levels && levels.includes(thinkingMode) ? thinkingMode : null;
  };
  const providerStorageAlias = isCompatible ? providerId : providerAlias;
  const assignmentModels = (() => {
    const byId = new Map();
    const add = (model) => { if (model?.id && !byId.has(model.id)) byId.set(model.id, model); };
    models.forEach(add);
    customModels.forEach((model) => {
      if (model.providerAlias === providerStorageAlias && (model.kind || model.type || "llm") === "llm") add(model);
    });
    const disabled = new Set(disabledModelIds);
    return [...byId.values()].filter((model) => !disabled.has(model.id));
  })();
  // Union of levels across this provider's reasoning models — drives the level picker options.
  // Include custom models too (e.g. manually added gpt-5.6-sol → max).
  const providerThinkingLevels = (() => {
    const set = new Set();
    const seen = new Set();
    const addLevels = (modelId) => {
      if (!modelId || seen.has(modelId)) return;
      seen.add(modelId);
      const lv = getThinkingLevels(providerId, modelId);
      if (lv) lv.forEach((l) => { if (l !== "none") set.add(l); });
    };
    for (const m of models) addLevels(m.id);
    for (const m of kiloFreeModels) addLevels(m.id);
    for (const entry of customModels) {
      if (entry.providerAlias !== providerStorageAlias) continue;
      if ((entry.kind || entry.type || "llm") !== "llm") continue;
      addLevels(entry.id);
    }
    return set.size ? ["auto", ...[...set]] : null;
  })();
  const providerDisplayAlias = isCompatible
    ? (providerNode?.prefix || providerId)
    : providerAlias;

  const fetchDisabledModels = useCallback(async () => {
    try {
      const res = await fetch(`/api/models/disabled?providerAlias=${encodeURIComponent(providerStorageAlias)}`, { cache: "no-store" });
      const data = await res.json();
      if (res.ok) setDisabledModelIds(data.ids || []);
    } catch (error) {
      console.log("Error fetching disabled models:", error);
    }
  }, [providerStorageAlias]);

  const handleDisableModel = async (modelId) => {
    try {
      const res = await fetch("/api/models/disabled", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ providerAlias: providerStorageAlias, ids: [modelId] }),
      });
      if (res.ok) await fetchDisabledModels();
    } catch (error) {
      console.log("Error disabling model:", error);
    }
  };

  const handleEnableModel = async (modelId) => {
    try {
      const res = await fetch(`/api/models/disabled?providerAlias=${encodeURIComponent(providerStorageAlias)}&id=${encodeURIComponent(modelId)}`, { method: "DELETE" });
      if (res.ok) await fetchDisabledModels();
    } catch (error) {
      console.log("Error enabling model:", error);
    }
  };

  const handleDisableAll = async (ids) => {
    if (!ids.length) return;
    setConfirmState({
      title: "Disable All Models",
      message: `Disable all ${ids.length} model(s)?`,
      onConfirm: async () => {
        setConfirmState(null);
        try {
          const res = await fetch("/api/models/disabled", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ providerAlias: providerStorageAlias, ids }),
          });
          if (res.ok) await fetchDisabledModels();
        } catch (error) {
          console.log("Error disabling all models:", error);
        }
      }
    });
  };

  const handleEnableAll = async () => {
    try {
      const res = await fetch(`/api/models/disabled?providerAlias=${encodeURIComponent(providerStorageAlias)}`, { method: "DELETE" });
      if (res.ok) await fetchDisabledModels();
    } catch (error) {
      console.log("Error enabling all models:", error);
    }
  };

  // Define callbacks BEFORE the useEffect that uses them
  const fetchAliases = useCallback(async () => {
    try {
      const res = await fetch("/api/models/alias");
      const data = await res.json();
      if (res.ok) {
        setModelAliases(data.aliases || {});
      }
    } catch (error) {
      console.log("Error fetching aliases:", error);
    }
  }, []);

  const fetchCustomModels = useCallback(async () => {
    try {
      const res = await fetch("/api/models/custom", { cache: "no-store" });
      const data = await res.json();
      if (res.ok) {
        setCustomModels(data.models || []);
      }
    } catch (error) {
      console.log("Error fetching custom models:", error);
    }
  }, []);

  // Close the import dropdown on an outside click.
  useEffect(() => {
    if (!showImportMenu) return;
    const handleClick = (e) => {
      if (importMenuRef.current && !importMenuRef.current.contains(e.target)) setShowImportMenu(false);
    };
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [showImportMenu]);

  // Fetch free models from Kilo API for kilocode provider
  useEffect(() => {
    if (providerId !== "kilocode") return;
    fetch("/api/providers/kilo/free-models")
      .then((res) => res.json())
      .then((data) => { if (data.models?.length) setKiloFreeModels(data.models); })
      .catch(() => {});
  }, [providerId]);

  const fetchConnections = useCallback(async () => {
    try {
      const [connectionsRes, nodesRes, proxyPoolsRes, settingsRes] = await Promise.all([
        fetch("/api/providers", { cache: "no-store" }),
        fetch("/api/provider-nodes", { cache: "no-store" }),
        fetch("/api/proxy-pools?isActive=true", { cache: "no-store" }),
        fetch("/api/settings", { cache: "no-store" }),
      ]);
      const connectionsData = await connectionsRes.json();
      const nodesData = await nodesRes.json();
      const proxyPoolsData = await proxyPoolsRes.json();
      const settingsData = settingsRes.ok ? await settingsRes.json() : {};
      if (connectionsRes.ok) {
        const filtered = (connectionsData.connections || []).filter(c => c.provider === providerId);
        setConnections(filtered);
      }
      if (proxyPoolsRes.ok) {
        setProxyPools(proxyPoolsData.proxyPools || []);
      }
      // Load per-provider strategy override
      const override = (settingsData.providerStrategies || {})[providerId] || {};
      setProviderStrategy(override.fallbackStrategy || null);
      setProviderStickyLimit(override.stickyRoundRobinLimit != null ? String(override.stickyRoundRobinLimit) : "1");
      setStrictModelAssignment(override.strictModelAssignment === true);
      // Load per-provider thinking config
      const thinkingCfg = (settingsData.providerThinking || {})[providerId] || {};
      setThinkingMode(thinkingCfg.mode || "auto");
      const autoPingSettingsKey = AUTO_PING_SETTINGS_KEYS[providerId];
      const apCfg = autoPingSettingsKey ? settingsData[autoPingSettingsKey] || {} : {};
      setAutoPing({ enabled: apCfg.enabled === true, connections: apCfg.connections || {} });
      if (nodesRes.ok) {
        let node = (nodesData.nodes || []).find((entry) => entry.id === providerId) || null;

        // Newly created compatible nodes can be briefly unavailable on one worker.
        // Retry a few times before showing "Provider not found".
        if (!node && isCompatible) {
          for (let attempt = 0; attempt < 3; attempt += 1) {
            await new Promise((resolve) => setTimeout(resolve, 150));
            const retryRes = await fetch("/api/provider-nodes", { cache: "no-store" });
            if (!retryRes.ok) continue;
            const retryData = await retryRes.json();
            node = (retryData.nodes || []).find((entry) => entry.id === providerId) || null;
            if (node) break;
          }
        }

        setProviderNode(node);
      }
    } catch (error) {
      console.log("Error fetching connections:", error);
    } finally {
      setLoading(false);
    }
  }, [providerId, isCompatible]);

  const handleUpdateNode = async (formData) => {
    try {
      const res = await fetch(`/api/provider-nodes/${providerId}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(formData),
      });
      const data = await res.json();
      if (res.ok) {
        setProviderNode(data.node);
        await fetchConnections();
        setShowEditNodeModal(false);
      }
    } catch (error) {
      console.log("Error updating provider node:", error);
    }
  };

  const saveProviderStrategy = async (strategy, stickyLimit) => {
    try {
      const settingsRes = await fetch("/api/settings", { cache: "no-store" });
      const settingsData = settingsRes.ok ? await settingsRes.json() : {};
      const current = settingsData.providerStrategies || {};

      // Build override: null strategy means remove override, use global
      const override = { ...(current[providerId] || {}) };
      if (strategy) override.fallbackStrategy = strategy;
      else {
        delete override.fallbackStrategy;
        delete override.stickyRoundRobinLimit;
      }
      if (strategy === "round-robin" && stickyLimit !== "") {
        override.stickyRoundRobinLimit = Number(stickyLimit) || 3;
      }

      const updated = { ...current };
      if (Object.keys(override).length === 0) {
        delete updated[providerId];
      } else {
        updated[providerId] = override;
      }

      await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ providerStrategies: updated }),
      });
    } catch (error) {
      console.log("Error saving provider strategy:", error);
    }
  };

  const handleStrictAssignmentToggle = async (enabled) => {
    setStrictModelAssignment(enabled);
    try {
      const settingsRes = await fetch("/api/settings", { cache: "no-store" });
      const settingsData = settingsRes.ok ? await settingsRes.json() : {};
      const current = settingsData.providerStrategies || {};
      await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ providerStrategies: {
          ...current,
          [providerId]: { ...(current[providerId] || {}), strictModelAssignment: enabled },
        } }),
      });
    } catch (error) {
      console.log("Error saving Freebuff strict assignment:", error);
    }
  };

  const handleModelAssignment = async (connectionId, assignedModel) => {
    try {
      const res = await fetch(`/api/providers/${connectionId}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ providerSpecificData: { assignedModel: assignedModel || null } }),
      });
      if (res.ok) {
        setConnections((prev) => prev.map((connection) => connection.id === connectionId
          ? { ...connection, providerSpecificData: { ...(connection.providerSpecificData || {}), assignedModel: assignedModel || null } }
          : connection));
      }
    } catch (error) {
      console.log("Error saving Freebuff model assignment:", error);
    }
  };

  const handleRoundRobinToggle = (enabled) => {
    const strategy = enabled ? "round-robin" : null;
    const sticky = enabled ? (providerStickyLimit || "1") : providerStickyLimit;
    if (enabled && !providerStickyLimit) setProviderStickyLimit("1");
    setProviderStrategy(strategy);
    saveProviderStrategy(strategy, sticky);
  };

  const handleStickyLimitChange = (value) => {
    setProviderStickyLimit(value);
    saveProviderStrategy("round-robin", value);
  };

  const saveThinkingConfig = async (mode) => {
    try {
      const settingsRes = await fetch("/api/settings", { cache: "no-store" });
      const settingsData = settingsRes.ok ? await settingsRes.json() : {};
      const current = settingsData.providerThinking || {};
      const updated = { ...current };
      if (!mode || mode === "auto") {
        delete updated[providerId];
      } else {
        updated[providerId] = { mode };
      }
      await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ providerThinking: updated }),
      });
    } catch (error) {
      console.log("Error saving thinking config:", error);
    }
  };

  const handleThinkingModeChange = (mode) => {
    setThinkingMode(mode);
    saveThinkingConfig(mode);
  };

  const saveAutoPing = async (next) => {
    const autoPingSettingsKey = AUTO_PING_SETTINGS_KEYS[providerId];
    if (!autoPingSettingsKey) return;

    setAutoPing(next);
    try {
      await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ [autoPingSettingsKey]: next }),
      });
    } catch (error) {
      console.log("Error saving auto-ping config:", error);
    }
  };

  const handleAutoPingConnection = (connectionId, on) => {
    saveAutoPing({ ...autoPing, connections: { ...autoPing.connections, [connectionId]: on } });
  };

  useEffect(() => {
    // Intentional initial synchronization with provider APIs.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    fetchConnections();
    fetchAliases();
    fetchCustomModels();
    fetchDisabledModels();
  }, [fetchConnections, fetchAliases, fetchCustomModels, fetchDisabledModels]);

  // Cursor's model availability is account-specific and changes frequently.
  // Load the active account's live catalog for the dashboard; the static
  // registry remains the fallback while the request is pending or unavailable.
  useEffect(() => {
    if (providerId !== "cursor") {
      // Intentional reset when leaving the Cursor provider.
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setLiveModels([]);
      return;
    }

    const connection = connections.find((item) => item.isActive !== false);
    if (!connection?.id) {
      setLiveModels([]);
      return;
    }

    let cancelled = false;
    fetch(`/api/providers/${connection.id}/models`, { cache: "no-store" })
      .then(async (res) => ({ ok: res.ok, data: await res.json() }))
      .then(({ ok, data }) => {
        if (!cancelled && ok && Array.isArray(data.models) && data.models.length > 0) {
          setLiveModels(data.models);
        }
      })
      .catch(() => {});

    return () => { cancelled = true; };
  }, [providerId, connections]);

  // Fetch suggested models from provider's public API (if configured)
  useEffect(() => {
    const fetcher = (OAUTH_PROVIDERS[providerId] || APIKEY_PROVIDERS[providerId] || FREE_PROVIDERS[providerId] || FREE_TIER_PROVIDERS[providerId])?.modelsFetcher;
    if (!fetcher) return;
    fetchSuggestedModels(fetcher).then(setSuggestedModels);
  }, [providerId]);

  const handleSetAlias = async (modelId, alias, providerAliasOverride = providerAlias) => {
    const fullModel = `${providerAliasOverride}/${modelId}`;
    try {
      const res = await fetch("/api/models/alias", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ model: fullModel, alias }),
      });
      if (res.ok) {
        await fetchAliases();
      } else {
        const data = await res.json();
        setNoticeState({ title: translate("Error"), message: data.error || "Failed to set alias", variant: "error" });
      }
    } catch (error) {
      console.log("Error setting alias:", error);
    }
  };

  const handleDeleteAlias = async (alias) => {
    try {
      const res = await fetch(`/api/models/alias?alias=${encodeURIComponent(alias)}`, {
        method: "DELETE",
      });
      if (res.ok) {
        await fetchAliases();
      }
    } catch (error) {
      console.log("Error deleting alias:", error);
    }
  };

  // `transport` pins a realtime STT dispatch marker (shared whitelist
  // STT_TRANSPORT_META); the API only honours it on type "stt" records.
  const handleAddCustomModel = async (modelId, type = "llm", providerAliasOverride = providerStorageAlias, caps, transport) => {
    try {
      const res = await fetch("/api/models/custom", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ providerAlias: providerAliasOverride, id: modelId, type, ...(caps ? { caps } : {}), ...(transport ? { transport } : {}) }),
      });
      if (res.ok) {
        await fetchCustomModels();
        if (typeof window !== "undefined") window.dispatchEvent(new CustomEvent("customModelChanged"));
      } else {
        const data = await res.json();
        setNoticeState({ title: translate("Error"), message: data.error || "Failed to add custom model", variant: "error" });
      }
    } catch (error) {
      console.log("Error adding custom model:", error);
    }
  };

  const handleDeleteCustomModel = async (modelId, type = "llm", providerAliasOverride = providerStorageAlias) => {
    try {
      const params = new URLSearchParams({ providerAlias: providerAliasOverride, id: modelId, type });
      const res = await fetch(`/api/models/custom?${params}`, { method: "DELETE" });
      if (res.ok) {
        await fetchCustomModels();
        if (typeof window !== "undefined") window.dispatchEvent(new CustomEvent("customModelChanged"));
      }
    } catch (error) {
      console.log("Error deleting custom model:", error);
    }
  };

  /**
   * Fetch the provider's live models and import the ones not yet available.
   * Uses the active connection's /models catalog when present (OAuth / API key /
   * compatible nodes all resolved server-side), otherwise falls back to the
   * provider's public modelsFetcher (free / no-auth providers).
   */
  const handleImportProviderModels = async (mode = "batch") => {
    if (importingModels) return;
    const activeConnection = connections.find((conn) => conn.isActive !== false);

    setImportingModels(true);
    try {
      let fetchedModels = [];
      if (activeConnection) {
        const res = await fetch(`/api/providers/${activeConnection.id}/models`);
        const data = await res.json();
        if (!res.ok) {
          setNoticeState({ title: translate("Error"), message: data.error || translate("Failed to fetch models"), variant: "error" });
          return;
        }
        fetchedModels = data.models || [];
        if (fetchedModels.length === 0) {
          const fetcher = (OAUTH_PROVIDERS[providerId] || APIKEY_PROVIDERS[providerId] || FREE_PROVIDERS[providerId] || FREE_TIER_PROVIDERS[providerId])?.modelsFetcher;
          if (fetcher) fetchedModels = await fetchSuggestedModels(fetcher);
        }
      } else {
        const fetcher = (OAUTH_PROVIDERS[providerId] || APIKEY_PROVIDERS[providerId] || FREE_PROVIDERS[providerId] || FREE_TIER_PROVIDERS[providerId])?.modelsFetcher;
        if (!fetcher) {
          setNoticeState({ title: translate("Notice"), message: translate("Please add an active connection first"), variant: "info" });
          return;
        }
        fetchedModels = await fetchSuggestedModels(fetcher);
      }

      const validModels = fetchedModels.filter((m) => m?.id || m?.name || m?.model);
      if (validModels.length === 0) {
        setNoticeState({ title: translate("Notice"), message: translate("No models returned"), variant: "info" });
        return;
      }

      // Some resolvers return ids prefixed with the provider (e.g. "qoder/auto"); store them bare.
      const prefixPattern = new RegExp(`^(?:${providerId}|${providerStorageAlias})/`);
      // Skip obvious non-chat ids (embeddings, tts, image gen, moderation...) so
      // importing from a raw /models catalog doesn't pollute the model list.
      const nonLlmIdPattern = /embed|whisper|dall-e|dalle|imagen|moderation|rerank|tts/i;

      const existingIds = new Set([
        ...models.map((m) => m.id),
        ...kiloFreeModels.map((m) => m.id),
        ...customModels
          .filter((entry) => entry.providerAlias === providerStorageAlias && (entry.kind || entry.type || "llm") === "llm")
          .map((entry) => entry.id),
      ]);
      for (const fullModel of Object.values(modelAliases)) {
        if (typeof fullModel !== "string") continue;
        if (fullModel.startsWith(`${providerStorageAlias}/`)) {
          existingIds.add(fullModel.slice(providerStorageAlias.length + 1));
        } else if (fullModel.startsWith(`${providerId}/`)) {
          existingIds.add(fullModel.slice(providerId.length + 1));
        }
      }

      const toImport = [];
      for (const model of validModels) {
        const rawId = model.id || model.name || model.model;
        const kind = getModelKind(model);
        if (kind && kind !== "llm") continue;
        if (nonLlmIdPattern.test(rawId)) continue;

        const modelId = rawId.replace(prefixPattern, "");
        if (!modelId || existingIds.has(modelId)) continue;

        toImport.push({ providerAlias: providerStorageAlias, id: modelId, type: "llm" });
        existingIds.add(modelId);
      }

      if (toImport.length === 0) {
        setNoticeState({ title: translate("Notice"), message: translate("All models already exist, no new models added"), variant: "info" });
      } else if (mode === "all") {
        const count = await importSelectedModels(toImport);
        await fetchCustomModels();
        if (typeof window !== "undefined") window.dispatchEvent(new CustomEvent("customModelChanged"));
        setNoticeState({ title: translate("Success"), message: translate("Successfully added") + ` ${count} ` + translate("models"), variant: "success" });
      } else {
        setImportPreview(toImport);
      }
    } catch (error) {
      console.log("Error importing provider models:", error);
      setNoticeState({ title: translate("Error"), message: translate("Error fetching models") + ": " + error.message, variant: "error" });
    } finally {
      setImportingModels(false);
    }
  };

  const confirmModelImport = async (selected) => {
    const count = await importSelectedModels(selected);
    await fetchCustomModels();
    window.dispatchEvent(new CustomEvent("customModelChanged"));
    setNoticeState({ title: translate("Success"), message: `${count} models imported`, variant: "success" });
  };

  const handleRunOneByOneTest = async () => {
    if (oneByOneRunning || connections.length === 0) return;

    const queuedState = Object.fromEntries(
      connections.map((connection) => [connection.id, { state: "queued", error: null }]),
    );

    stopOneByOneRef.current = false;
    setOneByOneRunning(true);
    setOneByOneStopping(false);
    setOneByOneCurrentConnectionId(null);
    setOneByOneResults(queuedState);
    setOneByOneSummary({ total: connections.length, completed: 0, passed: 0, failed: 0, stopped: false });

    let passed = 0;
    let failed = 0;

    try {
      for (let index = 0; index < connections.length; index += 1) {
        if (stopOneByOneRef.current) {
          setOneByOneSummary({
            total: connections.length,
            completed: index,
            passed,
            failed,
            stopped: true,
          });
          break;
        }

        const connection = connections[index];
        setOneByOneCurrentConnectionId(connection.id);
        setOneByOneResults((prev) => ({
          ...prev,
          [connection.id]: { state: "testing", error: null },
        }));

        try {
          const res = await fetch(`/api/providers/${connection.id}/test`, { method: "POST" });
          const data = await res.json();
          const valid = !!data.valid;

          if (valid) {
            passed += 1;
          } else {
            failed += 1;
          }

          setOneByOneResults((prev) => ({
            ...prev,
            [connection.id]: {
              state: valid ? "success" : "failed",
              error: valid ? null : (data.error || null),
            },
          }));
        } catch (error) {
          failed += 1;
          setOneByOneResults((prev) => ({
            ...prev,
            [connection.id]: {
              state: "failed",
              error: error.message || "Test failed",
            },
          }));
        }

        setOneByOneSummary({
          total: connections.length,
          completed: index + 1,
          passed,
          failed,
          stopped: false,
        });

        if (index < connections.length - 1) {
          await sleep(ONE_BY_ONE_DELAY_MS);
        }
      }
    } finally {
      setOneByOneCurrentConnectionId(null);
      setOneByOneRunning(false);
      setOneByOneStopping(false);
      stopOneByOneRef.current = false;
    }
  };

  const handleStopOneByOneTest = () => {
    if (!oneByOneRunning) return;
    stopOneByOneRef.current = true;
    setOneByOneStopping(true);
  };

  const handleDelete = async (id) => {
    setConfirmState({
      title: "Delete Connection",
      message: "Delete this connection?",
      onConfirm: async () => {
        setConfirmState(null);
        try {
          const res = await fetch(`/api/providers/${id}`, { method: "DELETE" });
          if (res.ok) {
            setConnections(prev => prev.filter(c => c.id !== id));
          }
        } catch (error) {
          console.log("Error deleting connection:", error);
        }
      }
    });
  };

  const handleBulkDelete = () => {
    const count = selectedConnectionIds.length;
    if (count === 0) return;
    setConfirmState({
      title: `Delete ${count} Connection${count > 1 ? "s" : ""}`,
      message: `Delete ${count} connection${count > 1 ? "s" : ""}? This cannot be undone.`,
      onConfirm: async () => {
        setConfirmState(null);
        let failed = 0;
        const idsToDelete = [...selectedConnectionIds];
        for (const id of idsToDelete) {
          try {
            const res = await fetch(`/api/providers/${id}`, { method: "DELETE" });
            if (!res.ok) failed += 1;
          } catch (error) {
            console.log("Error deleting connection:", error);
            failed += 1;
          }
        }
        setConnections(prev => prev.filter(c => !idsToDelete.includes(c.id)));
        setSelectedConnectionIds([]);
        if (failed > 0) setNoticeState({ title: translate("Error"), message: `Deleted ${idsToDelete.length - failed} connection(s), ${failed} failed.`, variant: "error" });
      }
    });
  };

  const handleOAuthSuccess = () => {
    fetchConnections();
    setShowOAuthModal(false);
  };

  const handleIFlowCookieSuccess = () => {
    fetchConnections();
    setShowIFlowCookieModal(false);
  };

  const handleSaveApiKey = async (formData) => {
    setAddConnectionError("");
    try {
      const res = await fetch("/api/providers", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ provider: providerId, ...formData }),
      });

      let data = null;
      try {
        data = await res.json();
      } catch {
        data = null;
      }

      if (res.ok) {
        await fetchConnections();
        setShowAddApiKeyModal(false);
        return;
      }

      setAddConnectionError(data?.error || "Failed to save connection");
    } catch (error) {
      console.log("Error saving connection:", error);
      setAddConnectionError("Failed to save connection");
    }
  };

  const handleUpdateConnection = async (formData) => {
    try {
      const res = await fetch(`/api/providers/${selectedConnection.id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(formData),
      });
      if (res.ok) {
        await fetchConnections();
        setShowEditModal(false);
      }
    } catch (error) {
      console.log("Error updating connection:", error);
    }
  };

  const handleUpdateConnectionStatus = async (id, isActive) => {
    try {
      const res = await fetch(`/api/providers/${id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ isActive }),
      });
      if (res.ok) {
        setConnections(prev => prev.map(c => c.id === id ? { ...c, isActive } : c));
      }
    } catch (error) {
      console.log("Error updating connection status:", error);
    }
  };

  const handleSwapPriority = async (index1, index2) => {
    // Optimistic update state
    const newConnections = [...connections];
    [newConnections[index1], newConnections[index2]] = [newConnections[index2], newConnections[index1]];
    setConnections(newConnections);

    try {
      await Promise.all([
        fetch(`/api/providers/${newConnections[index1].id}`, {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ priority: index1 }),
        }),
        fetch(`/api/providers/${newConnections[index2].id}`, {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ priority: index2 }),
        }),
      ]);
    } catch (error) {
      console.log("Error swapping priority:", error);
      await fetchConnections();
    }
  };

  const selectedConnections = connections.filter((conn) => selectedConnectionIds.includes(conn.id));

  const { currentPage: connectionPageClamped, totalPages: connectionTotalPages, items: pagedConnections, start: pagedStart } = computeConnectionPagination(connections, connectionPage);
  const allSelected = connections.length > 0 && selectedConnectionIds.length === connections.length;

  const toggleSelectConnection = (connectionId) => {
    setSelectedConnectionIds((prev) => (
      prev.includes(connectionId)
        ? prev.filter((id) => id !== connectionId)
        : [...prev, connectionId]
    ));
  };

  const toggleSelectAllConnections = () => {
    if (allSelected) {
      setSelectedConnectionIds([]);
      return;
    }
    setSelectedConnectionIds(connections.map((conn) => conn.id));
  };

  const clearSelection = () => {
    setSelectedConnectionIds([]);
    setBulkProxyPoolId("__none__");
  };

  useEffect(() => {
    // Intentional reconciliation with the current connection list.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setSelectedConnectionIds((prev) => prev.filter((id) => connections.some((conn) => conn.id === id)));
  }, [connections]);

  const selectedProxySummary = (() => {
    if (selectedConnections.length === 0) return "";
    const poolIds = new Set(selectedConnections.map((conn) => conn.providerSpecificData?.proxyPoolId || "__none__"));
    if (poolIds.size === 1) {
      const onlyId = [...poolIds][0];
      if (onlyId === "__none__") return "All selected currently unbound";
      const pool = proxyPools.find((p) => p.id === onlyId);
      return `All selected currently bound to ${pool?.name || onlyId}`;
    }
    return "Selected connections have mixed proxy bindings";
  })();

  const openBulkProxyModal = () => {
    if (selectedConnections.length === 0) return;
    const uniquePoolIds = [...new Set(selectedConnections.map((conn) => conn.providerSpecificData?.proxyPoolId || "__none__"))];
    setBulkProxyPoolId(uniquePoolIds.length === 1 ? uniquePoolIds[0] : "__none__");
    setShowBulkProxyModal(true);
  };

  const closeBulkProxyModal = () => {
    if (bulkUpdatingProxy) return;
    setShowBulkProxyModal(false);
  };

  const updateConnectionProxy = async (connectionId, body) => {
    const res = await fetch(`/api/providers/${connectionId}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      const error = await res.json().catch(() => ({}));
      throw new Error(error.error || `Proxy update failed (${res.status})`);
    }
    return res.json();
  };

  const applyProxyAssignments = async (assignments) => {
    setBulkUpdatingProxy(true);
    try {
      const failedConnectionIds = [];
      for (const { connectionId, proxyPoolId, proxyPoolIds, proxyRotationStrategy } of assignments) {
        try {
          const body = proxyPoolIds
            ? { proxyPoolIds, proxyRotationStrategy }
            : { proxyPoolId };

          await updateConnectionProxy(connectionId, body);
        } catch (e) {
          console.log("Error applying proxy for", connectionId, e);
          failedConnectionIds.push(connectionId);
        }
      }
      await fetchConnections();
      if (failedConnectionIds.length > 0) {
        setNoticeState({ title: translate("Error"), message: `Updated with ${failedConnectionIds.length} failed request(s). Retry the selected connections.`, variant: "error" });
        return;
      }
      setShowBulkProxyModal(false);
      clearSelection();
    } finally {
      setBulkUpdatingProxy(false);
    }
  };

  const handleApplySinglePool = (proxyPoolId) => {
    const targets = (selectedConnectionIds.length > 0 ? selectedConnections : connections).map((c) => ({
      connectionId: c.id,
      proxyPoolId,
    }));
    return applyProxyAssignments(targets);
  };

  const handleApplyOneToOne = () => {
    const activePools = proxyPools.filter((p) => p.isActive === true);
    if (activePools.length === 0) {
      setNoticeState({ title: translate("Notice"), message: "No active proxy pools available.", variant: "info" });
      return;
    }
    const targets = (selectedConnectionIds.length > 0 ? selectedConnections : connections).map((c, i) => ({
      connectionId: c.id,
      proxyPoolId: activePools[i % activePools.length].id,
    }));
    return applyProxyAssignments(targets);
  };

  const handleApplyAdvancedProxy = (strategy, ids) => {
    const activeIds = ids.filter((id) => proxyPools.find(p => p.id === id)?.isActive === true);
    if (!activeIds.length) {
      setNoticeState({ title: translate("Notice"), message: "No active proxy pools selected.", variant: "info" });
      return;
    }
    const targets = (selectedConnectionIds.length > 0 ? selectedConnections : connections).map((c) => ({
      connectionId: c.id,
      proxyPoolIds: activeIds,
      proxyRotationStrategy: strategy,
    }));
    return applyProxyAssignments(targets);
  };


  const isSelected = (connectionId) => selectedConnectionIds.includes(connectionId);

  const connectionsList = (
    <Box
      sx={{
        display: "flex",
        minWidth: 0,
        flexDirection: "column",
        "& > * + *": { borderTop: 1, borderColor: dividerFaint },
      }}
    >
      {pagedConnections.map((conn, pageIndex) => {
        const index = pagedStart + pageIndex;
        return (
          <Box key={conn.id} sx={{ display: "flex", minWidth: 0, alignItems: "stretch" }}>
            <Box sx={{ display: "flex", flexShrink: 0, alignItems: "center", pl: { xs: 0.5, sm: 1 } }}>
              <Checkbox
                checked={isSelected(conn.id)}
                onChange={() => toggleSelectConnection(conn.id)}
                size="small"
                sx={{ p: 0.5 }}
              />
            </Box>
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <ConnectionRow
                connection={conn}
                proxyPools={proxyPools}
                isOAuth={isOAuth}
                isFirst={index === 0}
                isLast={index === connections.length - 1}
                onMoveUp={() => handleSwapPriority(index, index - 1)}
                onMoveDown={() => handleSwapPriority(index, index + 1)}
                onToggleActive={(isActive) => handleUpdateConnectionStatus(conn.id, isActive)}
                autoPing={AUTO_PING_SETTINGS_KEYS[providerId] && conn.authType === "oauth" ? {
                  on: autoPing.connections[conn.id] === true,
                  onToggle: (on) => handleAutoPingConnection(conn.id, on),
                  provider: providerId,
                } : null}
                onUpdateProxy={async (proxySelection) => {
                  try {
                    const body = typeof proxySelection === "object"
                      ? proxySelection
                      : { proxyPoolId: proxySelection || null };
                    const res = await fetch(`/api/providers/${conn.id}`, {
                      method: "PUT",
                      headers: { "Content-Type": "application/json" },
                      body: JSON.stringify(body),
                    });
                    if (res.ok) {
                      const nextProviderSpecificData = { ...conn.providerSpecificData };
                      if (body.proxyPoolIds) {
                        nextProviderSpecificData.proxyPoolIds = body.proxyPoolIds;
                        nextProviderSpecificData.proxyRotationStrategy = body.proxyRotationStrategy;
                        delete nextProviderSpecificData.proxyPoolId;
                      } else if (body.proxyPoolId) {
                        nextProviderSpecificData.proxyPoolId = body.proxyPoolId;
                        delete nextProviderSpecificData.proxyPoolIds;
                        delete nextProviderSpecificData.proxyRotationStrategy;
                      } else {
                        delete nextProviderSpecificData.proxyPoolId;
                        delete nextProviderSpecificData.proxyPoolIds;
                        delete nextProviderSpecificData.proxyRotationStrategy;
                      }
                      setConnections(prev => prev.map(c =>
                        c.id === conn.id ? { ...c, providerSpecificData: nextProviderSpecificData } : c
                      ));
                    }
                  } catch (error) {
                    console.log("Error updating proxy:", error);
                  }
                }}
                onEdit={() => {
                  setSelectedConnection(conn);
                  setShowEditModal(true);
                }}
                onDelete={() => handleDelete(conn.id)}
                oneByOneStatus={oneByOneResults[conn.id] || null}
                modelAssignmentOptions={providerId === "freebuff" ? assignmentModels : null}
                onModelAssignmentChange={providerId === "freebuff" ? (model) => handleModelAssignment(conn.id, model) : null}
                strictModelAssignment={strictModelAssignment}
              />
            </Box>
          </Box>
        );
      })}
      {connections.length > CONNECTIONS_PER_PAGE && (
        <Pagination
          currentPage={connectionPageClamped}
          pageSize={CONNECTIONS_PER_PAGE}
          totalItems={connections.length}
          onPageChange={setConnectionPage}
        />
      )}
    </Box>
  );

  const activePools = proxyPools.filter((p) => p.isActive === true);

  const bulkActionModal = (
    <Modal
      isOpen={showBulkProxyModal}
      onClose={closeBulkProxyModal}
      title={selectedConnectionIds.length > 0 ? `Apply Proxy (${selectedConnectionIds.length} selected)` : `Apply Proxy (All ${connections.length} connections)`}
    >
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {selectedProxySummary && (
          <Box
            component="p"
            sx={{
              m: 0,
              fontSize: 12,
              color: "text.secondary",
              bgcolor: (theme) => (theme.palette.mode === "dark" ? "#303030" : "#f4f4f5"),
              px: 1.5,
              py: 1,
              borderRadius: 2,
              border: 1,
              borderColor: dividerSoft,
            }}
          >
            {selectedProxySummary}
          </Box>
        )}
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            border: 1,
            borderColor: dividerSoft,
            borderRadius: 3,
            overflow: "hidden",
            bgcolor: "background.paper",
            "& > * + *": { borderTop: 1, borderColor: dividerFaint },
          }}
        >
          <Box sx={{ p: 1.5, display: "flex", flexDirection: "column", gap: 0.5 }}>
            <Box
              component="p"
              sx={{
                m: 0,
                px: 1.5,
                pb: 0.5,
                fontSize: 10,
                fontWeight: 600,
                textTransform: "uppercase",
                letterSpacing: "0.06em",
                color: "text.secondary",
              }}
            >
              Dynamic Strategies
            </Box>
            <Box
              component="button"
              type="button"
              onClick={handleApplyOneToOne}
              disabled={bulkUpdatingProxy || activePools.length === 0}
              sx={proxyOptionSx}
            >
              <Box component="span" className="material-symbols-outlined" sx={{ color: "text.secondary", fontSize: 20 }}>sync_alt</Box>
              <Box sx={{ display: "flex", flexDirection: "column" }}>
                <Box component="span" sx={{ fontSize: 14, fontWeight: 500, color: "text.primary" }}>One-to-one (rotate)</Box>
                <Box component="span" sx={{ fontSize: 10, color: "text.secondary", mt: 0.25 }}>Distribute active proxy pools round-robin</Box>
              </Box>
            </Box>
            {["fill-first", "round-robin", "random", "smart"].map((strategy) => (
              <Box
                component="button"
                type="button"
                key={strategy}
                onClick={() => handleApplyAdvancedProxy(strategy, activePools.map((p) => p.id))}
                disabled={bulkUpdatingProxy || activePools.length === 0}
                sx={proxyOptionSx}
              >
                <Box component="span" className="material-symbols-outlined" sx={{ color: "text.secondary", fontSize: 20 }}>autorenew</Box>
                <Box sx={{ display: "flex", flexDirection: "column" }}>
                  <Box component="span" sx={{ fontSize: 14, fontWeight: 500, color: "text.primary", textTransform: "capitalize" }}>{strategy} strategy</Box>
                  <Box component="span" sx={{ fontSize: 10, color: "text.secondary", mt: 0.25 }}>
                    {strategy === "smart" ? "Bypass pools marked unfit for provider/model" : `Rotate over all active pools using ${strategy}`}
                  </Box>
                </Box>
              </Box>
            ))}
          </Box>

          <Box sx={{ p: 1.5, display: "flex", flexDirection: "column", gap: 0.5 }}>
            <Box
              component="p"
              sx={{
                m: 0,
                px: 1.5,
                pb: 0.5,
                fontSize: 10,
                fontWeight: 600,
                textTransform: "uppercase",
                letterSpacing: "0.06em",
                color: "text.secondary",
              }}
            >
              Static Assignment
            </Box>
            <Box
              component="button"
              type="button"
              onClick={() => handleApplySinglePool(null)}
              disabled={bulkUpdatingProxy}
              sx={proxyOptionSx}
            >
              <Box component="span" className="material-symbols-outlined" sx={{ color: "text.secondary", fontSize: 20 }}>link_off</Box>
              <Box sx={{ display: "flex", flexDirection: "column" }}>
                <Box component="span" sx={{ fontSize: 14, fontWeight: 500, color: "text.primary" }}>None (unbind all)</Box>
                <Box component="span" sx={{ fontSize: 10, color: "text.secondary", mt: 0.25 }}>Direct routing without proxy</Box>
              </Box>
            </Box>
            {proxyPools.map((pool) => (
              <Box
                component="button"
                type="button"
                key={pool.id}
                onClick={() => handleApplySinglePool(pool.id)}
                disabled={bulkUpdatingProxy || pool.isActive !== true}
                sx={proxyOptionSx}
              >
                <Box component="span" className="material-symbols-outlined" sx={{ color: "text.secondary", fontSize: 20 }}>lan</Box>
                <Box sx={{ display: "flex", flexDirection: "column", minWidth: 0, flex: 1 }}>
                  <Box component="span" sx={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontSize: 14, fontWeight: 500, color: "text.primary" }}>{pool.name}</Box>
                  <Box component="span" sx={{ fontSize: 10, color: "text.secondary", mt: 0.25, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{pool.proxyUrl}</Box>
                </Box>
                {pool.isActive !== true && (
                  <Box sx={{ ml: "auto", flexShrink: 0 }}>
                    <Badge variant="secondary" size="sm">inactive</Badge>
                  </Box>
                )}
              </Box>
            ))}
          </Box>
        </Box>

        {bulkUpdatingProxy && (
          <Box component="p" sx={{ m: 0, px: 0.5, fontSize: 12, color: "text.secondary", animation: "pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite" }}>
            Applying proxy configuration...
          </Box>
        )}

        <Button onClick={closeBulkProxyModal} variant="ghost" fullWidth disabled={bulkUpdatingProxy}>
          Cancel
        </Button>
      </Box>
    </Modal>
  );

  const handleTestModel = async (modelId) => {
    if (testingModelIds.has(modelId)) return;
    setTestingModelIds((prev) => new Set(prev).add(modelId));
    try {
      const res = await fetch("/api/models/test", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ model: `${providerStorageAlias}/${modelId}` }),
      });
      const data = await res.json();
      setModelTestResults((prev) => ({ ...prev, [modelId]: data.ok ? "ok" : "error" }));
      setModelsTestError(data.ok ? "" : (data.error || "Model not reachable"));
    } catch {
      setModelTestResults((prev) => ({ ...prev, [modelId]: "error" }));
      setModelsTestError("Network error");
    } finally {
      setTestingModelIds((prev) => { const n = new Set(prev); n.delete(modelId); return n; });
    }
  };

  const deletableModelRows = getProviderCustomModelRows({
    customModels,
    modelAliases,
    providerAlias: providerStorageAlias,
    builtInModels: models,
    type: "llm",
  });
  const allDeletableSelected =
    deletableModelRows.length > 0 && selectedModelIds.length === deletableModelRows.length;
  const toggleSelectModel = (id) =>
    setSelectedModelIds((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));
  const toggleSelectAllModels = () =>
    setSelectedModelIds(allDeletableSelected ? [] : deletableModelRows.map((m) => m.id));
  const handleBulkDeleteModels = () => {
    if (selectedModelIds.length === 0) return;
    const count = selectedModelIds.length;
    setConfirmState({
      title: "Remove Added Models",
      message: `Remove ${count} added model${count === 1 ? "" : "s"} and their aliases? This cannot be undone.`,
      confirmText: "Remove",
      variant: "danger",
      onConfirm: async () => {
        setBulkDeleteBusy(true);
        try {
          const chosen = deletableModelRows.filter((m) => selectedModelIds.includes(m.id));
          await deleteSelectedModels(chosen, providerStorageAlias, modelAliases);
          await Promise.all([fetchCustomModels(), fetchAliases()]);
          if (typeof window !== "undefined") window.dispatchEvent(new CustomEvent("customModelChanged"));
          setSelectedModelIds([]);
          setConfirmState(null);
        } catch (error) {
          setConfirmState(null);
          setNoticeState({ title: translate("Error"), message: error.message || "Failed to remove selected models", variant: "error" });
          await Promise.all([fetchCustomModels(), fetchAliases()]);
        } finally {
          setBulkDeleteBusy(false);
        }
      },
    });
  };

  const renderModelsSection = () => {
    if (isCompatible) {
      return (
        <CompatibleModelsSection
          providerStorageAlias={providerStorageAlias}
          providerDisplayAlias={providerDisplayAlias}
          modelAliases={modelAliases}
          customModels={customModels}
          copied={copied}
          onCopy={copy}
          onSetAlias={handleSetAlias}
          onDeleteAlias={handleDeleteAlias}
          onImportModels={confirmModelImport}
          onAddCustomModel={(modelId) => handleAddCustomModel(modelId, "llm", providerStorageAlias)}
          onDeleteCustomModel={(modelId) => handleDeleteCustomModel(modelId, "llm", providerStorageAlias)}
          connections={connections}
          isAnthropic={isAnthropicCompatible}
        />
      );
    }
    // Combine hardcoded models with Kilo free models (deduplicated)
    // Exclude non-llm models (embedding, tts, etc.) — they have dedicated pages under media-providers
    const allModels = [
      ...models,
      ...kiloFreeModels.filter((fm) => !models.some((m) => m.id === fm.id)),
    ].filter((m) => { const k = getModelKind(m); return !k || k === "llm"; });
    const disabledSet = new Set(disabledModelIds);
    const displayModels = allModels.filter((m) => !disabledSet.has(m.id));
    const disabledDisplayModels = allModels.filter((m) => disabledSet.has(m.id));
    const customModelRows = getProviderCustomModelRows({
      customModels,
      modelAliases,
      providerAlias: providerStorageAlias,
      builtInModels: models,
      type: "llm",
    });

    return (
      <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1.5 }}>
        {/* Custom models first */}
        {customModelRows.map((model) => (
          <ModelRow
            key={`${model.source}-${model.fullModel}`}
            model={{ id: model.id, name: model.name }}
            fullModel={`${providerDisplayAlias}/${model.id}`}
            alias={model.alias}
            copied={copied}
            onCopy={copy}
            onSetAlias={() => {}}
            onDeleteAlias={() => {
              if (model.source === "custom") {
                handleDeleteCustomModel(model.id, "llm", providerStorageAlias);
              } else {
                handleDeleteAlias(model.alias);
              }
            }}
            testStatus={modelTestResults[model.id]}
            onTest={connections.length > 0 || isFreeNoAuth ? () => handleTestModel(model.id) : undefined}
            isTesting={testingModelIds.has(model.id)}
            isCustom
            isFree={false}
            caps={getCaps(`${providerId}/${model.id}`)}
            thinkingSuffix={resolveThinkingSuffix(model.id)}
            selected={selectedModelIds.includes(model.id)}
            onToggleSelect={() => toggleSelectModel(model.id)}
          />
        ))}

        {displayModels.map((model) => {
          const fullModel = `${providerStorageAlias}/${model.id}`;
          const oldFormatModel = `${providerId}/${model.id}`;
          const existingAlias = Object.entries(modelAliases).find(
            ([, m]) => m === fullModel || m === oldFormatModel
          )?.[0];
          return (
            <ModelRow
              key={model.id}
              model={model}
              fullModel={`${providerDisplayAlias}/${model.id}`}
              alias={existingAlias}
              copied={copied}
              onCopy={copy}
              onSetAlias={(alias) => handleSetAlias(model.id, alias, providerStorageAlias)}
              onDeleteAlias={() => handleDeleteAlias(existingAlias)}
              testStatus={modelTestResults[model.id]}
              onTest={connections.length > 0 || isFreeNoAuth ? () => handleTestModel(model.id) : undefined}
              isTesting={testingModelIds.has(model.id)}
              isFree={model.isFree}
              onDisable={() => handleDisableModel(model.id)}
              caps={getCaps(`${providerId}/${model.id}`)}
              thinkingSuffix={resolveThinkingSuffix(model.id)}
            />
          );
        })}

        {/* Add model button — inline, same style as model chips */}
        <Button
          onClick={() => setShowAddCustomModel(true)}
          variant="outline"
          sx={{
            width: { xs: "100%", sm: "auto" },
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            gap: 0.75,
            borderRadius: 2,
            borderStyle: "dashed",
            borderColor: "rgba(229, 106, 74, 0.4)",
            px: 1.5,
            py: 1,
            minHeight: 34,
            fontSize: 12,
            textTransform: "none",
            color: "primary.main",
            "&:hover": { borderColor: "primary.main", bgcolor: "rgba(229, 106, 74, 0.05)" },
          }}
        >
          <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 14 }}>add</Box>
          Add Model
        </Button>

        {/* Suggested models from provider API — show only models not yet added */}
        {suggestedModels.length > 0 && (() => {
          const addedFullModels = new Set([
            ...Object.values(modelAliases),
            ...customModelRows.map((model) => model.fullModel),
          ]);
          const hardcodedIds = new Set(models.map((m) => m.id));
          const notAdded = suggestedModels.filter(
            (m) => !addedFullModels.has(`${providerStorageAlias}/${m.id}`) && !hardcodedIds.has(m.id)
          );
          if (notAdded.length === 0) return null;
          return (
            <Box sx={{ width: "100%", mt: 1 }}>
              <Typography sx={{ fontSize: 12, color: "text.secondary", mb: 1 }}>Suggested free models (≥200k context):</Typography>
              <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
                {notAdded.map((m) => (
                  <Box
                    component="button"
                    key={m.id}
                    onClick={async () => {
                      await handleAddCustomModel(m.id, "llm", providerStorageAlias);
                    }}
                    sx={modelChipSx}
                    title={`${m.name} · ${(m.contextLength / 1000).toFixed(0)}k ctx`}
                  >
                    <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 13 }}>add</Box>
                    {m.id.split("/").pop()}
                  </Box>
                ))}
              </Box>
            </Box>
          );
        })()}

        {/* Disabled models — restorable */}
        {disabledDisplayModels.length > 0 && (
          <Box sx={{ width: "100%", mt: 1 }}>
            <Typography sx={{ fontSize: 12, color: "text.secondary", mb: 1 }}>Disabled models ({disabledDisplayModels.length}):</Typography>
            <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
              {disabledDisplayModels.map((m) => (
                <Box
                  component="button"
                  key={m.id}
                  onClick={() => handleEnableModel(m.id)}
                  sx={modelChipDashedSx}
                  title="Restore model"
                >
                  <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 13 }}>add</Box>
                  {m.id}
                </Box>
              ))}
            </Box>
          </Box>
        )}
      </Box>
    );
  };

  if (loading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 4 }}>
        <CardSkeleton />
        <CardSkeleton />
      </Box>
    );
}

  if (!providerInfo) {
    return (
      <Box sx={{ textAlign: "center", py: 10 }}>
        <Typography sx={{ color: "text.secondary" }}>Provider not found</Typography>
        <Box component={Link} href="/dashboard/providers" sx={{ color: "primary.main", mt: 2, display: "inline-block" }}>
          Back to Providers
        </Box>
      </Box>
    );
  }

  // Determine icon path: OpenAI Compatible providers use specialized icons
  const getHeaderIconPath = () => {
    if (isOpenAICompatible && providerInfo.apiType) {
      return providerInfo.apiType === "responses" ? "/providers/oai-r.webp" : "/providers/oai-cc.webp";
    }
    if (isAnthropicCompatible) {
      return "/providers/anthropic-m.webp";
    }
    return getProviderIconSrc(providerInfo.id);
  };

  return (
    <Box sx={{ display: "flex", minWidth: 0, flexDirection: "column", gap: { xs: 3, sm: 4 }, px: { xs: 0.5, sm: 0 } }}>
      {/* Header */}
      <Box sx={{ minWidth: 0 }}>
        <Box
          component={Link}
          href="/dashboard/providers"
          sx={{
            display: "inline-flex",
            alignItems: "center",
            gap: 0.5,
            fontSize: 14,
            color: "text.secondary",
            textDecoration: "none",
            mb: 2,
            transition: "color 150ms ease",
            "&:hover": { color: "primary.main" },
          }}
        >
          <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 18 }}>arrow_back</Box>
          Back to Providers
        </Box>
        <Box sx={{ display: "flex", minWidth: 0, alignItems: "center", gap: { xs: 1.5, sm: 2 } }}>
          <Box sx={{ flexShrink: 0, "& img": { borderRadius: 2 } }}>
            <ProviderIcon
              providerId={providerInfo.id}
              size={48}
              alt={providerInfo.name}
              fallbackText={providerInfo.textIcon || providerInfo.id.slice(0, 2).toUpperCase()}
              fallbackColor={providerInfo.color}
            />
          </Box>
          <Box sx={{ minWidth: 0 }}>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
              <Typography
                component="h1"
                sx={{
                  fontSize: { xs: 24, sm: 30 },
                  fontWeight: 600,
                  letterSpacing: "-0.02em",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
              >
                {providerInfo.name}
              </Typography>
              {(providerInfo.notice?.apiKeyUrl || providerInfo.notice?.signupUrl || providerInfo.website) && (
                <Box
                  component="a"
                  href={providerInfo.notice?.apiKeyUrl || providerInfo.notice?.signupUrl || providerInfo.website}
                  target="_blank"
                  rel="noopener noreferrer"
                  sx={{
                    fontSize: 12,
                    color: "primary.main",
                    display: "inline-flex",
                    alignItems: "center",
                    gap: 0.5,
                    "&:hover": { textDecoration: "underline" },
                  }}
                >
                  <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 14 }}>open_in_new</Box>
                  {providerInfo.notice?.apiKeyUrl ? "Get API Key" : "Sign up / Learn more"}
                </Box>
              )}
            </Box>
            <Typography sx={{ color: "text.secondary" }}>
              {connections.length} connection{connections.length === 1 ? "" : "s"}
            </Typography>
          </Box>
        </Box>
      </Box>

      {providerInfo.deprecated && (
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 1,
            px: 1.5,
            py: 1,
            borderRadius: 2,
            bgcolor: "rgba(234, 179, 8, 0.1)",
            border: 1,
            borderColor: "rgba(234, 179, 8, 0.3)",
          }}
        >
          <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 16, color: "#eab308", mt: 0.25, flexShrink: 0 }}>warning</Box>
          <Typography sx={{ fontSize: 12, lineHeight: 1.6, color: (theme) => (theme.palette.mode === "dark" ? "#facc15" : "#dc2626") }}>
            {providerInfo.deprecationNotice}
          </Typography>
        </Box>
      )}

      {providerInfo.notice?.text && !providerInfo.deprecated && (
        <Box
          sx={{
            display: "flex",
            flexDirection: { xs: "column", sm: "row" },
            alignItems: { sm: "center" },
            gap: 1,
            borderRadius: 2,
            border: 1,
            borderColor: "rgba(59, 130, 246, 0.3)",
            bgcolor: "rgba(59, 130, 246, 0.1)",
            px: 1.5,
            py: 1,
          }}
        >
          <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 16, color: "#3b82f6", flexShrink: 0 }}>info</Box>
          <Typography sx={{ minWidth: 0, flex: 1, fontSize: 12, lineHeight: 1.6, color: (theme) => (theme.palette.mode === "dark" ? "#60a5fa" : "#2563eb") }}>
            {providerInfo.notice.text}
          </Typography>
          {providerInfo.notice.apiKeyUrl && (
            <Box
              component="a"
              href={providerInfo.notice.apiKeyUrl}
              target="_blank"
              rel="noopener noreferrer"
              sx={{
                display: "inline-flex",
                justifyContent: "center",
                borderRadius: 1,
                bgcolor: "#3b82f6",
                px: 1,
                py: { xs: 0.5, sm: 0.25 },
                fontSize: 12,
                fontWeight: 500,
                color: "#ffffff",
                textDecoration: "none",
                transition: "background-color 150ms ease",
                "&:hover": { bgcolor: "#2563eb" },
              }}
            >
              Get API Key →
            </Box>
          )}
        </Box>
      )}

      {isCompatible && providerNode && (
        <Card>
          <Box
            sx={{
              mb: 2,
              display: "flex",
              flexDirection: { xs: "column", sm: "row" },
              gap: 1.5,
              alignItems: { sm: "flex-start" },
              justifyContent: { sm: "space-between" },
            }}
          >
            <Box sx={{ minWidth: 0 }}>
              <Typography component="h2" sx={{ fontSize: 18, fontWeight: 600 }}>{isAnthropicCompatible ? "Anthropic Compatible Details" : "OpenAI Compatible Details"}</Typography>
              <Typography sx={{ wordBreak: "break-all", fontSize: 14, color: "text.secondary" }}>
                {isAnthropicCompatible ? "Messages API" : (providerNode.apiType === "responses" ? "Responses API" : "Chat Completions")} · {(providerNode.baseUrl || "").replace(/\/$/, "")}/
                {isAnthropicCompatible ? "messages" : (providerNode.apiType === "responses" ? "responses" : "chat/completions")}
              </Typography>
            </Box>
            <Box
              sx={{
                display: { xs: "grid", sm: "flex" },
                gridTemplateColumns: { xs: "1fr" },
                gap: 1,
                alignItems: { sm: "center" },
              }}
            >
              <Button
                size="sm"
                icon="add"
                onClick={() => {
                  setAddConnectionError("");
                  setShowAddApiKeyModal(true);
                }}
              >
                Add API Key
              </Button>
              <Button
                size="sm"
                variant="secondary"
                icon="edit"
                onClick={() => setShowEditNodeModal(true)}
              >
                Edit
              </Button>
              <Button
                size="sm"
                variant="secondary"
                icon="delete"
                onClick={async () => {
                  setConfirmState({
                    title: "Delete Compatible Node",
                    message: `Delete this ${isAnthropicCompatible ? "Anthropic" : "OpenAI"} Compatible node?`,
                    onConfirm: async () => {
                      setConfirmState(null);
                      try {
                        const res = await fetch(`/api/provider-nodes/${providerId}`, { method: "DELETE" });
                        if (res.ok) {
                          router.push("/dashboard/providers");
                        }
                      } catch (error) {
                        console.log("Error deleting provider node:", error);
                      }
                    }
                  });
                }}
              >
                Delete
              </Button>
            </Box>
          </Box>
        </Card>
      )}

      {/* Connections */}
      {isFreeNoAuth ? (
        <NoAuthProxyCard providerId={providerId} />
      ) : (
        <Card>
          <Box
            sx={{
              mb: 2,
              display: "flex",
              flexDirection: { xs: "column", sm: "row" },
              gap: 1.5,
              alignItems: { sm: "center" },
              justifyContent: { sm: "space-between" },
            }}
          >
            <Typography component="h2" sx={{ fontSize: 18, fontWeight: 600 }}>Connections</Typography>
            <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1.5, alignItems: { sm: "center" } }}>
              {connections.length > 0 && proxyPools.length > 0 && (
                <Button
                  size="sm"
                  variant="secondary"
                  icon="lan"
                  onClick={() => setShowBulkProxyModal(true)}
                >
                  Apply Proxy
                </Button>
              )}
              {connections.length > 0 && (
                <>
                  {selectedConnectionIds.length > 0 && (
                    <Button
                      size="sm"
                      variant="danger"
                      icon="delete"
                      onClick={handleBulkDelete}
                    >
                      Delete Selected ({selectedConnectionIds.length})
                    </Button>
                  )}
                  <Button
                    size="sm"
                    variant="secondary"
                    icon="sync"
                    onClick={handleRunOneByOneTest}
                    disabled={oneByOneRunning}
                  >
                    {oneByOneRunning ? "Testing Connection One-by-One..." : "Test Connection One-by-One"}
                  </Button>
                  {oneByOneRunning && (
                    <Button
                      size="sm"
                      variant="ghost"
                      icon="stop"
                      onClick={handleStopOneByOneTest}
                      disabled={oneByOneStopping}
                    >
                      {oneByOneStopping ? "Stopping..." : "Stop"}
                    </Button>
                  )}
                </>
              )}
              {/* Round Robin toggle */}
              <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 1 }}>
                <Typography component="span" sx={{ fontSize: 12, color: "text.secondary", fontWeight: 500 }}>Round Robin</Typography>
                <Toggle
                  checked={providerStrategy === "round-robin"}
                  onChange={handleRoundRobinToggle}
                />
                {providerStrategy === "round-robin" && (
                  <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
                    <Typography component="span" sx={{ fontSize: 12, color: "text.secondary" }}>Sticky:</Typography>
                    <Box
                      component="input"
                      type="number"
                      min={1}
                      value={providerStickyLimit}
                      onChange={(e) => handleStickyLimitChange(e.target.value)}
                      placeholder="1"
                      sx={{
                        width: 56,
                        px: 1,
                        py: 0.5,
                        fontSize: 12,
                        border: 1,
                        borderColor: "divider",
                        borderRadius: 1,
                        bgcolor: "background.default",
                        color: "text.primary",
                        outline: "none",
                        "&:focus": { borderColor: "primary.main" },
                      }}
                    />
                  </Box>
                )}
              </Box>
              {providerId === "freebuff" && (
                <Box
                  sx={{
                    display: "flex",
                    flexWrap: "wrap",
                    alignItems: "center",
                    gap: 1,
                    borderTop: 1,
                    borderColor: dividerFaint,
                    pt: 1,
                  }}
                >
                  <Box>
                    <Typography component="span" sx={{ fontSize: 12, color: "text.secondary", fontWeight: 500 }}>Strict Model Assignment</Typography>
                    <Typography sx={{ fontSize: 10, color: "text.secondary" }}>Only assigned accounts can serve each Freebuff model.</Typography>
                  </Box>
                  <Toggle checked={strictModelAssignment} onChange={handleStrictAssignmentToggle} />
                </Box>
              )}
            </Box>
          </Box>

          {connections.length === 0 ? (
            <Box
              sx={{
                display: "flex",
                flexDirection: { xs: "column", sm: "row" },
                gap: 1.5,
                alignItems: { sm: "center" },
                justifyContent: { sm: "space-between" },
              }}
            >
              <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
                <Box
                  sx={{
                    display: "inline-flex",
                    alignItems: "center",
                    justifyContent: "center",
                    width: 36,
                    height: 36,
                    borderRadius: "50%",
                    bgcolor: "rgba(229, 106, 74, 0.1)",
                    color: "primary.main",
                    flexShrink: 0,
                  }}
                >
                  <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 18 }}>{isOAuth ? "lock" : "key"}</Box>
                </Box>
                <Box sx={{ minWidth: 0 }}>
                  <Typography sx={{ fontSize: 14, color: "text.secondary" }}>No connections yet</Typography>
                  {hasDualAuthModes && (
                    <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
                      Choose {oauthConnectionLabel} or {apiKeyConnectionLabel}.
                    </Typography>
                  )}
                </Box>
              </Box>
              <Box sx={{ display: "flex", gap: 1 }}>
                {hasDualAuthModes ? (
                  <>
                    <Button size="sm" icon="lock" variant="secondary" onClick={triggerOAuthConnection}>
                      {oauthConnectionLabel}
                    </Button>
                    <Button size="sm" icon="key" onClick={triggerApiKeyConnection}>
                      {apiKeyConnectionLabel}
                    </Button>
                  </>
                ) : (
                  <>
                    {!isCompatible && providerId === "iflow" && (
                      <Button size="sm" icon="cookie" variant="secondary" onClick={() => setShowIFlowCookieModal(true)}>
                        Cookie
                      </Button>
                    )}
                    {providerId === "codex" && (
                      <Button size="sm" icon="playlist_add" variant="secondary" onClick={() => setShowBulkImportCodex(true)}>
                        {translate("Bulk Add")}
                      </Button>
                    )}
                    {providerId === "grok-cli" && (
                      <Button size="sm" icon="playlist_add" variant="secondary" onClick={() => setShowBulkImportGrokCli(true)}>
                        {translate("Bulk Add")}
                      </Button>
                    )}
                    <Button
                      size="sm"
                      icon="add"
                      onClick={triggerAddConnection}
                    >
                      {isCompatible ? "Add API Key" : (providerId === "iflow" ? "OAuth" : "Add Connection")}
                    </Button>
                  </>
                )}
              </Box>
            </Box>
          ) : (
            <>
              {oneByOneSummary && (
                <Box
                  sx={{
                    mb: 2,
                    borderRadius: 2,
                    border: 1,
                    borderColor: dividerSoft,
                    bgcolor: panelSubtle,
                    px: 1.5,
                    py: 1,
                    fontSize: 12,
                    color: "text.secondary",
                  }}
                >
                  <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 1.5 }}>
                    <Box component="span">Total: {oneByOneSummary.total}</Box>
                    <Box component="span">Completed: {oneByOneSummary.completed}</Box>
                    <Box component="span">Passed: {oneByOneSummary.passed}</Box>
                    <Box component="span">Failed: {oneByOneSummary.failed}</Box>
                    {oneByOneSummary.stopped && (
                      <Box component="span" sx={{ color: (theme) => (theme.palette.mode === "dark" ? "#fbbf24" : "#d97706") }}>Stopped</Box>
                    )}
                    {oneByOneRunning && oneByOneCurrentConnectionId && (
                      <Box component="span">Running: {connections.find((conn) => conn.id === oneByOneCurrentConnectionId)?.name || oneByOneCurrentConnectionId}</Box>
                    )}
                  </Box>
                </Box>
              )}
              {connections.length > 0 && (
                <Box
                  sx={{
                    mb: 1.5,
                    display: "flex",
                    alignItems: "center",
                    gap: 1,
                    borderBottom: 1,
                    borderColor: dividerFaint,
                    pb: 1,
                  }}
                >
                  <Box
                    component="label"
                    sx={{
                      display: "flex",
                      cursor: "pointer",
                      alignItems: "center",
                      gap: 0.75,
                      fontSize: 12,
                      color: "text.secondary",
                      transition: "color 150ms ease",
                      "&:hover": { color: "primary.main" },
                    }}
                  >
                    <Checkbox
                      checked={allSelected}
                      onChange={toggleSelectAllConnections}
                      size="small"
                      sx={{ p: 0.25 }}
                    />
                    Select All
                  </Box>
                </Box>
              )}
              {connectionsList}
              {!isCompatible && (
                <Box sx={{ mt: 2, display: { xs: "grid", sm: "flex" }, gridTemplateColumns: { xs: "1fr" }, gap: 1 }}>
                  {providerId === "iflow" && (
                    <Button
                      size="sm"
                      icon="cookie"
                      variant="secondary"
                      onClick={() => setShowIFlowCookieModal(true)}
                      title="Add connection using browser cookie"
                    >
                      Cookie
                    </Button>
                  )}
                  {providerId === "codex" && (
                    <Button
                      size="sm"
                      icon="playlist_add"
                      variant="secondary"
                      onClick={() => setShowBulkImportCodex(true)}
                      title={translate("Bulk import codex accounts from JSON")}
                    >
                      {translate("Bulk Add")}
                    </Button>
                  )}
                  {providerId === "grok-cli" && (
                    <Button
                      size="sm"
                      icon="playlist_add"
                      variant="secondary"
                      onClick={() => setShowBulkImportGrokCli(true)}
                      title={translate("Bulk import Grok CLI accounts from JSON")}
                    >
                      {translate("Bulk Add")}
                    </Button>
                  )}
                  {hasDualAuthModes ? (
                    <>
                      <Button
                        size="sm"
                        icon="lock"
                        variant="secondary"
                        onClick={triggerOAuthConnection}
                      >
                        {oauthConnectionLabel}
                      </Button>
                      <Button
                        size="sm"
                        icon="key"
                        onClick={triggerApiKeyConnection}
                      >
                        {apiKeyConnectionLabel}
                      </Button>
                    </>
                  ) : (
                    <Button
                      size="sm"
                      icon="add"
                      onClick={triggerAddConnection}
                    >
                      Add
                    </Button>
                  )}
                </Box>
              )}
            </>
          )}
        </Card>
      )}

      {/* Per-provider user overrides (custom headers / connect timeout) */}
      <CustomConfigCard providerId={providerId} />

      {/* Models */}
      <Card>
        <Box
          sx={{
            mb: 2,
            display: "flex",
            flexDirection: { xs: "column", sm: "row" },
            gap: 1,
            alignItems: { sm: "center" },
            justifyContent: { sm: "space-between" },
          }}
        >
          <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
            <Typography component="h2" sx={{ fontSize: 18, fontWeight: 600 }}>
              {"Available Models"}
            </Typography>
            {deletableModelRows.length > 0 && (
              <Box
                component="label"
                sx={{
                  display: "flex",
                  cursor: "pointer",
                  userSelect: "none",
                  alignItems: "center",
                  gap: 1,
                  fontSize: 12,
                  color: "text.secondary",
                  transition: "color 150ms ease",
                  "&:hover": { color: "primary.main" },
                }}
              >
                <Checkbox
                  checked={allDeletableSelected}
                  indeterminate={selectedModelIds.length > 0 && !allDeletableSelected}
                  onChange={toggleSelectAllModels}
                  size="small"
                  sx={{ p: 0.25 }}
                  inputProps={{ "aria-label": "Select all added models" }}
                />
                <Box component="span">{selectedModelIds.length > 0 ? `${selectedModelIds.length} selected` : `Select all (${deletableModelRows.length})`}</Box>
              </Box>
            )}
            {selectedModelIds.length > 0 && (
              <>
                <Button
                  size="sm"
                  variant="danger"
                  icon="delete"
                  disabled={bulkDeleteBusy}
                  loading={bulkDeleteBusy}
                  onClick={handleBulkDeleteModels}
                >
                  {`Remove (${selectedModelIds.length})`}
                </Button>
                <Button size="sm" variant="ghost" disabled={bulkDeleteBusy} onClick={() => setSelectedModelIds([])}>
                  Clear
                </Button>
              </>
            )}
            {providerThinkingLevels && (
              <Box
                component="select"
                value={thinkingMode}
                onChange={(e) => handleThinkingModeChange(e.target.value)}
                title="Appends (level) suffix to copied model names"
                sx={{
                  borderRadius: 1,
                  border: 1,
                  borderColor: "divider",
                  bgcolor: "background.default",
                  color: "text.primary",
                  px: 1,
                  py: 0.5,
                  fontSize: 12,
                  outline: "none",
                  "&:focus": { borderColor: "primary.main" },
                  "& option": { bgcolor: "background.paper", color: "text.primary" },
                }}
              >
                {providerThinkingLevels.map((opt) => (
                  <option key={opt} value={opt}>{`Thinking: ${opt.charAt(0).toUpperCase() + opt.slice(1)}`}</option>
                ))}
              </Box>
            )}
            {!isCompatible && (
              <Box sx={{ position: "relative" }} ref={importMenuRef}>
                <Button
                  size="sm"
                  variant="secondary"
                  icon="download"
                  loading={importingModels}
                  onClick={() => setShowImportMenu((prev) => !prev)}
                >
                  {translate("Import models")}
                  <Box component="span" className="material-symbols-outlined" sx={{ ml: 0.5, fontSize: 18 }}>
                    {showImportMenu ? "expand_less" : "expand_more"}
                  </Box>
                </Button>
                {showImportMenu && (
                  <Box
                    sx={{
                      position: "absolute",
                      left: { xs: 0, sm: "auto" },
                      right: { sm: 0 },
                      top: "100%",
                      zIndex: 50,
                      mt: 0.5,
                      minWidth: 210,
                      borderRadius: 3,
                      border: 1,
                      borderColor: dividerSoft,
                      bgcolor: "background.paper",
                      p: 0.5,
                      boxShadow: 6,
                    }}
                  >
                    <Box
                      component="button"
                      type="button"
                      onClick={() => { setShowImportMenu(false); handleImportProviderModels("all"); }}
                      sx={{
                        display: "flex",
                        width: "100%",
                        alignItems: "center",
                        gap: 1,
                        whiteSpace: "nowrap",
                        borderRadius: 2,
                        px: 1.5,
                        py: 1,
                        textAlign: "left",
                        fontSize: 14,
                        border: 0,
                        bgcolor: "transparent",
                        color: "text.primary",
                        cursor: "pointer",
                        transition: "background-color 150ms ease",
                        "&:hover": { bgcolor: hoverSubtle },
                      }}
                    >
                      <Box component="span" className="material-symbols-outlined" sx={{ flexShrink: 0, fontSize: 20, lineHeight: 1, color: "primary.main" }}>download</Box>
                      <Box component="span" sx={{ minWidth: 0, flex: 1, overflow: "hidden", textOverflow: "ellipsis" }}>{"Import all"}</Box>
                    </Box>
                    <Box
                      component="button"
                      type="button"
                      onClick={() => { setShowImportMenu(false); handleImportProviderModels("batch"); }}
                      sx={{
                        display: "flex",
                        width: "100%",
                        alignItems: "center",
                        gap: 1,
                        whiteSpace: "nowrap",
                        borderRadius: 2,
                        px: 1.5,
                        py: 1,
                        textAlign: "left",
                        fontSize: 14,
                        border: 0,
                        bgcolor: "transparent",
                        color: "text.primary",
                        cursor: "pointer",
                        transition: "background-color 150ms ease",
                        "&:hover": { bgcolor: hoverSubtle },
                      }}
                    >
                      <Box component="span" className="material-symbols-outlined" sx={{ flexShrink: 0, fontSize: 20, lineHeight: 1, color: "#10b981" }}>checklist</Box>
                      <Box component="span" sx={{ minWidth: 0, flex: 1, overflow: "hidden", textOverflow: "ellipsis" }}>{"Import batch"}</Box>
                    </Box>
                  </Box>
                )}
              </Box>
            )}
          </Box>
          {!isCompatible && (() => {
            const allIds = [
              ...models,
              ...kiloFreeModels.filter((fm) => !models.some((m) => m.id === fm.id)),
            ].filter((m) => { const k = getModelKind(m); return !k || k === "llm"; }).map((m) => m.id);
            const activeIds = allIds.filter((id) => !disabledModelIds.includes(id));
            return (
              <Box sx={{ display: "flex", gap: 1 }}>
                {disabledModelIds.length > 0 && (
                  <Button size="sm" variant="secondary" icon="restart_alt" onClick={handleEnableAll}>
                    Active All
                  </Button>
                )}
                {activeIds.length > 0 && (
                  <Button size="sm" variant="secondary" icon="block" onClick={() => handleDisableAll(activeIds)}>
                    Disable All
                  </Button>
                )}
              </Box>
            );
          })()}
        </Box>
        {!!modelsTestError && (
          <Typography sx={{ fontSize: 12, color: "#ef4444", mb: 1.5, wordBreak: "break-word" }}>{modelsTestError}</Typography>
        )}
        {renderModelsSection()}
      </Card>

      {bulkActionModal}

      {/* Modals */}
      {providerId === "kiro" ? (
        <KiroOAuthWrapper
          isOpen={showOAuthModal}
          providerInfo={providerInfo}
          onSuccess={handleOAuthSuccess}
          onClose={() => setShowOAuthModal(false)}
        />
      ) : providerId === "cursor" ? (
        <CursorAuthModal
          isOpen={showOAuthModal}
          onSuccess={handleOAuthSuccess}
          onClose={() => setShowOAuthModal(false)}
        />
      ) : providerId === "zed" ? (
        <ZedAuthModal
          isOpen={showOAuthModal}
          providerInfo={providerInfo}
          onSuccess={handleOAuthSuccess}
          onClose={() => setShowOAuthModal(false)}
        />
      ) : providerId === "gitlab" ? (
        <GitLabAuthModal
          isOpen={showOAuthModal}
          providerInfo={providerInfo}
          onSuccess={handleOAuthSuccess}
          onClose={() => setShowOAuthModal(false)}
        />
      ) : (
        <OAuthModal
          isOpen={showOAuthModal}
          provider={providerId}
          providerInfo={providerInfo}
          onSuccess={handleOAuthSuccess}
          onClose={() => setShowOAuthModal(false)}
        />
      )}
      {providerId === "iflow" && (
        <IFlowCookieModal
          isOpen={showIFlowCookieModal}
          onSuccess={handleIFlowCookieSuccess}
          onClose={() => setShowIFlowCookieModal(false)}
        />
      )}
      <AddApiKeyModal
        isOpen={showAddApiKeyModal}
        provider={providerId}
        providerName={providerInfo.name}
        isCompatible={isCompatible}
        isAnthropic={isAnthropicCompatible}
        authType={providerInfo?.authType}
        authHint={providerInfo?.authHint}
        website={providerInfo?.website}
        proxyPools={proxyPools}
        error={addConnectionError}
        existingNames={connections.map((c) => c.name).filter(Boolean)}
        onSave={handleSaveApiKey}
        onBulkDone={fetchConnections}
        onClose={() => {
          setAddConnectionError("");
          setShowAddApiKeyModal(false);
        }}
      />
      <EditConnectionModal
        isOpen={showEditModal}
        connection={selectedConnection}
        proxyPools={proxyPools}
        onSave={handleUpdateConnection}
        onClose={() => setShowEditModal(false)}
      />
      {isCompatible && (
        <EditCompatibleNodeModal
          isOpen={showEditNodeModal}
          node={providerNode}
          onSave={handleUpdateNode}
          onClose={() => setShowEditNodeModal(false)}
          isAnthropic={isAnthropicCompatible}
        />
      )}
      {!isCompatible && (
        <AddCustomModelModal
          isOpen={showAddCustomModel}
          providerAlias={providerStorageAlias}
          providerDisplayAlias={providerDisplayAlias}
          onSave={async (modelId, caps, transport) => {
            // caps.stt is a UI-only flag; the API accepts transports only on
            // type "stt" records, so the save derives the type from it.
            await handleAddCustomModel(modelId, caps?.stt ? "stt" : "llm", providerStorageAlias, caps, transport);
            setShowAddCustomModel(false);
          }}
          onClose={() => setShowAddCustomModel(false)}
        />
      )}

      {providerId === "codex" && (
        <BulkImportCodexModal
          isOpen={showBulkImportCodex}
          onClose={() => setShowBulkImportCodex(false)}
          onSuccess={fetchConnections}
        />
      )}

      {providerId === "grok-cli" && (
        <BulkImportGrokCliModal
          isOpen={showBulkImportGrokCli}
          onClose={() => setShowBulkImportGrokCli(false)}
          onSuccess={fetchConnections}
        />
      )}

      {importPreview && (
        <ModelImportModal
          models={importPreview}
          onClose={() => setImportPreview(null)}
          onConfirm={confirmModelImport}
          onTestConnection={async () => {
            const activeConnection = connections.find((conn) => conn.isActive !== false);
            if (!activeConnection) throw new Error(translate("Please add an active connection first"));
            const res = await fetch(`/api/providers/${activeConnection.id}/test`, { method: "POST" });
            const data = await res.json();
            if (!res.ok || !data.valid) throw new Error(data.error || "Connection failed");
            return translate("Connection OK");
          }}
        />
      )}

      {/* AG Risk Confirmation Modal */}
      <ConfirmModal
        isOpen={showAgRiskModal}
        onClose={() => setShowAgRiskModal(false)}
        onConfirm={handleAgRiskConfirm}
        title="Risk Notice"
        message={providerInfo?.deprecationNotice}
        confirmText="I Understand, Continue"
        cancelText="Cancel"
        variant="danger"
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

      {/* Notice Modal — informational success/error/info messages */}
      <Modal
        isOpen={!!noticeState}
        onClose={() => setNoticeState(null)}
        title={noticeState?.title || "Notice"}
        size="sm"
        footer={
          <>
            <Button variant={noticeState?.variant === "success" ? "success" : noticeState?.variant === "error" ? "danger" : "primary"} onClick={() => setNoticeState(null)}>
              OK
            </Button>
          </>
        }
      >
        <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1.5 }}>
          {noticeState?.variant === "success" && (
            <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 20, color: "#22c55e", mt: 0.25, flexShrink: 0 }}>check_circle</Box>
          )}
          {noticeState?.variant === "error" && (
            <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 20, color: "#ef4444", mt: 0.25, flexShrink: 0 }}>error</Box>
          )}
          {noticeState?.variant === "info" && (
            <Box component="span" className="material-symbols-outlined" sx={{ fontSize: 20, color: "#3b82f6", mt: 0.25, flexShrink: 0 }}>info</Box>
          )}
          <Typography sx={{ fontSize: 14, color: "text.secondary", lineHeight: 1.6, wordBreak: "break-word" }}>{noticeState?.message}</Typography>
        </Box>
      </Modal>
    </Box>
  );
}
