import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    host: true,
    port: 5173,
    strictPort: true,
    // cpolar / ngrok / cloudflare 隧道会带陌生 Host，不放行会 403
    allowedHosts: true,
    proxy: {
      "/api": "http://127.0.0.1:8888",
      "/healthz": "http://127.0.0.1:8888",
    },
  },
});
