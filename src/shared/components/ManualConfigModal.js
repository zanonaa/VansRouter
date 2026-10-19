"use client";

import { useState } from "react";
import Box from "@mui/material/Box";
import Modal from "./Modal";
import Button from "./Button";
import { useCopyToClipboard } from "@/shared/hooks/useCopyToClipboard";

const EMPTY_CONFIGS = [];

export default function ManualConfigModal({ isOpen, onClose, title = "Manual Configuration", configs = EMPTY_CONFIGS }) {
  const { copy } = useCopyToClipboard();
  const [copiedIndex, setCopiedIndex] = useState(null);

  const copyConfig = (text, index) => {
    copy(text, `manualconfig-${index}`);
    setCopiedIndex(index);
    setTimeout(() => setCopiedIndex(null), 2000);
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={title} size="xl">
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {configs.map((config, index) => (
          <Box key={config.filename} sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
              <Box component="span" sx={{ fontSize: 14, fontWeight: 500, color: "text.primary" }}>{config.filename}</Box>
              <Button variant="ghost" size="sm" onClick={() => copyConfig(config.content, index)}>
                <span className="material-symbols-outlined" style={{ fontSize: 14, marginRight: 4 }}>
                  {copiedIndex === index ? "check" : "content_copy"}
                </span>
                {copiedIndex === index ? "Copied!" : "Copy"}
              </Button>
            </Box>
            <Box
              component="pre"
              sx={{
                px: 1.5,
                py: 1,
                bgcolor: "rgba(0,0,0,0.05)",
                borderRadius: "4px",
                fontFamily: "ui-monospace, monospace",
                fontSize: 12,
                overflowX: "auto",
                whiteSpace: "pre-wrap",
                wordBreak: "break-all",
                maxHeight: 240,
                overflowY: "auto",
                border: "1px solid var(--color-border)",
                m: 0,
              }}
            >
              {config.content}
            </Box>
          </Box>
        ))}
      </Box>
    </Modal>
  );
}
