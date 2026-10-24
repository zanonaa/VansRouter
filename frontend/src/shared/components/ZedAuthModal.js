"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import PropTypes from "prop-types";
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
      <div className="flex flex-col gap-4">
        {(phase === "booting" || phase === "importing") && (
          <div className="flex items-center gap-2 px-3 py-2 border border-border rounded-lg bg-sidebar/50">
            <span className="material-symbols-outlined text-base text-primary animate-spin">
              progress_activity
            </span>
            <span className="text-sm">
              {phase === "importing"
                ? "Importing session from Zed IDE…"
                : "Detecting Zed IDE session…"}
            </span>
          </div>
        )}

        {phase === "ide-found" && ideSession && (
          <div className="space-y-3">
            <div className="bg-green-50 dark:bg-green-900/20 p-3 rounded-lg border border-green-200 dark:border-green-800">
              <div className="flex gap-2">
                <span className="material-symbols-outlined text-green-600 dark:text-green-400">
                  check_circle
                </span>
                <p className="text-sm text-green-800 dark:text-green-200">
                  Zed IDE session detected (user {ideSession.userId}). Import failed — retry below.
                </p>
              </div>
            </div>
            {reason && (
              <div className="bg-red-50 dark:bg-red-900/20 p-3 rounded-lg border border-red-200 dark:border-red-800">
                <p className="text-sm text-red-600 dark:text-red-400">{reason}</p>
              </div>
            )}
            <Button onClick={() => importIdeSession(ideSession)} fullWidth disabled={busy}>
              {busy ? "Importing…" : "Import from Zed IDE"}
            </Button>
          </div>
        )}

        {phase === "success" && (
          <div className="bg-green-50 dark:bg-green-900/20 p-3 rounded-lg border border-green-200 dark:border-green-800 text-sm text-green-800 dark:text-green-200">
            Connected successfully.
          </div>
        )}

        {phase === "unavailable" && (
          <div className="space-y-3">
            <div className="bg-yellow-500/10 p-3 rounded-lg border border-yellow-500/30">
              <div className="flex gap-2">
                <span className="material-symbols-outlined text-yellow-500">warning</span>
                <div className="space-y-1">
                  <p className="text-sm text-yellow-700 dark:text-yellow-300">
                    {reason}
                  </p>
                  <p className="text-xs text-text-muted">
                    Sign in to the Zed IDE on this machine first, then reopen this dialog —
                    the session is read from the OS keyring.
                  </p>
                </div>
              </div>
            </div>
            <Button onClick={onClose} variant="ghost" fullWidth>
              Close
            </Button>
          </div>
        )}
      </div>
    </Modal>
  );
}

ZedAuthModal.propTypes = {
  isOpen: PropTypes.bool.isRequired,
  providerInfo: PropTypes.object,
  onSuccess: PropTypes.func,
  onClose: PropTypes.func.isRequired,
};
