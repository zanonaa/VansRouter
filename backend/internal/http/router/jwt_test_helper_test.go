package router

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

func buildFakeJWT(claims map[string]any) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, _ := json.Marshal(claims)
	body := base64.RawURLEncoding.EncodeToString(payload)
	return strings.Join([]string{header, body, "fictitious-signature"}, ".")
}
