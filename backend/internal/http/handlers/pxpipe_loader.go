package handlers

// Porta de src/lib/pxpipe/loader.js. Divergência registrada: a origem importa
// o módulo ESM do pxpipe in-process (library mode); em Go o mesmo módulo é
// executado via subprocesso `node -e` por operação (load check e self-test),
// com cache de versão/loadedAt mantido in-process.
import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// PxpipeLoader guarda o estado "library mode": started/stopped via start/stop.
type PxpipeLoader struct {
	DataDir string

	// RunScript hook para testes; default roda `node -e` com PATH estendido.
	RunScript func(ctx context.Context, script string) (string, error)
	// RunScriptInput hook para testes do transform (corpo via stdin).
	RunScriptInput func(ctx context.Context, script string, stdin []byte) (string, error)
	// Timeout por execução do subprocesso.
	Timeout time.Duration

	mu       sync.Mutex
	version  string
	loadedAt int64
}

type pxpipeLoadedInfo struct {
	Loaded   bool
	Version  string
	LoadedAt int64
}

func (l *PxpipeLoader) info() pxpipeLoadedInfo {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.loadedAt == 0 {
		return pxpipeLoadedInfo{Loaded: false}
	}
	return pxpipeLoadedInfo{Loaded: true, Version: l.version, LoadedAt: l.loadedAt}
}

// PxpipeRunScript real: node -e com PATH estendido, sem janela no Windows.
func PxpipeRunScript(ctx context.Context, script string) (string, error) {
	return pxpipeRunNode(ctx, script, nil)
}

// PxpipeRunScriptInput igual, mas envia o corpo binário por stdin.
func PxpipeRunScriptInput(ctx context.Context, script string, stdin []byte) (string, error) {
	return pxpipeRunNode(ctx, script, stdin)
}

func pxpipeRunNode(ctx context.Context, script string, stdin []byte) (string, error) {
	node := "node"
	if runtime.GOOS == "windows" {
		node = "node.exe"
	}
	cmd := exec.CommandContext(ctx, node, "-e", script)
	cmd.Env = append(os.Environ(), "PATH="+pxpipeExtendedPath())
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	hideHeadroomWindow(cmd)
	out, err := cmd.Output()
	return string(out), err
}

func (l *PxpipeLoader) run(ctx context.Context, script string) (string, error) {
	return l.runWithInput(ctx, script, nil)
}

func (l *PxpipeLoader) runWithInput(ctx context.Context, script string, stdin []byte) (string, error) {
	timeout := l.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if stdin != nil {
		run := l.RunScriptInput
		if run == nil {
			run = PxpipeRunScriptInput
		}
		return run(runCtx, script, stdin)
	}
	run := l.RunScript
	if run == nil {
		run = PxpipeRunScript
	}
	return run(runCtx, script)
}

// pxpipeExportScript: cache-bust por versão (idem `?v=` da origem); a versão
// entra embutida no script.
func pxpipeExportScript(entry, version string) string {
	return fmt.Sprintf("import(%s + '?v=' + encodeURIComponent(%s)).then(m => { if (typeof m.transformAnthropicMessages !== 'function') process.exit(1); process.stdout.write('function') }).catch(e => { process.stderr.write(String(e && e.message || e)); process.exit(1) })", strconvEntry(entry), strconvEntry(version))
}

// Load espelha loadPxpipe: NOT_INSTALLED quando falta o pacote; erro quando o
// módulo não exporta transformAnthropicMessages. Sucesso: cache version+data.
func (l *PxpipeLoader) Load(ctx context.Context) error {
	info := pxpipeInstallInfo(l.DataDir)
	if installed, _ := info["installed"].(bool); !installed {
		return &pxpipeError{Code: "NOT_INSTALLED", Msg: "PXPIPE is not installed"}
	}
	version, _ := info["version"].(string)
	if version == "" {
		version = "0"
	}
	script := pxpipeExportScript(pxpipeAbsEntry(l.DataDir), version)
	if _, runErr := l.run(ctx, script); runErr != nil {
		return fmt.Errorf("installed pxpipe package does not export transformAnthropicMessages")
	}
	l.mu.Lock()
	l.version = version
	l.loadedAt = time.Now().UnixMilli()
	l.mu.Unlock()
	return nil
}

// Unload espelha unloadPxpipe.
func (l *PxpipeLoader) Unload() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	was := l.loadedAt != 0
	l.loadedAt = 0
	l.version = ""
	return was
}

// SelfTest espelha selfTest: ping sintético pelo transformador real.
func (l *PxpipeLoader) SelfTest(ctx context.Context) (map[string]any, error) {
	startedAt := time.Now()
	info := pxpipeInstallInfo(l.DataDir)
	if installed, _ := info["installed"].(bool); !installed {
		return nil, &pxpipeError{Code: "NOT_INSTALLED", Msg: "PXPIPE is not installed"}
	}
	version, _ := info["version"].(string)
	if version == "" {
		version = "0"
	}
	inline := `{"model":"claude-fable-5","max_tokens":16,"messages":[{"role":"user","content":"ping"}]}`
	script := fmt.Sprintf(`
const body = new TextEncoder().encode(%s);
import(%s + '?v=' + encodeURIComponent(%s)).then(async (m) => {
  if (typeof m.transformAnthropicMessages !== "function") {
    process.stderr.write("installed pxpipe package does not export transformAnthropicMessages");
    process.exit(1);
  }
  const result = await m.transformAnthropicMessages({ body, model: "claude-fable-5" });
  if (!result || typeof result.applied !== "boolean" || !(result.body instanceof Uint8Array)) {
    process.stderr.write("transform returned an unexpected shape");
    process.exit(1);
  }
  process.stdout.write(JSON.stringify({ reason: result.reason }));
}).catch(e => { process.stderr.write(String(e && e.message || e)); process.exit(1) });
`, strconvEntry(inline), strconvEntry(pxpipeAbsEntry(l.DataDir)), strconvEntry(version))
	out, runErr := l.run(ctx, script)
	if runErr != nil {
		return nil, runErr
	}
	var parsed struct {
		Reason string `json:"reason"`
	}
	if json.Unmarshal([]byte(out), &parsed) != nil {
		return nil, fmt.Errorf("transform returned an unexpected shape")
	}
	if parsed.Reason == "parse_error" {
		return nil, fmt.Errorf("PXPIPE self-test rejected the JSON fixture")
	}
	return map[string]any{
		"ok":         true,
		"reason":     parsed.Reason,
		"durationMs": time.Since(startedAt).Milliseconds(),
	}, nil
}

// pxpipeTransformed reproduz o resultado de transformAnthropicMessages.
type pxpipeTransformed struct {
	Applied bool
	Reason  string
	Detail  string
	Body    []byte
	Info    map[string]any
	Cache   map[string]any
}

// Transform porta a chamada real ao módulo pxpipe (library mode) via
// subprocesso Node. O corpo trafega por stdin e volta em base64, evitando
// limites de linha de comando e problemas de encoding.
func (l *PxpipeLoader) Transform(ctx context.Context, body []byte, model string, minChars int) (*pxpipeTransformed, error) {
	info := pxpipeInstallInfo(l.DataDir)
	if installed, _ := info["installed"].(bool); !installed {
		return nil, &pxpipeError{Code: "NOT_INSTALLED", Msg: "PXPIPE is not installed"}
	}
	version, _ := info["version"].(string)
	if version == "" {
		version = "0"
	}
	script := fmt.Sprintf(`
const chunks = [];
process.stdin.on("data", (chunk) => chunks.push(chunk));
process.stdin.on("end", () => {
  const body = new Uint8Array(Buffer.concat(chunks));
  import(%s + "?v=" + encodeURIComponent(%s)).then(async (m) => {
    if (typeof m.transformAnthropicMessages !== "function") {
      process.stderr.write("installed pxpipe package does not export transformAnthropicMessages");
      process.exit(1);
    }
    const result = await m.transformAnthropicMessages({ body, model: %s, options: { minCompressChars: %d } });
    if (!result) {
      process.stdout.write(JSON.stringify({ applied: false, reason: "passthrough" }));
      return;
    }
    const out = {
      applied: !!result.applied,
      reason: result.reason,
      detail: result.detail,
      info: result.info || {},
      cache: result.cache || {},
    };
    if (result.body) out.body = Buffer.from(result.body).toString("base64");
    process.stdout.write(JSON.stringify(out));
  }).catch((e) => { process.stderr.write(String((e && e.message) || e)); process.exit(1); });
});
`, strconvEntry(pxpipeAbsEntry(l.DataDir)), strconvEntry(version), strconvEntry(model), minChars)
	out, runErr := l.runWithInput(ctx, script, body)
	if runErr != nil {
		return nil, runErr
	}
	var parsed struct {
		Applied bool           `json:"applied"`
		Reason  string         `json:"reason"`
		Detail  string         `json:"detail"`
		Body    string         `json:"body"`
		Info    map[string]any `json:"info"`
		Cache   map[string]any `json:"cache"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return nil, fmt.Errorf("transform returned an unexpected shape: %w", err)
	}
	transformed := &pxpipeTransformed{
		Applied: parsed.Applied,
		Reason:  parsed.Reason,
		Detail:  parsed.Detail,
		Info:    parsed.Info,
		Cache:   parsed.Cache,
	}
	if parsed.Applied {
		if parsed.Body == "" {
			return nil, fmt.Errorf("transform returned applied without body")
		}
		decoded, err := base64.StdEncoding.DecodeString(parsed.Body)
		if err != nil {
			return nil, fmt.Errorf("transform returned invalid body: %w", err)
		}
		transformed.Body = decoded
	}
	return transformed, nil
}

func strconvEntry(value string) string {
	// JSON escape simples: aspas e barras invertidas.
	out := make([]byte, 0, len(value)+2)
	out = append(out, '"')
	for index := 0; index < len(value); index++ {
		ch := value[index]
		if ch == '"' || ch == '\\' {
			out = append(out, '\\')
		}
		out = append(out, ch)
	}
	out = append(out, '"')
	return string(out)
}

// pxpipeAbsEntry resolve o caminho absoluto com barras normais para URL file.
func pxpipeAbsEntry(dataDir string) string {
	entry, err := filepath.Abs(pxpipeLibraryEntry(dataDir))
	if err != nil {
		entry = pxpipeLibraryEntry(dataDir)
	}
	return filepath.ToSlash(entry)
}
