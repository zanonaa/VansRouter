"use client";

import Image from "next/image";
import Box from "@mui/material/Box";
import { cn } from "@/shared/utils/cn";

const AVATAR_SIZES = {
  xs: { size: 24, fontSize: 12 },
  sm: { size: 32, fontSize: 14 },
  md: { size: 40, fontSize: 16 },
  lg: { size: 48, fontSize: 18 },
  xl: { size: 64, fontSize: 20 },
};

// Get initials from name
function getInitials(name) {
  if (!name) return "?";
  const parts = name.split(" ");
  if (parts.length >= 2) {
    return `${parts[0][0]}${parts[1][0]}`.toUpperCase();
  }
  return name.substring(0, 2).toUpperCase();
}

// Generate color from name (previously bg-*-500 Tailwind utilities)
const COLORS = [
  "#ef4444", "#f97316", "#f59e0b", "#eab308", "#84cc16",
  "#22c55e", "#10b981", "#14b8a6", "#06b6d4", "#0ea5e9",
  "#3b82f6", "#6366f1", "#8b5cf6", "#a855f7", "#d946ef",
  "#ec4899", "#f43f5e",
];

function getColorFromName(name) {
  if (!name) return "var(--color-primary)";
  const index = name.charCodeAt(0) % COLORS.length;
  return COLORS[index];
}

export default function Avatar({
  src,
  alt = "Avatar",
  name,
  size = "md",
  className,
}) {
  const dims = AVATAR_SIZES[size] || AVATAR_SIZES.md;
  const ring = {
    outline: "2px solid var(--color-surface, #fff)",
    outlineOffset: 0,
    boxShadow: "0 1px 2px 0 rgba(0,0,0,0.05)",
  };

  if (src) {
    return (
      <Box
        component="span"
        className={cn(className)}
        sx={{
          position: "relative",
          display: "inline-block",
          borderRadius: "50%",
          overflow: "hidden",
          width: dims.size,
          height: dims.size,
          ...ring,
        }}
      >
        <Image
          src={src}
          alt={alt}
          fill
          sizes="48px"
          style={{ objectFit: "cover" }}
          unoptimized
        />
      </Box>
    );
  }

  return (
    <Box
      component="span"
      className={cn(className)}
      aria-label={alt}
      sx={{
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        borderRadius: "50%",
        fontWeight: 600,
        color: "#fff",
        width: dims.size,
        height: dims.size,
        fontSize: dims.fontSize,
        bgcolor: getColorFromName(name),
        ...ring,
      }}
    >
      {getInitials(name)}
    </Box>
  );
}
