import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig({
  root: "./frontend",
  publicDir: "public",
  base: "./",
  build: {
    outDir: "../dist/",
    emptyOutDir: true,
  },
  server: {
    port: 7071,
    proxy: {
      "/ws": { target: "ws://localhost:7071", ws: true },
    },
  },plugins: [vue()],
})
