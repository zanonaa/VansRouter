"use client";

import { useState, useEffect } from "react";
import dynamic from "next/dynamic";
import Link from "next/link";
import { usePathname } from "next/navigation";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import { APP_CONFIG, UPDATER_CONFIG, GITHUB_CONFIG } from "@/shared/constants/config";
import { MEDIA_PROVIDER_KINDS } from "@/shared/constants/providers";
import { useCopyToClipboard } from "@/shared/hooks/useCopyToClipboard";
import Button from "./Button";
import { ConfirmModal } from "./Modal";
const ChangelogModal = dynamic(() => import("./ChangelogModal"), { ssr: false });

// const VISIBLE_MEDIA_KINDS = ["embedding", "image", "imageToText", "tts", "stt", "webSearch", "webFetch", "video", "music"];
const VISIBLE_MEDIA_KINDS = ["embedding", "image", "video", "tts", "stt"];
// Combined entry: webSearch + webFetch share one page at /dashboard/media-providers/web
const COMBINED_WEB_ITEM = { id: "web", label: "Web Fetch & Search", icon: "travel_explore", href: "/dashboard/media-providers/web" };

const navItems = [
  { href: "/dashboard/endpoint", label: "Endpoint & Key", icon: "api" },
  { href: "/dashboard/providers", label: "Providers", icon: "dns" },
  // { href: "/dashboard/basic-chat", label: "Basic Chat", icon: "chat" }, // Hidden
  { href: "/dashboard/combos", label: "Combos", icon: "layers" },
  { href: "/dashboard/usage", label: "Usage", icon: "bar_chart" },
  { href: "/dashboard/quota", label: "Quota Tracker", icon: "data_usage" },
  { href: "/dashboard/token-saver", label: "Token Saver", icon: "savings" },
];

const debugItems = [
  { href: "/dashboard/console-log", label: "Console Log", icon: "terminal" },
  { href: "/dashboard/translator", label: "Translator", icon: "translate" },
];

const systemItems = [
  { href: "/dashboard/proxy-pools", label: "Proxy Pools", icon: "lan" },
  { href: "/dashboard/proxy-fitness", label: "Proxy Fitness", icon: "network_check" },
];

const navLinkSx = (active) => ({
  display: "flex",
  alignItems: "center",
  gap: 1.5,
  px: 1.5,
  py: 0.25,
  borderRadius: "8px",
  transition: "all 150ms",
  textDecoration: "none",
  bgcolor: active ? "rgba(229,106,74,0.1)" : "transparent",
  color: active ? "primary.main" : "text.secondary",
  "&:hover": active ? {} : { bgcolor: "var(--color-surface-2)", color: "text.primary" },
  "&:hover .ms-nav-icon": active ? {} : { color: "var(--color-primary)" },
});

const navIconSx = (active) => ({
  fontSize: 18,
  fontVariationSettings: active ? "'FILL' 1, 'wght' 400, 'GRAD' 0, 'opsz' 24" : undefined,
  transition: "color 150ms",
});

export default function Sidebar({ onClose }) {
  const pathname = usePathname();
  const [mediaOpen, setMediaOpen] = useState(false);
  const [showChangelogModal, setShowChangelogModal] = useState(false);
  const [isDisconnected, setIsDisconnected] = useState(false);
  const [updateInfo, setUpdateInfo] = useState(null);
  const [showUpdateModal, setShowUpdateModal] = useState(false);
  const [isUpdating, setIsUpdating] = useState(false);
  const [shutdownCountdown, setShutdownCountdown] = useState(0);
  const [enableTranslator, setEnableTranslator] = useState(false);
  const { copied, copy } = useCopyToClipboard(2000);

  const INSTALL_CMD = UPDATER_CONFIG.installCmdLatest;

  useEffect(() => {
    fetch("/api/settings")
      .then(res => res.json())
      .then(data => { if (data.enableTranslator) setEnableTranslator(true); })
      .catch(() => {});
  }, []);

  // Lazy check for new npm version in background after initial render
  useEffect(() => {
    const timer = setTimeout(() => {
      fetch("/api/version")
        .then(res => res.json())
        .then(data => { if (data.hasUpdate) setUpdateInfo(data); })
        .catch(() => {});
    }, 2500);
    return () => clearTimeout(timer);
  }, []);

  const isActive = (href) => {
    if (href === "/dashboard/endpoint") {
      return pathname === "/dashboard" || pathname.startsWith("/dashboard/endpoint");
    }
    return pathname.startsWith(href);
  };

  // Open manual update panel (no countdown yet — user must click Copy to trigger shutdown)
  const handleUpdate = () => {
    setShowUpdateModal(false);
    setIsUpdating(true);
  };

  // Triggered by Copy button inside ManualUpdatePanel: copy + countdown + shutdown
  const handleCopyAndShutdown = async () => {
    try { await navigator.clipboard.writeText(INSTALL_CMD); } catch { /* clipboard blocked */ }
    copy(INSTALL_CMD);
    let remaining = UPDATER_CONFIG.shutdownCountdownSec;
    setShutdownCountdown(remaining);
    const timer = setInterval(() => {
      remaining -= 1;
      setShutdownCountdown(remaining);
      if (remaining <= 0) {
        clearInterval(timer);
        fetch("/api/version/shutdown", { method: "POST" }).catch(() => {});
        setIsDisconnected(true);
      }
    }, 1000);
  };

  const handleCancelUpdate = () => {
    setIsUpdating(false);
    setShutdownCountdown(0);
  };

  // Note: legacy updater poll removed. New flow: copy install cmd + shutdown server,
  // user runs the command manually in another terminal.


  return (
    <>
      <Box
        component="aside"
        sx={{
          display: "flex",
          width: 288,
          flexDirection: "column",
          borderRight: "1px solid var(--color-border-subtle)",
          bgcolor: "var(--color-sidebar)",
          backdropFilter: "blur(20px)",
          transition: "background-color 300ms",
          minHeight: "100%",
        }}
      >
        {/* Traffic lights */}
        <Box sx={{ display: "flex", alignItems: "center", gap: 1, px: 3, pt: 2.5, pb: 1 }}>
          <Box sx={{ width: 12, height: 12, borderRadius: "50%", bgcolor: "#FF5F56" }} />
          <Box sx={{ width: 12, height: 12, borderRadius: "50%", bgcolor: "#FFBD2E" }} />
          <Box sx={{ width: 12, height: 12, borderRadius: "50%", bgcolor: "#27C93F" }} />
        </Box>

        {/* Logo */}
        <Box sx={{ px: 3, py: 2, display: "flex", flexDirection: "column", gap: 1 }}>
          <Box component={Link} href="/dashboard" sx={{ display: "flex", alignItems: "center", gap: 1.5, textDecoration: "none" }}>
            <Box sx={{ display: "flex", alignItems: "center", justifyContent: "center", width: 36, height: 36, borderRadius: "10px", background: "linear-gradient(135deg, #E56A4A, #a64027)", boxShadow: "var(--shadow-warm)" }}>
              <span className="material-symbols-outlined" style={{ color: "#fff", fontSize: 20 }}>hub</span>
            </Box>
            <Box sx={{ display: "flex", flexDirection: "column" }}>
              <Typography component="h1" sx={{ fontSize: 18, fontWeight: 600, letterSpacing: "-0.01em", color: "text.primary", m: 0 }}>
                {APP_CONFIG.name}
              </Typography>
              <Box component="span" sx={{ fontSize: 12, color: "text.secondary" }}>v{APP_CONFIG.version}</Box>
            </Box>
          </Box>
          {updateInfo && (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 0.75, borderRadius: "4px", p: 0.5, m: -0.5 }}>
              <Box component="span" sx={{ fontSize: 12, fontWeight: 600, color: "#16a34a" }}>
                ↑ New version available: v{updateInfo.latestVersion}
              </Box>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Box
                  component="button"
                  onClick={() => setShowUpdateModal(true)}
                  sx={{ px: 1, py: 0.5, borderRadius: "4px", bgcolor: "#16a34a", color: "#fff", fontSize: 11, fontWeight: 600, border: 0, cursor: "pointer", transition: "background-color 150ms", "&:hover": { bgcolor: "#15803d" } }}
                >
                  {updateInfo.canAutoRestart ? "Update & Restart" : "Update now"}
                </Box>
                <Box
                  component="button"
                  onClick={() => copy(updateInfo.installCommand || INSTALL_CMD)}
                  title="Copy install command"
                  sx={{ flex: 1, textAlign: "left", bgcolor: "transparent", border: 0, cursor: "pointer", minWidth: 0, "&:hover": { opacity: 0.8 } }}
                >
                  <Box component="code" sx={{ display: "block", fontSize: 10, color: "rgba(22,163,74,0.8)", fontFamily: "ui-monospace, monospace", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                    {copied ? "✓ copied!" : (updateInfo.installCommand || INSTALL_CMD)}
                  </Box>
                </Box>
              </Box>
              <Box sx={{ display: "flex", alignItems: "center", mt: 0.25 }}>
                <Box
                  component="button"
                  type="button"
                  onClick={() => setShowChangelogModal(true)}
                  sx={{ fontSize: 10, color: "rgba(22,163,74,0.8)", bgcolor: "transparent", border: 0, cursor: "pointer", display: "flex", alignItems: "center", gap: 0.5, "&:hover": { color: "#15803d", textDecoration: "underline" } }}
                >
                  <span className="material-symbols-outlined" style={{ fontSize: 12 }}>description</span>
                  Check new changelog
                </Box>
              </Box>
            </Box>
          )}
        </Box>

        {/* Navigation */}
        <Box component="nav" className="custom-scrollbar" sx={{ flex: 1, px: 2, py: 1, overflowY: "auto" }}>
          {navItems.map((item) => (
            <Box
              key={item.href}
              component={Link}
              href={item.href}
              onClick={onClose}
              sx={navLinkSx(isActive(item.href))}
            >
              <span className="material-symbols-outlined ms-nav-icon" style={navIconSx(isActive(item.href))}>
                {item.icon}
              </span>
              <Box component="span" sx={{ fontSize: 13, fontWeight: 500 }}>{item.label}</Box>
            </Box>
          ))}

          {/* System section */}
          <Box sx={{ pt: 1.5, mt: 1 }}>
            <Typography component="p" sx={{ px: 2, fontSize: 12, fontWeight: 600, color: "text.secondary", textTransform: "uppercase", letterSpacing: "0.05em", mb: 1, opacity: 0.6 }}>
              System
            </Typography>

            {/* Media Providers accordion */}
            <Box
              component="button"
              onClick={() => setMediaOpen((v) => !v)}
              sx={{ ...navLinkSx(pathname.startsWith("/dashboard/media-providers")), width: "100%", border: 0, bgcolor: pathname.startsWith("/dashboard/media-providers") ? "rgba(229,106,74,0.1)" : "transparent", cursor: "pointer" }}
            >
              <span className="material-symbols-outlined ms-nav-icon" style={{ fontSize: 18 }}>perm_media</span>
              <Box component="span" sx={{ fontSize: 13, fontWeight: 500, flex: 1, textAlign: "left" }}>Media Providers</Box>
              <span className="material-symbols-outlined" style={{ fontSize: 14, transition: "transform 150ms", transform: mediaOpen ? "rotate(180deg)" : "rotate(0deg)" }}>
                expand_more
              </span>
            </Box>
            {mediaOpen && (
              <Box sx={{ pl: 2 }}>
                {MEDIA_PROVIDER_KINDS.filter((k) => VISIBLE_MEDIA_KINDS.includes(k.id)).map((kind) => (
                  <Box
                    key={kind.id}
                    component={Link}
                    href={`/dashboard/media-providers/${kind.id}`}
                    onClick={onClose}
                    sx={navLinkSx(pathname.startsWith(`/dashboard/media-providers/${kind.id}`))}
                  >
                    <span className="material-symbols-outlined ms-nav-icon" style={{ fontSize: 16 }}>{kind.icon}</span>
                    <Box component="span" sx={{ fontSize: 14 }}>{kind.label}</Box>
                  </Box>
                ))}
                <Box
                  key={COMBINED_WEB_ITEM.id}
                  component={Link}
                  href={COMBINED_WEB_ITEM.href}
                  onClick={onClose}
                  sx={navLinkSx(pathname.startsWith(COMBINED_WEB_ITEM.href))}
                >
                  <span className="material-symbols-outlined ms-nav-icon" style={{ fontSize: 16 }}>{COMBINED_WEB_ITEM.icon}</span>
                  <Box component="span" sx={{ fontSize: 14 }}>{COMBINED_WEB_ITEM.label}</Box>
                </Box>
              </Box>
            )}

            {systemItems.map((item) => (
              <Box
                key={item.href}
                component={Link}
                href={item.href}
                onClick={onClose}
                sx={navLinkSx(isActive(item.href))}
              >
                <span className="material-symbols-outlined ms-nav-icon" style={navIconSx(isActive(item.href))}>
                  {item.icon}
                </span>
                <Box component="span" sx={{ fontSize: 13, fontWeight: 500 }}>{item.label}</Box>
              </Box>
            ))}

            {/* Debug items (inside System section, before Settings) */}
            {debugItems.map((item) => {
              const show = item.href !== "/dashboard/translator" || enableTranslator;
              return show ? (
                <Box
                  key={item.href}
                  component={Link}
                  href={item.href}
                  onClick={onClose}
                  sx={navLinkSx(isActive(item.href))}
                >
                  <span className="material-symbols-outlined ms-nav-icon" style={navIconSx(isActive(item.href))}>
                    {item.icon}
                  </span>
                  <Box component="span" sx={{ fontSize: 13, fontWeight: 500 }}>{item.label}</Box>
                </Box>
              ) : null;
            })}

            {/* Settings */}
            <Box
              component={Link}
              href="/dashboard/profile"
              onClick={onClose}
              sx={navLinkSx(isActive("/dashboard/profile"))}
            >
              <span className="material-symbols-outlined ms-nav-icon" style={navIconSx(isActive("/dashboard/profile"))}>
                settings
              </span>
              <Box component="span" sx={{ fontSize: 13, fontWeight: 500 }}>Settings</Box>
            </Box>
          </Box>
        </Box>

      </Box>

      {/* Changelog Modal */}
      <ChangelogModal isOpen={showChangelogModal} onClose={() => setShowChangelogModal(false)} />

      {/* Update Confirmation Modal */}
      <ConfirmModal
        isOpen={showUpdateModal}
        onClose={() => setShowUpdateModal(false)}
        onConfirm={handleUpdate}
        title="Update VansRouter"
        message={`Show install command for v${updateInfo?.latestVersion || ""}? You can copy it and shutdown to install manually.`}
        confirmText="Show Command"
        cancelText="Cancel"
        variant="primary"
      />

      {/* Disconnected / Updating Overlay */}
      {(isDisconnected || isUpdating) && (
        <Box sx={{ position: "fixed", inset: 0, zIndex: 50, display: "flex", alignItems: "center", justifyContent: "center", bgcolor: "rgba(0,0,0,0.8)", backdropFilter: "blur(4px)", p: 3 }}>
          {isUpdating ? (
            <ManualUpdatePanel
              latestVersion={updateInfo?.latestVersion}
              installCmd={INSTALL_CMD}
              copied={copied}
              onCopyAndShutdown={handleCopyAndShutdown}
              onCancel={handleCancelUpdate}
              countdown={shutdownCountdown}
              isDisconnected={isDisconnected}
            />
          ) : (
            <Box sx={{ textAlign: "center", p: 4 }}>
              <Box sx={{ display: "flex", alignItems: "center", justifyContent: "center", width: 64, height: 64, borderRadius: "50%", bgcolor: "rgba(239,68,68,0.2)", color: "#ef4444", mx: "auto", mb: 2 }}>
                <span className="material-symbols-outlined" style={{ fontSize: 32 }}>power_off</span>
              </Box>
              <Typography variant="h6" sx={{ fontSize: 20, fontWeight: 600, color: "#fff", mb: 1 }}>Server Disconnected</Typography>
              <Typography sx={{ color: "text.secondary", mb: 3 }}>The proxy server has been stopped.</Typography>
              <Button variant="secondary" onClick={() => globalThis.location.reload()}>
                Reload Page
              </Button>
            </Box>
          )}
        </Box>
      )}
    </>
  );
}


function ManualUpdatePanel({ latestVersion, installCmd, copied, onCopyAndShutdown, onCancel, countdown, isDisconnected }) {
  const isCountingDown = countdown > 0;
  return (
    <Box sx={{ width: "100%", maxWidth: 512, borderRadius: "12px", bgcolor: "rgba(23,23,23,0.95)", border: "1px solid rgba(255,255,255,0.1)", p: 3, color: "#fff" }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, mb: 2 }}>
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "center", width: 44, height: 44, borderRadius: "50%", bgcolor: "rgba(245,158,11,0.2)", color: "#fbbf24" }}>
          <span className="material-symbols-outlined" style={{ fontSize: 24 }}>content_copy</span>
        </Box>
        <Box>
          <Typography component="h2" sx={{ fontSize: 18, fontWeight: 600 }}>Update VansRouter{latestVersion ? ` to v${latestVersion}` : ""}</Typography>
          <Box component="p" sx={{ fontSize: 12, color: "rgba(255,255,255,0.6)", m: 0 }}>
            {isDisconnected
              ? "Server stopped. Paste the command into a terminal to install."
              : isCountingDown
                ? `Command copied. Server will stop in ${countdown}s...`
                : "Click the button below to copy the install command and shutdown."}
          </Box>
        </Box>
      </Box>

      <Box component="p" sx={{ fontSize: 14, color: "rgba(255,255,255,0.8)", mb: 1 }}>Install command:</Box>
      <Box sx={{ width: "100%", px: 1.5, py: 1, borderRadius: "4px", bgcolor: "rgba(255,255,255,0.05)", mb: 2 }}>
        <Box component="code" sx={{ fontSize: 12, fontFamily: "ui-monospace, monospace", color: "#fbbf24", wordBreak: "break-all" }}>{installCmd}</Box>
      </Box>

      <Box component="ol" sx={{ fontSize: 12, color: "rgba(255,255,255,0.7)", display: "flex", flexDirection: "column", gap: 0.5, listStyle: "decimal", pl: 2.5, m: 0, mb: 2 }}>
        <li>Click <strong>Copy &amp; Shutdown</strong> below.</li>
        <li>Paste the command into your terminal and press Enter.</li>
        <li>Run <Box component="code" sx={{ px: 0.5, borderRadius: "4px", bgcolor: "rgba(255,255,255,0.1)", color: "#4ade80" }}>vansrouter</Box> again after install.</li>
      </Box>

      {isDisconnected ? (
        <Button variant="secondary" fullWidth onClick={() => globalThis.location.reload()}>
          Reload Page
        </Button>
      ) : (
        <Box sx={{ display: "flex", gap: 1 }}>
          <Button variant="secondary" onClick={onCancel} disabled={isCountingDown}>
            Cancel
          </Button>
          <Button variant="primary" fullWidth onClick={onCopyAndShutdown} disabled={isCountingDown}>
            {copied ? "✓ Copied — shutting down..." : isCountingDown ? `Shutting down in ${countdown}s` : "Copy & Shutdown"}
          </Button>
        </Box>
      )}
    </Box>
  );
}
