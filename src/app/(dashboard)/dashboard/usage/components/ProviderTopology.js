"use client";

import { useMemo, useState, useEffect, useCallback, useRef } from "react";
import Image from "next/image";
import Box from "@mui/material/Box";

const EMPTY_PROVIDERS = [];
const EMPTY_REQUESTS = [];
import {
  ReactFlow,
  Handle,
  Position,
  Controls,
  BaseEdge,
  getBezierPath,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { AI_PROVIDERS } from "@/shared/constants/providers";
import { getProviderIconSrc, markProviderIconMissing } from "@/shared/utils/providerIcon";

// Force-stop FE animation if a provider stays active longer than this
const FE_ACTIVE_TIMEOUT_MS = 60000;
const FE_ACTIVE_TICK_MS = 1000;
const FIT_VIEW_OPTS = { padding: 0.2, duration: 200 };

// Kame + electric particles along active edges
const KAME_PARTICLE_COUNT = 6;
const SPARK_COUNT = 5;

const handleStyle = { background: "transparent", border: 0, width: 0, height: 0 };

const subtleBg = (t) => (t.palette.mode === "dark" ? "rgba(42,42,42,0.3)" : "rgba(244,244,245,0.3)");

function getProviderConfig(providerId) {
  return AI_PROVIDERS[providerId] || { color: "#6b7280", name: providerId };
}

// Custom provider node - rectangle with image + name
function ProviderNode({ data }) {
  const { providerId, label, color, imageUrl, textIcon, active } = data;
  const [imgError, setImgError] = useState(false);
  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        gap: 1.25,
        px: 2,
        py: 1.25,
        borderRadius: "8px",
        border: "2px solid",
        transition: "all 300ms ease",
        bgcolor: "background.default",
        minWidth: 150,
        borderColor: active ? color : "var(--color-border)",
        boxShadow: active ? `0 0 16px ${color}40` : "none",
      }}
    >
      <Handle type="target" position={Position.Top} id="top" style={handleStyle} />
      <Handle type="target" position={Position.Bottom} id="bottom" style={handleStyle} />
      <Handle type="target" position={Position.Left} id="left" style={handleStyle} />
      <Handle type="target" position={Position.Right} id="right" style={handleStyle} />

      {/* Provider icon */}
      <Box
        sx={{
          width: 32,
          height: 32,
          borderRadius: "6px",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          flexShrink: 0,
          bgcolor: `${color}15`,
        }}
      >
        {providerId === "a6api" || providerId === "a6api-cli" ? (
          <span
            className="a6api-custom-logo"
            style={{
              width: "24px",
              height: "24px",
              borderRadius: "50%",
              display: "grid",
              placeItems: "center",
              position: "relative",
              overflow: "hidden",
              color: "var(--navy, #1F2937)",
              fontSize: "9px",
              fontWeight: "bold",
              letterSpacing: 0,
              background: "radial-gradient(circle at 34% 28%, rgba(255, 255, 255, .38), transparent 22%), conic-gradient(from 210deg, #3157ff, #16b8a6, #74c86a, #3157ff)",
              boxShadow: "0 4px 8px #3157ff38",
              isolation: "isolate",
            }}
          >
            <span>A6</span>
          </span>
        ) : imageUrl && !imgError ? (
          <Image
            src={imageUrl}
            alt={label}
            style={{ width: 24, height: 24, borderRadius: 4, objectFit: "contain" }}
            width={24}
            height={24}
            unoptimized
            onError={() => {
              const m = imageUrl?.match(/^\/providers\/([^/]+)\.(png|webp)$/i);
              if (m) markProviderIconMissing(m[1]);
              setImgError(true);
            }}
          />
        ) : (
          <Box component="span" sx={{ fontSize: 14, fontWeight: 700, color }}>{textIcon}</Box>
        )}
      </Box>

      {/* Provider name */}
      <Box
        component="span"
        sx={{
          fontSize: 16,
          fontWeight: 500,
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
          color: active ? color : "var(--color-text)",
        }}
      >
        {label}
      </Box>

      {/* Active indicator */}
      {active && (
        <Box component="span" sx={{ position: "relative", display: "flex", height: 8, width: 8, flexShrink: 0 }}>
          <Box component="span" sx={{ animation: "pulse 1.5s cubic-bezier(0,0,0.2,1) infinite", position: "absolute", display: "inline-flex", height: "100%", width: "100%", borderRadius: "9999px", opacity: 0.75, bgcolor: color }} />
          <Box component="span" sx={{ position: "relative", display: "inline-flex", borderRadius: "9999px", height: 8, width: 8, bgcolor: color }} />
        </Box>
      )}
    </Box>
  );
}


// Center 9Router node — pulse/glow on card only (no expanding rings)
function RouterNode({ data }) {
  const powering = (data.activeCount || 0) > 0;
  return (
    <Box
      className={powering ? "topology-router-core" : undefined}
      sx={{
        position: "relative",
        zIndex: 1,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        px: 2.5,
        py: 1.5,
        borderRadius: "12px",
        border: "2px solid",
        minWidth: 130,
        ...(powering
          ? {
              background: "linear-gradient(to bottom right, rgba(229,106,74,0.3), rgba(250,204,21,0.2), rgba(34,211,238,0.25))",
            }
          : { borderColor: "primary.main", bgcolor: "rgba(229,106,74,0.05)", boxShadow: "0 4px 6px -1px rgba(0,0,0,0.1)" }),
      }}
    >
      <Handle type="source" position={Position.Top} id="top" style={handleStyle} />
      <Handle type="source" position={Position.Bottom} id="bottom" style={handleStyle} />
      <Handle type="source" position={Position.Left} id="left" style={handleStyle} />
      <Handle type="source" position={Position.Right} id="right" style={handleStyle} />

      <img
        src="/favicon.svg"
        alt="VansAI"
        className={powering ? "topology-router-icon" : undefined}
        style={{ width: 24, height: 24, marginRight: 8 }}
        loading="lazy"
        decoding="async"
        width={24}
        height={24}
      />
      <Box
        component="span"
        className={powering ? "topology-router-label" : undefined}
        sx={{ fontSize: 14, fontWeight: 700, color: powering ? "#fde047" : "primary.main" }}
      >
        VansAI
      </Box>
      {data.activeCount > 0 && (
        <Box component="span" className="topology-router-badge" sx={{ ml: 1, px: 0.75, py: 0.25, borderRadius: "9999px", bgcolor: "#facc15", color: "#000", fontSize: 12, fontWeight: 700 }}>
          {data.activeCount}
        </Box>
      )}
    </Box>
  );
}


// Active: electric kame beam (multi-layer stroke + sparks). Idle/last/error: solid BaseEdge.
function TopologyEdge({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  style = {},
  data,
}) {
  const [edgePath] = getBezierPath({
    sourceX,
    sourceY,
    sourcePosition,
    targetX,
    targetY,
    targetPosition,
  });
  const active = !!data?.active;
  const stroke = style.stroke || "var(--color-border)";
  const filterId = `topo-electric-${id}`;

  if (!active) {
    return <BaseEdge id={id} path={edgePath} style={{ ...style, stroke }} />;
  }

  return (
    <g className="topology-edge-electric">
      <defs>
        <filter id={filterId} x="-40%" y="-40%" width="180%" height="180%">
          <feTurbulence type="fractalNoise" baseFrequency="0.9" numOctaves="2" seed="2" result="noise">
            <animate attributeName="baseFrequency" values="0.8;1.4;0.8" dur="0.25s" repeatCount="indefinite" />
          </feTurbulence>
          <feDisplacementMap in="SourceGraphic" in2="noise" scale="3.5" xChannelSelector="R" yChannelSelector="G" />
        </filter>
      </defs>
      {/* Outer electric halo */}
      <path
        d={edgePath}
        fill="none"
        stroke="#22d3ee"
        strokeWidth={10}
        strokeOpacity={0.35}
        strokeLinecap="round"
        filter={`url(#${filterId})`}
        className="topology-edge-halo"
      />
      {/* Mid plasma */}
      <path
        d={edgePath}
        fill="none"
        stroke="#4ade80"
        strokeWidth={5}
        strokeOpacity={0.85}
        strokeLinecap="round"
        filter={`url(#${filterId})`}
        className="topology-edge-plasma"
      />
      {/* Hot white core */}
      <BaseEdge
        id={id}
        path={edgePath}
        style={{ stroke: "#f8fafc", strokeWidth: 2.2, opacity: 1 }}
        className="topology-edge-kame"
      />
      {/* Energy orbs */}
      {Array.from({ length: KAME_PARTICLE_COUNT }, (_, i) => (
        <circle
          key={`${id}-p-${i}`}
          r={i % 2 === 0 ? 4 : 2.5}
          fill={i % 3 === 0 ? "#fde047" : i % 3 === 1 ? "#67e8f9" : "#fff"}
          opacity={0.95}
          style={{ filter: "drop-shadow(0 0 4px #22d3ee)" }}
        >
          <animateMotion
            dur={`${0.4 + i * 0.08}s`}
            repeatCount="indefinite"
            path={edgePath}
            begin={`${i * 0.09}s`}
          />
        </circle>
      ))}
      {/* Electric sparks (short-lived blink along path) */}
      {Array.from({ length: SPARK_COUNT }, (_, i) => (
        <circle
          key={`${id}-s-${i}`}
          r={1.8}
          fill="#e0f2fe"
          opacity={0}
        >
          <animate
            attributeName="opacity"
            values="0;1;0;0;1;0"
            dur={`${0.35 + (i % 3) * 0.1}s`}
            begin={`${i * 0.07}s`}
            repeatCount="indefinite"
          />
          <animateMotion
            dur={`${0.28 + i * 0.05}s`}
            repeatCount="indefinite"
            path={edgePath}
            begin={`${i * 0.11}s`}
          />
        </circle>
      ))}
    </g>
  );
}

const nodeTypes = { provider: ProviderNode, router: RouterNode };
const edgeTypes = { topology: TopologyEdge };

// Place N nodes evenly along an ellipse around the router center.
function buildLayout(providers, activeSet, lastSet, errorSet) {
  const nodeW = 180;
  const nodeH = 30;
  const routerW = 120;
  const routerH = 44;
  const nodeGap = 24;

  const count = providers.length;

  // Compute rx so arc spacing between nodes >= nodeW + nodeGap
  const minRx = ((nodeW + nodeGap) * count) / (2 * Math.PI);
  const rx = Math.max(320, minRx);
  const ry = Math.max(200, rx * 0.55); // ellipse ratio ~0.55
  if (count === 0) {
    return {
      nodes: [{ id: "router", type: "router", position: { x: 0, y: 0 }, data: { activeCount: 0 }, draggable: false }],
      edges: [],
    };
  }

  const nodes = [];
  const edges = [];

  nodes.push({
    id: "router",
    type: "router",
    position: { x: -routerW / 2, y: -routerH / 2 },
    data: { activeCount: activeSet.size },
    draggable: false,
  });

  const edgeStyle = (active, last, error) => {
    if (error) return { stroke: "#ef4444", strokeWidth: 2.5, opacity: 0.9 };
    if (active) return { stroke: "#22d3ee", strokeWidth: 3.5, opacity: 1 };
    if (last) return { stroke: "#f59e0b", strokeWidth: 2, opacity: 0.7 };
    return { stroke: "var(--color-border)", strokeWidth: 1, opacity: 0.3 };
  };

  providers.forEach((p, i) => {
    const config = getProviderConfig(p.provider);
    const active = activeSet.has(p.provider?.toLowerCase());
    const last = !active && lastSet.has(p.provider?.toLowerCase());
    const error = !active && errorSet.has(p.provider?.toLowerCase());
    const nodeId = `provider-${p.provider}`;
    const data = {
      providerId: p.provider,
      label: (config.name !== p.provider ? config.name : null) || p.nodeName || p.name || p.provider,
      color: config.color || "#6b7280",
      imageUrl: getProviderIconSrc(p.provider),
      textIcon: config.textIcon || (p.provider || "?").slice(0, 2).toUpperCase(),
      active,
    };

    // Distribute evenly starting from top (−π/2), clockwise
    const angle = -Math.PI / 2 + (2 * Math.PI * i) / count;
    const cx = rx * Math.cos(angle);
    const cy = ry * Math.sin(angle);

    // Pick router handle closest to the node direction
    let sourceHandle, targetHandle;
    if (Math.abs(angle + Math.PI / 2) < Math.PI / 4 || Math.abs(angle - 3 * Math.PI / 2) < Math.PI / 4) {
      sourceHandle = "top"; targetHandle = "bottom";
    } else if (Math.abs(angle - Math.PI / 2) < Math.PI / 4) {
      sourceHandle = "bottom"; targetHandle = "top";
    } else if (cx > 0) {
      sourceHandle = "right"; targetHandle = "left";
    } else {
      sourceHandle = "left"; targetHandle = "right";
    }

    nodes.push({
      id: nodeId,
      type: "provider",
      position: { x: cx - nodeW / 2, y: cy - nodeH / 2 },
      data,
      draggable: false,
    });

    edges.push({
      id: `e-${nodeId}`,
      type: "topology",
      source: "router",
      sourceHandle,
      target: nodeId,
      targetHandle,
      // Built-in animated uses stroke-dasharray (CPU-heavy); use particle beam instead
      animated: false,
      data: { active },
      style: edgeStyle(active, last, error),
    });
  });

  return { nodes, edges };
}

export default function ProviderTopology({ providers = EMPTY_PROVIDERS, activeRequests = EMPTY_REQUESTS, lastProvider = "", errorProvider = "" }) {
  // Serialize to stable string keys so useMemo only re-runs when values actually change
  const activeKey = useMemo(
    () => activeRequests.flatMap((r) => { const p = r.provider?.toLowerCase(); return p ? [p] : []; }).sort().join(","),
    [activeRequests]
  );
  const lastKey = lastProvider?.toLowerCase() || "";
  const errorKey = errorProvider?.toLowerCase() || "";

  const rawActiveSet = useMemo(() => new Set(activeKey ? activeKey.split(",") : []), [activeKey]);
  const lastSet = useMemo(() => new Set(lastKey ? [lastKey] : []), [lastKey]);
  const errorSet = useMemo(() => new Set(errorKey ? [errorKey] : []), [errorKey]);

  // Track firstSeen per active provider; drop provider if running too long (BE stuck)
  const [firstSeen, setFirstSeen] = useState({});
  const [activeSet, setActiveSet] = useState(new Set());
  const [tick, setTick] = useState(0);

  // Tracks when each provider became active so we can drop providers that
  // are stuck (e.g. backend never marked them done). State (not ref) so
  // downstream useEffects can react to changes.
  useEffect(() => {
    const now = Date.now();
    // eslint-disable-next-line react-hooks/set-state-in-effect -- updater pattern avoids stale state; cascading renders are intentional here.
    setFirstSeen((prev) => {
      const next = { ...prev };
      for (const p of rawActiveSet) {
        if (!next[p]) next[p] = now;
      }
      for (const p of Object.keys(next)) {
        if (!rawActiveSet.has(p)) delete next[p];
      }
      return next;
    });
  }, [rawActiveSet]);

  useEffect(() => {
    if (rawActiveSet.size === 0) return;
    const id = setInterval(() => setTick((t) => t + 1), FE_ACTIVE_TICK_MS);
    return () => clearInterval(id);
  }, [rawActiveSet]);

  // Compute filtered activeSet in an effect so Date.now() is not called
  // during render. tick + firstSeen + rawActiveSet all trigger recompute.
  useEffect(() => {
    void tick; // re-run on tick
    const now = Date.now();
    const filtered = new Set();
    for (const p of rawActiveSet) {
      const ts = firstSeen[p];
      if (!ts || now - ts < FE_ACTIVE_TIMEOUT_MS) filtered.add(p);
    }
    // eslint-disable-next-line react-hooks/set-state-in-effect -- Date.now() is not allowed during render; computed in effect so render stays pure.
    setActiveSet(filtered);
  }, [rawActiveSet, tick, firstSeen]);

  const { nodes, edges } = useMemo(
    () => buildLayout(providers, activeSet, lastSet, errorSet),
    [providers, activeSet, lastSet, errorSet]
  );

  // Stable key — only remount when provider list changes
  const providersKey = useMemo(
    () => providers.map((p) => p.provider).sort().join(","),
    [providers]
  );

  const rfInstance = useRef(null);
  const containerRef = useRef(null);
  const onInit = useCallback((instance) => {
    rfInstance.current = instance;
    setTimeout(() => instance.fitView(FIT_VIEW_OPTS), 50);
  }, []);

  // Re-fit on container resize
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const ro = new ResizeObserver(() => {
      if (rfInstance.current) rfInstance.current.fitView(FIT_VIEW_OPTS);
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  // Re-fit when node count/layout changes
  useEffect(() => {
    if (rfInstance.current) {
      const id = setTimeout(() => rfInstance.current.fitView(FIT_VIEW_OPTS), 50);
      return () => clearTimeout(id);
    }
  }, [nodes.length]);

  return (
    <Box ref={containerRef} sx={{ height: { xs: 320, sm: 480 }, width: "100%", minWidth: 0, borderRadius: "8px", border: 1, borderColor: "divider", bgcolor: subtleBg }}>
      {providers.length === 0 ? (
        <Box sx={{ height: "100%", display: "flex", alignItems: "center", justifyContent: "center", color: "text.secondary", fontSize: 14 }}>
          No providers connected
        </Box>
      ) : (
        <ReactFlow
          key={providersKey}
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          edgeTypes={edgeTypes}
          fitView
          fitViewOptions={FIT_VIEW_OPTS}
          minZoom={0.1}
          maxZoom={2}
          onInit={onInit}
          proOptions={{ hideAttribution: true }}
          panOnDrag
          zoomOnScroll
          zoomOnPinch
          zoomOnDoubleClick
          preventScrolling={false}
          nodesDraggable={false}
          nodesConnectable={false}
          elementsSelectable={false}
        >
          <Controls showInteractive={false} className="react-flow-controls-custom" />
        </ReactFlow>
      )}
    </Box>
  );
}
