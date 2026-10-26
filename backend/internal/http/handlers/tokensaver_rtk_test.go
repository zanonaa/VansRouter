package handlers

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func requireOfficialRTK(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("rtk"); err != nil {
		t.Skip("official RTK binary not installed; runtime integration test")
	}
}

func TestRTKOfficialPipeParity(t *testing.T) {
	requireOfficialRTK(t)
	text := strings.Repeat("main.go:12:some matching content\n", 80)
	cmd := exec.Command("rtk", "pipe")
	cmd.Stdin = strings.NewReader(text)
	expected, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	stats := &rtkStats{}
	got := compressRTKText(text, stats, "test")
	if !bytes.Equal([]byte(got), expected) {
		t.Fatal("gateway differs from official RTK output")
	}
	if len(got) >= len(text) {
		t.Fatal("fixture did not compress")
	}
}

func TestRTKMissingBinaryDoesNotUseLegacyFilters(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	text := strings.Repeat("main.go:12:some matching content\n", 80)
	stats := &rtkStats{}
	if got := compressRTKText(text, stats, "test"); got != text || len(stats.Hits) != 0 {
		t.Fatal("missing binary must leave content unchanged, never use copied filters")
	}
}
