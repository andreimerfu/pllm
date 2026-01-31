import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom'
import Dashboard from '@/pages/Dashboard'
import Models from '@/pages/Models'
import ModelDetail from '@/pages/ModelDetail'
import AddModel from '@/pages/AddModel'
import EditModel from '@/pages/EditModel'
import Users from '@/pages/Users'
import Teams from '@/pages/Teams'
import Keys from '@/pages/Keys'
import Budget from '@/pages/Budget'
import Settings from '@/pages/Settings'
import AuditLogs from '@/pages/AuditLogs'
import Guardrails from '@/pages/Guardrails'
import GuardrailConfig from '@/pages/GuardrailConfig'
import GuardrailMarketplace from '@/pages/GuardrailMarketplace'
import Chat from '@/pages/Chat'
import Login from '@/pages/Login'
import Callback from '@/pages/Callback'
import SilentRenew from '@/pages/SilentRenew'
import Unauthorized from '@/pages/Unauthorized'
import Layout from '@/components/Layout'
import { Toaster } from '@/components/ui/toaster'
import { OIDCAuthProvider } from '@/contexts/OIDCAuthContext'
import { PermissionProvider } from '@/contexts/PermissionContext'
import { ConfigProvider } from '@/contexts/ConfigContext'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import { ThemeProvider } from '@/contexts/ThemeContext'

function App() {
  // Since the UI is only served when database is configured,
  // we don't need to check database status here
  return (
    <Router basename="/ui">
      <ThemeProvider>
        <OIDCAuthProvider>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/callback" element={<Callback />} />
          <Route path="/silent-renew" element={<SilentRenew />} />
          <Route path="/unauthorized" element={<Unauthorized />} />
          <Route path="/*" element={
            <ProtectedRoute>
              <ConfigProvider>
                <PermissionProvider>
                  <Layout>
                    <Routes>
                      <Route path="/" element={<Navigate to="/dashboard" replace />} />
                      <Route path="/dashboard" element={<Dashboard />} />
                      <Route path="/chat" element={<Chat />} />
                      <Route path="/models" element={<Models />} />
                      <Route path="/models/new" element={<AddModel />} />
                      <Route path="/models/edit/:modelId" element={<EditModel />} />
                      <Route path="/models/:modelId" element={<ModelDetail />} />
                      <Route path="/users" element={<Users />} />
                      <Route path="/teams" element={<Teams />} />
                      <Route path="/keys" element={<Keys />} />
                      <Route path="/budget" element={<Budget />} />
                      <Route path="/audit-logs" element={<AuditLogs />} />
                      <Route path="/guardrails" element={<Guardrails />} />
                      <Route path="/guardrails/marketplace" element={<GuardrailMarketplace />} />
                      <Route path="/guardrails/config/new" element={<GuardrailConfig />} />
                      <Route path="/guardrails/config/:id" element={<GuardrailConfig />} />
                      <Route path="/settings" element={<Settings />} />
                    </Routes>
                  </Layout>
                </PermissionProvider>
              </ConfigProvider>
            </ProtectedRoute>
          } />
        </Routes>
        <Toaster />
        </OIDCAuthProvider>
      </ThemeProvider>
    </Router>
  )
}

export default App