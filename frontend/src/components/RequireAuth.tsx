import { Navigate, Outlet } from 'react-router'
import { useMe } from '../api/client'
import { Layout } from './Layout'

export function RequireAuth() {
  const me = useMe()
  if (me.isPending) return <p className="state">Carregando…</p>
  if (me.isError) return <Navigate to="/login" replace />
  return (
    <Layout>
      <Outlet />
    </Layout>
  )
}
