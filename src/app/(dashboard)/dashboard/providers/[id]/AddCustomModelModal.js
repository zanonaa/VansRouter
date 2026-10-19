"use client";

import { useState, useReducer } from "react";
import PropTypes from "prop-types";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import TextField from "@mui/material/TextField";
import { Button, Modal, Select, Toggle } from "@/shared/components";
import { CAPACITY_META, STT_TRANSPORT_META, STT_TRANSPORTS } from "@/shared/constants/models";

const defaultCaps = () => Object.fromEntries(Object.keys(CAPACITY_META).map((key) => [key, false]));


function modalReducer(state, action) {
  switch (action.type) {
    case "RESET": return { modelId: "", testStatus: null, testError: "", saving: false };
    case "SET_MODEL": return { ...state, modelId: action.value, testStatus: null, testError: "" };
    case "TEST_START": return { ...state, testStatus: "testing", testError: "" };
    case "TEST_RESULT": return { ...state, testStatus: action.ok ? "ok" : "error", testError: action.error || "" };
    case "SAVE_START": return { ...state, saving: true };
    case "SAVE_DONE": return { ...state, saving: false };
    default: return state;
  }
}

export default function AddCustomModelModal({ isOpen, providerAlias, providerDisplayAlias, onSave, onClose }) {
  const [state, dispatch] = useReducer(modalReducer, { modelId: "", testStatus: null, testError: "", saving: false });
  const { modelId, testStatus, testError, saving } = state;
  const [prevIsOpen, setPrevIsOpen] = useState(false);
  // Capability toggles (whitelisted keys from CAPACITY_META) and the realtime
  // dispatch marker for the transport select; "" = provider default REST.
  const [caps, setCaps] = useState(defaultCaps);
  const [transport, setTransport] = useState("");

  // Reset state when modal opens (prev-prop pattern — no extra render cycle)
  if (isOpen && !prevIsOpen) {
    setPrevIsOpen(true);
    dispatch({ type: "RESET" });
    setCaps(defaultCaps());
    setTransport("");
  } else if (!isOpen && prevIsOpen) {
    setPrevIsOpen(false);
  }


  // Strip provider's own alias prefix (e.g. "cc/model" -> "model" for cc provider)
  const stripAlias = (id) => {
    const prefix = `${providerAlias}/`;
    return id.startsWith(prefix) ? id.slice(prefix.length) : id;
  };

  const handleTest = async () => {
    const cleanId = stripAlias(modelId.trim());
    if (!cleanId) return;
    dispatch({ type: "TEST_START" });
    try {
      const res = await fetch("/api/models/test", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ model: `${providerAlias}/${cleanId}` }),
      });
      const data = await res.json();
      dispatch({ type: "TEST_RESULT", ok: data.ok, error: data.error });
    } catch (err) {
      dispatch({ type: "TEST_RESULT", ok: false, error: err.message });
    }
  };

  const handleSave = async () => {
    const cleanId = stripAlias(modelId.trim());
    if (!cleanId || saving) return;
    dispatch({ type: "SAVE_START" });
    try {
      // caps.stt is UI-only; the parent save flow derives the model type from
      // it and forwards the pinned transport (null unless the caller picked one).
      await onSave(cleanId, caps, caps.stt ? transport : null);
    } finally {
      dispatch({ type: "SAVE_DONE" });
    }
  };

  const handleKeyDown = (e) => {
    if (e.key === "Enter") handleTest();
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Add Custom Model">
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Box>
          <Typography component="label" htmlFor="add-custom-model-id" sx={{ display: "block", fontSize: 14, fontWeight: 500, mb: 0.75 }}>Model ID</Typography>
          <Box sx={{ display: "flex", gap: 1 }}>
            <TextField
              id="add-custom-model-id"
              type="text"
              size="small"
              value={modelId}
              onChange={(e) => dispatch({ type: "SET_MODEL", value: e.target.value })}
              onKeyDown={handleKeyDown}
              placeholder="e.g. claude-opus-4-5"
              sx={{ flex: 1 }}
            />
            <Button
              variant="secondary"
              icon="science"
              loading={testStatus === "testing"}
              onClick={handleTest}
              disabled={!modelId.trim() || testStatus === "testing"}
            >
              {testStatus === "testing" ? "Testing..." : "Test"}
            </Button>
          </Box>
          <Typography sx={{ fontSize: 12, color: "text.secondary", mt: 0.5 }}>
            Sent to provider as: <Box component="code" sx={{ fontFamily: "monospace", bgcolor: "var(--color-sidebar)", px: 0.5, borderRadius: "4px" }}>{stripAlias(modelId.trim()) || "model-id"}</Box>
          </Typography>
        </Box>

        <Box>
          <Typography component="label" sx={{ display: "block", fontSize: 14, fontWeight: 500, mb: 0.75 }}>Capabilities</Typography>
          <Box sx={{ display: "flex", flexWrap: "wrap", gap: 2 }}>
            {Object.entries(CAPACITY_META).map(([key, meta]) => (
              <Toggle
                key={key}
                checked={!!caps[key]}
                onChange={(v) => setCaps((prev) => ({ ...prev, [key]: v }))}
                label={meta.label}
                description={meta.desc}
                size="sm"
              />
            ))}
          </Box>
        </Box>

        {/* STT is a model TYPE, not a chat capability: the save flow turns this
            flag into type "stt" (the API honours a transport only on stt
            records). The select pins the realtime dispatch marker persisted
            with the model; the whitelist is the shared STT_TRANSPORT_META. */}
        <Box>
          <Toggle
            checked={!!caps.stt}
            onChange={(v) => { setCaps((prev) => ({ ...prev, stt: v })); if (!v) setTransport(""); }}
            label="Speech to text"
            description="Transcribes audio via /v1/audio/transcriptions"
            size="sm"
          />
          {caps.stt && (
            <Box sx={{ mt: 1.5 }}>
              <Select
                label="Transport"
                value={transport}
                onChange={(e) => setTransport(e.target.value)}
                placeholder="Provider default (REST)"
                options={STT_TRANSPORTS.map((t) => ({ value: t, label: STT_TRANSPORT_META[t].label }))}
                hint="Realtime transport marker for the STT dispatcher. Empty keeps the provider's REST format."
              />
            </Box>
          )}
        </Box>

        {/* Test result */}
        {testStatus === "ok" && (
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, fontSize: 14, color: "success.main" }}>
            <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 16 }}>check_circle</Icon>
            Model is reachable
          </Box>
        )}
        {testStatus === "error" && (
          <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1, fontSize: 14, color: "error.main" }}>
            <Icon className="material-symbols-outlined" baseClassName="" sx={{ flexShrink: 0, fontSize: 16 }}>cancel</Icon>
            <Box component="span">{testError || "Model not reachable"}</Box>
          </Box>
        )}

        <Box sx={{ display: "flex", gap: 1, pt: 0.5 }}>
          <Button onClick={onClose} variant="ghost" fullWidth size="sm">Cancel</Button>
          <Button
            onClick={handleSave}
            fullWidth
            size="sm"
            disabled={!modelId.trim() || saving}
          >
            {saving ? "Adding..." : "Add Model"}
          </Button>
        </Box>
      </Box>
    </Modal>
  );
}
