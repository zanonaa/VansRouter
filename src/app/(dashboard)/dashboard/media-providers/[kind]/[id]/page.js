"use client";

import { useParams, notFound, useRouter } from "next/navigation";
import Link from "next/link";
import { useState, useEffect } from "react";
import { Card, Badge, Button, AddCustomEmbeddingModal, NoAuthProxyCard, ProviderInfoCard } from "@/shared/components";
import ProviderIcon from "@/shared/components/ProviderIcon";
import { MEDIA_PROVIDER_KINDS, AI_PROVIDERS, isCustomEmbeddingProvider } from "@/shared/constants/providers";
import ConnectionsCard from "@/app/(dashboard)/dashboard/providers/components/ConnectionsCard";
import ModelsCard from "@/app/(dashboard)/dashboard/providers/components/ModelsCard";
import { KIND_EXAMPLE_CONFIG } from "./components/exampleShared";
import { EmbeddingExampleCard } from "./components/EmbeddingExampleCard";
import { TtsExampleCard } from "./components/TtsExampleCard";
import { GenericExampleCard } from "./components/GenericExampleCard";
import { ExaSearchExampleCard } from "./components/ExaSearchExampleCard";
import { SttExampleCard } from "./components/SttExampleCard";
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

// MediaProviderDetailPage
export default function MediaProviderDetailPage() {
  const { kind, id } = useParams();
  const router = useRouter();
  const kindConfig = MEDIA_PROVIDER_KINDS.find((k) => k.id === kind);
  const isCustom = isCustomEmbeddingProvider(id) && kind === "embedding";

  const handleDeleteCustom = async () => {
    if (!confirm("Delete this Custom Embedding node?")) return;
    try {
      const res = await fetch(`/api/provider-nodes/${id}`, { method: "DELETE" });
      if (res.ok) router.push(`/dashboard/media-providers/${kind}`);
    } catch (error) {
      console.log("Error deleting custom embedding node:", error);
    }
  };

  const [customNode, setCustomNode] = useState(null);
  const [customLoading, setCustomLoading] = useState(isCustom);
  const [showEditModal, setShowEditModal] = useState(false);

  // Fetch custom node info from API for custom embedding nodes
  useEffect(() => {
    if (!isCustom) return;
    let cancelled = false;
    fetch("/api/provider-nodes", { cache: "no-store" })
      .then((r) => r.json())
      .then((d) => {
        if (cancelled) return;
        setCustomNode((d.nodes || []).find((n) => n.id === id) || null);
        setCustomLoading(false);
      })
      .catch(() => { if (!cancelled) setCustomLoading(false); });
    return () => { cancelled = true; };
  }, [id, isCustom]);

  if (!kindConfig) return notFound();

  const builtInProvider = AI_PROVIDERS[id];

  // For custom embedding nodes, build a synthetic provider object
  const provider = isCustom
    ? (customNode ? { id, name: customNode.name || "Custom Embedding", color: "#6366F1", textIcon: "CE" } : null)
    : builtInProvider;

  if (!isCustom && !builtInProvider) return notFound();
  if (isCustom && !customLoading && !customNode) return notFound();
  if (isCustom && customLoading) {
    return <Box sx={{ color: "text.secondary", fontSize: 14, py: 6, textAlign: "center" }}>Loading...</Box>;
  }

  const kinds = isCustom ? ["embedding"] : (provider.serviceKinds ?? ["llm"]);
  if (!isCustom && !kinds.includes(kind)) return notFound();

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 4 }}>
      {/* Back */}
      <Box>
        <Box
          component={Link}
          href={`/dashboard/media-providers/${kind}`}
          sx={{
            display: "inline-flex",
            alignItems: "center",
            gap: 0.5,
            fontSize: 14,
            color: "text.secondary",
            mb: 2,
            transition: "color 150ms ease",
            "&:hover": { color: "primary.main" },
          }}
        >
          <MsIcon sx={{ fontSize: 20 }}>arrow_back</MsIcon>
          {kindConfig.label}
        </Box>

        {/* Header */}
        <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: { xs: 1.5, sm: 2 }, alignItems: { sm: "center" } }}>
          <Box
            sx={{
              width: 48,
              height: 48,
              borderRadius: 2,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              flexShrink: 0,
              "& img": { objectFit: "contain", borderRadius: 2, maxWidth: 48, maxHeight: 48 },
            }}
            style={{ backgroundColor: `${provider.color}15` }}
          >
            <ProviderIcon
              src={`/providers/${provider.id}.webp`}
              alt={provider.name}
              size={48}
              fallbackText={provider.textIcon || provider.id.slice(0, 2).toUpperCase()}
              fallbackColor={provider.color}
            />
          </Box>
          <Box sx={{ flex: 1 }}>
            <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: { xs: 1, sm: 1.5 } }}>
              <Typography component="h1" sx={{ fontSize: 30, fontWeight: 600, letterSpacing: "-0.01em" }}>{provider.name}</Typography>
              {!isCustom && provider.notice?.apiKeyUrl && (
                <Box
                  component="a"
                  href={provider.notice.apiKeyUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  sx={{ fontSize: 12, color: "primary.main", display: "inline-flex", alignItems: "center", gap: 0.5, "&:hover": { textDecoration: "underline" } }}
                >
                  <MsIcon sx={{ fontSize: 14 }}>open_in_new</MsIcon>
                  Get API Key
                </Box>
              )}
            </Box>
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.75, mt: 0.5, flexWrap: "wrap" }}>
              {isCustom && <Badge variant="default" size="sm">Custom · {customNode?.prefix}</Badge>}
              {kinds.map((k) => (
                <Badge key={k} variant={k === kind ? "primary" : "default"} size="sm">
                  {k.toUpperCase()}
                </Badge>
              ))}
            </Box>
          </Box>
          {isCustom && (
            <Box sx={{ display: "flex", width: { xs: "100%", sm: "auto" }, flexDirection: { xs: "column", sm: "row" }, gap: 1, alignItems: { sm: "center" } }}>
              <Button size="sm" variant="secondary" icon="edit" onClick={() => setShowEditModal(true)}>
                Edit
              </Button>
              <Button size="sm" variant="secondary" icon="delete" onClick={handleDeleteCustom}>
                Delete
              </Button>
            </Box>
          )}
        </Box>
      </Box>

      {/* Kind-specific notice (e.g. codex/image requires Plus) */}
      {!isCustom && provider.kindNotice?.[kind] && (
        <Box
          sx={(theme) => ({
            display: "flex",
            alignItems: "flex-start",
            gap: 1.5,
            px: 2,
            py: 1.5,
            borderRadius: 2,
            bgcolor: alpha(theme.palette.warning.main, 0.1),
            border: `1px solid ${alpha(theme.palette.warning.main, 0.3)}`,
            color: theme.palette.mode === "dark" ? "#fbbf24" : "#b45309",
          })}
        >
          <MsIcon sx={{ fontSize: 20, mt: 0.25 }}>warning</MsIcon>
          <Typography sx={{ fontSize: 14 }}>{provider.kindNotice[kind]}</Typography>
        </Box>
      )}

      {/* Provider notice text (only when there's actual text content) */}
      {!isCustom && provider.notice?.text && !provider.deprecated && (
        <Box
          sx={(theme) => ({
            display: "flex",
            flexDirection: { xs: "column", sm: "row" },
            gap: 1,
            borderRadius: 2,
            border: `1px solid ${alpha(theme.palette.info.main, 0.3)}`,
            bgcolor: alpha(theme.palette.info.main, 0.1),
            px: 1.5,
            py: 1,
            alignItems: { sm: "center" },
          })}
        >
          <MsIcon sx={{ fontSize: 16, color: "info.main", flexShrink: 0 }}>info</MsIcon>
          <Typography sx={(theme) => ({ minWidth: 0, flex: 1, fontSize: 12, lineHeight: 1.625, color: theme.palette.mode === "dark" ? "#60a5fa" : "#2563eb" })}>{provider.notice.text}</Typography>
          {provider.notice.apiKeyUrl && (
            <Box
              component="a"
              href={provider.notice.apiKeyUrl}
              target="_blank"
              rel="noopener noreferrer"
              sx={{
                display: "inline-flex",
                justifyContent: "center",
                borderRadius: 0.5,
                bgcolor: "info.main",
                px: 1,
                py: 0.5,
                fontSize: 12,
                fontWeight: 500,
                color: "#fff",
                textDecoration: "none",
                transition: "background-color 150ms ease",
                "&:hover": { bgcolor: "info.dark" },
                py: { xs: 0.5, sm: 0.25 },
              }}
            >
              Get API Key →
            </Box>
          )}
        </Box>
      )}

      {/* Connections */}
      {!isCustom && provider.noAuth ? (
        <NoAuthProxyCard providerId={id} />
      ) : (
        <ConnectionsCard providerId={id} isOAuth={false} />
      )}

      {/* Models - hidden for tts/webSearch/webFetch (provider IS the model); custom uses prefix as alias */}
      {kind !== "tts" && kind !== "webSearch" && kind !== "webFetch" && (
        <ModelsCard
          providerId={id}
          kindFilter={kind}
          providerAliasOverride={isCustom ? customNode?.prefix : undefined}
        />
      )}

      {/* Provider Info — config-driven, supports searchConfig, fetchConfig, ttsConfig, embeddingConfig, searchViaChat */}
      {!isCustom && (provider.searchConfig || provider.fetchConfig || provider.ttsConfig || provider.sttConfig || provider.embeddingConfig || provider.searchViaChat) && (
        <ProviderInfoCard
          config={
            kind === "webFetch" ? provider.fetchConfig
              : kind === "tts" ? provider.ttsConfig
              : kind === "stt" ? provider.sttConfig
              : kind === "embedding" ? provider.embeddingConfig
              : provider.searchConfig || { mode: "chat-completions", defaultModel: provider.searchViaChat?.defaultModel, pricingUrl: provider.searchViaChat?.pricingUrl, freeTier: provider.searchViaChat?.freeTier }
          }
          provider={provider}
          title={`${kindConfig.label} Config`}
        />
      )}

      {/* Example — per kind */}
      {kind === "embedding" && (
        <EmbeddingExampleCard providerId={id} customAlias={customNode?.prefix} />
      )}
      {kind === "tts" && <TtsExampleCard providerId={id} />}
      {kind === "stt" && !isCustom && <SttExampleCard providerId={id} />}
      {!isCustom && kind === "webSearch" && id === "exa" && <ExaSearchExampleCard providerId={id} />}
      {!isCustom && KIND_EXAMPLE_CONFIG[kind] && (kind !== "webSearch" || id !== "exa") && <GenericExampleCard providerId={id} kind={kind} />}

      {isCustom && (
        <AddCustomEmbeddingModal
          isOpen={showEditModal}
          node={customNode}
          onClose={() => setShowEditModal(false)}
          onSaved={(updated) => {
            setCustomNode(updated);
            setShowEditModal(false);
          }}
        />
      )}
    </Box>
  );
}
