"use client";

import { useCallback, useEffect, useState } from "react";
import PropTypes from "prop-types";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import TextField from "@mui/material/TextField";
import { Card, Badge } from "@/shared/components";
import { useNotificationStore } from "@/store/notificationStore";

// Mirrors the server-side gate in /api/providers/[id]/overrides — client check is UX only
const BLOCKED_HEADERS = ["host", "content-length", "content-type", "connection", "transfer-encoding", "authorization", "cookie"];
const HEADER_NAME_RE = /^[A-Za-z0-9-]+$/;

export default function CustomConfigCard({ providerId }) {
  const notify = useNotificationStore();
  const [expanded, setExpanded] = useState(false);
  const [rows, setRows] = useState([{ name: "", value: "" }]);
  const [builtin, setBuiltin] = useState({});
  const [hasOverride, setHasOverride] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    let cancelled = false;
    fetch(`/api/providers/${providerId}/overrides`, { cache: "no-store" })
      .then((r) => (r.ok ? r.json() : null))
      .then((data) => {
        if (cancelled || !data) return;
        // Effective set = registry built-ins with user overrides layered on top
        const builtinHeaders = data.builtinHeaders || {};
        const effective = { ...builtinHeaders, ...(data.headers || {}) };
        const headerRows = Object.entries(effective).map(([name, value]) => ({ name, value }));
        setBuiltin(builtinHeaders);
        setRows(headerRows.length ? headerRows : [{ name: "", value: "" }]);
        setHasOverride(Object.keys(data.headers || {}).length > 0);
      })
      .catch(() => {});
    return () => { cancelled = true; };
  }, [providerId]);

  const setRow = (i, field, value) => {
    setRows((prev) => prev.map((r, idx) => (idx === i ? { ...r, [field]: value } : r)));
  };

  const save = useCallback(async () => {
    // Diff-on-save: only rows differing from the registry default become overrides,
    // so a code-side registry bump still wins for everything the user left alone.
    const headers = {};
    for (const r of rows.filter((r) => r.name.trim())) {
      const name = r.name.trim();
      if (!HEADER_NAME_RE.test(name)) {
        notify.error(`Invalid header name: ${name}`);
        return;
      }
      if (BLOCKED_HEADERS.includes(name.toLowerCase())) {
        notify.error(`Header ${name} cannot be overridden`);
        return;
      }
      if (name in headers) {
        notify.error(`Duplicate header name: ${name}`);
        return;
      }
      if (r.value !== builtin[name]) headers[name] = r.value;
    }

    setSaving(true);
    try {
      const res = await fetch(`/api/providers/${providerId}/overrides`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ headers }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        notify.error(err.error || "Failed to save");
        return;
      }
      setHasOverride(Object.keys(headers).length > 0);
      notify.success("Custom headers saved");
    } finally {
      setSaving(false);
    }
  }, [rows, builtin, providerId, notify]);

  const resetToBuiltin = () => {
    setRows(Object.entries(builtin).map(([name, value]) => ({ name, value })));
  };

  // Only render when there is something to customize: registry headers or existing overrides
  if (Object.keys(builtin).length === 0 && !hasOverride) return null;

  return (
    <Card padding="xs">
      <Box
        component="button"
        type="button"
        onClick={() => setExpanded((v) => !v)}
        sx={{ display: "flex", width: "100%", alignItems: "center", justifyContent: "space-between", textAlign: "left", border: 0, bgcolor: "transparent", cursor: "pointer", p: 0 }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 20, color: "primary.main" }}>tune</Icon>
          <Box component="span" sx={{ fontSize: 14, fontWeight: 600 }}>Custom Headers</Box>
          {hasOverride && (
            <Badge variant="success" size="sm">Active</Badge>
          )}
        </Box>
        <Icon className="material-symbols-outlined" baseClassName="" sx={{ color: "text.secondary" }}>
          {expanded ? "expand_less" : "expand_more"}
        </Icon>
      </Box>

      {expanded && (
        <Box sx={{ mt: 1.5, borderTop: 1, borderColor: "divider", pt: 1.5 }}>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            {rows.map((row, i) => {
              const overridden = row.name.trim() in builtin && row.value !== builtin[row.name.trim()];
              return (
                <Box key={i} sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                  <TextField
                    size="small"
                    value={row.name}
                    onChange={(e) => setRow(i, "name", e.target.value)}
                    placeholder="Header-Name"
                    spellCheck={false}
                    sx={{ width: 176 }}
                  />
                  <TextField
                    size="small"
                    value={row.value}
                    onChange={(e) => setRow(i, "value", e.target.value)}
                    placeholder="Value"
                    spellCheck={false}
                    title={overridden ? "Overridden" : row.name.trim() in builtin ? "Registry default" : ""}
                    sx={{
                      minWidth: 0,
                      flex: 1,
                      "& .MuiOutlinedInput-root": {
                        borderColor: overridden ? "rgba(251,191,36,0.6)" : "divider",
                      },
                    }}
                  />
                  <Box
                    component="button"
                    type="button"
                    title="Remove header"
                    onClick={() => setRows((prev) => (prev.length > 1 ? prev.filter((_, idx) => idx !== i) : [{ name: "", value: "" }]))}
                    sx={{ flexShrink: 0, border: 0, bgcolor: "transparent", color: "text.secondary", cursor: "pointer", p: 0.5, "&:hover": { color: "#ef4444" } }}
                  >
                    <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18 }}>delete</Icon>
                  </Box>
                </Box>
              );
            })}
          </Box>

          <Box sx={{ mt: 1.5, display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
            <Box
              component="button"
              type="button"
              onClick={() => setRows((prev) => [...prev, { name: "", value: "" }])}
              sx={{ display: "flex", alignItems: "center", gap: 0.5, border: 0, bgcolor: "transparent", cursor: "pointer", fontSize: 12, color: "primary.main", "&:hover": { textDecoration: "underline" } }}
            >
              <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 16 }}>add</Icon>
              Add header
            </Box>
            <Box sx={{ display: "flex", gap: 1 }}>
              <Box
                component="button"
                type="button"
                disabled={saving}
                onClick={resetToBuiltin}
                sx={{ borderRadius: "6px", border: 1, borderColor: "divider", px: 1.5, py: 0.75, fontSize: 12, bgcolor: "transparent", color: "text.primary", cursor: saving ? "not-allowed" : "pointer", opacity: saving ? 0.5 : 1, "&:hover": { bgcolor: "action.hover" } }}
              >
                Reset
              </Box>
              <Box
                component="button"
                type="button"
                disabled={saving}
                onClick={save}
                sx={{ borderRadius: "6px", border: 0, bgcolor: "primary.main", px: 1.5, py: 0.75, fontSize: 12, fontWeight: 500, color: "primary.contrastText", cursor: saving ? "not-allowed" : "pointer", opacity: saving ? 0.5 : 1, "&:hover": { opacity: 0.9 } }}
              >
                {saving ? "Saving..." : "Save"}
              </Box>
            </Box>
          </Box>
        </Box>
      )}
    </Card>
  );
}

CustomConfigCard.propTypes = {
  providerId: PropTypes.string.isRequired,
};
