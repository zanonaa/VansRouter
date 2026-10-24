package handlers

// Porta de src/lib/oauth/utils/zedCredentials.js + src/app/api/oauth/zed/auto-import/route.js.
import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const (
	zedDefaultCredentialsURL = "https://zed.dev"
	zedKeyringLabel          = "zed-github-account"
)

// ZedAutoImport lê a sessão do Zed IDE do OS secret store com hooks testáveis.
type ZedAutoImport struct {
	Home         string
	XDGConfig    string
	XDGData      string
	LocalAppData string
	// Run executa comandos do OS (secret-tool/security/powershell).
	Run func(ctx context.Context, timeout time.Duration, name string, args ...string) (stdout, stderr string, err error)
	// OpenKV lê `SELECT value FROM kv_store WHERE key=?` (db hook p/ testes).
	OpenKV func(path, key string) (string, error)
	// UUID para systemId ausente.
	UUID     func() string
	Platform string
}

func NewZedAutoImport() *ZedAutoImport {
	home, _ := os.UserHomeDir()
	return &ZedAutoImport{Home: home, Platform: platformName(), XDGConfig: os.Getenv("XDG_CONFIG_HOME"), XDGData: os.Getenv("XDG_DATA_HOME"), LocalAppData: os.Getenv("LOCALAPPDATA")}
}

func platformName() string { //nolint:runtimeGOOS
	switch runtime.GOOS {
	case "darwin":
		return "darwin"
	case "windows":
		return "win32"
	}
	return "linux"
}

// zedRun default: exec.CommandContext com timeout por chamada.
func (z *ZedAutoImport) run(ctx context.Context, timeout time.Duration, name string, args ...string) (string, string, error) {
	if z.Run != nil {
		return z.Run(ctx, timeout, name, args...)
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.CommandContext(runCtx, name, args...)
	hideHeadroomWindow(command)
	var stdout, stderr strings.Builder
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return stdout.String(), stderr.String(), err
}

// resolveCredentialsURL: credentials_url|server_url do settings.json, default zed.dev.
func (z *ZedAutoImport) resolveCredentialsURL() string {
	for _, settingsPath := range z.settingsPaths() {
		raw, err := os.ReadFile(settingsPath)
		if err != nil {
			continue
		}
		var settings struct {
			CredentialsURL string `json:"credentials_url"`
			ServerURL      string `json:"server_url"`
		}
		if json.Unmarshal(raw, &settings) != nil {
			continue
		}
		url := settings.CredentialsURL
		if url == "" {
			url = settings.ServerURL
		}
		if url != "" {
			return strings.TrimRight(url, "/")
		}
	}
	return zedDefaultCredentialsURL
}

func (z *ZedAutoImport) settingsPaths() []string {
	home := z.home()
	if z.Platform == "darwin" {
		return []string{filepath.Join(home, "Library/Application Support/Zed/settings.json")}
	}
	if z.Platform == "win32" {
		local := z.LocalAppData
		if local == "" {
			local = filepath.Join(home, "AppData", "Local")
		}
		return []string{filepath.Join(local, "Zed", "settings.json")}
	}
	xdg := z.XDGConfig
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}
	return []string{filepath.Join(xdg, "zed", "settings.json")}
}

// globalDbPaths: db.sqlite por canal (system_id).
func (z *ZedAutoImport) globalDbPaths() []string {
	home := z.home()
	channels := []string{"0-global", "0-stable", "0-preview"}
	if z.Platform == "darwin" {
		base := filepath.Join(home, "Library/Application Support/Zed/db")
		out := []string{}
		for _, channel := range channels {
			out = append(out, filepath.Join(base, channel, "db.sqlite"))
		}
		return out
	}
	if z.Platform == "win32" {
		local := z.LocalAppData
		if local == "" {
			local = filepath.Join(home, "AppData", "Local")
		}
		base := filepath.Join(local, "zed", "db")
		out := []string{}
		for _, channel := range channels {
			out = append(out, filepath.Join(base, channel, "db.sqlite"))
		}
		return out
	}
	base := z.XDGData
	if base == "" {
		base = filepath.Join(home, ".local", "share")
	}
	out := []string{}
	for _, channel := range channels {
		out = append(out, filepath.Join(base, "zed", "db", channel, "db.sqlite"))
	}
	return out
}

func (z *ZedAutoImport) home() string {
	if z.Home != "" {
		return z.Home
	}
	home, _ := os.UserHomeDir()
	return home
}

// readSystemId: kv_store system_id (falha silenciosa → null).
func (z *ZedAutoImport) readSystemId() string {
	for _, dbPath := range z.globalDbPaths() {
		if _, err := os.Stat(dbPath); err != nil {
			continue
		}
		if z.OpenKV != nil {
			if value, err := z.OpenKV(dbPath, "system_id"); err == nil && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
			continue
		}
		if value := z.queryKvStoreRuntime(dbPath, "system_id"); strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// queryKvStoreRuntime: sqlite via driver moderno (modo ro).
func (z *ZedAutoImport) queryKvStoreRuntime(dbPath, key string) string {
	database, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return ""
	}
	defer database.Close()
	row := database.QueryRow("SELECT value FROM kv_store WHERE key = ? LIMIT 1", key)
	var value string
	if err := row.Scan(&value); err != nil {
		return ""
	}
	return value
}

var secretUsernamePattern = regexp.MustCompile(`attribute\.username\s*=\s*(\S+)`)
var secretLabelPattern = regexp.MustCompile(`(?m)^\s*label\s*=\s*(.+)$`)
var secretSecretPattern = regexp.MustCompile(`(?m)^\s*secret\s*=\s*(.*)$`)

type zedSecretItem struct {
	Label    string
	Username string
	Secret   string
	secret   bool
}

// parseSecretToolSearch: attribute/label/secret do texto combinado.
func parseSecretToolSearch(text string) []zedSecretItem {
	username := ""
	if match := secretUsernamePattern.FindStringSubmatch(text); match != nil {
		username = match[1]
	}
	label := ""
	if match := secretLabelPattern.FindStringSubmatch(text); match != nil {
		label = strings.TrimSpace(match[1])
	}
	secret := ""
	hasSecretLine := false
	if match := secretSecretPattern.FindStringSubmatch(text); match != nil {
		secret = strings.TrimSpace(match[1])
		hasSecretLine = true
	}
	if username == "" && !hasSecretLine && label == "" {
		return []zedSecretItem{}
	}
	return []zedSecretItem{{Label: label, Username: username, Secret: secret, secret: hasSecretLine}}
}

// parseSecurityPassword: password: "..." do macOS security (com unescape).
func parseSecurityPassword(text string) (string, bool) {
	match := regexp.MustCompile(`(?m)password:\s*"(.*)"\s*$`).FindStringSubmatch(text)
	if match != nil {
		return unescapeSecurityPassword(match[1]), true
	}
	if regexp.MustCompile(`(?m)password:\s*$`).MatchString(text) {
		return "", true
	}
	return "", false
}

func unescapeSecurityPassword(value string) string {
	// security escapa \ e " no dump entre aspas.
	out := []rune{}
	for index := 0; index < len(value); index++ {
		if value[index] == '\\' && index+1 < len(value) {
			out = append(out, rune(value[index+1]))
			index++
			continue
		}
		out = append(out, rune(value[index]))
	}
	return string(out)
}

func stripURLScheme(value string) string {
	withoutScheme := regexp.MustCompile(`(?i)^https?://`).ReplaceAllString(value, "")
	return strings.TrimRight(withoutScheme, "/")
}

type zedKeyPair struct {
	UserID      string
	AccessToken string
	Error       string
}

func (z *ZedAutoImport) readLinux(ctx context.Context, credentialsURL string) zedKeyPair {
	// secret-tool search --all: attribute.* no stderr, label/secret no stdout.
	stdout, stderr, err := z.run(ctx, 8*time.Second, "secret-tool", "search", "--all", "url", credentialsURL)
	merged := stderr + "\n" + stdout
	if err == nil {
		items := parseSecretToolSearch(merged)
		match := zedSecretItem{}
		found := false
		for _, item := range items {
			if item.Label == zedKeyringLabel && item.Username != "" && item.Secret != "" {
				match = item
				found = true
				break
			}
		}
		if !found {
			for _, item := range items {
				if item.Username != "" && item.Secret != "" {
					match = item
					found = true
					break
				}
			}
		}
		if found && match.Username != "" && match.Secret != "" {
			return zedKeyPair{UserID: match.Username, AccessToken: match.Secret}
		}
	} else {
		if strings.Contains(err.Error(), "executable file not found") || strings.Contains(err.Error(), "no such file") {
			return zedKeyPair{Error: "secret-tool not found (install libsecret / secret-tools)"}
		}
		// Versões que saem não-zero mas imprimem saída útil.
		if strings.Contains(merged, "secret =") {
			items := parseSecretToolSearch(merged)
			for _, item := range items {
				if item.Username != "" && item.Secret != "" {
					return zedKeyPair{UserID: item.Username, AccessToken: item.Secret}
				}
			}
		}
	}
	// Fallback: lookup (secret solto) + search para o username.
	lookupOut, _, lookupErr := z.run(ctx, 8*time.Second, "secret-tool", "lookup", "url", credentialsURL)
	if lookupErr == nil && strings.TrimSpace(lookupOut) != "" {
		accessToken := strings.TrimSpace(lookupOut)
		searchOut, searchErrOut, searchErr := z.run(ctx, 8*time.Second, "secret-tool", "search", "--all", "url", credentialsURL)
		if searchErr == nil {
			items := parseSecretToolSearch(searchErrOut + "\n" + searchOut)
			for _, item := range items {
				if item.Username != "" {
					return zedKeyPair{UserID: item.Username, AccessToken: accessToken}
				}
			}
		}
		return zedKeyPair{Error: "Zed keyring entry missing username (user id)"}
	}
	if lookupErr != nil && (strings.Contains(lookupErr.Error(), "executable file not found") || strings.Contains(lookupErr.Error(), "no such file")) {
		return zedKeyPair{Error: "secret-tool not found (install libsecret / secret-tools)"}
	}
	return zedKeyPair{Error: "Failed to read Linux keyring"}
}

func (z *ZedAutoImport) readMac(ctx context.Context, credentialsURL string) zedKeyPair {
	// Internet password com kSecAttrServer = credentials URL (com/sem scheme).
	servers := []string{}
	seen := map[string]bool{}
	for _, server := range []string{credentialsURL, stripURLScheme(credentialsURL), "zed.dev"} {
		if server == "" || seen[server] {
			continue
		}
		seen[server] = true
		servers = append(servers, server)
	}
	lastError := ""
	for _, server := range servers {
		stdout, stderr, err := z.run(ctx, 8*time.Second, "security", "find-internet-password", "-s", server, "-g")
		combined := stdout + "\n" + stderr
		password, hasPassword := parseSecurityPassword(combined)
		account := ""
		if match := regexp.MustCompile(`"acct"<blob>="([^"]*)"`).FindStringSubmatch(combined); match != nil {
			account = match[1]
		} else if match := regexp.MustCompile(`"acct"<blob>=(0x[0-9A-Fa-f]+|\S+)\s+"([^"]*)"`).FindStringSubmatch(combined); match != nil && len(match) > 2 {
			account = match[2]
		}
		if hasPassword && account != "" {
			return zedKeyPair{UserID: account, AccessToken: password}
		}
		if hasPassword && account == "" {
			lastError = "Found Zed keychain password but missing account (user id)"
			continue
		}
		if err != nil && lastError == "" {
			lastError = stderr
			if lastError == "" {
				lastError = err.Error()
			}
		}
	}
	if lastError == "" {
		lastError = "Zed credentials not found in macOS Keychain"
	}
	return zedKeyPair{Error: lastError}
}

// windowsScript: PowerShell embutido (CredRead zed:url=<credentialsUrl>).
func zedPowerShellScript(target string) string {
	script := `
$ErrorActionPreference = 'Stop'
Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;
using System.Text;
public class ZedCred {
  [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
  public struct CREDENTIAL {
    public uint Flags; public uint Type; public string TargetName; public string Comment;
    public System.Runtime.InteropServices.ComTypes.FILETIME LastWritten;
    public uint CredentialBlobSize; public IntPtr CredentialBlob; public uint Persist;
    public uint AttributeCount; public IntPtr Attributes; public string TargetAlias; public string UserName;
  }
  [DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
  public static extern bool CredRead(string target, uint type, uint reservedFlag, out IntPtr credentialPtr);
  [DllImport("advapi32.dll", SetLastError = true)]
  public static extern void CredFree(IntPtr buffer);
  public static string Read(string target) {
    IntPtr p;
    if (!CredRead(target, 1, 0, out p)) return null;
    try {
      var c = (CREDENTIAL)Marshal.PtrToStructure(p, typeof(CREDENTIAL));
      string secret = "";
      if (c.CredentialBlob != IntPtr.Zero && c.CredentialBlobSize > 0) {
        byte[] bytes = new byte[c.CredentialBlobSize];
        Marshal.Copy(c.CredentialBlob, bytes, 0, (int)c.CredentialBlobSize);
        secret = Encoding.UTF8.GetString(bytes);
      }
      return (c.UserName ?? "") + "\n" + secret;
    } finally { CredFree(p); }
  }
}
"@
$r = [ZedCred]::Read('` + strings.ReplaceAll(target, "'", "''") + `')
if ($null -eq $r) { exit 2 }
Write-Output $r
`
	return strings.TrimSpace(script)
}

func (z *ZedAutoImport) readWindows(ctx context.Context, credentialsURL string) zedKeyPair {
	target := "zed:url=" + credentialsURL
	stdout, stderr, err := z.run(ctx, 15*time.Second, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", zedPowerShellScript(target))
	if err != nil {
		if strings.Contains(err.Error(), "exit status 2") {
			return zedKeyPair{Error: "Zed credentials not found in Windows Credential Manager"}
		}
		if stderr != "" {
			return zedKeyPair{Error: stderr}
		}
		return zedKeyPair{Error: "Failed to read Windows credentials"}
	}
	text := strings.TrimPrefix(stdout, "\ufeff")
	newline := strings.Index(text, "\n")
	if newline < 0 {
		return zedKeyPair{Error: "Malformed Windows credential payload"}
	}
	userID := strings.TrimSpace(text[:newline])
	accessToken := strings.TrimRight(text[newline+1:], "\r\n")
	if userID == "" || accessToken == "" {
		return zedKeyPair{Error: "Windows Credential Manager entry missing username or secret"}
	}
	return zedKeyPair{UserID: userID, AccessToken: accessToken}
}

func newUUIDFallback() string {
	buffer := make([]byte, 16)
	_, _ = rand.Read(buffer)
	buffer[6] = (buffer[6] & 0x0f) | 0x40
	buffer[8] = (buffer[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s", hex.EncodeToString(buffer[0:4]), hex.EncodeToString(buffer[4:6]), hex.EncodeToString(buffer[6:8]), hex.EncodeToString(buffer[8:10]), hex.EncodeToString(buffer[10:16]))
}

// Credentials espelha readZedIdeCredentials.
func (z *ZedAutoImport) Credentials(ctx context.Context) map[string]any {
	credentialsURL := z.resolveCredentialsURL()
	var pair zedKeyPair
	switch z.Platform {
	case "linux":
		pair = z.readLinux(ctx, credentialsURL)
	case "darwin":
		pair = z.readMac(ctx, credentialsURL)
	case "win32":
		pair = z.readWindows(ctx, credentialsURL)
	default:
		return map[string]any{"found": false, "error": fmt.Sprintf("Zed auto-import is not supported on platform %s", z.Platform), "credentialsUrl": credentialsURL}
	}
	if pair.UserID == "" || pair.AccessToken == "" {
		message := pair.Error
		if message == "" {
			message = "Zed IDE session not found in the system keyring. Sign in to Zed, then retry."
		}
		return map[string]any{"found": false, "error": message, "credentialsUrl": credentialsURL}
	}
	systemID := z.readSystemId()
	if systemID == "" {
		if z.UUID != nil {
			systemID = z.UUID()
		} else {
			systemID = newUUIDFallback()
		}
	}
	return map[string]any{
		"found":          true,
		"userId":         pair.UserID,
		"accessToken":    pair.AccessToken,
		"systemId":       systemID,
		"credentialsUrl": credentialsURL,
	}
}

// Get espelha a rota.
func (z *ZedAutoImport) Get(w http.ResponseWriter, r *http.Request) {
	result := z.Credentials(r.Context())
	if found, _ := result["found"].(bool); found {
		nodeReply(w, 200, result)
		return
	}
	nodeReply(w, 200, result)
}
