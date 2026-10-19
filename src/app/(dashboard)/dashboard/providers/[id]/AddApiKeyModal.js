"use client";

import { useState } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import { Button, Badge, Input, Modal, Select } from "@/shared/components";
import { AI_PROVIDERS } from "@/shared/constants/providers";
import { planBulkAdd } from "@/shared/utils/bulkAdd";

const BULK_PLACEHOLDER = `name1|sk-key1\nname2|sk-key2\nsk-key-only-auto-named`;

export default function AddApiKeyModal({ isOpen, provider, providerName, isCompatible, isAnthropic, authType, authHint, website, proxyPools, error, existingNames, onSave, onBulkDone, onClose }) {
  const NONE_PROXY_POOL_VALUE = "__none__";
  const isOllamaLocal = provider === "ollama-local";
  const isCookie = authType === "cookie";
  const isXaiApiKey = provider === "xai" && !isCookie;
  const credentialLabel = isCookie ? "Cookie Value" : "API Key";
  const credentialPlaceholder = isCookie
    ? (provider === "grok-web" ? "sso=xxxxx... or just the raw value" : "eyJhbGciOi...")
    : (isXaiApiKey ? "xai-..." : "");

  const isAzure = provider === "azure";
  const isCloudflareAi = provider === "cloudflare-ai";
  const providerRegions = AI_PROVIDERS?.[provider]?.regions || null;
  const defaultRegion = AI_PROVIDERS?.[provider]?.defaultRegion || providerRegions?.[0]?.id || "";

  const [formData, setFormData] = useState({
    name: "",
    apiKey: "",
    priority: 1,
    proxyPoolId: NONE_PROXY_POOL_VALUE,
    ollamaHostUrl: "",
  });
  const [azureData, setAzureData] = useState({
    azureEndpoint: "",
    apiVersion: "2024-10-01-preview",
    deployment: "",
    organization: "",
  });
  const [cloudflareData, setCloudflareData] = useState({ accountId: "" });
  const [region, setRegion] = useState(defaultRegion);
  const [validating, setValidating] = useState(false);
  const [validationResult, setValidationResult] = useState(null);
  const [saving, setSaving] = useState(false);
  const bulkPlaceholder = isCloudflareAi
    ? `name1|sk-key1|acc123456\nname2|sk-key2|def789012\nsk-key-only-auto-named`
    : provider === "exa"
      ? `account-1|exa-key-1\naccount-2|exa-key-2\nexa-key-only-auto-named`
      : BULK_PLACEHOLDER;

  const [mode, setMode] = useState("single"); // "single" | "bulk"
  const [bulkText, setBulkText] = useState("");
  const [bulkResult, setBulkResult] = useState(null); // { success, failed }

  const buildProviderSpecificData = () => {
    if (isOllamaLocal && formData.ollamaHostUrl.trim()) {
      return { baseUrl: formData.ollamaHostUrl.trim() };
    }
    if (isAzure) {
      return {
        azureEndpoint: azureData.azureEndpoint,
        apiVersion: azureData.apiVersion,
        deployment: azureData.deployment,
        organization: azureData.organization,
      };
    }
    if (isCloudflareAi) {
      return { accountId: cloudflareData.accountId };
    }
    if (providerRegions && region) {
      return { region };
    }
    return undefined;
  };

  const handleValidate = async () => {
    setValidating(true);
    try {
      const res = await fetch("/api/providers/validate", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ provider, apiKey: formData.apiKey, providerSpecificData: buildProviderSpecificData() }),
      });
      const data = await res.json();
      setValidationResult(data.valid ? "success" : "failed");
    } catch {
      setValidationResult("failed");
    } finally {
      setValidating(false);
    }
  };

  const handleSubmit = async () => {
    if (!provider) return;
    if (!isOllamaLocal && !formData.apiKey) return;
    if (!isOllamaLocal) {
      // Non-ollama providers require a name
      if (!formData.name) return;
    }


    setSaving(true);
    try {
      let isValid = false;
      try {
        setValidating(true);
        setValidationResult(null);
        const res = await fetch("/api/providers/validate", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ provider, apiKey: formData.apiKey, providerSpecificData: buildProviderSpecificData() }),
        });
        const data = await res.json();
        isValid = !!data.valid;
        setValidationResult(isValid ? "success" : "failed");
      } catch {
        setValidationResult("failed");
      } finally {
        setValidating(false);
      }

      await onSave({
        name: formData.name || (isOllamaLocal ? "Ollama Local" : ""),
        apiKey: formData.apiKey,
        priority: formData.priority,
        proxyPoolId: formData.proxyPoolId === NONE_PROXY_POOL_VALUE ? null : formData.proxyPoolId,
        testStatus: isValid ? "active" : "unknown",
        providerSpecificData: buildProviderSpecificData()
      });
    } finally {
      setSaving(false);
    }
  };

  const handleBulkSubmit = async () => {
    const lines = bulkText.split("\n");
    if (!lines.length) return;
    // Plan collision-free names against existing connections so a generated
    // "Key N" never matches a saved name (which the backend would upsert /
    // overwrite instead of inserting). See bulkAdd.js for the full rationale.
    const plan = planBulkAdd(lines, existingNames, { isCloudflareAi });
    if (!plan.length) return;
    setSaving(true);
    setBulkResult(null);
    let success = 0;
    let failed = 0;
    try {
      const res = await fetch("/api/providers/bulk", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ items: plan.map((entry) => ({ provider, apiKey: entry.apiKey, name: entry.name, priority: 1, providerSpecificData: entry.providerSpecificData })) }),
      });
      const data = await res.json();
      success = Array.isArray(data.results) ? data.results.filter((item) => item.ok).length : 0;
      failed = plan.length - success;
    } catch {
      failed = plan.length;
    }
    setSaving(false);
    setBulkResult({ success, failed });
    if (success > 0 && onBulkDone) onBulkDone();
  };

  if (!provider) return null;

  return (
    <Modal isOpen={isOpen} title={`Add ${providerName || provider} ${credentialLabel}`} onClose={onClose}>
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {/* Mode switcher */}
        <Box sx={{ display: "flex", gap: 1 }}>
          <Button size="sm" variant={mode === "single" ? "primary" : "ghost"} onClick={() => { setMode("single"); setBulkResult(null); }}>Single</Button>
          <Button size="sm" variant={mode === "bulk" ? "primary" : "ghost"} onClick={() => { setMode("bulk"); setBulkResult(null); }}>Bulk Add</Button>
        </Box>

        {mode === "bulk" && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
            <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
              {isCloudflareAi
                ? <>One key per line. Format: <Box component="code">name|apiKey|accountId</Box> or just <Box component="code">apiKey</Box> (auto-named by index).</>
                : provider === "exa"
                  ? <>One Exa account per line: <Box component="code">name|apiKey</Box> or <Box component="code">apiKey</Box>. Keys are stored without a billable probe; runtime rotates on auth, quota, and rate-limit errors.</>
                  : <>One key per line. Format: <Box component="code">name|apiKey</Box> or just <Box component="code">apiKey</Box> (auto-named by index).</>
              }
            </Typography>
            <Box
              component="textarea"
              sx={{
                width: "100%",
                borderRadius: "4px",
                border: 1,
                borderColor: "rgba(229,106,74,0.3)",
                bgcolor: "var(--color-sidebar)",
                p: 1,
                fontSize: 14,
                fontFamily: "monospace",
                resize: "vertical",
                minHeight: 140,
                "&:focus": { outline: "none", boxShadow: "0 0 0 1px var(--color-primary)" },
              }}
              placeholder={bulkPlaceholder}
              value={bulkText}
              onChange={(e) => setBulkText(e.target.value)}
            />
            {bulkResult && (
              <Box sx={{ fontSize: 14, fontWeight: 500, color: bulkResult.failed > 0 ? "#facc15" : "#4ade80" }}>
                ✓ {bulkResult.success} added{bulkResult.failed > 0 ? `, ✗ ${bulkResult.failed} failed` : ""}
              </Box>
            )}
            <Box sx={{ display: "flex", gap: 1 }}>
              <Button onClick={handleBulkSubmit} fullWidth disabled={saving || !bulkText.trim()}>
                {saving ? "Adding..." : "Add All Keys"}
              </Button>
              <Button onClick={onClose} variant="ghost" fullWidth>Cancel</Button>
            </Box>
          </Box>
        )}

        {mode === "single" && (<>
        <Input
          label="Name"
          value={formData.name}
          onChange={(e) => setFormData({ ...formData, name: e.target.value })}
          placeholder={isOllamaLocal ? "Ollama Local" : "Production Key"}
        />
        {isOllamaLocal && (
          <Box sx={{ display: "flex", gap: 1 }}>
            <Box sx={{ flex: 1 }}>
              <Input
                label="Ollama Host URL"
                value={formData.ollamaHostUrl}
                onChange={(e) => setFormData({ ...formData, ollamaHostUrl: e.target.value })}
                placeholder="http://localhost:11434"
              />
            </Box>
            <Box sx={{ pt: 3 }}>
              <Button onClick={handleValidate} disabled={validating || saving} variant="secondary">
                {validating ? "Checking..." : "Check"}
              </Button>
            </Box>
          </Box>
        )}
        {!isOllamaLocal && (
          <Box sx={{ display: "flex", gap: 1 }}>
            <Box sx={{ flex: 1 }}>
              <Input
                label={credentialLabel}
                type={isCookie ? "text" : "password"}
                value={formData.apiKey}
                onChange={(e) => setFormData({ ...formData, apiKey: e.target.value })}
                placeholder={credentialPlaceholder}
              />
            </Box>
            <Box sx={{ pt: 3 }}>
              <Button onClick={handleValidate} disabled={!formData.apiKey || validating || saving} variant="secondary">
                {validating ? "Checking..." : "Check"}
              </Button>
            </Box>
          </Box>
        )}
        {isXaiApiKey && (
          <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
            Use a direct xAI API key from console.x.ai. This is separate from Grok Build OAuth.
          </Typography>
        )}
        {isCookie && authHint && (
          <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
            {authHint}
            {website && (
              <>
                {" "}
                <Box component="a" href={website} target="_blank" rel="noopener noreferrer" sx={{ color: "primary.main", textDecoration: "underline" }}>
                  Open {website.replace(/^https?:\/\//, "")}
                </Box>
              </>
            )}
          </Typography>
        )}
        {providerRegions && (
          <Select
            label="Region"
            value={region}
            onChange={(e) => setRegion(e.target.value)}
            options={providerRegions.map((r) => ({ value: r.id, label: r.label }))}
          />
        )}

        {isOllamaLocal && (
          <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
            Leave blank to use <Box component="code">http://localhost:11434</Box>. For remote Ollama, enter the full host URL (e.g. <Box component="code">http://192.168.1.10:11434</Box>).
          </Typography>
        )}
        {validationResult && (
          <Badge variant={validationResult === "success" ? "success" : "error"}>
            {validationResult === "success" ? "Valid" : "Invalid"}
          </Badge>
        )}
        {error && (
          <Typography sx={{ fontSize: 12, color: "error.main", wordBreak: "break-word" }}>{error}</Typography>
        )}

        {isCloudflareAi && (
          <Box sx={{ bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(30,30,30,0.5)" : "rgba(244,241,236,0.5)"), p: 2, borderRadius: "8px", border: 1, borderColor: "rgba(229,106,74,0.2)" }}>
            <Typography component="h3" sx={{ fontWeight: 600, mb: 1.5, fontSize: 14 }}>Cloudflare Workers AI</Typography>
            <Input
              label="Account ID"
              value={cloudflareData.accountId}
              onChange={(e) => setCloudflareData({ ...cloudflareData, accountId: e.target.value })}
              placeholder="abc123def456..."
            />
            <Typography sx={{ fontSize: 12, color: "text.secondary", mt: 1 }}>
              Find your Account ID in the right sidebar of <Box component="a" href="https://dash.cloudflare.com" target="_blank" rel="noopener noreferrer" sx={{ color: "primary.main", textDecoration: "underline" }}>dash.cloudflare.com</Box>
            </Typography>
          </Box>
        )}
        {isAzure && (
          <Box sx={{ bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(30,30,30,0.5)" : "rgba(244,241,236,0.5)"), p: 2, borderRadius: "8px", border: 1, borderColor: "rgba(229,106,74,0.2)" }}>
            <Typography component="h3" sx={{ fontWeight: 600, mb: 1.5, fontSize: 14 }}>Azure OpenAI Configuration</Typography>
            <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
              <Input
                label="Azure Endpoint"
                value={azureData.azureEndpoint}
                onChange={(e) => setAzureData({ ...azureData, azureEndpoint: e.target.value })}
                placeholder="https://your-resource.openai.azure.com"
              />
              <Input
                label="Deployment Name"
                value={azureData.deployment}
                onChange={(e) => setAzureData({ ...azureData, deployment: e.target.value })}
                placeholder="gpt-4"
              />
              <Input
                label="API Version"
                value={azureData.apiVersion}
                onChange={(e) => setAzureData({ ...azureData, apiVersion: e.target.value })}
                placeholder="2024-10-01-preview"
              />
              <Input
                label="Organization"
                value={azureData.organization}
                onChange={(e) => setAzureData({ ...azureData, organization: e.target.value })}
                placeholder="Organization ID"
              />
            </Box>
          </Box>
        )}

        <Input
          label="Priority"
          type="number"
          value={formData.priority}
          onChange={(e) => setFormData({ ...formData, priority: Number.parseInt(e.target.value) || 1 })}
        />

        <Select
          label="Proxy Pool"
          value={formData.proxyPoolId}
          onChange={(e) => setFormData({ ...formData, proxyPoolId: e.target.value })}
          options={[
            { value: NONE_PROXY_POOL_VALUE, label: "None" },
            ...(proxyPools || []).map((pool) => ({ value: pool.id, label: pool.name })),
          ]}
          placeholder="None"
        />

        {(proxyPools || []).length === 0 && (
          <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
            No active proxy pools available. Create one in Proxy Pools page first.
          </Typography>
        )}

        <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
          Legacy manual proxy fields are still accepted by API for backward compatibility.
        </Typography>

        <Box sx={{ display: "flex", gap: 1 }}>
          <Button onClick={handleSubmit} fullWidth disabled={saving || (!isOllamaLocal && (!formData.name || !formData.apiKey)) || (isAzure && (!azureData.azureEndpoint || !azureData.deployment || !azureData.organization)) || (isCloudflareAi && !cloudflareData.accountId)}>
            {saving ? "Saving..." : "Save"}
          </Button>
          <Button onClick={onClose} variant="ghost" fullWidth>
            Cancel
          </Button>
        </Box>
        </>)}
      </Box>
    </Modal>
  );
}
