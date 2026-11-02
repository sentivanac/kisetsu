import {defineConfig, type Plugin} from 'vite'
import react from '@vitejs/plugin-react'

// Content-Security-Policy of the built app (see docs/adr/0034). Scripts
// and styles only come from the app itself; images may also come from
// image providers over HTTPS. Not applied under `wails dev`, where Vite
// needs inline scripts and a websocket for hot reload.
export const contentSecurityPolicy = [
  "default-src 'self'",
  "script-src 'self'",
  "style-src 'self'",
  "img-src 'self' https: data: blob:",
  "connect-src 'self'",
  "object-src 'none'",
  "base-uri 'none'",
  "form-action 'none'",
].join('; ')

function csp(): Plugin {
  return {
    name: 'kisetsu-csp',
    apply: 'build',
    transformIndexHtml: () => [
      {
        tag: 'meta',
        attrs: {'http-equiv': 'Content-Security-Policy', content: contentSecurityPolicy},
        injectTo: 'head-prepend',
      },
    ],
  }
}

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), csp()]
})
