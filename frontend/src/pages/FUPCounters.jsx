import { useState, useMemo } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import api from '../services/api'

const columnHelper = createColumnHelper()

function formatBytes(bytes) {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function QuotaBar({ used, total }) {
  if (total === 0) {
    return <span className="badge-gray">Unlimited</span>
  }

  const percent = Math.min((used / total) * 100, 100)
  const color = percent >= 100 ? 'bg-red-500' : percent >= 80 ? 'bg-yellow-500' : 'bg-green-500'

  return (
    <div style={{ minWidth: '100px' }}>
      <div className="flex justify-between text-[11px] mb-0.5">
        <span className="text-gray-700">{formatBytes(used)}</span>
        <span className="text-gray-400">{formatBytes(total)}</span>
      </div>
      <div className="wb-usage-bar">
        <div
          className={`wb-usage-bar-fill ${color}`}
          style={{ width: `${percent}%` }}
        />
      </div>
      <div className="text-[11px] text-gray-500 text-right">{percent.toFixed(1)}%</div>
    </div>
  )
}

export default function FUPCounters() {
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')
  const [fupStatus, setFupStatus] = useState('')
  const [quotaStatus, setQuotaStatus] = useState('')
  const [selectedRows, setSelectedRows] = useState({})

  // Fetch stats
  const { data: statsData } = useQuery({
    queryKey: ['fup-stats'],
    queryFn: () => api.get('/fup/stats').then(res => res.data.data),
    refetchInterval: 30000,
  })

  // Fetch quotas
  const { data: quotasData, isLoading } = useQuery({
    queryKey: ['fup-quotas', page, search, fupStatus, quotaStatus],
    queryFn: () => api.get('/fup/quotas', {
      params: { page, limit: 25, search, fup_status: fupStatus, quota_status: quotaStatus }
    }).then(res => res.data),
  })

  // Fetch top users
  const { data: topUsersData } = useQuery({
    queryKey: ['fup-top-users'],
    queryFn: () => api.get('/fup/top-users', { params: { limit: 5, period: 'monthly' } }).then(res => res.data.data),
  })

  // Reset FUP mutation
  const resetMutation = useMutation({
    mutationFn: (id) => api.post(`/fup/reset/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries(['fup-quotas'])
      queryClient.invalidateQueries(['fup-stats'])
    },
  })

  // Bulk reset mutation
  const bulkResetMutation = useMutation({
    mutationFn: (data) => api.post('/fup/bulk-reset', data),
    onSuccess: () => {
      queryClient.invalidateQueries(['fup-quotas'])
      queryClient.invalidateQueries(['fup-stats'])
      setSelectedRows({})
    },
  })

  // Reset all FUP mutation
  const resetAllMutation = useMutation({
    mutationFn: () => api.post('/fup/reset-all'),
    onSuccess: () => {
      queryClient.invalidateQueries(['fup-quotas'])
      queryClient.invalidateQueries(['fup-stats'])
    },
  })

  const columns = useMemo(() => [
    columnHelper.display({
      id: 'select',
      header: ({ table }) => (
        <input
          type="checkbox"
          checked={table.getIsAllRowsSelected()}
          onChange={table.getToggleAllRowsSelectedHandler()}
          className="border-[#a0a0a0]"
        />
      ),
      cell: ({ row }) => (
        <input
          type="checkbox"
          checked={row.getIsSelected()}
          onChange={row.getToggleSelectedHandler()}
          className="border-[#a0a0a0]"
        />
      ),
    }),
    columnHelper.accessor('username', {
      header: 'Username',
      cell: info => (
        <div>
          <div className="font-medium">{info.getValue()}</div>
          <div className="text-[11px] text-gray-500">{info.row.original.full_name}</div>
        </div>
      ),
    }),
    columnHelper.accessor('service_name', {
      header: 'Service',
    }),
    columnHelper.accessor('reseller_name', {
      header: 'Reseller',
    }),
    columnHelper.display({
      id: 'daily_quota',
      header: 'Daily Quota',
      cell: ({ row }) => (
        <QuotaBar
          used={row.original.daily_used}
          total={row.original.daily_quota}
        />
      ),
    }),
    columnHelper.display({
      id: 'monthly_quota',
      header: 'Monthly Quota',
      cell: ({ row }) => (
        <QuotaBar
          used={row.original.monthly_used}
          total={row.original.monthly_quota}
        />
      ),
    }),
    columnHelper.accessor('fup_level', {
      header: 'FUP Level',
      cell: info => {
        const level = info.getValue()
        return level === 0
          ? <span className="badge-success">Normal</span>
          : <span className="badge-danger">Level {level}</span>
      },
    }),
    columnHelper.accessor('is_online', {
      header: 'Status',
      cell: info => (
        info.getValue()
          ? <span className="badge-success">Online</span>
          : <span className="badge-gray">Offline</span>
      ),
    }),
    columnHelper.display({
      id: 'actions',
      header: 'Actions',
      cell: ({ row }) => (
        <button
          onClick={() => resetMutation.mutate(row.original.id)}
          disabled={resetMutation.isPending}
          className="btn btn-sm"
        >
          Reset
        </button>
      ),
    }),
  ], [resetMutation])

  const table = useReactTable({
    data: quotasData?.data || [],
    columns,
    getCoreRowModel: getCoreRowModel(),
    state: {
      rowSelection: selectedRows,
    },
    onRowSelectionChange: setSelectedRows,
    getRowId: (row) => String(row.id),
  })

  const selectedIds = Object.keys(selectedRows).map(Number)

  const handleBulkReset = (resetType) => {
    if (selectedIds.length === 0) return
    bulkResetMutation.mutate({
      subscriber_ids: selectedIds,
      reset_type: resetType,
    })
  }

  const stats = statsData || {}

  return (
    <div className="space-y-3" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Header */}
      <div className="wb-toolbar justify-between">
        <div className="flex items-center gap-2">
          <span className="text-[13px] font-semibold text-gray-800">FUP & Counters</span>
          <span className="text-[11px] text-gray-500">Manage Fair Usage Policy and quota counters</span>
        </div>
        <button
          onClick={() => resetAllMutation.mutate()}
          disabled={resetAllMutation.isPending || stats.active_fup === 0}
          className="btn btn-danger btn-sm"
        >
          Reset All FUP ({stats.active_fup || 0})
        </button>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-5 gap-2">
        <div className="stat-card">
          <div className="flex items-center gap-1 mb-0.5">
            <span className="wb-status-dot bg-[#316AC5]"></span>
            <span className="text-[11px] text-gray-500">Total Subscribers</span>
          </div>
          <div className="text-[16px] font-bold text-[#316AC5]">{stats.total_subscribers || 0}</div>
        </div>
        <div className="stat-card">
          <div className="flex items-center gap-1 mb-0.5">
            <span className="wb-status-dot bg-red-500"></span>
            <span className="text-[11px] text-gray-500">Active FUP</span>
          </div>
          <div className="text-[16px] font-bold text-red-600">{stats.active_fup || 0}</div>
        </div>
        <div className="stat-card">
          <div className="flex items-center gap-1 mb-0.5">
            <span className="wb-status-dot bg-yellow-500"></span>
            <span className="text-[11px] text-gray-500">Daily Exceeded</span>
          </div>
          <div className="text-[16px] font-bold text-yellow-600">{stats.daily_quota_exceeded || 0}</div>
        </div>
        <div className="stat-card">
          <div className="flex items-center gap-1 mb-0.5">
            <span className="wb-status-dot bg-orange-500"></span>
            <span className="text-[11px] text-gray-500">Monthly Exceeded</span>
          </div>
          <div className="text-[16px] font-bold text-orange-600">{stats.monthly_quota_exceeded || 0}</div>
        </div>
        <div className="stat-card">
          <div className="flex items-center gap-1 mb-0.5">
            <span className="wb-status-dot bg-green-500"></span>
            <span className="text-[11px] text-gray-500">Unlimited</span>
          </div>
          <div className="text-[16px] font-bold text-green-600">{stats.unlimited_quota || 0}</div>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-4 gap-2">
        {/* Main Table */}
        <div className="lg:col-span-3">
          {/* Filters */}
          <div className="wb-group mb-2">
            <div className="wb-group-title">Filters</div>
            <div className="wb-group-body">
              <div className="flex flex-wrap gap-2 items-center">
                <div style={{ minWidth: '180px', flex: 1, maxWidth: '250px' }}>
                  <input
                    type="text"
                    placeholder="Search username..."
                    value={search}
                    onChange={(e) => {
                      setSearch(e.target.value)
                      setPage(1)
                    }}
                    className="input"
                  />
                </div>
                <select
                  value={fupStatus}
                  onChange={(e) => {
                    setFupStatus(e.target.value)
                    setPage(1)
                  }}
                  className="input" style={{ width: '140px' }}
                >
                  <option value="">All FUP Status</option>
                  <option value="active">Active FUP</option>
                  <option value="normal">Normal</option>
                </select>
                <select
                  value={quotaStatus}
                  onChange={(e) => {
                    setQuotaStatus(e.target.value)
                    setPage(1)
                  }}
                  className="input" style={{ width: '160px' }}
                >
                  <option value="">All Quota Status</option>
                  <option value="daily_exceeded">Daily Exceeded</option>
                  <option value="monthly_exceeded">Monthly Exceeded</option>
                  <option value="warning">Warning (80%+)</option>
                  <option value="unlimited">Unlimited</option>
                </select>
              </div>

              {/* Bulk Actions */}
              {selectedIds.length > 0 && (
                <div className="mt-2 flex items-center gap-1 p-2 bg-[#e8e8f0] border border-[#a0a0a0]" style={{ borderRadius: '2px' }}>
                  <span className="text-[12px] font-medium text-gray-700 mr-2">
                    {selectedIds.length} selected:
                  </span>
                  <button
                    onClick={() => handleBulkReset('all')}
                    disabled={bulkResetMutation.isPending}
                    className="btn btn-primary btn-sm"
                  >
                    Reset All
                  </button>
                  <button
                    onClick={() => handleBulkReset('fup')}
                    disabled={bulkResetMutation.isPending}
                    className="btn btn-sm"
                    style={{ background: '#d4a017', color: 'white', borderColor: '#b8860b' }}
                  >
                    Reset FUP
                  </button>
                  <button
                    onClick={() => handleBulkReset('daily')}
                    disabled={bulkResetMutation.isPending}
                    className="btn btn-success btn-sm"
                  >
                    Reset Daily
                  </button>
                  <button
                    onClick={() => handleBulkReset('monthly')}
                    disabled={bulkResetMutation.isPending}
                    className="btn btn-sm"
                    style={{ background: '#8e24aa', color: 'white', borderColor: '#6a1b9a' }}
                  >
                    Reset Monthly
                  </button>
                </div>
              )}
            </div>
          </div>

          {/* Table */}
          <div className="table-container">
            <table className="table">
              <thead>
                {table.getHeaderGroups().map(headerGroup => (
                  <tr key={headerGroup.id}>
                    {headerGroup.headers.map(header => (
                      <th key={header.id}>
                        {flexRender(header.column.columnDef.header, header.getContext())}
                      </th>
                    ))}
                  </tr>
                ))}
              </thead>
              <tbody>
                {isLoading ? (
                  <tr>
                    <td colSpan={columns.length} className="text-center py-3">
                      <span className="text-[12px] text-gray-500">Loading...</span>
                    </td>
                  </tr>
                ) : table.getRowModel().rows.length === 0 ? (
                  <tr>
                    <td colSpan={columns.length} className="text-center py-3 text-[12px] text-gray-500">
                      No subscribers found
                    </td>
                  </tr>
                ) : (
                  table.getRowModel().rows.map(row => (
                    <tr key={row.id}>
                      {row.getVisibleCells().map(cell => (
                        <td key={cell.id}>
                          {flexRender(cell.column.columnDef.cell, cell.getContext())}
                        </td>
                      ))}
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>

          {/* Pagination */}
          {quotasData?.meta && (
            <div className="wb-statusbar mt-0">
              <div className="text-[12px] text-gray-500">
                Showing {((page - 1) * 25) + 1} to {Math.min(page * 25, quotasData.meta.total)} of {quotasData.meta.total}
              </div>
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
                  disabled={page >= quotasData.meta.totalPages}
                  className="btn btn-sm"
                >
                  Next
                </button>
              </div>
            </div>
          )}
        </div>

        {/* Top Users Sidebar */}
        <div className="wb-group">
          <div className="wb-group-title">Top Quota Users</div>
          <div className="wb-group-body">
            <div className="space-y-3">
              {topUsersData?.map((user, index) => (
                <div key={user.id} className="flex items-center gap-2">
                  <div
                    className="w-5 h-5 flex items-center justify-center text-white font-bold text-[11px]"
                    style={{
                      borderRadius: '2px',
                      background: index === 0 ? '#d4a017' : index === 1 ? '#9e9e9e' : index === 2 ? '#8d6e36' : '#bdbdbd',
                    }}
                  >
                    {index + 1}
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="font-medium text-[12px] truncate">{user.username}</p>
                    <p className="text-[11px] text-gray-500">{user.service_name}</p>
                  </div>
                  <div className="text-right">
                    <p className="text-[12px] font-medium">{formatBytes(user.quota_used)}</p>
                    <p className={`text-[11px] ${
                      user.percent >= 100 ? 'text-red-600' :
                      user.percent >= 80 ? 'text-yellow-600' : 'text-green-600'
                    }`}>
                      {user.percent.toFixed(1)}%
                    </p>
                  </div>
                </div>
              ))}
              {!topUsersData?.length && (
                <p className="text-[12px] text-gray-500 text-center py-2">No data available</p>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
