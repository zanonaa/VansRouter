"use client";

import { useEffect, useRef } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import IconButton from "@mui/material/IconButton";
import { cn } from "@/shared/utils/cn";

const DRAWER_WIDTHS = {
  sm: { xs: "100%", sm: 400 },
  md: { xs: "100%", sm: 500 },
  lg: { xs: "100%", sm: 600 },
  xl: { xs: "100%", sm: 800 },
  full: { xs: "100%", sm: "100%" },
};

export default function Drawer({
  isOpen,
  onClose,
  title,
  children,
  width = "md",
  className,
}) {
  const onCloseRef = useRef(onClose);
  useEffect(() => {
    onCloseRef.current = onClose;
  });

  useEffect(() => {
    if (isOpen) {
      document.body.style.overflow = "hidden";
    } else {
      document.body.style.overflow = "";
    }
    return () => { document.body.style.overflow = ""; };
  }, [isOpen]);

  useEffect(() => {
    const handleEscape = (e) => {
      if (e.key === "Escape" && isOpen) onCloseRef.current();
    };
    document.addEventListener("keydown", handleEscape);
    return () => document.removeEventListener("keydown", handleEscape);
  }, [isOpen]);

  if (!isOpen) return null;

  return (
    <Box sx={{ position: "fixed", inset: 0, zIndex: 50 }}>
      {/* Overlay */}
      <Box
        onClick={onClose}
        aria-hidden="true"
        sx={{
          position: "absolute",
          inset: 0,
          bgcolor: "rgba(0,0,0,0.5)",
          backdropFilter: "blur(2px)",
          animation: "fadeIn 0.2s ease-out forwards",
          cursor: "pointer",
        }}
      />

      {/* Drawer panel */}
      <Box
        className={cn(className)}
        sx={{
          position: "absolute",
          right: 0,
          top: 0,
          height: "100%",
          bgcolor: "background.paper",
          display: "flex",
          flexDirection: "column",
          maxWidth: "100%",
          boxShadow: "var(--shadow-elev)",
          animation: "slideInFromRight 0.25s cubic-bezier(0.22, 1, 0.36, 1) forwards",
          borderLeft: "1px solid var(--color-border-subtle)",
          width: DRAWER_WIDTHS[width] || DRAWER_WIDTHS.md,
        }}
      >
        {/* Header */}
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", p: 3, borderBottom: "1px solid var(--color-border-subtle)", flexShrink: 0 }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
            {title && (
              <Typography variant="h6" sx={{ fontSize: 18, fontWeight: 600, color: "text.primary" }}>{title}</Typography>
            )}
          </Box>
          <IconButton
            type="button"
            onClick={onClose}
            sx={{ p: 0.75, borderRadius: "10px", color: "text.secondary", "&:hover": { bgcolor: "var(--color-surface-2)", color: "text.primary" } }}
          >
            <span className="material-symbols-outlined" style={{ fontSize: 20 }}>close</span>
          </IconButton>
        </Box>

        {/* Body */}
        <Box className="custom-scrollbar" sx={{ flex: 1, overflowY: "auto", p: 3 }}>
          {children}
        </Box>
      </Box>
    </Box>
  );
}
