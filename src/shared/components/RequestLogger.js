"use client";

import { useState, useEffect } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Switch from "@mui/material/Switch";
import Table from "@mui/material/Table";
import TableHead from "@mui/material/TableHead";
import TableBody from "@mui/material/TableBody";
import TableRow from "@mui/material/TableRow";
import TableCell from "@mui/material/TableCell";
import Card from "./Card";

export default function RequestLogger() {
  const [logs, setLogs] = useState([]);
  const [loading, setLoading] = useState(true);
  const [autoRefresh, setAutoRefresh] = useState(true);

  /* eslint-disable react-hooks/immutability --
     One-time bootstrap fetch on mount; fetchLogs is declared below. */
  useEffect(() => {
    fetchLogs();
  }, []);

  useEffect(() => {
    let interval;
    if (autoRefresh) {
      interval = setInterval(() => {
        fetchLogs(false);
      }, 3000);
    }
    return () => clearInterval(interval);
  }, [autoRefresh]);

  const fetchLogs = async (showLoading = true) => {
    if (showLoading) setLoading(true);
    try {
      const res = await fetch("/api/usage/request-logs");
      if (res.ok) {
        const data = await res.json();
        setLogs(data);
      }
    } catch (error) {
      console.error("Failed to fetch logs:", error);
    } finally {
      if (showLoading) setLoading(false);
    }
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <Typography variant="h6" sx={{ fontSize: 20, fontWeight: 600 }}>Request Logs</Typography>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <Box component="span" sx={{ fontSize: 14, fontWeight: 500, color: "text.secondary" }}>Auto Refresh (3s)</Box>
            <Switch
              checked={autoRefresh}
              onChange={() => setAutoRefresh(!autoRefresh)}
              size="small"
              inputProps={{ "aria-label": "Auto Refresh (3s)" }}
            />
          </Box>
        </Box>
      </Box>

      <Card sx={{ overflow: "hidden", bgcolor: "var(--color-surface-2)" }}>
        <Box sx={{ overflowX: "auto", maxHeight: 600, overflowY: "auto", fontFamily: "ui-monospace, monospace", fontSize: 12 }}>
          {loading && logs.length === 0 ? (
            <Box sx={{ p: 4, textAlign: "center", color: "text.secondary" }}>Loading logs...</Box>
          ) : logs.length === 0 ? (
            <Box sx={{ p: 4, textAlign: "center", color: "text.secondary" }}>No logs recorded yet.</Box>
          ) : (
            <Table sx={{ width: "100%", whiteSpace: "nowrap" }}>
              <TableHead sx={{ position: "sticky", top: 0, zIndex: 10, bgcolor: "var(--color-surface-2)" }}>
                <TableRow>
                  {["DateTime", "Model", "Provider", "Account", "In", "Out", "Status"].map((h) => (
                    <TableCell key={h} sx={{ px: 1.5, py: 1, borderRight: "1px solid var(--color-border)", color: "text.secondary", fontWeight: 600 }}>
                      {h}
                    </TableCell>
                  ))}
                </TableRow>
              </TableHead>
              <TableBody>
                {logs.map((log, i) => {
                  const parts = log.split(" | ");
                  if (parts.length < 7) return null;

                  const status = parts[6];
                  const isPending = status.includes("PENDING");
                  const isFailed = status.includes("FAILED");
                  const isSuccess = status.includes("OK");
                  const cellSx = { px: 1.5, py: 0.75, borderRight: "1px solid var(--color-border)" };

                  return (
                    <TableRow key={`${parts[0]}-${parts[1]}-${parts[3]}`} hover sx={{ bgcolor: isPending ? "rgba(229,106,74,0.05)" : undefined }}>
                      <TableCell sx={{ ...cellSx, color: "text.secondary" }}>{parts[0]}</TableCell>
                      <TableCell sx={{ ...cellSx, fontWeight: 500 }}>{parts[1]}</TableCell>
                      <TableCell sx={cellSx}>
                        <Box component="span" sx={{ px: 0.75, py: 0.25, borderRadius: "4px", bgcolor: "var(--color-surface-2)", border: "1px solid var(--color-border)", fontSize: 10, textTransform: "uppercase", fontWeight: 700 }}>
                          {parts[2]}
                        </Box>
                      </TableCell>
                      <TableCell sx={{ ...cellSx, maxWidth: 150, overflow: "hidden", textOverflow: "ellipsis" }} title={parts[3]}>{parts[3]}</TableCell>
                      <TableCell sx={{ ...cellSx, textAlign: "right", color: "primary.main" }}>{parts[4]}</TableCell>
                      <TableCell sx={{ ...cellSx, textAlign: "right", color: "success.main" }}>{parts[5]}</TableCell>
                      <TableCell sx={{ px: 1.5, py: 0.75, fontWeight: 700, color: isSuccess ? "success.main" : isFailed ? "error.main" : "primary.main", animation: !isSuccess && !isFailed ? "pulse 2s cubic-bezier(0.4,0,0.6,1) infinite" : undefined }}>
                        {status}
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          )}
        </Box>
      </Card>
      <Box sx={{ fontSize: 10, color: "text.secondary", fontStyle: "italic" }}>
        Logs are loaded from the request history database.
      </Box>
    </Box>
  );
}
