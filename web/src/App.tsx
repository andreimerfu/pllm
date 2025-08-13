import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom'
import Dashboard from '@/pages/Dashboard'
import Models from '@/pages/Models'
import Users from '@/pages/Users'
import Teams from '@/pages/Teams'
import Keys from '@/pages/Keys'
import Settings from '@/pages/Settings'
import Chat from '@/pages/Chat'
import Login from '@/pages/Login'
import Layout from '@/components/Layout'
import { Toaster } from '@/components/ui/toaster'
import { AuthProvider } from '@/contexts/AuthContext'
import { ProtectedRoute } from '@/components/ProtectedRoute'

function App() {
  // Since the UI is only served when database is configured,
  // we don't need to check database status here
  return (
    <Router basename="/ui">
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/*" element={
            <ProtectedRoute>
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
            </ProtectedRoute>
          } />
        </Routes>
        <Toaster />
      </AuthProvider>
    </Router>
  )
}

export default App