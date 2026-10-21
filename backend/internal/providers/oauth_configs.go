package providers

import (
	_ "embed"
	"encoding/json"
)

//go:embed oauth-configs.json
var oauthConfigs []byte

type OAuthConfig struct {
	CookieKeyURL            string            `json:"cookieKeyUrl"`
	LoadCodeAssistEndpoint  string            `json:"loadCodeAssistEndpoint"`
	OnboardUserEndpoint     string            `json:"onboardUserEndpoint"`
	LoadCodeAssistUserAgent string            `json:"loadCodeAssistUserAgent"`
	WebAppURL               string            `json:"webAppUrl"`
	ValidationURL           string            `json:"validationUrl"`
	ProfileURL              string            `json:"profileUrl"`
	MuseKeyURL              string            `json:"museKeyUrl"`
	DefaultBaseURL          string            `json:"defaultBaseUrl"`
	AuthorizeURLPath        string            `json:"authorizeUrlPath"`
	TokenURLPath            string            `json:"tokenUrlPath"`
	UserInfoURLPath         string            `json:"userInfoUrlPath"`
	LoginURL                string            `json:"loginUrl"`
	DeviceTokenURL          string            `json:"deviceTokenUrl"`
	BaseURL                 string            `json:"baseUrl"`
	LoginCodePath           string            `json:"loginCodePath"`
	LoginStatusPath         string            `json:"loginStatusPath"`
	OAuthTimeoutMS          float64           `json:"oauthTimeoutMs"`
	Referrer                string            `json:"referrer"`
	StateURL                string            `json:"stateUrl"`
	Platform                string            `json:"platform"`
	PollInterval            float64           `json:"pollInterval"`
	TokenExchangeURL        string            `json:"tokenExchangeUrl"`
	ProviderID              string            `json:"providerId"`
	CLIInitURL              string            `json:"cliInitUrl"`
	CLIPollURL              string            `json:"cliPollUrl"`
	BusinessLoginURL        string            `json:"businessLoginUrl"`
	PlanAPIKeyName          string            `json:"planApiKeyName"`
	InitiateURL             string            `json:"initiateUrl"`
	PollURLBase             string            `json:"pollUrlBase"`
	APIBaseURL              string            `json:"apiBaseUrl"`
	AuthorizeDeviceURL      string            `json:"authorizeDeviceUrl"`
	ProjectURL              string            `json:"projectUrl"`
	DeviceCodeURL           string            `json:"deviceCodeUrl"`
	CopilotTokenURL         string            `json:"copilotTokenUrl"`
	UserInfoURL             string            `json:"userInfoUrl"`
	APIVersion              string            `json:"apiVersion"`
	UserAgent               string            `json:"userAgent"`
	DeviceScope             string            `json:"deviceScope"`
	ClientID                string            `json:"clientId"`
	ClientSecret            string            `json:"clientSecret"`
	TokenURL                string            `json:"tokenUrl"`
	AuthorizeURL            string            `json:"authorizeUrl"`
	RedirectURI             string            `json:"redirectUri"`
	CodeChallengeMethod     string            `json:"codeChallengeMethod"`
	Scope                   string            `json:"scope"`
	Scopes                  []string          `json:"scopes"`
	ExtraParams             map[string]string `json:"extraParams"`
	FixedPort               int               `json:"fixedPort"`
	CallbackPath            string            `json:"callbackPath"`
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
	// xai não tem bloco oauth no registry: os valores fixos vivem em
	// constants/xai.js da origem; espelhamos 1:1 aqui (inclui o clientId da transport).
	if id == "xai" {
		return &OAuthConfig{
			ClientID:    "b1a00492-073a-47ea-816f-4c329264a828",
			Scope:       "openid profile email offline_access grok-cli:access api:access",
			FixedPort:   56121,
			RedirectURI: "http://127.0.0.1:56121/callback",
		}
	}
	return nil
}
