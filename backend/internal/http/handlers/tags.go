package handlers

import "net/http"

func tagsHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
}
func Tags(w http.ResponseWriter, r *http.Request) {
	tagsHeaders(w)
	nodeReply(w, 200, map[string]any{"models": []map[string]any{
		{"name": "llama3.2", "modified_at": "2025-12-26T00:00:00Z", "size": 2000000000, "digest": "abc123def456", "details": map[string]string{"format": "gguf", "family": "llama", "parameter_size": "3B", "quantization_level": "Q4_K_M"}},
		{"name": "qwen2.5", "modified_at": "2025-12-26T00:00:00Z", "size": 4000000000, "digest": "def456abc123", "details": map[string]string{"format": "gguf", "family": "qwen", "parameter_size": "7B", "quantization_level": "Q4_K_M"}},
	}})
}
func TagsOptions(w http.ResponseWriter, r *http.Request) { tagsHeaders(w); w.WriteHeader(200) }
