import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom'
import Dashboard from '@/pages/Dashboard'
import Models from '@/pages/Models'
import Users from '@/pages/Users'
import Settings from '@/pages/Settings'
import Layout from '@/components/Layout'
import { Toaster } from '@/components/ui/toaster'

function App() {
  // Since the UI is only served when database is configured,
  // we don't need to check database status here
  return (
    <Router basename="/ui">
      <Layout>
        <Routes>
          <Route path="/" element={<Navigate to="/dashboard" replace />} />
          <Route path="/dashboard" element={<Dashboard />} />
          <Route path="/models" element={<Models />} />
          <Route path="/users" element={<Users />} />
          <Route path="/settings" element={<Settings />} />
        </Routes>
      </Layout>
      <Toaster />
    </Router>
  )
}

export default App