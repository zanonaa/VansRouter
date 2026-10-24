// Fase 4A — a página endpoint calculava machineId no SSR e repassava ao client,
// mas EndpointPageClient não consome a prop (verificação no lote). A rota SPA
// renderiza o client diretamente, eliminando a leitura de banco no navegador.
import EndpointPageClient from "@/app/(dashboard)/dashboard/endpoint/EndpointPageClient";

export default function EndpointRoute() {
  return <EndpointPageClient />;
}
