"use client";

import { useState } from "react";
import Box from "@mui/material/Box";
import IconButton from "@mui/material/IconButton";
import { LOCALE_COOKIE, normalizeLocale } from "@/i18n/config";
import { LOCALE_FLAGS } from "@/shared/constants/locales";
import LanguageSwitcher from "./LanguageSwitcher";

function getLocaleFromCookie() {
  if (typeof document === "undefined") return "en";
  const cookie = document.cookie
    .split(";")
    .find((c) => c.trim().startsWith(`${LOCALE_COOKIE}=`));
  const value = cookie ? decodeURIComponent(cookie.split("=")[1]) : "en";
  return normalizeLocale(value);
}

export default function HeaderLanguage() {
  const [open, setOpen] = useState(false);
  const locale = getLocaleFromCookie();

  return (
    <>
      <IconButton
        type="button"
        onClick={() => setOpen(true)}
        title="Language"
        data-i18n-skip="true"
        sx={{ p: 1, borderRadius: "8px", color: "text.secondary", "&:hover": { color: "text.primary", bgcolor: "var(--color-surface-2)" } }}
      >
        <Box component="span" sx={{ fontSize: 18, lineHeight: 1 }}>{LOCALE_FLAGS[locale] || "🌐"}</Box>
      </IconButton>

      <LanguageSwitcher
        hideTrigger
        isOpen={open}
        onClose={() => setOpen(false)}
      />
    </>
  );
}
