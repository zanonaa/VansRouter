"use client";

import { useState, useEffect } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import TextField from "@mui/material/TextField";
import MenuItem from "@mui/material/MenuItem";
import Table from "@mui/material/Table";
import TableHead from "@mui/material/TableHead";
import TableBody from "@mui/material/TableBody";
import TableRow from "@mui/material/TableRow";
import TableCell from "@mui/material/TableCell";
import TableContainer from "@mui/material/TableContainer";
import Card from "@/shared/components/Card";
import Button from "@/shared/components/Button";
import Drawer from "@/shared/components/Drawer";
import Pagination from "@/shared/components/Pagination";
import { AI_PROVIDERS, getProviderByAlias } from "@/shared/constants/providers";

let providerNameCache = null;
let providerNodesCache = null;

async function fetchProviderNames() {
  if (providerNameCache && providerNodesCache) {
    return { providerNameCache, providerNodesCache };
  }

  const nodesRes = await fetch("/api/provider-nodes");
  const nodesData = await nodesRes.json();
  const nodes = nodesData.nodes || [];
  providerNodesCache = {};

  for (const node of nodes) {
    providerNodesCache[node.id] = node.name;
  }

  providerNameCache = {
    ...AI_PROVIDERS,
    ...providerNodesCache
  };

  return { providerNameCache, providerNodesCache };
}

function getProviderName(providerId, cache) {
  if (!providerId) return providerId;
  if (!cache) return providerId;

  const cached = cache[providerId];

  if (typeof cached === 'string') {
    return cached;
  }

  if (cached?.name) {
    return cached.name;
  }

  const providerConfig = getProviderByAlias(providerId) || AI_PROVIDERS[providerId];
  return providerConfig?.name || providerId;
}

function CollapsibleSection({ title, children, defaultOpen = false, icon = null }) {
  const [isOpen, setIsOpen] = useState(defaultOpen);

  return (
    <Box sx={{ border: 1, borderColor: "divider", borderRadius: "8px", overflow: "hidden" }}>
      <Box
        component="button"
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        sx={{
          width: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          p: 1.5,
          bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(255,255,255,0.02)" : "rgba(0,0,0,0.02)"),
          border: 0,
          cursor: "pointer",
          transition: "background-color 150ms ease",
          "&:hover": { bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(255,255,255,0.04)" : "rgba(0,0,0,0.04)") },
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          {icon && <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18, color: "text.secondary" }}>{icon}</Icon>}
          <Typography component="span" sx={{ fontWeight: 600, fontSize: 14, color: "text.primary" }}>{title}</Typography>
        </Box>
        <Icon
          className="material-symbols-outlined"
          baseClassName=""
          sx={{
            fontSize: 20,
            color: "text.secondary",
            transition: "transform 200ms ease",
            transform: isOpen ? "rotate(90deg)" : "none",
          }}
        >
          chevron_right
        </Icon>
      </Box>

      {isOpen && (
        <Box sx={{ p: 2, borderTop: 1, borderColor: "divider" }}>
          {children}
        </Box>
      )}
    </Box>
  );
}

function getInputTokens(tokens) {
  if (!tokens) return 0;
  if (tokens.prompt_tokens !== undefined) {
    const prompt = Number(tokens.prompt_tokens) || 0;
    const cache = Number(tokens.cached_tokens || tokens.cache_read_input_tokens) || 0;
    return prompt < cache ? prompt + cache : prompt;
  }
  const input = Number(tokens.input_tokens) || 0;
  const cache = Number(tokens.cache_read_input_tokens || tokens.cached_tokens) || 0;
  return input + cache;
}

function maskKey(fullKey) {
  if (!fullKey) return "";
  return fullKey.length > 8 ? `${fullKey.slice(0, 8)}...` : fullKey;
}

// truncateField (requestDetailsRepo.js) replaces an over-cap field with this marker.
function isTruncated(field) {
  return field && typeof field === "object" && field._truncated === true;
}

function TruncatedNotice({ marker }) {
  return (
    <Box
      sx={{
        borderRadius: "8px",
        border: 1,
        borderColor: "rgba(245,158,11,0.3)",
        bgcolor: "rgba(245,158,11,0.1)",
        p: 1.5,
        fontFamily: "monospace",
        fontSize: 12,
        color: (t) => (t.palette.mode === "dark" ? "#fcd34d" : "#b45309"),
      }}
    >
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.75, fontFamily: "inherit", fontWeight: 600 }}>
        <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 16 }}>content_cut</Icon>
        Payload truncated
      </Box>
      <Box sx={{ mt: 0.5 }}>
        Original size: {((marker._originalSize || 0) / 1024).toFixed(1)} KB
      </Box>
      {marker._preview && (
        <Box component="pre" sx={{ mt: 1, maxHeight: 120, maxWidth: "100%", overflow: "auto", whiteSpace: "pre-wrap", wordBreak: "break-all", opacity: 0.8, m: 0 }}>
          {marker._preview}
        </Box>
      )}
    </Box>
  );
}

function RequestFilters({ filterProvider, setFilterProvider, filterStart, setFilterStart, filterEnd, setFilterEnd, providers, handleApplyFilters, handleClearFilters }) {
  return (
      <Card padding="md">
        <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "repeat(2, minmax(0,1fr))", lg: "repeat(4, minmax(0,1fr))" }, gap: 2 }}>
          <TextField
            id="provider-filter"
            select
            size="small"
            label="Provider"
            value={filterProvider}
            onChange={(e) => setFilterProvider(e.target.value)}
            fullWidth
          >
            <MenuItem value="">All Providers</MenuItem>
            {providers.map((provider) => (
              <MenuItem key={provider.id} value={provider.id}>{provider.name}</MenuItem>
            ))}
          </TextField>
          <TextField
            id="start-date-filter"
            type="datetime-local"
            size="small"
            label="Start Date"
            value={filterStart}
            onChange={(e) => setFilterStart(e.target.value)}
            fullWidth
            slotProps={{ inputLabel: { shrink: true } }}
          />
          <TextField
            id="end-date-filter"
            type="datetime-local"
            size="small"
            label="End Date"
            value={filterEnd}
            onChange={(e) => setFilterEnd(e.target.value)}
            fullWidth
            slotProps={{ inputLabel: { shrink: true } }}
          />
          <Box sx={{ display: "flex", minWidth: 0, flexDirection: "column", gap: 1, gridColumn: { xs: "span 2", lg: "span 1" } }}>
            <Typography sx={{ display: { xs: "none", lg: "block" }, fontSize: 14, fontWeight: 500, color: "text.primary", opacity: 0 }} aria-hidden="true">Actions</Typography>
            <Box sx={{ display: "flex", gap: 1, "& > *": { flex: 1 } }}>
              <Button onClick={handleApplyFilters}>Search</Button>
              <Button variant="ghost" onClick={handleClearFilters}
                disabled={!filterProvider && !filterStart && !filterEnd}>
                Clear
              </Button>
            </Box>
          </Box>
        </Box>
      </Card>
  );
}


function RequestRow({ detail, index, handleViewDetail, providerNameCache }) {
  return (
                  <TableRow
                    key={`${detail.id}-${index}`}
                    sx={{ transition: "background-color 150ms ease", "&:hover": { bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(255,255,255,0.02)" : "rgba(0,0,0,0.02)") } }}
                  >
                    <TableCell sx={{ whiteSpace: "nowrap", p: 2, fontSize: 14, color: "text.primary" }}>
                      {new Date(detail.timestamp).toLocaleString()}
                    </TableCell>
                    <TableCell sx={{ maxWidth: 260, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", p: 2, fontFamily: "monospace", fontSize: 14, color: "text.primary" }}>
                      {detail.model}
                    </TableCell>
                    <TableCell sx={{ maxWidth: 180, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", p: 2, fontSize: 14, color: "text.primary" }}>
                       <Box component="span" sx={{ fontWeight: 500 }}>
                         {getProviderName(detail.provider, providerNameCache)}
                       </Box>
                     </TableCell>
                    <TableCell sx={{ p: 2, fontSize: 14, color: "text.primary", textAlign: "right", fontFamily: "monospace" }}>
                      {getInputTokens(detail.tokens).toLocaleString()}
                    </TableCell>
                    <TableCell sx={{ p: 2, fontSize: 14, color: "text.primary", textAlign: "right", fontFamily: "monospace" }}>
                      {detail.tokens?.completion_tokens?.toLocaleString() || 0}
                    </TableCell>
                    <TableCell sx={{ p: 2, fontSize: 14, color: "text.secondary" }}>
                      <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25 }}>
                        <Box>TTFT: <Box component="span" sx={{ fontFamily: "monospace" }}>{detail.latency?.ttft || 0}ms</Box></Box>
                        <Box>Total: <Box component="span" sx={{ fontFamily: "monospace" }}>{detail.latency?.total || 0}ms</Box></Box>
                      </Box>
                    </TableCell>
                    <TableCell sx={{ p: 2, textAlign: "center" }}>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => handleViewDetail(detail)}
                      >
                        Detail
                      </Button>
                    </TableCell>
                  </TableRow>
  );
}


export default function RequestDetailsTab() {
  const [details, setDetails] = useState([]);
  const [totalItems, setTotalItems] = useState(0);
  const [totalPages, setTotalPages] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [loading, setLoading] = useState(false);
  const [selectedDetail, setSelectedDetail] = useState(null);
  const [isDrawerOpen, setIsDrawerOpen] = useState(false);
  const [providers, setProviders] = useState([]);
  const [providerNameCache, setProviderNameCache] = useState(null);
  // Filter input state (not applied until Search clicked)
  const [filterProvider, setFilterProvider] = useState("");
  const [filterStart, setFilterStart] = useState(() => {
    if (typeof window === "undefined") return "";
    const d = new Date();
    d.setDate(d.getDate() - 7);
    return d.toISOString().slice(0, 16);
  });
  const [filterEnd, setFilterEnd] = useState("");
  // Applied filter state (triggers fetch)
  const [appliedProvider, setAppliedProvider] = useState("");
  const [appliedStart, setAppliedStart] = useState(() => {
    if (typeof window === "undefined") return "";
    const d = new Date();
    d.setDate(d.getDate() - 7);
    return d.toISOString().slice(0, 16);
  });
  const [appliedEnd, setAppliedEnd] = useState("");
  // Guard: don't fetch until default filter is ready
  const [isFilterReady, setIsFilterReady] = useState(true);

  // Fetch providers once
  useEffect(() => {
    const controller = new AbortController();
    fetch("/api/usage/providers", { signal: controller.signal })
      .then(r => r.json())
      .then(async d => {
        if (controller.signal.aborted) return;
        setProviders(d.providers || []);
        const cache = await fetchProviderNames();
        if (!controller.signal.aborted) setProviderNameCache(cache.providerNameCache);
      })
      .catch(() => {});
    return () => controller.abort();
  }, []);

  // Fetch details — guarded by isFilterReady to prevent unfiltered initial request
  useEffect(() => {
    if (!isFilterReady) return;
    if (!appliedStart) return;

    const controller = new AbortController();
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch with abort controller; setLoading(true) is intentional at the start of the request.
    setLoading(true);
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    if (appliedProvider) params.append("provider", appliedProvider);
    params.append("startDate", appliedStart);
    if (appliedEnd) params.append("endDate", appliedEnd);

    fetch(`/api/usage/request-details?${params}`, { signal: controller.signal })
      .then(r => {
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return r.json();
      })
      .then(data => {
        if (controller.signal.aborted) return;
        setDetails(Array.isArray(data.details) ? data.details : []);
        setTotalItems(data.pagination?.totalItems ?? 0);
        setTotalPages(data.pagination?.totalPages ?? 0);
      })
      .catch(err => {
        if (!controller.signal.aborted) console.error("request-details fetch failed:", err);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [isFilterReady, page, pageSize, appliedProvider, appliedStart, appliedEnd]);

  const handleApplyFilters = () => {
    setPage(1);
    setAppliedProvider(filterProvider);
    setAppliedStart(filterStart);
    setAppliedEnd(filterEnd);
  };

  const handleClearFilters = () => {
    const weekAgo = (() => { const d = new Date(); d.setDate(d.getDate() - 7); return d.toISOString().slice(0, 16); })();
    setFilterProvider(""); setFilterStart(weekAgo); setFilterEnd("");
    setPage(1);
    setAppliedProvider(""); setAppliedStart(weekAgo); setAppliedEnd("");
  };

  const handlePageChange = (newPage) => setPage(newPage);
  const handlePageSizeChange = (newSize) => { setPageSize(newSize); setPage(1); };
  const handleViewDetail = (detail) => {
    setSelectedDetail(detail);
    setIsDrawerOpen(true);
    // Fetch full detail (list strips heavy request/response bodies)
    fetch(`/api/usage/request-details/${encodeURIComponent(detail.id)}`)
      .then(r => r.ok ? r.json() : null)
      .then(data => { if (data?.detail) setSelectedDetail(data.detail); })
      .catch(() => {});
  };

  const pagination = { page, pageSize, totalItems, totalPages, hasNext: page < totalPages, hasPrev: page > 1 };

  return (
    <Box sx={{ display: "flex", minWidth: 0, flexDirection: "column", gap: 3 }}>
      <RequestFilters filterProvider={filterProvider} setFilterProvider={setFilterProvider} filterStart={filterStart} setFilterStart={setFilterStart} filterEnd={filterEnd} setFilterEnd={setFilterEnd} providers={providers} handleApplyFilters={handleApplyFilters} handleClearFilters={handleClearFilters} />

      <Card padding="none">
        <TableContainer sx={{ overflowX: "auto" }}>
          <Table sx={{ width: "100%", minWidth: 880 }}>
            <TableHead>
              <TableRow sx={{ borderBottom: 1, borderColor: "divider" }}>
                <TableCell sx={{ textAlign: "left", p: 2, fontSize: 14, fontWeight: 600, color: "text.primary" }}>Timestamp</TableCell>
                <TableCell sx={{ textAlign: "left", p: 2, fontSize: 14, fontWeight: 600, color: "text.primary" }}>Model</TableCell>
                <TableCell sx={{ textAlign: "left", p: 2, fontSize: 14, fontWeight: 600, color: "text.primary" }}>Provider</TableCell>
                <TableCell sx={{ textAlign: "right", p: 2, fontSize: 14, fontWeight: 600, color: "text.primary" }}>Input Tokens</TableCell>
                <TableCell sx={{ textAlign: "right", p: 2, fontSize: 14, fontWeight: 600, color: "text.primary" }}>Output Tokens</TableCell>
                <TableCell sx={{ textAlign: "left", p: 2, fontSize: 14, fontWeight: 600, color: "text.primary" }}>Latency</TableCell>
                <TableCell sx={{ textAlign: "center", p: 2, fontSize: 14, fontWeight: 600, color: "text.primary" }}>Action</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {loading || !isFilterReady ? (
                <TableRow>
                  <TableCell colSpan="7" sx={{ p: 4, textAlign: "center", color: "text.secondary" }}>
                    <Box sx={{ display: "flex", alignItems: "center", justifyContent: "center", gap: 1 }}>
                      <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 20, animation: "spin 1s linear infinite" }}>progress_activity</Icon>
                      Loading...
                    </Box>
                  </TableCell>
                </TableRow>
              ) : details.length === 0 ? (
                <TableRow>
                  <TableCell colSpan="7" sx={{ p: 4, textAlign: "center", color: "text.secondary" }}>
                    No request details found
                  </TableCell>
                </TableRow>
              ) : (
                details.map((detail, index) => (
                  <RequestRow key={index} detail={detail} index={index} handleViewDetail={handleViewDetail} providerNameCache={providerNameCache} />
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>

        {!loading && details.length > 0 && (
          <Box sx={{ borderTop: 1, borderColor: "divider" }}>
            <Pagination
              currentPage={pagination.page}
              pageSize={pagination.pageSize}
              totalItems={pagination.totalItems}
              onPageChange={handlePageChange}
              onPageSizeChange={handlePageSizeChange}
            />
          </Box>
        )}
      </Card>

      <Drawer
        isOpen={isDrawerOpen}
        onClose={() => setIsDrawerOpen(false)}
        title="Request Details"
        width="lg"
      >
        {selectedDetail && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
            <Box sx={{ display: "grid", minWidth: 0, gridTemplateColumns: { xs: "1fr", sm: "repeat(2, minmax(0,1fr))" }, gap: 2, fontSize: 14 }}>
              <Box>
                <Box component="span" sx={{ color: "text.secondary" }}>ID:</Box>{" "}
                <Box component="span" sx={{ wordBreak: "break-all", fontFamily: "monospace", color: "text.primary" }}>{selectedDetail.id}</Box>
              </Box>
              <Box>
                <Box component="span" sx={{ color: "text.secondary" }}>Timestamp:</Box>{" "}
                <Box component="span" sx={{ color: "text.primary" }}>{new Date(selectedDetail.timestamp).toLocaleString()}</Box>
              </Box>
              <Box>
                 <Box component="span" sx={{ color: "text.secondary" }}>Provider:</Box>{" "}
                 <Box component="span" sx={{ color: "text.primary", fontWeight: 500 }}>{getProviderName(selectedDetail.provider, providerNameCache)}</Box>
               </Box>
              <Box>
                <Box component="span" sx={{ color: "text.secondary" }}>Model:</Box>{" "}
                <Box component="span" sx={{ color: "text.primary", fontFamily: "monospace" }}>{selectedDetail.model}</Box>
              </Box>
              <Box>
                <Box component="span" sx={{ color: "text.secondary" }}>API Key Name:</Box>{" "}
                <Box component="span" sx={{ color: "text.primary" }}>
                  {selectedDetail.apiKeyName || "-"}
                </Box>
              </Box>
              <Box>
                <Box component="span" sx={{ color: "text.secondary" }}>API Key:</Box>{" "}
                <Box component="span" sx={{ color: "text.primary", fontFamily: "monospace" }}>
                  {selectedDetail.apiKeyName && selectedDetail.apiKey ? maskKey(selectedDetail.apiKey) : "-"}
                </Box>
              </Box>
              <Box>
                <Box component="span" sx={{ color: "text.secondary" }}>Status:</Box>{" "}
                <Box component="span" sx={{ fontWeight: 500, color: selectedDetail.status === "success" ? "success.main" : "error.main" }}>
                  {selectedDetail.status}
                </Box>
              </Box>
              <Box>
                <Box component="span" sx={{ color: "text.secondary" }}>Latency:</Box>{" "}
                <Box component="span" sx={{ color: "text.primary", fontFamily: "monospace" }}>
                  TTFT {selectedDetail.latency?.ttft || 0}ms / Total {selectedDetail.latency?.total || 0}ms
                </Box>
              </Box>
              <Box>
                <Box component="span" sx={{ color: "text.secondary" }}>Input Tokens:</Box>{" "}
                <Box component="span" sx={{ color: "text.primary", fontFamily: "monospace" }}>
                  {getInputTokens(selectedDetail.tokens).toLocaleString()}
                </Box>
              </Box>
              <Box>
                <Box component="span" sx={{ color: "text.secondary" }}>Output Tokens:</Box>{" "}
                <Box component="span" sx={{ color: "text.primary", fontFamily: "monospace" }}>
                  {selectedDetail.tokens?.completion_tokens?.toLocaleString() || 0}
                </Box>
              </Box>
            </Box>

            {selectedDetail.pxpipe && (
              <Box sx={{ borderRadius: "8px", border: 1, borderColor: "divider", p: 2 }}>
                <Box sx={{ display: "flex", alignItems: "center", gap: 1, mb: 1 }}>
                  <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18, color: "text.secondary" }}>image</Icon>
                  <Box component="span" sx={{ fontWeight: 600, fontSize: 14, color: "text.primary" }}>PXPIPE</Box>
                  <Box component="span" sx={{ fontSize: 12, px: 1, py: 0.25, borderRadius: "4px", bgcolor: selectedDetail.pxpipe.applied ? "rgba(34,197,94,0.15)" : "rgba(245,158,11,0.15)", color: selectedDetail.pxpipe.applied ? "success.main" : "warning.main" }}>
                    {selectedDetail.pxpipe.applied ? "Activated" : "Skipped"}
                  </Box>
                </Box>
                {selectedDetail.pxpipe.applied ? (
                  <Box sx={{ display: "grid", gridTemplateColumns: { xs: "repeat(2, minmax(0,1fr))", sm: "repeat(4, minmax(0,1fr))" }, gap: 1, fontSize: 14 }}>
                    <Box>
                      <Box component="span" sx={{ color: "text.secondary", display: "block", fontSize: 12 }}>Original (est.)</Box>
                      <Box component="span" sx={{ fontFamily: "monospace" }}>{(selectedDetail.pxpipe.tokensBeforeEst || 0).toLocaleString()} tokens</Box>
                    </Box>
                    <Box>
                      <Box component="span" sx={{ color: "text.secondary", display: "block", fontSize: 12 }}>Compressed (est.)</Box>
                      <Box component="span" sx={{ fontFamily: "monospace" }}>{(selectedDetail.pxpipe.tokensAfterEst || 0).toLocaleString()} tokens</Box>
                    </Box>
                    <Box>
                      <Box component="span" sx={{ color: "text.secondary", display: "block", fontSize: 12 }}>Saved</Box>
                      <Box component="span" sx={{ fontFamily: "monospace", color: "success.main" }}>{selectedDetail.pxpipe.savedPct || 0}%</Box>
                    </Box>
                    <Box>
                      <Box component="span" sx={{ color: "text.secondary", display: "block", fontSize: 12 }}>Images</Box>
                      <Box component="span" sx={{ fontFamily: "monospace" }}>{selectedDetail.pxpipe.imageCount || 0} ({selectedDetail.pxpipe.durationMs || 0}ms)</Box>
                    </Box>
                  </Box>
                ) : (
                  <Typography sx={{ fontSize: 14, color: "text.secondary" }}>
                    Reason: <Box component="span" sx={{ fontFamily: "monospace" }}>{selectedDetail.pxpipe.reason}</Box>
                    {selectedDetail.pxpipe.detail ? ` — ${selectedDetail.pxpipe.detail}` : ""}
                  </Typography>
                )}
              </Box>
            )}

            <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
              <CollapsibleSection title="1. Client Request (Input)" defaultOpen={true} icon="input">
                {isTruncated(selectedDetail.request) ? (
                  <TruncatedNotice marker={selectedDetail.request} />
                ) : (
                  <Box component="pre" sx={{ maxHeight: 300, maxWidth: "100%", overflow: "auto", borderRadius: "8px", border: 1, borderColor: "divider", bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(255,255,255,0.05)" : "rgba(0,0,0,0.05)"), p: { xs: 1.5, sm: 2 }, fontFamily: "monospace", fontSize: 12, color: "text.primary", m: 0 }}>
                    {JSON.stringify(selectedDetail.request, null, 2)}
                  </Box>
                )}
              </CollapsibleSection>

              {selectedDetail.providerRequest && (
                <CollapsibleSection title="2. Provider Request (Translated)" icon="translate">
                  {isTruncated(selectedDetail.providerRequest) ? (
                    <TruncatedNotice marker={selectedDetail.providerRequest} />
                  ) : (
                    <Box component="pre" sx={{ maxHeight: 300, maxWidth: "100%", overflow: "auto", borderRadius: "8px", border: 1, borderColor: "divider", bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(255,255,255,0.05)" : "rgba(0,0,0,0.05)"), p: { xs: 1.5, sm: 2 }, fontFamily: "monospace", fontSize: 12, color: "text.primary", m: 0 }}>
                      {JSON.stringify(selectedDetail.providerRequest, null, 2)}
                    </Box>
                  )}
                </CollapsibleSection>
              )}

              {selectedDetail.providerResponse && (
                <CollapsibleSection title="3. Provider Response (Raw)" icon="data_object">
                  {isTruncated(selectedDetail.providerResponse) ? (
                    <TruncatedNotice marker={selectedDetail.providerResponse} />
                  ) : (
                    <Box component="pre" sx={{ maxHeight: 300, maxWidth: "100%", overflow: "auto", borderRadius: "8px", border: 1, borderColor: "divider", bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(255,255,255,0.05)" : "rgba(0,0,0,0.05)"), p: { xs: 1.5, sm: 2 }, fontFamily: "monospace", fontSize: 12, color: "text.primary", m: 0 }}>
                      {typeof selectedDetail.providerResponse === 'object'
                        ? JSON.stringify(selectedDetail.providerResponse, null, 2)
                        : selectedDetail.providerResponse
                      }
                    </Box>
                  )}
                </CollapsibleSection>
              )}

              <CollapsibleSection title="4. Client Response (Final)" defaultOpen={true} icon="output">
                {isTruncated(selectedDetail.response) ? (
                  <TruncatedNotice marker={selectedDetail.response} />
                ) : (
                  <>
                    {selectedDetail.response?.thinking && (
                      <Box sx={{ mb: 2 }}>
                        <Typography component="h4" sx={{ fontWeight: 600, color: "text.primary", mb: 1, display: "flex", alignItems: "center", gap: 1, fontSize: 12, textTransform: "uppercase", letterSpacing: "0.025em", opacity: 0.7 }}>
                          <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 16 }}>psychology</Icon>
                          Thinking Process
                        </Typography>
                        <Box component="pre" sx={{ maxHeight: 200, maxWidth: "100%", overflow: "auto", borderRadius: "8px", border: 1, borderColor: (t) => (t.palette.mode === "dark" ? "#92400e" : "#fde68a"), bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(69,26,3,0.3)" : "#fffbeb"), p: { xs: 1.5, sm: 2 }, fontFamily: "monospace", fontSize: 12, color: (t) => (t.palette.mode === "dark" ? "#fef3c7" : "#78350f"), m: 0 }}>
                          {selectedDetail.response.thinking}
                        </Box>
                      </Box>
                    )}

                    <Typography component="h4" sx={{ fontWeight: 600, color: "text.primary", mb: 1, fontSize: 12, textTransform: "uppercase", letterSpacing: "0.025em", opacity: 0.7 }}>
                      Content
                    </Typography>
                    <Box component="pre" sx={{ maxHeight: 300, maxWidth: "100%", overflow: "auto", borderRadius: "8px", border: 1, borderColor: "divider", bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(255,255,255,0.05)" : "rgba(0,0,0,0.05)"), p: { xs: 1.5, sm: 2 }, fontFamily: "monospace", fontSize: 12, color: "text.primary", m: 0 }}>
                      {selectedDetail.response?.content || "[No content]"}
                    </Box>
                  </>
                )}
              </CollapsibleSection>
            </Box>
          </Box>
        )}
      </Drawer>
    </Box>
  );
}
