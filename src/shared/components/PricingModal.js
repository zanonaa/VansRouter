"use client";

import { useState, useEffect } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Table from "@mui/material/Table";
import TableHead from "@mui/material/TableHead";
import TableBody from "@mui/material/TableBody";
import TableRow from "@mui/material/TableRow";
import TableCell from "@mui/material/TableCell";
import { getDefaultPricing, formatCost } from "open-sse/providers/pricing.js";

export default function PricingModal({ isOpen, onClose, onSave }) {
  const [pricingData, setPricingData] = useState({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  /* eslint-disable react-hooks/immutability --
     loadPricing is declared below the JSX section. */
  useEffect(() => {
    if (isOpen) {
      loadPricing();
    }
  }, [isOpen]);

  const loadPricing = async () => {
    setLoading(true);
    try {
      const response = await fetch("/api/pricing");
      if (response.ok) {
        const data = await response.json();
        setPricingData(data);
      } else {
        // Fallback to defaults
        const defaults = getDefaultPricing();
        setPricingData(defaults);
      }
    } catch (error) {
      console.error("Failed to load pricing:", error);
      const defaults = getDefaultPricing();
      setPricingData(defaults);
    } finally {
      setLoading(false);
    }
  };

  const handlePricingChange = (provider, model, field, value) => {
    const numValue = parseFloat(value);
    if (isNaN(numValue) || numValue < 0) return;

    setPricingData(prev => {
      const newData = { ...prev };
      if (!newData[provider]) newData[provider] = {};
      if (!newData[provider][model]) newData[provider][model] = {};
      newData[provider][model][field] = numValue;
      return newData;
    });
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const response = await fetch("/api/pricing", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(pricingData)
      });

      if (response.ok) {
        onSave?.();
        onClose();
      } else {
        const error = await response.json();
        alert(`Failed to save pricing: ${error.error}`);
      }
    } catch (error) {
      console.error("Failed to save pricing:", error);
      alert("Failed to save pricing");
    } finally {
      setSaving(false);
    }
  };

  const handleReset = async () => {
    if (!confirm("Reset all pricing to defaults? This cannot be undone.")) return;

    try {
      const response = await fetch("/api/pricing", { method: "DELETE" });
      if (response.ok) {
        const defaults = getDefaultPricing();
        setPricingData(defaults);
      }
    } catch (error) {
      console.error("Failed to reset pricing:", error);
      alert("Failed to reset pricing");
    }
  };

  if (!isOpen) return null;

  // Get all unique providers and models for display
  const allProviders = Object.keys(pricingData).sort();
  const pricingFields = ["input", "output", "cached", "reasoning", "cache_creation"];

  const btnBase = {
    px: 2,
    py: 1,
    fontSize: 14,
    borderRadius: "4px",
    border: "1px solid var(--color-border)",
    bgcolor: "transparent",
    color: "text.secondary",
    cursor: "pointer",
    transition: "all 150ms",
  };

  return (
    <Box sx={{ position: "fixed", inset: 0, bgcolor: "rgba(0,0,0,0.5)", display: "flex", alignItems: "center", justifyContent: "center", zIndex: 50, p: 2 }}>
      <Box sx={{ bgcolor: "background.default", border: "1px solid var(--color-border)", borderRadius: "8px", boxShadow: "0 20px 25px -5px rgba(0,0,0,0.1)", maxWidth: 1152, width: "100%", maxHeight: "90vh", overflow: "hidden", display: "flex", flexDirection: "column" }}>
        {/* Header */}
        <Box sx={{ p: 2, borderBottom: "1px solid var(--color-border)", display: "flex", alignItems: "center", justifyContent: "space-between" }}>
          <Typography variant="h6" sx={{ fontSize: 20, fontWeight: 600 }}>Pricing Configuration</Typography>
          <Box component="button" onClick={onClose} sx={{ border: 0, bgcolor: "transparent", color: "text.secondary", fontSize: 24, lineHeight: 1, cursor: "pointer", "&:hover": { color: "text.primary" } }}>
            ×
          </Box>
        </Box>

        {/* Content */}
        <Box sx={{ flex: 1, overflow: "auto", p: 2 }}>
          {loading ? (
            <Box sx={{ textAlign: "center", py: 4, color: "text.secondary" }}>Loading pricing data...</Box>
          ) : (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
              {/* Instructions */}
              <Box sx={{ bgcolor: "var(--color-surface-2)", border: "1px solid var(--color-border)", borderRadius: "8px", p: 1.5, fontSize: 14 }}>
                <Typography sx={{ fontWeight: 500, mb: 0.5 }}>Pricing Rates Format</Typography>
                <Typography sx={{ color: "text.secondary" }}>
                  All rates are in <strong>dollars per million tokens</strong> ($/1M tokens).
                  Example: Input rate of 2.50 means $2.50 per 1,000,000 input tokens.
                </Typography>
              </Box>

              {/* Pricing Tables */}
              {allProviders.map(provider => {
                const models = Object.keys(pricingData[provider]).sort();
                return (
                  <Box key={provider} sx={{ border: "1px solid var(--color-border)", borderRadius: "8px", overflow: "hidden" }}>
                    <Box sx={{ bgcolor: "var(--color-surface-2)", px: 2, py: 1, fontWeight: 600, fontSize: 14 }}>
                      {provider.toUpperCase()}
                    </Box>
                    <Box sx={{ overflowX: "auto" }}>
                      <Table sx={{ width: "100%", fontSize: 14 }}>
                        <TableHead sx={{ bgcolor: "var(--color-surface-3)" }}>
                          <TableRow>
                            <TableCell sx={{ px: 1.5, py: 1, textAlign: "left", textTransform: "uppercase", fontSize: 12, color: "text.secondary" }}>Model</TableCell>
                            <TableCell sx={{ px: 1.5, py: 1, textAlign: "right", textTransform: "uppercase", fontSize: 12, color: "text.secondary" }}>Input</TableCell>
                            <TableCell sx={{ px: 1.5, py: 1, textAlign: "right", textTransform: "uppercase", fontSize: 12, color: "text.secondary" }}>Output</TableCell>
                            <TableCell sx={{ px: 1.5, py: 1, textAlign: "right", textTransform: "uppercase", fontSize: 12, color: "text.secondary" }}>Cached</TableCell>
                            <TableCell sx={{ px: 1.5, py: 1, textAlign: "right", textTransform: "uppercase", fontSize: 12, color: "text.secondary" }}>Reasoning</TableCell>
                            <TableCell sx={{ px: 1.5, py: 1, textAlign: "right", textTransform: "uppercase", fontSize: 12, color: "text.secondary" }}>Cache Creation</TableCell>
                          </TableRow>
                        </TableHead>
                        <TableBody>
                          {models.map(model => (
                            <TableRow key={model} hover>
                              <TableCell sx={{ px: 1.5, py: 1, fontWeight: 500 }}>{model}</TableCell>
                              {pricingFields.map(field => (
                                <TableCell key={field} sx={{ px: 1.5, py: 1 }}>
                                  <Box
                                    component="input"
                                    type="number"
                                    step="0.01"
                                    min="0"
                                    value={pricingData[provider][model][field] || 0}
                                    onChange={(e) => handlePricingChange(provider, model, field, e.target.value)}
                                    sx={{
                                      width: 80,
                                      px: 1,
                                      py: 0.5,
                                      textAlign: "right",
                                      bgcolor: "background.default",
                                      border: "1px solid var(--color-border)",
                                      borderRadius: "4px",
                                      color: "text.primary",
                                      outline: "none",
                                      "&:focus": { borderColor: "primary.main" },
                                    }}
                                  />
                                </TableCell>
                              ))}
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    </Box>
                  </Box>
                );
              })}

              {allProviders.length === 0 && (
                <Box sx={{ textAlign: "center", py: 4, color: "text.secondary" }}>
                  No pricing data available
                </Box>
              )}
            </Box>
          )}
        </Box>

        {/* Footer */}
        <Box sx={{ p: 2, borderTop: "1px solid var(--color-border)", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
          <Box
            component="button"
            onClick={handleReset}
            disabled={saving}
            sx={{ ...btnBase, color: "error.main", borderColor: "rgba(239,68,68,0.2)", "&:hover": { bgcolor: "rgba(239,68,68,0.1)" } }}
          >
            Reset to Defaults
          </Box>
          <Box sx={{ display: "flex", gap: 1 }}>
            <Box component="button" onClick={onClose} disabled={saving} sx={{ ...btnBase, "&:hover": { color: "text.primary" } }}>
              Cancel
            </Box>
            <Box
              component="button"
              onClick={handleSave}
              disabled={saving}
              sx={{ ...btnBase, bgcolor: "primary.main", color: "#fff", borderColor: "primary.main", "&:hover": { bgcolor: "primary.dark" }, "&:disabled": { opacity: 0.5 } }}
            >
              {saving ? "Saving..." : "Save Changes"}
            </Box>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
