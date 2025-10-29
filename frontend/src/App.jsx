import { Routes, Route, Navigate } from 'react-router-dom'
import Chatbot from './components/Chatbot'
import AdminLogin from './components/AdminLogin'
import AdminDashboard from './components/AdminDashboard'
import ThemeToggle from './components/ThemeToggle'

function App() {
  return (
    <div className="min-h-screen bg-gray-50 text-gray-900 dark:bg-neutral-900 dark:text-neutral-100">
      <Routes>
        <Route path="/" element={<Chatbot />} />
        <Route path="/admin/login" element={<AdminLogin />} />
        <Route path="/admin/dashboard" element={<RequireAuth><AdminDashboard /></RequireAuth>} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
      <ThemeToggle />
    </div>
  )
}

function RequireAuth({ children }) {
  const token = localStorage.getItem('token')
  if (!token) return <Navigate to="/admin/login" replace />
  return children
}

export default App
