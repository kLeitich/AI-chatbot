"use client"
import { createContext, useContext, useEffect, useState, useCallback } from 'react'
import api from '../lib/api'

const PermissionContext = createContext({
  user: null,
  role: '',
  permissions: [],
  loading: true,
  error: null,
  can: () => false,
  refresh: () => Promise.resolve(),
})

export function PermissionProvider({ tenant = 'default', children }) {
  const [user, setUser] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  const fetchProfile = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await api.get(`/t/${tenant}/me`)
      setUser(res.data)
    } catch (err) {
      setError(err)
      setUser(null)
    } finally {
      setLoading(false)
    }
  }, [tenant])

  useEffect(() => {
    fetchProfile()
  }, [fetchProfile])

  const can = useCallback(
    (permission) => {
      if (!user) return false
      if (user.role === 'tenant_admin') return true
      const userPerms = user.permissions || []
      return userPerms.includes(permission)
    },
    [user]
  )

  const value = {
    user,
    role: user?.role || '',
    permissions: user?.permissions || [],
    loading,
    error,
    can,
    refresh: fetchProfile,
  }

  return (
    <PermissionContext.Provider value={value}>
      {children}
    </PermissionContext.Provider>
  )
}

export function usePermissions() {
  const context = useContext(PermissionContext)
  return context || {
    user: null,
    role: '',
    permissions: [],
    loading: false,
    error: null,
    can: () => false,
    refresh: () => Promise.resolve(),
  }
}

export default PermissionContext
