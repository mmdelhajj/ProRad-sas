import { useState, useMemo, useRef, useEffect } from 'react'
import clsx from 'clsx'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from 'react-router-dom'
import { subscriberApi, serviceApi, nasApi, resellerApi, settingsApi } from '../services/api'
import { useAuthStore } from '../store/authStore'
import { formatDate } from '../utils/timezone'
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  flexRender,
} from '@tanstack/react-table'
import {
  PlusIcon,
  MagnifyingGlassIcon,
  FunnelIcon,
  ArrowPathIcon,
  PencilIcon,
  TrashIcon,
  ArrowsRightLeftIcon,
  ClockIcon,
  XCircleIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  ArrowUpTrayIcon,
  DocumentArrowUpIcon,
  ArchiveBoxIcon,
  ArrowUturnLeftIcon,
  XMarkIcon,
  ComputerDesktopIcon,
  CalendarDaysIcon,
  ArrowDownTrayIcon,
  EyeIcon,
  EyeSlashIcon,
  WifiIcon,
  PlayIcon,
  PauseIcon,
  BanknotesIcon,
  IdentificationIcon,
  Squares2X2Icon,
  CheckIcon,
  SignalIcon,
  MapPinIcon,
  ShieldExclamationIcon,
  ServerIcon,
} from '@heroicons/react/24/outline'
// clsx not needed - WinBox design uses inline styles and design system classes
import toast from 'react-hot-toast'
import 'leaflet/dist/leaflet.css'
import L from 'leaflet'

delete L.Icon.Default.prototype._getIconUrl
L.Icon.Default.mergeOptions({
  iconUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon.png',
  iconRetinaUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon-2x.png',
  shadowUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-shadow.png',
})

function ViewLocationMap({ lat, lng }) {
  const containerRef = useRef(null)
  const mapRef = useRef(null)
  useEffect(() => {
    if (!containerRef.current || mapRef.current) return
    const map = L.map(containerRef.current, { center: [lat, lng], zoom: 16 })
    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      attribution: '© <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
    }).addTo(map)
    L.marker([lat, lng]).addTo(map)
    mapRef.current = map
    return () => { map.remove(); mapRef.current = null }
  }, [lat, lng])
  return <div ref={containerRef} style={{ height: '320px', width: '100%' }} />
}

const statusFilters = [
  { value: '', label: 'All Status' },
  { value: 'online', label: 'Online' },
  { value: 'offline', label: 'Offline' },
  { value: 'active', label: 'Active' },
  { value: 'inactive', label: 'Inactive' },
  { value: 'expired', label: 'Expired' },
  { value: 'expiring', label: 'Expiring Soon' },
]

// Status color helper - WinBox dots
const getStatusDisplay = (subscriber) => {
  const isExpired = subscriber.expiry_date && new Date(subscriber.expiry_date) < new Date()
  const isExpiring = subscriber.expiry_date &&
    new Date(subscriber.expiry_date) < new Date(Date.now() + 7 * 24 * 60 * 60 * 1000) &&
    !isExpired

  if (subscriber.status === 0) {
    return { dotColor: '#9e9e9e', text: 'Inactive', textColor: '#757575' }
  }
  if (subscriber.status === 2) {
    return { dotColor: '#9e9e9e', text: 'Suspended', textColor: '#757575' }
  }
  if (isExpired) {
    return { dotColor: '#FF9800', text: 'Expired', textColor: '#e65100' }
  }
  if (subscriber.is_online) {
    return { dotColor: '#4CAF50', text: 'Online', textColor: '#2e7d32' }
  }
  return { dotColor: '#f44336', text: 'Offline', textColor: '#c62828' }
}

// Format bytes to human readable
const formatBytes = (bytes) => {
  if (!bytes || bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

export default function Subscribers() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { hasPermission, isAdmin } = useAuthStore()
  const fileInputRef = useRef(null)
  const [page, setPage] = useState(1)
  const [limit, setLimit] = useState(25)
  const [search, setSearch] = useState('')
  const [status, setStatus] = useState('')
  const [serviceId, setServiceId] = useState('')
  const [nasId, setNasId] = useState('')
  const [resellerId, setResellerId] = useState('')
  const [fupLevel, setFupLevel] = useState('')
  const [monthlyFup, setMonthlyFup] = useState(false)
  const [sortBy, setSortBy] = useState('')
  const [showFilters, setShowFilters] = useState(false)
  const [sorting, setSorting] = useState([])
  const [viewMode, setViewMode] = useState('active')

  // Archived view filters
  const [archivedResellerId, setArchivedResellerId] = useState('')
  const [archivedDeletedBy, setArchivedDeletedBy] = useState('')
  const [archivedFromDate, setArchivedFromDate] = useState('')
  const [archivedToDate, setArchivedToDate] = useState('')

  // Selected rows - stores subscriber IDs
  const [selectedIds, setSelectedIds] = useState(new Set())

  // Column visibility - load from localStorage
  const [visibleColumns, setVisibleColumns] = useState(() => {
    const defaultColumns = {
      username: true,
      full_name: true,
      phone: true,
      mac_address: true,
      ip_address: true,
      service: true,
      reseller: true,
      status: true,
      expiry_date: true,
      last_seen: true,
      daily_quota: true,
      monthly_quota: true,
      price: false,
      balance: false,
      address: false,
      region: false,
      building: false,
      notes: false,
      created_at: false,
      cdn_usage: false,
    }
    try {
      const saved = localStorage.getItem('subscriberColumns')
      if (saved) {
        return { ...defaultColumns, ...JSON.parse(saved) }
      }
    } catch (e) {
      console.error('Failed to load column settings:', e)
    }
    return defaultColumns
  })
  const [showColumnSettings, setShowColumnSettings] = useState(false)

  // Save column visibility to localStorage when it changes
  useEffect(() => {
    try {
      localStorage.setItem('subscriberColumns', JSON.stringify(visibleColumns))
    } catch (e) {
      console.error('Failed to save column settings:', e)
    }
  }, [visibleColumns])

  // Modal states
  const [showBulkImport, setShowBulkImport] = useState(false)
  const [importFile, setImportFile] = useState(null)
  const [importServiceId, setImportServiceId] = useState('')
  const [importResults, setImportResults] = useState(null)

  // Action modal states (for forms that need input)
  const [actionModal, setActionModal] = useState(null)
  const [actionValue, setActionValue] = useState('')
  const [actionReason, setActionReason] = useState('')
  const [changeServiceOptions, setChangeServiceOptions] = useState({
    extend_expiry: false,
    reset_fup: false,
    charge_price: false,
    prorate_price: true,
  })
  const [priceCalculation, setPriceCalculation] = useState(null)
  const [calculatingPrice, setCalculatingPrice] = useState(false)

  // Delete confirmation modal state
  const [deleteConfirm, setDeleteConfirm] = useState(null)

  // Torch modal state
  const [torchModal, setTorchModal] = useState(null)
  // Map modal state
  const [mapModal, setMapModal] = useState(null)
  const [torchData, setTorchData] = useState(null)
  const [torchLoading, setTorchLoading] = useState(false)
  const [torchAutoRefresh, setTorchAutoRefresh] = useState(true)

  // Fetch subscribers
  const { data, isLoading, refetch } = useQuery({
    queryKey: ['subscribers', page, limit, search, status, serviceId, nasId, resellerId, fupLevel, viewMode, monthlyFup, sortBy, archivedResellerId, archivedDeletedBy, archivedFromDate, archivedToDate],
    queryFn: () => {
      if (viewMode === 'archived') {
        return subscriberApi.listArchived({
          page, limit, search,
          reseller_id: archivedResellerId || undefined,
          deleted_by: archivedDeletedBy || undefined,
          from: archivedFromDate || undefined,
          to: archivedToDate || undefined,
        }).then((r) => r.data)
      }
      return subscriberApi
        .list({ page, limit, search, status, service_id: serviceId, nas_id: nasId, reseller_id: resellerId, fup_level: fupLevel, monthly_fup: monthlyFup ? 'true' : '', sort_by: sortBy })
        .then((r) => r.data)
    },
  })

  const { data: services } = useQuery({
    queryKey: ['services-list'],
    queryFn: () => serviceApi.list().then((r) => r.data.data),
  })

  const { data: nasList } = useQuery({
    queryKey: ['nas-list'],
    queryFn: () => nasApi.list().then((r) => r.data.data),
  })

  const { data: resellers } = useQuery({
    queryKey: ['resellers-list'],
    queryFn: () => resellerApi.list().then((r) => r.data.data),
  })

  // WAN check settings come from subscriber list meta (no settings.view permission needed)
  const wanCheckEnabled = data?.meta?.wan_check_enabled || false
  const wanCheckPort = parseInt(data?.meta?.wan_check_port) || 8084

  // Get selected subscribers
  const selectedSubscribers = useMemo(() => {
    const rows = data?.data || []
    return rows.filter(r => selectedIds.has(r.id))
  }, [data?.data, selectedIds])

  const selectedCount = selectedIds.size

  // Toggle row selection
  const toggleRowSelection = (id) => {
    setSelectedIds(prev => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  // Select all visible rows
  const selectAll = () => {
    const rows = data?.data || []
    if (selectedIds.size === rows.length) {
      setSelectedIds(new Set())
    } else {
      setSelectedIds(new Set(rows.map(r => r.id)))
    }
  }

  // Clear selection
  const clearSelection = () => {
    setSelectedIds(new Set())
  }

  // Single subscriber mutations
  const renewMutation = useMutation({
    mutationFn: (id) => subscriberApi.renew(id),
    onSuccess: () => {
      toast.success('Subscriber renewed successfully')
      queryClient.invalidateQueries(['subscribers'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to renew'),
  })

  const disconnectMutation = useMutation({
    mutationFn: (id) => subscriberApi.disconnect(id),
    onSuccess: () => {
      toast.success('Subscriber disconnected')
      queryClient.invalidateQueries(['subscribers'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to disconnect'),
  })

  const resetFupMutation = useMutation({
    mutationFn: (id) => subscriberApi.resetFup(id),
    onSuccess: () => {
      toast.success('FUP quota reset successfully')
      queryClient.invalidateQueries(['subscribers'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to reset FUP'),
  })

  const resetMacMutation = useMutation({
    mutationFn: ({ id, mac_address, reason }) => subscriberApi.resetMac(id, { mac_address, reason }),
    onSuccess: () => {
      toast.success('MAC address reset successfully')
      setActionModal(null)
      setActionValue('')
      setActionReason('')
      queryClient.invalidateQueries(['subscribers'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to reset MAC'),
  })

  const renameMutation = useMutation({
    mutationFn: ({ id, new_username, reason }) => subscriberApi.rename(id, { new_username, reason }),
    onSuccess: () => {
      toast.success('Username changed successfully')
      setActionModal(null)
      setActionValue('')
      setActionReason('')
      queryClient.invalidateQueries(['subscribers'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to rename'),
  })

  const addDaysMutation = useMutation({
    mutationFn: ({ id, days, reason }) => subscriberApi.addDays(id, { days, reason }),
    onSuccess: () => {
      toast.success('Days added successfully')
      setActionModal(null)
      setActionValue('')
      setActionReason('')
      queryClient.invalidateQueries(['subscribers'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to add days'),
  })

  const changeServiceMutation = useMutation({
    mutationFn: ({ id, service_id, extend_expiry, reset_fup, charge_price, prorate_price, reason }) =>
      subscriberApi.changeService(id, { service_id, extend_expiry, reset_fup, charge_price, prorate_price, reason }),
    onSuccess: (res) => {
      const data = res.data?.data
      let message = 'Service changed successfully'
      if (data?.charge_amount) {
        message += `. Charged: $${data.charge_amount.toFixed(2)}`
      }
      toast.success(message)
      setActionModal(null)
      setActionValue('')
      setActionReason('')
      setPriceCalculation(null)
      setChangeServiceOptions({ extend_expiry: false, reset_fup: false, charge_price: false, prorate_price: true })
      queryClient.invalidateQueries(['subscribers'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to change service'),
  })

  // Fetch price calculation when service is selected
  const fetchPriceCalculation = async (subscriberId, serviceId) => {
    if (!subscriberId || !serviceId) {
      setPriceCalculation(null)
      return
    }
    setCalculatingPrice(true)
    try {
      const res = await subscriberApi.calculateChangeServicePrice(subscriberId, serviceId)
      setPriceCalculation(res.data?.data)
    } catch (err) {
      console.error('Failed to calculate price:', err)
      setPriceCalculation(null)
    } finally {
      setCalculatingPrice(false)
    }
  }

  // Fetch torch data for a subscriber
  const fetchTorchData = async (subscriber) => {
    if (!subscriber || !subscriber.is_online) return
    setTorchLoading(true)
    try {
      const res = await subscriberApi.getTorch(subscriber.id, 2)
      if (res.data?.success) {
        setTorchData(res.data.data)
      } else {
        toast.error(res.data?.message || 'Failed to get torch data')
        setTorchData(null)
      }
    } catch (err) {
      toast.error(err.response?.data?.message || 'Failed to get torch data')
      setTorchData(null)
    } finally {
      setTorchLoading(false)
    }
  }

  // Auto-refresh torch data
  useEffect(() => {
    let interval
    if (torchModal && torchAutoRefresh && !torchLoading) {
      interval = setInterval(() => {
        fetchTorchData(torchModal)
      }, 2000)
    }
    return () => {
      if (interval) clearInterval(interval)
    }
  }, [torchModal, torchAutoRefresh, torchLoading])

  const activateMutation = useMutation({
    mutationFn: (id) => subscriberApi.activate(id),
    onSuccess: () => {
      toast.success('Subscriber activated')
      queryClient.invalidateQueries(['subscribers'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to activate'),
  })

  const deactivateMutation = useMutation({
    mutationFn: (id) => subscriberApi.deactivate(id),
    onSuccess: () => {
      toast.success('Subscriber deactivated')
      queryClient.invalidateQueries(['subscribers'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to deactivate'),
  })

  const addBalanceMutation = useMutation({
    mutationFn: ({ id, amount, reason }) => subscriberApi.addBalance(id, { amount, reason }),
    onSuccess: (res) => {
      const data = res.data?.data
      toast.success(data ? `Added $${data.amount.toFixed(2)} to wallet (Balance: $${data.balance_after.toFixed(2)})` : 'Balance added successfully')
      setActionModal(null)
      setActionValue('')
      setActionReason('')
      queryClient.invalidateQueries(['subscribers'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to add balance'),
  })

  const pingMutation = useMutation({
    mutationFn: (id) => subscriberApi.ping(id),
    onSuccess: (res) => {
      const data = res.data.data
      toast.custom((t) => (
        <div
          onClick={() => toast.dismiss(t.id)}
          style={{
            maxWidth: '28rem', width: '100%', background: '#fff', border: '1px solid #a0a0a0',
            borderRadius: '2px', boxShadow: '2px 2px 8px rgba(0,0,0,0.3)',
            pointerEvents: 'auto', cursor: 'pointer',
            opacity: t.visible ? 1 : 0, transition: 'opacity 0.2s',
          }}
        >
          <div style={{ padding: '4px 10px', color: '#fff', fontSize: '11px', fontWeight: 600, background: 'linear-gradient(to bottom, #4a7ab5, #2d5a87)' }}>
            Ping Result
          </div>
          <div style={{ padding: '10px' }}>
            <pre style={{ fontSize: '11px', color: '#555', whiteSpace: 'pre-wrap', fontFamily: 'monospace, monospace' }}>{data.output}</pre>
            <p style={{ marginTop: '6px', fontSize: '10px', color: '#999' }}>Click anywhere to close</p>
          </div>
        </div>
      ), { duration: 30000 })
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to ping'),
  })

  const deleteMutation = useMutation({
    mutationFn: (id) => subscriberApi.delete(id),
    onSuccess: () => {
      toast.success('Subscriber deleted')
      clearSelection()
      queryClient.invalidateQueries(['subscribers'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to delete'),
  })

  // Bulk mutations
  const bulkImportMutation = useMutation({
    mutationFn: ({ file, serviceId }) => {
      const formData = new FormData()
      formData.append('file', file)
      formData.append('service_id', serviceId)
      return subscriberApi.bulkImport(formData)
    },
    onSuccess: (res) => {
      setImportResults(res.data.data)
      toast.success(res.data.message)
      queryClient.invalidateQueries(['subscribers'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to import'),
  })

  const bulkActionMutation = useMutation({
    mutationFn: ({ ids, action }) => subscriberApi.bulkAction({ ids, action }),
    onSuccess: (res) => {
      toast.success(res.data.message)
      clearSelection()
      queryClient.invalidateQueries(['subscribers'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to perform action'),
  })

  const restoreMutation = useMutation({
    mutationFn: (id) => subscriberApi.restore(id),
    onSuccess: () => {
      toast.success('Subscriber restored')
      clearSelection()
      queryClient.invalidateQueries(['subscribers'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to restore'),
  })

  const permanentDeleteMutation = useMutation({
    mutationFn: (id) => subscriberApi.permanentDelete(id),
    onSuccess: () => {
      toast.success('Subscriber permanently deleted')
      clearSelection()
      queryClient.invalidateQueries(['subscribers'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to delete'),
  })

  const wanSkipMutation = useMutation({
    mutationFn: (id) => subscriberApi.wanCheckSkip(id),
    onSuccess: () => {
      toast.success('WAN check skipped — subscriber will reconnect with normal speed')
      queryClient.invalidateQueries(['subscribers'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to skip WAN check'),
  })

  const wanRecheckMutation = useMutation({
    mutationFn: (id) => subscriberApi.wanCheckRecheck(id),
    onSuccess: () => {
      toast.success('WAN re-check queued — will run on next cycle')
      queryClient.invalidateQueries(['subscribers'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to queue re-check'),
  })

  // Execute action on selected subscribers
  const executeAction = (action) => {
    const ids = Array.from(selectedIds)
    if (ids.length === 0) return

    if (ids.length > 1) {
      bulkActionMutation.mutate({ ids, action })
    } else {
      const id = ids[0]
      const sub = selectedSubscribers[0]
      switch (action) {
        case 'renew':
          renewMutation.mutate(id)
          break
        case 'disconnect':
          disconnectMutation.mutate(id)
          break
        case 'reset_fup':
          resetFupMutation.mutate(id)
          break
        case 'enable':
          activateMutation.mutate(id)
          break
        case 'disable':
          deactivateMutation.mutate(id)
          break
        case 'ping':
          pingMutation.mutate(id)
          break
        case 'delete':
          if (confirm('Are you sure you want to delete this subscriber?')) {
            deleteMutation.mutate(id)
          }
          break
        case 'reset_mac':
          setActionModal({ type: 'resetMac', subscriber: sub })
          setActionValue(sub?.mac_address || '')
          break
        case 'add_days':
          setActionModal({ type: 'addDays', subscriber: sub })
          break
        case 'change_service':
          setActionModal({ type: 'changeService', subscriber: sub })
          setActionValue(sub?.service_id?.toString() || '')
          break
        case 'rename':
          setActionModal({ type: 'rename', subscriber: sub })
          break
        case 'refill':
        case 'add_balance':
          setActionModal({ type: 'add_balance', subscriber: sub })
          break
      }
    }
  }

  // Execute bulk action
  const executeBulkAction = (action) => {
    const ids = Array.from(selectedIds)
    if (ids.length === 0) return

    if (action === 'delete') {
      const rows = data?.data || []
      const names = ids.map(id => {
        const sub = rows.find(r => r.id === id)
        return sub ? (sub.full_name || sub.username) : `ID ${id}`
      })
      setDeleteConfirm({ ids, names })
      return
    }

    bulkActionMutation.mutate({ ids, action })
  }

  // Handle bulk import
  const handleBulkImport = () => {
    if (!importFile || !importServiceId) {
      toast.error('Please select a file and service')
      return
    }
    bulkImportMutation.mutate({ file: importFile, serviceId: importServiceId })
  }

  // Export to CSV
  const handleExport = () => {
    const rows = data?.data || []
    if (rows.length === 0) {
      toast.error('No data to export')
      return
    }

    const headers = ['Username', 'Full Name', 'Phone', 'Email', 'Service', 'MAC Address', 'IP Address', 'Status', 'Expiry Date']
    const csvData = rows.map(r => [
      r.username,
      r.full_name,
      r.phone,
      r.email,
      r.service?.name,
      r.mac_address,
      r.ip_address || r.static_ip,
      r.status,
      r.expiry_date ? formatDate(r.expiry_date) : ''
    ])

    const csv = [headers, ...csvData].map(row => row.map(cell => `"${cell || ''}"`).join(',')).join('\n')
    const blob = new Blob([csv], { type: 'text/csv' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `subscribers_${new Date().toISOString().split('T')[0]}.csv`
    a.click()
    URL.revokeObjectURL(url)
    toast.success('Exported to CSV')
  }

  // Check if all visible rows are selected (must be before columns useMemo)
  const allSelected = (data?.data || []).length > 0 && selectedIds.size === (data?.data || []).length

  const columns = useMemo(
    () => [
      {
        id: 'checkbox',
        header: () => (
          <input
            type="checkbox"
            checked={allSelected}
            onChange={selectAll}
            style={{ width: 13, height: 13 }}
          />
        ),
        cell: ({ row }) => (
          <input
            type="checkbox"
            checked={selectedIds.has(row.original.id)}
            onChange={() => toggleRowSelection(row.original.id)}
            onClick={(e) => e.stopPropagation()}
            style={{ width: 13, height: 13 }}
          />
        ),
        size: 28,
      },
      {
        id: 'status_indicator',
        header: '',
        cell: ({ row }) => {
          const statusInfo = getStatusDisplay(row.original)
          if (sortBy === 'daily_usage' || sortBy === 'monthly_usage') {
            const rank = (page - 1) * limit + row.index + 1
            return (
              <span style={{
                fontSize: '9px', fontWeight: 'bold', padding: '1px 3px',
                borderRadius: '2px', minWidth: '18px', textAlign: 'center', display: 'inline-block',
                background: rank === 1 ? '#FFD700' : rank === 2 ? '#C0C0C0' : rank === 3 ? '#CD7F32' : '#e0e0e0',
                color: rank <= 3 ? '#333' : '#666'
              }}>#{rank}</span>
            )
          }
          return (
            <span
              style={{
                display: 'inline-block',
                width: '8px',
                height: '8px',
                borderRadius: '50%',
                backgroundColor: statusInfo.dotColor,
              }}
            />
          )
        },
        size: 28,
      },
      ...(visibleColumns.username ? [{
        accessorKey: 'username',
        header: 'Username',
        cell: ({ row }) => (
          <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
            <Link
              to={`/subscribers/${row.original.id}`}
              className="font-semibold text-[#316AC5] dark:text-[#05d1f5] hover:underline hover:text-[#1a4a7a] dark:hover:text-[#3cdffa]"
              onClick={(e) => e.stopPropagation()}
            >
              {row.original.username}
            </Link>
            {wanCheckEnabled && row.original.port_open && row.original.is_online && (
              <a
                href={`http://${row.original.ip_address || row.original.static_ip}:${wanCheckPort || 8084}`}
                target="_blank"
                rel="noopener noreferrer"
                onClick={(e) => e.stopPropagation()}
                title={`Open ${row.original.ip_address || row.original.static_ip}:${wanCheckPort || 8084}`}
              >
                <ServerIcon style={{ width: 14, height: 14, color: '#4CAF50', flexShrink: 0, cursor: 'pointer' }} />
              </a>
            )}
            {wanCheckEnabled && row.original.wan_check_status === 'failed' && (
              <ShieldExclamationIcon style={{ width: 14, height: 14, color: '#EF4444', flexShrink: 0 }} title="WAN check failed — blocked" />
            )}
            {wanCheckEnabled && row.original.wan_check_status === 'unchecked' && row.original.is_online && (
              <ShieldExclamationIcon style={{ width: 14, height: 14, color: '#F59E0B', flexShrink: 0 }} title="WAN check pending" />
            )}
            {row.original.is_online && hasPermission('subscribers.torch') && (
              <button
                onClick={(e) => {
                  e.stopPropagation()
                  setTorchModal(row.original)
                  setTorchData(null)
                  fetchTorchData(row.original)
                }}
                style={{ color: '#4CAF50', background: 'none', border: 'none', cursor: 'pointer', padding: 0 }}
                title="Live Traffic (Torch)"
              >
                <SignalIcon style={{ width: 14, height: 14 }} />
              </button>
            )}
            {!!(row.original.latitude && row.original.longitude) && parseFloat(row.original.latitude) !== 0 && parseFloat(row.original.longitude) !== 0 && (
              <button
                onClick={(e) => { e.stopPropagation(); setMapModal(row.original) }}
                style={{ color: '#2196F3', background: 'none', border: 'none', cursor: 'pointer', padding: 0 }}
                title="View Location"
              >
                <MapPinIcon style={{ width: 14, height: 14 }} />
              </button>
            )}
            {row.original.fup_level > 0 && (
              <span className={`fup-badge-${Math.min(row.original.fup_level, 6)}`}>
                FUP{row.original.fup_level}
              </span>
            )}
            {row.original.cdn_fup_level > 0 && (
              <span style={{ fontSize: 8, fontWeight: 700, padding: '1px 4px', borderRadius: 3, backgroundColor: '#ff6d00', color: '#fff' }}>
                CDN{row.original.cdn_fup_level}
              </span>
            )}
            {sortBy === 'daily_usage' && (() => {
              const bytes = (row.original.daily_download_used || 0) + (row.original.daily_upload_used || 0)
              const gb = bytes / 1073741824
              const label = gb >= 1 ? `${gb.toFixed(1)} GB` : `${(bytes / 1048576).toFixed(0)} MB`
              return <span className="badge-cyan">{label}</span>
            })()}
            {sortBy === 'monthly_usage' && (() => {
              const bytes = (row.original.monthly_download_used || 0) + (row.original.monthly_upload_used || 0)
              const gb = bytes / 1073741824
              const label = gb >= 1 ? `${gb.toFixed(1)} GB` : `${(bytes / 1048576).toFixed(0)} MB`
              return <span className="badge-indigo">{label}</span>
            })()}
          </div>
        ),
      }] : []),
      ...(visibleColumns.full_name ? [{
        accessorKey: 'full_name',
        header: 'Fullname',
      }] : []),
      ...(visibleColumns.phone ? [{
        accessorKey: 'phone',
        header: 'Phone',
      }] : []),
      ...(visibleColumns.mac_address ? [{
        accessorKey: 'mac_address',
        header: 'MAC',
        cell: ({ row }) => (
          <div style={{ display: 'flex', alignItems: 'center', gap: '3px' }}>
            <span style={{ fontFamily: 'monospace', fontSize: '10px' }}>{row.original.mac_address || 'N/A'}</span>
            {row.original.mac_address && (
              <button
                onClick={(e) => {
                  e.stopPropagation()
                  setActionModal({ type: 'resetMac', subscriber: row.original })
                  setActionValue(row.original.mac_address)
                }}
                style={{ color: '#999', background: 'none', border: 'none', cursor: 'pointer', padding: 0 }}
                title="Reset MAC"
              >
                <ArrowPathIcon style={{ width: 12, height: 12 }} />
              </button>
            )}
          </div>
        ),
      }] : []),
      ...(visibleColumns.ip_address ? [{
        accessorKey: 'ip_address',
        header: 'IP',
        cell: ({ row }) => (
          <span style={{
            fontFamily: 'monospace',
            fontSize: '10px',
            color: row.original.is_online ? '#2e7d32' : '#999',
            fontWeight: row.original.is_online ? 600 : 400
          }}>
            {row.original.ip_address || row.original.static_ip || 'N/A'}
          </span>
        ),
      }] : []),
      ...(visibleColumns.service ? [{
        accessorKey: 'service.name',
        header: 'Service',
        cell: ({ row }) => {
          const serviceName = row.original.service?.name || '-'
          if (visibleColumns.daily_quota) {
            return <span style={{ fontWeight: 500 }}>{serviceName}</span>
          }
          const used = row.original.daily_quota_used || 0
          const svcLimit = row.original.service?.daily_quota || 0
          if (svcLimit === 0) {
            return <span style={{ fontWeight: 500 }}>{serviceName}</span>
          }
          const percent = Math.min(100, (used / svcLimit) * 100)
          return (
            <div style={{ minWidth: '90px' }}>
              <div style={{ fontWeight: 500, fontSize: '10px', textAlign: 'center' }}>{serviceName}</div>
              <div className="wb-usage-bar" style={{ marginTop: '2px' }}>
                <div
                  className="wb-usage-bar-fill"
                  style={{
                    width: `${percent}%`,
                    backgroundColor: percent >= 80 ? '#f44336' : percent >= 50 ? '#FF9800' : '#4CAF50',
                  }}
                />
              </div>
              <div style={{ fontSize: '9px', textAlign: 'center', color: '#666', marginTop: '1px' }}>{formatBytes(used)}</div>
            </div>
          )
        },
      }] : []),
      ...(visibleColumns.daily_quota && viewMode !== 'archived' ? [{
        id: 'daily_quota',
        header: () => (
          <button
            onClick={() => {
              setSortBy(sortBy === 'daily_usage' ? '' : 'daily_usage')
              setSorting([])
              setMonthlyFup(false)
              setFupLevel('')
              setPage(1)
            }}
            style={{
              background: 'none', border: 'none', cursor: 'pointer', padding: 0,
              fontWeight: 600, fontSize: '10px', whiteSpace: 'nowrap',
              color: sortBy === 'daily_usage' ? '#00838f' : 'inherit'
            }}
          >
            Top Daily {sortBy === 'daily_usage' ? '\u25BC' : '\u2195'}
          </button>
        ),
        cell: ({ row }) => {
          const used = (row.original.daily_download_used || 0) + (row.original.daily_upload_used || 0)
          const svcLimit = row.original.service?.daily_quota || 0
          if (used === 0 && svcLimit === 0) return <span style={{ color: '#bbb', fontSize: '9px' }}>&mdash;</span>
          const percent = svcLimit > 0 ? Math.min(100, (used / svcLimit) * 100) : 0
          return (
            <div style={{ minWidth: '70px' }}>
              <div style={{ fontSize: '9px', fontWeight: 600, textAlign: 'center', color: '#00838f', marginBottom: '2px' }}>{formatBytes(used)}</div>
              {svcLimit > 0 && (
                <div className="wb-usage-bar">
                  <div
                    className="wb-usage-bar-fill"
                    style={{
                      width: `${percent}%`,
                      backgroundColor: percent >= 80 ? '#f44336' : percent >= 50 ? '#FF9800' : '#4CAF50',
                    }}
                  />
                </div>
              )}
            </div>
          )
        },
      }] : []),
      ...(visibleColumns.reseller ? [{
        id: 'reseller',
        header: 'Reseller',
        cell: ({ row }) => {
          const reseller = row.original.reseller
          if (!reseller) return <span style={{ color: '#bbb' }}>-</span>
          return (
            <div style={{ fontSize: '10px' }}>
              <div style={{ fontWeight: 500 }}>{reseller.user?.username || reseller.name}</div>
              {reseller.parent && (
                <div style={{ color: '#999', fontSize: '9px' }}>
                  Sub: {reseller.parent.user?.username || reseller.parent.name}
                </div>
              )}
            </div>
          )
        },
      }] : []),
      ...(visibleColumns.status && viewMode !== 'archived' ? [{
        accessorKey: 'status',
        header: 'Status',
        cell: ({ row }) => {
          const statusInfo = getStatusDisplay(row.original)
          return (
            <span style={{ fontSize: '10px', fontWeight: 500, color: statusInfo.textColor }}>
              {statusInfo.text}
            </span>
          )
        },
      }] : []),
      ...(visibleColumns.expiry_date && viewMode !== 'archived' ? [{
        accessorKey: 'expiry_date',
        header: 'Expiry',
        cell: ({ row }) => {
          if (!row.original.expiry_date) return '-'
          const expiry = new Date(row.original.expiry_date)
          const isExpired = expiry < new Date()
          const daysLeft = Math.ceil((expiry - new Date()) / (1000 * 60 * 60 * 24))
          return (
            <div>
              <div style={{ fontSize: '10px', color: isExpired ? '#c62828' : '#333' }}>
                {formatDate(row.original.expiry_date)}
              </div>
              <div style={{ fontSize: '9px', color: isExpired ? '#e53935' : '#999' }}>
                {isExpired ? `${Math.abs(daysLeft)}d ago` : `${daysLeft}d left`}
              </div>
            </div>
          )
        },
      }] : []),
      ...(visibleColumns.last_seen && viewMode !== 'archived' ? [{
        accessorKey: 'last_seen',
        header: 'Last Seen',
        cell: ({ row }) => {
          if (row.original.is_online) {
            return <span style={{ color: '#2e7d32', fontSize: '10px', fontWeight: 600 }}>Online Now</span>
          }
          if (!row.original.last_seen) {
            return <span style={{ color: '#bbb', fontSize: '10px' }}>Never</span>
          }
          const lastSeen = new Date(row.original.last_seen)
          const now = new Date()
          const diffMs = now - lastSeen
          const diffMins = Math.floor(diffMs / (1000 * 60))
          const diffHours = Math.floor(diffMs / (1000 * 60 * 60))
          const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))

          let timeAgo
          if (diffMins < 1) timeAgo = 'Just now'
          else if (diffMins < 60) timeAgo = `${diffMins}m ago`
          else if (diffHours < 24) timeAgo = `${diffHours}h ago`
          else if (diffDays < 30) timeAgo = `${diffDays}d ago`
          else timeAgo = formatDate(row.original.last_seen)

          return <span style={{ fontSize: '10px', color: '#666' }}>{timeAgo}</span>
        },
      }] : []),
      ...(visibleColumns.monthly_quota && viewMode !== 'archived' ? [{
        id: 'monthly_quota',
        header: 'Monthly',
        cell: ({ row }) => {
          const used = row.original.monthly_quota_used || 0
          const svcLimit = row.original.service?.monthly_quota || 0
          if (svcLimit === 0) return <span style={{ color: '#bbb', fontSize: '9px' }}>Unlimited</span>
          const percent = Math.min(100, (used / svcLimit) * 100)
          return (
            <div style={{ minWidth: '70px' }}>
              <div style={{ fontSize: '9px', fontWeight: 500, textAlign: 'center', marginBottom: '2px' }}>{formatBytes(used)}</div>
              <div className="wb-usage-bar">
                <div
                  className="wb-usage-bar-fill"
                  style={{
                    width: `${percent}%`,
                    backgroundColor: percent >= 80 ? '#f44336' : percent >= 50 ? '#FF9800' : '#4CAF50',
                  }}
                />
              </div>
            </div>
          )
        },
      }] : []),
      ...(visibleColumns.cdn_usage && viewMode !== 'archived' ? [{
        id: 'cdn_usage',
        header: 'CDN Usage',
        cell: ({ row }) => {
          const used = row.original.cdn_daily_download_used || 0
          const cdnFup = row.original.cdn_fup_level || 0
          if (used === 0 && cdnFup === 0) return <span style={{ color: '#bbb', fontSize: '9px' }}>&mdash;</span>
          return (
            <div style={{ minWidth: '60px', textAlign: 'center' }}>
              <div style={{ fontSize: '9px', fontWeight: 500 }}>{formatBytes(used)}</div>
              {cdnFup > 0 && <span style={{ fontSize: 8, fontWeight: 700, padding: '0px 3px', borderRadius: 3, backgroundColor: '#ff6d00', color: '#fff' }}>CDN {cdnFup}</span>}
            </div>
          )
        },
      }] : []),
      ...(visibleColumns.price ? [{
        accessorKey: 'price',
        header: 'Price',
        cell: ({ row }) => {
          const sub = row.original
          const price = sub.override_price ? sub.price : sub.service?.price
          return (
            <span style={{ fontWeight: 500 }}>
              ${(price || 0).toFixed(2)}
              {sub.override_price && <span style={{ color: '#FF9800', marginLeft: '2px', fontSize: '10px' }} title="Custom price">*</span>}
            </span>
          )
        },
      }] : []),
      ...(visibleColumns.balance ? [{
        accessorKey: 'balance',
        header: 'Balance',
        cell: ({ row }) => {
          const bal = row.original.balance || 0
          return (
            <span style={{ fontWeight: 500, color: bal > 0 ? '#059669' : '#6b7280' }}>${bal.toFixed(2)}</span>
          )
        },
      }] : []),
      ...(visibleColumns.address ? [{
        accessorKey: 'address',
        header: 'Address',
      }] : []),
      ...(visibleColumns.region ? [{
        accessorKey: 'region',
        header: 'Region',
      }] : []),
      ...(visibleColumns.building ? [{
        accessorKey: 'building',
        header: 'Building',
      }] : []),
      ...(visibleColumns.notes ? [{
        accessorKey: 'note',
        header: 'Notes',
        cell: ({ row }) => (
          <span style={{ fontSize: '10px', color: '#666', maxWidth: '120px', overflow: 'hidden', textOverflow: 'ellipsis', display: 'block' }} title={row.original.note}>
            {row.original.note || '-'}
          </span>
        ),
      }] : []),
      ...(visibleColumns.created_at ? [{
        accessorKey: 'created_at',
        header: 'Created At',
        cell: ({ row }) => (
          <span style={{ fontSize: '10px', color: '#666', whiteSpace: 'nowrap' }}>
            {row.original.created_at ? formatDate(row.original.created_at) : '-'}
          </span>
        ),
      }] : []),
      ...(viewMode === 'archived' ? [{
        accessorKey: 'deleted_by_name',
        header: 'Deleted By',
        cell: ({ row }) => (
          <span style={{ fontSize: '10px', whiteSpace: 'nowrap' }}>
            {row.original.deleted_by_name || <span style={{ color: '#999' }}>—</span>}
          </span>
        ),
      }, {
        accessorKey: 'deleted_at',
        header: 'Deleted At',
        cell: ({ row }) => (
          <span style={{ fontSize: '10px', color: '#666', whiteSpace: 'nowrap' }}>
            {row.original.deleted_at ? formatDate(row.original.deleted_at) : '—'}
          </span>
        ),
      }] : []),
    ],
    [viewMode, visibleColumns, sortBy, page, limit, selectedIds, allSelected, wanCheckEnabled, wanCheckPort]
  )

  const table = useReactTable({
    data: data?.data || [],
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    manualPagination: true,
    pageCount: Math.ceil((data?.meta?.total || 0) / limit),
  })

  const totalPages = Math.ceil((data?.meta?.total || 0) / limit)
  const stats = data?.stats || {}

  return (
    <div>
      {/* === Tabs: Active / Archived === */}
      <div style={{ display: 'flex', alignItems: 'flex-end', gap: '2px', marginBottom: '-1px', position: 'relative', zIndex: 1 }}>
        <button
          onClick={() => { setViewMode('active'); setPage(1); clearSelection(); }}
          className={viewMode === 'active' ? 'wb-tab active' : 'wb-tab'}
        >
          Active
        </button>
        <button
          onClick={() => { setViewMode('archived'); setPage(1); clearSelection(); }}
          className={viewMode === 'archived' ? 'wb-tab active' : 'wb-tab'}
          style={{ display: 'flex', alignItems: 'center', gap: '3px' }}
        >
          <ArchiveBoxIcon style={{ width: 12, height: 12 }} />
          Archived
        </button>
      </div>

      {/* === Main panel === */}
      <div className="card" style={{ borderTop: '1px solid #a0a0a0' }}>

        {/* === Archived Stats + Filters === */}
        {viewMode === 'archived' && (
          <>
            {/* Stats cards — clickable to filter by date */}
            <div className="flex items-center gap-3 px-3 py-2 border-b border-[#ccc] dark:border-[#374151] bg-[#fafafa] dark:bg-[#1f2937] flex-wrap text-[11px]">
              <div
                className={`flex items-center gap-1.5 px-2 py-1 rounded cursor-pointer border ${archivedFromDate === new Date().toISOString().split('T')[0] && !archivedToDate ? 'bg-red-200 dark:bg-red-800/50 border-red-400 dark:border-red-600' : 'bg-red-50 dark:bg-red-900/30 border-red-200 dark:border-red-800 hover:bg-red-100 dark:hover:bg-red-900/50'}`}
                onClick={() => {
                  const today = new Date().toISOString().split('T')[0]
                  if (archivedFromDate === today && !archivedToDate) { setArchivedFromDate(''); } else { setArchivedFromDate(today); setArchivedToDate(''); }
                  setPage(1)
                }}
              >
                <TrashIcon style={{ width: 12, height: 12, color: '#dc2626' }} />
                <strong className="text-red-600 dark:text-red-400">{stats.deleted_today || 0}</strong>
                <span className="text-[#666] dark:text-gray-400">Today</span>
              </div>
              <div
                className={`flex items-center gap-1.5 px-2 py-1 rounded cursor-pointer border ${(() => { const d = new Date(); d.setDate(d.getDate() - d.getDay()); return archivedFromDate === d.toISOString().split('T')[0] && !archivedToDate })() ? 'bg-orange-200 dark:bg-orange-800/50 border-orange-400 dark:border-orange-600' : 'bg-orange-50 dark:bg-orange-900/30 border-orange-200 dark:border-orange-800 hover:bg-orange-100 dark:hover:bg-orange-900/50'}`}
                onClick={() => {
                  const d = new Date(); d.setDate(d.getDate() - d.getDay())
                  const weekStart = d.toISOString().split('T')[0]
                  if (archivedFromDate === weekStart && !archivedToDate) { setArchivedFromDate(''); } else { setArchivedFromDate(weekStart); setArchivedToDate(''); }
                  setPage(1)
                }}
              >
                <CalendarDaysIcon style={{ width: 12, height: 12, color: '#ea580c' }} />
                <strong className="text-orange-600 dark:text-orange-400">{stats.deleted_this_week || 0}</strong>
                <span className="text-[#666] dark:text-gray-400">This Week</span>
              </div>
              <div
                className={`flex items-center gap-1.5 px-2 py-1 rounded cursor-pointer border ${(() => { const d = new Date(); return archivedFromDate === `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,'0')}-01` && !archivedToDate })() ? 'bg-blue-200 dark:bg-blue-800/50 border-blue-400 dark:border-blue-600' : 'bg-blue-50 dark:bg-blue-900/30 border-blue-200 dark:border-blue-800 hover:bg-blue-100 dark:hover:bg-blue-900/50'}`}
                onClick={() => {
                  const d = new Date()
                  const monthStart = `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,'0')}-01`
                  if (archivedFromDate === monthStart && !archivedToDate) { setArchivedFromDate(''); } else { setArchivedFromDate(monthStart); setArchivedToDate(''); }
                  setPage(1)
                }}
              >
                <CalendarDaysIcon style={{ width: 12, height: 12, color: '#2563eb' }} />
                <strong className="text-blue-600 dark:text-blue-400">{stats.deleted_this_month || 0}</strong>
                <span className="text-[#666] dark:text-gray-400">This Month</span>
              </div>
              {stats.top_deleters && stats.top_deleters.length > 0 && (
                <>
                  <span className="w-px h-4 bg-[#ccc] dark:bg-[#4b5563]" />
                  <span className="text-[10px] text-[#888] dark:text-gray-500">Top:</span>
                  {stats.top_deleters.slice(0, 5).map((d, i) => (
                    <span key={i} className="text-[10px] px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300">
                      {d.name} <strong>({d.count})</strong>
                    </span>
                  ))}
                </>
              )}
              <div className="ml-auto flex items-center gap-1">
                <strong className="text-[#333] dark:text-gray-200">{data?.meta?.total || 0}</strong>
                <span className="text-[#666] dark:text-gray-400">Total Archived</span>
              </div>
            </div>
            {/* Filters row */}
            <div className="flex items-center gap-2 px-3 py-1.5 border-b border-[#ccc] dark:border-[#374151] bg-white dark:bg-[#111827] flex-wrap text-[11px]">
              {resellers && resellers.length > 0 && (
                <select
                  value={archivedResellerId}
                  onChange={(e) => { setArchivedResellerId(e.target.value); setPage(1); }}
                  className="text-[11px] px-1.5 py-1 border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 dark:text-white"
                  style={{ maxWidth: 140 }}
                >
                  <option value="">All Resellers</option>
                  {resellers.map((r) => (
                    <option key={r.id} value={r.id}>{r.user?.username || r.company_name || `Reseller #${r.id}`}</option>
                  ))}
                </select>
              )}
              <select
                value={archivedDeletedBy}
                onChange={(e) => { setArchivedDeletedBy(e.target.value); setPage(1); }}
                className="text-[11px] px-1.5 py-1 border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 dark:text-white"
                style={{ maxWidth: 140 }}
              >
                <option value="">Deleted By: All</option>
                {stats.top_deleters && stats.top_deleters.map((d, i) => (
                  <option key={i} value={d.deleted_by_id}>{d.name} ({d.count})</option>
                ))}
              </select>
              <input
                type="date"
                value={archivedFromDate}
                onChange={(e) => { setArchivedFromDate(e.target.value); setPage(1); }}
                className="text-[11px] px-1.5 py-1 border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 dark:text-white"
                placeholder="From"
                title="From date"
              />
              <input
                type="date"
                value={archivedToDate}
                onChange={(e) => { setArchivedToDate(e.target.value); setPage(1); }}
                className="text-[11px] px-1.5 py-1 border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 dark:text-white"
                placeholder="To"
                title="To date"
              />
              {(archivedResellerId || archivedDeletedBy || archivedFromDate || archivedToDate) && (
                <button
                  onClick={() => { setArchivedResellerId(''); setArchivedDeletedBy(''); setArchivedFromDate(''); setArchivedToDate(''); setPage(1); }}
                  className="text-[10px] px-1.5 py-1 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded"
                >
                  Clear Filters
                </button>
              )}
            </div>
          </>
        )}

        {/* === Stats Row (Active view) === */}
        {viewMode === 'active' && <div className="flex items-center gap-3 px-2 py-1 border-b border-[#ccc] dark:border-[#374151] bg-[#f8f8f8] dark:bg-[#1f2937] flex-wrap text-[11px]">
          <div
            className={`flex items-center gap-1 cursor-pointer px-1.5 py-0.5 rounded-sm ${status === 'online' ? 'bg-[#e8f5e9] dark:bg-green-900/40' : ''}`}
            onClick={() => { setStatus(status === 'online' ? '' : 'online'); setPage(1); }}
          >
            <span className="wb-status-dot" style={{ backgroundColor: '#4CAF50' }} />
            <strong className="text-[#2e7d32] dark:text-green-400">{stats.online || 0}</strong>
            <span className="text-[#666] dark:text-gray-400">Online</span>
          </div>
          <div
            className={`flex items-center gap-1 cursor-pointer px-1.5 py-0.5 rounded-sm ${status === 'offline' ? 'bg-[#ffebee] dark:bg-red-900/40' : ''}`}
            onClick={() => { setStatus(status === 'offline' ? '' : 'offline'); setPage(1); }}
          >
            <span className="wb-status-dot" style={{ backgroundColor: '#f44336' }} />
            <strong className="text-[#c62828] dark:text-red-400">{stats.offline || 0}</strong>
            <span className="text-[#666] dark:text-gray-400">Offline</span>
          </div>
          <div
            className={`flex items-center gap-1 cursor-pointer px-1.5 py-0.5 rounded-sm ${status === 'expired' ? 'bg-[#fff3e0] dark:bg-orange-900/40' : ''}`}
            onClick={() => { setStatus(status === 'expired' ? '' : 'expired'); setPage(1); }}
          >
            <span className="wb-status-dot" style={{ backgroundColor: '#FF9800' }} />
            <strong className="text-[#e65100] dark:text-orange-400">{stats.expired || 0}</strong>
            <span className="text-[#666] dark:text-gray-400">Expired</span>
          </div>

          <span className="w-px h-3.5 bg-[#ccc] dark:bg-[#4b5563]" />

          <div
            className={`flex items-center gap-1 cursor-pointer px-1.5 py-0.5 rounded-sm ${fupLevel === '1' ? 'bg-[#e3f2fd] dark:bg-blue-900/40' : ''}`}
            onClick={() => { setFupLevel(fupLevel === '1' ? '' : '1'); setPage(1); }}
          >
            <span className="fup-badge-1">FUP1</span>
            <strong className="text-[#1565c0] dark:text-blue-400">{stats.fup1 || 0}</strong>
          </div>
          <div
            className={`flex items-center gap-1 cursor-pointer px-1.5 py-0.5 rounded-sm ${fupLevel === '2' ? 'bg-[#fff3e0] dark:bg-orange-900/40' : ''}`}
            onClick={() => { setFupLevel(fupLevel === '2' ? '' : '2'); setPage(1); }}
          >
            <span className="fup-badge-2">FUP2</span>
            <strong className="text-[#e65100] dark:text-orange-400">{stats.fup2 || 0}</strong>
          </div>
          <div
            className={`flex items-center gap-1 cursor-pointer px-1.5 py-0.5 rounded-sm ${fupLevel === '3' ? 'bg-[#ffebee] dark:bg-red-900/40' : ''}`}
            onClick={() => { setFupLevel(fupLevel === '3' ? '' : '3'); setPage(1); }}
          >
            <span className="fup-badge-3">FUP3</span>
            <strong className="text-[#c62828] dark:text-red-400">{stats.fup3 || 0}</strong>
          </div>
          {stats.fup4 > 0 && (
            <div
              className={`flex items-center gap-1 cursor-pointer px-1.5 py-0.5 rounded-sm ${fupLevel === '4' ? 'bg-[#f3e5f5] dark:bg-purple-900/40' : ''}`}
              onClick={() => { setFupLevel(fupLevel === '4' ? '' : '4'); setPage(1); }}
            >
              <span className="fup-badge-4">FUP4</span>
              <strong className="text-[#00897b] dark:text-teal-400">{stats.fup4 || 0}</strong>
            </div>
          )}
          {stats.fup5 > 0 && (
            <div
              className={`flex items-center gap-1 cursor-pointer px-1.5 py-0.5 rounded-sm ${fupLevel === '5' ? 'bg-[#e8eaf6] dark:bg-indigo-900/40' : ''}`}
              onClick={() => { setFupLevel(fupLevel === '5' ? '' : '5'); setPage(1); }}
            >
              <span className="fup-badge-5">FUP5</span>
              <strong className="text-[#3949ab] dark:text-indigo-400">{stats.fup5 || 0}</strong>
            </div>
          )}
          {stats.fup6 > 0 && (
            <div
              className={`flex items-center gap-1 cursor-pointer px-1.5 py-0.5 rounded-sm ${fupLevel === '6' ? 'bg-[#eceff1] dark:bg-gray-700/40' : ''}`}
              onClick={() => { setFupLevel(fupLevel === '6' ? '' : '6'); setPage(1); }}
            >
              <span className="fup-badge-6">FUP6</span>
              <strong className="text-[#455a64] dark:text-gray-400">{stats.fup6 || 0}</strong>
            </div>
          )}

          <span className="w-px h-3.5 bg-[#ccc] dark:bg-[#4b5563]" />

          <div
            className={`flex items-center gap-1 cursor-pointer px-1.5 py-0.5 rounded-sm ${monthlyFup ? 'bg-[#fce4ec] dark:bg-pink-900/40' : ''}`}
            onClick={() => { setMonthlyFup(!monthlyFup); setFupLevel(''); setSortBy(''); setPage(1); }}
          >
            <span className="text-[10px] text-[#666] dark:text-gray-400">Monthly FUP:</span>
            <strong className="text-[#c2185b] dark:text-pink-400">{stats.monthly_fup || 0}</strong>
          </div>

          {stats.cdn_fup > 0 && (
            <div
              className={`flex items-center gap-1 cursor-pointer px-1.5 py-0.5 rounded-sm ${sortBy === 'cdn_fup' ? 'bg-[#fff3e0] dark:bg-orange-900/40' : ''}`}
              onClick={() => { setSortBy(sortBy === 'cdn_fup' ? '' : 'cdn_fup'); setSorting([]); setMonthlyFup(false); setFupLevel(''); setPage(1); }}
            >
              <span style={{ fontSize: 9, fontWeight: 700, padding: '1px 4px', borderRadius: 3, backgroundColor: '#ff6d00', color: '#fff' }}>CDN</span>
              <strong className="text-[#e65100] dark:text-orange-400">{stats.cdn_fup || 0}</strong>
            </div>
          )}

          <div
            className={`flex items-center gap-1 cursor-pointer px-1.5 py-0.5 rounded-sm ${sortBy === 'daily_usage' ? 'bg-[#e0f7fa] dark:bg-cyan-900/40' : ''}`}
            onClick={() => { setSortBy(sortBy === 'daily_usage' ? '' : 'daily_usage'); setSorting([]); setMonthlyFup(false); setFupLevel(''); setPage(1); }}
          >
            <span className="text-[10px] text-[#666] dark:text-gray-400">Top Daily</span>
            {sortBy === 'daily_usage' && <span className="text-[#00838f] dark:text-cyan-400 text-[10px]">{'\u25BC'}</span>}
          </div>

          <div
            className={`flex items-center gap-1 cursor-pointer px-1.5 py-0.5 rounded-sm ${sortBy === 'monthly_usage' ? 'bg-[#e8eaf6] dark:bg-indigo-900/40' : ''}`}
            onClick={() => { setSortBy(sortBy === 'monthly_usage' ? '' : 'monthly_usage'); setSorting([]); setMonthlyFup(false); setFupLevel(''); setPage(1); }}
          >
            <span className="text-[10px] text-[#666] dark:text-gray-400">Top Monthly</span>
            {sortBy === 'monthly_usage' && <span className="text-[#283593] dark:text-indigo-400 text-[10px]">{'\u25BC'}</span>}
          </div>

          <div className="ml-auto flex items-center gap-1">
            <strong className="text-[#333] dark:text-gray-200">{data?.meta?.total || 0}</strong>
            <span className="text-[#666] dark:text-gray-400">Total</span>
          </div>
        </div>}

        {/* === Toolbar === */}
        <div className="wb-toolbar" style={{ flexWrap: 'wrap', position: 'sticky', top: 0, zIndex: 20 }}>
          {/* Select All */}
          <button
            onClick={selectAll}
            className={allSelected ? 'btn btn-primary btn-sm' : 'btn btn-sm'}
            title={allSelected ? 'Deselect All' : 'Select All'}
          >
            <Squares2X2Icon style={{ width: 14, height: 14, marginRight: 2 }} />
            <span className="hide-mobile">{allSelected ? 'Deselect' : 'Select All'}</span>
          </button>

          <span className="wb-toolbar-separator hide-mobile" />

          {hasPermission('subscribers.renew') && (
            <button onClick={() => executeBulkAction('renew')} disabled={selectedCount === 0} className="btn btn-sm" title="Renew">
              <ClockIcon style={{ width: 14, height: 14, marginRight: 2 }} />
              <span className="hide-mobile">Renew</span>
            </button>
          )}
          {hasPermission('subscribers.inactivate') && (
            <button onClick={() => executeBulkAction('enable')} disabled={selectedCount === 0} className="btn btn-sm" title="Activate">
              <PlayIcon style={{ width: 14, height: 14, marginRight: 2 }} />
              <span className="hide-mobile">Activate</span>
            </button>
          )}
          {hasPermission('subscribers.inactivate') && (
            <button onClick={() => executeBulkAction('disable')} disabled={selectedCount === 0} className="btn btn-sm" title="Deactivate">
              <PauseIcon style={{ width: 14, height: 14, marginRight: 2 }} />
              <span className="hide-mobile">Deactivate</span>
            </button>
          )}
          {hasPermission('subscribers.disconnect') && (
            <button onClick={() => executeBulkAction('disconnect')} disabled={selectedCount === 0} className="btn btn-sm" title="Disconnect">
              <XCircleIcon style={{ width: 14, height: 14, marginRight: 2 }} />
              <span className="hide-mobile">Disconnect</span>
            </button>
          )}
          {hasPermission('subscribers.reset_fup') && (
            <button onClick={() => executeBulkAction('reset_fup')} disabled={selectedCount === 0} className="btn btn-sm" title="Reset FUP">
              <ArrowPathIcon style={{ width: 14, height: 14, marginRight: 2 }} />
              <span className="hide-mobile">Reset FUP</span>
            </button>
          )}

          <span className="wb-toolbar-separator hide-mobile" />

          {hasPermission('subscribers.add_days') && (
            <button onClick={() => executeAction('add_days')} disabled={selectedCount !== 1} className="btn btn-sm" title="Add Days">
              <CalendarDaysIcon style={{ width: 14, height: 14, marginRight: 2 }} />
              <span className="hide-mobile">Add Days</span>
            </button>
          )}
          {hasPermission('subscribers.change_service') && (
            <button onClick={() => executeAction('change_service')} disabled={selectedCount !== 1} className="btn btn-sm" title="Change Service">
              <ArrowsRightLeftIcon style={{ width: 14, height: 14, marginRight: 2 }} />
              <span className="hide-mobile">Change Service</span>
            </button>
          )}
          {hasPermission('subscribers.refill_quota') && (
            <button onClick={() => executeAction('add_balance')} disabled={selectedCount !== 1} className="btn btn-sm" title="Add Balance" style={{ color: '#059669' }}>
              <BanknotesIcon style={{ width: 14, height: 14, marginRight: 2 }} />
              <span className="hide-mobile">Add Balance</span>
            </button>
          )}
          {hasPermission('subscribers.ping') && (
            <button onClick={() => executeAction('ping')} disabled={selectedCount !== 1} className="btn btn-sm" title="Ping">
              <WifiIcon style={{ width: 14, height: 14, marginRight: 2 }} />
              <span className="hide-mobile">Ping</span>
            </button>
          )}
          {isAdmin() && selectedCount === 1 && selectedSubscribers[0]?.wan_check_status === 'failed' && (
            <>
              <button onClick={() => wanSkipMutation.mutate(selectedSubscribers[0].id)} className="btn btn-sm" title="Skip WAN Check" style={{ color: '#F59E0B' }}>
                <ShieldExclamationIcon style={{ width: 14, height: 14, marginRight: 2 }} />
                <span className="hide-mobile">Skip WAN</span>
              </button>
              <button onClick={() => wanRecheckMutation.mutate(selectedSubscribers[0].id)} className="btn btn-sm" title="Re-check WAN">
                <ArrowPathIcon style={{ width: 14, height: 14, marginRight: 2 }} />
                <span className="hide-mobile">Re-check</span>
              </button>
            </>
          )}

          <span className="wb-toolbar-separator hide-mobile" />

          {hasPermission('subscribers.delete') && (
            <button onClick={() => executeBulkAction('delete')} disabled={selectedCount === 0} className="btn btn-danger btn-sm" title="Delete">
              <TrashIcon style={{ width: 14, height: 14, marginRight: 2 }} />
              <span className="hide-mobile">Delete</span>
            </button>
          )}

          {/* Right side: search + status + buttons */}
          <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '4px', flexWrap: 'wrap' }}>
            {selectedCount > 0 && (
              <span style={{ fontSize: '11px', fontWeight: 600, color: '#316AC5', marginRight: '4px' }}>
                {selectedCount} selected
                <button onClick={clearSelection} style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#999', marginLeft: '3px' }}>
                  <XMarkIcon style={{ width: 12, height: 12 }} />
                </button>
              </span>
            )}

            {viewMode === 'active' && (
              <select
                value={status}
                onChange={(e) => { setStatus(e.target.value); setPage(1); }}
                className="input input-sm"
                style={{ width: '110px' }}
              >
                {statusFilters.map((f) => (
                  <option key={f.value} value={f.value}>{f.label}</option>
                ))}
              </select>
            )}

            <div style={{ position: 'relative' }}>
              <MagnifyingGlassIcon style={{ position: 'absolute', left: 4, top: '50%', transform: 'translateY(-50%)', width: 13, height: 13, color: '#999' }} />
              <input
                type="text"
                placeholder="Search..."
                value={search}
                onChange={(e) => { setSearch(e.target.value); setPage(1) }}
                className="input input-sm"
                style={{ paddingLeft: '20px', width: '140px' }}
              />
            </div>

            {viewMode === 'active' && (
              <button
                onClick={() => setShowFilters(!showFilters)}
                className={showFilters ? 'btn btn-primary btn-sm' : 'btn btn-sm'}
                title="Filters"
              >
                <FunnelIcon style={{ width: 13, height: 13 }} />
              </button>
            )}
            <button
              onClick={() => setShowColumnSettings(!showColumnSettings)}
              className={showColumnSettings ? 'btn btn-primary btn-sm' : 'btn btn-sm'}
              title="Columns"
            >
              <EyeIcon style={{ width: 13, height: 13 }} />
            </button>
            <button onClick={() => refetch()} className="btn btn-sm" title="Refresh">
              <ArrowPathIcon style={{ width: 13, height: 13 }} />
            </button>
            <button onClick={handleExport} className="btn btn-sm" title="Export CSV">
              <ArrowDownTrayIcon style={{ width: 13, height: 13 }} />
            </button>
            {hasPermission('subscribers.create') && (
              <button onClick={() => setShowBulkImport(true)} className="btn btn-sm" title="Import CSV">
                <ArrowUpTrayIcon style={{ width: 13, height: 13 }} />
              </button>
            )}
            {hasPermission('subscribers.create') && (
              <Link to="/subscribers/new" className="btn btn-primary btn-sm" title="Add Subscriber">
                <PlusIcon style={{ width: 13, height: 13, marginRight: 2 }} />
                <span className="hide-mobile">Add</span>
              </Link>
            )}
          </div>
        </div>

        {/* === Filter / Column panels === */}
        {(showFilters || showColumnSettings) && (
          <div className="px-2 py-1.5 border-b border-[#ccc] dark:border-[#374151] bg-[#f4f4f4] dark:bg-[#1f2937]">
            {showColumnSettings && (
              <div>
                <div className="text-[10px] font-semibold text-[#666] dark:text-gray-400 mb-1">Visible Columns</div>
                <div className="flex flex-wrap gap-1">
                  {Object.entries(visibleColumns).map(([key, visible]) => (
                    <label key={key} className={clsx(
                      'flex items-center gap-1 px-1.5 py-0.5 border rounded-sm text-[10px] cursor-pointer',
                      visible
                        ? 'bg-[#e3f2fd] dark:bg-blue-900/40 border-[#ccc] dark:border-blue-700'
                        : 'bg-[#f5f5f5] dark:bg-[#374151] border-[#ccc] dark:border-[#4b5563]'
                    )}>
                      <input
                        type="checkbox"
                        checked={visible}
                        onChange={(e) => setVisibleColumns({ ...visibleColumns, [key]: e.target.checked })}
                        className="w-[11px] h-[11px]"
                      />
                      <span className="capitalize dark:text-gray-300">{key.replace('_', ' ')}</span>
                    </label>
                  ))}
                </div>
              </div>
            )}

            {showFilters && viewMode === 'active' && (
              <div className={showColumnSettings ? 'mt-2 pt-2 border-t border-[#ccc] dark:border-[#374151]' : ''}>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px', alignItems: 'flex-end' }}>
                  <div>
                    <label style={{ fontSize: '10px', color: '#666', display: 'block' }}>Service</label>
                    <select value={serviceId} onChange={(e) => { setServiceId(e.target.value); setPage(1); }} className="input input-sm" style={{ width: '130px' }}>
                      <option value="">All Services</option>
                      {services?.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
                    </select>
                  </div>
                  <div>
                    <label style={{ fontSize: '10px', color: '#666', display: 'block' }}>NAS</label>
                    <select value={nasId} onChange={(e) => { setNasId(e.target.value); setPage(1); }} className="input input-sm" style={{ width: '130px' }}>
                      <option value="">All NAS</option>
                      {nasList?.map((n) => <option key={n.id} value={n.id}>{n.name}</option>)}
                    </select>
                  </div>
                  <div>
                    <label style={{ fontSize: '10px', color: '#666', display: 'block' }}>Reseller</label>
                    <select value={resellerId} onChange={(e) => { setResellerId(e.target.value); setPage(1); }} className="input input-sm" style={{ width: '130px' }}>
                      <option value="">All Resellers</option>
                      {resellers?.map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
                    </select>
                  </div>
                  <div>
                    <label style={{ fontSize: '10px', color: '#666', display: 'block' }}>FUP Level</label>
                    <select value={fupLevel} onChange={(e) => { setFupLevel(e.target.value); setPage(1); }} className="input input-sm" style={{ width: '100px' }}>
                      <option value="">All FUP</option>
                      <option value="0">FUP 0</option>
                      <option value="1">FUP 1</option>
                      <option value="2">FUP 2</option>
                      <option value="3">FUP 3</option>
                      <option value="4">FUP 4</option>
                      <option value="5">FUP 5</option>
                      <option value="6">FUP 6</option>
                    </select>
                  </div>
                  <div>
                    <label style={{ fontSize: '10px', color: '#666', display: 'block' }}>Per Page</label>
                    <select value={limit} onChange={(e) => { setLimit(parseInt(e.target.value)); setPage(1); }} className="input input-sm" style={{ width: '65px' }}>
                      <option value="10">10</option>
                      <option value="25">25</option>
                      <option value="50">50</option>
                      <option value="100">100</option>
                    </select>
                  </div>
                  <button onClick={() => { setSearch(''); setStatus(''); setServiceId(''); setNasId(''); setResellerId(''); setFupLevel(''); setPage(1); }} className="btn btn-sm">
                    Clear
                  </button>
                </div>
              </div>
            )}
          </div>
        )}

        {/* === Table === */}
        <div className="table-container" style={{ border: 'none' }}>
          <table className="table table-compact">
            <thead>
              {table.getHeaderGroups().map((headerGroup) => (
                <tr key={headerGroup.id}>
                  {headerGroup.headers.map((header) => (
                    <th
                      key={header.id}
                      onClick={header.column.getCanSort() ? header.column.getToggleSortingHandler() : undefined}
                      style={{ cursor: header.column.getCanSort() ? 'pointer' : 'default' }}
                    >
                      <div style={{ display: 'flex', alignItems: 'center', gap: '2px' }}>
                        {flexRender(header.column.columnDef.header, header.getContext())}
                        {header.column.getIsSorted() === 'asc' && ' \u25B2'}
                        {header.column.getIsSorted() === 'desc' && ' \u25BC'}
                      </div>
                    </th>
                  ))}
                </tr>
              ))}
            </thead>
            <tbody>
              {isLoading ? (
                <tr>
                  <td colSpan={columns.length} style={{ textAlign: 'center', padding: '20px' }}>
                    Loading...
                  </td>
                </tr>
              ) : table.getRowModel().rows.length === 0 ? (
                <tr>
                  <td colSpan={columns.length} style={{ textAlign: 'center', padding: '20px', color: '#999' }}>
                    {viewMode === 'archived' ? 'No archived subscribers' : 'No subscribers found'}
                  </td>
                </tr>
              ) : (
                table.getRowModel().rows.map((row) => {
                  const isSelected = selectedIds.has(row.original.id)
                  return (
                    <tr
                      key={row.id}
                      className={isSelected ? 'selected' : ''}
                      style={{ cursor: 'pointer' }}
                      onClick={() => toggleRowSelection(row.original.id)}
                    >
                      {row.getVisibleCells().map((cell) => (
                        <td key={cell.id}>
                          {flexRender(cell.column.columnDef.cell, cell.getContext())}
                        </td>
                      ))}
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
        </div>

        {/* === Status Bar / Pagination === */}
        <div className="wb-statusbar">
          <span>
            {((page - 1) * limit) + 1}-{Math.min(page * limit, data?.meta?.total || 0)} of {data?.meta?.total || 0}
          </span>
          <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
            <button
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page === 1}
              className="btn btn-xs"
            >
              <ChevronLeftIcon style={{ width: 12, height: 12 }} />
            </button>
            <span style={{ fontSize: '11px', padding: '0 4px' }}>{page}/{totalPages || 1}</span>
            <button
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              disabled={page >= totalPages}
              className="btn btn-xs"
            >
              <ChevronRightIcon style={{ width: 12, height: 12 }} />
            </button>
          </div>
        </div>
      </div>

      {/* ===================== MODALS ===================== */}

      {/* Reset MAC Modal */}
      {actionModal?.type === 'resetMac' && (
        <div className="modal-overlay">
          <div className="modal">
            <div className="modal-header">
              <span>Reset MAC Address</span>
              <button onClick={() => { setActionModal(null); setActionValue(''); setActionReason(''); }} className="btn btn-ghost btn-xs" style={{ color: 'white' }}>
                <XMarkIcon style={{ width: 14, height: 14 }} />
              </button>
            </div>
            <div className="modal-body" style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
              <div style={{ color: '#666' }}>User: <strong>{actionModal.subscriber.username}</strong></div>
              <div style={{ color: '#666' }}>Current MAC: <code style={{ background: '#e8e8e8', padding: '1px 4px', borderRadius: '2px' }}>{actionModal.subscriber.mac_address || 'None'}</code></div>
              <div>
                <label className="label">New MAC Address (leave empty to clear)</label>
                <input type="text" value={actionValue} onChange={(e) => setActionValue(e.target.value)} className="input" placeholder="XX:XX:XX:XX:XX:XX" style={{ fontFamily: 'monospace' }} />
              </div>
              <div>
                <label className="label">Reason</label>
                <select value={actionReason} onChange={(e) => setActionReason(e.target.value)} className="input">
                  <option value="">Select reason...</option>
                  <option value="device_change">User switched device</option>
                  <option value="account_sharing">Prevent account sharing</option>
                  <option value="network_card_change">Network card changed</option>
                  <option value="sync_from_radius">Sync from RADIUS</option>
                  <option value="security">Security issue</option>
                  <option value="other">Other</option>
                </select>
              </div>
            </div>
            <div className="modal-footer">
              <button onClick={() => { setActionModal(null); setActionValue(''); setActionReason(''); }} className="btn">Cancel</button>
              <button
                onClick={() => resetMacMutation.mutate({ id: actionModal.subscriber.id, mac_address: actionValue || null, reason: actionReason })}
                disabled={resetMacMutation.isPending}
                className="btn btn-primary"
              >
                {resetMacMutation.isPending ? 'Resetting...' : 'Reset MAC'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Rename Modal */}
      {actionModal?.type === 'rename' && (
        <div className="modal-overlay">
          <div className="modal">
            <div className="modal-header">
              <span>Rename Username</span>
              <button onClick={() => { setActionModal(null); setActionValue(''); setActionReason(''); }} className="btn btn-ghost btn-xs" style={{ color: 'white' }}>
                <XMarkIcon style={{ width: 14, height: 14 }} />
              </button>
            </div>
            <div className="modal-body" style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
              <div style={{ color: '#666' }}>Current Username: <strong>{actionModal.subscriber.username}</strong></div>
              <div>
                <label className="label">New Username *</label>
                <input type="text" value={actionValue} onChange={(e) => setActionValue(e.target.value)} className="input" placeholder="Enter new username" />
              </div>
              <div>
                <label className="label">Reason</label>
                <input type="text" value={actionReason} onChange={(e) => setActionReason(e.target.value)} className="input" placeholder="Enter reason for change" />
              </div>
            </div>
            <div className="modal-footer">
              <button onClick={() => { setActionModal(null); setActionValue(''); setActionReason(''); }} className="btn">Cancel</button>
              <button
                onClick={() => renameMutation.mutate({ id: actionModal.subscriber.id, new_username: actionValue, reason: actionReason })}
                disabled={!actionValue || renameMutation.isPending}
                className="btn btn-primary"
              >
                {renameMutation.isPending ? 'Renaming...' : 'Rename'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Add Days Modal */}
      {actionModal?.type === 'addDays' && (
        <div className="modal-overlay">
          <div className="modal">
            <div className="modal-header">
              <span>Add/Subtract Days</span>
              <button onClick={() => { setActionModal(null); setActionValue(''); setActionReason(''); }} className="btn btn-ghost btn-xs" style={{ color: 'white' }}>
                <XMarkIcon style={{ width: 14, height: 14 }} />
              </button>
            </div>
            <div className="modal-body" style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
              <div style={{ color: '#666' }}>User: <strong>{actionModal.subscriber.username}</strong></div>
              <div style={{ color: '#666' }}>Current Expiry: <strong>{actionModal.subscriber.expiry_date ? formatDate(actionModal.subscriber.expiry_date) : 'N/A'}</strong></div>
              <div>
                <label className="label">Days *</label>
                <input type="number" value={actionValue} onChange={(e) => setActionValue(e.target.value)} className="input" placeholder="Enter days (negative to subtract)" />
                <p style={{ fontSize: '10px', color: '#999', marginTop: '2px' }}>Use negative number to subtract days</p>
              </div>
              <div>
                <label className="label">Reason</label>
                <select value={actionReason} onChange={(e) => setActionReason(e.target.value)} className="input">
                  <option value="">Select reason...</option>
                  <option value="compensation">Compensation</option>
                  <option value="overdue_fix">Overdue fix</option>
                  <option value="promotion">Promotion</option>
                  <option value="manual_adjustment">Manual adjustment</option>
                  <option value="other">Other</option>
                </select>
              </div>
            </div>
            <div className="modal-footer">
              <button onClick={() => { setActionModal(null); setActionValue(''); setActionReason(''); }} className="btn">Cancel</button>
              <button
                onClick={() => addDaysMutation.mutate({ id: actionModal.subscriber.id, days: parseInt(actionValue), reason: actionReason })}
                disabled={!actionValue || addDaysMutation.isPending}
                className="btn btn-primary"
              >
                {addDaysMutation.isPending ? 'Updating...' : 'Apply'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Change Service Modal */}
      {actionModal?.type === 'changeService' && (
        <div className="modal-overlay">
          <div className="modal modal-lg" style={{ maxWidth: '500px' }}>
            <div className="modal-header">
              <span>Change Service</span>
              <button onClick={() => { setActionModal(null); setActionValue(''); setActionReason(''); setPriceCalculation(null); setChangeServiceOptions({ extend_expiry: false, reset_fup: false, charge_price: false, prorate_price: true }); }} className="btn btn-ghost btn-xs" style={{ color: 'white' }}>
                <XMarkIcon style={{ width: 14, height: 14 }} />
              </button>
            </div>
            <div className="modal-body" style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
              <div style={{ color: '#666' }}>User: <strong>{actionModal.subscriber.username}</strong></div>
              <div style={{ color: '#666' }}>Current Service: <strong>{actionModal.subscriber.service?.name || 'N/A'}</strong> - ${actionModal.subscriber.service?.price?.toFixed(2) || '0.00'}</div>
              <div style={{ color: '#666' }}>Expiry: <strong>{actionModal.subscriber.expiry_date ? new Date(actionModal.subscriber.expiry_date).toLocaleDateString() : 'N/A'}</strong></div>
              <div>
                <label className="label">New Service *</label>
                <select
                  value={actionValue}
                  onChange={(e) => {
                    setActionValue(e.target.value)
                    if (e.target.value) fetchPriceCalculation(actionModal.subscriber.id, e.target.value)
                    else setPriceCalculation(null)
                  }}
                  className="input"
                >
                  <option value="">Select Service</option>
                  {services?.map((s) => <option key={s.id} value={s.id}>{s.name} - ${s.price?.toFixed(2)}</option>)}
                </select>
              </div>

              {calculatingPrice && <div style={{ background: '#f8f8f8', padding: '6px 8px', border: '1px solid #ccc', borderRadius: '2px', color: '#666' }}>Calculating price...</div>}
              {priceCalculation && !calculatingPrice && (
                <div style={{ background: '#f8f8f8', padding: '8px', border: '1px solid #ccc', borderRadius: '2px' }}>
                  <div style={{ fontWeight: 600, marginBottom: '4px' }}>
                    {priceCalculation.is_upgrade ? 'Upgrade' : priceCalculation.is_downgrade ? 'Downgrade' : 'Same Price'}
                  </div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '2px', color: '#555' }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between' }}><span>Remaining days:</span><span style={{ fontWeight: 500 }}>{priceCalculation.remaining_days} days</span></div>
                    <div style={{ display: 'flex', justifyContent: 'space-between' }}><span>Old day price:</span><span>${priceCalculation.old_day_price?.toFixed(2)}/day</span></div>
                    <div style={{ display: 'flex', justifyContent: 'space-between' }}><span>New day price:</span><span>${priceCalculation.new_day_price?.toFixed(2)}/day</span></div>
                    <div style={{ display: 'flex', justifyContent: 'space-between' }}><span>Credit from old:</span><span style={{ color: '#2e7d32' }}>-${priceCalculation.old_credit?.toFixed(2)}</span></div>
                    <div style={{ display: 'flex', justifyContent: 'space-between' }}><span>Cost for new:</span><span style={{ color: '#c62828' }}>+${priceCalculation.new_cost?.toFixed(2)}</span></div>
                    {priceCalculation.change_fee > 0 && <div style={{ display: 'flex', justifyContent: 'space-between' }}><span>Change fee:</span><span>+${priceCalculation.change_fee?.toFixed(2)}</span></div>}
                    <div style={{ borderTop: '1px solid #ccc', paddingTop: '4px', marginTop: '4px', display: 'flex', justifyContent: 'space-between', fontWeight: 600 }}>
                      <span>Total to {priceCalculation.total_charge >= 0 ? 'charge' : 'refund'}:</span>
                      <span style={{ color: priceCalculation.total_charge >= 0 ? '#c62828' : '#2e7d32' }}>${Math.abs(priceCalculation.total_charge)?.toFixed(2)}</span>
                    </div>
                  </div>
                  {priceCalculation.is_downgrade && !priceCalculation.downgrade_allowed && (
                    <div style={{ marginTop: '6px', color: '#c62828', fontWeight: 500 }}>Downgrade is not allowed by system settings</div>
                  )}
                </div>
              )}

              <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', padding: '8px', background: '#f9fafb', borderRadius: '6px', border: '1px solid #e5e7eb' }} className="dark:bg-gray-700 dark:border-gray-600">
                <div style={{ fontSize: '12px', fontWeight: 600, color: '#374151', marginBottom: '2px' }} className="dark:text-gray-300">Options</div>
                <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer', fontSize: '13px', color: '#374151' }} className="dark:text-gray-200">
                  <input type="checkbox" checked={changeServiceOptions.extend_expiry} onChange={(e) => setChangeServiceOptions({ ...changeServiceOptions, extend_expiry: e.target.checked })} style={{ width: 16, height: 16, accentColor: '#2563eb' }} />
                  Extend Expiry
                </label>
                <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer', fontSize: '13px', color: '#374151' }} className="dark:text-gray-200">
                  <input type="checkbox" checked={changeServiceOptions.reset_fup} onChange={(e) => setChangeServiceOptions({ ...changeServiceOptions, reset_fup: e.target.checked })} style={{ width: 16, height: 16, accentColor: '#2563eb' }} />
                  Reset FUP Quota
                </label>
                <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer', fontSize: '13px', color: '#374151' }} className="dark:text-gray-200">
                  <input type="checkbox" checked={changeServiceOptions.prorate_price} onChange={(e) => setChangeServiceOptions({ ...changeServiceOptions, prorate_price: e.target.checked, charge_price: false })} style={{ width: 16, height: 16, accentColor: '#2563eb' }} />
                  Prorate Price (recommended)
                </label>
                <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer', fontSize: '13px', color: '#374151' }} className="dark:text-gray-200">
                  <input type="checkbox" checked={changeServiceOptions.charge_price} onChange={(e) => setChangeServiceOptions({ ...changeServiceOptions, charge_price: e.target.checked, prorate_price: false })} style={{ width: 16, height: 16, accentColor: '#2563eb' }} />
                  Charge Full Price
                </label>
              </div>
              <div>
                <label className="label">Reason</label>
                <input type="text" value={actionReason} onChange={(e) => setActionReason(e.target.value)} className="input" placeholder="Enter reason for change" />
              </div>
            </div>
            <div className="modal-footer">
              <button onClick={() => { setActionModal(null); setActionValue(''); setActionReason(''); setPriceCalculation(null); setChangeServiceOptions({ extend_expiry: false, reset_fup: false, charge_price: false, prorate_price: true }); }} className="btn">Cancel</button>
              <button
                onClick={() => changeServiceMutation.mutate({ id: actionModal.subscriber.id, service_id: parseInt(actionValue), ...changeServiceOptions, reason: actionReason })}
                disabled={!actionValue || changeServiceMutation.isPending || (priceCalculation?.is_downgrade && !priceCalculation?.downgrade_allowed)}
                className="btn btn-primary"
              >
                {changeServiceMutation.isPending ? 'Changing...' : 'Change Service'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Add Balance Modal */}
      {actionModal?.type === 'add_balance' && (
        <div className="modal-overlay">
          <div className="modal">
            <div className="modal-header">
              <span>Add Balance</span>
              <button onClick={() => { setActionModal(null); setActionValue(''); setActionReason(''); }} className="btn btn-ghost btn-xs" style={{ color: 'white' }}>
                <XMarkIcon style={{ width: 14, height: 14 }} />
              </button>
            </div>
            <div className="modal-body" style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
              <div style={{ color: '#666' }}>User: <strong>{actionModal.subscriber.username}</strong></div>
              <div style={{ color: '#059669', fontWeight: 500 }}>Current Balance: ${(actionModal.subscriber.balance || 0).toFixed(2)}</div>
              <div>
                <label className="label">Amount *</label>
                <input type="number" step="0.01" min="0" value={actionValue} onChange={(e) => setActionValue(e.target.value)} className="input" placeholder="Enter amount" />
              </div>
              <div>
                <label className="label">Reason</label>
                <select value={actionReason} onChange={(e) => setActionReason(e.target.value)} className="input">
                  <option value="">Select reason...</option>
                  <option value="cash_payment">Cash Payment</option>
                  <option value="bank_transfer">Bank Transfer</option>
                  <option value="prepaid_card">Prepaid Card</option>
                  <option value="credit">Credit</option>
                  <option value="other">Other</option>
                </select>
              </div>
            </div>
            <div className="modal-footer">
              <button onClick={() => { setActionModal(null); setActionValue(''); setActionReason(''); }} className="btn">Cancel</button>
              <button
                onClick={() => addBalanceMutation.mutate({ id: actionModal.subscriber.id, amount: parseFloat(actionValue), reason: actionReason })}
                disabled={!actionValue || parseFloat(actionValue) <= 0 || addBalanceMutation.isPending}
                className="btn btn-primary"
              >
                {addBalanceMutation.isPending ? 'Adding...' : 'Add Balance'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Bulk Import Modal */}
      {showBulkImport && (
        <div className="modal-overlay">
          <div className="modal modal-lg" style={{ maxWidth: '500px' }}>
            <div className="modal-header">
              <span>Bulk Import Subscribers</span>
              <button onClick={() => { setShowBulkImport(false); setImportFile(null); setImportResults(null); }} className="btn btn-ghost btn-xs" style={{ color: 'white' }}>
                <XMarkIcon style={{ width: 14, height: 14 }} />
              </button>
            </div>
            <div className="modal-body" style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
              <div>
                <label className="label">CSV File</label>
                <div
                  style={{ border: '2px dashed #ccc', borderRadius: '2px', padding: '20px', textAlign: 'center', cursor: 'pointer', background: '#fafafa' }}
                  onClick={() => fileInputRef.current?.click()}
                >
                  <DocumentArrowUpIcon style={{ width: 30, height: 30, color: '#999', margin: '0 auto 6px' }} />
                  <p style={{ fontSize: '11px', color: '#666' }}>
                    {importFile ? importFile.name : 'Click to select CSV file'}
                  </p>
                  <p style={{ fontSize: '10px', color: '#999', marginTop: '4px' }}>
                    Columns: username, password, full_name, email, phone, address
                  </p>
                </div>
                <input ref={fileInputRef} type="file" accept=".csv" onChange={(e) => setImportFile(e.target.files?.[0] || null)} style={{ display: 'none' }} />
              </div>
              <div>
                <label className="label">Service Plan *</label>
                <select value={importServiceId} onChange={(e) => setImportServiceId(e.target.value)} className="input">
                  <option value="">Select Service</option>
                  {services?.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
                </select>
              </div>
              {importResults && (
                <div style={{ background: '#f8f8f8', border: '1px solid #ccc', borderRadius: '2px', padding: '8px', maxHeight: '150px', overflowY: 'auto' }}>
                  <div style={{ fontWeight: 600, marginBottom: '4px' }}>Import Results</div>
                  <p style={{ color: '#2e7d32' }}>Created: {importResults.created}</p>
                  <p style={{ color: '#c62828' }}>Failed: {importResults.failed}</p>
                  {importResults.results?.filter(r => !r.success).slice(0, 10).map((r, i) => (
                    <p key={i} style={{ fontSize: '10px', color: '#c62828' }}>Row {r.row}: {r.message}</p>
                  ))}
                </div>
              )}
            </div>
            <div className="modal-footer">
              <button onClick={() => { setShowBulkImport(false); setImportFile(null); setImportResults(null); }} className="btn">Close</button>
              <button onClick={handleBulkImport} disabled={!importFile || !importServiceId || bulkImportMutation.isPending} className="btn btn-primary">
                {bulkImportMutation.isPending ? 'Importing...' : 'Import'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Torch Modal */}
      {torchModal && (
        <div className="modal-overlay">
          <div className="modal modal-lg" style={{ maxWidth: '650px' }}>
            <div className="modal-header">
              <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                <SignalIcon style={{ width: 16, height: 16 }} />
                <span>Live Traffic - {torchModal.username}</span>
                {torchAutoRefresh && <span style={{ width: 8, height: 8, borderRadius: '50%', background: '#4CAF50', display: 'inline-block' }} />}
              </div>
              <button onClick={() => { setTorchModal(null); setTorchData(null); setTorchAutoRefresh(false); }} className="btn btn-ghost btn-xs" style={{ color: 'white' }}>
                <XMarkIcon style={{ width: 14, height: 14 }} />
              </button>
            </div>
            <div className="modal-body" style={{ maxHeight: '60vh', overflow: 'auto' }}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '8px', flexWrap: 'wrap', gap: '6px' }}>
                <span style={{ fontSize: '11px' }}>IP: <code style={{ background: '#e8e8e8', padding: '1px 4px', borderRadius: '2px', fontFamily: 'monospace' }}>{torchModal.ip_address || 'N/A'}</code></span>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <label style={{ display: 'flex', alignItems: 'center', gap: '4px', fontSize: '11px', cursor: 'pointer' }}>
                    <input type="checkbox" checked={torchAutoRefresh} onChange={(e) => setTorchAutoRefresh(e.target.checked)} style={{ width: 13, height: 13 }} />
                    Auto
                  </label>
                  <button onClick={() => fetchTorchData(torchModal)} disabled={torchLoading} className="btn btn-sm">
                    <ArrowPathIcon style={{ width: 13, height: 13, ...(torchLoading ? { animation: 'spin 1s linear infinite' } : {}) }} />
                  </button>
                </div>
              </div>

              {torchLoading && !torchData && (
                <div style={{ textAlign: 'center', padding: '20px', color: '#999' }}>Capturing...</div>
              )}

              {torchData && (
                <div>
                  <div style={{ background: '#2d2d2d', color: '#e0e0e0', padding: '6px 10px', borderRadius: '2px 2px 0 0', display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '4px' }}>
                    <span style={{ fontSize: '11px' }}>
                      <span style={{ color: '#aaa' }}>Interface: </span>
                      <span style={{ color: '#4CAF50', fontFamily: 'monospace' }}>{torchData.interface}</span>
                    </span>
                    <div style={{ display: 'flex', gap: '12px', fontSize: '11px' }}>
                      <span style={{ color: '#4CAF50', fontWeight: 700 }}>DL: {((torchData.total_tx || 0) * 8 / 1000000).toFixed(1)} Mbps</span>
                      <span style={{ color: '#2196F3', fontWeight: 700 }}>UL: {((torchData.total_rx || 0) * 8 / 1000000).toFixed(1)} Mbps</span>
                    </div>
                  </div>

                  {torchData.entries && torchData.entries.length > 0 ? (
                    <div style={{ border: '1px solid #a0a0a0', borderTop: 'none', borderRadius: '0 0 2px 2px', overflow: 'hidden' }}>
                      <div style={{ maxHeight: '280px', overflowY: 'auto' }}>
                        <table className="table table-compact" style={{ margin: 0 }}>
                          <thead>
                            <tr>
                              <th>Proto</th>
                              <th>Src. Address</th>
                              <th>Dst. Address</th>
                              <th style={{ textAlign: 'right' }}>Download</th>
                              <th style={{ textAlign: 'right' }}>Upload</th>
                            </tr>
                          </thead>
                          <tbody>
                            {torchData.entries.slice(0, 100).map((entry, idx) => (
                              <tr key={idx}>
                                <td>
                                  <span style={{ textTransform: 'uppercase', color: entry.protocol === 'tcp' ? '#1565c0' : entry.protocol === 'udp' ? '#6a1b9a' : '#e65100' }}>
                                    {entry.protocol || '-'}
                                  </span>
                                </td>
                                <td style={{ fontFamily: 'monospace' }}>
                                  {entry.src_address}{entry.src_port > 0 && `:${entry.src_port}`}
                                </td>
                                <td style={{ fontFamily: 'monospace' }}>
                                  {entry.dst_address}{entry.dst_port > 0 && `:${entry.dst_port}`}
                                </td>
                                <td style={{ textAlign: 'right', color: '#2e7d32', fontWeight: 500 }}>
                                  {entry.tx_rate > 1000000 ? `${(entry.tx_rate * 8 / 1000000).toFixed(1)} Mbps` : entry.tx_rate > 1000 ? `${(entry.tx_rate * 8 / 1000).toFixed(1)} kbps` : `${(entry.tx_rate * 8).toFixed(0)} bps`}
                                </td>
                                <td style={{ textAlign: 'right', color: '#1565c0', fontWeight: 500 }}>
                                  {entry.rx_rate > 1000000 ? `${(entry.rx_rate * 8 / 1000000).toFixed(1)} Mbps` : entry.rx_rate > 1000 ? `${(entry.rx_rate * 8 / 1000).toFixed(1)} kbps` : `${(entry.rx_rate * 8).toFixed(0)} bps`}
                                </td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                      <div style={{ background: '#f0f0f0', padding: '3px 8px', fontSize: '10px', color: '#666', borderTop: '1px solid #ccc', display: 'flex', justifyContent: 'space-between' }}>
                        <span>{torchData.entries.length} flows</span>
                        <span>{torchData.duration}</span>
                      </div>
                    </div>
                  ) : (
                    <div style={{ border: '1px solid #a0a0a0', borderTop: 'none', borderRadius: '0 0 2px 2px', padding: '20px', textAlign: 'center', color: '#999', background: '#fafafa' }}>
                      No active traffic flows
                    </div>
                  )}
                </div>
              )}

              {!torchLoading && !torchData && (
                <div style={{ textAlign: 'center', padding: '20px', color: '#999' }}>Click Refresh to capture traffic</div>
              )}
            </div>
            <div className="modal-footer">
              <button onClick={() => { setTorchModal(null); setTorchData(null); setTorchAutoRefresh(false); }} className="btn">Close</button>
            </div>
          </div>
        </div>
      )}

      {/* Location Map Modal */}
      {mapModal && (
        <div className="modal-overlay" onClick={() => setMapModal(null)}>
          <div className="modal modal-lg" style={{ maxWidth: '500px' }} onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                <MapPinIcon style={{ width: 16, height: 16 }} />
                <span>{mapModal.full_name || mapModal.username}</span>
              </div>
              <button onClick={() => setMapModal(null)} className="btn btn-ghost btn-xs" style={{ color: 'white' }}>
                <XMarkIcon style={{ width: 14, height: 14 }} />
              </button>
            </div>
            <div style={{ overflow: 'hidden' }}>
              <ViewLocationMap lat={parseFloat(mapModal.latitude)} lng={parseFloat(mapModal.longitude)} />
            </div>
            <div className="modal-footer" style={{ justifyContent: 'space-between' }}>
              <span style={{ fontSize: '10px', color: '#666' }}>
                {parseFloat(mapModal.latitude).toFixed(6)}, {parseFloat(mapModal.longitude).toFixed(6)}
              </span>
              <a
                href={`https://www.google.com/maps/dir/?api=1&destination=${mapModal.latitude},${mapModal.longitude}`}
                target="_blank"
                rel="noopener noreferrer"
                className="btn btn-sm"
              >
                <MapPinIcon style={{ width: 12, height: 12, marginRight: 3 }} />
                Navigate
              </a>
            </div>
          </div>
        </div>
      )}

      {/* Delete Confirmation Modal */}
      {deleteConfirm && (
        <div className="modal-overlay" onClick={() => setDeleteConfirm(null)}>
          <div className="modal" style={{ maxWidth: '420px' }} onClick={e => e.stopPropagation()}>
            <div className="modal-header" style={{ background: 'linear-gradient(to bottom, #e74c3c, #c0392b)' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                <TrashIcon style={{ width: 16, height: 16 }} />
                <span>Delete {deleteConfirm.ids.length} Subscriber{deleteConfirm.ids.length > 1 ? 's' : ''}?</span>
              </div>
              <button onClick={() => setDeleteConfirm(null)} className="btn btn-ghost btn-xs" style={{ color: 'white' }}>
                <XMarkIcon style={{ width: 14, height: 14 }} />
              </button>
            </div>
            <div className="modal-body">
              <p style={{ color: '#666', marginBottom: '6px' }}>The following will be permanently deleted:</p>
              <div style={{ maxHeight: '200px', overflowY: 'auto' }}>
                {deleteConfirm.names.slice(0, 20).map((name, i) => (
                  <div key={i} style={{ padding: '3px 6px', marginBottom: '2px', background: '#fff0f0', border: '1px solid #ffcdd2', borderRadius: '2px' }}>
                    <strong style={{ color: '#c62828' }}>{name}</strong>
                  </div>
                ))}
                {deleteConfirm.names.length > 20 && (
                  <div style={{ color: '#999', fontSize: '10px', padding: '3px 6px' }}>... and {deleteConfirm.names.length - 20} more</div>
                )}
              </div>
            </div>
            <div className="modal-footer">
              <button onClick={() => setDeleteConfirm(null)} className="btn">Cancel</button>
              <button
                onClick={() => { bulkActionMutation.mutate({ ids: deleteConfirm.ids, action: 'delete' }); setDeleteConfirm(null); }}
                className="btn btn-danger"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
