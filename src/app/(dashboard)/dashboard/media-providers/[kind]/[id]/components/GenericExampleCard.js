"use client";

import { useState, useEffect } from "react";
import { Card } from "@/shared/components";
import { MEDIA_PROVIDER_KINDS, getProviderAlias, resolveProviderId } from "@/shared/constants/providers";
import { getModelsByProviderId, getModelKind } from "@/shared/constants/models";
import { useCopyToClipboard } from "@/shared/hooks/useCopyToClipboard";
import { Row, KIND_EXAMPLE_CONFIG } from "./exampleShared";
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

const CLOUDFLARE_TEST_IMAGE_URL = "https://pub-1fb693cb11cc46b2b2f656f51e015a2c.r2.dev/dog.webp";
const CLOUDFLARE_TEST_MASK_URL = "https://pub-1fb693cb11cc46b2b2f656f51e015a2c.r2.dev/dog-mask.webp";

function getImageEditDefaults(providerId, modelId) {
  if (providerId !== "cloudflare-ai") return {};
  if (modelId === "@cf/runwayml/stable-diffusion-v1-5-img2img") {
    return { image: CLOUDFLARE_TEST_IMAGE_URL };
  }
  if (modelId === "@cf/runwayml/stable-diffusion-v1-5-inpainting") {
    return { image: CLOUDFLARE_TEST_IMAGE_URL, mask_image: CLOUDFLARE_TEST_MASK_URL };
  }
  return {};
}

function toImagePreviewSrc(value) {
  const trimmed = typeof value === "string" ? value.trim() : "";
  if (!trimmed) return "";
  if (/^(data:image\/|https?:\/\/)/i.test(trimmed)) return trimmed;
  return `data:image/png;base64,${trimmed}`;
}

export function GenericExampleCard({ providerId, kind }) {
  const providerAlias = getProviderAlias(providerId);
  const resolvedId = resolveProviderId(providerAlias);
  const safeProviderAlias = resolvedId === providerId ? providerAlias : providerId;
  const kindConfig = MEDIA_PROVIDER_KINDS.find((k) => k.id === kind);
  const exConfig = KIND_EXAMPLE_CONFIG[kind];
  const safeExConfig = exConfig || {};

  // Get models for this kind (e.g., type="image")
  const kindModels = getModelsByProviderId(providerId).filter((m) => getModelKind(m) === kind);
  // Kinds that need a model identifier in the request (image/video/music)
  const KIND_NEEDS_MODEL = new Set(["image", "video", "music", "imageToText"]);
  const needsModel = KIND_NEEDS_MODEL.has(kind);
  const allowManualModel = needsModel && kindModels.length === 0;
  const [selectedModel, setSelectedModel] = useState(kindModels[0]?.id ?? "");
  const selectedModelObj = kindModels.find((m) => m.id === selectedModel);
  const supportsEdit = !!selectedModelObj?.capabilities?.includes("edit");
  const supportsMask = !!selectedModelObj?.capabilities?.includes("mask");

  const [input, setInput] = useState(safeExConfig.defaultInput || "");
  const [refImage, setRefImage] = useState("");
  const [maskImage, setMaskImage] = useState("");
  const [extraValues, setExtraValues] = useState(() =>
    (safeExConfig.extraFields || []).reduce((acc, f) => { acc[f.key] = f.default ?? ""; return acc; }, {})
  );
  const [apiKey, setApiKey] = useState("");
  const [useTunnel, setUseTunnel] = useState(false);
  const [localEndpoint, setLocalEndpoint] = useState("");
  const [tunnelEndpoint, setTunnelEndpoint] = useState("");
  const [result, setResult] = useState(null);
  const [progress, setProgress] = useState(null); // { stage, bytesReceived }
  const [partialImage, setPartialImage] = useState(null);
  const [imageOutputFormat, setImageOutputFormat] = useState("json"); // json | binary
  const [binaryImageUrl, setBinaryImageUrl] = useState("");
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
      .then((d) => { setApiKey((d.keys || []).find((k) => k.isActive !== false)?.key || ""); })
      .catch(() => {});
    fetch("/api/tunnel/status")
      .then((r) => r.json())
      .then((d) => { if (d.publicUrl) setTunnelEndpoint(d.publicUrl); })
      .catch(() => {});
    // Load active connections of this provider for pinning
    fetch("/api/providers/client")
      .then((r) => r.json())
      .then((d) => {
        const conns = (d.connections || []).filter((c) => c.provider === providerId && c.isActive !== false);
        setConnections(conns);
      })
      .catch(() => {});
  }, [providerId]);

  // Safe to early-return now that all hooks are declared
  if (!kindConfig || !exConfig) return null;

  const endpoint = useTunnel ? tunnelEndpoint : localEndpoint;
  const apiPath = kindConfig.endpoint.path;
  // webSearch/webFetch: use safeProviderAlias only. Other kinds: append model when present.
  const modelFull = !needsModel
    ? safeProviderAlias
    : (selectedModel ? `${safeProviderAlias}/${selectedModel}` : (allowManualModel ? "" : safeProviderAlias));
  const imageEditDefaults = getImageEditDefaults(providerId, selectedModel);
  const effectiveRefImage = refImage.trim() || imageEditDefaults.image || "";
  const effectiveMaskImage = maskImage.trim() || imageEditDefaults.mask_image || "";
  const refImagePreviewSrc = toImagePreviewSrc(effectiveRefImage);
  const maskImagePreviewSrc = toImagePreviewSrc(effectiveMaskImage);

  // Build request body with optional extra fields (only non-empty values)
  const extraBodyFromFields = Object.entries(extraValues).reduce((acc, [k, v]) => {
    if (v === "" || v === null || v === undefined) return acc;
    if (typeof v === "number" && Number.isNaN(v)) return acc;
    acc[k] = v;
    return acc;
  }, {});
  const requestBody = {
    model: modelFull,
    [exConfig.bodyKey]: input,
    ...exConfig.extraBody,
    ...extraBodyFromFields,
    ...(supportsEdit && effectiveRefImage ? { image: effectiveRefImage } : {}),
    ...(supportsMask && effectiveMaskImage ? { mask_image: effectiveMaskImage } : {}),
  };

  // Streaming supported for codex image (Plus/Pro accounts) — disabled when binary output requested
  const wantBinary = kind === "image" && imageOutputFormat === "binary";
  const useStreaming = kind === "image" && providerId === "codex" && !wantBinary;
  const apiPathWithQuery = `${apiPath}${wantBinary ? "?response_format=binary" : ""}`;
  const headersPreview = `-H "Content-Type: application/json" \\\n  -H "Authorization: Bearer ${apiKey || "YOUR_KEY"}"${pinnedConnectionId ? ` \\\n  -H "x-connection-id: ${pinnedConnectionId}"` : ""}${useStreaming ? ` \\\n  -H "Accept: text/event-stream"` : ""}`;
  const curlSnippet = `curl -X ${kindConfig.endpoint.method} ${endpoint}${apiPathWithQuery} \\
  ${headersPreview.replace(/\\\n  /g, "\\\n  ")} \\
  -d '${JSON.stringify(requestBody)}'${wantBinary ? " \\\n  --output image.webp" : ""}`;

  const handleRun = async () => {
    if (!input.trim() || !modelFull) return;
    setRunning(true);
    setError("");
    setResult(null);
    setProgress(null);
    setPartialImage(null);
    if (binaryImageUrl) { try { URL.revokeObjectURL(binaryImageUrl); } catch {} setBinaryImageUrl(""); }
    const start = Date.now();
    try {
      const headers = { "Content-Type": "application/json" };
      if (apiKey) headers["Authorization"] = `Bearer ${apiKey}`;
      if (pinnedConnectionId) headers["x-connection-id"] = pinnedConnectionId;
      if (useStreaming) headers["Accept"] = "text/event-stream";
      const body = { ...requestBody, model: modelFull };
      const res = await fetch(`/api${apiPathWithQuery}`, {
        method: kindConfig.endpoint.method,
        headers,
        body: JSON.stringify(body),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        setError(data?.error?.message || data?.error || `HTTP ${res.status}`);
        return;
      }
      const ctype = res.headers.get("content-type") || "";
      // Binary image response — convert to blob URL
      if (ctype.startsWith("image/")) {
        const blob = await res.blob();
        const objUrl = URL.createObjectURL(blob);
        setBinaryImageUrl(objUrl);
        setResult({ data: { binary: true, mime: ctype, size: blob.size }, latencyMs: Date.now() - start });
        return;
      }
      const isSse = ctype.includes("text/event-stream");
      if (isSse && res.body) {
        // Parse SSE: progress / partial_image / done / error
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buf = "";
        let finalData = null;
        let streamErr = null;
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buf += decoder.decode(value, { stream: true });
          let sep;
          while ((sep = buf.indexOf("\n\n")) !== -1) {
            const block = buf.slice(0, sep);
            buf = buf.slice(sep + 2);
            let evt = null, dataStr = "";
            for (const line of block.split("\n")) {
              if (line.startsWith("event:")) evt = line.slice(6).trim();
              else if (line.startsWith("data:")) dataStr += line.slice(5).trim();
            }
            if (!evt) continue;
            try {
              const payload = dataStr ? JSON.parse(dataStr) : {};
              if (evt === "progress") setProgress(payload);
              else if (evt === "partial_image") setPartialImage(payload);
              else if (evt === "done") finalData = payload;
              else if (evt === "error") streamErr = payload?.message || "Stream error";
            } catch {}
          }
        }
        const latencyMs = Date.now() - start;
        if (streamErr) { setError(streamErr); return; }
        if (finalData) setResult({ data: finalData, latencyMs });
      } else {
        const data = await res.json();
        const latencyMs = Date.now() - start;
        setResult({ data, latencyMs });
      }
    } catch (e) {
      setError(e.message || "Network error");
    } finally {
      setRunning(false);
    }
  };

  // Mask large b64_json strings in JSON view to keep it readable
  const maskB64 = (obj) => {
    if (!obj || typeof obj !== "object") return obj;
    if (Array.isArray(obj)) return obj.map(maskB64);
    const out = {};
    for (const [k, v] of Object.entries(obj)) {
      out[k] = (k === "b64_json" && typeof v === "string" && v.length > 100)
        ? `<${v.length} chars base64>`
        : maskB64(v);
    }
    return out;
  };
  const resultJson = result ? JSON.stringify(maskB64(result.data), null, 2) : "";

  return (
    <Card>
      <Typography component="h2" sx={{ fontSize: 18, fontWeight: 600, mb: 2 }}>Example</Typography>
      <Box sx={{ display: "flex", flexDirection: "column", gap: 1.25 }}>
        {/* Model selector — dropdown if presets exist, else manual input for media kinds */}
        {kindModels.length > 0 ? (
          <Row label="Model">
            <Box
              component="select"
              value={selectedModel}
              onChange={(e) => setSelectedModel(e.target.value)}
              sx={inputSx}
            >
              {kindModels.map((m) => (
                <option key={m.id} value={m.id}>{m.name || m.id}</option>
              ))}
            </Box>
          </Row>
        ) : allowManualModel ? (
          <Row label="Model">
            <Box
              component="input"
              value={selectedModel}
              onChange={(e) => setSelectedModel(e.target.value)}
              placeholder="Enter model id (provider-specific)"
              sx={{ ...inputSx, fontFamily: "monospace" }}
            />
          </Row>
        ) : null}

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

        {/* Connection picker - only show when 2+ connections (or any with email) */}
        {connections.length > 0 && (
          <Row label="Connection">
            <Box
              component="select"
              value={pinnedConnectionId}
              onChange={(e) => setPinnedConnectionId(e.target.value)}
              sx={inputSx}
            >
              <option value="">Auto (by priority)</option>
              {connections.map((c) => {
                const plan = c.providerSpecificData?.chatgptPlanType;
                const label = c.email || c.name || c.id.slice(0, 8);
                return (
                  <option key={c.id} value={c.id}>
                    {label}{plan ? ` [${plan}]` : ""}
                  </option>
                );
              })}
            </Box>
          </Row>
        )}

        {/* Input */}
        <Row label={exConfig.inputLabel}>
          <Box sx={{ position: "relative" }}>
            <Box
              component="input"
              value={input}
              onChange={(e) => setInput(e.target.value)}
              placeholder={exConfig.inputPlaceholder}
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

        {/* Reference image (only for edit-capable image models) */}
        {supportsEdit && (
          <Row label="Ref Image (URL)">
            <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
              <Box sx={{ position: "relative" }}>
                <Box
                  component="input"
                  value={refImage}
                  onChange={(e) => setRefImage(e.target.value)}
                  placeholder={imageEditDefaults.image || "https://example.com/source.webp"}
                  sx={{ ...inputSx, pr: 3 }}
                />
                {refImage && (
                  <Box
                    component="button"
                    type="button"
                    onClick={() => setRefImage("")}
                    sx={{ position: "absolute", right: 8, top: "50%", transform: "translateY(-50%)", color: "text.secondary", border: 0, background: "none", cursor: "pointer", transition: "color 150ms ease", "&:hover": { color: "primary.main" } }}
                  >
                    <MsIcon sx={{ fontSize: 14 }}>close</MsIcon>
                  </Box>
                )}
              </Box>
              {refImagePreviewSrc && (
                <Box
                  component="img"
                  src={refImagePreviewSrc}
                  alt="Reference"
                  onError={(e) => { e.currentTarget.style.display = "none"; }}
                  onLoad={(e) => { e.currentTarget.style.display = "block"; }}
                  loading="lazy"
                  decoding="async"
                  sx={{ maxHeight: 160, borderRadius: 2, border: 1, borderColor: "divider", objectFit: "contain", bgcolor: "action.hover" }}
                />
              )}
            </Box>
          </Row>
        )}

        {supportsMask && (
          <Row label="Mask (URL)">
            <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
              <Box sx={{ position: "relative" }}>
                <Box
                  component="input"
                  value={maskImage}
                  onChange={(e) => setMaskImage(e.target.value)}
                  placeholder={imageEditDefaults.mask_image || "https://example.com/mask.webp"}
                  sx={{ ...inputSx, pr: 3 }}
                />
                {maskImage && (
                  <Box
                    component="button"
                    type="button"
                    onClick={() => setMaskImage("")}
                    sx={{ position: "absolute", right: 8, top: "50%", transform: "translateY(-50%)", color: "text.secondary", border: 0, background: "none", cursor: "pointer", transition: "color 150ms ease", "&:hover": { color: "primary.main" } }}
                  >
                    <MsIcon sx={{ fontSize: 14 }}>close</MsIcon>
                  </Box>
                )}
              </Box>
              {maskImagePreviewSrc && (
                <Box
                  component="img"
                  src={maskImagePreviewSrc}
                  alt="Mask"
                  onError={(e) => { e.currentTarget.style.display = "none"; }}
                  onLoad={(e) => { e.currentTarget.style.display = "block"; }}
                  loading="lazy"
                  decoding="async"
                  sx={{ maxHeight: 160, borderRadius: 2, border: 1, borderColor: "divider", objectFit: "contain", bgcolor: "action.hover" }}
                />
              )}
            </Box>
          </Row>
        )}

        {/* Extra fields — for kinds without model concept (webSearch/webFetch), show all; otherwise filter by model.params */}
        {(exConfig.extraFields || [])
          .filter((f) => kindModels.length === 0 || (Array.isArray(selectedModelObj?.params) && selectedModelObj.params.includes(f.key)))
          .map((f) => (
          <Row key={f.key} label={f.label}>
            {f.type === "select" ? (
              <Box
                component="select"
                value={extraValues[f.key] ?? ""}
                onChange={(e) => setExtraValues((s) => ({ ...s, [f.key]: e.target.value }))}
                sx={inputSx}
              >
                {(f.options || []).map((opt) => (
                  <option key={opt} value={opt}>{opt === "" ? "(default)" : opt}</option>
                ))}
              </Box>
            ) : f.type === "text" ? (
              <Box
                component="input"
                type="text"
                value={extraValues[f.key] ?? ""}
                placeholder={f.placeholder}
                onChange={(e) => setExtraValues((s) => ({ ...s, [f.key]: e.target.value }))}
                sx={inputSx}
              />
            ) : (
              <Box
                component="input"
                type="number"
                value={extraValues[f.key] ?? ""}
                min={f.min}
                max={f.max}
                onChange={(e) => setExtraValues((s) => ({ ...s, [f.key]: e.target.value === "" ? "" : Number(e.target.value) }))}
                sx={inputSx}
              />
            )}
          </Row>
        ))}

        {/* Output Format toggle (image only) — last */}
        {kind === "image" && (
          <Row label="Output Format">
            <Box
              component="select"
              value={imageOutputFormat}
              onChange={(e) => setImageOutputFormat(e.target.value)}
              sx={inputSx}
            >
              <option value="json">JSON (Base64)</option>
              <option value="binary">Binary File</option>
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

        {/* Streaming progress */}
        {(running || progress) && useStreaming && (
          <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1, alignItems: { sm: "center" }, px: 1.5, py: 1, borderRadius: 2, bgcolor: "action.hover", border: 1, borderColor: "divider" }}>
            <MsIcon sx={{ fontSize: 16, color: "primary.main", ...(running ? { animation: "spin 1s linear infinite" } : {}) }}>
              {running ? "progress_activity" : "check_circle"}
            </MsIcon>
            <Box component="span" sx={{ fontSize: 12, color: "text.secondary" }}>
              {progress?.stage || "starting"}
              {!running && progress?.bytesReceived ? ` · ${(progress.bytesReceived / 1024).toFixed(1)} KB` : ""}
            </Box>
          </Box>
        )}

        {/* Partial image preview (codex stream) */}
        {partialImage?.b64_json && !result && (
          <Box>
            <Box component="span" sx={sectionLabelSx}>Partial preview</Box>
            <Box
              component="img"
              src={`data:image/png;base64,${partialImage.b64_json}`}
              alt="Partial"
              loading="lazy"
              decoding="async"
              sx={{ maxWidth: "100%", borderRadius: 2, border: 1, borderColor: "divider", mt: 0.75, opacity: 0.8 }}
            />
          </Box>
        )}

        {/* Error */}
        {error && <Typography sx={{ fontSize: 12, color: "error.main", wordBreak: "break-word" }}>{error}</Typography>}

        {/* Response */}
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
            {result ? resultJson : exConfig.defaultResponse}
          </Box>
          {kind === "image" && (binaryImageUrl || result?.data?.data?.[0]) && (
            <Box sx={{ mt: 1 }}>
              <Box sx={{ display: "flex", alignItems: "center", justifyContent: "flex-end", mb: 0.75 }}>
                <Box
                  component="a"
                  href={binaryImageUrl || (result?.data?.data?.[0]?.b64_json ? `data:image/png;base64,${result.data.data[0].b64_json}` : result?.data?.data?.[0]?.url || "")}
                  download="image.webp"
                  sx={copyButtonSx}
                >
                  <MsIcon sx={{ fontSize: 14 }}>download</MsIcon>
                  Download
                </Box>
              </Box>
              <Box
                component="img"
                src={binaryImageUrl || (result?.data?.data?.[0]?.b64_json ? `data:image/png;base64,${result.data.data[0].b64_json}` : result?.data?.data?.[0]?.url)}
                alt="Generated"
                loading="lazy"
                decoding="async"
                sx={{ maxWidth: "100%", borderRadius: 2, border: 1, borderColor: "divider" }}
              />
            </Box>
          )}
        </Box>
      </Box>
    </Card>
  );
}
