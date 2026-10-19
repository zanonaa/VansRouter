"use client";

import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";

const FEATURES = [
  { icon: "link", title: "Unified Endpoint", desc: "Access all providers via a single standard API URL.", accent: "#3b82f6" },
  { icon: "bolt", title: "Easy Setup", desc: "Get up and running in minutes with npx command.", accent: "#f97316" },
  { icon: "shield_with_heart", title: "Model Fallback", desc: "Automatically switch providers on failure or high latency.", accent: "#f43f5e" },
  { icon: "monitoring", title: "Usage Tracking", desc: "Detailed analytics and cost monitoring across all models.", accent: "#a855f7" },
  { icon: "key", title: "OAuth & API Keys", desc: "Securely manage credentials in one vault.", accent: "#f59e0b" },
  { icon: "cloud_sync", title: "Cloud Sync", desc: "Sync your configurations across devices instantly.", accent: "#0ea5e9" },
  { icon: "terminal", title: "CLI Support", desc: "Works with Claude Code, Codex, Cline, Cursor, and more.", accent: "#10b981" },
  { icon: "dashboard", title: "Dashboard", desc: "Visual dashboard for real-time traffic analysis.", accent: "#d946ef" },
];

export default function Features() {
  return (
    <Box component="section" sx={{ py: 12, px: 3 }} id="features">
      <Box sx={{ maxWidth: 1280, mx: "auto" }}>
        <Box sx={{ mb: 8 }}>
          <Typography component="h2" sx={{ fontSize: { xs: 30, md: 36 }, fontWeight: 700, mb: 2, color: "#ffffff" }}>
            Powerful Features
          </Typography>
          <Typography sx={{ color: "#9ca3af", maxWidth: 576, fontSize: 18 }}>
            Everything you need to manage your AI infrastructure in one place, built for scale.
          </Typography>
        </Box>

        <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)", lg: "repeat(4, 1fr)" }, gap: 2 }}>
          {FEATURES.map((feature) => (
            <Box
              key={feature.title}
              sx={{
                p: 3,
                borderRadius: "12px",
                bgcolor: "#1a1433",
                border: "1px solid #3a2f27",
                transition: "border-color 300ms, background-color 300ms",
                "&:hover": { borderColor: `${feature.accent}80`, bgcolor: `${feature.accent}0d` },
                "&:hover .feature-icon": { transform: "scale(1.1)" },
                "&:hover .feature-title": { color: feature.accent },
              }}
            >
              <Box
                className="feature-icon"
                sx={{
                  width: 40,
                  height: 40,
                  borderRadius: "8px",
                  bgcolor: `${feature.accent}1a`,
                  color: feature.accent,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  mb: 2,
                  transition: "transform 300ms",
                }}
              >
                <Icon className="material-symbols-outlined" baseClassName="">{feature.icon}</Icon>
              </Box>
              <Typography component="h3" className="feature-title" sx={{ fontSize: 18, fontWeight: 700, mb: 1, color: "#ffffff", transition: "color 150ms" }}>
                {feature.title}
              </Typography>
              <Typography sx={{ fontSize: 14, color: "#9ca3af", lineHeight: 1.625 }}>{feature.desc}</Typography>
            </Box>
          ))}
        </Box>
      </Box>
    </Box>
  );
}
