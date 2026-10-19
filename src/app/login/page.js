"use client";

import { useState, useEffect } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Divider from "@mui/material/Divider";
import CircularProgress from "@mui/material/CircularProgress";
import { Card, Button, Input } from "@/shared/components";

function Code({ children }) {
  return (
    <Box
      component="code"
      sx={{ bgcolor: "var(--color-sidebar)", px: 0.5, borderRadius: "4px", fontFamily: "ui-monospace, monospace" }}
    >
      {children}
    </Box>
  );
}

export default function LoginPage() {
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [resetHint, setResetHint] = useState("");
  const [retryAfter, setRetryAfter] = useState(0);
  const [loading, setLoading] = useState(false);
  const [hasPassword, setHasPassword] = useState(null);
  const [authMode, setAuthMode] = useState("password");
  const [oidcConfigured, setOidcConfigured] = useState(false);
  const [oidcLoginLabel, setOidcLoginLabel] = useState("Sign in with OIDC");
  const [mustChange, setMustChange] = useState(false);
  const [newPassword, setNewPassword] = useState("");

  // Countdown for rate-limit
  useEffect(() => {
    if (retryAfter <= 0) return;
    const id = setInterval(() => setRetryAfter((s) => (s > 0 ? s - 1 : 0)), 1000);
    return () => clearInterval(id);
  }, [retryAfter]);

  useEffect(() => {
    const controller = new AbortController();
    let timeoutId;
    async function checkAuth() {
      timeoutId = setTimeout(() => controller.abort(), 5000);
      const baseUrl = typeof window !== "undefined" ? window.location.origin : "";

      try {
        const res = await fetch(`${baseUrl}/api/auth/status`, {
          signal: controller.signal,
        });
        clearTimeout(timeoutId);

        if (res.ok) {
          const data = await res.json();
          if (data.requireLogin === false) {
            window.location.assign("/dashboard");
            return;
          }
          setHasPassword(!!data.hasPassword);
          setAuthMode(data.authMode || "password");
          setOidcConfigured(data.oidcConfigured === true);
          setOidcLoginLabel(data.oidcLoginLabel || "Sign in with OIDC");
        } else {
          // Safe fallback on non-OK response to avoid infinite loading state.
          setHasPassword(true);
        }
      } catch (err) {
        clearTimeout(timeoutId);
        setHasPassword(true);
      }
    }
    checkAuth();
    return () => {
      controller.abort();
      if (timeoutId) clearTimeout(timeoutId);
    };
  }, []);

  const handleLogin = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError("");
    setResetHint("");

    try {
      const res = await fetch("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ password }),
      });

      if (res.ok) {
        const data = await res.json();
        if (data.mustChangePassword) {
          setMustChange(true);
          return;
        }
        window.location.assign("/dashboard");
      } else {
        const data = await res.json();
        setError(data.error || "Invalid password");
        if (data.resetHint) setResetHint(data.resetHint);
        if (data.retryAfter) setRetryAfter(Number(data.retryAfter));
      }
    } catch (err) {
      setError("An error occurred. Please try again.");
    } finally {
      setLoading(false);
    }
  };

  // Force a new password before entering the dashboard (default + remote).
  const handleSetNewPassword = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError("");
    try {
      const res = await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ currentPassword: password, newPassword }),
      });
      if (res.ok) {
        window.location.assign("/dashboard");
      } else {
        const data = await res.json();
        setError(data.error || "Failed to set password");
      }
    } catch (err) {
      setError("An error occurred. Please try again.");
    } finally {
      setLoading(false);
    }
  };

  const handleOidcLogin = () => {
    window.location.href = "/api/auth/oidc/start";
  };

  const oidcAvailable = oidcConfigured && ["oidc", "both"].includes(authMode);
  const passwordAvailable = authMode !== "oidc" || !oidcConfigured;

  // Show loading state while checking password
  if (hasPassword === null) {
    return (
      <Box sx={{ minHeight: "100vh", display: "flex", alignItems: "center", justifyContent: "center", bgcolor: "background.default", p: 2 }}>
        <Box sx={{ textAlign: "center" }}>
          <CircularProgress size={32} />
          <Typography sx={{ color: "text.secondary", mt: 2 }}>Loading...</Typography>
        </Box>
      </Box>
    );
  }

  return (
    <Box
      sx={{
        minHeight: "100vh",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        bgcolor: "background.default",
        p: 2,
        position: "relative",
        overflow: "hidden",
      }}
    >
      {/* Faint grid background */}
      <Box className="landing-grid" aria-hidden="true" sx={{ position: "absolute", inset: 0, pointerEvents: "none" }} />
      <Box sx={{ position: "relative", zIndex: 10, width: "100%", maxWidth: 448 }}>
        <Box sx={{ textAlign: "center", mb: 4 }}>
          <Typography variant="h4" sx={{ fontWeight: 700, color: "primary.main", mb: 1 }}>
            VansRouter
          </Typography>
          <Typography sx={{ color: "text.secondary" }}>
            {authMode === "oidc" && oidcConfigured
              ? "Sign in with your OIDC provider to access the dashboard"
              : "Enter your password to access the dashboard"}
          </Typography>
        </Box>

        <Card>
          {mustChange ? (
            <Box component="form" onSubmit={handleSetNewPassword} sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
              <Typography variant="body2" sx={{ color: "warning.main", textAlign: "center" }}>
                Set a new password before accessing the dashboard remotely.
              </Typography>
              <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                <Typography component="label" variant="body2" sx={{ fontWeight: 500 }}>
                  New password
                </Typography>
                <Input
                  type="password"
                  placeholder="Enter new password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  required
                  autoFocus
                />
                {error && (
                  <Typography variant="caption" sx={{ color: "error.main" }}>
                    {error}
                  </Typography>
                )}
              </Box>
              <Button type="submit" variant="primary" fullWidth loading={loading} disabled={!newPassword}>
                Set password
              </Button>
            </Box>
          ) : (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
              {oidcAvailable && (
                <Button type="button" variant="primary" fullWidth onClick={handleOidcLogin}>
                  {oidcLoginLabel}
                </Button>
              )}

              {oidcAvailable && passwordAvailable && <Divider sx={{ borderColor: "divider" }} />}

              {passwordAvailable ? (
                <Box component="form" onSubmit={handleLogin} sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
                  {((authMode === "oidc" && !oidcConfigured) || (authMode === "both" && !oidcConfigured)) && (
                    <Typography variant="caption" sx={{ color: "warning.main", textAlign: "center" }}>
                      OIDC login is enabled, but the issuer/client fields are not configured yet. Password login is still available for recovery.
                    </Typography>
                  )}

                  {authMode === "both" && oidcConfigured && (
                    <Typography variant="caption" sx={{ color: "text.secondary", textAlign: "center" }}>
                      Password and OIDC login are both enabled.
                    </Typography>
                  )}

                  <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                    <Typography component="label" variant="body2" sx={{ fontWeight: 500 }}>
                      Password
                    </Typography>
                    <Input
                      type="password"
                      placeholder="Enter password"
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      required
                      autoFocus={!oidcAvailable}
                    />
                    {error && (
                      <Typography variant="caption" sx={{ color: "error.main" }}>
                        {error}
                      </Typography>
                    )}
                    {retryAfter > 0 && (
                      <Typography variant="caption" sx={{ color: "warning.main" }}>
                        Locked. Retry in <Box component="span" sx={{ fontFamily: "ui-monospace, monospace" }}>{retryAfter}s</Box>.
                      </Typography>
                    )}
                    {resetHint && (
                      <Typography variant="caption" sx={{ color: "text.secondary" }}>
                        Forgot password? Open <Code>vansrouter</Code> CLI on the host → <b>Settings</b> → <b>Set/Replace Dashboard Password</b>.
                      </Typography>
                    )}
                  </Box>

                  <Button
                    type="submit"
                    variant="primary"
                    fullWidth
                    loading={loading}
                    disabled={retryAfter > 0}
                  >
                    {retryAfter > 0 ? `Wait ${retryAfter}s` : "Login"}
                  </Button>

                  {hasPassword === false && (
                    <Typography variant="caption" sx={{ textAlign: "center", color: "warning.main" }}>
                      New installation default: <Code>123456</Code>. Change it before exposing the dashboard.
                    </Typography>
                  )}
                </Box>
              ) : (
                error && (
                  <Typography variant="caption" sx={{ color: "error.main" }}>
                    {error}
                  </Typography>
                )
              )}
            </Box>
          )}
        </Card>
      </Box>
    </Box>
  );
}
