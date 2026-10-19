"use client";

import Box from "@mui/material/Box";

// Lightweight hover tooltip. Kept custom (not MUI Tooltip) to preserve the
// original `text` / `position` / `color` prop API used across the dashboard.
export default function Tooltip({ text, children, position = "top", color }) {
  const placement = {
    top: { bottom: "100%", left: "50%", transform: "translateX(-50%)", mb: "6px" },
    bottom: { top: "100%", left: "50%", transform: "translateX(-50%)", mt: "6px" },
    left: { right: "100%", top: "50%", transform: "translateY(-50%)", mr: "6px" },
    right: { left: "100%", top: "50%", transform: "translateY(-50%)", ml: "6px" },
  }[position];

  return (
    <Box sx={{ position: "relative", display: "inline-flex", "&:hover .ms-tooltip": { opacity: 1 } }}>
      {children}
      <Box
        className="ms-tooltip"
        sx={{
          pointerEvents: "none",
          position: "absolute",
          zIndex: 50,
          width: "max-content",
          maxWidth: 224,
          borderRadius: "4px",
          px: 1,
          py: 0.5,
          fontSize: 11,
          lineHeight: 1.35,
          bgcolor: color || "#111827",
          color: "#fff",
          opacity: 0,
          transition: "opacity 150ms ease",
          whiteSpace: "normal",
          ...placement,
        }}
      >
        {text}
      </Box>
    </Box>
  );
}
