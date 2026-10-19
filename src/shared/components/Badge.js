"use client";

import Box from "@mui/material/Box";
import { cn } from "@/shared/utils/cn";

// variant -> { bg, color } using the 9Router token palette (mirrors the old
// Tailwind utilities: bg-*-500/10 + text-*-600/400).
const VARIANTS = {
  default: { bgcolor: "var(--color-surface-2)", color: "text.secondary" },
  secondary: { bgcolor: "var(--color-surface-2)", color: "text.secondary" },
  neutral: { bgcolor: "var(--color-surface-2)", color: "text.secondary" },
  primary: { bgcolor: "rgba(229,106,74,0.1)", color: "var(--color-brand-600)" },
  success: { bgcolor: "rgba(34,197,94,0.1)", color: "#16a34a" },
  warning: { bgcolor: "rgba(234,179,8,0.1)", color: "#ca8a04" },
  error: { bgcolor: "rgba(239,68,68,0.1)", color: "#dc2626" },
  info: { bgcolor: "rgba(59,130,246,0.1)", color: "#2563eb" },
};

const DOT_COLORS = {
  default: "#6b7280",
  secondary: "#6b7280",
  neutral: "#6b7280",
  primary: "var(--color-brand-500)",
  success: "#22c55e",
  warning: "#eab308",
  error: "#ef4444",
  info: "#3b82f6",
};

const SIZES = {
  sm: { px: 1, py: 0.25, fontSize: 10 },
  md: { px: 1.25, py: 0.5, fontSize: 12 },
  lg: { px: 1.5, py: 0.75, fontSize: 14 },
};

export default function Badge({
  children,
  variant = "default",
  size = "md",
  dot = false,
  icon,
  className,
}) {
  const tone = VARIANTS[variant] || VARIANTS.default;
  const dims = SIZES[size] || SIZES.md;

  return (
    <Box
      component="span"
      className={cn(className)}
      sx={{
        display: "inline-flex",
        alignItems: "center",
        gap: 0.75,
        borderRadius: "9999px",
        fontWeight: 600,
        lineHeight: 1.4,
        ...tone,
        ...dims,
      }}
    >
      {dot && (
        <Box
          component="span"
          sx={{
            width: 6,
            height: 6,
            borderRadius: "50%",
            bgcolor: DOT_COLORS[variant] || DOT_COLORS.default,
          }}
        />
      )}
      {icon && (
        <span className="material-symbols-outlined" style={{ fontSize: 14 }}>
          {icon}
        </span>
      )}
      {children}
    </Box>
  );
}
