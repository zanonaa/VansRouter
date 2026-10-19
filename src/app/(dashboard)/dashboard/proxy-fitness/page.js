"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Badge, Button, Card, CardSkeleton, Input, ConfirmModal } from "@/shared/components";
import { useNotificationStore } from "@/store/notificationStore";
import Box from "@mui/material/Box";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import Icon from "@mui/material/Icon";
import Select from "@mui/material/Select";
import MenuItem from "@mui/material/MenuItem";
import Table from "@mui/material/Table";
import TableContainer from "@mui/material/TableContainer";
import TableHead from "@mui/material/TableHead";
import TableBody from "@mui/material/TableBody";
import TableRow from "@mui/material/TableRow";
import TableCell from "@mui/material/TableCell";
import { alpha } from "@mui/material/styles";

function MsIcon({ children, sx, ...props }) {
  return (
    <Icon className="material-symbols-outlined" baseClassName="" sx={{ fontSize: 18, ...sx }} {...props}>
      {children}
    </Icon>
  );
}

function recordsOf(fitness, pools, now = Date.now()) {
  const names = new Map((pools || []).map((p) => [p.id, p]));
  return Object.entries(fitness || {}).flatMap(([poolId, scopes]) => Object.entries(scopes || {}).flatMap(([scope, info]) => {
    const until = Number(info?.until || 0);
    if (until <= now) return [];
    const [provider, model] = String(scope).split("::");
    const pool = names.get(poolId);
    return [{ poolId, scope, provider, model: model === "*" ? "all models" : model, until, reason: info?.reason || "blocked", poolName: pool?.name || poolId.slice(0, 8), proxyUrl: pool?.proxyUrl || "" }];
  }));
}

export default function ProxyFitnessPage() {
  const [pools, setPools] = useState([]);
  const [fitness, setFitness] = useState({});
  const [loading, setLoading] = useState(true);
  const [provider, setProvider] = useState("all");
  const [search, setSearch] = useState("");
  const [confirm, setConfirm] = useState(false);
  const notify = useNotificationStore();
  const fetchAll = useCallback(async () => {
    try {
      const [p, f] = await Promise.all([fetch("/api/proxy-pools?includeUsage=true", { cache: "no-store" }), fetch("/api/proxy-pools/fitness", { cache: "no-store" })]);
      setPools((await p.json()).proxyPools || []);
      setFitness(f.ok ? ((await f.json()).pools || {}) : {});
    } catch {
      // Keep the empty state when the dashboard APIs are unavailable.
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    fetchAll();
  }, [fetchAll]);
  const records = useMemo(() => recordsOf(fitness, pools).filter((r) => (provider === "all" || r.provider === provider) && `${r.proxyUrl} ${r.poolName} ${r.model}`.toLowerCase().includes(search.toLowerCase())), [fitness, pools, provider, search]);
  const providers = useMemo(() => [...new Set(recordsOf(fitness, pools).map((r) => r.provider))].sort(), [fitness, pools]);
  const clearAll = async () => {
    await fetch("/api/proxy-pools/fitness/clear-all", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(provider === "all" ? {} : { provider }) });
    setConfirm(false); notify.success("Proxy fitness cleared"); fetchAll();
  };
  return (
    <Box sx={{ mx: "auto", display: "flex", width: "100%", maxWidth: 1024, flexDirection: "column", gap: { xs: 2, sm: 3 }, px: { xs: 0.5, sm: 0 } }}>
      {loading ? (
        <CardSkeleton />
      ) : (
        <>
          <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1.5, alignItems: { sm: "flex-start" }, justifyContent: { sm: "space-between" } }}>
            <Box sx={{ minWidth: 0 }}>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Typography component="h1" variant="h5" sx={{ fontWeight: 600, fontSize: { xs: 20, sm: 24 } }}>
                  Proxy Fitness
                </Typography>
                <Badge variant={records.length ? "error" : "default"}>
                  {records.length} active blocks
                </Badge>
              </Box>
              <Typography sx={{ fontSize: 14, color: "text.secondary", mt: 0.5 }}>
                Smart rotation skips pools marked unfit for a provider/model.
              </Typography>
            </Box>
            <Box sx={{ display: "flex", gap: 1, alignItems: { sm: "center" } }}>
              <Button variant="secondary" size="sm" onClick={fetchAll} icon="refresh">
                Refresh
              </Button>
              {records.length > 0 && (
                <Button variant="danger" size="sm" onClick={() => setConfirm(true)} icon="clear_all">
                  Clear All
                </Button>
              )}
            </Box>
          </Box>

          <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 2, alignItems: { sm: "center" }, justifyContent: { sm: "space-between" } }}>
            <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 1.5 }}>
              <Select
                value={provider}
                onChange={(e) => setProvider(e.target.value)}
                size="small"
                sx={{
                  height: 42,
                  minWidth: 160,
                  fontSize: 14,
                  bgcolor: "action.hover",
                  borderRadius: "10px",
                  "& fieldset": { border: "1px solid transparent" },
                  "&:hover fieldset": { borderColor: "transparent" },
                  "&.Mui-focused fieldset": { borderColor: "primary.main" },
                }}
              >
                <MenuItem value="all">All providers</MenuItem>
                {providers.map((p) => (
                  <MenuItem key={p} value={p}>{p}</MenuItem>
                ))}
              </Select>
              <Box sx={{ width: { xs: "100%", sm: 288 } }}>
                <Input
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  placeholder="Search IP / proxy / pool..."
                  icon="search"
                />
              </Box>
            </Box>
          </Box>

          <Card padding="none" sx={{ overflowX: "auto" }}>
            <TableContainer>
            <Table sx={{ minWidth: 650 }}>
              <TableHead>
                <TableRow sx={{ "& th": { borderBottomColor: "divider" } }}>
                  <TableCell sx={{ px: 2.5, py: 1.5, fontSize: 12, textTransform: "uppercase", color: "text.secondary" }}>Provider</TableCell>
                  <TableCell sx={{ px: 2.5, py: 1.5, fontSize: 12, textTransform: "uppercase", color: "text.secondary" }}>Model</TableCell>
                  <TableCell sx={{ px: 2.5, py: 1.5, fontSize: 12, textTransform: "uppercase", color: "text.secondary" }}>Pool</TableCell>
                  <TableCell sx={{ px: 2.5, py: 1.5, fontSize: 12, textTransform: "uppercase", color: "text.secondary" }}>Reason</TableCell>
                  <TableCell sx={{ px: 2.5, py: 1.5, fontSize: 12, textTransform: "uppercase", color: "text.secondary" }}>Until</TableCell>
                  <TableCell sx={{ px: 2.5, py: 1.5 }} align="right" />
                </TableRow>
              </TableHead>
              <TableBody>
                {records.length ? (
                  records.map((r) => (
                    <TableRow
                      key={`${r.poolId}:${r.scope}`}
                      hover
                      sx={{ "& td": { borderBottomColor: "divider" } }}
                    >
                      <TableCell sx={{ px: 2.5, py: 1.75, fontWeight: 500, fontSize: 14 }}>{r.provider}</TableCell>
                      <TableCell sx={{ px: 2.5, py: 1.75 }}>
                        <Box component="code" sx={{ px: 0.75, py: 0.25, borderRadius: 0.5, bgcolor: "action.selected", fontSize: 12, fontFamily: "monospace" }}>
                          {r.model}
                        </Box>
                      </TableCell>
                      <TableCell sx={{ px: 2.5, py: 1.75, color: "text.secondary", fontSize: 14 }}>{r.poolName}</TableCell>
                      <TableCell sx={{ px: 2.5, py: 1.75 }}>
                        <Box
                          component="span"
                          sx={(theme) => ({
                            display: "inline-flex",
                            alignItems: "center",
                            gap: 0.75,
                            px: 1,
                            py: 0.25,
                            borderRadius: 999,
                            fontSize: 12,
                            fontWeight: 500,
                            bgcolor: alpha(theme.palette.error.main, 0.1),
                            color: "error.main",
                          })}
                        >
                          <Box component="span" sx={{ width: 6, height: 6, borderRadius: "50%", bgcolor: "error.main" }} />
                          {r.reason}
                        </Box>
                      </TableCell>
                      <TableCell sx={{ px: 2.5, py: 1.75, color: "text.secondary", fontSize: 14 }}>
                        {new Date(r.until).toLocaleTimeString()}
                      </TableCell>
                      <TableCell sx={{ px: 2.5, py: 1.75, fontSize: 14 }} align="right">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={async () => {
                            await fetch(`/api/proxy-pools/${r.poolId}/fitness/clear`, {
                              method: "POST",
                              headers: { "Content-Type": "application/json" },
                              body: JSON.stringify({ scope: r.scope }),
                            });
                            fetchAll();
                          }}
                        >
                          Clear
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))
                ) : (
                  <TableRow>
                    <TableCell colSpan={6} sx={{ px: 2.5, py: 8, textAlign: "center", color: "text.secondary", borderBottom: 0 }}>
                      <Stack sx={{ alignItems: "center", justifyContent: "center" }} spacing={1}>
                        <MsIcon sx={{ fontSize: 32, color: "text.secondary", opacity: 0.5 }}>
                          verified_user
                        </MsIcon>
                        <Typography sx={{ fontWeight: 500, fontSize: 14, color: "text.primary" }}>No active blocks</Typography>
                        <Typography sx={{ fontSize: 12, color: "text.secondary" }}>
                          All proxy pools are healthy and fit for routing.
                        </Typography>
                      </Stack>
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
            </TableContainer>
          </Card>
        </>
      )}
      <ConfirmModal
        isOpen={confirm}
        onClose={() => setConfirm(false)}
        onConfirm={clearAll}
        title="Clear proxy fitness"
        message="Clear active proxy fitness blocks?"
        confirmText="Clear All"
        cancelText="Cancel"
        variant="danger"
      />
    </Box>
  );
}

export { recordsOf };
