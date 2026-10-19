"use client";

import { useMemo } from "react";
import { usePathname } from "next/navigation";
import AppBar from "@mui/material/AppBar";
import Toolbar from "@mui/material/Toolbar";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import IconButton from "@mui/material/IconButton";
import Icon from "@mui/material/Icon";
import ThemeToggle from "@/shared/components/ThemeToggle";
import HeaderMenu from "@/shared/components/HeaderMenu";
import { getPageInfo } from "@/shared/components/Header";

function MsIcon({ children, ...props }) {
  return (
    <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 22 }} {...props}>
      {children}
    </Icon>
  );
}

export default function MuiHeader({ onMenuClick }) {
  const pathname = usePathname();
  const { title, description, icon } = useMemo(() => getPageInfo(pathname), [pathname]);

  const handleLogout = async () => {
    try {
      const res = await fetch("/api/auth/logout", { method: "POST" });
      if (res.ok) window.location.assign("/login");
    } catch (err) {
      console.error("Failed to logout:", err);
    }
  };

  return (
    <AppBar position="sticky" elevation={0} color="transparent">
      <Toolbar sx={{ gap: 1.5, minHeight: 64 }}>
        <IconButton
          edge="start"
          onClick={onMenuClick}
          aria-label="Open menu"
          sx={{ display: { xs: "inline-flex", lg: "none" } }}
        >
          <MsIcon>menu</MsIcon>
        </IconButton>

        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            {icon && (
              <MsIcon sx={{ color: "primary.main", fontSize: { xs: 22, lg: 26 } }}>{icon}</MsIcon>
            )}
            <Typography variant="h6" noWrap sx={{ fontSize: { xs: 17, lg: 22 } }}>
              {title || "Dashboard"}
            </Typography>
          </Box>
          {description && (
            <Typography
              variant="body2"
              color="text.secondary"
              noWrap
              sx={{ display: { xs: "none", lg: "block" } }}
            >
              {description}
            </Typography>
          )}
        </Box>

        <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
          <ThemeToggle />
          <HeaderMenu onLogout={handleLogout} />
        </Box>
      </Toolbar>
    </AppBar>
  );
}
