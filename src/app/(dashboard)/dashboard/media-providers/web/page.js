"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Card, Badge, Button } from "@/shared/components";
import ProviderIcon from "@/shared/components/ProviderIcon";
import { AI_PROVIDERS, getProvidersByKind } from "@/shared/constants/providers";
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

function getEffectiveStatus(conn) {
  const isCooldown = Object.entries(conn).some(
    ([k, v]) => k.startsWith("modelLock_") && v && new Date(v).getTime() > Date.now()
  );
  return conn.testStatus === "unavailable" && !isCooldown ? "active" : conn.testStatus;
}

function WebProviderCardStatus({ isNoAuth, allDisabled, total, connected, error }) {
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

function ProviderCard({ provider, kind, connections }) {
  const providerInfo = AI_PROVIDERS[provider.id];
  const isNoAuth = !!providerInfo?.noAuth;
  const providerConns = connections.filter((c) => c.provider === provider.id);
  const connected = providerConns.filter((c) => { const s = getEffectiveStatus(c); return s === "active" || s === "success"; }).length;
  const error = providerConns.filter((c) => { const s = getEffectiveStatus(c); return s === "error" || s === "expired" || s === "unavailable"; }).length;
  const total = providerConns.length;
  const allDisabled = total > 0 && providerConns.every((c) => c.isActive === false);

  return (
    <Box component={Link} href={`/dashboard/media-providers/${kind}/${provider.id}`} sx={{ display: "block", height: "100%" }}>
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
          <Box>
            <Typography component="h3" sx={{ fontWeight: 600, fontSize: 14 }}>{provider.name}</Typography>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1, mt: 0.25, flexWrap: "wrap" }}><WebProviderCardStatus isNoAuth={isNoAuth} allDisabled={allDisabled} total={total} connected={connected} error={error} /></Box>
          </Box>
        </Box>
      </Card>
    </Box>
  );
}

function ComboList({ combos }) {
  if (combos.length === 0) {
    return <Typography sx={{ fontSize: 12, color: "text.secondary", fontStyle: "italic" }}>No combos yet.</Typography>;
  }
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
              {/* Provider icons preview */}
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

function Section({ title, icon, kind, providers, connections, combos, onCreateCombo }) {
  return (
    <Box>
      {/* Header — title left, Create Combo right */}
      <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1.5, alignItems: { sm: "center" }, justifyContent: { sm: "space-between" }, mb: 1.5 }}>
        <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 1 }}>
          <MsIcon sx={{ color: "primary.main" }}>{icon}</MsIcon>
          <Typography component="h2" sx={{ fontSize: 16, fontWeight: 600 }}>{title}</Typography>
          <Box component="span" sx={{ fontSize: 12, color: "text.secondary" }}>({providers.length} providers · {combos.length} combos)</Box>
        </Box>
        <Button size="sm" icon="add" onClick={onCreateCombo}>Create Combo</Button>
      </Box>

      {/* Combos — top */}
      {combos.length > 0 && (
        <Box sx={{ mb: 2 }}>
          <ComboList combos={combos} />
        </Box>
      )}

      {/* Providers grid — bottom */}
      {providers.length === 0 ? (
        <Box sx={{ textAlign: "center", py: 4, border: "1px dashed", borderColor: "divider", borderRadius: 3, color: "text.secondary", fontSize: 14 }}>
          No providers.
        </Box>
      ) : (
        <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)", lg: "repeat(3, 1fr)", xl: "repeat(4, 1fr)" }, gap: 2 }}>
          {providers.map((p) => (
            <ProviderCard key={p.id} provider={p} kind={kind} connections={connections} />
          ))}
        </Box>
      )}
    </Box>
  );
}

export default function WebProvidersPage() {
  const router = useRouter();
  const [connections, setConnections] = useState([]);
  const [combos, setCombos] = useState([]);

  const fetchAll = async () => {
    try {
      const [connsRes, combosRes] = await Promise.all([
        fetch("/api/providers", { cache: "no-store" }),
        fetch("/api/combos", { cache: "no-store" }),
      ]);
      if (connsRes.ok) setConnections((await connsRes.json()).connections || []);
      if (combosRes.ok) setCombos((await combosRes.json()).combos || []);
    } catch { /* noop */ }
  };

  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => { fetchAll(); }, []);

  const searchProviders = getProvidersByKind("webSearch");
  const fetchProviders = getProvidersByKind("webFetch");
  const searchCombos = combos.filter((c) => c.kind === "webSearch");
  const fetchCombos = combos.filter((c) => c.kind === "webFetch");

  const handleCreateCombo = async (kind) => {
    // Generate unique default name
    const base = kind === "webSearch" ? "search-combo" : "fetch-combo";
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
    <Box sx={{ display: "flex", flexDirection: "column", gap: 4 }}>
      <Section
        title="Web Search" icon="search" kind="webSearch"
        providers={searchProviders} connections={connections} combos={searchCombos}
        onCreateCombo={() => handleCreateCombo("webSearch")}
      />

      {/* Divider between sections */}
      <Box sx={{ borderTop: 1, borderColor: "divider" }} />

      <Section
        title="Web Fetch" icon="travel_explore" kind="webFetch"
        providers={fetchProviders} connections={connections} combos={fetchCombos}
        onCreateCombo={() => handleCreateCombo("webFetch")}
      />
    </Box>
  );
}
