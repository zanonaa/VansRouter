package handlers

import (
	"context"
	"encoding/json"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

var (
	localVoicesOnce  sync.Once
	localVoicesCache []map[string]any
	macVoicePattern  = regexp.MustCompile(`^([^\s].*?)\s{2,}([a-z]{2}_[A-Z]{2})`)
)

func FetchLocalDeviceVoicesPublic(context.Context, string) ([]map[string]any, error) {
	localVoicesOnce.Do(func() {
		var err error
		if runtime.GOOS == "windows" {
			localVoicesCache, err = fetchWindowsVoices()
		} else {
			localVoicesCache, err = fetchMacVoices()
		}
		if err != nil {
			localVoicesCache = []map[string]any{}
		}
	})
	return append([]map[string]any{}, localVoicesCache...), nil
}

func fetchMacVoices() ([]map[string]any, error) {
	output, err := exec.Command("say", "-v", "?").Output()
	if err != nil {
		return nil, err
	}
	voices := []map[string]any{}
	for _, line := range strings.Split(string(output), "\n") {
		match := macVoicePattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		name, locale := strings.TrimSpace(match[1]), strings.TrimSpace(match[2])
		parts := strings.SplitN(locale, "_", 2)
		country := ""
		if len(parts) == 2 {
			country = parts[1]
		}
		voices = append(voices, map[string]any{"id": name, "name": name, "locale": locale, "lang": parts[0], "country": country, "gender": ""})
	}
	return voices, nil
}

func fetchWindowsVoices() ([]map[string]any, error) {
	script := "Add-Type -AssemblyName System.Speech; $s = New-Object System.Speech.Synthesis.SpeechSynthesizer; $s.GetInstalledVoices() | ForEach-Object { $v = $_.VoiceInfo; [PSCustomObject]@{ Name=$v.Name; Culture=$v.Culture.Name; Gender=$v.Gender } } | ConvertTo-Json -Compress"
	output, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script).Output()
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(string(output))
	if raw == "" {
		raw = "[]"
	}
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, err
	}
	items, ok := decoded.([]any)
	if !ok {
		items = []any{decoded}
	}
	voices := []map[string]any{}
	genders := map[string]string{"1": "Male", "2": "Female", "Male": "Male", "Female": "Female"}
	for _, item := range items {
		entry, _ := item.(map[string]any)
		culture := ttsString(entry["Culture"])
		if culture == "" {
			culture = "en-US"
		}
		parts := strings.SplitN(culture, "-", 2)
		country := ""
		if len(parts) == 2 {
			country = parts[1]
		}
		name := ttsString(entry["Name"])
		voices = append(voices, map[string]any{"id": name, "name": name, "locale": strings.ReplaceAll(culture, "-", "_"), "lang": parts[0], "country": country, "gender": genders[ttsString(entry["Gender"])]})
	}
	return voices, nil
}
