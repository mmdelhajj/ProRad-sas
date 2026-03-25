import { useState, useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { logsApi } from '../services/api'
import { formatDateTime } from '../utils/timezone'

const EVENT_BADGE = {
  auth_accept: 'badge-success',
  auth_reject: 'badge-danger',
  acct_start: 'badge-info',
  acct_stop: 'badge-gray',
  acct_update: 'badge-secondary',
  coa: 'badge-purple',
}

const LEVEL_BADGE = {
  info: 'badge-info',
  warning: 'badge-warning',
  error: 'badge-danger',
}

export default function Logs() {
  const [activeTab, setActiveTab] = useState('radius')

  const tabs = [
    { id: 'radius', label: 'RADIUS Log' },
    { id: 'auth', label: 'Auth Log' },
    { id: 'system', label: 'System Log' },
  ]

  return (
    <div className="space-y-2" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Title */}
      <div className="wb-toolbar">
        <span className="text-[13px] font-semibold">Logs</span>
      </div>

      {/* Tabs */}
      <div className="wb-toolbar gap-0">
        {tabs.map(tab => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`px-3 py-1.5 text-[12px] font-medium border-b-2 transition-colors ${
              activeTab === tab.id
                ? 'border-blue-500 text-blue-600 dark:text-blue-400'
                : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {activeTab === 'radius' && <RadiusLogTab />}
      {activeTab === 'auth' && <AuthLogTab />}
      {activeTab === 'system' && <SystemLogTab />}
    </div>
  )
}

function RadiusLogTab() {
  const [page, setPage] = useState(1)
  const [eventType, setEventType] = useState('')
  const [username, setUsername] = useState('')
  const [nasIP, setNasIP] = useState('')
  const [dateFrom, setDateFrom] = useState('')
  const [dateTo, setDateTo] = useState('')
  const [autoRefresh, setAutoRefresh] = useState(false)

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['radius-logs', page, eventType, username, nasIP, dateFrom, dateTo],
    queryFn: () => logsApi.listRadius({ page, event_type: eventType, username, nas_ip: nasIP, date_from: dateFrom, date_to: dateTo }).then(res => res.data),
    refetchInterval: autoRefresh ? 10000 : false,
  })

  const logs = data?.data || []
  const meta = data?.meta || {}

  if (isLoading && logs.length === 0) {
    return <div className="flex items-center justify-center h-32"><div className="animate-spin h-5 w-5 border-2 border-[#316AC5] border-t-transparent" style={{ borderRadius: '50%' }}></div></div>
  }

  return (
    <>
      {/* Filters */}
      <div className="wb-toolbar flex-wrap gap-2">
        <select value={eventType} onChange={(e) => { setEventType(e.target.value); setPage(1) }} className="input" style={{ width: 'auto', minWidth: 130 }}>
          <option value="">All Events</option>
          <option value="auth_accept">Auth Accept</option>
          <option value="auth_reject">Auth Reject</option>
          <option value="acct_start">Acct Start</option>
          <option value="acct_stop">Acct Stop</option>
          <option value="acct_update">Acct Update</option>
          <option value="coa">CoA</option>
        </select>
        <input
          type="text" value={username} onChange={(e) => { setUsername(e.target.value); setPage(1) }}
          className="input" style={{ width: 'auto', minWidth: 140 }} placeholder="Username..."
        />
        <input
          type="text" value={nasIP} onChange={(e) => { setNasIP(e.target.value); setPage(1) }}
          className="input" style={{ width: 'auto', minWidth: 120 }} placeholder="NAS IP..."
        />
        <input type="date" value={dateFrom} onChange={(e) => { setDateFrom(e.target.value); setPage(1) }} className="input" style={{ width: 'auto' }} />
        <input type="date" value={dateTo} onChange={(e) => { setDateTo(e.target.value); setPage(1) }} className="input" style={{ width: 'auto' }} />
        <button onClick={() => { setEventType(''); setUsername(''); setNasIP(''); setDateFrom(''); setDateTo(''); setPage(1) }} className="btn">Clear</button>
        <label className="flex items-center gap-1 ml-auto text-[11px] text-gray-500 dark:text-gray-400 cursor-pointer">
          <input type="checkbox" checked={autoRefresh} onChange={(e) => setAutoRefresh(e.target.checked)} />
          Auto-refresh
        </label>
      </div>

      {/* Table */}
      <div className="table-container">
        <table className="table">
          <thead>
            <tr>
              <th>Time</th>
              <th>Event</th>
              <th>Username</th>
              <th>NAS IP</th>
              <th>Client IP</th>
              <th>MAC</th>
              <th>Reason</th>
              <th>Session</th>
            </tr>
          </thead>
          <tbody>
            {logs.map(log => (
              <tr key={log.id}>
                <td className="whitespace-nowrap">{formatDateTime(log.created_at)}</td>
                <td><span className={EVENT_BADGE[log.event_type] || 'badge-gray'}>{log.event_type}</span></td>
                <td className="font-medium">{log.username}</td>
                <td>{log.nas_ip}</td>
                <td>{log.client_ip}</td>
                <td className="text-[10px]">{log.mac_address}</td>
                <td>{log.reason && <span className="text-red-600 dark:text-red-400">{log.reason}</span>}</td>
                <td className="text-[10px] text-gray-400">{log.session_id ? log.session_id.substring(0, 12) : ''}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {logs.length === 0 && <div className="text-center py-4 text-[12px] text-gray-500">No RADIUS logs found</div>}

      {meta.totalPages > 1 && (
        <div className="wb-statusbar">
          <span>Page {page} of {meta.totalPages} ({meta.total} total)</span>
          <div className="flex gap-1">
            <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1} className="btn btn-sm">Previous</button>
            <button onClick={() => setPage(p => p + 1)} disabled={page >= meta.totalPages} className="btn btn-sm">Next</button>
          </div>
        </div>
      )}
    </>
  )
}

function AuthLogTab() {
  const [page, setPage] = useState(1)
  const [result, setResult] = useState('')
  const [reason, setReason] = useState('')
  const [username, setUsername] = useState('')
  const [dateFrom, setDateFrom] = useState('')
  const [dateTo, setDateTo] = useState('')
  const [autoRefresh, setAutoRefresh] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['auth-logs', page, result, reason, username, dateFrom, dateTo],
    queryFn: () => logsApi.listAuth({ page, result, reason, username, date_from: dateFrom, date_to: dateTo }).then(res => res.data),
    refetchInterval: autoRefresh ? 10000 : false,
  })

  const logs = data?.data || []
  const meta = data?.meta || {}
  const summary = data?.summary || {}

  if (isLoading && logs.length === 0) {
    return <div className="flex items-center justify-center h-32"><div className="animate-spin h-5 w-5 border-2 border-[#316AC5] border-t-transparent" style={{ borderRadius: '50%' }}></div></div>
  }

  return (
    <>
      {/* Summary Cards */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
        <div className="bg-white dark:bg-gray-800 rounded border border-gray-200 dark:border-gray-700 p-3 text-center">
          <div className="text-[20px] font-bold text-gray-900 dark:text-white">{summary.total_24h || 0}</div>
          <div className="text-[11px] text-gray-500 dark:text-gray-400">Total (24h)</div>
        </div>
        <div className="bg-white dark:bg-gray-800 rounded border border-green-200 dark:border-green-900 p-3 text-center">
          <div className="text-[20px] font-bold text-green-600 dark:text-green-400">{summary.accepts_24h || 0}</div>
          <div className="text-[11px] text-gray-500 dark:text-gray-400">Accepts (24h)</div>
        </div>
        <div className="bg-white dark:bg-gray-800 rounded border border-red-200 dark:border-red-900 p-3 text-center">
          <div className="text-[20px] font-bold text-red-600 dark:text-red-400">{summary.rejects_24h || 0}</div>
          <div className="text-[11px] text-gray-500 dark:text-gray-400">Rejects (24h)</div>
        </div>
        <div className="bg-white dark:bg-gray-800 rounded border border-orange-200 dark:border-orange-900 p-3 text-center">
          <div className="text-[20px] font-bold text-orange-600 dark:text-orange-400">{(summary.reject_percent || 0).toFixed(1)}%</div>
          <div className="text-[11px] text-gray-500 dark:text-gray-400">Reject Rate</div>
        </div>
      </div>

      {/* Filters */}
      <div className="wb-toolbar flex-wrap gap-2">
        <select value={result} onChange={(e) => { setResult(e.target.value); setPage(1) }} className="input" style={{ width: 'auto', minWidth: 120 }}>
          <option value="">All Results</option>
          <option value="accept">Accept</option>
          <option value="reject">Reject</option>
        </select>
        <select value={reason} onChange={(e) => { setReason(e.target.value); setPage(1) }} className="input" style={{ width: 'auto', minWidth: 140 }}>
          <option value="">All Reasons</option>
          <option value="not_found">Not Found</option>
          <option value="inactive">Inactive</option>
          <option value="expired">Expired</option>
          <option value="wrong_password">Wrong Password</option>
          <option value="mac_mismatch">MAC Mismatch</option>
          <option value="password_not_found">Password Not Found</option>
        </select>
        <input
          type="text" value={username} onChange={(e) => { setUsername(e.target.value); setPage(1) }}
          className="input" style={{ width: 'auto', minWidth: 140 }} placeholder="Username..."
        />
        <input type="date" value={dateFrom} onChange={(e) => { setDateFrom(e.target.value); setPage(1) }} className="input" style={{ width: 'auto' }} />
        <input type="date" value={dateTo} onChange={(e) => { setDateTo(e.target.value); setPage(1) }} className="input" style={{ width: 'auto' }} />
        <button onClick={() => { setResult(''); setReason(''); setUsername(''); setDateFrom(''); setDateTo(''); setPage(1) }} className="btn">Clear</button>
        <label className="flex items-center gap-1 ml-auto text-[11px] text-gray-500 dark:text-gray-400 cursor-pointer">
          <input type="checkbox" checked={autoRefresh} onChange={(e) => setAutoRefresh(e.target.checked)} />
          Auto-refresh
        </label>
      </div>

      {/* Table */}
      <div className="table-container">
        <table className="table">
          <thead>
            <tr>
              <th>Time</th>
              <th>Result</th>
              <th>Username</th>
              <th>NAS IP</th>
              <th>MAC</th>
              <th>Reason</th>
              <th>Duration</th>
            </tr>
          </thead>
          <tbody>
            {logs.map(log => (
              <tr key={log.id}>
                <td className="whitespace-nowrap">{formatDateTime(log.created_at)}</td>
                <td>
                  <span className={log.event_type === 'auth_accept' ? 'badge-success' : 'badge-danger'}>
                    {log.event_type === 'auth_accept' ? 'ACCEPT' : 'REJECT'}
                  </span>
                </td>
                <td className="font-medium">{log.username}</td>
                <td>{log.nas_ip}</td>
                <td className="text-[10px]">{log.mac_address}</td>
                <td>
                  {log.reason && <span className="text-red-600 dark:text-red-400">{log.reason}</span>}
                  {log.details && <span className="text-[10px] text-gray-400 ml-1">({log.details})</span>}
                </td>
                <td>{log.duration_ms > 0 ? `${log.duration_ms}ms` : ''}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {logs.length === 0 && <div className="text-center py-4 text-[12px] text-gray-500">No auth logs found</div>}

      {meta.totalPages > 1 && (
        <div className="wb-statusbar">
          <span>Page {page} of {meta.totalPages} ({meta.total} total)</span>
          <div className="flex gap-1">
            <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1} className="btn btn-sm">Previous</button>
            <button onClick={() => setPage(p => p + 1)} disabled={page >= meta.totalPages} className="btn btn-sm">Next</button>
          </div>
        </div>
      )}
    </>
  )
}

function SystemLogTab() {
  const [page, setPage] = useState(1)
  const [level, setLevel] = useState('')
  const [module, setModule] = useState('')
  const [search, setSearch] = useState('')
  const [dateFrom, setDateFrom] = useState('')
  const [dateTo, setDateTo] = useState('')
  const [autoRefresh, setAutoRefresh] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['system-logs', page, level, module, search, dateFrom, dateTo],
    queryFn: () => logsApi.listSystem({ page, level, module, search, date_from: dateFrom, date_to: dateTo }).then(res => res.data),
    refetchInterval: autoRefresh ? 10000 : false,
  })

  const logs = data?.data || []
  const meta = data?.meta || {}
  const modules = data?.modules || []

  if (isLoading && logs.length === 0) {
    return <div className="flex items-center justify-center h-32"><div className="animate-spin h-5 w-5 border-2 border-[#316AC5] border-t-transparent" style={{ borderRadius: '50%' }}></div></div>
  }

  return (
    <>
      {/* Filters */}
      <div className="wb-toolbar flex-wrap gap-2">
        <select value={level} onChange={(e) => { setLevel(e.target.value); setPage(1) }} className="input" style={{ width: 'auto', minWidth: 110 }}>
          <option value="">All Levels</option>
          <option value="info">Info</option>
          <option value="warning">Warning</option>
          <option value="error">Error</option>
        </select>
        <select value={module} onChange={(e) => { setModule(e.target.value); setPage(1) }} className="input" style={{ width: 'auto', minWidth: 130 }}>
          <option value="">All Modules</option>
          {modules.map(m => (
            <option key={m} value={m}>{m}</option>
          ))}
        </select>
        <input
          type="text" value={search} onChange={(e) => { setSearch(e.target.value); setPage(1) }}
          className="input" style={{ width: 'auto', minWidth: 160 }} placeholder="Search message..."
        />
        <input type="date" value={dateFrom} onChange={(e) => { setDateFrom(e.target.value); setPage(1) }} className="input" style={{ width: 'auto' }} />
        <input type="date" value={dateTo} onChange={(e) => { setDateTo(e.target.value); setPage(1) }} className="input" style={{ width: 'auto' }} />
        <button onClick={() => { setLevel(''); setModule(''); setSearch(''); setDateFrom(''); setDateTo(''); setPage(1) }} className="btn">Clear</button>
        <label className="flex items-center gap-1 ml-auto text-[11px] text-gray-500 dark:text-gray-400 cursor-pointer">
          <input type="checkbox" checked={autoRefresh} onChange={(e) => setAutoRefresh(e.target.checked)} />
          Auto-refresh
        </label>
      </div>

      {/* Table */}
      <div className="table-container">
        <table className="table">
          <thead>
            <tr>
              <th>Time</th>
              <th>Level</th>
              <th>Module</th>
              <th>Message</th>
              <th>Details</th>
            </tr>
          </thead>
          <tbody>
            {logs.map(log => (
              <tr key={log.id}>
                <td className="whitespace-nowrap">{formatDateTime(log.created_at)}</td>
                <td><span className={LEVEL_BADGE[log.level] || 'badge-gray'}>{log.level}</span></td>
                <td className="font-medium">{log.module}</td>
                <td style={{ maxWidth: 400, overflow: 'hidden', textOverflow: 'ellipsis' }}>{log.message}</td>
                <td className="text-[10px] text-gray-400" style={{ maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis' }}>{log.details}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {logs.length === 0 && <div className="text-center py-4 text-[12px] text-gray-500">No system logs found</div>}

      {meta.totalPages > 1 && (
        <div className="wb-statusbar">
          <span>Page {page} of {meta.totalPages} ({meta.total} total)</span>
          <div className="flex gap-1">
            <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1} className="btn btn-sm">Previous</button>
            <button onClick={() => setPage(p => p + 1)} disabled={page >= meta.totalPages} className="btn btn-sm">Next</button>
          </div>
        </div>
      )}
    </>
  )
}
