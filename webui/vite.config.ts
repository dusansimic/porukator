import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";
import { execSync } from "node:child_process";

// Each Porukator service path is proxied to the Go server so requests stay
// same-origin in dev. The server mounts services at /porukator.v1.<Service>/.
const proxyTarget = "http://localhost:8080";

// Git SHA the bundle is built from, shown in the UI. In Docker the .git dir is
// absent, so CI/the image build passes VITE_GIT_SHA; locally we read git.
function gitSha(): string {
  const fromEnv = process.env.VITE_GIT_SHA;
  if (fromEnv) return fromEnv.slice(0, 7);
  try {
    return execSync("git rev-parse --short HEAD").toString().trim();
  } catch {
    return "unknown";
  }
}

export default defineConfig({
  define: {
    __GIT_SHA__: JSON.stringify(gitSha()),
  },
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    proxy: {
      "/porukator.v1.AdminService": { target: proxyTarget, changeOrigin: true },
      "/porukator.v1.ProducerService": { target: proxyTarget, changeOrigin: true },
    },
  },
});
