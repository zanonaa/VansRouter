"use client";
import { useEffect, useState } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import ProviderIcon from "@/shared/components/ProviderIcon";

const CLI_TOOLS = [
  { id: "claude", name: "Claude Code", image: "/providers/claude.webp" },
  { id: "codex", name: "OpenAI Codex", image: "/providers/codex.webp" },
  { id: "cline", name: "Cline", image: "/providers/cline.webp" },
  { id: "cursor", name: "Cursor", image: "/providers/cursor.webp" },
];

const PROVIDERS = [
  { id: "openai", name: "OpenAI", color: "#10b981" },
  { id: "anthropic", name: "Anthropic", color: "#fb923c" },
  { id: "gemini", name: "Gemini", color: "#3b82f6" },
  { id: "github", name: "GitHub Copilot", color: "#374151" },
];

export default function FlowAnimation() {
  const [activeFlow, setActiveFlow] = useState(0);

  useEffect(() => {
    const interval = setInterval(() => {
      setActiveFlow((prev) => (prev + 1) % PROVIDERS.length);
    }, 2000);
    return () => clearInterval(interval);
  }, []);

  return (
    <Box sx={{ mt: 8, width: "100%", maxWidth: 896, position: "relative", height: 360, display: { xs: "none", md: "flex" }, alignItems: "center", justifyContent: "center" }}>
      {/* VansRouter Hub - Center */}
      <Box sx={{ position: "relative", zIndex: 20, width: 128, height: 128, borderRadius: "50%", bgcolor: "#1a1433", border: "2px solid #6366f1", boxShadow: "0 0 40px rgba(99,102,241,0.3)", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 0.5, cursor: "pointer", transition: "transform 500ms", "&:hover": { transform: "scale(1.05)" } }}>
        <Box component="svg" viewBox="0 0 32 32" sx={{ width: 40, height: 40 }} fill="none">
          <path d="M16 5L22 14L16 27L10 14L16 5Z" fill="#6366f1" opacity="0.9" />
          <path d="M10 14L16 27L10 20L6 14H10Z" fill="#6366f1" opacity="0.6" />
          <path d="M22 14L16 27L22 20L26 14H22Z" fill="#6366f1" opacity="0.6" />
          <circle cx="16" cy="9" r="2" fill="#6366f1" />
        </Box>
        <Typography component="span" sx={{ fontSize: 12, fontWeight: 700, color: "#ffffff", letterSpacing: "0.1em", textTransform: "uppercase" }}>
          VansRouter
        </Typography>
        <Box sx={{ position: "absolute", inset: 0, borderRadius: "50%", border: "1px solid rgba(99,102,241,0.3)", opacity: 0.2 }} />
      </Box>

      {/* CLI Tools - Left side */}
      <Box sx={{ position: "absolute", left: 0, top: "50%", transform: "translateY(-50%)", display: "flex", flexDirection: "column", gap: 3.5 }}>
        {CLI_TOOLS.map((tool) => (
          <Box
            key={tool.id}
            sx={{ display: "flex", alignItems: "center", gap: 1.5, opacity: 0.7, transition: "opacity 150ms", "&:hover": { opacity: 1 } }}
          >
            <Box sx={{ width: 64, height: 64, borderRadius: "16px", bgcolor: "#1a1433", border: "1px solid #3a2f27", display: "flex", alignItems: "center", justifyContent: "center", overflow: "hidden", p: 1, transition: "border-color 150ms, transform 150ms", "&:hover": { borderColor: "rgba(99,102,241,0.5)", transform: "scale(1.05)" } }}>
              <ProviderIcon
                src={tool.image}
                alt={tool.name}
                size={48}
                fallbackText={tool.name.slice(0, 2).toUpperCase()}
              />
            </Box>
          </Box>
        ))}
      </Box>

      {/* SVG Lines from CLI to VansRouter */}
      <Box
        component="svg"
        sx={{ position: "absolute", inset: 0, width: "100%", height: "100%", zIndex: 10, pointerEvents: "none", stroke: "#a16207" }}
        xmlns="http://www.w3.org/2000/svg"
      >
        <path d="M 60 50 C 250 70, 250 180, 360 180" fill="none" strokeDasharray="5,5" strokeWidth="2" />
        <path d="M 60 140 C 250 140, 250 180, 360 180" fill="none" strokeDasharray="5,5" strokeWidth="2" />
        <path d="M 60 210 C 250 210, 250 180, 360 180" fill="none" strokeDasharray="5,5" strokeWidth="2" />
        <path d="M 60 300 C 250 280, 250 180, 360 180" fill="none" strokeDasharray="5,5" strokeWidth="2" />
      </Box>

      {/* SVG Lines from VansRouter to Providers */}
      <Box
        component="svg"
        sx={{ position: "absolute", inset: 0, width: "100%", height: "100%", zIndex: 10, pointerEvents: "none" }}
        xmlns="http://www.w3.org/2000/svg"
      >
        <path d="M 440 180 C 550 180, 550 50, 740 50" fill="none" stroke={activeFlow === 0 ? "#6366f1" : "rgb(75, 85, 99)"} strokeWidth={activeFlow === 0 ? "3" : "2"} style={activeFlow === 0 ? { animation: "pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite" } : undefined} />
        <path d="M 440 180 C 550 180, 550 130, 740 130" fill="none" stroke={activeFlow === 1 ? "#6366f1" : "rgb(75, 85, 99)"} strokeWidth={activeFlow === 1 ? "3" : "2"} style={activeFlow === 1 ? { animation: "pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite" } : undefined} />
        <path d="M 440 180 C 550 180, 550 230, 740 230" fill="none" stroke={activeFlow === 2 ? "#6366f1" : "rgb(75, 85, 99)"} strokeWidth={activeFlow === 2 ? "3" : "2"} style={activeFlow === 2 ? { animation: "pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite" } : undefined} />
        <path d="M 440 180 C 550 180, 550 310, 740 310" fill="none" stroke={activeFlow === 3 ? "#6366f1" : "rgb(75, 85, 99)"} strokeWidth={activeFlow === 3 ? "3" : "2"} style={activeFlow === 3 ? { animation: "pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite" } : undefined} />
      </Box>

      {/* AI Providers - Right side */}
      <Box sx={{ position: "absolute", right: 0, top: 0, bottom: 0, display: "flex", flexDirection: "column", justifyContent: "space-between", py: 3 }}>
        {PROVIDERS.map((provider, idx) => (
          <Box
            key={provider.id}
            title={provider.name}
            sx={{
              px: 2,
              py: 1,
              borderRadius: "8px",
              bgcolor: provider.color,
              color: "#ffffff",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              fontWeight: 700,
              fontSize: 12,
              transition: "transform 150ms, box-shadow 150ms",
              cursor: "help",
              minWidth: 140,
              transform: activeFlow === idx ? "scale(1.1)" : "none",
              boxShadow: activeFlow === idx ? "0 0 0 4px rgba(99,102,241,0.5)" : "0 10px 15px -3px rgba(0,0,0,0.1)",
              "&:hover": { transform: "scale(1.1)" },
            }}
          >
            {provider.name}
          </Box>
        ))}
      </Box>

      {/* Mobile fallback */}
      <Box sx={{ display: { xs: "block", md: "none" }, mt: 4, width: "100%", p: 2, borderRadius: "8px", bgcolor: "#1a1433", border: "1px solid #3a2f27" }}>
        <Typography sx={{ fontSize: 14, textAlign: "center", color: "#9ca3af" }}>
          Interactive diagram visible on desktop
        </Typography>
      </Box>
    </Box>
  );
}
