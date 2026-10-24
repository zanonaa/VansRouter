export default {
  id: "kimchi",
  priority: 25,
  alias: "kimchi",
  uiAlias: "kimchi",
  display: {
    name: "Kimchi",
    icon: "local_dining",
    color: "#FF6B35",
    textIcon: "KC",
    website: "https://kimchi.dev",
    notice: {
      apiKeyUrl: "https://app.kimchi.dev/settings",
      signupUrl: "https://app.kimchi.dev",
    },
  },
  category: "freeTier",
  hasFree: true,
  authType: "apikey",
  hasOAuth: true,
  authModes: ["apikey", "oauth"],
  serviceKinds: ["llm", "webSearch"],
  searchConfig: {
    baseUrl: "https://llm.kimchi.dev/v1/search",
    authType: "apikey",
    authHeader: "bearer",
    format: "openai",
    timeoutMs: 25000,
  },
  transport: {
    // Kimchi API key works with llm.kimchi.dev endpoint.
    // api.moonshot.ai requires Moonshot API key (different auth).
    baseUrl: "https://llm.kimchi.dev/openai/v1/chat/completions",
    format: "openai",
    timeoutMs: 20000,
    headers: {
      Accept: "text/event-stream,application/json",
    },
    auth: {
      apiKey: {
        header: "Authorization",
        scheme: "bearer",
        hooks: ["kimchiHeaders"],
      },
      oauth: {
        header: "Authorization",
        scheme: "bearer",
        hooks: ["kimchiHeaders"],
      },
    },
    forceStream: false,
    preserveAccept: true,
    retry: {
      429: { attempts: 3, delayMs: 500 },
      502: { attempts: 1, delayMs: 500 },
      503: { attempts: 3, delayMs: 1000 },
    },
  },
  oauth: {
    webAppUrl: "https://app.kimchi.dev",
    validationUrl: "https://api.cast.ai/v1/llm/openai/supported-providers",
    userInfoUrl: "https://app.kimchi.dev/api/v1/me",
    modelsUrl: "https://llm.kimchi.dev/v1/models/metadata?include_in_cli=true",
  },
  passthroughModels: true,
  // Exactly the 4 models advertised by the Kimchi CLI (kimchi-dev provider).
  models: [
    { id: "kimi-k2.7", name: "Kimi K2.7" },
    { id: "minimax-m3", name: "MiniMax M3" },
    { id: "nemotron-3-ultra-fp4", name: "Nemotron 3 Ultra FP4" },
    { id: "deepseek-v4-flash", name: "DeepSeek V4 Flash" },
  ],
};
