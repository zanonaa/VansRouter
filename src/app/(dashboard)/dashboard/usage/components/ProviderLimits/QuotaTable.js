"use client";

import { useMemo, useState } from "react";
import {
  Box,
  Button,
  Table,
  TableBody,
  TableCell,
  TableRow,
  Typography,
} from "@mui/material";
import { formatResetTime, getRemainingPercentage } from "./utils";

const PAGE_SIZE = 10;

/**
 * Format reset time display (Today, 12:00 PM)
 */
function formatResetTimeDisplay(resetTime) {
  if (!resetTime) return null;

  try {
    const date = new Date(resetTime);
    const now = new Date();
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    const tomorrow = new Date(today);
    tomorrow.setDate(tomorrow.getDate() + 1);

    let dayStr = "";
    if (date >= today && date < tomorrow) {
      dayStr = "Today";
    } else if (date >= tomorrow && date < new Date(tomorrow.getTime() + 24 * 60 * 60 * 1000)) {
      dayStr = "Tomorrow";
    } else {
      dayStr = date.toLocaleDateString("en-US", { month: "short", day: "numeric" });
    }

    const timeStr = date.toLocaleTimeString("en-US", {
      hour: "numeric",
      minute: "2-digit",
      hour12: true,
    });

    return `${dayStr}, ${timeStr}`;
  } catch {
    return null;
  }
}

/**
 * Get colors based on remaining percentage
 */
function getColorConfig(remainingPercentage) {
  if (remainingPercentage > 70) {
    return {
      text: (theme) => (theme.palette.mode === "dark" ? "#4ade80" : "#16a34a"),
      bg: "#22c55e",
      bgLight: "rgba(34, 197, 94, 0.1)",
      emoji: "🟢",
    };
  }

  if (remainingPercentage >= 30) {
    return {
      text: (theme) => (theme.palette.mode === "dark" ? "#facc15" : "#ca8a04"),
      bg: "#eab308",
      bgLight: "rgba(234, 179, 8, 0.1)",
      emoji: "🟡",
    };
  }

  return {
    text: (theme) => (theme.palette.mode === "dark" ? "#f87171" : "#dc2626"),
    bg: "#ef4444",
    bgLight: "rgba(239, 68, 68, 0.1)",
    emoji: "🔴",
  };
}

function sortQuotas(quotas, sortMode) {
  if (sortMode === "remaining-asc") {
    return quotas.toSorted((a, b) => a.remaining - b.remaining || a.name.localeCompare(b.name));
  }

  if (sortMode === "remaining-desc") {
    return quotas.toSorted((a, b) => b.remaining - a.remaining || a.name.localeCompare(b.name));
  }

  return quotas;
}

const EMPTY_QUOTAS = [];
const borderSubtle = (theme) =>
  theme.palette.mode === "dark" ? "rgba(255, 255, 255, 0.05)" : "rgba(0, 0, 0, 0.05)";
const borderMuted = (theme) =>
  theme.palette.mode === "dark" ? "rgba(255, 255, 255, 0.1)" : "rgba(0, 0, 0, 0.1)";
const surfaceSubtle = (theme) =>
  theme.palette.mode === "dark" ? "rgba(255, 255, 255, 0.03)" : "rgba(0, 0, 0, 0.02)";
const hoverSubtle = (theme) =>
  theme.palette.mode === "dark" ? "rgba(255, 255, 255, 0.02)" : "rgba(0, 0, 0, 0.02)";

/**
 * Quota Table Component - Table-based display for quota data
 */
export default function QuotaTable({
  quotas = EMPTY_QUOTAS,
  compact = false,
  sortMode = "default",
  showSortLabel = false,
}) {
  const [page, setPage] = useState(1);

  const normalizedQuotas = useMemo(
    () => quotas.map((quota, index) => ({
      ...quota,
      index,
      remaining: getRemainingPercentage(quota),
    })),
    [quotas],
  );

  const sortedQuotas = useMemo(
    () => sortQuotas(normalizedQuotas, sortMode),
    [normalizedQuotas, sortMode],
  );

  const totalPages = Math.max(1, Math.ceil(sortedQuotas.length / PAGE_SIZE));

  const [prevSortMode, setPrevSortMode] = useState(sortMode);
  const [prevQuotas, setPrevQuotas] = useState(quotas);
  const [prevTotalPages, setPrevTotalPages] = useState(totalPages);
  if (sortMode !== prevSortMode || quotas !== prevQuotas) {
    setPrevSortMode(sortMode);
    setPrevQuotas(quotas);
    setPrevTotalPages(totalPages);
    setPage(1);
  } else if (totalPages !== prevTotalPages) {
    setPrevTotalPages(totalPages);
    setPage((currentPage) => Math.min(currentPage, totalPages));
  }

  if (!quotas || quotas.length === 0) {
    return null;
  }

  const currentPageRows = sortedQuotas.slice(
    (page - 1) * PAGE_SIZE,
    page * PAGE_SIZE,
  );
  const pageStart = sortedQuotas.length === 0 ? 0 : (page - 1) * PAGE_SIZE + 1;
  const pageEnd = Math.min(page * PAGE_SIZE, sortedQuotas.length);

  const cellSx = compact ? { py: 0.5, px: 0.75 } : { py: 1, px: 1.5 };
  const nameFontSize = compact ? 11 : 14;
  const resetPrimaryFontSize = compact ? 11 : 14;
  const resetSecondaryFontSize = compact ? 10 : 12;
  const sortLabel = "Sorted by account remaining";
  const chipSx = {
    borderRadius: 1,
    border: 1,
    borderColor: borderMuted,
    bgcolor: surfaceSubtle,
    px: 1,
    py: 0.5,
    fontSize: 10,
  };

  const paginationButtonSx = {
    minWidth: 0,
    height: 24,
    px: 1,
    fontSize: 10,
    lineHeight: 1,
    border: 1,
    borderColor: borderMuted,
    color: "text.primary",
    "&:hover": { bgcolor: hoverSubtle },
    "&.Mui-disabled": { opacity: 0.4 },
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography sx={{ fontSize: 10, color: "text.secondary" }}>
          {sortedQuotas.length} quota{sortedQuotas.length > 1 ? "s" : ""}
        </Typography>
        {showSortLabel && (
          <Box sx={chipSx}>
            {sortLabel}
          </Box>
        )}
      </Box>

      <Box sx={{ overflowX: "auto" }}>
        <Table
          size="small"
          sx={{ width: "100%", minWidth: 420, tableLayout: "fixed" }}
        >
          <TableBody>
            {currentPageRows.map((quota) => {
              const isUnlimited = quota.unlimited === true;
              const isCreditBalance = quota.isCreditBalance === true;
              const colors = isCreditBalance
                ? {
                    text: (theme) => (theme.palette.mode === "dark" ? "#60a5fa" : "#2563eb"),
                    bg: "#3b82f6",
                    bgLight: "rgba(59, 130, 246, 0.1)",
                    emoji: "💰",
                  }
                : getColorConfig(quota.remaining);
              const countdown = formatResetTime(quota.resetAt);
              const resetDisplay = formatResetTimeDisplay(quota.resetAt);

              return (
                <TableRow
                  key={`${quota.name}-${quota.index}`}
                  sx={{
                    borderBottom: 1,
                    borderColor: borderSubtle,
                    transition: "background-color 150ms ease",
                    "&:hover": { bgcolor: hoverSubtle },
                    "&:last-child td": { borderBottom: 0 },
                  }}
                >
                  <TableCell sx={{ ...cellSx, width: "30%", borderBottom: 0 }}>
                    <Box sx={{ display: "flex", alignItems: "center", gap: 0.75, minWidth: 0 }}>
                      <Box component="span" sx={{ fontSize: 10, flexShrink: 0 }}>
                        {colors.emoji}
                      </Box>
                      <Typography
                        noWrap
                        sx={{
                          fontSize: nameFontSize,
                          fontWeight: 500,
                          color: "text.primary",
                          minWidth: 0,
                        }}
                      >
                        {quota.name}
                      </Typography>
                    </Box>
                  </TableCell>

                  <TableCell sx={{ ...cellSx, width: "45%", borderBottom: 0 }}>
                    <Box sx={{ display: "flex", flexDirection: "column", gap: compact ? 0.5 : 0.75 }}>
                      {!isUnlimited && !isCreditBalance && (
                        <Box
                          sx={{
                            height: compact ? 4 : 6,
                            borderRadius: 999,
                            overflow: "hidden",
                            border: 1,
                            bgcolor: colors.bgLight,
                            borderColor:
                              quota.remaining === 0 ? borderMuted : "transparent",
                          }}
                        >
                          <Box
                            sx={{
                              height: "100%",
                              width: `${Math.min(quota.remaining, 100)}%`,
                              bgcolor: colors.bg,
                              transition: "width 300ms ease",
                            }}
                          />
                        </Box>
                      )}

                      <Box
                        sx={{
                          display: "flex",
                          alignItems: "center",
                          justifyContent: "space-between",
                          fontSize: compact ? 10 : 12,
                        }}
                      >
                        <Box
                          component="span"
                          title={
                            isUnlimited
                              ? `${quota.used.toLocaleString()} used · Unlimited`
                              : isCreditBalance
                                ? `Credit balance: ${quota.total.toFixed(2)} ${quota.currency || ""}`
                                : `${quota.used.toLocaleString()} / ${quota.total > 0 ? quota.total.toLocaleString() : "∞"}`
                          }
                          sx={{ color: "text.secondary" }}
                        >
                          {isUnlimited
                            ? `${quota.used.toLocaleString()} used · Unlimited`
                            : isCreditBalance
                              ? `Credit: ${quota.total.toFixed(2)} ${quota.currency || ""}`
                              : `${quota.used.toLocaleString()} / ${quota.total > 0 ? quota.total.toLocaleString() : "∞"}`}
                        </Box>
                        <Box
                          component="span"
                          sx={{
                            fontWeight: 500,
                            color: isUnlimited
                              ? (theme) => (theme.palette.mode === "dark" ? "#4ade80" : "#16a34a")
                              : colors.text,
                          }}
                        >
                          {isUnlimited ? "Unlimited" : isCreditBalance ? "" : `${quota.remaining}%`}
                        </Box>
                      </Box>
                    </Box>
                  </TableCell>

                  <TableCell sx={{ ...cellSx, width: "25%", borderBottom: 0 }}>
                    {countdown !== "-" || resetDisplay ? (
                      compact ? (
                        <Typography
                          noWrap
                          title={resetDisplay || ""}
                          sx={{
                            fontSize: resetPrimaryFontSize,
                            fontWeight: 500,
                            color: "text.primary",
                          }}
                        >
                          {countdown !== "-" ? `in ${countdown}` : resetDisplay}
                        </Typography>
                      ) : (
                        <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25 }}>
                          {countdown !== "-" && (
                            <Box
                              sx={{
                                fontSize: resetPrimaryFontSize,
                                fontWeight: 500,
                                color: "text.primary",
                              }}
                            >
                              in {countdown}
                            </Box>
                          )}
                          {resetDisplay && (
                            <Box sx={{ fontSize: resetSecondaryFontSize, color: "text.secondary" }}>
                              {resetDisplay}
                            </Box>
                          )}
                        </Box>
                      )
                    ) : (
                      <Box
                        sx={{
                          fontSize: resetPrimaryFontSize,
                          fontStyle: "italic",
                          color: "text.secondary",
                        }}
                      >
                        N/A
                      </Box>
                    )}
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </Box>

      {totalPages > 1 && (
        <Box
          sx={{
            borderRadius: 1,
            border: 1,
            borderColor: borderMuted,
            bgcolor: surfaceSubtle,
            px: 1,
            py: 0.75,
          }}
        >
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              gap: 1,
              fontSize: 10,
              color: "text.secondary",
            }}
          >
            <Box component="span">
              Showing {pageStart}-{pageEnd} of {sortedQuotas.length}
            </Box>
            <Box component="span">
              Page {page} / {totalPages}
            </Box>
          </Box>
          <Box sx={{ mt: 0.75, display: "flex", alignItems: "center", justifyContent: "flex-end", gap: 0.5 }}>
            <Button
              type="button"
              variant="outlined"
              onClick={() => setPage((currentPage) => Math.max(1, currentPage - 1))}
              disabled={page === 1}
              sx={paginationButtonSx}
            >
              Prev
            </Button>
            <Button
              type="button"
              variant="outlined"
              onClick={() => setPage((currentPage) => Math.min(totalPages, currentPage + 1))}
              disabled={page === totalPages}
              sx={paginationButtonSx}
            >
              Next
            </Button>
          </Box>
        </Box>
      )}
    </Box>
  );
}
