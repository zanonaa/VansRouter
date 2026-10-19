"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import Drawer from "@mui/material/Drawer";
import Box from "@mui/material/Box";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemIcon from "@mui/material/ListItemIcon";
import ListItemText from "@mui/material/ListItemText";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import Divider from "@mui/material/Divider";
import { APP_CONFIG } from "@/shared/constants/config";

const NAV = [
  { href: "/dashboard/endpoint", label: "Endpoint & Key", icon: "api" },
  { href: "/dashboard/providers", label: "Providers", icon: "dns" },
  { href: "/dashboard/combos", label: "Combos", icon: "layers" },
  { href: "/dashboard/usage", label: "Usage", icon: "bar_chart" },
  { href: "/dashboard/quota", label: "Quota Tracker", icon: "data_usage" },
  { href: "/dashboard/token-saver", label: "Token Saver", icon: "savings" },
];

const SYSTEM = [
  { href: "/dashboard/proxy-pools", label: "Proxy Pools", icon: "lan" },
  { href: "/dashboard/proxy-fitness", label: "Proxy Fitness", icon: "network_check" },
];

const DRAWER_WIDTH = 248;

function MsIcon({ children }) {
  return (
    <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 20, display: "inline-flex" }}>
      {children}
    </Icon>
  );
}

function NavSection({ title, items, pathname, onNavigate }) {
  const active = (href) => pathname === href || pathname.startsWith(`${href}/`);
  return (
    <>
      <Typography
        variant="overline"
        sx={{ px: 2, pt: 1.5, pb: 0.5, color: "text.secondary", fontSize: 10, letterSpacing: 1 }}
      >
        {title}
      </Typography>
      <List dense disablePadding sx={{ px: 1 }}>
        {items.map((item) => (
          <ListItem key={item.href} disablePadding>
            <ListItemButton
              component={Link}
              href={item.href}
              selected={active(item.href)}
              onClick={onNavigate}
            >
              <ListItemIcon>
                <MsIcon>{item.icon}</MsIcon>
              </ListItemIcon>
              <ListItemText
                primary={item.label}
                primaryTypographyProps={{ fontSize: 13.5, fontWeight: active(item.href) ? 600 : 500 }}
              />
            </ListItemButton>
          </ListItem>
        ))}
      </List>
    </>
  );
}

export default function MuiSidebar({ open, onClose }) {
  const pathname = usePathname();
  const [updateInfo, setUpdateInfo] = useState(null);

  useEffect(() => {
    let cancelled = false;
    const timer = setTimeout(() => {
      fetch("/api/version")
        .then((res) => (res.ok ? res.json() : null))
        .then((data) => { if (!cancelled && data?.hasUpdate) setUpdateInfo(data); })
        .catch(() => {});
    }, 2500);
    return () => { cancelled = true; clearTimeout(timer); };
  }, []);

  const content = (
    <Box sx={{ display: "flex", flexDirection: "column", height: "100%", py: 1.5 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.25, px: 2.5, pb: 1.5 }}>
        <Box
          sx={{
            width: 30,
            height: 30,
            borderRadius: "9px",
            bgcolor: "primary.main",
            color: "#fff",
            display: "grid",
            placeItems: "center",
            fontWeight: 800,
            fontSize: 14,
          }}
        >
          V
        </Box>
        <Typography variant="subtitle1" sx={{ fontWeight: 700, letterSpacing: "-0.02em" }}>
          VansRouter
        </Typography>
      </Box>

      <Box sx={{ flex: 1, overflowY: "auto" }}>
        <NavSection title="Main" items={NAV} pathname={pathname} onNavigate={onClose} />
        <Divider sx={{ my: 1.5 }} />
        <NavSection title="System" items={SYSTEM} pathname={pathname} onNavigate={onClose} />
      </Box>

      <Box sx={{ px: 1, pt: 1 }}>
        <List dense disablePadding>
          <ListItem disablePadding>
            <ListItemButton
              component={Link}
              href="/dashboard/profile"
              selected={pathname.startsWith("/dashboard/profile")}
              onClick={onClose}
            >
              <ListItemIcon>
                <MsIcon>settings</MsIcon>
              </ListItemIcon>
              <ListItemText primary="Settings" primaryTypographyProps={{ fontSize: 13.5, fontWeight: 500 }} />
            </ListItemButton>
          </ListItem>
        </List>
        <Box sx={{ px: 2.5, pb: 1.5, pt: 1 }}>
          <Typography variant="caption" sx={{ color: "text.secondary", fontVariantNumeric: "tabular-nums" }}>
            v{APP_CONFIG.version}
          </Typography>
          {updateInfo?.latestVersion && (
            <Typography variant="caption" sx={{ display: "block", color: "primary.main" }}>
              ↑ New version available: v{updateInfo.latestVersion}
            </Typography>
          )}
        </Box>
      </Box>
    </Box>
  );

  return (
    <Box component="nav" sx={{ width: { lg: DRAWER_WIDTH }, flexShrink: { lg: 0 } }}>
      {/* Mobile: temporary drawer */}
      <Drawer
        variant="temporary"
        open={open}
        onClose={onClose}
        ModalProps={{ keepMounted: true }}
        sx={{
          display: { xs: "block", lg: "none" },
          "& .MuiDrawer-paper": { width: DRAWER_WIDTH, boxSizing: "border-box" },
        }}
      >
        {content}
      </Drawer>
      {/* Desktop: fixed drawer */}
      <Drawer
        variant="permanent"
        open
        sx={{
          display: { xs: "none", lg: "block" },
          "& .MuiDrawer-paper": {
            width: DRAWER_WIDTH,
            boxSizing: "border-box",
            position: "fixed",
            height: "100vh",
            overflowX: "hidden",
          },
        }}
      >
        {content}
      </Drawer>
    </Box>
  );
}

export { DRAWER_WIDTH };
