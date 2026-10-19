"use client";

import MuiTooltip from "@mui/material/Tooltip";
import Icon from "@mui/material/Icon";

/** Inline tooltip, Claude Code CLI style */
export default function Tooltip({ text }) {
  return (
    <MuiTooltip
      title={text}
      placement="top"
      slotProps={{
        tooltip: {
          sx: {
            maxWidth: 288,
            fontSize: 12,
            lineHeight: 1.4,
            bgcolor: "#1f2937",
            "& .MuiTooltip-arrow": { color: "#1f2937" },
          },
        },
      }}
    >
      <Icon
        className="material-symbols-outlined"
        baseClassName=""
        sx={{ fontSize: 14, color: "text.secondary", cursor: "help", display: "inline-flex" }}
      >
        help
      </Icon>
    </MuiTooltip>
  );
}
