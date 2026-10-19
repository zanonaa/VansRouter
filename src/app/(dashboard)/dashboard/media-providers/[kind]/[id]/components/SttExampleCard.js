"use client";

import { useState, useEffect } from "react";
import { Card } from "@/shared/components";
import { getProviderAlias } from "@/shared/constants/providers";
import { getModelKind } from "@/shared/constants/models";
import { getModelsByProviderId } from "@/shared/constants/models";
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

export function SttExampleCard({ providerId }) {
  const providerAlias = getProviderAlias(providerId);
  const builtinSttModels = getModelsByProviderId(providerId).filter((m) => getModelKind(m) === "stt");
  const [customSttModels, setCustomSttModels] = useState([]);
  const sttModels = [...builtinSttModels, ...customSttModels];

  const [selectedModel, setSelectedModel] = useState(builtinSttModels[0]?.id ?? "");
  const selectedModelObj = sttModels.find((m) => m.id === selectedModel);
  const allowedParams = Array.isArray(selectedModelObj?.params) ? selectedModelObj.params : [];

  const [audioFile, setAudioFile] = useState(null);
  const [language, setLanguage] = useState("");
  const [prompt, setPrompt] = useState("");
  const [responseFormat, setResponseFormat] = useState("json");
  const [temperature, setTemperature] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [useTunnel, setUseTunnel] = useState(false);
  const [localEndpoint, setLocalEndpoint] = useState("");
  const [tunnelEndpoint, setTunnelEndpoint] = useState("");
  const [result, setResult] = useState(null);
  const [latency, setLatency] = useState(null);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState("");
  const { copied: copiedCurl, copy: copyCurl } = useCopyToClipboard();
  const { copied: copiedRes, copy: copyRes } = useCopyToClipboard();

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time client-side hydration of window.location.origin; effect is the canonical pattern for window access.
    setLocalEndpoint(window.location.origin);
    fetch("/api/keys")
      .then((r) => r.json())
      // eslint-disable-next-line react-hooks/set-state-in-effect -- async fetch callback; setState happens after await, not synchronously.
      .then((d) => { setApiKey((d.keys || []).find((k) => k.isActive !== false)?.key || ""); })
      .catch(() => {});
    fetch("/api/tunnel/status")
      .then((r) => r.json())
      .then((d) => { if (d.publicUrl) setTunnelEndpoint(d.publicUrl); })
      .catch(() => {});
    const loadCustom = () => {
      fetch("/api/models/custom", { cache: "no-store" })
        .then((r) => r.json())
        .then((d) => {
          const list = (d.models || []).filter((m) => getModelKind(m) === "stt" && m.providerAlias === providerAlias);
          setCustomSttModels(list);
        })
        .catch(() => {});
    };
    loadCustom();
    window.addEventListener("focus", loadCustom);
    window.addEventListener("customModelChanged", loadCustom);
    return () => {
      window.removeEventListener("focus", loadCustom);
      window.removeEventListener("customModelChanged", loadCustom);
    };
  }, [providerAlias]);

  const endpoint = useTunnel ? tunnelEndpoint : localEndpoint;
  const modelFull = selectedModel ? `${providerAlias}/${selectedModel}` : "";

  const curlSnippet = `curl -X POST ${endpoint}/v1/audio/transcriptions \\
  -H "Authorization: Bearer ${apiKey || "YOUR_KEY"}" \\
  -F "file=@${audioFile?.name || "audio.mp3"}" \\
  -F "model=${modelFull}"${allowedParams.includes("language") && language ? ` \\\n  -F "language=${language}"` : ""}${allowedParams.includes("response_format") ? ` \\\n  -F "response_format=${responseFormat}"` : ""}${allowedParams.includes("temperature") && temperature ? ` \\\n  -F "temperature=${temperature}"` : ""}${allowedParams.includes("prompt") && prompt ? ` \\\n  -F "prompt=${prompt}"` : ""}`;

  const handleRun = async () => {
    if (!audioFile || !modelFull) return;
    setRunning(true);
    setError("");
    setResult(null);
    // eslint-disable-next-line react-hooks/purity -- event handler, not render; Date.now() is the right tool for measuring latency.
    const start = Date.now();
    try {
      const fd = new FormData();
      fd.append("file", audioFile);
      fd.append("model", modelFull);
      if (allowedParams.includes("language") && language) fd.append("language", language);
      if (allowedParams.includes("response_format")) fd.append("response_format", responseFormat);
      if (allowedParams.includes("temperature") && temperature) fd.append("temperature", temperature);
      if (allowedParams.includes("prompt") && prompt) fd.append("prompt", prompt);

      const headers = {};
      if (apiKey) headers["Authorization"] = `Bearer ${apiKey}`;
      const res = await fetch("/api/v1/audio/transcriptions", { method: "POST", headers, body: fd });
      // eslint-disable-next-line react-hooks/purity -- event handler; computing elapsed time.
      setLatency(Date.now() - start);
      const ct = res.headers.get("content-type") || "";
      const data = ct.includes("application/json") ? await res.json() : await res.text();
      if (!res.ok) {
        setError(data?.error?.message || data?.error || data || `HTTP ${res.status}`);
        return;
      }
      setResult(data);
    } catch (e) {
      setError(e.message || "Network error");
    } finally {
      setRunning(false);
    }
  };

  const resultStr = typeof result === "string" ? result : (result ? JSON.stringify(result, null, 2) : `{\n  "text": "Hello world..."\n}`);

  return (
    <Card>
      <Typography component="h2" sx={{ fontSize: 18, fontWeight: 600, mb: 2 }}>Example</Typography>
      <Box sx={{ display: "flex", flexDirection: "column", gap: 1.25 }}>
        {/* Model */}
        {sttModels.length > 0 ? (
          <Row label="Model">
            <Box
              component="select"
              value={selectedModel}
              onChange={(e) => setSelectedModel(e.target.value)}
              sx={inputSx}
            >
              {sttModels.map((m) => (
                <option key={m.id} value={m.id}>{m.name || m.id}</option>
              ))}
            </Box>
          </Row>
        ) : (
          <Row label="Model">
            <Box
              component="input"
              value={selectedModel}
              onChange={(e) => setSelectedModel(e.target.value)}
              placeholder="Enter model id"
              sx={{ ...inputSx, fontFamily: "monospace" }}
            />
          </Row>
        )}

        {/* Endpoint */}
        <Row label="Endpoint">
          <Box sx={{ display: "flex", width: { xs: "100%", sm: "auto" }, flexDirection: { xs: "column", sm: "row" }, gap: 1, alignItems: { sm: "center" } }}>
            <Box component="span" sx={{ ...readOnlySx, minWidth: 0, flex: 1 }}>
              {endpoint}/v1/audio/transcriptions
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
          <Box component="span" sx={{ px: 1.5, py: 0.75, fontSize: 14, fontFamily: "monospace", color: "text.primary", bgcolor: "action.hover", borderRadius: 2, display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
            {apiKey ? `${apiKey.slice(0, 8)}${"\u2022".repeat(Math.min(20, Math.max(0, apiKey.length - 8)))}` : <Box component="span" sx={{ color: "text.secondary", fontStyle: "italic" }}>No key configured</Box>}
          </Box>
        </Row>

        {/* Audio file */}
        <Row label="Audio File">
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            <Box
              component="input"
              type="file"
              accept="audio/*,video/mp4,.m4a,.mp3,.wav,.ogg,.flac,.webm,.opus"
              onChange={(e) => setAudioFile(e.target.files?.[0] || null)}
              sx={{
                width: "100%",
                fontSize: 12,
                color: "text.secondary",
                "&::file-selector-button": {
                  mr: 1,
                  py: 0.5,
                  px: 1.25,
                  borderRadius: 2,
                  border: 1,
                  borderColor: "divider",
                  bgcolor: "background.default",
                  color: "text.primary",
                  cursor: "pointer",
                },
                "&:hover::file-selector-button": { bgcolor: "action.hover" },
              }}
            />
            {audioFile && (
              <Box component="span" sx={{ fontSize: 12, color: "text.secondary", fontFamily: "monospace" }}>
                {audioFile.name} · {(audioFile.size / 1024).toFixed(1)} KB
              </Box>
            )}
          </Box>
        </Row>

        {/* Language (if model supports) */}
        {allowedParams.includes("language") && (
          <Row label="Language">
            <Box
              component="input"
              value={language}
              onChange={(e) => setLanguage(e.target.value)}
              placeholder="e.g. en, vi, ja (auto-detect if empty)"
              sx={{ ...inputSx, fontFamily: "monospace" }}
            />
          </Row>
        )}

        {/* Prompt (if model supports) */}
        {allowedParams.includes("prompt") && (
          <Row label="Prompt">
            <Box
              component="input"
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              placeholder="optional context to improve accuracy"
              sx={inputSx}
            />
          </Row>
        )}

        {/* Temperature (if model supports) */}
        {allowedParams.includes("temperature") && (
          <Row label="Temperature">
            <Box
              component="input"
              type="number"
              step="0.1"
              min="0"
              max="1"
              value={temperature}
              onChange={(e) => setTemperature(e.target.value)}
              placeholder="0 - 1 (default 0)"
              sx={inputSx}
            />
          </Row>
        )}

        {/* Response format (if model supports) */}
        {allowedParams.includes("response_format") && (
          <Row label="Response Format">
            <Box
              component="select"
              value={responseFormat}
              onChange={(e) => setResponseFormat(e.target.value)}
              sx={inputSx}
            >
              <option value="json">json</option>
              <option value="text">text</option>
              <option value="srt">srt</option>
              <option value="verbose_json">verbose_json</option>
              <option value="vtt">vtt</option>
            </Box>
          </Row>
        )}

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
                disabled={running || !audioFile || !modelFull}
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
                {running ? "Transcribing..." : "Run"}
              </Box>
            </Box>
          </Box>
          <Box component="pre" sx={codeBlockSx}>{curlSnippet}</Box>
        </Box>

        {error && <Typography sx={{ fontSize: 12, color: "error.main", wordBreak: "break-word" }}>{error}</Typography>}

        {/* Response */}
        <Box>
          <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1, alignItems: { sm: "center" }, justifyContent: { sm: "space-between" }, mb: 0.75 }}>
            <Box component="span" sx={sectionLabelSx}>
              Response {result && latency && <Box component="span" sx={{ fontWeight: 400, textTransform: "none" }}>&#9889; {latency}ms</Box>}
            </Box>
            {result && (
              <Box component="button" onClick={() => copyRes(resultStr)} sx={copyButtonSx}>
                <MsIcon sx={{ fontSize: 14 }}>{copiedRes ? "check" : "content_copy"}</MsIcon>
                {copiedRes ? "Copied" : "Copy"}
              </Box>
            )}
          </Box>
          <Box component="pre" sx={{ ...codeBlockSx, opacity: 0.7 }}>
            {resultStr}
          </Box>
        </Box>
      </Box>
    </Card>
  );
}
