"use client";

import Box from "@mui/material/Box";
import { cn } from "@/shared/utils/cn";

const EMPTY_OPTIONS = [];

export default function Select({
  label,
  options = EMPTY_OPTIONS,
  value,
  onChange,
  placeholder = "Select an option",
  error,
  hint,
  disabled = false,
  required = false,
  className,
  selectClassName,
  ...props
}) {
  return (
    <Box className={cn(className)} sx={{ display: "flex", flexDirection: "column", gap: 0.75 }}>
      {label && (
        <Box component="label" sx={{ fontSize: 14, fontWeight: 500, color: "text.primary" }}>
          {label}
          {required && <Box component="span" sx={{ color: "error.main", ml: 0.5 }}>*</Box>}
        </Box>
      )}
      <Box sx={{ position: "relative" }}>
        <Box
          component="select"
          value={value}
          onChange={onChange}
          disabled={disabled}
          className={cn(selectClassName)}
          sx={{
            width: "100%",
            py: 1.25,
            px: 1.5,
            pr: 5,
            fontSize: { xs: 16, sm: 14 },
            color: "text.primary",
            bgcolor: "var(--color-surface-2)",
            border: "1px solid transparent",
            borderRadius: "10px",
            appearance: "none",
            transition: "all 150ms ease",
            "&:focus": {
              outline: "none",
              borderColor: "rgba(229,106,74,0.4)",
              boxShadow: "0 0 0 2px rgba(229,106,74,0.3)",
            },
            "&:disabled": { opacity: 0.5, cursor: "not-allowed" },
            ...(error
              ? {
                  borderColor: "rgba(239,68,68,0.4)",
                  boxShadow: "0 0 0 1px #ef4444",
                  "&:focus": { boxShadow: "0 0 0 2px rgba(239,68,68,0.4)" },
                }
              : {}),
          }}
          {...props}
        >
          <option value="" disabled>
            {placeholder}
          </option>
          {options.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </Box>
        <Box
          sx={{
            position: "absolute",
            top: 0,
            bottom: 0,
            right: 0,
            display: "flex",
            alignItems: "center",
            pr: 1.5,
            pointerEvents: "none",
            color: "text.secondary",
          }}
        >
          <span className="material-symbols-outlined" style={{ fontSize: 20 }}>
            expand_more
          </span>
        </Box>
      </Box>
      {error && (
        <Box component="p" sx={{ fontSize: 12, color: "error.main", display: "flex", alignItems: "center", gap: 0.5 }}>
          <span className="material-symbols-outlined" style={{ fontSize: 14 }}>
            error
          </span>
          {error}
        </Box>
      )}
      {hint && !error && (
        <Box component="p" sx={{ fontSize: 12, color: "text.secondary" }}>
          {hint}
        </Box>
      )}
    </Box>
  );
}
