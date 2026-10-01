import { Navigate, Route, Routes } from 'react-router'
import { RequireAuth } from './components/RequireAuth'
import Login from './pages/Login'
import Overview from './pages/Overview'

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route element={<RequireAuth />}>
        <Route path="/" element={<Overview />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
