"use client";

import Link from "next/link";
import { Box, Button, Container, Stack, Typography } from "@mui/material";

export default function GetStarted() {
  return (
    <Box component="section" sx={{ py: 12, px: 3, bgcolor: "#0f0d1a", color: "#fff" }}>
      <Container maxWidth="lg">
        <Stack spacing={4} alignItems="center" textAlign="center">
          <Typography variant="h3" component="h2" sx={{ fontWeight: 700 }}>
            Get Started
          </Typography>
          <Typography color="grey.400" sx={{ maxWidth: 640 }}>
            Run the web gateway with Docker, open the dashboard, and configure providers and API keys.
          </Typography>
          <Box
            component="pre"
            sx={{
              width: "100%",
              maxWidth: 720,
              p: 3,
              borderRadius: 2,
              bgcolor: "#1e1e1e",
              border: "1px solid #3a2f27",
              color: "#d4d4d4",
              textAlign: "left",
              overflowX: "auto",
              fontSize: 13,
            }}
          >{`docker run -d -p 20128:20128 -v vansrouter-data:/app/data \\
  -e INITIAL_PASSWORD='change-this-password' \\
  ghcr.io/zanonaa/vansrouter:1.2.0`}</Box>
          <Stack direction={{ xs: "column", sm: "row" }} spacing={2} alignItems="center">
            <Button component={Link} href="/masuk" variant="contained">Open Dashboard</Button>
            <Typography variant="body2" color="grey.500">
              Gateway endpoint: http://localhost:20128/v1
            </Typography>
          </Stack>
        </Stack>
      </Container>
    </Box>
  );
}
