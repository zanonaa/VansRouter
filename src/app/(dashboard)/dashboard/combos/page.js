"use client";

import { useState, useEffect, useCallback } from "react";
import { Card, Button, CardSkeleton, ModelSelectModal, ConfirmModal, CapacityBadges, Select, ComboFormModal } from "@/shared/components";
import { useCopyToClipboard } from "@/shared/hooks/useCopyToClipboard";
import { useModelCaps } from "@/shared/hooks/useModelCaps";
import { isOpenAICompatibleProvider, isAnthropicCompatibleProvider } from "@/shared/constants/providers";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import { alpha } from "@mui/material/styles";

function MsIcon({ children, sx, ...props }) {
  return <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18, ...sx }} {...props}>{children}</Icon>;
}

export default function CombosPage() {
  const [combos, setCombos] = useState([]);
  const [loading, setLoading] = useState(true);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [editingCombo, setEditingCombo] = useState(null);
  const [activeProviders, setActiveProviders] = useState([]);
  const [comboStrategies, setComboStrategies] = useState({});
  const [presetLoading, setPresetLoading] = useState(null);
  const [selectedIds, setSelectedIds] = useState([]);
  const [bulkBusy, setBulkBusy] = useState(false);
  const { getCaps } = useModelCaps();
  const [confirmState, setConfirmState] = useState(null);
  const { copied, copy } = useCopyToClipboard();

  /* eslint-disable react-hooks/immutability, react-hooks/set-state-in-effect --
     One-time bootstrap fetch on mount; fetchData is declared below. */
  useEffect(() => {
    fetchData();
  }, []);

  // Drop stale selection when the combo list changes (delete / refresh).
  useEffect(() => {
    const alive = new Set(combos.map((c) => c.id));
    setSelectedIds((prev) => prev.filter((id) => alive.has(id)));
  }, [combos]);

  const selectedCombos = combos.filter((c) => selectedIds.includes(c.id));
  const allSelected = combos.length > 0 && selectedIds.length === combos.length;
  const someSelected = selectedIds.length > 0;

  const toggleSelect = (id) => {
    setSelectedIds((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));
  };

  const toggleSelectAll = () => setSelectedIds(allSelected ? [] : combos.map((c) => c.id));
  const clearSelection = () => setSelectedIds([]);

  const fetchData = async () => {
    try {
      const [combosRes, providersRes, settingsRes] = await Promise.all([
        fetch("/api/combos", { cache: "no-store" }),
        fetch("/api/providers", { cache: "no-store" }),
        fetch("/api/settings", { cache: "no-store" }),
      ]);

      const combosData = await combosRes.json();
      const providersData = await providersRes.json();
      const settingsData = settingsRes.ok ? await settingsRes.json() : {};
      // Only LLM combos here - webSearch/webFetch combos belong to media-providers/web
      if (combosRes.ok) setCombos((combosData.combos || []).filter(c => !c.kind || c.kind === "llm"));
      if (providersRes.ok) setActiveProviders(providersData.connections || []);
      setComboStrategies(settingsData.comboStrategies || {});
    } catch (error) {
      console.log("Error fetching data:", error);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async (data) => {
    try {
      const res = await fetch("/api/combos", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(data),
      });
      if (res.ok) {
        await fetchData();
        setShowCreateModal(false);
      } else {
        const err = await res.json();
        alert(err.error || "Failed to create combo");
      }
    } catch (error) {
      console.log("Error creating combo:", error);
    }
  };

  const handleUpdate = async (id, data) => {
    try {
      const res = await fetch(`/api/combos/${id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(data),
      });
      if (res.ok) {
        await fetchData();
        setEditingCombo(null);
      } else {
        const err = await res.json();
        alert(err.error || "Failed to update combo");
      }
    } catch (error) {
      console.log("Error updating combo:", error);
    }
  };

  const handleGeneratePresets = async (source) => {
    const label = source === "cursor" ? "Cursor Default" : "Claude Default";
    setPresetLoading(source);
    try {
      const previewRes = await fetch(`/api/combos/presets?source=${source}`);
      const preview = await previewRes.json();
      if (!previewRes.ok) {
        alert(preview.error || `Failed to preview ${label}`);
        return;
      }
      const total = (preview.items || []).length;
      const toCreate = preview.toCreate ?? (preview.items || []).filter((i) => !i.exists).length;
      const toSkip = preview.toSkip ?? (preview.items || []).filter((i) => i.exists).length;
      if (total === 0) {
        alert(`No ${label} models available to generate.`);
        return;
      }
      if (toCreate === 0) {
        alert(`All ${total} ${label} combos already exist. Nothing to create.`);
        return;
      }
      setConfirmState({
        title: `Generate ${label}`,
        message: `Create ${toCreate} combo${toCreate === 1 ? "" : "s"} named like ${source === "cursor" ? "Cursor" : "Claude"} model IDs (seeded with cu/… or cc/…). ${toSkip} already exist and will be skipped. You can edit any combo afterward to add fallbacks.`,
        confirmText: "Generate",
        variant: "primary",
        onConfirm: async () => {
          setConfirmState((prev) => (prev ? { ...prev, loading: true } : null));
          try {
            const res = await fetch("/api/combos/presets", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ source }),
            });
            const data = await res.json();
            if (!res.ok) {
              alert(data.error || `Failed to generate ${label}`);
              setConfirmState((prev) => (prev ? { ...prev, loading: false } : null));
              return;
            }
            await fetchData();
            setConfirmState(null);
          } catch (error) {
            console.log(`Error generating ${label}:`, error);
            alert(`Failed to generate ${label}`);
            setConfirmState((prev) => (prev ? { ...prev, loading: false } : null));
          }
        },
      });
    } catch (error) {
      console.log(`Error previewing ${label}:`, error);
      alert(`Failed to preview ${label}`);
    } finally {
      setPresetLoading(null);
    }
  };

  const pruneStrategiesForNames = (names, base = comboStrategies) => {
    const updated = { ...base };
    for (const name of names) delete updated[name];
    return updated;
  };

  const persistComboStrategies = async (updated) => {
    await fetch("/api/settings", {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ comboStrategies: updated }),
    });
    setComboStrategies(updated);
  };

  const handleDelete = async (id) => {
    const combo = combos.find((c) => c.id === id);
    setConfirmState({
      title: "Delete Combo",
      message: combo ? `Delete combo "${combo.name}"?` : "Delete this combo?",
      onConfirm: async () => {
        setConfirmState((prev) => (prev ? { ...prev, loading: true } : null));
        try {
          const res = await fetch(`/api/combos/${id}`, { method: "DELETE" });
          if (res.ok) {
            if (combo?.name) await persistComboStrategies(pruneStrategiesForNames([combo.name]));
            setCombos((prev) => prev.filter((c) => c.id !== id));
            setSelectedIds((prev) => prev.filter((x) => x !== id));
          }
          setConfirmState(null);
        } catch (error) {
          console.log("Error deleting combo:", error);
          setConfirmState((prev) => (prev ? { ...prev, loading: false } : null));
        }
      }
    });
  };

  const handleBulkDelete = () => {
    if (selectedCombos.length === 0) return;
    const count = selectedCombos.length;
    setConfirmState({
      title: "Delete Selected Combos",
      message: `Delete ${count} selected combo${count === 1 ? "" : "s"}? This cannot be undone.`,
      confirmText: "Delete",
      variant: "danger",
      onConfirm: async () => {
        setConfirmState((prev) => (prev ? { ...prev, loading: true } : null));
        setBulkBusy(true);
        try {
          const ids = selectedCombos.map((c) => c.id);
          const results = await Promise.all(ids.map((id) => fetch(`/api/combos/${id}`, { method: "DELETE" })));
          const ok = selectedCombos.filter((_, i) => results[i]?.ok);
          const okIds = new Set(ok.map((c) => c.id));
          const failed = ids.length - ok.length;
          if (ok.length > 0) {
            await persistComboStrategies(pruneStrategiesForNames(ok.map((c) => c.name)));
            setCombos((prev) => prev.filter((c) => !okIds.has(c.id)));
            setSelectedIds((prev) => prev.filter((id) => !okIds.has(id)));
          }
          setConfirmState(null);
          if (failed > 0) alert(`Deleted with ${failed} failure${failed === 1 ? "" : "s"}.`);
        } catch (error) {
          console.log("Error bulk deleting combos:", error);
          alert("Failed to delete selected combos");
          setConfirmState((prev) => (prev ? { ...prev, loading: false } : null));
        } finally {
          setBulkBusy(false);
        }
      },
    });
  };

  // Merge a per-combo strategy patch into settings.comboStrategies. Passing an empty patch (strategy back to default "fallback") drops the entry entirely.
  const handleSetComboStrategy = async (comboName, patch) => {
    try {
      const updated = { ...comboStrategies };
      const next = { ...(updated[comboName] || {}), ...patch };
      if (!next.fallbackStrategy || next.fallbackStrategy === "fallback") delete updated[comboName];
      else updated[comboName] = next;
      await persistComboStrategies(updated);
    } catch (error) {
      console.log("Error updating combo strategy:", error);
    }
  };

  const handleBulkSetStrategy = async (strategy) => {
    if (selectedCombos.length === 0 || !strategy) return;
    setBulkBusy(true);
    try {
      const updated = { ...comboStrategies };
      for (const combo of selectedCombos) {
        if (strategy === "fallback") delete updated[combo.name];
        else updated[combo.name] = { ...(updated[combo.name] || {}), fallbackStrategy: strategy };
      }
      await persistComboStrategies(updated);
    } catch (error) {
      console.log("Error bulk updating combo strategy:", error);
      alert("Failed to update strategy for selected combos");
    } finally {
      setBulkBusy(false);
    }
  };

  if (loading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <CardSkeleton />
        <CardSkeleton />
      </Box>
    );
  }

  return (
    <Box sx={{ display: "flex", minWidth: 0, flexDirection: "column", gap: 3, px: { xs: 0.5, sm: 0 } }}>
      <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 2, alignItems: { sm: "flex-start" }, justifyContent: { sm: "space-between" } }}>
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Typography sx={{ fontSize: 14, color: "text.secondary" }}>Group models under one name, then pick a strategy per combo.</Typography>
          <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)" }, gap: 1, mt: 1.5 }}>
            {[["arrow_downward", "Fallback", "Tries models in order, next on failure"], ["sync", "Round Robin", "Rotates across requests to spread load"], ["hub", "Fusion", "Parallel query + judge synthesis (N+1 calls)"], ["auto_awesome", "Capacity auto-switch", "Routes image/PDF/audio to capable models"]].map(([icon, title, desc]) => (
              <Box key={title} sx={{ display: "flex", alignItems: "flex-start", gap: 1, p: 1, borderRadius: 2, bgcolor: "action.hover" }}>
                <MsIcon sx={{ fontSize: 18, color: "primary.main", mt: 0.25, flexShrink: 0 }}>{icon}</MsIcon>
                <Box><Typography sx={{ fontSize: 12, fontWeight: 500 }}>{title}</Typography><Typography sx={{ fontSize: 12, color: "text.secondary" }}>{desc}</Typography></Box>
              </Box>
            ))}
          </Box>
          <Typography sx={{ fontSize: 12, color: "text.secondary", mt: 1.5, maxWidth: 672 }}>
            <Box component="span" sx={{ fontWeight: 500, color: "text.primary" }}>Cursor / Claude Default</Box> create combos named exactly like those clients&apos; model IDs (e.g. <Box component="code" sx={{ fontFamily: "monospace" }}>composer-2.5</Box>, <Box component="code" sx={{ fontFamily: "monospace" }}>opus</Box>), seeded with the matching <Box component="code" sx={{ fontFamily: "monospace" }}>cu/…</Box> or <Box component="code" sx={{ fontFamily: "monospace" }}>cc/…</Box> route so traffic can hit the router without the prefix.
            {" "}Note: Cursor IDE often blocks built-in Composer / Grok from Override OpenAI Base URL (&quot;model does not support custom API&quot;); add them via Cursor&apos;s <Box component="span" sx={{ fontWeight: 500, color: "text.primary" }}>Add Custom Model</Box> using the combo name, or pick a model Cursor allows through the custom endpoint.
          </Typography>
        </Box>
        <Box sx={{ display: "flex", width: { xs: "100%", sm: "auto" }, flexDirection: "column", gap: 1, flexShrink: 0 }}>
          <Button icon="add" onClick={() => setShowCreateModal(true)}>Create Combo</Button>
          <Box sx={{ display: "grid", gridTemplateColumns: "repeat(2, 1fr)", gap: 1 }}>
            <Button variant="secondary" size="sm" icon="edit_note" loading={presetLoading === "cursor"} disabled={!!presetLoading} onClick={() => handleGeneratePresets("cursor")}>Cursor Default</Button>
            <Button variant="secondary" size="sm" icon="smart_toy" loading={presetLoading === "claude"} disabled={!!presetLoading} onClick={() => handleGeneratePresets("claude")}>Claude Default</Button>
          </Box>
        </Box>
      </Box>

      {combos.length === 0 ? (
        <Card>
          <Box sx={{ textAlign: "center", py: 6 }}>
            <Box sx={{ display: "inline-flex", alignItems: "center", justifyContent: "center", width: 64, height: 64, borderRadius: "50%", bgcolor: "action.selected", color: "primary.main", mb: 2 }}><MsIcon sx={{ fontSize: 32 }}>layers</MsIcon></Box>
            <Typography sx={{ color: "text.primary", fontWeight: 500, mb: 0.5 }}>No combos yet</Typography>
            <Typography sx={{ fontSize: 14, color: "text.secondary", mb: 2 }}>Create model combos with fallback support</Typography>
            <Button icon="add" onClick={() => setShowCreateModal(true)}>Create Combo</Button>
          </Box>
        </Card>
      ) : (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          <Box sx={{ display: "flex", minWidth: 0, flexDirection: { xs: "column", sm: "row" }, gap: 1, borderRadius: 2, border: "1px solid", borderColor: "divider", bgcolor: "action.hover", px: 1.5, py: 1, alignItems: { sm: "center" }, justifyContent: { sm: "space-between" } }}>
            <Box component="label" sx={{ display: "flex", alignItems: "center", gap: 1, fontSize: 12, color: "text.secondary", cursor: "pointer", userSelect: "none", "&:hover": { color: "primary.main" } }}>
              <Box component="input" type="checkbox" checked={allSelected} ref={(el) => { if (el) el.indeterminate = someSelected && !allSelected; }} onChange={toggleSelectAll} aria-label="Select all combos" sx={{ width: 14, height: 14, accentColor: "primary.main" }} />
              <Box component="span">{someSelected ? `${selectedIds.length} selected` : `Select all (${combos.length})`}</Box>
            </Box>
            <Box sx={{ display: "flex", minWidth: 0, flexWrap: "wrap", alignItems: "center", gap: 1 }}>
              {someSelected && <>
                <Box sx={{ width: { xs: "100%", sm: 200 }, minWidth: 160 }}>
                  <Select options={STRATEGY_OPTIONS} value="" placeholder="Set strategy…" disabled={bulkBusy} onChange={(e) => { const v = e.target.value; if (v) handleBulkSetStrategy(v); }} selectClassName="py-1.5 text-xs" />
                </Box>
                <Button size="sm" variant="danger" icon="delete" disabled={bulkBusy} loading={bulkBusy} onClick={handleBulkDelete}>Delete ({selectedIds.length})</Button>
                <Button size="sm" variant="ghost" onClick={clearSelection} disabled={bulkBusy}>Clear</Button>
              </>}
            </Box>
          </Box>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
            {combos.map((combo) => <ComboCard key={combo.id} combo={combo} getCaps={getCaps} activeProviders={activeProviders} copied={copied} onCopy={copy} onEdit={() => setEditingCombo(combo)} onDelete={() => handleDelete(combo.id)} strategy={comboStrategies[combo.name] || {}} onSetStrategy={(patch) => handleSetComboStrategy(combo.name, patch)} selected={selectedIds.includes(combo.id)} onToggleSelect={() => toggleSelect(combo.id)} />)}
          </Box>
        </Box>
      )}

      {showCreateModal && <ComboFormModal key="create" isOpen={showCreateModal} onClose={() => setShowCreateModal(false)} onSave={handleCreate} activeProviders={activeProviders} />}
      {editingCombo && <ComboFormModal key={editingCombo.id} isOpen={!!editingCombo} combo={editingCombo} onClose={() => setEditingCombo(null)} onSave={(data) => handleUpdate(editingCombo.id, data)} activeProviders={activeProviders} />}
      <ConfirmModal isOpen={!!confirmState} onClose={() => !confirmState?.loading && setConfirmState(null)} onConfirm={confirmState?.onConfirm} title={confirmState?.title || "Confirm"} message={confirmState?.message} confirmText={confirmState?.confirmText || "Confirm"} variant={confirmState?.variant || "danger"} loading={!!confirmState?.loading} />
    </Box>
  );
}

const STRATEGY_OPTIONS = [
  { value: "fallback", label: "Fallback — try in order" },
  { value: "round-robin", label: "Round Robin — rotate" },
  { value: "fusion", label: "Fusion — panel + judge" },
];

function ComboCard({ combo, getCaps, activeProviders = [], copied, onCopy, onEdit, onDelete, strategy = {}, onSetStrategy, selected = false, onToggleSelect }) {
  const [showJudgeSelect, setShowJudgeSelect] = useState(false);
  const current = strategy.fallbackStrategy || "fallback";
  const judge = strategy.judgeModel || "";
  const isFusion = current === "fusion";
  const subtleHover = { transition: "background-color 150ms ease, color 150ms ease", "&:hover": { bgcolor: (theme) => alpha(theme.palette.text.primary, 0.05), color: "primary.main" } };

  return (
    <Card padding="sm" sx={{ ...(selected ? { outline: "1px solid", outlineColor: "primary.main", bgcolor: "action.selected" } : {}) }}>
      <Box sx={{ display: "flex", minWidth: 0, flexDirection: { xs: "column", sm: "row" }, gap: 1.5, alignItems: { sm: "center" }, justifyContent: { sm: "space-between" } }}>
        <Box sx={{ display: "flex", minWidth: 0, flex: 1, alignItems: { xs: "flex-start", sm: "center" }, gap: 1.5 }}>
          <Box component="label" title="Select combo" sx={{ display: "flex", flexShrink: 0, alignItems: "center", pt: { xs: 0.5, sm: 0 }, cursor: "pointer" }}>
            <Box component="input" type="checkbox" checked={selected} onChange={onToggleSelect} onClick={(e) => e.stopPropagation()} aria-label={`Select ${combo.name}`} sx={{ width: 16, height: 16, accentColor: "primary.main" }} />
          </Box>
          <Box sx={{ width: 32, height: 32, borderRadius: 2, bgcolor: "action.selected", display: "flex", alignItems: "center", justifyContent: "center", flexShrink: 0 }}><MsIcon sx={{ color: "primary.main", fontSize: 18 }}>layers</MsIcon></Box>
          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Box component="code" sx={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontFamily: "monospace", fontSize: 14, fontWeight: 500 }}>{combo.name}</Box>
            <Box sx={{ mt: 0.5, display: "flex", minWidth: 0, flexWrap: "wrap", alignItems: "center", gap: 0.5 }}>
              {combo.models.length === 0 ? <Typography sx={{ fontSize: 12, color: "text.secondary", fontStyle: "italic" }}>No models</Typography> : combo.models.slice(0, 3).map((model, index) => (
                <Box key={index} component="code" sx={{ display: "inline-flex", alignItems: "center", gap: 0.5, borderRadius: 1, bgcolor: "action.hover", px: 0.75, py: 0.25, fontFamily: "monospace", fontSize: 12, color: "text.secondary" }}><Box component="span">{model}</Box><CapacityBadges caps={getCaps?.(model)} /></Box>
              ))}
              {combo.models.length > 3 && <Typography sx={{ fontSize: 10, color: "text.secondary" }}>+{combo.models.length - 3} more</Typography>}
            </Box>
            {combo.models.length > 0 && combo.models.every((model) => getCaps?.(model)?.vision === false) && (
              <Box component="button" type="button" onClick={onEdit} title="Edit combo to add a vision-capable model" sx={{ mt: 0.5, display: "inline-flex", alignItems: "center", gap: 0.5, fontSize: 11, color: "warning.main", border: 0, bgcolor: "transparent", cursor: "pointer", "&:hover": { color: "warning.dark" } }}>
                <MsIcon sx={{ fontSize: 14 }}>visibility_off</MsIcon>Cannot see images — edit combo
              </Box>
            )}
            {isFusion && (
              <Box sx={{ mt: 1, display: "flex", minWidth: 0, flexWrap: "wrap", alignItems: "center", gap: 0.75 }}>
                <Typography sx={{ fontSize: 11, fontWeight: 500, color: "text.secondary" }}>Judge</Typography>
                <Box component="button" onClick={() => setShowJudgeSelect(true)} title="Pick the model that fuses panel answers" sx={{ display: "inline-flex", maxWidth: "100%", alignItems: "center", gap: 0.5, borderRadius: 1, border: "1px dashed", borderColor: "primary.main", px: 0.75, py: 0.25, fontFamily: "monospace", fontSize: 11, color: "primary.main", bgcolor: "transparent", cursor: "pointer", "&:hover": { bgcolor: "action.hover" } }}>
                  <MsIcon sx={{ fontSize: 13 }}>gavel</MsIcon><Box component="span" sx={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{judge || `Auto — ${combo.models[0] || "first model"}`}</Box>
                </Box>
                {judge && <Box component="button" onClick={() => onSetStrategy({ judgeModel: "" })} title="Reset judge to Auto" sx={{ p: 0.25, borderRadius: 1, color: "text.secondary", border: 0, bgcolor: "transparent", cursor: "pointer", "&:hover": { color: "error.main", bgcolor: "action.hover" } }}><MsIcon sx={{ fontSize: 13 }}>close</MsIcon></Box>}
              </Box>
            )}
          </Box>
        </Box>
        <Box sx={{ display: "flex", width: { xs: "100%", sm: "auto" }, flexDirection: { xs: "column", sm: "row" }, gap: 1, alignItems: { sm: "center" }, flexShrink: 0 }}>
          <Box sx={{ width: { xs: "100%", sm: 200 } }}><Select options={STRATEGY_OPTIONS} value={current} onChange={(e) => onSetStrategy({ fallbackStrategy: e.target.value })} selectClassName="py-1.5 text-xs" /></Box>
          <Box sx={{ display: "grid", gridTemplateColumns: { xs: "repeat(3,1fr)", sm: "repeat(3,auto)" }, gap: 0.5 }}>
            <Box component="button" onClick={(e) => { e.stopPropagation(); onCopy(combo.name, `combo-${combo.id}`); }} title="Copy combo name" sx={{ ...subtleHover, display: "flex", flexDirection: "column", alignItems: "center", borderRadius: 1, px: 1, py: 0.5, color: "text.secondary", border: 0, bgcolor: "transparent", cursor: "pointer" }}><MsIcon sx={{ fontSize: 18 }}>{copied === `combo-${combo.id}` ? "check" : "content_copy"}</MsIcon><Typography sx={{ fontSize: 10, lineHeight: 1.2 }}>Copy</Typography></Box>
            <Box component="button" onClick={onEdit} title="Edit" sx={{ ...subtleHover, display: "flex", flexDirection: "column", alignItems: "center", borderRadius: 1, px: 1, py: 0.5, color: "text.secondary", border: 0, bgcolor: "transparent", cursor: "pointer" }}><MsIcon sx={{ fontSize: 18 }}>edit</MsIcon><Typography sx={{ fontSize: 10, lineHeight: 1.2 }}>Edit</Typography></Box>
            <Box component="button" onClick={onDelete} title="Delete" sx={{ ...subtleHover, display: "flex", flexDirection: "column", alignItems: "center", borderRadius: 1, px: 1, py: 0.5, color: "error.main", border: 0, bgcolor: "transparent", cursor: "pointer", "&:hover": { bgcolor: (theme) => alpha(theme.palette.error.main, 0.1), color: "error.main" } }}><MsIcon sx={{ fontSize: 18 }}>delete</MsIcon><Typography sx={{ fontSize: 10, lineHeight: 1.2 }}>Delete</Typography></Box>
          </Box>
        </Box>
      </Box>
      {showJudgeSelect && <ModelSelectModal isOpen={showJudgeSelect} onClose={() => setShowJudgeSelect(false)} onSelect={(m) => { onSetStrategy({ judgeModel: m?.value || "" }); setShowJudgeSelect(false); }} activeProviders={activeProviders} title="Select Judge Model" addedModelValues={judge ? [judge] : []} closeOnSelect={true} />}
    </Card>
  );
}
