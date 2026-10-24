package providers

import "testing"

func TestUpstreamModelID(t *testing.T) {
	// ds/... é o alias roteado; deepseek/... é o id interno. Ambos mapeiam.
	for _, provider := range []string{"ds", "deepseek"} {
		if got := UpstreamModelID(provider, "deepseek-v4-pro-max"); got != "deepseek-v4-pro" {
			t.Fatalf("%s: esperava deepseek-v4-pro, veio %q", provider, got)
		}
	}
	// Modelo sem mapeamento permanece igual.
	if got := UpstreamModelID("ds", "deepseek-v4-pro"); got != "deepseek-v4-pro" {
		t.Fatalf("identidade quebrada: %q", got)
	}
	// Revisão do Codex cai no modelo base.
	if got := UpstreamModelID("codex", "gpt-6.1-sol-review"); got != "gpt-6.1-sol" {
		t.Fatalf("review suffix: %q", got)
	}
	// Provider-node (id com UUID) não tem mapeamento e não deve ser tocado.
	node := "openai-compatible-chat-2ab1e1b8-5893-4779-b316-49b59514e296"
	if got := UpstreamModelID(node, "MiniMaxAI/MiniMax-M2.7"); got != "MiniMaxAI/MiniMax-M2.7" {
		t.Fatalf("node alterado: %q", got)
	}
}

func TestReservedPrefix(t *testing.T) {
	for _, prefix := range []string{"ds", "deepseek", "cx", "openai", "xmtp"} {
		if !ReservedPrefix(prefix) {
			t.Fatalf("%s deveria ser reservado", prefix)
		}
	}
	if ReservedPrefix("da") {
		t.Fatal("prefixo de provider-node marcado como reservado")
	}
}
