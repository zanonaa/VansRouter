"use client";

/**
 * ModelAvailabilityBadge — compact inline status indicator
 *
 * Shows green when all models are operational, or amber/red when there are
 * issues, with a hover popover for details and cooldown clearing.
 */

import { useState, useEffect, useCallback, useRef } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import { Button } from "@/shared/components";
import { useNotificationStore } from "@/store/notificationStore";

const STATUS_CONFIG = {
  available: { icon: "check_circle", color: "#22c55e", label: "Available" },
  cooldown: { icon: "schedule", color: "#f59e0b", label: "Cooldown" },
  unavailable: { icon: "error", color: "#ef4444", label: "Unavailable" },
  unknown: { icon: "help", color: "#6b7280", label: "Unknown" },
};

const iconButtonSx = {
  p: 0.5,
  border: 0,
  bgcolor: "transparent",
  borderRadius: "8px",
  color: "text.secondary",
  cursor: "pointer",
  display: "inline-flex",
  transition: "background-color 150ms ease, color 150ms ease",
  "&:hover": { bgcolor: "action.hover", color: "text.primary" },
};

export default function ModelAvailabilityBadge() {
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [expanded, setExpanded] = useState(false);
  const [clearing, setClearing] = useState(null);
  const ref = useRef(null);
  const notify = useNotificationStore();

  const fetchStatus = useCallback(async () => {
    try {
      const res = await fetch("/api/models/availability");
      if (res.ok) {
        const json = await res.json();
        setData(json);
      }
    } catch {
      // silent fail — will retry
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- periodic status poll with setInterval cleanup.
    fetchStatus();
    const interval = setInterval(fetchStatus, 30000);
    return () => clearInterval(interval);
  }, [fetchStatus]);

  // Close popover on outside click
  useEffect(() => {
    const handleClick = (e) => {
      if (ref.current && !ref.current.contains(e.target)) setExpanded(false);
    };
    if (expanded) document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [expanded]);

  const handleClearCooldown = async (provider, model) => {
    setClearing(`${provider}:${model}`);
    try {
      const res = await fetch("/api/models/availability", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action: "clearCooldown", provider, model }),
      });
      if (res.ok) {
        notify.success(`Cooldown cleared for ${model}`);
        await fetchStatus();
      } else {
        notify.error("Failed to clear cooldown");
      }
    } catch {
      notify.error("Failed to clear cooldown");
    } finally {
      setClearing(null);
    }
  };

  if (loading) return null;

  const models = data?.models || [];
  const unavailableCount = data?.unavailableCount || models.filter((m) => m.status !== "available").length;
  const isHealthy = unavailableCount === 0;

  // Group unhealthy models by provider
  const byProvider = {};
  models.forEach((m) => {
    if (m.status === "available") return;
    const key = m.provider || "unknown";
    if (!byProvider[key]) byProvider[key] = [];
    byProvider[key].push(m);
  });

  return (
    <Box sx={{ position: "relative" }} ref={ref}>
      {expanded && (
        <Box
          sx={{
            position: "absolute",
            top: "100%",
            right: 0,
            mt: 1,
            width: 320,
            bgcolor: "background.paper",
            border: 1,
            borderColor: "divider",
            borderRadius: "12px",
            boxShadow: "0 25px 50px -12px rgba(0,0,0,0.25)",
            zIndex: 50,
            overflow: "hidden",
          }}
        >
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              px: 2,
              py: 1.5,
              borderBottom: 1,
              borderColor: "divider",
              bgcolor: "background.default",
            }}
          >
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              <Icon
                className="material-symbols-outlined"
                baseClassName=""
                sx={{ fontSize: 16, color: isHealthy ? "#22c55e" : "#f59e0b" }}
              >
                {isHealthy ? "verified" : "warning"}
              </Icon>
              <Box component="span" sx={{ fontSize: 14, fontWeight: 600, color: "text.primary" }}>Model Status</Box>
            </Box>
            <Box
              component="button"
              type="button"
              onClick={fetchStatus}
              sx={iconButtonSx}
              title="Refresh"
            >
              <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 14 }}>refresh</Icon>
            </Box>
          </Box>

          <Box sx={{ px: 2, py: 1.5, maxHeight: 240, overflowY: "auto" }}>
            {isHealthy ? (
              <Typography sx={{ fontSize: 14, color: "text.secondary", textAlign: "center", py: 1 }}>
                All models are responding normally.
              </Typography>
            ) : (
              <Box sx={{ display: "flex", flexDirection: "column", gap: 1.25 }}>
                {Object.entries(byProvider).map(([provider, provModels]) => (
                  <Box key={provider}>
                    <Typography sx={{ fontSize: 12, fontWeight: 600, color: "text.primary", mb: 0.75, textTransform: "capitalize" }}>{provider}</Typography>
                    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
                      {provModels.map((m) => {
                        const status = STATUS_CONFIG[m.status] || STATUS_CONFIG.unknown;
                        const isClearing = clearing === `${m.provider}:${m.model}`;
                        return (
                          <Box
                            key={`${m.provider}-${m.model}`}
                            sx={{
                              display: "flex",
                              alignItems: "center",
                              justifyContent: "space-between",
                              px: 1.25,
                              py: 0.75,
                              borderRadius: "8px",
                              bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(38,38,38,0.3)" : "rgba(255,255,255,0.3)"),
                            }}
                          >
                            <Box sx={{ display: "flex", alignItems: "center", gap: 0.75, minWidth: 0 }}>
                              <Icon
                                className="material-symbols-outlined"
                                baseClassName=""
                                sx={{ fontSize: 14, flexShrink: 0, color: status.color }}
                              >
                                {status.icon}
                              </Icon>
                              <Box component="span" sx={{ fontFamily: "monospace", fontSize: 12, color: "text.primary", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{m.model}</Box>
                            </Box>
                            {m.status === "cooldown" && (
                              <Box sx={{ ml: 1, "& .MuiButton-root": { fontSize: 10, px: 0.75, py: 0.25, minHeight: 0, height: "auto" } }}>
                                <Button
                                  size="sm"
                                  variant="ghost"
                                  onClick={() => handleClearCooldown(m.provider, m.model)}
                                  disabled={isClearing}
                                >
                                  {isClearing ? "..." : "Clear"}
                                </Button>
                              </Box>
                            )}
                          </Box>
                        );
                      })}
                    </Box>
                  </Box>
                ))}
              </Box>
            )}
          </Box>
        </Box>
      )}
    </Box>
  );
}
