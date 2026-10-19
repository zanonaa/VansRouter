"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import MuiLink from "@mui/material/Link";
import Icon from "@mui/material/Icon";
import { Card, Button, Input, Select, Toggle } from "@/shared/components";
import { AI_PROVIDERS, AUTH_METHODS } from "@/shared/constants/config";

const providerOptions = Object.values(AI_PROVIDERS).map((p) => ({
  value: p.id,
  label: p.name,
}));

const authMethodOptions = Object.values(AUTH_METHODS).map((m) => ({
  value: m.id,
  label: m.name,
}));

export default function NewProviderPage() {
  const router = useRouter();
  const [loading, setLoading] = useState(false);
  const [formData, setFormData] = useState({
    provider: "",
    authMethod: "api_key",
    apiKey: "",
    displayName: "",
    isActive: true,
  });
  const [errors, setErrors] = useState({});

  const handleChange = (field, value) => {
    setFormData((prev) => ({ ...prev, [field]: value }));
    if (errors[field]) {
      setErrors((prev) => ({ ...prev, [field]: null }));
    }
  };

  const validate = () => {
    const newErrors = {};
    if (!formData.provider) newErrors.provider = "Please select a provider";
    if (formData.authMethod === "api_key" && !formData.apiKey) {
      newErrors.apiKey = "API Key is required";
    }
    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!validate()) return;

    setLoading(true);
    try {
      const response = await fetch("/api/providers", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(formData),
      });

      if (response.ok) {
        router.push("/dashboard/providers");
      } else {
        const data = await response.json();
        setErrors({ submit: data.error || "Failed to create provider" });
      }
    } catch (error) {
      setErrors({ submit: "An error occurred. Please try again." });
    } finally {
      setLoading(false);
    }
  };

  const selectedProvider = AI_PROVIDERS[formData.provider];

  return (
    <Box sx={{ maxWidth: 672, mx: "auto" }}>
      {/* Header */}
      <Box sx={{ mb: 4 }}>
        <MuiLink
          component={Link}
          href="/dashboard/providers"
          underline="none"
          sx={{
            display: "inline-flex",
            alignItems: "center",
            gap: 0.5,
            fontSize: 14,
            color: "text.secondary",
            mb: 2,
            transition: "color 150ms ease",
            "&:hover": { color: "primary.main" },
          }}
        >
          <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18 }}>arrow_back</Icon>
          Back to Providers
        </MuiLink>
        <Typography variant="h4" sx={{ fontWeight: 600, letterSpacing: "-0.025em" }}>Add New Provider</Typography>
        <Typography sx={{ color: "text.secondary", mt: 1 }}>
          Configure a new AI provider to use with your applications.
        </Typography>
      </Box>

      {/* Form */}
      <Card>
        <Box component="form" onSubmit={handleSubmit} sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
          {/* Provider Selection */}
          <Select
            label="Provider"
            options={providerOptions}
            value={formData.provider}
            onChange={(e) => handleChange("provider", e.target.value)}
            placeholder="Select a provider"
            error={errors.provider}
            required
          />

          {/* Provider Info */}
          {selectedProvider && (
            <Card.Section sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
              <Box
                sx={{
                  width: 40,
                  height: 40,
                  borderRadius: "8px",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  bgcolor: "background.default",
                  border: 1,
                  borderColor: "divider",
                }}
              >
                <Icon
                  className="material-symbols-outlined"
                  baseClassName=""
                  sx={{ fontSize: 20, color: selectedProvider.color }}
                >
                  {selectedProvider.icon}
                </Icon>
              </Box>
              <Box>
                <Typography sx={{ fontWeight: 500 }}>{selectedProvider.name}</Typography>
                <Typography sx={{ fontSize: 14, color: "text.secondary" }}>
                  Selected provider
                </Typography>
              </Box>
            </Card.Section>
          )}

          {/* Auth Method */}
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
            <Typography component="span" sx={{ fontSize: 14, fontWeight: 500 }}>
              Authentication Method <Box component="span" sx={{ color: "error.main" }}>*</Box>
            </Typography>
            <Box sx={{ display: "flex", gap: 1.5 }}>
              {authMethodOptions.map((method) => {
                const selected = formData.authMethod === method.value;
                return (
                  <Box
                    key={method.value}
                    component="button"
                    type="button"
                    onClick={() => handleChange("authMethod", method.value)}
                    sx={{
                      flex: 1,
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                      gap: 1,
                      p: 2,
                      borderRadius: "8px",
                      border: 1,
                      borderColor: selected ? "primary.main" : "divider",
                      bgcolor: selected ? "rgba(229,106,74,0.05)" : "transparent",
                      color: selected ? "primary.main" : "text.primary",
                      cursor: "pointer",
                      transition: "all 150ms ease",
                      "&:hover": { borderColor: selected ? "primary.main" : "rgba(229,106,74,0.5)" },
                    }}
                  >
                    <Icon className="material-symbols-outlined" baseClassName="">
                      {method.value === "api_key" ? "key" : "lock"}
                    </Icon>
                    <Box component="span" sx={{ fontWeight: 500 }}>{method.label}</Box>
                  </Box>
                );
              })}
            </Box>
          </Box>

          {/* API Key Input */}
          {formData.authMethod === "api_key" && (
            <Input
              label="API Key"
              type="password"
              placeholder="Enter your API key"
              value={formData.apiKey}
              onChange={(e) => handleChange("apiKey", e.target.value)}
              error={errors.apiKey}
              hint="Your API key will be encrypted and stored securely."
              required
            />
          )}

          {/* OAuth2 Button */}
          {formData.authMethod === "oauth2" && (
            <Card.Section>
              <Typography sx={{ fontSize: 14, color: "text.secondary", mb: 2 }}>
                Connect your account using OAuth2 authentication.
              </Typography>
              <Button type="button" variant="secondary" icon="link">
                Connect with OAuth2
              </Button>
            </Card.Section>
          )}

          {/* Display Name */}
          <Input
            label="Display Name"
            placeholder="e.g., Production API, Dev Environment"
            value={formData.displayName}
            onChange={(e) => handleChange("displayName", e.target.value)}
            hint="Optional. A friendly name to identify this configuration."
          />

          {/* Active Toggle */}
          <Toggle
            checked={formData.isActive}
            onChange={(checked) => handleChange("isActive", checked)}
            label="Active"
            description="Enable this provider for use in your applications"
          />

          {/* Error Message */}
          {errors.submit && (
            <Box sx={{ p: 2, borderRadius: "8px", bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(127,29,29,0.2)" : "#fef2f2"), border: 1, borderColor: (t) => (t.palette.mode === "dark" ? "#991b1b" : "#fecaca"), color: (t) => (t.palette.mode === "dark" ? "#f87171" : "#dc2626"), fontSize: 14 }}>
              {errors.submit}
            </Box>
          )}

          {/* Actions */}
          <Box sx={{ display: "flex", gap: 1.5, pt: 2, borderTop: 1, borderColor: "divider", "& > *": { flex: 1 } }}>
            <Link href="/dashboard/providers">
              <Button type="button" variant="ghost" fullWidth>
                Cancel
              </Button>
            </Link>
            <Button type="submit" loading={loading} fullWidth>
              Create Provider
            </Button>
          </Box>
        </Box>
      </Card>
    </Box>
  );
}
