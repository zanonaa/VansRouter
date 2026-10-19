"use client";

import Paper from "@mui/material/Paper";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import { cn } from "@/shared/utils/cn";

const CARD_PADDINGS = {
  none: 0,
  xs: 1.5,
  sm: 2,
  md: 3,
  lg: 4,
};

function MsIcon({ children }) {
  return (
    <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 20 }}>
      {children}
    </Icon>
  );
}

// MUI-backed Card. Same API (title, subtitle, icon, action, padding, hover, elev).
export default function Card({
  children,
  title,
  subtitle,
  icon,
  action,
  padding = "md",
  hover = false,
  elev = false,
  className,
  sx,
  ...props
}) {
  return (
    <Paper
      elevation={0}
      variant="outlined"
      className={cn(className)}
      sx={{
        p: CARD_PADDINGS[padding] ?? 3,
        borderRadius: "14px",
        bgcolor: "background.paper",
        borderColor: "divider",
        boxShadow: elev ? "var(--shadow-elev)" : "var(--shadow-soft)",
        transition: "box-shadow 150ms ease, border-color 150ms ease",
        ...(hover
          ? {
              cursor: "pointer",
              "&:hover": { boxShadow: "var(--shadow-warm)", borderColor: "primary.main" },
            }
          : {}),
        ...sx,
      }}
      {...props}
    >
      {(title || action) && (
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", mb: 2 }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, minWidth: 0 }}>
            {icon && (
              <Box sx={{ p: 1, borderRadius: "10px", bgcolor: "action.hover", color: "text.secondary", display: "grid", placeItems: "center" }}>
                <MsIcon>{icon}</MsIcon>
              </Box>
            )}
            <Box sx={{ minWidth: 0 }}>
              {title && (
                <Typography variant="subtitle1" sx={{ fontWeight: 600, color: "text.primary" }}>
                  {title}
                </Typography>
              )}
              {subtitle && (
                <Typography variant="body2" color="text.secondary">
                  {subtitle}
                </Typography>
              )}
            </Box>
          </Box>
          {action}
        </Box>
      )}
      {children}
    </Paper>
  );
}

Card.Section = function CardSection({ children, className, ...props }) {
  return (
    <Box
      className={cn(className)}
      sx={{ p: 2, borderRadius: "10px", bgcolor: "action.hover", border: 1, borderColor: "divider" }}
      {...props}
    >
      {children}
    </Box>
  );
};

Card.Row = function CardRow({ children, className, ...props }) {
  return (
    <Box
      className={cn(className)}
      sx={{
        p: 1.5,
        borderBottom: 1,
        borderColor: "divider",
        transition: "background-color 150ms ease",
        "&:last-of-type": { borderBottom: 0 },
        "&:hover": { bgcolor: "action.hover" },
      }}
      {...props}
    >
      {children}
    </Box>
  );
};

Card.ListItem = function CardListItem({ children, actions, className, ...props }) {
  return (
    <Box
      className={cn("group", className)}
      sx={{
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        p: 1.5,
        borderBottom: 1,
        borderColor: "divider",
        transition: "background-color 150ms ease",
        "&:last-of-type": { borderBottom: 0 },
        "&:hover": { bgcolor: "action.hover" },
      }}
      {...props}
    >
      <Box sx={{ flex: 1, minWidth: 0 }}>{children}</Box>
      {actions && (
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 0.5,
            opacity: 0,
            transition: "opacity 150ms ease",
            ".group:hover &": { opacity: 1 },
          }}
        >
          {actions}
        </Box>
      )}
    </Box>
  );
};
