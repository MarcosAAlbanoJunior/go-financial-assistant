import { Navigate, Route, Routes } from 'react-router'
import { RequireAuth } from './components/RequireAuth'
import Login from './pages/Login'
import Budget from './pages/Budget'
import Coach from './pages/Coach'
import Categorize from './pages/Categorize'
import Compare from './pages/Compare'
import Goals from './pages/Goals'
import Investments from './pages/Investments'
import Overview from './pages/Overview'
import Panel from './pages/Panel'
import Projection from './pages/Projection'
import Review from './pages/Review'
import Spending from './pages/Spending'
import Transactions from './pages/Transactions'

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
        <Route path="/classificar" element={<Categorize />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
