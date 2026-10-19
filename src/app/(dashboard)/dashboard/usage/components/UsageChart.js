"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import dynamic from "next/dynamic";
import Box from "@mui/material/Box";
import ButtonBase from "@mui/material/ButtonBase";
import Card from "@/shared/components/Card";
import { fmtTokens } from "./format";

const RechartsChart = dynamic(() => import("recharts").then(mod => {
  const { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } = mod;
  function Chart({ data, cfg }) {
    return (
      <ResponsiveContainer width="100%" height={220}>
        <AreaChart data={data} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
          <defs>
            <linearGradient id="gradTokens" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="#6366f1" stopOpacity={0.25} />
              <stop offset="95%" stopColor="#6366f1" stopOpacity={0} />
            </linearGradient>
            <linearGradient id="gradRequests" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="#14b8a6" stopOpacity={0.25} />
              <stop offset="95%" stopColor="#14b8a6" stopOpacity={0} />
            </linearGradient>
            <linearGradient id="gradCost" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="#f59e0b" stopOpacity={0.25} />
              <stop offset="95%" stopColor="#f59e0b" stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid strokeDasharray="3 3" strokeOpacity={0.1} />
          <XAxis
            dataKey="label"
            tick={{ fontSize: 10, fill: "currentColor", fillOpacity: 0.5 }}
            tickLine={false}
            axisLine={false}
            interval="preserveStartEnd"
          />
          <YAxis
            tick={{ fontSize: 10, fill: "currentColor", fillOpacity: 0.5 }}
            tickLine={false}
            axisLine={false}
            tickFormatter={cfg.formatter}
            width={50}
          />
          <Tooltip
            contentStyle={{
              backgroundColor: "var(--color-bg)",
              border: "1px solid var(--color-border)",
              borderRadius: "8px",
              fontSize: "12px",
            }}
            formatter={(value) => [cfg.formatter(value), cfg.label]}
          />
          <Area
            type="monotone"
            dataKey={cfg.dataKey}
            stroke={cfg.color}
            strokeWidth={2}
            fill={`url(#${cfg.gradId})`}
            dot={false}
            activeDot={{ r: 4 }}
          />
        </AreaChart>
      </ResponsiveContainer>
    );
  }
  Chart.displayName = "RechartsChart";
  return { default: Chart };
}), { ssr: false, loading: () => <Box sx={{ height: 220, width: "100%", borderRadius: "8px", border: 1, borderColor: "divider", bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(42,42,42,0.3)" : "rgba(244,244,245,0.3)") }} aria-label="Loading chart" /> });

const fmtCost = (n) => `$${(n || 0).toFixed(4)}`;
const fmtRequests = (n) => String(n || 0);

const VIEW_MODES = [
  { value: "tokens", label: "Tokens" },
  { value: "requests", label: "Requests" },
  { value: "cost", label: "Cost" },
];

const VIEW_CONFIG = {
  tokens:   { dataKey: "tokens",   color: "#6366f1", gradId: "gradTokens",   formatter: fmtTokens,   label: "Tokens" },
  requests: { dataKey: "requests", color: "#14b8a6", gradId: "gradRequests", formatter: fmtRequests, label: "Requests" },
  cost:     { dataKey: "cost",     color: "#f59e0b", gradId: "gradCost",     formatter: fmtCost,     label: "Cost" },
};

const subtleBg = (t) => (t.palette.mode === "dark" ? "#2a2a2a" : "#f4f4f5");

const viewBtnSx = (active) => ({
  px: 1.5,
  py: 0.5,
  borderRadius: "6px",
  fontSize: 14,
  fontWeight: 500,
  color: active ? "#fff" : "text.secondary",
  bgcolor: active ? "primary.main" : "transparent",
  transition: "background-color 150ms ease, color 150ms ease",
  "&:hover": active ? {} : { color: "text.primary", bgcolor: "action.hover" },
});

export default function UsageChart({ period = "7d" }) {
  const [data, setData] = useState([]);
  const [loading, setLoading] = useState(true);
  const [viewMode, setViewMode] = useState("tokens");
  const fetchedPeriodRef = useRef(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch(`/api/usage/chart?period=${period}`);
      if (res.ok) {
        const json = await res.json();
        setData(json);
      }
    } catch (e) {
      console.error("Failed to fetch chart data:", e);
    } finally {
      setLoading(false);
    }
  }, [period]);

  useEffect(() => {
    if (fetchedPeriodRef.current === period) return;
    fetchedPeriodRef.current = period;
    fetchData();
  }, [fetchData, period]);

  const cfg = VIEW_CONFIG[viewMode];
  const hasData = data.some((d) => (d[cfg.dataKey] || 0) > 0);

  return (
    <Card sx={{ display: "flex", minWidth: 0, flexDirection: "column", gap: 1.5, p: { xs: 1.5, sm: 2 } }}>
      <Box
        sx={{
          display: "grid",
          width: { xs: "100%", sm: "auto" },
          gridTemplateColumns: "repeat(3, 1fr)",
          alignItems: "center",
          gap: 0.5,
          borderRadius: "8px",
          border: 1,
          borderColor: "divider",
          bgcolor: subtleBg,
          p: 0.5,
          alignSelf: { sm: "flex-start" },
        }}
      >
        {VIEW_MODES.map((m) => (
          <ButtonBase
            key={m.value}
            onClick={() => setViewMode(m.value)}
            sx={viewBtnSx(viewMode === m.value)}
          >
            {m.label}
          </ButtonBase>
        ))}
      </Box>

      {loading ? (
        <Box sx={{ height: 192, display: "flex", alignItems: "center", justifyContent: "center", color: "text.secondary", fontSize: 14 }}>
          Loading...
        </Box>
      ) : !hasData ? (
        <Box sx={{ height: 192, display: "flex", alignItems: "center", justifyContent: "center", color: "text.secondary", fontSize: 14 }}>
          No data for this period
        </Box>
      ) : (
        <RechartsChart data={data} cfg={VIEW_CONFIG[viewMode]} />
      )}
    </Card>
  );
}
