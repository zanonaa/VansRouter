"use client";

import { useState } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import TextField from "@mui/material/TextField";
import { Button } from "@/shared/components";
import { getProviderCustomModelRows } from "@/shared/utils/providerCustomModels";

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

function IconButtonLike({ sx, ...props }) {
  return (
    <Box
      component="button"
      sx={{
        p: 0.25,
        border: 0,
        bgcolor: "transparent",
        borderRadius: "4px",
        color: "text.secondary",
        cursor: "pointer",
        transition: "background-color 150ms ease, color 150ms ease",
        "&:hover": { bgcolor: "var(--color-sidebar)", color: "primary.main" },
        ...sx,
      }}
      {...props}
    />
  );
}

function PassthroughModelRow({ modelId, fullModel, copied, onCopy, onDeleteAlias, onTest, testStatus, isTesting }) {
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
          <Box sx={{ position: "relative", "&:hover .pm-tooltip": { opacity: 1 } }}>
            <IconButtonLike
              onClick={() => onCopy(fullModel, `model-${modelId}`)}
            >
              <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 14 }}>
                {copied === `model-${modelId}` ? "check" : "content_copy"}
              </Icon>
            </IconButtonLike>
            <Box component="span" className="pm-tooltip" sx={tooltipSx}>
              {copied === `model-${modelId}` ? "Copied!" : "Copy"}
            </Box>
          </Box>
          {onTest && (
            <Box sx={{ position: "relative", "&:hover .pm-tooltip": { opacity: 1 } }}>
              <IconButtonLike
                onClick={onTest}
                disabled={isTesting}
              >
                <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 14, animation: isTesting ? "spin 1s linear infinite" : undefined }}>
                  {isTesting ? "progress_activity" : "science"}
                </Icon>
              </IconButtonLike>
              <Box component="span" className="pm-tooltip" sx={tooltipSx}>
                {isTesting ? "Testing..." : "Test"}
              </Box>
            </Box>
          )}
        </Box>
      </Box>

      {/* Delete button */}
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


export default function PassthroughModelsSection({ providerAlias, modelAliases, customModels, copied, onCopy, onDeleteAlias, onAddCustomModel, onDeleteCustomModel }) {
  const [newModel, setNewModel] = useState("");
  const [adding, setAdding] = useState(false);

  const allModels = getProviderCustomModelRows({
    customModels,
    modelAliases,
    providerAlias,
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

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Typography sx={{ fontSize: 14, color: "text.secondary" }}>
        OpenRouter supports any model. Add models and create aliases for quick access.
      </Typography>

      {/* Add new model */}
      <Box sx={{ display: "flex", alignItems: "flex-end", gap: 1 }}>
        <Box sx={{ flex: 1 }}>
          <Typography component="label" htmlFor="new-model-input" sx={{ display: "block", fontSize: 12, color: "text.secondary", mb: 0.5 }}>Model ID (from OpenRouter)</Typography>
          <TextField
            id="new-model-input"
            type="text"
            fullWidth
            size="small"
            value={newModel}
            onChange={(e) => setNewModel(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && handleAdd()}
            placeholder="anthropic/claude-3-opus"
          />
        </Box>
        <Button size="sm" icon="add" onClick={handleAdd} disabled={!newModel.trim() || adding}>
          {adding ? "Adding..." : "Add"}
        </Button>
      </Box>

      {/* Models list */}
      {allModels.length > 0 && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          {allModels.map(({ id, fullModel, alias, source }) => (
            <PassthroughModelRow
              key={`${source}-${fullModel}`}
              modelId={id}
              fullModel={fullModel}
              copied={copied}
              onCopy={onCopy}
              onDeleteAlias={() => source === "custom" ? onDeleteCustomModel(id) : onDeleteAlias(alias)}
            />
          ))}
        </Box>
      )}
    </Box>
  );
}
