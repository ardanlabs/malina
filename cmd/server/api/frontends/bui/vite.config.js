import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  base: "/admin/",
  server: {
    proxy: {
      "/v1": "http://127.0.0.1:8080",
    },
  },
  build: {
    outDir: "../../services/malina/static/site",
    emptyOutDir: true,
  },
});
