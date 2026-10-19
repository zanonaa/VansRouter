"use client";

import Box from "@mui/material/Box";
import ThemeToggle from "../ThemeToggle";

export default function AuthLayout({ children }) {
  return (
    <Box
      sx={{
        minHeight: "100vh",
        display: "flex",
        flexDirection: "column",
        position: "relative",
        bgcolor: "background.default",
        transition: "background-color 500ms",
        overflowX: "hidden",
        "&::selection": { bgcolor: "rgba(229,106,74,0.2)", color: "primary.main" },
      }}
    >
      {/* Background effects */}
      <Box
        aria-hidden="true"
        sx={{
          position: "fixed",
          top: "50%",
          left: "50%",
          transform: "translate(-50%, -50%)",
          width: 800,
          height: 800,
          borderRadius: "50%",
          bgcolor: "rgba(229,106,74,0.05)",
          filter: "blur(100px)",
          pointerEvents: "none",
          zIndex: 0,
        }}
      />
      <Box
        aria-hidden="true"
        sx={{
          position: "fixed",
          bottom: 0,
          right: 0,
          width: 600,
          height: 600,
          transform: "translate(33.333%, 33.333%)",
          borderRadius: "50%",
          bgcolor: "rgba(251,146,60,0.2)",
          filter: "blur(120px)",
          pointerEvents: "none",
          zIndex: 0,
        }}
      />

      {/* Theme toggle */}
      <Box sx={{ position: "absolute", top: 24, right: 24, zIndex: 20 }}>
        <ThemeToggle variant="card" />
      </Box>

      {/* Content */}
      <Box
        component="main"
        sx={{
          flex: 1,
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          justifyContent: "center",
          p: { xs: 2, sm: 3 },
          zIndex: 10,
          width: "100%",
          height: "100%",
        }}
      >
        {children}
      </Box>
    </Box>
  );
}
