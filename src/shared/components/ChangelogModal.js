"use client";

import { useEffect, useState, useRef } from "react";
import { createPortal } from "react-dom";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import IconButton from "@mui/material/IconButton";
import { GITHUB_CONFIG } from "@/shared/constants/config";

function sanitizeHtml(html) {
  if (typeof window === "undefined") return "";
  const DOMPurify = require("dompurify");
  return DOMPurify.sanitize(html, { FORBID_TAGS: ["script", "iframe", "object", "embed", "form"], FORBID_ATTR: ["onerror", "onload", "onclick"] });
}

export default function ChangelogModal({ isOpen, onClose }) {
  const [fetchState, setFetchState] = useState({ html: "", loading: false, error: "" });
  const modalRef = useRef(null);
  const hasFetched = useRef(false);

  useEffect(() => {
    if (!isOpen || hasFetched.current) return;
    hasFetched.current = true;
    setFetchState(prev => ({ ...prev, loading: true, error: "" }));
    fetch(GITHUB_CONFIG.changelogUrl)
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.text();
      })
      .then((md) => import("marked").then(({ marked }) => ({ md, marked })))
      .then(({ md, marked }) => {
        marked.setOptions({ gfm: true, breaks: true });
        setFetchState(prev => ({ ...prev, html: sanitizeHtml(marked.parse(md)), loading: false }));
      })
      .catch((err) => setFetchState(prev => ({ ...prev, error: err.message || "Failed to load", loading: false })));
  }, [isOpen]);

  const onCloseRef = useRef(onClose);
  useEffect(() => {
    onCloseRef.current = onClose;
  });

  useEffect(() => {
    const handleClickOutside = (e) => {
      if (modalRef.current && !modalRef.current.contains(e.target)) {
        onCloseRef.current();
      }
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

  const { html, loading, error } = fetchState;

  return createPortal(
    <Box sx={{ position: "fixed", inset: 0, zIndex: 50, display: "flex", alignItems: "center", justifyContent: "center", p: 2 }}>
      {/* Overlay */}
      <Box
        onClick={onClose}
        aria-hidden="true"
        sx={{ position: "absolute", inset: 0, bgcolor: "rgba(0,0,0,0.3)", backdropFilter: "blur(4px)" }}
      />

      {/* Modal content */}
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
        {/* Header */}
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", p: 1.5, borderBottom: "1px solid var(--color-border-subtle)" }}>
          <Typography variant="h6" sx={{ fontSize: 18, fontWeight: 600, color: "text.primary" }}>Change Log</Typography>
          <IconButton
            type="button"
            onClick={onClose}
            aria-label="Close"
            sx={{ p: 0.75, borderRadius: "8px", color: "text.secondary", "&:hover": { bgcolor: "var(--color-surface-2)" } }}
          >
            <span className="material-symbols-outlined" style={{ fontSize: 20 }}>close</span>
          </IconButton>
        </Box>

        {/* Body */}
        <Box sx={{ p: 3, overflowY: "auto", flex: 1 }}>
          {loading && (
            <Box sx={{ display: "flex", alignItems: "center", justifyContent: "center", py: 5, color: "text.secondary" }}>
              <span className="material-symbols-outlined" style={{ animation: "spin 1s linear infinite",  marginRight: 8 }}>progress_activity</span>
              Loading...
            </Box>
          )}
          {error && (
            <Box sx={{ color: "error.main", py: 2 }}>Failed to load changelog: {error}</Box>
          )}
          {!loading && !error && html && (
            <Box
              className="changelog-body"
              sx={{ color: "text.primary" }}
              // SECURITY: html is sanitized via DOMPurify.sanitize() before reaching this element
              dangerouslySetInnerHTML={{ __html: html }}
            />
          )}
        </Box>
      </Box>
    </Box>,
    document.body
  );
}
