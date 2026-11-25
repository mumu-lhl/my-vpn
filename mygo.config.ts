import { defineConfig } from "mygo-cli";

export default defineConfig({
  name: "MyVPN",
  identifier: "io.github.mumulhl.myvpn",
  version: "0.1.0",
  devUrl: "http://localhost:5173",
  devCommand: "bun --cwd frontend run dev",
  buildCommand: "bun --cwd frontend run build",
  frontendDist: "frontend/dist",
  bindings: "frontend/src/lib/mygo.ts",
  out: "build",
  android: {
    permissions: [
      "INTERNET",
      "ACCESS_NETWORK_STATE",
      "FOREGROUND_SERVICE",
      "FOREGROUND_SERVICE_SPECIAL_USE",
      "POST_NOTIFICATIONS",
    ],
    splash: "#0f172a",
  },
});
