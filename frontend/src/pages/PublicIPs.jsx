import { useState, useMemo, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { publicIPApi, subscriberApi } from '../services/api'
import { useAuthStore } from '../store/authStore'
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  flexRender,
} from '@tanstack/react-table'
import {
  PlusIcon,
  PencilIcon,
  TrashIcon,
  XMarkIcon,
  ArrowPathIcon,
} from '@heroicons/react/24/outline'
import toast from 'react-hot-toast'
import clsx from 'clsx'

export default function PublicIPs() {
  const queryClient = useQueryClient()
  const { hasPermission } = useAuthStore()
  const canManage = hasPermission('public_ips.manage')

  const [activeTab, setActiveTab] = useState('pools')
  const [showPoolModal, setShowPoolModal] = useState(false)
  const [editingPool, setEditingPool] = useState(null)
  const [showAssignModal, setShowAssignModal] = useState(false)
  const [showReserveModal, setShowReserveModal] = useState(false)

  // Pool form
  const [poolForm, setPoolForm] = useState({
    name: '', cidr: '', gateway: '', monthly_price: 0, description: '', is_active: true,
  })

  // Assign form
  const [assignForm, setAssignForm] = useState({ pool_id: '', subscriber_id: '', ip_address: '', notes: '' })
  // Reserve form
  const [reserveForm, setReserveForm] = useState({ pool_id: '', ip_address: '', notes: '' })
  const [subscriberSearch, setSubscriberSearch] = useState('')
  const [subscriberResults, setSubscriberResults] = useState([])

  // Assignment filters
  const [assignmentFilters, setAssignmentFilters] = useState({ pool_id: '', status: '', search: '' })
  const [assignmentPage, setAssignmentPage] = useState(1)

  // Sorting
  const [poolSorting, setPoolSorting] = useState([])
  const [assignmentSorting, setAssignmentSorting] = useState([])

  // Fetch pools
  const { data: poolsData, isLoading: poolsLoading } = useQuery({
    queryKey: ['public-ip-pools'],
    queryFn: () => publicIPApi.listPools().then(r => r.data),
  })
  const pools = poolsData?.data || []

  // Fetch assignments
  const { data: assignmentsData, isLoading: assignmentsLoading } = useQuery({
    queryKey: ['public-ip-assignments', assignmentFilters, assignmentPage],
    queryFn: () => publicIPApi.listAssignments({
      ...assignmentFilters,
      page: assignmentPage,
      limit: 25,
    }).then(r => r.data),
  })
  const rawAssignments = assignmentsData?.data || []
  const assignmentsTotal = assignmentsData?.total || 0

  // Group bandwidth customer IPs into single consolidated rows
  const assignments = useMemo(() => {
    const bwGroups = {}
    const result = []

    for (const a of rawAssignments) {
      if (a.bandwidth_customer_id && a.bandwidth_customer) {
        if (!bwGroups[a.bandwidth_customer_id]) {
          bwGroups[a.bandwidth_customer_id] = {
            ...a,
            _ips: [a.ip_address],
            _ids: [a.id],
            _isBWGroup: true,
          }
        } else {
          bwGroups[a.bandwidth_customer_id]._ips.push(a.ip_address)
          bwGroups[a.bandwidth_customer_id]._ids.push(a.id)
        }
      } else {
        result.push(a)
      }
    }

    for (const group of Object.values(bwGroups)) {
      group._ips.sort((a, b) => {
        const na = a.split('.').map(Number)
        const nb = b.split('.').map(Number)
        for (let i = 0; i < 4; i++) { if (na[i] !== nb[i]) return na[i] - nb[i] }
        return 0
      })
      result.unshift(group) // BW customer rows first
    }

    return result
  }, [rawAssignments])

  // Pool mutations
  const savePoolMutation = useMutation({
    mutationFn: (data) => editingPool
      ? publicIPApi.updatePool(editingPool.id, data)
      : publicIPApi.createPool(data),
    onSuccess: (res) => {
      toast.success(res.data.message || (editingPool ? 'Pool updated' : 'Pool created'))
      queryClient.invalidateQueries(['public-ip-pools'])
      closePoolModal()
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to save pool'),
  })

  const deletePoolMutation = useMutation({
    mutationFn: (id) => publicIPApi.deletePool(id),
    onSuccess: () => {
      toast.success('Pool deleted')
      queryClient.invalidateQueries(['public-ip-pools'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to delete pool'),
  })

  // Assignment mutations
  const assignMutation = useMutation({
    mutationFn: (data) => publicIPApi.assignIP(data),
    onSuccess: (res) => {
      toast.success(res.data.message || 'IP assigned')
      queryClient.invalidateQueries(['public-ip-assignments'])
      queryClient.invalidateQueries(['public-ip-pools'])
      setShowAssignModal(false)
      setAssignForm({ pool_id: '', subscriber_id: '', ip_address: '', notes: '' })
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to assign IP'),
  })

  const releaseMutation = useMutation({
    mutationFn: (id) => publicIPApi.releaseIP(id),
    onSuccess: (res) => {
      toast.success(res.data.message || 'IP released')
      queryClient.invalidateQueries(['public-ip-assignments'])
      queryClient.invalidateQueries(['public-ip-pools'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to release IP'),
  })

  const reserveMutation = useMutation({
    mutationFn: (data) => publicIPApi.reserveIP(data),
    onSuccess: (res) => {
      toast.success(res.data.message || 'IP reserved')
      queryClient.invalidateQueries(['public-ip-assignments'])
      queryClient.invalidateQueries(['public-ip-pools'])
      setShowReserveModal(false)
      setReserveForm({ pool_id: '', ip_address: '', notes: '' })
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to reserve IP'),
  })

  // Subscriber search for assignment
  useEffect(() => {
    if (subscriberSearch.length < 2) {
      setSubscriberResults([])
      return
    }
    const timer = setTimeout(async () => {
      try {
        const res = await subscriberApi.list({ search: subscriberSearch, limit: 10 })
        setSubscriberResults(res.data.data || [])
      } catch {}
    }, 300)
    return () => clearTimeout(timer)
  }, [subscriberSearch])

  // Pool modal helpers
  const openPoolModal = (pool = null) => {
    if (pool) {
      setEditingPool(pool)
      setPoolForm({
        name: pool.name,
        cidr: pool.cidr,
        gateway: pool.gateway || '',
        monthly_price: pool.monthly_price || 0,
        description: pool.description || '',
        is_active: pool.is_active,
      })
    } else {
      setEditingPool(null)
      setPoolForm({ name: '', cidr: '', gateway: '', monthly_price: 0, description: '', is_active: true })
    }
    setShowPoolModal(true)
  }

  const closePoolModal = () => {
    setShowPoolModal(false)
    setEditingPool(null)
  }

  // Pool columns
  const poolColumns = useMemo(() => [
    { accessorKey: 'name', header: 'Name' },
    {
      accessorKey: 'cidr',
      header: 'CIDR',
      cell: ({ row }) => <span className="font-mono text-sm">{row.original.cidr}</span>,
    },
    {
      accessorKey: 'ip_version',
      header: 'Version',
      cell: ({ row }) => (
        <span className={clsx('px-2 py-0.5 rounded text-xs font-semibold',
          row.original.ip_version === 4 ? 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200' : 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200'
        )}>
          IPv{row.original.ip_version}
        </span>
      ),
    },
    {
      accessorKey: 'gateway',
      header: 'Gateway',
      cell: ({ row }) => <span className="font-mono text-sm">{row.original.gateway || '-'}</span>,
    },
    {
      accessorKey: 'monthly_price',
      header: 'Price/mo',
      cell: ({ row }) => row.original.monthly_price > 0
        ? <span className="text-green-600 dark:text-green-400 font-semibold">${row.original.monthly_price.toFixed(2)}</span>
        : <span className="text-gray-400">Free</span>,
    },
    {
      id: 'usage',
      header: 'Usage',
      cell: ({ row }) => {
        const { used_ips, total_ips } = row.original
        const pct = total_ips > 0 ? Math.round((used_ips / total_ips) * 100) : 0
        return (
          <div className="flex items-center gap-2">
            <div className="w-20 bg-gray-200 dark:bg-gray-700 rounded-full h-2">
              <div className={clsx('h-2 rounded-full', pct > 80 ? 'bg-red-500' : pct > 50 ? 'bg-yellow-500' : 'bg-green-500')}
                style={{ width: `${Math.min(pct, 100)}%` }} />
            </div>
            <span className="text-xs text-gray-500 dark:text-gray-400">{used_ips}/{total_ips}</span>
          </div>
        )
      },
    },
    {
      accessorKey: 'is_active',
      header: 'Status',
      cell: ({ row }) => (
        <span className={clsx('px-2 py-0.5 rounded text-xs font-semibold',
          row.original.is_active ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' : 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400'
        )}>
          {row.original.is_active ? 'Active' : 'Inactive'}
        </span>
      ),
    },
    ...(canManage ? [{
      id: 'actions',
      header: 'Actions',
      cell: ({ row }) => (
        <div className="flex gap-2">
          <button onClick={() => openPoolModal(row.original)} className="text-blue-600 hover:text-blue-800 dark:text-blue-400" title="Edit">
            <PencilIcon className="w-4 h-4" />
          </button>
          <button onClick={() => { if (confirm(`Delete pool "${row.original.name}"?`)) deletePoolMutation.mutate(row.original.id) }}
            className="text-red-600 hover:text-red-800 dark:text-red-400" title="Delete">
            <TrashIcon className="w-4 h-4" />
          </button>
        </div>
      ),
    }] : []),
  ], [canManage])

  // Assignment columns
  const assignmentColumns = useMemo(() => [
    {
      accessorKey: 'ip_address',
      header: 'IP Address',
      cell: ({ row }) => {
        const o = row.original
        if (o._isBWGroup) {
          // Show compact IP list: .2, .3, .4, .5, .6
          const parts = o._ips.map(ip => ip.split('.'))
          const prefix = parts[0].slice(0, 3).join('.')
          const shortList = parts.map(p => '.' + p[3]).join(', ')
          return (
            <div>
              <span className="font-mono font-semibold text-sm">{prefix}</span>
              <span className="font-mono text-sm text-gray-600 dark:text-gray-300">{shortList}</span>
              <span className="ml-1.5 px-1.5 py-0.5 rounded bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300 text-xs font-bold">{o._ips.length} IPs</span>
            </div>
          )
        }
        return <span className="font-mono font-semibold">{o.ip_address}</span>
      },
    },
    {
      id: 'subscriber',
      header: 'Customer',
      cell: ({ row }) => {
        const o = row.original
        if (o.bandwidth_customer) {
          return <span className="text-purple-600 dark:text-purple-400 font-medium">{o.bandwidth_customer.name}</span>
        }
        if (o.subscriber) {
          return <span className="text-blue-600 dark:text-blue-400">{o.subscriber.username}</span>
        }
        return <span className="text-gray-400">-</span>
      },
    },
    {
      id: 'pool',
      header: 'Pool',
      cell: ({ row }) => row.original.pool?.name || '-',
    },
    {
      accessorKey: 'ip_version',
      header: 'Version',
      cell: ({ row }) => <span className="text-xs">IPv{row.original.ip_version}</span>,
    },
    {
      accessorKey: 'status',
      header: 'Status',
      cell: ({ row }) => {
        const s = row.original.status
        return (
          <span className={clsx('px-2 py-0.5 rounded text-xs font-semibold', {
            'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200': s === 'active',
            'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400': s === 'released',
            'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200': s === 'suspended',
            'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200': s === 'reserved',
          })}>
            {s}
          </span>
        )
      },
    },
    {
      accessorKey: 'monthly_price',
      header: 'Price/mo',
      cell: ({ row }) => row.original.monthly_price > 0
        ? `$${row.original.monthly_price.toFixed(2)}`
        : 'Free',
    },
    {
      accessorKey: 'assigned_at',
      header: 'Assigned',
      cell: ({ row }) => row.original.assigned_at
        ? new Date(row.original.assigned_at).toLocaleDateString()
        : '-',
    },
    {
      accessorKey: 'next_billing_at',
      header: 'Next Billing',
      cell: ({ row }) => row.original.next_billing_at
        ? new Date(row.original.next_billing_at).toLocaleDateString()
        : '-',
    },
    ...(canManage ? [{
      id: 'actions',
      header: 'Actions',
      cell: ({ row }) => {
        const o = row.original
        if (o.status !== 'active' && o.status !== 'reserved') return null
        if (o._isBWGroup) {
          return (
            <button onClick={async () => {
              if (!confirm(`Release all ${o._ids.length} IPs for "${o.bandwidth_customer?.name}"?`)) return
              for (const aid of o._ids) { await releaseMutation.mutateAsync(aid).catch(() => {}) }
            }}
              className="text-red-600 hover:text-red-800 dark:text-red-400 text-xs font-semibold">
              Release All
            </button>
          )
        }
        return (
          <button onClick={() => { if (confirm(`Release IP ${o.ip_address}?`)) releaseMutation.mutate(o.id) }}
            className="text-red-600 hover:text-red-800 dark:text-red-400 text-xs font-semibold">
            Release
          </button>
        )
      },
    }] : []),
  ], [canManage])

  const poolTable = useReactTable({
    data: pools,
    columns: poolColumns,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    state: { sorting: poolSorting },
    onSortingChange: setPoolSorting,
  })

  const assignmentTable = useReactTable({
    data: assignments,
    columns: assignmentColumns,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    state: { sorting: assignmentSorting },
    onSortingChange: setAssignmentSorting,
  })

  const renderTable = (table, loading) => (
    <div className="overflow-x-auto">
      <table className="w-full border-collapse">
        <thead className="bg-gray-50 dark:bg-gray-800 border-b dark:border-gray-700">
          {table.getHeaderGroups().map(headerGroup => (
            <tr key={headerGroup.id}>
              {headerGroup.headers.map(header => (
                <th key={header.id}
                  onClick={header.column.getToggleSortingHandler()}
                  className="px-4 py-3 text-left text-xs font-semibold text-gray-600 dark:text-gray-300 uppercase tracking-wider cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-700">
                  <div className="flex items-center gap-1">
                    {flexRender(header.column.columnDef.header, header.getContext())}
                    {header.column.getIsSorted() && (
                      <span>{header.column.getIsSorted() === 'asc' ? ' \u2191' : ' \u2193'}</span>
                    )}
                  </div>
                </th>
              ))}
            </tr>
          ))}
        </thead>
        <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
          {loading ? (
            <tr><td colSpan={100} className="px-4 py-8 text-center text-gray-500">Loading...</td></tr>
          ) : table.getRowModel().rows.length === 0 ? (
            <tr><td colSpan={100} className="px-4 py-8 text-center text-gray-500">No data</td></tr>
          ) : table.getRowModel().rows.map(row => (
            <tr key={row.id} className="hover:bg-gray-50 dark:hover:bg-gray-800">
              {row.getVisibleCells().map(cell => (
                <td key={cell.id} className="px-4 py-3 text-sm text-gray-900 dark:text-gray-100">
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )

  return (
    <div className="p-4 sm:p-6">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Public IP Management</h1>
        <div className="flex gap-2">
          <button onClick={() => { queryClient.invalidateQueries(['public-ip-pools']); queryClient.invalidateQueries(['public-ip-assignments']) }}
            className="btn btn-secondary flex items-center gap-1">
            <ArrowPathIcon className="w-4 h-4" /> Refresh
          </button>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b border-gray-300 dark:border-gray-600 mb-4">
        {['pools', 'assignments'].map(tab => (
          <button key={tab} onClick={() => setActiveTab(tab)}
            className={clsx('px-4 py-2 font-semibold text-sm rounded-t-lg transition-colors',
              activeTab === tab
                ? 'border-b-2 border-blue-600 text-blue-600 dark:text-blue-400 dark:border-blue-400'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200'
            )}>
            {tab === 'pools' ? 'IP Pools' : 'Assignments'}
          </button>
        ))}
      </div>

      {/* Pools Tab */}
      {activeTab === 'pools' && (
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow">
          <div className="flex items-center justify-between p-4 border-b dark:border-gray-700">
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white">IP Pools</h2>
            {canManage && (
              <button onClick={() => openPoolModal()} className="btn btn-primary flex items-center gap-1">
                <PlusIcon className="w-4 h-4" /> Add Pool
              </button>
            )}
          </div>
          {renderTable(poolTable, poolsLoading)}
        </div>
      )}

      {/* Assignments Tab */}
      {activeTab === 'assignments' && (
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow">
          <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between p-4 border-b dark:border-gray-700 gap-3">
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white">IP Assignments</h2>
            <div className="flex flex-wrap items-center gap-2">
              <select value={assignmentFilters.pool_id}
                onChange={e => { setAssignmentFilters(f => ({ ...f, pool_id: e.target.value })); setAssignmentPage(1) }}
                className="input text-sm">
                <option value="">All Pools</option>
                {pools.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
              </select>
              <select value={assignmentFilters.status}
                onChange={e => { setAssignmentFilters(f => ({ ...f, status: e.target.value })); setAssignmentPage(1) }}
                className="input text-sm">
                <option value="">All Status</option>
                <option value="active">Active</option>
                <option value="released">Released</option>
                <option value="suspended">Suspended</option>
                <option value="reserved">Reserved</option>
              </select>
              <input type="text" placeholder="Search IP..." value={assignmentFilters.search}
                onChange={e => { setAssignmentFilters(f => ({ ...f, search: e.target.value })); setAssignmentPage(1) }}
                className="input text-sm w-40" />
              {canManage && (
                <>
                  <button onClick={() => setShowReserveModal(true)} className="btn btn-secondary flex items-center gap-1 text-sm">
                    Reserve IP
                  </button>
                  <button onClick={() => setShowAssignModal(true)} className="btn btn-primary flex items-center gap-1 text-sm">
                    <PlusIcon className="w-4 h-4" /> Assign IP
                  </button>
                </>
              )}
            </div>
          </div>
          {renderTable(assignmentTable, assignmentsLoading)}
          {/* Pagination */}
          {assignmentsTotal > 25 && (
            <div className="flex items-center justify-between p-4 border-t dark:border-gray-700">
              <span className="text-sm text-gray-500 dark:text-gray-400">
                Showing {((assignmentPage - 1) * 25) + 1} to {Math.min(assignmentPage * 25, assignmentsTotal)} of {assignmentsTotal}
              </span>
              <div className="flex gap-2">
                <button disabled={assignmentPage <= 1} onClick={() => setAssignmentPage(p => p - 1)}
                  className="btn btn-secondary text-sm">Previous</button>
                <button disabled={assignmentPage * 25 >= assignmentsTotal} onClick={() => setAssignmentPage(p => p + 1)}
                  className="btn btn-secondary text-sm">Next</button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Pool Modal */}
      {showPoolModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-gray-800 rounded-lg p-6 w-full max-w-lg shadow-xl">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-bold text-gray-900 dark:text-white">
                {editingPool ? 'Edit Pool' : 'Add IP Pool'}
              </h2>
              <button onClick={closePoolModal} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                <XMarkIcon className="w-5 h-5" />
              </button>
            </div>
            <form onSubmit={e => { e.preventDefault(); savePoolMutation.mutate(poolForm) }}>
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">Name *</label>
                  <input type="text" value={poolForm.name}
                    onChange={e => setPoolForm(f => ({ ...f, name: e.target.value }))}
                    className="input w-full" required placeholder="e.g. Public IPv4 Pool" />
                </div>
                {!editingPool && (
                  <div>
                    <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">CIDR *</label>
                    <input type="text" value={poolForm.cidr}
                      onChange={e => setPoolForm(f => ({ ...f, cidr: e.target.value }))}
                      className="input w-full font-mono" required placeholder="e.g. 203.0.113.0/24" />
                    <p className="text-xs text-gray-500 mt-1">IPv4 (e.g. 203.0.113.0/24) or IPv6 (e.g. 2001:db8::/48)</p>
                  </div>
                )}
                <div>
                  <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">Gateway</label>
                  <input type="text" value={poolForm.gateway}
                    onChange={e => setPoolForm(f => ({ ...f, gateway: e.target.value }))}
                    className="input w-full font-mono" placeholder="e.g. 203.0.113.1" />
                </div>
                <div>
                  <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">Monthly Price ($)</label>
                  <input type="number" step="0.01" min="0" value={poolForm.monthly_price}
                    onChange={e => setPoolForm(f => ({ ...f, monthly_price: parseFloat(e.target.value) || 0 }))}
                    className="input w-full" />
                  <p className="text-xs text-gray-500 mt-1">Set to 0 for free public IPs</p>
                </div>
                <div>
                  <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">Description</label>
                  <textarea value={poolForm.description}
                    onChange={e => setPoolForm(f => ({ ...f, description: e.target.value }))}
                    className="input w-full" rows={2} />
                </div>
                <div className="flex items-center gap-2">
                  <input type="checkbox" id="pool-active" checked={poolForm.is_active}
                    onChange={e => setPoolForm(f => ({ ...f, is_active: e.target.checked }))}
                    className="w-4 h-4" />
                  <label htmlFor="pool-active" className="text-sm text-gray-700 dark:text-gray-300">Active</label>
                </div>
              </div>
              <div className="flex gap-2 mt-6">
                <button type="submit" disabled={savePoolMutation.isPending}
                  className="flex-1 btn btn-primary">
                  {savePoolMutation.isPending ? 'Saving...' : 'Save'}
                </button>
                <button type="button" onClick={closePoolModal} className="flex-1 btn btn-secondary">Cancel</button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Reserve IP Modal */}
      {showReserveModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-gray-800 rounded-lg p-6 w-full max-w-lg shadow-xl">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-bold text-gray-900 dark:text-white">Reserve IP (Exclude from Allocation)</h2>
              <button onClick={() => setShowReserveModal(false)} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                <XMarkIcon className="w-5 h-5" />
              </button>
            </div>
            <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
              Reserve an IP to prevent it from being assigned. Use this for IPs that are already in use externally.
            </p>
            <form onSubmit={e => {
              e.preventDefault()
              reserveMutation.mutate({
                pool_id: parseInt(reserveForm.pool_id),
                ip_address: reserveForm.ip_address,
                notes: reserveForm.notes,
              })
            }}>
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">Pool *</label>
                  <select value={reserveForm.pool_id}
                    onChange={e => setReserveForm(f => ({ ...f, pool_id: e.target.value }))}
                    className="input w-full" required>
                    <option value="">-- Select Pool --</option>
                    {pools.filter(p => p.is_active).map(p => (
                      <option key={p.id} value={p.id}>{p.name} ({p.cidr})</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">IP Address *</label>
                  <input type="text" value={reserveForm.ip_address}
                    onChange={e => setReserveForm(f => ({ ...f, ip_address: e.target.value }))}
                    className="input w-full font-mono" required placeholder="e.g. 203.0.113.10" />
                </div>
                <div>
                  <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">Notes</label>
                  <input type="text" value={reserveForm.notes}
                    onChange={e => setReserveForm(f => ({ ...f, notes: e.target.value }))}
                    className="input w-full" placeholder="e.g. Used on external router" />
                </div>
              </div>
              <div className="flex gap-2 mt-6">
                <button type="submit" disabled={reserveMutation.isPending || !reserveForm.pool_id || !reserveForm.ip_address}
                  className="flex-1 btn btn-primary">
                  {reserveMutation.isPending ? 'Reserving...' : 'Reserve IP'}
                </button>
                <button type="button" onClick={() => setShowReserveModal(false)} className="flex-1 btn btn-secondary">Cancel</button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Assign IP Modal */}
      {showAssignModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-gray-800 rounded-lg p-6 w-full max-w-lg shadow-xl">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-bold text-gray-900 dark:text-white">Assign Public IP</h2>
              <button onClick={() => setShowAssignModal(false)} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                <XMarkIcon className="w-5 h-5" />
              </button>
            </div>
            <form onSubmit={e => {
              e.preventDefault()
              assignMutation.mutate({
                pool_id: parseInt(assignForm.pool_id),
                subscriber_id: parseInt(assignForm.subscriber_id),
                ip_address: assignForm.ip_address || undefined,
                notes: assignForm.notes,
              })
            }}>
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">Pool *</label>
                  <select value={assignForm.pool_id}
                    onChange={e => setAssignForm(f => ({ ...f, pool_id: e.target.value }))}
                    className="input w-full" required>
                    <option value="">-- Select Pool --</option>
                    {pools.filter(p => p.is_active).map(p => (
                      <option key={p.id} value={p.id}>
                        {p.name} ({p.cidr}) - {p.used_ips}/{p.total_ips} used {p.monthly_price > 0 ? `- $${p.monthly_price}/mo` : '- Free'}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">Subscriber *</label>
                  <input type="text" placeholder="Search subscriber..."
                    value={subscriberSearch}
                    onChange={e => setSubscriberSearch(e.target.value)}
                    className="input w-full mb-1" />
                  {subscriberResults.length > 0 && (
                    <div className="border dark:border-gray-600 rounded max-h-40 overflow-y-auto">
                      {subscriberResults.map(s => (
                        <button key={s.id} type="button"
                          onClick={() => {
                            setAssignForm(f => ({ ...f, subscriber_id: String(s.id) }))
                            setSubscriberSearch(s.username + (s.full_name ? ` (${s.full_name})` : ''))
                            setSubscriberResults([])
                          }}
                          className="w-full text-left px-3 py-2 hover:bg-gray-100 dark:hover:bg-gray-700 text-sm">
                          {s.username} {s.full_name && <span className="text-gray-500">({s.full_name})</span>}
                        </button>
                      ))}
                    </div>
                  )}
                  {assignForm.subscriber_id && (
                    <p className="text-xs text-green-600 mt-1">Selected: ID {assignForm.subscriber_id}</p>
                  )}
                </div>
                <div>
                  <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">Specific IP (optional)</label>
                  <input type="text" value={assignForm.ip_address}
                    onChange={e => setAssignForm(f => ({ ...f, ip_address: e.target.value }))}
                    className="input w-full font-mono" placeholder="Leave empty for auto-assign" />
                </div>
                <div>
                  <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">Notes</label>
                  <input type="text" value={assignForm.notes}
                    onChange={e => setAssignForm(f => ({ ...f, notes: e.target.value }))}
                    className="input w-full" placeholder="Optional notes" />
                </div>
              </div>
              <div className="flex gap-2 mt-6">
                <button type="submit" disabled={assignMutation.isPending || !assignForm.pool_id || !assignForm.subscriber_id}
                  className="flex-1 btn btn-primary">
                  {assignMutation.isPending ? 'Assigning...' : 'Assign IP'}
                </button>
                <button type="button" onClick={() => setShowAssignModal(false)} className="flex-1 btn btn-secondary">Cancel</button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
