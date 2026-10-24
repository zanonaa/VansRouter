package providers

import "testing"

func TestCatalog(t *testing.T) {
	entries := Catalog()
	if len(entries) != 160 {
		t.Fatalf("snapshot count: %d", len(entries))
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		if seen[entry.ID] || entry.ID == "" {
			t.Fatalf("duplicate/empty ID: %s", entry.ID)
		}
		seen[entry.ID] = true
		if entry.DisplayName != entry.ID {
			t.Fatalf("source response mismatch: %s", entry.ID)
		}
		if entry.Aliases == nil || entry.ServiceKinds == nil || entry.AuthModes == nil {
			t.Fatalf("null array: %s", entry.ID)
		}
	}
	for _, id := range []string{"openai", "codex", "ollama-local", "anthropic"} {
		if !seen[id] {
			t.Fatalf("missing %s", id)
		}
	}
	entries[0].ID = "changed"
	if Catalog()[0].ID == "changed" {
		t.Fatal("snapshot mutated")
	}
}
func TestNormalize(t *testing.T) {
	for input, want := range map[string]string{" OpenAI ": "openai", "Ollama Local": "ollama-local", " unknown-id ": "unknown-id", "Codex": "codex"} {
		if got := NormalizeID(input); got != want {
			t.Fatalf("%q: %s want %s", input, got, want)
		}
	}
	for _, entry := range Catalog() {
		for _, alias := range entry.Aliases {
			if AliasMap()[alias] == "" {
				t.Fatalf("alias missing: %s", alias)
			}
		}
	}
}
