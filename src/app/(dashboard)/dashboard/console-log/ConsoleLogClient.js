"use client";

import { useState, useEffect, useRef, useMemo } from "react";
import { Card, Button, Badge, Input } from "@/shared/components";
import { CONSOLE_LOG_CONFIG } from "@/shared/constants/config";
import Box from "@mui/material/Box";
import Stack from "@mui/material/Stack";
import Icon from "@mui/material/Icon";

function MsIcon({ children, sx, ...props }) {
  return (
    <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18, ...sx }} {...props}>
      {children}
    </Icon>
  );
}

const LOG_LEVEL_COLORS = {
  LOG: "#34d399",
  INFO: "#60a5fa",
  WARN: "#facc15",
  ERROR: "#f87171",
  DEBUG: "#c084fc",
};

function detectLogLevel(line) {
  if (!line || typeof line !== "string") return "INFO";
  const upper = line.toUpperCase();
  if (upper.includes("ERROR") || upper.includes("EXCEPTION") || upper.includes("FAIL") || upper.includes("PM2-ERROR") || upper.includes("ERR_")) {
    return "ERROR";
  }
  if (upper.includes("WARN") || upper.includes("WARNING")) {
    return "WARN";
  }
  if (upper.includes("DEBUG")) {
    return "DEBUG";
  }
  if (upper.includes("LOG") || upper.includes("INFO") || upper.includes("[PM2-OUT]")) {
    return "INFO";
  }
  return "INFO";
}

function renderFormattedLine(line, key) {
  const level = detectLogLevel(line);
  const color = LOG_LEVEL_COLORS[level] || "#d1d5db";

  let bgSx = {};
  if (level === "ERROR") {
    bgSx = { bgcolor: "rgba(239,68,68,0.1)", borderLeft: "2px solid #ef4444", pl: 1, py: 0.25, my: 0.25, borderTopRightRadius: 4, borderBottomRightRadius: 4 };
  } else if (level === "WARN") {
    bgSx = { bgcolor: "rgba(234,179,8,0.1)", borderLeft: "2px solid #eab308", pl: 1, py: 0.25, my: 0.25, borderTopRightRadius: 4, borderBottomRightRadius: 4 };
  }

  return (
    <Box
      key={key}
      sx={{
        fontFamily: "monospace",
        fontSize: 12,
        lineHeight: 1.625,
        wordBreak: "break-all",
        color,
        fontWeight: level === "ERROR" ? 500 : 400,
        ...bgSx,
      }}
    >
      {line}
    </Box>
  );
}

export default function ConsoleLogClient() {
  const [appLogs, setAppLogs] = useState([]);
  const [pm2Logs, setPm2Logs] = useState([]);
  const [dockerLogs, setDockerLogs] = useState([]);
  const [pm2Info, setPm2Info] = useState({ available: false, errorCount: 0 });
  const [dockerInfo, setDockerInfo] = useState({ isDocker: false, available: false });

  const [activeTab, setActiveTab] = useState("all"); // 'all' | 'app' | 'pm2' | 'docker'
  const [levelFilter, setLevelFilter] = useState("ALL"); // 'ALL' | 'ERROR' | 'WARN' | 'INFO'
  const [searchQuery, setSearchQuery] = useState("");
  const [autoScroll, setAutoScroll] = useState(true);
  const [connected, setConnected] = useState(false);
  const [loading, setLoading] = useState(true);

  const logRef = useRef(null);

  // Fetch initial PM2, Docker, and App logs
  const fetchSystemLogs = async () => {
    try {
      setLoading(true);
      const res = await fetch("/api/translator/console-logs");
      const data = await res.json();
      if (data.success) {
        if (data.logs) setAppLogs(data.logs.slice(-CONSOLE_LOG_CONFIG.maxLines));
        if (data.pm2Logs) setPm2Logs(data.pm2Logs);
        if (data.pm2Info) setPm2Info(data.pm2Info);
        if (data.dockerLogs) setDockerLogs(data.dockerLogs);
        if (data.dockerInfo) setDockerInfo(data.dockerInfo);
      }
    } catch (err) {
      console.error("Failed to fetch system logs:", err);
    } finally {
      setLoading(false);
    }
  };

  const handleClearAppLogs = async () => {
    try {
      await fetch("/api/translator/console-logs", { method: "DELETE" });
      setAppLogs([]);
    } catch (err) {
      console.error("Failed to clear console logs:", err);
    }
  };

  useEffect(() => {
    // Intentional initial synchronization with the external log stream.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    fetchSystemLogs();

    const es = new EventSource("/api/translator/console-logs/stream");

    es.onopen = () => setConnected(true);

    es.onmessage = (e) => {
      const msg = JSON.parse(e.data);
      if (msg.type === "init") {
        setAppLogs(msg.logs.slice(-CONSOLE_LOG_CONFIG.maxLines));
      } else if (msg.type === "line") {
        setAppLogs((prev) => {
          const next = [...prev, msg.line];
          return next.length > CONSOLE_LOG_CONFIG.maxLines ? next.slice(-CONSOLE_LOG_CONFIG.maxLines) : next;
        });
      } else if (msg.type === "lines") {
        setAppLogs((prev) => {
          const next = [...prev, ...msg.lines];
          return next.length > CONSOLE_LOG_CONFIG.maxLines ? next.slice(-CONSOLE_LOG_CONFIG.maxLines) : next;
        });
      } else if (msg.type === "clear") {
        setAppLogs([]);
      }
    };

    es.onerror = () => setConnected(false);

    return () => es.close();
  }, []);

  // Combine logs based on active tab
  const combinedLogs = useMemo(() => {
    let sourceList = [];
    if (activeTab === "all") {
      sourceList = [...appLogs, ...pm2Logs, ...dockerLogs];
    } else if (activeTab === "app") {
      sourceList = appLogs;
    } else if (activeTab === "pm2") {
      sourceList = pm2Logs;
    } else if (activeTab === "docker") {
      sourceList = dockerLogs;
    }

    return sourceList.filter((line) => {
      if (!line) return false;
      const level = detectLogLevel(line);

      if (levelFilter === "ERROR" && level !== "ERROR") return false;
      if (levelFilter === "WARN" && level !== "WARN") return false;
      if (levelFilter === "INFO" && level !== "INFO" && level !== "LOG") return false;

      if (searchQuery.trim()) {
        return line.toLowerCase().includes(searchQuery.toLowerCase());
      }
      return true;
    });
  }, [activeTab, levelFilter, searchQuery, appLogs, pm2Logs, dockerLogs]);

  // Compute error count across active tab logs
  const totalErrorCount = useMemo(() => {
    const sourceList = activeTab === "all" ? [...appLogs, ...pm2Logs, ...dockerLogs] :
                       activeTab === "app" ? appLogs :
                       activeTab === "pm2" ? pm2Logs : dockerLogs;
    return sourceList.filter((line) => detectLogLevel(line) === "ERROR").length;
  }, [activeTab, appLogs, pm2Logs, dockerLogs]);

  // Auto-scroll to bottom on new logs
  useEffect(() => {
    if (!autoScroll || !logRef.current) return;
    logRef.current.scrollTop = logRef.current.scrollHeight;
  }, [combinedLogs, autoScroll]);

  const handleCopyLogs = () => {
    const text = combinedLogs.join("\n");
    navigator.clipboard.writeText(text);
  };

  const tabButtonSx = (active) => ({
    px: 1.5,
    py: 0.75,
    fontSize: 12,
    fontWeight: 500,
    borderRadius: 2,
    transition: "background-color 150ms ease, color 150ms ease",
    display: "inline-flex",
    alignItems: "center",
    gap: 0.75,
    flexShrink: 0,
    border: 0,
    cursor: "pointer",
    color: active ? "#ffffff" : "text.secondary",
    bgcolor: active ? "primary.main" : "action.hover",
    "&:hover": { color: active ? "#ffffff" : "text.primary" },
  });

  const chipSx = (bg) => ({
    px: 0.75,
    py: 0.25,
    borderRadius: 999,
    fontSize: 10,
    bgcolor: bg || "rgba(0,0,0,0.2)",
  });

  const levelButtonSx = (active, tone) => {
    const tones = {
      all: { bg: "rgba(229,106,74,0.2)", color: "primary.main", border: "rgba(229,106,74,0.4)", hover: "text.primary" },
      error: { bg: "rgba(239,68,68,0.2)", color: "#f87171", border: "rgba(239,68,68,0.5)", hover: "#f87171" },
      warn: { bg: "rgba(234,179,8,0.2)", color: "#facc15", border: "rgba(234,179,8,0.5)", hover: "#facc15" },
      info: { bg: "rgba(59,130,246,0.2)", color: "#60a5fa", border: "rgba(59,130,246,0.5)", hover: "#60a5fa" },
    };
    const t = tones[tone];
    return {
      px: 1.25,
      py: 0.5,
      fontSize: 12,
      borderRadius: 1,
      transition: "background-color 150ms ease, color 150ms ease",
      display: "inline-flex",
      alignItems: "center",
      gap: 0.5,
      border: active ? `1px solid ${t.border}` : "1px solid transparent",
      cursor: "pointer",
      color: active ? t.color : "text.secondary",
      bgcolor: active ? t.bg : "action.hover",
      fontWeight: active ? 700 : 400,
      "&:hover": { color: active ? t.color : t.hover },
    };
  };

  return (
    <Stack spacing={2}>
      {/* Header Controls */}
      <Card padding="sm">
        <Box sx={{ display: "flex", flexDirection: { xs: "column", md: "row" }, gap: 2, alignItems: { md: "center" }, justifyContent: { md: "space-between" } }}>
          {/* Source Tabs */}
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, overflowX: "auto", pb: { xs: 0.5, md: 0 } }}>
            <Box component="button" onClick={() => setActiveTab("all")} sx={tabButtonSx(activeTab === "all")}>
              <Box component="span">All Logs</Box>
              <Box component="span" sx={chipSx()}>
                {appLogs.length + pm2Logs.length + dockerLogs.length}
              </Box>
            </Box>

            <Box component="button" onClick={() => setActiveTab("app")} sx={tabButtonSx(activeTab === "app")}>
              <MsIcon sx={{ fontSize: 14 }}>terminal</MsIcon>
              <Box component="span">App Console</Box>
              <Box component="span" sx={chipSx()}>{appLogs.length}</Box>
            </Box>

            <Box component="button" onClick={() => setActiveTab("pm2")} sx={tabButtonSx(activeTab === "pm2")}>
              <MsIcon sx={{ fontSize: 14 }}>dns</MsIcon>
              <Box component="span">PM2 Server Logs</Box>
              {pm2Info.errorCount > 0 && (
                <Box component="span" sx={{ px: 0.75, py: 0.25, borderRadius: 999, fontSize: 10, bgcolor: "error.main", color: "#fff", fontWeight: 700 }}>
                  {pm2Info.errorCount} Err
                </Box>
              )}
            </Box>

            {(dockerInfo.available || dockerLogs.length > 0) && (
              <Box component="button" onClick={() => setActiveTab("docker")} sx={tabButtonSx(activeTab === "docker")}>
                <MsIcon sx={{ fontSize: 14 }}>dataset</MsIcon>
                <Box component="span">Docker Logs</Box>
                <Box component="span" sx={chipSx()}>{dockerLogs.length}</Box>
              </Box>
            )}
          </Box>

          {/* Quick Actions */}
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
              <Box
                component="span"
                sx={{
                  display: "inline-block",
                  width: 8,
                  height: 8,
                  borderRadius: "50%",
                  bgcolor: connected ? "success.main" : "error.main",
                  animation: connected ? "pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite" : "none",
                }}
              />
              <Box component="span" sx={{ fontSize: 12, color: "text.secondary" }}>{connected ? "Live Stream" : "Disconnected"}</Box>
            </Box>

            <Button size="sm" variant="outline" icon="sync" onClick={fetchSystemLogs} loading={loading}>
              Refresh
            </Button>
            <Button size="sm" variant="outline" icon="content_copy" onClick={handleCopyLogs}>
              Copy
            </Button>
            <Button size="sm" variant="outline" icon="delete" onClick={handleClearAppLogs}>
              Clear App Logs
            </Button>
          </Box>
        </Box>

        {/* Filter Bar */}
        <Box sx={{ mt: 2, pt: 1.5, borderTop: 1, borderColor: "divider", display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1.5, alignItems: { sm: "center" }, justifyContent: { sm: "space-between" } }}>
          {/* Level Filter Buttons */}
          <Box sx={{ display: "flex", alignItems: "center", gap: 0.75, flexWrap: "wrap" }}>
            <Box component="span" sx={{ fontSize: 12, fontWeight: 500, color: "text.secondary", mr: 0.5 }}>Filter Level:</Box>
            <Box component="button" onClick={() => setLevelFilter("ALL")} sx={levelButtonSx(levelFilter === "ALL", "all")}>
              ALL
            </Box>
            <Box component="button" onClick={() => setLevelFilter("ERROR")} sx={levelButtonSx(levelFilter === "ERROR", "error")}>
              <Box component="span">ERROR ONLY</Box>
              {totalErrorCount > 0 && (
                <Box component="span" sx={{ px: 0.75, py: "1px", borderRadius: 999, fontSize: 10, bgcolor: "error.main", color: "#fff", fontWeight: 700 }}>
                  {totalErrorCount}
                </Box>
              )}
            </Box>
            <Box component="button" onClick={() => setLevelFilter("WARN")} sx={levelButtonSx(levelFilter === "WARN", "warn")}>
              WARN
            </Box>
            <Box component="button" onClick={() => setLevelFilter("INFO")} sx={levelButtonSx(levelFilter === "INFO", "info")}>
              INFO
            </Box>
          </Box>

          {/* Search Input & Auto-scroll */}
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <Box sx={{ position: "relative", width: { xs: 192, sm: 256 } }}>
              <Input
                size="sm"
                placeholder="Search log text..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
              />
              {searchQuery && (
                <Box
                  component="button"
                  onClick={() => setSearchQuery("")}
                  sx={{
                    position: "absolute",
                    right: 8,
                    top: "50%",
                    transform: "translateY(-50%)",
                    color: "text.secondary",
                    fontSize: 12,
                    border: 0,
                    background: "none",
                    cursor: "pointer",
                    "&:hover": { color: "text.primary" },
                  }}
                >
                  ✕
                </Box>
              )}
            </Box>

            <Box
              component="button"
              onClick={() => setAutoScroll(!autoScroll)}
              sx={{
                px: 1.25,
                py: 0.5,
                fontSize: 12,
                borderRadius: 1,
                transition: "background-color 150ms ease, color 150ms ease",
                display: "inline-flex",
                alignItems: "center",
                gap: 0.5,
                cursor: "pointer",
                color: autoScroll ? "#4ade80" : "text.secondary",
                bgcolor: autoScroll ? "rgba(34,197,94,0.1)" : "action.hover",
                border: autoScroll ? "1px solid rgba(34,197,94,0.3)" : "1px solid",
                borderColor: autoScroll ? "rgba(34,197,94,0.3)" : "divider",
              }}
            >
              <MsIcon sx={{ fontSize: 14 }}>{autoScroll ? "vertical_align_bottom" : "pause"}</MsIcon>
              <Box component="span">{autoScroll ? "Auto-scroll" : "Paused"}</Box>
            </Box>
          </Box>
        </Box>
      </Card>

      {/* Main Console Output Terminal */}
      <Card sx={{ overflow: "hidden" }}>
        <Box
          ref={logRef}
          sx={{
            bgcolor: "rgba(0,0,0,0.95)",
            p: 2,
            fontFamily: "monospace",
            fontSize: 12,
            height: "calc(100vh - 280px)",
            overflowY: "auto",
            display: "flex",
            flexDirection: "column",
            gap: 0.5,
          }}
        >
          {combinedLogs.length === 0 ? (
            <Box sx={{ display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", height: "100%", color: "text.secondary", py: 6 }}>
              <MsIcon sx={{ fontSize: 36, mb: 1, color: "text.secondary", opacity: 0.5 }}>terminal</MsIcon>
              <Box component="p">No console or system logs match your filter criteria.</Box>
              {levelFilter !== "ALL" && (
                <Box
                  component="button"
                  onClick={() => setLevelFilter("ALL")}
                  sx={{ fontSize: 12, color: "primary.main", mt: 1, border: 0, background: "none", cursor: "pointer", "&:hover": { textDecoration: "underline" } }}
                >
                  Reset level filter
                </Box>
              )}
            </Box>
          ) : (
            combinedLogs.map((line, i) => renderFormattedLine(line, i))
          )}
        </Box>
      </Card>
    </Stack>
  );
}
