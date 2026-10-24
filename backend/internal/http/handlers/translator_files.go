package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

var translatorAllowedFiles = map[string]bool{"1_req_client.json": true, "2_req_source.json": true, "3_req_openai.json": true, "4_req_target.json": true, "5_res_provider.txt": true, "6_res_openai.txt": true, "7_res_client.txt": true, "7_res_client.json": true}

type TranslatorFiles struct{ DataDir string }

func (t *TranslatorFiles) Load(w http.ResponseWriter, r *http.Request) {
	fail := func(status int, message string) {
		nodeReply(w, status, map[string]any{"success": false, "error": message})
	}
	file := r.URL.Query().Get("file")
	if file == "" {
		fail(400, "File parameter required")
		return
	}
	if !translatorAllowedFiles[file] {
		fail(400, "Invalid file name")
		return
	}
	content, err := os.ReadFile(filepath.Join(t.DataDir, "logs", "translator", file))
	if os.IsNotExist(err) {
		fail(404, "File not found")
		return
	}
	if err != nil {
		fail(500, err.Error())
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "content": string(content)})
}
func (t *TranslatorFiles) Save(w http.ResponseWriter, r *http.Request) {
	fail := func(status int, message string) {
		nodeReply(w, status, map[string]any{"success": false, "error": message})
	}
	body, err := poolInput(r)
	if err != nil {
		fail(500, err.Error())
		return
	}
	file, ok := body["file"].(string)
	content, exists := body["content"]
	if !nodeTruthy(body["file"]) || !exists {
		fail(400, "File and content required")
		return
	}
	if !ok || !translatorAllowedFiles[file] {
		fail(400, "Invalid file name")
		return
	}
	text, ok := content.(string)
	if !ok {
		fail(500, fmt.Sprintf("The \"data\" argument must be of type string or an instance of Buffer, TypedArray, or DataView. Received %s", translatorReceivedType(content)))
		return
	}
	directory := filepath.Join(t.DataDir, "logs", "translator")
	if err := os.MkdirAll(directory, 0755); err != nil {
		fail(500, err.Error())
		return
	}
	if err := os.WriteFile(filepath.Join(directory, file), []byte(text), 0644); err != nil {
		fail(500, err.Error())
		return
	}
	nodeReply(w, 200, map[string]any{"success": true})
}
func translatorReceivedType(value any) string {
	if value == nil {
		return "null"
	}
	switch value.(type) {
	case bool:
		return fmt.Sprintf("type boolean (%v)", value)
	case float64:
		return fmt.Sprintf("type number (%v)", value)
	case []any:
		return "an instance of Array"
	default:
		return "an instance of Object"
	}
}
