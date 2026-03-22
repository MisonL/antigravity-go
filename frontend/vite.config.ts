import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const backendURL = process.env.ANTIGRAVITY_BACKEND_URL || "http://127.0.0.1:8888";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      "/api": {
        target: backendURL,
        changeOrigin: true,
      },
      "/ws": {
        target: backendURL,
        ws: true,
        changeOrigin: true,
      },
    },
  },
});
