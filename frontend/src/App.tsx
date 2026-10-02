import { Navigate, Route, Routes } from 'react-router'
import { RequireAuth } from './features/auth/RequireAuth'
import Login from './features/auth/Login'
import Budget from './features/budget/Budget'
import Coach from './features/coach/Coach'
import Categorize from './features/categorize/Categorize'
import Compare from './features/compare/Compare'
import Goals from './features/goals/Goals'
import Investments from './features/investments/Investments'
import Overview from './features/overview/Overview'
import Panel from './features/balances/Panel'
import Projection from './features/projection/Projection'
import Review from './features/review/Review'
import Settings from './features/settings/Settings'
import Spending from './features/spending/Spending'
import Transactions from './features/transactions/Transactions'

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route element={<RequireAuth />}>
        <Route path="/" element={<Panel />} />
        <Route path="/visao-geral" element={<Overview />} />
        <Route path="/investimentos" element={<Investments />} />
        <Route path="/comparacoes" element={<Compare />} />
        <Route path="/transacoes" element={<Transactions />} />
        <Route path="/contas" element={<Navigate to="/" replace />} />
        <Route path="/projecao" element={<Projection />} />
        <Route path="/orcamento" element={<Budget />} />
        <Route path="/revisao" element={<Review />} />
        <Route path="/metas" element={<Goals />} />
        <Route path="/coach" element={<Coach />} />
        <Route path="/gastos" element={<Spending />} />
        <Route path="/configuracoes" element={<Settings />} />
        <Route path="/classificar" element={<Categorize />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
