package handlers

// Porta de src/app/api/oauth/kiro/auto-import/route.js (somente filesystem).
import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// KiroAutoImport lê ~/.aws/sso/cache + profile.json do Kiro IDE.
type KiroAutoImport struct {
	Home string
	// AppData para o profile.json no Windows (default segue a origem).
	AppData string
}

func NewKiroAutoImport() *KiroAutoImport {
	home, _ := os.UserHomeDir()
	return &KiroAutoImport{Home: home}
}

func (k *KiroAutoImport) home() string {
	if k.Home != "" {
		return k.Home
	}
	home, _ := os.UserHomeDir()
	return home
}

var kiroTokenPrefix = "aorAAAAAG"

// kiroProfilePaths espelha os candidatos do profile.json (com normalização us-east-1 do ARN).
func (k *KiroAutoImport) profilePaths() []string {
	home := k.home()
	appData := k.AppData
	if appData == "" {
		appData = os.Getenv("APPDATA")
	}
	if appData == "" {
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	return []string{
		filepath.Join(appData, "Kiro", "User", "globalStorage", "kiro.kiroagent", "profile.json"),
		filepath.Join(home, ".config", "Kiro", "User", "globalStorage", "kiro.kiroagent", "profile.json"),
	}
}

// Get espelha a rota.
func (k *KiroAutoImport) Get(w http.ResponseWriter, r *http.Request) {
	cachePath := filepath.Join(k.home(), ".aws", "sso", "cache")
	entries, readErr := os.ReadDir(cachePath)
	if readErr != nil {
		nodeReply(w, 200, map[string]any{
			"found": false,
			"error": "AWS SSO cache not found. Please login to Kiro IDE first.",
		})
		return
	}
	refreshToken := ""
	foundFile := ""
	var tokenData map[string]any
	// kiro-auth-token.json primeiro.
	paths := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			if entry.Name() == "kiro-auth-token.json" {
				paths = append([]string{entry.Name()}, paths...)
			} else {
				paths = append(paths, entry.Name())
			}
		}
	}
	for _, file := range paths {
		raw, err := os.ReadFile(filepath.Join(cachePath, file))
		if err != nil {
			continue
		}
		var data map[string]any
		if json.Unmarshal(raw, &data) != nil {
			continue
		}
		if token, ok := data["refreshToken"].(string); ok && strings.HasPrefix(token, kiroTokenPrefix) {
			refreshToken = token
			foundFile = file
			tokenData = data
			break
		}
	}
	if refreshToken == "" {
		nodeReply(w, 200, map[string]any{
			"found": false,
			"error": "Kiro token not found in AWS SSO cache. Please login to Kiro IDE first.",
		})
		return
	}
	clientID, clientSecret := "", ""
	if clientIDHash, _ := tokenData["clientIdHash"].(string); clientIDHash != "" {
		clientRaw, err := os.ReadFile(filepath.Join(cachePath, clientIDHash+".json"))
		if err == nil {
			var clientData struct {
				ClientID     string `json:"clientId"`
				ClientSecret string `json:"clientSecret"`
			}
			if json.Unmarshal(clientRaw, &clientData) == nil && clientData.ClientID != "" && clientData.ClientSecret != "" {
				clientID = clientData.ClientID
				clientSecret = clientData.ClientSecret
			}
		}
	}
	profileArn := ""
	for _, profilePath := range k.profilePaths() {
		profileRaw, err := os.ReadFile(profilePath)
		if err != nil {
			continue
		}
		var profileData struct {
			Arn string `json:"arn"`
		}
		if json.Unmarshal(profileRaw, &profileData) != nil || profileData.Arn == "" {
			continue
		}
		// Runtime usa us-east-1 no ARN independente da região IDC.
		profileArn = kiroNormalizeArn(profileData.Arn)
		break
	}
	region, _ := tokenData["region"].(string)
	authMethod, _ := tokenData["authMethod"].(string)
	out := map[string]any{
		"found":        true,
		"refreshToken": refreshToken,
		"source":       foundFile,
		"clientId":     nilOrNilStringValue(clientID),
		"clientSecret": nilOrNilStringValue(clientSecret),
		"region":       nilOrNilStringValue(region),
		"authMethod":   nilOrNilStringValue(authMethod),
		"profileArn":   nilOrNilStringValue(profileArn),
	}
	nodeReply(w, 200, out)
}

// kiroNormalizeArn: arn:aws:codewhisperer:<região>: → us-east-1.
func kiroNormalizeArn(arn string) string {
	parts := strings.Split(arn, ":")
	if len(parts) >= 5 && parts[0] == "arn" && parts[1] == "aws" && parts[2] == "codewhisperer" && parts[3] != "" {
		parts[3] = "us-east-1"
		return strings.Join(parts, ":")
	}
	return arn
}
