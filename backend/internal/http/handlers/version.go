package handlers

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed app-version.txt
var appVersion string

func NewVersion() *Version { return &Version{Current: strings.TrimSpace(appVersion)} }

// Version mantém cache/backoff e cliente injetável para testes sem rede externa.
type Version struct {
	Current         string
	Client          *http.Client
	URL             string
	mu              sync.Mutex
	latest          string
	fetched, failed time.Time
}

func newer(a, b string) bool {
	aa, bb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		if i >= len(aa) || i >= len(bb) {
			return false
		}
		x, e1 := strconv.Atoi(aa[i])
		y, e2 := strconv.Atoi(bb[i])
		if e1 != nil || e2 != nil {
			continue
		}
		if x != y {
			return x > y
		}
	}
	return false
}
func (v *Version) Get(w http.ResponseWriter, r *http.Request) {
	var latest any
	if os.Getenv("VANSROUTER_SKIP_UPDATE_CHECK") != "1" && os.Getenv("VANROUTER_SKIP_UPDATE_CHECK") != "1" {
		v.mu.Lock()
		if (v.latest != "" && time.Since(v.fetched) < 5*time.Minute) || (!v.failed.IsZero() && time.Since(v.failed) < 30*time.Second) {
			if v.latest != "" {
				latest = v.latest
			}
		} else {
			client := v.Client
			if client == nil {
				client = &http.Client{Timeout: 4 * time.Second}
			}
			target := v.URL
			if target == "" {
				target = "https://registry.npmjs.org/vansrouter/latest"
			}
			req, err := http.NewRequestWithContext(r.Context(), "GET", target, nil)
			if err == nil {
				var response *http.Response
				response, err = client.Do(req)
				if err == nil {
					var value struct {
						Version string `json:"version"`
					}
					err = json.NewDecoder(response.Body).Decode(&value)
					response.Body.Close()
					if err == nil && value.Version != "" {
						v.latest = value.Version
						v.fetched = time.Now()
						v.failed = time.Time{}
						latest = v.latest
					} else {
						v.failed = time.Now()
					}
				}
			}
			if err != nil {
				v.failed = time.Now()
			}
		}
		v.mu.Unlock()
	}
	update := false
	if text, ok := latest.(string); ok {
		update = newer(text, v.Current)
	}
	var build any
	if id := os.Getenv("RELEASE_BUILD_ID"); id != "" {
		build = id
	}
	nodeReply(w, 200, map[string]any{"currentVersion": v.Current, "latestVersion": latest, "hasUpdate": update, "buildId": build})
}
