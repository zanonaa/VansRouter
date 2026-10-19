"use client";

import { useEffect, useRef, useState } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import { Modal, Button } from "@/shared/components";

/**
 * iFlow Cookie Authentication Modal
 * User pastes browser cookie to get fresh API key
 */
export default function IFlowCookieModal({ isOpen, onSuccess, onClose }) {
  const [cookie, setCookie] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(false);
  const flowIdRef = useRef(0);

  useEffect(() => {
    if (!isOpen) flowIdRef.current += 1;
  }, [isOpen]);

  const handleSubmit = async () => {
    if (!cookie.trim()) {
      setError("Please paste your cookie");
      return;
    }

    const flowId = ++flowIdRef.current;
    setLoading(true);
    setError(null);

    try {
      const res = await fetch("/api/oauth/iflow/cookie", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ cookie: cookie.trim() }),
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(data.error || "Authentication failed");
      }
      if (flowId !== flowIdRef.current || !isOpen) return;

      setSuccess(true);
      setTimeout(() => {
        if (flowId !== flowIdRef.current || !isOpen) return;
        onSuccess?.();
        handleClose();
      }, 1500);
    } catch (err) {
      if (flowId === flowIdRef.current && isOpen) setError(err.message);
    } finally {
      if (flowId === flowIdRef.current) setLoading(false);
    }
  };

  const handleClose = () => {
    flowIdRef.current += 1;
    setCookie("");
    setError(null);
    setSuccess(false);
    onClose?.();
  };

  return (
    <Modal isOpen={isOpen} onClose={handleClose} title="iFlow Cookie Authentication">
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {success ? (
          <Box sx={{ textAlign: "center", py: 4 }}>
            <Box sx={{ fontSize: 60, mb: 2 }}>✅</Box>
            <Typography sx={{ fontSize: 18, fontWeight: 500, color: "text.primary" }}>Authentication Successful!</Typography>
            <Typography sx={{ fontSize: 14, color: "text.secondary", mt: 1 }}>Fresh API key obtained</Typography>
          </Box>
        ) : (
          <>
            <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
              <Typography sx={{ fontSize: 14, color: "text.secondary" }}>
                To get a fresh API key, paste your browser cookie from{" "}
                <Box component="a" href="https://platform.iflow.cn" target="_blank" rel="noopener noreferrer" sx={{ color: "primary.main", "&:hover": { textDecoration: "underline" } }}>
                  platform.iflow.cn
                </Box>
              </Typography>
              <Box sx={{ bgcolor: "var(--color-surface-2)", p: 1.5, borderRadius: "8px", fontSize: 12, display: "flex", flexDirection: "column", gap: 1 }}>
                <Typography sx={{ fontWeight: 500, color: "text.primary" }}>How to get cookie:</Typography>
                <Box component="ol" sx={{ listStyle: "decimal", pl: 2.5, m: 0, display: "flex", flexDirection: "column", gap: 0.5, color: "text.secondary" }}>
                  <li>Open platform.iflow.cn in your browser</li>
                  <li>Login to your account</li>
                  <li>Open DevTools (F12) → Application/Storage → Cookies</li>
                  <li>Copy the entire cookie string (must include BXAuth)</li>
                  <li>Paste it below</li>
                </Box>
              </Box>
            </Box>

            <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
              <Box component="label" htmlFor="iflow-cookie-string" sx={{ display: "block", fontSize: 14, fontWeight: 500, color: "text.primary" }}>
                Cookie String
              </Box>
              <Box
                component="textarea"
                id="iflow-cookie-string"
                value={cookie}
                onChange={(e) => setCookie(e.target.value)}
                placeholder="BXAuth=xxx; ..."
                rows={4}
                disabled={loading}
                sx={{
                  width: "100%",
                  px: 1.5,
                  py: 1,
                  bgcolor: "var(--color-surface-2)",
                  border: "1px solid var(--color-border)",
                  borderRadius: "8px",
                  fontSize: 14,
                  color: "text.primary",
                  resize: "none",
                  outline: "none",
                  "&::placeholder": { color: "text.secondary" },
                  "&:focus": { boxShadow: "0 0 0 2px rgba(229,106,74,0.3)" },
                }}
              />
            </Box>

            {error && (
              <Box sx={{ p: 1.5, bgcolor: "rgba(239,68,68,0.1)", border: "1px solid rgba(239,68,68,0.2)", borderRadius: "8px" }}>
                <Typography sx={{ fontSize: 14, color: "error.main" }}>{error}</Typography>
              </Box>
            )}

            <Box sx={{ display: "flex", gap: 1.5, pt: 1 }}>
              <Button variant="secondary" onClick={handleClose} disabled={loading} fullWidth>
                Cancel
              </Button>
              <Button onClick={handleSubmit} loading={loading} fullWidth>
                Authenticate
              </Button>
            </Box>
          </>
        )}
      </Box>
    </Modal>
  );
}
