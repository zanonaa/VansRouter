package handlers

import (
	"encoding/json"
	"os"
	"testing"
)

func TestContextLengthOriginal(t *testing.T) {
	raw, err := os.ReadFile("testdata/context-length-original.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Input    any
		Expected struct {
			OK    bool `json:"ok"`
			Value any
			Error string
		}
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tt := range cases {
		value, message := contextLength(tt.Input)
		if (message == "") != tt.Expected.OK || value != tt.Expected.Value || message != tt.Expected.Error {
			t.Fatalf("input %v: %v %s want %+v", tt.Input, value, message, tt.Expected)
		}
	}
}
