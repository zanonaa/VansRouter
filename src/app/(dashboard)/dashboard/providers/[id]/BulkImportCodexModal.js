"use client";

import { useState } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import { Button, Modal } from "@/shared/components";
import { translate } from "@/i18n/runtime";

const PLACEHOLDER = `[
  {
    "accessToken": "eyJhbGc...",
    "refreshToken": "rt_...",
    "idToken": "eyJhbGc...",
    "email": "user@example.com"
  }
]`;

function normalizeToArray(parsed) {
  if (Array.isArray(parsed)) return parsed;
  if (parsed && typeof parsed === "object") {
    if (Array.isArray(parsed.accounts)) return parsed.accounts;
    return [parsed];
  }
  return null;
}

export default function BulkImportCodexModal({ isOpen, onClose, onSuccess }) {
  const [jsonText, setJsonText] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [parseError, setParseError] = useState("");
  const [result, setResult] = useState(null);

  const handleClose = () => {
    if (submitting) return;
    setJsonText("");
    setParseError("");
    setResult(null);
    onClose();
  };

  const handleSubmit = async () => {
    setParseError("");
    setResult(null);

    const trimmed = jsonText.trim();
    if (!trimmed) return;

    let parsed;
    try {
      parsed = JSON.parse(trimmed);
    } catch (err) {
      setParseError(`${translate("Invalid JSON")}: ${err.message}`);
      return;
    }

    const accounts = normalizeToArray(parsed);
    if (!accounts || accounts.length === 0) {
      setParseError(translate("No accounts found in input"));
      return;
    }

    setSubmitting(true);
    try {
      const res = await fetch("/api/oauth/codex/bulk-import", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ accounts }),
      });
      const data = await res.json();
      if (!res.ok) {
        setParseError(data?.error || `Request failed: ${res.status}`);
        return;
      }
      setResult(data);
      if (data.success > 0 && typeof onSuccess === "function") {
        onSuccess();
      }
    } catch (err) {
      setParseError(err.message || translate("Request failed"));
    } finally {
      setSubmitting(false);
    }
  };

  const failedItems = result?.results?.filter((r) => !r.ok) || [];

  return (
    <Modal isOpen={isOpen} title={translate("Bulk Add Codex Accounts")} onClose={handleClose}>
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
          {translate(
            "Paste an array of codex account JSON objects. Each must include accessToken (and ideally refreshToken, idToken)."
          )}
        </Typography>

        <Box
          component="textarea"
          sx={{
            width: "100%",
            borderRadius: "4px",
            border: 1,
            borderColor: "rgba(229,106,74,0.3)",
            bgcolor: "var(--color-sidebar)",
            p: 1,
            fontSize: 14,
            fontFamily: "monospace",
            resize: "vertical",
            minHeight: 240,
            "&:focus": { outline: "none", boxShadow: "0 0 0 1px var(--color-primary)" },
          }}
          placeholder={PLACEHOLDER}
          value={jsonText}
          onChange={(e) => setJsonText(e.target.value)}
          disabled={submitting}
        />

        {parseError && (
          <Typography sx={{ fontSize: 12, color: "error.main", wordBreak: "break-word" }}>{parseError}</Typography>
        )}

        {result && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            <Box
              sx={{
                fontSize: 14,
                fontWeight: 500,
                color: result.failed > 0 ? "#facc15" : "#4ade80",
              }}
            >
              ✓ {result.success} {translate("added")}
              {result.failed > 0 ? `, ✗ ${result.failed} ${translate("failed")}` : ""}
            </Box>
            {failedItems.length > 0 && (
              <Box component="ul" sx={{ borderRadius: "4px", border: 1, borderColor: "rgba(229,106,74,0.2)", bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(30,30,30,0.5)" : "rgba(244,241,236,0.5)"), p: 1, fontSize: 12, fontFamily: "monospace", maxHeight: 160, overflowY: "auto", m: 0, listStyle: "none" }}>
                {failedItems.map((item) => (
                  <Box component="li" key={item.index} sx={{ color: "#f87171" }}>
                    [{item.index}] {item.error}
                  </Box>
                ))}
              </Box>
            )}
          </Box>
        )}

        <Box sx={{ display: "flex", gap: 1 }}>
          <Button
            onClick={handleSubmit}
            fullWidth
            disabled={submitting || !jsonText.trim()}
          >
            {submitting ? translate("Importing...") : translate("Import All")}
          </Button>
          <Button onClick={handleClose} variant="ghost" fullWidth disabled={submitting}>
            {translate("Close")}
          </Button>
        </Box>
      </Box>
    </Modal>
  );
}
