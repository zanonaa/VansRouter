"use client";

import { useState, useEffect } from "react";
import { Card } from "@/shared/components";
import { getProviderAlias, isCustomEmbeddingProvider } from "@/shared/constants/providers";
import { getModelsByProviderId, getModelKind } from "@/shared/constants/models";
import { useCopyToClipboard } from "@/shared/hooks/useCopyToClipboard";
import { Row } from "./exampleShared";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
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

const copyButtonSx = {
  display: "inline-flex",
  alignItems: "center",
  gap: 0.5,
  fontSize: 12,
  color: "text.secondary",
  border: 0,
  background: "none",
  cursor: "pointer",
  transition: "color 150ms ease",
  "&:hover": { color: "primary.main" },
};

const sectionLabelSx = {
  fontSize: 12,
  fontWeight: 600,
  color: "text.secondary",
  textTransform: "uppercase",
  letterSpacing: "0.05em",
};

const codeBlockSx = {
  bgcolor: "action.hover",
  borderRadius: 2,
  px: 1.5,
  py: 1.25,
  fontSize: 12,
  fontFamily: "monospace",
  color: "text.primary",
  overflowX: "auto",
  whiteSpace: "pre-wrap",
  wordBreak: "break-all",
  m: 0,
};

const DEFAULT_RESPONSE_EXAMPLE = `{
  "object": "list",
  "data": [{
    "object": "embedding",
    "index": 0,
    "embedding": [0.002301, -0.019212, 0.004815, -0.031249, ...]
  }],
  "model": "...",
  "usage": { "prompt_tokens": 9, "total_tokens": 9 }
}`;

export function EmbeddingExampleCard({ providerId, customAlias }) {
  const isCustom = isCustomEmbeddingProvider(providerId);
  const providerAlias = isCustom ? (customAlias || providerId) : getProviderAlias(providerId);
  const embeddingModels = isCustom ? [] : getModelsByProviderId(providerId).filter((m) => getModelKind(m) === "embedding");

  const [selectedModel, setSelectedModel] = useState(embeddingModels[0]?.id ?? "");
  const [input, setInput] = useState("The quick brown fox jumps over the lazy dog");
  const [dimensions, setDimensions] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [useTunnel, setUseTunnel] = useState(false);
  const [localEndpoint, setLocalEndpoint] = useState("");
  const [tunnelEndpoint, setTunnelEndpoint] = useState("");
  const [result, setResult] = useState(null);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState("");
  const { copied: copiedCurl, copy: copyCurl } = useCopyToClipboard();
  const { copied: copiedRes, copy: copyRes } = useCopyToClipboard();

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time client-side hydration of window.location.origin.
    setLocalEndpoint(window.location.origin);
    fetch("/api/keys")
      .then((r) => r.json())
      // eslint-disable-next-line react-hooks/set-state-in-effect -- async fetch callback.
      .then((d) => { setApiKey((d.keys || []).find((k) => k.isActive !== false)?.key || ""); })
      .catch(() => {});
    fetch("/api/tunnel/status")
      .then((r) => r.json())
      .then((d) => { if (d.publicUrl) setTunnelEndpoint(d.publicUrl); })
      .catch(() => {});
  }, []);

  const endpoint = useTunnel ? tunnelEndpoint : localEndpoint;
  const modelFull = selectedModel ? `${providerAlias}/${selectedModel}` : "";

  // Build request body — include dimensions only if user provided a positive number
  const buildBody = () => {
    const body = { model: modelFull, input: input.trim() };
    const dim = Number(dimensions);
    if (dimensions && Number.isFinite(dim) && dim > 0) body.dimensions = dim;
    return body;
  };

  const curlSnippet = `curl -X POST ${endpoint}/v1/embeddings \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${apiKey || "YOUR_KEY"}" \\
  -d '${JSON.stringify(buildBody())}'`;

  const handleRun = async () => {
    if (!input.trim() || !modelFull) return;
    setRunning(true);
    setError("");
    setResult(null);
    const start = Date.now();
    try {
      const headers = { "Content-Type": "application/json" };
      if (apiKey) headers["Authorization"] = `Bearer ${apiKey}`;
      const res = await fetch("/api/v1/embeddings", {
        method: "POST",
        headers,
        body: JSON.stringify(buildBody()),
      });
      const latencyMs = Date.now() - start;
      const data = await res.json();
      if (!res.ok) { setError(data?.error?.message || data?.error || `HTTP ${res.status}`); return; }
      setResult({ data, latencyMs });
    } catch (e) {
      setError(e.message || "Network error");
    } finally {
      setRunning(false);
    }
  };

  // Compact embedding array: first 4 values + count
  const formatResultJson = (data) => {
    if (!data) return DEFAULT_RESPONSE_EXAMPLE;
    const clone = JSON.parse(JSON.stringify(data));
    (clone.data || []).forEach((item) => {
      if (Array.isArray(item.embedding) && item.embedding.length > 4) {
        item.embedding = [...item.embedding.slice(0, 4).map((v) => parseFloat(v.toFixed(6))), `... (${item.embedding.length} dims)`];
      }
    });
    return JSON.stringify(clone, null, 2);
  };

  const resultJson = result ? JSON.stringify(result.data, null, 2) : "";

  return (
    <Card>
      <Typography component="h2" sx={{ fontSize: 18, fontWeight: 600, mb: 2 }}>Example</Typography>

      <Box sx={{ display: "flex", flexDirection: "column", gap: 1.25 }}>
        {/* Model — text input for custom node, dropdown otherwise */}
        <Row label="Model">
          {isCustom ? (
            <Box
              component="input"
              value={selectedModel}
              onChange={(e) => setSelectedModel(e.target.value)}
              placeholder="e.g. voyage-3, embed-english-v3.0, text-embedding-3-small"
              sx={{ ...inputSx, fontFamily: "monospace" }}
            />
          ) : (
            <Box
              component="select"
              value={selectedModel}
              onChange={(e) => setSelectedModel(e.target.value)}
              sx={inputSx}
            >
              {embeddingModels.map((m) => (
                <option key={m.id} value={m.id}>{m.name || m.id}</option>
              ))}
            </Box>
          )}
        </Row>

        {/* Endpoint */}
        <Row label="Endpoint">
          <Box sx={{ display: "flex", width: { xs: "100%", sm: "auto" }, flexDirection: { xs: "column", sm: "row" }, gap: 1, alignItems: { sm: "center" } }}>
            <Box
              component="input"
              value={endpoint}
              onChange={(e) => useTunnel ? setTunnelEndpoint(e.target.value) : setLocalEndpoint(e.target.value)}
              sx={{ ...inputSx, minWidth: 0, flex: 1, fontFamily: "monospace" }}
              placeholder="http://localhost:3000"
            />
            {/* Tunnel toggle — only show if tunnel URL is available */}
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
                  border: 1,
                  borderColor: useTunnel ? "primary.main" : "divider",
                  bgcolor: useTunnel ? "action.selected" : "transparent",
                  color: useTunnel ? "primary.main" : "text.secondary",
                  cursor: "pointer",
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
          <Box
            component="input"
            type="password"
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            placeholder="sk-..."
            sx={{ ...inputSx, fontFamily: "monospace" }}
          />
        </Row>

        {/* Input */}
        <Row label="Input">
          <Box sx={{ position: "relative" }}>
            <Box
              component="input"
              value={input}
              onChange={(e) => setInput(e.target.value)}
              sx={{ ...inputSx, pr: 3 }}
            />
            {input && (
              <Box
                component="button"
                type="button"
                onClick={() => setInput("")}
                sx={{ position: "absolute", right: 8, top: "50%", transform: "translateY(-50%)", color: "text.secondary", border: 0, background: "none", cursor: "pointer", transition: "color 150ms ease", "&:hover": { color: "primary.main" } }}
              >
                <MsIcon sx={{ fontSize: 14 }}>close</MsIcon>
              </Box>
            )}
          </Box>
        </Row>

        {/* Dimensions (optional) — truncate embedding vector length */}
        <Row label="Dimensions">
          <Box
            component="input"
            type="number"
            min="1"
            value={dimensions}
            onChange={(e) => setDimensions(e.target.value)}
            placeholder="optional, e.g. 512, 1024 (leave empty for default)"
            sx={inputSx}
          />
        </Row>

        {/* Curl + Run */}
        <Box sx={{ mt: 0.5 }}>
          <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1, alignItems: { sm: "center" }, justifyContent: { sm: "space-between" }, mb: 0.75 }}>
            <Box component="span" sx={sectionLabelSx}>Request</Box>
            <Box sx={{ display: "flex", width: { xs: "100%", sm: "auto" }, flexDirection: { xs: "column", sm: "row" }, gap: 1, alignItems: { sm: "center" } }}>
              <Box component="button" onClick={() => copyCurl(curlSnippet)} sx={copyButtonSx}>
                <MsIcon sx={{ fontSize: 14 }}>{copiedCurl ? "check" : "content_copy"}</MsIcon>
                {copiedCurl ? "Copied" : "Copy"}
              </Box>
              <Box
                component="button"
                onClick={handleRun}
                disabled={running || !input.trim() || !modelFull}
                sx={{
                  display: "flex",
                  width: { xs: "100%", sm: "auto" },
                  alignItems: "center",
                  justifyContent: "center",
                  gap: 0.75,
                  px: 1.5,
                  py: 0.5,
                  borderRadius: 2,
                  bgcolor: "primary.main",
                  color: "#fff",
                  fontSize: 12,
                  fontWeight: 500,
                  border: 0,
                  cursor: "pointer",
                  transition: "background-color 150ms ease",
                  "&:hover": { bgcolor: "primary.dark" },
                  "&:disabled": { opacity: 0.5, cursor: "not-allowed" },
                }}
              >
                <MsIcon sx={{ fontSize: 14, ...(running ? { animation: "spin 1s linear infinite" } : {}) }}>
                  play_arrow
                </MsIcon>
                {running ? "Running..." : "Run"}
              </Box>
            </Box>
          </Box>
          <Box component="pre" sx={codeBlockSx}>{curlSnippet}</Box>
        </Box>

        {/* Error */}
        {error && <Typography sx={{ fontSize: 12, color: "error.main", wordBreak: "break-word" }}>{error}</Typography>}

        {/* Response — default example or real result */}
        <Box>
          <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1, alignItems: { sm: "center" }, justifyContent: { sm: "space-between" }, mb: 0.75 }}>
            <Box component="span" sx={sectionLabelSx}>
              Response {result && <Box component="span" sx={{ fontWeight: 400, textTransform: "none" }}>&#9889; {result.latencyMs}ms</Box>}
            </Box>
            {result && (
              <Box component="button" onClick={() => copyRes(resultJson)} sx={copyButtonSx}>
                <MsIcon sx={{ fontSize: 14 }}>{copiedRes ? "check" : "content_copy"}</MsIcon>
                {copiedRes ? "Copied" : "Copy"}
              </Box>
            )}
          </Box>
          <Box component="pre" sx={{ ...codeBlockSx, opacity: 0.7 }}>
            {formatResultJson(result?.data)}
          </Box>
        </Box>
      </Box>
    </Card>
  );
}
