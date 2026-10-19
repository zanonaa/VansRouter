"use client";

import { useState, useEffect } from "react";
import { usePathname } from "next/navigation";
import Box from "@mui/material/Box";
import Paper from "@mui/material/Paper";
import IconButton from "@mui/material/IconButton";
import Typography from "@mui/material/Typography";
import { useNotificationStore } from "@/store/notificationStore";
import Sidebar from "../Sidebar";
import Header from "../Header";

function getToastStyle(type) {
  if (type === "success") {
    return {
      bgcolor: "rgba(34,197,94,0.1)",
      borderColor: "rgba(34,197,94,0.3)",
      color: "#16a34a",
      icon: "check_circle",
    };
  }
  if (type === "error") {
    return {
      bgcolor: "rgba(239,68,68,0.1)",
      borderColor: "rgba(239,68,68,0.3)",
      color: "#dc2626",
      icon: "error",
    };
  }
  if (type === "warning") {
    return {
      bgcolor: "rgba(245,158,11,0.1)",
      borderColor: "rgba(245,158,11,0.3)",
      color: "#d97706",
      icon: "warning",
    };
  }
  return {
    bgcolor: "rgba(59,130,246,0.1)",
    borderColor: "rgba(59,130,246,0.3)",
    color: "#2563eb",
    icon: "info",
  };
}

export default function DashboardLayout({ children }) {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const pathname = usePathname();
  const notifications = useNotificationStore((state) => state.notifications);
  const removeNotification = useNotificationStore((state) => state.removeNotification);

  useEffect(() => {
    if (!sidebarOpen) return;
    const onEsc = (e) => { if (e.key === "Escape") setSidebarOpen(false); };
    document.addEventListener("keydown", onEsc);
    return () => document.removeEventListener("keydown", onEsc);
  }, [sidebarOpen]);

  const isBasicChat = pathname === "/dashboard/basic-chat";

  return (
    <Box sx={{ display: "flex", width: "100%", height: "100vh", overflow: "hidden", bgcolor: "background.default" }}>
      <Box sx={{ position: "fixed", top: 16, right: 16, zIndex: 80, display: "flex", width: "min(92vw, 380px)", flexDirection: "column", gap: 1 }}>
        {notifications.map((notification) => {
          const style = getToastStyle(notification.type);
          return (
            <Paper
              key={notification.id}
              elevation={4}
              sx={{
                borderRadius: "8px",
                border: "1px solid",
                borderColor: style.borderColor,
                bgcolor: style.bgcolor,
                color: style.color,
                px: 1.5,
                py: 1,
                backdropFilter: "blur(8px)",
              }}
            >
              <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1 }}>
                <span className="material-symbols-outlined" style={{ fontSize: 18, lineHeight: "20px" }}>
                  {style.icon}
                </span>
                <Box sx={{ minWidth: 0, flex: 1 }}>
                  {notification.title ? (
                    <Typography sx={{ fontSize: 12, fontWeight: 600, mb: 0.5, color: "inherit" }}>
                      {notification.title}
                    </Typography>
                  ) : null}
                  <Typography sx={{ fontSize: 12, whiteSpace: "pre-wrap", overflowWrap: "anywhere", color: "inherit" }}>
                    {notification.message}
                  </Typography>
                </Box>
                {notification.dismissible ? (
                  <IconButton
                    type="button"
                    size="small"
                    onClick={() => removeNotification(notification.id)}
                    aria-label="Dismiss notification"
                    sx={{ p: 0, color: "inherit", opacity: 0.7, "&:hover": { opacity: 1 } }}
                  >
                    <span className="material-symbols-outlined" style={{ fontSize: 16 }}>close</span>
                  </IconButton>
                ) : null}
              </Box>
            </Paper>
          );
        })}
      </Box>

      {/* Mobile sidebar overlay */}
      {sidebarOpen && (
        <Box
          onClick={() => setSidebarOpen(false)}
          aria-hidden="true"
          sx={{ display: { xs: "block", lg: "none" }, position: "fixed", inset: 0, zIndex: 40, bgcolor: "rgba(0,0,0,0.2)" }}
        />
      )}

      {/* One responsive sidebar avoids duplicate settings/version requests. */}
      <Box
        sx={{
          position: { xs: "fixed", lg: "static" },
          top: { xs: 0, lg: "auto" },
          bottom: { xs: 0, lg: "auto" },
          left: { xs: 0, lg: "auto" },
          zIndex: { xs: 50, lg: "auto" },
          display: { lg: "flex" },
          height: { lg: "100%" },
          transform: { xs: sidebarOpen ? "translateX(0)" : "translateX(-100%)", lg: "translateX(0)" },
          transition: "transform 300ms ease-in-out",
        }}
      >
        <Sidebar onClose={() => setSidebarOpen(false)} />
      </Box>

      {/* Main content */}
      <Box
        component="main"
        sx={{
          display: "flex",
          flex: 1,
          flexDirection: "column",
          height: "100%",
          minWidth: 0,
          position: "relative",
          transition: "background-color 300ms",
          isolation: "isolate",
        }}
      >
        {/* Faint grid background */}
        <Box className="landing-grid" aria-hidden="true" sx={{ position: "absolute", inset: 0, pointerEvents: "none", zIndex: -1 }} />
        <Header key={pathname} onMenuClick={() => setSidebarOpen(true)} />
        <Box
          className="custom-scrollbar"
          sx={{
            flex: 1,
            overflowY: isBasicChat ? "hidden" : "auto",
            p: isBasicChat ? 0 : { xs: 3, lg: 5 },
            display: isBasicChat ? "flex" : "block",
            flexDirection: isBasicChat ? "column" : undefined,
          }}
        >
          <Box
            sx={isBasicChat
              ? { flex: 1, width: "100%", height: "100%", display: "flex", flexDirection: "column" }
              : { maxWidth: 1280, mx: "auto" }}
          >
            {children}
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
