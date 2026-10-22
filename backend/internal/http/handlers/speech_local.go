package handlers

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type localSpeechCommand func(context.Context, string, ...string) error

func runLocalSpeechCommand(ctx context.Context, name string, args ...string) error {
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(output)))
	}
	return nil
}
func synthesizeLocalAudio(ctx context.Context, text, voice, parent string, run localSpeechCommand) ([]byte, error) {
	dir, err := os.MkdirTemp(parent, "tts-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	aiff, mp3 := filepath.Join(dir, "out.aiff"), filepath.Join(dir, "out.mp3")
	args := []string{"-o", aiff, text}
	if voice != "" {
		args = append([]string{"-v", voice}, args...)
	}
	if err := run(ctx, "say", args...); err != nil {
		return nil, err
	}
	if err := run(ctx, "ffmpeg", "-y", "-i", aiff, "-codec:a", "libmp3lame", "-qscale:a", "4", mp3); err != nil {
		return nil, err
	}
	return os.ReadFile(mp3)
}
func (g *Gateway) synthesizeLocal(w http.ResponseWriter, r *http.Request, voice string, body map[string]any) {
	ctx, cancel := context.WithTimeout(r.Context(), 300*time.Second)
	defer cancel()
	audio, err := synthesizeLocalAudio(ctx, strings.TrimSpace(ttsString(body["input"])), voice, "", runLocalSpeechCommand)
	if err != nil {
		g.webError(w, 502, err.Error())
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.URL.Query().Get("response_format") == "json" {
		nodeReply(w, 200, map[string]any{"audio": base64.StdEncoding.EncodeToString(audio), "format": "mp3"})
		return
	}
	w.Header().Set("Content-Type", "audio/mp3")
	w.Header().Set("Content-Length", strconv.Itoa(len(audio)))
	w.Write(audio)
}
