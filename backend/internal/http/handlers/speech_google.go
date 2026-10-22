package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type googleSpeechToken struct {
	sid, build string
	fetched    time.Time
}

var googleSpeechSID = regexp.MustCompile(`"FdrFJe":"(.*?)"`)
var googleSpeechBuild = regexp.MustCompile(`"cfb2h":"(.*?)"`)
var googleSpeechClean = regexp.MustCompile("[@^*()\\\\/\\-_+=><\"'“”【】]")

const googleSpeechUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"

func (g *Gateway) googleToken(ctx context.Context) (googleSpeechToken, uint64, error) {
	g.googleSpeechMu.Lock()
	defer g.googleSpeechMu.Unlock()
	token := g.googleSpeechCache
	if token.sid == "" || time.Since(token.fetched) >= 11*time.Minute {
		request, err := http.NewRequestWithContext(ctx, "GET", "https://translate.google.com/", nil)
		if err != nil {
			return token, 0, err
		}
		request.Header.Set("User-Agent", googleSpeechUA)
		client := g.Client
		if client == nil {
			client = http.DefaultClient
		}
		response, err := client.Do(request)
		if err != nil {
			return token, 0, err
		}
		raw, readErr := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		response.Body.Close()
		if readErr != nil {
			return token, 0, readErr
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return token, 0, fmt.Errorf("Google translate fetch failed: %d", response.StatusCode)
		}
		sid, build := googleSpeechSID.FindSubmatch(raw), googleSpeechBuild.FindSubmatch(raw)
		if len(sid) < 2 || len(build) < 2 {
			return token, 0, fmt.Errorf("Failed to parse Google token")
		}
		token = googleSpeechToken{sid: string(sid[1]), build: string(build[1]), fetched: time.Now()}
		g.googleSpeechCache = token
	}
	g.googleSpeechIndex++
	return token, g.googleSpeechIndex, nil
}
func (g *Gateway) synthesizeGoogle(w http.ResponseWriter, r *http.Request, lang string, body map[string]any) {
	ctx, cancel := context.WithTimeout(r.Context(), 300*time.Second)
	defer cancel()
	token, index, err := g.googleToken(ctx)
	if err != nil {
		g.webError(w, 502, err.Error())
		return
	}
	if lang == "" {
		lang = "en"
	}
	text := strings.ReplaceAll(googleSpeechClean.ReplaceAllString(strings.TrimSpace(ttsString(body["input"])), " "), ", ", ". ")
	nonce := make([]byte, 4)
	if _, err := rand.Read(nonce); err != nil {
		g.webError(w, 500, err.Error())
		return
	}
	query := url.Values{"rpcids": {"jQ1olc"}, "f.sid": {token.sid}, "bl": {token.build}, "hl": {lang}, "soc-app": {"1"}, "soc-platform": {"1"}, "soc-device": {"1"}, "_reqid": {strconv.FormatUint(index*100000+1000+uint64(binary.BigEndian.Uint32(nonce)%9000), 10)}, "rt": {"c"}}
	inner, _ := json.Marshal([]any{text, lang, nil, "undefined", []any{0}})
	envelope, _ := json.Marshal([]any{[]any{[]any{"jQ1olc", string(inner), nil, "generic"}}})
	request, err := http.NewRequestWithContext(ctx, "POST", "https://translate.google.com/_/TranslateWebserverUi/data/batchexecute?"+query.Encode(), strings.NewReader(url.Values{"f.req": {string(envelope)}}.Encode()))
	if err != nil {
		g.webError(w, 500, err.Error())
		return
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Referer", "https://translate.google.com/")
	client := g.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		g.webError(w, 502, err.Error())
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		g.webError(w, 502, fmt.Sprintf("Google TTS failed: %d", response.StatusCode))
		return
	}
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		g.webError(w, 502, err.Error())
		return
	}
	lines := strings.Split(string(raw), "\n")
	var outer []json.RawMessage
	if len(lines) < 4 || json.Unmarshal([]byte(lines[3]), &outer) != nil || len(outer) == 0 {
		g.webError(w, 502, "Invalid Google TTS RPC response")
		return
	}
	var row []json.RawMessage
	if json.Unmarshal(outer[0], &row) != nil || len(row) < 3 {
		g.webError(w, 502, "Invalid Google TTS RPC response")
		return
	}
	var encoded string
	if json.Unmarshal(row[2], &encoded) != nil {
		g.webError(w, 502, "Invalid Google TTS RPC response")
		return
	}
	var audioValues []json.RawMessage
	if json.Unmarshal([]byte(encoded), &audioValues) != nil || len(audioValues) == 0 {
		g.webError(w, 502, "Google TTS returned empty audio")
		return
	}
	var audioBase64 string
	json.Unmarshal(audioValues[0], &audioBase64)
	if len(audioBase64) < 100 {
		g.webError(w, 502, "Google TTS returned empty audio")
		return
	}
	audio, err := base64.StdEncoding.DecodeString(audioBase64)
	if err != nil {
		g.webError(w, 502, "Invalid Google audio encoding")
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.URL.Query().Get("response_format") == "json" {
		nodeReply(w, 200, map[string]any{"audio": audioBase64, "format": "mp3"})
		return
	}
	w.Header().Set("Content-Type", "audio/mp3")
	w.Header().Set("Content-Length", strconv.Itoa(len(audio)))
	w.Write(audio)
}
