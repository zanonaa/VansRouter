"use client";

import { useEffect, useState } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Card from "./Card";
import Select from "./Select";
import Badge from "./Badge";

const NONE_PROXY_POOL_VALUE = "__none__";

export default function NoAuthProxyCard({ providerId, isFreeNoAuth = true }) {
  const [proxyPools, setProxyPools] = useState([]);
  const [proxyPoolId, setProxyPoolId] = useState(NONE_PROXY_POOL_VALUE);
  const [rotateStrategy, setRotateStrategy] = useState("none");
  const [targetProxyPoolIds, setTargetProxyPoolIds] = useState([]);
  const [saving, setSaving] = useState(false);
  const [savedFlash, setSavedFlash] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    Promise.all([
      fetch("/api/proxy-pools?isActive=true", { cache: "no-store", signal: controller.signal }).then((r) => r.ok ? r.json() : { proxyPools: [] }),
      fetch("/api/settings", { cache: "no-store", signal: controller.signal }).then((r) => r.ok ? r.json() : {}),
    ]).then(([poolData, settingsData]) => {
      if (controller.signal.aborted) return;
      setProxyPools(poolData.proxyPools || []);
      const override = (settingsData.providerStrategies || {})[providerId] || {};
      setProxyPoolId(override.proxyPoolId || NONE_PROXY_POOL_VALUE);
      setRotateStrategy(override.rotateStrategy || "none");
      setTargetProxyPoolIds(Array.isArray(override.targetProxyPoolIds) ? override.targetProxyPoolIds : []);
    }).catch(() => {});
    return () => controller.abort();
  }, [providerId]);

  const handleSave = async (updatedFields) => {
    setSaving(true);
    try {
      const res = await fetch("/api/settings", { cache: "no-store" });
      const data = res.ok ? await res.json() : {};
      const current = data.providerStrategies || {};
      const override = { ...(current[providerId] || {}) };

      if ("rotateStrategy" in updatedFields) {
        if (updatedFields.rotateStrategy === "none") {
          delete override.rotateStrategy;
        } else {
          override.rotateStrategy = updatedFields.rotateStrategy;
        }
      }
      if ("proxyPoolId" in updatedFields) {
        if (updatedFields.proxyPoolId === NONE_PROXY_POOL_VALUE) {
          delete override.proxyPoolId;
        } else {
          override.proxyPoolId = updatedFields.proxyPoolId;
        }
      }
      if ("targetProxyPoolIds" in updatedFields) {
        const ids = Array.isArray(updatedFields.targetProxyPoolIds) ? updatedFields.targetProxyPoolIds.filter(Boolean) : [];
        if (ids.length === 0) delete override.targetProxyPoolIds;
        else override.targetProxyPoolIds = ids;
      }

      const updated = { ...current };
      if (Object.keys(override).length === 0) {
        delete updated[providerId];
      } else {
        updated[providerId] = override;
      }

      await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ providerStrategies: updated }),
      });
      setSavedFlash(true);
      setTimeout(() => setSavedFlash(false), 1500);
    } catch (e) {
      console.log("Save proxy override error:", e);
    } finally {
      setSaving(false);
    }
  };

  const handleStrategyChange = (newVal) => {
    setRotateStrategy(newVal);
    handleSave({ rotateStrategy: newVal });
  };

  const handlePoolChange = (newVal) => {
    setProxyPoolId(newVal);
    handleSave({ proxyPoolId: newVal });
  };

  const toggleTargetPool = (poolId) => {
    const next = targetProxyPoolIds.includes(poolId)
      ? targetProxyPoolIds.filter((id) => id !== poolId)
      : [...targetProxyPoolIds, poolId];
    setTargetProxyPoolIds(next);
    handleSave({ targetProxyPoolIds: next });
  };

  return (
    <Card>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, mb: 2 }}>
        <Box sx={{ display: "inline-flex", alignItems: "center", justifyContent: "center", width: 40, height: 40, borderRadius: "50%", bgcolor: "rgba(34,197,94,0.1)", color: "#22c55e" }}>
          <span className="material-symbols-outlined" style={{ fontSize: 20 }}>lan</span>
        </Box>
        <Box sx={{ flex: 1 }}>
          <Typography sx={{ fontSize: 14, fontWeight: 500 }}>Proxy Routing &amp; Strategy</Typography>
          <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
            Configure how traffic for this provider is routed through proxy pools.
          </Typography>
        </Box>
        {savedFlash && <Badge variant="success" size="sm">Saved</Badge>}
      </Box>

      <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" }, gap: 2 }}>
        <Select
          label="Proxy Strategy"
          value={rotateStrategy}
          onChange={(e) => handleStrategyChange(e.target.value)}
          disabled={saving}
          options={[
            { value: "none", label: "None (Direct / Static)" },
            { value: "round-robin", label: "Round-Robin (Rotate active pools)" },
            { value: "random", label: "Random (Rotate active pools)" },
            { value: "smart", label: "Smart (Skip unfit pools)" },
          ]}
        />

        {rotateStrategy === "none" ? (
          isFreeNoAuth ? (
            <Select
              label="Static Proxy Pool"
              value={proxyPoolId}
              onChange={(e) => handlePoolChange(e.target.value)}
              disabled={saving}
              options={[
                { value: NONE_PROXY_POOL_VALUE, label: "None (direct)" },
                ...proxyPools.map((pool) => ({ value: pool.id, label: pool.name })),
              ]}
            />
          ) : (
            <Box sx={{ display: "flex", flexDirection: "column", justifyContent: "flex-end", pb: 0.75 }}>
              <Box component="span" sx={{ fontSize: 12, color: "text.secondary", fontStyle: "italic" }}>
                Using connection-level proxy settings. Select specific proxies on connections below or click &quot;Apply Proxy&quot;.
              </Box>
            </Box>
          )
        ) : (
          <Box sx={{ display: "flex", flexDirection: "column", justifyContent: "flex-end", pb: 0.75 }}>
            <Box component="span" sx={{ fontSize: 12, color: "var(--color-brand-500)", fontWeight: 500 }}>
              🔄 Dynamic Rotation active. Individual connection proxy settings will be bypassed.
            </Box>
          </Box>
        )}
      </Box>

      {rotateStrategy !== "none" && (
            <Box sx={{ mt: 2, borderRadius: "12px", border: "1px solid var(--color-border)", bgcolor: "color-mix(in srgb, var(--color-surface) 40%, transparent)", p: 2 }}>
          <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1.5, mb: 1.5 }}>
            <Box>
              <Typography sx={{ fontSize: 14, fontWeight: 500 }}>Rotation proxy pool subset</Typography>
              <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
                Select which active pools are eligible for {rotateStrategy}. Leave empty to use all active pools.
              </Typography>
            </Box>
            <Badge variant="secondary" size="sm">
              {targetProxyPoolIds.length || proxyPools.length} / {proxyPools.length || 0} pools
            </Badge>
          </Box>
          {proxyPools.length === 0 ? (
            <Typography sx={{ fontSize: 12, color: "warning.main" }}>No active proxy pools with URLs found.</Typography>
          ) : (
            <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" }, gap: 1 }}>
              {proxyPools.map((pool) => {
                const checked = targetProxyPoolIds.includes(pool.id);
                return (
                  <Box
                    component="label"
                    key={pool.id}
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      gap: 1.5,
                      borderRadius: "8px",
                      border: "1px solid",
                      px: 1.5,
                      py: 1,
                      fontSize: 14,
                      cursor: "pointer",
                      transition: "all 150ms",
                      borderColor: checked ? "rgba(229,106,74,0.6)" : "var(--color-border)",
                      bgcolor: checked ? "rgba(229,106,74,0.1)" : "transparent",
                      "&:hover": { bgcolor: checked ? "rgba(229,106,74,0.15)" : "var(--color-surface-2)" },
                    }}
                  >
                    <input
                      type="checkbox"
                      checked={checked}
                      disabled={saving}
                      onChange={() => toggleTargetPool(pool.id)}
                    />
                    <Box component="span" sx={{ minWidth: 0, flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{pool.name || pool.id}</Box>
                    <Box component="span" sx={{ fontSize: 10, color: "text.secondary", fontFamily: "ui-monospace, monospace" }}>{pool.id.slice(0, 8)}</Box>
                  </Box>
                );
              })}
            </Box>
          )}
        </Box>
      )}
    </Card>
  );
}
