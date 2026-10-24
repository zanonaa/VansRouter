package database

import "testing"

func TestIdentityNonComparableJSON(t *testing.T) {
	for _, value := range []any{map[string]any{"id": "x"}, []any{"x"}} {
		old := map[string]any{"provider": "codex", "authType": "oauth", "email": "e", "providerSpecificData": map[string]any{"chatgptAccountId": value}}
		incoming := map[string]any{"provider": "codex", "authType": "oauth", "email": "e", "providerSpecificData": map[string]any{"chatgptAccountId": value}}
		if sameIdentity(old, incoming) {
			t.Fatal("JSON object must not act as scalar identity")
		}
	}
}
