import { Suspense } from "react";
import Box from "@mui/material/Box";
import { CardSkeleton } from "@/shared/components/Loading";
import ProviderLimits from "../usage/components/ProviderLimits";

export default function QuotaPage() {
  return (
    <Suspense fallback={<CardSkeleton />}>
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <ProviderLimits />
      </Box>
    </Suspense>
  );
}
