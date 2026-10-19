"use client";

import { useEffect } from "react";
import Link from "next/link";
import Box from "@mui/material/Box";
import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";
import MuiButton from "@mui/material/Button";
import Icon from "@mui/material/Icon";

function MsIcon({ children, sx }) {
  return (
    <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 24, ...sx }}>
      {children}
    </Icon>
  );
}

export default function Error({ error, reset }) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <Box
      component="main"
      sx={{
        display: "flex",
        minHeight: "70vh",
        alignItems: "center",
        justifyContent: "center",
        bgcolor: "background.default",
        color: "text.primary",
        px: 3,
        py: 8,
      }}
    >
      <Paper
        elevation={0}
        variant="outlined"
        sx={{ width: "100%", maxWidth: 512, borderRadius: "10px", borderColor: "divider", bgcolor: "background.paper", p: 3, boxShadow: "var(--shadow-soft)" }}
      >
        <Box
          sx={{
            mb: 2,
            display: "flex",
            height: 44,
            width: 44,
            alignItems: "center",
            justifyContent: "center",
            borderRadius: "10px",
            bgcolor: "rgba(239,68,68,0.10)",
            color: "error.main",
          }}
        >
          <MsIcon>error</MsIcon>
        </Box>
        <Typography sx={{ fontSize: 20, fontWeight: 600 }}>Something went wrong</Typography>
        <Typography variant="body2" sx={{ mt: 1, lineHeight: 1.6, color: "text.secondary" }}>
          The page hit an unexpected error. Try again, or return to the dashboard if it keeps happening.
        </Typography>
        {error?.message ? (
          <Box
            component="pre"
            sx={{
              mt: 2,
              maxHeight: 144,
              overflow: "auto",
              borderRadius: "6px",
              border: 1,
              borderColor: "divider",
              bgcolor: "var(--color-surface-2)",
              p: 1.5,
              fontSize: 12,
              color: "text.secondary",
              whiteSpace: "pre-wrap",
            }}
          >
            {error.message}
          </Box>
        ) : null}
        <Box sx={{ mt: 3, display: "flex", flexWrap: "wrap", gap: 1.5 }}>
          <MuiButton
            variant="contained"
            disableElevation
            onClick={reset}
            startIcon={<MsIcon sx={{ fontSize: 18 }}>refresh</MsIcon>}
          >
            Try again
          </MuiButton>
          <MuiButton
            component={Link}
            href="/dashboard"
            variant="outlined"
            color="inherit"
            startIcon={<MsIcon sx={{ fontSize: 18 }}>dashboard</MsIcon>}
            sx={{
              borderColor: "divider",
              color: "text.primary",
              bgcolor: "background.paper",
              "&:hover": { bgcolor: "var(--color-surface-2)", borderColor: "divider" },
            }}
          >
            Dashboard
          </MuiButton>
        </Box>
      </Paper>
    </Box>
  );
}
