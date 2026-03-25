import { useState, useMemo } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { sessionApi } from '../services/api'
import { useAuthStore } from '../store/authStore'
import { formatDateTime } from '../utils/timezone'
import {
  useReactTable,
  getCoreRowModel,
  flexRender,
} from '@tanstack/react-table'
import {
  ArrowPathIcon,
  XCircleIcon,
  SignalIcon,
  ClockIcon,
  ArrowDownTrayIcon,
  ArrowUpTrayIcon,
  MagnifyingGlassIcon,
} from '@heroicons/react/24/outline'
import toast from 'react-hot-toast'
import clsx from 'clsx'

function formatBytes(bytes) {
  if (!bytes) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function formatDuration(seconds) {
  if (!seconds) return '0s'
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const secs = seconds % 60
  if (hours > 0) return `${hours}h ${minutes}m`
  if (minutes > 0) return `${minutes}m ${secs}s`
  return `${secs}s`
}

export default function Sessions() {
  const queryClient = useQueryClient()
  const { hasPermission } = useAuthStore()
  const [search, setSearch] = useState('')
  const [exportStartDate, setExportStartDate] = useState('')
  const [exportEndDate, setExportEndDate] = useState('')
  const [exporting, setExporting] = useState(false)

  const { data: sessions, isLoading, refetch } = useQuery({
    queryKey: ['sessions'],
    queryFn: () => sessionApi.list({ limit: 500 }).then((r) => r.data.data),
    refetchInterval: 10000, // Refresh every 10 seconds
  })

  const disconnectMutation = useMutation({
    mutationFn: (id) => sessionApi.disconnect(id),
    onSuccess: () => {
      toast.success('Session disconnected')
      queryClient.invalidateQueries(['sessions'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to disconnect'),
  })

  const filteredSessions = useMemo(() => {
    if (!sessions) return []
    if (!search) return sessions
    const s = search.toLowerCase()
    return sessions.filter(
      (session) =>
        session.username?.toLowerCase().includes(s) ||
        session.framed_ip_address?.includes(s) ||
        session.nas_ip_address?.includes(s) ||
        session.calling_station_id?.toLowerCase().includes(s) ||
        session.full_name?.toLowerCase().includes(s)
    )
  }, [sessions, search])

  const handleExportCSV = async () => {
    setExporting(true)
    try {
      const params = {}
      if (exportStartDate) params.start_date = exportStartDate
      if (exportEndDate) params.end_date = exportEndDate
      const response = await sessionApi.exportCSV(params)
      const url = window.URL.createObjectURL(new Blob([response.data]))
      const link = document.createElement('a')
      link.href = url
      link.setAttribute('download', `sessions_${new Date().toISOString().split('T')[0]}.csv`)
      document.body.appendChild(link)
      link.click()
      link.remove()
      window.URL.revokeObjectURL(url)
      toast.success('CSV exported successfully')
    } catch (err) {
      toast.error('Failed to export CSV')
    } finally {
      setExporting(false)
    }
  }

  const columns = useMemo(
    () => [
      {
        accessorKey: 'username',
        header: 'Username',
        cell: ({ row }) => (
          <div style={{ display: 'flex', flexDirection: 'column' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <span style={{ display: 'inline-block', width: 8, height: 8, borderRadius: '50%', backgroundColor: '#4CAF50' }}></span>
              <span className="dark:text-white" style={{ fontWeight: 500, fontSize: 11 }}>{row.original.username}</span>
            </div>
            {row.original.full_name && (
              <span className="text-gray-500 dark:text-gray-400" style={{ fontSize: 10, marginLeft: 12 }}>{row.original.full_name}</span>
            )}
          </div>
        ),
      },
      {
        accessorKey: 'framed_ip_address',
        header: 'IP Address',
        cell: ({ row }) => (
          <code className="bg-[#f0f0f0] dark:bg-gray-700 border border-[#d0d0d0] dark:border-gray-600 dark:text-gray-200" style={{ borderRadius: 2, fontSize: 11, padding: '1px 4px' }}>
            {row.original.framed_ip_address || '-'}
          </code>
        ),
      },
      {
        accessorKey: 'calling_station_id',
        header: 'MAC Address',
        cell: ({ row }) => (
          <code className="bg-[#f0f0f0] dark:bg-gray-700 border border-[#d0d0d0] dark:border-gray-600 dark:text-gray-200" style={{ borderRadius: 2, fontSize: 10, padding: '1px 4px' }}>
            {row.original.calling_station_id || '-'}
          </code>
        ),
      },
      {
        accessorKey: 'nas_name',
        header: 'NAS',
        cell: ({ row }) => (
          <div style={{ display: 'flex', flexDirection: 'column' }}>
            <span className="dark:text-gray-200" style={{ fontSize: 11 }}>{row.original.nas_name || row.original.nas_ip_address || '-'}</span>
            {row.original.nas_name && row.original.nas_ip_address && (
              <span className="text-gray-500 dark:text-gray-400" style={{ fontSize: 10 }}>{row.original.nas_ip_address}</span>
            )}
          </div>
        ),
      },
      {
        accessorKey: 'acct_start_time',
        header: 'Started',
        cell: ({ row }) => (
          <span className="dark:text-gray-200" style={{ fontSize: 11 }}>
            {formatDateTime(row.original.acct_start_time)}
          </span>
        ),
      },
      {
        accessorKey: 'session_duration',
        header: 'Duration',
        cell: ({ row }) => (
          <div className="dark:text-gray-200" style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 11 }}>
            <ClockIcon className="w-3 h-3 text-gray-500 dark:text-gray-400" />
            {formatDuration(row.original.session_duration)}
          </div>
        ),
      },
      {
        accessorKey: 'traffic',
        header: 'Traffic',
        cell: ({ row }) => (
          <div style={{ fontSize: 11 }}>
            <div className="text-green-700 dark:text-green-400" style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <ArrowDownTrayIcon className="w-3 h-3" />
              {formatBytes(row.original.acct_input_octets)}
            </div>
            <div className="text-blue-700 dark:text-blue-400" style={{ display: 'flex', alignItems: 'center', gap: 4, marginTop: 2 }}>
              <ArrowUpTrayIcon className="w-3 h-3" />
              {formatBytes(row.original.acct_output_octets)}
            </div>
          </div>
        ),
      },
      {
        id: 'actions',
        header: 'Actions',
        cell: ({ row }) => (
          hasPermission('subscribers.disconnect') ? (
            <button
              onClick={() => {
                if (confirm('Disconnect this session?')) {
                  disconnectMutation.mutate(row.original.id)
                }
              }}
              className="btn btn-sm btn-danger"
              title="Disconnect"
              style={{ padding: '2px 6px', fontSize: 11 }}
            >
              <XCircleIcon className="w-3.5 h-3.5 inline mr-1" />
              Disconnect
            </button>
          ) : null
        ),
      },
    ],
    [disconnectMutation, hasPermission]
  )

  const table = useReactTable({
    data: filteredSessions,
    columns,
    getCoreRowModel: getCoreRowModel(),
  })

  const totalDownload = sessions?.reduce((sum, s) => sum + (s.acct_input_octets || 0), 0) || 0
  const totalUpload = sessions?.reduce((sum, s) => sum + (s.acct_output_octets || 0), 0) || 0

  return (
    <div style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Page Title + Toolbar */}
      <div className="wb-toolbar" style={{ marginBottom: 4, display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: 4 }}>
        <span className="dark:text-gray-100" style={{ fontSize: 13, fontWeight: 600 }}>Active Sessions</span>
        <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
          <input type="date" value={exportStartDate} onChange={e => setExportStartDate(e.target.value)} className="px-1 py-0.5 border border-[#a0a0a0] dark:border-[#555] bg-white dark:bg-[#333] dark:text-white text-[10px]" style={{ borderRadius: 2 }} title="Export start date" />
          <input type="date" value={exportEndDate} onChange={e => setExportEndDate(e.target.value)} className="px-1 py-0.5 border border-[#a0a0a0] dark:border-[#555] bg-white dark:bg-[#333] dark:text-white text-[10px]" style={{ borderRadius: 2 }} title="Export end date" />
          <button onClick={handleExportCSV} disabled={exporting} className="btn btn-sm" style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            <ArrowDownTrayIcon className="w-3.5 h-3.5" />
            {exporting ? 'Exporting...' : 'Export CSV'}
          </button>
          <button onClick={() => refetch()} className="btn btn-sm" style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            <ArrowPathIcon className="w-3.5 h-3.5" />
            Refresh
          </button>
        </div>
      </div>

      {/* Stats */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 4, marginBottom: 4 }}>
        <div className="stat-card dark:bg-gray-800 dark:border-gray-600" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ display: 'inline-block', width: 8, height: 8, borderRadius: '50%', backgroundColor: '#4CAF50' }}></span>
          <div>
            <div className="label dark:text-gray-400" style={{ fontSize: 10 }}>Online Users</div>
            <div className="dark:text-white" style={{ fontSize: 13, fontWeight: 700 }}>{sessions?.length || 0}</div>
          </div>
        </div>
        <div className="stat-card dark:bg-gray-800 dark:border-gray-600" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <ArrowDownTrayIcon className="w-3.5 h-3.5 text-blue-600" />
          <div>
            <div className="label dark:text-gray-400" style={{ fontSize: 10 }}>Total Download</div>
            <div className="dark:text-white" style={{ fontSize: 13, fontWeight: 700 }}>{formatBytes(totalDownload)}</div>
          </div>
        </div>
        <div className="stat-card dark:bg-gray-800 dark:border-gray-600" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <ArrowUpTrayIcon className="w-3.5 h-3.5 text-purple-600" />
          <div>
            <div className="label dark:text-gray-400" style={{ fontSize: 10 }}>Total Upload</div>
            <div className="dark:text-white" style={{ fontSize: 13, fontWeight: 700 }}>{formatBytes(totalUpload)}</div>
          </div>
        </div>
        <div className="stat-card dark:bg-gray-800 dark:border-gray-600" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <ClockIcon className="w-3.5 h-3.5 text-yellow-600" />
          <div>
            <div className="label dark:text-gray-400" style={{ fontSize: 10 }}>Auto-refresh</div>
            <div className="dark:text-white" style={{ fontSize: 13, fontWeight: 700 }}>Every 10s</div>
          </div>
        </div>
      </div>

      {/* Search */}
      <div className="card dark:bg-gray-800 dark:border-gray-600" style={{ padding: '4px 8px', marginBottom: 4 }}>
        <div style={{ position: 'relative', maxWidth: 320 }}>
          <MagnifyingGlassIcon className="w-3.5 h-3.5 text-gray-500 dark:text-gray-400" style={{ position: 'absolute', left: 6, top: '50%', transform: 'translateY(-50%)' }} />
          <input
            type="text"
            placeholder="Search by username, IP, MAC..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="input input-sm"
            style={{ paddingLeft: 24, fontSize: 11, height: 24 }}
          />
        </div>
      </div>

      {/* Table */}
      <div className="table-container">
        <table className="table">
          <thead>
            {table.getHeaderGroups().map((headerGroup) => (
              <tr key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <th key={header.id} style={{ fontSize: 11, padding: '4px 8px' }}>
                    {flexRender(header.column.columnDef.header, header.getContext())}
                  </th>
                ))}
              </tr>
            ))}
          </thead>
          <tbody>
            {isLoading ? (
              <tr>
                <td colSpan={columns.length} className="text-center dark:text-gray-400" style={{ padding: 24, fontSize: 11 }}>
                  Loading...
                </td>
              </tr>
            ) : table.getRowModel().rows.length === 0 ? (
              <tr>
                <td colSpan={columns.length} className="text-center text-gray-500 dark:text-gray-400" style={{ padding: 24, fontSize: 11 }}>
                  No active sessions
                </td>
              </tr>
            ) : (
              table.getRowModel().rows.map((row) => (
                <tr key={row.id}>
                  {row.getVisibleCells().map((cell) => (
                    <td key={cell.id} style={{ padding: '3px 8px', fontSize: 11 }}>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
