"use client";

import { useState, useEffect } from "react";
import { Card, Button, Input, Modal, Toggle, ConfirmModal } from "@/shared/components";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";

function MsIcon({ children, sx, ...props }) {
  return <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 20, ...sx }} {...props}>{children}</Icon>;
}
import { useCopyToClipboard } from "@/shared/hooks/useCopyToClipboard";
import { getCurrentLocale, onLocaleChange } from "@/i18n/runtime";
import { useHeadroom } from "./useHeadroom";
import { usePxpipe } from "./usePxpipe";
import TokenSaverSettings from "./TokenSaverSettings";
import {
  WENYAN_LOCALES,
  CAVEMAN_LEVELS,
  PONYTAIL_LEVELS,
} from "../endpoint/endpointConstants";

export default function TokenSaverClient() {
  const [rtkEnabled, setRtkEnabledState] = useState(true);
  const [headroomEnabled, setHeadroomEnabled] = useState(false);
  const [headroomUrl, setHeadroomUrl] = useState("http://localhost:8787");
  const [headroomTimeoutMs, setHeadroomTimeoutMs] = useState(3000);
  const [cavemanEnabled, setCavemanEnabled] = useState(false);
  const [cavemanLevel, setCavemanLevel] = useState("full");
  const [ponytailEnabled, setPonytailEnabled] = useState(false);
  const [ponytailLevel, setPonytailLevel] = useState("full");
  const [codeAware, setCodeAware] = useState(false);
  const [kompress, setKompress] = useState(true);
  const [pxpipeEnabled, setPxpipeEnabled] = useState(false);
  const [guards, setGuards] = useState({
    loopGuard: true,
    circuitBreaker: true,
    semaphore: true,
  });
  const [locale, setLocale] = useState(getCurrentLocale);

  const { copied, copy } = useCopyToClipboard();

  const patchSetting = async (patch) => {
    try {
      await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(patch),
      });
    } catch (error) {
      console.log("Error updating setting:", error);
    }
  };

  const headroom = useHeadroom({
    patchSetting,
    setHeadroomEnabled,
    headroomUrl,
    setHeadroomUrl,
    codeAware,
    kompress,
    setCodeAware,
    setKompress,
  });
  const pxpipe = usePxpipe({ patchSetting, setPxpipeEnabled });
  const {
    headroomStatus,
    showInstallModal: showHeadroomInstallModal,
    setShowInstallModal: setShowHeadroomInstallModal,
    actionLoading: headroomActionLoading,
    actionError: headroomActionError,
    headroomExtras,
    pendingExtras,
    extrasActionLoading,
    extrasActionError,
    removingExtra,
    installLog,
    extrasConfirm,
    setExtrasConfirm,
    restartingProxy,
    refresh: refreshHeadroomStatus,
    start: handleHeadroomStart,
    stop: handleHeadroomStop,
    toggleExtraActive,
    installExtras: installExtrasConfirmed,
    removeExtra: removeExtraConfirmed,
    toggleEnabled: handleHeadroomEnabled,
    blurUrl: handleHeadroomUrlBlur,
    setPendingExtras,
  } = headroom;
  const {
    pxpipeStatus,
    pxpipeHealth,
    showModal: showPxpipeModal,
    setShowModal: setShowPxpipeModal,
    actionLoading: pxpipeActionLoading,
    actionError: pxpipeActionError,
    minChars: pxpipeMinChars,
    setMinChars: setPxpipeMinChars,
    refresh: refreshPxpipeStatus,
    health: runPxpipeHealth,
    action: pxpipeAction,
    setEnabled: handlePxpipeEnabled,
    blurMinChars: handlePxpipeMinCharsBlur,
  } = pxpipe;

  useEffect(() => onLocaleChange(() => setLocale(getCurrentLocale())), []);

  const isWenyanLocale = WENYAN_LOCALES.includes(locale);
  const visibleCavemanLevels = isWenyanLocale
    ? CAVEMAN_LEVELS
    : CAVEMAN_LEVELS.filter((lvl) => !lvl.wenyan);
  const effectiveCavemanLevel = !isWenyanLocale && CAVEMAN_LEVELS.find((lvl) => lvl.id === cavemanLevel)?.wenyan
    ? "ultra"
    : cavemanLevel;

  useEffect(() => {
    if (effectiveCavemanLevel !== cavemanLevel) patchSetting({ cavemanLevel: effectiveCavemanLevel });
  }, [cavemanLevel, effectiveCavemanLevel]);

  const handleRtkEnabled = async (value) => {
    try {
      const res = await fetch("/api/settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ rtkEnabled: value }),
      });
      if (res.ok) setRtkEnabledState(value);
    } catch (error) {
      console.log("Error updating rtkEnabled:", error);
    }
  };

  const handleCavemanEnabled = (value) => {
    setCavemanEnabled(value);
    patchSetting({ cavemanEnabled: value });
  };

  const handleCavemanLevel = (level) => {
    setCavemanLevel(level);
    patchSetting({ cavemanLevel: level });
  };

  const handlePonytailEnabled = (value) => {
    setPonytailEnabled(value);
    patchSetting({ ponytailEnabled: value });
  };

  const handlePonytailLevel = (level) => {
    setPonytailLevel(level);
    patchSetting({ ponytailLevel: level });
  };

  const togglePendingExtra = (extra) => {
    setPendingExtras((current) =>
      current.includes(extra)
        ? current.filter((item) => item !== extra)
        : [...current, extra]
    );
  };

  const handleInstallExtras = () => {
    if (pendingExtras.length === 0) return;
    if (pendingExtras.includes("ml")) {
      setExtrasConfirm({
        title: "Install [ml]",
        message: "[ml] downloads ~1 GB (torch + huggingface-hub). Continue?",
        confirmText: "Install",
        variant: "primary",
        onConfirm: installExtrasConfirmed,
      });
      return;
    }
    installExtrasConfirmed();
  };

  const handleRemoveExtra = (extra) => {
    setExtrasConfirm({
      title: `Remove [${extra}]`,
      message: `Remove [${extra}] and its packages?`,
      confirmText: "Remove",
      variant: "danger",
      onConfirm: () => removeExtraConfirmed(extra),
    });
  };

  const updateGuard = (key, value) => {
    setGuards((prev) => ({ ...prev, [key]: value }));
    patchSetting({ [`${key}Enabled`]: value });
  };

  const handleHeadroomTimeoutBlur = () => {
    const raw = Math.round(Number(headroomTimeoutMs));
    const next = Number.isFinite(raw) && raw > 0 ? raw : 3000;
    setHeadroomTimeoutMs(next);
    patchSetting({ headroomTimeoutMs: next });
  };

  useEffect(() => {
    const loadSettings = async () => {
      try {
        const res = await fetch("/api/settings");
        if (res.ok) {
          const data = await res.json();
          setRtkEnabledState(data.rtkEnabled !== false);
          setHeadroomEnabled(!!data.headroomEnabled);
          setHeadroomUrl(data.headroomUrl || "http://localhost:8787");
          if (typeof data.headroomTimeoutMs === "number") setHeadroomTimeoutMs(data.headroomTimeoutMs);
          setCodeAware(data.headroomCodeAware === true);
          setKompress(data.headroomKompress !== false);
          setCavemanEnabled(!!data.cavemanEnabled);
          setCavemanLevel(data.cavemanLevel || "full");
          setPonytailEnabled(!!data.ponytailEnabled);
          setPonytailLevel(data.ponytailLevel || "full");
          setPxpipeEnabled(!!data.pxpipeEnabled);
          if (typeof data.pxpipeMinChars === "number") setPxpipeMinChars(data.pxpipeMinChars);
          setGuards({
            loopGuard: data.loopGuardEnabled !== false,
            circuitBreaker: data.circuitBreakerEnabled !== false,
            semaphore: data.semaphoreEnabled !== false,
          });
          refreshHeadroomStatus();
          refreshPxpipeStatus().then(runPxpipeHealth);
        }
      } catch {}
    };
    loadSettings();
  }, [refreshHeadroomStatus, refreshPxpipeStatus, runPxpipeHealth, setPxpipeMinChars]);

  const headroomRunning = !!headroomStatus.running;
  const headroomStatusLabel = headroomStatus.loading
    ? "Checking…"
    : headroomRunning
      ? "Running"
      : headroomStatus.localUrl !== false && !headroomStatus.installed
        ? "Not installed"
        : headroomStatus.localUrl !== false
          ? "Stopped"
          : "External";
  const headroomLocalUrl = headroomStatus.localUrl !== false;
  const headroomCanStart = !!headroomStatus.canStart;
  const headroomManaged =
    headroomLocalUrl && !!headroomStatus.managedPid;

  const pxpipeHealthy = pxpipeHealth?.healthy === true;
  const pxpipeStatusLabel = pxpipeStatus.loading
    ? "Checking…"
    : pxpipeStatus.installing
      ? "Installing…"
      : !pxpipeStatus.installed
        ? "Not installed"
        : pxpipeHealthy
          ? "Healthy"
          : pxpipeStatus.running
            ? "Running"
            : "Stopped";
  const pxpipeChipClass =
    pxpipeHealthy || pxpipeStatus.running
      ? "bg-success/15 text-success"
      : "bg-warning/15 text-warning";

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3, p: 3 }}>
      <TokenSaverSettings
        rtkEnabled={rtkEnabled}
        handleRtkEnabled={handleRtkEnabled}
        headroomRunning={headroomRunning}
        headroomStatusLabel={headroomStatusLabel}
        setShowHeadroomInstallModal={setShowHeadroomInstallModal}
        headroomEnabled={headroomEnabled}
        handleHeadroomEnabled={handleHeadroomEnabled}
        headroomStatus={headroomStatus}
        headroomExtras={headroomExtras}
        pendingExtras={pendingExtras}
        codeAware={codeAware}
        kompress={kompress}
        restartingProxy={restartingProxy}
        toggleExtraActive={toggleExtraActive}
        handleRemoveExtra={handleRemoveExtra}
        removingExtra={removingExtra}
        togglePendingExtra={togglePendingExtra}
        handleInstallExtras={handleInstallExtras}
        extrasActionLoading={extrasActionLoading}
        extrasActionError={extrasActionError}
        installLog={installLog}
        cavemanEnabled={cavemanEnabled}
        visibleCavemanLevels={visibleCavemanLevels}
        handleCavemanLevel={handleCavemanLevel}
        cavemanLevel={effectiveCavemanLevel}
        cavemanLevels={CAVEMAN_LEVELS}
        handleCavemanEnabled={handleCavemanEnabled}
        ponytailEnabled={ponytailEnabled}
        ponytailLevels={PONYTAIL_LEVELS}
        handlePonytailLevel={handlePonytailLevel}
        ponytailLevel={ponytailLevel}
        handlePonytailEnabled={handlePonytailEnabled}
        pxpipeChipClass={pxpipeChipClass}
        pxpipeStatusLabel={pxpipeStatusLabel}
        setShowPxpipeModal={setShowPxpipeModal}
        pxpipeStatus={pxpipeStatus}
        pxpipeEnabled={pxpipeEnabled}
        handlePxpipeEnabled={handlePxpipeEnabled}
      />

      <Card id="guards">
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", mb: 1 }}>
          <Typography component="h2" sx={{ fontSize: 18, fontWeight: 600, display: "flex", alignItems: "center", gap: 1 }}>
            <MsIcon sx={{ color: "primary.main" }}>shield</MsIcon>
            Guards & Shields
          </Typography>
        </Box>

        {/* Loop Guard */}
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", pt: 1, pb: 2, borderBottom: 1, borderColor: "divider", gap: 2 }}>
          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Typography sx={{ fontWeight: 500 }}>Loop Guard</Typography>
            <Typography sx={{ fontSize: 14, color: "text.secondary" }}>
              Detects repeating tool call sequences and text-only planning loops.
              Injects corrections to prevent infinite agent run-loops.
            </Typography>
          </Box>
          <Toggle checked={guards.loopGuard} onChange={() => updateGuard("loopGuard", !guards.loopGuard)} />
        </Box>

        {/* Circuit Breaker */}
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", py: 2, borderBottom: 1, borderColor: "divider", gap: 2, flexWrap: "wrap" }}>
          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Typography sx={{ fontWeight: 500 }}>Circuit Breaker</Typography>
            <Typography sx={{ fontSize: 14, color: "text.secondary" }}>
              Temporarily halts upstream calls for offline or failing providers
              to prevent spamming and conserve connection resource.
            </Typography>
          </Box>
          <Toggle checked={guards.circuitBreaker} onChange={() => updateGuard("circuitBreaker", !guards.circuitBreaker)} />
        </Box>

        {/* Semaphore Limiter */}
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", py: 2, gap: 2, flexWrap: "wrap" }}>
          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Typography sx={{ fontWeight: 500 }}>Semaphore (Concurrency Limiter)</Typography>
            <Typography sx={{ fontSize: 14, color: "text.secondary" }}>
              Enforces simultaneous request limits per provider account. Prevents
              rate-limits (429) and account blockings.
            </Typography>
          </Box>
          <Toggle checked={guards.semaphore} onChange={() => updateGuard("semaphore", !guards.semaphore)} />
        </Box>
      </Card>

      <Modal
        isOpen={showHeadroomInstallModal}
        title={headroomRunning ? "Headroom" : "Setup Headroom"}
        onClose={() => setShowHeadroomInstallModal(false)}
      >
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", fontSize: 14 }}>
            <Box component="span">Status</Box>
            <Box component="span" sx={{ color: headroomRunning ? "success.main" : "warning.main" }}>{headroomStatusLabel}</Box>
          </Box>
          {headroomRunning && (
            <Box component="a" href="/api/headroom/proxy/dashboard" target="_blank" rel="noreferrer" sx={{ width: "100%", borderRadius: 1, border: 1, borderColor: "divider", px: 2, py: 1, textAlign: "center", fontSize: 14, color: "text.primary", "&:hover": { bgcolor: "action.hover" } }}>
              Open Headroom Dashboard
            </Box>
          )}
          <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
            <Typography sx={{ fontSize: 14, fontWeight: 500 }}>Proxy URL</Typography>
            <Input
              value={headroomUrl}
              onChange={(e) => setHeadroomUrl(e.target.value)}
              onBlur={handleHeadroomUrlBlur}
              placeholder="http://localhost:8787"
              inputClassName="font-mono text-sm"
            />
            <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
              Use a local proxy for Start/Stop, or an external Docker sidecar
              like http://headroom:8787.
            </Typography>
          </Box>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
            <Typography sx={{ fontSize: 14, fontWeight: 500 }}>Timeout (ms)</Typography>
            <Input
              value={String(headroomTimeoutMs)}
              onChange={(e) => setHeadroomTimeoutMs(e.target.value)}
              onBlur={handleHeadroomTimeoutBlur}
              placeholder="3000"
              inputClassName="font-mono text-sm"
            />
            <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
              Request timeout in milliseconds. Defaults to 3000 ms.
            </Typography>
          </Box>
          {headroomManaged ? (
            <Button
              onClick={handleHeadroomStop}
              variant="ghost"
              fullWidth
              disabled={headroomActionLoading}
            >
              {headroomActionLoading ? "Stopping…" : "Stop Headroom"}
            </Button>
          ) : headroomRunning ? (
            <Typography sx={{ fontSize: 14, color: "success.main" }}>
              Headroom proxy is reachable. You can enable the token saver.
            </Typography>
          ) : headroomCanStart ? (
            <Button
              onClick={handleHeadroomStart}
              fullWidth
              disabled={headroomActionLoading}
            >
              {headroomActionLoading ? "Starting…" : "Start Headroom"}
            </Button>
          ) : !headroomLocalUrl ? (
            <Typography sx={{ fontSize: 14, color: "warning.main" }}>
              Start Headroom separately at the configured URL, then recheck.
            </Typography>
          ) : !headroomStatus.python ? (
            <Typography sx={{ fontSize: 14, color: "warning.main" }}>
              Python ≥ 3.10 required for local managed mode. Install Python
              first, or use an external proxy URL.
            </Typography>
          ) : (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
              <Typography sx={{ fontSize: 14, fontWeight: 500 }}>Install then click Start:</Typography>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Box component="pre" sx={{ flex: 1, borderRadius: 1, bgcolor: "action.hover", p: 1, fontSize: 12, fontFamily: "monospace", overflowX: "auto" }}>
                  {`pip install "headroom-ai[proxy]"`}
                </Box>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() =>
                    copy(`pip install "headroom-ai[proxy]"`)
                  }
                >
                  {copied ? "Copied" : "Copy"}
                </Button>
              </Box>
            </Box>
          )}
          {headroomActionError && (
            <Typography sx={{ fontSize: 14, color: "warning.main" }}>{headroomActionError}</Typography>
          )}
          <Box sx={{ display: "flex", gap: 1 }}>
            <Button
              onClick={() => refreshHeadroomStatus()}
              variant="ghost"
              fullWidth
            >
              Recheck
            </Button>
            <Button
              onClick={() => setShowHeadroomInstallModal(false)}
              fullWidth
            >
              Done
            </Button>
          </Box>
        </Box>
      </Modal>

      <Modal
        isOpen={showPxpipeModal}
        title={pxpipeStatus.installed ? "PXPIPE" : "Setup PXPIPE"}
        onClose={() => setShowPxpipeModal(false)}
      >
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Typography sx={{ fontSize: 14, color: "text.secondary" }}>
            Compress prompts using multimodal encoding. Runs in-process — no
            extra server or environment variables required.
          </Typography>
          <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", fontSize: 14 }}>
            <Box component="span">Status</Box>
            <Box component="span" sx={{ color: pxpipeHealthy || pxpipeStatus.running ? "success.main" : "warning.main" }}>
              {pxpipeStatusLabel}
              {pxpipeStatus.version ? ` · v${pxpipeStatus.version}` : ""}
            </Box>
          </Box>
          {pxpipeHealth?.checks?.length > 0 && (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5, border: 1, borderColor: "divider", borderRadius: 1, p: 1.5 }}>
              <Typography sx={{ fontSize: 14, fontWeight: 500, mb: 0.5 }}>Health check</Typography>
              {pxpipeHealth.checks.map((check) => (
                <Box key={check.id} sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", fontSize: 12 }}>
                  <Box component="span" sx={{ color: check.ok ? "success.main" : "warning.main" }}>
                    {check.ok ? "●" : "○"} {check.label}
                  </Box>
                  {check.detail && (
                    <Box component="span" sx={{ color: "text.secondary", fontFamily: "monospace", overflow: "hidden", textOverflow: "ellipsis", maxWidth: "50%" }}>{check.detail}</Box>
                  )}
                </Box>
              ))}
              {pxpipeHealth.error && <Typography sx={{ fontSize: 12, color: "warning.main", mt: 0.5 }}>{pxpipeHealth.error}</Typography>}
            </Box>
          )}
          {!pxpipeStatus.installed ? (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
              <Typography sx={{ fontSize: 14, color: "warning.main" }}>PXPIPE is not installed.</Typography>
              <Button
                onClick={() => pxpipeAction("install")}
                fullWidth
                disabled={pxpipeActionLoading || pxpipeStatus.installing}
              >
                {pxpipeActionLoading || pxpipeStatus.installing ? "Installing…" : "Install"}
              </Button>
              <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
                Installs the npm package <Box component="code" sx={{ fontFamily: "monospace" }}>pxpipe-proxy</Box> into
                the 9Router data directory. May take a few minutes.
              </Typography>
            </Box>
          ) : (
            <Box sx={{ display: "grid", gridTemplateColumns: "repeat(2, 1fr)", gap: 1 }}>
              {pxpipeStatus.running ? (
                <>
                  <Button onClick={() => pxpipeAction("restart")} variant="ghost" disabled={pxpipeActionLoading}>
                    Restart
                  </Button>
                  <Button onClick={() => pxpipeAction("stop")} variant="ghost" disabled={pxpipeActionLoading}>
                    Stop
                  </Button>
                </>
              ) : (
                <Button onClick={() => pxpipeAction("start")} disabled={pxpipeActionLoading}>
                  {pxpipeActionLoading ? "Starting…" : "Start"}
                </Button>
              )}
              <Button onClick={() => pxpipeAction("install")} variant="ghost" disabled={pxpipeActionLoading}>
                Repair
              </Button>
              <Box component="a" href="/dashboard/pxpipe#logs" sx={{ gridColumn: "span 2", borderRadius: 1, border: 1, borderColor: "divider", px: 2, py: 1, textAlign: "center", fontSize: 14, color: "text.primary", "&:hover": { bgcolor: "action.hover" } }}>
                Open Logs
              </Box>
            </Box>
          )}
          <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
            <Typography sx={{ fontSize: 14, fontWeight: 500 }}>Minimum prompt size (chars)</Typography>
            <Input
              value={String(pxpipeMinChars)}
              onChange={(e) => setPxpipeMinChars(e.target.value)}
              onBlur={handlePxpipeMinCharsBlur}
              placeholder="25000"
              inputClassName="font-mono text-sm"
            />
            <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
              Requests smaller than this bypass PXPIPE and are sent as-is.
            </Typography>
          </Box>
          {pxpipeActionError && <Typography sx={{ fontSize: 14, color: "warning.main" }}>{pxpipeActionError}</Typography>}
          <Box sx={{ display: "flex", gap: 1 }}>
            <Button
              onClick={() => refreshPxpipeStatus().then(runPxpipeHealth)}
              variant="ghost"
              fullWidth
            >
              Recheck
            </Button>
            <Button onClick={() => setShowPxpipeModal(false)} fullWidth>
              Done
            </Button>
          </Box>
        </Box>
      </Modal>

      <ConfirmModal
        isOpen={!!extrasConfirm}
        onClose={() => setExtrasConfirm(null)}
        onConfirm={() => {
          const fn = extrasConfirm?.onConfirm;
          setExtrasConfirm(null);
          fn?.();
        }}
        title={extrasConfirm?.title}
        message={extrasConfirm?.message}
        confirmText={extrasConfirm?.confirmText}
        variant={extrasConfirm?.variant}
      />
    </Box>
  );
}
