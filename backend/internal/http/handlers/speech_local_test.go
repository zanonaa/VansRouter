package handlers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalSpeechUsesSeparatedArgumentsAndCleansDirectory(t *testing.T) {
	root := t.TempDir()
	calls := 0
	text := "hello; $(not-a-command)"
	voice := "voice with spaces"
	audio, err := synthesizeLocalAudio(context.Background(), text, voice, root, func(ctx context.Context, name string, args ...string) error {
		calls++
		if calls == 1 {
			if name != "say" || len(args) != 5 || args[0] != "-v" || args[1] != voice || args[4] != text {
				t.Fatal(name, args)
			}
			return os.WriteFile(args[3], []byte("aiff"), 0600)
		}
		if calls != 2 || name != "ffmpeg" || args[0] != "-y" {
			t.Fatal(name, args)
		}
		return os.WriteFile(args[len(args)-1], []byte("mp3-audio"), 0600)
	})
	if err != nil || string(audio) != "mp3-audio" || calls != 2 {
		t.Fatalf("%v %s calls %d", err, audio, calls)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("temporary audio not cleaned")
	}
}
func TestLocalSpeechCleansOnCommandFailure(t *testing.T) {
	root := t.TempDir()
	_, err := synthesizeLocalAudio(context.Background(), "hello", "", root, func(context.Context, string, ...string) error { return fmt.Errorf("missing executable") })
	if err == nil {
		t.Fatal("ignored failure")
	}
	entries, _ := filepath.Glob(filepath.Join(root, "tts-*"))
	if len(entries) != 0 {
		t.Fatal(entries)
	}
}
