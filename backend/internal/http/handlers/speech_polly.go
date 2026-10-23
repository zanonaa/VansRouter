package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func pollyHMAC(key []byte, value string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(value))
	return h.Sum(nil)
}

// signPolly signs the exact JSON bytes sent over the wire using AWS SigV4.
func signPolly(request *http.Request, payload []byte, accessKey, secret, region, sessionToken string, now time.Time) {
	stamp, day := now.UTC().Format("20060102T150405Z"), now.UTC().Format("20060102")
	digest := sha256.Sum256(payload)
	payloadHash := hex.EncodeToString(digest[:])
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Amz-Date", stamp)
	request.Header.Set("X-Amz-Content-Sha256", payloadHash)
	signedHeaders := "content-type;host;x-amz-content-sha256;x-amz-date"
	canonicalHeaders := "content-type:application/json\nhost:" + request.URL.Host + "\nx-amz-content-sha256:" + payloadHash + "\nx-amz-date:" + stamp + "\n"
	if sessionToken != "" {
		request.Header.Set("X-Amz-Security-Token", sessionToken)
		signedHeaders += ";x-amz-security-token"
		canonicalHeaders += "x-amz-security-token:" + sessionToken + "\n"
	}
	canonical := request.Method + "\n" + request.URL.EscapedPath() + "\n" + request.URL.Query().Encode() + "\n" + canonicalHeaders + "\n" + signedHeaders + "\n" + payloadHash
	canonicalHash := sha256.Sum256([]byte(canonical))
	scope := day + "/" + region + "/polly/aws4_request"
	toSign := "AWS4-HMAC-SHA256\n" + stamp + "\n" + scope + "\n" + hex.EncodeToString(canonicalHash[:])
	key := pollyHMAC([]byte("AWS4"+secret), day)
	key = pollyHMAC(key, region)
	key = pollyHMAC(key, "polly")
	key = pollyHMAC(key, "aws4_request")
	signature := hex.EncodeToString(pollyHMAC(key, toSign))
	request.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+accessKey+"/"+scope+", SignedHeaders="+signedHeaders+", Signature="+signature)
}

func (g *Gateway) synthesizePolly(w http.ResponseWriter, r *http.Request, model, secret string, connection map[string]any, body map[string]any) {
	metadata, _ := connection["providerSpecificData"].(map[string]any)
	accessKey := ttsString(metadata["accessKeyId"])
	if accessKey == "" {
		g.webError(w, 400, "AWS Polly requires providerSpecificData.accessKeyId")
		return
	}
	region := firstNonEmptyString(metadata["region"], "us-east-1")
	engine, voice := model, firstNonEmptyString(body["voice"], "Joanna")
	if parts := strings.SplitN(model, "/", 2); len(parts) == 2 {
		engine, voice = parts[0], parts[1]
	}
	switch engine {
	case "standard", "neural", "long-form", "generative":
	default:
		g.webError(w, 400, "Invalid AWS Polly engine")
		return
	}
	format := firstNonEmptyString(body["response_format"], "mp3")
	if format == "json" {
		format = "mp3"
	}
	if format != "mp3" && format != "ogg_vorbis" && format != "pcm" {
		g.webError(w, 400, "Unsupported AWS Polly audio format")
		return
	}
	payload := map[string]any{"Engine": engine, "VoiceId": voice, "Text": strings.TrimSpace(ttsString(body["input"])), "OutputFormat": format}
	if language := ttsString(body["language"]); language != "" {
		payload["LanguageCode"] = language
	}
	if sample := ttsString(body["sample_rate"]); sample != "" {
		payload["SampleRate"] = sample
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		g.webError(w, 500, err.Error())
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), "POST", fmt.Sprintf("https://polly.%s.amazonaws.com/v1/speech", region), bytes.NewReader(encoded))
	if err != nil {
		g.webError(w, 400, err.Error())
		return
	}
	signPolly(request, encoded, accessKey, secret, region, ttsString(metadata["sessionToken"]), time.Now())
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
	audio, err := io.ReadAll(response.Body)
	if err != nil {
		g.webError(w, 502, err.Error())
		return
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		g.webError(w, response.StatusCode, string(audio))
		return
	}
	if len(audio) == 0 {
		g.webError(w, 502, "AWS Polly returned empty audio")
		return
	}
	if format == "ogg_vorbis" {
		format = "ogg"
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.URL.Query().Get("response_format") == "json" {
		nodeReply(w, 200, map[string]any{"audio": base64.StdEncoding.EncodeToString(audio), "format": format})
		return
	}
	w.Header().Set("Content-Type", "audio/"+format)
	w.WriteHeader(200)
	_, _ = w.Write(audio)
}
