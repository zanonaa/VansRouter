package providers

import (
	_ "embed"
	"encoding/json"
)

//go:embed oauth-configs.json
var oauthConfigs []byte

type OAuthConfig struct {
	ProviderID          string            `json:"providerId"`
	CLIInitURL          string            `json:"cliInitUrl"`
	CLIPollURL          string            `json:"cliPollUrl"`
	BusinessLoginURL    string            `json:"businessLoginUrl"`
	PlanAPIKeyName      string            `json:"planApiKeyName"`
	InitiateURL         string            `json:"initiateUrl"`
	PollURLBase         string            `json:"pollUrlBase"`
	APIBaseURL          string            `json:"apiBaseUrl"`
	AuthorizeDeviceURL  string            `json:"authorizeDeviceUrl"`
	ProjectURL          string            `json:"projectUrl"`
	DeviceCodeURL       string            `json:"deviceCodeUrl"`
	CopilotTokenURL     string            `json:"copilotTokenUrl"`
	UserInfoURL         string            `json:"userInfoUrl"`
	APIVersion          string            `json:"apiVersion"`
	UserAgent           string            `json:"userAgent"`
	DeviceScope         string            `json:"deviceScope"`
	ClientID            string            `json:"clientId"`
	ClientSecret        string            `json:"clientSecret"`
	TokenURL            string            `json:"tokenUrl"`
	AuthorizeURL        string            `json:"authorizeUrl"`
	RedirectURI         string            `json:"redirectUri"`
	CodeChallengeMethod string            `json:"codeChallengeMethod"`
	Scope               string            `json:"scope"`
	Scopes              []string          `json:"scopes"`
	ExtraParams         map[string]string `json:"extraParams"`
	FixedPort           int               `json:"fixedPort"`
	CallbackPath        string            `json:"callbackPath"`
}

func OAuthConfigs() map[string]OAuthConfig {
	var all map[string]OAuthConfig
	if err := json.Unmarshal(oauthConfigs, &all); err != nil {
		panic(err)
	}
	return all
}
func GetOAuthConfig(id string) *OAuthConfig {
	if c, ok := OAuthConfigs()[id]; ok {
		return &c
	}
	return nil
}
