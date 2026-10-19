"use client";

import { useReducer } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import { Modal, Button, Input, OAuthModal } from "@/shared/components";

const GITLAB_COM = "https://gitlab.com";

function getRedirectUri() {
  if (typeof window === "undefined") return "http://localhost/callback";
  const port = window.location.port || (window.location.protocol === "https:" ? "443" : "80");
  return `http://localhost:${port}/callback`;
}

const INIT_STATE = { mode: null, baseUrl: GITLAB_COM, clientId: "", clientSecret: "", pat: "", loading: false, error: null, showOAuth: false, oauthMeta: null };

function gitlabReducer(state, action) {
  switch (action.type) {
    case "RESET": return INIT_STATE;
    case "SET_MODE": return { ...state, mode: action.value, error: null };
    case "SET_BASE_URL": return { ...state, baseUrl: action.value };
    case "SET_CLIENT_ID": return { ...state, clientId: action.value };
    case "SET_CLIENT_SECRET": return { ...state, clientSecret: action.value };
    case "SET_PAT": return { ...state, pat: action.value };
    case "ERROR": return { ...state, error: action.error };
    case "PAT_START": return { ...state, loading: true, error: null };
    case "PAT_DONE": return { ...state, loading: false, error: action.error || null };
    case "OAUTH_START": return { ...state, error: null, oauthMeta: action.meta, showOAuth: true };
    case "OAUTH_CLOSE": return { ...state, showOAuth: false };
    default: return state;
  }
}

const modeCardSx = {
  display: "flex",
  flexDirection: "column",
  alignItems: "center",
  gap: 1,
  p: 2,
  borderRadius: "8px",
  border: "1px solid var(--color-border)",
  bgcolor: "transparent",
  textAlign: "left",
  cursor: "pointer",
  transition: "all 150ms",
  "&:hover": { borderColor: "primary.main", bgcolor: "rgba(229,106,74,0.05)" },
};

/**
 * GitLab Duo Authentication Modal
 * Supports two modes:
 * - OAuth (PKCE): requires OAuth App Client ID (and optional Client Secret)
 * - PAT: requires Personal Access Token
 */
export default function GitLabAuthModal({ isOpen, providerInfo, onSuccess, onClose }) {
  const [state, dispatch] = useReducer(gitlabReducer, INIT_STATE);
  const { mode, baseUrl, clientId, clientSecret, pat, loading, error, showOAuth, oauthMeta } = state;

  const reset = () => dispatch({ type: "RESET" });

  const handleClose = () => {
    reset();
    onClose();
  };

  const handleOAuthStart = () => {
    if (!clientId.trim()) {
      dispatch({ type: "ERROR", error: "Client ID is required" });
      return;
    }
    dispatch({ type: "OAUTH_START", meta: { baseUrl: baseUrl.trim() || GITLAB_COM, clientId: clientId.trim(), clientSecret: clientSecret.trim() } });
  };

  const handlePATSubmit = async () => {
    if (!pat.trim()) {
      dispatch({ type: "ERROR", error: "Personal Access Token is required" });
      return;
    }
    dispatch({ type: "PAT_START" });
    try {
      const res = await fetch("/api/oauth/gitlab/pat", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token: pat.trim(), baseUrl: baseUrl.trim() || GITLAB_COM }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "Authentication failed");
      onSuccess?.();
      handleClose();
    } catch (err) {
      dispatch({ type: "PAT_DONE", error: err.message });
    }
  };

  if (!isOpen) return null;

  // Sub-modal for OAuth PKCE flow
  if (showOAuth && oauthMeta) {
    return (
      <OAuthModal
        isOpen
        provider="gitlab"
        providerInfo={providerInfo}
        oauthMeta={oauthMeta}
        onSuccess={() => { onSuccess?.(); handleClose(); }}
        onClose={() => dispatch({ type: "OAUTH_CLOSE" })}
      />
    );
  }

  return (
    <Modal isOpen={isOpen} title="Connect GitLab Duo" onClose={handleClose} size="lg">
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {/* Mode selection */}
        {!mode && (
          <>
            <Typography sx={{ fontSize: 14, color: "text.secondary" }}>
              Choose how to authenticate with GitLab Duo:
            </Typography>
            <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 1.5 }}>
              <Box component="button" type="button" onClick={() => dispatch({ type: "SET_MODE", value: "oauth" })} sx={modeCardSx}>
                <span className="material-symbols-outlined" style={{ fontSize: 24, color: "var(--color-primary)" }}>lock_open</span>
                <Box>
                  <Typography sx={{ fontSize: 14, fontWeight: 500 }}>OAuth App</Typography>
                  <Typography sx={{ fontSize: 12, color: "text.secondary" }}>Use a GitLab OAuth application</Typography>
                </Box>
              </Box>
              <Box component="button" type="button" onClick={() => dispatch({ type: "SET_MODE", value: "pat" })} sx={modeCardSx}>
                <span className="material-symbols-outlined" style={{ fontSize: 24, color: "var(--color-primary)" }}>key</span>
                <Box>
                  <Typography sx={{ fontSize: 14, fontWeight: 500 }}>Personal Access Token</Typography>
                  <Typography sx={{ fontSize: 12, color: "text.secondary" }}>Use a GitLab PAT with api scope</Typography>
                </Box>
              </Box>
            </Box>
          </>
        )}

        {/* OAuth mode */}
        {mode === "oauth" && (
          <>
            <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
              Create an OAuth app at{" "}
              <Box component="a" href={`${baseUrl.trim() || GITLAB_COM}/-/profile/applications`} target="_blank" rel="noreferrer" sx={{ color: "primary.main", textDecoration: "underline" }}>
                GitLab Applications
              </Box>{" "}
              with redirect URI{" "}
              <Box component="code" sx={{ bgcolor: "var(--color-sidebar)", px: 0.5, borderRadius: "4px", fontSize: 12 }}>{getRedirectUri()}</Box>
            </Typography>
            <Input label="GitLab Base URL" value={baseUrl} onChange={(e) => dispatch({ type: "SET_BASE_URL", value: e.target.value })} placeholder={GITLAB_COM} />
            <Input label="Client ID" value={clientId} onChange={(e) => dispatch({ type: "SET_CLIENT_ID", value: e.target.value })} placeholder="Your OAuth application client ID" />
            <Input label="Client Secret (optional for PKCE)" value={clientSecret} onChange={(e) => dispatch({ type: "SET_CLIENT_SECRET", value: e.target.value })} placeholder="Leave empty for public PKCE app" />
            {error && <Typography sx={{ fontSize: 14, color: "error.main" }}>{error}</Typography>}
            <Box sx={{ display: "flex", gap: 1 }}>
              <Button onClick={handleOAuthStart} fullWidth disabled={!clientId.trim()}>
                Authorize
              </Button>
              <Button onClick={() => dispatch({ type: "SET_MODE", value: null })} variant="ghost" fullWidth>
                Back
              </Button>
            </Box>
          </>
        )}

        {/* PAT mode */}
        {mode === "pat" && (
          <>
            <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
              Create a PAT at{" "}
              <Box component="a" href={`${baseUrl.trim() || GITLAB_COM}/-/user_settings/personal_access_tokens`} target="_blank" rel="noreferrer" sx={{ color: "primary.main", textDecoration: "underline" }}>
                GitLab Access Tokens
              </Box>{" "}
              with scopes: <Box component="code" sx={{ bgcolor: "var(--color-sidebar)", px: 0.5, borderRadius: "4px", fontSize: 12 }}>api</Box>,{" "}
              <Box component="code" sx={{ bgcolor: "var(--color-sidebar)", px: 0.5, borderRadius: "4px", fontSize: 12 }}>read_user</Box>, and{" "}
              <Box component="code" sx={{ bgcolor: "var(--color-sidebar)", px: 0.5, borderRadius: "4px", fontSize: 12 }}>ai_features</Box>.
            </Typography>
            <Input label="GitLab Base URL" value={baseUrl} onChange={(e) => dispatch({ type: "SET_BASE_URL", value: e.target.value })} placeholder={GITLAB_COM} />
            <Input label="Personal Access Token" value={pat} onChange={(e) => dispatch({ type: "SET_PAT", value: e.target.value })} placeholder="glpat-xxxxxxxxxxxxxxxxxxxx" type="password" />
            {error && <Typography sx={{ fontSize: 14, color: "error.main" }}>{error}</Typography>}
            <Box sx={{ display: "flex", gap: 1 }}>
              <Button onClick={handlePATSubmit} fullWidth disabled={!pat.trim() || loading} loading={loading}>
                Connect
              </Button>
              <Button onClick={() => dispatch({ type: "SET_MODE", value: null })} variant="ghost" fullWidth>
                Back
              </Button>
            </Box>
          </>
        )}
      </Box>
    </Modal>
  );
}
