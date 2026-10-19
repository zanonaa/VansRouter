"use client";

import Box from "@mui/material/Box";
import Icon from "@mui/material/Icon";

/** Security warning banner with optional action link */
export default function SecurityWarning({ message, action }) {
  return (
    <Box
      sx={(theme) => ({
        display: "flex",
        alignItems: "center",
        gap: 1,
        px: 1.5,
        py: 1,
        borderRadius: "8px",
        bgcolor: "rgba(245,158,11,0.10)",
        border: 1,
        borderColor: "rgba(245,158,11,0.20)",
        color: theme.palette.mode === "dark" ? "#fbbf24" : "#b45309",
      })}
    >
      <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 16, flexShrink: 0, mt: 0.25 }}>
        warning
      </Icon>
      <Box component="p" sx={{ fontSize: 12, flex: 1, m: 0 }}>{message}</Box>
      {action && (
        <Box
          component="a"
          href={action.href}
          onClick={action.href.startsWith("#") ? (e) => {
            e.preventDefault();
            document.getElementById(action.href.slice(1))?.scrollIntoView({ behavior: "smooth" });
          } : undefined}
          sx={{ fontSize: 12, fontWeight: 500, textDecoration: "underline", flexShrink: 0, color: "inherit", "&:hover": { opacity: 0.8 } }}
        >
          {action.label}
        </Box>
      )}
    </Box>
  );
}
