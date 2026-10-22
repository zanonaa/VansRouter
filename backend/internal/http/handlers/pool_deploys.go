package handlers

// Porta de src/app/api/proxy-pools/{cloudflare-deploy,deno-deploy,vercel-deploy}/route.js.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// relayTargetGuardSource: JS embutido nos relays (ssrfGuard.js), sem dependências.
const relayTargetGuardSource = `function assertTrustedTarget(rawUrl) {
  const parsed = new URL(rawUrl);
  const host = parsed.hostname.toLowerCase();
  if (!["http:", "https:"].includes(parsed.protocol) || parsed.username || parsed.password) {
    throw new Error("Blocked URL: unsupported target");
  }
  if (["localhost", "ip6-localhost", "ip6-loopback"].includes(host) ||
      host.endsWith(".internal") || host.endsWith(".local") || host.endsWith(".localhost")) {
    throw new Error("Blocked URL: internal host");
  }
  const ipv4 = host.split(".");
  if (ipv4.length === 4 && ipv4.every((part) => /^\d{1,3}$/.test(part) && Number(part) <= 255)) {
    const ip = ipv4.reduce((value, part) => value * 256 + Number(part), 0) >>> 0;
    const blocked = [[0, 8], [167772160, 8], [2130706432, 8], [2851995648, 16], [2852039168, 12], [2886729728, 12], [3232235520, 16], [1681915904, 10], [3758096384, 4], [4026531840, 4]];
    if (blocked.some(([base, bits]) => (ip & ((0xffffffff << (32 - bits)) >>> 0)) === (base & ((0xffffffff << (32 - bits)) >>> 0)))) {
      throw new Error("Blocked URL: private IP");
    }
  }
  const ipv6 = host.replace(/^\[|\]$/g, "").toLowerCase();
  if (ipv6 === "::" || ipv6 === "::1" || ipv6.startsWith("::ffff:") || ipv6.startsWith("fe80:") || ipv6.startsWith("fc") || ipv6.startsWith("fd")) {
    throw new Error("Blocked URL: private IP");
  }
}`

// cloudflareRelayWorkerCode: worker com guard embutido (idem template JS).
const cloudflareRelayWorkerCode = `
` + relayTargetGuardSource + `
export default {
  async fetch(request, env, ctx) {
    const target = request.headers.get("x-relay-target");
    const relayPath = request.headers.get("x-relay-path") || "/";

    if (!target) {
      return new Response(JSON.stringify({ error: "Missing x-relay-target header" }), {
        status: 400,
        headers: { "content-type": "application/json" },
      });
    }

    let targetUrl;
    try {
      const base = target.replace(/\/+$/, "");
      const path = relayPath.startsWith("/") ? relayPath : "/" + relayPath;
      targetUrl = base + path;
      assertTrustedTarget(targetUrl);
    } catch (error) {
      return new Response(JSON.stringify({ error: error.message }), { status: 400, headers: { "content-type": "application/json" } });
    }
    const newRequestInit = {
      method: request.method,
      headers: new Headers(request.headers),
    };

    if (request.method !== "GET" && request.method !== "HEAD") {
      newRequestInit.body = request.body;
      newRequestInit.duplex = "half";
    }

    newRequestInit.headers.delete("x-relay-target");
    newRequestInit.headers.delete("x-relay-path");
    newRequestInit.headers.delete("host");

    try {
      const response = await fetch(targetUrl, newRequestInit);
      return new Response(response.body, {
        status: response.status,
        headers: response.headers,
      });
    } catch (error) {
      return new Response(JSON.stringify({ error: error.message }), {
        status: 502,
        headers: { "content-type": "application/json" },
      });
    }
  },
};
`

const denoRelayCode = `Deno.serve(async (request) => {
  ` + relayTargetGuardSource + `
  const target = request.headers.get("x-relay-target");
  const relayPath = request.headers.get("x-relay-path") || "/";

  if (!target) {
    return new Response(JSON.stringify({ error: "Missing x-relay-target header" }), {
      status: 400,
      headers: { "content-type": "application/json" },
    });
  }

  let targetUrl;
  try {
    const base = target.replace(/\/+$/, "");
    const path = relayPath.startsWith("/") ? relayPath : "/" + relayPath;
    targetUrl = base + path;
    assertTrustedTarget(targetUrl);
  } catch (error) {
    return new Response(JSON.stringify({ error: error.message }), { status: 400, headers: { "content-type": "application/json" } });
  }
  const newHeaders = new Headers(request.headers);
  newHeaders.delete("x-relay-target");
  newHeaders.delete("x-relay-path");
  newHeaders.delete("host");

  const init = {
    method: request.method,
    headers: newHeaders,
  };

  if (request.method !== "GET" && request.method !== "HEAD") {
    init.body = request.body;
    init.duplex = "half";
  }

  try {
    const response = await fetch(targetUrl, init);
    return new Response(response.body, {
      status: response.status,
      headers: response.headers,
    });
  } catch (error) {
    return new Response(JSON.stringify({ error: error.message }), {
      status: 502,
      headers: { "content-type": "application/json" },
    });
  }
});`

const vercelRelayFunctionCode = `
` + relayTargetGuardSource + `
export const config = { runtime: "edge" };

export default async function handler(req) {
  const target = req.headers.get("x-relay-target");
  const relayPath = req.headers.get("x-relay-path") || "/";
  if (!target) {
    return new Response(JSON.stringify({ error: "Missing x-relay-target header" }), {
      status: 400,
      headers: { "content-type": "application/json" },
    });
  }

  let targetUrl;
  try {
    const base = target.replace(/\/+$/, "");
    const path = relayPath.startsWith("/") ? relayPath : "/" + relayPath;
    targetUrl = base + path;
    assertTrustedTarget(targetUrl);
  } catch (error) {
    return new Response(JSON.stringify({ error: error.message }), { status: 400, headers: { "content-type": "application/json" } });
  }

  const rawHeaders = {};
  for (const [key, value] of req.headers.entries()) rawHeaders[key] = value;
  delete rawHeaders["x-relay-target"];
  delete rawHeaders["x-relay-path"];
  delete rawHeaders["host"];

  const response = await fetch(targetUrl, {
    method: req.method,
    headers: rawHeaders,
    body: req.method !== "GET" && req.method !== "HEAD" ? req.body : undefined,
    duplex: "half",
  });

  return new Response(response.body, {
    status: response.status,
    headers: response.headers,
  });
}
`

// deployHTTP: cliente único com hook de teste.
type deployHTTP struct {
	Do func(request *http.Request) (*http.Response, error)
}

func (d *deployHTTP) client() deployClient {
	if d.Do != nil {
		return directDo(d.Do)
	}
	return http.DefaultClient
}

// deployClient: assinatura client-like (http.DefaultClient ou stub).
type deployClient interface {
	Do(request *http.Request) (*http.Response, error)
}

func directDo(fn func(*http.Request) (*http.Response, error)) deployClient {
	return stubDoer{fn}
}

type stubDoer struct {
	fn func(*http.Request) (*http.Response, error)
}

func (s stubDoer) Do(request *http.Request) (*http.Response, error) { return s.fn(request) }

// deployResponseTextBody com limite de leitura.
func deployReadAll(response *http.Response) []byte {
	if response == nil || response.Body == nil {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	return body
}

// deployResponse: resposta simples em memória para stubs.
func deployResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}
}

// deployJSONBody devolve os bytes do JS JSON.stringify.
func deployJSONBody(value map[string]any) []byte {
	encoded, _ := json.Marshal(value)
	return encoded
}

func deployNewBody(data []byte) io.Reader {
	return bytes.NewReader(data)
}

func trimmedDeployString(body map[string]any, key string) string {
	if raw, ok := body[key].(string); ok {
		return strings.TrimSpace(raw)
	}
	return ""
}

func deployProjectName(body map[string]any) string {
	if name := trimmedDeployString(body, "projectName"); name != "" {
		return name
	}
	return "relay-" + strconv.FormatInt(time.Now().UnixMilli(), 36)
}

// createRelayPool cria a entrada do pool do tipo.
func (s *RelayDeploys) createRelayPool(r *http.Request, name, proxyURL, kind string) (map[string]any, error) {
	return s.Store.Create(r.Context(), map[string]any{
		"name":        name,
		"proxyUrl":    proxyURL,
		"type":        kind,
		"noProxy":     "",
		"isActive":    true,
		"strictProxy": false,
	})
}

// RelayDeploys: deploys cloudflare/deno/vercel com Store das pools.
type RelayDeploys struct {
	Store PoolStore
	HTTP  *deployHTTP
}

func NewRelayDeploys(store PoolStore) *RelayDeploys {
	return &RelayDeploys{Store: store, HTTP: &deployHTTP{}}
}

func (s *RelayDeploys) doer() *deployHTTP {
	if s.HTTP != nil {
		return s.HTTP
	}
	return &deployHTTP{}
}

// ── Cloudflare ──────────────────────────────────────────────────────────────

func (s *RelayDeploys) CloudflareDeploy(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil || body == nil {
		body = map[string]any{}
	}
	accountID := trimmedDeployString(body, "accountId")
	apiToken := trimmedDeployString(body, "apiToken")
	projectName := deployProjectName(body)
	if accountID == "" || apiToken == "" {
		nodeReply(w, 400, map[string]any{"error": "Cloudflare Account ID and API Token are required"})
		return
	}
	// 1. PUT multipart do worker.
	workerURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/workers/scripts/%s", accountID, projectName)
	payload := &bytes.Buffer{}
	multipartWriter := multipart.NewWriter(payload)
	part, _ := multipartWriter.CreateFormFile("index.js", "index.js")
	_, _ = part.Write([]byte(cloudflareRelayWorkerCode))
	metadata, _ := multipartWriter.CreateFormFile("metadata", "metadata.json")
	_, _ = metadata.Write(deployJSONBody(map[string]any{
		"main_module":        "index.js",
		"compatibility_date": "2024-03-20",
		"observability":      map[string]any{"enabled": true},
	}))
	_ = multipartWriter.Close()
	uploadRequest, _ := http.NewRequestWithContext(r.Context(), "PUT", workerURL, payload)
	uploadRequest.Header.Set("Authorization", "Bearer "+apiToken)
	uploadRequest.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	uploadResponse, err := s.doer().client().Do(uploadRequest)
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": errFieldOrDefault(err, "Deploy failed")})
		return
	}
	if uploadResponse.StatusCode < 200 || uploadResponse.StatusCode >= 300 {
		var errBody struct {
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		_ = json.Unmarshal(deployReadAll(uploadResponse), &errBody)
		message := "Failed to upload Worker to Cloudflare"
		if len(errBody.Errors) > 0 && errBody.Errors[0].Message != "" {
			message = errBody.Errors[0].Message
		}
		nodeReply(w, uploadResponse.StatusCode, map[string]any{"error": message})
		return
	}
	// 2. Habilita subdomain workers.dev (best effort).
	enableRequest, _ := http.NewRequestWithContext(r.Context(), "POST",
		fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/workers/scripts/%s/subdomain", accountID, projectName),
		deployNewBody(deployJSONBody(map[string]any{"enabled": true})))
	enableRequest.Header.Set("Authorization", "Bearer "+apiToken)
	enableRequest.Header.Set("Content-Type", "application/json")
	if enableResponse, enableErr := s.doer().client().Do(enableRequest); enableErr == nil {
		_ = deployReadAll(enableResponse)
	}
	// 3. Subdomain da conta → deployUrl.
	subdomainRequest, _ := http.NewRequestWithContext(r.Context(), "GET",
		fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/workers/subdomain", accountID), nil)
	subdomainRequest.Header.Set("Authorization", "Bearer "+apiToken)
	deployURL := ""
	if subdomainResponse, err := s.doer().client().Do(subdomainRequest); err == nil && subdomainResponse.StatusCode == 200 {
		var data struct {
			Result struct {
				Subdomain string `json:"subdomain"`
			} `json:"result"`
		}
		if json.Unmarshal(deployReadAll(subdomainResponse), &data) == nil && data.Result.Subdomain != "" {
			deployURL = fmt.Sprintf("https://%s.%s.workers.dev", projectName, data.Result.Subdomain)
		}
	}
	if deployURL == "" {
		nodeReply(w, 400, map[string]any{"error": "Worker deployed but failed to retrieve workers.dev subdomain. Make sure you have setup a workers.dev subdomain in Cloudflare Dashboard."})
		return
	}
	proxyPool, createErr := s.createRelayPool(r, projectName, deployURL, "cloudflare")
	if createErr != nil {
		nodeReply(w, 500, map[string]any{"error": devErrorString(createErr)})
		return
	}
	nodeReply(w, 201, map[string]any{"proxyPool": proxyPool, "deployUrl": deployURL})
}

func errFieldOrDefault(err error, fallback string) string {
	if err.Error() != "" {
		return err.Error()
	}
	return fallback
}

func devErrorString(err error) string {
	if err != nil && err.Error() != "" {
		return err.Error()
	}
	return "Deploy failed"
}

// ── Deno Deploy ─────────────────────────────────────────────────────────────

const denoV2API = "https://api.deno.com/v2"

func (s *RelayDeploys) DenoDeploy(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil || body == nil {
		body = map[string]any{}
	}
	denoToken := trimmedDeployString(body, "denoToken")
	orgDomain := trimmedDeployString(body, "orgDomain")
	projectName := deployProjectName(body)
	if orgDomain == "" {
		nodeReply(w, 400, map[string]any{"error": "Organization domain is required"})
		return
	}
	if denoToken == "" {
		nodeReply(w, 400, map[string]any{"error": "Deno Deploy API token is required"})
		return
	}
	createAppRequest, _ := http.NewRequestWithContext(r.Context(), "POST", denoV2API+"/apps", deployNewBody(deployJSONBody(map[string]any{
		"slug":   projectName,
		"labels": map[string]any{"custom.kind": "9router-relay"},
		"config": map[string]any{
			"install": "deno install",
			"runtime": map[string]any{"type": "dynamic", "entrypoint": "main.ts"},
		},
	})))
	createAppRequest.Header.Set("Authorization", "Bearer "+denoToken)
	createAppRequest.Header.Set("Content-Type", "application/json")
	createAppResponse, err := s.doer().client().Do(createAppRequest)
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": errFieldOrDefault(err, "Deploy failed")})
		return
	}
	if createAppResponse.StatusCode == 409 {
		nodeReply(w, 409, map[string]any{"error": fmt.Sprintf("App %q already exists. Choose a different name.", projectName)})
		return
	}
	if createAppResponse.StatusCode < 200 || createAppResponse.StatusCode >= 300 {
		text := string(deployReadAll(createAppResponse))
		nodeReply(w, createAppResponse.StatusCode, map[string]any{"error": fmt.Sprintf("Failed to create app (%d): %s", createAppResponse.StatusCode, text)})
		return
	}
	var app struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(deployReadAll(createAppResponse), &app) != nil || app.ID == "" {
		nodeReply(w, 500, map[string]any{"error": "Deploy failed"})
		return
	}
	deployRequest, _ := http.NewRequestWithContext(r.Context(), "POST", fmt.Sprintf("%s/apps/%s/deploy", denoV2API, app.ID), deployNewBody(deployJSONBody(map[string]any{
		"assets": map[string]any{
			"main.ts": map[string]any{"kind": "file", "content": denoRelayCode, "encoding": "utf-8"},
		},
	})))
	deployRequest.Header.Set("Authorization", "Bearer "+denoToken)
	deployRequest.Header.Set("Content-Type", "application/json")
	deployResponse, deployErr := s.doer().client().Do(deployRequest)
	if deployErr != nil {
		nodeReply(w, 500, map[string]any{"error": errFieldOrDefault(deployErr, "Deploy failed")})
		return
	}
	if deployResponse.StatusCode < 200 || deployResponse.StatusCode >= 300 {
		text := string(deployReadAll(deployResponse))
		s.denoDeleteApp(r, denoToken, app.ID)
		nodeReply(w, deployResponse.StatusCode, map[string]any{"error": fmt.Sprintf("Deploy failed (%d): %s", deployResponse.StatusCode, text)})
		return
	}
	var revision struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	_ = json.Unmarshal(deployReadAll(deployResponse), &revision)
	status := revision.Status
	deadline := time.Now().Add(60 * time.Second)
	for status == "queued" || status == "building" {
		if time.Now().After(deadline) {
			s.denoDeleteApp(r, denoToken, app.ID)
			nodeReply(w, 500, map[string]any{"error": "Deploy timed out after 60 seconds"})
			return
		}
		time.Sleep(2 * time.Second)
		statusRequest, _ := http.NewRequestWithContext(r.Context(), "GET", fmt.Sprintf("%s/revisions/%s", denoV2API, revision.ID), nil)
		statusRequest.Header.Set("Authorization", "Bearer "+denoToken)
		statusResponse, statusErr := s.doer().client().Do(statusRequest)
		if statusErr != nil {
			continue
		}
		var statusData struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(deployReadAll(statusResponse), &statusData) == nil && statusData.Status != "" {
			status = statusData.Status
		}
	}
	if status != "succeeded" {
		s.denoDeleteApp(r, denoToken, app.ID)
		nodeReply(w, 500, map[string]any{"error": fmt.Sprintf("Deploy failed with status: %s", status)})
		return
	}
	orgSlug := orgDomain
	if index := strings.Index(orgDomain, "."); index >= 0 {
		orgSlug = orgDomain[:index]
	}
	deployURL := fmt.Sprintf("https://%s.%s.deno.net", projectName, orgSlug)
	proxyPool, createErr := s.createRelayPool(r, projectName, deployURL, "deno")
	if createErr != nil {
		nodeReply(w, 500, map[string]any{"error": devErrorString(createErr)})
		return
	}
	nodeReply(w, 201, map[string]any{"proxyPool": proxyPool, "deployUrl": deployURL})
}

func (s *RelayDeploys) denoDeleteApp(r *http.Request, denoToken, appID string) {
	deleteRequest, _ := http.NewRequestWithContext(r.Context(), "DELETE", fmt.Sprintf("%s/apps/%s", denoV2API, appID), nil)
	deleteRequest.Header.Set("Authorization", "Bearer "+denoToken)
	if response, err := s.doer().client().Do(deleteRequest); err == nil {
		_ = deployReadAll(response)
	}
}

// ── Vercel ──────────────────────────────────────────────────────────────────

const vercelAPI = "https://api.vercel.com"

func (s *RelayDeploys) VercelDeploy(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil || body == nil {
		body = map[string]any{}
	}
	vercelToken := trimmedDeployString(body, "vercelToken")
	projectName := deployProjectName(body)
	if vercelToken == "" {
		nodeReply(w, 400, map[string]any{"error": "Vercel API token is required"})
		return
	}
	deployRequest, _ := http.NewRequestWithContext(r.Context(), "POST", vercelAPI+"/v13/deployments", deployNewBody(deployJSONBody(map[string]any{
		"name": projectName,
		"files": []any{
			map[string]any{"file": "api/relay.js", "data": vercelRelayFunctionCode},
			map[string]any{"file": "package.json", "data": deployJSONString(map[string]any{"name": projectName, "version": "1.0.0"})},
			map[string]any{"file": "vercel.json", "data": deployJSONString(map[string]any{"rewrites": []any{map[string]any{"source": "/(.*)", "destination": "/api/relay"}}})},
		},
		"projectSettings": map[string]any{"framework": nil},
		"target":          "production",
	})))
	deployRequest.Header.Set("Authorization", "Bearer "+vercelToken)
	deployRequest.Header.Set("Content-Type", "application/json")
	deployResponse, doErr := s.doer().client().Do(deployRequest)
	if doErr != nil {
		nodeReply(w, 500, map[string]any{"error": errFieldOrDefault(doErr, "Deploy failed")})
		return
	}
	if deployResponse.StatusCode < 200 || deployResponse.StatusCode >= 300 {
		var errBody map[string]any
		message := "Failed to create Vercel deployment"
		if json.Unmarshal(deployReadAll(deployResponse), &errBody) == nil {
			if nested, ok := errBody["error"].(map[string]any); ok {
				if msg, ok := nested["message"].(string); ok && msg != "" {
					message = msg
				}
			}
		}
		nodeReply(w, deployResponse.StatusCode, map[string]any{"error": message})
		return
	}
	var deployment struct {
		ID        string `json:"id"`
		UID       string `json:"uid"`
		ProjectID string `json:"projectId"`
	}
	_ = json.Unmarshal(deployReadAll(deployResponse), &deployment)
	deploymentID := deployment.ID
	if deploymentID == "" {
		deploymentID = deployment.UID
	}
	projectID := deployment.ProjectID
	if projectID == "" {
		projectID = projectName
	}
	// Desativa proteção de deployment (best effort).
	patchRequest, _ := http.NewRequestWithContext(r.Context(), "PATCH", fmt.Sprintf("%s/v9/projects/%s", vercelAPI, projectID), deployNewBody(deployJSONBody(map[string]any{"ssoProtection": nil})))
	patchRequest.Header.Set("Authorization", "Bearer "+vercelToken)
	patchRequest.Header.Set("Content-Type", "application/json")
	if patchResponse, patchErr := s.doer().client().Do(patchRequest); patchErr == nil {
		_ = deployReadAll(patchResponse)
	}
	// Poll em 3 s até 120 s.
	deadline := time.Now().Add(120 * time.Second)
	deployURL := ""
	for {
		pollRequest, _ := http.NewRequestWithContext(r.Context(), "GET", fmt.Sprintf("%s/v13/deployments/%s", vercelAPI, deploymentID), nil)
		pollRequest.Header.Set("Authorization", "Bearer "+vercelToken)
		pollResponse, pollErr := s.doer().client().Do(pollRequest)
		if pollErr != nil {
			break
		}
		bodyBytes := deployReadAll(pollResponse)
		var data struct {
			ReadyState string `json:"readyState"`
			URL        string `json:"url"`
		}
		_ = json.Unmarshal(bodyBytes, &data)
		if data.ReadyState == "READY" {
			deployURL = "https://" + data.URL
			break
		}
		if data.ReadyState == "ERROR" || data.ReadyState == "CANCELED" {
			nodeReply(w, 500, map[string]any{"error": fmt.Sprintf("Deployment failed: %s", data.ReadyState)})
			return
		}
		if time.Now().After(deadline) {
			nodeReply(w, 500, map[string]any{"error": "Deployment timed out"})
			return
		}
		time.Sleep(3 * time.Second)
	}
	if deployURL == "" {
		nodeReply(w, 500, map[string]any{"error": "Deployment timed out"})
		return
	}
	proxyPool, createErr := s.createRelayPool(r, projectName, deployURL, "vercel")
	if createErr != nil {
		nodeReply(w, 500, map[string]any{"error": devErrorString(createErr)})
		return
	}
	nodeReply(w, 201, map[string]any{"proxyPool": proxyPool, "deployUrl": deployURL})
}

func deployJSONString(value map[string]any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
