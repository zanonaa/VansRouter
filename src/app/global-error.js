"use client";

import { useEffect } from "react";
import "./globals.css";
import Box from "@mui/material/Box";
import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";
import MuiButton from "@mui/material/Button";
import Icon from "@mui/material/Icon";
import { AppRouterCacheProvider } from "@mui/material-nextjs/v16-appRouter";
import MuiProviders from "@/shared/mui/MuiProviders";

function MsIcon({ children, sx }) {
  return (
    <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 24, ...sx }}>
      {children}
    </Icon>
  );
}

export default function GlobalError({ error, reset }) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <html lang="en">
      <body>
        {/* global-error replaces the root layout, so the MUI cache + theme have
            to be provided locally here. */}
        <AppRouterCacheProvider options={{ key: "mui" }}>
          <MuiProviders>
            <Box
              component="main"
              sx={{
                display: "flex",
                minHeight: "100vh",
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
                  <MsIcon>emergency_home</MsIcon>
                </Box>
                <Typography sx={{ fontSize: 20, fontWeight: 600 }}>VansAI could not load</Typography>
                <Typography variant="body2" sx={{ mt: 1, lineHeight: 1.6, color: "text.secondary" }}>
                  A critical error stopped the app shell from rendering. Try loading it again.
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
                <MuiButton
                  variant="contained"
                  disableElevation
                  onClick={reset}
                  sx={{ mt: 3 }}
                  startIcon={<MsIcon sx={{ fontSize: 18 }}>refresh</MsIcon>}
                >
                  Reload
                </MuiButton>
              </Paper>
            </Box>
          </MuiProviders>
        </AppRouterCacheProvider>
      </body>
    </html>
  );
}
