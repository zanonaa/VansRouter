"use client";

import { useState } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Modal from "./Modal";
import Button from "./Button";

// Mounted only while a catalog preview is open, so cancel/reopen resets selection.
export default function ModelImportModal({ models, onClose, onConfirm, onTestConnection }) {
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState(new Set());
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [test, setTest] = useState({ state: "idle", message: "" });
  const visible = models.filter((m) => `${m.id} ${m.name || ""}`.toLowerCase().includes(query.toLowerCase()));
  const toggle = (key) => setSelected((prev) => {
    const next = new Set(prev);
    if (next.has(key)) next.delete(key); else next.add(key);
    return next;
  });
  const close = () => { if (!busy) onClose(); };
  const runTest = async () => {
    if (!onTestConnection || test.state === "running") return;
    setTest({ state: "running", message: "" });
    try {
      const message = await onTestConnection();
      setTest({ state: "ok", message: message || "Connection OK" });
    } catch (err) {
      setTest({ state: "error", message: err.message || "Connection failed" });
    }
  };
  const confirm = async () => {
    setBusy(true);
    setError("");
    try {
      await onConfirm(models.filter((_, i) => selected.has(i)));
      onClose();
    } catch (err) {
      setError(err.message || "Failed to import models");
    } finally { setBusy(false); }
  };
  return (
    <Modal isOpen onClose={close} title="Select models to import" size="xl" footer={<>
      <Button variant="secondary" onClick={close} disabled={busy}>Close</Button>
      <Button onClick={confirm} loading={busy} disabled={busy || selected.size === 0}>Import {selected.size} models</Button>
    </>}>
      <Box
        component="input"
        aria-label="Search models"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="Search models"
        sx={{
          width: "100%",
          borderRadius: "8px",
          border: "1px solid var(--color-border)",
          bgcolor: "background.default",
          p: 1,
          color: "text.primary",
          outline: "none",
          "&:focus": { borderColor: "primary.main" },
        }}
      />
      <Box sx={{ my: 1.5, display: "flex", flexWrap: "wrap", alignItems: "center", gap: 1.5 }}>
        <Button size="sm" variant="secondary" disabled={busy} onClick={() => setSelected((prev) => new Set([...prev, ...visible.map((m) => models.indexOf(m))]))}>Select all</Button>
        <Button size="sm" variant="secondary" disabled={busy} onClick={() => setSelected(new Set())}>Clear</Button>
        {onTestConnection && (
          <Button size="sm" variant="secondary" icon="science" loading={test.state === "running"} disabled={busy} onClick={runTest}>Test connection</Button>
        )}
        <Box component="span" sx={{ fontSize: 12, color: "text.secondary" }}>{selected.size} selected / {models.length} available</Box>
      </Box>
      {test.state === "ok" && <Typography sx={{ mb: 1.5, fontSize: 12, color: "success.main" }}>{test.message}</Typography>}
      {test.state === "error" && <Typography role="alert" sx={{ mb: 1.5, fontSize: 12, color: "error.main" }}>{test.message}</Typography>}
      {error && <Typography role="alert" sx={{ mb: 1.5, color: "error.main" }}>{error}</Typography>}
      <Box sx={{ maxHeight: 320, overflowY: "auto", display: "flex", flexDirection: "column", gap: 1 }}>
        {visible.map((model) => {
          const key = models.indexOf(model);
          return (
            <Box
              component="label"
              key={key}
              sx={{ display: "flex", gap: 1.5, alignItems: "center", borderRadius: "8px", border: "1px solid var(--color-border)", p: 1, cursor: "pointer" }}
            >
              <input type="checkbox" disabled={busy} checked={selected.has(key)} onChange={() => toggle(key)} />
              <Box component="span" sx={{ minWidth: 0, wordBreak: "break-all", fontSize: 14 }}>{model.id}</Box>
            </Box>
          );
        })}
        {visible.length === 0 && <Typography sx={{ color: "text.secondary" }}>No matching models</Typography>}
      </Box>
    </Modal>
  );
}
