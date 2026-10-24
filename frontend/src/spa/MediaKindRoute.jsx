// Página de mídia sem leitura direta do banco (Fase 4A): os mesmos três
// conjuntos que o page.js server-side obtinha de localDb agora vêm dos
// endpoints Go, preservando o filtro custom-embedding do nó.
import { useEffect, useState } from "react";
import MediaKindClient from "@/app/(dashboard)/dashboard/media-providers/[kind]/MediaKindClient";

async function fetchJSON(url) {
  const res = await fetch(url, { headers: { Accept: "application/json" } });
  if (!res.ok) return null;
  return res.json();
}

export default function MediaKindRoute() {
  const [state, setState] = useState({ connections: [], nodes: [], combos: [] });

  useEffect(() => {
    let active = true;
    Promise.all([
      fetchJSON("/api/providers?pageSize=500"),
      fetchJSON("/api/nodes"),
      fetchJSON("/api/combos"),
    ])
      .then(([providers, nodes, combos]) => {
        if (!active) return;
        setState({
          connections: providers?.connections ?? [],
          nodes: (nodes?.nodes ?? nodes ?? []).filter?.((n) => n?.type === "custom-embedding") ?? [],
          combos: combos?.combos ?? combos ?? [],
        });
      })
      .catch(() => {});
    return () => {
      active = false;
    };
  }, []);

  return (
    <MediaKindClient
      initialConnections={state.connections}
      initialNodes={state.nodes}
      initialCombos={state.combos}
    />
  );
}
