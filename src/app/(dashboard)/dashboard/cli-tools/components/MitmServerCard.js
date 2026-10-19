"use client";

import { useState, useEffect, useCallback, useReducer } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import OutlinedInput from "@mui/material/OutlinedInput";
import Dialog from "@mui/material/Dialog";
import DialogTitle from "@mui/material/DialogTitle";
import DialogContent from "@mui/material/DialogContent";
import DialogActions from "@mui/material/DialogActions";
import MuiButton from "@mui/material/Button";
import { Card, Button, Badge, Input } from "@/shared/components";

const DEFAULT_MITM_ROUTER_BASE = "http://localhost:20128";

function MsIcon({ children, sx }) {
  return (
    <Icon className="material-symbols-outlined" baseClassName="" sx={sx}>
      {children}
    </Icon>
  );
}

/**
 * Shared MITM infrastructure card — manages SSL cert + server start/stop.
 * DNS per-tool is handled separately in MitmToolCard.
 */
function MitmServerModals({ showPasswordModal, sudoPassword, dispatch, loading, modalError, handleConfirmPassword, port443Conflict, handleKillAndStart }) {
  return (
    <>
      {/* Password Modal */}
      <Dialog open={showPasswordModal} onClose={() => !loading && dispatch({ showPasswordModal: false })} maxWidth="xs" fullWidth>
        <DialogTitle sx={{ fontSize: 16, fontWeight: 600 }}>Sudo Password Required</DialogTitle>
        <DialogContent sx={{ display: "flex", flexDirection: "column", gap: 2, pt: "8px !important" }}>
          <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1.5, p: 1.5, bgcolor: "rgba(234,179,8,0.10)", border: 1, borderColor: "rgba(234,179,8,0.30)", borderRadius: "8px" }}>
            <MsIcon sx={{ color: "#eab308", fontSize: 20 }}>warning</MsIcon>
            <Typography sx={{ fontSize: 12, color: "text.secondary" }}>Required for SSL certificate and server startup</Typography>
          </Box>
          <Input
            type="password"
            placeholder="Enter sudo password"
            value={sudoPassword}
            onChange={(e) => dispatch({ sudoPassword: e.target.value })}
            onKeyDown={(e) => { if (e.key === "Enter" && !loading) handleConfirmPassword(); }}
          />
          {modalError && (
            <Box sx={{ display: "flex", alignItems: "center", gap: 1, px: 1, py: 0.75, borderRadius: "4px", fontSize: 12, bgcolor: "rgba(239,68,68,0.10)", color: "#dc2626" }}>
              <MsIcon sx={{ fontSize: 14 }}>error</MsIcon>
              <Box component="span">{modalError}</Box>
            </Box>
          )}
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button variant="ghost" size="sm" onClick={() => { dispatch({ showPasswordModal: false }); dispatch({ sudoPassword: "" }); dispatch({ modalError: null }); }} disabled={loading}>
            Cancel
          </Button>
          <Button variant="primary" size="sm" onClick={handleConfirmPassword} loading={loading}>
            Confirm
          </Button>
        </DialogActions>
      </Dialog>

      {/* Port 443 Conflict Modal */}
      <Dialog open={!!port443Conflict} onClose={() => !loading && dispatch({ port443Conflict: null })} maxWidth="sm" fullWidth>
        <DialogTitle sx={{ fontSize: 16, fontWeight: 600 }}>Port 443 Already In Use</DialogTitle>
        <DialogContent sx={{ display: "flex", flexDirection: "column", gap: 2, pt: "8px !important" }}>
          <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1.5, p: 1.5, bgcolor: "rgba(234,179,8,0.10)", border: 1, borderColor: "rgba(234,179,8,0.30)", borderRadius: "8px" }}>
            <MsIcon sx={{ color: "#eab308", fontSize: 20 }}>warning</MsIcon>
            <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5, fontSize: 12, color: "text.secondary" }}>
              <Box component="p" sx={{ m: 0 }}>Port 443 is currently used by another process:</Box>
              <Box component="p" sx={{ m: 0, fontFamily: "ui-monospace, monospace", color: "text.primary" }} data-i18n-skip="true">
                {port443Conflict?.owner?.name} (PID {port443Conflict?.owner?.pid})
              </Box>
              <Box component="p" sx={{ m: 0 }}>Kill this process to start MITM Server?</Box>
            </Box>
          </Box>
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button variant="ghost" size="sm" onClick={() => { dispatch({ port443Conflict: null }); dispatch({ loading: false }); }} disabled={loading}>
            Cancel
          </Button>
          <Button variant="primary" size="sm" onClick={handleKillAndStart} loading={loading}>
            Kill &amp; Start
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
}


export default function MitmServerCard({ apiKeys, cloudEnabled, onStatusChange }) {
  const [state, dispatch] = useReducer(
    (s, a) => ({ ...s, ...a }),
    { status: null, loading: false, showPasswordModal: false, sudoPassword: "", pendingAction: null, modalError: null, actionError: null, mitmRouterBaseUrl: DEFAULT_MITM_ROUTER_BASE, port443Conflict: null }
  );
  const { status, loading, showPasswordModal, sudoPassword, pendingAction, modalError, actionError, mitmRouterBaseUrl, port443Conflict } = state;
  const [selectedApiKey, setSelectedApiKey] = useState(() => apiKeys?.[0]?.key || "");

  const serverIsWindows = status?.isWin === true;
  const canRunWithoutPassword = serverIsWindows || status?.hasCachedPassword || status?.needsSudoPassword === false;
  const isAdmin = status?.isAdmin !== false;
  // No privilege: not admin/root AND (Win OR no cached sudo password)
  const noPrivilege = !isAdmin && (serverIsWindows || (!status?.hasCachedPassword && status?.needsSudoPassword !== false));

  const fetchStatus = useCallback(async () => {
    try {
      const res = await fetch("/api/cli-tools/antigravity-mitm");
      if (res.ok) {
        const data = await res.json();
        dispatch({ status: data });
        if (data.mitmRouterBaseUrl) {
          dispatch({ mitmRouterBaseUrl: data.mitmRouterBaseUrl });
        }
        onStatusChange?.(data);
      }
    } catch {
      dispatch({ status: { running: false, certExists: false, dnsStatus: {} } });
    }
  }, [onStatusChange]);

  useEffect(() => {
    queueMicrotask(() => {
      fetchStatus();
    });
  }, [fetchStatus]);

  const handleAction = (action) => {
    dispatch({ actionError: null });
    // Wait for status to load before deciding whether to show sudo modal
    if (!status) return;
    if (canRunWithoutPassword) {
      doAction(action, "");
    } else {
      dispatch({ pendingAction: action });
      dispatch({ showPasswordModal: true });
      dispatch({ modalError: null });
    }
  };

  const doAction = async (action, password, forceKillPort443 = false) => {
    dispatch({ loading: true });
    dispatch({ actionError: null });
    try {
      let res;
      if (action === "trust-cert") {
        res = await fetch("/api/cli-tools/antigravity-mitm", {
          method: "PATCH",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ action: "trust-cert", sudoPassword: password }),
        });
      } else if (action === "start") {
        const keyToUse = selectedApiKey?.trim()
          || (apiKeys?.length > 0 ? apiKeys[0].key : null)
          || (!cloudEnabled ? "sk_9router" : null);
        res = await fetch("/api/cli-tools/antigravity-mitm", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            apiKey: keyToUse,
            sudoPassword: password,
            mitmRouterBaseUrl: mitmRouterBaseUrl.trim() || DEFAULT_MITM_ROUTER_BASE,
            forceKillPort443,
          }),
        });
      } else {
        res = await fetch("/api/cli-tools/antigravity-mitm", {
          method: "DELETE",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ sudoPassword: password }),
        });
      }
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        if (data.code === "PORT_443_BUSY" && data.portOwner) {
          dispatch({ showPasswordModal: false });
          dispatch({ port443Conflict: { owner: data.portOwner, password } });
          return;
        }
        dispatch({ actionError: data.error || `Failed to ${action} MITM server` });
        return;
      }
      dispatch({ showPasswordModal: false });
      dispatch({ sudoPassword: "" });
      dispatch({ port443Conflict: null });
      await fetchStatus();
    } catch (e) {
      dispatch({ actionError: e.message || "Network error" });
    } finally {
      dispatch({ loading: false });
      dispatch({ pendingAction: null });
    }
  };

  const handleKillAndStart = () => {
    const pwd = port443Conflict?.password || "";
    doAction("start", pwd, true);
  };

  const handleConfirmPassword = () => {
    if (!sudoPassword.trim()) {
      dispatch({ modalError: "Sudo password is required" });
      return;
    }
    doAction(pendingAction, sudoPassword);
  };

  const isRunning = status?.running;
  const compactInputSx = {
    flex: 1,
    minWidth: 0,
    fontSize: 12,
    bgcolor: "background.paper",
    "& .MuiOutlinedInput-input": { py: 0.75, px: 1 },
    "& .MuiOutlinedInput-notchedOutline": { borderColor: "divider" },
  };

  return (
    <>
      <Card padding="sm" sx={{ borderColor: "rgba(229,106,74,0.20)", bgcolor: "rgba(229,106,74,0.05)" }}>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          {/* Header */}
          <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1, alignItems: { sm: "center" }, justifyContent: { sm: "space-between" } }}>
            <Box sx={{ display: "flex", minWidth: 0, flexWrap: "wrap", alignItems: "center", gap: 1 }}>
              <MsIcon sx={{ color: "primary.main", fontSize: 20 }}>security</MsIcon>
              <Typography component="span" sx={{ fontWeight: 600, fontSize: 14, color: "text.primary" }}>MITM Server</Typography>
              {isRunning ? (
                <Badge variant="success" size="sm">Running</Badge>
              ) : (
                <Badge variant="default" size="sm">Stopped</Badge>
              )}
            </Box>
            <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 0.5, fontSize: 12, color: "text.secondary" }} data-i18n-skip="true">
              {[
                { label: "Cert", ok: status?.certExists },
                { label: "Trusted", ok: status?.certTrusted },
                { label: "Server", ok: isRunning },
              ].map(({ label, ok }) => (
                <Box component="span" key={label} sx={{ display: "flex", alignItems: "center", gap: 0.25, px: 0.75, py: 0.25, borderRadius: "4px", color: ok ? "#16a34a" : "text.secondary" }}>
                  <MsIcon sx={{ fontSize: 12 }}>
                    {ok ? "check_circle" : "cancel"}
                  </MsIcon>
                  {label}
                </Box>
              ))}
            </Box>
          </Box>

          {/* Purpose & How it works */}
          <Box sx={{ px: 1, py: 1, borderRadius: "8px", bgcolor: "rgba(255,255,255,0.5)", border: 1, borderColor: "rgba(229,231,235,0.5)", display: "flex", flexDirection: "column", gap: 1 }}>
            <Typography sx={{ fontSize: 11, color: "text.secondary", lineHeight: 1.6 }}>
              <Box component="span" sx={{ fontWeight: 500, color: "text.primary" }}>Purpose:</Box> Use Antigravity IDE & GitHub Copilot → with ANY provider/model from VansAI
            </Typography>
            <Typography sx={{ fontSize: 11, color: "text.secondary", lineHeight: 1.6 }}>
              <Box component="span" sx={{ fontWeight: 500, color: "text.primary" }}>How it works:</Box> Antigravity/Copilot IDE request → DNS redirect to localhost:443 → MITM proxy intercepts → VansAI → response to Antigravity/Copilot
            </Typography>
          </Box>

          {/* Base URL + API Key — same row pattern as Claude Code / cli-tools */}
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            <Box sx={{ display: "grid", gap: 0.5, gridTemplateColumns: { xs: "1fr", sm: "8rem auto 1fr" }, alignItems: { sm: "center" }, columnGap: { sm: 1 } }}>
              <Box component="span" sx={{ fontSize: 12, fontWeight: 600, color: "text.primary", textAlign: { sm: "right" } }}>VansAI Base URL</Box>
              <MsIcon sx={{ display: { xs: "none", sm: "inline" }, color: "text.secondary", fontSize: 14 }}>arrow_forward</MsIcon>
              <OutlinedInput
                type="text"
                value={mitmRouterBaseUrl}
                onChange={(e) => dispatch({ mitmRouterBaseUrl: e.target.value })}
                placeholder={DEFAULT_MITM_ROUTER_BASE}
                disabled={isRunning}
                inputProps={{ "aria-label": "VansAI Base URL" }}
                size="small"
                sx={compactInputSx}
              />
            </Box>
            {!isRunning && (
              <Box sx={{ display: "grid", gap: 0.5, gridTemplateColumns: { xs: "1fr", sm: "8rem auto 1fr" }, alignItems: { sm: "center" }, columnGap: { sm: 1 } }}>
                <Box component="span" sx={{ fontSize: 12, fontWeight: 600, color: "text.primary", textAlign: { sm: "right" } }}>API Key</Box>
                <MsIcon sx={{ display: { xs: "none", sm: "inline" }, color: "text.secondary", fontSize: 14 }}>arrow_forward</MsIcon>
                <OutlinedInput
                  type="text"
                  value={selectedApiKey}
                  onChange={(e) => setSelectedApiKey(e.target.value)}
                  placeholder={cloudEnabled ? "Enter or pick API key" : "sk_9router (default)"}
                  inputProps={{ "aria-label": "API Key", list: "mitm-api-keys" }}
                  size="small"
                  sx={compactInputSx}
                />
                {apiKeys?.length > 0 && (
                  <datalist id="mitm-api-keys">
                    {apiKeys.map((key) => (
                      <option key={key.id} value={key.key}>{key.name || key.key}</option>
                    ))}
                  </datalist>
                )}
              </Box>
            )}
          </Box>

          {/* Action buttons */}
          <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, flexWrap: { sm: "wrap" }, alignItems: { sm: "center" }, gap: 1 }} data-i18n-skip="true">
            {status?.certExists && !status?.certTrusted && (
              <MuiButton
                type="button"
                variant="outlined"
                size="small"
                onClick={() => handleAction("trust-cert")}
                disabled={loading}
                startIcon={<MsIcon sx={{ fontSize: 16 }}>verified_user</MsIcon>}
                sx={{ width: { xs: "100%", sm: "auto" }, fontSize: 12, fontWeight: 500, textTransform: "none", color: "#ca8a04", borderColor: "rgba(234,179,8,0.30)", bgcolor: "rgba(234,179,8,0.10)", py: { sm: 0.75 }, "&:hover": { bgcolor: "rgba(234,179,8,0.20)", borderColor: "rgba(234,179,8,0.30)" } }}
              >
                Trust Cert
              </MuiButton>
            )}
            {isRunning ? (
              <MuiButton
                type="button"
                variant="outlined"
                size="small"
                onClick={() => handleAction("stop")}
                disabled={loading}
                startIcon={<MsIcon sx={{ fontSize: 16 }}>stop_circle</MsIcon>}
                sx={{ width: { xs: "100%", sm: "auto" }, fontSize: 12, fontWeight: 500, textTransform: "none", color: "#ef4444", borderColor: "rgba(239,68,68,0.30)", bgcolor: "rgba(239,68,68,0.10)", py: { sm: 0.75 }, "&:hover": { bgcolor: "rgba(239,68,68,0.20)", borderColor: "rgba(239,68,68,0.30)" } }}
              >
                Stop Server
              </MuiButton>
            ) : (
              <MuiButton
                type="button"
                variant="outlined"
                size="small"
                onClick={() => handleAction("start")}
                disabled={loading || !status || (serverIsWindows && !isAdmin)}
                title={serverIsWindows && !isAdmin ? "Administrator required" : undefined}
                startIcon={<MsIcon sx={{ fontSize: 16 }}>play_circle</MsIcon>}
                sx={{ width: { xs: "100%", sm: "auto" }, fontSize: 12, fontWeight: 500, textTransform: "none", color: "primary.main", borderColor: "rgba(229,106,74,0.30)", bgcolor: "rgba(229,106,74,0.10)", py: { sm: 0.75 }, "&:hover": { bgcolor: "rgba(229,106,74,0.20)", borderColor: "rgba(229,106,74,0.30)" } }}
              >
                Start Server
              </MuiButton>
            )}
            {isRunning && (
              <Typography sx={{ fontSize: 12, color: "text.secondary" }}>Enable DNS per tool below to activate interception</Typography>
            )}
          </Box>

          {/* Action error */}
          {actionError && (
            <Box sx={(theme) => ({ display: "flex", alignItems: "flex-start", gap: 1, px: 1, py: 0.75, borderRadius: "4px", fontSize: 12, bgcolor: "rgba(239,68,68,0.10)", color: theme.palette.mode === "dark" ? "#f87171" : "#dc2626", border: 1, borderColor: "rgba(239,68,68,0.20)" })}>
              <MsIcon sx={{ fontSize: 14, mt: 0.25, flexShrink: 0 }}>error</MsIcon>
              <Box component="span">{actionError}</Box>
            </Box>
          )}

          {/* Windows admin warning */}
          {serverIsWindows && !isAdmin && (
            <Box sx={{ display: "flex", alignItems: "center", gap: 1, px: 1, py: 0.75, borderRadius: "4px", fontSize: 12, bgcolor: "rgba(239,68,68,0.10)", color: "#dc2626", border: 1, borderColor: "rgba(239,68,68,0.20)" }}>
              <MsIcon sx={{ fontSize: 14 }}>shield_lock</MsIcon>
              <Box component="span">Administrator required — restart VansAI as Administrator to use MITM</Box>
            </Box>
          )}
        </Box>
      </Card>

      <MitmServerModals showPasswordModal={showPasswordModal} sudoPassword={sudoPassword} dispatch={dispatch} loading={loading} modalError={modalError} handleConfirmPassword={handleConfirmPassword} port443Conflict={port443Conflict} handleKillAndStart={handleKillAndStart} />
    </>
  );
}
