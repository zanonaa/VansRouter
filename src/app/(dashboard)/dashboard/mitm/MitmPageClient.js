"use client";

import { useState, useEffect, useReducer } from "react";
import Box from "@mui/material/Box";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import { alpha } from "@mui/material/styles";
import { MITM_TOOLS } from "@/shared/constants/cliTools";
import { getModelsByProviderId } from "@/shared/constants/models";
import { isOpenAICompatibleProvider, isAnthropicCompatibleProvider } from "@/shared/constants/providers";
import { MitmServerCard, MitmToolCard } from "@/app/(dashboard)/dashboard/cli-tools/components";

function MsIcon({ children, sx, ...props }) {
  return (
    <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18, ...sx }} {...props}>
      {children}
    </Icon>
  );
}

function dataReducer(state, action) {
  switch (action.type) {
    case "SET_CONNECTIONS": return { ...state, connections: action.value };
    case "SET_API_KEYS": return { ...state, apiKeys: action.value };
    case "SET_ALIASES": return { ...state, modelAliases: action.value };
    case "SET_CLOUD": return { ...state, cloudEnabled: action.value };
    case "SET_MITM_STATUS": return { ...state, mitmStatus: action.value };
    default: return state;
  }
}

export default function MitmPageClient() {
  const [data, dispatch] = useReducer(dataReducer, {
    connections: [],
    apiKeys: [],
    modelAliases: {},
    cloudEnabled: false,
    mitmStatus: { running: false, certExists: false, dnsStatus: {}, hasCachedPassword: false },
  });
  const { connections, apiKeys, modelAliases, cloudEnabled, mitmStatus } = data;
  const [expandedTool, setExpandedTool] = useState(null);

  /* eslint-disable react-hooks/immutability --
     One-time bootstrap fetch on mount. The fetch* functions are declared
     further down; captured by closure at runtime. */
  useEffect(() => {
    fetchConnections();
    fetchApiKeys();
    fetchAliases();
    fetchCloudSettings();
  }, []);

  const fetchConnections = async () => {
    try {
      const res = await fetch("/api/providers");
      if (res.ok) {
        const d = await res.json();
        dispatch({ type: "SET_CONNECTIONS", value: d.connections || [] });
      }
    } catch { /* ignore */ }
  };

  const fetchApiKeys = async () => {
    try {
      const res = await fetch("/api/keys");
      if (res.ok) {
        const d = await res.json();
        dispatch({ type: "SET_API_KEYS", value: d.keys || [] });
      }
    } catch { /* ignore */ }
  };

  const fetchAliases = async () => {
    try {
      const res = await fetch("/api/models/alias");
      if (res.ok) {
        const d = await res.json();
        dispatch({ type: "SET_ALIASES", value: d.aliases || {} });
      }
    } catch { /* ignore */ }
  };

  const fetchCloudSettings = async () => {
    try {
      const res = await fetch("/api/settings");
      if (res.ok) {
        const d = await res.json();
        dispatch({ type: "SET_CLOUD", value: d.cloudEnabled || false });
      }
    } catch { /* ignore */ }
  };

  const getActiveProviders = () => connections.filter(c => c.isActive !== false);

  const hasActiveProviders = () => {
    const active = getActiveProviders();
    return active.some(conn =>
      getModelsByProviderId(conn.provider).length > 0 ||
      isOpenAICompatibleProvider(conn.provider) ||
      isAnthropicCompatibleProvider(conn.provider)
    );
  };

  const mitmTools = Object.entries(MITM_TOOLS);

  return (
    <Stack spacing={3} sx={{ width: "100%" }}>
      <Box
        sx={(theme) => ({
          display: "flex",
          alignItems: "flex-start",
          gap: 1,
          px: 1.5,
          py: 1,
          borderRadius: 2,
          bgcolor: alpha(theme.palette.warning.main, 0.1),
          border: `1px solid ${alpha(theme.palette.warning.main, 0.3)}`,
        })}
      >
        <MsIcon sx={{ fontSize: 16, color: "warning.main", mt: 0.25, flexShrink: 0 }}>warning</MsIcon>
        <Typography
          sx={(theme) => ({
            fontSize: 12,
            lineHeight: 1.625,
            color: theme.palette.mode === "dark" ? "warning.main" : "error.main",
          })}
        >
          ⚠️ MITM intercepts HTTPS traffic of IDE tools (Antigravity, GitHub Copilot, Kiro) via local CA to redirect requests to your providers. May violate ToS → account ban. Use at your own risk.
        </Typography>
      </Box>

      {/* MITM Server Card */}
      <MitmServerCard
        apiKeys={apiKeys}
        cloudEnabled={cloudEnabled}
        onStatusChange={(v) => dispatch({ type: "SET_MITM_STATUS", value: v })}
      />

      {/* Tool Cards */}
      <Box sx={{ display: "grid", gap: { xs: 1.5, sm: 2 } }}>
        {mitmTools.map(([toolId, tool]) => (
          <MitmToolCard
            key={toolId}
            tool={tool}
            status={{ isExpanded: expandedTool === toolId, hasCachedPassword: mitmStatus.hasCachedPassword || false, needsSudoPassword: mitmStatus.needsSudoPassword !== false, isWin: mitmStatus.isWin === true }}
            onToggle={() => setExpandedTool(expandedTool === toolId ? null : toolId)}
            serverRunning={mitmStatus.running}
            dnsActive={mitmStatus.dnsStatus?.[toolId] || false}
            apiKeys={apiKeys}
            activeProviders={getActiveProviders()}
            hasActiveProviders={hasActiveProviders()}
            modelAliases={modelAliases}
            cloudEnabled={cloudEnabled}
            onDnsChange={(d) => dispatch({ type: "SET_MITM_STATUS", value: { ...mitmStatus, dnsStatus: d.dnsStatus ?? mitmStatus.dnsStatus } })}
          />
        ))}
      </Box>
    </Stack>
  );
}
