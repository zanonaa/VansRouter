"use client";

import { useState, useCallback, useMemo, Fragment, useEffect } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Table from "@mui/material/Table";
import TableHead from "@mui/material/TableHead";
import TableBody from "@mui/material/TableBody";
import TableRow from "@mui/material/TableRow";
import TableCell from "@mui/material/TableCell";
import TableContainer from "@mui/material/TableContainer";
import Icon from "@mui/material/Icon";
import Card from "@/shared/components/Card";
import Badge from "@/shared/components/Badge";

const _nf = new Intl.NumberFormat();
const fmt = (n) => _nf.format(n || 0);
const fmtCost = (n) => `$${(n || 0).toFixed(2)}`;

function fmtTime(iso) {
  if (!iso) return "Never";
  const diffMins = Math.floor((Date.now() - new Date(iso)) / 60000);
  if (diffMins < 1) return "Just now";
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffMins < 1440) return `${Math.floor(diffMins / 60)}h ago`;
  return new Date(iso).toLocaleDateString();
}

function SortIcon({ field, currentSort, currentOrder }) {
  if (currentSort !== field) return <Box component="span" sx={{ ml: 0.5, opacity: 0.2 }}>↕</Box>;
  return <Box component="span" sx={{ ml: 0.5 }}>{currentOrder === "asc" ? "↑" : "↓"}</Box>;
}

const rightCellSx = { px: 3, py: 1.5, textAlign: "right", color: "text.secondary" };
const totalCellSx = { px: 3, py: 1.5, textAlign: "right", fontWeight: 500 };

/**
 * Render 3 token or cost cells based on viewMode
 */
function ValueCells({ item, viewMode, isSummary = false }) {
  if (viewMode === "tokens") {
    return (
      <>
        <TableCell sx={rightCellSx}>
          {isSummary && item.promptTokens === undefined ? "—" : fmt(item.promptTokens)}
        </TableCell>
        <TableCell sx={rightCellSx}>
          {isSummary && item.completionTokens === undefined ? "—" : fmt(item.completionTokens)}
        </TableCell>
        <TableCell sx={totalCellSx}>
          {fmt(item.totalTokens)}
        </TableCell>
      </>
    );
  }
  return (
    <>
      <TableCell sx={rightCellSx}>
        {isSummary && item.inputCost === undefined ? "—" : fmtCost(item.inputCost)}
      </TableCell>
      <TableCell sx={rightCellSx}>
        {isSummary && item.outputCost === undefined ? "—" : fmtCost(item.outputCost)}
      </TableCell>
      <TableCell sx={{ ...totalCellSx, color: "warning.main" }}>
        {fmtCost(item.totalCost || item.cost)}
      </TableCell>
    </>
  );
}

const headCellSx = {
  px: 3,
  py: 1.5,
  fontSize: 12,
  textTransform: "uppercase",
  color: "text.secondary",
  cursor: "pointer",
  whiteSpace: "nowrap",
  "&:hover": { bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(42,42,42,0.5)" : "rgba(244,244,245,0.5)") },
};

const rowHoverSx = {
  "&:hover": { bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(42,42,42,0.5)" : "rgba(244,244,245,0.5)") },
};

/**
 * Reusable sortable usage table with expandable group rows.
 *
 * @param {object} props
 * @param {string} props.title - Table title
 * @param {Array} props.columns - Column definitions [{field, label}]
 * @param {Array} props.groupedData - Grouped data from groupDataByKey
 * @param {string} props.tableType - Table type key for sort URL params
 * @param {string} props.sortBy - Current sort field
 * @param {string} props.sortOrder - Current sort order
 * @param {function} props.onToggleSort - Sort toggle handler
 * @param {string} props.viewMode - "tokens" or "costs"
 * @param {string} props.storageKey - localStorage key for expanded state
 * @param {function} props.renderGroupLabel - Render group summary first cell content
 * @param {function} props.renderDetailCells - Render detail row custom cells (before value cells)
 * @param {function} props.renderSummaryCells - Render summary row cells after group label (placeholder cols)
 * @param {string} props.emptyMessage - Empty state message
 */
export default function UsageTable({
  title,
  columns,
  groupedData,
  tableType,
  sortBy,
  sortOrder,
  onToggleSort,
  viewMode,
  storageKey,
  renderDetailCells: detailCells,
  renderSummaryCells: summaryCells,
  emptyMessage,
}) {
  const [expanded, setExpanded] = useState(() => {
    try {
      const saved = typeof window !== "undefined" ? localStorage.getItem(storageKey) : null;
      if (saved) return new Set(JSON.parse(saved));
    } catch { /* ignore */ }
    return new Set();
  });

  // Save expanded state to localStorage
  const toggleGroup = useCallback((groupKey) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      next.has(groupKey) ? next.delete(groupKey) : next.add(groupKey);
      return next;
    });
  }, []);

  useEffect(() => {
    try {
      localStorage.setItem(storageKey, JSON.stringify([...expanded]));
    } catch (e) {
      console.error(`Failed to save ${storageKey}:`, e);
    }
  }, [expanded, storageKey]);

  const valueColumns = useMemo(() => {
    if (viewMode === "tokens") {
      return [
        { field: "promptTokens", label: "Input Tokens" },
        { field: "completionTokens", label: "Output Tokens" },
        { field: "totalTokens", label: "Total Tokens" },
      ];
    }
    return [
      { field: "promptTokens", label: "Input Cost" },
      { field: "completionTokens", label: "Output Cost" },
      { field: "cost", label: "Total Cost" },
    ];
  }, [viewMode]);

  const totalColSpan = columns.length + valueColumns.length;

  return (
    <Card sx={{ overflow: "hidden" }}>
      <Box sx={{ p: 2, borderBottom: 1, borderColor: "divider", bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(42,42,42,0.5)" : "rgba(244,244,245,0.5)") }}>
        <Typography sx={{ fontWeight: 600 }}>{title}</Typography>
      </Box>
      <TableContainer sx={{ overflowX: "auto" }}>
        <Table sx={{ width: "100%", fontSize: 14, textAlign: "left" }}>
          <TableHead sx={{ bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(42,42,42,0.3)" : "rgba(244,244,245,0.3)") }}>
            <TableRow>
              {columns.map((col) => (
                <TableCell
                  key={col.field}
                  sx={{ ...headCellSx, textAlign: col.align === "right" ? "right" : "left" }}
                  onClick={() => onToggleSort(tableType, col.field)}
                >
                  {col.label}{" "}
                  <SortIcon field={col.field} currentSort={sortBy} currentOrder={sortOrder} />
                </TableCell>
              ))}
              {valueColumns.map((col) => (
                <TableCell
                  key={col.field}
                  sx={{ ...headCellSx, textAlign: "right" }}
                  onClick={() => onToggleSort(tableType, col.field)}
                >
                  {col.label}{" "}
                  <SortIcon field={col.field} currentSort={sortBy} currentOrder={sortOrder} />
                </TableCell>
              ))}
            </TableRow>
          </TableHead>
          <TableBody>
            {groupedData.map((group) => (
              <Fragment key={group.groupKey}>
                {/* Group summary row */}
                <TableRow
                  sx={{ cursor: "pointer", transition: "background-color 150ms ease", ...rowHoverSx }}
                  onClick={() => toggleGroup(group.groupKey)}
                >
                  <TableCell sx={{ px: 3, py: 1.5 }}>
                    <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                      <Icon
                        className="material-symbols-outlined"
                        baseClassName=""
                        sx={{
                          fontSize: 18,
                          color: "text.secondary",
                          transition: "transform 150ms ease",
                          transform: expanded.has(group.groupKey) ? "rotate(90deg)" : "none",
                        }}
                      >
                        chevron_right
                      </Icon>
                      <Box
                        component="span"
                        sx={{ fontWeight: 500, transition: "color 150ms ease", color: group.summary.pending > 0 ? "primary.main" : "inherit" }}
                      >
                        {group.groupKey}
                      </Box>
                    </Box>
                  </TableCell>
                  {summaryCells(group)}
                  <ValueCells item={group.summary} viewMode={viewMode} isSummary />
                </TableRow>
                {/* Detail rows */}
                {expanded.has(group.groupKey) && group.items.map((item) => (
                  <TableRow
                    key={`detail-${item.key}`}
                    sx={{ transition: "background-color 150ms ease", "&:hover": { bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(42,42,42,0.2)" : "rgba(244,244,245,0.2)") } }}
                  >
                    {detailCells(item)}
                    <ValueCells item={item} viewMode={viewMode} />
                  </TableRow>
                ))}
              </Fragment>
            ))}
            {groupedData.length === 0 && (
              <TableRow>
                <TableCell colSpan={totalColSpan} sx={{ px: 3, py: 4, textAlign: "center", color: "text.secondary" }}>
                  {emptyMessage}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </TableContainer>
    </Card>
  );
}

// Re-export utilities for use in UsageStats orchestrator
export { fmt, fmtCost, fmtTime };
