"use client";

import { useState, useMemo } from "react";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Cell,
} from "recharts";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import ButtonBase from "@mui/material/ButtonBase";
import Card from "@/shared/components/Card";
import { fmtTokens } from "./format";

const COLORS = ["#6366f1", "#14b8a6", "#f59e0b", "#ef4444", "#8b5cf6"];

const truncate = (s, max = 22) => (s && s.length > max ? s.slice(0, max) + "…" : s || "");

const subtleBg = (t) => (t.palette.mode === "dark" ? "#2a2a2a" : "#f4f4f5");

const viewBtnSx = (active) => ({
  px: 1.25,
  py: 0.25,
  borderRadius: "6px",
  fontSize: 12,
  fontWeight: 500,
  color: active ? "#fff" : "text.secondary",
  bgcolor: active ? "primary.main" : "transparent",
  transition: "background-color 150ms ease, color 150ms ease",
  "&:hover": active ? {} : { color: "text.primary", bgcolor: "action.hover" },
});

export default function TopModelsChart({ byModel }) {
  const [viewMode, setViewMode] = useState("tokens");

  const chartData = useMemo(() => {
    if (!byModel) return [];
    return Object.values(byModel)
      .map((data) => ({
        name: truncate(data.rawModel || "Unknown"),
        tokens: (data.promptTokens || 0) + (data.completionTokens || 0),
        requests: data.requests || 0,
      }))
      .filter((d) => d[viewMode] > 0)
      .sort((a, b) => b[viewMode] - a[viewMode])
      .slice(0, 5);
  }, [byModel, viewMode]);

  const fmt = viewMode === "tokens" ? fmtTokens : String;
  const label = viewMode === "tokens" ? "Tokens" : "Requests";

  return (
    <Card sx={{ display: "flex", minWidth: 0, flexDirection: "column", gap: 1.5, p: { xs: 1.5, sm: 2 } }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography sx={{ fontSize: 14, fontWeight: 600, color: "text.secondary", textTransform: "uppercase", letterSpacing: "0.025em" }}>
          Top Models
        </Typography>
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: "repeat(2, 1fr)",
            alignItems: "center",
            gap: 0.5,
            borderRadius: "8px",
            border: 1,
            borderColor: "divider",
            bgcolor: subtleBg,
            p: 0.5,
          }}
        >
          <ButtonBase sx={viewBtnSx(viewMode === "tokens")} onClick={() => setViewMode("tokens")}>
            Tokens
          </ButtonBase>
          <ButtonBase sx={viewBtnSx(viewMode === "requests")} onClick={() => setViewMode("requests")}>
            Requests
          </ButtonBase>
        </Box>
      </Box>

      {!chartData.length ? (
        <Box sx={{ height: 176, display: "flex", alignItems: "center", justifyContent: "center", color: "text.secondary", fontSize: 14 }}>
          No model usage yet
        </Box>
      ) : (
        <ResponsiveContainer width="100%" height={180}>
          <BarChart data={chartData} layout="vertical" margin={{ top: 4, right: 40, left: 4, bottom: 4 }}>
            <CartesianGrid strokeDasharray="3 3" strokeOpacity={0.1} horizontal={false} />
            <XAxis
              type="number"
              tick={{ fontSize: 10, fill: "currentColor", fillOpacity: 0.5 }}
              tickLine={false}
              axisLine={false}
              tickFormatter={fmt}
            />
            <YAxis
              type="category"
              dataKey="name"
              tick={{ fontSize: 10, fill: "currentColor", fillOpacity: 0.7 }}
              tickLine={false}
              axisLine={false}
              width={90}
            />
            <Tooltip
              contentStyle={{
                backgroundColor: "var(--color-bg)",
                border: "1px solid var(--color-border)",
                borderRadius: "8px",
                fontSize: "12px",
              }}
              formatter={(value) => [fmt(value), label]}
            />
            <Bar dataKey={viewMode} radius={[0, 4, 4, 0]}>
              {chartData.map((_, i) => (
                <Cell key={i} fill={COLORS[i % COLORS.length]} fillOpacity={0.85} />
              ))}
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      )}
    </Card>
  );
}
