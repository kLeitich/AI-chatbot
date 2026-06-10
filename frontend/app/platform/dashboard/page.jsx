"use client"
import { useEffect, useState } from 'react'
import DashboardNav from '../../../components/DashboardNav'
import api from '../../../lib/api'

export default function Page() {
  const [companies, setCompanies] = useState([])
  const [users, setUsers] = useState([])
  const [error, setError] = useState('')

  useEffect(() => {
    const loadPlatformData = async () => {
      try {
        const [companiesRes, usersRes] = await Promise.all([
          api.get('/platform/companies'),
          api.get('/platform/users'),
        ])
        setCompanies(companiesRes.data || [])
        setUsers(usersRes.data || [])
      } catch (err) {
        setError('Unable to load platform data. Please log in and try again.')
        console.error(err)
      }
    }
    loadPlatformData()
  }, [])

  return (
      <div className="min-h-screen bg-slate-50">
          <DashboardNav isPlatform={true} />
          <div className="mx-auto max-w-6xl p-6 space-y-6">
        <div className="rounded-3xl bg-white p-6 shadow-sm">
          <h1 className="text-2xl font-semibold text-slate-900">Platform Dashboard</h1>
          <p className="mt-2 text-sm text-slate-600">View all tenant companies and registered users across the platform.</p>
        </div>

        {error ? (
          <div className="rounded-xl bg-red-50 p-4 text-sm text-red-700">{error}</div>
        ) : null}

        <div className="grid gap-6 lg:grid-cols-2">
          <section className="rounded-3xl bg-white p-6 shadow-sm">
            <h2 className="text-xl font-semibold text-slate-900">Companies</h2>
            <div className="mt-4 space-y-3">
              {companies.length === 0 ? (
                <p className="text-sm text-slate-600">No companies found yet.</p>
              ) : (
                companies.map((company) => (
                  <div key={company.id} className="rounded-2xl border border-slate-200 p-4">
                    <p className="font-semibold text-slate-900">{company.name}</p>
                    <p className="text-sm text-slate-600">Slug: {company.slug}</p>
                    <p className="text-sm text-slate-600">Created: {new Date(company.created_at).toLocaleDateString()}</p>
                  </div>
                ))
              )}
            </div>
          </section>

          <section className="rounded-3xl bg-white p-6 shadow-sm">
            <h2 className="text-xl font-semibold text-slate-900">Users</h2>
            <div className="mt-4 space-y-3">
              {users.length === 0 ? (
                <p className="text-sm text-slate-600">No users found yet.</p>
              ) : (
                users.map((user) => (
                  <div key={user.id} className="rounded-2xl border border-slate-200 p-4">
                    <p className="font-semibold text-slate-900">{user.email}</p>
                    <p className="text-sm text-slate-600">Role: {user.role || 'user'}</p>
                    <p className="text-sm text-slate-600">Tenant ID: {user.tenant_id}</p>
                    <p className="text-sm text-slate-600">Joined: {new Date(user.created_at).toLocaleDateString()}</p>
                  </div>
                ))
              )}
            </div>
          </section>
        </div>
      </div>
    </div>
  )
}
