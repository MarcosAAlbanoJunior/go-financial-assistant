import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // Em desenvolvimento, /api vai para o app Go (publicado em 127.0.0.1:3000 pelo compose).
    // Sem changeOrigin de propósito: o Host precisa continuar igual ao Origin do navegador,
    // senão a checagem de mesma origem do login (CSRF) recusa a requisição.
    proxy: { '/api': 'http://127.0.0.1:3000' },
  },
  test: { include: ['src/**/*.test.ts'] },
})
