"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import OutlinedInput from "@mui/material/OutlinedInput";
import Dialog from "@mui/material/Dialog";
import DialogTitle from "@mui/material/DialogTitle";
import DialogContent from "@mui/material/DialogContent";
import DialogActions from "@mui/material/DialogActions";
import MuiButton from "@mui/material/Button";
import { Card, Button, Badge, Input, ModelSelectModal } from "@/shared/components";
import { TOOL_HOSTS } from "@/shared/constants/mitmToolHosts.cjs";
import Image from "next/image";

const EMPTY_ALIASES = {};
const EMPTY_STATUS = {};

function MsIcon({ children, sx }) {
  return (
    <Icon className="material-symbols-outlined" baseClassName="" sx={sx}>
      {children}
    </Icon>
  );
}

const compactInputSx = {
  width: "100%",
  minWidth: 0,
  fontSize: 12,
  bgcolor: "background.paper",
  "& .MuiOutlinedInput-input": { py: 1, px: 1 },
  "& .MuiOutlinedInput-notchedOutline": { borderColor: "divider" },
};

/**
 * Per-tool MITM card — shows DNS status + model mappings.
 * - Auto-saves model mapping on blur or modal select
 * - Skips sudo modal if password is already cached
 * - Model mappings can only be edited when DNS is active
 */
function MitmExpandedSection({ dnsActive, handleDnsToggle, handleMappingBlur, handleModelMappingChange, hasActiveProviders, loading, mitmHosts, modelMappings, openModelSelector, saveMappings, serverRunning, tool, warning }) {
  return (
    <Box sx={{ mt: 2, pt: 2, borderTop: 1, borderColor: "divider", display: "flex", flexDirection: "column", gap: 2 }}>
      {/* Hosts */}
      {mitmHosts.length > 0 && (
        <Box sx={{ mt: 1, borderRadius: "6px", border: 1, borderColor: "divider", bgcolor: "rgba(255,255,255,0.5)", px: 1, py: 0.75 }}>
          <Typography sx={{ fontSize: 10, fontWeight: 500, letterSpacing: 0.3, color: "text.primary", opacity: 0.8, mb: 0.5 }}>
            Edit hosts file manually to add the following entries:
          </Typography>
          <Box component="ul" sx={{ listStyle: "none", m: 0, p: 0, fontFamily: "ui-monospace, monospace", fontSize: 10, color: "text.secondary", wordBreak: "break-all", "& li + li": { mt: 0.25 } }}>
            {mitmHosts.map((h) => (
              <li key={h}>127.0.0.1 {h}</li>
            ))}
          </Box>
        </Box>
      )}
      {/* Info */}
      <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, fontSize: 11, color: "text.secondary", px: 1 }}>
        <Box component="p" sx={{ m: 0 }}>Toggle DNS to redirect {tool.name} traffic through VansAI via MITM.</Box>
        {!dnsActive && (
          <Typography sx={{ color: "#d97706", fontSize: 10, mt: 0.5 }}>
            ⚠️ Enable DNS to edit model mappings
          </Typography>
        )}
      </Box>

      {/* Model Mappings */}
      {tool.defaultModels?.length > 0 && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
          {tool.defaultModels.map((model) => (
            <Box key={model.alias} sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "9rem auto 1fr auto" }, gap: 0.75, alignItems: { sm: "center" }, columnGap: { sm: 1 } }}>
              <Box component="span" sx={{ fontSize: 12, fontWeight: 600, color: "text.primary", textAlign: { sm: "right" } }}>{model.name}</Box>
              <MsIcon sx={{ display: { xs: "none", sm: "inline" }, color: "text.secondary", fontSize: 14 }}>arrow_forward</MsIcon>
              <Box sx={{ position: "relative", width: "100%", minWidth: 0 }}>
                <OutlinedInput
                  type="text"
                  value={modelMappings[model.alias] || ""}
                  onChange={(e) => handleModelMappingChange(model.alias, e.target.value)}
                  onBlur={(e) => handleMappingBlur(model.alias, e.target.value)}
                  inputProps={{ "aria-label": "Model ID", placeholder: "provider/model-id" }}
                  disabled={!dnsActive}
                  size="small"
                  sx={{ ...compactInputSx, pr: 3.5, opacity: dnsActive ? 1 : 0.5 }}
                />
                {modelMappings[model.alias] && (
                  <MuiButton
                    type="button"
                    onClick={() => {
                      handleModelMappingChange(model.alias, "");
                      saveMappings({ ...modelMappings, [model.alias]: "" });
                    }}
                    title="Clear"
                    sx={{ position: "absolute", right: 2, top: "50%", transform: "translateY(-50%)", minWidth: 0, p: 0.25, color: "text.secondary", "&:hover": { color: "#ef4444", bgcolor: "transparent" } }}
                  >
                    <MsIcon sx={{ fontSize: 14 }}>close</MsIcon>
                  </MuiButton>
                )}
              </Box>
              <MuiButton
                type="button"
                onClick={() => openModelSelector(model.alias)}
                disabled={!hasActiveProviders || !dnsActive}
                variant="outlined"
                size="small"
                sx={{
                  fontSize: 12,
                  textTransform: "none",
                  py: { xs: 1, sm: 0.75 },
                  opacity: hasActiveProviders && dnsActive ? 1 : 0.5,
                  borderColor: "divider",
                  color: "text.primary",
                  bgcolor: "background.paper",
                  "&:hover": { borderColor: hasActiveProviders && dnsActive ? "primary.main" : "divider", bgcolor: "background.paper" },
                }}
              >
                Select
              </MuiButton>
            </Box>
          ))}
        </Box>
      )}

      {tool.defaultModels?.length === 0 && (
        <Typography sx={{ fontSize: 12, color: "text.secondary", px: 1 }}>Model mappings will be available soon.</Typography>
      )}

      {/* Start / Stop DNS button */}
      <Box sx={{ display: "flex", flexDirection: "column", gap: 1, alignItems: { sm: "flex-start" } }}>
        {dnsActive ? (
          <MuiButton
            type="button"
            variant="outlined"
            size="small"
            onClick={handleDnsToggle}
            disabled={!serverRunning || loading}
            startIcon={<MsIcon sx={{ fontSize: 16 }}>stop_circle</MsIcon>}
            sx={{ width: { xs: "100%", sm: "auto" }, fontSize: 12, fontWeight: 500, textTransform: "none", color: "#ef4444", borderColor: "rgba(239,68,68,0.30)", bgcolor: "rgba(239,68,68,0.10)", py: { sm: 0.75 }, "&:hover": { bgcolor: "rgba(239,68,68,0.20)", borderColor: "rgba(239,68,68,0.30)" } }}
          >
            Stop DNS
          </MuiButton>
        ) : (
          <MuiButton
            type="button"
            variant="outlined"
            size="small"
            onClick={handleDnsToggle}
            disabled={!serverRunning || loading}
            startIcon={<MsIcon sx={{ fontSize: 16 }}>play_circle</MsIcon>}
            sx={{ width: { xs: "100%", sm: "auto" }, fontSize: 12, fontWeight: 500, textTransform: "none", color: "primary.main", borderColor: "rgba(229,106,74,0.30)", bgcolor: "rgba(229,106,74,0.10)", py: { sm: 0.75 }, "&:hover": { bgcolor: "rgba(229,106,74,0.20)", borderColor: "rgba(229,106,74,0.30)" } }}
          >
            Start DNS
          </MuiButton>
        )}

        {/* Warning below button */}
        {warning && (
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, px: 1, py: 0.75, borderRadius: "4px", fontSize: 12, color: "#f59e0b" }}>
            <MsIcon sx={{ fontSize: 14 }}>warning</MsIcon>
            <Box component="span">{warning}</Box>
          </Box>
        )}
      </Box>
    </Box>
  );
}


export default function MitmToolCard({
  tool,
  status: { isExpanded, hasCachedPassword, needsSudoPassword, isWin } = EMPTY_STATUS,
  onToggle,
  serverRunning,
  dnsActive,
  apiKeys,
  activeProviders,
  hasActiveProviders,
  modelAliases = EMPTY_ALIASES,
  cloudEnabled,
  onDnsChange,
}) {
  const [loading, setLoading] = useState(false);
  const [warning, setWarning] = useState(null);
  const [showPasswordModal, setShowPasswordModal] = useState(false);
  const [sudoPassword, setSudoPassword] = useState("");
  const [pendingDnsAction, setPendingDnsAction] = useState(null);
  const [modalError, setModalError] = useState(null);
  const [modelMappings, setModelMappings] = useState({});
  const [modalOpen, setModalOpen] = useState(false);
  const [currentEditingAlias, setCurrentEditingAlias] = useState(null);

  const mitmHosts = TOOL_HOSTS[tool.id] ?? [];
  const canRunWithoutPassword = isWin || hasCachedPassword || needsSudoPassword === false;

  const loadSavedMappings = useCallback(async () => {
    try {
      const res = await fetch(`/api/cli-tools/antigravity-mitm/alias?tool=${tool.id}`);
      if (res.ok) {
        const data = await res.json();
        if (Object.keys(data.aliases || {}).length > 0) setModelMappings(data.aliases);
      }
    } catch { /* ignore */ }
  }, [tool.id]);

  const mappingsFetchedRef = useRef(false);

  const initializeCard = useCallback(async () => {
    if (!mappingsFetchedRef.current) {
      mappingsFetchedRef.current = true;
      await loadSavedMappings();
    }
  }, [loadSavedMappings]);

  const handleToggle = useCallback(() => {
    if (!isExpanded) initializeCard();
    onToggle();
  }, [isExpanded, initializeCard, onToggle]);

  useEffect(() => { initializeCard(); }, [initializeCard]);

  const saveMappings = useCallback(async (mappings) => {
    try {
      await fetch("/api/cli-tools/antigravity-mitm/alias", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ tool: tool.id, mappings }),
      });
    } catch { /* ignore */ }
  }, [tool.id]);

  const handleMappingBlur = (alias, value) => {
    saveMappings({ ...modelMappings, [alias]: value });
  };

  const handleModelMappingChange = (alias, value) => {
    setModelMappings(prev => ({ ...prev, [alias]: value }));
  };

  const openModelSelector = (alias) => {
    setCurrentEditingAlias(alias);
    setModalOpen(true);
  };

  const handleModelSelect = (model) => {
    if (!currentEditingAlias || model.isPlaceholder) return;
    const updated = { ...modelMappings, [currentEditingAlias]: model.value };
    setModelMappings(updated);
    saveMappings(updated);
  };

  const handleDnsToggle = () => {
    if (!serverRunning) return;
    const action = dnsActive ? "disable" : "enable";
    if (canRunWithoutPassword) {
      doDnsAction(action, "");
    } else {
      setPendingDnsAction(action);
      setShowPasswordModal(true);
      setModalError(null);
    }
  };

  const doDnsAction = async (action, password) => {
    setLoading(true);
    setWarning(null);
    try {
      const res = await fetch("/api/cli-tools/antigravity-mitm", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ tool: tool.id, action, sudoPassword: password }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "Failed to toggle DNS");

      if (action === "enable") {
        setWarning(`Restart ${tool.name} to apply changes`);
      }

      setShowPasswordModal(false);
      setSudoPassword("");
      onDnsChange?.(data);
    } catch { /* ignore */ } finally {
      setLoading(false);
      setPendingDnsAction(null);
    }
  };

  const handleConfirmPassword = () => {
    if (!sudoPassword.trim()) {
      setModalError("Sudo password is required");
      return;
    }
    doDnsAction(pendingDnsAction, sudoPassword);
  };

  return (
    <>
      <Card padding="xs" sx={{ overflow: "hidden" }}>
        <Box
          component="button"
          type="button"
          onClick={onToggle}
          aria-expanded={isExpanded}
          aria-label="Toggle section"
          sx={{ display: "flex", width: "100%", alignItems: { xs: "flex-start", sm: "center" }, justifyContent: "space-between", gap: 1.5, cursor: "pointer", textAlign: "left", background: "none", border: 0, p: 0, color: "inherit" }}
        >
          <Box sx={{ display: "flex", minWidth: 0, alignItems: "center", gap: 1.5 }}>
            <Box sx={{ width: 32, height: 32, display: "flex", alignItems: "center", justifyContent: "center", flexShrink: 0 }}>
              <Image
                src={tool.image}
                alt={tool.name}
                width={32}
                height={32}
                style={{ width: 32, height: 32, objectFit: "contain", borderRadius: 8 }}
                sizes="32px"
                onError={(e) => { e.target.style.display = "none"; }}
                loading="lazy"
                decoding="async"
              />
            </Box>
            <Box sx={{ minWidth: 0 }}>
              <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 1 }}>
                <Typography component="h3" sx={{ fontWeight: 500, fontSize: 14 }}>{tool.name}</Typography>
                {!serverRunning ? (
                  <Badge variant="default" size="sm">Server off</Badge>
                ) : dnsActive ? (
                  <Badge variant="success" size="sm">Active</Badge>
                ) : (
                  <Badge variant="warning" size="sm">DNS off</Badge>
                )}
              </Box>
              <Typography sx={{ fontSize: 12, color: "text.secondary", overflow: { sm: "hidden" }, textOverflow: { sm: "ellipsis" }, whiteSpace: { sm: "nowrap" } }}>Intercept {tool.name} requests via MITM proxy</Typography>
            </Box>
          </Box>
          <MsIcon sx={{ color: "text.secondary", fontSize: 20, transition: "transform 150ms", transform: isExpanded ? "rotate(180deg)" : "none", flexShrink: 0 }}>
            expand_more
          </MsIcon>
        </Box>

      {isExpanded && <MitmExpandedSection dnsActive={dnsActive} handleDnsToggle={handleDnsToggle} handleMappingBlur={handleMappingBlur} handleModelMappingChange={handleModelMappingChange} hasActiveProviders={hasActiveProviders} loading={loading} mitmHosts={mitmHosts} modelMappings={modelMappings} openModelSelector={openModelSelector} saveMappings={saveMappings} serverRunning={serverRunning} tool={tool} warning={warning} />}
      </Card>

      {/* Password Modal */}
      <Dialog open={showPasswordModal} onClose={() => !loading && setShowPasswordModal(false)} maxWidth="xs" fullWidth>
        <DialogTitle sx={{ fontSize: 16, fontWeight: 600 }}>Sudo Password Required</DialogTitle>
        <DialogContent sx={{ display: "flex", flexDirection: "column", gap: 2, pt: "8px !important" }}>
          <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1.5, p: 1.5, bgcolor: "rgba(234,179,8,0.10)", border: 1, borderColor: "rgba(234,179,8,0.30)", borderRadius: "8px" }}>
            <MsIcon sx={{ color: "#eab308", fontSize: 20 }}>warning</MsIcon>
            <Typography sx={{ fontSize: 12, color: "text.secondary" }}>Required to modify /etc/hosts and flush DNS cache</Typography>
          </Box>
          <Input
            type="password"
            placeholder="Enter sudo password"
            value={sudoPassword}
            onChange={(e) => setSudoPassword(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter" && !loading) handleConfirmPassword(); }}
          />
          {modalError && (
            <Box sx={{ display: "flex", alignItems: "center", gap: 1, px: 1, py: 0.75, borderRadius: "4px", fontSize: 12, bgcolor: "rgba(239,68,68,0.10)", color: "#dc2626" }}>
              <MsIcon sx={{ fontSize: 14 }}>error</MsIcon>
              <Box component="span">{modalError}</Box>
            </Box>
          )}
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button variant="ghost" size="sm" onClick={() => { setShowPasswordModal(false); setSudoPassword(""); setModalError(null); }} disabled={loading}>
            Cancel
          </Button>
          <Button variant="primary" size="sm" onClick={handleConfirmPassword} loading={loading}>
            Confirm
          </Button>
        </DialogActions>
      </Dialog>

      {/* Model Select Modal */}
      {modalOpen && (
        <ModelSelectModal
          isOpen={modalOpen}
          onClose={() => setModalOpen(false)}
          onSelect={handleModelSelect}
          selectedModel={currentEditingAlias ? modelMappings[currentEditingAlias] : null}
          activeProviders={activeProviders}
          modelAliases={modelAliases}
          title={`Select model for ${currentEditingAlias}`}
        />
      )}
    </>
  );
}
