"use client";

import { Suspense, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";

function MsIcon({ children, sx, className = "" }) {
  return (
    <Icon
      className={`material-symbols-outlined ${className}`.trim()}
      baseClassName=""
      sx={{ fontSize: 30, ...sx }}
    >
      {children}
    </Icon>
  );
}

/**
 * OAuth Callback Page Content
 */
function CallbackContent() {
  const searchParams = useSearchParams();
  const [status, setStatus] = useState("processing");

  useEffect(() => {
    const code = searchParams.get("code");
    const state = searchParams.get("state");
    const error = searchParams.get("error");
    const errorDescription = searchParams.get("error_description");

    const callbackData = {
      code,
      state,
      error,
      errorDescription,
      fullUrl: window.location.href,
    };

    let relayed = false;

    // Trusted origins that may receive this callback. The OAuth code/state
    // must only be relayed to the dashboard window we expect to be the opener
    // (same origin) or the Codex helper that listens on a fixed loopback port.
    // Any other origin is treated as hostile (drive-by attacker that opened
    // the popup against the well-known redirect_uri to phish the code).
    const expectedOrigins = [
      window.location.origin, // Same origin (for most providers)
      "http://localhost:1455", // Codex specific port
    ];

    // Method 1: postMessage to opener (popup mode)
    // Send once per expected origin. The browser delivers the message only
    // when the opener's origin matches the targetOrigin we pass — using "*"
    // here would leak the code/state to any opener (e.g. an attacker page
    // that opened this URL in a popup), so iterate over the allowlist.
    if (window.opener) {
      for (const origin of expectedOrigins) {
        try {
          window.opener.postMessage({ type: "oauth_callback", data: callbackData }, origin);
          relayed = true;
        } catch (e) {
          console.log("postMessage failed:", e);
        }
      }
    }

    // Method 2: BroadcastChannel (same origin tabs)
    try {
      const channel = new BroadcastChannel("oauth_callback");
      channel.postMessage(callbackData);
      channel.close();
      relayed = true;
    } catch (e) {
      console.log("BroadcastChannel failed:", e);
    }

    // Method 3: localStorage event (fallback)
    try {
      localStorage.setItem("oauth_callback_v1", JSON.stringify({ ...callbackData, timestamp: Date.now() }));
      relayed = true;
    } catch (e) {
      console.log("localStorage failed:", e);
    }

    if (!(code || error)) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time callback status update after URL params parsed.
      const t = setTimeout(() => setStatus("manual"), 0);
      return () => clearTimeout(t);
    }

    // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time callback success state.
    setStatus("success");
    const t1 = setTimeout(() => {
      window.close();
      const t2 = setTimeout(() => setStatus("done"), 500);
      return () => clearTimeout(t2);
    }, 1500);
    return () => clearTimeout(t1);
  }, [searchParams]);

  return (
    <Box sx={{ minHeight: "100vh", display: "flex", alignItems: "center", justifyContent: "center", bgcolor: "background.default" }}>
      <Box sx={{ textAlign: "center", p: 4, maxWidth: 448 }}>
        {status === "processing" && (
          <>
            <Box sx={{ width: 64, height: 64, mx: "auto", mb: 2, borderRadius: "50%", bgcolor: "rgba(229,106,74,0.10)", display: "flex", alignItems: "center", justifyContent: "center" }}>
              <MsIcon className="material-symbols-outlined" sx={{ color: "primary.main", animation: "mui-spinner-rotate 1s linear infinite", "@keyframes mui-spinner-rotate": { to: { transform: "rotate(360deg)" } } }}>progress_activity</MsIcon>
            </Box>
            <Typography sx={{ fontSize: 20, fontWeight: 600, mb: 1 }}>Processing...</Typography>
            <Typography sx={{ color: "text.secondary" }}>Please wait while we complete the authorization.</Typography>
          </>
        )}

        {(status === "success" || status === "done") && (
          <>
            <Box sx={{ width: 64, height: 64, mx: "auto", mb: 2, borderRadius: "50%", bgcolor: "rgba(34,197,94,0.15)", display: "flex", alignItems: "center", justifyContent: "center" }}>
              <MsIcon sx={{ color: "#16a34a" }}>check_circle</MsIcon>
            </Box>
            <Typography sx={{ fontSize: 20, fontWeight: 600, mb: 1 }}>Authorization Successful!</Typography>
            <Typography sx={{ color: "text.secondary" }}>
              {status === "success" ? "This window will close automatically..." : "You can close this tab now."}
            </Typography>
          </>
        )}

        {status === "manual" && (
          <>
            <Box sx={{ width: 64, height: 64, mx: "auto", mb: 2, borderRadius: "50%", bgcolor: "rgba(234,179,8,0.15)", display: "flex", alignItems: "center", justifyContent: "center" }}>
              <MsIcon sx={{ color: "#ca8a04" }}>info</MsIcon>
            </Box>
            <Typography sx={{ fontSize: 20, fontWeight: 600, mb: 1 }}>Copy This URL</Typography>
            <Typography sx={{ color: "text.secondary", mb: 2 }}>
              Please copy the URL from the address bar and paste it in the application.
            </Typography>
            <Box sx={{ bgcolor: "background.paper", border: 1, borderColor: "divider", borderRadius: "8px", p: 1.5, textAlign: "left" }}>
              <Box component="code" sx={{ fontSize: 12, wordBreak: "break-all" }}>
                {typeof window !== "undefined" ? window.location.href : ""}
              </Box>
            </Box>
          </>
        )}
      </Box>
    </Box>
  );
}

/**
 * OAuth Callback Page
 * Receives callback from OAuth providers and sends data back via multiple methods
 */
export default function CallbackPage() {
  return (
    <Suspense fallback={
      <Box sx={{ minHeight: "100vh", display: "flex", alignItems: "center", justifyContent: "center", bgcolor: "background.default" }}>
        <Box sx={{ textAlign: "center", p: 4 }}>
          <Box sx={{ width: 64, height: 64, mx: "auto", mb: 2, borderRadius: "50%", bgcolor: "rgba(229,106,74,0.10)", display: "flex", alignItems: "center", justifyContent: "center" }}>
            <MsIcon className="material-symbols-outlined" sx={{ color: "primary.main", animation: "mui-spinner-rotate 1s linear infinite", "@keyframes mui-spinner-rotate": { to: { transform: "rotate(360deg)" } } }}>progress_activity</MsIcon>
          </Box>
          <Typography sx={{ color: "text.secondary" }}>Loading...</Typography>
        </Box>
      </Box>
    }>
      <CallbackContent />
    </Suspense>
  );
}
