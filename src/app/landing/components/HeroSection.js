"use client";

import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import MuiButton from "@mui/material/Button";
import Icon from "@mui/material/Icon";

export default function HeroSection() {
  return (
    <Box component="section" sx={{ position: "relative", pt: 16, pb: 10, px: 3, minHeight: "90vh", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", overflow: "hidden" }}>
      {/* Glow effect */}
      <Box sx={{ position: "absolute", top: 0, left: "50%", transform: "translateX(-50%)", width: 1000, height: 500, bgcolor: "rgba(99,102,241,0.10)", borderRadius: "50%", filter: "blur(120px)", pointerEvents: "none" }} />

      <Box sx={{ position: "relative", zIndex: 10, maxWidth: 896, width: "100%", textAlign: "center", display: "flex", flexDirection: "column", alignItems: "center", gap: 4 }}>
        {/* Version badge */}
        <Box sx={{ display: "inline-flex", alignItems: "center", gap: 1, borderRadius: "999px", border: "1px solid #3a2f27", bgcolor: "rgba(26,20,51,0.5)", px: 1.5, py: 0.5, fontSize: 12, fontWeight: 500, color: "#6366f1" }}>
          <Box sx={{ width: 8, height: 8, borderRadius: "50%", bgcolor: "#6366f1", animation: "pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite" }} />
          v1.0 is now live
        </Box>

        {/* Main heading */}
        <Typography component="h1" sx={{ fontSize: { xs: 48, md: 72 }, fontWeight: 900, lineHeight: 1.1, letterSpacing: "-0.02em", color: "#ffffff" }}>
          One Endpoint for <br />
          <Box component="span" sx={{ color: "#6366f1" }}>All AI Providers</Box>
        </Typography>

        {/* Description */}
        <Typography sx={{ fontSize: { xs: 18, md: 20 }, color: "#9ca3af", maxWidth: 672, mx: "auto", fontWeight: 300 }}>
          AI endpoint proxy with web dashboard - A JavaScript port of CLIProxyAPI. Works seamlessly with Claude Code, OpenAI Codex, Cline, RooCode, and other CLI tools.
        </Typography>

        {/* CTA Buttons */}
        <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", justifyContent: "center", gap: 2, width: "100%" }}>
          <MuiButton
            type="button"
            disableElevation
            sx={{
              height: 48,
              px: 4,
              borderRadius: "8px",
              bgcolor: "#4f46e5",
              color: "#ffffff",
              fontSize: 16,
              fontWeight: 700,
              textTransform: "none",
              boxShadow: "0 0 15px rgba(79,70,229,0.4)",
              display: "flex",
              alignItems: "center",
              gap: 1,
              "&:hover": { bgcolor: "#4338ca" },
            }}
          >
            <Icon className="material-symbols-outlined" baseClassName="">rocket_launch</Icon>
            Get Started
          </MuiButton>
          <MuiButton
            component="a"
            href="https://github.com/Vanszs/VansRouter"
            target="_blank"
            rel="noopener noreferrer"
            disableElevation
            sx={{
              height: 48,
              px: 4,
              borderRadius: "8px",
              border: "1px solid #3a2f27",
              bgcolor: "#1a1433",
              color: "#ffffff",
              fontSize: 16,
              fontWeight: 700,
              textTransform: "none",
              display: "flex",
              alignItems: "center",
              gap: 1,
              "&:hover": { bgcolor: "#3a2f27", borderColor: "#3a2f27" },
            }}
          >
            <Icon className="material-symbols-outlined" baseClassName="">code</Icon>
            View on GitHub
          </MuiButton>
        </Box>
      </Box>
    </Box>
  );
}
