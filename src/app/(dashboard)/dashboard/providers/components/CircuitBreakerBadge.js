/**
 * CircuitBreakerBadge — compact badge showing circuit breaker state for a provider.
 * Integrates into existing provider cards without requiring a new page.
 */
import Box from "@mui/material/Box";
import Icon from "@mui/material/Icon";

const stateConfig = {
  DEGRADED: {
    bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(120,53,15,0.3)" : "#fef3c7"),
    color: (t) => (t.palette.mode === "dark" ? "#fbbf24" : "#b45309"),
    icon: "warning",
    label: "Degraded",
  },
  OPEN: {
    bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(127,29,29,0.3)" : "#fee2e2"),
    color: (t) => (t.palette.mode === "dark" ? "#f87171" : "#b91c1c"),
    icon: "power",
    label: "Circuit Open",
  },
  HALF_OPEN: {
    bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(30,58,138,0.3)" : "#dbeafe"),
    color: (t) => (t.palette.mode === "dark" ? "#60a5fa" : "#1d4ed8"),
    icon: "autorenew",
    label: "Recovering",
  },
};

export default function CircuitBreakerBadge({ status, onReset }) {
  if (!status || status.state === "CLOSED") return null;

  const config = stateConfig[status.state] || stateConfig.DEGRADED;
  const retryIn = status.retryAfterMs > 0 ? ` (${Math.ceil(status.retryAfterMs / 1000)}s)` : "";

  return (
    <Box
      component="span"
      sx={{
        display: "inline-flex",
        alignItems: "center",
        gap: 0.5,
        borderRadius: "9999px",
        px: 1,
        py: 0.25,
        fontSize: 10,
        fontWeight: 500,
        bgcolor: config.bgcolor,
        color: config.color,
      }}
    >
      <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 12 }}>{config.icon}</Icon>
      {config.label}{retryIn}
      {status.state === "OPEN" && onReset && (
        <Box
          component="button"
          type="button"
          onClick={(e) => { e.preventDefault(); e.stopPropagation(); onReset(); }}
          sx={{
            ml: 0.25,
            p: 0,
            border: 0,
            bgcolor: "transparent",
            color: "inherit",
            cursor: "pointer",
            display: "inline-flex",
            transition: "opacity 150ms ease",
            "&:hover": { opacity: 0.7 },
          }}
          title="Reset circuit breaker"
        >
          <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 12 }}>refresh</Icon>
        </Box>
      )}
    </Box>
  );
}
