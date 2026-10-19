"use client";

import Link from "next/link";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";

const LINK_SX = {
  color: "#9ca3af",
  fontSize: 14,
  textDecoration: "none",
  transition: "color 150ms",
  "&:hover": { color: "#6366f1" },
};

const BOTTOM_LINK_SX = {
  color: "#4b5563",
  fontSize: 14,
  textDecoration: "none",
  transition: "color 150ms",
  "&:hover": { color: "#ffffff" },
};

export default function Footer() {
  return (
    <Box component="footer" sx={{ borderTop: "1px solid #3a2f27", bgcolor: "#0f0d1a", pt: 8, pb: 4, px: 3 }}>
      <Box sx={{ maxWidth: 1280, mx: "auto" }}>
        <Box sx={{ display: "grid", gridTemplateColumns: { xs: "repeat(2, 1fr)", md: "repeat(4, 1fr)", lg: "repeat(5, 1fr)" }, gap: 4, mb: 8 }}>
          {/* Brand */}
          <Box sx={{ gridColumn: { xs: "span 2", lg: "span 2" } }}>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, mb: 3 }}>
              <Box sx={{ width: 24, height: 24, borderRadius: "4px", bgcolor: "#6366f1", display: "flex", alignItems: "center", justifyContent: "center", color: "#ffffff" }}>
                <Box component="svg" viewBox="0 0 32 32" sx={{ width: 16, height: 16 }} fill="none">
                  <path d="M16 5L22 14L16 27L10 14L16 5Z" fill="white" opacity="0.9" />
                  <path d="M10 14L16 27L10 20L6 14H10Z" fill="white" opacity="0.6" />
                  <path d="M22 14L16 27L22 20L26 14H22Z" fill="white" opacity="0.6" />
                  <circle cx="16" cy="9" r="2" fill="white" />
                </Box>
              </Box>
              <Typography component="h3" sx={{ color: "#ffffff", fontSize: 18, fontWeight: 700 }}>VansRouter</Typography>
            </Box>
            <Typography sx={{ color: "#6b7280", fontSize: 14, maxWidth: 320, mb: 3 }}>
              The unified endpoint for AI generation. Connect, route, and manage your AI providers with ease.
            </Typography>
            <Box sx={{ display: "flex", gap: 2 }}>
              <Box component="a" href="https://github.com/Vanszs/VansRouter" target="_blank" rel="noopener noreferrer" sx={{ color: "#9ca3af", transition: "color 150ms", "&:hover": { color: "#ffffff" } }}>
                <Icon className="material-symbols-outlined" baseClassName="">code</Icon>
              </Box>
            </Box>
          </Box>

          {/* Product */}
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            <Typography component="h4" sx={{ fontWeight: 700, color: "#ffffff" }}>Product</Typography>
            <Box component="a" href="#features" sx={LINK_SX}>Features</Box>
            <Box component={Link} href="/dashboard" sx={LINK_SX}>Dashboard</Box>
            <Box component="a" href="https://github.com/Vanszs/VansRouter" target="_blank" rel="noopener noreferrer" sx={LINK_SX}>Changelog</Box>
          </Box>

          {/* Resources */}
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            <Typography component="h4" sx={{ fontWeight: 700, color: "#ffffff" }}>Resources</Typography>
            <Box component="a" href="https://github.com/Vanszs/VansRouter#readme" target="_blank" rel="noopener noreferrer" sx={LINK_SX}>Documentation</Box>
            <Box component="a" href="https://github.com/Vanszs/VansRouter" target="_blank" rel="noopener noreferrer" sx={LINK_SX}>GitHub</Box>
            <Box component="a" href="https://www.npmjs.com/package/vansrouter" target="_blank" rel="noopener noreferrer" sx={LINK_SX}>NPM</Box>
          </Box>

          {/* Legal */}
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            <Typography component="h4" sx={{ fontWeight: 700, color: "#ffffff" }}>Legal</Typography>
            <Box component="a" href="https://github.com/Vanszs/VansRouter/blob/main/LICENSE" target="_blank" rel="noopener noreferrer" sx={LINK_SX}>MIT License</Box>
          </Box>
        </Box>

        {/* Bottom */}
        <Box sx={{ borderTop: "1px solid #3a2f27", pt: 4, display: "flex", flexDirection: { xs: "column", md: "row" }, justifyContent: "space-between", alignItems: "center", gap: 2 }}>
          <Typography sx={{ color: "#4b5563", fontSize: 14 }}>© 2025 VansRouter. All rights reserved.</Typography>
          <Box sx={{ display: "flex", gap: 3 }}>
            <Box component="a" href="https://github.com/Vanszs/VansRouter" target="_blank" rel="noopener noreferrer" sx={BOTTOM_LINK_SX}>GitHub</Box>
            <Box component="a" href="https://www.npmjs.com/package/vansrouter" target="_blank" rel="noopener noreferrer" sx={BOTTOM_LINK_SX}>NPM</Box>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
