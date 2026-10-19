"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import Image from "next/image";
import { Badge, Button } from "@/shared/components";
import { getModelsByProviderId } from "@/shared/constants/models";
import { isAnthropicCompatibleProvider, isOpenAICompatibleProvider } from "@/shared/constants/providers";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";

function MsIcon({ children, sx, ...props }) {
  return <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18, ...sx }} {...props}>{children}</Icon>;
}

const STORAGE_KEYS = {
  sessions: "basic-chat.sessions",
  activeSessionId: "basic-chat.activeSessionId",
  activeProviderId: "basic-chat.activeProviderId",
  draft: "basic-chat.draft",
};

function createId() {
  if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID();
  return `chat_${Date.now()}_${Math.random().toString(16).slice(2)}`;
}

function ensureSessionForModel(model) {
  if (!model) return null;
  return {
    id: createId(),
    title: "New chat",
    providerId: model.providerId,
    providerName: model.providerName,
    modelId: model.id,
    modelName: model.name,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    messages: [],
  };
}

function safeParse(value, fallback) {
  try {
    return JSON.parse(value);
  } catch {
    return fallback;
  }
}

function textValue(value) {
  if (typeof value === "string") return value;
  if (value == null) return "";
  if (Array.isArray(value)) return value.flatMap(v => { const t = textValue(v); return t ? [t] : []; }).join(" ");
  if (typeof value === "object") {
    if (typeof value.message === "string") return value.message;
    if (typeof value.error === "string") return value.error;
    try {
      return JSON.stringify(value);
    } catch {
      return String(value);
    }
  }
  return String(value);
}

function humanize(value = "") {
  return String(value)
    .replace(/[-_]/g, " ")
    .replace(/\b\w/g, (char) => char.toUpperCase())
    .trim() || "Unknown";
}

function formatRelativeTime(value) {
  if (!value) return "Now";
  const time = new Date(value).getTime();
  if (Number.isNaN(time)) return "Now";
  const diffMinutes = Math.max(1, Math.round((Date.now() - time) / 60000));
  if (diffMinutes < 60) return `${diffMinutes}m`;
  const diffHours = Math.round(diffMinutes / 60);
  if (diffHours < 24) return `${diffHours}h`;
  return `${Math.round(diffHours / 24)}d`;
}

function makeSessionTitle(text = "") {
  const normalized = textValue(text).replace(/\s+/g, " ").trim();
  if (!normalized) return "New chat";
  return normalized.length > 52 ? `${normalized.slice(0, 52).trimEnd()}…` : normalized;
}

function buildUserContent(message) {
  const text = textValue(message.content).trim();
  const attachments = Array.isArray(message.attachments) ? message.attachments : [];

  if (attachments.length === 0) return text;

  const content = [];
  if (text) content.push({ type: "text", text });

  for (const attachment of attachments) {
    if (attachment?.dataUrl) {
      content.push({ type: "image_url", image_url: { url: attachment.dataUrl } });
    }
  }

  return content.length > 0 ? content : text;
}

function readAssistantText(chunk) {
  if (!chunk || typeof chunk !== "object") return "";
  const choice = chunk.choices?.[0];
  const delta = choice?.delta || {};
  const pieces = [delta.content, choice?.message?.content, chunk.output_text, chunk.text]
    .map(textValue)
    .filter(Boolean);
  return pieces[0] || "";
}

async function fileToDataUrl(file) {
  return await new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result || ""));
    reader.onerror = () => reject(reader.error || new Error("Failed to read file"));
    reader.readAsDataURL(file);
  });
}

function cloneSession(session) {
  return {
    ...session,
    messages: Array.isArray(session.messages) ? session.messages.map((message) => ({ ...message })) : [],
  };
}

function getProviderLabel(connection) {
  return connection?.name || humanize(connection?.provider || connection?.id || "provider");
}

function normalizeStaticModel(model, connection) {
  if (!model?.id) return null;
  return {
    id: `${connection.provider}/${model.id}`,
    requestModel: `${connection.provider}/${model.id}`,
    name: model.name || model.id,
    providerId: connection.provider,
    providerName: getProviderLabel(connection),
    source: "static",
  };
}

function normalizeLiveModel(model, connection) {
  const rawId = typeof model === "string" ? model : model?.id || model?.name || model?.model || "";
  if (!rawId) return null;

  const displayName = typeof model === "string"
    ? model
    : model?.name || model?.displayName || rawId;

  let requestModel = rawId;
  const isCompatible = isOpenAICompatibleProvider(connection.provider) || isAnthropicCompatibleProvider(connection.provider);
  if (isCompatible && !rawId.includes("/")) {
    requestModel = `${connection.provider}/${rawId}`;
  }

  return {
    id: requestModel,
    requestModel,
    name: displayName,
    providerId: connection.provider,
    providerName: getProviderLabel(connection),
    source: "live",
  };
}

function parseProviderModelsPayload(data) {
  if (Array.isArray(data?.models)) return data.models;
  if (Array.isArray(data?.data)) return data.data;
  if (Array.isArray(data?.results)) return data.results;
  if (Array.isArray(data)) return data;
  return [];
}

function dedupeModels(models) {
  const map = new Map();
  for (const model of models) {
    if (!model?.id) continue;
    if (!map.has(model.id)) map.set(model.id, model);
  }
  return Array.from(map.values());
}

export default function BasicChatPageClient() {
  const [providerGroups, setProviderGroups] = useState([]);
  const [loadingData, setLoadingData] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [sessions, setSessions] = useState(() => {
    if (typeof window === "undefined") return [];
    try {
      const saved = safeParse(globalThis.localStorage.getItem(STORAGE_KEYS.sessions), []);
      return Array.isArray(saved) ? saved.map((session) => ({
        ...session,
        messages: Array.isArray(session.messages) ? session.messages : [],
      })) : [];
    } catch { return []; }
  });
  const [activeSessionId, setActiveSessionId] = useState(() => {
    if (typeof window === "undefined") return "";
    return globalThis.localStorage.getItem(STORAGE_KEYS.activeSessionId) || "";
  });
  const [activeProviderId, setActiveProviderId] = useState(() => {
    if (typeof window === "undefined") return "";
    return globalThis.localStorage.getItem(STORAGE_KEYS.activeProviderId) || "";
  });
  const [activeModelId, setActiveModelId] = useState("");
  const [draft, setDraft] = useState(() => {
    if (typeof window === "undefined") return "";
    return globalThis.localStorage.getItem(STORAGE_KEYS.draft) || "";
  });
  const [attachments, setAttachments] = useState([]);
  const [isSending, setIsSending] = useState(false);
  const [streamingMessageId, setStreamingMessageId] = useState("");
  const [streamingText, setStreamingText] = useState("");
  const [isHydrated] = useState(() => typeof window !== "undefined");
  const [modelMenuOpen, setModelMenuOpen] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const fileInputRef = useRef(null);
  const abortRef = useRef(null);
  const initializedRef = useRef(false);
  const modelMenuRef = useRef(null);
  const historyMenuRef = useRef(null);

  useEffect(() => {
    const controller = new AbortController();
    let cancelled = false;

    async function loadData() {
      setLoadingData(true);
      setLoadError("");

      try {
        const providersRes = await fetch("/api/providers", { cache: "no-store", signal: controller.signal });
        const providersData = await providersRes.json().catch(() => ({}));
        const connections = Array.isArray(providersData.connections)
          ? providersData.connections.filter((connection) => connection?.isActive !== false)
          : [];

        if (connections.length === 0) {
          if (!cancelled) {
            setProviderGroups([]);
            setLoadError("No providers connected yet.");
          }
          return;
        }

        const providerMap = new Map();

        for (const connection of connections) {
          const providerId = connection.provider || connection.id;
          const providerName = getProviderLabel(connection);
          const providerType = isOpenAICompatibleProvider(providerId)
            ? "openai-compatible"
            : isAnthropicCompatibleProvider(providerId)
              ? "anthropic-compatible"
              : providerId;

          if (!providerMap.has(providerId)) {
            providerMap.set(providerId, {
              providerId,
              providerName,
              providerType,
              connections: [],
              models: [],
            });
          }

          const group = providerMap.get(providerId);
          group.providerName = group.providerName || providerName;
          group.providerType = group.providerType || providerType;
          group.connections.push(connection);

          const staticModels = getModelsByProviderId(providerId)
            .flatMap((model) => { const m = normalizeStaticModel(model, connection); return m ? [m] : []; });
          group.models.push(...staticModels);
        }

        const liveResults = await Promise.all(
          connections.map(async (connection) => {
            try {
              const response = await fetch(`/api/providers/${connection.id}/models`, { cache: "no-store", signal: controller.signal });
              const data = await response.json().catch(() => ({}));
              if (!response.ok) return { connection, models: [] };
              const models = parseProviderModelsPayload(data)
                .flatMap((model) => { const m = normalizeLiveModel(model, connection); return m ? [m] : []; });
              return { connection, models };
            } catch {
              return { connection, models: [] };
            }
          })
        );

        for (const result of liveResults) {
          const providerId = result.connection.provider || result.connection.id;
          const group = providerMap.get(providerId);
          if (!group) continue;
          group.models.push(...result.models);
        }

        const normalized = Array.from(providerMap.values())
          .reduce((acc, group) => {
            const models = dedupeModels(group.models).sort((a, b) => a.name.localeCompare(b.name));
            if (models.length > 0) acc.push({ ...group, models });
            return acc;
          }, [])
          .sort((a, b) => a.providerName.localeCompare(b.providerName));

        if (!cancelled) {
          setProviderGroups(normalized);
          if (normalized.length === 0) {
            setLoadError("Providers connected but no models available.");
          }
        }
      } catch (error) {
        if (!cancelled) {
          setLoadError(textValue(error?.message) || "Failed to load providers/models.");
          setProviderGroups([]);
        }
      } finally {
        if (!cancelled) setLoadingData(false);
      }
    }

    loadData();
    return () => {
      cancelled = true;
      controller.abort();
    };
  }, []);

  useEffect(() => {
    const handleClickOutside = (event) => {
      if (modelMenuRef.current && !modelMenuRef.current.contains(event.target)) {
        setModelMenuOpen(false);
      }
      if (historyMenuRef.current && !historyMenuRef.current.contains(event.target)) {
        setHistoryOpen(false);
      }
    };

    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  const modelIndex = useMemo(() => {
    const map = new Map();
    for (const group of providerGroups) {
      for (const model of group.models) {
        map.set(model.id, {
          ...model,
          providerId: group.providerId,
          providerName: group.providerName,
        });
      }
    }
    return map;
  }, [providerGroups]);

  const activeProviderGroup = useMemo(() => {
    return providerGroups.find((group) => group.providerId === activeProviderId) || providerGroups[0] || null;
  }, [providerGroups, activeProviderId]);

  const activeModel = useMemo(() => {
    if (activeModelId && modelIndex.has(activeModelId)) return modelIndex.get(activeModelId);
    if (activeSessionId) {
      const session = sessions.find((item) => item.id === activeSessionId);
      if (session?.modelId && modelIndex.has(session.modelId)) return modelIndex.get(session.modelId);
    }
    return activeProviderGroup?.models?.[0] || null;
  }, [activeModelId, modelIndex, activeProviderGroup, sessions, activeSessionId]);

  const currentSession = useMemo(() => sessions.find((session) => session.id === activeSessionId) || null, [sessions, activeSessionId]);
  const currentMessages = currentSession?.messages || [];
  const sessionItems = useMemo(() => sessions.toSorted((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()), [sessions]);
  const canSend = !isSending && !!activeModel && (draft.trim().length > 0 || attachments.length > 0);

  useEffect(() => {
    if (!isHydrated) return;
    try {
      globalThis.localStorage.setItem(STORAGE_KEYS.sessions, JSON.stringify(sessions));
      globalThis.localStorage.setItem(STORAGE_KEYS.activeSessionId, activeSessionId);
      globalThis.localStorage.setItem(STORAGE_KEYS.activeProviderId, activeProviderId);
      globalThis.localStorage.setItem(STORAGE_KEYS.draft, draft);
    } catch {
      // Ignore storage errors.
    }
  }, [isHydrated, sessions, activeSessionId, activeProviderId, draft]);

  useEffect(() => {
    if (!isHydrated || loadingData || initializedRef.current) return;
    if (providerGroups.length === 0) return;

    const savedProvider = providerGroups.find((group) => group.providerId === activeProviderId) || providerGroups[0];
    const savedModel = activeModelId && modelIndex.has(activeModelId)
      ? modelIndex.get(activeModelId)
      : savedProvider.models[0];

    if (sessions.length > 0) {
      const session = sessions.find((item) => item.id === activeSessionId) || sessions[0];
      const sessionModel = session?.modelId && modelIndex.has(session.modelId)
        ? modelIndex.get(session.modelId)
        : savedModel;
      initializedRef.current = true;
      // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time persisted-session bootstrap.
      setActiveSessionId(session.id);
      setActiveProviderId(sessionModel?.providerId || savedProvider.providerId);
      setActiveModelId(sessionModel?.id || savedModel.id);
      return;
    }

    const session = {
      id: createId(),
      title: "New chat",
      providerId: savedProvider.providerId,
      providerName: savedProvider.providerName,
      modelId: savedModel.id,
      modelName: savedModel.name,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      messages: [],
    };

    initializedRef.current = true;
    setSessions([session]);
    setActiveSessionId(session.id);
    setActiveProviderId(savedProvider.providerId);
    setActiveModelId(savedModel.id);
  }, [isHydrated, loadingData, providerGroups, modelIndex, activeProviderId, activeModelId, sessions, activeSessionId]);

  const updateSession = (sessionId, updater) => {
    setSessions((prev) => prev.map((session) => (session.id === sessionId ? updater(cloneSession(session)) : session)));
  };

  const handleNewChat = () => {
    if (!activeModel) return;
    const session = ensureSessionForModel(activeModel);
    if (!session) return;
    setSessions((prev) => [session, ...prev]);
    setActiveSessionId(session.id);
    setActiveProviderId(session.providerId);
    setActiveModelId(session.modelId);
    setDraft("");
    setAttachments([]);
    setStreamingMessageId("");
    setStreamingText("");
  };

  const handleSelectSession = (sessionId) => {
    const session = sessions.find((item) => item.id === sessionId);
    if (!session) return;
    setActiveSessionId(sessionId);
    setActiveProviderId(session.providerId || activeProviderId);
    setActiveModelId(session.modelId || activeModelId);
    setHistoryOpen(false);
  };

  const handleDeleteCurrentChat = () => {
    if (!activeSessionId) return;
    const nextSessions = sessions.filter((session) => session.id !== activeSessionId);
    const fallback = nextSessions[0] || null;
    setSessions(nextSessions);
    if (fallback) {
      setActiveSessionId(fallback.id);
      setActiveProviderId(fallback.providerId);
      setActiveModelId(fallback.modelId);
    } else {
      setActiveSessionId("");
      setActiveProviderId("");
      setActiveModelId("");
    }
  };

  const handleSelectProvider = (providerId) => {
    const group = providerGroups.find((item) => item.providerId === providerId);
    if (!group || group.models.length === 0) return;
    const nextModel = group.models[0];

    const current = sessions.find((session) => session.id === activeSessionId);
    if (current && current.messages.length > 0) {
      const session = ensureSessionForModel(nextModel);
      if (!session) return;
      setSessions((prev) => [session, ...prev]);
      setActiveSessionId(session.id);
    } else if (current) {
      setSessions((prev) => prev.map((item) => (item.id === current.id ? {
        ...item,
        providerId: group.providerId,
        providerName: group.providerName,
        modelId: nextModel.id,
        modelName: nextModel.name,
      } : item)));
      setActiveSessionId(current.id);
    }

    setActiveProviderId(group.providerId);
    setActiveModelId(nextModel.id);
    setModelMenuOpen(false);
  };

  const handleSelectModel = (modelId) => {
    const model = modelIndex.get(modelId);
    if (!model) return;

    const current = sessions.find((session) => session.id === activeSessionId);
    if (current && current.messages.length > 0) {
      const session = ensureSessionForModel(model);
      if (!session) return;
      setSessions((prev) => [session, ...prev]);
      setActiveSessionId(session.id);
    } else if (current) {
      setSessions((prev) => prev.map((item) => (item.id === current.id ? {
        ...item,
        providerId: model.providerId,
        providerName: model.providerName,
        modelId: model.id,
        modelName: model.name,
      } : item)));
      setActiveSessionId(current.id);
    } else {
      const session = ensureSessionForModel(model);
      if (!session) return;
      setSessions((prev) => [session, ...prev]);
      setActiveSessionId(session.id);
    }

    setActiveProviderId(model.providerId);
    setActiveModelId(model.id);
    setModelMenuOpen(false);
  };

  const handleAttachFiles = async (event) => {
    const files = Array.from(event.target.files || []);
    if (files.length === 0) return;

    const images = files.filter((file) => file.type.startsWith("image/"));
    if (images.length === 0) {
      event.target.value = "";
      return;
    }

    const converted = await Promise.all(images.map(async (file) => ({
      id: createId(),
      name: file.name,
      type: file.type,
      size: file.size,
      dataUrl: await fileToDataUrl(file),
    })));

    setAttachments((prev) => [...prev, ...converted]);
    event.target.value = "";
  };

  const removeAttachment = (attachmentId) => {
    setAttachments((prev) => prev.filter((attachment) => attachment.id !== attachmentId));
  };

  const handleStop = () => {
    abortRef.current?.abort();
  };

  const finalizeSessionTitle = (sessionId, titleSeed) => {
    const title = makeSessionTitle(titleSeed);
    updateSession(sessionId, (session) => ({
      ...session,
      title: session.title === "New chat" ? title : session.title,
      updatedAt: new Date().toISOString(),
    }));
  };

  const sendMessage = async () => {
    const model = activeModel || activeProviderGroup?.models?.[0] || null;
    if (!model) return;

    const userText = draft.trim();
    if (!userText && attachments.length === 0) return;

    let sessionId = activeSessionId;
    let session = sessions.find((item) => item.id === sessionId);
    if (!session) {
      session = ensureSessionForModel(model);
      if (!session) return;
      sessionId = session.id;
      setSessions((prev) => [session, ...prev]);
      setActiveSessionId(sessionId);
    }

    const userMessage = {
      id: createId(),
      role: "user",
      content: userText,
      attachments: attachments.map((attachment) => ({
        id: attachment.id,
        name: attachment.name,
        type: attachment.type,
        dataUrl: attachment.dataUrl,
      })),
      createdAt: new Date().toISOString(),
    };

    const assistantMessageId = createId();
    const assistantMessage = {
      id: assistantMessageId,
      role: "assistant",
      content: "",
      createdAt: new Date().toISOString(),
      status: "streaming",
    };

    const nextMessages = [...(session.messages || []), userMessage, assistantMessage];
    setSessions((prev) => prev.map((item) => (item.id === sessionId ? {
      ...item,
      providerId: model.providerId,
      providerName: model.providerName,
      modelId: model.id,
      modelName: model.name,
      messages: nextMessages,
      updatedAt: new Date().toISOString(),
      title: item.title === "New chat" ? makeSessionTitle(userText) : item.title,
    } : item)));
    setDraft("");
    setAttachments([]);
    setIsSending(true);
    setStreamingMessageId(assistantMessageId);
    setStreamingText("");
    abortRef.current?.abort();
    abortRef.current = new AbortController();

    const requestMessages = nextMessages.reduce((acc, message) => {
      if (!(message.role === "assistant" && message.id === assistantMessageId)) {
        acc.push({ role: message.role, content: message.role === "user" ? buildUserContent(message) : message.content });
      }
      return acc;
    }, []);

    try {
      const response = await fetch("/api/dashboard/chat/completions", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "text/event-stream",
        },
        body: JSON.stringify({
          model: model.requestModel || model.id,
          messages: requestMessages,
          stream: true,
        }),
        signal: abortRef.current.signal,
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw new Error(textValue(errorData.error || errorData.message || `Request failed (${response.status})`));
      }

      const reader = response.body?.getReader();
      if (!reader) {
        const data = await response.json().catch(() => ({}));
        const fallbackText = textValue(data?.choices?.[0]?.message?.content || data?.output_text || data?.error || data?.message || "");
        updateSession(sessionId, (currentSession) => ({
          ...currentSession,
          messages: currentSession.messages.map((message) => (message.id === assistantMessageId ? { ...message, content: fallbackText, status: "done" } : message)),
          updatedAt: new Date().toISOString(),
        }));
        return;
      }

      const decoder = new TextDecoder();
      let buffer = "";
      let assistantText = "";

      while (true) {
        const { value, done } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split(/\r?\n/);
        buffer = lines.pop() || "";

        for (const line of lines) {
          const trimmed = line.trim();
          if (!trimmed.startsWith("data:")) continue;

          const payload = trimmed.slice(5).trim();
          if (!payload || payload === "[DONE]") continue;

          try {
            const chunk = JSON.parse(payload);
            const text = readAssistantText(chunk);
            if (!text) continue;

            assistantText += text;
            setStreamingText(assistantText);
            updateSession(sessionId, (currentSession) => ({
              ...currentSession,
              messages: currentSession.messages.map((message) => (message.id === assistantMessageId ? { ...message, content: assistantText, status: "streaming" } : message)),
              updatedAt: new Date().toISOString(),
            }));
          } catch {
            // Ignore malformed chunks.
          }
        }
      }

      updateSession(sessionId, (currentSession) => ({
        ...currentSession,
        messages: currentSession.messages.map((message) => (message.id === assistantMessageId ? { ...message, content: assistantText || message.content, status: "done" } : message)),
        updatedAt: new Date().toISOString(),
      }));
      finalizeSessionTitle(sessionId, userText);
    } catch (error) {
      if (error.name !== "AbortError") {
        const errorText = textValue(error?.message || error);
        updateSession(sessionId, (currentSession) => ({
          ...currentSession,
          messages: currentSession.messages.map((message) => (message.id === assistantMessageId ? { ...message, content: message.content || `Error: ${errorText}`, status: "error" } : message)),
          updatedAt: new Date().toISOString(),
        }));
        setLoadError(errorText || "Failed to send message.");
      }
    } finally {
      setIsSending(false);
      setStreamingMessageId("");
      setStreamingText("");
      abortRef.current = null;
    }
  };

  const handleKeyDown = (event) => {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      if (canSend) sendMessage();
    }
  };

  const modelLabel = activeModel ? `${activeModel.name}` : "Select model";
  const modelSubLabel = activeModel ? activeModel.requestModel : "Choose from connected providers";

  const panelSx = {
    border: "1px solid rgba(255,255,255,0.1)",
    borderRadius: 4,
    bgcolor: "rgba(255,255,255,0.05)",
    color: "#fff",
    transition: "background-color 150ms ease",
    "&:hover": { bgcolor: "rgba(255,255,255,0.08)" },
  };
  const optionSx = (active) => ({
    width: "100%",
    border: "1px solid",
    borderColor: active ? "rgba(96,165,250,0.4)" : "rgba(255,255,255,0.1)",
    borderRadius: 3.5,
    px: 1.5,
    py: 1.5,
    textAlign: "left",
    color: "#fff",
    bgcolor: active ? "rgba(59,130,246,0.15)" : "rgba(255,255,255,0.05)",
    cursor: "pointer",
    transition: "background-color 150ms ease",
    "&:hover": { bgcolor: active ? "rgba(59,130,246,0.15)" : "rgba(255,255,255,0.08)" },
  });

  return (
    <Box sx={{ position: "relative", flex: 1, display: "flex", flexDirection: "column", height: "100%", minHeight: 0, minWidth: 0, bgcolor: "#212121", color: "#fff", overflow: "hidden" }}>
      <Box sx={{ position: "relative", mx: "auto", display: "flex", flex: 1, height: "100%", minHeight: 0, width: "100%", maxWidth: 896, flexDirection: "column" }}>
        <Box sx={{ display: "flex", flexShrink: 0, alignItems: "center", justifyContent: "space-between", gap: 1.5, px: { xs: 2, lg: 3 }, py: 1.5 }}>
          <Box ref={modelMenuRef} sx={{ position: "relative" }}>
            <Box
              component="button"
              type="button"
              onClick={() => setModelMenuOpen((value) => !value)}
              sx={{ ...panelSx, display: "flex", alignItems: "center", gap: 1.5, px: 2, py: 1.5, textAlign: "left", cursor: "pointer" }}
            >
              <Box sx={{ minWidth: 0 }}>
                <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                  <Typography sx={{ fontSize: 14, fontWeight: 600, color: "#fff" }}>{modelLabel}</Typography>
                  <MsIcon sx={{ fontSize: 18, color: "rgba(255,255,255,0.7)" }}>expand_more</MsIcon>
                </Box>
                <Typography sx={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontSize: 12, color: "rgba(255,255,255,0.55)" }}>{modelSubLabel}</Typography>
              </Box>
            </Box>

            {modelMenuOpen ? (
              <Box sx={{ position: "absolute", left: 0, top: "calc(100% + 10px)", zIndex: 30, width: "min(520px, calc(100vw - 2rem)", overflow: "hidden", borderRadius: 5, border: "1px solid rgba(255,255,255,0.1)", bgcolor: "#262626", boxShadow: "0 25px 50px rgba(0,0,0,0.5)" }}>
                <Box sx={{ borderBottom: "1px solid rgba(255,255,255,0.1)", px: 2, py: 1.5 }}>
                  <Typography sx={{ fontSize: 12, textTransform: "uppercase", letterSpacing: "0.22em", color: "rgba(255,255,255,0.45)" }}>Models</Typography>
                  <Typography sx={{ fontSize: 14, color: "rgba(255,255,255,0.75)" }}>Only from connected providers</Typography>
                </Box>
                <Box sx={{ maxHeight: "60vh", overflowY: "auto", p: 1 }}>
                  {providerGroups.map((group) => (
                    <Box key={group.providerId} sx={{ mb: 1, borderRadius: 4, border: "1px solid rgba(255,255,255,0.1)", bgcolor: "rgba(0,0,0,0.2)", p: 1 }}>
                      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", px: 1, py: 1 }}>
                        <Typography sx={{ fontSize: 14, fontWeight: 600, color: "#fff" }}>{group.providerName}</Typography>
                        <Badge size="sm" variant="default">{group.models.length}</Badge>
                      </Box>
                      <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)" }, gap: 1 }}>
                        {group.models.map((model) => {
                          const isActive = model.id === activeModelId;
                          return (
                            <Box key={model.id} component="button" type="button" onClick={() => handleSelectModel(model.id)} sx={optionSx(isActive)}>
                              <Box sx={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 1.5 }}>
                                <Box sx={{ minWidth: 0 }}>
                                  <Typography sx={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontSize: 14, fontWeight: 500, color: "#fff" }}>{model.name}</Typography>
                                  <Typography sx={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontSize: 11, color: "rgba(255,255,255,0.45)" }}>{model.requestModel}</Typography>
                                </Box>
                                {isActive ? <MsIcon sx={{ fontSize: 18, color: "#93c5fd" }}>check_circle</MsIcon> : null}
                              </Box>
                            </Box>
                          );
                        })}
                      </Box>
                    </Box>
                  ))}
                </Box>
              </Box>
            ) : null}
          </Box>

          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <Box component="button" type="button" onClick={() => setHistoryOpen((value) => !value)} sx={{ ...panelSx, px: 2, py: 1.5, fontSize: 14, color: "rgba(255,255,255,0.8)", cursor: "pointer" }}>
              History
            </Box>
            <Button variant="ghost" size="sm" icon="delete" onClick={handleDeleteCurrentChat} disabled={!activeSessionId || sessions.length === 0}>Clear</Button>
          </Box>
        </Box>

        {historyOpen ? (
          <Box ref={historyMenuRef} sx={{ position: "absolute", right: { xs: 2, lg: 3 }, top: 72, zIndex: 20, width: "min(360px, calc(100vw - 2rem))", borderRadius: 5, border: "1px solid rgba(255,255,255,0.1)", bgcolor: "#262626", p: 1, boxShadow: "0 25px 50px rgba(0,0,0,0.5)" }}>
            <Box sx={{ px: 1.5, py: 1 }}><Typography sx={{ fontSize: 12, textTransform: "uppercase", letterSpacing: "0.22em", color: "rgba(255,255,255,0.45)" }}>Recent chats</Typography></Box>
            <Box sx={{ maxHeight: "48vh", overflowY: "auto", p: 0.5, display: "flex", flexDirection: "column", gap: 1 }}>
              {sessionItems.length === 0 ? (
                <Box sx={{ borderRadius: 4, border: "1px dashed rgba(255,255,255,0.1)", bgcolor: "rgba(255,255,255,0.05)", p: 2, fontSize: 14, color: "rgba(255,255,255,0.55)" }}>No conversations yet.</Box>
              ) : sessionItems.map((session) => {
                const isActive = session.id === activeSessionId;
                const latestMessage = [...(session.messages || [])].reverse().find((message) => message.role === "user") || session.messages?.[0];
                return (
                  <Box key={session.id} component="button" type="button" onClick={() => handleSelectSession(session.id)} sx={optionSx(isActive)}>
                    <Box sx={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 1.5 }}>
                      <Box sx={{ minWidth: 0, flex: 1 }}>
                        <Typography sx={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontSize: 14, fontWeight: 500, color: "#fff" }}>{session.title}</Typography>
                        <Typography sx={{ mt: 0.5, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontSize: 12, color: "rgba(255,255,255,0.5)" }}>{textValue(latestMessage?.content) || "Empty chat"}</Typography>
                      </Box>
                      <Typography sx={{ flexShrink: 0, fontSize: 10, color: "rgba(255,255,255,0.4)" }}>{formatRelativeTime(session.updatedAt)}</Typography>
                    </Box>
                  </Box>
                );
              })}
            </Box>
          </Box>
        ) : null}

        {loadError ? (
          <Box sx={{ mt: 2, borderRadius: 4, border: "1px solid rgba(244,63,94,0.2)", bgcolor: "rgba(244,63,94,0.1)", px: 2, py: 1.5, color: "#ffe4e6" }}>
            <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1.5 }}>
              <MsIcon sx={{ fontSize: 20 }}>error</MsIcon>
              <Typography sx={{ fontSize: 14, lineHeight: 1.5 }}>{loadError}</Typography>
            </Box>
          </Box>
        ) : null}

        <Box sx={{ display: "flex", flex: 1, flexDirection: "column", minHeight: 0 }}>
          <Box sx={{ flex: 1, overflowY: "auto", py: 2 }}>
            {currentMessages.length === 0 ? (
              <Box sx={{ display: "flex", minHeight: "50vh", alignItems: "center", justifyContent: "center", px: 2, textAlign: "center" }}>
                <Box sx={{ maxWidth: 576, display: "flex", flexDirection: "column", gap: 2 }}>
                  <Box sx={{ mx: "auto", display: "flex", width: 64, height: 64, alignItems: "center", justifyContent: "center", borderRadius: 5, border: "1px solid rgba(255,255,255,0.1)", bgcolor: "rgba(255,255,255,0.05)", color: "rgba(255,255,255,0.8)" }}>
                    <MsIcon sx={{ fontSize: 30 }}>chat</MsIcon>
                  </Box>
                  <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                    <Typography component="h2" sx={{ fontSize: 24, fontWeight: 600, color: "#fff" }}>Start a conversation</Typography>
                    <Typography sx={{ fontSize: 14, lineHeight: 1.5, color: "rgba(255,255,255,0.6)" }}>
                      Simple chat interface to interact with any AI model from connected providers. Select a model and start chatting!
                    </Typography>
                  </Box>
                </Box>
              </Box>
            ) : null}

            <Box sx={{ mx: "auto", display: "flex", width: "100%", maxWidth: 768, flexDirection: "column", gap: 2, px: 2 }}>
              {currentMessages.map((message) => {
                const isUser = message.role === "user";
                const isAssistant = message.role === "assistant";
                const isStreaming = isAssistant && message.id === streamingMessageId && message.status === "streaming";
                const content = textValue(message.content) || (isAssistant ? streamingText : "");

                return (
                  <Box key={message.id} sx={{ display: "flex", width: "100%", justifyContent: isUser ? "flex-end" : "flex-start", mb: 3 }}>
                    <Box sx={{ maxWidth: "min(88%, 42rem)", ...(isUser ? { borderRadius: 6, bgcolor: "#2f2f2f", px: 2.5, py: 1.75, color: "#fff" } : { color: "rgba(255,255,255,0.9)" }) }}>
                      <Box sx={{ mb: 0.5, display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1.5 }}>
                        <Typography sx={{ fontSize: 12, fontWeight: 600 }}>{isUser ? "You" : activeModel?.name || "Assistant"}</Typography>
                      </Box>

                      {message.attachments?.length ? (
                        <Box sx={{ mb: 1.5, mt: 1, display: "grid", gridTemplateColumns: { xs: "repeat(2, 1fr)", sm: "repeat(3, 1fr)" }, gap: 1 }}>
                          {message.attachments.map((attachment) => (
                            <Box key={attachment.id} component="a" href={attachment.dataUrl} target="_blank" rel="noreferrer" sx={{ overflow: "hidden", borderRadius: 4, border: "1px solid rgba(255,255,255,0.1)", bgcolor: "rgba(0,0,0,0.2)" }}>
                              <Image src={attachment.dataUrl} alt={attachment.name} width={0} height={0} sizes="100vw" style={{ width: "100%", height: "112px", objectFit: "cover" }} unoptimized />
                            </Box>
                          ))}
                        </Box>
                      ) : null}

                      <Box sx={{ whiteSpace: "pre-wrap", overflowWrap: "break-word", fontSize: 15, lineHeight: 1.75 }}>
                        {content}
                        {isAssistant && isStreaming && !streamingText ? <Box component="span" sx={{ display: "inline-block", animation: "pulse 1.5s cubic-bezier(0.4, 0, 0.6, 1) infinite" }}>▋</Box> : null}
                      </Box>
                    </Box>
                  </Box>
                );
              })}
            </Box>
          </Box>

          <Box sx={{ flexShrink: 0, pt: 1 }}>
            {attachments.length > 0 ? (
              <Box sx={{ mx: "auto", mb: 1.5, display: "flex", width: "100%", maxWidth: 768, flexWrap: "wrap", gap: 1, px: 2 }}>
                {attachments.map((attachment) => (
                  <Box key={attachment.id} sx={{ display: "flex", alignItems: "center", gap: 1, borderRadius: 999, border: "1px solid rgba(255,255,255,0.1)", bgcolor: "rgba(255,255,255,0.05)", px: 1.5, py: 1 }}>
                    <Typography sx={{ maxWidth: 192, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontSize: 12, color: "rgba(255,255,255,0.8)" }}>{attachment.name}</Typography>
                    <Box component="button" type="button" onClick={() => removeAttachment(attachment.id)} aria-label="Remove attachment" sx={{ color: "rgba(255,255,255,0.55)", border: 0, bgcolor: "transparent", cursor: "pointer", "&:hover": { color: "#fff" } }}>
                      <MsIcon sx={{ fontSize: 18 }}>close</MsIcon>
                    </Box>
                  </Box>
                ))}
              </Box>
            ) : null}

            <Box sx={{ mx: "auto", width: "100%", maxWidth: 768, px: 2, pb: 1 }}>
              <Box sx={{ borderRadius: 6, bgcolor: "#2f2f2f", px: 1.5, pt: 1.5, pb: 1, boxShadow: "0 0 15px rgba(0,0,0,0.10)", border: "1px solid rgba(255,255,255,0.05)" }}>
                <Box
                  component="textarea"
                  value={draft}
                  onChange={(event) => setDraft(event.target.value)}
                  onKeyDown={handleKeyDown}
                  placeholder="Message AI"
                  aria-label="Message AI"
                  rows={1}
                  sx={{ width: "100%", resize: "none", bgcolor: "transparent", px: 1, fontSize: 15, lineHeight: 1.5, color: "#fff", outline: "none", border: 0, maxHeight: "25vh", overflowY: "auto", "&::placeholder": { color: "rgba(255,255,255,0.4)" } }}
                />

                <Box sx={{ mt: 1, display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1.5 }}>
                  <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                    <Box component="button" type="button" onClick={() => fileInputRef.current?.click()} disabled={!activeModel || loadingData} sx={{ p: 1, color: "rgba(255,255,255,0.5)", border: 0, bgcolor: "transparent", cursor: "pointer", borderRadius: 999, transition: "background-color 150ms ease, color 150ms ease", "&:hover": { color: "#fff", bgcolor: "rgba(255,255,255,0.05)" }, "&:disabled": { cursor: "not-allowed", opacity: 0.5 } }}>
                      <MsIcon sx={{ fontSize: 20 }}>attach_file</MsIcon>
                    </Box>
                    <Box component="input" ref={fileInputRef} type="file" accept="image/*" multiple onChange={handleAttachFiles} aria-label="Attach images" sx={{ display: "none" }} />
                    <Typography sx={{ maxWidth: 120, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontSize: 12, fontWeight: 500, color: "rgba(255,255,255,0.3)" }}>{activeModel ? activeModel.name : "No model"}</Typography>
                  </Box>

                  <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                    {isSending ? (
                      <Box component="button" type="button" onClick={handleStop} sx={{ p: 1, color: "#fff", bgcolor: "rgba(255,255,255,0.1)", border: 0, cursor: "pointer", borderRadius: 999, width: 32, height: 32, display: "flex", alignItems: "center", justifyContent: "center", transition: "background-color 150ms ease", "&:hover": { bgcolor: "rgba(255,255,255,0.2)" } }}>
                        <MsIcon sx={{ fontSize: 16 }}>stop</MsIcon>
                      </Box>
                    ) : null}
                    <Box component="button" type="button" onClick={sendMessage} disabled={!canSend} sx={{ width: 32, height: 32, borderRadius: 999, display: "flex", alignItems: "center", justifyContent: "center", transition: "opacity 150ms ease", border: 0, cursor: canSend ? "pointer" : "not-allowed", bgcolor: canSend ? "#fff" : "rgba(255,255,255,0.1)", color: canSend ? "#000" : "rgba(255,255,255,0.3)", "&:hover": canSend ? { opacity: 0.9 } : {} }}>
                      <MsIcon sx={{ fontSize: 16 }}>arrow_upward</MsIcon>
                    </Box>
                  </Box>
                </Box>
              </Box>
            </Box>

            <Typography sx={{ mx: "auto", mt: 1, maxWidth: 768, px: 2, pb: 2, textAlign: "center", fontSize: 11, color: "rgba(255,255,255,0.3)" }}>
              Model list is filtered from connected providers.
            </Typography>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
