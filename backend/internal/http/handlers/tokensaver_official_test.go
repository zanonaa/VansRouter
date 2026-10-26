package handlers

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPxpipeBundledPackageDoesNotInstall(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PXPIPE_PACKAGE_ROOT", root)
	if err := os.MkdirAll(filepath.Join(root, "dist", "core"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"version":"0.15.1"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dist", "core", "library.js"), []byte("export const transformAnthropicMessages = () => {};"), 0644); err != nil {
		t.Fatal(err)
	}
	info, err := pxpipeRunInstall(context.Background(), t.TempDir(), func() string { t.Fatal("bundled runtime must not invoke npm"); return "" })
	if err != nil || info["version"] != pxpipeVersion {
		t.Fatalf("%v %v", info, err)
	}
	t.Setenv("PXPIPE_PACKAGE_ROOT", filepath.Join(root, "missing"))
	if _, err = pxpipeRunInstall(context.Background(), t.TempDir(), func() string { return "" }); err == nil {
		t.Fatal("missing bundle should fail explicitly")
	}
}

func TestOfficialPonytailAvailable(t *testing.T) {
	if len(officialPonytailPrompt) == 0 || ponytailPrompts["official"] != officialPonytailPrompt {
		t.Fatal("official rules unavailable")
	}
}
