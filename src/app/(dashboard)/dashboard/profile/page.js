"use client";

import { useState, useEffect, useRef } from "react";
import Box from "@mui/material/Box";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import { Card, Button, Toggle, Input } from "@/shared/components";
import Modal, { ConfirmModal } from "@/shared/components/Modal";
import LanguageSwitcher from "@/shared/components/LanguageSwitcher";
import { useTheme } from "@/shared/hooks/useTheme";
import { APP_CONFIG } from "@/shared/constants/config";
import { LOCALE_COOKIE, normalizeLocale } from "@/i18n/config";
import { LOCALE_FLAGS } from "@/shared/constants/locales";

function MsIcon({ children, sx }) {
  return (
    <Icon className="material-symbols-outlined" baseClassName="" sx={sx}>
      {children}
    </Icon>
  );
}

/** Card section header: tinted icon box + title (matches the old Tailwind header). */
function CardHeader({ icon, title, subtitle, color = "#E56A4A", iconFontSize = 20, sx }) {
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, mb: 2, ...sx }}>
      <Box sx={{ p: 1, borderRadius: "8px", bgcolor: `${color}1a`, color, flexShrink: 0, display: "grid", placeItems: "center" }}>
        <MsIcon sx={{ fontSize: iconFontSize }}>{icon}</MsIcon>
      </Box>
      <Box sx={{ minWidth: 0 }}>
        <Typography sx={{ fontSize: 16, fontWeight: 600 }}>{title}</Typography>
        {subtitle && <Typography sx={{ fontSize: 12, color: "text.secondary" }}>{subtitle}</Typography>}
      </Box>
    </Box>
  );
}

/** Row: label block on the left, control on the right (stacked on mobile). */
function SettingRow({ title, description, control, sx }) {
  return (
    <Box sx={{ display: "flex", alignItems: { xs: "flex-start", sm: "center" }, justifyContent: "space-between", gap: 2, ...sx }}>
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography sx={{ fontWeight: 500, fontSize: { xs: 14, sm: 16 } }}>{title}</Typography>
        {description && (
          <Typography sx={{ fontSize: { xs: 12, sm: 14 }, color: "text.secondary" }}>{description}</Typography>
        )}
      </Box>
      {control}
    </Box>
  );
}

/** Full-width on mobile, auto on desktop, matching the old `w-full sm:w-auto`. */
function ResponsiveButton({ children, ...props }) {
  return (
    <Box sx={{ width: { xs: "100%", sm: "auto" } }}>
      <Button fullWidth {...props}>{children}</Button>
    </Box>
  );
}

function getLocaleFromCookie() {
  if (typeof document === "undefined") return "en";
  const cookie = document.cookie
    .split(";")
    .find((c) => c.trim().startsWith(`${LOCALE_COOKIE}=`));
  const value = cookie ? decodeURIComponent(cookie.split("=")[1]) : "en";
  return normalizeLocale(value);
}

export default function ProfilePage() {
  const { theme, setTheme, isDark } = useTheme();
  const [locale, setLocale] = useState("en");
  const [langOpen, setLangOpen] = useState(false);
  const [shutdownOpen, setShutdownOpen] = useState(false);
  const [isShuttingDown, setIsShuttingDown] = useState(false);
  const [settings, setSettings] = useState({ fallbackStrategy: "fill-first" });
  const [loading, setLoading] = useState(true);
  const [passwords, setPasswords] = useState({ current: "", new: "", confirm: "" });
  const [passStatus, setPassStatus] = useState({ type: "", message: "" });
  const [passLoading, setPassLoading] = useState(false);
  const [dbLoading, setDbLoading] = useState(false);
  const [dbStatus, setDbStatus] = useState({ type: "", message: "" });
  const [dbAuth, setDbAuth] = useState({ open: false, mode: "", password: "" });
  const pendingImportRef = useRef(null);
  const [oidcForm, setOidcForm] = useState({
    authMode: "password",
    oidcIssuerUrl: "",
    oidcClientId: "",
    oidcScopes: "openid profile email",
    oidcLoginLabel: "Sign in with OIDC",
  });
  const [oidcClientSecret, setOidcClientSecret] = useState("");
  const [oidcStatus, setOidcStatus] = useState({ type: "", message: "" });
  const [oidcLoading, setOidcLoading] = useState(false);
  const [oidcTestLoading, setOidcTestLoading] = useState(false);
  const [oidcTestStatus, setOidcTestStatus] = useState({ type: "", message: "" });
  const [oidcRedirectUri, setOidcRedirectUri] = useState("/api/auth/oidc/callback");
  const [oidcExpanded, setOidcExpanded] = useState(false);
  const importFileRef = useRef(null);
  const [proxyForm, setProxyForm] = useState({
    outboundProxyEnabled: false,
    outboundProxyUrl: "",
    outboundNoProxy: "",
  });
  const [proxyStatus, setProxyStatus] = useState({ type: "", message: "" });
  const [proxyLoading, setProxyLoading] = useState(false);
  const [proxyTestLoading, setProxyTestLoading] = useState(false);

  /* eslint-disable react-hooks/set-state-in-effect --
     Locale sync on mount and bootstrap fetch of /api/settings. setState is
     intentional; the effect runs once or when langOpen toggles. */
  useEffect(() => {
    setLocale(getLocaleFromCookie());
  }, [langOpen]);

  useEffect(() => {
    fetch("/api/settings")
      .then((res) => res.json())
      .then((data) => {
        setSettings(data);
        setOidcForm({
          authMode: data?.authMode || "password",
          oidcIssuerUrl: data?.oidcIssuerUrl || "",
          oidcClientId: data?.oidcClientId || "",
          oidcScopes: data?.oidcScopes || "openid profile email",
          oidcLoginLabel: data?.oidcLoginLabel || "Sign in with OIDC",
        });
        setOidcClientSecret("");
        if (data?.authMode === "oidc" || data?.authMode === "both") setOidcExpanded(true);
        setProxyForm({
          outboundProxyEnabled: data?.outboundProxyEnabled === true,
          outboundProxyUrl: data?.outboundProxyUrl || "",
          outboundNoProxy: data?.outboundNoProxy || "",
        });
        setLoading(false);
      })
      .catch((err) => {
        console.error("Failed to fetch settings:", err);
        setLoading(false);
      });
  }, []);

  useEffect(() => {
    if (typeof window !== "undefined") {
      setOidcRedirectUri(`${window.location.origin}/api/auth/oidc/callback`);
    }
  }, []);

  const updateOutboundProxy = async (e) => {
    e.preventDefault();
    if (settings.outboundProxyEnabled !== true) return;
    setProxyLoading(true);
    setProxyStatus({ type: "", message: "" });

    try {
      const res = await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          outboundProxyUrl: proxyForm.outboundProxyUrl,
          outboundNoProxy: proxyForm.outboundNoProxy,
        }),
      });

      const data = await res.json();
      if (res.ok) {
        setSettings((prev) => ({ ...prev, ...data }));
        setProxyStatus({ type: "success", message: "Proxy settings applied" });
      } else {
        setProxyStatus({ type: "error", message: data.error || "Failed to update proxy settings" });
      }
    } catch (err) {
      setProxyStatus({ type: "error", message: "An error occurred" });
    } finally {
      setProxyLoading(false);
    }
  };

  const testOutboundProxy = async () => {
    if (settings.outboundProxyEnabled !== true) return;

    const proxyUrl = (proxyForm.outboundProxyUrl || "").trim();
    if (!proxyUrl) {
      setProxyStatus({ type: "error", message: "Please enter a Proxy URL to test" });
      return;
    }

    setProxyTestLoading(true);
    setProxyStatus({ type: "", message: "" });

    try {
      const res = await fetch("/api/settings/proxy-test", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ proxyUrl }),
      });

      const data = await res.json();
      if (res.ok && data?.ok) {
        setProxyStatus({
          type: "success",
          message: `Proxy test OK (${data.status}) in ${data.elapsedMs}ms`,
        });
      } else {
        setProxyStatus({
          type: "error",
          message: data?.error || "Proxy test failed",
        });
      }
    } catch (err) {
      setProxyStatus({ type: "error", message: "An error occurred" });
    } finally {
      setProxyTestLoading(false);
    }
  };

  const updateOutboundProxyEnabled = async (outboundProxyEnabled) => {
    setProxyLoading(true);
    setProxyStatus({ type: "", message: "" });

    try {
      const res = await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ outboundProxyEnabled }),
      });

      const data = await res.json();
      if (res.ok) {
        setSettings((prev) => ({ ...prev, ...data }));
        setProxyForm((prev) => ({ ...prev, outboundProxyEnabled: data?.outboundProxyEnabled === true }));
        setProxyStatus({
          type: "success",
          message: outboundProxyEnabled ? "Proxy enabled" : "Proxy disabled",
        });
      } else {
        setProxyStatus({ type: "error", message: data.error || "Failed to update proxy settings" });
      }
    } catch (err) {
      setProxyStatus({ type: "error", message: "An error occurred" });
    } finally {
      setProxyLoading(false);
    }
  };

  const handlePasswordChange = async (e) => {
    e.preventDefault();
    if (passwords.new !== passwords.confirm) {
      setPassStatus({ type: "error", message: "Passwords do not match" });
      return;
    }

    setPassLoading(true);
    setPassStatus({ type: "", message: "" });

    try {
      const res = await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          currentPassword: passwords.current,
          newPassword: passwords.new,
        }),
      });

      const data = await res.json();

      if (res.ok) {
        setPassStatus({ type: "success", message: "Password updated successfully" });
        setPasswords({ current: "", new: "", confirm: "" });
      } else {
        setPassStatus({ type: "error", message: data.error || "Failed to update password" });
      }
    } catch (err) {
      setPassStatus({ type: "error", message: "An error occurred" });
    } finally {
      setPassLoading(false);
    }
  };

  const updateFallbackStrategy = async (strategy) => {
    try {
      const res = await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ fallbackStrategy: strategy }),
      });
      if (res.ok) {
        setSettings(prev => ({ ...prev, fallbackStrategy: strategy }));
      }
    } catch (err) {
      console.error("Failed to update settings:", err);
    }
  };

  const updateComboStrategy = async (strategy) => {
    try {
      const res = await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ comboStrategy: strategy }),
      });
      if (res.ok) {
        setSettings(prev => ({ ...prev, comboStrategy: strategy }));
      }
    } catch (err) {
      console.error("Failed to update combo strategy:", err);
    }
  };

  const updateStickyLimit = async (limit) => {
    const numLimit = parseInt(limit);
    if (isNaN(numLimit) || numLimit < 1) return;

    try {
      const res = await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ stickyRoundRobinLimit: numLimit }),
      });
      if (res.ok) {
        setSettings(prev => ({ ...prev, stickyRoundRobinLimit: numLimit }));
      }
    } catch (err) {
      console.error("Failed to update sticky limit:", err);
    }
  };

  const updateComboStickyLimit = async (limit) => {
    const numLimit = parseInt(limit);
    if (isNaN(numLimit) || numLimit < 1) return;

    try {
      const res = await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ comboStickyRoundRobinLimit: numLimit }),
      });
      if (res.ok) {
        setSettings(prev => ({ ...prev, comboStickyRoundRobinLimit: numLimit }));
      }
    } catch (err) {
      console.error("Failed to update combo sticky limit:", err);
    }
  };

  const updateRequireLogin = async (requireLogin) => {
    try {
      const res = await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ requireLogin }),
      });
      if (res.ok) {
        setSettings(prev => ({ ...prev, requireLogin }));
      }
    } catch (err) {
      console.error("Failed to update require login:", err);
    }
  };

  const updateOidcForm = (field, value) => {
    setOidcForm((prev) => ({ ...prev, [field]: value }));
  };

  const saveOidcSettings = async (authMode = oidcForm.authMode || "password") => {
    const issuerUrl = oidcForm.oidcIssuerUrl.trim();
    const clientId = oidcForm.oidcClientId.trim();
    const scopes = oidcForm.oidcScopes.trim();
    const loginLabel = oidcForm.oidcLoginLabel.trim();
    const secret = oidcClientSecret.trim();

    if (authMode !== "password" && (!issuerUrl || !clientId || !secret) && !settings.oidcConfigured) {
      setOidcStatus({ type: "error", message: "Issuer URL, client ID, and client secret are required to enable OIDC." });
      return;
    }

    setOidcLoading(true);
    setOidcStatus({ type: "", message: "" });
    setOidcTestStatus({ type: "", message: "" });

    try {
      const payload = {
        authMode,
        oidcIssuerUrl: issuerUrl,
        oidcClientId: clientId,
        oidcScopes: scopes || "openid profile email",
        oidcLoginLabel: loginLabel || "Sign in with OIDC",
      };
      if (secret) {
        payload.oidcClientSecret = secret;
      }

      const res = await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });

      const data = await res.json();
      if (res.ok) {
        setSettings((prev) => ({ ...prev, ...data }));
        setOidcForm({
          authMode: data?.authMode || authMode,
          oidcIssuerUrl: data?.oidcIssuerUrl || issuerUrl,
          oidcClientId: data?.oidcClientId || clientId,
          oidcScopes: data?.oidcScopes || scopes || "openid profile email",
          oidcLoginLabel: data?.oidcLoginLabel || loginLabel || "Sign in with OIDC",
        });
        setOidcClientSecret("");
        setOidcStatus({
          type: "success",
          message:
            authMode === "oidc"
              ? "OIDC login enabled"
              : authMode === "both"
                ? "Password and OIDC login enabled"
                : "OIDC settings saved",
        });
      } else {
        setOidcStatus({ type: "error", message: data.error || "Failed to save OIDC settings" });
      }
    } catch (err) {
      setOidcStatus({ type: "error", message: "An error occurred" });
    } finally {
      setOidcLoading(false);
    }
  };

  const testOidcConnection = async () => {
    const issuerUrl = oidcForm.oidcIssuerUrl.trim();
    const clientId = oidcForm.oidcClientId.trim();
    const scopes = oidcForm.oidcScopes.trim();
    const secret = oidcClientSecret.trim();

    if (!issuerUrl || !clientId) {
      setOidcTestStatus({ type: "error", message: "Issuer URL and client ID are required to test the connection." });
      return;
    }

    setOidcTestLoading(true);
    setOidcStatus({ type: "", message: "" });
    setOidcTestStatus({ type: "", message: "" });

    try {
      const saveRes = await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          authMode: oidcForm.authMode || settings.authMode || "password",
          oidcIssuerUrl: issuerUrl,
          oidcClientId: clientId,
          oidcScopes: scopes || "openid profile email",
          oidcLoginLabel: oidcForm.oidcLoginLabel.trim() || "Sign in with OIDC",
          ...(secret ? { oidcClientSecret: secret } : {}),
        }),
      });

      const saved = await saveRes.json().catch(() => ({}));
      if (!saveRes.ok) {
        setOidcTestStatus({
          type: "error",
          message: saved.error || "Failed to save OIDC settings before testing",
        });
        return;
      }

      const res = await fetch("/api/auth/oidc/test", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          issuerUrl: saved.oidcIssuerUrl || issuerUrl,
          clientId: saved.oidcClientId || clientId,
          scopes: saved.oidcScopes || scopes || "openid profile email",
        }),
      });

      const data = await res.json().catch(() => ({}));
      if (res.ok && data?.ok) {
        const statusMessage = data.clientSecretTested
          ? data.clientSecretValid === true
            ? `Connection OK. Discovery loaded from ${data.issuerUrl}. Client secret validated too.`
            : `Connection OK. Discovery loaded from ${data.issuerUrl}. Client secret was not checked.`
          : `Connection OK. Discovery loaded from ${data.issuerUrl}.`;
        setOidcTestStatus({
          type: "success",
          message: statusMessage,
        });
      } else {
        setOidcTestStatus({ type: "error", message: data.error || "OIDC connection test failed" });
      }
    } catch (err) {
      setOidcTestStatus({ type: "error", message: "An error occurred" });
    } finally {
      setOidcTestLoading(false);
    }
  };

  const updateObservabilityEnabled = async (enabled) => {
    try {
      const res = await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enableObservability: enabled }),
      });
      if (res.ok) {
        setSettings(prev => ({ ...prev, enableObservability: enabled }));
      }
    } catch (err) {
      console.error("Failed to update enableObservability:", err);
    }
  };

  const reloadSettings = async () => {
    try {
      const res = await fetch("/api/settings");
      if (!res.ok) return;
      const data = await res.json();
      setSettings(data);
    } catch (err) {
      console.error("Failed to reload settings:", err);
    }
  };

  const handleExportDatabase = async (password) => {
    setDbLoading(true);
    setDbStatus({ type: "", message: "" });
    try {
      const res = await fetch("/api/settings/database", {
        headers: { "x-9r-password": password },
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || "Failed to export database");
      }

      const payload = await res.json();
      const content = JSON.stringify(payload, null, 2);
      const blob = new Blob([content], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      const stamp = new Date().toISOString().replace(/[.:]/g, "-");
      anchor.href = url;
      anchor.download = `9router-backup-${stamp}.json`;
      document.body.appendChild(anchor);
      anchor.click();
      document.body.removeChild(anchor);
      URL.revokeObjectURL(url);

      setDbStatus({ type: "success", message: "Database backup downloaded" });
    } catch (err) {
      setDbStatus({ type: "error", message: err.message || "Failed to export database" });
    } finally {
      setDbLoading(false);
    }
  };

  const handleImportDatabase = (event) => {
    const file = event.target.files?.[0];
    if (importFileRef.current) importFileRef.current.value = "";
    if (!file) return;
    pendingImportRef.current = file;
    setDbStatus({ type: "", message: "" });
    setDbAuth({ open: true, mode: "import", password: "" });
  };

  const runImportDatabase = async (password) => {
    const file = pendingImportRef.current;
    if (!file) return;
    setDbLoading(true);
    try {
      const raw = await file.text();
      const payload = JSON.parse(raw);

      const res = await fetch("/api/settings/database", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...payload, password }),
      });

      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        throw new Error(data.error || "Failed to import database");
      }

      await reloadSettings();
      setDbStatus({ type: "success", message: "Database imported successfully" });
    } catch (err) {
      setDbStatus({ type: "error", message: err.message || "Invalid backup file" });
    } finally {
      pendingImportRef.current = null;
      setDbLoading(false);
    }
  };

  // Confirm password modal, then run export or import.
  const handleDbAuthConfirm = async () => {
    const { mode, password } = dbAuth;
    setDbAuth({ open: false, mode: "", password: "" });
    if (mode === "export") await handleExportDatabase(password);
    else if (mode === "import") await runImportDatabase(password);
  };

  const observabilityEnabled = settings.enableObservability === true;

  const handleShutdown = async () => {
    setIsShuttingDown(true);
    try {
      await fetch("/api/version/shutdown", { method: "POST" });
    } catch (e) {
      // Expected to fail as server shuts down; ignore error
    }
    setIsShuttingDown(false);
    setShutdownOpen(false);
  };

  const handleLogout = async () => {
    try {
      const res = await fetch("/api/auth/logout", { method: "POST" });
      if (res.ok) {
        window.location.assign("/login");
      }
    } catch (err) {
      console.error("Failed to logout:", err);
    }
  };

  const statusColor = (type) => (type === "error" ? "#dc2626" : "#16a34a");

  return (
    <Box sx={{ maxWidth: 672, mx: "auto", px: { xs: 2, sm: 0 } }}>
      <Stack spacing={3}>
        {/* Local Mode Info */}
        <Card>
          <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, alignItems: { sm: "center" }, justifyContent: { sm: "space-between" }, gap: 2, mb: 2 }}>
            <Box sx={{ display: "flex", alignItems: "center", gap: { xs: 1.5, sm: 2 } }}>
              <Box sx={{ width: { xs: 40, sm: 48 }, height: { xs: 40, sm: 48 }, borderRadius: "8px", bgcolor: "rgba(34,197,94,0.10)", color: "#22c55e", display: "flex", alignItems: "center", justifyContent: "center", flexShrink: 0 }}>
                <MsIcon sx={{ fontSize: { xs: 20, sm: 24 } }}>computer</MsIcon>
              </Box>
              <Box>
                <Typography sx={{ fontSize: { xs: 18, sm: 20 }, fontWeight: 600 }}>Local Mode</Typography>
                <Typography sx={{ fontSize: 14, color: "text.secondary" }}>Running on your machine</Typography>
              </Box>
            </Box>
            <Box
              sx={(theme) => ({
                display: "inline-flex",
                p: 0.5,
                borderRadius: "8px",
                bgcolor: theme.palette.mode === "dark" ? "rgba(255,255,255,0.05)" : "rgba(0,0,0,0.05)",
                width: { xs: "100%", sm: "auto" },
              })}
            >
              {["light", "dark", "system"].map((option) => (
                <Box
                  key={option}
                  component="button"
                  type="button"
                  onClick={() => setTheme(option)}
                  sx={(theme) => ({
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    gap: { xs: 0.5, sm: 0.75 },
                    px: { xs: 1, sm: 1.5 },
                    py: 0.75,
                    borderRadius: "6px",
                    fontWeight: 500,
                    transition: "all 150ms",
                    flex: { xs: 1, sm: "initial" },
                    cursor: "pointer",
                    border: 0,
                    bgcolor: theme === option ? (theme.palette.mode === "dark" ? "rgba(255,255,255,0.10)" : "#ffffff") : "transparent",
                    color: theme === option ? theme.palette.text.primary : theme.palette.text.secondary,
                    boxShadow: theme === option ? "0 1px 2px rgba(0,0,0,0.05)" : "none",
                    "&:hover": { color: theme.palette.text.primary },
                  })}
                >
                  <MsIcon sx={{ fontSize: 18 }}>
                    {option === "light" ? "light_mode" : option === "dark" ? "dark_mode" : "contrast"}
                  </MsIcon>
                  <Box component="span" sx={{ textTransform: "capitalize", fontSize: { xs: 12, sm: 14 } }}>{option}</Box>
                </Box>
              ))}
            </Box>
          </Box>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5, pt: 2, borderTop: 1, borderColor: "divider" }}>
            <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, alignItems: { sm: "center" }, justifyContent: { sm: "space-between" }, p: 1.5, borderRadius: "8px", bgcolor: "background.default", border: 1, borderColor: "divider", gap: 1 }}>
              <Box>
                <Typography sx={{ fontWeight: 500, fontSize: { xs: 14, sm: 16 } }}>Database Location</Typography>
                <Typography sx={{ fontSize: { xs: 12, sm: 14 }, color: "text.secondary", fontFamily: "ui-monospace, monospace", wordBreak: "break-all" }}>
                  ~/.9router/db/data.sqlite
                </Typography>
              </Box>
            </Box>
            <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1 }}>
              <ResponsiveButton
                variant="secondary"
                icon="download"
                onClick={() => setDbAuth({ open: true, mode: "export", password: "" })}
                loading={dbLoading}
              >
                Download Backup
              </ResponsiveButton>
              <ResponsiveButton
                variant="outline"
                icon="upload"
                onClick={() => importFileRef.current?.click()}
                disabled={dbLoading}
              >
                Import Backup
              </ResponsiveButton>
              <Box
                component="input"
                ref={importFileRef}
                type="file"
                accept="application/json,.json"
                onChange={handleImportDatabase}
                sx={{ display: "none" }}
              />
            </Box>
            {dbStatus.message && (
              <Typography sx={{ fontSize: 14, color: statusColor(dbStatus.type) }}>
                {dbStatus.message}
              </Typography>
            )}
          </Box>
        </Card>

        {/* Language */}
        <Card>
          <CardHeader icon="language" title="Language" color="#3b82f6" />
          <Box
            component="button"
            onClick={() => setLangOpen(true)}
            data-i18n-skip="true"
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              width: "100%",
              p: 1.5,
              borderRadius: "8px",
              bgcolor: "background.default",
              border: 1,
              borderColor: "divider",
              cursor: "pointer",
              transition: "border-color 150ms",
              "&:hover": { borderColor: "rgba(229,106,74,0.5)" },
            }}
          >
            <Box component="span" sx={{ fontSize: 14, color: "text.secondary" }}>Display language</Box>
            <Box component="span" sx={{ fontSize: 24 }}>{LOCALE_FLAGS[locale] || "🌐"}</Box>
          </Box>
        </Card>

        {/* Security */}
        <Card>
          <CardHeader icon="shield" title="Security" color="#E56A4A" />
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            <SettingRow
              title="Require login"
              description="When ON, dashboard requires password. When OFF, access without login."
              control={
                <Toggle
                  checked={settings.requireLogin === true}
                  onChange={() => updateRequireLogin(!settings.requireLogin)}
                  disabled={loading}
                />
              }
            />
            {settings.requireLogin === true && (
              <Box component="form" onSubmit={handlePasswordChange} sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 2, borderTop: 1, borderColor: "divider" }}>
                {settings.hasPassword && (
                  <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                    <Typography component="label" sx={{ fontSize: { xs: 12, sm: 14 }, fontWeight: 500 }}>Current Password</Typography>
                    <Input
                      type="password"
                      placeholder="Enter current password"
                      value={passwords.current}
                      onChange={(e) => setPasswords({ ...passwords, current: e.target.value })}
                      required
                    />
                  </Box>
                )}
                <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" }, gap: 2 }}>
                  <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                    <Typography component="label" sx={{ fontSize: { xs: 12, sm: 14 }, fontWeight: 500 }}>New Password</Typography>
                    <Input
                      type="password"
                      placeholder="Enter new password"
                      value={passwords.new}
                      onChange={(e) => setPasswords({ ...passwords, new: e.target.value })}
                      required
                    />
                  </Box>
                  <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                    <Typography component="label" sx={{ fontSize: { xs: 12, sm: 14 }, fontWeight: 500 }}>Confirm New Password</Typography>
                    <Input
                      type="password"
                      placeholder="Confirm new password"
                      value={passwords.confirm}
                      onChange={(e) => setPasswords({ ...passwords, confirm: e.target.value })}
                      required
                    />
                  </Box>
                </Box>

                {passStatus.message && (
                  <Typography sx={{ fontSize: { xs: 12, sm: 14 }, color: statusColor(passStatus.type) }}>
                    {passStatus.message}
                  </Typography>
                )}

                <Box sx={{ pt: 1 }}>
                  <ResponsiveButton type="submit" variant="primary" loading={passLoading}>
                    {settings.hasPassword ? "Update Password" : "Set Password"}
                  </ResponsiveButton>
                </Box>
              </Box>
            )}
          </Box>
        </Card>

        {/* OIDC */}
        <Card>
          <Box
            component="button"
            type="button"
            onClick={() => setOidcExpanded((v) => !v)}
            sx={{ width: "100%", display: "flex", alignItems: "center", gap: 1.5, textAlign: "left", background: "none", border: 0, p: 0, cursor: "pointer", color: "inherit" }}
          >
            <Box sx={{ p: 1, borderRadius: "8px", bgcolor: "rgba(99,102,241,0.10)", color: "#6366f1", flexShrink: 0, display: "grid", placeItems: "center" }}>
              <MsIcon sx={{ fontSize: 20 }}>lock_open</MsIcon>
            </Box>
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Typography sx={{ fontSize: { xs: 16, sm: 18 }, fontWeight: 600 }}>OIDC Dashboard Login</Typography>
              <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
                {settings.authMode === "oidc" ? "OIDC active" : settings.authMode === "both" ? "Password + OIDC active" : "Optional SSO via Authentik/Keycloak/Google"}
              </Typography>
            </Box>
            <MsIcon sx={{ color: "text.secondary", flexShrink: 0 }}>
              {oidcExpanded ? "expand_less" : "expand_more"}
            </MsIcon>
          </Box>
          {oidcExpanded && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2, mt: 2 }}>
            <Typography sx={{ fontSize: { xs: 12, sm: 14 }, color: "text.secondary" }}>
              Use Authentik or any OIDC provider to sign in to the dashboard. You can enable password-only, OIDC-only, or both for the dashboard; model API access still uses API keys.
            </Typography>

            <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
              <Typography component="label" sx={{ fontWeight: 500, fontSize: { xs: 14, sm: 16 } }}>Auth Mode</Typography>
              <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "repeat(3, 1fr)" }, gap: 1 }}>
                {[
                  {
                    value: "password",
                    title: "Password only",
                    desc: "Keep the legacy password login.",
                  },
                  {
                    value: "oidc",
                    title: "OIDC only",
                    desc: "Require OIDC for dashboard access.",
                  },
                  {
                    value: "both",
                    title: "Both",
                    desc: "Allow either password or OIDC.",
                  },
                ].map((option) => {
                  const active = oidcForm.authMode === option.value;
                  return (
                    <Box
                      key={option.value}
                      component="button"
                      type="button"
                      onClick={() => updateOidcForm("authMode", option.value)}
                      disabled={loading || oidcLoading}
                      sx={(theme) => ({
                        textAlign: "left",
                        borderRadius: "8px",
                        border: 1,
                        p: 1.5,
                        cursor: "pointer",
                        transition: "border-color 150ms, background-color 150ms",
                        borderColor: active ? "primary.main" : theme.palette.divider,
                        bgcolor: active
                          ? "rgba(229,106,74,0.05)"
                          : theme.palette.mode === "dark"
                            ? theme.palette.background.default
                            : theme.palette.background.default,
                        "&:hover": { bgcolor: theme.palette.mode === "dark" ? "rgba(255,255,255,0.05)" : "rgba(0,0,0,0.05)" },
                      })}
                    >
                      <Typography sx={{ fontWeight: 500, fontSize: { xs: 14, sm: 16 } }}>{option.title}</Typography>
                      <Typography sx={{ fontSize: { xs: 12, sm: 14 }, color: "text.secondary", mt: 0.5 }}>{option.desc}</Typography>
                    </Box>
                  );
                })}
              </Box>
            </Box>

            <Box sx={{ display: "grid", gridTemplateColumns: "1fr", gap: 2 }}>
              <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                <Typography component="label" sx={{ fontWeight: 500, fontSize: { xs: 14, sm: 16 } }}>Issuer URL</Typography>
                <Input
                  placeholder="https://auth.example.com/application/o/9router/"
                  value={oidcForm.oidcIssuerUrl}
                  onChange={(e) => updateOidcForm("oidcIssuerUrl", e.target.value)}
                  disabled={loading || oidcLoading}
                />
              </Box>

              <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                <Typography component="label" sx={{ fontWeight: 500, fontSize: { xs: 14, sm: 16 } }}>Client ID</Typography>
                <Input
                  placeholder="9router-dashboard"
                  value={oidcForm.oidcClientId}
                  onChange={(e) => updateOidcForm("oidcClientId", e.target.value)}
                  disabled={loading || oidcLoading}
                />
              </Box>

              <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                <Typography component="label" sx={{ fontWeight: 500, fontSize: { xs: 14, sm: 16 } }}>Client Secret</Typography>
                <Input
                  type="password"
                  placeholder="Leave blank to keep existing secret"
                  value={oidcClientSecret}
                  onChange={(e) => setOidcClientSecret(e.target.value)}
                  disabled={loading || oidcLoading}
                />
                <Typography sx={{ fontSize: { xs: 12, sm: 14 }, color: "text.secondary" }}>This value is write-only after saving.</Typography>
              </Box>

              <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                <Typography component="label" sx={{ fontWeight: 500, fontSize: { xs: 14, sm: 16 } }}>Scopes</Typography>
                <Input
                  placeholder="openid profile email"
                  value={oidcForm.oidcScopes}
                  onChange={(e) => updateOidcForm("oidcScopes", e.target.value)}
                  disabled={loading || oidcLoading}
                />
              </Box>

              <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                <Typography component="label" sx={{ fontWeight: 500, fontSize: { xs: 14, sm: 16 } }}>Login Button Label</Typography>
                <Input
                  placeholder="Sign in with OIDC"
                  value={oidcForm.oidcLoginLabel}
                  onChange={(e) => updateOidcForm("oidcLoginLabel", e.target.value)}
                  disabled={loading || oidcLoading}
                />
              </Box>
            </Box>

            <Box sx={{ borderRadius: "8px", border: 1, borderColor: "divider", bgcolor: "background.default", p: 1.5, fontSize: { xs: 12, sm: 14 }, color: "text.secondary" }}>
              <Typography sx={{ fontWeight: 500, color: "text.primary", mb: 0.5 }}>Redirect URI</Typography>
              <Box component="code" sx={{ display: "block", wordBreak: "break-all", fontFamily: "ui-monospace, monospace" }}>{oidcRedirectUri}</Box>
            </Box>

            <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1, pt: 1, borderTop: 1, borderColor: "divider" }}>
              <ResponsiveButton type="button" variant="primary" loading={oidcLoading} onClick={() => saveOidcSettings()}>
                Save auth mode
              </ResponsiveButton>
              <ResponsiveButton type="button" variant="outline" loading={oidcTestLoading} onClick={testOidcConnection}>
                Test connection
              </ResponsiveButton>
            </Box>

            {oidcTestStatus.message && (
              <Typography sx={{ fontSize: { xs: 12, sm: 14 }, color: statusColor(oidcTestStatus.type) }}>
                {oidcTestStatus.message}
              </Typography>
            )}

            {oidcStatus.message && (
              <Typography sx={{ fontSize: { xs: 12, sm: 14 }, color: statusColor(oidcStatus.type) }}>
                {oidcStatus.message}
              </Typography>
            )}

            {settings.authMode === "oidc" && (
              <Typography sx={{ fontSize: { xs: 12, sm: 14 }, color: "#d97706" }}>
                OIDC login is currently active. Password login is disabled until you switch back.
              </Typography>
            )}

            {settings.authMode === "both" && (
              <Typography sx={{ fontSize: { xs: 12, sm: 14 }, color: "#d97706" }}>
                Password and OIDC login are both active.
              </Typography>
            )}
          </Box>
          )}
        </Card>

        {/* Routing Preferences */}
        <Card>
          <CardHeader icon="route" title="Routing Strategy" color="#3b82f6" />
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            <SettingRow
              title="Round Robin"
              description="Cycle through accounts to distribute load"
              control={
                <Toggle
                  checked={settings.fallbackStrategy === "round-robin"}
                  onChange={() => updateFallbackStrategy(settings.fallbackStrategy === "round-robin" ? "fill-first" : "round-robin")}
                  disabled={loading}
                />
              }
            />

            {/* Sticky Round Robin Limit */}
            {settings.fallbackStrategy === "round-robin" && (
              <SettingRow
                title="Sticky Limit"
                description="Calls per account before switching"
                sx={{ pt: 1, borderTop: 1, borderColor: "divider" }}
                control={
                  <Box sx={{ width: { xs: 64, sm: 80 }, flexShrink: 0 }}>
                    <Input
                      type="number"
                      min="1"
                      max="10"
                      value={settings.stickyRoundRobinLimit || 3}
                      onChange={(e) => updateStickyLimit(e.target.value)}
                      disabled={loading}
                      inputClassName="text-center"
                    />
                  </Box>
                }
              />
            )}

            {/* Combo Round Robin */}
            <SettingRow
              title="Combo Round Robin"
              description="Cycle through providers in combos instead of always starting with first"
              sx={{ pt: 2, borderTop: 1, borderColor: "divider" }}
              control={
                <Toggle
                  checked={settings.comboStrategy === "round-robin"}
                  onChange={() => updateComboStrategy(settings.comboStrategy === "round-robin" ? "fallback" : "round-robin")}
                  disabled={loading}
                />
              }
            />

            {/* Combo Sticky Round Robin Limit */}
            {settings.comboStrategy === "round-robin" && (
              <SettingRow
                title="Combo Sticky Limit"
                description="Calls per combo model before switching"
                sx={{ pt: 1, borderTop: 1, borderColor: "divider" }}
                control={
                  <Box sx={{ width: 80 }}>
                    <Input
                      type="number"
                      min="1"
                      max="100"
                      value={settings.comboStickyRoundRobinLimit || 1}
                      onChange={(e) => updateComboStickyLimit(e.target.value)}
                      disabled={loading}
                      inputClassName="text-center"
                    />
                  </Box>
                }
              />
            )}

            <Typography sx={{ fontSize: 12, color: "text.secondary", fontStyle: "italic", pt: 1, borderTop: 1, borderColor: "divider" }}>
              {settings.fallbackStrategy === "round-robin"
                ? `Currently distributing requests across all available accounts with ${settings.stickyRoundRobinLimit || 3} calls per account.`
                : "Currently using accounts in priority order (Fill First)."}
              {settings.comboStrategy === "round-robin"
                ? ` Combos rotate after ${settings.comboStickyRoundRobinLimit || 1} call${(settings.comboStickyRoundRobinLimit || 1) === 1 ? "" : "s"} per model.`
                : " Combos always start with their first model."}
            </Typography>
          </Box>
        </Card>

        {/* Network */}
        <Card>
          <CardHeader icon="wifi" title="Network" color="#a855f7" />

          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            <SettingRow
              title="Outbound Proxy"
              description="Enable proxy for OAuth + provider outbound requests."
              control={
                <Toggle
                  checked={settings.outboundProxyEnabled === true}
                  onChange={() => updateOutboundProxyEnabled(!(settings.outboundProxyEnabled === true))}
                  disabled={loading || proxyLoading}
                />
              }
            />

            {settings.outboundProxyEnabled === true && (
              <Box component="form" onSubmit={updateOutboundProxy} sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 1, borderTop: 1, borderColor: "divider" }}>
                <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                  <Typography component="label" sx={{ fontWeight: 500, fontSize: { xs: 14, sm: 16 } }}>Proxy URL</Typography>
                  <Input
                    placeholder="http://127.0.0.1:7897"
                    value={proxyForm.outboundProxyUrl}
                    onChange={(e) => setProxyForm((prev) => ({ ...prev, outboundProxyUrl: e.target.value }))}
                    disabled={loading || proxyLoading}
                  />
                  <Typography sx={{ fontSize: { xs: 12, sm: 14 }, color: "text.secondary" }}>Leave empty to inherit existing env proxy (if any).</Typography>
                </Box>

                <Box sx={{ display: "flex", flexDirection: "column", gap: 1, pt: 1, borderTop: 1, borderColor: "divider" }}>
                  <Typography component="label" sx={{ fontWeight: 500, fontSize: { xs: 14, sm: 16 } }}>No Proxy</Typography>
                  <Input
                    placeholder="localhost,127.0.0.1"
                    value={proxyForm.outboundNoProxy}
                    onChange={(e) => setProxyForm((prev) => ({ ...prev, outboundNoProxy: e.target.value }))}
                    disabled={loading || proxyLoading}
                  />
                  <Typography sx={{ fontSize: { xs: 12, sm: 14 }, color: "text.secondary" }}>Comma-separated hostnames/domains to bypass the proxy.</Typography>
                </Box>

                <Box sx={{ pt: 1, borderTop: 1, borderColor: "divider", display: "flex", flexDirection: { xs: "column", sm: "row" }, alignItems: { sm: "center" }, gap: 1 }}>
                  <ResponsiveButton
                    type="button"
                    variant="secondary"
                    loading={proxyTestLoading}
                    disabled={loading || proxyLoading}
                    onClick={testOutboundProxy}
                  >
                    Test proxy URL
                  </ResponsiveButton>
                  <ResponsiveButton type="submit" variant="primary" loading={proxyLoading}>
                    Apply
                  </ResponsiveButton>
                </Box>
              </Box>
            )}

            {proxyStatus.message && (
              <Typography sx={{ fontSize: { xs: 12, sm: 14 }, color: statusColor(proxyStatus.type), pt: 1, borderTop: 1, borderColor: "divider" }}>
                {proxyStatus.message}
              </Typography>
            )}
          </Box>
        </Card>

        {/* Observability Settings */}
        <Card>
          <CardHeader icon="monitoring" title="Observability" color="#f97316" />
          <SettingRow
            title="Enable Observability"
            description="Record request details for inspection in the logs view"
            control={
              <Toggle
                checked={observabilityEnabled}
                onChange={updateObservabilityEnabled}
                disabled={loading}
              />
            }
          />
        </Card>

        {/* Account actions */}
        <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1 }}>
          <Button
            variant="outline"
            fullWidth
            icon="power_settings_new"
            onClick={() => setShutdownOpen(true)}
            sx={{ color: "#ef4444", borderColor: "rgba(239,68,68,0.4)", textTransform: "none", fontWeight: 600, "&:hover": { bgcolor: "rgba(239,68,68,0.05)", borderColor: "rgba(239,68,68,0.5)" } }}
          >
            Shutdown
          </Button>
          <Button
            variant="outline"
            fullWidth
            icon="logout"
            onClick={handleLogout}
          >
            Logout
          </Button>
        </Box>

        {/* App Info */}
        <Box sx={{ textAlign: "center", fontSize: { xs: 12, sm: 14 }, color: "text.secondary", py: 2 }}>
          <Box component="p" sx={{ m: 0 }}>{APP_CONFIG.name} v{APP_CONFIG.version}</Box>
          <Box component="p" sx={{ mt: 0.5, mb: 0 }}>Local Mode - All data stored on your machine</Box>
        </Box>
      </Stack>

      <LanguageSwitcher
        hideTrigger
        isOpen={langOpen}
        onClose={(next) => {
          setLangOpen(false);
          setLocale(next);
        }}
      />
      <ConfirmModal
        isOpen={shutdownOpen}
        onClose={() => setShutdownOpen(false)}
        onConfirm={handleShutdown}
        title="Close Proxy"
        message="Are you sure you want to close the proxy server?"
        confirmText="Close"
        cancelText="Cancel"
        variant="danger"
        loading={isShuttingDown}
      />

      <Modal
        isOpen={dbAuth.open}
        onClose={() => setDbAuth({ open: false, mode: "", password: "" })}
        title="Confirm Password"
        size="sm"
        footer={
          <>
            <Button variant="ghost" onClick={() => setDbAuth({ open: false, mode: "", password: "" })} disabled={dbLoading}>
              Cancel
            </Button>
            <Button variant="primary" onClick={handleDbAuthConfirm} loading={dbLoading} disabled={!dbAuth.password}>
              Confirm
            </Button>
          </>
        }
      >
        <Typography sx={{ color: "text.secondary", mb: 1.5, fontSize: 14 }}>
          Enter your current password to {dbAuth.mode === "export" ? "export" : "import"} the database.
        </Typography>
        <Input
          type="password"
          value={dbAuth.password}
          onChange={(e) => setDbAuth((s) => ({ ...s, password: e.target.value }))}
          onKeyDown={(e) => { if (e.key === "Enter" && dbAuth.password) handleDbAuthConfirm(); }}
          placeholder="Current password"
          autoFocus
        />
      </Modal>
    </Box>
  );
}
