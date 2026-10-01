import { Navigate, Route, Routes } from 'react-router'
import { RequireAuth } from './components/RequireAuth'
import Login from './pages/Login'
import Accounts from './pages/Accounts'
import Compare from './pages/Compare'
import Overview from './pages/Overview'
import Spending from './pages/Spending'
import Transactions from './pages/Transactions'

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route element={<RequireAuth />}>
        <Route path="/" element={<Overview />} />
        <Route path="/comparacoes" element={<Compare />} />
        <Route path="/transacoes" element={<Transactions />} />
        <Route path="/contas" element={<Accounts />} />
        <Route path="/gastos" element={<Spending />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
