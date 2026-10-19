"use client";

import Link from "next/link";
import { Button } from "@/shared/components";

export default function GetStarted() {
  return (
    <section className="px-6 py-24 bg-[#0f0d1a] text-white">
      <div className="max-w-4xl mx-auto flex flex-col items-center gap-6 text-center">
        <h2 className="text-3xl md:text-4xl font-bold">Get Started</h2>
        <p className="max-w-2xl text-gray-400">
          Run the web gateway with Docker, open the dashboard, and configure providers and API keys.
        </p>
        <pre className="w-full max-w-2xl overflow-x-auto rounded-xl border border-[#3a2f27] bg-[#1e1e1e] p-5 text-left text-sm text-gray-200">
{`docker run -d \\
  --name vansrouter \\
  -p 20128:20128 \\
  -v vansrouter-data:/app/data \\
  -e INITIAL_PASSWORD='replace-with-a-strong-password' \\
  ghcr.io/zanonaa/vansrouter:1.2.0`}
        </pre>
        <div className="flex flex-col sm:flex-row items-center gap-4">
          <Button component={Link} href="/masuk">Open Dashboard</Button>
          <span className="text-sm text-gray-400">Gateway endpoint: http://localhost:20128/v1</span>
        </div>
      </div>
    </section>
  );
}
