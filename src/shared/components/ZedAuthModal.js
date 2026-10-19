"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import PropTypes from "prop-types";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import { Modal, Button } from "@/shared/components";

/**
 * Zed Connect modal.
 *
 * Zed auth is OAuth, but the only flow this build can serve is the IDE session
 * import: the browser/paste flow needs /api/oauth/zed/{start-proxy,authorize,
 * register-session,poll-status,exchange,stop-proxy}, none of which exist here
 * (the generic /api/oauth/[provider]/[action] route only serves codex and xai).
 * Rendering those steps produced buttons that could never succeed, so the modal
 * now offers only what works.
 */
export default function ZedAuthModal({ isOpen, providerInfo, onSuccess, onClose }) {
  const [phase, setPhase] = useState("booting"); // booting | ide-found | importing | success | unavailable
  const [ideSession, setIdeSession] = useState(null);
  const [reason, setReason] = useState(null);
  const [busy, setBusy] = useState(false);
  const openedRef = useRef(false);
  const isOpenRef = useRef(isOpen);
  const onSuccessRef = useRef(onSuccess);
  const onCloseRef = useRef(onClose);

  useEffect(() => {
    isOpenRef.current = isOpen;
    onSuccessRef.current = onSuccess;
    onCloseRef.current = onClose;
  });

  const finishSuccess = useCallback(() => {
    setPhase("success");
    onSuccessRef.current?.();
    setTimeout(() => onCloseRef.current?.(), 600);
  }, []);

  const importIdeSession = useCallback(async (session) => {
    if (!session?.accessToken || !session?.userId) return;
    setBusy(true);
    setReason(null);
    setPhase("importing");
    try {
      const res = await fetch("/api/oauth/zed/import", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          accessToken: session.accessToken,
          userId: session.userId,
          ...(session.systemId ? { systemId: session.systemId } : {}),
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "Import failed");
      finishSuccess();
    } catch (err) {
      setReason(err.message);
      setPhase("ide-found");
    } finally {
      setBusy(false);
    }
  }, [finishSuccess]);

  useEffect(() => {
    if (!isOpen) return;
    if (openedRef.current) return;
    openedRef.current = true;
    setPhase("booting");
    setIdeSession(null);
    setReason(null);
    setBusy(false);

    let cancelled = false;

    (async () => {
      try {
        const res = await fetch("/api/oauth/zed/auto-import", {
          signal: AbortSignal.timeout(12000),
        });
        const data = await res.json();
        if (cancelled || !isOpenRef.current) return;

        if (data.found && data.accessToken && data.userId) {
          const session = {
            accessToken: data.accessToken,
            userId: String(data.userId),
            systemId: data.systemId || "",
          };
          setIdeSession(session);
          await importIdeSession(session);
          return;
        }

        setReason(data.error || "No Zed IDE session found on this machine.");
        setPhase("unavailable");
      } catch (err) {
        if (cancelled || !isOpenRef.current) return;
        setReason(err.message || "Could not read the Zed IDE session.");
        setPhase("unavailable");
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [isOpen, importIdeSession]);

  useEffect(() => {
    if (isOpen) return;
    openedRef.current = false;
  }, [isOpen]);

  const title = `Connect ${providerInfo?.name || "Zed"}`;

  return (
    <Modal isOpen={isOpen} title={title} onClose={onClose} size="lg">
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {(phase === "booting" || phase === "importing") && (
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, px: 1.5, py: 1, border: "1px solid var(--color-border)", borderRadius: "8px", bgcolor: "rgba(244,241,236,0.5)" }}>
            <span className="material-symbols-outlined" style={{ animation: "spin 1s linear infinite",  fontSize: 16, color: "var(--color-primary)" }}>
              progress_activity
            </span>
            <Typography sx={{ fontSize: 14 }}>
              {phase === "importing"
                ? "Importing session from Zed IDE…"
                : "Detecting Zed IDE session…"}
            </Typography>
          </Box>
        )}

        {phase === "ide-found" && ideSession && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
            <Box sx={{ bgcolor: "rgba(34,197,94,0.1)", p: 1.5, borderRadius: "8px", border: "1px solid rgba(34,197,94,0.3)" }}>
              <Box sx={{ display: "flex", gap: 1 }}>
                <span className="material-symbols-outlined" style={{ color: "#16a34a" }}>check_circle</span>
                <Typography sx={{ fontSize: 14, color: "#166534" }}>
                  Zed IDE session detected (user {ideSession.userId}). Import failed — retry below.
                </Typography>
              </Box>
            </Box>
            {reason && (
              <Box sx={{ bgcolor: "rgba(239,68,68,0.1)", p: 1.5, borderRadius: "8px", border: "1px solid rgba(239,68,68,0.3)" }}>
                <Typography sx={{ fontSize: 14, color: "#dc2626" }}>{reason}</Typography>
              </Box>
            )}
            <Button onClick={() => importIdeSession(ideSession)} fullWidth disabled={busy}>
              {busy ? "Importing…" : "Import from Zed IDE"}
            </Button>
          </Box>
        )}

        {phase === "success" && (
          <Box sx={{ bgcolor: "rgba(34,197,94,0.1)", p: 1.5, borderRadius: "8px", border: "1px solid rgba(34,197,94,0.3)", fontSize: 14, color: "#166534" }}>
            Connected successfully.
          </Box>
        )}

        {phase === "unavailable" && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
            <Box sx={{ bgcolor: "rgba(234,179,8,0.1)", p: 1.5, borderRadius: "8px", border: "1px solid rgba(234,179,8,0.3)" }}>
              <Box sx={{ display: "flex", gap: 1 }}>
                <span className="material-symbols-outlined" style={{ color: "#eab308" }}>warning</span>
                <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
                  <Typography sx={{ fontSize: 14, color: "#a16207" }}>
                    {reason}
                  </Typography>
                  <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
                    Sign in to the Zed IDE on this machine first, then reopen this dialog —
                    the session is read from the OS keyring.
                  </Typography>
                </Box>
              </Box>
            </Box>
            <Button onClick={onClose} variant="ghost" fullWidth>
              Close
            </Button>
          </Box>
        )}
      </Box>
    </Modal>
  );
}

ZedAuthModal.propTypes = {
  isOpen: PropTypes.bool.isRequired,
  providerInfo: PropTypes.object,
  onSuccess: PropTypes.func,
  onClose: PropTypes.func.isRequired,
};
