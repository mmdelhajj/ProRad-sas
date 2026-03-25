import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import api from '../services/api'
import { formatDateTime } from '../utils/timezone'

const ACTION_BADGE = {
  create: 'badge-success',
  update: 'badge-info',
  delete: 'badge-danger',
  login: 'badge-purple',
  logout: 'badge-gray',
  renew: 'badge-cyan',
  disconnect: 'badge-orange',
  transfer: 'badge-warning'
}

export default function AuditLogs() {
  const [page, setPage] = useState(1)
  const [action, setAction] = useState('')
  const [entityType, setEntityType] = useState('')
  const [dateFrom, setDateFrom] = useState('')
  const [dateTo, setDateTo] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['audit-logs', page, action, entityType, dateFrom, dateTo],
    queryFn: () => api.get('/audit', {
      params: { page, action, entity_type: entityType, date_from: dateFrom, date_to: dateTo }
    }).then(res => res.data)
  })

  const { data: actions } = useQuery({
    queryKey: ['audit-actions'],
    queryFn: () => api.get('/audit/actions').then(res => res.data.data)
  })

  const { data: entityTypes } = useQuery({
    queryKey: ['audit-entity-types'],
    queryFn: () => api.get('/audit/entity-types').then(res => res.data.data)
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin h-5 w-5 border-2 border-[#316AC5] border-t-transparent" style={{ borderRadius: '50%' }}></div>
      </div>
    )
  }

  const logs = data?.data || []
  const meta = data?.meta || {}

  return (
    <div className="space-y-2" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Title */}
      <div className="wb-toolbar">
        <span className="text-[13px] font-semibold">Audit Logs</span>
      </div>

      {/* Filters */}
      <div className="wb-toolbar flex-wrap gap-2">
        <select
          value={action}
          onChange={(e) => { setAction(e.target.value); setPage(1) }}
          className="input"
          style={{ width: 'auto', minWidth: 120 }}
        >
          <option value="">All Actions</option>
          {(actions || []).map(a => (
            <option key={a} value={a}>{a}</option>
          ))}
        </select>
        <select
          value={entityType}
          onChange={(e) => { setEntityType(e.target.value); setPage(1) }}
          className="input"
          style={{ width: 'auto', minWidth: 140 }}
        >
          <option value="">All Entity Types</option>
          {(entityTypes || []).map(et => (
            <option key={et} value={et}>{et}</option>
          ))}
        </select>
        <input
          type="date"
          value={dateFrom}
          onChange={(e) => { setDateFrom(e.target.value); setPage(1) }}
          className="input"
          style={{ width: 'auto' }}
          placeholder="From Date"
        />
        <input
          type="date"
          value={dateTo}
          onChange={(e) => { setDateTo(e.target.value); setPage(1) }}
          className="input"
          style={{ width: 'auto' }}
          placeholder="To Date"
        />
        <button
          onClick={() => {
            setAction('')
            setEntityType('')
            setDateFrom('')
            setDateTo('')
            setPage(1)
          }}
          className="btn"
        >
          Clear Filters
        </button>
      </div>

      {/* Logs Table */}
      <div className="table-container">
        <table className="table">
          <thead>
            <tr>
              <th>Timestamp</th>
              <th>User</th>
              <th>Action</th>
              <th>Entity</th>
              <th>Description</th>
            </tr>
          </thead>
          <tbody>
            {logs.map(log => (
              <tr key={log.id}>
                <td>{formatDateTime(log.created_at)}</td>
                <td>
                  <div className="font-semibold">{log.username || log.user?.username}</div>
                  <div className="text-[11px] text-gray-500">
                    {['Subscriber', 'Reseller', 'Support', 'Admin'][log.user_type - 1] || 'Unknown'}
                  </div>
                </td>
                <td>
                  <span className={ACTION_BADGE[log.action] || 'badge-gray'}>
                    {log.action}
                  </span>
                </td>
                <td>
                  <div>{log.entity_type}</div>
                  {log.entity_id > 0 && (
                    <div className="text-[11px] text-gray-400">ID: {log.entity_id}</div>
                  )}
                </td>
                <td style={{ maxWidth: 300, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                  {log.description}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {logs.length === 0 && (
        <div className="text-center py-4 text-[12px] text-gray-500">
          No audit logs found
        </div>
      )}

      {/* Pagination */}
      {meta.totalPages > 1 && (
        <div className="wb-statusbar">
          <span>
            Page {page} of {meta.totalPages} ({meta.total} total logs)
          </span>
          <div className="flex gap-1">
            <button
              onClick={() => setPage(p => Math.max(1, p - 1))}
              disabled={page === 1}
              className="btn btn-sm"
            >
              Previous
            </button>
            <button
              onClick={() => setPage(p => p + 1)}
              disabled={page >= meta.totalPages}
              className="btn btn-sm"
            >
              Next
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
