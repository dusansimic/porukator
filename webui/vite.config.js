import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";
// Each Porukator service path is proxied to the Go server so requests stay
// same-origin in dev. The server mounts services at /porukator.v1.<Service>/.
const proxyTarget = "http://localhost:8080";
export default defineConfig({
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
