package handlers

// Porta das rotas src/app/api/pxpipe/{health,install,status,start,stop,restart,logs,stats}
// e de src/lib/pxpipe/service.js (getPxpipeStatus + runHealthCheck).
import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// Pxpipe agrega o estado do library mode e as fontes de settings.
type Pxpipe struct {
	DataDir    string
	Read       func(context.Context) (map[string]any, error)
	Loader     *PxpipeLoader
	FindNpm    func() string
	RunInstall func(ctx context.Context, dataDir string) (map[string]any, error)
}

func NewPxpipe(dataDir string, read func(context.Context) (map[string]any, error)) *Pxpipe {
	return &Pxpipe{DataDir: dataDir, Read: read, Loader: &PxpipeLoader{DataDir: dataDir}}
}

func (p *Pxpipe) findNpm() string {
	if p.FindNpm != nil {
		return p.FindNpm()
	}
	return pxpipeFindNpm()
}

func (p *Pxpipe) install(ctx context.Context) (map[string]any, error) {
	run := p.RunInstall
	if run == nil {
		findNpm := p.findNpm
		run = func(runCtx context.Context, runDataDir string) (map[string]any, error) {
			return pxpipeRunInstall(runCtx, runDataDir, findNpm)
		}
	}
	return pxpipeState.pxpipeInstall(ctx, p.DataDir, run)
}

// pxpipeVersionText: `v${install.version}` do JS (null → "vnull").
func pxpipeVersionText(info map[string]any) string {
	if version, ok := info["version"].(string); ok {
		return "v" + version
	}
	return "vnull"
}

// statusMap espelha getPxpipeStatus.
func (p *Pxpipe) statusMap() map[string]any {
	install := pxpipeInstallInfo(p.DataDir)
	loaded := p.Loader.info()
	now := time.Now().UnixMilli()
	return map[string]any{
		"installed":    install["installed"],
		"installing":   pxpipeState.isInstalling(),
		"version":      install["version"],
		"path":         install["path"],
		"running":      loaded.Loaded,
		"loadedAt":     nilOrNil(loaded.LoadedAt),
		"uptimeMs":     uptimeMs(loaded, now),
		"npmAvailable": p.findNpm() != "",
		"mode":         "library",
	}
}

func nilOrNil(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}

func uptimeMs(loaded pxpipeLoadedInfo, now int64) int64 {
	if !loaded.Loaded {
		return 0
	}
	return now - loaded.LoadedAt
}

// pxpipeTruthy: semântica JS incluindo int/float (settings fora de JSON).
func pxpipeTruthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		return v != ""
	case float64:
		return v != 0
	case float32:
		return v != 0
	case int:
		return v != 0
	case int64:
		return v != 0
	default:
		return true
	}
}

// settingsValues lê as settings pxpipe (booleans + defaults nulos do JS).
func (p *Pxpipe) settingsValues(ctx context.Context) (enabled, autoInstall bool, minChars, timeoutMs any, err error) {
	settings := map[string]any{}
	if p.Read != nil {
		settings, err = p.Read(ctx)
		if err != nil {
			return false, false, nil, nil, err
		}
	}
	enabled = pxpipeTruthy(settings["pxpipeEnabled"])
	autoInstall = pxpipeTruthy(settings["pxpipeAutoInstall"])
	if value, exists := settings["pxpipeMinChars"]; exists {
		minChars = value
	}
	if value, exists := settings["pxpipeTimeoutMs"]; exists {
		timeoutMs = value
	}
	return enabled, autoInstall, minChars, timeoutMs, nil
}

// StatusMapWithSettings para as rotas status/start/restart.
func (p *Pxpipe) statusMapWithSettings(ctx context.Context) (map[string]any, error) {
	enabled, autoInstall, minChars, timeoutMs, err := p.settingsValues(ctx)
	if err != nil {
		return nil, err
	}
	out := p.statusMap()
	out["enabled"] = enabled
	out["autoInstall"] = autoInstall
	// Chaves ausentes nas settings = undefined do JS (omitidas no JSON).
	if minChars != nil {
		out["minChars"] = minChars
	}
	if timeoutMs != nil {
		out["timeoutMs"] = timeoutMs
	}
	return out, nil
}

// pxpipeLimit: Math.min(Number(q) || 100, 500).
func pxpipeLimit(query string) int {
	value, err := strconv.Atoi(query)
	if err != nil || value == 0 {
		value = 100
	}
	if value > 500 {
		return 500
	}
	return value
}

// runHealthCheck em library mode: installed → módulo carrega → transform de teste.
func (p *Pxpipe) check(id, label string, ok bool, detail any) map[string]any {
	return map[string]any{"id": id, "label": label, "ok": ok, "detail": detail}
}

func (p *Pxpipe) health(ctx context.Context) map[string]any {
	checks := []map[string]any{}
	install := pxpipeInstallInfo(p.DataDir)
	detail := any(nil)
	installedBool, _ := install["installed"].(bool)
	if pxpipeTruthyVersion(install["version"]) {
		detail = pxpipeVersionText(install)
	}
	checks = append(checks, p.check("installed", "PXPIPE installed", installedBool, detail))
	if !installedBool {
		return failingHealth(checks, "pxpipe not installed")
	}
	if err := p.Loader.Load(ctx); err != nil {
		checks = append(checks, p.check("module", "Transform module loads", false, err.Error()))
		return failingHealth(checks, "Cannot load module: "+err.Error())
	}
	checks = append(checks, p.check("module", "Transform module loads", true, pxpipeVersionText(install)))
	test, err := p.Loader.SelfTest(ctx)
	if err != nil {
		checks = append(checks, p.check("transform", "Test request transforms", false, err.Error()))
		return failingHealth(checks, "Self-test failed: "+err.Error())
	}
	reason, _ := test["reason"].(string)
	duration, _ := test["durationMs"].(int64)
	checks = append(checks, p.check("transform", "Test request transforms", true, fmtMillis(duration, reason)))
	return map[string]any{"healthy": true, "checks": checks, "error": nil}
}

func pxpipeTruthyVersion(value any) bool {
	text, ok := value.(string)
	return ok && text != ""
}

func fmtMillis(duration int64, reason string) string {
	return strconv.FormatInt(duration, 10) + "ms (" + reason + ")"
}

func failingHealth(checks []map[string]any, message string) map[string]any {
	return map[string]any{"healthy": false, "checks": checks, "error": message}
}

// errorBody: corpo `{error, code}` dos catches 500/409.
func (p *Pxpipe) errorBody(err error) map[string]any {
	return map[string]any{"error": err.Error(), "code": pxpipeErrorCode(err)}
}

func (p *Pxpipe) Health(w http.ResponseWriter, r *http.Request) {
	// GET = POST na origem (a carta sonda no load sem mutação). Ambos 200 OK.
	nodeReply(w, 200, p.health(r.Context()))
}

func (p *Pxpipe) Install(w http.ResponseWriter, r *http.Request) {
	info, err := p.install(r.Context())
	if err != nil {
		nodeReply(w, 500, p.errorBody(err))
		return
	}
	p.Loader.Unload() // descarta versão anterior para o health carregar a nova
	nodeReply(w, 200, map[string]any{
		"installed": info["installed"],
		"version":   info["version"],
		"path":      info["path"],
		"health":    p.health(r.Context()),
	})
}

func (p *Pxpipe) Status(w http.ResponseWriter, r *http.Request) {
	status, err := p.statusMapWithSettings(r.Context())
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": err.Error()})
		return
	}
	nodeReply(w, 200, status)
}

func (p *Pxpipe) Start(w http.ResponseWriter, r *http.Request) {
	info := pxpipeInstallInfo(p.DataDir)
	if installed, _ := info["installed"].(bool); !installed {
		_, autoInstall, _, _, err := p.settingsValues(r.Context())
		if err != nil {
			nodeReply(w, 500, map[string]any{"error": err.Error()})
			return
		}
		if !autoInstall {
			nodeReply(w, 409, map[string]any{"error": "PXPIPE is not installed", "code": "NOT_INSTALLED"})
			return
		}
		if _, installErr := p.install(r.Context()); installErr != nil {
			nodeReply(w, 500, p.errorBody(installErr))
			return
		}
	}
	if err := p.Loader.Load(r.Context()); err != nil {
		nodeReply(w, 500, p.errorBody(err))
		return
	}
	status, err := p.statusMapWithSettings(r.Context())
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": err.Error()})
		return
	}
	nodeReply(w, 200, status)
}

func (p *Pxpipe) Stop(w http.ResponseWriter, r *http.Request) {
	wasLoaded := p.Loader.Unload()
	status, err := p.statusMapWithSettings(r.Context())
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": err.Error()})
		return
	}
	out := map[string]any{"stopped": wasLoaded}
	for key, value := range status {
		if _, exists := out[key]; !exists {
			out[key] = value
		}
	}
	nodeReply(w, 200, out)
}

func (p *Pxpipe) Restart(w http.ResponseWriter, r *http.Request) {
	p.Loader.Unload()
	if err := p.Loader.Load(r.Context()); err != nil {
		nodeReply(w, 500, p.errorBody(err))
		return
	}
	status, err := p.statusMapWithSettings(r.Context())
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": err.Error()})
		return
	}
	nodeReply(w, 200, status)
}

func (p *Pxpipe) Logs(w http.ResponseWriter, r *http.Request) {
	limit := pxpipeLimit(r.URL.Query().Get("limit"))
	events := PxpipeReadEvents(p.DataDir, nil, limit)
	reversed := make([]map[string]any, 0, len(events))
	for index := len(events) - 1; index >= 0; index-- {
		reversed = append(reversed, events[index])
	}
	nodeReply(w, 200, map[string]any{
		"installLog": PxpipeInstallLogTail(p.DataDir, 200),
		"events":     reversed,
	})
}

func (p *Pxpipe) Stats(w http.ResponseWriter, r *http.Request) {
	recentLimit := pxpipeLimit(r.URL.Query().Get("limit"))
	nodeReply(w, 200, PxpipeStats(p.DataDir, 30, recentLimit))
}
