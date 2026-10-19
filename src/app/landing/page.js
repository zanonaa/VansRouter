"use client";
import { useRouter } from "next/navigation";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import MuiButton from "@mui/material/Button";
import Navigation from "./components/Navigation";
import HeroSection from "./components/HeroSection";
import FlowAnimation from "./components/FlowAnimation";
import HowItWorks from "./components/HowItWorks";
import Features from "./components/Features";
import GetStarted from "./components/GetStarted";
import Footer from "./components/Footer";

export default function LandingPage() {
  const router = useRouter();
  return (
    <Box
      sx={{
        position: "relative",
        color: "#ffffff",
        overflowX: "hidden",
        "& ::selection": { bgcolor: "#6366f1", color: "#ffffff" },
      }}
    >
      {/* Animated Background */}
      <Box sx={{ position: "fixed", inset: 0, zIndex: 0, overflow: "hidden", pointerEvents: "none", bgcolor: "#141121" }}>
        {/* Grid pattern */}
        <Box
          sx={{
            position: "absolute",
            inset: 0,
            opacity: 0.06,
            backgroundImage:
              "linear-gradient(to right, #6366f1 1px, transparent 1px), linear-gradient(to bottom, #6366f1 1px, transparent 1px)",
            backgroundSize: "50px 50px",
          }}
        />

        {/* Animated gradient orbs */}
        <Box sx={{ position: "absolute", top: 0, left: "25%", width: 700, height: 700, bgcolor: "rgba(99,102,241,0.12)", borderRadius: "50%", filter: "blur(130px)" }} />
        <Box sx={{ position: "absolute", top: "33%", right: "25%", width: 600, height: 600, bgcolor: "rgba(168,85,247,0.10)", borderRadius: "50%", filter: "blur(130px)" }} />
        <Box sx={{ position: "absolute", bottom: 0, left: "50%", width: 650, height: 650, bgcolor: "rgba(59,130,246,0.08)", borderRadius: "50%", filter: "blur(130px)" }} />

        {/* Vignette effect */}
        <Box
          sx={{
            position: "absolute",
            inset: 0,
            background: "radial-gradient(circle at center, transparent 0%, rgba(20, 17, 33, 0.4) 100%)",
          }}
        />
      </Box>

      <Box sx={{ position: "relative", zIndex: 10 }}>
        <Navigation />

        <Box component="main">
          {/* Hero with Flow Animation */}
          <Box sx={{ position: "relative" }}>
            <HeroSection />
            <Box sx={{ display: "flex", justifyContent: "center", pb: 10 }}>
              <FlowAnimation />
            </Box>
          </Box>

          <GetStarted />
          <HowItWorks />
          <Features />

          {/* CTA Section */}
          <Box component="section" sx={{ py: 16, px: 3, position: "relative", overflow: "hidden" }}>
            <Box sx={{ position: "absolute", inset: 0, background: "linear-gradient(to top, rgba(99,102,241,0.05), transparent)", pointerEvents: "none" }} />
            <Box sx={{ maxWidth: 896, mx: "auto", textAlign: "center", position: "relative", zIndex: 10 }}>
              <Typography component="h2" sx={{ fontSize: { xs: 36, md: 48 }, fontWeight: 900, mb: 3 }}>
                Ready to Simplify Your AI Infrastructure?
              </Typography>
              <Typography sx={{ fontSize: 20, color: "#9ca3af", mb: 5, maxWidth: 672, mx: "auto" }}>
                Join developers who are streamlining their AI integrations with VansRouter. Open source and free to start.
              </Typography>
              <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, alignItems: "center", justifyContent: "center", gap: 2 }}>
                <MuiButton
                  type="button"
                  onClick={() => router.push("/dashboard")}
                  disableElevation
                  sx={{
                    width: { xs: "100%", sm: "auto" },
                    height: 56,
                    px: 5,
                    borderRadius: "8px",
                    bgcolor: "#6366f1",
                    color: "#ffffff",
                    fontSize: 18,
                    fontWeight: 700,
                    textTransform: "none",
                    boxShadow: "0 0 20px rgba(99,102,241,0.5)",
                    "&:hover": { bgcolor: "#4f46e5" },
                  }}
                >
                  Start Free
                </MuiButton>
                <MuiButton
                  type="button"
                  onClick={() => window.open("https://github.com/Vanszs/VansRouter#readme", "_blank")}
                  sx={{
                    width: { xs: "100%", sm: "auto" },
                    height: 56,
                    px: 5,
                    borderRadius: "8px",
                    border: "1px solid #3a2f27",
                    color: "#ffffff",
                    fontSize: 18,
                    fontWeight: 700,
                    textTransform: "none",
                    "&:hover": { bgcolor: "#1a1433", borderColor: "#3a2f27" },
                  }}
                >
                  Read Documentation
                </MuiButton>
              </Box>
            </Box>
          </Box>
        </Box>

        <Footer />
      </Box>
    </Box>
  );
}
