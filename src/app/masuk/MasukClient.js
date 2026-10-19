"use client";

import { useState, useEffect, useReducer } from "react";
import Card from "@/shared/components/Card";
import Button from "@/shared/components/Button";
import Input from "@/shared/components/Input";
import { useRouter } from "next/navigation";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Divider from "@mui/material/Divider";
import CircularProgress from "@mui/material/CircularProgress";

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

function handleOidcLogin() {
  window.location.href = "/api/auth/oidc/start";
}

function loginReducer(state, action) {
  switch (action.type) {
    case "SUBMIT": return { ...state, loading: true, error: "", resetHint: "" };
    case "ERROR": return { ...state, loading: false, error: action.error, resetHint: action.resetHint || "", retryAfter: action.retryAfter || 0 };
    case "DONE": return { ...state, loading: false };
    case "MUST_CHANGE": return { ...state, loading: false, mustChangePassword: true, error: "" };
    case "RESET": return { ...state, loading: false, mustChangePassword: false, error: "", resetHint: "" };
    case "TICK": return { ...state, retryAfter: state.retryAfter > 0 ? state.retryAfter - 1 : 0 };
    default: return state;
  }
}

export default function MasukClient({ initialAuth }) {
  const [password, setPassword] = useState("");
  const [state, dispatch] = useReducer(loginReducer, { error: "", resetHint: "", retryAfter: 0, loading: false, mustChangePassword: false });
  const { error, resetHint, retryAfter, loading, mustChangePassword } = state;
  const [newPassword, setNewPassword] = useState("");
  const hasPassword = initialAuth?.hasPassword ?? null;
  const authMode = initialAuth?.authMode || "password";
  const oidcConfigured = initialAuth?.oidcConfigured || false;
  const oidcLoginLabel = initialAuth?.oidcLoginLabel || "Masuk dengan OIDC";
  const router = useRouter();

  useEffect(() => {
    if (retryAfter <= 0) return;
    const id = setInterval(() => dispatch({ type: "TICK" }), 1000);
    return () => clearInterval(id);
  }, [retryAfter]);

  if (initialAuth?.requireLogin === false) {
    router.push("/dashboard");
    return null;
  }

  const handleLogin = async (e) => {
    e.preventDefault();
    dispatch({ type: "SUBMIT" });

    try {
      const res = await fetch("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ password }),
      });

      const data = await res.json().catch(() => ({}));
      if (data.mustChangePassword) {
        // Remote login with the new-installation default: the server handed us a
        // password-change-only grant, so the operator can replace it here.
        dispatch({ type: "MUST_CHANGE" });
        return;
      }
      if (res.ok) {
        router.push("/dashboard");
        router.refresh();
      } else {
        dispatch({ type: "ERROR", error: data.error || "Password salah", resetHint: data.resetHint, retryAfter: data.retryAfter ? Number(data.retryAfter) : 0 });
      }
    } catch (err) {
      dispatch({ type: "ERROR", error: "Terjadi kesalahan. Silakan coba lagi." });
    }
  };

  const handleChangePassword = async (e) => {
    e.preventDefault();
    dispatch({ type: "SUBMIT" });
    try {
      const res = await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ currentPassword: password, newPassword }),
      });
      const data = await res.json().catch(() => ({}));
      if (res.ok) {
        dispatch({ type: "RESET" });
        router.push("/dashboard");
        router.refresh();
        return;
      }
      dispatch({ type: "ERROR", error: data.error || "Gagal mengganti password" });
    } catch (err) {
      dispatch({ type: "ERROR", error: "Terjadi kesalahan. Silakan coba lagi." });
    }
  };

  const oidcAvailable = oidcConfigured && ["oidc", "both"].includes(authMode);
  const passwordAvailable = authMode !== "oidc" || !oidcConfigured;

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
      component="main"
      role="main"
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
      <Box className="landing-grid" aria-hidden="true" sx={{ position: "absolute", inset: 0, pointerEvents: "none" }} />
      <Box sx={{ position: "relative", zIndex: 10, width: "100%", maxWidth: 448 }}>
        <Box sx={{ textAlign: "center", mb: 4 }}>
          <Box sx={{ display: "flex", justifyContent: "center", mb: 2 }}>
            <Box component="svg" viewBox="0 0 32 32" sx={{ width: 48, height: 48 }} fill="none">
              <path d="M16 5L22 14L16 27L10 14L16 5Z" fill="#6366f1" opacity="0.9" />
              <path d="M10 14L16 27L10 20L6 14H10Z" fill="#6366f1" opacity="0.6" />
              <path d="M22 14L16 27L22 20L26 14H22Z" fill="#6366f1" opacity="0.6" />
              <circle cx="16" cy="9" r="2" fill="#6366f1" />
            </Box>
          </Box>
          <Typography variant="h4" sx={{ fontWeight: 700, color: "primary.main", mb: 1 }}>
            VansAI
          </Typography>
          <Typography variant="body2" sx={{ color: "text.secondary" }}>
            {authMode === "oidc" && oidcConfigured
              ? "Masuk dengan OIDC provider untuk mengakses dashboard"
              : "Masukkan password untuk mengakses dashboard"}
          </Typography>
        </Box>

        <Card>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            {oidcAvailable && (
              <Button type="button" variant="primary" fullWidth onClick={handleOidcLogin}>
                {oidcLoginLabel}
              </Button>
            )}

            {oidcAvailable && passwordAvailable && <Divider sx={{ borderColor: "divider" }} />}

            {passwordAvailable && !mustChangePassword ? (
              <Box component="form" onSubmit={handleLogin} sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
                {((authMode === "oidc" && !oidcConfigured) || (authMode === "both" && !oidcConfigured)) && (
                  <Typography variant="caption" sx={{ color: "warning.main", textAlign: "center" }}>
                    OIDC login aktif, tapi issuer/client belum dikonfigurasi. Password login masih tersedia.
                  </Typography>
                )}

                {authMode === "both" && oidcConfigured && (
                  <Typography variant="caption" sx={{ color: "text.secondary", textAlign: "center" }}>
                    Password dan OIDC login keduanya aktif.
                  </Typography>
                )}

                <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                  <Typography component="label" htmlFor="masuk-password" variant="body2" sx={{ fontWeight: 600, color: "text.primary" }}>
                    Password
                  </Typography>
                  <Input
                    id="masuk-password"
                    type="password"
                    placeholder="Masukkan password"
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
                      Terkunci. Coba lagi dalam <Box component="span" sx={{ fontFamily: "ui-monospace, monospace" }}>{retryAfter}s</Box>.
                    </Typography>
                  )}
                  {resetHint && (
                    <Typography variant="caption" sx={{ color: "text.secondary" }}>
                      Lupa password? Buka <Code>vansrouter</Code> CLI di host → <b>Settings</b> → <b>Set/Replace Dashboard Password</b>.
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
                  {retryAfter > 0 ? `Tunggu ${retryAfter}s` : "Masuk"}
                </Button>

                {hasPassword === false && (
                  <Typography variant="caption" sx={{ textAlign: "center", color: "warning.main" }}>
                    Password default instalasi baru: <Code>123456</Code>. Ganti sebelum dashboard dipublikasikan.
                  </Typography>
                )}
              </Box>
            ) : null}

            {mustChangePassword ? (
              <Box component="form" onSubmit={handleChangePassword} sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
                <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                  <Typography component="label" htmlFor="ganti-password-baru" variant="body2" sx={{ fontWeight: 600, color: "text.primary" }}>
                    Password baru
                  </Typography>
                  <Input
                    id="ganti-password-baru"
                    type="password"
                    placeholder="Minimal 12 karakter"
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                    required
                    minLength={12}
                    autoFocus
                  />
                  <Typography variant="caption" sx={{ color: "text.secondary" }}>
                    Password default hanya berlaku di komputer ini. Ganti dengan password strong (minimal 12 karakter) sebelum melanjutkan.
                  </Typography>
                  {error && (
                    <Typography variant="caption" sx={{ color: "error.main" }}>
                      {error}
                    </Typography>
                  )}
                </Box>
                <Button type="submit" variant="primary" fullWidth loading={loading} disabled={newPassword.length < 12}>
                  Simpan &amp; Lanjutkan
                </Button>
              </Box>
            ) : null}
          </Box>
        </Card>
      </Box>
    </Box>
  );
}
