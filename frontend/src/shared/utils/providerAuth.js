export function getProviderAuthTypes(provider, providerId) {
  if (providerId === "kiro") return ["oauth", "apikey", "api_key"];
  if (Array.isArray(provider?.authModes) && provider.authModes.length > 0) {
    return provider.authModes;
  }
  return provider?.hasOAuth ? ["oauth"] : ["apikey"];
}
