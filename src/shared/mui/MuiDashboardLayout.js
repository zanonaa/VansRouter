"use client";

import { useState } from "react";
import Box from "@mui/material/Box";
import MuiSidebar, { DRAWER_WIDTH } from "./MuiSidebar";
import MuiHeader from "./MuiHeader";

// MUI dashboard chrome. The MUI emotion cache + theme provider live in the root
// layout (src/app/layout.js) so they are shared with every route; this component
// only owns the responsive sidebar/header frame.
export default function MuiDashboardLayout({ children }) {
  const [open, setOpen] = useState(false);

  return (
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
  );
}
