"use client";

import Box from "@mui/material/Box";
import IconButton from "@mui/material/IconButton";
import { useTheme } from "@/shared/hooks/useTheme";
import { cn } from "@/shared/utils/cn";

const THEME_VARIANTS = {
  default: { width: 40, height: 40 },
  card: { width: 44, height: 44 },
};

export default function ThemeToggle({ className, variant = "default" }) {
  const { isDark, toggleTheme } = useTheme();
  const dims = THEME_VARIANTS[variant] || THEME_VARIANTS.default;
  const isCard = variant === "card";

  return (
    <IconButton
      type="button"
      onClick={toggleTheme}
      className={cn(className)}
      aria-label={`Switch to ${isDark ? "light" : "dark"} mode`}
      title={`Switch to ${isDark ? "light" : "dark"} mode`}
      sx={{
        width: dims.width,
        height: dims.height,
        color: "text.secondary",
        "&:hover": { color: isCard ? "primary.main" : "text.primary", bgcolor: isCard ? undefined : "var(--color-surface-2)" },
        ...(isCard
          ? {
              bgcolor: "var(--color-surface)",
              border: "1px solid var(--color-border)",
              backdropFilter: "blur(12px)",
              boxShadow: "0 1px 2px rgba(0,0,0,0.05)",
              "&:hover": {
                bgcolor: "var(--color-surface)",
                color: "primary.main",
                boxShadow: "var(--shadow-warm)",
              },
            }
          : {}),
        "& .ms-icon": { transition: "transform 300ms" },
        "&:hover .ms-icon": isCard ? { transform: "rotate(12deg)" } : {},
      }}
    >
      <span className="material-symbols-outlined ms-icon" style={{ fontSize: 22 }}>
        {isDark ? "light_mode" : "dark_mode"}
      </span>
    </IconButton>
  );
}
