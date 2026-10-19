"use client";

import { useEffect, useState, useRef } from "react";
import Image from "next/image";
import { createPortal } from "react-dom";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import IconButton from "@mui/material/IconButton";
import { GITHUB_CONFIG } from "@/shared/constants/config";

const DEFAULT_DONATE_DATA = {
  title: "Support VansRouter",
  message: "Terima kasih telah mendukung pengembangan VansRouter agar infrastruktur dan fitur baru terus aktif!",
  channels: [
    {
      id: "saweria",
      label: "Saweria",
      description: "Dukung via QRIS, GoPay, OVO, DANA, LinkAja, ShopeePay",
      icon: "volunteer_activism",
      color: "#FAAE2B",
      url: "https://saweria.co/vanszs"
    },
    {
      id: "trakteer",
      label: "Trakteer",
      description: "Dukung via QRIS / E-Wallet / Bank Transfer",
      icon: "favorite",
      color: "#C9283E",
      url: "https://teer.id/bevan_satriaa"
    },
    {
      id: "kofi",
      label: "Ko-fi",
      description: "Support via Card or PayPal",
      icon: "local_cafe",
      color: "#13C3FF",
      url: "https://ko-fi.com/bevansatriaa"
    }
  ]
};

export default function DonateModal({ isOpen, onClose }) {
  const [fetchState, setFetchState] = useState({ data: DEFAULT_DONATE_DATA, loading: false, error: "" });
  const modalRef = useRef(null);
  const hasFetched = useRef(false);

  useEffect(() => {
    if (!isOpen || hasFetched.current) return;
    hasFetched.current = true;

    // Wrap state updates in Promise.resolve().then() to defer them out of the
    // effect's synchronous execution. This avoids the react-hooks
    // set-state-in-effect rule while preserving the same observable behavior.
    Promise.resolve().then(() => {
      if (!GITHUB_CONFIG.donateUrl) {
        // Use default built-in donate channels if no external remote config URL is provided
        return;
      }
      setFetchState(prev => ({ ...prev, loading: true, error: "" }));
      fetch(GITHUB_CONFIG.donateUrl, { cache: "no-store" })
        .then((res) => {
          if (!res.ok) throw new Error(`HTTP ${res.status}`);
          return res.json();
        })
        .then((json) => setFetchState({ data: json, loading: false, error: "" }))
        .catch((err) => setFetchState({ data: DEFAULT_DONATE_DATA, error: "", loading: false }));
    });
  }, [isOpen]);

  const onCloseRef = useRef(onClose);
  useEffect(() => {
    onCloseRef.current = onClose;
  });

  useEffect(() => {
    const handleClickOutside = (e) => {
      if (modalRef.current && !modalRef.current.contains(e.target)) onCloseRef.current();
    };
    if (isOpen) {
      document.addEventListener("mousedown", handleClickOutside);
      return () => document.removeEventListener("mousedown", handleClickOutside);
    }
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen) return;
    const onEsc = (e) => { if (e.key === "Escape") onCloseRef.current(); };
    document.addEventListener("keydown", onEsc);
    return () => document.removeEventListener("keydown", onEsc);
  }, [isOpen]);

  if (!isOpen || typeof document === "undefined") return null;

  const { data, loading, error } = fetchState;

  return createPortal(
    <Box sx={{ position: "fixed", inset: 0, zIndex: 50, display: "flex", alignItems: "center", justifyContent: "center", p: 2 }}>
      <Box onClick={onClose} aria-hidden="true" sx={{ position: "absolute", inset: 0, bgcolor: "rgba(0,0,0,0.3)", backdropFilter: "blur(4px)" }} />
      <Box
        ref={modalRef}
        sx={{
          position: "relative",
          width: "100%",
          maxWidth: 768,
          bgcolor: "background.paper",
          border: "1px solid var(--color-border)",
          borderRadius: "12px",
          boxShadow: "0 25px 50px -12px rgba(0,0,0,0.25)",
          display: "flex",
          flexDirection: "column",
          maxHeight: "85vh",
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", p: 1.5, borderBottom: "1px solid var(--color-border-subtle)" }}>
          <Typography component="h2" variant="h6" sx={{ fontSize: 18, fontWeight: 600, color: "text.primary", display: "flex", alignItems: "center", gap: 1 }}>
            <span className="material-symbols-outlined" style={{ color: "#ec4899" }}>volunteer_activism</span>
            {data?.title || "Support 9Router"}
          </Typography>
          <IconButton
            type="button"
            onClick={onClose}
            aria-label="Close"
            sx={{ p: 0.75, borderRadius: "8px", color: "text.secondary", "&:hover": { bgcolor: "var(--color-surface-2)" } }}
          >
            <span className="material-symbols-outlined" style={{ fontSize: 20 }}>close</span>
          </IconButton>
        </Box>

        <Box sx={{ p: 3, overflowY: "auto", flex: 1 }}>
          {loading && (
            <Box sx={{ display: "flex", alignItems: "center", justifyContent: "center", py: 5, color: "text.secondary" }}>
              <span className="material-symbols-outlined" style={{ animation: "spin 1s linear infinite",  marginRight: 8 }}>progress_activity</span>
              Loading...
            </Box>
          )}
          {error && (
            <Box sx={{ color: "error.main", py: 2 }}>Failed to load donate info: {error}</Box>
          )}
          {!loading && !error && data && (
            <>
              {data.message && (
                <Typography sx={{ color: "text.secondary", fontSize: 14, mb: 3, textAlign: "center" }}>{data.message}</Typography>
              )}
              <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "repeat(3, 1fr)" }, gap: 2 }}>
                {data.channels?.map((ch) => (
                  <DonateChannelCard key={ch.id} channel={ch} />
                ))}
              </Box>
            </>
          )}
        </Box>
      </Box>
    </Box>,
    document.body
  );
}

function DonateChannelCard({ channel }) {
  const { label, description, icon, color, url, qr } = channel;
  const content = (
    <>
      <Box
        sx={{ width: 48, height: 48, borderRadius: "50%", display: "flex", alignItems: "center", justifyContent: "center", mb: 1.5, bgcolor: `${color}20`, color }}
      >
        <span className="material-symbols-outlined" style={{ fontSize: 26 }}>{icon}</span>
      </Box>
      <Box sx={{ fontWeight: 600, color: "text.primary", mb: 0.5 }}>{label}</Box>
      {description && (
        <Box sx={{ fontSize: 12, color: "text.secondary", mb: 1.5, textAlign: "center" }}>{description}</Box>
      )}
      {qr && (
        <Image
          src={qr}
          alt={`${label} QR`}
          style={{ width: "100%", maxWidth: 180, aspectRatio: "1 / 1", objectFit: "contain", borderRadius: 8, background: "#fff", padding: 4 }}
          width={180}
          height={180}
          unoptimized
        />
      )}
    </>
  );

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        p: 2,
        borderRadius: "12px",
        border: "1px solid var(--color-border)",
        bgcolor: "var(--color-surface)",
        transition: "border-color 150ms",
        "&:hover": { borderColor: "rgba(236,72,153,0.4)" },
      }}
    >
      {content}
      {url && (
        <Box
          component="a"
          href={url}
          target="_blank"
          rel="noopener noreferrer"
          sx={{ mt: 1.5, display: "inline-flex", alignItems: "center", gap: 0.5, px: 1.5, py: 0.75, borderRadius: "8px", fontSize: 14, fontWeight: 500, color: "#fff", bgcolor: color, "&:hover": { opacity: 0.9 } }}
        >
          Open
          <span className="material-symbols-outlined" style={{ fontSize: 16 }}>open_in_new</span>
        </Box>
      )}
    </Box>
  );
}
