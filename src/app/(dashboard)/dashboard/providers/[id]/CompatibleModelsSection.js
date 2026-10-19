"use client";

import { useState, useEffect, useRef } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import TextField from "@mui/material/TextField";
import { Button } from "@/shared/components";
import ModelImportModal from "@/shared/components/ModelImportModal.js";
import { getProviderCustomModelRows } from "@/shared/utils/providerCustomModels";

const tooltipSx = {
  pointerEvents: "none",
  position: "absolute",
  top: 20,
  left: "50%",
  transform: "translateX(-50%)",
  fontSize: 10,
  color: "text.secondary",
  whiteSpace: "nowrap",
  opacity: 0,
  transition: "opacity 150ms ease",
};

const iconButtonSx = {
  p: 0.25,
  border: 0,
  bgcolor: "transparent",
  borderRadius: "4px",
  color: "text.secondary",
  cursor: "pointer",
  transition: "background-color 150ms ease, color 150ms ease",
  "&:hover": { bgcolor: "var(--color-sidebar)", color: "primary.main" },
};

function CompatibleModelRow({ modelId, fullModel, copied, onCopy, onDeleteAlias, onTest, testStatus, isTesting }) {
  const borderColor = testStatus === "ok"
    ? "rgba(34,197,94,0.4)"
    : testStatus === "error"
    ? "rgba(239,68,68,0.4)"
    : "divider";

  const iconColor = testStatus === "ok"
    ? "#22c55e"
    : testStatus === "error"
    ? "#ef4444"
    : undefined;

  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        gap: 1.5,
        p: 1.5,
        borderRadius: "8px",
        border: 1,
        borderColor,
        transition: "background-color 150ms ease",
        "&:hover": { bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(30,30,30,0.5)" : "rgba(244,241,236,0.5)") },
      }}
    >
      <Icon
        className="material-symbols-outlined"
        baseClassName=""
        sx={{ fontSize: 16, color: iconColor || "text.secondary" }}
      >
        {testStatus === "ok" ? "check_circle" : testStatus === "error" ? "cancel" : "smart_toy"}
      </Icon>
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography sx={{ fontSize: 14, fontWeight: 500, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{modelId}</Typography>
        <Box sx={{ display: "flex", alignItems: "center", gap: 0.5, mt: 0.5 }}>
          <Box component="code" sx={{ fontSize: 12, color: "text.secondary", fontFamily: "monospace", bgcolor: "var(--color-sidebar)", px: 0.75, py: 0.25, borderRadius: "4px" }}>{fullModel}</Box>
          <Box sx={{ position: "relative", "&:hover .cm-tooltip": { opacity: 1 } }}>
            <Box
              component="button"
              onClick={() => onCopy(fullModel, `model-${modelId}`)}
              sx={iconButtonSx}
            >
              <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 14 }}>
                {copied === `model-${modelId}` ? "check" : "content_copy"}
              </Icon>
            </Box>
            <Box component="span" className="cm-tooltip" sx={tooltipSx}>
              {copied === `model-${modelId}` ? "Copied!" : "Copy"}
            </Box>
          </Box>
          {onTest && (
            <Box sx={{ position: "relative", "&:hover .cm-tooltip": { opacity: 1 } }}>
              <Box
                component="button"
                onClick={onTest}
                disabled={isTesting}
                sx={iconButtonSx}
              >
                <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 14, animation: isTesting ? "spin 1s linear infinite" : undefined }}>
                  {isTesting ? "progress_activity" : "science"}
                </Icon>
              </Box>
              <Box component="span" className="cm-tooltip" sx={tooltipSx}>
                {isTesting ? "Testing..." : "Test"}
              </Box>
            </Box>
          )}
        </Box>
      </Box>
      <Box
        component="button"
        onClick={onDeleteAlias}
        sx={{
          p: 0.5,
          border: 0,
          bgcolor: "transparent",
          borderRadius: "4px",
          color: "#ef4444",
          cursor: "pointer",
          transition: "background-color 150ms ease",
          "&:hover": { bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(239,68,68,0.15)" : "#fef2f2") },
        }}
        title="Remove model"
      >
        <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 14 }}>delete</Icon>
      </Box>
    </Box>
  );
}

export default function CompatibleModelsSection({ providerStorageAlias, providerDisplayAlias, modelAliases, customModels, copied, onCopy, onDeleteAlias, onAddCustomModel, onDeleteCustomModel, connections, isAnthropic, onImportModels }) {
  const [newModel, setNewModel] = useState("");
  const [adding, setAdding] = useState(false);
  const [importing, setImporting] = useState(false);
  const [importPreview, setImportPreview] = useState(null);
  const [showImportMenu, setShowImportMenu] = useState(false);
  const importMenuRef = useRef(null);

  useEffect(() => {
    if (!showImportMenu) return;
    const handleClick = (e) => {
      if (importMenuRef.current && !importMenuRef.current.contains(e.target)) setShowImportMenu(false);
    };
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [showImportMenu]);
  const [testingModelId, setTestingModelId] = useState(null);
  const [modelTestResults, setModelTestResults] = useState({});

  const handleTestModel = async (modelId) => {
    if (testingModelId) return;
    setTestingModelId(modelId);
    try {
      const res = await fetch("/api/models/test", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ model: `${providerStorageAlias}/${modelId}` }),
      });
      const data = await res.json();
      setModelTestResults((prev) => ({ ...prev, [modelId]: data.ok ? "ok" : "error" }));
    } catch {
      setModelTestResults((prev) => ({ ...prev, [modelId]: "error" }));
    } finally {
      setTestingModelId(null);
    }
  };

  const allModels = getProviderCustomModelRows({
    customModels,
    modelAliases,
    providerAlias: providerStorageAlias,
    type: "llm",
  });

  const handleAdd = async () => {
    if (!newModel.trim() || adding) return;
    const modelId = newModel.trim();
    if (allModels.some((model) => model.id === modelId)) {
      alert("Model already exists for this provider.");
      return;
    }

    setAdding(true);
    try {
      await onAddCustomModel(modelId);
      setNewModel("");
    } catch (error) {
      console.log("Error adding model:", error);
    } finally {
      setAdding(false);
    }
  };

  const handleImport = async (mode = "batch") => {
    if (importing) return;
    const activeConnection = connections.find((conn) => conn.isActive !== false);
    if (!activeConnection) return;

    setImporting(true);
    try {
      const res = await fetch(`/api/providers/${activeConnection.id}/models`);
      const data = await res.json();
      if (!res.ok) {
        alert(data.error || "Failed to import models");
        return;
      }
      const models = data.models || [];
      if (models.length === 0) {
        alert("No models returned from /models.");
        return;
      }
      const seen = new Set(allModels.map((entry) => entry.id));
      const candidates = [];
      for (const model of models) {
        const id = model.id || model.name || model.model;
        if (typeof id !== "string" || !id.trim() || seen.has(id)) continue;
        seen.add(id);
        candidates.push({ id, providerAlias: providerStorageAlias, type: "llm" });
      }
      if (candidates.length === 0) alert("No new models were added.");
      else if (mode === "all") await onImportModels(candidates);
      else setImportPreview(candidates);
    } catch (error) {
      console.log("Error importing models:", error);
    } finally {
      setImporting(false);
    }
  };

  const confirmImport = async (selected) => {
    await onImportModels(selected);
  };

  const canImport = connections.some((conn) => conn.isActive !== false);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {importPreview && (
        <ModelImportModal
          models={importPreview}
          onClose={() => setImportPreview(null)}
          onConfirm={confirmImport}
          onTestConnection={async () => {
            const activeConnection = connections.find((conn) => conn.isActive !== false);
            if (!activeConnection) throw new Error("Add a connection first");
            const res = await fetch(`/api/providers/${activeConnection.id}/test`, { method: "POST" });
            const data = await res.json();
            if (!res.ok || !data.valid) throw new Error(data.error || "Connection failed");
            return "Connection OK";
          }}
        />
      )}
      <Typography sx={{ fontSize: 14, color: "text.secondary" }}>
        Add {isAnthropic ? "Anthropic" : "OpenAI"}-compatible models manually or import them from the /models endpoint.
      </Typography>

      <Box sx={{ display: "flex", alignItems: "flex-end", gap: 1, flexWrap: "wrap" }}>
        <Box sx={{ flex: 1, minWidth: 240 }}>
          <Typography component="label" htmlFor="new-compatible-model-input" sx={{ display: "block", fontSize: 12, color: "text.secondary", mb: 0.5 }}>Model ID</Typography>
          <TextField
            id="new-compatible-model-input"
            type="text"
            fullWidth
            size="small"
            value={newModel}
            onChange={(e) => setNewModel(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && handleAdd()}
            placeholder={isAnthropic ? "claude-3-opus-20240229" : "gpt-4o"}
          />
        </Box>
        <Button size="sm" icon="add" onClick={handleAdd} disabled={!newModel.trim() || adding}>
          {adding ? "Adding..." : "Add"}
        </Button>
        <Box sx={{ position: "relative" }} ref={importMenuRef}>
          <Button size="sm" variant="secondary" icon="download" onClick={() => setShowImportMenu((prev) => !prev)} disabled={!canImport || importing}>
            {importing ? "Importing..." : "Import from /models"}
            <Icon className="material-symbols-outlined" baseClassName="" sx={{ ml: 0.5, fontSize: 18 }}>
              {showImportMenu ? "expand_less" : "expand_more"}
            </Icon>
          </Button>
          {showImportMenu && (
            <Box sx={{ position: "absolute", left: 0, top: "100%", zIndex: 50, mt: 0.5, minWidth: 210, borderRadius: "12px", border: 1, borderColor: (t) => (t.palette.mode === "dark" ? "rgba(255,255,255,0.1)" : "rgba(0,0,0,0.1)"), bgcolor: "background.paper", p: 0.5, boxShadow: "0 20px 25px -5px rgba(0,0,0,0.1)", "@media (min-width:600px)": { left: "auto", right: 0 } }}>
              <Box
                component="button"
                type="button"
                onClick={() => { setShowImportMenu(false); handleImport("all"); }}
                sx={{ display: "flex", width: "100%", alignItems: "center", gap: 1, whiteSpace: "nowrap", borderRadius: "8px", px: 1.5, py: 1, textAlign: "left", border: 0, bgcolor: "transparent", cursor: "pointer", fontSize: 14, color: "text.primary", transition: "background-color 150ms ease", "&:hover": { bgcolor: "action.hover" } }}
              >
                <Icon className="material-symbols-outlined" baseClassName="" sx={{ flexShrink: 0, fontSize: 20, lineHeight: 1, color: "primary.main" }}>download</Icon>
                <Box component="span" sx={{ minWidth: 0, flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{"Import all"}</Box>
              </Box>
              <Box
                component="button"
                type="button"
                onClick={() => { setShowImportMenu(false); handleImport("batch"); }}
                sx={{ display: "flex", width: "100%", alignItems: "center", gap: 1, whiteSpace: "nowrap", borderRadius: "8px", px: 1.5, py: 1, textAlign: "left", border: 0, bgcolor: "transparent", cursor: "pointer", fontSize: 14, color: "text.primary", transition: "background-color 150ms ease", "&:hover": { bgcolor: "action.hover" } }}
              >
                <Icon className="material-symbols-outlined" baseClassName="" sx={{ flexShrink: 0, fontSize: 20, lineHeight: 1, color: "#10b981" }}>checklist</Icon>
                <Box component="span" sx={{ minWidth: 0, flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{"Import batch"}</Box>
              </Box>
            </Box>
          )}
        </Box>
      </Box>

      {!canImport && (
        <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
          Add a connection to enable importing models.
        </Typography>
      )}

      {allModels.length > 0 && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          {allModels.map(({ id, alias, source }) => (
            <CompatibleModelRow
              key={`${source}-${providerStorageAlias}/${id}`}
              modelId={id}
              fullModel={`${providerDisplayAlias}/${id}`}
              copied={copied}
              onCopy={onCopy}
              onDeleteAlias={() => source === "custom" ? onDeleteCustomModel(id) : onDeleteAlias(alias)}
              onTest={connections.length > 0 ? () => handleTestModel(id) : undefined}
              testStatus={modelTestResults[id]}
              isTesting={testingModelId === id}
            />
          ))}
        </Box>
      )}
    </Box>
  );
}
