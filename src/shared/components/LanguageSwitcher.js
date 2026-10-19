"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { createPortal } from "react-dom";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import { LOCALES, LOCALE_COOKIE, normalizeLocale } from "@/i18n/config";
import { reloadTranslations } from "@/i18n/runtime";

function getLocaleFromCookie() {
  if (typeof document === "undefined") return "en";
  const cookie = document.cookie
    .split(";")
    .find((c) => c.trim().startsWith(`${LOCALE_COOKIE}=`));
  const value = cookie ? decodeURIComponent(cookie.split("=")[1]) : "en";
  return normalizeLocale(value);
}

// Locale display names and flags - will be translated by runtime i18n
const getLocaleInfo = (locale) => {
  const locales = {
    "en": { name: "English", flag: "🇺🇸" },
    "vi": { name: "Tiếng Việt", flag: "🇻🇳" },
    "zh-CN": { name: "简体中文", flag: "🇨🇳" },
    "zh-TW": { name: "繁體中文", flag: "🇹🇼" },
    "ja": { name: "日本語", flag: "🇯🇵" },
    "pt-BR": { name: "Português (Brasil)", flag: "🇧🇷" },
    "pt-PT": { name: "Português (Portugal)", flag: "🇵🇹" },
    "ko": { name: "한국어", flag: "🇰🇷" },
    "es": { name: "Español", flag: "🇪🇸" },
    "de": { name: "Deutsch", flag: "🇩🇪" },
    "fr": { name: "Français", flag: "🇫🇷" },
    "he": { name: "עברית", flag: "🇮🇱" },
    "ar": { name: "العربية", flag: "🇸🇦" },
    "ru": { name: "Русский", flag: "🇷🇺" },
    "pl": { name: "Polski", flag: "🇵🇱" },
    "cs": { name: "Čeština", flag: "🇨🇿" },
    "nl": { name: "Nederlands", flag: "🇳🇱" },
    "tr": { name: "Türkçe", flag: "🇹🇷" },
    "uk": { name: "Українська", flag: "🇺🇦" },
    "tl": { name: "Tagalog", flag: "🇵🇭" },
    "id": { name: "Indonesia", flag: "🇮🇩" },
    "th": { name: "ไทย", flag: "🇹🇭" },
    "hi": { name: "हिन्दी", flag: "🇮🇳" },
    "bn": { name: "বাংলা", flag: "🇧🇩" },
    "ur": { name: "اردو", flag: "🇵🇰" },
    "fa": { name: "فارسی", flag: "🇮🇷" },
    "ro": { name: "Română", flag: "🇷🇴" },
    "sv": { name: "Svenska", flag: "🇸🇪" },
    "it": { name: "Italiano", flag: "🇮🇹" },
    "el": { name: "Ελληνικά", flag: "🇬🇷" },
    "hu": { name: "Magyar", flag: "🇭🇺" },
    "fi": { name: "Suomi", flag: "🇫🇮" },
    "da": { name: "Dansk", flag: "🇩🇰" },
    "no": { name: "Norsk", flag: "🇳🇴" }
  };
  return locales[locale] || { name: locale, flag: "🌐" };
};

const triggerSx = {
  display: "flex",
  alignItems: "center",
  gap: 1,
  px: 1.5,
  py: 1,
  borderRadius: "8px",
  border: 0,
  bgcolor: "transparent",
  color: "text.secondary",
  cursor: "pointer",
  transition: "color 150ms, background-color 150ms",
  "&:hover": { color: "text.primary", bgcolor: "var(--color-surface)" },
};

export default function LanguageSwitcher({ className = "", isOpen: controlledOpen, onClose, hideTrigger = false }) {
  const [locale, setLocale] = useState(() => getLocaleFromCookie());
  const [isPending, setIsPending] = useState(false);
  const [internalOpen, setInternalOpen] = useState(false);
  const modalRef = useRef(null);

  const isControlled = typeof controlledOpen === "boolean";
  const isOpen = isControlled ? controlledOpen : internalOpen;
  const setIsOpen = useCallback((value) => {
    if (isControlled) {
      if (!value && onClose) onClose(locale);
    } else {
      setInternalOpen(value);
    }
  }, [isControlled, onClose, locale]);
  const setIsOpenRef = useRef(setIsOpen);
  useEffect(() => {
    setIsOpenRef.current = setIsOpen;
  });

  // Close modal when clicking outside
  useEffect(() => {
    function handleClickOutside(event) {
      if (modalRef.current && !modalRef.current.contains(event.target)) {
        setIsOpenRef.current(false);
      }
    }
    if (isOpen) {
      document.addEventListener("mousedown", handleClickOutside);
      return () => document.removeEventListener("mousedown", handleClickOutside);
    }
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen) return;
    const onEsc = (e) => { if (e.key === "Escape") setIsOpenRef.current(false); };
    document.addEventListener("keydown", onEsc);
    return () => document.removeEventListener("keydown", onEsc);
  }, [isOpen]);

  const handleSetLocale = async (nextLocale) => {
    if (nextLocale === locale || isPending) return;

    setIsPending(true);
    setIsOpen(false);
    try {
      await fetch("/api/locale", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ locale: nextLocale }),
      });

      // Reload translations without full page reload
      await reloadTranslations();
      setLocale(nextLocale);
    } catch (err) {
      console.error("Failed to set locale:", err);
    } finally {
      setIsPending(false);
    }
  };

  // Prevent SSR document reference error
  const [mounted, setMounted] = useState(false);
  useEffect(() => {
    // Intentional client-mount gate for portal/browser-only rendering.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setMounted(true);
  }, []);

  if (!mounted) {
    return (
      <Box className={className}>
        {!hideTrigger && (
          <Box component="button" type="button" disabled title="Language" sx={{ ...triggerSx, opacity: 0.7, cursor: "default" }}>
            <span className="material-symbols-outlined" style={{ fontSize: 20 }}>language</span>
            <Box component="span" sx={{ fontSize: 14, fontWeight: 500 }}>{getLocaleInfo(locale).name}</Box>
          </Box>
        )}
      </Box>
    );
  }

  return (
    <Box className={className}>
      {/* Trigger button */}
      {!hideTrigger && (
        <Box
          component="button"
          type="button"
          onClick={() => setIsOpen(!isOpen)}
          disabled={isPending}
          title="Language"
          data-i18n-skip="true"
          sx={triggerSx}
        >
          <span className="material-symbols-outlined" style={{ fontSize: 20 }}>language</span>
          <Box component="span" sx={{ fontSize: 14, fontWeight: 500 }}>{getLocaleInfo(locale).name}</Box>
          <Box component="span" sx={{ fontSize: 18 }}>{getLocaleInfo(locale).flag}</Box>
        </Box>
      )}

      {/* Portal modal - renders at document.body to avoid parent layout constraints */}
      {isOpen && createPortal(
        <Box sx={{ position: "fixed", inset: 0, zIndex: 50, display: "flex", alignItems: "center", justifyContent: "center", p: 2 }} data-i18n-skip="true">
          {/* Overlay */}
          <Box
            onClick={() => setIsOpen(false)}
            aria-hidden="true"
            sx={{ position: "absolute", inset: 0, bgcolor: "rgba(0,0,0,0.3)", backdropFilter: "blur(4px)" }}
          />

          {/* Modal content */}
          <Box
            ref={modalRef}
            sx={{
              position: "relative",
              width: "100%",
              maxWidth: 672,
              bgcolor: "background.paper",
              border: "1px solid var(--color-border)",
              borderRadius: "12px",
              boxShadow: "0 25px 50px -12px rgba(0,0,0,0.25)",
              display: "flex",
              flexDirection: "column",
              maxHeight: "80vh",
            }}
          >
            {/* Modal header */}
            <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", p: 1.5, borderBottom: "1px solid var(--color-border-subtle)" }}>
              <Typography variant="h6" sx={{ fontSize: 18, fontWeight: 600, color: "text.primary" }}>Select Language</Typography>
              <Box
                component="button"
                type="button"
                onClick={() => setIsOpen(false)}
                aria-label="Close"
                sx={{ p: 0.75, borderRadius: "8px", border: 0, bgcolor: "transparent", color: "text.secondary", cursor: "pointer", "&:hover": { bgcolor: "var(--color-surface-2)" } }}
              >
                <span className="material-symbols-outlined" style={{ fontSize: 20 }}>close</span>
              </Box>
            </Box>

            {/* Modal body - fixed grid columns, equal sizing */}
            <Box sx={{ p: 3, overflowY: "auto", flex: 1 }}>
              <Box sx={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(100px, 1fr))", gap: 1 }}>
                {LOCALES.map((item) => {
                  const active = locale === item;
                  const info = getLocaleInfo(item);
                  return (
                    <Box
                      component="button"
                      type="button"
                      key={item}
                      onClick={() => handleSetLocale(item)}
                      disabled={isPending}
                      title={info.name}
                      sx={{
                        display: "flex",
                        flexDirection: "column",
                        alignItems: "center",
                        justifyContent: "flex-start",
                        gap: 0.5,
                        px: 1,
                        py: 1.5,
                        borderRadius: "8px",
                        border: 0,
                        fontSize: 12,
                        fontWeight: 500,
                        width: "100%",
                        cursor: "pointer",
                        transition: "background-color 150ms, color 150ms",
                        bgcolor: active ? "rgba(229,106,74,0.15)" : "transparent",
                        color: active ? "primary.main" : "text.primary",
                        boxShadow: active ? "0 0 0 2px var(--color-primary)" : "none",
                        "&:hover": { bgcolor: active ? "rgba(229,106,74,0.2)" : "var(--color-surface-2)" },
                        ...(isPending ? { opacity: 0.7, cursor: "wait" } : {}),
                      }}
                    >
                      <Box component="span" sx={{ fontSize: 24 }}>{info.flag}</Box>
                      {/* Fixed 2-line height so all cards are uniform */}
                      <Box component="span" sx={{ textAlign: "center", lineHeight: 1.2, height: 32, display: "flex", alignItems: "center", overflow: "hidden" }}>{info.name}</Box>
                      {active && <span className="material-symbols-outlined" style={{ fontSize: 14 }}>check</span>}
                    </Box>
                  );
                })}
              </Box>
            </Box>
          </Box>
        </Box>,
        document.body
      )}
    </Box>
  );
}
