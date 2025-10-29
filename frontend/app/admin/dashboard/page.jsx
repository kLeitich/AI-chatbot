"use client"
import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import AdminDashboard from '../../../components/AdminDashboard'

export default function Page() {
  const router = useRouter()
  useEffect(() => {
    const token = typeof window !== 'undefined' && localStorage.getItem('token')
    if (!token) router.replace('/admin/login')
  }, [router])
  return <AdminDashboard />
}
