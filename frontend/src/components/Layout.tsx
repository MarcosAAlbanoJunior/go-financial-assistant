import { useQueryClient } from '@tanstack/react-query'
import {
  ArrowLeftRight,
  Calculator,
  ChartPie,
  GitCompareArrows,
  Landmark,
  LayoutDashboard,
  LogOut,
  Menu,
  Moon,
  ScanSearch,
  Scale,
  Sun,
  Target,
  TrendingUp,
  X,
  type LucideIcon,
} from 'lucide-react'
import { useEffect, useState, type ReactNode } from 'react'
import { NavLink, useNavigate } from 'react-router'
import { logout } from '../api/client'
import { useTheme } from '../lib/theme'

interface Item {
  to: string
  label: string
  Icon: LucideIcon
}

// O menu é agrupado por assunto; a ordem dentro de cada grupo vai do mais usado ao menos usado.
const SECTIONS: { title?: string; items: Item[] }[] = [
  { items: [{ to: '/', label: 'Visão geral', Icon: LayoutDashboard }] },
  {
    title: 'Dia a dia',
    items: [
      { to: '/transacoes', label: 'Transações', Icon: ArrowLeftRight },
      { to: '/gastos', label: 'Gastos', Icon: ChartPie },
      { to: '/orcamento', label: 'Orçamento', Icon: Scale },
      { to: '/contas', label: 'Contas e cartões', Icon: Landmark },
    ],
  },
  {
    title: 'Planejamento',
    items: [
      { to: '/revisao', label: 'Revisão', Icon: ScanSearch },
      { to: '/metas', label: 'Metas', Icon: Target },
      { to: '/projecao', label: 'Projeção', Icon: Calculator },
      { to: '/comparacoes', label: 'Comparações', Icon: GitCompareArrows },
    ],
  },
  { title: 'Patrimônio', items: [{ to: '/investimentos', label: 'Investimentos', Icon: TrendingUp }] },
]

export function Layout({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const { theme, toggle } = useTheme()
  const [open, setOpen] = useState(false)

  // Em telas estreitas o menu é uma gaveta: Esc fecha.
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false)
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open])

  async function handleLogout() {
    // Se o logout falhar, o cookie expira sozinho; mesmo assim saímos da tela.
    await logout().catch(() => undefined)
    queryClient.clear()
    navigate('/login', { replace: true })
  }

  return (
    <div className="app">
      <header className="mobilebar">
        <button type="button" className="icon-btn" aria-label="Abrir menu" aria-expanded={open} aria-controls="sidebar" onClick={() => setOpen(true)}>
          <Menu size={22} aria-hidden="true" />
        </button>
        <span className="brand">FinAssist</span>
      </header>

      {open && <div className="scrim" onClick={() => setOpen(false)} aria-hidden="true" />}

      <aside id="sidebar" className={`sidebar ${open ? 'is-open' : ''}`}>
        <div className="side-brand">
          <span className="brand">FinAssist</span>
          <button type="button" className="icon-btn side-close" aria-label="Fechar menu" onClick={() => setOpen(false)}>
            <X size={20} aria-hidden="true" />
          </button>
        </div>

        <nav aria-label="Principal" className="side-nav">
          {SECTIONS.map((section, i) => (
            <div className="side-section" key={section.title ?? i}>
              {section.title && <p className="side-title">{section.title}</p>}
              {section.items.map(({ to, label, Icon }) => (
                <NavLink key={to} to={to} end={to === '/'} className="side-link" onClick={() => setOpen(false)}>
                  <Icon size={18} aria-hidden="true" />
                  {label}
                </NavLink>
              ))}
            </div>
          ))}
        </nav>

        <div className="side-foot">
          <button type="button" className="side-link" onClick={toggle}>
            {theme === 'dark' ? <Sun size={18} aria-hidden="true" /> : <Moon size={18} aria-hidden="true" />}
            {theme === 'dark' ? 'Tema claro' : 'Tema escuro'}
          </button>
          <button type="button" className="side-link" onClick={handleLogout}>
            <LogOut size={18} aria-hidden="true" />
            Sair
          </button>
        </div>
      </aside>

      <main className="content">
        <div className="container">{children}</div>
      </main>
    </div>
  )
}
