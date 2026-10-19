"use client";

import Box from "@mui/material/Box";
import { cn } from "@/shared/utils/cn";

const SEGMENT_SIZES = {
  sm: { height: 28, fontSize: 12 },
  md: { height: 36, fontSize: 14 },
  lg: { height: 44, fontSize: 16 },
};

const EMPTY_OPTIONS = [];

export default function SegmentedControl({
  options = EMPTY_OPTIONS,
  value,
  onChange,
  size = "md",
  className,
}) {
  const dims = SEGMENT_SIZES[size] || SEGMENT_SIZES.md;

  return (
    <Box
      className={cn(className)}
      sx={{
        display: "inline-flex",
        alignItems: "center",
        p: 0.5,
        borderRadius: "10px",
        overflowX: "auto",
        bgcolor: "var(--color-surface-2)",
      }}
    >
      {options.map((option) => {
        const selected = value === option.value;
        return (
          <Box
            component="button"
            type="button"
            key={option.value}
            onClick={() => onChange(option.value)}
            sx={{
              flexShrink: 0,
              px: 2,
              height: dims.height,
              fontSize: dims.fontSize,
              borderRadius: "8px",
              fontWeight: 500,
              border: 0,
              transition: "all 150ms",
              display: "inline-flex",
              alignItems: "center",
              bgcolor: selected ? "var(--color-surface)" : "transparent",
              color: selected ? "text.primary" : "text.secondary",
              boxShadow: selected ? "0 1px 2px rgba(0,0,0,0.05)" : "none",
              "&:hover": { color: "text.primary" },
            }}
          >
            {option.icon && (
              <span className="material-symbols-outlined" style={{ fontSize: 16, marginRight: 6 }}>
                {option.icon}
              </span>
            )}
            {option.label}
          </Box>
        );
      })}
    </Box>
  );
}
