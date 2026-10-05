import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // Em desenvolvimento, /api vai para o app Go (publicado em 127.0.0.1:3000 pelo compose).
    // changeOrigin: false explícito: o Host precisa continuar igual ao Origin do navegador, senão a checagem de mesma
    // origem (CSRF) recusa as escritas. A forma curta ('/api': 'http://...') liga changeOrigin no Vite.
    proxy: { '/api': { target: 'http://127.0.0.1:3000', changeOrigin: false } },
  },
  test: { include: ['src/**/*.test.ts'] },
})
