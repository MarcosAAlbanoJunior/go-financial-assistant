import { Navigate, Route, Routes } from 'react-router'
import { RequireAuth } from './components/RequireAuth'
import Login from './pages/Login'
import Accounts from './pages/Accounts'
import Budget from './pages/Budget'
import Compare from './pages/Compare'
import Investments from './pages/Investments'
import Overview from './pages/Overview'
import Projection from './pages/Projection'
import Spending from './pages/Spending'
import Transactions from './pages/Transactions'

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route element={<RequireAuth />}>
        <Route path="/" element={<Overview />} />
        <Route path="/investimentos" element={<Investments />} />
        <Route path="/comparacoes" element={<Compare />} />
        <Route path="/transacoes" element={<Transactions />} />
        <Route path="/contas" element={<Accounts />} />
        <Route path="/projecao" element={<Projection />} />
        <Route path="/orcamento" element={<Budget />} />
        <Route path="/gastos" element={<Spending />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
