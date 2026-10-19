"use client";

import { useState, useEffect, useRef } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import { Modal, Button, Input } from "@/shared/components";

const methodBtnSx = {
  width: "100%",
  p: 2,
  textAlign: "left",
  border: "1px solid var(--color-border)",
  borderRadius: "8px",
  bgcolor: "transparent",
  cursor: "pointer",
  transition: "background-color 150ms",
  "&:hover": { bgcolor: "var(--color-sidebar)" },
};

function MethodButton({ onClick, icon, title, children }) {
  return (
    <Box component="button" type="button" onClick={onClick} sx={methodBtnSx}>
      <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1.5 }}>
        <span className="material-symbols-outlined" style={{ color: "var(--color-primary)", marginTop: 2 }}>{icon}</span>
        <Box sx={{ flex: 1 }}>
          <Typography component="h3" sx={{ fontWeight: 600, mb: 0.5 }}>{title}</Typography>
          <Typography sx={{ fontSize: 14, color: "text.secondary" }}>{children}</Typography>
        </Box>
      </Box>
    </Box>
  );
}

/**
 * Kiro Auth Method Selection Modal
 * Auto-detects token from AWS SSO cache or allows manual import
 */
export default function KiroAuthModal({ isOpen, onMethodSelect, onClose }) {
  const [selectedMethod, setSelectedMethod] = useState(null);
  const [idcStartUrl, setIdcStartUrl] = useState("");
  const [idcRegion, setIdcRegion] = useState("us-east-1");
  const [refreshToken, setRefreshToken] = useState("");
  const [cliProxyJson, setCliProxyJson] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [apiKeyRegion, setApiKeyRegion] = useState("us-east-1");
  const [error, setError] = useState(null);
  const [importing, setImporting] = useState(false);
  const [autoDetecting, setAutoDetecting] = useState(false);
  const [autoDetected, setAutoDetected] = useState(false);
  const [idcCredentials, setIdcCredentials] = useState(null);
  const flowIdRef = useRef(0);

  // Auto-detect token when import method is selected
  useEffect(() => {
    if (selectedMethod !== "import" || !isOpen) return;

    const flowId = ++flowIdRef.current;
    const autoDetect = async () => {
      setAutoDetecting(true);
      setError(null);
      setAutoDetected(false);
      setIdcCredentials(null);

      try {
        const res = await fetch("/api/oauth/kiro/auto-import");
        const data = await res.json();
        if (flowId !== flowIdRef.current || !isOpen || selectedMethod !== "import") return;

        if (data.found) {
          setRefreshToken(data.refreshToken);
          setAutoDetected(true);
          // Store IDC/organization credentials if present
          if (data.clientId && data.clientSecret) {
            setIdcCredentials({
              clientId: data.clientId,
              clientSecret: data.clientSecret,
              region: data.region,
              authMethod: data.authMethod,
              profileArn: data.profileArn,
            });
          }
        } else {
          setError(data.error || "Could not auto-detect token");
        }
      } catch (err) {
        if (flowId === flowIdRef.current && isOpen && selectedMethod === "import") {
          setError("Failed to auto-detect token");
        }
      } finally {
        if (flowId === flowIdRef.current) setAutoDetecting(false);
      }
    };

    autoDetect();
  }, [selectedMethod, isOpen]);

  const handleMethodSelect = (method) => {
    flowIdRef.current += 1;
    setSelectedMethod(method);
    setError(null);
  };

  const handleBack = () => {
    flowIdRef.current += 1;
    setSelectedMethod(null);
    setError(null);
  };

  const handleImportToken = async () => {
    if (!refreshToken.trim()) {
      setError("Please enter a refresh token");
      return;
    }

    const flowId = ++flowIdRef.current;
    setImporting(true);
    setError(null);

    try {
      const res = await fetch("/api/oauth/kiro/import", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          refreshToken: refreshToken.trim(),
          ...(idcCredentials || {}),
        }),
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(data.error || "Import failed");
      }
      if (flowId !== flowIdRef.current || !isOpen) return;

      // Success - notify parent to refresh connections
      onMethodSelect("import");
    } catch (err) {
      if (flowId === flowIdRef.current && isOpen) setError(err.message);
    } finally {
      if (flowId === flowIdRef.current) setImporting(false);
    }
  };

  const handleImportCliProxyJson = async () => {
    if (!cliProxyJson.trim()) {
      setError("Please paste CLIProxyAPI auth JSON");
      return;
    }

    setImporting(true);
    setError(null);

    try {
      const res = await fetch("/api/oauth/kiro/import-cli-proxy", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ json: cliProxyJson.trim() }),
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(data.error || "CLIProxyAPI import failed");
      }

      onMethodSelect("import-cli-proxy");
    } catch (err) {
      setError(err.message);
    } finally {
      setImporting(false);
    }
  };

  const handleIdcContinue = () => {
    if (!idcStartUrl.trim()) {
      setError("Please enter your IDC start URL");
      return;
    }
    onMethodSelect("idc", { startUrl: idcStartUrl.trim(), region: idcRegion });
  };

  const handleApiKeyImport = async () => {
    if (!apiKey.trim()) {
      setError("Please enter an API key");
      return;
    }

    setImporting(true);
    setError(null);

    try {
      const res = await fetch("/api/oauth/kiro/api-key", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          apiKey: apiKey.trim(),
          region: apiKeyRegion.trim() || "us-east-1",
        }),
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(data.error || "Import failed");
      }

      // Success - notify parent to refresh connections
      onMethodSelect("api-key");
    } catch (err) {
      setError(err.message);
    } finally {
      setImporting(false);
    }
  };

  const handleSocialLogin = (provider) => {
    onMethodSelect("social", { provider });
  };

  const noteSx = (bg, border) => ({ bgcolor: bg, p: 1.5, borderRadius: "8px", border: `1px solid ${border}`, display: "flex", gap: 1 });

  return (
    <Modal isOpen={isOpen} title="Connect Kiro" onClose={onClose} size="lg">
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {/* Method Selection */}
        {!selectedMethod && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
            <Typography sx={{ fontSize: 14, color: "text.secondary", mb: 2 }}>
              Choose your authentication method:
            </Typography>

            <MethodButton onClick={() => onMethodSelect("builder-id")} icon="shield" title="AWS Builder ID">
              Recommended for most users. Free AWS account required.
            </MethodButton>

            <MethodButton onClick={() => handleMethodSelect("idc")} icon="business" title="AWS IAM Identity Center">
              For enterprise users with custom AWS IAM Identity Center.
            </MethodButton>

            <MethodButton onClick={() => handleMethodSelect("api-key")} icon="key" title="API Key">
              Use a long-lived Kiro/CodeWhisperer API key (headless auth).
            </MethodButton>

            <MethodButton onClick={() => handleSocialLogin("google")} icon="account_circle" title="Google Account">
              Login with your Google account via device authorization code.
            </MethodButton>

            <MethodButton onClick={() => handleSocialLogin("github")} icon="code" title="GitHub Account">
              Login with your GitHub account via device authorization code.
            </MethodButton>

            <MethodButton onClick={() => handleMethodSelect("import")} icon="file_upload" title="Import Token">
              Paste refresh token from Kiro IDE.
            </MethodButton>

            <MethodButton onClick={() => handleMethodSelect("import-cli-proxy")} icon="data_object" title="Import CLIProxyAPI JSON">
              Paste external_idp auth JSON from CLIProxyAPI/Kiro Microsoft login.
            </MethodButton>
          </Box>
        )}

        {/* IDC Configuration */}
        {selectedMethod === "idc" && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            <Box>
              <Box component="label" sx={{ display: "block", fontSize: 14, fontWeight: 500, mb: 1 }}>
                IDC Start URL <Box component="span" sx={{ color: "error.main" }}>*</Box>
              </Box>
              <Input
                value={idcStartUrl}
                onChange={(e) => setIdcStartUrl(e.target.value)}
                placeholder="https://your-org.awsapps.com/start"
                inputClassName="font-mono text-sm"
              />
              <Typography sx={{ fontSize: 12, color: "text.secondary", mt: 0.5 }}>
                Your organization&apos;s AWS IAM Identity Center URL
              </Typography>
            </Box>

            <Box>
              <Box component="label" sx={{ display: "block", fontSize: 14, fontWeight: 500, mb: 1 }}>
                AWS Region
              </Box>
              <Input
                value={idcRegion}
                onChange={(e) => setIdcRegion(e.target.value)}
                placeholder="us-east-1"
                inputClassName="font-mono text-sm"
              />
              <Typography sx={{ fontSize: 12, color: "text.secondary", mt: 0.5 }}>
                AWS region for your Identity Center (default: us-east-1)
              </Typography>
            </Box>

            {error && (
              <Typography sx={{ fontSize: 14, color: "#dc2626" }}>{error}</Typography>
            )}

            <Box sx={{ display: "flex", gap: 1 }}>
              <Button onClick={handleIdcContinue} fullWidth>
                Continue
              </Button>
              <Button onClick={handleBack} variant="ghost" fullWidth>
                Back
              </Button>
            </Box>
          </Box>
        )}

        {/* API Key */}
        {selectedMethod === "api-key" && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            <Box sx={noteSx("rgba(59,130,246,0.1)", "rgba(59,130,246,0.3)")}>
              <span className="material-symbols-outlined" style={{ color: "#2563eb" }}>info</span>
              <Typography sx={{ fontSize: 14, color: "#1e40af" }}>
                Paste a long-lived Kiro/CodeWhisperer API key. It is validated
                against AWS and stored directly as a bearer credential (no refresh).
              </Typography>
            </Box>

            <Box>
              <Box component="label" sx={{ display: "block", fontSize: 14, fontWeight: 500, mb: 1 }}>
                API Key <Box component="span" sx={{ color: "error.main" }}>*</Box>
              </Box>
              <Input
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                placeholder="Paste your Kiro API key..."
                inputClassName="font-mono text-sm"
              />
            </Box>

            <Box>
              <Box component="label" sx={{ display: "block", fontSize: 14, fontWeight: 500, mb: 1 }}>
                AWS Region
              </Box>
              <Input
                value={apiKeyRegion}
                onChange={(e) => setApiKeyRegion(e.target.value)}
                placeholder="us-east-1"
                inputClassName="font-mono text-sm"
              />
              <Typography sx={{ fontSize: 12, color: "text.secondary", mt: 0.5 }}>
                AWS region for the key (default: us-east-1)
              </Typography>
            </Box>

            {error && (
              <Box sx={noteSx("rgba(239,68,68,0.1)", "rgba(239,68,68,0.3)")}>
                <Typography sx={{ fontSize: 14, color: "#dc2626" }}>{error}</Typography>
              </Box>
            )}

            <Box sx={{ display: "flex", gap: 1 }}>
              <Button onClick={handleApiKeyImport} fullWidth disabled={importing || !apiKey.trim()}>
                {importing ? "Validating..." : "Add API Key"}
              </Button>
              <Button onClick={handleBack} variant="ghost" fullWidth>
                Back
              </Button>
            </Box>
          </Box>
        )}

        {/* Social Login Info (Google) */}
        {selectedMethod === "social-google" && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            <Box sx={{ bgcolor: "rgba(245,158,11,0.1)", p: 2, borderRadius: "8px", border: "1px solid rgba(245,158,11,0.3)" }}>
              <Box sx={{ display: "flex", gap: 1 }}>
                <span className="material-symbols-outlined" style={{ color: "#d97706" }}>info</span>
                <Box sx={{ flex: 1, fontSize: 14 }}>
                  <Typography sx={{ fontWeight: 500, color: "#78350f", mb: 0.5 }}>
                    Manual Callback Required
                  </Typography>
                  <Typography sx={{ color: "#92400e" }}>
                    After login, you&apos;ll need to copy the callback URL from your browser and paste it back here.
                  </Typography>
                </Box>
              </Box>
            </Box>

            <Box sx={{ display: "flex", gap: 1 }}>
              <Button onClick={() => handleSocialLogin("google")} fullWidth>
                Continue with Google
              </Button>
              <Button onClick={handleBack} variant="ghost" fullWidth>
                Back
              </Button>
            </Box>
          </Box>
        )}

        {/* Social Login Info (GitHub) */}
        {selectedMethod === "social-github" && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            <Box sx={{ bgcolor: "rgba(245,158,11,0.1)", p: 2, borderRadius: "8px", border: "1px solid rgba(245,158,11,0.3)" }}>
              <Box sx={{ display: "flex", gap: 1 }}>
                <span className="material-symbols-outlined" style={{ color: "#d97706" }}>info</span>
                <Box sx={{ flex: 1, fontSize: 14 }}>
                  <Typography sx={{ fontWeight: 500, color: "#78350f", mb: 0.5 }}>
                    Manual Callback Required
                  </Typography>
                  <Typography sx={{ color: "#92400e" }}>
                    After login, you&apos;ll need to copy the callback URL from your browser and paste it back here.
                  </Typography>
                </Box>
              </Box>
            </Box>

            <Box sx={{ display: "flex", gap: 1 }}>
              <Button onClick={() => handleSocialLogin("github")} fullWidth>
                Continue with GitHub
              </Button>
              <Button onClick={handleBack} variant="ghost" fullWidth>
                Back
              </Button>
            </Box>
          </Box>
        )}

        {/* Import Token */}
        {selectedMethod === "import" && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            {/* Auto-detecting state */}
            {autoDetecting && (
              <Box sx={{ textAlign: "center", py: 3 }}>
                <Box sx={{ width: 64, height: 64, mx: "auto", mb: 2, borderRadius: "50%", bgcolor: "rgba(229,106,74,0.1)", display: "flex", alignItems: "center", justifyContent: "center" }}>
                  <span className="material-symbols-outlined" style={{ animation: "spin 1s linear infinite",  fontSize: 30, color: "var(--color-primary)" }}>
                    progress_activity
                  </span>
                </Box>
                <Typography variant="h6" sx={{ fontSize: 18, fontWeight: 600, mb: 1 }}>Auto-detecting token...</Typography>
                <Typography sx={{ fontSize: 14, color: "text.secondary" }}>
                  Reading from AWS SSO cache
                </Typography>
              </Box>
            )}

            {/* Form (shown after auto-detect completes) */}
            {!autoDetecting && (
              <>
                {/* Success message if auto-detected */}
                {autoDetected && (
                  <Box sx={noteSx("rgba(34,197,94,0.1)", "rgba(34,197,94,0.3)")}>
                    <span className="material-symbols-outlined" style={{ color: "#16a34a" }}>check_circle</span>
                    <Typography sx={{ fontSize: 14, color: "#166534" }}>
                      Token auto-detected from Kiro IDE successfully!
                    </Typography>
                  </Box>
                )}

                {/* Info message if not auto-detected */}
                {!autoDetected && !error && (
                  <Box sx={noteSx("rgba(59,130,246,0.1)", "rgba(59,130,246,0.3)")}>
                    <span className="material-symbols-outlined" style={{ color: "#2563eb" }}>info</span>
                    <Typography sx={{ fontSize: 14, color: "#1e40af" }}>
                      Kiro IDE not detected. Please paste your refresh token manually.
                    </Typography>
                  </Box>
                )}

                <Box>
                  <Box component="label" sx={{ display: "block", fontSize: 14, fontWeight: 500, mb: 1 }}>
                    Refresh Token <Box component="span" sx={{ color: "error.main" }}>*</Box>
                  </Box>
                  <Input
                    value={refreshToken}
                    onChange={(e) => setRefreshToken(e.target.value)}
                    placeholder="Token will be auto-filled..."
                    inputClassName="font-mono text-sm"
                  />
                </Box>

                {error && (
                  <Box sx={noteSx("rgba(239,68,68,0.1)", "rgba(239,68,68,0.3)")}>
                    <Typography sx={{ fontSize: 14, color: "#dc2626" }}>{error}</Typography>
                  </Box>
                )}

                <Box sx={{ display: "flex", gap: 1 }}>
                  <Button onClick={handleImportToken} fullWidth disabled={importing || !refreshToken.trim()}>
                    {importing ? "Importing..." : "Import Token"}
                  </Button>
                  <Button onClick={handleBack} variant="ghost" fullWidth>
                    Back
                  </Button>
                </Box>
              </>
            )}
          </Box>
        )}

        {/* Import CLIProxyAPI JSON */}
        {selectedMethod === "import-cli-proxy" && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            <Box sx={noteSx("rgba(59,130,246,0.1)", "rgba(59,130,246,0.3)")}>
              <span className="material-symbols-outlined" style={{ color: "#2563eb" }}>info</span>
              <Typography sx={{ fontSize: 14, color: "#1e40af" }}>
                Paste the Kiro CLIProxyAPI auth JSON containing auth_method=external_idp. Only Microsoft login token endpoints are accepted.
              </Typography>
            </Box>

            <Box>
              <Box component="label" sx={{ display: "block", fontSize: 14, fontWeight: 500, mb: 1 }}>
                CLIProxyAPI Auth JSON <Box component="span" sx={{ color: "error.main" }}>*</Box>
              </Box>
              <Box
                component="textarea"
                value={cliProxyJson}
                onChange={(e) => setCliProxyJson(e.target.value)}
                placeholder={'{"auth_method":"external_idp","access_token":"...","refresh_token":"...","client_id":"...","token_endpoint":"https://login.microsoftonline.com/.../oauth2/v2.0/token","profile_arn":"...","scopes":"..."}'}
                sx={{ minHeight: 160, width: "100%", borderRadius: "6px", border: "1px solid var(--color-border)", bgcolor: "background.default", p: 1.5, fontFamily: "ui-monospace, monospace", fontSize: 14, color: "text.primary", outline: "none", "&:focus": { borderColor: "primary.main" } }}
              />
            </Box>

            {error && (
              <Box sx={noteSx("rgba(239,68,68,0.1)", "rgba(239,68,68,0.3)")}>
                <Typography sx={{ fontSize: 14, color: "#dc2626" }}>{error}</Typography>
              </Box>
            )}

            <Box sx={{ display: "flex", gap: 1 }}>
              <Button onClick={handleImportCliProxyJson} fullWidth disabled={importing || !cliProxyJson.trim()}>
                {importing ? "Importing..." : "Import CLIProxyAPI JSON"}
              </Button>
              <Button onClick={handleBack} variant="ghost" fullWidth>
                Back
              </Button>
            </Box>
          </Box>
        )}
      </Box>
    </Modal>
  );
}
