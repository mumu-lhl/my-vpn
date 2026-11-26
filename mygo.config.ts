export default {
  name: "Looma",
  identifier: "io.github.mumulhl.looma",
  version: "0.1.0",
  main: "./cmd/app",
  devUrl: "http://localhost:5173",
  devCommand: "bun --cwd frontend run dev",
  buildCommand: "bun --cwd frontend run build",
  frontendDist: "frontend/dist",
  bindings: "frontend/src/lib/mygo.ts",
  out: "build",
  android: {
    keystore: process.env.ANDROID_KEYSTORE_PATH,
    keyAlias: "looma",
    permissions: [
      "INTERNET",
      "ACCESS_NETWORK_STATE",
      "FOREGROUND_SERVICE",
      "FOREGROUND_SERVICE_SPECIAL_USE",
      "POST_NOTIFICATIONS",
    ],
    splash: "#0f172a",
  },
};
