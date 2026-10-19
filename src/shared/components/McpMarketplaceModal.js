"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import Image from "next/image";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Modal from "./Modal";

const REGISTRY_ENDPOINT = "/api/cli-tools/cowork-mcp-registry";
const TOOLS_ENDPOINT = "/api/cli-tools/cowork-mcp-tools";
const EMPTY_ADDED_NAMES = [];

export default function McpMarketplaceModal({ isOpen, onClose, onAdd, addedNames = EMPTY_ADDED_NAMES }) {
  const [servers, setServers] = useState([]);
  const [loadState, setLoadState] = useState({ loading: false, error: null });
  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState("all");
  const [expandedUrl, setExpandedUrl] = useState(null);
  const [toolsCache, setToolsCache] = useState({});
  const [toolsLoading, setToolsLoading] = useState({});
  const [toolSelection, setToolSelection] = useState({});
  const hasFetchedServers = useRef(false);

  useEffect(() => {
    if (!isOpen || hasFetchedServers.current) return;
    hasFetchedServers.current = true;
    const controller = new AbortController();
    setLoadState({ loading: true, error: null });
    fetch(REGISTRY_ENDPOINT, { signal: controller.signal })
      .then((r) => r.json())
      .then((d) => {
        if (controller.signal.aborted) return;
        if (d.error) setLoadState({ loading: false, error: d.error });
        else { setServers(d.servers || []); setLoadState({ loading: false, error: null }); }
      })
      .catch((e) => {
        if (!controller.signal.aborted) setLoadState({ loading: false, error: e.message });
      });
    return () => controller.abort();
  }, [isOpen]);

  const addedSet = useMemo(() => new Set(addedNames), [addedNames]);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return servers.filter((s) => {
      if (filter === "authless" && s.oauth) return false;
      if (filter === "oauth" && !s.oauth) return false;
      if (!q) return true;
      return (
        (s.title || "").toLowerCase().includes(q) ||
        (s.description || "").toLowerCase().includes(q) ||
        (s.name || "").toLowerCase().includes(q)
      );
    });
  }, [servers, search, filter]);

  const fetchTools = async (server) => {
    if (toolsCache[server.url]) return;
    setToolsLoading((p) => ({ ...p, [server.url]: true }));
    try {
      const r = await fetch(TOOLS_ENDPOINT, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ url: server.url }),
      });
      const d = await r.json();
      const tools = d.tools || [];
      const fallback = Array.isArray(server.toolNames) ? server.toolNames : [];
      const toolNames = tools.length > 0 ? tools.map((t) => t.name) : fallback;
      setToolsCache((p) => ({ ...p, [server.url]: { tools, requiresAuth: !!d.requiresAuth, error: d.error } }));
      // Default: all checked
      setToolSelection((p) => ({ ...p, [server.url]: Object.fromEntries(toolNames.map((t) => [t, true])) }));
    } catch (e) {
      setToolsCache((p) => ({ ...p, [server.url]: { tools: [], error: e.message } }));
    } finally {
      setToolsLoading((p) => ({ ...p, [server.url]: false }));
    }
  };

  const expandServer = (server) => {
    if (expandedUrl === server.url) {
      setExpandedUrl(null);
      return;
    }
    setExpandedUrl(server.url);
    fetchTools(server);
  };

  const toggleTool = (url, tool) => {
    setToolSelection((prev) => ({ ...prev, [url]: { ...prev[url], [tool]: !prev[url]?.[tool] } }));
  };

  const setAllTools = (url, value) => {
    const sel = toolSelection[url] || {};
    setToolSelection((prev) => ({ ...prev, [url]: Object.fromEntries(Object.keys(sel).map((t) => [t, value])) }));
  };

  const confirmAdd = (server) => {
    const sel = toolSelection[server.url] || {};
    const enabled = Object.keys(sel).filter((t) => sel[t]);
    onAdd?.({
      name: server.slug || server.name,
      title: server.title,
      description: server.description,
      url: server.url,
      transport: server.transport,
      oauth: server.oauth,
      toolNames: enabled,
    });
    setExpandedUrl(null);
  };

  const { loading, error } = loadState;

  const inputSx = {
    px: 1,
    py: 0.75,
    bgcolor: "var(--color-surface)",
    borderRadius: "4px",
    fontSize: 12,
    border: "1px solid var(--color-border)",
    color: "text.primary",
    outline: "none",
    "&:focus": { boxShadow: "0 0 0 1px rgba(229,106,74,0.5)" },
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Browse MCP Marketplace" size="lg">
      <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Box
            component="input"
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search by name or description..."
            aria-label="Search MCP servers"
            sx={{ ...inputSx, flex: 1 }}
          />
          <Box
            component="select"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            aria-label="Filter MCP servers"
            sx={inputSx}
          >
            <option value="all">All</option>
            <option value="authless">Authless</option>
            <option value="oauth">OAuth</option>
          </Box>
        </Box>

        {error && (
          <Box sx={{ px: 1, py: 0.75, borderRadius: "4px", fontSize: 12, bgcolor: "rgba(239,68,68,0.1)", color: "#dc2626" }}>{error}</Box>
        )}

        {loading && (
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, color: "text.secondary", fontSize: 12, py: 2, justifyContent: "center" }}>
            <span className="material-symbols-outlined" style={{ animation: "spin 1s linear infinite",  fontSize: 18 }}>progress_activity</span>
            <span>Loading registry...</span>
          </Box>
        )}

        {!loading && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5, maxHeight: "60vh", overflowY: "auto" }}>
            {filtered.length === 0 && (
              <Box sx={{ textAlign: "center", fontSize: 12, color: "text.secondary", py: 3 }}>No servers match filter</Box>
            )}
            {filtered.map((s) => {
              const added = addedSet.has(s.slug || s.name);
              const expanded = expandedUrl === s.url;
              const cache = toolsCache[s.url];
              const isLoadingTools = toolsLoading[s.url];
              const sel = toolSelection[s.url] || {};
              const toolKeys = Object.keys(sel);
              const selectedCount = Object.values(sel).filter(Boolean).length;
              return (
                <Box key={s.url} sx={{ borderRadius: "4px", border: "1px solid transparent", "&:hover": { borderColor: "var(--color-border)" } }}>
                  <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1, px: 1, py: 1, "&:hover": { bgcolor: "var(--color-surface-2)" } }}>
                    {s.iconUrl ? (
                      <Image src={s.iconUrl} alt="" style={{ width: 28, height: 28, borderRadius: 4, flexShrink: 0, objectFit: "contain" }} width={28} height={28} onError={(e) => { e.target.style.display = "none"; }} unoptimized />
                    ) : (
                      <Box sx={{ width: 28, height: 28, borderRadius: "4px", bgcolor: "var(--color-surface)", flexShrink: 0 }} />
                    )}
                    <Box sx={{ flex: 1, minWidth: 0 }}>
                      <Box sx={{ display: "flex", alignItems: "center", gap: 0.75, flexWrap: "wrap" }}>
                        <Box component="span" sx={{ fontWeight: 500, fontSize: 12 }}>{s.title}</Box>
                        {s.oauth ? (
                          <Box component="span" sx={{ px: 0.5, py: 0.25, fontSize: 9, borderRadius: "4px", bgcolor: "rgba(245,158,11,0.1)", color: "#d97706" }}>OAuth</Box>
                        ) : (
                          <Box component="span" sx={{ px: 0.5, py: 0.25, fontSize: 9, borderRadius: "4px", bgcolor: "rgba(34,197,94,0.1)", color: "#16a34a" }}>Authless</Box>
                        )}
                        {s.toolCount > 0 && (
                          <Box component="span" sx={{ fontSize: 10, color: "text.secondary" }}>{s.toolCount} tools</Box>
                        )}
                      </Box>
                      {s.description && (
                        <Typography sx={{ fontSize: 10, color: "text.secondary", mt: 0.25, display: "-webkit-box", WebkitLineClamp: 2, WebkitBoxOrient: "vertical", overflow: "hidden" }}>{s.description}</Typography>
                      )}
                    </Box>
                    <Box
                      component="button"
                      type="button"
                      onClick={() => added ? null : expandServer(s)}
                      disabled={added}
                      sx={{
                        flexShrink: 0,
                        px: 1,
                        py: 0.5,
                        borderRadius: "4px",
                        fontSize: 10,
                        fontWeight: 500,
                        cursor: added ? "default" : "pointer",
                        transition: "all 150ms",
                        border: added ? "1px solid transparent" : expanded ? "1px solid var(--color-border)" : "1px solid rgba(229,106,74,0.4)",
                        bgcolor: added ? "rgba(34,197,94,0.1)" : expanded ? "var(--color-surface)" : "rgba(229,106,74,0.1)",
                        color: added ? "#16a34a" : expanded ? "text.secondary" : "primary.main",
                        "&:hover": added ? {} : { bgcolor: expanded ? "var(--color-surface-2)" : "rgba(229,106,74,0.2)" },
                      }}
                    >
                      {added ? "Added" : expanded ? "Cancel" : "+ Add"}
                    </Box>
                  </Box>
                  {expanded && (
                    <Box sx={{ px: 1.5, py: 1, bgcolor: "rgba(255,255,255,0.4)", borderTop: "1px solid var(--color-border)", display: "flex", flexDirection: "column", gap: 1 }}>
                      {isLoadingTools && (
                        <Box sx={{ display: "flex", alignItems: "center", gap: 1, color: "text.secondary", fontSize: 10, py: 0.5 }}>
                          <span className="material-symbols-outlined" style={{ animation: "spin 1s linear infinite",  fontSize: 14 }}>progress_activity</span>
                          <span>Probing server for tools...</span>
                        </Box>
                      )}
                      {!isLoadingTools && cache?.requiresAuth && (
                        <Box component="p" sx={{ fontSize: 10, color: "#d97706", bgcolor: "rgba(245,158,11,0.1)", px: 1, py: 0.5, borderRadius: "4px", m: 0 }}>
                          🔐 OAuth required. Add now and authenticate after Apply; tool list will be discovered after first connect.
                        </Box>
                      )}
                      {!isLoadingTools && cache?.error && !cache?.requiresAuth && (
                        <Box component="p" sx={{ fontSize: 10, color: "#dc2626", bgcolor: "rgba(239,68,68,0.1)", px: 1, py: 0.5, borderRadius: "4px", m: 0 }}>Probe failed: {cache.error}</Box>
                      )}
                      {!isLoadingTools && toolKeys.length === 0 && !cache?.requiresAuth && !cache?.error && (
                        <Box component="p" sx={{ fontSize: 10, color: "text.secondary", m: 0 }}>No tools advertised by server.</Box>
                      )}
                      {!isLoadingTools && toolKeys.length > 0 && (
                        <>
                          <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                            <Box component="span" sx={{ fontSize: 10, color: "text.secondary" }}>{selectedCount}/{toolKeys.length} tools enabled</Box>
                            <Box sx={{ display: "flex", gap: 0.5 }}>
                              <Box component="button" type="button" onClick={() => setAllTools(s.url, true)} sx={{ fontSize: 10, color: "primary.main", border: 0, bgcolor: "transparent", cursor: "pointer", "&:hover": { textDecoration: "underline" } }}>All</Box>
                              <Box component="span" sx={{ fontSize: 10, color: "text.secondary" }}>·</Box>
                              <Box component="button" type="button" onClick={() => setAllTools(s.url, false)} sx={{ fontSize: 10, color: "primary.main", border: 0, bgcolor: "transparent", cursor: "pointer", "&:hover": { textDecoration: "underline" } }}>None</Box>
                            </Box>
                          </Box>
                          <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 0.5, maxHeight: 160, overflowY: "auto" }}>
                            {toolKeys.map((t) => (
                              <Box component="label" key={t} sx={{ display: "flex", alignItems: "center", gap: 0.75, fontSize: 10, cursor: "pointer", px: 0.5, borderRadius: "4px", "&:hover": { bgcolor: "var(--color-surface-2)" } }}>
                                <input type="checkbox" checked={!!sel[t]} onChange={() => toggleTool(s.url, t)} style={{ width: 12, height: 12 }} />
                                <Box component="span" sx={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{t}</Box>
                              </Box>
                            ))}
                          </Box>
                        </>
                      )}
                      <Box
                        component="button"
                        type="button"
                        onClick={() => confirmAdd(s)}
                        sx={{ alignSelf: "flex-end", px: 1, py: 0.5, borderRadius: "4px", fontSize: 10, fontWeight: 500, bgcolor: "primary.main", color: "#fff", border: 0, cursor: "pointer", "&:hover": { bgcolor: "primary.dark" } }}
                      >
                        ✓ Confirm Add
                      </Box>
                    </Box>
                  )}
                </Box>
              );
            })}
          </Box>
        )}

        <Box sx={{ fontSize: 10, color: "text.secondary", textAlign: "right" }}>
          {filtered.length} of {servers.length} servers
        </Box>
      </Box>
    </Modal>
  );
}
