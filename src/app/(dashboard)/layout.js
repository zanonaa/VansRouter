import { DashboardLayout } from "@/shared/components";
import MuiDashboardLayout from "@/shared/mui/MuiDashboardLayout";

// NEXT_PUBLIC_MUI_SHELL=1 swaps the dashboard chrome for the MUI pilot without
// changing any page. Unset (or 0) keeps the Tailwind shell.
const USE_MUI_SHELL = process.env.NEXT_PUBLIC_MUI_SHELL === "1";

export default function DashboardRootLayout({ children }) {
  if (USE_MUI_SHELL) {
    return <MuiDashboardLayout>{children}</MuiDashboardLayout>;
  }
  return <DashboardLayout>{children}</DashboardLayout>;
}
