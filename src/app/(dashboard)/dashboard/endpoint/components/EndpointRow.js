"use client";

import Box from "@mui/material/Box";
import Icon from "@mui/material/Icon";
import IconButton from "@mui/material/IconButton";
import { Input } from "@/shared/components";

/** Reusable endpoint row component */
export default function EndpointRow({ label, url, copyId, copied, onCopy, badge, actions }) {
  const highlighted = badge === "CF" || badge === "TS";
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
      <Box
        component="span"
        sx={{
          fontSize: 12,
          fontFamily: "ui-monospace, monospace",
          px: 0.75,
          py: 0.25,
          borderRadius: "4px",
          flexShrink: 0,
          minWidth: 88,
          textAlign: "center",
          bgcolor: highlighted ? "rgba(229,106,74,0.10)" : "var(--color-surface-2)",
          color: highlighted ? "primary.main" : "text.secondary",
        }}
      >
        {label}
      </Box>
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Input value={url} readOnly />
      </Box>
      <IconButton
        onClick={() => onCopy(url, copyId)}
        aria-label={`Copy ${label} URL`}
        sx={{ color: "text.secondary", "&:hover": { color: "primary.main", bgcolor: "var(--color-surface-2)" } }}
      >
        <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18 }}>
          {copied === copyId ? "check" : "content_copy"}
        </Icon>
      </IconButton>
      {actions}
    </Box>
  );
}
