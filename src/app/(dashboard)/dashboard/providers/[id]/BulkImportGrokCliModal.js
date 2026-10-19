"use client";

import { useState, useRef } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import { Modal, Button } from "@/shared/components";
import { translate } from "@/i18n/runtime";

const PLACEHOLDER = `[
  {
    "access_token": "eyJ0eXAiOiJhdCtqd3Qi...",
    "refresh_token": "LZhriF9bf88pPykpXCuZ9...",
    "id_token": "eyJ0eXAiOiJKV1QiLCJhbGci...",
    "email": "account1@example.com"
  },
  {
    "access_token": "eyJ0eXAiOiJhdCtqd3Qi...",
    "refresh_token": "LZhriF9bf88pPykpXCuZ9...",
    "id_token": "eyJ0eXAiOiJKV1QiLCJhbGci...",
    "email": "account2@example.com"
  }
]`;

function parseAccountsInput(rawText) {
  const trimmed = rawText.trim();
  if (!trimmed) return [];

  let parsed;
  try {
    parsed = JSON.parse(trimmed);
  } catch (initialErr) {
    // If direct parse failed, try handling concatenated or comma-separated JSON objects
    try {
      let fixed = trimmed;
      if (!fixed.startsWith("[")) {
        fixed = fixed.replace(/\}\s*,\s*\{/g, "},{").replace(/\}\s*\{/g, "},{");
        if (fixed.endsWith(",")) fixed = fixed.slice(0, -1);
        fixed = `[${fixed}]`;
      }
      parsed = JSON.parse(fixed);
    } catch {
      throw initialErr;
    }
  }

  if (Array.isArray(parsed)) {
    return parsed;
  }
  if (parsed && typeof parsed === "object") {
    if (Array.isArray(parsed.accounts)) return parsed.accounts;
    return [parsed];
  }

  throw new Error("Input must be a JSON object or array of objects");
}

export default function BulkImportGrokCliModal({ isOpen, onClose, onSuccess }) {
  const [jsonText, setJsonText] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [parseError, setParseError] = useState("");
  const [result, setResult] = useState(null);
  const [isDragging, setIsDragging] = useState(false);
  const [fileCountInfo, setFileCountInfo] = useState(null);
  const fileInputRef = useRef(null);

  const handleClose = () => {
    if (submitting) return;
    setJsonText("");
    setParseError("");
    setResult(null);
    setFileCountInfo(null);
    setIsDragging(false);
    onClose();
  };

  const processFiles = async (files) => {
    if (!files || files.length === 0) return;
    setParseError("");
    const jsonFiles = Array.from(files).filter(
      (file) => file.name.endsWith(".json") || file.type === "application/json" || file.type === ""
    );

    if (jsonFiles.length === 0) {
      setParseError(translate("Please select valid .json files"));
      return;
    }

    try {
      const allAccounts = [];
      for (const file of jsonFiles) {
        const text = await file.text();
        const accountsFromFile = parseAccountsInput(text);
        if (Array.isArray(accountsFromFile)) {
          allAccounts.push(...accountsFromFile);
        } else if (accountsFromFile) {
          allAccounts.push(accountsFromFile);
        }
      }

      if (allAccounts.length === 0) {
        setParseError(translate("No accounts found in selected files"));
        return;
      }

      setJsonText(JSON.stringify(allAccounts, null, 2));
      setFileCountInfo({
        filesCount: jsonFiles.length,
        accountsCount: allAccounts.length,
      });
    } catch (err) {
      setParseError(`${translate("Error reading files")}: ${err.message}`);
    }
  };

  const handleFileInputChange = (e) => {
    processFiles(e.target.files);
    if (e.target) e.target.value = "";
  };

  const handleDragOver = (e) => {
    e.preventDefault();
    setIsDragging(true);
  };

  const handleDragLeave = (e) => {
    e.preventDefault();
    setIsDragging(false);
  };

  const handleDrop = (e) => {
    e.preventDefault();
    setIsDragging(false);
    if (e.dataTransfer?.files?.length > 0) {
      processFiles(e.dataTransfer.files);
    }
  };

  const handleSubmit = async () => {
    setParseError("");
    setResult(null);

    let accounts;
    try {
      accounts = parseAccountsInput(jsonText);
    } catch (err) {
      setParseError(`${translate("Invalid JSON")}: ${err.message}`);
      return;
    }

    if (!accounts || accounts.length === 0) {
      setParseError(translate("No accounts found in input"));
      return;
    }

    setSubmitting(true);
    try {
      const res = await fetch("/api/oauth/grok-cli/bulk-import", {
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
    <Modal isOpen={isOpen} title={translate("Bulk Add Grok CLI Accounts")} onClose={handleClose}>
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
          <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
            {translate("Upload multiple .json files or paste JSON array / object.")}
          </Typography>
          <Box
            component="input"
            ref={fileInputRef}
            type="file"
            accept=".json,application/json"
            multiple
            sx={{ display: "none" }}
            onChange={handleFileInputChange}
          />
          <Button
            type="button"
            size="sm"
            variant="secondary"
            icon="upload_file"
            onClick={() => fileInputRef.current?.click()}
            disabled={submitting}
          >
            {translate("Upload JSON Files")}
          </Button>
        </Box>

        <Box
          onDragOver={handleDragOver}
          onDragLeave={handleDragLeave}
          onDrop={handleDrop}
          sx={{
            position: "relative",
            borderRadius: "4px",
            border: 1,
            borderColor: isDragging ? "primary.main" : "rgba(229,106,74,0.3)",
            bgcolor: isDragging ? "rgba(229,106,74,0.1)" : "var(--color-sidebar)",
            transition: "border-color 150ms ease, background-color 150ms ease",
            boxShadow: isDragging ? "0 0 0 2px rgba(229,106,74,0.3)" : "none",
          }}
        >
          <Box
            component="textarea"
            sx={{
              width: "100%",
              borderRadius: "4px",
              bgcolor: "transparent",
              p: 1.25,
              fontSize: 14,
              fontFamily: "monospace",
              resize: "vertical",
              minHeight: 240,
              border: 0,
              color: "inherit",
              "&:focus": { outline: "none", boxShadow: "0 0 0 1px var(--color-primary)" },
            }}
            placeholder={PLACEHOLDER}
            value={jsonText}
            onChange={(e) => {
              setJsonText(e.target.value);
              setFileCountInfo(null);
            }}
            disabled={submitting}
          />

          {isDragging && (
            <Box sx={{ position: "absolute", inset: 0, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(30,30,30,0.9)" : "rgba(244,241,236,0.9)"), borderRadius: "4px", pointerEvents: "none", backdropFilter: "blur(4px)" }}>
              <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 30, color: "primary.main", mb: 0.5 }}>upload_file</Icon>
              <Box component="span" sx={{ fontSize: 14, fontWeight: 500, color: "primary.main" }}>
                {translate("Drop .json files here")}
              </Box>
            </Box>
          )}
        </Box>

        {fileCountInfo && (
          <Box sx={{ display: "flex", alignItems: "center", gap: 0.75, fontSize: 12, color: "#4ade80", fontWeight: 500, bgcolor: "rgba(34,197,94,0.1)", border: 1, borderColor: "rgba(34,197,94,0.2)", px: 1.25, py: 0.75, borderRadius: "4px" }}>
            <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 14 }}>check_circle</Icon>
            <Box component="span">
              {translate("Loaded")} {fileCountInfo.accountsCount} {translate("account(s) from")}{" "}
              {fileCountInfo.filesCount} {translate("file(s)")}
            </Box>
          </Box>
        )}

        {parseError && (
          <Typography sx={{ fontSize: 12, color: "error.main", wordBreak: "break-word" }}>{parseError}</Typography>
        )}

        {result && result.failed > 0 && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            <Box sx={{ fontSize: 14, fontWeight: 500, color: "#facc15" }}>
              ✗ {result.failed} {translate("failed")}
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
