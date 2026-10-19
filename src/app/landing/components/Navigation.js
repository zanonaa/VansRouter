"use client";
import { useState } from "react";
import { useRouter } from "next/navigation";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import MuiButton from "@mui/material/Button";
import Icon from "@mui/material/Icon";

const NAV_LINK_SX = {
  color: "#d1d5db",
  fontSize: 14,
  fontWeight: 500,
  textDecoration: "none",
  transition: "color 150ms",
  "&:hover": { color: "#ffffff" },
};

export default function Navigation() {
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const router = useRouter();

  return (
    <Box
      component="nav"
      sx={{ position: "fixed", top: 0, zIndex: 50, width: "100%", bgcolor: "rgba(20,17,33,0.8)", backdropFilter: "blur(12px)", borderBottom: "1px solid #3a2f27" }}
    >
      <Box sx={{ maxWidth: 1280, mx: "auto", px: 3, height: 64, display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        {/* Logo */}
        <Box
          component="button"
          type="button"
          sx={{ display: "flex", alignItems: "center", gap: 1.5, cursor: "pointer", bgcolor: "transparent", border: "none", p: 0 }}
          onClick={() => router.push("/")}
          aria-label="VansRouter Home"
        >
          <Box sx={{ width: 32, height: 32, borderRadius: "4px", background: "linear-gradient(to bottom right, #6366f1, #8b5cf6)", display: "flex", alignItems: "center", justifyContent: "center", color: "#ffffff" }}>
            <Box component="svg" viewBox="0 0 32 32" sx={{ width: 20, height: 20 }} fill="none">
              <path d="M16 5L22 14L16 27L10 14L16 5Z" fill="white" opacity="0.9" />
              <path d="M10 14L16 27L10 20L6 14H10Z" fill="white" opacity="0.6" />
              <path d="M22 14L16 27L22 20L26 14H22Z" fill="white" opacity="0.6" />
              <circle cx="16" cy="9" r="2" fill="white" />
            </Box>
          </Box>
          <Typography component="span" sx={{ color: "#ffffff", fontSize: 20, fontWeight: 700, letterSpacing: "-0.02em" }}>
            VansRouter
          </Typography>
        </Box>

        {/* Desktop menu */}
        <Box sx={{ display: { xs: "none", md: "flex" }, alignItems: "center", gap: 4 }}>
          <Box component="a" href="#features" sx={NAV_LINK_SX}>Features</Box>
          <Box component="a" href="#how-it-works" sx={NAV_LINK_SX}>How it Works</Box>
          <Box component="a" href="https://github.com/Vanszs/VansRouter#readme" target="_blank" rel="noopener noreferrer" sx={NAV_LINK_SX}>Docs</Box>
          <Box component="a" href="https://github.com/Vanszs/VansRouter" target="_blank" rel="noopener noreferrer" sx={{ ...NAV_LINK_SX, display: "flex", alignItems: "center", gap: 0.5 }}>
            GitHub <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 14 }}>open_in_new</Icon>
          </Box>
        </Box>

        {/* CTA + Mobile menu */}
        <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
          <MuiButton
            type="button"
            onClick={() => router.push("/dashboard")}
            disableElevation
            sx={{
              display: { xs: "none", sm: "flex" },
              height: 36,
              borderRadius: "8px",
              px: 2,
              bgcolor: "#6366f1",
              color: "#ffffff",
              fontSize: 14,
              fontWeight: 700,
              textTransform: "none",
              boxShadow: "0 0 15px rgba(99,102,241,0.4)",
              "&:hover": { bgcolor: "#4f46e5", boxShadow: "0 0 20px rgba(99,102,241,0.6)" },
            }}
          >
            Get Started
          </MuiButton>
          <Box
            component="button"
            type="button"
            onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
            sx={{ display: { md: "none" }, color: "#ffffff", bgcolor: "transparent", border: "none", p: 0, cursor: "pointer" }}
            aria-label="Toggle menu"
          >
            <Icon className="material-symbols-outlined" baseClassName="">{mobileMenuOpen ? "close" : "menu"}</Icon>
          </Box>
        </Box>
      </Box>

      {/* Mobile menu dropdown */}
      {mobileMenuOpen && (
        <Box sx={{ display: { md: "none" }, borderTop: "1px solid #3a2f27", bgcolor: "rgba(20,17,33,0.95)", backdropFilter: "blur(12px)" }}>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2, p: 3 }}>
            <Box component="a" href="#features" onClick={() => setMobileMenuOpen(false)} sx={NAV_LINK_SX}>Features</Box>
            <Box component="a" href="#how-it-works" onClick={() => setMobileMenuOpen(false)} sx={NAV_LINK_SX}>How it Works</Box>
            <Box component="a" href="https://github.com/Vanszs/VansRouter#readme" target="_blank" rel="noopener noreferrer" sx={NAV_LINK_SX}>Docs</Box>
            <Box component="a" href="https://github.com/Vanszs/VansRouter" target="_blank" rel="noopener noreferrer" sx={NAV_LINK_SX}>GitHub</Box>
            <MuiButton
              type="button"
              onClick={() => router.push("/dashboard")}
              disableElevation
              sx={{ height: 36, borderRadius: "8px", bgcolor: "#6366f1", color: "#ffffff", fontSize: 14, fontWeight: 700, textTransform: "none", "&:hover": { bgcolor: "#4f46e5" } }}
            >
              Get Started
            </MuiButton>
          </Box>
        </Box>
      )}
    </Box>
  );
}
