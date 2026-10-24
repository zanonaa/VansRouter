import { useEffect, useState } from "react";

// The running backend is authoritative (including VANSROUTER_VERSION).
// package.json describes the release, not the deployed CI build.
export async function fetchAppVersion() {
  const response = await fetch("/api/version", { cache: "no-store" });
  if (!response.ok) throw new Error("Unable to read running version");
  const data = await response.json();
  if (typeof data.currentVersion !== "string" || !data.currentVersion.trim()) {
    throw new Error("Missing running version");
  }
  return { ...data, currentVersion: data.currentVersion.trim() };
}

export function useAppVersion() {
  const [info, setInfo] = useState(null);
  useEffect(() => {
    let active = true;
    fetchAppVersion()
      .then(data => { if (active) setInfo(data); })
      .catch(() => {});
    return () => { active = false; };
  }, []);
  return info;
}
