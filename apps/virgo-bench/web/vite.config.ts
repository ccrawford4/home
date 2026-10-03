import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    // `npm run dev` proxies the API to a locally running Go server.
    proxy: { "/api": "http://localhost:8080" },
  },
  build: { outDir: "dist", emptyOutDir: true },
});
