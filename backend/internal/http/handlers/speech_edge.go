package handlers

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type edgeSpeechToken struct {
	key, token, cookie string
	fetched            time.Time
}

var edgeSpeechTokenPattern = regexp.MustCompile(`params_AbusePreventionHelper\s*=\s*\[([^,]+),([^,]+),`)
var edgeSpeechMale = regexp.MustCompile(`(?i)(?:^|-)male`)

func edgeXML(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return replacer.Replace(value)
}

func (g *Gateway) edgeToken(ctx context.Context, force bool) (edgeSpeechToken, error) {
	g.edgeSpeechMu.Lock()
	defer g.edgeSpeechMu.Unlock()
	token := g.edgeSpeechCache
	if !force && token.key != "" && time.Since(token.fetched) < 5*time.Minute {
		return token, nil
	}
	request, err := http.NewRequestWithContext(ctx, "GET", "https://www.bing.com/translator", nil)
	if err != nil {
		return token, err
	}
	request.Header.Set("User-Agent", googleSpeechUA)
	request.Header.Set("Accept-Language", "vi,en-US;q=0.9,en;q=0.8")
	client := g.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return token, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	response.Body.Close()
	if readErr != nil {
		return token, readErr
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return token, fmt.Errorf("Bing translator fetch failed: %d", response.StatusCode)
	}
	match := edgeSpeechTokenPattern.FindSubmatch(raw)
	if len(match) < 3 {
		return token, fmt.Errorf("Failed to parse Bing token")
	}
	cookies := []string{}
	for _, cookie := range response.Cookies() {
		cookies = append(cookies, cookie.Name+"="+cookie.Value)
	}
	token = edgeSpeechToken{key: string(match[1]), token: strings.ReplaceAll(string(match[2]), `"`, ""), cookie: strings.Join(cookies, "; "), fetched: time.Now()}
	g.edgeSpeechCache = token
	return token, nil
}

func (g *Gateway) synthesizeEdge(w http.ResponseWriter, r *http.Request, voice string, body map[string]any) {
	ctx, cancel := context.WithTimeout(r.Context(), 300*time.Second)
	defer cancel()
	if voice == "" {
		voice = "vi-VN-HoaiMyNeural"
	}
	text := strings.TrimSpace(ttsString(body["input"]))
	requestAudio := func(token edgeSpeechToken) (*http.Response, error) {
		parts := strings.Split(voice, "-")
		language := voice
		if len(parts) >= 2 {
			language = strings.Join(parts[:2], "-")
		}
		gender := "Female"
		if edgeSpeechMale.MatchString(voice) {
			gender = "Male"
		}
		ssml := fmt.Sprintf("<speak version='1.0' xml:lang='%s'><voice xml:lang='%s' xml:gender='%s' name='%s'><prosody rate='0.00%%'>%s</prosody></voice></speak>", edgeXML(language), edgeXML(language), edgeXML(gender), edgeXML(voice), edgeXML(text))
		form := url.Values{"ssml": {ssml}, "token": {token.token}, "key": {token.key}}
		request, err := http.NewRequestWithContext(ctx, "POST", "https://www.bing.com/tfettts?isVertical=1&&IG=1&IID=translator.5023&SFX=1", strings.NewReader(form.Encode()))
		if err != nil {
			return nil, err
		}
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.Header.Set("Accept", "*/*")
		request.Header.Set("Origin", "https://www.bing.com")
		request.Header.Set("Referer", "https://www.bing.com/translator")
		request.Header.Set("User-Agent", googleSpeechUA)
		if token.cookie != "" {
			request.Header.Set("Cookie", token.cookie)
		}
		client := g.Client
		if client == nil {
			client = http.DefaultClient
		}
		return client.Do(request)
	}
	token, err := g.edgeToken(ctx, false)
	if err != nil {
		g.webError(w, 502, err.Error())
		return
	}
	response, err := requestAudio(token)
	if err != nil {
		g.webError(w, 502, err.Error())
		return
	}
	if response.StatusCode == 429 || response.StatusCode == 403 {
		response.Body.Close()
		token, err = g.edgeToken(ctx, true)
		if err != nil {
			g.webError(w, 502, err.Error())
			return
		}
		response, err = requestAudio(token)
		if err != nil {
			g.webError(w, 502, err.Error())
			return
		}
	}
	defer response.Body.Close()
	audio, err := io.ReadAll(response.Body)
	if err != nil {
		g.webError(w, 502, err.Error())
		return
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		g.webError(w, 502, fmt.Sprintf("Bing TTS failed: %d - %s", response.StatusCode, strings.TrimSpace(string(audio))))
		return
	}
	if len(audio) < 1024 {
		g.webError(w, 502, "Bing TTS returned empty audio")
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
