"use client";

import { useState, useCallback, useEffect } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import TextField from "@mui/material/TextField";
import { Card, Button, Modal } from "@/shared/components";
import { getModelsByProviderId, getModelKind } from "@/shared/constants/models";
import { getProviderAlias } from "@/shared/constants/providers";
import { useCopyToClipboard } from "@/shared/hooks/useCopyToClipboard";

const tooltipSx = {
  pointerEvents: "none",
  position: "absolute",
  mt: 0.5,
  top: 20,
  left: "50%",
  transform: "translateX(-50%)",
  fontSize: 10,
  color: "text.secondary",
  whiteSpace: "nowrap",
  opacity: 0,
  transition: "opacity 150ms ease",
};

function HoverHint({ children }) {
  return <Box component="span" className="mc-tooltip" sx={tooltipSx}>{children}</Box>;
}

// ── ModelRow ───────────────────────────────────────────────────
export function ModelRow({ model, fullModel, copied, onCopy, testStatus, isCustom, isFree, onDeleteAlias, onTest, isTesting }) {
  const borderColor = testStatus === "ok" ? "rgba(34,197,94,0.4)" : testStatus === "error" ? "rgba(239,68,68,0.4)" : "divider";
  const iconColor = testStatus === "ok" ? "#22c55e" : testStatus === "error" ? "#ef4444" : undefined;

  return (
    <Box
      sx={{
        px: 1.5,
        py: 1,
        borderRadius: "8px",
        border: 1,
        borderColor,
        transition: "background-color 150ms ease",
        "&:hover": { bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(30,30,30,0.5)" : "rgba(244,241,236,0.5)") },
        "&:hover .mc-hover": { opacity: 1 },
      }}
    >
      <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
        <Icon
          className="material-symbols-outlined"
          baseClassName=""
          sx={{ fontSize: 16, color: iconColor }}
        >
          {testStatus === "ok" ? "check_circle" : testStatus === "error" ? "cancel" : "smart_toy"}
        </Icon>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
          <Box component="code" sx={{ fontSize: 12, color: "text.secondary", fontFamily: "monospace", bgcolor: "var(--color-sidebar)", px: 0.75, py: 0.25, borderRadius: "4px" }}>{fullModel}</Box>
          {model.name && <Box component="span" sx={{ fontSize: 9, color: (t) => (t.palette.mode === "dark" ? "rgba(156,163,175,0.7)" : "rgba(107,114,128,0.7)"), fontStyle: "italic", pl: 0.5 }}>{model.name}</Box>}
        </Box>
        {onTest && (
          <Box sx={{ position: "relative", "&:hover .mc-tooltip": { opacity: 1 } }}>
            <Box
              component="button"
              onClick={onTest}
              disabled={isTesting}
              className={isTesting ? undefined : "mc-hover"}
              sx={{
                p: 0.25,
                border: 0,
                bgcolor: "transparent",
                borderRadius: "4px",
                color: "text.secondary",
                cursor: "pointer",
                opacity: isTesting ? 1 : 0,
                transition: "opacity 150ms ease, background-color 150ms ease, color 150ms ease",
                "&:hover": { bgcolor: "var(--color-sidebar)", color: "primary.main" },
              }}
            >
              <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 14, animation: isTesting ? "spin 1s linear infinite" : undefined }}>
                {isTesting ? "progress_activity" : "science"}
              </Icon>
            </Box>
            <HoverHint>{isTesting ? "Testing..." : "Test"}</HoverHint>
          </Box>
        )}
        <Box sx={{ position: "relative", "&:hover .mc-tooltip": { opacity: 1 } }}>
          <Box
            component="button"
            onClick={() => onCopy(fullModel, `model-${model.id}`)}
            sx={{
              p: 0.25,
              border: 0,
              bgcolor: "transparent",
              borderRadius: "4px",
              color: "text.secondary",
              cursor: "pointer",
              transition: "background-color 150ms ease, color 150ms ease",
              "&:hover": { bgcolor: "var(--color-sidebar)", color: "primary.main" },
            }}
          >
            <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 14 }}>{copied === `model-${model.id}` ? "check" : "content_copy"}</Icon>
          </Box>
          <HoverHint>{copied === `model-${model.id}` ? "Copied!" : "Copy"}</HoverHint>
        </Box>
        {isFree && <Box component="span" sx={{ fontSize: 10, fontWeight: 700, color: "#22c55e", bgcolor: "rgba(34,197,94,0.1)", px: 0.75, py: 0.25, borderRadius: "4px" }}>FREE</Box>}
        {isCustom && (
          <Box
            component="button"
            onClick={onDeleteAlias}
            className="mc-hover"
            sx={{
              p: 0.25,
              ml: "auto",
              border: 0,
              bgcolor: "transparent",
              borderRadius: "4px",
              color: "text.secondary",
              cursor: "pointer",
              opacity: 0,
              transition: "opacity 150ms ease, background-color 150ms ease, color 150ms ease",
              "&:hover": { bgcolor: "rgba(239,68,68,0.1)", color: "#ef4444" },
            }}
            title="Remove custom model"
          >
            <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 14 }}>close</Icon>
          </Box>
        )}
      </Box>
    </Box>
  );
}

// ── AddCustomModelModal ────────────────────────────────────────
function AddCustomModelModal({ isOpen, onSave, onClose }) {
  const [modelId, setModelId] = useState("");

  const handleSave = () => {
    if (!modelId.trim()) return;
    onSave(modelId.trim());
    setModelId("");
  };

  return (
    <Modal isOpen={isOpen} title="Add Custom Model" onClose={onClose}>
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Box>
          <Typography component="label" sx={{ display: "block", fontSize: 12, color: "text.secondary", mb: 0.5 }}>Model ID</Typography>
          <TextField
            value={modelId}
            onChange={(e) => setModelId(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && handleSave()}
            placeholder="e.g. tts-1-hd"
            autoFocus
            fullWidth
            size="small"
          />
        </Box>
        <Box sx={{ display: "flex", gap: 1 }}>
          <Button onClick={handleSave} fullWidth disabled={!modelId.trim()}>Add</Button>
          <Button onClick={onClose} variant="ghost" fullWidth>Cancel</Button>
        </Box>
      </Box>
    </Modal>
  );
}


// ── ModelsCard ─────────────────────────────────────────────────
// Self-contained card: shows models for a provider, filtered by optional `kindFilter`.
// kindFilter: if provided, only shows models with matching type/kinds field.
export default function ModelsCard({ providerId, kindFilter, providerAliasOverride }) {
  const { copied, copy } = useCopyToClipboard();
  const [modelAliases, setModelAliases] = useState({});
  const [customModels, setCustomModels] = useState([]);
  const [modelTestResults, setModelTestResults] = useState({});
  const [testingModelId, setTestingModelId] = useState(null);
  const [testError, setTestError] = useState("");
  const [showAddCustomModel, setShowAddCustomModel] = useState(false);

  const providerAlias = providerAliasOverride || getProviderAlias(providerId);
  const effectiveType = kindFilter || "llm";

  const fetchData = useCallback(async () => {
    try {
      const [aliasRes, customRes] = await Promise.all([
        fetch("/api/models/alias"),
        fetch("/api/models/custom", { cache: "no-store" }),
      ]);
      const aliasData = await aliasRes.json();
      const customData = await customRes.json();
      if (aliasRes.ok) setModelAliases(aliasData.aliases || {});
      if (customRes.ok) setCustomModels(customData.models || []);
    } catch (e) { console.log("ModelsCard fetch error:", e); }
  }, []);

  useEffect(() => { /* eslint-disable-next-line react-hooks/set-state-in-effect -- bootstrap fetch on mount and when providerId changes. */
    fetchData(); }, [fetchData]);

  const handleSetAlias = async (modelId, alias) => {
    const fullModel = `${providerAlias}/${modelId}`;
    try {
      const res = await fetch("/api/models/alias", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ model: fullModel, alias }),
      });
      if (res.ok) await fetchData();
    } catch (e) { console.log("set alias error:", e); }
  };

  const handleDeleteAlias = async (alias) => {
    try {
      const res = await fetch(`/api/models/alias?alias=${encodeURIComponent(alias)}`, { method: "DELETE" });
      if (res.ok) await fetchData();
    } catch (e) { console.log("delete alias error:", e); }
  };

  const handleAddCustomModel = async (modelId) => {
    try {
      const res = await fetch("/api/models/custom", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ providerAlias, id: modelId, type: effectiveType }),
      });
      if (res.ok) {
        await fetchData();
        window.dispatchEvent(new CustomEvent("customModelChanged"));
      }
    } catch (e) { console.log("add custom model error:", e); }
  };

  const handleDeleteCustomModel = async (modelId) => {
    try {
      const params = new URLSearchParams({ providerAlias, id: modelId, type: effectiveType });
      const res = await fetch(`/api/models/custom?${params}`, { method: "DELETE" });
      if (res.ok) {
        await fetchData();
        window.dispatchEvent(new CustomEvent("customModelChanged"));
      }
    } catch (e) { console.log("delete custom model error:", e); }
  };

  const handleTestModel = async (modelId) => {
    if (testingModelId) return;
    setTestingModelId(modelId);
    try {
      const res = await fetch("/api/models/test", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ model: `${providerAlias}/${modelId}`, kind: kindFilter }),
      });
      const data = await res.json();
      setModelTestResults((prev) => ({ ...prev, [modelId]: data.ok ? "ok" : "error" }));
      setTestError(data.ok ? "" : (data.error || "Model not reachable"));
    } catch {
      setModelTestResults((prev) => ({ ...prev, [modelId]: "error" }));
      setTestError("Network error");
    } finally { setTestingModelId(null); }
  };

  // Built-in models — filter by kindFilter if provided
  const allBuiltIn = getModelsByProviderId(providerId);
  const builtInModels = kindFilter
    ? allBuiltIn.filter((m) => {
        if (m.kinds) return m.kinds.includes(kindFilter);
        return getModelKind(m, "llm") === kindFilter;
      })
    : allBuiltIn;

  // Custom models for this provider + kind, dedupe vs built-in
  const myCustomModels = customModels.filter(
    (m) => m.providerAlias === providerAlias
      && getModelKind(m, "llm") === effectiveType
      && !builtInModels.some((b) => b.id === m.id)
  );

  const displayModels = builtInModels;

  return (
    <>
      <Card>
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", mb: 2 }}>
          <Typography component="h2" sx={{ fontSize: 18, fontWeight: 600 }}>Models{kindFilter ? ` — ${kindFilter.toUpperCase()}` : ""}</Typography>
        </Box>
        {testError && <Typography sx={{ fontSize: 12, color: "error.main", mb: 1.5, wordBreak: "break-word" }}>{testError}</Typography>}

        <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1.5 }}>
          {displayModels.map((model) => {
            const fullModel = `${providerAlias}/${model.id}`;
            const existingAlias = Object.entries(modelAliases).find(([, m]) => m === fullModel)?.[0];
            return (
              <ModelRow
                key={model.id}
                model={model}
                fullModel={`${providerAlias}/${model.id}`}
                alias={existingAlias}
                copied={copied}
                onCopy={copy}
                onSetAlias={(alias) => handleSetAlias(model.id, alias)}
                onDeleteAlias={() => handleDeleteAlias(existingAlias)}
                testStatus={modelTestResults[model.id]}
                onTest={() => handleTestModel(model.id)}
                isTesting={testingModelId === model.id}
                isFree={model.isFree}
              />
            );
          })}

          {myCustomModels.map((model) => (
            <ModelRow
              key={`${model.id}-${model.type}`}
              model={{ id: model.id, name: model.name }}
              fullModel={`${providerAlias}/${model.id}`}
              copied={copied}
              onCopy={copy}
              onSetAlias={() => {}}
              onDeleteAlias={() => handleDeleteCustomModel(model.id)}
              testStatus={modelTestResults[model.id]}
              onTest={() => handleTestModel(model.id)}
              isTesting={testingModelId === model.id}
              isCustom
            />
          ))}

          <Box
            component="button"
            onClick={() => setShowAddCustomModel(true)}
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 0.75,
              px: 1.5,
              py: 1,
              borderRadius: "8px",
              border: "1px dashed",
              borderColor: (t) => (t.palette.mode === "dark" ? "rgba(255,255,255,0.15)" : "rgba(0,0,0,0.15)"),
              bgcolor: "transparent",
              fontSize: 12,
              color: "text.secondary",
              cursor: "pointer",
              transition: "color 150ms ease, border-color 150ms ease",
              "&:hover": { color: "primary.main", borderColor: "rgba(229,106,74,0.4)" },
            }}
          >
            <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 14 }}>add</Icon>
            Add Model
          </Box>
        </Box>
      </Card>

      <AddCustomModelModal
        isOpen={showAddCustomModel}
        onSave={async (modelId) => {
          await handleAddCustomModel(modelId);
          setShowAddCustomModel(false);
        }}
        onClose={() => setShowAddCustomModel(false)}
      />
    </>
  );
}
