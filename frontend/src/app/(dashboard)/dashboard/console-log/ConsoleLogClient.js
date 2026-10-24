"use client";

import { useState, useEffect, useRef, useMemo } from "react";
import { Card, Button, Badge, Input } from "@/shared/components";
import { CONSOLE_LOG_CONFIG } from "@/shared/constants/config";

const LOG_LEVEL_COLORS = {
  LOG: "text-emerald-400",
  INFO: "text-blue-400",
  WARN: "text-yellow-400",
  ERROR: "text-red-400 font-medium",
  DEBUG: "text-purple-400",
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
  const colorClass = LOG_LEVEL_COLORS[level] || "text-gray-300";

  let bgClass = "";
  if (level === "ERROR") {
    bgClass = "bg-red-500/10 border-l-2 border-red-500 pl-2 py-0.5 my-0.5 rounded-r";
  } else if (level === "WARN") {
    bgClass = "bg-yellow-500/10 border-l-2 border-yellow-500 pl-2 py-0.5 my-0.5 rounded-r";
  }

  return (
    <div key={key} className={`font-mono text-xs leading-relaxed break-all ${bgClass} ${colorClass}`}>
      {line}
    </div>
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

  return (
    <div className="flex flex-col gap-4">
      {/* Header Controls */}
      <Card className="p-4">
        <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
          {/* Source Tabs */}
          <div className="flex items-center gap-2 overflow-x-auto pb-1 md:pb-0">
            <button
              onClick={() => setActiveTab("all")}
              className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-colors flex items-center gap-1.5 shrink-0 ${
                activeTab === "all"
                  ? "bg-primary text-white"
                  : "bg-surface-hover text-text-muted hover:text-text-primary"
              }`}
            >
              <span>All Logs</span>
              <span className="px-1.5 py-0.5 rounded-full text-[10px] bg-black/20">
                {appLogs.length + pm2Logs.length + dockerLogs.length}
              </span>
            </button>

            <button
              onClick={() => setActiveTab("app")}
              className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-colors flex items-center gap-1.5 shrink-0 ${
                activeTab === "app"
                  ? "bg-primary text-white"
                  : "bg-surface-hover text-text-muted hover:text-text-primary"
              }`}
            >
              <span className="material-symbols-outlined text-sm">terminal</span>
              <span>App Console</span>
              <span className="px-1.5 py-0.5 rounded-full text-[10px] bg-black/20">{appLogs.length}</span>
            </button>

            <button
              onClick={() => setActiveTab("pm2")}
              className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-colors flex items-center gap-1.5 shrink-0 ${
                activeTab === "pm2"
                  ? "bg-primary text-white"
                  : "bg-surface-hover text-text-muted hover:text-text-primary"
              }`}
            >
              <span className="material-symbols-outlined text-sm">dns</span>
              <span>PM2 Server Logs</span>
              {pm2Info.errorCount > 0 && (
                <span className="px-1.5 py-0.5 rounded-full text-[10px] bg-red-500 text-white font-bold">
                  {pm2Info.errorCount} Err
                </span>
              )}
            </button>

            {(dockerInfo.available || dockerLogs.length > 0) && (
              <button
                onClick={() => setActiveTab("docker")}
                className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-colors flex items-center gap-1.5 shrink-0 ${
                  activeTab === "docker"
                    ? "bg-primary text-white"
                    : "bg-surface-hover text-text-muted hover:text-text-primary"
                }`}
              >
                <span className="material-symbols-outlined text-sm">dataset</span>
                <span>Docker Logs</span>
                <span className="px-1.5 py-0.5 rounded-full text-[10px] bg-black/20">{dockerLogs.length}</span>
              </button>
            )}
          </div>

          {/* Quick Actions */}
          <div className="flex items-center gap-2">
            <div className="flex items-center gap-1">
              <span
                className={`inline-block size-2 rounded-full ${
                  connected ? "bg-green-500 animate-pulse" : "bg-red-500"
                }`}
              />
              <span className="text-xs text-text-muted">{connected ? "Live Stream" : "Disconnected"}</span>
            </div>

            <Button size="sm" variant="outline" icon="sync" onClick={fetchSystemLogs} loading={loading}>
              Refresh
            </Button>
            <Button size="sm" variant="outline" icon="content_copy" onClick={handleCopyLogs}>
              Copy
            </Button>
            <Button size="sm" variant="outline" icon="delete" onClick={handleClearAppLogs}>
              Clear App Logs
            </Button>
          </div>
        </div>

        {/* Filter Bar */}
        <div className="mt-4 pt-3 border-t border-border-primary flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          {/* Level Filter Buttons */}
          <div className="flex items-center gap-1.5 flex-wrap">
            <span className="text-xs font-medium text-text-muted mr-1">Filter Level:</span>
            <button
              onClick={() => setLevelFilter("ALL")}
              className={`px-2.5 py-1 text-xs rounded-md transition-colors ${
                levelFilter === "ALL"
                  ? "bg-primary/20 text-primary font-bold border border-primary/40"
                  : "bg-surface-hover text-text-muted hover:text-text-primary"
              }`}
            >
              ALL
            </button>
            <button
              onClick={() => setLevelFilter("ERROR")}
              className={`px-2.5 py-1 text-xs rounded-md transition-colors flex items-center gap-1 ${
                levelFilter === "ERROR"
                  ? "bg-red-500/20 text-red-400 font-bold border border-red-500/50"
                  : "bg-surface-hover text-text-muted hover:text-red-400"
              }`}
            >
              <span>ERROR ONLY</span>
              {totalErrorCount > 0 && (
                <span className="px-1.5 py-0.2 rounded-full text-[10px] bg-red-500 text-white font-bold">
                  {totalErrorCount}
                </span>
              )}
            </button>
            <button
              onClick={() => setLevelFilter("WARN")}
              className={`px-2.5 py-1 text-xs rounded-md transition-colors ${
                levelFilter === "WARN"
                  ? "bg-yellow-500/20 text-yellow-400 font-bold border border-yellow-500/50"
                  : "bg-surface-hover text-text-muted hover:text-yellow-400"
              }`}
            >
              WARN
            </button>
            <button
              onClick={() => setLevelFilter("INFO")}
              className={`px-2.5 py-1 text-xs rounded-md transition-colors ${
                levelFilter === "INFO"
                  ? "bg-blue-500/20 text-blue-400 font-bold border border-blue-500/50"
                  : "bg-surface-hover text-text-muted hover:text-blue-400"
              }`}
            >
              INFO
            </button>
          </div>

          {/* Search Input & Auto-scroll */}
          <div className="flex items-center gap-2">
            <div className="relative w-48 sm:w-64">
              <Input
                size="sm"
                placeholder="Search log text..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="w-full text-xs"
              />
              {searchQuery && (
                <button
                  onClick={() => setSearchQuery("")}
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-text-muted hover:text-text-primary text-xs"
                >
                  ✕
                </button>
              )}
            </div>

            <button
              onClick={() => setAutoScroll(!autoScroll)}
              className={`px-2.5 py-1 text-xs rounded-md border transition-colors flex items-center gap-1 ${
                autoScroll
                  ? "bg-green-500/10 text-green-400 border-green-500/30"
                  : "bg-surface-hover text-text-muted border-border-primary"
              }`}
            >
              <span className="material-symbols-outlined text-sm">{autoScroll ? "vertical_align_bottom" : "pause"}</span>
              <span>{autoScroll ? "Auto-scroll" : "Paused"}</span>
            </button>
          </div>
        </div>
      </Card>

      {/* Main Console Output Terminal */}
      <Card className="overflow-hidden">
        <div
          ref={logRef}
          className="bg-black/95 p-4 font-mono text-xs h-[calc(100vh-280px)] overflow-y-auto space-y-1 scrollbar-thin"
        >
          {combinedLogs.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full text-text-muted py-12">
              <span className="material-symbols-outlined text-4xl mb-2 text-text-muted/50">terminal</span>
              <p>No console or system logs match your filter criteria.</p>
              {levelFilter !== "ALL" && (
                <button
                  onClick={() => setLevelFilter("ALL")}
                  className="text-xs text-primary hover:underline mt-2"
                >
                  Reset level filter
                </button>
              )}
            </div>
          ) : (
            combinedLogs.map((line, i) => renderFormattedLine(line, i))
          )}
        </div>
      </Card>
    </div>
  );
}
