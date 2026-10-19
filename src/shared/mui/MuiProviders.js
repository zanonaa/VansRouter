"use client";

import { useEffect, useMemo, useState } from "react";
import { ThemeProvider } from "@mui/material/styles";
import CssBaseline from "@mui/material/CssBaseline";
import { getMuiTheme } from "./theme";

// MUI does not read the app's theme store; mirror the `.dark` class that the
// existing ThemeProvider toggles on <html>.
function useDocumentMode() {
  const [mode, setMode] = useState("light");

  useEffect(() => {
    const read = () =>
      setMode(document.documentElement.classList.contains("dark") ? "dark" : "light");
    read();
    const observer = new MutationObserver(read);
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });
    return () => observer.disconnect();
  }, []);

  return mode;
}

export default function MuiProviders({ children }) {
  const mode = useDocumentMode();
  const theme = useMemo(() => getMuiTheme(mode), [mode]);

  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      {children}
    </ThemeProvider>
  );
}
