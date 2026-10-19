"use client";

import { useState } from "react";
import { Card, Button } from "@/shared/components";
import { useCopyToClipboard } from "@/shared/hooks/useCopyToClipboard";
import { useTheme } from "@/shared/hooks/useTheme";
import dynamic from "next/dynamic";
import Box from "@mui/material/Box";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import { alpha } from "@mui/material/styles";

const Editor = dynamic(() => import("@monaco-editor/react"), { ssr: false });

function MsIcon({ children, sx, ...props }) {
  return (
    <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18, ...sx }} {...props}>
      {children}
    </Icon>
  );
}

function saveTranslatorFile(file, content) {
  return fetch("/api/translator/save", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ file, content })
  }).catch(() => {});
}

const META_BADGE_COLORS = {
  blue: "#3b82f6",
  orange: "#f97316",
  green: "#22c55e",
  purple: "#a855f7",
};

// 7 steps matching requestLogger files exactly
const STEPS = [
  { id: 1, label: "Client Request",         file: "1_req_client.json",  lang: "json", desc: "Raw request from client" },
  { id: 2, label: "Source Body",            file: "2_req_source.json",  lang: "json", desc: "After initial conversion" },
  { id: 3, label: "OpenAI Intermediate",    file: "3_req_openai.json",  lang: "json", desc: "source → openai" },
  { id: 4, label: "Target Request",         file: "4_req_target.json",  lang: "json", desc: "openai → target + URL + headers" },
  { id: 5, label: "Provider Response",      file: "5_res_provider.txt", lang: "text", desc: "Raw SSE from provider" },
  { id: 6, label: "OpenAI Response",        file: "6_res_openai.txt",   lang: "text", desc: "target → openai (response)" },
  { id: 7, label: "Client Response",        file: "7_res_client.txt",   lang: "text", desc: "Final response to client" },
];

const EDITOR_OPTIONS = {
  minimap: { enabled: false },
  fontSize: 12,
  lineNumbers: "on",
  scrollBeyondLastLine: false,
  wordWrap: "on",
  automaticLayout: true,
};

export default function TranslatorPage() {
  const { isDark } = useTheme();
  const [contents, setContents] = useState({});
  const [expanded, setExpanded] = useState({ 1: true });
  const [loading, setLoading] = useState({});
  // Detected from step 1: { provider, model, sourceFormat, targetFormat }
  const [meta, setMeta] = useState(null);

  const setLoad = (key, val) => setLoading(prev => ({ ...prev, [key]: val }));
  const setContent = (id, val) => setContents(prev => ({ ...prev, [id]: val }));
  const toggle = (id) => setExpanded(prev => ({ ...prev, [id]: !prev[id] }));

  const openNext = (nextId) => setExpanded(prev => {
    const next = {};
    STEPS.forEach(s => { next[s.id] = false; });
    next[nextId] = true;
    return next;
  });

  // Load file from logs/translator/
  const handleLoad = async (stepId) => {
    const step = STEPS.find(s => s.id === stepId);
    setLoad(`load-${stepId}`, true);
    try {
      const res = await fetch(`/api/translator/load?file=${step.file}`);
      const data = await res.json();
      if (data.success) {
        setContent(stepId, data.content);
        if (stepId === 1) await detectMeta(data.content);
      } else {
        alert(data.error || "File not found");
      }
    } catch (e) {
      alert(e.message);
    }
    setLoad(`load-${stepId}`, false);
  };

  // Step 1: detect provider/format from model field
  const detectMeta = async (rawContent) => {
    try {
      const body = typeof rawContent === "string" ? JSON.parse(rawContent) : rawContent;
      const res = await fetch("/api/translator/translate", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ step: 1, body })
      });
      const data = await res.json();
      if (data.success) setMeta(data.result);
    } catch { /* ignore */ }
  };

  const save = saveTranslatorFile;

  // Step 1 → Step 3: source → OpenAI intermediate
  const handleToOpenAI = async () => {
    setLoad("toOpenAI", true);
    try {
      const raw = contents[1];
      const body = JSON.parse(raw);
      // Save input: 1_req_client.json + 2_req_source.json (body only)
      save("1_req_client.json", raw);
      save("2_req_source.json", JSON.stringify({ timestamp: new Date().toISOString(), headers: {}, body: body.body || body }, null, 2));

      const res = await fetch("/api/translator/translate", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ step: 2, body })
      });
      const data = await res.json();
      if (!data.success) { alert(data.error); return; }
      const str = JSON.stringify(data.result.body, null, 2);
      setContent(3, str);
      openNext(3);
    } catch (e) { alert(e.message); }
    setLoad("toOpenAI", false);
  };

  // Step 3 → Step 4: OpenAI → target + build URL/headers
  const handleToTarget = async () => {
    setLoad("toTarget", true);
    try {
      const raw = contents[3];
      const openaiBody = JSON.parse(raw);
      // Save input: 3_req_openai.json
      save("3_req_openai.json", raw);

      const res = await fetch("/api/translator/translate", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ step: 3, body: { ...openaiBody, provider: meta?.provider, model: meta?.model } })
      });
      const data = await res.json();
      if (!data.success) { alert(data.error); return; }
      // Embed provider + model so Send works even without meta
      const step4Content = { ...data.result, provider: meta?.provider, model: meta?.model };
      setContent(4, JSON.stringify(step4Content, null, 2));
      openNext(4);
    } catch (e) { alert(e.message); }
    setLoad("toTarget", false);
  };

  // Step 4 → Step 5: send to provider via executor
  const handleSend = async () => {
    setLoad("send", true);
    try {
      const raw = contents[4];
      const step4 = JSON.parse(raw);
      // Save input: 4_req_target.json
      save("4_req_target.json", raw);

      // Read provider/model from step4 content (embedded during build), fallback to meta
      const provider = step4.provider || meta?.provider;
      const model = step4.model || meta?.model;

      if (!provider || !model) {
        alert("Missing provider or model. Please run step 1 first to detect them.");
        return;
      }

      const res = await fetch("/api/translator/send", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ provider, model, body: step4.body || step4 })
      });

      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }));
        alert(err.error || "Send failed");
        return;
      }

      // Accumulate streaming response
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let full = "";
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        full += decoder.decode(value, { stream: true });
      }

      setContent(5, full);
      openNext(5);

      // Save to logs/translator/5_res_provider.txt
      await fetch("/api/translator/save", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ file: "5_res_provider.txt", content: full })
      });
    } catch (e) {
      alert(e.message);
    } finally {
      setLoad("send", false);
    }
  };

  const { copy } = useCopyToClipboard();

  const handleCopy = async (id) => {
    if (!contents[id]) return;
    copy(contents[id], `translator-step-${id}`);
  };

  const handleFormat = (id) => {
    try {
      const obj = JSON.parse(contents[id]);
      setContent(id, JSON.stringify(obj, null, 2));
    } catch { /* not JSON, skip */ }
  };

  // Render action button per step
  const getAction = (stepId) => {
    if (stepId === 1) return <Button size="sm" icon="arrow_forward" loading={loading["toOpenAI"]} onClick={handleToOpenAI}>→ OpenAI</Button>;
    if (stepId === 3) return <Button size="sm" icon="arrow_forward" loading={loading["toTarget"]} onClick={handleToTarget}>→ Target</Button>;
    if (stepId === 4) return <Button size="sm" icon="send" loading={loading["send"]} onClick={handleSend}>Send</Button>;
    return null;
  };

  return (
    <Stack spacing={1.5} sx={{ p: 4 }}>
      {/* Header */}
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", mb: 2 }}>
        <Box>
          <Typography component="h1" variant="h5" sx={{ fontWeight: 700, color: "text.primary" }}>
            Translator Debug
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
            Replay request flow — matches log files
          </Typography>
        </Box>
        {meta && (
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap", justifyContent: "flex-end" }}>
            <MetaBadge label="src" value={meta.sourceFormat} color="blue" />
            <MsIcon sx={{ color: "text.secondary", fontSize: 14 }}>arrow_forward</MsIcon>
            <MetaBadge label="dst" value={meta.targetFormat} color="orange" />
            <MetaBadge label="provider" value={meta.provider} color="green" />
            <MetaBadge label="model" value={meta.model} color="purple" />
          </Box>
        )}
      </Box>

      {STEPS.map((step) => {
        const action = getAction(step.id);
        const isExpanded = !!expanded[step.id];
        const content = contents[step.id] || "";

        return (
          <Card key={step.id}>
            <Stack spacing={1.5} sx={{ p: 2 }}>
              {/* Step header */}
              <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                <Box
                  component="button"
                  type="button"
                  onClick={() => toggle(step.id)}
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    gap: 1,
                    flex: 1,
                    textAlign: "left",
                    background: "none",
                    border: 0,
                    p: 0,
                    cursor: "pointer",
                    color: "inherit",
                    "&:hover .step-chevron": { color: "primary.main" },
                  }}
                >
                  <MsIcon className="material-symbols-outlined step-chevron" sx={{ fontSize: 20, color: "text.secondary", transition: "color 150ms ease" }}>
                    {isExpanded ? "expand_more" : "chevron_right"}
                  </MsIcon>
                  <Box component="span" sx={{ fontSize: 12, fontFamily: "monospace", color: "text.secondary", opacity: 0.6, width: 16 }}>
                    {step.id}
                  </Box>
                  <Typography component="h3" variant="body2" sx={{ fontWeight: 600, color: "text.primary" }}>
                    {step.label}
                  </Typography>
                  <Box component="span" sx={{ fontSize: 12, color: "text.secondary", opacity: 0.6, fontFamily: "monospace" }}>
                    {step.file}
                  </Box>
                  {content && <Box component="span" sx={{ fontSize: 12, color: "success.main" }}>({content.length} chars)</Box>}
                </Box>
                {!isExpanded && (
                  <Box sx={{ display: "flex", gap: 0.5, flexShrink: 0 }}>
                    <Button size="sm" variant="ghost" icon="folder_open" loading={loading[`load-${step.id}`]} onClick={() => handleLoad(step.id)} />
                    {action}
                  </Box>
                )}
              </Box>

              {/* Expanded content */}
              {isExpanded && (
                <>
                  <Box sx={{ border: 1, borderColor: "divider", borderRadius: 2, overflow: "hidden" }}>
                    <Editor
                      height="400px"
                      defaultLanguage={step.lang === "text" ? "plaintext" : "json"}
                      value={content}
                      onChange={(v) => {
                        setContent(step.id, v || "");
                        if (step.id === 1) detectMeta(v || "");
                      }}
                      theme={isDark ? "vs-dark" : "light"}
                      options={EDITOR_OPTIONS}
                    />
                  </Box>
                  <Box sx={{ display: "flex", gap: 1, flexWrap: "wrap" }}>
                    <Button size="sm" variant="outline" icon="folder_open" loading={loading[`load-${step.id}`]} onClick={() => handleLoad(step.id)}>Load</Button>
                    <Button size="sm" variant="outline" icon="data_object" onClick={() => handleFormat(step.id)}>Format</Button>
                    <Button size="sm" variant="outline" icon="content_copy" onClick={() => handleCopy(step.id)}>Copy</Button>
                    {action}
                  </Box>
                </>
              )}
            </Stack>
          </Card>
        );
      })}
    </Stack>
  );
}

function MetaBadge({ label, value, color }) {
  const c = META_BADGE_COLORS[color];
  return (
    <Box
      component="span"
      sx={{
        display: "inline-flex",
        alignItems: "center",
        gap: 0.5,
        px: 1,
        py: 0.25,
        borderRadius: 0.5,
        fontSize: 12,
        fontFamily: "monospace",
        color: c,
        bgcolor: alpha(c, 0.1),
      }}
    >
      <Box component="span" sx={{ color: "text.secondary", opacity: 0.7, fontFamily: "inherit", fontSize: 10 }}>{label}:</Box>{value}
    </Box>
  );
}
