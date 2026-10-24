package handlers

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestNodeGeneratedJSValues(t *testing.T) {
	raw, err := os.ReadFile("testdata/js-values.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Value  any            `json:"value"`
		String string         `json:"string"`
		Spread map[string]any `json:"spread"`
	}
	if err = json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		if got := jsString(fixture.Value); got != fixture.String {
			t.Fatalf("String(%v): %s != %s", fixture.Value, got, fixture.String)
		}
		if got := jsSpread(fixture.Value); !reflect.DeepEqual(got, fixture.Spread) {
			t.Fatalf("spread(%v): %v != %v", fixture.Value, got, fixture.Spread)
		}
	}
}
