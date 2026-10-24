"use client";

import Link from "next/link";

export default function Footer() {
  return (
    <footer className="border-t border-[#3a2f27] bg-[#0f0d1a] pt-16 pb-8 px-6">
      <div className="max-w-7xl mx-auto">
        <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-5 gap-8 mb-16">
          {/* Brand */}
          <div className="col-span-2 lg:col-span-2">
            <div className="flex items-center gap-3 mb-6">
              <img src="/vansgo-logo.webp" alt="" width="40" height="40" className="size-10 rounded-xl object-contain" />
              <h3 className="text-white text-lg font-bold">VansGo</h3>
            </div>
            <p className="text-gray-500 text-sm max-w-xs mb-6">
              The unified endpoint for AI generation. Connect, route, and manage your AI providers with ease.
            </p>
            <div className="flex gap-4">
              <a className="text-gray-400 hover:text-white transition-colors" href="https://github.com/Vanszs/VansRouter" target="_blank" rel="noopener noreferrer">
                <span className="material-symbols-outlined">code</span>
              </a>
            </div>
          </div>
          
          {/* Product */}
          <div className="flex flex-col gap-4">
            <h4 className="font-bold text-white">Product</h4>
            <a className="text-gray-400 hover:text-[#007d9c] text-sm transition-colors" href="#features">Features</a>
            <Link className="text-gray-400 hover:text-[#007d9c] text-sm transition-colors" href="/dashboard">Dashboard</Link>
            <a className="text-gray-400 hover:text-[#007d9c] text-sm transition-colors" href="https://github.com/Vanszs/VansRouter" target="_blank" rel="noopener noreferrer">Changelog</a>
          </div>
          
          {/* Resources */}
          <div className="flex flex-col gap-4">
            <h4 className="font-bold text-white">Resources</h4>
            <a className="text-gray-400 hover:text-[#007d9c] text-sm transition-colors" href="https://github.com/Vanszs/VansRouter#readme" target="_blank" rel="noopener noreferrer">Documentation</a>
            <a className="text-gray-400 hover:text-[#007d9c] text-sm transition-colors" href="https://github.com/Vanszs/VansRouter" target="_blank" rel="noopener noreferrer">GitHub</a>
            <a className="text-gray-400 hover:text-[#007d9c] text-sm transition-colors" href="https://www.npmjs.com/package/vansrouter" target="_blank" rel="noopener noreferrer">NPM</a>
          </div>
          
          {/* Legal */}
          <div className="flex flex-col gap-4">
            <h4 className="font-bold text-white">Legal</h4>
            <a className="text-gray-400 hover:text-[#007d9c] text-sm transition-colors" href="https://github.com/Vanszs/VansRouter/blob/main/LICENSE" target="_blank" rel="noopener noreferrer">MIT License</a>
          </div>
        </div>
        
        {/* Bottom */}
        <div className="border-t border-[#3a2f27] pt-8 flex flex-col md:flex-row justify-between items-center gap-4">
          <p className="text-gray-600 text-sm">© 2025 VansGo. All rights reserved.</p>
          <div className="flex gap-6">
            <a className="text-gray-600 hover:text-white text-sm transition-colors" href="https://github.com/Vanszs/VansRouter" target="_blank" rel="noopener noreferrer">GitHub</a>
            <a className="text-gray-600 hover:text-white text-sm transition-colors" href="https://www.npmjs.com/package/vansrouter" target="_blank" rel="noopener noreferrer">NPM</a>
          </div>
        </div>
      </div>
    </footer>
  );
}

