"use client";

import { Suspense, useState } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import Box from "@mui/material/Box";
import { RequestLogger, CardSkeleton, SegmentedControl } from "@/shared/components";
import UsageStats from "@/shared/components/UsageStats";
import RequestDetailsTab from "./components/RequestDetailsTab";

const PERIODS = [
  { value: "today", label: "Today" },
  { value: "24h", label: "24h" },
  { value: "7d", label: "7D" },
  { value: "30d", label: "30D" },
  { value: "60d", label: "60D" },
  { value: "all", label: "All" },
];

export default function UsagePage() {
  return (
    <Suspense fallback={<CardSkeleton />}>
      <UsageContent />
    </Suspense>
  );
}

function UsageContent() {
  const searchParams = useSearchParams();
  const router = useRouter();

  const [period, setPeriod] = useState("today");

  const tabFromUrl = searchParams.get("tab");
  const activeTab = tabFromUrl && ["overview", "logs", "details"].includes(tabFromUrl)
    ? tabFromUrl
    : "overview";

  const handleTabChange = (value) => {
    if (value === activeTab) return;
    const params = new URLSearchParams(searchParams);
    params.set("tab", value);
    router.push(`/dashboard/usage?${params.toString()}`, { scroll: false });
  };

  const controlWrapperSx = {
    display: "flex",
    width: { xs: "100%", sm: "auto" },
    "& > *": { width: { xs: "100%", sm: "auto" } },
  };

  return (
    <Box sx={{ display: "flex", minWidth: 0, flexDirection: "column", gap: 3, px: { xs: 0.5, sm: 0 } }}>
      {/* Tabs + period selector on same row */}
      <Box
        sx={{
          display: "flex",
          flexDirection: { xs: "column", sm: "row" },
          gap: 1,
          alignItems: { sm: "center" },
          justifyContent: { sm: "space-between" },
        }}
      >
        <Box sx={controlWrapperSx}>
          <SegmentedControl
            options={[
              { value: "overview", label: "Overview" },
              { value: "details", label: "Details" },
            ]}
            value={activeTab}
            onChange={handleTabChange}
          />
        </Box>
        {activeTab === "overview" && (
          <Box sx={controlWrapperSx}>
            <SegmentedControl
              options={PERIODS}
              value={period}
              onChange={setPeriod}
              size="sm"
            />
          </Box>
        )}
      </Box>

      {activeTab === "overview" && (
        <Suspense fallback={<CardSkeleton />}>
          <UsageStats period={period} setPeriod={setPeriod} hidePeriodSelector />
        </Suspense>
      )}
      {activeTab === "logs" && <RequestLogger />}
      {activeTab === "details" && <RequestDetailsTab />}
    </Box>
  );
}
