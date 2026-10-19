"use client";

import MuiButton from "@mui/material/Button";
import CircularProgress from "@mui/material/CircularProgress";
import Icon from "@mui/material/Icon";
import { cn } from "@/shared/utils/cn";

// MUI-backed Button. Keeps the original prop API (variant, size, icon,
// iconRight, loading, fullWidth) so every page keeps working unchanged.
const VARIANT_MAP = {
  primary: { variant: "contained", color: "primary" },
  secondary: { variant: "outlined", color: "inherit" },
  outline: { variant: "outlined", color: "inherit" },
  ghost: { variant: "text", color: "inherit" },
  danger: { variant: "contained", color: "error" },
  success: { variant: "contained", color: "success" },
};

const SIZE_MAP = {
  sm: { size: "small", sx: { height: 28, px: 1.5, fontSize: 12 } },
  md: { size: "medium", sx: { height: 36, px: 2, fontSize: 13.5 } },
  lg: { size: "large", sx: { height: 44, px: 3, fontSize: 14 } },
};

function MsIcon({ children }) {
  return (
    <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18 }}>
      {children}
    </Icon>
  );
}

export default function Button({
  children,
  variant = "primary",
  size = "md",
  icon,
  iconRight,
  disabled = false,
  loading = false,
  fullWidth = false,
  className,
  ...props
}) {
  const mapped = VARIANT_MAP[variant] || VARIANT_MAP.primary;
  const sizeCfg = SIZE_MAP[size] || SIZE_MAP.md;

  return (
    <MuiButton
      variant={mapped.variant}
      color={mapped.color}
      size={sizeCfg.size}
      disableElevation
      fullWidth={fullWidth}
      disabled={disabled || loading}
      startIcon={loading ? <CircularProgress size={16} color="inherit" /> : icon ? <MsIcon>{icon}</MsIcon> : undefined}
      endIcon={iconRight && !loading ? <MsIcon>{iconRight}</MsIcon> : undefined}
      className={cn(className)}
      sx={{ textTransform: "none", fontWeight: 600, whiteSpace: "nowrap", ...sizeCfg.sx }}
      {...props}
    >
      {children}
    </MuiButton>
  );
}
