import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const backendURL = process.env.ANTIGRAVITY_BACKEND_URL || "http://127.0.0.1:8888";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes("node_modules/@monaco-editor") || id.includes("node_modules/monaco-editor")) {
            return "monaco";
          }
          if (id.includes("node_modules/xterm") || id.includes("node_modules/xterm-addon-fit")) {
            return "terminal";
          }
          if (id.includes("node_modules/react") || id.includes("node_modules/react-dom")) {
            return "react-vendor";
          }
          return undefined;
        },
      },
    },
  },
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
