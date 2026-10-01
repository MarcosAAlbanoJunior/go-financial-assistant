import { useQueryClient } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { NavLink, useNavigate } from 'react-router'
import { logout } from '../api/client'
import { useTheme } from '../lib/theme'

export function Layout({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const { theme, toggle } = useTheme()

  async function handleLogout() {
    // Se o logout falhar, o cookie expira sozinho; mesmo assim saímos da tela.
    await logout().catch(() => undefined)
    queryClient.clear()
    navigate('/login', { replace: true })
  }

  return (
    <>
      <header className="topbar">
        <div className="container">
          <span className="brand">FinAssist</span>
          <nav className="nav" aria-label="Principal">
            <NavLink to="/" end>
              Visão geral
            </NavLink>
            <NavLink to="/gastos">Gastos</NavLink>
            <NavLink to="/comparacoes">Comparações</NavLink>
            <NavLink to="/transacoes">Transações</NavLink>
            <NavLink to="/contas">Contas</NavLink>
          </nav>
          <div className="topbar-actions">
            <button type="button" className="btn" onClick={toggle}>
              {theme === 'dark' ? 'Tema claro' : 'Tema escuro'}
            </button>
            <button type="button" className="btn" onClick={handleLogout}>
              Sair
            </button>
          </div>
        </div>
      </header>
      <main className="container">{children}</main>
    </>
  )
}
