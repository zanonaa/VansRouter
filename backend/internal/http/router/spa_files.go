package router

import (
	"fmt"
	"io"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type spaFile struct {
	data    []byte
	modTime time.Time
}

const spaMaxFileBytes = 64 << 20

// readSPAFile lê um arquivo do build da SPA com proteção contra path traversal.
func readSPAFile(root, name string) (spaFile, error) {
	clean, err := secureSPAPath(root, name)
	if err != nil {
		return spaFile{}, err
	}
	info, err := os.Stat(clean)
	if err != nil || info.IsDir() {
		return spaFile{}, fmt.Errorf("arquivo não encontrado: %s", name)
	}
	handle, err := os.Open(clean)
	if err != nil {
		return spaFile{}, err
	}
	defer handle.Close()
	data, err := io.ReadAll(io.LimitReader(handle, spaMaxFileBytes))
	if err != nil {
		return spaFile{}, err
	}
	return spaFile{data: data, modTime: info.ModTime()}, nil
}

// lookupSPAFile resolve um caminho de URL para um arquivo público do build.
func lookupSPAFile(root, urlPath string) (spaFile, bool) {
	if !strings.HasPrefix(urlPath, "/") || strings.Contains(urlPath, "..") {
		return spaFile{}, false
	}
	file, err := readSPAFile(root, strings.TrimPrefix(path.Clean(urlPath), "/"))
	if err != nil {
		return spaFile{}, false
	}
	return file, true
}

func secureSPAPath(root, name string) (string, error) {
	clean := path.Clean("/" + name)
	joined := filepath.Join(root, filepath.FromSlash(clean))
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	if abs != absRoot && !strings.HasPrefix(abs, absRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("caminho fora do diretório da SPA")
	}
	return abs, nil
}

// spaContentType mantém tipos essenciais mesmo sem mime.types do sistema.
func init() {
	for ext, ctype := range map[string]string{
		".js":    "text/javascript; charset=utf-8",
		".mjs":   "text/javascript; charset=utf-8",
		".css":   "text/css; charset=utf-8",
		".json":  "application/json; charset=utf-8",
		".svg":   "image/svg+xml",
		".woff2": "font/woff2",
		".webp":  "image/webp",
		".png":   "image/png",
		".map":   "application/json; charset=utf-8",
	} {
		_ = mime.AddExtensionType(ext, ctype)
	}
}
