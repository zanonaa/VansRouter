"use client";

import { useState, useEffect } from "react";
import { Card, Button } from "@/shared/components";
import { MEDIA_PROVIDER_KINDS, getProviderAlias } from "@/shared/constants/providers";
import { useCopyToClipboard } from "@/shared/hooks/useCopyToClipboard";
import { Row } from "./exampleShared";
import Box from "@mui/material/Box";
import Icon from "@mui/material/Icon";

function MsIcon({ children, sx, ...props }) {
  return (
    <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18, ...sx }} {...props}>
      {children}
    </Icon>
  );
}

const inputSx = {
  width: "100%",
  px: 1.5,
  py: 0.75,
  fontSize: 14,
  border: 1,
  borderColor: "divider",
  borderRadius: 2,
  bgcolor: "background.default",
  color: "text.primary",
  outline: "none",
  "&:focus": { borderColor: "primary.main" },
};

const readOnlySx = {
  ...inputSx,
  fontFamily: "monospace",
  bgcolor: "action.hover",
  overflow: "hidden",
  textOverflow: "ellipsis",
  whiteSpace: "nowrap",
  display: "block",
};

const copyButtonSx = {
  display: "inline-flex",
  alignItems: "center",
  gap: 0.5,
  fontSize: 12,
  color: "primary.main",
  border: 0,
  background: "none",
  cursor: "pointer",
  "&:hover": { textDecoration: "underline" },
};

const codeBlockSx = {
  p: 1.5,
  fontSize: 12,
  fontFamily: "monospace",
  bgcolor: "action.hover",
  border: 1,
  borderColor: "divider",
  borderRadius: 2,
  overflowX: "auto",
  color: "text.primary",
  m: 0,
};

const EXA_PRESETS = [
  {
    id: "coding",
    name: "Coding Agent (Fast & Highlights)",
    description: "Ideal for coding assistants searching for docs, errors, or APIs with highlights and code blocks.",
    config: {
      query: "Next.js 16 App Router server actions streaming",
      type: "fast",
      numResults: 5,
      includeDomains: ["nextjs.org", "github.com"],
      contents: {
        text: { maxCharacters: 1000, verbosity: "compact" },
        highlights: { query: "server actions streaming", numSentences: 2, highlightsPerUrl: 3 },
        extras: { links: 2, codeBlocks: 3 }
      }
    }
  },
  {
    id: "deep-research",
    name: "Deep Research with Summary",
    description: "Deep crawl across multiple queries with AI summaries and custom structured output schema.",
    config: {
      query: "Latest breakthroughs in AI reasoning models 2026",
      type: "deep",
      numResults: 5,
      additionalQueries: [
        "frontier AI reasoning architectures 2026",
        "test-time compute scaling laws"
      ],
      systemPrompt: "Synthesize findings focusing on test-time compute and verifiable domains.",
      contents: {
        text: true,
        summary: { query: "Key technical innovations and scaling results" },
        subpages: 2
      }
    }
  },
  {
    id: "livecrawl",
    name: "Live Web & Real-Time",
    description: "Bypasses cache to crawl real-time web contents with fallback timeout.",
    config: {
      query: "OpenAI latest announcement today",
      type: "instant",
      numResults: 3,
      contents: {
        text: { includeHtmlTags: false },
        livecrawl: "always",
        livecrawlTimeout: 5000,
        extras: { links: 5 }
      }
    }
  },
  {
    id: "hipaa",
    name: "HIPAA Compliant",
    description: "Strict HIPAA compliance mode with cache-only content.",
    config: {
      query: "Medical diagnostic AI benchmark protocols",
      type: "instant",
      numResults: 3,
      compliance: "hipaa",
      contents: {
        text: true,
        maxAgeHours: -1
      }
    }
  }
];

export function ExaSearchExampleCard({ providerId }) {
  const providerAlias = getProviderAlias(providerId);
  const kindConfig = MEDIA_PROVIDER_KINDS.find((k) => k.id === "webSearch");

  const [activePreset, setActivePreset] = useState("coding");
  const [requestJson, setRequestJson] = useState(() =>
    JSON.stringify(
      {
        provider: providerAlias,
        ...EXA_PRESETS[0].config
      },
      null,
      2
    )
  );

  const [apiKey, setApiKey] = useState("");
  const [useTunnel, setUseTunnel] = useState(false);
  const [localEndpoint, setLocalEndpoint] = useState("");
  const [tunnelEndpoint, setTunnelEndpoint] = useState("");
  const [result, setResult] = useState(null);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState("");
  const [connections, setConnections] = useState([]);
  const [pinnedConnectionId, setPinnedConnectionId] = useState("");
  const { copied: copiedCurl, copy: copyCurl } = useCopyToClipboard();
  const { copied: copiedRes, copy: copyRes } = useCopyToClipboard();

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time client-side hydration of window.location.origin.
    setLocalEndpoint(window.location.origin);
    fetch("/api/keys")
      .then((r) => r.json())
      // eslint-disable-next-line react-hooks/set-state-in-effect -- async fetch callback.
      .then((d) => {
        setApiKey((d.keys || []).find((k) => k.isActive !== false)?.key || "");
      })
      .catch(() => {});
    fetch("/api/tunnel/status")
      .then((r) => r.json())
      // eslint-disable-next-line react-hooks/set-state-in-effect -- async fetch callback.
      .then((d) => {
        if (d.publicUrl) setTunnelEndpoint(d.publicUrl);
      })
      .catch(() => {});
    fetch("/api/providers/client")
      .then((r) => r.json())
      // eslint-disable-next-line react-hooks/set-state-in-effect -- async fetch callback.
      .then((d) => {
        const conns = (d.connections || []).filter(
          (c) => c.provider === providerId && c.isActive !== false
        );
        setConnections(conns);
      })
      .catch(() => {});
  }, [providerId]);

  const endpoint = useTunnel ? tunnelEndpoint : localEndpoint;
  const apiPath = kindConfig?.endpoint?.path || "/v1/search";

  const handleSelectPreset = (presetId) => {
    setActivePreset(presetId);
    const preset = EXA_PRESETS.find((p) => p.id === presetId);
    if (preset) {
      setRequestJson(
        JSON.stringify(
          {
            provider: providerAlias,
            ...preset.config
          },
          null,
          2
        )
      );
    }
  };

  let parsedBody = {};
  let jsonError = null;
  try {
    parsedBody = JSON.parse(requestJson);
  } catch (err) {
    jsonError = err.message;
  }

  const headersPreview = `-H "Content-Type: application/json" \\\n  -H "Authorization: Bearer ${apiKey || "YOUR_KEY"}"${
    pinnedConnectionId ? ` \\\n  -H "x-connection-id: ${pinnedConnectionId}"` : ""
  }`;

  const curlSnippet = `curl -X POST ${endpoint}${apiPath} \\
  ${headersPreview.replace(/\\\n  /g, "\\\n  ")} \\
  -d '${requestJson.replace(/'/g, "'\\''")}'`;

  const handleRun = async () => {
    if (jsonError) {
      setError(`Invalid JSON payload: ${jsonError}`);
      return;
    }
    setRunning(true);
    setError("");
    setResult(null);
    const start = Date.now();
    try {
      const headers = { "Content-Type": "application/json" };
      if (apiKey) headers["Authorization"] = `Bearer ${apiKey}`;
      if (pinnedConnectionId) headers["x-connection-id"] = pinnedConnectionId;

      const bodyToSend = {
        provider: providerAlias,
        ...parsedBody
      };

      const res = await fetch(`/api${apiPath}`, {
        method: "POST",
        headers,
        body: JSON.stringify(bodyToSend)
      });

      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        setError(data?.error?.message || data?.error || `HTTP ${res.status}`);
        return;
      }

      const ctype = res.headers.get("content-type") || "";
      if (ctype.includes("text/event-stream") && res.body) {
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buf = "";
        let chunks = [];
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buf += decoder.decode(value, { stream: true });
        }
        setResult({ data: { stream_text: buf }, latencyMs: Date.now() - start });
      } else {
        const data = await res.json();
        setResult({ data, latencyMs: Date.now() - start });
      }
    } catch (e) {
      setError(e.message || "Network error");
    } finally {
      setRunning(false);
    }
  };

  const resultJson = result ? JSON.stringify(result.data, null, 2) : "";

  return (
    <Card>
      <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", justifyContent: "space-between", gap: 1, mb: 2 }}>
        <Box>
          <Box component="h2" sx={{ fontSize: 18, fontWeight: 600 }}>Exa Search 1:1 Request Playground</Box>
          <Box component="p" sx={{ fontSize: 12, color: "text.secondary", m: 0 }}>
            Test all official Exa Search API parameters including deep search, highlights, livecrawl, and HIPAA compliance.
          </Box>
        </Box>
        <Box
          component="a"
          href="https://exa.ai/docs/reference/search-api-guide-for-coding-agents"
          target="_blank"
          rel="noopener noreferrer"
          sx={{ fontSize: 12, color: "primary.main", display: "inline-flex", alignItems: "center", gap: 0.5, "&:hover": { textDecoration: "underline" } }}
        >
          <MsIcon sx={{ fontSize: 14 }}>open_in_new</MsIcon>
          Exa Docs
        </Box>
      </Box>

      <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
        {/* Preset Tabs */}
        <Row label="Presets">
          <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.75 }}>
            {EXA_PRESETS.map((preset) => {
              const active = activePreset === preset.id;
              return (
                <Box
                  key={preset.id}
                  component="button"
                  type="button"
                  onClick={() => handleSelectPreset(preset.id)}
                  sx={{
                    px: 1.25,
                    py: 0.5,
                    fontSize: 12,
                    borderRadius: 1,
                    fontWeight: 500,
                    transition: "background-color 150ms ease, color 150ms ease",
                    cursor: "pointer",
                    bgcolor: active ? "primary.main" : "action.hover",
                    color: active ? "#fff" : "text.secondary",
                    border: active ? "1px solid transparent" : "1px solid",
                    borderColor: active ? "transparent" : "divider",
                    "&:hover": { color: active ? "#fff" : "text.primary" },
                  }}
                >
                  {preset.name}
                </Box>
              );
            })}
          </Box>
        </Row>

        {/* Endpoint */}
        <Row label="Endpoint">
          <Box sx={{ display: "flex", width: { xs: "100%", sm: "auto" }, flexDirection: { xs: "column", sm: "row" }, gap: 1, alignItems: { sm: "center" } }}>
            <Box component="span" sx={{ ...readOnlySx, minWidth: 0, flex: 1 }}>
              {endpoint}{apiPath}
            </Box>
            {tunnelEndpoint && (
              <Box
                component="button"
                onClick={() => setUseTunnel((v) => !v)}
                title={useTunnel ? "Using tunnel" : "Using local"}
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 0.5,
                  fontSize: 12,
                  px: 1,
                  py: 0.75,
                  borderRadius: 2,
                  flexShrink: 0,
                  transition: "color 150ms ease, border-color 150ms ease, background-color 150ms ease",
                  cursor: "pointer",
                  border: 1,
                  borderColor: useTunnel ? "primary.main" : "divider",
                  bgcolor: useTunnel ? "action.selected" : "transparent",
                  color: useTunnel ? "primary.main" : "text.secondary",
                  "&:hover": { color: "primary.main" },
                }}
              >
                <MsIcon sx={{ fontSize: 14 }}>wifi_tethering</MsIcon>
                Tunnel
              </Box>
            )}
          </Box>
        </Row>

        {/* API Key */}
        <Row label="API Key">
          <Box component="span" sx={{ px: 1.5, py: 0.75, fontSize: 14, fontFamily: "monospace", color: "text.primary", bgcolor: "action.hover", borderRadius: 2, display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
            {apiKey ? (
              `${apiKey.slice(0, 8)}${"\u2022".repeat(Math.min(20, apiKey.length - 8))}`
            ) : (
              <Box component="span" sx={{ color: "text.secondary", fontStyle: "italic" }}>No key configured</Box>
            )}
          </Box>
        </Row>

        {/* Connection picker */}
        {connections.length > 0 && (
          <Row label="Connection">
            <Box
              component="select"
              value={pinnedConnectionId}
              onChange={(e) => setPinnedConnectionId(e.target.value)}
              sx={inputSx}
            >
              <option value="">Auto (Round-robin / Failover)</option>
              {connections.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name || c.id.slice(0, 8)}
                </option>
              ))}
            </Box>
          </Row>
        )}

        {/* Editable JSON Payload */}
        <Row label="Payload">
          <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5, width: "100%" }}>
            <Box
              component="textarea"
              value={requestJson}
              onChange={(e) => setRequestJson(e.target.value)}
              rows={12}
              sx={{
                width: "100%",
                p: 1.25,
                fontSize: 12,
                fontFamily: "monospace",
                border: 1,
                borderColor: jsonError ? "error.main" : "divider",
                borderRadius: 2,
                bgcolor: "action.hover",
                color: "text.primary",
                outline: "none",
                resize: "vertical",
                "&:focus": { borderColor: jsonError ? "error.main" : "primary.main" },
              }}
            />
            {jsonError && (
              <Box component="span" sx={{ fontSize: 11, color: "error.main", fontFamily: "monospace" }}>Invalid JSON: {jsonError}</Box>
            )}
          </Box>
        </Row>

        {/* Action Button */}
        <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1, pt: 1 }}>
          <Button
            size="sm"
            onClick={handleRun}
            disabled={running || !!jsonError}
            icon={running ? "sync" : "play_arrow"}
          >
            {running ? "Searching Exa..." : "Send Request"}
          </Button>
        </Box>

        {/* Error */}
        {error && (
          <Box sx={(theme) => ({ p: 1.5, fontSize: 12, bgcolor: "rgba(239,68,68,0.1)", border: "1px solid rgba(239,68,68,0.3)", color: theme.palette.mode === "dark" ? "#f87171" : "#dc2626", borderRadius: 2 })}>
            {error}
          </Box>
        )}

        {/* cURL preview */}
        <Box sx={{ position: "relative", mt: 1 }}>
          <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", mb: 0.75 }}>
            <Box component="span" sx={{ fontSize: 12, fontWeight: 500, color: "text.secondary" }}>cURL Request</Box>
            <Box component="button" onClick={() => copyCurl(curlSnippet)} sx={copyButtonSx}>
              <MsIcon sx={{ fontSize: 14 }}>
                {copiedCurl ? "check" : "content_copy"}
              </MsIcon>
              {copiedCurl ? "Copied" : "Copy cURL"}
            </Box>
          </Box>
          <Box component="pre" sx={{ ...codeBlockSx, maxHeight: 192 }}>
            {curlSnippet}
          </Box>
        </Box>

        {/* Response preview */}
        {result && (
          <Box sx={{ position: "relative", mt: 1 }}>
            <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", mb: 0.75 }}>
              <Box component="span" sx={{ fontSize: 12, fontWeight: 500, color: "text.secondary" }}>
                Response ({result.latencyMs}ms)
              </Box>
              <Box component="button" onClick={() => copyRes(resultJson)} sx={copyButtonSx}>
                <MsIcon sx={{ fontSize: 14 }}>
                  {copiedRes ? "check" : "content_copy"}
                </MsIcon>
                {copiedRes ? "Copied" : "Copy JSON"}
              </Box>
            </Box>
            <Box component="pre" sx={{ ...codeBlockSx, maxHeight: 384 }}>
              {resultJson}
            </Box>
          </Box>
        )}
      </Box>
    </Card>
  );
}
