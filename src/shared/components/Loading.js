"use client";

import Box from "@mui/material/Box";
import { cn } from "@/shared/utils/cn";

const SPINNER_SIZES = { sm: 16, md: 24, lg: 32, xl: 48 };

// Spinner loading
export function Spinner({ size = "md", className }) {
  return (
    <span
      className={`material-symbols-outlined${className ? ` ${cn(className)}` : ""}`}
      style={{ fontSize: SPINNER_SIZES[size] || SPINNER_SIZES.md, color: "var(--color-brand-500)", animation: "spin 1s linear infinite" }}
    >
      progress_activity
    </span>
  );
}

// Full page loading
export function PageLoading({ message = "Loading..." }) {
  return (
    <Box
      sx={{
        position: "fixed",
        inset: 0,
        zIndex: 50,
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        bgcolor: "background.default",
      }}
    >
      <Spinner size="xl" />
      <Box component="p" sx={{ mt: 2, color: "text.secondary" }}>
        {message}
      </Box>
    </Box>
  );
}

// Skeleton loading
export function Skeleton({ className, ...props }) {
  return (
    <Box
      className={cn(className)}
      sx={{
        animation: "pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite",
        borderRadius: "10px",
        bgcolor: "var(--color-surface-2)",
      }}
      {...props}
    />
  );
}

// Card skeleton
export function CardSkeleton() {
  return (
    <Box
      sx={{
        p: 3,
        borderRadius: "14px",
        border: "1px solid var(--color-border-subtle)",
        bgcolor: "background.paper",
        boxShadow: "var(--shadow-soft)",
      }}
    >
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", mb: 2 }}>
        <Skeleton sx={{ height: 16, width: 96 }} />
        <Skeleton sx={{ width: 40, height: 40, borderRadius: "10px" }} />
      </Box>
      <Skeleton sx={{ height: 32, width: 64, mb: 1 }} />
      <Skeleton sx={{ height: 12, width: 80 }} />
    </Box>
  );
}

export default function Loading({ type = "spinner", ...props }) {
  switch (type) {
    case "page":
      return <PageLoading {...props} />;
    case "skeleton":
      return <Skeleton {...props} />;
    case "card":
      return <CardSkeleton {...props} />;
    default:
      return <Spinner {...props} />;
  }
}
