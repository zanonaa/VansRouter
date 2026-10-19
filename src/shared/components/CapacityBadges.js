"use client";

import Box from "@mui/material/Box";
import { CAPACITY_META } from "@/shared/constants/models";
import Tooltip from "./Tooltip";

// CAPACITY_META exposes Tailwind text-color classes; map them to raw colors so
// this component no longer depends on Tailwind utilities. A caller-supplied
// colorOverride is still applied as a className passthrough.
const CAP_COLORS = {
  "text-blue-500": "#3b82f6",
  "text-amber-500": "#f59e0b",
};

// Render small icon badges for a model's capabilities (only those set true).
// colorOverride: force a single color class for all badges (default: per-cap color).
// size: icon font-size in px (default 16).
export default function CapacityBadges({ caps, className = "", colorOverride, size = 16 }) {
  if (!caps) return null;
  const active = Object.keys(CAPACITY_META).filter((k) => caps[k]);
  if (active.length === 0) return null;

  return (
    <Box component="span" className={className} sx={{ display: "inline-flex", alignItems: "center", gap: 0.5 }}>
      {active.map((k) => (
        <Tooltip key={k} text={`${CAPACITY_META[k].label} — ${CAPACITY_META[k].desc}`}>
          <span
            className="material-symbols-outlined"
            style={{ fontSize: `${size}px`, lineHeight: 1, cursor: "help", color: colorOverride ? undefined : CAP_COLORS[CAPACITY_META[k].color] }}
          >
            {CAPACITY_META[k].icon}
          </span>
        </Tooltip>
      ))}
    </Box>
  );
}
