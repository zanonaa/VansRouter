"use client";

import { useEffect, useState } from "react";
import { Button, Input, Modal } from "@/shared/components";

async function nordRequest(path, body, method = "POST") {
  const response = await fetch(`/api/nordvpn/${path}`, {
    method,
    ...(body !== undefined ? { headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) } : {}),
  });
  const data = await response.json();
  if (!response.ok) throw new Error(data.error || "NordVPN request failed");
  return data;
}

export default function NordVPNModal({ onClose, onChanged }) {
  const [countries, setCountries] = useState([]);
  const [selected, setSelected] = useState([]);
  const [replicas, setReplicas] = useState(1);
  const [token, setToken] = useState("");
  const [exchangeToken, setExchangeToken] = useState("");
  const [login, setLogin] = useState(null);
  const [connected, setConnected] = useState(false);
  const [tunnels, setTunnels] = useState([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    Promise.all([
      nordRequest("countries", undefined, "GET"),
      nordRequest("tunnels", undefined, "GET"),
    ]).then(([list, current]) => {
      if (active) { setCountries(list.countries || []); setTunnels(current.tunnels || []); }
    }).catch((e) => { if (active) setError(e.message); });
    return () => { active = false; };
  }, []);

  async function action(work) {
    setBusy(true);
    setError("");
    try { await work(); } catch (e) { setError(e.message); }
    finally { setBusy(false); }
  }

  async function refresh() {
    const current = await nordRequest("tunnels", undefined, "GET");
    setTunnels(current.tunnels || []);
    await onChanged();
  }

  return (
    <Modal isOpen title="NordVPN" onClose={onClose}>
      <div className="space-y-4">
        <p className="text-sm text-text-muted">Crie túneis por país e vincule os pools às conexões dos providers. O restante do tráfego não usa a VPN.</p>
        <p className="text-xs text-warning">A sessão da conta é salva cifrada no servidor; sessões OAuth são renovadas automaticamente quando a Nord permite. Tokens manuais exigem nova autenticação ao expirar. Túneis são restaurados automaticamente após restart; os pools ficam indisponíveis até o IP ser verificado. Tokens são credenciais sensíveis; não os compartilhe.</p>
        {error && <p role="alert" className="text-sm text-red-500">{error}</p>}
        <fieldset disabled={busy} className="space-y-3">
          <legend className="mb-2 font-medium">Autenticação</legend>
          <Button variant="secondary" onClick={() => action(async () => {
            setLogin(await nordRequest("oauth/start", {}));
          })}>Conectar com Nord Account / Google</Button>
          {login && <div className="space-y-2 rounded border border-black/10 p-3 dark:border-white/10">
            <a href={login.url} target="_blank" rel="noopener noreferrer" className="text-primary underline">Abrir login seguro na Nord Account</a>
            <p className="text-xs text-text-muted">Conclua o login no navegador da Nord (Google e MFA incluídos). Se o navegador oferecer abrir o aplicativo NordVPN, copie o link de retorno e cole abaixo. Esta etapa é manual; não é um redirecionamento automático ao VansRouter.</p>
            <Input label="Link de retorno ou exchange_token" type="password" value={exchangeToken} onChange={(e) => setExchangeToken(e.target.value)} />
            <Button disabled={!exchangeToken.trim()} onClick={() => action(async () => {
              let value = exchangeToken.trim();
              try { const url = new URL(value); value = url.searchParams.get("exchange_token") || value; } catch { /* token pasted directly */ }
              await nordRequest("oauth/complete", { state: login.state, exchangeToken: value });
              setConnected(true); setExchangeToken(""); setLogin(null);
            })}>Concluir autenticação</Button>
          </div>}
          <details>
            <summary className="cursor-pointer text-sm">Alternativa: token do dashboard Nord Account</summary>
            <div className="mt-2 space-y-2">
              <Input label="Token de acesso NordVPN" type="password" autoComplete="off" value={token} onChange={(e) => setToken(e.target.value)} />
              <Button disabled={!token.trim()} onClick={() => action(async () => {
                await nordRequest("token", { token }); setToken(""); setConnected(true);
              })}>Validar token</Button>
            </div>
          </details>
          {connected && <p role="status" className="text-sm text-green-600">Conta conectada nesta sessão do servidor.</p>}
        </fieldset>
        <fieldset disabled={busy} className="space-y-3">
          <legend className="mb-2 font-medium">Criar conexões</legend>
          <label className="block text-sm" htmlFor="nord-countries">Países (seleção múltipla)</label>
          <select id="nord-countries" multiple size={7} value={selected} onChange={(e) => setSelected(Array.from(e.target.selectedOptions, (option) => option.value))}
            className="w-full rounded border border-black/20 bg-bg-card p-2 dark:border-white/20">
            {countries.map((country) => <option key={country.code} value={country.code.toLowerCase()}>{country.name} ({country.code})</option>)}
          </select>
          <Input label="Túneis por país" type="number" min="1" max="10" value={replicas} onChange={(e) => setReplicas(Number(e.target.value))} />
          <p className="text-xs text-text-muted">Total: {selected.length * replicas}. Respeite o limite de conexões simultâneas da sua assinatura. País/servidor diferente não garante IP exclusivo ou aceitação pelo provider.</p>
          <Button disabled={!selected.length || !Number.isInteger(replicas) || replicas < 1 || replicas > 10} onClick={() => action(async () => {
            try { await nordRequest("tunnels", { countries: selected, replicas }); }
            finally { await refresh(); }
          })}>Criar túneis e pools</Button>
        </fieldset>
        <div className="space-y-2">
          <div className="flex items-center justify-between"><h3 className="font-medium">Túneis</h3>
            <Button variant="secondary" disabled={busy} onClick={() => action(async () => {
              const result = await nordRequest("tunnels/health", {}); setTunnels(result.tunnels || []);
            })}>Verificar IPs</Button>
          </div>
          {tunnels.length === 0 && <p className="text-sm text-text-muted">Nenhum túnel ativo.</p>}
          {tunnels.map((tunnel) => <div key={tunnel.id} className="space-y-2 rounded border border-black/10 p-3 dark:border-white/10">
            <p className="text-sm font-medium">{tunnel.id} — {tunnel.hostname}</p>
            <p className="break-all text-xs">{tunnel.lastIp || "IP ainda não verificado"} · {tunnel.connected ? "Conectado" : "Aguardando verificação"}</p>
            {tunnel.lastError && <p className="break-all text-xs text-red-500">{tunnel.lastError}</p>}
            <div className="flex gap-2">
              <Button variant="secondary" disabled={busy} onClick={() => action(async () => {
                await nordRequest(`tunnels/${encodeURIComponent(tunnel.id)}/rotate`, {}); await refresh();
              })}>Rotacionar servidor</Button>
              <Button variant="secondary" disabled={busy} onClick={() => action(async () => {
                await nordRequest(`tunnels/${encodeURIComponent(tunnel.id)}`, undefined, "DELETE"); await refresh();
              })}>Encerrar túnel</Button>
            </div>
          </div>)}
        </div>
        {busy && <p role="status" className="text-sm">Processando…</p>}
      </div>
    </Modal>
  );
}
