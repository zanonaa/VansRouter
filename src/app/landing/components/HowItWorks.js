"use client";

import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";

export default function HowItWorks() {
  return (
    <Box component="section" sx={{ py: 12, borderTop: "1px solid #3a2f27", borderBottom: "1px solid #3a2f27", bgcolor: "rgba(26,20,51,0.30)" }} id="how-it-works">
      <Box sx={{ maxWidth: 1280, mx: "auto", px: 3 }}>
        <Box sx={{ mb: 8 }}>
          <Typography component="h2" sx={{ fontSize: { xs: 30, md: 36 }, fontWeight: 700, mb: 2, color: "#ffffff" }}>
            How VansRouter Works
          </Typography>
          <Typography sx={{ color: "#9ca3af", maxWidth: 576, fontSize: 18 }}>
            Data flows seamlessly from your application through our intelligent routing layer to the best provider for the job.
          </Typography>
        </Box>

        <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", md: "repeat(3, 1fr)" }, gap: 4, position: "relative" }}>
          {/* Connection line */}
          <Box sx={{ display: { xs: "none", md: "block" }, position: "absolute", top: 48, left: "16%", right: "16%", height: 2, background: "linear-gradient(to right, #374151, #6366f1, #374151)", zIndex: 0 }} />

          {/* Step 1: CLI & SDKs */}
          <Box sx={{ display: "flex", flexDirection: "column", gap: 3, position: "relative", "&:hover .step-icon": { borderColor: "#6b7280" } }}>
            <Box className="step-icon" sx={{ width: 96, height: 96, borderRadius: "16px", bgcolor: "#141121", border: "1px solid #3a2f27", display: "flex", alignItems: "center", justifyContent: "center", boxShadow: "0 20px 25px -5px rgba(0,0,0,0.1)", transition: "border-color 150ms", zIndex: 10, mx: { xs: "auto", md: 0 } }}>
              <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 36, color: "#d1d5db" }}>terminal</Icon>
            </Box>
            <Box>
              <Typography component="h3" sx={{ fontSize: 20, fontWeight: 700, mb: 1, color: "#ffffff" }}>1. CLI &amp; SDKs</Typography>
              <Typography sx={{ fontSize: 14, color: "#9ca3af" }}>
                Your requests start from your favorite tools or our unified SDK. Just change the base URL.
              </Typography>
            </Box>
          </Box>

          {/* Step 2: VansRouter Hub */}
          <Box sx={{ display: "flex", flexDirection: "column", gap: 3, position: "relative", alignItems: { md: "center" }, textAlign: { md: "center" } }}>
            <Box sx={{ width: 96, height: 96, borderRadius: "16px", bgcolor: "#141121", border: "2px solid #6366f1", display: "flex", alignItems: "center", justifyContent: "center", boxShadow: "0 0 30px rgba(99,102,241,0.2)", zIndex: 10, mx: "auto" }}>
              <Box component="svg" viewBox="0 0 32 32" sx={{ width: 40, height: 40 }} fill="none">
                <path d="M16 5L22 14L16 27L10 14L16 5Z" fill="#6366f1" opacity="0.9" />
                <path d="M10 14L16 27L10 20L6 14H10Z" fill="#6366f1" opacity="0.6" />
                <path d="M22 14L16 27L22 20L26 14H22Z" fill="#6366f1" opacity="0.6" />
                <circle cx="16" cy="9" r="2" fill="#6366f1" />
              </Box>
            </Box>
            <Box>
              <Typography component="h3" sx={{ fontSize: 20, fontWeight: 700, mb: 1, color: "#6366f1" }}>2. VansRouter Hub</Typography>
              <Typography sx={{ fontSize: 14, color: "#9ca3af" }}>
                Our engine analyzes the prompt, checks provider health, and routes for lowest latency or cost.
              </Typography>
            </Box>
          </Box>

          {/* Step 3: AI Providers */}
          <Box sx={{ display: "flex", flexDirection: "column", gap: 3, position: "relative", alignItems: { md: "flex-end" }, textAlign: { md: "right" }, "&:hover .step-icon": { borderColor: "#6b7280" } }}>
            <Box className="step-icon" sx={{ width: 96, height: 96, borderRadius: "16px", bgcolor: "#141121", border: "1px solid #3a2f27", display: "flex", alignItems: "center", justifyContent: "center", boxShadow: "0 20px 25px -5px rgba(0,0,0,0.1)", transition: "border-color 150ms", zIndex: 10, mx: { xs: "auto", md: 0 } }}>
              <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 1 }}>
                <Box sx={{ width: 24, height: 24, borderRadius: "4px", bgcolor: "rgba(255,255,255,0.10)" }} />
                <Box sx={{ width: 24, height: 24, borderRadius: "4px", bgcolor: "rgba(255,255,255,0.10)" }} />
                <Box sx={{ width: 24, height: 24, borderRadius: "4px", bgcolor: "rgba(255,255,255,0.10)" }} />
                <Box sx={{ width: 24, height: 24, borderRadius: "4px", bgcolor: "rgba(255,255,255,0.10)" }} />
              </Box>
            </Box>
            <Box>
              <Typography component="h3" sx={{ fontSize: 20, fontWeight: 700, mb: 1, color: "#ffffff" }}>3. AI Providers</Typography>
              <Typography sx={{ fontSize: 14, color: "#9ca3af" }}>
                The request is fulfilled by OpenAI, Anthropic, Gemini, or others instantly.
              </Typography>
            </Box>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
