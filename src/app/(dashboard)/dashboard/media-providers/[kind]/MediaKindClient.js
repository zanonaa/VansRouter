"use client";

import { useParams, notFound, useRouter } from "next/navigation";
import Link from "next/link";
import { useEffect, useState } from "react";
import { Card, Badge, Button, Toggle, AddCustomEmbeddingModal } from "@/shared/components";
import ProviderIcon from "@/shared/components/ProviderIcon";
import { MEDIA_PROVIDER_KINDS, AI_PROVIDERS, getProvidersByKind } from "@/shared/constants/providers";
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

// Kinds that support combos (currently disabled for image/tts — temporarily hidden).
// webSearch/webFetch handled by /web page.
const COMBO_KINDS = new Set([]);
const COMBO_BASE_NAMES = { image: "image-combo", tts: "tts-combo" };

function getEffectiveStatus(conn) {
  const isCooldown = Object.entries(conn).some(
    ([k, v]) => k.startsWith("modelLock_") && v && new Date(v).getTime() > Date.now()
  );
  return conn.testStatus === "unavailable" && !isCooldown ? "active" : conn.testStatus;
}

function MediaProviderCardStatus({ isNoAuth, allDisabled, total, connected, error }) {
  if (isNoAuth) return <Badge variant="success" size="sm">Ready</Badge>;
  if (allDisabled) return <Badge variant="default" size="sm">Disabled</Badge>;
  if (total === 0) return <Box component="span" sx={{ fontSize: 12, color: "text.secondary" }}>No connections</Box>;
  return (
    <>
      {connected > 0 && <Badge variant="success" size="sm" dot>{connected} Connected</Badge>}
      {error > 0 && <Badge variant="error" size="sm" dot>{error} Error</Badge>}
      {connected === 0 && error === 0 && <Badge variant="default" size="sm">{total} Added</Badge>}
    </>
  );
}

function MediaProviderCard({ provider, kind, connections, isCustom, onToggle }) {
  const providerInfo = AI_PROVIDERS[provider.id];
  const isNoAuth = !!providerInfo?.noAuth;

  const providerConns = connections.filter((c) => c.provider === provider.id);
  const connected = providerConns.filter((c) => { const s = getEffectiveStatus(c); return s === "active" || s === "success"; }).length;
  const error = providerConns.filter((c) => { const s = getEffectiveStatus(c); return s === "error" || s === "expired" || s === "unavailable"; }).length;
  const total = providerConns.length;
  const allDisabled = total > 0 && providerConns.every((c) => c.isActive === false);

  const handleToggleClick = (e) => {
    e.preventDefault();
    e.stopPropagation();
    if (onToggle) onToggle(provider.id, allDisabled);
  };

  return (
    <Box
      component={Link}
      href={`/dashboard/media-providers/${kind}/${provider.id}`}
      sx={{
        display: "block",
        height: "100%",
        "& .provider-toggle": { opacity: { xs: 1, sm: 0 }, transition: "opacity 150ms ease" },
        "&:hover .provider-toggle": { opacity: 1 },
      }}
    >
      <Card
        padding="xs"
        sx={{
          height: "100%",
          transition: "background-color 150ms ease, border-color 150ms ease",
          cursor: "pointer",
          opacity: allDisabled ? 0.5 : 1,
          "&:hover": { bgcolor: (theme) => alpha(theme.palette.text.primary, 0.01) },
        }}
      >
        <Box sx={{ display: "flex", minWidth: 0, alignItems: "center", justifyContent: "space-between", gap: 1.5 }}>
          <Box sx={{ display: "flex", minWidth: 0, alignItems: "center", gap: 1.5 }}>
            <Box
              sx={{
                width: 32,
                height: 32,
                borderRadius: 2,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                flexShrink: 0,
                "& img": { objectFit: "contain", borderRadius: 2, maxWidth: 30, maxHeight: 30 },
              }}
              style={{ backgroundColor: `${provider.color?.length > 7 ? provider.color : (provider.color ?? "#888") + "15"}` }}
            >
              <ProviderIcon
                src={`/providers/${provider.id}.webp`}
                alt={provider.name}
                size={30}
                fallbackText={provider.textIcon || provider.id.slice(0, 2).toUpperCase()}
                fallbackColor={provider.color}
              />
            </Box>
            <Box sx={{ minWidth: 0 }}>
              <Typography component="h3" sx={{ fontWeight: 600, fontSize: 14 }}>{provider.name}</Typography>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1, mt: 0.25, flexWrap: "wrap" }}>
                {isCustom && <Badge variant="default" size="sm">Custom</Badge>}
                <MediaProviderCardStatus isNoAuth={isNoAuth} allDisabled={allDisabled} total={total} connected={connected} error={error} />
              </Box>
            </Box>
          </Box>
          {total > 0 && (
            <Box
              component="span"
              role="button"
              tabIndex={0}
              className="provider-toggle"
              sx={{ flexShrink: 0, cursor: "pointer", display: "inline-flex" }}
              onClick={handleToggleClick}
              aria-label={allDisabled ? "Enable provider" : "Disable provider"}
            >
              <Toggle
                size="sm"
                checked={!allDisabled}
                onChange={() => {}}
                title={allDisabled ? "Enable provider" : "Disable provider"}
              />
            </Box>
          )}
        </Box>
      </Card>
    </Box>
  );
}

function ComboList({ combos }) {
  if (combos.length === 0) return null;
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      {combos.map((combo) => (
        <Box key={combo.id} component={Link} href={`/dashboard/media-providers/combo/${combo.id}`} sx={{ display: "block" }}>
          <Card
            padding="xs"
            sx={{
              transition: "background-color 150ms ease",
              cursor: "pointer",
              "&:hover": { bgcolor: (theme) => alpha(theme.palette.text.primary, 0.02) },
            }}
          >
            <Box sx={{ display: "flex", minWidth: 0, alignItems: "center", gap: 1.5 }}>
              <MsIcon sx={{ color: "primary.main", fontSize: 18 }}>layers</MsIcon>
              <Box component="code" sx={{ fontSize: 14, fontFamily: "monospace", fontWeight: 500, flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{combo.name}</Box>
              <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 0.5, flexShrink: { sm: 0 } }}>
                {combo.models.slice(0, 6).map((entry, i) => {
                  const pid = typeof entry === "string" ? entry.split("/")[0] : "";
                  const p = AI_PROVIDERS[pid];
                  return (
                    <Box
                      key={`${entry}-${i}`}
                      title={p?.name || entry}
                      sx={{
                        width: 20,
                        height: 20,
                        borderRadius: 0.5,
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "center",
                        "& img": { objectFit: "contain", borderRadius: 0.5, maxWidth: 18, maxHeight: 18 },
                      }}
                      style={{ backgroundColor: `${(p?.color ?? "#888")}15` }}
                    >
                      <ProviderIcon
                        src={`/providers/${pid}.webp`}
                        alt={p?.name || pid}
                        size={18}
                        fallbackText={p?.textIcon || pid.slice(0, 2).toUpperCase()}
                        fallbackColor={p?.color}
                      />
                    </Box>
                  );
                })}
                {combo.models.length > 6 && (
                  <Box component="span" sx={{ fontSize: 10, color: "text.secondary", ml: 0.5 }}>+{combo.models.length - 6}</Box>
                )}
              </Box>
              <Box component="span" sx={{ fontSize: 11, color: "text.secondary", flexShrink: 0 }}>{combo.models.length}</Box>
              <MsIcon sx={{ color: "text.secondary", fontSize: 16 }}>chevron_right</MsIcon>
            </Box>
          </Card>
        </Box>
      ))}
    </Box>
  );
}

export default function MediaKindClient({ initialConnections, initialNodes, initialCombos }) {
  const { kind } = useParams();
  const router = useRouter();
  const [connections, setConnections] = useState(initialConnections || []);
  const [customNodes, setCustomNodes] = useState(initialNodes || []);
  const [combos, setCombos] = useState(initialCombos || []);
  const [showAddCustomEmbedding, setShowAddCustomEmbedding] = useState(false);

  // webSearch/webFetch listing pages are merged into /web — return null and let server handle
  const kindConfig = MEDIA_PROVIDER_KINDS.find((k) => k.id === kind);
  const isEmbedding = kind === "embedding";
  const supportsCombo = COMBO_KINDS.has(kind);

  if (kind === "webSearch" || kind === "webFetch") {
    router.replace("/dashboard/media-providers/web");
    return null;
  }
  if (!kindConfig) return notFound();

  const providers = getProvidersByKind(kind);
  const kindCombos = combos.filter((c) => c.kind === kind);

  // Map custom nodes to MediaProviderCard shape
  const customProviders = customNodes.map((n) => ({
    id: n.id,
    name: n.name || "Custom Embedding",
    color: "#6366F1",
    textIcon: "CE",
  }));

  const allProviders = [...providers, ...customProviders];

  const handleToggleProvider = async (providerId, newActive) => {
    const providerConns = connections.filter((c) => c.provider === providerId);
    setConnections((prev) =>
      prev.map((c) => (c.provider === providerId ? { ...c, isActive: newActive } : c))
    );
    await Promise.allSettled(
      providerConns.map((c) =>
        fetch(`/api/providers/${c.id}`, {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ isActive: newActive }),
        })
      )
    );
  };

  const handleCreateCombo = async () => {
    const base = COMBO_BASE_NAMES[kind] || `${kind}-combo`;
    let name = base;
    let i = 1;
    const existing = new Set(combos.map((c) => c.name));
    while (existing.has(name)) { name = `${base}-${i++}`; }
    const res = await fetch("/api/combos", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name, models: [], kind }),
    });
    if (res.ok) {
      const created = await res.json();
      router.push(`/dashboard/media-providers/combo/${created.id}`);
    } else {
      const err = await res.json();
      alert(err.error || "Failed to create combo");
    }
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      {(isEmbedding || supportsCombo) && (
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "flex-end", gap: 1 }}>
          {supportsCombo && (
            <Button size="sm" icon="add" onClick={handleCreateCombo}>Create Combo</Button>
          )}
          {isEmbedding && (
            <Button size="sm" icon="add" onClick={() => setShowAddCustomEmbedding(true)}>
              Add Custom Embedding
            </Button>
          )}
        </Box>
      )}

      {supportsCombo && kindCombos.length > 0 && (
        <ComboList combos={kindCombos} />
      )}

      {allProviders.length === 0 ? (
        <Box sx={{ textAlign: "center", py: 6, border: "1px dashed", borderColor: "divider", borderRadius: 3, color: "text.secondary", fontSize: 14 }}>
          No providers support <Box component="strong">{kindConfig.label}</Box> yet.
        </Box>
      ) : (
        <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)", lg: "repeat(3, 1fr)", xl: "repeat(4, 1fr)" }, gap: 2 }}>
          {providers.map((provider) => (
            <MediaProviderCard
              key={provider.id}
              provider={provider}
              kind={kind}
              connections={connections}
              onToggle={handleToggleProvider}
            />
          ))}
          {customProviders.map((provider) => (
            <MediaProviderCard
              key={provider.id}
              provider={provider}
              kind={kind}
              connections={connections}
              isCustom
              onToggle={handleToggleProvider}
            />
          ))}
        </Box>
      )}

      {isEmbedding && (
        <AddCustomEmbeddingModal
          isOpen={showAddCustomEmbedding}
          onClose={() => setShowAddCustomEmbedding(false)}
          onCreated={(node) => {
            setCustomNodes((prev) => [...prev, node]);
            setShowAddCustomEmbedding(false);
          }}
        />
      )}
    </Box>
  );
}
