"use client";

import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Card from "./Card";

// Only show fields user actually cares about
const FIELD_SCHEMA = {
  mode:             { label: "Mode",       format: (v) => v },
  defaultModel:     { label: "Model",      format: (v) => v, mono: true },
  baseUrl:          { label: "Endpoint",   format: (v) => v, isLink: true, mono: true },
  costPerQuery:     { label: "Cost / call", format: (v) => v === 0 ? "Free" : `$${v.toFixed(4)}` },
  pricingUrl:       { label: "Pricing",    format: () => "View pricing", isLink: true },
  freeTier:         { label: "Free tier",  format: (v) => v },
  freeMonthlyQuota: { label: "Free quota",  format: (v) => v === 0 ? "—" : v >= 999999 ? "Unlimited" : `${v.toLocaleString()} / mo` },
  searchTypes:      { label: "Types",      format: (v) => v.join(", ") },
  formats:          { label: "Formats",    format: (v) => v.join(", ") },
  maxMaxResults:    { label: "Max results", format: (v) => v },
  maxCharacters:    { label: "Max chars",  format: (v) => v.toLocaleString() },
};

export default function ProviderInfoCard({ config, provider, title = "Provider Info" }) {
  if (!config) return null;

  const rows = Object.entries(FIELD_SCHEMA).reduce((acc, [key, schema]) => {
    if (config[key] !== undefined && config[key] !== null && config[key] !== "") {
      acc.push({ key, label: schema.label, value: schema.format(config[key]), isLink: schema.isLink, mono: schema.mono, raw: config[key] });
    }
    return acc;
  }, []);

  const signupUrl = provider?.notice?.apiKeyUrl || provider?.website;
  const noticeText = provider?.notice?.text;

  return (
    <Card>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", mb: 1.5 }}>
        <Typography variant="h6" sx={{ fontSize: 18, fontWeight: 600 }}>{title}</Typography>
        {signupUrl && (
          <Box
            component="a"
            href={signupUrl}
            target="_blank"
            rel="noopener noreferrer"
            sx={{ fontSize: 12, color: "primary.main", display: "inline-flex", alignItems: "center", gap: 0.5, "&:hover": { textDecoration: "underline" } }}
          >
            <span className="material-symbols-outlined" style={{ fontSize: 14 }}>open_in_new</span>
            Get API Key
          </Box>
        )}
      </Box>
      <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" }, columnGap: 3, rowGap: 1 }}>
        {rows.map((r) => (
          <Box key={r.key} sx={{ display: "flex", alignItems: "center", gap: 1.5, minWidth: 0 }}>
            <Box component="span" sx={{ fontSize: 12, color: "text.secondary", width: 112, flexShrink: 0 }}>{r.label}</Box>
            {r.isLink ? (
              <Box
                component="a"
                href={r.raw}
                target="_blank"
                rel="noopener noreferrer"
                sx={{ fontSize: 14, color: "primary.main", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontFamily: r.mono ? "ui-monospace, monospace" : undefined, "&:hover": { textDecoration: "underline" } }}
              >
                {r.value}
              </Box>
            ) : (
              <Box component="span" sx={{ fontSize: 14, color: "text.primary", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontFamily: r.mono ? "ui-monospace, monospace" : undefined }}>
                {r.value}
              </Box>
            )}
          </Box>
        ))}
        {noticeText && (
          <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1.5, minWidth: 0, gridColumn: { sm: "span 2" } }}>
            <Box component="span" sx={{ fontSize: 12, color: "text.secondary", width: 112, flexShrink: 0, mt: 0.25 }}>Notice</Box>
            <Box component="span" sx={{ fontSize: 14, color: "text.primary", lineHeight: 1.6 }}>{noticeText}</Box>
          </Box>
        )}
      </Box>
    </Card>
  );
}
