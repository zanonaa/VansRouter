"use client";
import { useCopyToClipboard } from "@/shared/hooks/useCopyToClipboard";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";

const STEP_NUMBER_SX = {
  flex: "none",
  width: 32,
  height: 32,
  borderRadius: "50%",
  bgcolor: "rgba(99,102,241,0.20)",
  color: "#818cf8",
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  fontWeight: 700,
};

export default function GetStarted() {
  const { copied, copy } = useCopyToClipboard();

  const handleCopy = (text) => {
    copy(text, "landing");
  };

  return (
    <Box component="section" sx={{ py: 12, px: 3, bgcolor: "#0f0d1a" }}>
      <Box sx={{ maxWidth: 1280, mx: "auto" }}>
        <Box sx={{ display: "flex", flexDirection: { xs: "column", lg: "row" }, gap: 8, alignItems: "flex-start" }}>
          {/* Left: Steps */}
          <Box sx={{ flex: 1 }}>
            <Typography component="h2" sx={{ fontSize: { xs: 30, md: 36 }, fontWeight: 700, mb: 3, color: "#ffffff" }}>
              Get Started in 30 Seconds
            </Typography>
            <Typography sx={{ color: "#9ca3af", fontSize: 18, mb: 4 }}>
              Install VansRouter, configure your providers via web dashboard, and start routing AI requests.
            </Typography>

            <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
              <Box sx={{ display: "flex", gap: 2 }}>
                <Box sx={STEP_NUMBER_SX}>1</Box>
                <Box>
                  <Typography component="h3" sx={{ fontWeight: 700, fontSize: 18, color: "#ffffff" }}>Install VansRouter</Typography>
                  <Typography sx={{ fontSize: 14, color: "#9ca3af", mt: 0.5 }}>Run npx command to start the server instantly</Typography>
                </Box>
              </Box>

              <Box sx={{ display: "flex", gap: 2 }}>
                <Box sx={STEP_NUMBER_SX}>2</Box>
                <Box>
                  <Typography component="h3" sx={{ fontWeight: 700, fontSize: 18, color: "#ffffff" }}>Open Dashboard</Typography>
                  <Typography sx={{ fontSize: 14, color: "#9ca3af", mt: 0.5 }}>Configure providers and API keys via web interface</Typography>
                </Box>
              </Box>

              <Box sx={{ display: "flex", gap: 2 }}>
                <Box sx={STEP_NUMBER_SX}>3</Box>
                <Box>
                  <Typography component="h3" sx={{ fontWeight: 700, fontSize: 18, color: "#ffffff" }}>Route Requests</Typography>
                  <Typography sx={{ fontSize: 14, color: "#9ca3af", mt: 0.5 }}>Open http://127.0.0.1:20128/masuk; new installs default to 123456, so change it before exposure.</Typography>
                </Box>
              </Box>
            </Box>
          </Box>

          {/* Right: Code block */}
          <Box sx={{ flex: 1, width: "100%" }}>
            <Box sx={{ borderRadius: "12px", overflow: "hidden", bgcolor: "#1e1e1e", border: "1px solid #3a2f27", boxShadow: "0 25px 50px -12px rgba(0,0,0,0.25)" }}>
              {/* Terminal header */}
              <Box sx={{ display: "flex", alignItems: "center", gap: 1, px: 2, py: 1.5, bgcolor: "#252526", borderBottom: "1px solid #374151" }}>
                <Box sx={{ width: 12, height: 12, borderRadius: "50%", bgcolor: "#ef4444" }} />
                <Box sx={{ width: 12, height: 12, borderRadius: "50%", bgcolor: "#eab308" }} />
                <Box sx={{ width: 12, height: 12, borderRadius: "50%", bgcolor: "#22c55e" }} />
                <Box sx={{ ml: 1, fontSize: 12, color: "#6b7280", fontFamily: "ui-monospace, monospace" }}>terminal</Box>
              </Box>

              {/* Terminal content */}
              <Box sx={{ p: 3, fontFamily: "ui-monospace, monospace", fontSize: 14, lineHeight: 1.6, overflowX: "auto", color: "#e5e7eb" }}>
                <Box
                  component="button"
                  type="button"
                  sx={{ display: "flex", alignItems: "center", gap: 1, mb: 2, cursor: "pointer", background: "none", border: 0, p: 0, width: "100%", color: "inherit", "&:hover .copy-hint": { opacity: 1 } }}
                  onClick={() => handleCopy("npx vansrouter")}
                  aria-label="Copy command: npx vansrouter"
                >
                  <Box component="span" sx={{ color: "#4ade80" }}>$</Box>
                  <Box component="span" sx={{ color: "#ffffff" }}>npx vansrouter</Box>
                  <Box component="span" className="copy-hint" sx={{ ml: "auto", color: "#6b7280", fontSize: 12, opacity: 0, transition: "opacity 150ms" }}>
                    {copied === "landing" ? "✓ Copied" : "Copy"}
                  </Box>
                </Box>

                <Box sx={{ color: "#9ca3af", mb: 3 }}>
                  <Box component="span" sx={{ color: "#6366f1" }}>&gt;</Box> Starting VansRouter...<br />
                  <Box component="span" sx={{ color: "#6366f1" }}>&gt;</Box> Server running on <Box component="span" sx={{ color: "#60a5fa" }}>http://localhost:20128</Box><br />
                  <Box component="span" sx={{ color: "#6366f1" }}>&gt;</Box> Dashboard: <Box component="span" sx={{ color: "#60a5fa" }}>http://localhost:20128/masuk</Box><br />
                  <Box component="span" sx={{ color: "#4ade80" }}>&gt;</Box> Ready to route! ✓
                </Box>

                <Box sx={{ fontSize: 12, color: "#6b7280", mb: 1, borderTop: "1px solid #374151", pt: 2 }}>
                  📝 Configure providers in dashboard or use environment variables
                </Box>

                <Box sx={{ color: "#9ca3af", fontSize: 12 }}>
                  <Box component="span" sx={{ color: "#c084fc" }}>Data Location:</Box><br />
                  <Box component="span" sx={{ color: "#6b7280" }}>  macOS/Linux:</Box> ~/.9router/db/data.sqlite<br />
                  <Box component="span" sx={{ color: "#6b7280" }}>  Windows:</Box> %APPDATA%\9router\db\data.sqlite
                </Box>
              </Box>
            </Box>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
