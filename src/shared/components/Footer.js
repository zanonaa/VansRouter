"use client";

import Link from "next/link";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import IconButton from "@mui/material/IconButton";
import { APP_CONFIG } from "@/shared/constants/config";

const footerLinks = {
  product: [
    { label: "Features", href: "#features" },
    { label: "Pricing", href: "#pricing" },
    { label: "Changelog", href: "#" },
  ],
  resources: [
    { label: "Documentation", href: "#" },
    { label: "API Reference", href: "#" },
    { label: "Help Center", href: "#" },
  ],
  company: [
    { label: "About", href: "#" },
    { label: "Blog", href: "#" },
    { label: "Contact", href: "#" },
  ],
};

function FooterColumn({ title, links }) {
  return (
    <Box>
      <Typography variant="subtitle2" sx={{ fontWeight: 600, color: "text.primary", mb: 2 }}>{title}</Typography>
      <Box component="ul" sx={{ display: "flex", flexDirection: "column", gap: 1.5, m: 0, p: 0, listStyle: "none", fontSize: 14, color: "text.secondary", fontWeight: 300 }}>
        {links.map((link) => (
          <Box component="li" key={link.label}>
            <Box component={Link} href={link.href} sx={{ color: "inherit", "&:hover": { color: "primary.main" }, transition: "color 150ms" }}>
              {link.label}
            </Box>
          </Box>
        ))}
      </Box>
    </Box>
  );
}

export default function Footer() {
  return (
    <Box component="footer" sx={{ bgcolor: "background.default", borderTop: "1px solid var(--color-border)", pt: 8, pb: 6 }}>
      <Box sx={{ maxWidth: 1280, mx: "auto", px: { xs: 2, sm: 3, lg: 4 } }}>
        <Box sx={{ display: "grid", gridTemplateColumns: { xs: "repeat(2, 1fr)", md: "repeat(4, 1fr)", lg: "repeat(5, 1fr)" }, gap: 5, mb: 6 }}>
          {/* Brand */}
          <Box sx={{ gridColumn: { xs: "span 2", lg: "span 2" } }}>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1, mb: 3 }}>
              <Box sx={{ width: 24, height: 24, color: "primary.main" }}>
                <svg style={{ width: "100%", height: "100%" }} fill="currentColor" viewBox="0 0 48 48">
                  <path
                    clipRule="evenodd"
                    d="M12.08 24L4 19.25L9.96 8.75L18.04 13.5L18.04 4H29.96L29.96 13.5L38.04 8.75L44 19.25L35.92 24L44 28.75L38.04 39.25L29.96 34.5L29.96 44H18.04L18.04 34.5L9.96 39.25L4 28.75L12.08 24Z"
                    fillRule="evenodd"
                  />
                </svg>
              </Box>
              <Typography sx={{ fontSize: 20, fontWeight: 700, color: "text.primary" }}>
                {APP_CONFIG.name}
              </Typography>
            </Box>
            <Typography sx={{ color: "text.secondary", mb: 3, maxWidth: 384, fontWeight: 300 }}>
              The unified interface for modern AI infrastructure. Secure, observable, and scalable.
            </Typography>
            {/* Social links */}
            <Box sx={{ display: "flex", gap: 2 }}>
              {["Twitter", "GitHub"].map((label, i) => (
                <IconButton key={label} type="button" aria-label={label} sx={{ p: 0, color: "#9ca3af", "&:hover": { color: "primary.main" } }}>
                  <svg style={{ width: 20, height: 20 }} fill="currentColor" viewBox="0 0 24 24">
                    {i === 0 ? (
                      <path d="M22.46 6c-.77.35-1.6.58-2.46.69.88-.53 1.56-1.37 1.88-2.38-.83.5-1.75.85-2.72 1.05C18.37 4.5 17.26 4 16 4c-2.35 0-4.27 1.92-4.27 4.29 0 .34.04.67.11.98C8.28 9.09 5.11 7.38 3 4.79c-.37.63-.58 1.37-.58 2.15 0 1.49.75 2.81 1.91 3.56-.71 0-1.37-.2-1.95-.5v.03c0 2.08 1.48 3.82 3.44 4.21a4.22 4.22 0 0 1-1.93.07 4.28 4.28 0 0 0 4 2.98 8.521 8.521 0 0 1-5.33 1.84c-.34 0-.68-.02-1.02-.06C3.44 20.29 5.7 21 8.12 21 16 21 20.33 14.46 20.33 8.79c0-.19 0-.37-.01-.56.84-.6 1.56-1.36 2.14-2.23z" />
                    ) : (
                      <path d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0 1 12 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0 0 22 12.017C22 6.484 17.522 2 12 2z" />
                    )}
                  </svg>
                </IconButton>
              ))}
            </Box>
          </Box>

          <FooterColumn title="Product" links={footerLinks.product} />
          <FooterColumn title="Resources" links={footerLinks.resources} />
          <FooterColumn title="Company" links={footerLinks.company} />
        </Box>

        {/* Bottom */}
        <Box sx={{ borderTop: "1px solid var(--color-border)", pt: 4, display: "flex", flexDirection: { xs: "column", md: "row" }, justifyContent: "space-between", alignItems: "center", gap: 2 }}>
          <Typography sx={{ fontSize: 14, color: "text.secondary" }}>
            © 2025 {APP_CONFIG.name} Inc. All rights reserved.
          </Typography>
          <Box sx={{ display: "flex", gap: 3, fontSize: 14, color: "text.secondary" }}>
            <Box component={Link} href="#" sx={{ color: "inherit", "&:hover": { color: "primary.main" }, transition: "color 150ms" }}>
              Privacy Policy
            </Box>
            <Box component={Link} href="#" sx={{ color: "inherit", "&:hover": { color: "primary.main" }, transition: "color 150ms" }}>
              Terms of Service
            </Box>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
