import { QueryCache, QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router'
import App from './App'
import './index.css'
import { applyStoredTheme } from './shared/lib/theme'
import { ApiError } from './shared/api/request'

applyStoredTheme()

const queryClient: QueryClient = new QueryClient({
  // Sessão expirada no meio do uso: revalida /api/me, que leva ao login. O 401 do próprio
  // /api/me é o resultado esperado de "sem sessão" e não pode se reinvalidar (loop de requisições).
  queryCache: new QueryCache({
    onError: (error, query) => {
      // O mesmo vale para {"setup": "required"} (setup reaberto): /api/me leva à tela do setup.
      if (error instanceof ApiError && (error.status === 401 || error.setupRequired) && query.queryKey[0] !== 'me') {
        void queryClient.invalidateQueries({ queryKey: ['me'] })
      }
    },
  }),
  defaultOptions: { queries: { retry: 1, refetchOnWindowFocus: false } },
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>,
)
