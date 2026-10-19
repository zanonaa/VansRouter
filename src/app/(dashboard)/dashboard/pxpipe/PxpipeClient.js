"use client";

import { useState, useEffect, useCallback } from "react";
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from "recharts";
import { Card, Button } from "@/shared/components";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import Table from "@mui/material/Table";
import TableContainer from "@mui/material/TableContainer";
import TableHead from "@mui/material/TableHead";
import TableBody from "@mui/material/TableBody";
import TableRow from "@mui/material/TableRow";
import TableCell from "@mui/material/TableCell";
import { alpha } from "@mui/material/styles";

function MsIcon({ children, sx, ...props }) {
  return <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 20, ...sx }} {...props}>{children}</Icon>;
}

const fmtTokens = (n) => {
  if (n >= 1000000) return `${(n / 1000000).toFixed(2)}M`;
  if (n >= 1000) return `${(n / 1000).toFixed(1)}K`;
  return String(n || 0);
};

const fmtUptime = (ms) => {
  if (!ms || ms <= 0) return "—";
  const m = Math.floor(ms / 60000);
  const h = Math.floor(m / 60);
  return h > 0 ? `${h}h${String(m % 60).padStart(2, "0")}m` : `${m}m`;
};

const WINDOW_TABS = [
  { id: "today", label: "Today" },
  { id: "yesterday", label: "Yesterday" },
  { id: "last7d", label: "7 days" },
  { id: "last30d", label: "30 days" },
  { id: "all", label: "All time" },
];

const REASON_LABELS = {
  applied: "Prompt exceeded threshold",
  below_threshold: "Below size threshold",
  not_profitable: "Compression not profitable",
  below_min_chars: "Below minimum chars",
  below_min_tokens: "Below minimum tokens",
  unsupported_model: "Model not in allowlist",
  unsupported_format: "Non-Claude request format",
  timeout: "Compression timed out",
  transform_error: "Transform error",
  passthrough: "Passthrough",
  disabled: "Disabled",
  not_installed: "Not installed",
};

function SummaryCard({ label, value, sub, tone }) {
  const color = tone === "text-success" ? "success.main" : tone === "text-warning" ? "warning.main" : "text.secondary";
  return (
    <Card padding="sm">
      <Typography sx={{ fontSize: 12, color: "text.secondary", textTransform: "uppercase", letterSpacing: "0.04em" }}>{label}</Typography>
      <Typography sx={{ fontSize: 20, fontWeight: 600, mt: 0.5, color }}>{value}</Typography>
      {sub && <Typography sx={{ fontSize: 12, color: "text.secondary", mt: 0.25 }}>{sub}</Typography>}
    </Card>
  );
}

export default function PxpipeClient() {
  const [status, setStatus] = useState(null);
  const [health, setHealth] = useState(null);
  const [stats, setStats] = useState(null);
  const [logs, setLogs] = useState(null);
  const [windowId, setWindowId] = useState("last7d");
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const [statusRes, statsRes, logsRes] = await Promise.all([
        fetch("/api/pxpipe/status", { headers: { "Cache-Control": "no-store" } }),
        fetch("/api/pxpipe/stats"),
        fetch("/api/pxpipe/logs?limit=50"),
      ]);
      setStatus(await statusRes.json());
      setStats(await statsRes.json());
      setLogs(await logsRes.json());
      const healthRes = await fetch("/api/pxpipe/health", { method: "POST" });
      setHealth(await healthRes.json());
    } catch {
      /* sections render placeholders */
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    // Intentional initial synchronization with the PXPIPE service.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    refresh();
  }, [refresh]);

  const w = stats?.windows?.[windowId];
  const statusLabel = !status
    ? "—"
    : !status.installed
      ? "Not installed"
      : health?.healthy
        ? "Healthy"
        : status.running
          ? "Running"
          : "Stopped";

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3, p: 3 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", flexWrap: "wrap", gap: 1.5 }}>
        <Typography component="h2" sx={{ fontSize: 18, fontWeight: 600, display: "flex", alignItems: "center", gap: 1 }}>
          <MsIcon sx={{ color: "primary.main" }}>image</MsIcon>
          PXPIPE Dashboard
        </Typography>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Box component="a" href="/dashboard/token-saver" sx={{ fontSize: 12, color: "primary.main", textDecoration: "underline", "&:hover": { opacity: 0.8 } }}>
            Token Saver settings
          </Box>
          <Button size="sm" variant="ghost" onClick={refresh} disabled={loading}>
            {loading ? "Refreshing…" : "Refresh"}
          </Button>
        </Box>
      </Box>

      <Box sx={{ display: "grid", gridTemplateColumns: { xs: "repeat(2, 1fr)", md: "repeat(3, 1fr)", lg: "repeat(6, 1fr)" }, gap: 1.5 }}>
        <SummaryCard label="Status" value={statusLabel} tone={health?.healthy ? "text-success" : status?.installed ? "text-warning" : "text-text-muted"} sub={status?.enabled ? "Enabled in pipeline" : "Disabled in pipeline"} />
        <SummaryCard label="Version" value={status?.version ? `v${status.version}` : "—"} sub="pxpipe-proxy" />
        <SummaryCard label="Uptime" value={fmtUptime(status?.uptimeMs)} sub="module loaded" />
        <SummaryCard label="Requests" value={w ? w.requests.toLocaleString() : "—"} />
        <SummaryCard label="Compressed" value={w ? w.compressed.toLocaleString() : "—"} tone="text-success" />
        <SummaryCard label="Bypassed" value={w ? w.bypassed.toLocaleString() : "—"} />
      </Box>

      <Card padding="sm">
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", flexWrap: "wrap", gap: 1.5, mb: 2 }}>
          <Typography sx={{ fontWeight: 500 }}>Token savings (estimated)</Typography>
          <Box sx={{ display: "flex", alignItems: "center", gap: 0.5, borderRadius: 2, border: 1, borderColor: "divider", bgcolor: "action.hover", p: 0.5 }}>
            {WINDOW_TABS.map((tab) => (
              <Box
                key={tab.id}
                component="button"
                onClick={() => setWindowId(tab.id)}
                sx={{
                  px: 1.5,
                  py: 0.5,
                  border: 0,
                  borderRadius: 1.5,
                  fontSize: 12,
                  fontWeight: 500,
                  cursor: "pointer",
                  transition: "color 150ms ease, background-color 150ms ease",
                  bgcolor: windowId === tab.id ? "primary.main" : "transparent",
                  color: windowId === tab.id ? "#fff" : "text.secondary",
                  boxShadow: windowId === tab.id ? 1 : "none",
                  "&:hover": { color: windowId === tab.id ? "#fff" : "text.primary", bgcolor: windowId === tab.id ? "primary.main" : "action.selected" },
                }}
              >{tab.label}</Box>
            ))}
          </Box>
        </Box>
        <Box sx={{ display: "grid", gridTemplateColumns: { xs: "repeat(2,1fr)", md: "repeat(4,1fr)" }, gap: 2, textAlign: "center" }}>
          {[["Original tokens", w && fmtTokens(w.tokensBeforeEst), false], ["After PXPIPE", w && fmtTokens(w.tokensAfterEst), false], ["Saved", w && fmtTokens(w.tokensSavedEst), true], ["Reduction", w && `${w.savedPct}%`, true]].map(([label, value, success]) => (
            <Box key={label}>
              <Typography sx={{ fontSize: 12, color: "text.secondary" }}>{label}</Typography>
              <Typography sx={{ fontSize: 18, fontWeight: 600, color: success ? "success.main" : "text.primary" }}>{value || "—"}</Typography>
            </Box>
          ))}
        </Box>
        <Typography sx={{ fontSize: 12, color: "text.secondary", mt: 1.5 }}>
          Estimates from body size before/after imaging; billed usage per request
          (recorded on the Usage page) remains the ground truth. Images generated:{" "}
          {w ? w.imagesGenerated.toLocaleString() : "—"} · avg compression time:{" "}
          {w ? `${w.avgCompressionMs}ms` : "—"} · errors: {w ? w.errors : "—"}
        </Typography>
      </Card>

      <Card padding="sm">
        <Typography component="h3" sx={{ fontWeight: 500, mb: 1.5 }}>Tokens saved — last 30 days</Typography>
        {stats?.timeline?.some((d) => d.tokensSavedEst > 0) ? (
          <ResponsiveContainer width="100%" height={220}>
            <AreaChart data={stats.timeline} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
              <defs>
                <linearGradient id="gradPxpipe" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#10b981" stopOpacity={0.25} />
                  <stop offset="95%" stopColor="#10b981" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" strokeOpacity={0.2} />
              <XAxis dataKey="date" tick={{ fontSize: 11 }} tickFormatter={(d) => d.slice(5)} />
              <YAxis tick={{ fontSize: 11 }} tickFormatter={fmtTokens} width={48} />
              <Tooltip formatter={(v) => [fmtTokens(v), "Tokens saved"]} labelFormatter={(d) => d} />
              <Area type="monotone" dataKey="tokensSavedEst" stroke="#10b981" fill="url(#gradPxpipe)" strokeWidth={2} />
            </AreaChart>
          </ResponsiveContainer>
        ) : (
          <Box sx={{ height: 128, display: "flex", alignItems: "center", justifyContent: "center", color: "text.secondary", fontSize: 14, textAlign: "center" }}>
            No savings recorded yet — enable PXPIPE in the Token Saver and route a large Claude-format request.
          </Box>
        )}
      </Card>

      <Card padding="sm">
        <Typography component="h3" sx={{ fontWeight: 500, mb: 1.5 }}>History</Typography>
        <TableContainer>
          <Table sx={{ minWidth: 700 }}>
            <TableHead>
              <TableRow>
                {[["Time", "left"], ["Model", "left"], ["Original", "right"], ["Compressed", "right"], ["Saved", "right"], ["%", "right"], ["Duration", "right"], ["Status", "left"]].map(([label, align]) => (
                  <TableCell key={label} align={align} sx={{ py: 1, pr: 1.5, fontSize: 12, color: "text.secondary", borderBottomColor: "divider" }}>{label}</TableCell>
                ))}
              </TableRow>
            </TableHead>
            <TableBody>
              {(stats?.recent || []).slice(0, 50).map((ev, i) => (
                <TableRow key={`${ev.ts}-${i}`}>
                  <TableCell sx={{ py: 0.75, pr: 1.5, whiteSpace: "nowrap", color: "text.secondary", borderBottomColor: "divider" }}>{new Date(ev.ts).toLocaleString()}</TableCell>
                  <TableCell sx={{ py: 0.75, pr: 1.5, fontFamily: "monospace", fontSize: 12, borderBottomColor: "divider" }}>{ev.provider ? `${ev.provider}/${ev.model}` : ev.model || "—"}</TableCell>
                  <TableCell align="right" sx={{ py: 0.75, pr: 1.5, fontFamily: "monospace", fontSize: 12, borderBottomColor: "divider" }}>{ev.applied ? fmtTokens(ev.tokensBeforeEst) : "—"}</TableCell>
                  <TableCell align="right" sx={{ py: 0.75, pr: 1.5, fontFamily: "monospace", fontSize: 12, borderBottomColor: "divider" }}>{ev.applied ? fmtTokens(ev.tokensAfterEst) : "—"}</TableCell>
                  <TableCell align="right" sx={{ py: 0.75, pr: 1.5, fontFamily: "monospace", fontSize: 12, color: "success.main", borderBottomColor: "divider" }}>{ev.applied ? fmtTokens(ev.tokensSavedEst) : "—"}</TableCell>
                  <TableCell align="right" sx={{ py: 0.75, pr: 1.5, fontFamily: "monospace", fontSize: 12, borderBottomColor: "divider" }}>{ev.applied ? `${ev.savedPct}%` : "—"}</TableCell>
                  <TableCell align="right" sx={{ py: 0.75, pr: 1.5, fontFamily: "monospace", fontSize: 12, borderBottomColor: "divider" }}>{ev.durationMs != null ? `${ev.durationMs}ms` : "—"}</TableCell>
                  <TableCell sx={{ py: 0.75, borderBottomColor: "divider" }}>
                    <Box component="span" title={ev.detail || ""} sx={(theme) => ({ fontSize: 12, px: 1, py: 0.25, borderRadius: 1, bgcolor: ev.applied ? alpha(theme.palette.success.main, 0.15) : ev.reason === "transform_error" || ev.reason === "timeout" ? alpha(theme.palette.error.main, 0.15) : alpha(theme.palette.warning.main, 0.15), color: ev.applied ? "success.main" : ev.reason === "transform_error" || ev.reason === "timeout" ? "error.main" : "warning.main" })}>
                      {ev.applied ? "Compressed" : REASON_LABELS[ev.reason] || ev.reason}
                    </Box>
                  </TableCell>
                </TableRow>
              ))}
              {(!stats?.recent || stats.recent.length === 0) && (
                <TableRow><TableCell colSpan={8} sx={{ py: 3, textAlign: "center", color: "text.secondary", fontSize: 14, border: 0 }}>No PXPIPE activity yet</TableCell></TableRow>
              )}
            </TableBody>
          </Table>
        </TableContainer>
      </Card>

      <Card padding="sm" id="logs">
        <Typography component="h3" sx={{ fontWeight: 500, mb: 1.5 }}>PXPIPE Logs</Typography>
        {logs?.installLog ? (
          <Box component="pre" sx={{ borderRadius: 1, bgcolor: (theme) => alpha(theme.palette.text.primary, 0.05), p: 1.5, fontSize: 12, fontFamily: "monospace", overflowX: "auto", maxHeight: 256, overflowY: "auto", whiteSpace: "pre-wrap" }}>
            {logs.installLog}
          </Box>
        ) : (
          <Typography sx={{ fontSize: 14, color: "text.secondary" }}>No install log yet.</Typography>
        )}
      </Card>
    </Box>
  );
}
