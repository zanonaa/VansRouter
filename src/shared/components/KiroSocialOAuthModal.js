"use client";

import { useState, useEffect, useRef } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import { Modal, Button } from "@/shared/components";
import { useCopyToClipboard } from "@/shared/hooks/useCopyToClipboard";
import { getNextKiroSocialPollInterval } from "@/lib/oauth/kiroSocialPoll";

/**
 * Kiro Social OAuth Modal (Google/GitHub Device Code Flow)
 */
export default function KiroSocialOAuthModal({
  isOpen,
  provider,
  targetProvider,
  providerLabel = "Kiro",
  onSuccess,
  onClose,
}) {
  const [step, setStep] = useState("loading"); // "loading" | "polling" | "success" | "error"
  const [error, setError] = useState(null);
  const [userCode, setUserCode] = useState("");
  const [authUrl, setAuthUrl] = useState("");
  const { copied, copy } = useCopyToClipboard();
  const pollRef = useRef(null);
  const onSuccessRef = useRef(onSuccess);

  useEffect(() => {
    onSuccessRef.current = onSuccess;
  }, [onSuccess]);

  useEffect(() => {
    if (!isOpen || !provider) return;
    let cancelled = false;

    const stopPolling = () => {
      if (pollRef.current) clearTimeout(pollRef.current);
      pollRef.current = null;
    };

    const fail = (message) => {
      stopPolling();
      if (cancelled) return;
      setError(message);
      setStep("error");
    };

    const initAuth = async () => {
      try {
        setError(null);
        setStep("loading");

        const res = await fetch(`/api/oauth/kiro/social-authorize?provider=${provider}`);
        const data = await res.json();
        if (cancelled) return;

        if (!res.ok) {
          throw new Error(data.error || "Failed to start authorization");
        }

        setUserCode(data.userCode || "");
        setAuthUrl(data.authUrl || "");
        setStep("polling");

        const baseIntervalMs = Math.max(1, Number(data.interval) || 5) * 1000;
        let currentIntervalMs = baseIntervalMs;
        const expiresAt = Date.now() + Math.max(1, Number(data.expiresIn) || 300) * 1000;

        const schedule = (delayMs) => {
          if (cancelled) return;
          pollRef.current = setTimeout(poll, delayMs);
        };

        let consecutiveErrors = 0;
        const poll = async () => {
          pollRef.current = null;
          if (cancelled) return;
          if (Date.now() >= expiresAt) {
            fail("Authorization code expired. Please try again.");
            return;
          }

          try {
            const pollRes = await fetch("/api/oauth/kiro/social-exchange", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ deviceCode: data.deviceCode, provider, targetProvider }),
            });
            const pollData = await pollRes.json();
            if (cancelled) return;

            if (pollData.success) {
              stopPolling();
              setStep("success");
              onSuccessRef.current?.();
              return;
            }

            if (!pollData.pending) {
              fail(pollData.error || "Authorization failed");
              return;
            }

            consecutiveErrors = 0;
            currentIntervalMs = getNextKiroSocialPollInterval(currentIntervalMs, pollData.error);
            schedule(currentIntervalMs);
          } catch {
            consecutiveErrors += 1;
            if (consecutiveErrors >= 5) {
              fail("Network error: Unable to reach authorization server. Please try again.");
              return;
            }
            schedule(currentIntervalMs);
          }
        };

        schedule(baseIntervalMs);
      } catch (err) {
        fail(err.message);
      }
    };

    initAuth();

    return () => {
      cancelled = true;
      stopPolling();
    };
  }, [isOpen, provider, targetProvider]);

  const handleClose = () => {
    if (pollRef.current) {
      clearTimeout(pollRef.current);
      pollRef.current = null;
    }
    onClose?.();
  };

  const providerName = provider === "google" ? "Google" : "GitHub";

  const circleSx = (bgcolor) => ({ width: 64, height: 64, mx: "auto", mb: 2, borderRadius: "50%", bgcolor, display: "flex", alignItems: "center", justifyContent: "center" });
  const copyBtnSx = { flexShrink: 0, p: 0.5, borderRadius: "4px", border: 0, bgcolor: "transparent", color: "text.secondary", cursor: "pointer", "&:hover": { bgcolor: "var(--color-sidebar)", color: "text.primary" } };

  return (
    <Modal
      isOpen={isOpen}
      title={`Connect ${providerLabel} via ${providerName}`}
      onClose={handleClose}
      size="lg"
    >
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {step === "loading" && (
          <Box sx={{ textAlign: "center", py: 3 }}>
            <Box sx={circleSx("rgba(229,106,74,0.1)")}>
              <span className="material-symbols-outlined" style={{ animation: "spin 1s linear infinite",  fontSize: 30, color: "var(--color-primary)" }}>
                progress_activity
              </span>
            </Box>
            <Typography variant="h6" sx={{ fontSize: 18, fontWeight: 600, mb: 1 }}>Initializing...</Typography>
            <Typography sx={{ fontSize: 14, color: "text.secondary" }}>Setting up {providerName} authentication</Typography>
          </Box>
        )}

        {step === "polling" && (
          <Box sx={{ textAlign: "center", py: 3 }}>
            <Box sx={circleSx("rgba(229,106,74,0.1)")}>
              <span className="material-symbols-outlined" style={{ animation: "pulse 2s cubic-bezier(0.4,0,0.6,1) infinite",  fontSize: 30, color: "var(--color-primary)" }}>
                open_in_browser
              </span>
            </Box>
            <Typography variant="h6" sx={{ fontSize: 18, fontWeight: 600, mb: 1 }}>Authorize in Browser</Typography>
            <Typography sx={{ fontSize: 14, color: "text.secondary", mb: 1.5 }}>
              Open the link below and enter your verification code:
            </Typography>
            {authUrl && (
              <Box sx={{ mb: 2 }}>
                <Box sx={{ display: "flex", alignItems: "center", gap: 1, justifyContent: "center" }}>
                  <Box
                    component="a"
                    href={authUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    sx={{ fontSize: 12, fontFamily: "ui-monospace, monospace", color: "primary.main", textDecoration: "underline", wordBreak: "break-all", maxWidth: 448, display: "inline-block" }}
                  >
                    {authUrl.length > 80 ? authUrl.slice(0, 80) + "..." : authUrl}
                  </Box>
                  <Box component="button" onClick={() => copy(authUrl, "auth_url")} title="Copy Link" sx={copyBtnSx}>
                    <span className="material-symbols-outlined" style={{ fontSize: 16 }}>
                      {copied === "auth_url" ? "check" : "content_copy"}
                    </span>
                  </Box>
                </Box>
              </Box>
            )}
            {userCode && (
              <Box sx={{ mb: 2 }}>
                <Typography sx={{ fontSize: 12, color: "text.secondary", mb: 0.5 }}>Verification Code</Typography>
                <Box sx={{ display: "flex", alignItems: "center", justifyContent: "center", gap: 1 }}>
                  <Box component="p" sx={{ fontFamily: "ui-monospace, monospace", fontSize: 24, fontWeight: 700, letterSpacing: "0.1em", m: 0 }}>{userCode}</Box>
                  <Box component="button" onClick={() => copy(userCode, "user_code")} title="Copy Code" sx={copyBtnSx}>
                    <span className="material-symbols-outlined" style={{ fontSize: 16 }}>
                      {copied === "user_code" ? "check" : "content_copy"}
                    </span>
                  </Box>
                </Box>
              </Box>
            )}
            <Box sx={{ display: "flex", alignItems: "center", justifyContent: "center", gap: 1, fontSize: 14, color: "text.secondary" }}>
              <span className="material-symbols-outlined" style={{ animation: "spin 1s linear infinite",  fontSize: 16 }}>
                progress_activity
              </span>
              Waiting for authorization in browser...
            </Box>
            <Box sx={{ mt: 3 }}>
              <Button onClick={handleClose} variant="ghost" fullWidth>
                Cancel
              </Button>
            </Box>
          </Box>
        )}

        {step === "success" && (
          <Box sx={{ textAlign: "center", py: 3 }}>
            <Box sx={circleSx("rgba(34,197,94,0.15)")}>
              <span className="material-symbols-outlined" style={{ fontSize: 30, color: "#16a34a" }}>
                check_circle
              </span>
            </Box>
            <Typography variant="h6" sx={{ fontSize: 18, fontWeight: 600, mb: 1 }}>Connected Successfully!</Typography>
            <Typography sx={{ fontSize: 14, color: "text.secondary", mb: 2 }}>
              Your {providerLabel} account via {providerName} has been connected.
            </Typography>
            <Button onClick={handleClose} fullWidth>
              Done
            </Button>
          </Box>
        )}

        {step === "error" && (
          <Box sx={{ textAlign: "center", py: 3 }}>
            <Box sx={circleSx("rgba(239,68,68,0.15)")}>
              <span className="material-symbols-outlined" style={{ fontSize: 30, color: "#dc2626" }}>error</span>
            </Box>
            <Typography variant="h6" sx={{ fontSize: 18, fontWeight: 600, mb: 1 }}>Connection Failed</Typography>
            <Typography sx={{ fontSize: 14, color: "#dc2626", mb: 2 }}>{error}</Typography>
            <Box sx={{ display: "flex", gap: 1 }}>
              <Button onClick={handleClose} variant="ghost" fullWidth>
                Close
              </Button>
            </Box>
          </Box>
        )}
      </Box>
    </Modal>
  );
}
