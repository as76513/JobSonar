import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/jobs": "http://127.0.0.1:8080",
      "/applications": "http://127.0.0.1:8080",
      "/profile": "http://127.0.0.1:8080",
      "/companies": "http://127.0.0.1:8080",
    },
  },
});
