"use client"
import { useEffect, useState, useCallback } from 'react'
import api from '../lib/api'

export default function AuditLogTable({ tenant = 'default' }) {
  const [logs, setLogs] = useState([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [limit] = useState(25)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  // Filters
  const [filterAction, setFilterAction] = useState('')
  const [filterEntity, setFilterEntity] = useState('')

  const fetchLogs = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const params = {
        page,
        limit,
      }
      if (filterAction.trim()) params.action = filterAction.trim()
      if (filterEntity.trim()) params.entity = filterEntity.trim()

      const res = await api.get(`/t/${tenant}/admin/audit`, { params })
      const data = res.data || {}
      setLogs(data.logs || [])
      setTotal(data.total || 0)
    } catch (err) {
      setError(err?.response?.data?.message || 'Failed to load audit logs')
      setLogs([])
    } finally {
      setLoading(false)
    }
  }, [tenant, page, limit, filterAction, filterEntity])

  useEffect(() => {
    fetchLogs()
  }, [fetchLogs])

  const totalPages = Math.max(1, Math.ceil(total / limit))

  const handleResetFilters = () => {
    setFilterAction('')
    setFilterEntity('')
    setPage(1)
  }

  const formatTimestamp = (ts) => {
    if (!ts) return '-'
    try {
      return new Date(ts).toLocaleString(undefined, {
        year: 'numeric',
        month: 'short',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
      })
    } catch {
      return ts
    }
  }

  return (
    <div className="bg-white border rounded-xl shadow-sm p-5 space-y-4">
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 border-b pb-3">
        <div>
          <h2 className="text-lg font-bold text-gray-900">Security Audit Logs</h2>
          <p className="text-xs text-gray-500">Immutable audit trail of access, changes, and authentication events.</p>
        </div>
        <div className="text-xs text-gray-500 font-medium">
          Total Events: <span className="font-bold text-gray-900">{total}</span>
        </div>
      </div>

      {/* Filters */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 bg-gray-50 p-3 rounded-lg border">
        <div>
          <label className="block text-xs font-semibold text-gray-700 mb-1">Filter Action</label>
          <input
            type="text"
            placeholder="e.g. login_success, update_status"
            className="w-full border rounded p-2 text-xs bg-white focus:outline-none focus:ring-1 focus:ring-blue-500"
            value={filterAction}
            onChange={(e) => {
              setFilterAction(e.target.value)
              setPage(1)
            }}
          />
        </div>
        <div>
          <label className="block text-xs font-semibold text-gray-700 mb-1">Filter Entity</label>
          <input
            type="text"
            placeholder="e.g. user, settings, auth"
            className="w-full border rounded p-2 text-xs bg-white focus:outline-none focus:ring-1 focus:ring-blue-500"
            value={filterEntity}
            onChange={(e) => {
              setFilterEntity(e.target.value)
              setPage(1)
            }}
          />
        </div>
        <div className="flex items-end">
          <button
            type="button"
            onClick={handleResetFilters}
            className="w-full py-2 px-3 border rounded text-xs font-medium text-gray-700 hover:bg-gray-100 transition"
          >
            Clear Filters
          </button>
        </div>
      </div>

      {error && (
        <div className="p-3 bg-red-50 text-red-700 border border-red-200 text-xs rounded-lg">
          {error}
        </div>
      )}

      {/* Table */}
      <div className="overflow-x-auto">
        <table className="min-w-full text-xs">
          <thead className="bg-gray-100 text-left text-gray-700 uppercase tracking-wider font-semibold">
            <tr>
              <th className="p-2.5">Timestamp</th>
              <th className="p-2.5">Actor</th>
              <th className="p-2.5">Action</th>
              <th className="p-2.5">Entity</th>
              <th className="p-2.5">IP Address</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-200 font-mono">
            {loading ? (
              <tr>
                <td colSpan={5} className="p-6 text-center text-gray-500 font-sans">Loading audit events...</td>
              </tr>
            ) : logs.length === 0 ? (
              <tr>
                <td colSpan={5} className="p-6 text-center text-gray-500 font-sans">No audit events found.</td>
              </tr>
            ) : (
              logs.map((log) => (
                <tr key={log.id} className="hover:bg-gray-50">
                  <td className="p-2.5 whitespace-nowrap text-gray-600 font-sans">
                    {formatTimestamp(log.at)}
                  </td>
                  <td className="p-2.5 whitespace-nowrap">
                    <div className="font-sans">
                      <span className="font-semibold text-gray-900">User #{log.actor_user_id || '0'}</span>{' '}
                      <span className="text-[11px] text-gray-500">({log.actor_role})</span>
                    </div>
                  </td>
                  <td className="p-2.5 whitespace-nowrap">
                    <span className={`px-2 py-0.5 rounded text-[11px] font-bold ${
                      log.action.includes('failure') ? 'bg-red-100 text-red-800' :
                      log.action.includes('success') ? 'bg-green-100 text-green-800' :
                      log.action.includes('update') ? 'bg-blue-100 text-blue-800' :
                      'bg-gray-100 text-gray-800'
                    }`}>
                      {log.action}
                    </span>
                  </td>
                  <td className="p-2.5 whitespace-nowrap font-sans">
                    <span className="font-medium text-gray-800">{log.entity}</span>
                    {log.entity_id > 0 && <span className="text-gray-500 ml-1">#{log.entity_id}</span>}
                  </td>
                  <td className="p-2.5 whitespace-nowrap text-gray-500">
                    {log.ip || '-'}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      <div className="flex items-center justify-between pt-3 border-t text-xs text-gray-600">
        <div>
          Showing page <span className="font-bold text-gray-900">{page}</span> of{' '}
          <span className="font-bold text-gray-900">{totalPages}</span>
        </div>
        <div className="flex gap-2">
          <button
            type="button"
            disabled={page <= 1 || loading}
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            className="px-3 py-1.5 border rounded hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed transition"
          >
            ← Previous
          </button>
          <button
            type="button"
            disabled={page >= totalPages || loading}
            onClick={() => setPage((p) => p + 1)}
            className="px-3 py-1.5 border rounded hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed transition"
          >
            Next →
          </button>
        </div>
      </div>
    </div>
  )
}

