"use client";

import { useEffect, useReducer, useRef } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import { Modal, Button, Input } from "@/shared/components";

function cursorReducer(state, action) {
  switch (action.type) {
    case "SET_TOKEN": return { ...state, accessToken: action.value };
    case "SET_MACHINE_ID": return { ...state, machineId: action.value };
    case "AUTO_DETECT_START": return { ...state, autoDetecting: true, error: null, autoDetected: false, windowsManual: false };
    case "AUTO_DETECT_DONE": return { ...state, autoDetecting: false, autoDetected: action.found || false, accessToken: action.accessToken || state.accessToken, machineId: action.machineId || state.machineId, windowsManual: action.windowsManual || false, error: action.error || null };
    case "IMPORT_START": return { ...state, importing: true, error: null };
    case "IMPORT_DONE": return { ...state, importing: false, error: action.error || null };
    case "ERROR": return { ...state, error: action.error };
    default: return state;
  }
}

/**
 * Cursor Auth Modal
 * Auto-detect and import token from Cursor IDE's local SQLite database
 */
export default function CursorAuthModal({ isOpen, onSuccess, onClose }) {
  const [state, dispatch] = useReducer(cursorReducer, { accessToken: "", machineId: "", error: null, importing: false, autoDetecting: false, autoDetected: false, windowsManual: false });
  const { accessToken, machineId, error, importing, autoDetecting, autoDetected, windowsManual } = state;
  const flowIdRef = useRef(0);

  const runAutoDetect = async () => {
    const flowId = ++flowIdRef.current;
    dispatch({ type: "AUTO_DETECT_START" });
    try {
      const res = await fetch("/api/oauth/cursor/auto-import");
      const data = await res.json();
      if (flowId !== flowIdRef.current || !isOpen) return;

      if (data.found) {
        dispatch({ type: "AUTO_DETECT_DONE", found: true, accessToken: data.accessToken, machineId: data.machineId });
      } else if (data.windowsManual) {
        dispatch({ type: "AUTO_DETECT_DONE", windowsManual: true });
      } else {
        dispatch({ type: "AUTO_DETECT_DONE", error: data.error || "Could not auto-detect tokens" });
      }
    } catch (err) {
      if (flowId === flowIdRef.current && isOpen) {
        dispatch({ type: "AUTO_DETECT_DONE", error: "Failed to auto-detect tokens" });
      }
    }
  };

  // Auto-detect tokens when modal opens
  useEffect(() => {
    if (!isOpen) {
      flowIdRef.current += 1;
      return;
    }
    runAutoDetect();
  }, [isOpen]);

  const handleImportToken = async () => {
    if (!accessToken.trim()) {
      dispatch({ type: "ERROR", error: "Please enter an access token" });
      return;
    }

    if (!machineId.trim()) {
      dispatch({ type: "ERROR", error: "Please enter a machine ID" });
      return;
    }

    const flowId = flowIdRef.current;
    dispatch({ type: "IMPORT_START" });

    try {
      const res = await fetch("/api/oauth/cursor/import", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          accessToken: accessToken.trim(),
          machineId: machineId.trim(),
        }),
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(data.error || "Import failed");
      }
      if (flowId !== flowIdRef.current || !isOpen) return;

      onSuccess?.();
      onClose();
    } catch (err) {
      if (flowId === flowIdRef.current && isOpen) {
        dispatch({ type: "IMPORT_DONE", error: err.message });
      }
    }
  };

  const noteSx = (bg, border, iconColor, textColor) => ({
    bgcolor: bg,
    p: 1.5,
    borderRadius: "8px",
    border: `1px solid ${border}`,
    display: "flex",
    gap: 1,
    alignItems: "flex-start",
  });

  return (
    <Modal isOpen={isOpen} title="Connect Cursor IDE" onClose={onClose}>
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {/* Auto-detecting state */}
        {autoDetecting && (
          <Box sx={{ textAlign: "center", py: 3 }}>
            <Box sx={{ width: 64, height: 64, mx: "auto", mb: 2, borderRadius: "50%", bgcolor: "rgba(229,106,74,0.1)", display: "flex", alignItems: "center", justifyContent: "center" }}>
              <span className="material-symbols-outlined" style={{ animation: "spin 1s linear infinite",  fontSize: 30, color: "var(--color-primary)" }}>
                progress_activity
              </span>
            </Box>
            <Typography variant="h6" sx={{ fontSize: 18, fontWeight: 600, mb: 1 }}>Auto-detecting tokens...</Typography>
            <Typography sx={{ fontSize: 14, color: "text.secondary" }}>
              Reading from Cursor IDE database
            </Typography>
          </Box>
        )}

        {/* Form (shown after auto-detect completes) */}
        {!autoDetecting && (
          <>
            {/* Success message if auto-detected */}
            {autoDetected && (
              <Box sx={noteSx("rgba(34,197,94,0.1)", "rgba(34,197,94,0.3)", "success.main", "success.main")}>
                <span className="material-symbols-outlined" style={{ color: "#16a34a" }}>check_circle</span>
                <Typography sx={{ fontSize: 14, color: "#166534" }}>
                  Tokens auto-detected from Cursor IDE successfully!
                </Typography>
              </Box>
            )}

            {/* Windows manual instructions */}
            {windowsManual && (
              <Box sx={{ ...noteSx("rgba(245,158,11,0.1)", "rgba(245,158,11,0.3)"), flexDirection: "column" }}>
                <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
                  <span className="material-symbols-outlined" style={{ color: "#d97706" }}>info</span>
                  <Typography sx={{ fontSize: 14, fontWeight: 500, color: "#92400e" }}>
                    Could not read Cursor database automatically.
                  </Typography>
                </Box>
                <Typography sx={{ fontSize: 12, color: "#b45309" }}>
                  Make sure Cursor IDE has been opened at least once, then click <strong>Retry</strong>. If the problem persists, paste your tokens manually below.
                </Typography>
                <Button onClick={runAutoDetect} variant="outline" fullWidth>
                  Retry
                </Button>
              </Box>
            )}

            {/* Info message if not auto-detected */}
            {!autoDetected && !windowsManual && !error && (
              <Box sx={noteSx("rgba(59,130,246,0.1)", "rgba(59,130,246,0.3)", "info.main", "info.main")}>
                <span className="material-symbols-outlined" style={{ color: "#2563eb" }}>info</span>
                <Typography sx={{ fontSize: 14, color: "#1e40af" }}>
                  Cursor IDE not detected. Please paste your tokens manually.
                </Typography>
              </Box>
            )}

            {/* Access Token Input */}
            <Box>
              <Box component="label" htmlFor="cursor-access-token" sx={{ display: "block", fontSize: 14, fontWeight: 500, mb: 1 }}>
                Access Token <Box component="span" sx={{ color: "error.main" }}>*</Box>
              </Box>
              <Box
                component="textarea"
                id="cursor-access-token"
                value={accessToken}
                onChange={(e) => dispatch({ type: "SET_TOKEN", value: e.target.value })}
                placeholder="Access token will be auto-filled..."
                rows={3}
                sx={{
                  width: "100%",
                  px: 1.5,
                  py: 1,
                  fontSize: 14,
                  fontFamily: "ui-monospace, monospace",
                  border: "1px solid var(--color-border)",
                  borderRadius: "8px",
                  bgcolor: "background.default",
                  color: "text.primary",
                  resize: "none",
                  outline: "none",
                  "&:focus": { borderColor: "primary.main" },
                }}
              />
            </Box>

            {/* Machine ID Input */}
            <Box>
              <Box component="label" htmlFor="cursor-machine-id" sx={{ display: "block", fontSize: 14, fontWeight: 500, mb: 1 }}>
                Machine ID <Box component="span" sx={{ color: "error.main" }}>*</Box>
              </Box>
              <Input
                id="cursor-machine-id"
                value={machineId}
                onChange={(e) => dispatch({ type: "SET_MACHINE_ID", value: e.target.value })}
                placeholder="Machine ID will be auto-filled..."
                inputClassName="font-mono text-sm"
              />
            </Box>

            {/* Error Display */}
            {error && (
              <Box sx={noteSx("rgba(239,68,68,0.1)", "rgba(239,68,68,0.3)", "error.main", "error.main")}>
                <Typography sx={{ fontSize: 14, color: "#dc2626" }}>{error}</Typography>
              </Box>
            )}

            {/* Action Buttons */}
            <Box sx={{ display: "flex", gap: 1 }}>
              <Button
                onClick={handleImportToken}
                fullWidth
                disabled={importing || !accessToken.trim() || !machineId.trim()}
              >
                {importing ? "Importing..." : "Import Token"}
              </Button>
              <Button onClick={onClose} variant="ghost" fullWidth>
                Cancel
              </Button>
            </Box>
          </>
        )}
      </Box>
    </Modal>
  );
}
