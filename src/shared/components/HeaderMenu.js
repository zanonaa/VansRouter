"use client";

import { useState, useEffect, useRef } from "react";
import dynamic from "next/dynamic";
import Box from "@mui/material/Box";
import IconButton from "@mui/material/IconButton";
import { useTheme } from "@/shared/hooks/useTheme";
const ChangelogModal = dynamic(() => import("./ChangelogModal"), { ssr: false });
import { ConfirmModal } from "./Modal";

function MenuItemRow({ icon, label, onClick, trailing, danger }) {
  return (
    <Box
      component="button"
      type="button"
      onClick={onClick}
      sx={{
        display: "flex",
        alignItems: "center",
        gap: 1.5,
        width: "100%",
        px: 2,
        py: 1.25,
        fontSize: 14,
        border: 0,
        bgcolor: "transparent",
        cursor: "pointer",
        transition: "background-color 150ms",
        color: danger ? "error.main" : "text.primary",
        "&:hover": { bgcolor: danger ? "rgba(239,68,68,0.1)" : "var(--color-surface-2)" },
      }}
    >
      <span className="material-symbols-outlined" style={{ fontSize: 20, color: danger ? "#ef4444" : "var(--color-text-muted)" }}>{icon}</span>
      <Box component="span" sx={{ flex: 1, textAlign: "left" }}>{label}</Box>
      {trailing && <Box component="span" sx={{ fontSize: 16 }}>{trailing}</Box>}
    </Box>
  );
}

export default function HeaderMenu({ onLogout }) {
  const [isOpen, setIsOpen] = useState(false);
  const [changelogOpen, setChangelogOpen] = useState(false);
  const [shutdownOpen, setShutdownOpen] = useState(false);
  const [isShuttingDown, setIsShuttingDown] = useState(false);
  const { toggleTheme, isDark } = useTheme();
  const menuRef = useRef(null);

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

  useEffect(() => {
    const handleClickOutside = (e) => {
      if (menuRef.current && !menuRef.current.contains(e.target)) {
        setIsOpen(false);
      }
    };
    if (isOpen) {
      document.addEventListener("mousedown", handleClickOutside);
      return () => document.removeEventListener("mousedown", handleClickOutside);
    }
  }, [isOpen]);

  const close = () => setIsOpen(false);

  return (
    <>
      <Box ref={menuRef} sx={{ position: "relative" }}>
        <IconButton
          type="button"
          onClick={() => setIsOpen((v) => !v)}
          title="Menu"
          sx={{ p: 1, borderRadius: "8px", color: "text.secondary", "&:hover": { color: "text.primary", bgcolor: "var(--color-surface-2)" } }}
        >
          <span className="material-symbols-outlined">grid_view</span>
        </IconButton>

        {isOpen && (
          <Box
            sx={{
              position: "absolute",
              right: 0,
              top: "100%",
              mt: 1,
              width: 240,
              bgcolor: "background.paper",
              border: "1px solid var(--color-border)",
              borderRadius: "12px",
              boxShadow: "0 25px 50px -12px rgba(0,0,0,0.25)",
              zIndex: 50,
              overflow: "hidden",
              py: 0.5,
            }}
          >
            <MenuItemRow icon="history" label="Change Log" onClick={() => { close(); setChangelogOpen(true); }} />
            <MenuItemRow icon={isDark ? "light_mode" : "dark_mode"} label="Theme" onClick={() => { toggleTheme(); close(); }} />
            <MenuItemRow icon="power_settings_new" label="Shutdown" danger onClick={() => { close(); setShutdownOpen(true); }} />
            <MenuItemRow icon="logout" label="Logout" danger onClick={() => { close(); onLogout(); }} />
          </Box>
        )}
      </Box>

      <ChangelogModal isOpen={changelogOpen} onClose={() => setChangelogOpen(false)} />
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
    </>
  );
}
