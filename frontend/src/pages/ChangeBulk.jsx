import { useState, useMemo } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import api, { serviceApi, resellerApi, nasApi } from '../services/api'
import { formatDate } from '../utils/timezone'
import {
  useReactTable,
  getCoreRowModel,
  flexRender,
} from '@tanstack/react-table'
import {
  PlusIcon,
  TrashIcon,
  EyeIcon,
  FunnelIcon,
  BoltIcon,
  UsersIcon,
  AdjustmentsHorizontalIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  ExclamationTriangleIcon,
  CheckCircleIcon,
  XMarkIcon,
  CalendarDaysIcon,
  CurrencyDollarIcon,
  ServerStackIcon,
  UserGroupIcon,
  ArrowPathIcon,
  NoSymbolIcon,
  CheckIcon,
  CircleStackIcon,
  SignalIcon,
  KeyIcon,
  WifiIcon,
  ClockIcon,
} from '@heroicons/react/24/outline'
import toast from 'react-hot-toast'

export default function ChangeBulk() {
  const [filters, setFilters] = useState({
    reseller_id: 0,
    service_id: 0,
    nas_id: 0,
    status_filter: 'all',
    online_filter: 'all',
    fup_level_filter: 'all',
    include_sub_resellers: false,
  })
  const [action, setAction] = useState('')
  const [actionValue, setActionValue] = useState('')
  const [customFilters, setCustomFilters] = useState([])
  const [newFilter, setNewFilter] = useState({ field: 'username', rule: 'like', value: '' })
  const [previewData, setPreviewData] = useState(null)
  const [previewTotal, setPreviewTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(50)
  const [showConfirmModal, setShowConfirmModal] = useState(false)

  // Fetch services
  const { data: services } = useQuery({
    queryKey: ['services'],
    queryFn: () => serviceApi.list().then(r => r.data.data || []),
  })

  // Fetch resellers
  const { data: resellers } = useQuery({
    queryKey: ['resellers'],
    queryFn: () => resellerApi.list().then(r => r.data.data || []),
  })

  // Fetch NAS devices
  const { data: nasList } = useQuery({
    queryKey: ['nas'],
    queryFn: () => nasApi.list().then(r => r.data.data || []),
  })

  // Preview mutation
  const previewMutation = useMutation({
    mutationFn: (data) => api.post(`/subscribers/change-bulk?page=${page}&limit=${pageSize}`, { ...data, preview: true }),
    onSuccess: (res) => {
      setPreviewData(res.data.data || [])
      setPreviewTotal(res.data.meta?.total || 0)
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to preview'),
  })

  // Execute mutation
  const executeMutation = useMutation({
    mutationFn: (data) => api.post('/subscribers/change-bulk', { ...data, preview: false }),
    onSuccess: (res) => {
      toast.success(res.data.message || 'Bulk action completed successfully')
      setPreviewData(null)
      setPreviewTotal(0)
      setShowConfirmModal(false)
    },
    onError: (err) => {
      toast.error(err.response?.data?.message || 'Failed to execute')
      setShowConfirmModal(false)
    },
  })

  const handleAddFilter = () => {
    if (newFilter.value.trim()) {
      setCustomFilters([...customFilters, { ...newFilter }])
      setNewFilter({ field: 'username', rule: 'like', value: '' })
    }
  }

  const handleRemoveFilter = (index) => {
    setCustomFilters(customFilters.filter((_, i) => i !== index))
  }

  const handlePreview = () => {
    const data = {
      ...filters,
      action,
      action_value: actionValue,
      filters: customFilters,
    }
    previewMutation.mutate(data)
  }

  const handleExecute = () => {
    if (!action) {
      toast.error('Please select an action')
      return
    }
    const actionsNeedingValue = ['set_expiry', 'set_service', 'set_reseller', 'set_monthly_quota', 'set_daily_quota', 'set_price', 'renew', 'add_days', 'set_nas', 'set_password', 'set_static_ip']
    if (actionsNeedingValue.includes(action) && !actionValue) {
      toast.error('Please enter a value for the action')
      return
    }
    setShowConfirmModal(true)
  }

  const confirmExecute = () => {
    const data = {
      ...filters,
      action,
      action_value: actionValue,
      filters: customFilters,
    }
    executeMutation.mutate(data)
  }

  // Table columns
  const columns = useMemo(() => [
    { accessorKey: 'username', header: 'Username' },
    { accessorKey: 'full_name', header: 'Name' },
    {
      accessorKey: 'Reseller',
      header: 'Reseller',
      cell: ({ row }) => row.original.Reseller?.User?.username || row.original.Reseller?.name || '-',
    },
    {
      accessorKey: 'Service',
      header: 'Service',
      cell: ({ row }) => row.original.Service?.name || '-',
    },
    {
      accessorKey: 'Nas',
      header: 'NAS',
      cell: ({ row }) => row.original.Nas?.name || '-',
    },
    {
      accessorKey: 'is_online',
      header: 'Online',
      cell: ({ getValue }) => getValue() ?
        <span className="badge-success">Online</span> :
        <span className="badge-gray">Offline</span>
    },
    {
      accessorKey: 'status',
      header: 'Status',
      cell: ({ getValue }) => getValue() === 'active' ?
        <span className="badge-success">Active</span> :
        <span className="badge-danger">Inactive</span>
    },
    { accessorKey: 'price', header: 'Price', cell: ({ getValue }) => `$${getValue()?.toFixed(2) || '0.00'}` },
    {
      accessorKey: 'expiry_date',
      header: 'Expiry',
      cell: ({ getValue }) => formatDate(getValue()),
    },
  ], [])

  const table = useReactTable({
    data: previewData || [],
    columns,
    getCoreRowModel: getCoreRowModel(),
  })

  const actionOptions = [
    { value: '', label: 'Select an action...', icon: null, category: '' },
    // Status Actions
    { value: 'set_active', label: 'Activate Subscribers', icon: CheckIcon, category: 'Status' },
    { value: 'set_inactive', label: 'Deactivate Subscribers', icon: NoSymbolIcon, category: 'Status' },
    { value: 'disconnect', label: 'Disconnect (Kick Online)', icon: WifiIcon, category: 'Status' },
    // Date Actions
    { value: 'set_expiry', label: 'Set Expiry Date', icon: CalendarDaysIcon, category: 'Date' },
    { value: 'add_days', label: 'Add Days to Expiry', icon: ClockIcon, category: 'Date' },
    { value: 'renew', label: 'Renew (Reset + Add Days)', icon: ArrowPathIcon, category: 'Date' },
    // Assignment Actions
    { value: 'set_service', label: 'Change Service', icon: ServerStackIcon, category: 'Assignment' },
    { value: 'set_reseller', label: 'Change Reseller', icon: UserGroupIcon, category: 'Assignment' },
    { value: 'set_nas', label: 'Change NAS', icon: SignalIcon, category: 'Assignment' },
    // Quota Actions
    { value: 'set_monthly_quota', label: 'Set Monthly Quota (GB)', icon: CircleStackIcon, category: 'Quota' },
    { value: 'set_daily_quota', label: 'Set Daily Quota (MB)', icon: CircleStackIcon, category: 'Quota' },
    { value: 'reset_fup', label: 'Reset Daily FUP', icon: ArrowPathIcon, category: 'Quota' },
    { value: 'reset_monthly_fup', label: 'Reset Monthly FUP', icon: ArrowPathIcon, category: 'Quota' },
    { value: 'reset_all_counters', label: 'Reset All Counters', icon: ArrowPathIcon, category: 'Quota' },
    // Account Actions
    { value: 'set_price', label: 'Set Price', icon: CurrencyDollarIcon, category: 'Account' },
    { value: 'set_password', label: 'Set Password', icon: KeyIcon, category: 'Account' },
    { value: 'set_static_ip', label: 'Set Static IP', icon: SignalIcon, category: 'Account' },
    { value: 'reset_mac', label: 'Reset MAC Address', icon: ArrowPathIcon, category: 'Account' },
    // Danger Actions
    { value: 'delete', label: 'Delete Subscribers', icon: TrashIcon, category: 'Danger' },
  ]

  const filterFields = [
    { value: 'username', label: 'Username' },
    { value: 'name', label: 'Full Name' },
    { value: 'address', label: 'Address' },
    { value: 'phone', label: 'Phone' },
    { value: 'expiry', label: 'Expiry Date' },
    { value: 'created', label: 'Created Date' },
    { value: 'price', label: 'Price' },
    { value: 'daily_usage', label: 'Daily Usage (bytes)' },
    { value: 'monthly_usage', label: 'Monthly Usage (bytes)' },
  ]

  const filterRules = [
    { value: 'equal', label: '= Equal' },
    { value: 'notequal', label: '≠ Not Equal' },
    { value: 'greater', label: '> Greater Than' },
    { value: 'less', label: '< Less Than' },
    { value: 'like', label: '~ Contains' },
  ]

  const renderActionInput = () => {
    switch (action) {
      case 'set_expiry':
        return (
          <input
            type="date"
            className="input mt-0.5"
            value={actionValue}
            onChange={(e) => setActionValue(e.target.value)}
          />
        )
      case 'set_service':
        return (
          <select
            className="input mt-0.5"
            value={actionValue}
            onChange={(e) => setActionValue(e.target.value)}
          >
            <option value="">Select service...</option>
            {services?.map(s => (
              <option key={s.id} value={s.id}>{s.name}</option>
            ))}
          </select>
        )
      case 'set_reseller':
        return (
          <select
            className="input mt-0.5"
            value={actionValue}
            onChange={(e) => setActionValue(e.target.value)}
          >
            <option value="">Select reseller...</option>
            {resellers?.map(r => (
              <option key={r.id} value={r.id}>{r.User?.username || r.name}</option>
            ))}
          </select>
        )
      case 'set_nas':
        return (
          <select
            className="input mt-0.5"
            value={actionValue}
            onChange={(e) => setActionValue(e.target.value)}
          >
            <option value="">Select NAS...</option>
            {nasList?.map(n => (
              <option key={n.id} value={n.id}>{n.name} ({n.ip_address})</option>
            ))}
          </select>
        )
      case 'set_monthly_quota':
        return (
          <div className="relative mt-0.5">
            <input
              type="number"
              className="input pr-8"
              placeholder="Enter quota"
              value={actionValue}
              onChange={(e) => setActionValue(e.target.value)}
            />
            <span className="absolute right-2 top-1/2 -translate-y-1/2 text-[11px] text-gray-500">GB</span>
          </div>
        )
      case 'set_daily_quota':
        return (
          <div className="relative mt-0.5">
            <input
              type="number"
              className="input pr-8"
              placeholder="Enter quota"
              value={actionValue}
              onChange={(e) => setActionValue(e.target.value)}
            />
            <span className="absolute right-2 top-1/2 -translate-y-1/2 text-[11px] text-gray-500">MB</span>
          </div>
        )
      case 'set_price':
        return (
          <div className="relative mt-0.5">
            <span className="absolute left-2 top-1/2 -translate-y-1/2 text-[11px] text-gray-500">$</span>
            <input
              type="number"
              step="0.01"
              className="input pl-5"
              placeholder="0.00"
              value={actionValue}
              onChange={(e) => setActionValue(e.target.value)}
            />
          </div>
        )
      case 'renew':
      case 'add_days':
        return (
          <div className="relative mt-0.5">
            <input
              type="number"
              className="input pr-12"
              placeholder="30"
              value={actionValue}
              onChange={(e) => setActionValue(e.target.value)}
            />
            <span className="absolute right-2 top-1/2 -translate-y-1/2 text-[11px] text-gray-500">days</span>
          </div>
        )
      case 'set_password':
        return (
          <input
            type="text"
            className="input mt-0.5"
            placeholder="New password"
            value={actionValue}
            onChange={(e) => setActionValue(e.target.value)}
          />
        )
      case 'set_static_ip':
        return (
          <input
            type="text"
            className="input mt-0.5"
            placeholder="10.0.0.100 (leave empty to remove)"
            value={actionValue}
            onChange={(e) => setActionValue(e.target.value)}
          />
        )
      default:
        return null
    }
  }

  const getActionIcon = () => {
    const actionDef = actionOptions.find(a => a.value === action)
    if (actionDef?.icon) {
      const Icon = actionDef.icon
      return <Icon className="w-3.5 h-3.5" />
    }
    return <BoltIcon className="w-3.5 h-3.5" />
  }

  const getActionDescription = () => {
    const descriptions = {
      'set_active': 'This will activate all matching subscribers.',
      'set_inactive': 'This will deactivate all matching subscribers.',
      'disconnect': 'This will disconnect all online matching subscribers from the network.',
      'set_expiry': 'This will update the expiry date for all matching subscribers.',
      'add_days': 'This will add the specified number of days to each subscriber\'s current expiry date.',
      'renew': 'This will reset FUP counters and extend expiry by the specified days (default 30).',
      'set_service': 'This will change the service plan for all matching subscribers.',
      'set_reseller': 'This will transfer all matching subscribers to another reseller.',
      'set_nas': 'This will assign a new NAS to all matching subscribers.',
      'set_monthly_quota': 'This will set the monthly quota limit for all matching subscribers.',
      'set_daily_quota': 'This will set the daily quota limit for all matching subscribers.',
      'reset_fup': 'This will reset daily FUP level and counters for all matching subscribers.',
      'reset_monthly_fup': 'This will reset monthly FUP level and counters for all matching subscribers.',
      'reset_all_counters': 'This will reset ALL usage counters (daily + monthly) for all matching subscribers.',
      'set_price': 'This will update the price for all matching subscribers.',
      'set_password': 'This will set a new password for all matching subscribers.',
      'set_static_ip': 'This will set or remove static IP for all matching subscribers.',
      'reset_mac': 'This will reset the MAC address binding for all matching subscribers.',
      'delete': 'DANGER: This will permanently delete all matching subscribers!',
    }
    return descriptions[action] || ''
  }

  // Group actions by category for the dropdown
  const groupedActions = useMemo(() => {
    const groups = {}
    actionOptions.forEach(opt => {
      if (!opt.category) return
      if (!groups[opt.category]) groups[opt.category] = []
      groups[opt.category].push(opt)
    })
    return groups
  }, [])

  return (
    <div className="space-y-3" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Header Toolbar */}
      <div className="wb-toolbar justify-between">
        <div className="flex items-center gap-2">
          <AdjustmentsHorizontalIcon className="w-4 h-4 text-[#316AC5]" />
          <span className="text-[13px] font-semibold">Bulk Operations</span>
        </div>
        {previewTotal > 0 && (
          <div className="flex items-center gap-1">
            <UsersIcon className="w-3.5 h-3.5 text-[#316AC5]" />
            <span className="text-[12px] font-medium text-[#316AC5]">{previewTotal} subscribers selected</span>
          </div>
        )}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-3">
        {/* Left Column - Filters */}
        <div className="lg:col-span-2 space-y-3">
          {/* Basic Filters */}
          <div className="wb-group">
            <div className="wb-group-title flex items-center gap-1.5">
              <FunnelIcon className="w-3.5 h-3.5 text-gray-500" />
              Filter Subscribers
            </div>
            <div className="wb-group-body">
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                {/* Reseller Filter */}
                <div>
                  <label className="label">Reseller</label>
                  <select
                    className="input"
                    value={filters.reseller_id}
                    onChange={(e) => setFilters({ ...filters, reseller_id: parseInt(e.target.value) })}
                  >
                    <option value={0}>All Resellers</option>
                    {resellers?.map(r => (
                      <option key={r.id} value={r.id}>{r.User?.username || r.name}</option>
                    ))}
                  </select>
                </div>

                {/* Service Filter */}
                <div>
                  <label className="label">Service</label>
                  <select
                    className="input"
                    value={filters.service_id}
                    onChange={(e) => setFilters({ ...filters, service_id: parseInt(e.target.value) })}
                  >
                    <option value={0}>All Services</option>
                    {services?.map(s => (
                      <option key={s.id} value={s.id}>{s.name}</option>
                    ))}
                  </select>
                </div>

                {/* NAS Filter */}
                <div>
                  <label className="label">NAS / Router</label>
                  <select
                    className="input"
                    value={filters.nas_id}
                    onChange={(e) => setFilters({ ...filters, nas_id: parseInt(e.target.value) })}
                  >
                    <option value={0}>All NAS</option>
                    {nasList?.map(n => (
                      <option key={n.id} value={n.id}>{n.name}</option>
                    ))}
                  </select>
                </div>

                {/* Status Filter */}
                <div>
                  <label className="label">Account Status</label>
                  <select
                    className="input"
                    value={filters.status_filter}
                    onChange={(e) => setFilters({ ...filters, status_filter: e.target.value })}
                  >
                    <option value="all">All Statuses</option>
                    <option value="active">Active Only</option>
                    <option value="inactive">Inactive Only</option>
                    <option value="active_inactive">Active & Inactive</option>
                    <option value="expired">Expired Only</option>
                  </select>
                </div>

                {/* Online Filter */}
                <div>
                  <label className="label">Online Status</label>
                  <select
                    className="input"
                    value={filters.online_filter}
                    onChange={(e) => setFilters({ ...filters, online_filter: e.target.value })}
                  >
                    <option value="all">All (Online & Offline)</option>
                    <option value="online">Online Only</option>
                    <option value="offline">Offline Only</option>
                  </select>
                </div>

                {/* FUP Level Filter */}
                <div>
                  <label className="label">FUP Level</label>
                  <select
                    className="input"
                    value={filters.fup_level_filter}
                    onChange={(e) => setFilters({ ...filters, fup_level_filter: e.target.value })}
                  >
                    <option value="all">All FUP Levels</option>
                    <option value="0">Level 0 (Full Speed)</option>
                    <option value="1">Level 1</option>
                    <option value="2">Level 2</option>
                    <option value="3">Level 3 (Lowest)</option>
                  </select>
                </div>

                {/* Include Sub-resellers */}
                <div className="flex items-center md:col-span-2 lg:col-span-3 pt-1">
                  <label className="flex items-center gap-1.5 cursor-pointer text-[12px] text-gray-700">
                    <input
                      type="checkbox"
                      checked={filters.include_sub_resellers}
                      onChange={(e) => setFilters({ ...filters, include_sub_resellers: e.target.checked })}
                      className="w-3.5 h-3.5"
                    />
                    Include Sub-resellers
                  </label>
                </div>
              </div>
            </div>
          </div>

          {/* Custom Filters */}
          <div className="wb-group">
            <div className="wb-group-title flex items-center gap-1.5">
              <AdjustmentsHorizontalIcon className="w-3.5 h-3.5 text-gray-500" />
              Advanced Filters
            </div>
            <div className="wb-group-body">
              <div className="flex gap-2 items-end flex-wrap">
                <div className="flex-1 min-w-[120px]">
                  <label className="label">Field</label>
                  <select
                    className="input"
                    value={newFilter.field}
                    onChange={(e) => setNewFilter({ ...newFilter, field: e.target.value })}
                  >
                    {filterFields.map(f => (
                      <option key={f.value} value={f.value}>{f.label}</option>
                    ))}
                  </select>
                </div>

                <div className="flex-1 min-w-[120px]">
                  <label className="label">Condition</label>
                  <select
                    className="input"
                    value={newFilter.rule}
                    onChange={(e) => setNewFilter({ ...newFilter, rule: e.target.value })}
                  >
                    {filterRules.map(r => (
                      <option key={r.value} value={r.value}>{r.label}</option>
                    ))}
                  </select>
                </div>

                <div className="flex-1 min-w-[140px]">
                  <label className="label">Value</label>
                  <input
                    type="text"
                    className="input"
                    value={newFilter.value}
                    onChange={(e) => setNewFilter({ ...newFilter, value: e.target.value })}
                    placeholder="Enter value..."
                    onKeyDown={(e) => e.key === 'Enter' && handleAddFilter()}
                  />
                </div>

                <button
                  onClick={handleAddFilter}
                  className="btn btn-primary"
                >
                  <PlusIcon className="w-3.5 h-3.5 mr-1" />
                  Add
                </button>
              </div>

              {/* Active Custom Filters */}
              {customFilters.length > 0 && (
                <div className="mt-3 pt-3 border-t border-[#ccc]">
                  <p className="text-[11px] text-gray-500 mb-2">Active filters:</p>
                  <div className="flex flex-wrap gap-1.5">
                    {customFilters.map((f, i) => (
                      <div
                        key={i}
                        className="inline-flex items-center gap-1 px-2 py-0.5 bg-[#e8e8f0] border border-[#a0a0a0] text-[11px]"
                        style={{ borderRadius: '2px' }}
                      >
                        <span className="font-medium text-[#316AC5]">{filterFields.find(ff => ff.value === f.field)?.label}</span>
                        <span className="text-gray-500">{filterRules.find(r => r.value === f.rule)?.label.split(' ')[0]}</span>
                        <span className="font-semibold text-gray-800">"{f.value}"</span>
                        <button
                          onClick={() => handleRemoveFilter(i)}
                          className="ml-0.5 text-gray-400 hover:text-red-500"
                        >
                          <XMarkIcon className="w-3 h-3" />
                        </button>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Right Column - Action */}
        <div className="space-y-3">
          <div className="wb-group sticky top-3">
            <div className="wb-group-title flex items-center gap-1.5">
              <BoltIcon className="w-3.5 h-3.5 text-[#9C27B0]" />
              Action to Perform
            </div>
            <div className="wb-group-body space-y-3">
              <div>
                <label className="label">Select Action</label>
                <select
                  className="input"
                  value={action}
                  onChange={(e) => {
                    setAction(e.target.value)
                    setActionValue('')
                  }}
                >
                  <option value="">Select an action...</option>
                  {Object.entries(groupedActions).map(([category, actions]) => (
                    <optgroup key={category} label={`-- ${category} --`}>
                      {actions.map(opt => (
                        <option key={opt.value} value={opt.value}>{opt.label}</option>
                      ))}
                    </optgroup>
                  ))}
                </select>
              </div>

              {/* Action Value */}
              {action && !['set_active', 'set_inactive', 'reset_mac', 'disconnect', 'reset_fup', 'reset_monthly_fup', 'reset_all_counters', 'delete'].includes(action) && (
                <div>
                  <label className="label">
                    {actionOptions.find(a => a.value === action)?.label}
                  </label>
                  {renderActionInput()}
                </div>
              )}

              {/* Action description */}
              {action && (
                <div className={`p-2 border text-[11px] ${action === 'delete' ? 'bg-[#fde8e8] border-[#e74c3c]' : 'bg-[#fff8e1] border-[#FF9800]'}`} style={{ borderRadius: '2px' }}>
                  <div className="flex gap-1.5">
                    <ExclamationTriangleIcon className={`w-3.5 h-3.5 flex-shrink-0 mt-0.5 ${action === 'delete' ? 'text-[#e74c3c]' : 'text-[#FF9800]'}`} />
                    <p className={action === 'delete' ? 'text-[#c0392b]' : 'text-[#e65100]'}>
                      {getActionDescription()}
                    </p>
                  </div>
                </div>
              )}

              {/* Action Buttons */}
              <div className="pt-2 space-y-2 border-t border-[#ccc]">
                <button
                  onClick={handlePreview}
                  disabled={previewMutation.isPending}
                  className="btn w-full"
                >
                  <EyeIcon className="w-3.5 h-3.5 mr-1" />
                  {previewMutation.isPending ? 'Loading Preview...' : 'Preview Changes'}
                </button>
                <button
                  onClick={handleExecute}
                  disabled={!action || executeMutation.isPending}
                  className={`w-full ${action === 'delete' ? 'btn btn-danger' : 'btn btn-primary'}`}
                >
                  {getActionIcon()}
                  <span className="ml-1">{executeMutation.isPending ? 'Executing...' : 'Execute Action'}</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Preview Table */}
      {previewData && previewData.length > 0 && (
        <div className="wb-group">
          <div className="wb-group-title flex items-center justify-between">
            <div className="flex items-center gap-1.5">
              <CheckCircleIcon className="w-3.5 h-3.5 text-[#4CAF50]" />
              Preview Results
              <span className="badge-success ml-1">{previewTotal} subscribers</span>
            </div>
            <div className="flex items-center gap-2">
              <span className="text-[11px] text-gray-500">Rows per page:</span>
              <select
                className="input-sm"
                style={{ width: '60px' }}
                value={pageSize}
                onChange={(e) => {
                  setPageSize(parseInt(e.target.value))
                  setPage(1)
                }}
              >
                <option value={10}>10</option>
                <option value={30}>30</option>
                <option value={50}>50</option>
                <option value={100}>100</option>
              </select>
            </div>
          </div>

          <div className="table-container" style={{ border: 'none' }}>
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
                {table.getRowModel().rows.map((row) => (
                  <tr key={row.id}>
                    {row.getVisibleCells().map(cell => (
                      <td key={cell.id}>
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Pagination */}
          {previewTotal > pageSize && (
            <div className="wb-statusbar">
              <span className="text-[11px]">
                Showing {((page - 1) * pageSize) + 1} to {Math.min(page * pageSize, previewTotal)} of {previewTotal}
              </span>
              <div className="flex items-center gap-1">
                <button
                  className="btn btn-sm"
                  disabled={page === 1}
                  onClick={() => {
                    setPage(page - 1)
                    setTimeout(handlePreview, 0)
                  }}
                >
                  <ChevronLeftIcon className="w-3 h-3 mr-0.5" />
                  Previous
                </button>
                <span className="text-[11px] px-2">
                  Page {page} of {Math.ceil(previewTotal / pageSize)}
                </span>
                <button
                  className="btn btn-sm"
                  disabled={page >= Math.ceil(previewTotal / pageSize)}
                  onClick={() => {
                    setPage(page + 1)
                    setTimeout(handlePreview, 0)
                  }}
                >
                  Next
                  <ChevronRightIcon className="w-3 h-3 ml-0.5" />
                </button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Empty preview state */}
      {previewData && previewData.length === 0 && (
        <div className="wb-group">
          <div className="wb-group-body text-center py-2">
            <UsersIcon className="w-8 h-8 text-gray-400 mx-auto mb-2" />
            <p className="text-[12px] font-medium text-gray-700">No subscribers found</p>
            <p className="text-[11px] text-gray-500">Try adjusting your filters to match more subscribers.</p>
          </div>
        </div>
      )}

      {/* Confirmation Modal */}
      {showConfirmModal && (
        <div className="modal-overlay">
          <div className="modal modal-sm">
            <div className="modal-header">
              <div className="flex items-center gap-1.5">
                <ExclamationTriangleIcon className="w-4 h-4" />
                Confirm Bulk Action
              </div>
              <button onClick={() => setShowConfirmModal(false)} className="text-white hover:text-gray-200">
                <XMarkIcon className="w-4 h-4" />
              </button>
            </div>
            <div className="modal-body">
              <p className="text-[12px] text-gray-700 mb-3">
                You are about to perform <span className={`font-semibold ${action === 'delete' ? 'text-[#e74c3c]' : 'text-gray-900'}`}>"{actionOptions.find(a => a.value === action)?.label}"</span> on
                <span className="font-semibold text-[#316AC5]"> {previewTotal || 'all matching'} subscribers</span>.
              </p>
              <p className="text-[11px] text-gray-500">This action cannot be undone. Are you sure you want to continue?</p>
            </div>
            <div className="modal-footer">
              <button
                onClick={() => setShowConfirmModal(false)}
                className="btn"
              >
                Cancel
              </button>
              <button
                onClick={confirmExecute}
                disabled={executeMutation.isPending}
                className={action === 'delete' ? 'btn btn-danger' : 'btn btn-primary'}
              >
                {executeMutation.isPending ? (
                  <>
                    <ArrowPathIcon className="w-3.5 h-3.5 mr-1 animate-spin" />
                    Executing...
                  </>
                ) : (
                  <>
                    <CheckIcon className="w-3.5 h-3.5 mr-1" />
                    Yes, Execute
                  </>
                )}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
