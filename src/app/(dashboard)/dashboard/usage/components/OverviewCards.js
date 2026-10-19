"use client";

import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Card from "@/shared/components/Card";

const fmt = (n) => new Intl.NumberFormat().format(n || 0);
const fmtCost = (n) => `$${(n || 0).toFixed(2)}`;

const cardSx = {
  display: "flex",
  minWidth: 0,
  flexDirection: "column",
  gap: 0.5,
  px: 2,
  py: 1.5,
};

const labelSx = {
  color: "text.secondary",
  fontSize: 14,
  textTransform: "uppercase",
  fontWeight: 600,
};

const valueSx = {
  fontSize: "1.5rem",
  fontWeight: 700,
  overflow: "hidden",
  textOverflow: "ellipsis",
  whiteSpace: "nowrap",
};

const hintSx = { fontSize: 10, color: "text.secondary" };

export default function OverviewCards({ stats }) {
  return (
    <Box
      sx={{
        display: "grid",
        minWidth: 0,
        gridTemplateColumns: {
          xs: "repeat(1, minmax(0, 1fr))",
          sm: "repeat(2, minmax(0, 1fr))",
          md: "repeat(3, minmax(0, 1fr))",
          lg: "repeat(5, minmax(0, 1fr))",
        },
        gap: { xs: 1.5, sm: 2 },
      }}
    >
      <Card sx={cardSx}>
        <Typography sx={labelSx}>Total Requests</Typography>
        <Typography sx={valueSx}>{fmt(stats.totalRequests)}</Typography>
      </Card>
      <Card sx={cardSx}>
        <Typography sx={labelSx}>Total Input Tokens</Typography>
        <Typography sx={{ ...valueSx, color: "primary.main" }}>{fmt(stats.totalPromptTokens)}</Typography>
      </Card>
      <Card sx={cardSx}>
        <Typography sx={labelSx}>Cached Tokens</Typography>
        <Typography sx={{ ...valueSx, color: "info.main" }}>{fmt(stats.totalCachedTokens)}</Typography>
        <Typography sx={hintSx}>
          {stats.totalPromptTokens > 0 ? `${((stats.totalCachedTokens / stats.totalPromptTokens) * 100).toFixed(1)}% hit rate` : "0.0% hit rate"}
        </Typography>
      </Card>
      <Card sx={cardSx}>
        <Typography sx={labelSx}>Output Tokens</Typography>
        <Typography sx={{ ...valueSx, color: "success.main" }}>{fmt(stats.totalCompletionTokens)}</Typography>
      </Card>
      <Card sx={cardSx}>
        <Typography sx={labelSx}>Est. Cost</Typography>
        <Typography sx={{ ...valueSx, color: "warning.main" }}>~{fmtCost(stats.totalCost)}</Typography>
        <Typography sx={hintSx}>Estimated, not actual billing</Typography>
      </Card>
    </Box>
  );
}
