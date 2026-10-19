"use client";

import { useState, useEffect, useRef } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import { DndContext, closestCenter, KeyboardSensor, PointerSensor, useSensor, useSensors } from "@dnd-kit/core";
import { arrayMove, SortableContext, sortableKeyboardCoordinates, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { restrictToVerticalAxis, restrictToParentElement } from "@dnd-kit/modifiers";
import Modal from "./Modal";
import Input from "./Input";
import Button from "./Button";
import ModelSelectModal from "./ModelSelectModal";
import { useModelCaps } from "@/shared/hooks/useModelCaps";

const VALID_NAME_REGEX = /^[a-zA-Z0-9_.\-]+$/;

const iconBtnSx = {
  p: 0.25,
  borderRadius: "4px",
  border: 0,
  bgcolor: "transparent",
  cursor: "pointer",
  display: "inline-flex",
  alignItems: "center",
  justifyContent: "center",
};

// Inline editable model item
function ModelItem({ id, index, model, isFirst, isLast, onEdit, onMoveUp, onMoveDown, onRemove }) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useSortable({ id });
  const style = {
    transform: CSS.Transform.toString(transform),
    opacity: isDragging ? 0.4 : 1,
    zIndex: isDragging ? 999 : undefined,
  };
  const [editing, setEditing] = useState(false);
  const [draftOverride, setDraft] = useState(null);
  const [prevModel, setPrevModel] = useState(model);
  if (prevModel !== model) {
    setPrevModel(model);
    setDraft(null);
  }
  const draft = draftOverride ?? model;
  const commit = () => {
    const trimmed = draft.trim();
    if (trimmed && trimmed !== model) onEdit(trimmed);
    else setDraft(null);
    setEditing(false);
  };
  const handleKeyDown = (e) => {
    if (e.key === "Enter") commit();
    if (e.key === "Escape") { setDraft(null); setEditing(false); }
  };
  return (
    <Box
      ref={setNodeRef}
      style={style}
      sx={{
        display: "flex",
        minWidth: 0,
        alignItems: "center",
        gap: 0.75,
        borderRadius: "6px",
        bgcolor: "rgba(0,0,0,0.02)",
        px: 1,
        py: 0.5,
        transition: "background-color 150ms",
        "&:hover": { bgcolor: "rgba(0,0,0,0.04)" },
        ...(isDragging ? { boxShadow: "0 4px 6px -1px rgba(0,0,0,0.1), 0 0 0 1px rgba(229,106,74,0.3)" } : {}),
      }}
    >
      <Box component="button" {...attributes} {...listeners} type="button" title="Drag to reorder" aria-label="Drag to reorder" sx={{ ...iconBtnSx, cursor: "grab", touchAction: "none", color: "text.secondary", flexShrink: 0, "&:hover": { color: "primary.main" } }}>
        <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
          <circle cx="9" cy="4" r="2" /><circle cx="15" cy="4" r="2" />
          <circle cx="9" cy="12" r="2" /><circle cx="15" cy="12" r="2" />
          <circle cx="9" cy="20" r="2" /><circle cx="15" cy="20" r="2" />
        </svg>
      </Box>
      <Box component="span" sx={{ fontSize: 10, fontWeight: 500, color: "text.secondary", width: 12, textAlign: "center", flexShrink: 0 }}>{index + 1}</Box>
      {editing ? (
        <Box
          component="input"
          autoFocus
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onBlur={commit}
          onKeyDown={handleKeyDown}
          aria-label={`Edit model ${index + 1}`}
          sx={{
            minWidth: 0,
            flex: 1,
            borderRadius: "4px",
            border: "1px solid rgba(229,106,74,0.4)",
            bgcolor: "background.paper",
            px: 0.75,
            py: 0.25,
            fontFamily: "ui-monospace, monospace",
            fontSize: 12,
            color: "text.primary",
            outline: "none",
          }}
        />
      ) : (
        <Box
          component="button"
          type="button"
          onClick={() => setEditing(true)}
          title="Click to edit"
          aria-label={`Edit model: ${model}`}
          sx={{ minWidth: 0, flex: 1, cursor: "text", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", borderRadius: "4px", px: 0.75, py: 0.25, fontFamily: "ui-monospace, monospace", fontSize: 12, color: "text.primary", textAlign: "left", border: 0, bgcolor: "transparent", "&:hover": { bgcolor: "rgba(0,0,0,0.05)" } }}
        >
          {model}
        </Box>
      )}
      <Box sx={{ display: "flex", flexShrink: 0, alignItems: "center", gap: 0.25 }}>
        <Box component="button" type="button" onClick={onMoveUp} disabled={isFirst} title="Move up" sx={{ ...iconBtnSx, color: isFirst ? "rgba(107,114,128,0.2)" : "text.secondary", cursor: isFirst ? "not-allowed" : "pointer", "&:hover": isFirst ? {} : { color: "primary.main", bgcolor: "rgba(0,0,0,0.05)" } }}>
          <span className="material-symbols-outlined" style={{ fontSize: 12 }}>arrow_upward</span>
        </Box>
        <Box component="button" type="button" onClick={onMoveDown} disabled={isLast} title="Move down" sx={{ ...iconBtnSx, color: isLast ? "rgba(107,114,128,0.2)" : "text.secondary", cursor: isLast ? "not-allowed" : "pointer", "&:hover": isLast ? {} : { color: "primary.main", bgcolor: "rgba(0,0,0,0.05)" } }}>
          <span className="material-symbols-outlined" style={{ fontSize: 12 }}>arrow_downward</span>
        </Box>
      </Box>
      <Box component="button" type="button" onClick={onRemove} title="Remove" sx={{ ...iconBtnSx, color: "text.secondary", "&:hover": { bgcolor: "rgba(239,68,68,0.1)", color: "error.main" } }}>
        <span className="material-symbols-outlined" style={{ fontSize: 12 }}>close</span>
      </Box>
    </Box>
  );
}

// Reusable Combo create/edit modal. forcePrefix auto-prepends to name.
export default function ComboFormModal({ isOpen, combo, onClose, onSave, activeProviders, kindFilter = null, forcePrefix = "", title }) {
  // Strip prefix when editing existing combo so user only edits suffix
  const initialName = combo?.name
    ? (forcePrefix && combo.name.startsWith(forcePrefix) ? combo.name.slice(forcePrefix.length) : combo.name)
    : "";
  const [name, setName] = useState(initialName);
  const [models, setModels] = useState(combo?.models || []);
  const [showModelSelect, setShowModelSelect] = useState(false);
  const [modelPickerCap, setModelPickerCap] = useState(null);
  const [saving, setSaving] = useState(false);
  const [nameError, setNameError] = useState("");
  const [modelAliases, setModelAliases] = useState({});
  const { getCaps } = useModelCaps();

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates })
  );
  const modelItems = models.map((model, i) => ({ uid: `item-${i}`, model }));
  const handleDragEnd = ({ active, over }) => {
    if (!over || active.id === over.id) return;
    const oldIndex = modelItems.findIndex((m) => m.uid === active.id);
    const newIndex = modelItems.findIndex((m) => m.uid === over.id);
    if (oldIndex !== -1 && newIndex !== -1) setModels((prev) => arrayMove(prev, oldIndex, newIndex));
  };

  const wasOpenRef = useRef(false);

  useEffect(() => {
    if (!isOpen) { wasOpenRef.current = false; return; }
    if (wasOpenRef.current) return;
    wasOpenRef.current = true;
    const controller = new AbortController();
    fetch("/api/models/alias", { signal: controller.signal }).then((r) => r.ok ? r.json() : null).then((d) => { if (d && !controller.signal.aborted) setModelAliases(d.aliases || {}); }).catch(() => {});
    return () => controller.abort();
  }, [isOpen]);

  const validateName = (value) => {
    if (!value.trim()) { setNameError("Name is required"); return false; }
    const full = forcePrefix + value;
    if (!VALID_NAME_REGEX.test(full)) { setNameError("Only letters, numbers, -, _ and . allowed"); return false; }
    setNameError("");
    return true;
  };

  const handleNameChange = (e) => {
    let value = e.target.value;
    // If user types prefix manually, strip it (we always prepend)
    if (forcePrefix && value.startsWith(forcePrefix)) value = value.slice(forcePrefix.length);
    setName(value);
    if (value) validateName(value); else setNameError("");
  };

  const handleAddModel = (model) => {
    if (!models.includes(model.value)) setModels([...models, model.value]);
  };
  const handleDeselectModel = (model) => {
    setModels(models.filter((m) => m !== model.value));
  };
  const handleRemoveModel = (i) => setModels(models.filter((_, idx) => idx !== i));
  const handleMoveUp = (i) => {
    if (i === 0) return;
    const a = [...models]; [a[i - 1], a[i]] = [a[i], a[i - 1]]; setModels(a);
  };
  const handleMoveDown = (i) => {
    if (i === models.length - 1) return;
    const a = [...models]; [a[i], a[i + 1]] = [a[i + 1], a[i]]; setModels(a);
  };

  const handleSave = async () => {
    if (!validateName(name)) return;
    setSaving(true);
    await onSave({ name: forcePrefix + name.trim(), models });
    setSaving(false);
  };

  const isEdit = !!combo;

  return (
    <>
      <Modal isOpen={isOpen} onClose={onClose} title={title || (isEdit ? "Edit Combo" : "Create Combo")}>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          <Box>
            {forcePrefix ? (
              <>
                <Box component="label" htmlFor="combo-name-input" sx={{ fontSize: 14, fontWeight: 500, mb: 0.5, display: "block" }}>Combo Name</Box>
                <Box sx={{ display: "flex", alignItems: "stretch" }}>
                  <Box component="span" sx={{ display: "inline-flex", alignItems: "center", px: 1, borderRadius: "4px 0 0 4px", border: "1px solid var(--color-border)", borderRight: 0, bgcolor: "rgba(0,0,0,0.04)", color: "text.secondary", fontFamily: "ui-monospace, monospace", fontSize: 14 }}>{forcePrefix}</Box>
                  <Box
                    component="input"
                    id="combo-name-input"
                    value={name}
                    onChange={handleNameChange}
                    placeholder="my-combo"
                    sx={{ flex: 1, minWidth: 0, borderRadius: "0 4px 4px 0", border: "1px solid var(--color-border)", bgcolor: "background.paper", px: 1, py: 0.75, fontFamily: "ui-monospace, monospace", fontSize: 14, color: "text.primary", outline: "none", "&:focus": { borderColor: "primary.main" } }}
                  />
                </Box>
                {nameError && <Typography sx={{ fontSize: 11, color: "error.main", mt: 0.25 }}>{nameError}</Typography>}
              </>
            ) : (
              <Input label="Combo Name" value={name} onChange={handleNameChange} placeholder="my-combo" error={nameError} />
            )}
            <Typography sx={{ fontSize: 10, color: "text.secondary", mt: 0.25 }}>
              {forcePrefix ? `Auto-prefixed with "${forcePrefix}". ` : ""}Only letters, numbers, -, _ and . allowed
            </Typography>
          </Box>

          <Box>
            <Box component="span" sx={{ fontSize: 14, fontWeight: 500, mb: 0.75, display: "block" }}>Models</Box>
            {models.length === 0 ? (
              <Box sx={{ textAlign: "center", py: 2, border: "1px dashed var(--color-border)", borderRadius: "8px", bgcolor: "rgba(0,0,0,0.01)" }}>
                <span className="material-symbols-outlined" style={{ color: "var(--color-text-muted)", fontSize: 20, display: "block", marginBottom: 4 }}>layers</span>
                <Typography sx={{ fontSize: 12, color: "text.secondary" }}>No models added yet</Typography>
              </Box>
            ) : (
              <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd} modifiers={[restrictToVerticalAxis, restrictToParentElement]}>
                <SortableContext items={modelItems.map((m) => m.uid)} strategy={verticalListSortingStrategy}>
                  <Box sx={{ display: "flex", maxHeight: { xs: "55vh", sm: 350 }, minWidth: 0, flexDirection: "column", gap: 0.5, overflowY: "auto" }}>
                    {modelItems.map(({ uid, model }, index) => (
                      <ModelItem key={uid} id={uid} index={index} model={model}
                        isFirst={index === 0} isLast={index === modelItems.length - 1}
                        onEdit={(v) => { const a = [...models]; a[index] = v; setModels(a); }}
                        onMoveUp={() => handleMoveUp(index)}
                        onMoveDown={() => handleMoveDown(index)}
                        onRemove={() => handleRemoveModel(index)} />
                    ))}
                  </Box>
                </SortableContext>
              </DndContext>
            )}
            {models.length > 0 && models.every((model) => getCaps?.(model)?.vision === false) && (
              <Box sx={{ mt: 1, display: "flex", alignItems: "center", gap: 0.5, fontSize: 11, color: "#d97706" }}>
                <span className="material-symbols-outlined" style={{ fontSize: 14 }}>visibility_off</span>
                Cannot see images — add a vision-capable model.
                <Box component="button" type="button" onClick={() => { setModelPickerCap("vision"); setShowModelSelect(true); }} sx={{ fontWeight: 500, textDecoration: "underline", textUnderlineOffset: 2, border: 0, bgcolor: "transparent", color: "inherit", cursor: "pointer", "&:hover": { textDecoration: "none" } }}>Add one</Box>
              </Box>
            )}
            <Box
              component="button"
              type="button"
              onClick={() => { setModelPickerCap(null); setShowModelSelect(true); }}
              sx={{ width: "100%", mt: 1, py: 1, border: "1px dashed var(--color-border)", borderRadius: "8px", fontSize: 12, color: "primary.main", fontWeight: 500, bgcolor: "transparent", cursor: "pointer", transition: "all 150ms", display: "flex", alignItems: "center", justifyContent: "center", gap: 0.5, "&:hover": { borderColor: "rgba(229,106,74,0.5)" } }}
            >
              <span className="material-symbols-outlined" style={{ fontSize: 16 }}>add</span>
              Add Model
            </Box>
          </Box>

          <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1, pt: 0.5 }}>
            <Button onClick={onClose} variant="ghost" fullWidth size="sm">Cancel</Button>
            <Button onClick={handleSave} fullWidth size="sm" disabled={!name.trim() || !!nameError || saving}>
              {saving ? "Saving..." : isEdit ? "Save" : "Create"}
            </Button>
          </Box>
        </Box>
      </Modal>

      {showModelSelect && (
        <ModelSelectModal isOpen={showModelSelect} onClose={() => setShowModelSelect(false)}
          onSelect={handleAddModel} onDeselect={handleDeselectModel}
          activeProviders={activeProviders} modelAliases={modelAliases}
          title="Add Model to Combo" kindFilter={kindFilter} capFilter={modelPickerCap}
          addedModelValues={models} closeOnSelect={false}
          onBack={() => setShowModelSelect(false)} />
      )}
    </>
  );
}
