"use client";

import { useEffect, useRef } from "react";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import IconButton from "@mui/material/IconButton";
import { cn } from "@/shared/utils/cn";
import Button from "./Button";
import Tooltip from "./Tooltip";

const MODAL_SIZES = {
  sm: 384,
  md: 448,
  lg: 512,
  xl: 576,
  full: 896,
};

export default function Modal({
  isOpen,
  onClose,
  title,
  children,
  footer,
  size = "md",
  closeOnOverlay = true,
  showTrafficLights = true,
  className,
}) {
  const maxWidth = MODAL_SIZES[size] || MODAL_SIZES.md;
  const onCloseRef = useRef(onClose);
  useEffect(() => {
    onCloseRef.current = onClose;
  });

  useEffect(() => {
    if (isOpen) {
      document.body.style.overflow = "hidden";
    } else {
      document.body.style.overflow = "";
    }
    return () => { document.body.style.overflow = ""; };
  }, [isOpen]);

  useEffect(() => {
    const handleEscape = (e) => {
      if (e.key === "Escape" && isOpen) onCloseRef.current();
    };
    document.addEventListener("keydown", handleEscape);
    return () => document.removeEventListener("keydown", handleEscape);
  }, [isOpen]);

  if (!isOpen) return null;

  return (
    <Box sx={{ position: "fixed", inset: 0, zIndex: 50, display: "flex", alignItems: "center", justifyContent: "center", p: 2 }}>
      {/* Overlay */}
      <Box
        onClick={closeOnOverlay ? onClose : undefined}
        aria-hidden="true"
        sx={{ position: "absolute", inset: 0, bgcolor: "rgba(0,0,0,0.5)", backdropFilter: "blur(2px)", animation: "fadeIn 0.2s ease-out forwards" }}
      />

      {/* Modal content */}
      <Box
        className={cn(className)}
        sx={{
          position: "relative",
          width: "100%",
          maxWidth,
          bgcolor: "background.paper",
          border: "1px solid var(--color-border-subtle)",
          borderRadius: "14px",
          boxShadow: "var(--shadow-elev)",
          animation: "fadeIn 0.2s ease-out forwards",
        }}
      >
        {/* Header */}
        {(title || showTrafficLights) && (
          <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", p: 1, borderBottom: "1px solid var(--color-border-subtle)" }}>
            <Box sx={{ display: "flex", alignItems: "center" }}>
              {/* Traffic lights — desktop only */}
              {showTrafficLights && (
                <Box sx={{ display: { xs: "none", md: "flex" }, alignItems: "center", gap: 1, mr: 2, ml: 1 }}>
                  <Tooltip text="Close" position="top" color="#FF5F56">
                    <Box
                      component="button"
                      type="button"
                      onClick={onClose}
                      aria-label="Close"
                      title="Close"
                      sx={{
                        width: 16,
                        height: 16,
                        borderRadius: "50%",
                        bgcolor: "#FF5F56",
                        border: 0,
                        p: 0,
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "center",
                        cursor: "pointer",
                        "&:hover": { filter: "brightness(0.9)" },
                        "& .x": { fontSize: 9, fontWeight: 700, color: "#fff", opacity: 0, transition: "opacity 150ms", lineHeight: 1 },
                        "&:hover .x": { opacity: 1 },
                      }}
                    >
                      <span className="x">✕</span>
                    </Box>
                  </Tooltip>
                  <Box sx={{ width: 16, height: 16, borderRadius: "50%", bgcolor: "rgba(58,58,58,0.2)", cursor: "not-allowed" }} />
                  <Box sx={{ width: 16, height: 16, borderRadius: "50%", bgcolor: "rgba(58,58,58,0.2)", cursor: "not-allowed" }} />
                </Box>
              )}
              {title && (
                <Typography variant="h6" sx={{ fontSize: 18, fontWeight: 600, color: "text.primary" }}>{title}</Typography>
              )}
            </Box>
            {/* X button — mobile only */}
            <IconButton
              type="button"
              onClick={onClose}
              aria-label="Close"
              sx={{ display: { md: "none" }, p: 0.75, borderRadius: "10px", color: "text.secondary", "&:hover": { bgcolor: "var(--color-surface-2)", color: "text.primary" } }}
            >
              <span className="material-symbols-outlined" style={{ fontSize: 20 }}>close</span>
            </IconButton>
          </Box>
        )}

        {/* Body */}
        <Box className="custom-scrollbar" sx={{ p: 3, maxHeight: "calc(85vh - 100px)", overflowY: "auto" }}>{children}</Box>

        {/* Footer */}
        {footer && (
          <Box sx={{ display: "flex", alignItems: "center", justifyContent: "flex-end", gap: 1.5, p: 3, borderTop: "1px solid var(--color-border-subtle)" }}>
            {footer}
          </Box>
        )}
      </Box>
    </Box>
  );
}

export function ConfirmModal({
  isOpen,
  onClose,
  onConfirm,
  title = "Confirm",
  message,
  confirmText = "Confirm",
  cancelText = "Cancel",
  variant = "danger",
  loading = false,
}) {
  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={title}
      size="sm"
      footer={
        <>
          <Button variant="ghost" onClick={onClose} disabled={loading}>
            {cancelText}
          </Button>
          <Button variant={variant} onClick={onConfirm} loading={loading}>
            {confirmText}
          </Button>
        </>
      }
    >
      <Typography sx={{ color: "text.secondary" }}>{message}</Typography>
    </Modal>
  );
}
