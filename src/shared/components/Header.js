"use client";

import { useEffect, useMemo, useState } from "react";
import { usePathname } from "next/navigation";
import Link from "next/link";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import IconButton from "@mui/material/IconButton";
import ProviderIcon from "@/shared/components/ProviderIcon";
import HeaderMenu from "@/shared/components/HeaderMenu";
import HeaderLanguage from "@/shared/components/HeaderLanguage";
import ThemeToggle from "@/shared/components/ThemeToggle";
import DonateModal from "@/shared/components/DonateModal";
import { useHeaderSearchStore } from "@/store/headerSearchStore";
import { OAUTH_PROVIDERS, APIKEY_PROVIDERS } from "@/shared/constants/config";
import { MEDIA_PROVIDER_KINDS, AI_PROVIDERS } from "@/shared/constants/providers";
import { getProviderIconSrc } from "@/shared/utils/providerIcon";
import { translate } from "@/i18n/runtime";

export const getPageInfo = (pathname) => {
  if (!pathname) return { title: "", description: "", breadcrumbs: [] };

  // Media provider detail: /dashboard/media-providers/[kind]/[id]
  const mediaDetailMatch = pathname.match(/\/media-providers\/([^/]+)\/([^/]+)$/);
  if (mediaDetailMatch) {
    const kindId = mediaDetailMatch[1];
    const providerId = mediaDetailMatch[2];
    const kindConfig = MEDIA_PROVIDER_KINDS.find((k) => k.id === kindId);
    const provider = AI_PROVIDERS[providerId];
    return {
      title: provider?.name || providerId,
      description: "",
      breadcrumbs: [
        { label: "Media Providers", href: `/dashboard/media-providers/${kindId}` },
        { label: kindConfig?.label || kindId, href: `/dashboard/media-providers/${kindId}` },
        { label: provider?.name || providerId, image: getProviderIconSrc(providerId) },
      ],
    };
  }

  // Media provider kind: /dashboard/media-providers/[kind]
  const mediaKindMatch = pathname.match(/\/media-providers\/([^/]+)$/);
  if (mediaKindMatch) {
    const kindId = mediaKindMatch[1];
    const kindConfig = MEDIA_PROVIDER_KINDS.find((k) => k.id === kindId);
    return {
      title: kindConfig?.label || kindId,
      description: `Manage your ${kindConfig?.label || kindId} providers`,
      icon: kindConfig?.icon || "perm_media",
      breadcrumbs: [],
    };
  }

  // Provider detail page: /dashboard/providers/[id]
  const providerMatch = pathname.match(/\/providers\/([^/]+)$/);
  if (providerMatch) {
    const providerId = providerMatch[1];
    const providerInfo =
      OAUTH_PROVIDERS[providerId] || APIKEY_PROVIDERS[providerId];
    if (providerInfo) {
      return {
        title: providerInfo.name,
        description: "",
        breadcrumbs: [
          { label: "Providers", href: "/dashboard/providers" },
          {
            label: providerInfo.name,
            image: getProviderIconSrc(providerInfo.id),
          },
        ],
      };
    }
  }

  if (pathname.includes("/providers") && !pathname.includes("/media-providers"))
    return {
      title: "Providers",
      description: "Manage your AI provider connections",
      icon: "dns",
      breadcrumbs: [],
    };
  if (pathname.includes("/combos"))
    return {
      title: "Combos",
      description: "Model combos with fallback",
      icon: "layers",
      breadcrumbs: [],
    };
  if (pathname.includes("/usage"))
    return {
      title: "Usage & Analytics",
      description:
        "Monitor your API usage, token consumption, and request logs",
      icon: "bar_chart",
      breadcrumbs: [],
    };
  if (pathname.includes("/auth-files"))
    return {
      title: "Auth Files",
      description: "Map provider credentials stored in the local database",
      icon: "vpn_key",
      breadcrumbs: [],
    };
  if (pathname.includes("/quota"))
    return {
      title: "Quota Tracker",
      description: "Track and manage your API quota limits",
      icon: "data_usage",
      breadcrumbs: [],
    };
  if (pathname.includes("/mitm"))
    return {
      title: "MITM Proxy",
      description: "Intercept CLI tool traffic and route through 9Router",
      icon: "security",
      breadcrumbs: [],
    };
  if (pathname.includes("/token-saver"))
    return {
      title: "Token Saver",
      description: "Compress prompts and outputs to save tokens",
      icon: "savings",
      breadcrumbs: [],
    };
  if (pathname.includes("/proxy-pools"))
    return {
      title: "Proxy Pools",
      description: "Manage your proxy pool configurations",
      icon: "lan",
      breadcrumbs: [],
    };
  if (pathname.includes("/endpoint"))
    return {
      title: "Endpoint",
      description: "API endpoint configuration",
      icon: "api",
      breadcrumbs: [],
    };
  if (pathname.includes("/profile"))
    return {
      title: "Settings",
      description: "Manage your preferences",
      icon: "settings",
      breadcrumbs: [],
    };
  if (pathname.includes("/translator"))
    return {
      title: "Translator",
      description: "Debug translation flow between formats",
      icon: "translate",
      breadcrumbs: [],
    };
  if (pathname.includes("/console-log"))
    return {
      title: "Console Log",
      description: "Live server console output",
      icon: "monitor",
      breadcrumbs: [],
    };
  if (pathname === "/dashboard")
    return {
      title: "Endpoint",
      description: "API endpoint configuration",
      icon: "api",
      breadcrumbs: [],
    };
  return { title: "", description: "", breadcrumbs: [] };
};

export default function Header({ onMenuClick, showMenuButton = true }) {
  const pathname = usePathname();
  const [displayName, setDisplayName] = useState("");
  const [loginMethod, setLoginMethod] = useState("");
  const [donateOpen, setDonateOpen] = useState(false);

  // Memoize page info to prevent unnecessary recalculations
  const pageInfo = useMemo(() => getPageInfo(pathname), [pathname]);
  const { title, description, icon, breadcrumbs } = pageInfo;

  useEffect(() => {
    let cancelled = false;

    async function loadAuthStatus() {
      try {
        const res = await fetch("/api/auth/status", { cache: "no-store" });
        if (!res.ok) return;
        const data = await res.json();
        if (!cancelled) {
          setDisplayName(data?.displayName || data?.oidcName || data?.oidcEmail || "");
          setLoginMethod(data?.loginMethod || "");
        }
      } catch {
        if (!cancelled) {
          setDisplayName("");
          setLoginMethod("");
        }
      }
    }

    loadAuthStatus();
    return () => {
      cancelled = true;
    };
  }, []);

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

  return (
    <Box
      component="header"
      sx={{
        flexShrink: 0,
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        gap: 1.5,
        px: { xs: 2, lg: 4 },
        pt: 1.5,
        pb: 1,
        borderBottom: "1px solid var(--color-border-subtle)",
        bgcolor: { xs: "color-mix(in srgb, var(--color-surface) 60%, transparent)", lg: "transparent" },
        backdropFilter: { xs: "blur(20px)", lg: "none" },
        zIndex: 20,
      }}
    >
      {/* Mobile menu button */}
      <Box sx={{ display: { lg: "none" }, alignItems: "center", gap: 1.5, flexShrink: 0 }}>
        {showMenuButton && (
          <IconButton
            onClick={onMenuClick}
            sx={{ color: "text.primary", "&:hover": { color: "primary.main" }, p: 0.5 }}
          >
            <span className="material-symbols-outlined">menu</span>
          </IconButton>
        )}
      </Box>

      {/* Page title with breadcrumbs */}
      <Box sx={{ display: "flex", flexDirection: "column", minWidth: 0, flex: 1 }}>
        {breadcrumbs.length > 0 ? (
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            {breadcrumbs.map((crumb, index) => (
              <Box
                key={`${crumb.label}-${crumb.href || "current"}`}
                sx={{ display: "flex", alignItems: "center", gap: 1 }}
              >
                {index > 0 && (
                  <span className="material-symbols-outlined" style={{ color: "var(--color-text-muted)", fontSize: 16 }}>
                    chevron_right
                  </span>
                )}
                {crumb.href ? (
                  <Box
                    component={Link}
                    href={crumb.href}
                    sx={{ color: "text.secondary", "&:hover": { color: "primary.main" }, transition: "color 150ms" }}
                  >
                    {crumb.label}
                  </Box>
                ) : (
                  <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                    {crumb.image && (
                      <ProviderIcon
                        src={crumb.image}
                        alt={crumb.label}
                        size={28}
                        fallbackText={crumb.label.slice(0, 2).toUpperCase()}
                      />
                    )}
                    <Box component="h1" sx={{ fontSize: { xs: 16, lg: 24 }, fontWeight: 600, color: "text.primary", letterSpacing: "-0.01em", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", m: 0 }}>
                      {translate(crumb.label)}
                    </Box>
                  </Box>
                )}
              </Box>
            ))}
          </Box>
        ) : title ? (
          <Box>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              {icon && (
                <span className="material-symbols-outlined" style={{ color: "var(--color-primary)", fontSize: 24 }}>
                  {icon}
                </span>
              )}
              <Box component="h1" sx={{ fontSize: { xs: 16, lg: 24 }, fontWeight: 600, letterSpacing: "-0.01em", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", m: 0 }}>
                {translate(title)}
              </Box>
            </Box>
            {description && (
              <Typography sx={{ display: { xs: "none", lg: "block" }, fontSize: 14, color: "text.secondary", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                {translate(description)}
              </Typography>
            )}
          </Box>
        ) : null}
      </Box>

      {/* Right actions */}
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.5, flexShrink: 0 }}>
        {displayName && loginMethod === "OIDC" && (
          <Box sx={{ display: { xs: "none", sm: "flex" }, alignItems: "center", maxWidth: 220, px: 1.5, py: 0.75, borderRadius: "9999px", border: "1px solid var(--color-border)", bgcolor: "color-mix(in srgb, var(--color-surface) 70%, transparent)", fontSize: 12, color: "text.secondary", overflow: "hidden" }}>
            <span className="material-symbols-outlined" style={{ fontSize: 14, marginRight: 6, color: "var(--color-primary)" }}>person</span>
            <Box component="span" sx={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{displayName}</Box>
            <Box component="span" sx={{ ml: 1, flexShrink: 0, borderRadius: "9999px", bgcolor: "rgba(229,106,74,0.1)", px: 1, py: 0.25, fontSize: 10, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em", color: "primary.main" }}>
              OIDC
            </Box>
          </Box>
        )}
        <HeaderSearch />
        <Box
          component="button"
          onClick={() => setDonateOpen(true)}
          aria-label="Donate Me :3"
          sx={{ display: "flex", alignItems: "center", gap: 0.75, px: 1.5, height: 32, borderRadius: "8px", border: "1px solid rgba(236,72,153,0.3)", bgcolor: "rgba(236,72,153,0.1)", color: "#db2777", fontSize: 14, fontWeight: 500, cursor: "pointer", "&:hover": { bgcolor: "rgba(236,72,153,0.2)" } }}
        >
          <span className="material-symbols-outlined" style={{ fontSize: 18 }}>volunteer_activism</span>
          <Box component="span" sx={{ display: { xs: "none", sm: "inline" } }}>Donate Me :3</Box>
        </Box>
        <ThemeToggle />
        <HeaderLanguage />
        <HeaderMenu onLogout={handleLogout} />
      </Box>
      <DonateModal isOpen={donateOpen} onClose={() => setDonateOpen(false)} />
    </Box>
  );
}

function HeaderSearch() {
  const visible = useHeaderSearchStore((s) => s.visible);
  const query = useHeaderSearchStore((s) => s.query);
  const placeholder = useHeaderSearchStore((s) => s.placeholder);
  const setQuery = useHeaderSearchStore((s) => s.setQuery);

  if (!visible) return null;

  return (
    <Box sx={{ position: "relative", width: { xs: 160, sm: 220 } }}>
      <span className="material-symbols-outlined" style={{ position: "absolute", left: 8, top: "50%", transform: "translateY(-50%)", color: "var(--color-text-muted)", fontSize: 16, pointerEvents: "none" }}>
        search
      </span>
      <Box
        component="input"
        type="text"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder={placeholder}
        sx={{ width: "100%", height: 32, pl: 3.5, pr: 3.5, borderRadius: "8px", border: "1px solid var(--color-border)", bgcolor: "color-mix(in srgb, var(--color-surface) 60%, transparent)", fontSize: 14, color: "text.primary", outline: "none", transition: "border-color 150ms", "&:focus": { borderColor: "rgba(229,106,74,0.5)" } }}
      />
      {query && (
        <Box
          component="button"
          type="button"
          onClick={() => setQuery("")}
          aria-label="Clear search"
          sx={{ position: "absolute", right: 4, top: "50%", transform: "translateY(-50%)", color: "text.secondary", p: 0.25, borderRadius: "4px", border: 0, bgcolor: "transparent", cursor: "pointer", "&:hover": { color: "text.primary" } }}
        >
          <span className="material-symbols-outlined" style={{ fontSize: 16 }}>close</span>
        </Box>
      )}
    </Box>
  );
}
