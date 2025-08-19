import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom'
import Dashboard from '@/pages/Dashboard'
import Models from '@/pages/Models'
import Users from '@/pages/Users'
import Teams from '@/pages/Teams'
import Keys from '@/pages/Keys'
import Settings from '@/pages/Settings'
import Chat from '@/pages/Chat'
import Login from '@/pages/Login'
import Callback from '@/pages/Callback'
import SilentRenew from '@/pages/SilentRenew'
import Unauthorized from '@/pages/Unauthorized'
import Layout from '@/components/Layout'
import { Toaster } from '@/components/ui/toaster'
import { OIDCAuthProvider } from '@/contexts/OIDCAuthContext'
import { PermissionProvider } from '@/contexts/PermissionContext'
import { ProtectedRoute } from '@/components/ProtectedRoute'

function App() {
  // Since the UI is only served when database is configured,
  // we don't need to check database status here
  return (
    <Router basename="/ui">
      <OIDCAuthProvider>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/callback" element={<Callback />} />
          <Route path="/silent-renew" element={<SilentRenew />} />
          <Route path="/unauthorized" element={<Unauthorized />} />
          <Route path="/*" element={
            <ProtectedRoute>
              <PermissionProvider>
                <Layout>
                  <Routes>
                    <Route path="/" element={<Navigate to="/dashboard" replace />} />
                    <Route path="/dashboard" element={<Dashboard />} />
                    <Route path="/chat" element={<Chat />} />
                    <Route path="/models" element={<Models />} />
                    <Route path="/users" element={<Users />} />
                    <Route path="/teams" element={<Teams />} />
                    <Route path="/keys" element={<Keys />} />
                    <Route path="/settings" element={<Settings />} />
                  </Routes>
                </Layout>
              </PermissionProvider>
            </ProtectedRoute>
          } />
        </Routes>
        <Toaster />
      </OIDCAuthProvider>
    </Router>
  )
}

export default App