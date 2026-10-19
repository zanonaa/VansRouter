"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import Box from "@mui/material/Box";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import MuiButton from "@mui/material/Button";
import Card from "@/shared/components/Card";
import PricingModal from "@/shared/components/PricingModal";

export default function PricingSettingsPage() {
  const router = useRouter();
  const [showModal, setShowModal] = useState(false);
  const [currentPricing, setCurrentPricing] = useState(null);
  const [loading, setLoading] = useState(true);

  /* eslint-disable react-hooks/immutability --
     One-time bootstrap fetch on mount; loadPricing is declared below. */
  useEffect(() => {
    loadPricing();
  }, []);

  const loadPricing = async () => {
    setLoading(true);
    try {
      const response = await fetch("/api/pricing");
      if (response.ok) {
        const data = await response.json();
        setCurrentPricing(data);
      }
    } catch (error) {
      console.error("Failed to load pricing:", error);
    } finally {
      setLoading(false);
    }
  };

  const handlePricingUpdated = () => {
    loadPricing();
  };

  // Count total models with pricing
  const getModelCount = () => {
    if (!currentPricing) return 0;
    let count = 0;
    for (const provider in currentPricing) {
      count += Object.keys(currentPricing[provider]).length;
    }
    return count;
  };

  // Get providers list
  const getProviders = () => {
    if (!currentPricing) return [];
    return Object.keys(currentPricing).sort();
  };

  return (
    <Box sx={{ maxWidth: 1152, mx: "auto", p: 3 }}>
      <Stack spacing={3}>
        {/* Header */}
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
          <Box>
            <Typography sx={{ fontSize: 30, fontWeight: 700, lineHeight: 1.2 }}>Pricing Settings</Typography>
            <Typography sx={{ color: "text.secondary", mt: 0.5 }}>
              Configure pricing rates for cost tracking and calculations
            </Typography>
          </Box>
          <MuiButton
            variant="contained"
            disableElevation
            onClick={() => setShowModal(true)}
          >
            Edit Pricing
          </MuiButton>
        </Box>

        {/* Quick Stats */}
        <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", md: "repeat(3, 1fr)" }, gap: 2 }}>
          <Card padding="sm">
            <Typography sx={{ color: "text.secondary", fontSize: 14, textTransform: "uppercase", fontWeight: 600 }}>
              Total Models
            </Typography>
            <Typography sx={{ fontSize: 24, fontWeight: 700, mt: 0.5 }}>
              {loading ? "..." : getModelCount()}
            </Typography>
          </Card>
          <Card padding="sm">
            <Typography sx={{ color: "text.secondary", fontSize: 14, textTransform: "uppercase", fontWeight: 600 }}>
              Providers
            </Typography>
            <Typography sx={{ fontSize: 24, fontWeight: 700, mt: 0.5 }}>
              {loading ? "..." : getProviders().length}
            </Typography>
          </Card>
          <Card padding="sm">
            <Typography sx={{ color: "text.secondary", fontSize: 14, textTransform: "uppercase", fontWeight: 600 }}>
              Status
            </Typography>
            <Typography sx={{ fontSize: 24, fontWeight: 700, mt: 0.5, color: "success.main" }}>
              {loading ? "..." : "Active"}
            </Typography>
          </Card>
        </Box>

        {/* Info Section */}
        <Card padding="md">
          <Typography sx={{ fontSize: 20, fontWeight: 600, mb: 2 }}>How Pricing Works</Typography>
          <Stack spacing={1.5} sx={{ fontSize: 14, color: "text.secondary" }}>
            <Typography variant="inherit" sx={{ fontSize: 14, color: "text.secondary" }}>
              <strong>Cost Calculation:</strong> Costs are calculated based on token usage and pricing rates.
              Each request&apos;s cost is determined by: (input_tokens × input_rate) + (output_tokens × output_rate) + (cached_tokens × cached_rate)
            </Typography>
            <Typography variant="inherit" sx={{ fontSize: 14, color: "text.secondary" }}>
              <strong>Pricing Format:</strong> All rates are in <strong>dollars per million tokens</strong> ($/1M tokens).
              Example: An input rate of 2.50 means $2.50 per 1,000,000 input tokens.
            </Typography>
            <Typography variant="inherit" sx={{ fontSize: 14, color: "text.secondary" }}>
              <strong>Token Types:</strong>
            </Typography>
            <Box component="ul" sx={{ listStyle: "disc", listStylePosition: "inside", ml: 2, "& li": { mb: 0.5 } }}>
              <li><strong>Input:</strong> Standard prompt tokens</li>
              <li><strong>Output:</strong> Completion/response tokens</li>
              <li><strong>Cached:</strong> Cached input tokens (typically 50% of input rate)</li>
              <li><strong>Reasoning:</strong> Special reasoning/thinking tokens (fallback to output rate)</li>
              <li><strong>Cache Creation:</strong> Tokens used to create cache entries (fallback to input rate)</li>
            </Box>
            <Typography variant="inherit" sx={{ fontSize: 14, color: "text.secondary" }}>
              <strong>Custom Pricing:</strong> You can override default pricing for specific models.
              Reset to defaults anytime to restore standard rates.
            </Typography>
          </Stack>
        </Card>

        {/* Current Pricing Preview */}
        <Card padding="md">
          <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", mb: 2 }}>
            <Typography sx={{ fontSize: 20, fontWeight: 600 }}>Current Pricing Overview</Typography>
            <MuiButton
              variant="text"
              size="small"
              onClick={() => setShowModal(true)}
              sx={{ color: "primary.main", textTransform: "none", "&:hover": { textDecoration: "underline", bgcolor: "transparent" } }}
            >
              View Full Details
            </MuiButton>
          </Box>

          {loading ? (
            <Box sx={{ textAlign: "center", py: 2, color: "text.secondary" }}>Loading pricing data...</Box>
          ) : currentPricing ? (
            <Stack spacing={1.5}>
              {Object.keys(currentPricing).slice(0, 5).map((provider) => (
                <Typography key={provider} sx={{ fontSize: 14 }}>
                  <Box component="span" sx={{ fontWeight: 600 }}>{provider.toUpperCase()}:</Box>{" "}
                  <Box component="span" sx={{ color: "text.secondary" }}>
                    {Object.keys(currentPricing[provider]).length} models
                  </Box>
                </Typography>
              ))}
              {Object.keys(currentPricing).length > 5 && (
                <Typography sx={{ fontSize: 14, color: "text.secondary" }}>
                  + {Object.keys(currentPricing).length - 5} more providers
                </Typography>
              )}
            </Stack>
          ) : (
            <Typography sx={{ color: "text.secondary" }}>No pricing data available</Typography>
          )}
        </Card>

        {/* Pricing Modal */}
        {showModal && (
          <PricingModal
            isOpen={showModal}
            onClose={() => setShowModal(false)}
            onSave={handlePricingUpdated}
          />
        )}
      </Stack>
    </Box>
  );
}
