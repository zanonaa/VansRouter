import { useCallback, useEffect, useRef, useState } from "react";

const DEFAULT_EXTRAS = { version: null, extras: { code: false, ml: false }, available: ["code", "ml"], loading: false };

export function useHeadroom({ patchSetting, setHeadroomEnabled, headroomUrl, setHeadroomUrl, codeAware, kompress, setCodeAware, setKompress }) {
  const [headroomStatus, setHeadroomStatus] = useState({ installed: false, running: false, python: null, loading: true });
  const [showInstallModal, setShowInstallModal] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [actionError, setActionError] = useState("");
  const [headroomExtras, setHeadroomExtras] = useState(DEFAULT_EXTRAS);
  const [pendingExtras, setPendingExtras] = useState([]);
  const [extrasActionLoading, setExtrasActionLoading] = useState(false);
  const [extrasActionError, setExtrasActionError] = useState("");
  const [removingExtra, setRemovingExtra] = useState(null);
  const [installLog, setInstallLog] = useState("");
  const [extrasConfirm, setExtrasConfirm] = useState(null);
  const [restartingProxy, setRestartingProxy] = useState(false);
  const logPollRef = useRef(null);

  const refresh = useCallback(async () => {
    setHeadroomStatus((s) => ({ ...s, loading: true }));
    try {
      const res = await fetch("/api/headroom/status", { headers: { "Cache-Control": "no-store" } });
      const data = await res.json();
      setHeadroomStatus({ ...data, loading: false });
      if (!data?.installed) { setHeadroomExtras(DEFAULT_EXTRAS); setPendingExtras([]); return; }
      try {
        const er = await fetch("/api/headroom/extras", { headers: { "Cache-Control": "no-store" } });
        if (!er.ok) throw new Error("extras status failed");
        const ed = await er.json();
        setHeadroomExtras((s) => ({ ...s, version: ed.version ?? null, extras: ed.extras || DEFAULT_EXTRAS.extras, available: ed.available || DEFAULT_EXTRAS.available, loading: false }));
      } catch { setHeadroomExtras(DEFAULT_EXTRAS); }
      setPendingExtras([]);
    } catch {
      setHeadroomStatus({ installed: false, running: false, python: null, loading: false });
      setHeadroomExtras(DEFAULT_EXTRAS); setPendingExtras([]);
    }
  }, []);

  useEffect(() => () => { if (logPollRef.current) clearInterval(logPollRef.current); }, []);

  const start = useCallback(async () => {
    setActionError(""); setActionLoading(true);
    try { const res = await fetch("/api/headroom/start", { method: "POST" }); const data = await res.json().catch(() => ({})); if (!res.ok) throw new Error(data.error || "Failed to start proxy"); await refresh(); }
    catch (e) { setActionError(e.message); } finally { setActionLoading(false); }
  }, [refresh]);
  const stop = useCallback(async () => {
    setActionLoading(true);
    try { await fetch("/api/headroom/stop", { method: "POST" }); await refresh(); } finally { setActionLoading(false); }
  }, [refresh]);
  const startLogPolling = useCallback(() => {
    setInstallLog(""); if (logPollRef.current) clearInterval(logPollRef.current);
    const tick = async () => { try { const r = await fetch("/api/headroom/extras?log=1", { headers: { "Cache-Control": "no-store" } }); const d = await r.json().catch(() => ({})); if (typeof d.log === "string") setInstallLog(d.log); } catch { /* ignore transient poll errors */ } };
    tick(); logPollRef.current = setInterval(tick, 1500);
  }, []);
  const stopLogPolling = useCallback(() => { if (logPollRef.current) { clearInterval(logPollRef.current); logPollRef.current = null; } }, []);
  const installExtras = useCallback(async () => {
    if (!pendingExtras.length) return; setExtrasActionLoading(true); setExtrasActionError(""); startLogPolling();
    try { const res = await fetch("/api/headroom/extras", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ extras: pendingExtras }) }); const data = await res.json().catch(() => ({})); if (!res.ok) throw new Error(data.error || "Install failed"); setHeadroomExtras((s) => ({ ...s, version: data.version ?? s.version, extras: data.extras || s.extras })); setPendingExtras([]); }
    catch (e) { setExtrasActionError(e.message); } finally { stopLogPolling(); setExtrasActionLoading(false); }
  }, [pendingExtras, startLogPolling, stopLogPolling]);
  const removeExtra = useCallback(async (extra) => {
    setRemovingExtra(extra); setExtrasActionError(""); startLogPolling();
    try { const res = await fetch("/api/headroom/extras", { method: "DELETE", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ extras: [extra] }) }); const data = await res.json().catch(() => ({})); if (!res.ok) throw new Error(data.error || "Remove failed"); setHeadroomExtras((s) => ({ ...s, version: data.version ?? s.version, extras: data.extras || s.extras })); }
    catch (e) { setExtrasActionError(e.message); } finally { stopLogPolling(); setRemovingExtra(null); }
  }, [startLogPolling, stopLogPolling]);
  const toggleExtraActive = useCallback(async (extra, value) => {
    setExtrasActionError(""); if (extra === "code") setCodeAware(value); if (extra === "ml") setKompress(value);
    await patchSetting({ [extra === "code" ? "headroomCodeAware" : "headroomKompress"]: value }); if (!headroomStatus.running) return;
    setRestartingProxy(true); try { const res = await fetch("/api/headroom/restart", { method: "POST" }); const data = await res.json().catch(() => ({})); if (!res.ok) throw new Error(data.error || "Restart failed"); await refresh(); } catch (e) { setExtrasActionError(e.message); } finally { setRestartingProxy(false); }
  }, [headroomStatus.running, patchSetting, refresh, setCodeAware, setKompress]);
  const toggleEnabled = (value) => { const nextUrl = headroomUrl.trim() || "http://localhost:8787"; setHeadroomUrl(nextUrl); setHeadroomEnabled(value); patchSetting({ headroomEnabled: value, headroomUrl: nextUrl }); };
  const blurUrl = async () => { const next = headroomUrl.trim() || "http://localhost:8787"; setHeadroomUrl(next); await patchSetting({ headroomUrl: next }); refresh(); };
  return { headroomStatus, showInstallModal, setShowInstallModal, actionLoading, actionError, headroomExtras, pendingExtras, extrasActionLoading, extrasActionError, removingExtra, installLog, extrasConfirm, setExtrasConfirm, codeAware, kompress, restartingProxy, refresh, start, stop, startLogPolling, toggleExtraActive, installExtras, removeExtra, toggleEnabled, blurUrl, setPendingExtras };
}
