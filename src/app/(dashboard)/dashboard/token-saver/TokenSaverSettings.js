"use client";

import { Card, Toggle } from "@/shared/components";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";

function MsIcon({ children, sx }) {
  return <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 20, ...sx }}>{children}</Icon>;
}

function LevelButtons({ levels, selected, onSelect }) {
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
      {levels.map((lvl) => (
        <Box
          key={lvl.id}
          component="button"
          onClick={() => onSelect(lvl.id)}
          title={lvl.desc}
          sx={{
            px: 1.5,
            py: 0.75,
            borderRadius: 1,
            fontSize: 12,
            fontWeight: 500,
            cursor: "pointer",
            border: 1,
            transition: "background-color 150ms ease, color 150ms ease",
            bgcolor: selected === lvl.id ? "primary.main" : "transparent",
            borderColor: selected === lvl.id ? "primary.main" : "divider",
            color: selected === lvl.id ? "#fff" : "text.secondary",
            "&:hover": { bgcolor: selected === lvl.id ? "primary.main" : "action.hover" },
          }}
        >
          {lvl.label}
        </Box>
      ))}
    </Box>
  );
}

export default function TokenSaverSettings({
  rtkEnabled,
  handleRtkEnabled,
  headroomRunning,
  headroomStatusLabel,
  setShowHeadroomInstallModal,
  headroomEnabled,
  handleHeadroomEnabled,
  headroomStatus,
  headroomExtras,
  pendingExtras,
  codeAware,
  kompress,
  restartingProxy,
  toggleExtraActive,
  handleRemoveExtra,
  removingExtra,
  togglePendingExtra,
  handleInstallExtras,
  extrasActionLoading,
  extrasActionError,
  installLog,
  cavemanEnabled,
  visibleCavemanLevels,
  handleCavemanLevel,
  cavemanLevel,
  cavemanLevels,
  handleCavemanEnabled,
  ponytailEnabled,
  ponytailLevels,
  handlePonytailLevel,
  ponytailLevel,
  handlePonytailEnabled,
  pxpipeChipClass,
  pxpipeStatusLabel,
  setShowPxpipeModal,
  pxpipeStatus,
  pxpipeEnabled,
  handlePxpipeEnabled,
}) {
  const statusChip = (active, label) => (
    <Box component="span" sx={{ fontSize: 12, px: 1, py: 0.25, borderRadius: 1, bgcolor: active ? "success.light" : "warning.light", color: active ? "success.dark" : "warning.dark" }}>{label}</Box>
  );

  return (
    <Card id="rtk">
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", mb: 1 }}>
        <Typography component="h2" sx={{ fontSize: 18, fontWeight: 600, display: "flex", alignItems: "center", gap: 1 }}>
          <MsIcon sx={{ color: "primary.main" }}>bolt</MsIcon>
          Token Saver
        </Typography>
      </Box>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", pt: 1, pb: 2, borderBottom: 1, borderColor: "divider", gap: 2 }}>
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Typography sx={{ fontWeight: 500 }}>
            Compress tool output{" "}
            <Box component="a" href="https://github.com/rtk-ai/rtk" target="_blank" rel="noreferrer" sx={{ fontSize: 12, fontWeight: 400, color: "primary.main", textDecoration: "underline", "&:hover": { opacity: 0.8 } }}>(RTK)</Box>
          </Typography>
          <Typography sx={{ fontSize: 14, color: "text.secondary" }}>git/grep/ls/tree/logs → 60-90% fewer input tokens</Typography>
        </Box>
        <Toggle checked={rtkEnabled} onChange={() => handleRtkEnabled(!rtkEnabled)} />
      </Box>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", py: 2, gap: 2, flexWrap: "wrap" }}>
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
            <Typography sx={{ fontWeight: 500 }}>
              Compress context{" "}
              <Box component="a" href="https://github.com/chopratejas/headroom" target="_blank" rel="noreferrer" sx={{ fontSize: 12, fontWeight: 400, color: "primary.main", textDecoration: "underline", "&:hover": { opacity: 0.8 } }}>(Headroom)</Box>
            </Typography>
            {statusChip(headroomRunning, headroomStatusLabel)}
            <Box component="button" type="button" onClick={() => setShowHeadroomInstallModal(true)} sx={{ fontSize: 12, color: "primary.main", textDecoration: "underline", border: 0, bgcolor: "transparent", cursor: "pointer", "&:hover": { opacity: 0.8 } }}>{headroomRunning ? "Manage" : "Setup"}</Box>
          </Box>
          <Typography sx={{ fontSize: 14, color: "text.secondary", mt: 0.5 }}>Compress prompts via /v1/compress before routing to the model</Typography>
        </Box>
        <Toggle checked={headroomEnabled} onChange={() => handleHeadroomEnabled(!headroomEnabled)} />
      </Box>
      {headroomStatus.installed && (
        <Box sx={{ mb: 1.5, ml: 0.5, pl: 1.5, pb: 2, borderLeft: 2, borderColor: "divider" }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
            <Typography sx={{ fontSize: 12, color: "text.secondary" }}>Compression extras{headroomExtras.version ? ` · v${headroomExtras.version}` : ""}:</Typography>
            {headroomExtras.available.map((extra) => {
              const installed = !!headroomExtras.extras[extra];
              const pending = pendingExtras.includes(extra);
              const extraTitle = extra === "code" ? "tree-sitter AST compression for code responses" : "Kompress-v2 HF model for prose/agentic traces (~+1GB)";
              if (installed) {
                const active = extra === "code" ? codeAware : kompress;
                return (
                  <Box key={extra} sx={{ display: "flex", alignItems: "center", gap: 1, fontSize: 12, px: 1, py: 0.5, borderRadius: 1, border: 1, borderColor: "success.main", bgcolor: "action.hover" }} title={extraTitle}>
                    <Toggle size="sm" checked={active} disabled={restartingProxy} onChange={() => toggleExtraActive(extra, !active)} />
                    <Typography sx={{ fontSize: 12, fontWeight: 500 }}>[{extra}]</Typography>
                    <Box component="button" type="button" onClick={() => handleRemoveExtra(extra)} disabled={removingExtra === extra} title={`Uninstall [${extra}]`} sx={{ ml: 0.5, color: "error.main", textDecoration: "underline", border: 0, bgcolor: "transparent", cursor: "pointer", "&:disabled": { opacity: 0.5 } }}>{removingExtra === extra ? "Uninstalling…" : "Uninstall"}</Box>
                  </Box>
                );
              }
              return (
                <Box
                  key={extra}
                  component="label"
                  title={extraTitle}
                  sx={{ display: "flex", alignItems: "center", gap: 1, fontSize: 12, px: 1, py: 0.5, borderRadius: 1, border: 1, cursor: "pointer", transition: "background-color 150ms ease", borderColor: pending ? "primary.main" : "divider", bgcolor: pending ? "action.selected" : "transparent", color: pending ? "primary.main" : "text.secondary", "&:hover": { bgcolor: "action.hover" } }}
                >
                  <Box component="input" type="checkbox" checked={pending} onChange={() => togglePendingExtra(extra)} sx={{ width: 12, height: 12 }} />
                  <Typography sx={{ fontSize: 12, fontWeight: 500 }}>[{extra}]</Typography>
                  <Box component="span" sx={{ opacity: 0.7 }}>not installed</Box>
                </Box>
              );
            })}
            {pendingExtras.length > 0 && <Box component="button" onClick={handleInstallExtras} disabled={extrasActionLoading} sx={{ fontSize: 12, px: 1.25, py: 0.5, borderRadius: 1, bgcolor: "primary.main", color: "#fff", border: 0, cursor: "pointer", "&:hover": { opacity: 0.9 }, "&:disabled": { opacity: 0.5 } }}>{extrasActionLoading ? "Installing…" : `Install [proxy,${pendingExtras.join(",")}]`}</Box>}
          </Box>
          {extrasActionError && <Typography sx={{ fontSize: 12, color: "error.main", mt: 0.5 }}>{extrasActionError}</Typography>}
          {restartingProxy && <Typography sx={{ fontSize: 12, color: "text.secondary", mt: 0.5 }}>Restarting proxy…</Typography>}
          {(extrasActionLoading || removingExtra) && installLog && <Box component="pre" sx={{ mt: 1, maxHeight: 128, overflow: "auto", borderRadius: 1, bgcolor: "action.hover", p: 1, fontSize: 10, lineHeight: 1.2, color: "text.secondary", whiteSpace: "pre-wrap" }}>{installLog}</Box>}
          <Typography sx={{ fontSize: 12, color: "text.secondary", mt: 0.5 }}>
            Installing adds the package; use <code>on</code>/<code>off</code>{" "}
            to activate it (restarts the proxy). Default install is <code>[proxy]</code> only (SmartCrusher for JSON). Adding <code>[code]</code> enables AST compression (Python/JS/TS/Go/Rust/Java/C/C++/Perl). Adding <code>[ml]</code> enables the Kompress-v2 HF model for prose/agentic traces but adds ~1 GB (torch + huggingface-hub).
          </Typography>
        </Box>
      )}
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", pt: 2, borderTop: 1, borderColor: "divider", gap: 2, flexWrap: "wrap" }}>
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Typography sx={{ fontWeight: 500 }}>Compress LLM output{" "}<Box component="a" href="https://github.com/JuliusBrussee/caveman" target="_blank" rel="noreferrer" sx={{ fontSize: 12, fontWeight: 400, color: "primary.main", textDecoration: "underline", "&:hover": { opacity: 0.8 } }}>(Caveman)</Box></Typography>
          <Typography sx={{ fontSize: 14, color: "text.secondary" }}>Terse-style system prompt → ~65% fewer output tokens (up to 87%)</Typography>
        </Box>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexShrink: 0 }}>
          {cavemanEnabled && <Box sx={{ display: "flex", flexDirection: "column", alignItems: "flex-end", gap: 0.5 }}><LevelButtons levels={visibleCavemanLevels} selected={cavemanLevel} onSelect={handleCavemanLevel} /><Typography sx={{ fontSize: 12, color: "primary.main" }}>{cavemanLevels.find((lvl) => lvl.id === cavemanLevel)?.desc}</Typography></Box>}
          <Toggle checked={cavemanEnabled} onChange={() => handleCavemanEnabled(!cavemanEnabled)} />
        </Box>
      </Box>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", pt: 2, mt: 2, borderTop: 1, borderColor: "divider", gap: 2, flexWrap: "wrap" }}>
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Typography sx={{ fontWeight: 500 }}>Lazy senior dev{" "}<Box component="a" href="https://github.com/DietrichGebert/ponytail" target="_blank" rel="noreferrer" sx={{ fontSize: 12, fontWeight: 400, color: "primary.main", textDecoration: "underline", "&:hover": { opacity: 0.8 } }}>(Ponytail)</Box></Typography>
          <Typography sx={{ fontSize: 14, color: "text.secondary" }}>Bias the model toward minimal code: YAGNI, reuse stdlib, deletion over addition</Typography>
        </Box>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexShrink: 0 }}>
          {ponytailEnabled && <Box sx={{ display: "flex", flexDirection: "column", alignItems: "flex-end", gap: 0.5 }}><LevelButtons levels={ponytailLevels} selected={ponytailLevel} onSelect={handlePonytailLevel} /><Typography sx={{ fontSize: 12, color: "primary.main" }}>{ponytailLevels.find((lvl) => lvl.id === ponytailLevel)?.desc}</Typography></Box>}
          <Toggle checked={ponytailEnabled} onChange={() => handlePonytailEnabled(!ponytailEnabled)} />
        </Box>
      </Box>
      {/* PXPIPE integration card */}
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", pt: 2, mt: 2, borderTop: 1, borderColor: "divider", gap: 2, flexWrap: "wrap" }}>
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
            <Typography sx={{ fontWeight: 500 }}>Compress prompts as images{" "}<Box component="a" href="https://github.com/teamchong/pxpipe" target="_blank" rel="noreferrer" sx={{ fontSize: 12, fontWeight: 400, color: "primary.main", textDecoration: "underline", "&:hover": { opacity: 0.8 } }}>(PXPIPE)</Box></Typography>
            <Box component="span" sx={{ fontSize: 12, px: 1, py: 0.25, borderRadius: 1, bgcolor: pxpipeChipClass.includes("success") ? "success.light" : "warning.light", color: pxpipeChipClass.includes("success") ? "success.dark" : "warning.dark" }}>{pxpipeStatusLabel}</Box>
            <Box component="button" type="button" onClick={() => setShowPxpipeModal(true)} sx={{ fontSize: 12, color: "primary.main", textDecoration: "underline", border: 0, bgcolor: "transparent", cursor: "pointer", "&:hover": { opacity: 0.8 } }}>{pxpipeStatus.installed ? "Manage" : "Setup"}</Box>
            <Box component="a" href="/dashboard/pxpipe" sx={{ fontSize: 12, color: "primary.main", textDecoration: "underline", "&:hover": { opacity: 0.8 } }}>Dashboard</Box>
          </Box>
          <Typography sx={{ fontSize: 14, color: "text.secondary", mt: 0.5 }}>Transforms large textual context into optimized images before sending to the LLM. Ideal for huge prompts, tool outputs and long conversations.</Typography>
        </Box>
        <Toggle checked={pxpipeEnabled} disabled={!pxpipeStatus.installed} onChange={() => handlePxpipeEnabled(!pxpipeEnabled)} />
      </Box>
    </Card>
  );
}
