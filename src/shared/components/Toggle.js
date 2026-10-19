"use client";

import Box from "@mui/material/Box";
import { cn } from "@/shared/utils/cn";

const TOGGLE_SIZES = {
  sm: { trackW: 32, trackH: 16, thumb: 12, translate: 16, thumbTop: 2, thumbLeftOff: 2 },
  md: { trackW: 44, trackH: 24, thumb: 20, translate: 20, thumbTop: 2, thumbLeftOff: 2 },
  lg: { trackW: 56, trackH: 28, thumb: 24, translate: 28, thumbTop: 2, thumbLeftOff: 2 },
};

export default function Toggle({
  checked = false,
  onChange,
  label,
  description,
  disabled = false,
  size = "md",
  className,
  "aria-label": ariaLabel,
  title,
}) {
  const dims = TOGGLE_SIZES[size] || TOGGLE_SIZES.md;

  const handleClick = () => {
    if (!disabled && onChange) onChange(!checked);
  };

  return (
    <Box
      className={cn(className)}
      sx={{ display: "flex", alignItems: "center", gap: 1.5, opacity: disabled ? 0.5 : 1, cursor: disabled ? "not-allowed" : "default" }}
    >
      <Box
        component="button"
        type="button"
        role="switch"
        aria-checked={checked}
        aria-label={ariaLabel || label || title || "Toggle"}
        disabled={disabled}
        onClick={handleClick}
        sx={{
          position: "relative",
          display: "inline-flex",
          flexShrink: 0,
          cursor: "pointer",
          borderRadius: "9999px",
          border: 0,
          p: 0,
          transition: "background-color 200ms ease-in-out",
          "&:focus": { outline: "none", boxShadow: "0 0 0 2px rgba(229,106,74,0.3)" },
          bgcolor: checked ? "var(--color-brand-500)" : "var(--color-surface-3)",
          width: dims.trackW,
          height: dims.trackH,
        }}
      >
        <Box
          component="span"
          sx={{
            pointerEvents: "none",
            display: "inline-block",
            borderRadius: "50%",
            bgcolor: "#fff",
            boxShadow: "0 1px 2px 0 rgba(0,0,0,0.05)",
            width: dims.thumb,
            height: dims.thumb,
            position: "absolute",
            top: dims.thumbTop,
            left: checked ? dims.thumbLeftOff + dims.translate : dims.thumbLeftOff,
            transition: "left 200ms ease-in-out",
          }}
        />
      </Box>
      {(label || description) && (
        <Box sx={{ display: "flex", flexDirection: "column" }}>
          {label && <Box component="span" sx={{ fontSize: 14, fontWeight: 500, color: "text.primary" }}>{label}</Box>}
          {description && <Box component="span" sx={{ fontSize: 12, color: "text.secondary" }}>{description}</Box>}
        </Box>
      )}
    </Box>
  );
}
