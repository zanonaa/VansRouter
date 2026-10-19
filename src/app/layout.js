import { Inter } from "next/font/google";
import localFont from "next/font/local";
import "./globals.css";
import { ThemeProvider } from "@/shared/components/ThemeProvider";
import { AppRouterCacheProvider } from "@mui/material-nextjs/v16-appRouter";
import MuiProviders from "@/shared/mui/MuiProviders";
import "@/lib/network/initOutboundProxy"; // Auto-initialize outbound proxy env
import "@/shared/services/bootstrap"; // Auto-run initializeApp (watchdog, auto-resume tunnel)
import { initConsoleLogCapture } from "@/lib/consoleLogBuffer";
import { RuntimeI18nProvider } from "@/i18n/RuntimeI18nProvider";

// Hook console immediately at module load time (server-side only, runs once)
initConsoleLogCapture();

const inter = Inter({
  subsets: ["latin"],
  variable: "--font-inter",
});

const materialSymbols = localFont({
  src: "./fonts/material-symbols-outlined-subset.woff2",
  variable: "--font-material-symbols",
  display: "swap",
});

export const metadata = {
  title: "VansAI - AI Infrastructure Management",
  description: "One endpoint for all your AI providers. Manage keys, monitor usage, and scale effortlessly.",
  icons: {
    icon: "/favicon.svg",
  },
};

export const viewport = {
  themeColor: "#0a0a0a",
};

export default function RootLayout({ children }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body className={`${inter.variable} ${materialSymbols.variable}`}>
        <ThemeProvider>
          <RuntimeI18nProvider>
            {/* MUI is the app-wide UI layer now. The emotion cache + theme live at
                the root so every route (login, landing, dashboard) resolves the
                9Router palette without each page wrapping its own providers. */}
            <AppRouterCacheProvider options={{ key: "mui" }}>
              <MuiProviders>{children}</MuiProviders>
            </AppRouterCacheProvider>
          </RuntimeI18nProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
