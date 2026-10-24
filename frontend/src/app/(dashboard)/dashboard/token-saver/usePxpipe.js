import { useCallback, useState } from "react";

export function usePxpipe({ patchSetting, setPxpipeEnabled }) {
  const [pxpipeStatus, setPxpipeStatus] = useState({ installed: false, installing: false, running: false, version: null, loading: true });
  const [pxpipeHealth, setPxpipeHealth] = useState(null);
  const [showModal, setShowModal] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [actionError, setActionError] = useState("");
  const [minChars, setMinChars] = useState(25000);
  const refresh = useCallback(async () => { setPxpipeStatus((s) => ({ ...s, loading: true })); try { const res = await fetch("/api/pxpipe/status", { headers: { "Cache-Control": "no-store" } }); const data = await res.json(); setPxpipeStatus({ ...data, loading: false }); if (typeof data.minChars === "number") setMinChars(data.minChars); } catch { setPxpipeStatus({ installed: false, installing: false, running: false, version: null, loading: false }); } }, []);
  const health = useCallback(async () => { try { const res = await fetch("/api/pxpipe/health", { method: "POST" }); setPxpipeHealth(await res.json()); } catch (e) { setPxpipeHealth({ healthy: false, checks: [], error: e.message }); } }, []);
  const action = useCallback(async (endpoint) => { setActionError(""); setActionLoading(true); try { const res = await fetch(`/api/pxpipe/${endpoint}`, { method: "POST" }); const data = await res.json().catch(() => ({})); if (!res.ok) throw new Error(data.error || `PXPIPE ${endpoint} failed`); await refresh(); await health(); } catch (e) { setActionError(e.message); } finally { setActionLoading(false); } }, [health, refresh]);
  const setEnabled = (value) => {
    setPxpipeEnabled(value);
    patchSetting({ pxpipeEnabled: value });
  };
  const blurMinChars = () => { const next = Math.max(0, Number(minChars) || 25000); setMinChars(next); patchSetting({ pxpipeMinChars: next }); };
  return { pxpipeStatus, pxpipeHealth, showModal, setShowModal, actionLoading, actionError, minChars, setMinChars, refresh, health, action, setEnabled, blurMinChars };
}
