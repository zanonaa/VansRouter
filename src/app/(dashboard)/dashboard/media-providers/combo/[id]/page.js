"use client";

import { useParams, notFound, useRouter } from "next/navigation";
import { useEffect, useState, useCallback } from "react";
import Link from "next/link";
import Image from "next/image";
import { Card, Button, Input, Toggle, ModelSelectModal } from "@/shared/components";
import ProviderIcon from "@/shared/components/ProviderIcon";
import { AI_PROVIDERS, MEDIA_PROVIDER_KINDS } from "@/shared/constants/providers";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import { alpha } from "@mui/material/styles";

function MsIcon({ children, sx, ...props }) {
  return (
    <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18, ...sx }} {...props}>
      {children}
    </Icon>
  );
}

const codeBlockSx = {
  fontSize: 12,
  fontFamily: "monospace",
  bgcolor: "action.hover",
  p: 1.5,
  borderRadius: 2,
  overflowX: "auto",
  whiteSpace: "pre-wrap",
  wordBreak: "break-all",
  m: 0,
};

const iconButtonSx = (disabled) => ({
  p: 0.5,
  borderRadius: 1,
  border: 0,
  background: "none",
  cursor: disabled ? "not-allowed" : "pointer",
  color: disabled ? "text.disabled" : "text.secondary",
  transition: "color 150ms ease, background-color 150ms ease",
  "&:hover": disabled ? {} : { color: "primary.main", bgcolor: "action.hover" },
});

// Parse "providerId/model" or just "providerId" → { providerId, model }
function parseModelEntry(entry) {
  if (typeof entry !== "string") return { providerId: "", model: "" };
  const idx = entry.indexOf("/");
  if (idx < 0) return { providerId: entry, model: "" };
  return { providerId: entry.slice(0, idx), model: entry.slice(idx + 1) };
}

const VALID_NAME_REGEX = /^[a-zA-Z0-9_.\-]+$/;

const KIND_LABELS = {
  webSearch: "Web Search",
  webFetch: "Web Fetch",
  image: "Text to Image",
  tts: "Text To Speech",
};

const EXAMPLE_PATHS = {
  webSearch: "/v1/search",
  webFetch: "/v1/web/fetch",
  image: "/v1/images/generations",
  tts: "/v1/audio/speech",
};

const EXAMPLE_BODIES = {
  webSearch: (n) => ({ model: n, query: "What is the latest news about AI?", search_type: "web", max_results: 5 }),
  webFetch: (n) => ({ model: n, url: "https://example.com", format: "markdown" }),
  image: (n) => ({ model: n, prompt: "A cute cat playing piano", n: 1, size: "1024x1024" }),
  tts: (n) => ({ model: n, input: "Hello, this is a test.", voice: "alloy" }),
};

// Map combo.kind → listing route to go back to
function getListingHref(kind) {
  if (kind === "webSearch" || kind === "webFetch") return "/dashboard/media-providers/web";
  return `/dashboard/media-providers/${kind}`;
}

function ComboProvidersCard({ providers, onAdd, onMove, onRemove }) {
  return (
    <>
      {/* Providers Card */}
      <Card>
        <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1.5, alignItems: { sm: "center" }, justifyContent: { sm: "space-between" }, mb: 1.5 }}>
          <Box>
            <Typography component="h2" sx={{ fontSize: 18, fontWeight: 600 }}>Providers</Typography>
            <Typography sx={{ fontSize: 12, color: "text.secondary" }}>Tried in order (top-down) or rotated when round-robin is on.</Typography>
          </Box>
          <Button size="sm" icon="add" onClick={onAdd}>Add Provider</Button>
        </Box>
        {providers.length === 0 ? (
          <Box sx={{ textAlign: "center", py: 3, border: "1px dashed", borderColor: "divider", borderRadius: 2, color: "text.secondary", fontSize: 14 }}>
            No providers yet.
          </Box>
        ) : (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            {providers.map((entry, idx) => {
              const { providerId, model } = parseModelEntry(entry);
              const p = AI_PROVIDERS[providerId];
              const first = idx === 0;
              const last = idx === providers.length - 1;
              return (
                <Box key={`${entry}-${idx}`} sx={{ display: "flex", alignItems: "center", gap: 1.5, p: 1, borderRadius: 2, bgcolor: (theme) => alpha(theme.palette.text.primary, 0.02) }}>
                  <Box component="span" sx={{ fontSize: 12, color: "text.secondary", width: 20, textAlign: "center" }}>{idx + 1}</Box>
                  <Box sx={{ flexShrink: 0, "& img": { objectFit: "contain", borderRadius: 0.5 } }}>
                    <ProviderIcon
                      src={`/providers/${providerId}.webp`}
                      alt={p?.name || providerId}
                      size={24}
                      fallbackText={p?.textIcon || providerId.slice(0, 2).toUpperCase()}
                      fallbackColor={p?.color}
                    />
                  </Box>
                  <Box sx={{ minWidth: 0, flex: 1 }}>
                    <Box sx={{ fontSize: 14, fontWeight: 500, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{p?.name || providerId}</Box>
                    {model && <Box component="code" sx={{ fontSize: 10, color: "text.secondary", fontFamily: "monospace", display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{model}</Box>}
                  </Box>
                  <Box sx={{ display: "flex", alignItems: "center", gap: 0.25 }}>
                  <Box component="button" type="button" onClick={() => onMove(idx, -1)} disabled={first} sx={iconButtonSx(first)} title="Move up">
                    <MsIcon sx={{ fontSize: 16 }}>arrow_upward</MsIcon>
                  </Box>
                  <Box component="button" type="button" onClick={() => onMove(idx, 1)} disabled={last} sx={iconButtonSx(last)} title="Move down">
                    <MsIcon sx={{ fontSize: 16 }}>arrow_downward</MsIcon>
                  </Box>
                  <Box
                    component="button"
                    type="button"
                    onClick={() => onRemove(idx)}
                    title="Remove"
                    sx={{
                      p: 0.5,
                      borderRadius: 1,
                      border: 0,
                      background: "none",
                      cursor: "pointer",
                      color: "text.secondary",
                      transition: "color 150ms ease, background-color 150ms ease",
                      "&:hover": { color: "error.main", bgcolor: (theme) => alpha(theme.palette.error.main, 0.1) },
                    }}
                  >
                    <MsIcon sx={{ fontSize: 16 }}>close</MsIcon>
                  </Box>
                  </Box>
                </Box>
              );
            })}
          </Box>
        )}
      </Card>
    </>
  );
}


// Mask large b64_json strings to keep JSON view readable
function maskB64(obj) {
  if (!obj || typeof obj !== "object") return obj;
  if (Array.isArray(obj)) return obj.map(maskB64);
  const out = {};
  for (const [k, v] of Object.entries(obj)) {
    out[k] = (k === "b64_json" && typeof v === "string" && v.length > 100)
      ? `<${v.length} chars base64>`
      : maskB64(v);
  }
  return out;
}

export default function ComboDetailPage() {
  const { id } = useParams();
  const router = useRouter();
  const [combo, setCombo] = useState(null);
  const [loading, setLoading] = useState(true);
  const [name, setName] = useState("");
  const [nameError, setNameError] = useState("");
  const [providers, setProviders] = useState([]);
  const [roundRobin, setRoundRobin] = useState(false);
  const [showPicker, setShowPicker] = useState(false);
  const [logs, setLogs] = useState([]);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState(null);
  const [testError, setTestError] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [connections, setConnections] = useState([]);
  const [modelAliases, setModelAliases] = useState({});

  const fetchAll = useCallback(async () => {
    try {
      const [comboRes, settingsRes, logsRes, keysRes, connsRes, aliasesRes] = await Promise.all([
        fetch(`/api/combos/${id}`, { cache: "no-store" }),
        fetch("/api/settings", { cache: "no-store" }),
        fetch("/api/usage/logs", { cache: "no-store" }),
        fetch("/api/keys", { cache: "no-store" }),
        fetch("/api/providers", { cache: "no-store" }),
        fetch("/api/models/alias", { cache: "no-store" }),
      ]);
      if (aliasesRes.ok) setModelAliases((await aliasesRes.json()).aliases || {});
      if (keysRes.ok) {
        const k = await keysRes.json();
        setApiKey((k.keys || []).find((x) => x.isActive !== false)?.key || "");
      }
      if (connsRes.ok) setConnections((await connsRes.json()).connections || []);
      if (!comboRes.ok) { setCombo(null); setLoading(false); return; }
      const c = await comboRes.json();
      setCombo(c);
      setName(c.name);
      setProviders(c.models || []);
      const s = settingsRes.ok ? await settingsRes.json() : {};
      setRoundRobin(s.comboStrategies?.[c.name]?.fallbackStrategy === "round-robin");
      const allLogs = logsRes.ok ? await logsRes.json() : [];
      setLogs(allLogs.filter((l) => typeof l === "string" && l.includes(c.name)).slice(0, 50));
    } catch { /* noop */ }
    setLoading(false);
  }, [id]);

  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => { fetchAll(); }, [fetchAll]);

  const validateName = (v) => {
    if (!v.trim()) { setNameError("Name is required"); return false; }
    if (!VALID_NAME_REGEX.test(v)) { setNameError("Only letters, numbers, -, _ and ."); return false; }
    setNameError("");
    return true;
  };

  const saveCombo = async (patch) => {
    const res = await fetch(`/api/combos/${id}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(patch),
    });
    if (!res.ok) { const err = await res.json(); alert(err.error || "Failed to save"); return false; }
    return true;
  };

  const handleSaveName = async () => {
    if (!validateName(name)) return;
    if (name === combo.name) return;
    const ok = await saveCombo({ name });
    if (ok) await fetchAll();
  };
  const handleAddModel = async (model) => {
    const value = model?.value || model;
    if (!value || providers.includes(value)) return;
    const next = [...providers, value];
    setProviders(next);
    await saveCombo({ models: next });
  };
  const handleDeselectModel = async (model) => {
    const value = model?.value || model;
    if (!value || !providers.includes(value)) return;
    const next = providers.filter((p) => p !== value);
    setProviders(next);
    await saveCombo({ models: next });
  };
  const handleRemoveProvider = async (idx) => {
    const next = providers.filter((_, i) => i !== idx);
    setProviders(next);
    await saveCombo({ models: next });
  };
  const handleMove = async (idx, dir) => {
    const next = [...providers];
    const swap = idx + dir;
    if (swap < 0 || swap >= next.length) return;
    [next[idx], next[swap]] = [next[swap], next[idx]];
    setProviders(next);
    await saveCombo({ models: next });
  };
  const handleToggleRoundRobin = async (enabled) => {
    setRoundRobin(enabled);
    const settingsRes = await fetch("/api/settings", { cache: "no-store" });
    const s = settingsRes.ok ? await settingsRes.json() : {};
    const updated = { ...(s.comboStrategies || {}) };
    if (enabled) updated[combo.name] = { fallbackStrategy: "round-robin" };
    else delete updated[combo.name];
    await fetch("/api/settings", {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ comboStrategies: updated }),
    });
  };
  const handleDelete = async () => {
    if (!confirm(`Delete combo "${combo.name}"?`)) return;
    const res = await fetch(`/api/combos/${id}`, { method: "DELETE" });
    if (res.ok) router.push(getListingHref(combo.kind));
  };
  const handleTest = async () => {
    setTesting(true);
    setTestResult(null);
    setTestError("");
    if (testResult?.audioUrl) { try { URL.revokeObjectURL(testResult.audioUrl); } catch {} }
    if (testResult?.imageUrl?.startsWith("blob:")) { try { URL.revokeObjectURL(testResult.imageUrl); } catch {} }
    const start = Date.now();
    try {
      const path = EXAMPLE_PATHS[combo.kind];
      const body = EXAMPLE_BODIES[combo.kind](combo.name);
      const headers = { "Content-Type": "application/json" };
      if (apiKey) headers["Authorization"] = `Bearer ${apiKey}`;
      const res = await fetch(`/api${path}`, { method: "POST", headers, body: JSON.stringify(body) });
      const latencyMs = Date.now() - start;
      if (!res.ok) {
        const d = await res.json().catch(() => ({}));
        setTestError(d?.error?.message || d?.error || `HTTP ${res.status}`);
        setTestResult({ json: JSON.stringify(d, null, 2), latencyMs });
        return;
      }
      const ctype = res.headers.get("content-type") || "";
      // Binary image
      if (ctype.startsWith("image/")) {
        const blob = await res.blob();
        setTestResult({ imageUrl: URL.createObjectURL(blob), latencyMs });
        return;
      }
      // Binary audio
      if (ctype.startsWith("audio/") || ctype === "application/octet-stream") {
        const blob = await res.blob();
        setTestResult({ audioUrl: URL.createObjectURL(blob), latencyMs });
        return;
      }
      // JSON — could be image (data[0].b64_json/url) or generic
      const data = await res.json();
      const first = data?.data?.[0];
      const imageUrl = first?.b64_json
        ? `data:image/png;base64,${first.b64_json}`
        : (first?.url || "");
      setTestResult({ json: JSON.stringify(maskB64(data), null, 2), imageUrl, latencyMs });
    } catch (e) {
      setTestError(e.message || "Network error");
    }
    setTesting(false);
  };
  if (loading) return <Box sx={{ color: "text.secondary", fontSize: 14 }}>Loading...</Box>;
  if (!combo) return notFound();
  const kindLabel = KIND_LABELS[combo.kind] || MEDIA_PROVIDER_KINDS.find((k) => k.id === combo.kind)?.label || "Combo";
  const examplePath = EXAMPLE_PATHS[combo.kind];
  const exampleBody = combo.kind && EXAMPLE_BODIES[combo.kind] ? EXAMPLE_BODIES[combo.kind](combo.name) : null;
  const curlExample = examplePath
    ? `curl -X POST http://localhost:20128${examplePath} \\\n  -H "Content-Type: application/json" \\\n  -H "Authorization: Bearer ${apiKey || "YOUR_KEY"}" \\\n  -d '${JSON.stringify(exampleBody)}'`
    : "";
  const backHref = getListingHref(combo.kind);
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      {/* Header */}
      <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 2, alignItems: { sm: "center" }, justifyContent: { sm: "space-between" } }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, minWidth: 0 }}>
          <Box component={Link} href={backHref} sx={{ color: "text.secondary", transition: "color 150ms ease", "&:hover": { color: "primary.main" } }}>
            <MsIcon>arrow_back</MsIcon>
          </Box>
          <Box sx={{ width: 40, height: 40, borderRadius: 2, bgcolor: "action.selected", display: "flex", alignItems: "center", justifyContent: "center" }}>
            <MsIcon sx={{ color: "primary.main" }}>layers</MsIcon>
          </Box>
          <Box sx={{ minWidth: 0 }}>
            <Typography sx={{ fontSize: 12, color: "text.secondary" }}>{kindLabel} Combo</Typography>
            <Box component="code" sx={{ fontSize: 18, fontWeight: 600, fontFamily: "monospace" }}>{combo.name}</Box>
          </Box>
        </Box>
        <Button
          variant="outline"
          icon="delete"
          onClick={handleDelete}
          sx={{
            textTransform: "none",
            fontWeight: 600,
            whiteSpace: "nowrap",
            height: 36,
            px: 2,
            fontSize: 13.5,
            color: "error.main",
            borderColor: (theme) => alpha(theme.palette.error.main, 0.3),
            "&:hover": { bgcolor: (theme) => alpha(theme.palette.error.main, 0.05), borderColor: (theme) => alpha(theme.palette.error.main, 0.5) },
          }}
        >
          Delete
        </Button>
      </Box>
      {/* Settings Card */}
      <Card>
        <Typography component="h2" sx={{ fontSize: 18, fontWeight: 600, mb: 1.5 }}>Settings</Typography>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Box>
            <Input label="Combo Name" value={name} onChange={(e) => { setName(e.target.value); validateName(e.target.value); }} onBlur={handleSaveName} error={nameError} />
            <Typography sx={{ fontSize: 10, color: "text.secondary", mt: 0.25 }}>Only letters, numbers, -, _ and .</Typography>
          </Box>
          <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
            <Box>
              <Typography sx={{ fontSize: 14, fontWeight: 500 }}>Round Robin</Typography>
              <Typography sx={{ fontSize: 12, color: "text.secondary" }}>Rotate providers across requests instead of strict fallback order.</Typography>
            </Box>
            <Toggle checked={roundRobin} onChange={handleToggleRoundRobin} />
          </Box>
        </Box>
      </Card>
      <ComboProvidersCard
        providers={providers}
        onAdd={() => setShowPicker(true)}
        onMove={handleMove}
        onRemove={handleRemoveProvider}
      />
      {/* Test Example Card */}
      {combo.kind && examplePath && (
        <Card>
          <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1.5, alignItems: { sm: "center" }, justifyContent: { sm: "space-between" }, mb: 1.5 }}>
            <Typography component="h2" sx={{ fontSize: 18, fontWeight: 600 }}>Test Example</Typography>
            <Button size="sm" icon="play_arrow" onClick={handleTest} disabled={testing || providers.length === 0}>
              {testing ? "Running..." : "Run"}
            </Button>
          </Box>
          <Box component="pre" sx={codeBlockSx}>
            {curlExample}
          </Box>
          {testError && (
            <Typography sx={{ mt: 1.5, fontSize: 12, color: "error.main", wordBreak: "break-word" }}>{testError}</Typography>
          )}
          {testResult && (
            <Box sx={{ mt: 1.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
              {testResult.latencyMs != null && (
                <Box component="span" sx={{ fontSize: 11, color: "text.secondary" }}>⚡ {testResult.latencyMs}ms</Box>
              )}
              {testResult.imageUrl && (
                <Box>
                  <Box sx={{ display: "flex", alignItems: "center", justifyContent: "flex-end", mb: 0.75 }}>
                    <Box component="a" href={testResult.imageUrl} download="image.webp" sx={{ display: "inline-flex", alignItems: "center", gap: 0.5, fontSize: 12, color: "text.secondary", transition: "color 150ms ease", "&:hover": { color: "primary.main" } }}>
                      <MsIcon sx={{ fontSize: 14 }}>download</MsIcon>
                      Download
                    </Box>
                  </Box>
                  <Image src={testResult.imageUrl} alt="Generated" width={512} height={512} style={{ maxWidth: "100%", height: "auto", borderRadius: 8, border: "1px solid rgba(128,128,128,0.3)" }} unoptimized />
                </Box>
              )}
              {testResult.audioUrl && (
                <Box>
                  <Box sx={{ display: "flex", alignItems: "center", justifyContent: "flex-end", mb: 0.75 }}>
                    <Box component="a" href={testResult.audioUrl} download="speech.mp3" sx={{ display: "inline-flex", alignItems: "center", gap: 0.5, fontSize: 12, color: "text.secondary", transition: "color 150ms ease", "&:hover": { color: "primary.main" } }}>
                      <MsIcon sx={{ fontSize: 14 }}>download</MsIcon>
                      Download
                    </Box>
                  </Box>
                  <Box component="audio" controls src={testResult.audioUrl} sx={{ width: "100%" }} aria-label="Generated speech audio">
                    <track kind="captions" />
                  </Box>
                </Box>
              )}
              {testResult.json && (
                <Box component="pre" sx={{ ...codeBlockSx, overflow: "auto", maxHeight: 300 }}>
                  {testResult.json}
                </Box>
              )}
            </Box>
          )}
        </Card>
      )}
      {/* Usage Logs Card */}
      <Card>
        <Typography component="h2" sx={{ fontSize: 18, fontWeight: 600, mb: 1.5 }}>Usage Logs</Typography>
        {logs.length === 0 ? (
          <Typography sx={{ fontSize: 12, color: "text.secondary", fontStyle: "italic" }}>No usage yet.</Typography>
        ) : (
          <Box component="pre" sx={{ ...codeBlockSx, fontSize: 11, overflow: "auto", maxHeight: 400 }}>
            {logs.join("\n")}
          </Box>
        )}
      </Card>
      {showPicker && (
        <ModelSelectModal
          isOpen={showPicker}
          onClose={() => setShowPicker(false)}
          onSelect={handleAddModel}
          onDeselect={handleDeselectModel}
          activeProviders={connections}
          modelAliases={modelAliases}
          title={`Add ${kindLabel} Model`}
          kindFilter={combo.kind}
          addedModelValues={providers}
          closeOnSelect={false}
        />
      )}
    </Box>
  );
}
