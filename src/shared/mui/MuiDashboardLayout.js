"use client";

import { useState } from "react";
import Box from "@mui/material/Box";
import { AppRouterCacheProvider } from "@mui/material-nextjs/v16-appRouter";
import MuiProviders from "./MuiProviders";
import MuiSidebar, { DRAWER_WIDTH } from "./MuiSidebar";
import MuiHeader from "./MuiHeader";

// MUI pilot shell for the dashboard chrome. Enabled with
// NEXT_PUBLIC_MUI_SHELL=1 so the Tailwind shell stays as a fallback while the
// migration is reviewed screen by screen.
export default function MuiDashboardLayout({ children }) {
  const [open, setOpen] = useState(false);

  return (
    <AppRouterCacheProvider options={{ key: "mui" }}>
      <MuiProviders>
        <Box sx={{ display: "flex", height: "100vh", overflow: "hidden", bgcolor: "background.default" }}>
          <MuiSidebar open={open} onClose={() => setOpen(false)} />
          <Box
            sx={{
              flex: 1,
              minWidth: 0,
              display: "flex",
              flexDirection: "column",
              width: { lg: `calc(100% - ${DRAWER_WIDTH}px)` },
            }}
          >
            <MuiHeader onMenuClick={() => setOpen(true)} />
            <Box component="main" sx={{ flex: 1, overflowY: "auto", p: { xs: 2, lg: 4 } }}>
              <Box sx={{ maxWidth: 1280, mx: "auto" }}>{children}</Box>
            </Box>
          </Box>
        </Box>
      </MuiProviders>
    </AppRouterCacheProvider>
  );
}
