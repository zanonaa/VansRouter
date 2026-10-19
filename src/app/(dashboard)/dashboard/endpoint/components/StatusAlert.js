"use client";

import Box from "@mui/material/Box";

const TONES = {
  success: { bgcolor: "rgba(34,197,94,0.10)", light: "#16a34a", dark: "#4ade80" },
  warning: { bgcolor: "rgba(234,179,8,0.10)", light: "#ca8a04", dark: "#facc15" },
  info: { bgcolor: "rgba(59,130,246,0.10)", light: "#2563eb", dark: "#60a5fa" },
  error: { bgcolor: "rgba(239,68,68,0.10)", light: "#dc2626", dark: "#f87171" },
};

/** Reusable status alert */
export default function StatusAlert({ status, className = "", sx }) {
  const tone = TONES[status.type] || TONES.error;
  const renderMessage = (msg) => {
    const parts = msg.split(/(https?:\/\/[^\s]+)/g);
    return parts.map((part, i) =>
      /^https?:\/\//.test(part)
        ? <Box key={i} component="a" href={part} target="_blank" rel="noreferrer" sx={{ textDecoration: "underline", fontWeight: 500, color: "inherit" }}>{part}</Box>
        : part
    );
  };

  return (
    <Box
      className={className}
      sx={(theme) => ({
        p: 1,
        borderRadius: "4px",
        fontSize: 14,
        bgcolor: tone.bgcolor,
        color: theme.palette.mode === "dark" ? tone.dark : tone.light,
        ...(typeof sx === "function" ? sx(theme) : sx),
      })}
    >
      {renderMessage(status.message)}
    </Box>
  );
}
