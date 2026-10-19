import Box from "@mui/material/Box";
import Icon from "@mui/material/Icon";
import Checkbox from "@mui/material/Checkbox";
import { CapacityBadges } from "@/shared/components";

const tooltipSx = {
  pointerEvents: "none",
  position: "absolute",
  mt: 0.5,
  top: 20,
  left: "50%",
  transform: "translateX(-50%)",
  fontSize: 10,
  color: "text.secondary",
  whiteSpace: "nowrap",
  opacity: 0,
  transition: "opacity 150ms ease",
};

export default function ModelRow({ model, fullModel, alias, copied, onCopy, testStatus, isCustom, isFree, onDeleteAlias, onTest, isTesting, onDisable, caps, selected, onToggleSelect }) {
  const borderColor = testStatus === "ok"
    ? "rgba(34,197,94,0.4)"
    : testStatus === "error"
    ? "rgba(239,68,68,0.4)"
    : "divider";

  const iconColor = testStatus === "ok"
    ? "#22c55e"
    : testStatus === "error"
    ? "#ef4444"
    : undefined;

  return (
    <Box
      sx={{
        minWidth: 0,
        maxWidth: "100%",
        borderRadius: "8px",
        border: 1,
        borderColor,
        px: 1.5,
        py: 1,
        transition: "background-color 150ms ease",
        "&:hover": { bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(30,30,30,0.5)" : "rgba(244,241,236,0.5)") },
        "&:hover .mr-hover": { opacity: 1 },
        ...(selected ? { borderColor: "rgba(229,106,74,0.4)", bgcolor: "rgba(229,106,74,0.03)" } : {}),
      }}
    >
      <Box sx={{ display: "flex", minWidth: 0, alignItems: { xs: "flex-start", sm: "center" }, gap: 1 }}>
        {onToggleSelect && (
          <Checkbox
            checked={!!selected}
            onChange={onToggleSelect}
            size="small"
            inputProps={{ "aria-label": `Select ${model.id}` }}
            sx={{ p: 0, flexShrink: 0 }}
          />
        )}
        <Icon
          className="material-symbols-outlined"
          baseClassName=""
          sx={{ fontSize: 16, flexShrink: 0, color: iconColor }}
        >
          {testStatus === "ok" ? "check_circle" : testStatus === "error" ? "cancel" : "smart_toy"}
        </Icon>
        <Box sx={{ display: "flex", minWidth: 0, flex: 1, flexDirection: "column", gap: 0.5 }}>
          <Box component="code" sx={{ maxWidth: { xs: "72vw", sm: 360 }, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", borderRadius: "4px", bgcolor: "var(--color-sidebar)", px: 0.75, py: 0.25, fontFamily: "monospace", fontSize: 12, color: "text.secondary" }}>{fullModel}</Box>
          <Box component="span" sx={{ display: "flex", minWidth: 0, alignItems: "center", fontSize: 9, gap: 0.5, pl: 0.5 }}>
            {model.name && <Box component="span" sx={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontSize: 9, fontStyle: "italic", color: (t) => (t.palette.mode === "dark" ? "rgba(156,163,175,0.7)" : "rgba(107,114,128,0.7)") }}>{model.name}</Box>}
            <CapacityBadges caps={caps} colorOverride="text-text-muted/70" size={12} />
          </Box>
        </Box>
        {onTest && (
          <Box sx={{ position: "relative", flexShrink: 0, "&:hover .mr-tooltip": { opacity: 1 } }}>
            <Box
              component="button"
              onClick={onTest}
              disabled={isTesting}
              className={isTesting ? undefined : "mr-hover"}
              sx={{
                borderRadius: "4px",
                p: 0.25,
                border: 0,
                bgcolor: "transparent",
                color: "text.secondary",
                cursor: "pointer",
                opacity: isTesting ? 1 : { xs: 1, sm: 0 },
                transition: "opacity 150ms ease, background-color 150ms ease, color 150ms ease",
                "&:hover": { bgcolor: "var(--color-sidebar)", color: "primary.main" },
              }}
            >
              <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 14, animation: isTesting ? "spin 1s linear infinite" : undefined }}>
                {isTesting ? "progress_activity" : "science"}
              </Icon>
            </Box>
            <Box component="span" className="mr-tooltip" sx={tooltipSx}>
              {isTesting ? "Testing..." : "Test"}
            </Box>
          </Box>
        )}
        <Box sx={{ position: "relative", flexShrink: 0, "&:hover .mr-tooltip": { opacity: 1 } }}>
          <Box
            component="button"
            onClick={() => onCopy(fullModel, `model-${model.id}`)}
            sx={{
              borderRadius: "4px",
              p: 0.25,
              border: 0,
              bgcolor: "transparent",
              color: "text.secondary",
              cursor: "pointer",
              transition: "background-color 150ms ease, color 150ms ease",
              "&:hover": { bgcolor: "var(--color-sidebar)", color: "primary.main" },
            }}
          >
            <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 14 }}>
              {copied === `model-${model.id}` ? "check" : "content_copy"}
            </Icon>
          </Box>
          <Box component="span" className="mr-tooltip" sx={tooltipSx}>
            {copied === `model-${model.id}` ? "Copied!" : "Copy"}
          </Box>
        </Box>
        {isCustom ? (
          <Box
            component="button"
            onClick={onDeleteAlias}
            className="mr-hover"
            sx={{
              ml: "auto",
              borderRadius: "4px",
              p: 0.25,
              border: 0,
              bgcolor: "transparent",
              color: "text.secondary",
              cursor: "pointer",
              opacity: { xs: 1, sm: 0 },
              transition: "opacity 150ms ease, background-color 150ms ease, color 150ms ease",
              "&:hover": { bgcolor: "rgba(239,68,68,0.1)", color: "#ef4444" },
            }}
            title="Remove custom model"
          >
            <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 14 }}>close</Icon>
          </Box>
        ) : onDisable ? (
          <Box
            component="button"
            onClick={onDisable}
            className="mr-hover"
            sx={{
              ml: "auto",
              borderRadius: "4px",
              p: 0.25,
              border: 0,
              bgcolor: "transparent",
              color: "text.secondary",
              cursor: "pointer",
              opacity: { xs: 1, sm: 0 },
              transition: "opacity 150ms ease, background-color 150ms ease, color 150ms ease",
              "&:hover": { bgcolor: "rgba(239,68,68,0.1)", color: "#ef4444" },
            }}
            title="Disable this model"
          >
            <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 14 }}>close</Icon>
          </Box>
        ) : null}
      </Box>
    </Box>
  );
}
