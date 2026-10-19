"use client";

import { useState, useEffect } from "react";
import { Card } from "@/shared/components";
import { AI_PROVIDERS, getProviderAlias } from "@/shared/constants/providers";
import { getModelsByProviderId, getModelKind } from "@/shared/constants/models";
import { useCopyToClipboard } from "@/shared/hooks/useCopyToClipboard";
import { TTS_PROVIDER_CONFIG } from "@/shared/constants/ttsProviders";
import { getTtsVoicesForModel } from "open-sse/config/ttsModels.js";
import { GOOGLE_TTS_LANGUAGES } from "open-sse/config/googleTtsLanguages.js";
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

const DEFAULT_TTS_RESPONSE_EXAMPLE = `// Audio will appear here after running.
// Example JSON response (response_format=json):
{
  "format": "mp3",
  "audio": "//NExAANaAIIAUAAANNNNNNNN..." // base64 encoded MP3
}`;

export function TtsExampleCard({ providerId }) {
  const providerAlias = getProviderAlias(providerId);
  const config = TTS_PROVIDER_CONFIG[providerId] || TTS_PROVIDER_CONFIG["edge-tts"];

  // Voice state
  const [selectedVoice, setSelectedVoice]     = useState(config.defaultVoiceId || "");
  const [selectedVoiceName, setSelectedVoiceName] = useState("");
  const [voiceId, setVoiceId]               = useState(config.defaultVoiceId || ""); // editable voice id (elevenlabs/config providers)
  // Voices shown below Voice row after language selected
  const [countryVoices, setCountryVoices]     = useState([]);
  const [selectedLang, setSelectedLang]       = useState("");
  const [selectedModel, setSelectedModel]     = useState(() => {
    const cfgModels = AI_PROVIDERS[providerId]?.ttsConfig?.models;
    if (cfgModels?.length) return cfgModels[0].id;
    if (config.hasModelSelector && config.modelKey) {
      const models = getModelsByProviderId(config.modelKey);
      return models?.[0]?.id || "";
    }
    return "";
  });

  // Form state
  const [input, setInput]               = useState("Hello, this is a text to speech test.");
  const [style, setStyle]               = useState("");
  const [apiKey, setApiKey]             = useState("");
  const [useTunnel, setUseTunnel]       = useState(false);
  const [localEndpoint, setLocalEndpoint]   = useState("");
  const [tunnelEndpoint, setTunnelEndpoint] = useState("");
  const [responseFormat, setResponseFormat] = useState("mp3"); // mp3 | json
  const [audioUrl, setAudioUrl]         = useState("");
  const [jsonResponse, setJsonResponse] = useState(null); // Store JSON response
  const [running, setRunning]           = useState(false);
  const [error, setError]               = useState("");
  const [latency, setLatency]           = useState(null);
  const { copied: copiedCurl, copy: copyCurl } = useCopyToClipboard();

  // Country picker modal state
  const [modalOpen, setModalOpen]           = useState(false);
  const [languages, setLanguages]           = useState([]);
  const [modalLoading, setModalLoading]     = useState(false);
  const [modalSearch, setModalSearch]       = useState("");
  const [modalError, setModalError]         = useState("");
  const [byLang, setByLang]                 = useState({});
  // Language hint (e.g. Gemini): controls the spoken language without affecting voice selection
  const [languageHint, setLanguageHint]     = useState("");

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

    // Pre-select default voice based on provider config
    if (config.voiceSource === "hardcoded") {
      const defaultModel = config.hasModelSelector && config.modelKey
        ? (getModelsByProviderId(config.modelKey)?.[0]?.id || "")
        : "";
      // Use per-model voices if available, else flat list
      const voices = (config.voicesPerModel && defaultModel)
        ? (getTtsVoicesForModel(providerId, defaultModel) || [])
        : getModelsByProviderId(config.voiceKey || providerId).filter((m) => getModelKind(m) === "tts");
      if (voices.length) {
        if (config.hasBrowseButton) {
          // Google TTS: pre-select "en" (English) as default, show as single voice chip
          const defaultVoice = voices.find((v) => v.id === "en") || voices[0];
          setSelectedLang(defaultVoice.id);
          setSelectedVoice(defaultVoice.id);
          setSelectedVoiceName(defaultVoice.name);
          setCountryVoices([{ id: defaultVoice.id, name: defaultVoice.name }]);
        } else {
          // OpenAI/OpenRouter: set voice chips directly (no language picker)
          setCountryVoices(voices);
          setSelectedVoice(voices[0].id);
          setSelectedVoiceName(voices[0].name || voices[0].id);
        }
      }
    }
    // api-language (edge-tts, local-device, elevenlabs): NO default load, wait for user to pick language
    // config (nvidia, hyperbolic, deepgram, huggingface, cartesia, playht, coqui, tortoise, inworld, qwen):
    // use ttsConfig.models for model selector; voice is empty by default (backend uses provider default)
  }, [providerId]);

  // Update voices when model changes (voicesPerModel providers)
  useEffect(() => {
    if (!config.voicesPerModel || !selectedModel) return;
    const voices = getTtsVoicesForModel(providerId, selectedModel) || [];
    // eslint-disable-next-line react-hooks/set-state-in-effect -- derived state from props (model selection); recomputed when model changes.
    setCountryVoices(voices);
    if (voices.length) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- derived state from props.
      setSelectedVoice(voices[0].id);
      // eslint-disable-next-line react-hooks/set-state-in-effect -- derived state from props.
      setSelectedVoiceName(voices[0].name || voices[0].id);
    }
  }, [selectedModel]);

  // Open modal — load language list
  const openModal = async () => {
    setModalOpen(true);
    setModalSearch("");
    setModalError("");
    if (languages.length) return; // already loaded
    setModalLoading(true);
    try {
      if (config.voiceSource === "hardcoded") {
        // Build languages/byLang from static providerModels data
        const voiceKey = config.voiceKey || providerId;
        const voices = getModelsByProviderId(voiceKey).filter((m) => getModelKind(m) === "tts");
        const byLangMap = {};
        for (const v of voices) {
          if (!byLangMap[v.id]) byLangMap[v.id] = { code: v.id, name: v.name, voices: [{ id: v.id, name: v.name }] };
        }
        setByLang(byLangMap);
        setLanguages(Object.values(byLangMap).sort((a, b) => a.name.localeCompare(b.name)));
      } else {
        // Use provider-specific apiEndpoint if available, else default to edge-tts voices API
        const url = config.apiEndpoint
          ? config.apiEndpoint
          : `/api/media-providers/tts/voices?provider=${providerId === "local-device" ? "local-device" : "edge-tts"}`;
        const r = await fetch(url);
        const d = await r.json();
        if (d.error) { setModalError(d.error); return; }
        setLanguages(d.languages || []);
        setByLang(d.byLang || {});
      }
    } catch (e) {
      setModalError(e.message);
    } finally {
      setModalLoading(false);
    }
  };

  // Click language → close modal → show voices below
  const handlePickLanguage = (lang) => {
    setModalOpen(false);
    setSelectedLang(lang.code);
    const voices = byLang[lang.code]?.voices || [];
    setCountryVoices(voices);
    // Auto-select first voice
    if (voices.length) {
      setSelectedVoice(voices[0].id);
      setSelectedVoiceName(voices[0].name);
      if (config.hasVoiceIdInput) setVoiceId(voices[0].id);
    }
  };

  const filteredLanguages = modalSearch
    ? languages.filter((c) =>
        c.name.toLowerCase().includes(modalSearch.toLowerCase()) ||
        c.code.toLowerCase().includes(modalSearch.toLowerCase())
      )
    : languages;

  const endpoint = useTunnel ? tunnelEndpoint : localEndpoint;
  // For ElevenLabs/config-driven: prefer manual voiceId (if any), else fall back to selectedVoice
  const activeVoiceId = config.hasVoiceIdInput ? (voiceId || selectedVoice) : selectedVoice;
  const modelFull = (() => {
    if (config.hasModelSelector && selectedModel && activeVoiceId) return `${providerAlias}/${selectedModel}/${activeVoiceId}`;
    if (config.hasModelSelector && selectedModel) return `${providerAlias}/${selectedModel}`;
    if (activeVoiceId) return `${providerAlias}/${activeVoiceId}`;
    return "";
  })();

  const ttsBody = (() => {
    const b = { model: modelFull, input };
    if (config.hasLanguageHint && languageHint) b.language = languageHint;
    if (config.hasStyleInput && style.trim()) b.style = style.trim().slice(0, 1000);
    return b;
  })();
  const curlSnippet = `curl -X POST ${endpoint}/v1/audio/speech${responseFormat === "json" ? "?response_format=json" : ""} \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${apiKey || "YOUR_KEY"}" \\
  -d '${JSON.stringify(ttsBody)}' \\
  ${responseFormat === "json" ? "" : `--output speech.${providerId === "xiaomi-mimo" ? "wav" : "mp3"}`}`;

  const handleRun = async () => {
    if (!input.trim() || !modelFull) return;
    setRunning(true);
    setError("");
    setAudioUrl("");
    setJsonResponse(null);
    const start = Date.now();
    try {
      const headers = { "Content-Type": "application/json" };
      if (apiKey) headers["Authorization"] = `Bearer ${apiKey}`;
      const url = `/api/v1/audio/speech${responseFormat === "json" ? "?response_format=json" : ""}`;
      const res = await fetch(url, {
        method: "POST",
        headers,
        body: JSON.stringify({ ...ttsBody, input: input.trim() }),
      });
      setLatency(Date.now() - start);
      if (!res.ok) {
        const d = await res.json().catch(() => ({}));
        setError(d?.error?.message || d?.error || `HTTP ${res.status}`);
        return;
      }
      
      if (responseFormat === "json") {
        const data = await res.json();
        setJsonResponse(data); // Store full JSON response
        const format = data.format || "mp3";
        const audioBlob = await fetch(`data:audio/${format};base64,${data.audio}`).then(r => r.blob());
        setAudioUrl(URL.createObjectURL(audioBlob));
      } else {
        const blob = await res.blob();
        setAudioUrl(URL.createObjectURL(blob));
      }
    } catch (e) {
      setError(e.message || "Network error");
    } finally {
      setRunning(false);
    }
  };

  return (
    <>
      <Card>
        <Typography component="h2" sx={{ fontSize: 18, fontWeight: 600, mb: 2 }}>Example</Typography>

        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.25 }}>
          {/* Endpoint + API Key as read-only text */}
          <Row label="Endpoint">
            <Box sx={{ display: "flex", width: { xs: "100%", sm: "auto" }, flexDirection: { xs: "column", sm: "row" }, gap: 1, alignItems: { sm: "center" } }}>
              <Box component="span" sx={{ ...readOnlySx, minWidth: 0, flex: 1 }}>
                {endpoint}/v1/audio/speech
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
          <Row label="API Key">
            <Box component="span" sx={{ px: 1.5, py: 0.75, fontSize: 14, fontFamily: "monospace", color: "text.primary", bgcolor: "action.hover", borderRadius: 2, display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
              {apiKey
                ? `${apiKey.slice(0, 8)}${"•".repeat(Math.min(20, Math.max(0, apiKey.length - 8)))}`
                : <Box component="span" sx={{ color: "text.secondary", fontStyle: "italic" }}>No key configured</Box>}
            </Box>
          </Row>

          {/* Model selector — prefer PROVIDER_MODELS[kind=tts], else providerModels via modelKey */}
          {config.hasModelSelector && (config.modelKey || getModelsByProviderId(providerId).some(m => getModelKind(m) === "tts")) && (
            <Row label="Model">
              <Box
                component="select"
                value={selectedModel}
                onChange={(e) => setSelectedModel(e.target.value)}
                sx={inputSx}
              >
                {(() => {
                  const ttsModels = getModelsByProviderId(providerId).filter(m => getModelKind(m) === "tts");
                  return (ttsModels.length ? ttsModels : getModelsByProviderId(config.modelKey) || []).map((m) => (
                    <option key={m.id} value={m.id}>{m.name || m.id}</option>
                  ));
                })()}
              </Box>
            </Row>
          )}

          {/* Language hint dropdown (Gemini) — sends body.language to guide pronunciation */}
          {config.hasLanguageHint && (
            <Row label="Language">
              <Box
                component="select"
                value={languageHint}
                onChange={(e) => setLanguageHint(e.target.value)}
                sx={inputSx}
              >
                <option value="">Auto-detect</option>
                {GOOGLE_TTS_LANGUAGES.map((l) => (
                  <option key={l.id} value={l.name}>{l.name}</option>
                ))}
              </Box>
            </Row>
          )}

          {/* Language row + Browse button (edge-tts, local-device, elevenlabs) */}
          {config.hasBrowseButton && (
            <Row label="Language">
              <Box sx={{ display: "flex", width: { xs: "100%", sm: "auto" }, flexDirection: { xs: "column", sm: "row" }, gap: 1, alignItems: { sm: "center" } }}>
                <Box
                  component="button"
                  onClick={openModal}
                  sx={{
                    width: { xs: "100%", sm: "auto" },
                    minWidth: 0,
                    flex: 1,
                    px: 1.5,
                    py: 0.75,
                    fontSize: 14,
                    border: 1,
                    borderColor: "divider",
                    borderRadius: 2,
                    bgcolor: "background.default",
                    fontFamily: "monospace",
                    textAlign: "left",
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                    cursor: "pointer",
                    transition: "border-color 150ms ease",
                    "&:hover": { borderColor: "primary.main" },
                  }}
                >
                  {selectedLang
                    ? <Box component="span" sx={{ color: "text.primary" }}>{languages.find((l) => l.code === selectedLang)?.name || selectedLang}</Box>
                    : <Box component="span" sx={{ color: "text.secondary" }}>No language selected</Box>}
                </Box>
                <Box
                  component="button"
                  onClick={openModal}
                  sx={{
                    display: "flex",
                    width: { xs: "100%", sm: "auto" },
                    alignItems: "center",
                    justifyContent: "center",
                    gap: 0.5,
                    fontSize: 12,
                    px: 1.25,
                    py: 0.75,
                    borderRadius: 2,
                    border: 1,
                    borderColor: "divider",
                    color: "text.secondary",
                    bgcolor: "transparent",
                    cursor: "pointer",
                    flexShrink: { sm: 0 },
                    transition: "color 150ms ease, border-color 150ms ease",
                    "&:hover": { color: "primary.main", borderColor: "primary.main" },
                  }}
                >
                  <MsIcon sx={{ fontSize: 14 }}>language</MsIcon>
                  Select language
                </Box>
              </Box>
            </Row>
          )}

          {/* Voice chips — shown after language picked (edge-tts, local-device) or always (OpenAI/ElevenLabs) */}
          {countryVoices.length > 0 && (
            <Row label="Voice">
              <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.75 }}>
                {countryVoices.map((v) => {
                  const active = selectedVoice === v.id;
                  return (
                    <Box
                      key={v.id}
                      component="button"
                      onClick={() => {
                        setSelectedVoice(v.id);
                        setSelectedVoiceName(v.name);
                        if (config.hasVoiceIdInput) setVoiceId(v.id);
                      }}
                      sx={{
                        px: 1.25,
                        py: 0.5,
                        borderRadius: 999,
                        fontSize: 12,
                        cursor: "pointer",
                        transition: "color 150ms ease, border-color 150ms ease",
                        bgcolor: active ? "action.selected" : "transparent",
                        border: 1,
                        borderColor: active ? "primary.main" : "divider",
                        color: active ? "primary.main" : "text.secondary",
                        fontWeight: active ? 500 : 400,
                        "&:hover": { color: "primary.main", borderColor: "primary.main" },
                      }}
                    >
                      {v.name}{v.gender ? ` · ${v.gender[0].toUpperCase()}` : ""}
                      {v.free_users_allowed === true && (
                        <Box component="span" sx={{ ml: 0.75, px: 0.5, py: 0.25, fontSize: 9, fontWeight: 600, borderRadius: 0.5, bgcolor: "rgba(34,197,94,0.15)", color: "#16a34a", border: "1px solid rgba(34,197,94,0.2)" }}>Free</Box>
                      )}
                      {v.free_users_allowed === false && (
                        <Box component="span" sx={{ ml: 0.75, px: 0.5, py: 0.25, fontSize: 9, fontWeight: 600, borderRadius: 0.5, bgcolor: "rgba(245,158,11,0.15)", color: "#d97706", border: "1px solid rgba(245,158,11,0.2)" }}>Paid</Box>
                      )}
                    </Box>
                  );
                })}
              </Box>
            </Row>
          )}

          {/* Voice ID input (ElevenLabs) — manual entry or auto-fill from chip */}
          {config.hasVoiceIdInput && (
            <Row label="Voice ID">
              <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
                <Box sx={{ position: "relative" }}>
                  <Box
                    component="input"
                    value={voiceId}
                    onChange={(e) => {
                      setVoiceId(e.target.value);
                      setSelectedVoice(e.target.value);
                    }}
                    placeholder="e.g. CwhRBWXzGAHq8TQ4Fs17"
                    sx={{ ...inputSx, pr: 3, fontFamily: "monospace" }}
                  />
                  {voiceId && (
                    <Box
                      component="button"
                      type="button"
                      onClick={() => { setVoiceId(""); setSelectedVoice(""); }}
                      sx={{ position: "absolute", right: 8, top: "50%", transform: "translateY(-50%)", color: "text.secondary", border: 0, background: "none", cursor: "pointer", transition: "color 150ms ease", "&:hover": { color: "primary.main" } }}
                    >
                      <MsIcon sx={{ fontSize: 14 }}>close</MsIcon>
                    </Box>
                  )}
                </Box>
              </Box>
            </Row>
          )}

          {/* Google TTS: Language dropdown */}
          {config.hasLanguageDropdown && (
            <Row label="Language">
              <Box
                component="select"
                value={selectedVoice}
                onChange={(e) => {
                  const m = getModelsByProviderId(providerId).filter((m) => getModelKind(m) === "tts").find((m) => m.id === e.target.value);
                  setSelectedVoice(e.target.value);
                  setSelectedVoiceName(m?.name || e.target.value);
                }}
                sx={inputSx}
              >
                {getModelsByProviderId(providerId).filter((m) => getModelKind(m) === "tts").map((m) => (
                  <option key={m.id} value={m.id}>{m.name || m.id}</option>
                ))}
              </Box>
            </Row>
          )}

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

          {config.hasStyleInput && (
            <Row label="Style">
              <Box
                component="textarea"
                value={style}
                onChange={(e) => setStyle(e.target.value)}
                maxLength={1000}
                placeholder="e.g. a warm, gentle voice, speaking slowly"
                rows={2}
                sx={{ ...inputSx, resize: "none" }}
              />
            </Row>
          )}

          {/* Output Format */}
          <Row label="Output Format">
            <Box
              component="select"
              value={responseFormat}
              onChange={(e) => setResponseFormat(e.target.value)}
              sx={inputSx}
            >
              <option value="mp3">MP3 (Binary)</option>
              <option value="json">JSON (Base64)</option>
            </Box>
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
                  {running ? "Generating..." : "Run"}
                </Box>
              </Box>
            </Box>
            <Box component="pre" sx={codeBlockSx}>{curlSnippet}</Box>
          </Box>

          {error && <Typography sx={{ fontSize: 12, color: "error.main", wordBreak: "break-word" }}>{error}</Typography>}

          {/* Audio player */}
          {audioUrl ? (
            <Box>
              <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1, alignItems: { sm: "center" }, justifyContent: { sm: "space-between" }, mb: 0.75 }}>
                <Box component="span" sx={sectionLabelSx}>
                  Response {latency && <Box component="span" sx={{ fontWeight: 400, textTransform: "none" }}>&#9889; {latency}ms</Box>}
                </Box>
                <Box component="a" href={audioUrl} download={`speech.${jsonResponse?.format || responseFormat}`} sx={copyButtonSx}>
                  <MsIcon sx={{ fontSize: 14 }}>download</MsIcon>
                  Download
                </Box>
              </Box>
              <Box component="audio" controls src={audioUrl} sx={{ width: "100%" }} />
              
              {/* JSON Response (if format is json) */}
              {jsonResponse && (
                <Box sx={{ mt: 1.5 }}>
                  <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1, alignItems: { sm: "center" }, justifyContent: { sm: "space-between" }, mb: 0.75 }}>
                    <Box component="span" sx={sectionLabelSx}>JSON Response</Box>
                  </Box>
                  <Box component="pre" sx={codeBlockSx}>
                    {JSON.stringify({
                      format: jsonResponse.format,
                      audio: jsonResponse.audio ? `${jsonResponse.audio.substring(0, 100)}...` : ""
                    }, null, 2)}
                  </Box>
                </Box>
              )}
            </Box>
          ) : (
            <Box>
            <Box component="span" sx={sectionLabelSx}>Response</Box>
            <Box component="pre" sx={{ ...codeBlockSx, mt: 0.75, opacity: 0.5 }}>{DEFAULT_TTS_RESPONSE_EXAMPLE}</Box>
          </Box>
          )}
        </Box>
      </Card>

      {/* Country Picker Modal */}
      {modalOpen && (
        <Box
          sx={{ position: "fixed", inset: 0, zIndex: 50, display: "flex", alignItems: { xs: "flex-end", sm: "center" }, justifyContent: "center" }}
          style={{ backgroundColor: "rgba(0,0,0,0.6)", backdropFilter: "blur(2px)" }}
          onClick={() => setModalOpen(false)}
        >
          <Box
            sx={{ border: 1, borderColor: "divider", borderRadius: 3, boxShadow: 24, width: "100%", maxWidth: 448, mx: 2, display: "flex", flexDirection: "column", maxHeight: "80vh", overflow: "hidden" }}
            style={{ backgroundColor: "var(--color-bg)", isolation: "isolate" }}
            onClick={(e) => e.stopPropagation()}
          >
            {/* Header */}
            <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", px: 2, py: 1.5, borderBottom: 1, borderColor: "divider", flexShrink: 0 }}>
              <Box component="h3" sx={{ fontSize: 14, fontWeight: 600, m: 0 }}>Select Language</Box>
              <Box component="button" onClick={() => setModalOpen(false)} sx={{ color: "text.secondary", border: 0, background: "none", cursor: "pointer", transition: "color 150ms ease", "&:hover": { color: "primary.main" } }}>
                <MsIcon sx={{ fontSize: 20 }}>close</MsIcon>
              </Box>
            </Box>

            {/* Search */}
            <Box sx={{ px: 2, py: 1.25, borderBottom: 1, borderColor: "divider", flexShrink: 0 }}>
              <Box
                component="input"
                autoFocus
                value={modalSearch}
                onChange={(e) => setModalSearch(e.target.value)}
                placeholder="Search language..."
                sx={inputSx}
              />
            </Box>

            {/* Language list */}
            <Box sx={{ overflowY: "auto", flex: 1, p: 1 }}>
              {modalError && <Box component="p" sx={{ fontSize: 12, color: "error.main", px: 1, py: 0.5, m: 0 }}>{modalError}</Box>}
              {modalLoading ? (
                <Box component="p" sx={{ fontSize: 12, color: "text.secondary", px: 1, py: 1.5, m: 0 }}>Loading...</Box>
              ) : (
                <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25 }}>
                  {filteredLanguages.map((c) => {
                    const active = selectedLang === c.code;
                    return (
                      <Box
                        key={c.code}
                        component="button"
                        onClick={() => handlePickLanguage(c)}
                        sx={{
                          display: "flex",
                          alignItems: "center",
                          justifyContent: "space-between",
                          width: "100%",
                          px: 1.5,
                          py: 1,
                          borderRadius: 2,
                          textAlign: "left",
                          cursor: "pointer",
                          border: 0,
                          bgcolor: active ? "action.selected" : "transparent",
                          color: active ? "primary.main" : "text.primary",
                          transition: "background-color 150ms ease",
                          "&:hover": { bgcolor: "action.hover" },
                        }}
                      >
                        <Box component="span" sx={{ fontSize: 14 }}>{c.name}</Box>
                        <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexShrink: 0 }}>
                          <Box component="span" sx={{ fontSize: 12, color: "text.secondary" }}>{c.voices.length} voices</Box>
                          {active && (
                            <MsIcon sx={{ fontSize: 16, color: "primary.main" }}>check</MsIcon>
                          )}
                        </Box>
                      </Box>
                    );
                  })}
                  {filteredLanguages.length === 0 && (
                    <Box component="p" sx={{ fontSize: 12, color: "text.secondary", px: 1, py: 1.5, m: 0 }}>No languages found.</Box>
                  )}
                </Box>
              )}
            </Box>
          </Box>
        </Box>
      )}
    </>
  );
}
