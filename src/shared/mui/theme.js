"use client";

import { createTheme } from "@mui/material/styles";

// 9Router palette, mirrored from src/app/globals.css so MUI and the existing
// Tailwind tokens agree while the shell is migrated piece by piece.
const BRAND = {
  500: "#E56A4A",
  600: "#cc5236",
};

const LIGHT = {
  bg: "#FDFAF6",
  surface: "#ffffff",
  surface2: "#f4f4f5",
  text: "#0a0a0a",
  muted: "#6B7280",
  border: "#e5e7eb",
  subtle: "#f1f1f3",
};

const DARK = {
  bg: "#1a1a1a",
  surface: "#262626",
  surface2: "#303030",
  text: "#ededed",
  muted: "#9ca3af",
  border: "#333333",
  subtle: "#2a2a2a",
};

export function getMuiTheme(mode = "light") {
  const dark = mode === "dark";
  const c = dark ? DARK : LIGHT;

  return createTheme({
    palette: {
      mode,
      primary: { main: BRAND[500], dark: BRAND[600], contrastText: "#ffffff" },
      background: { default: c.bg, paper: c.surface },
      text: { primary: c.text, secondary: c.muted },
      divider: c.border,
    },
    shape: { borderRadius: 12 },
    typography: {
      fontFamily: "var(--font-inter), ui-sans-serif, system-ui, sans-serif",
      h6: { fontWeight: 700, letterSpacing: "-0.01em" },
      subtitle2: { fontWeight: 600 },
      button: { textTransform: "none", fontWeight: 600 },
    },
    components: {
      MuiCssBaseline: {
        styleOverrides: {
          body: { backgroundColor: c.bg, color: c.text },
        },
      },
      MuiPaper: {
        styleOverrides: {
          root: { backgroundImage: "none" },
        },
      },
      MuiDrawer: {
        styleOverrides: {
          paper: {
            backgroundColor: dark ? c.surface : "rgba(244, 241, 236, 0.85)",
            backdropFilter: "blur(12px)",
            borderRight: `1px solid ${c.border}`,
          },
        },
      },
      MuiListItemButton: {
        defaultProps: { disableRipple: true },
        styleOverrides: {
          root: {
            borderRadius: 10,
            marginBottom: 2,
            transition: "background-color 150ms ease, color 150ms ease",
            "&.Mui-selected": {
              backgroundColor: `${BRAND[500]}1f`,
              color: BRAND[500],
              "& .MuiListItemIcon-root": { color: BRAND[500] },
              "&:hover": { backgroundColor: `${BRAND[500]}2b` },
            },
          },
        },
      },
      MuiListItemIcon: { styleOverrides: { root: { minWidth: 34, color: c.muted } } },
      MuiAppBar: {
        styleOverrides: {
          root: {
            backgroundColor: dark ? "rgba(26,26,26,0.75)" : "rgba(255,255,255,0.65)",
            backdropFilter: "blur(12px)",
            color: c.text,
            boxShadow: "none",
            borderBottom: `1px solid ${c.subtle}`,
          },
        },
      },
      MuiIconButton: { styleOverrides: { root: { borderRadius: 10 } } },
      MuiAvatar: { styleOverrides: { root: { width: 30, height: 30, fontSize: 13 } } },
    },
  });
}
