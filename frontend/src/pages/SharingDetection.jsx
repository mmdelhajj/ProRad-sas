import { useState, useMemo } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { sharingApi } from '../services/api'
import { useBrandingStore } from '../store/brandingStore'
import {
  useReactTable,
  getCoreRowModel,
  getFilteredRowModel,
  flexRender,
} from '@tanstack/react-table'
import {
  ExclamationTriangleIcon,
  ShieldExclamationIcon,
  ShieldCheckIcon,
  SignalIcon,
  ArrowPathIcon,
  ArrowTrendingUpIcon,
  ArrowTrendingDownIcon,
  ArrowRightIcon,
  MagnifyingGlassIcon,
  InformationCircleIcon,
  WifiIcon,
  ComputerDesktopIcon,
  CogIcon,
  CheckCircleIcon,
  XCircleIcon,
  ServerIcon,
  ClockIcon,
  ChartBarIcon,
  PlayIcon,
  CalendarDaysIcon,
  UserGroupIcon,
} from '@heroicons/react/24/outline'
import clsx from 'clsx'
import toast from 'react-hot-toast'

function getSuspicionBadge(level) {
  switch (level) {
    case 'high':
      return <span className="badge-danger">High Risk</span>
    case 'medium':
      return <span className="badge-warning">Medium</span>
    default:
      return <span className="badge-success">Normal</span>
  }
}

function getTTLBadge(status, ttlValues) {
  if (status === 'router_detected' || status === 'double_router') {
    return <span className="badge-danger">Router Detected</span>
  }
  if (status === 'multiple_os') {
    return <span className="badge-warning">Multiple OS</span>
  }
  if (ttlValues && ttlValues.length > 0) {
    return <span className="badge-gray">TTL: {ttlValues.join(', ')}</span>
  }
  return (
    <span className="text-[11px] text-gray-400">No TTL data</span>
  )
}

export default function SharingDetection() {
  const { companyName } = useBrandingStore()
  const brandName = companyName || 'ISP'
  const queryClient = useQueryClient()

  const [activeTab, setActiveTab] = useState('live')
  const [search, setSearch] = useState('')
  const [showOnlySuspicious, setShowOnlySuspicious] = useState(false)
  const [showConfig, setShowConfig] = useState(false)
  const [historyDays, setHistoryDays] = useState(7)
  const [historyLevel, setHistoryLevel] = useState('')
  const [scoreMonth, setScoreMonth] = useState(new Date().toISOString().slice(0, 7))
  const [scoreCategory, setScoreCategory] = useState('')
  const [whitelistModal, setWhitelistModal] = useState(null) // { id, username, whitelisted }
  const [whitelistReason, setWhitelistReason] = useState('business')

  // Live detection query
  const { data, isLoading, refetch, isFetching } = useQuery({
    queryKey: ['sharing-detection'],
    queryFn: () => sharingApi.list().then((r) => r.data),
    refetchInterval: 60000,
    enabled: activeTab === 'live',
  })

  // History query
  const { data: historyData, isLoading: isLoadingHistory, refetch: refetchHistory } = useQuery({
    queryKey: ['sharing-history', historyDays, historyLevel],
    queryFn: () => sharingApi.getHistory({ days: historyDays, suspicion_level: historyLevel }).then((r) => r.data),
    enabled: activeTab === 'history',
  })

  // Trends query
  const { data: trendsData, isLoading: isLoadingTrends } = useQuery({
    queryKey: ['sharing-trends', historyDays],
    queryFn: () => sharingApi.getTrends({ days: historyDays }).then((r) => r.data),
    enabled: activeTab === 'history',
  })

  // Repeat offenders query
  const { data: offendersData, isLoading: isLoadingOffenders } = useQuery({
    queryKey: ['sharing-offenders'],
    queryFn: () => sharingApi.getRepeatOffenders({ days: 30, min_count: 3 }).then((r) => r.data),
    enabled: activeTab === 'history',
  })

  // Settings query
  const { data: settingsData, isLoading: isLoadingSettings, refetch: refetchSettings } = useQuery({
    queryKey: ['sharing-settings'],
    queryFn: () => sharingApi.getSettings().then((r) => r.data),
    enabled: activeTab === 'settings',
  })

  // NAS rules query
  const { data: nasRulesData, refetch: refetchNasRules, isLoading: isLoadingRules } = useQuery({
    queryKey: ['sharing-nas-rules'],
    queryFn: () => sharingApi.getNasRuleStatus().then((r) => r.data),
    enabled: showConfig,
  })

  // Monthly scores query
  const { data: scoresData, isLoading: isLoadingScores } = useQuery({
    queryKey: ['sharing-scores', scoreMonth, scoreCategory],
    queryFn: () => sharingApi.getMonthlyScores({ month: scoreMonth, category: scoreCategory }).then((r) => r.data),
    enabled: activeTab === 'scores',
  })

  // Whitelisted subscribers query
  const { data: whitelistData, refetch: refetchWhitelist } = useQuery({
    queryKey: ['sharing-whitelist'],
    queryFn: () => sharingApi.getWhitelistedSubscribers().then((r) => r.data),
    enabled: activeTab === 'settings',
  })

  // Action logs query
  const { data: actionLogsData } = useQuery({
    queryKey: ['sharing-action-logs'],
    queryFn: () => sharingApi.getActionLogs({ days: 30 }).then((r) => r.data),
    enabled: activeTab === 'settings',
  })

  // Mutations
  const generateRulesMutation = useMutation({
    mutationFn: (nasId) => sharingApi.generateTTLRules(nasId),
    onSuccess: (res) => {
      toast.success(res.data?.message || 'TTL rules generated successfully')
      refetchNasRules()
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to generate rules'),
  })

  const removeRulesMutation = useMutation({
    mutationFn: (nasId) => sharingApi.removeTTLRules(nasId),
    onSuccess: (res) => {
      toast.success(res.data?.message || 'TTL rules removed successfully')
      refetchNasRules()
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to remove rules'),
  })

  const manualScanMutation = useMutation({
    mutationFn: () => sharingApi.runManualScan(),
    onSuccess: (res) => {
      toast.success(res.data?.message || `Scan completed. Found ${res.data?.saved || 0} suspicious accounts.`)
      refetchHistory()
      queryClient.invalidateQueries(['sharing-trends'])
      queryClient.invalidateQueries(['sharing-offenders'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Scan failed'),
  })

  const updateSettingsMutation = useMutation({
    mutationFn: (settings) => sharingApi.updateSettings(settings),
    onSuccess: () => {
      toast.success('Settings saved')
      refetchSettings()
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to save settings'),
  })

  const toggleWhitelistMutation = useMutation({
    mutationFn: ({ id, whitelisted, reason }) => sharingApi.toggleWhitelist(id, { whitelisted, reason }),
    onSuccess: () => {
      toast.success('Whitelist updated')
      setWhitelistModal(null)
      queryClient.invalidateQueries(['sharing-scores'])
      refetchWhitelist()
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to update whitelist'),
  })

  const accounts = data?.data || []
  const stats = data?.stats || {}
  const nasRules = nasRulesData?.data || []
  const history = historyData?.data || []
  const trends = trendsData?.data || []
  const offenders = offendersData?.data || []
  const settings = settingsData?.data || {}
  const scores = scoresData?.data || []
  const scoreSummary = scoresData?.summary || { good: 0, warning: 0, bad: 0 }
  const whitelistedSubs = whitelistData?.data || []

  const filteredAccounts = useMemo(() => {
    let result = accounts
    if (showOnlySuspicious) {
      result = result.filter(a => a.suspicion_level === 'high' || a.suspicion_level === 'medium')
    }
    if (search) {
      const s = search.toLowerCase()
      result = result.filter(
        (a) =>
          a.username?.toLowerCase().includes(s) ||
          a.full_name?.toLowerCase().includes(s) ||
          a.ip_address?.includes(s)
      )
    }
    return result
  }, [accounts, search, showOnlySuspicious])

  const columns = useMemo(
    () => [
      {
        accessorKey: 'username',
        header: 'Subscriber',
        cell: ({ row }) => (
          <div>
            <span className="font-medium text-gray-900">{row.original.username}</span>
            {row.original.full_name && (
              <div className="text-[11px] text-gray-500">{row.original.full_name}</div>
            )}
          </div>
        ),
      },
      {
        accessorKey: 'ip_address',
        header: 'IP Address',
        cell: ({ row }) => (
          <span className="font-mono text-[11px]">
            {row.original.ip_address || '-'}
          </span>
        ),
      },
      {
        accessorKey: 'connection_count',
        header: 'Connections',
        cell: ({ row }) => {
          const count = row.original.connection_count || 0
          return (
            <span className={clsx(
              'font-mono font-medium',
              count >= 400 ? 'text-[#f44336]' :
              count >= 200 ? 'text-[#FF9800]' :
              'text-gray-700'
            )}>
              {count}
            </span>
          )
        },
      },
      {
        accessorKey: 'ttl_status',
        header: 'TTL Status',
        cell: ({ row }) => getTTLBadge(row.original.ttl_status, row.original.ttl_values),
      },
      {
        accessorKey: 'suspicion_level',
        header: 'Risk Level',
        cell: ({ row }) => getSuspicionBadge(row.original.suspicion_level),
      },
      {
        accessorKey: 'reasons',
        header: 'Reasons',
        cell: ({ row }) => {
          const reasons = row.original.reasons || []
          if (reasons.length === 0) return <span className="text-gray-400">-</span>
          return (
            <div style={{ maxWidth: '180px' }}>
              {reasons.slice(0, 2).map((r, i) => (
                <div key={i} className="text-[11px] text-gray-600 truncate">{r}</div>
              ))}
              {reasons.length > 2 && (
                <span className="text-[11px] text-gray-400">+{reasons.length - 2} more</span>
              )}
            </div>
          )
        },
      },
      {
        accessorKey: 'nas_name',
        header: 'NAS',
        cell: ({ row }) => (
          <span className="text-[12px] text-gray-600">
            {row.original.nas_name || row.original.nas_ip_address || '-'}
          </span>
        ),
      },
    ],
    []
  )

  const table = useReactTable({
    data: filteredAccounts,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
  })

  const tabs = [
    { id: 'live', label: 'Live Analysis' },
    { id: 'scores', label: 'Customer Scores' },
    { id: 'history', label: 'History & Trends' },
    { id: 'settings', label: 'Settings' },
  ]

  return (
    <div className="space-y-3" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Header Toolbar */}
      <div className="wb-toolbar justify-between">
        <div className="flex items-center gap-2">
          <ShieldExclamationIcon className="w-4 h-4 text-[#316AC5]" />
          <span className="text-[13px] font-semibold">Sharing Detection</span>
        </div>
        <div className="flex gap-1">
          {activeTab === 'live' && (
            <>
              <button
                onClick={() => setShowConfig(!showConfig)}
                className="btn btn-sm"
              >
                <CogIcon className="w-3.5 h-3.5 mr-1" />
                TTL Rules
              </button>
              <button
                onClick={() => refetch()}
                disabled={isFetching}
                className="btn btn-sm"
              >
                <ArrowPathIcon className={clsx('w-3.5 h-3.5 mr-1', isFetching && 'animate-spin')} />
                {isFetching ? 'Analyzing...' : 'Refresh'}
              </button>
            </>
          )}
          {activeTab === 'history' && (
            <button
              onClick={() => manualScanMutation.mutate()}
              disabled={manualScanMutation.isPending}
              className="btn btn-primary btn-sm"
            >
              <PlayIcon className={clsx('w-3.5 h-3.5 mr-1', manualScanMutation.isPending && 'animate-pulse')} />
              {manualScanMutation.isPending ? 'Scanning...' : 'Run Manual Scan'}
            </button>
          )}
        </div>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-[#a0a0a0]">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`wb-tab ${activeTab === tab.id ? 'active' : ''}`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Live Analysis Tab */}
      {activeTab === 'live' && (
        <>
          {/* TTL Rules Configuration Panel */}
          {showConfig && (
            <div className="wb-group">
              <div className="wb-group-title flex items-center justify-between">
                <div className="flex items-center gap-1.5">
                  <CogIcon className="w-3.5 h-3.5 text-[#316AC5]" />
                  TTL Detection Rules Configuration
                </div>
                <button onClick={() => setShowConfig(false)} className="text-gray-400 hover:text-gray-600">
                  <XCircleIcon className="w-4 h-4" />
                </button>
              </div>
              <div className="wb-group-body">
                <p className="text-[11px] text-gray-500 mb-2">
                  Configure MikroTik mangle rules for TTL-based sharing detection.
                </p>

                <div className="text-[12px] font-medium text-gray-700 mb-2">NAS Devices - TTL Rule Status:</div>
                {isLoadingRules ? (
                  <div className="flex items-center justify-center py-3">
                    <ArrowPathIcon className="h-5 w-5 animate-spin text-[#316AC5]" />
                  </div>
                ) : nasRules.length === 0 ? (
                  <p className="text-[12px] text-gray-500 text-center py-2">No active NAS devices found</p>
                ) : (
                  <div className="space-y-1">
                    {nasRules.map((nas) => (
                      <div key={nas.nas_id} className="flex items-center justify-between p-2 border border-[#ccc] bg-[#f7f7f7]" style={{ borderRadius: '2px' }}>
                        <div className="flex items-center gap-2">
                          <ServerIcon className="w-3.5 h-3.5 text-gray-400" />
                          <div>
                            <div className="text-[12px] font-medium text-gray-900">{nas.nas_name}</div>
                            <div className="text-[11px] text-gray-500">{nas.nas_ip_address}</div>
                          </div>
                        </div>
                        <div className="flex items-center gap-2">
                          {nas.error ? (
                            <span className="text-[11px] text-[#f44336] flex items-center gap-0.5">
                              <XCircleIcon className="w-3 h-3" />
                              Error
                            </span>
                          ) : nas.rules_configured ? (
                            <span className="text-[11px] text-[#4CAF50] flex items-center gap-0.5">
                              <CheckCircleIcon className="w-3 h-3" />
                              {nas.rule_count} rules
                            </span>
                          ) : (
                            <span className="text-[11px] text-[#FF9800] flex items-center gap-0.5">
                              <ExclamationTriangleIcon className="w-3 h-3" />
                              Not configured
                            </span>
                          )}
                          <div className="flex gap-1">
                            {nas.rules_configured ? (
                              <button
                                onClick={() => removeRulesMutation.mutate(nas.nas_id)}
                                disabled={removeRulesMutation.isPending}
                                className="btn btn-danger btn-xs"
                              >
                                Remove
                              </button>
                            ) : (
                              <button
                                onClick={() => generateRulesMutation.mutate(nas.nas_id)}
                                disabled={generateRulesMutation.isPending}
                                className="btn btn-primary btn-xs"
                              >
                                Generate
                              </button>
                            )}
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          )}

          {/* Stats Cards */}
          <div className="grid grid-cols-2 md:grid-cols-5 gap-2">
            <div className="stat-card">
              <div className="flex items-center gap-2">
                <div className="wb-status-dot bg-[#2196F3]"></div>
                <div>
                  <div className="text-[11px] text-gray-500">Online Users</div>
                  <div className="text-[16px] font-bold text-gray-900">{stats.total_online || 0}</div>
                </div>
              </div>
            </div>
            <div className="stat-card">
              <div className="flex items-center gap-2">
                <div className="wb-status-dot bg-[#FF9800]"></div>
                <div>
                  <div className="text-[11px] text-gray-500">Suspicious</div>
                  <div className="text-[16px] font-bold text-[#FF9800]">{stats.suspicious_count || 0}</div>
                </div>
              </div>
            </div>
            <div className="stat-card">
              <div className="flex items-center gap-2">
                <div className="wb-status-dot bg-[#f44336]"></div>
                <div>
                  <div className="text-[11px] text-gray-500">High Risk</div>
                  <div className="text-[16px] font-bold text-[#f44336]">{stats.high_risk_count || 0}</div>
                </div>
              </div>
            </div>
            <div className="stat-card">
              <div className="flex items-center gap-2">
                <div className="wb-status-dot bg-[#9C27B0]"></div>
                <div>
                  <div className="text-[11px] text-gray-500">Router Detected</div>
                  <div className="text-[16px] font-bold text-[#9C27B0]">{stats.router_detected || 0}</div>
                </div>
              </div>
            </div>
            <div className="stat-card">
              <div className="flex items-center gap-2">
                <div className="wb-status-dot bg-[#FF9800]"></div>
                <div>
                  <div className="text-[11px] text-gray-500">High Connections</div>
                  <div className="text-[16px] font-bold text-[#e65100]">{stats.high_connections || 0}</div>
                </div>
              </div>
            </div>
          </div>

          {/* Filters */}
          <div className="wb-group">
            <div className="wb-group-body">
              <div className="flex flex-col sm:flex-row gap-2 items-start sm:items-center">
                <div className="relative flex-1 max-w-xs">
                  <MagnifyingGlassIcon className="absolute left-2 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-gray-400" />
                  <input
                    type="text"
                    placeholder="Search by username, name, IP..."
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    className="input pl-7"
                  />
                </div>
                <label className="flex items-center gap-1.5 cursor-pointer text-[12px] text-gray-700">
                  <input
                    type="checkbox"
                    checked={showOnlySuspicious}
                    onChange={(e) => setShowOnlySuspicious(e.target.checked)}
                    className="w-3.5 h-3.5"
                  />
                  Show only suspicious
                </label>
              </div>
            </div>
          </div>

          {/* Table */}
          <div className="table-container">
            <table className="table">
              <thead>
                {table.getHeaderGroups().map((headerGroup) => (
                  <tr key={headerGroup.id}>
                    {headerGroup.headers.map((header) => (
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
                      <ArrowPathIcon className="h-5 w-5 animate-spin mx-auto text-[#316AC5] mb-1" />
                      <span className="text-[12px] text-gray-500">Analyzing connections...</span>
                    </td>
                  </tr>
                ) : table.getRowModel().rows.length === 0 ? (
                  <tr>
                    <td colSpan={columns.length} className="text-center py-3 text-[12px] text-gray-500">
                      {showOnlySuspicious ? 'No suspicious accounts found' : 'No online users'}
                    </td>
                  </tr>
                ) : (
                  table.getRowModel().rows.map((row) => (
                    <tr
                      key={row.id}
                      className={clsx(
                        row.original.suspicion_level === 'high' && '!bg-[#fde8e8]',
                        row.original.suspicion_level === 'medium' && '!bg-[#fff8e1]'
                      )}
                    >
                      {row.getVisibleCells().map((cell) => (
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
        </>
      )}

      {/* Customer Scores Tab */}
      {activeTab === 'scores' && (
        <>
          {/* Controls */}
          <div className="wb-group">
            <div className="wb-group-body">
              <div className="flex flex-wrap gap-2 items-center">
                <div className="flex items-center gap-1.5">
                  <label className="label mb-0">Month:</label>
                  <input
                    type="month"
                    value={scoreMonth}
                    onChange={(e) => setScoreMonth(e.target.value)}
                    className="input"
                    style={{ width: '160px' }}
                  />
                </div>
                <div className="flex items-center gap-1.5">
                  <label className="label mb-0">Category:</label>
                  <select
                    value={scoreCategory}
                    onChange={(e) => setScoreCategory(e.target.value)}
                    className="input"
                    style={{ width: '130px' }}
                  >
                    <option value="">All</option>
                    <option value="good">Good</option>
                    <option value="warning">Warning</option>
                    <option value="bad">Bad</option>
                  </select>
                </div>
              </div>
            </div>
          </div>

          {/* Summary Cards */}
          <div className="grid grid-cols-3 gap-2">
            <div className="wb-group">
              <div className="wb-group-body text-center">
                <div className="text-[20px] font-bold text-green-600">{scoreSummary.good || 0}</div>
                <div className="text-[11px] text-gray-500">Good</div>
              </div>
            </div>
            <div className="wb-group">
              <div className="wb-group-body text-center">
                <div className="text-[20px] font-bold text-orange-500">{scoreSummary.warning || 0}</div>
                <div className="text-[11px] text-gray-500">Warning</div>
              </div>
            </div>
            <div className="wb-group">
              <div className="wb-group-body text-center">
                <div className="text-[20px] font-bold text-red-600">{scoreSummary.bad || 0}</div>
                <div className="text-[11px] text-gray-500">Bad</div>
              </div>
            </div>
          </div>

          {/* Scores Table */}
          <div className="wb-group">
            <div className="wb-group-title flex items-center gap-1.5">
              <ChartBarIcon className="w-3.5 h-3.5 text-[#316AC5]" />
              Customer Scores — {scoreMonth}
            </div>
            <div className="overflow-x-auto">
              <table className="wb-table">
                <thead>
                  <tr>
                    <th>Subscriber</th>
                    <th>Service</th>
                    <th>Score</th>
                    <th>Category</th>
                    <th>Detections</th>
                    <th>Avg Confidence</th>
                    <th>Trend</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {isLoadingScores ? (
                    <tr>
                      <td colSpan={8} className="text-center py-3">
                        <ArrowPathIcon className="h-5 w-5 animate-spin mx-auto text-[#316AC5]" />
                      </td>
                    </tr>
                  ) : scores.length === 0 ? (
                    <tr>
                      <td colSpan={8} className="text-center py-3 text-[12px] text-gray-500">
                        No scores for this month
                      </td>
                    </tr>
                  ) : (
                    scores.map((sc) => (
                      <tr key={sc.id}>
                        <td>
                          <span className="font-medium">{sc.username}</span>
                          {sc.full_name && <div className="text-[10px] text-gray-500">{sc.full_name}</div>}
                        </td>
                        <td className="text-[11px]">{sc.service_name || '-'}</td>
                        <td>
                          <div className="flex items-center gap-1.5">
                            <div className="w-16 h-2 bg-gray-200 rounded-full overflow-hidden">
                              <div
                                className={clsx(
                                  'h-full rounded-full',
                                  sc.score <= 30 ? 'bg-green-500' : sc.score <= 60 ? 'bg-orange-500' : 'bg-red-500'
                                )}
                                style={{ width: `${sc.score}%` }}
                              />
                            </div>
                            <span className="text-[11px] font-medium">{sc.score}</span>
                          </div>
                        </td>
                        <td>
                          <span className={clsx(
                            'text-[10px] px-1.5 py-0.5 rounded font-medium',
                            sc.category === 'good' ? 'bg-green-100 text-green-700' :
                            sc.category === 'warning' ? 'bg-orange-100 text-orange-700' :
                            'bg-red-100 text-red-700'
                          )}>
                            {sc.category === 'good' ? 'Good' : sc.category === 'warning' ? 'Warning' : 'Bad'}
                          </span>
                        </td>
                        <td className="text-[11px]">{sc.detection_count}</td>
                        <td className="text-[11px]">{Math.round(sc.avg_confidence)}%</td>
                        <td>
                          {sc.trend === 'worsening' && (
                            <span className="flex items-center gap-0.5 text-red-600 text-[11px]">
                              <ArrowTrendingUpIcon className="w-3.5 h-3.5" /> Worsening
                            </span>
                          )}
                          {sc.trend === 'stable' && (
                            <span className="flex items-center gap-0.5 text-gray-500 text-[11px]">
                              <ArrowRightIcon className="w-3.5 h-3.5" /> Stable
                            </span>
                          )}
                          {sc.trend === 'improving' && (
                            <span className="flex items-center gap-0.5 text-green-600 text-[11px]">
                              <ArrowTrendingDownIcon className="w-3.5 h-3.5" /> Improving
                            </span>
                          )}
                        </td>
                        <td>
                          <button
                            onClick={() => setWhitelistModal({ id: sc.subscriber_id, username: sc.username, whitelisted: false })}
                            className="btn btn-sm"
                            title="Toggle Whitelist"
                          >
                            <ShieldCheckIcon className="w-3.5 h-3.5" />
                          </button>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>

          {/* Whitelist Modal */}
          {whitelistModal && (
            <div className="fixed inset-0 bg-black/40 z-50 flex items-center justify-center">
              <div className="bg-white rounded-lg shadow-xl p-4 w-80">
                <h3 className="text-[13px] font-semibold mb-2">
                  <ShieldCheckIcon className="w-4 h-4 inline mr-1 text-[#316AC5]" />
                  Whitelist Subscriber
                </h3>
                <p className="text-[11px] text-gray-600 mb-2">
                  Whitelisted subscribers will be skipped during sharing detection scans.
                </p>
                <div className="text-[12px] font-medium mb-2">{whitelistModal.username}</div>
                <div className="mb-3">
                  <label className="label">Reason</label>
                  <select
                    value={whitelistReason}
                    onChange={(e) => setWhitelistReason(e.target.value)}
                    className="input w-full"
                  >
                    <option value="business">Business</option>
                    <option value="family">Family Plan</option>
                    <option value="server">Server</option>
                    <option value="other">Other</option>
                  </select>
                </div>
                <div className="flex gap-2 justify-end">
                  <button onClick={() => setWhitelistModal(null)} className="btn btn-sm">Cancel</button>
                  <button
                    onClick={() => toggleWhitelistMutation.mutate({ id: whitelistModal.id, whitelisted: true, reason: whitelistReason })}
                    disabled={toggleWhitelistMutation.isPending}
                    className="btn btn-primary btn-sm"
                  >
                    {toggleWhitelistMutation.isPending ? 'Saving...' : 'Whitelist'}
                  </button>
                </div>
              </div>
            </div>
          )}
        </>
      )}

      {/* History Tab */}
      {activeTab === 'history' && (
        <>
          {/* Filters */}
          <div className="wb-group">
            <div className="wb-group-body">
              <div className="flex flex-wrap gap-2 items-center">
                <div className="flex items-center gap-1.5">
                  <label className="label mb-0">Period:</label>
                  <select
                    value={historyDays}
                    onChange={(e) => setHistoryDays(Number(e.target.value))}
                    className="input" style={{ width: '130px' }}
                  >
                    <option value={7}>Last 7 days</option>
                    <option value={14}>Last 14 days</option>
                    <option value={30}>Last 30 days</option>
                  </select>
                </div>
                <div className="flex items-center gap-1.5">
                  <label className="label mb-0">Risk:</label>
                  <select
                    value={historyLevel}
                    onChange={(e) => setHistoryLevel(e.target.value)}
                    className="input" style={{ width: '120px' }}
                  >
                    <option value="">All Levels</option>
                    <option value="high">High Risk</option>
                    <option value="medium">Medium</option>
                  </select>
                </div>
                <button
                  onClick={() => refetchHistory()}
                  className="btn btn-sm"
                >
                  <ArrowPathIcon className="w-3.5 h-3.5 mr-1" />
                  Refresh
                </button>
              </div>
            </div>
          </div>

          {/* Trends Summary */}
          {!isLoadingTrends && trends.length > 0 && (
            <div className="wb-group">
              <div className="wb-group-title flex items-center gap-1.5">
                <ChartBarIcon className="w-3.5 h-3.5 text-[#316AC5]" />
                Detection Trends (Last {historyDays} Days)
              </div>
              <div className="wb-group-body">
                <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
                  <div className="stat-card">
                    <div className="text-[11px] text-gray-500">Total Detections</div>
                    <div className="text-[16px] font-bold text-gray-900">
                      {trends.reduce((sum, t) => sum + t.total_detected, 0)}
                    </div>
                  </div>
                  <div className="stat-card" style={{ borderLeft: '3px solid #f44336' }}>
                    <div className="text-[11px] text-[#f44336]">High Risk</div>
                    <div className="text-[16px] font-bold text-[#f44336]">
                      {trends.reduce((sum, t) => sum + t.high_risk_count, 0)}
                    </div>
                  </div>
                  <div className="stat-card" style={{ borderLeft: '3px solid #FF9800' }}>
                    <div className="text-[11px] text-[#FF9800]">Medium Risk</div>
                    <div className="text-[16px] font-bold text-[#FF9800]">
                      {trends.reduce((sum, t) => sum + t.medium_risk_count, 0)}
                    </div>
                  </div>
                  <div className="stat-card" style={{ borderLeft: '3px solid #2196F3' }}>
                    <div className="text-[11px] text-[#2196F3]">Avg Confidence</div>
                    <div className="text-[16px] font-bold text-[#2196F3]">
                      {Math.round(trends.reduce((sum, t) => sum + t.avg_confidence, 0) / (trends.length || 1))}%
                    </div>
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* Repeat Offenders */}
          {!isLoadingOffenders && offenders.length > 0 && (
            <div className="wb-group">
              <div className="wb-group-title flex items-center gap-1.5">
                <UserGroupIcon className="w-3.5 h-3.5 text-[#f44336]" />
                Repeat Offenders (Detected 3+ times in 30 days)
              </div>
              <div className="wb-group-body p-0">
                <div className="table-container" style={{ border: 'none' }}>
                  <table className="table">
                    <thead>
                      <tr>
                        <th>Username</th>
                        <th>Detections</th>
                        <th>High Risk Count</th>
                        <th>Avg Confidence</th>
                        <th>Last Detected</th>
                      </tr>
                    </thead>
                    <tbody>
                      {offenders.slice(0, 10).map((o) => (
                        <tr key={o.subscriber_id}>
                          <td>
                            <div>
                              <div className="font-medium text-gray-900">{o.username}</div>
                              <div className="text-[11px] text-gray-500">{o.full_name}</div>
                            </div>
                          </td>
                          <td>
                            <span className="font-bold text-[#f44336]">{o.detection_count}x</span>
                          </td>
                          <td>{o.high_risk_count}</td>
                          <td>{Math.round(o.avg_confidence)}%</td>
                          <td className="text-[11px] text-gray-500">
                            {new Date(o.last_detected_at).toLocaleDateString()}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
          )}

          {/* History Table */}
          <div className="wb-group">
            <div className="wb-group-title">Detection History</div>
            <div className="wb-group-body p-0">
              <div className="table-container" style={{ border: 'none' }}>
                <table className="table">
                  <thead>
                    <tr>
                      <th>Date</th>
                      <th>Username</th>
                      <th>Connections</th>
                      <th>TTL Status</th>
                      <th>Risk Level</th>
                      <th>Confidence</th>
                      <th>Scan Type</th>
                    </tr>
                  </thead>
                  <tbody>
                    {isLoadingHistory ? (
                      <tr>
                        <td colSpan={7} className="text-center py-3">
                          <ArrowPathIcon className="h-5 w-5 animate-spin mx-auto text-[#316AC5]" />
                        </td>
                      </tr>
                    ) : history.length === 0 ? (
                      <tr>
                        <td colSpan={7} className="text-center py-3 text-[12px] text-gray-500">
                          No detection history found. Run a manual scan or wait for the nightly automatic scan.
                        </td>
                      </tr>
                    ) : (
                      history.map((h) => (
                        <tr key={h.id}>
                          <td className="text-[11px] text-gray-500">
                            {new Date(h.detected_at).toLocaleString()}
                          </td>
                          <td>
                            <span className="font-medium text-gray-900">{h.username}</span>
                          </td>
                          <td className="font-mono">{h.connection_count}</td>
                          <td>{getTTLBadge(h.ttl_status, h.ttl_values ? JSON.parse(h.ttl_values) : [])}</td>
                          <td>{getSuspicionBadge(h.suspicion_level)}</td>
                          <td>
                            <span className={clsx(
                              'font-medium',
                              h.confidence_score >= 70 ? 'text-[#f44336]' :
                              h.confidence_score >= 40 ? 'text-[#FF9800]' :
                              'text-gray-600'
                            )}>
                              {h.confidence_score}%
                            </span>
                          </td>
                          <td>
                            <span className={h.scan_type === 'automatic' ? 'badge-info' : 'badge-gray'}>
                              {h.scan_type}
                            </span>
                          </td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        </>
      )}

      {/* Settings Tab */}
      {activeTab === 'settings' && (<>
        <div className="wb-group">
          <div className="wb-group-title flex items-center gap-1.5">
            <CogIcon className="w-3.5 h-3.5 text-[#316AC5]" />
            Automatic Scan Settings
          </div>
          <div className="wb-group-body">
            {isLoadingSettings ? (
              <div className="flex items-center justify-center py-3">
                <ArrowPathIcon className="h-5 w-5 animate-spin text-[#316AC5]" />
              </div>
            ) : (
              <div className="space-y-2 max-w-md">
                <div className="flex items-center justify-between">
                  <div>
                    <div className="text-[12px] font-medium text-gray-900">Enable Automatic Scanning</div>
                    <div className="text-[11px] text-gray-500">Run nightly scans automatically</div>
                  </div>
                  <label className="flex items-center gap-1.5 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={settings.enabled || false}
                      onChange={() => updateSettingsMutation.mutate({ enabled: !settings.enabled })}
                      className="w-3.5 h-3.5"
                    />
                    <span className="text-[12px] text-gray-600">{settings.enabled ? 'On' : 'Off'}</span>
                  </label>
                </div>

                <div className="border-t border-[#ccc] pt-2">
                  <label className="label">Scan Time</label>
                  <input
                    type="time"
                    value={settings.scan_time || '03:00'}
                    onChange={(e) => updateSettingsMutation.mutate({ scan_time: e.target.value })}
                    className="input" style={{ width: '120px' }}
                  />
                  <p className="text-[11px] text-gray-500 mt-0.5">
                    Recommended: 03:00 - 05:00 (off-peak hours)
                  </p>
                </div>

                <div>
                  <label className="label">Retention Days</label>
                  <select
                    value={settings.retention_days || 30}
                    onChange={(e) => updateSettingsMutation.mutate({ retention_days: Number(e.target.value) })}
                    className="input" style={{ width: '120px' }}
                  >
                    <option value={7}>7 days</option>
                    <option value={14}>14 days</option>
                    <option value={30}>30 days</option>
                    <option value={60}>60 days</option>
                    <option value={90}>90 days</option>
                  </select>
                  <p className="text-[11px] text-gray-500 mt-0.5">
                    Old detection records will be automatically deleted
                  </p>
                </div>

                <div>
                  <label className="label">Minimum Risk Level to Save</label>
                  <select
                    value={settings.min_suspicion_level || 'medium'}
                    onChange={(e) => updateSettingsMutation.mutate({ min_suspicion_level: e.target.value })}
                    className="input" style={{ width: '160px' }}
                  >
                    <option value="low">Low (save all)</option>
                    <option value="medium">Medium (recommended)</option>
                    <option value="high">High only</option>
                  </select>
                </div>

                <div>
                  <label className="label">Connection Threshold</label>
                  <input
                    type="number"
                    value={settings.connection_threshold || 500}
                    onChange={(e) => updateSettingsMutation.mutate({ connection_threshold: Number(e.target.value) })}
                    className="input" style={{ width: '120px' }}
                    min={100}
                    max={2000}
                    step={100}
                  />
                  <p className="text-[11px] text-gray-500 mt-0.5">
                    Users with more connections are flagged as suspicious
                  </p>
                </div>

                <div className="pt-2 border-t border-[#ccc]">
                  <p className="text-[11px] text-gray-500">
                    <InformationCircleIcon className="w-3.5 h-3.5 inline mr-0.5 -mt-0.5" />
                    The automatic scan will run daily at the configured time and save suspicious accounts to history.
                  </p>
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Automated Actions */}
        <div className="wb-group">
          <div className="wb-group-title flex items-center gap-1.5">
            <ExclamationTriangleIcon className="w-3.5 h-3.5 text-orange-500" />
            Automated Actions
          </div>
          <div className="wb-group-body">
            <div className="space-y-3 max-w-lg">
              {/* Auto-Flag */}
              <div className="flex items-start gap-3 p-2 border border-[#ccc] rounded">
                <input
                  type="checkbox"
                  checked={settings.auto_flag_enabled || false}
                  onChange={() => updateSettingsMutation.mutate({ auto_flag_enabled: !settings.auto_flag_enabled })}
                  className="w-3.5 h-3.5 mt-0.5"
                />
                <div className="flex-1">
                  <div className="text-[12px] font-medium">Auto-Flag Subscribers</div>
                  <div className="text-[11px] text-gray-500 mb-1">Append a note to subscribers with high sharing scores</div>
                  <div className="flex items-center gap-1.5">
                    <label className="text-[11px] text-gray-600">Score threshold:</label>
                    <input
                      type="number"
                      value={settings.auto_flag_threshold || 70}
                      onChange={(e) => updateSettingsMutation.mutate({ auto_flag_threshold: Number(e.target.value) })}
                      className="input"
                      style={{ width: '60px' }}
                      min={1}
                      max={100}
                    />
                  </div>
                </div>
              </div>

              {/* Speed Reduction */}
              <div className="flex items-start gap-3 p-2 border border-[#ccc] rounded">
                <input
                  type="checkbox"
                  checked={settings.speed_reduction_enabled || false}
                  onChange={() => updateSettingsMutation.mutate({ speed_reduction_enabled: !settings.speed_reduction_enabled })}
                  className="w-3.5 h-3.5 mt-0.5"
                />
                <div className="flex-1">
                  <div className="text-[12px] font-medium">Speed Reduction Note</div>
                  <div className="text-[11px] text-gray-500 mb-1">Add a speed reduction recommendation note to flagged subscribers</div>
                  <div className="flex items-center gap-2">
                    <div className="flex items-center gap-1.5">
                      <label className="text-[11px] text-gray-600">Score:</label>
                      <input
                        type="number"
                        value={settings.speed_reduction_threshold || 80}
                        onChange={(e) => updateSettingsMutation.mutate({ speed_reduction_threshold: Number(e.target.value) })}
                        className="input"
                        style={{ width: '60px' }}
                        min={1}
                        max={100}
                      />
                    </div>
                    <div className="flex items-center gap-1.5">
                      <label className="text-[11px] text-gray-600">Reduce to:</label>
                      <input
                        type="number"
                        value={settings.speed_reduction_percent || 50}
                        onChange={(e) => updateSettingsMutation.mutate({ speed_reduction_percent: Number(e.target.value) })}
                        className="input"
                        style={{ width: '60px' }}
                        min={10}
                        max={100}
                      />
                      <span className="text-[11px] text-gray-500">%</span>
                    </div>
                  </div>
                </div>
              </div>

              {/* WhatsApp Notification */}
              <div className="flex items-start gap-3 p-2 border border-[#ccc] rounded">
                <input
                  type="checkbox"
                  checked={settings.whatsapp_notify_enabled || false}
                  onChange={() => updateSettingsMutation.mutate({ whatsapp_notify_enabled: !settings.whatsapp_notify_enabled })}
                  className="w-3.5 h-3.5 mt-0.5"
                />
                <div className="flex-1">
                  <div className="text-[12px] font-medium">WhatsApp Notification</div>
                  <div className="text-[11px] text-gray-500 mb-1">Send WhatsApp notification to subscribers with high scores</div>
                  <div className="flex items-center gap-1.5 mb-1.5">
                    <label className="text-[11px] text-gray-600">Score threshold:</label>
                    <input
                      type="number"
                      value={settings.whatsapp_notify_threshold || 60}
                      onChange={(e) => updateSettingsMutation.mutate({ whatsapp_notify_threshold: Number(e.target.value) })}
                      className="input"
                      style={{ width: '60px' }}
                      min={1}
                      max={100}
                    />
                  </div>
                  <textarea
                    value={settings.whatsapp_notify_template || ''}
                    onChange={(e) => updateSettingsMutation.mutate({ whatsapp_notify_template: e.target.value })}
                    placeholder="Message template. Variables: {username}, {score}, {category}"
                    className="input w-full"
                    rows={2}
                    style={{ fontSize: '11px' }}
                  />
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Whitelisted Subscribers */}
        <div className="wb-group">
          <div className="wb-group-title flex items-center gap-1.5">
            <ShieldCheckIcon className="w-3.5 h-3.5 text-green-600" />
            Whitelisted Subscribers
            {whitelistedSubs.length > 0 && (
              <span className="ml-1 bg-green-100 text-green-700 text-[10px] px-1.5 py-0.5 rounded-full font-medium">
                {whitelistedSubs.length}
              </span>
            )}
          </div>
          <div className="wb-group-body">
            {whitelistedSubs.length === 0 ? (
              <p className="text-[11px] text-gray-500">No whitelisted subscribers. Use the shield icon in Customer Scores to whitelist.</p>
            ) : (
              <div className="space-y-1">
                {whitelistedSubs.map((sub) => (
                  <div key={sub.id} className="flex items-center justify-between px-2 py-1 border border-[#ccc] rounded text-[11px]">
                    <div>
                      <span className="font-medium">{sub.username}</span>
                      {sub.full_name && <span className="text-gray-500 ml-1">({sub.full_name})</span>}
                      <span className="ml-2 text-[10px] px-1.5 py-0.5 bg-gray-100 rounded">{sub.reason || 'N/A'}</span>
                    </div>
                    <button
                      onClick={() => toggleWhitelistMutation.mutate({ id: sub.id, whitelisted: false, reason: '' })}
                      className="text-red-500 hover:text-red-700 text-[10px]"
                    >
                      Remove
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </>)}
    </div>
  )
}
