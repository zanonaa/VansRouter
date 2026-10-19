"use client";

import { useState, useEffect, useRef } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import { getStatusVariant as getConnectionStatusVariant } from "@/shared/utils/connectionStatus";
import { Badge, Toggle, Tooltip } from "@/shared/components";
import CooldownTimer from "./CooldownTimer";

const hoverBg = (t) => (t.palette.mode === "dark" ? "rgba(255,255,255,0.02)" : "rgba(0,0,0,0.02)");
const controlHoverBg = (t) => (t.palette.mode === "dark" ? "rgba(255,255,255,0.05)" : "rgba(0,0,0,0.05)");

const arrowButtonSx = (disabled) => ({
  p: 0.25,
  border: 0,
  borderRadius: "4px",
  bgcolor: "transparent",
  cursor: disabled ? "not-allowed" : "pointer",
  color: disabled
    ? (t) => (t.palette.mode === "dark" ? "rgba(156,163,175,0.3)" : "rgba(107,114,128,0.3)")
    : "text.secondary",
  transition: "background-color 150ms ease, color 150ms ease",
  "&:hover": disabled ? {} : { bgcolor: "var(--color-sidebar)", color: "primary.main" },
});

const controlButtonSx = (active) => ({
  display: "flex",
  width: { xs: "100%", sm: "auto" },
  flexDirection: "column",
  alignItems: "center",
  borderRadius: "4px",
  px: 1,
  py: 0.5,
  border: 0,
  bgcolor: "transparent",
  cursor: "pointer",
  transition: "background-color 150ms ease, color 150ms ease",
  color: active ? "primary.main" : "text.secondary",
  "&:hover": { bgcolor: controlHoverBg, color: "primary.main" },
});

const deleteButtonSx = {
  display: "flex",
  width: { xs: "100%", sm: "auto" },
  flexDirection: "column",
  alignItems: "center",
  borderRadius: "4px",
  px: 1,
  py: 0.5,
  border: 0,
  bgcolor: "transparent",
  cursor: "pointer",
  color: "#ef4444",
  transition: "background-color 150ms ease",
  "&:hover": { bgcolor: "rgba(239,68,68,0.1)" },
};

const dropdownItemSx = (active, disabled) => ({
  width: "100%",
  px: 1.5,
  py: 0.75,
  textAlign: "left",
  border: 0,
  bgcolor: "transparent",
  cursor: disabled ? "not-allowed" : "pointer",
  fontSize: 14,
  color: active ? "primary.main" : "text.primary",
  fontWeight: active ? 500 : 400,
  opacity: disabled ? 0.5 : 1,
  "&:hover": disabled ? {} : { bgcolor: controlHoverBg },
});

export default function ConnectionRow({ connection, proxyPools, isOAuth, isFirst, isLast, onMoveUp, onMoveDown, onToggleActive, onUpdateProxy, onEdit, onDelete, oneByOneStatus = null, autoPing = null, modelAssignmentOptions = null, onModelAssignmentChange = null, strictModelAssignment = false }) {
  const [showProxyDropdown, setShowProxyDropdown] = useState(false);
  const [updatingProxy, setUpdatingProxy] = useState(false);
  const proxyDropdownRef = useRef(null);

  const proxyPoolMap = new Map((proxyPools || []).map((pool) => [pool.id, pool]));
  const selectedProxyIds = connection.providerSpecificData?.proxyPoolIds || (connection.providerSpecificData?.proxyPoolId ? [connection.providerSpecificData.proxyPoolId] : []);
  const rotationStrategy = connection.providerSpecificData?.proxyRotationStrategy || "none";
  const boundProxyPoolId = selectedProxyIds[0] || connection.providerSpecificData?.proxyPoolId || null;
  const boundProxyPool = boundProxyPoolId ? proxyPoolMap.get(boundProxyPoolId) : null;
  const hasLegacyProxy = connection.providerSpecificData?.connectionProxyEnabled === true && !!connection.providerSpecificData?.connectionProxyUrl;
  const hasAnyProxy = !!boundProxyPoolId || hasLegacyProxy;
  const proxyDisplayText = selectedProxyIds.length > 1
    ? `${selectedProxyIds.length} pools (${rotationStrategy})`
    : boundProxyPool
    ? `Pool: ${boundProxyPool.name}`
    : boundProxyPoolId
      ? `Pool: ${boundProxyPoolId} (inactive/missing)`
      : hasLegacyProxy
        ? `Legacy: ${connection.providerSpecificData?.connectionProxyUrl}`
        : "";
  const autoPingTooltip = autoPing?.provider === "codex"
    ? "Auto-starts the next 5h Codex window after reset by sending a tiny gpt-5.5 request. Consumes a small amount of quota."
    : "When your 5h quota runs out, auto-sends a request the moment it resets so a new window starts right away.";

  let maskedProxyUrl = "";
  if (boundProxyPool?.proxyUrl || connection.providerSpecificData?.connectionProxyUrl) {
    const rawProxyUrl = boundProxyPool?.proxyUrl || connection.providerSpecificData?.connectionProxyUrl;
    try {
      const parsed = new URL(rawProxyUrl);
      maskedProxyUrl = `${parsed.protocol}//${parsed.hostname}${parsed.port ? `:${parsed.port}` : ""}`;
    } catch {
      maskedProxyUrl = rawProxyUrl;
    }
  }

  const noProxyText = boundProxyPool?.noProxy || connection.providerSpecificData?.connectionNoProxy || "";

  let proxyBadgeVariant = "default";
  if (boundProxyPool?.isActive === true) {
    proxyBadgeVariant = "success";
  } else if (boundProxyPoolId || hasLegacyProxy) {
    proxyBadgeVariant = "error";
  }

  // Close dropdown when clicking outside
  useEffect(() => {
    if (!showProxyDropdown) return;
    const handler = (e) => {
      if (proxyDropdownRef.current && !proxyDropdownRef.current.contains(e.target)) {
        setShowProxyDropdown(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [showProxyDropdown]);

  const handleSelectProxy = async (poolId) => {
    setUpdatingProxy(true);
    try {
      if (poolId === "__none__") {
        await onUpdateProxy(null);
      } else {
        await onUpdateProxy(poolId);
      }
      setShowProxyDropdown(false);
    } finally {
      setUpdatingProxy(false);
    }
  };

  const handleAdvancedProxy = async (strategy, ids) => {
    const activeIds = ids.filter((id) => proxyPoolMap.get(id)?.isActive === true);
    if (!activeIds.length) return;
    setUpdatingProxy(true);
    try {
      await onUpdateProxy({ proxyPoolIds: activeIds, proxyRotationStrategy: strategy });
      setShowProxyDropdown(false);
    } finally {
      setUpdatingProxy(false);
    }
  };

  const rowAuthType = connection.authType || (isOAuth ? "oauth" : "apikey");
  const isOAuthConnection = rowAuthType === "oauth";
  const isCookieConnection = rowAuthType === "cookie";
  const authIcon = isCookieConnection ? "cookie" : isOAuthConnection ? "lock" : "key";
  const authLabel = isOAuthConnection ? "OAuth" : isCookieConnection ? "Cookie" : "API Key";
  const displayName = connection.name?.trim()
    || connection.email?.trim()
    || connection.displayName?.trim()
    || (isOAuthConnection ? "OAuth Account" : isCookieConnection ? "Cookie Account" : "API Key");
  const secondaryDisplayName = connection.name?.trim() && connection.email?.trim() && connection.name.trim() !== connection.email.trim()
    ? connection.email.trim()
    : connection.name?.trim() && connection.displayName?.trim() && connection.name.trim() !== connection.displayName.trim()
      ? connection.displayName.trim()
      : null;

  // Use useState + useEffect for impure Date.now() to avoid calling during render
  const [isCooldown, setIsCooldown] = useState(false);

  // Get earliest model lock timestamp (useEffect handles the Date.now() comparison)
  const modelLockUntil = Object.entries(connection)
    .filter(([k]) => k.startsWith("modelLock_"))
    .map(([, v]) => v)
    .filter(v => !!v)
    .sort()[0] || null;

  useEffect(() => {
    let interval = null;
    const checkCooldown = () => {
      const until = Object.entries(connection)
        .filter(([k]) => k.startsWith("modelLock_"))
        .map(([, v]) => v)
        .filter(v => v && new Date(v).getTime() > Date.now())
        .sort()[0] || null;
      setIsCooldown(!!until);
    };

    checkCooldown();
    if (modelLockUntil) {
      interval = setInterval(checkCooldown, 1000);
    }
    return () => {
      if (interval) clearInterval(interval);
    };
  }, [modelLockUntil, connection]);

  // Determine effective status (override unavailable if cooldown expired)
  const effectiveStatus = (connection.testStatus === "unavailable" && !isCooldown)
    ? "active"  // Cooldown expired u2192 treat as active
    : connection.testStatus;

  const getStatusVariant = () => getConnectionStatusVariant(connection.isActive, effectiveStatus);

  const getOneByOneVariant = () => {
    if (!oneByOneStatus) return "default";
    if (oneByOneStatus.state === "success") return "success";
    if (oneByOneStatus.state === "failed") return "error";
    if (oneByOneStatus.state === "testing") return "primary";
    return "default";
  };

  const getOneByOneLabel = () => {
    if (!oneByOneStatus) return null;
    if (oneByOneStatus.state === "queued") return "queued";
    if (oneByOneStatus.state === "testing") return "testing";
    if (oneByOneStatus.state === "success") return "success";
    if (oneByOneStatus.state === "failed") return oneByOneStatus.error ? `failed: ${oneByOneStatus.error}` : "failed";
    return null;
  };

  return (
    <Box
      sx={{
        display: "flex",
        minWidth: 0,
        flexDirection: { xs: "column", sm: "row" },
        gap: 1.5,
        borderRadius: "8px",
        p: 1,
        opacity: connection.isActive === false ? 0.6 : 1,
        transition: "background-color 150ms ease",
        "&:hover": { bgcolor: hoverBg },
        alignItems: { sm: "center" },
        justifyContent: { sm: "space-between" },
      }}
    >
      <Box sx={{ display: "flex", minWidth: 0, flex: 1, alignItems: { xs: "flex-start", sm: "center" }, gap: { xs: 1, sm: 1.5 } }}>
        {/* Priority arrows */}
        <Box sx={{ display: "flex", flexShrink: 0, flexDirection: "column" }}>
          <Box
            component="button"
            onClick={onMoveUp}
            disabled={isFirst}
            sx={arrowButtonSx(isFirst)}
          >
            <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 14 }}>keyboard_arrow_up</Icon>
          </Box>
          <Box
            component="button"
            onClick={onMoveDown}
            disabled={isLast}
            sx={arrowButtonSx(isLast)}
          >
            <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 14 }}>keyboard_arrow_down</Icon>
          </Box>
        </Box>
        <Icon className="material-symbols-outlined" baseClassName="" sx={{ flexShrink: 0, fontSize: 16, color: "text.secondary" }}>
          {authIcon}
        </Icon>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography sx={{ fontSize: 14, fontWeight: 500, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{displayName}</Typography>
          {secondaryDisplayName && (
            <Typography sx={{ fontSize: 12, color: "text.secondary", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{secondaryDisplayName}</Typography>
          )}
          <Box sx={{ mt: 0.5, display: "flex", minWidth: 0, flexWrap: "wrap", alignItems: "center", gap: { xs: 0.75, sm: 1 } }}>
            <Badge variant={getStatusVariant()} size="sm" dot>
              {connection.isActive === false ? "disabled" : (effectiveStatus || "Unknown")}
            </Badge>
            <Badge variant="default" size="sm">
              {authLabel}
            </Badge>
            {hasAnyProxy && (
              <Badge variant={proxyBadgeVariant} size="sm">
                Proxy
              </Badge>
            )}
            {isCooldown && connection.isActive !== false && <CooldownTimer until={modelLockUntil} />}
            {connection.lastError && connection.isActive !== false && (
              <Box component="span" sx={{ maxWidth: { xs: "100%", sm: 300 }, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontSize: 12, color: "error.main" }} title={connection.lastError}>
                {connection.lastError}
              </Box>
            )}
            <Box component="span" sx={{ fontSize: 12, color: "text.secondary" }}>#{connection.priority}</Box>
            {connection.globalPriority && (
              <Box component="span" sx={{ fontSize: 12, color: "text.secondary" }}>Auto: {connection.globalPriority}</Box>
            )}
            {getOneByOneLabel() && (
              <Badge variant={getOneByOneVariant()} size="sm">
                {getOneByOneLabel()}
              </Badge>
            )}
          </Box>
          {hasAnyProxy && (
            <Box sx={{ mt: 0.5, display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
              <Box component="span" sx={{ maxWidth: { xs: "100%", sm: 420 }, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontSize: 11, color: "text.secondary" }} title={proxyDisplayText}>
                {proxyDisplayText}
              </Box>
              {maskedProxyUrl && (
                <Box component="code" sx={{ maxWidth: { xs: "100%", sm: 260 }, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", borderRadius: "4px", bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(255,255,255,0.05)" : "rgba(0,0,0,0.05)"), px: 0.5, py: 0.25, fontFamily: "monospace", fontSize: 10, color: "text.secondary" }}>
                  {maskedProxyUrl}
                </Box>
              )}
              {noProxyText && (
                <Box component="span" sx={{ maxWidth: { xs: "100%", sm: 320 }, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontSize: 11, color: "text.secondary" }} title={noProxyText}>
                  no_proxy: {noProxyText}
                </Box>
              )}
            </Box>
          )}
          {modelAssignmentOptions && onModelAssignmentChange && (
            <Box
              component="select"
              value={Object.prototype.hasOwnProperty.call(connection.providerSpecificData || {}, "assignedModel")
                ? (connection.providerSpecificData.assignedModel || "")
                : (connection.providerSpecificData?.freebuffModel || "")}
              onChange={(event) => onModelAssignmentChange(event.target.value)}
              disabled={!strictModelAssignment}
              sx={{
                mt: 1,
                maxWidth: "100%",
                borderRadius: "6px",
                border: 1,
                borderColor: "divider",
                bgcolor: "background.default",
                px: 1,
                py: 0.5,
                fontSize: 11,
                color: "text.primary",
              }}
              title="Model assignment"
            >
              <option value="">Unassigned</option>
              {modelAssignmentOptions.map((model) => (
                <option key={model.id} value={model.id}>{model.name || model.id}</option>
              ))}
            </Box>
          )}
        </Box>
      </Box>
      <Box sx={{ display: "flex", width: { xs: "100%", sm: "auto" }, alignItems: "center", justifyContent: { xs: "space-between", sm: "flex-end" }, gap: 1 }}>
        <Box sx={{ display: { xs: "grid", sm: "flex" }, flex: { xs: 1, sm: "none" }, gridTemplateColumns: "repeat(3, minmax(0,1fr))", gap: 0.5 }}>
          {/* Proxy button with inline dropdown */}
          {(proxyPools || []).length > 0 && (
            <Box sx={{ position: "relative" }} ref={proxyDropdownRef}>
               <Box
                 component="button"
                 type="button"
                 onClick={() => setShowProxyDropdown((v) => !v)}
                 sx={controlButtonSx(hasAnyProxy)}
                 disabled={updatingProxy}
               >
                 <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18 }}>
                   {updatingProxy ? "progress_activity" : "lan"}
                 </Icon>
                 <Box component="span" sx={{ fontSize: 10, lineHeight: 1.25 }}>Proxy</Box>
               </Box>
               {showProxyDropdown && (
                 <Box sx={{ position: "absolute", right: 0, top: "100%", zIndex: 50, mt: 0.5, maxWidth: "78vw", minWidth: 160, borderRadius: "8px", border: 1, borderColor: "divider", bgcolor: "background.default", py: 0.5, boxShadow: "0 10px 15px -3px rgba(0,0,0,0.1)" }}>
                   {rotationStrategy === "smart" && <Typography sx={{ px: 1.5, py: 0.5, fontSize: 10, color: "text.secondary" }}>Smart skips unfit pools for this provider/model.</Typography>}
                    <Box
                      component="button"
                      type="button"
                      onClick={() => handleSelectProxy("__none__")}
                      sx={dropdownItemSx(!boundProxyPoolId, false)}
                    >
                      None
                    </Box>
                    <Typography sx={{ px: 1.5, pt: 1, fontSize: 10, fontWeight: 500, textTransform: "uppercase", color: "text.secondary" }}>Rotation</Typography>
                    {["fill-first", "round-robin", "random", "smart"].map((strategy) => (
                       <Box
                         component="button"
                         type="button"
                         key={strategy}
                         onClick={() => handleAdvancedProxy(strategy, selectedProxyIds.length > 1 ? selectedProxyIds : (proxyPools || []).filter((pool) => pool.isActive).map((pool) => pool.id))}
                         disabled={updatingProxy}
                         sx={dropdownItemSx(rotationStrategy === strategy, updatingProxy)}
                       >
                         {strategy === "smart" ? "Smart (provider/model aware)" : strategy}
                       </Box>
                    ))}
                    <Typography sx={{ px: 1.5, pt: 1, fontSize: 10, fontWeight: 500, textTransform: "uppercase", color: "text.secondary" }}>Single pool</Typography>
                    {(proxyPools || []).map((pool) => (
                       <Box
                         component="button"
                         type="button"
                         key={pool.id}
                         onClick={() => handleSelectProxy(pool.id)}
                         disabled={updatingProxy || pool.isActive !== true}
                         sx={dropdownItemSx(boundProxyPoolId === pool.id, updatingProxy || pool.isActive !== true)}
                       >
                         {pool.name}
                       </Box>
                    ))}
                 </Box>
               )}
            </Box>
          )}
          {autoPing && (
            <Tooltip text={autoPingTooltip}>
              <Box
                component="button"
                onClick={() => autoPing.onToggle(!autoPing.on)}
                sx={controlButtonSx(autoPing.on)}
              >
                <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18 }}>bolt</Icon>
                <Box component="span" sx={{ fontSize: 10, lineHeight: 1.25 }}>Auto-ping</Box>
              </Box>
            </Tooltip>
          )}
          <Box component="button" onClick={onEdit} sx={controlButtonSx(false)}>
            <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18 }}>edit</Icon>
            <Box component="span" sx={{ fontSize: 10, lineHeight: 1.25 }}>Edit</Box>
          </Box>
          <Box component="button" onClick={onDelete} sx={deleteButtonSx}>
            <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18 }}>delete</Icon>
            <Box component="span" sx={{ fontSize: 10, lineHeight: 1.25 }}>Delete</Box>
          </Box>
        </Box>
        <Toggle
          size="sm"
          checked={connection.isActive ?? true}
          onChange={onToggleActive}
          title={(connection.isActive ?? true) ? "Disable connection" : "Enable connection"}
        />
      </Box>
    </Box>
  );
}
