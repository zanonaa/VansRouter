"use client";

import Box from "@mui/material/Box";
import Button from "./Button";

const pageBtnStyle = { width: 36, paddingInline: 0, minWidth: 36 };

export default function Pagination({
  currentPage,
  pageSize,
  totalItems,
  onPageChange,
  onPageSizeChange,
  className,
}) {
  const totalPages = Math.ceil(totalItems / pageSize);
  const startItem = totalItems > 0 ? (currentPage - 1) * pageSize + 1 : 0;
  const endItem = Math.min(currentPage * pageSize, totalItems);

  const getPageNumbers = () => {
    const pages = [];
    const showMax = 5;

    let start = Math.max(1, currentPage - 2);
    let end = Math.min(totalPages, start + showMax - 1);

    if (end - start + 1 < showMax) {
      start = Math.max(1, end - showMax + 1);
    }

    for (let i = start; i <= end; i++) {
      pages.push(i);
    }
    return pages;
  };

  const pageNumbers = getPageNumbers();

  return (
    <Box
      className={className}
      sx={{
        display: "flex",
        flexDirection: { xs: "column", sm: "row" },
        alignItems: "center",
        justifyContent: "space-between",
        gap: 2,
        py: 2,
        px: 1,
      }}
    >
      {/* Info text */}
      {totalItems > 0 && (
        <Box sx={{ fontSize: 14, color: "text.secondary" }}>
          Showing <Box component="span" sx={{ fontWeight: 500, color: "text.primary" }}>{startItem}</Box> to{" "}
          <Box component="span" sx={{ fontWeight: 500, color: "text.primary" }}>{endItem}</Box> of{" "}
          <Box component="span" sx={{ fontWeight: 500, color: "text.primary" }}>{totalItems}</Box> results
        </Box>
      )}

      <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", justifyContent: "center", gap: { xs: 1, sm: 2 } }}>
        {/* Page size selector */}
        {onPageSizeChange && (
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <Box component="span" sx={{ fontSize: 14, color: "text.secondary" }}>Rows:</Box>
            <Box
              component="select"
              value={pageSize}
              onChange={(e) => onPageSizeChange(Number(e.target.value))}
              sx={{
                height: 36,
                borderRadius: "8px",
                border: "1px solid var(--color-border)",
                bgcolor: "background.paper",
                fontSize: 14,
                color: "text.primary",
                cursor: "pointer",
                outline: "none",
                "&:focus": { boxShadow: "0 0 0 2px rgba(229,106,74,0.2)" },
              }}
              style={{ colorScheme: "auto" }}
            >
              {[10, 20, 50].map((size) => (
                <option key={size} value={size}>
                  {size}
                </option>
              ))}
            </Box>
          </Box>
        )}

        {totalPages > 1 && (
          <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
            <Button
              variant="outline"
              size="sm"
              onClick={() => onPageChange(currentPage - 1)}
              disabled={currentPage === 1}
              style={pageBtnStyle}
            >
              <span className="material-symbols-outlined" style={{ fontSize: 18 }}>chevron_left</span>
            </Button>

            {pageNumbers[0] > 1 && (
              <>
                <Box sx={{ display: { xs: "none", sm: "inline-flex" } }}>
                  <Button variant="ghost" size="sm" onClick={() => onPageChange(1)} style={pageBtnStyle}>
                    1
                  </Button>
                </Box>
                {pageNumbers[0] > 2 && (
                  <Box component="span" sx={{ color: "text.secondary", px: 0.5, display: { xs: "none", sm: "inline" } }}>...</Box>
                )}
              </>
            )}

            {pageNumbers.map((page) => {
              const isActive = currentPage === page;
              return (
                <Box key={page} sx={{ display: isActive ? "inline-flex" : { xs: "none", sm: "inline-flex" } }}>
                  <Button
                    variant={isActive ? "primary" : "ghost"}
                    size="sm"
                    onClick={() => onPageChange(page)}
                    style={pageBtnStyle}
                  >
                    {page}
                  </Button>
                </Box>
              );
            })}

            {pageNumbers[pageNumbers.length - 1] < totalPages && (
              <>
                {pageNumbers[pageNumbers.length - 1] < totalPages - 1 && (
                  <Box component="span" sx={{ color: "text.secondary", px: 0.5, display: { xs: "none", sm: "inline" } }}>...</Box>
                )}
                <Box sx={{ display: { xs: "none", sm: "inline-flex" } }}>
                  <Button variant="ghost" size="sm" onClick={() => onPageChange(totalPages)} style={pageBtnStyle}>
                    {totalPages}
                  </Button>
                </Box>
              </>
            )}

            <Button
              variant="outline"
              size="sm"
              onClick={() => onPageChange(currentPage + 1)}
              disabled={currentPage === totalPages}
              style={pageBtnStyle}
            >
              <span className="material-symbols-outlined" style={{ fontSize: 18 }}>chevron_right</span>
            </Button>
          </Box>
        )}
      </Box>
    </Box>
  );
}
