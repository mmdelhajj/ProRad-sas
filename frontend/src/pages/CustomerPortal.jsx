import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store/authStore'
import { useBrandingStore } from '../store/brandingStore'
import { formatDate, formatDateTime } from '../utils/timezone'
import {
  WifiIcon,
  ClockIcon,
  ArrowDownTrayIcon,
  ArrowUpTrayIcon,
  CalendarDaysIcon,
  SignalIcon,
  UserCircleIcon,
  ArrowRightOnRectangleIcon,
  ChartBarIcon,
  ChatBubbleLeftRightIcon,
  PlusIcon,
  XMarkIcon,
  PaperAirplaneIcon,
  BellAlertIcon,
  BanknotesIcon
} from '@heroicons/react/24/outline'
import api from '../services/api'

// Format bytes to human readable
function formatBytes(bytes) {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

// Circular progress ring component
function CircularProgressRing({ value = 0, total = 0, label, size = 110, strokeWidth = 10 }) {
  const percentage = total > 0 ? Math.min((value / total) * 100, 100) : 0
  const roundedPercent = Math.round(percentage * 10) / 10
  const radius = (size - strokeWidth) / 2
  const circumference = 2 * Math.PI * radius
  const dashoffset = circumference - (circumference * percentage) / 100
  const center = size / 2

  const getColor = () => {
    if (percentage >= 95) return '#ef4444'
    if (percentage >= 80) return '#f59e0b'
    return '#10b981'
  }
  const color = getColor()

  return (
    <div className="flex flex-col items-center">
      <div className="relative" style={{ width: size, height: size }}>
        <svg width={size} height={size}>
          <circle
            cx={center} cy={center} r={radius}
            fill="none" stroke="#f1f5f9" strokeWidth={strokeWidth}
            className="dark:stroke-[#334155]"
          />
          <circle
            cx={center} cy={center} r={radius}
            fill="none" stroke={color} strokeWidth={strokeWidth}
            strokeLinecap="round"
            strokeDasharray={circumference}
            strokeDashoffset={dashoffset}
            transform={`rotate(-90 ${center} ${center})`}
            style={{ transition: 'stroke-dashoffset 0.8s ease-out' }}
          />
        </svg>
        <div className="absolute inset-0 flex items-center justify-center">
          <span className="text-[15px] font-bold" style={{ color }}>{roundedPercent}%</span>
        </div>
      </div>
      <span className="text-[11px] font-semibold text-gray-900 dark:text-[#e0e0e0] mt-1">
        {formatBytes(value)}{total > 0 ? ` / ${formatBytes(total)}` : ''}
      </span>
      {label && <span className="text-[11px] text-gray-500 dark:text-[#aaa]">{label}</span>}
    </div>
  )
}

// Format duration
function formatDuration(seconds) {
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  if (hours > 0) {
    return `${hours}h ${minutes}m`
  }
  return `${minutes}m`
}

export default function CustomerPortal() {
  const navigate = useNavigate()
  const { isAuthenticated, isCustomer, customerData, logout, refreshUser } = useAuthStore()
  const { companyName, companyLogo, fetchBranding, loaded } = useBrandingStore()
  const [dashboard, setDashboard] = useState(null)
  const [sessions, setSessions] = useState([])
  const [usageHistory, setUsageHistory] = useState([])
  const [tickets, setTickets] = useState([])
  const [selectedTicket, setSelectedTicket] = useState(null)
  const [showCreateTicket, setShowCreateTicket] = useState(false)
  const [ticketForm, setTicketForm] = useState({ subject: '', description: '', category: 'general' })
  const [replyText, setReplyText] = useState('')
  const [invoices, setInvoices] = useState([])
  const [viewInvoiceId, setViewInvoiceId] = useState(null)
  const [viewInvoice, setViewInvoice] = useState(null)
  const [viewLoading, setViewLoading] = useState(false)
  const [activeTab, setActiveTab] = useState('dashboard')
  const [loading, setLoading] = useState(true)
  const [publicIPData, setPublicIPData] = useState(null)
  const [publicIPLoading, setPublicIPLoading] = useState(false)
  const [walletTransactions, setWalletTransactions] = useState([])
  const [walletTransactionsLoading, setWalletTransactionsLoading] = useState(false)
  const [banners, setBanners] = useState([])
  const [dismissedBanners, setDismissedBanners] = useState(() => {
    try { return JSON.parse(localStorage.getItem('proisp-dismissed-banners') || '[]') } catch { return [] }
  })

  // Fetch branding
  useEffect(() => {
    if (!loaded) {
      fetchBranding()
    }
  }, [loaded, fetchBranding])

  // Fetch notification banners
  useEffect(() => {
    if (!isAuthenticated || !isCustomer) return
    const fetchBanners = async () => {
      try {
        const res = await api.get('/customer/active-banners')
        if (res.data.success) setBanners(res.data.data || [])
      } catch {}
    }
    fetchBanners()
    const interval = setInterval(fetchBanners, 5 * 60 * 1000)
    return () => clearInterval(interval)
  }, [isAuthenticated, isCustomer])

  const dismissBanner = (id) => {
    const updated = [...dismissedBanners, id]
    setDismissedBanners(updated)
    localStorage.setItem('proisp-dismissed-banners', JSON.stringify(updated))
  }

  // Redirect if not authenticated as customer
  useEffect(() => {
    if (!isAuthenticated) {
      navigate('/login')
      return
    }
    if (!isCustomer) {
      // If logged in as admin/reseller, redirect to admin dashboard
      navigate('/')
      return
    }
    fetchDashboard()
  }, [isAuthenticated, isCustomer, navigate])

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  const fetchDashboard = async () => {
    setLoading(true)
    try {
      const res = await api.get('/customer/dashboard')
      if (res.data.success) {
        setDashboard(res.data.data)
      }
    } catch (err) {
      if (err.response?.status === 401) {
        handleLogout()
      }
    } finally {
      setLoading(false)
    }
  }

  const fetchSessions = async () => {
    try {
      const res = await api.get('/customer/sessions')
      if (res.data.success) {
        setSessions(res.data.data)
      }
    } catch (err) {
      console.error('Failed to fetch sessions', err)
    }
  }

  const fetchUsageHistory = async () => {
    try {
      const res = await api.get('/customer/usage')
      if (res.data.success) {
        const raw = res.data.data
        // Handle both nested { daily: [...] } and flat array responses
        setUsageHistory(Array.isArray(raw) ? raw : (raw?.daily || []))
      }
    } catch (err) {
      console.error('Failed to fetch usage history', err)
    }
  }

  const fetchPublicIP = async () => {
    setPublicIPLoading(true)
    try {
      const res = await api.get('/customer/public-ip')
      if (res.data.success) {
        setPublicIPData(res.data)
      }
    } catch (err) {
      console.error('Failed to fetch public IP data', err)
    } finally {
      setPublicIPLoading(false)
    }
  }

  const fetchWalletTransactions = async () => {
    setWalletTransactionsLoading(true)
    try {
      const res = await api.get('/customer/transactions')
      if (res.data.success) {
        setWalletTransactions(res.data.data || [])
      }
    } catch (err) {
      console.error('Failed to fetch wallet transactions', err)
    } finally {
      setWalletTransactionsLoading(false)
    }
  }

  const handleBuyPublicIP = async (poolId) => {
    try {
      const res = await api.post('/customer/public-ip/buy', { pool_id: poolId })
      if (res.data.success) {
        alert('Public IP purchased successfully! Your connection will reconnect shortly.')
        fetchPublicIP()
        fetchDashboard() // Refresh balance
      } else {
        alert(res.data.message || 'Failed to purchase IP')
      }
    } catch (err) {
      alert(err.response?.data?.message || 'Failed to purchase IP')
    }
  }

  const handleReleasePublicIP = async () => {
    if (!confirm('Are you sure you want to release your public IP? Your connection will reconnect.')) return
    try {
      const res = await api.post('/customer/public-ip/release')
      if (res.data.success) {
        alert('Public IP released successfully.')
        fetchPublicIP()
      } else {
        alert(res.data.message || 'Failed to release IP')
      }
    } catch (err) {
      alert(err.response?.data?.message || 'Failed to release IP')
    }
  }

  const fetchTickets = async () => {
    try {
      const res = await api.get('/customer/tickets')
      if (res.data.success) {
        setTickets(res.data.data || [])
      }
    } catch (err) {
      console.error('Failed to fetch tickets', err)
    }
  }

  const fetchTicketDetail = async (ticketId) => {
    try {
      const res = await api.get(`/customer/tickets/${ticketId}`)
      if (res.data.success) {
        setSelectedTicket(res.data.data)
      }
    } catch (err) {
      console.error('Failed to fetch ticket', err)
    }
  }

  const fetchInvoices = async () => {
    try {
      const res = await api.get('/customer/invoices')
      if (res.data.success) {
        setInvoices(res.data.data || [])
      }
    } catch (err) {
      console.error('Failed to fetch invoices', err)
    }
  }

  const openInvoice = async (invoiceId) => {
    setViewInvoiceId(invoiceId)
    setViewLoading(true)
    try {
      const res = await api.get(`/customer/invoices/${invoiceId}`)
      if (res.data.success) {
        setViewInvoice(res.data.data)
      }
    } catch (err) {
      console.error('Failed to fetch invoice', err)
    }
    setViewLoading(false)
  }

  const closeInvoice = () => {
    setViewInvoiceId(null)
    setViewInvoice(null)
  }

  const handleCreateTicket = async (e) => {
    e.preventDefault()
    try {
      const res = await api.post('/customer/tickets', ticketForm)
      if (res.data.success) {
        setShowCreateTicket(false)
        setTicketForm({ subject: '', description: '', category: 'general' })
        fetchTickets()
      }
    } catch (err) {
      console.error('Failed to create ticket', err)
    }
  }

  const handleReplyTicket = async (e) => {
    e.preventDefault()
    if (!replyText.trim() || !selectedTicket) return
    try {
      const res = await api.post(`/customer/tickets/${selectedTicket.id}/reply`, { message: replyText })
      if (res.data.success) {
        setReplyText('')
        fetchTicketDetail(selectedTicket.id)
      }
    } catch (err) {
      console.error('Failed to reply', err)
    }
  }

  useEffect(() => {
    if (isCustomer && activeTab === 'sessions') {
      fetchSessions()
    } else if (isCustomer && activeTab === 'usage') {
      fetchUsageHistory()
    } else if (isCustomer && activeTab === 'invoices') {
      fetchInvoices()
    } else if (isCustomer && activeTab === 'tickets') {
      fetchTickets()
    } else if (isCustomer && activeTab === 'public-ip') {
      fetchPublicIP()
    } else if (isCustomer && activeTab === 'wallet') {
      fetchWalletTransactions()
    }
  }, [isCustomer, activeTab])

  // Loading state
  if (loading) {
    return (
      <div className="min-h-screen bg-[#c0c0c0] dark:bg-[#2d2d2d] flex items-center justify-center" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
        <svg className="animate-spin h-8 w-8 text-[#316AC5]" fill="none" viewBox="0 0 24 24">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
        </svg>
      </div>
    )
  }

  // Dashboard
  return (
    <div className="min-h-screen bg-[#c0c0c0] dark:bg-[#2d2d2d]" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Header */}
      <header className="wb-toolbar justify-between">
        <div className="flex items-center gap-2">
          {companyLogo ? (
            <img src={companyLogo} alt={companyName} className="h-7 object-contain" />
          ) : (
            <div className="w-7 h-7 bg-[#316AC5] flex items-center justify-center" style={{ borderRadius: '2px' }}>
              <WifiIcon className="w-4 h-4 text-white" />
            </div>
          )}
          <div>
            {!companyLogo && <span className="text-[13px] font-semibold text-gray-900 dark:text-[#e0e0e0]">{companyName}</span>}
            <span className="text-[12px] text-gray-500 dark:text-[#aaa] ml-2">{dashboard?.username || customerData?.username}</span>
          </div>
        </div>
        <button
          onClick={handleLogout}
          className="btn btn-sm flex items-center gap-1"
        >
          <ArrowRightOnRectangleIcon className="w-3.5 h-3.5" />
          <span className="hidden sm:inline">Logout</span>
        </button>
      </header>

      {/* Notification Banners */}
      {banners.filter(b => !dismissedBanners.includes(b.id)).map(banner => {
        const bgColor = banner.banner_type === 'warning' ? 'bg-amber-500' :
          banner.banner_type === 'error' ? 'bg-red-600' :
          banner.banner_type === 'success' ? 'bg-green-600' : 'bg-blue-600'
        return (
          <div key={banner.id} className={`${bgColor} text-white text-[12px] px-3 py-1.5 flex items-center gap-2`}>
            <BellAlertIcon className="w-4 h-4 flex-shrink-0" />
            <span className="font-semibold flex-shrink-0">{banner.title}</span>
            <span className="truncate">{banner.message}</span>
            <div className="flex-1" />
            {banner.dismissible && (
              <button onClick={() => dismissBanner(banner.id)} className="p-0.5 hover:bg-white/20 rounded flex-shrink-0">
                <XMarkIcon className="w-3.5 h-3.5" />
              </button>
            )}
          </div>
        )
      })}

      {/* Tabs */}
      <div className="max-w-7xl mx-auto px-3 mt-3">
        <div className="flex gap-0 border-b border-[#a0a0a0] dark:border-[#555]">
          {['dashboard', 'sessions', 'usage', 'wallet', 'invoices', 'tickets', 'public-ip'].map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`wb-tab ${activeTab === tab ? 'active' : ''}`}
            >
              {tab === 'public-ip' ? 'Public IP' : tab.charAt(0).toUpperCase() + tab.slice(1)}
            </button>
          ))}
        </div>
      </div>

      {/* Content */}
      <main className="max-w-7xl mx-auto px-3 py-3">
        {activeTab === 'dashboard' && dashboard && (
          <div className="space-y-3">
            {/* Status Card */}
            <div className={`card p-3 text-white ${
              dashboard.status === 'active' && dashboard.days_left > 0
                ? 'bg-[#4CAF50] border-[#388E3C]'
                : dashboard.status === 'expired' || dashboard.days_left <= 0
                ? 'bg-[#f44336] border-[#c62828]'
                : 'bg-[#FF9800] border-[#F57C00]'
            }`}>
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-white/80 text-[11px]">Account Status</p>
                  <p className="text-[16px] font-bold capitalize mt-0.5">{dashboard.status}</p>
                </div>
                <div className={`w-8 h-8 flex items-center justify-center ${
                  dashboard.is_online ? 'bg-white/20' : 'bg-white/10'
                }`} style={{ borderRadius: '2px' }}>
                  <SignalIcon className={`w-5 h-5 ${dashboard.is_online ? 'text-white' : 'text-white/50'}`} />
                </div>
              </div>
              <div className="mt-3 flex items-center gap-4 text-[12px]">
                <div>
                  <p className="text-white/70 text-[11px]">Expires</p>
                  <p className="font-medium">{formatDate(dashboard.expiry_date)}</p>
                </div>
                <div className="h-6 w-px bg-white/20" />
                <div>
                  <p className="text-white/70 text-[11px]">Days Left</p>
                  <p className="font-medium">{dashboard.days_left} days</p>
                </div>
                <div className="h-6 w-px bg-white/20" />
                <div>
                  <p className="text-white/70 text-[11px]">Connection</p>
                  <p className="font-medium">{dashboard.is_online ? 'Online' : 'Offline'}</p>
                </div>
              </div>
            </div>

            {/* Info Grid */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-2">
              {/* Service */}
              <div className="stat-card">
                <div className="flex items-center gap-2">
                  <WifiIcon className="w-4 h-4 text-[#316AC5]" />
                  <div>
                    <p className="text-[11px] text-gray-500 dark:text-[#aaa]">Service Plan</p>
                    <p className="text-[12px] font-bold text-gray-900 dark:text-[#e0e0e0]">{dashboard.service_name}</p>
                  </div>
                </div>
              </div>

              {/* Speed */}
              <div className="stat-card">
                <div className="flex items-center gap-2">
                  <SignalIcon className="w-4 h-4 text-[#4CAF50]" />
                  <div>
                    <p className="text-[11px] text-gray-500 dark:text-[#aaa]">Current Speed</p>
                    <p className="text-[12px] font-bold text-gray-900 dark:text-[#e0e0e0]">
                      {dashboard.current_download_speed}k / {dashboard.current_upload_speed}k
                    </p>
                    {dashboard.monthly_fup_level > 0 ? (
                      <p className="text-[11px] text-[#FF9800]">Monthly FUP</p>
                    ) : dashboard.fup_level > 0 ? (
                      <p className="text-[11px] text-[#FF9800]">Daily FUP</p>
                    ) : null}
                  </div>
                </div>
              </div>

              {/* IP Address */}
              <div className="stat-card">
                <div className="flex items-center gap-2">
                  <UserCircleIcon className="w-4 h-4 text-[#9C27B0]" />
                  <div>
                    <p className="text-[11px] text-gray-500 dark:text-[#aaa]">IP Address</p>
                    <p className="text-[12px] font-bold text-gray-900 dark:text-[#e0e0e0]">{dashboard.ip_address || 'N/A'}</p>
                  </div>
                </div>
              </div>

              {/* MAC Address */}
              <div className="stat-card">
                <div className="flex items-center gap-2">
                  <ClockIcon className="w-4 h-4 text-[#FF9800]" />
                  <div>
                    <p className="text-[11px] text-gray-500 dark:text-[#aaa]">MAC Address</p>
                    <p className="text-[11px] font-bold text-gray-900 dark:text-[#e0e0e0]">{dashboard.mac_address || 'N/A'}</p>
                  </div>
                </div>
              </div>

              {/* Monthly Price */}
              {dashboard.price > 0 && (
                <div className="stat-card">
                  <div className="flex items-center gap-2">
                    <BanknotesIcon className="w-4 h-4 text-[#4CAF50]" />
                    <div>
                      <p className="text-[11px] text-gray-500 dark:text-[#aaa]">Monthly Price</p>
                      <p className="text-[12px] font-bold text-gray-900 dark:text-[#e0e0e0]">
                        ${dashboard.price.toFixed(2)}
                        {dashboard.override_price && (
                          <span className="ml-1 text-[11px] text-[#FF9800]" title="Custom price">*</span>
                        )}
                      </p>
                    </div>
                  </div>
                </div>
              )}

              {/* Wallet Balance */}
              <div className="stat-card" style={{ cursor: 'pointer' }} onClick={() => setActiveTab('wallet')}>
                <div className="flex items-center gap-2">
                  <BanknotesIcon className="w-4 h-4 text-[#2196F3]" />
                  <div>
                    <p className="text-[11px] text-gray-500 dark:text-[#aaa]">Wallet Balance</p>
                    <p className={`text-[12px] font-bold ${(dashboard.balance || 0) > 0 ? 'text-[#4CAF50]' : 'text-gray-400 dark:text-gray-500'}`}>
                      ${(dashboard.balance || 0).toFixed(2)}
                    </p>
                  </div>
                </div>
              </div>
            </div>

            {/* Usage Cards */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
              {/* Daily Usage */}
              <div className="wb-group">
                <div className="wb-group-title flex items-center gap-1">
                  <CalendarDaysIcon className="w-4 h-4 text-[#316AC5]" />
                  Daily Usage
                  <span className="ml-auto text-[10px] text-gray-400 dark:text-[#888] font-normal">Resets at midnight</span>
                </div>
                <div className="wb-group-body">
                  <div className="flex justify-around items-center py-2">
                    <CircularProgressRing
                      label="Download"
                      value={dashboard.daily_download_used}
                      total={dashboard.daily_quota}
                    />
                    <CircularProgressRing
                      label="Upload"
                      value={dashboard.daily_upload_used}
                      total={dashboard.daily_upload_quota || dashboard.daily_quota}
                    />
                  </div>
                </div>
              </div>

              {/* Monthly Usage */}
              <div className="wb-group">
                <div className="wb-group-title flex items-center gap-1">
                  <ChartBarIcon className="w-4 h-4 text-[#4CAF50]" />
                  Monthly Usage
                  {dashboard.monthly_reset_date && (
                    <span className="ml-auto text-[10px] text-gray-400 dark:text-[#888] font-normal">
                      Resets {formatDate(dashboard.monthly_reset_date)}
                    </span>
                  )}
                </div>
                <div className="wb-group-body">
                  <div className="flex justify-around items-center py-2">
                    <CircularProgressRing
                      label="Download"
                      value={dashboard.monthly_download_used}
                      total={dashboard.monthly_quota}
                    />
                    <CircularProgressRing
                      label="Upload"
                      value={dashboard.monthly_upload_used}
                      total={dashboard.monthly_upload_quota || dashboard.monthly_quota}
                    />
                  </div>
                </div>
              </div>
            </div>

            {/* Profile Info */}
            <div className="wb-group">
              <div className="wb-group-title">Profile Information</div>
              <div className="wb-group-body">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <div>
                    <div className="label">Full Name</div>
                    <div className="text-[12px] text-gray-900 dark:text-[#e0e0e0]">{dashboard.full_name || '-'}</div>
                  </div>
                  <div>
                    <div className="label">Username</div>
                    <div className="text-[12px] text-gray-900 dark:text-[#e0e0e0]">{dashboard.username}</div>
                  </div>
                  <div>
                    <div className="label">Email</div>
                    <div className="text-[12px] text-gray-900 dark:text-[#e0e0e0]">{dashboard.email || '-'}</div>
                  </div>
                  <div>
                    <div className="label">Phone</div>
                    <div className="text-[12px] text-gray-900 dark:text-[#e0e0e0]">{dashboard.phone || '-'}</div>
                  </div>
                  <div className="md:col-span-2">
                    <div className="label">Address</div>
                    <div className="text-[12px] text-gray-900 dark:text-[#e0e0e0]">{dashboard.address || '-'}</div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {activeTab === 'sessions' && (
          <div className="wb-group">
            <div className="wb-group-title">Session History (Last 30 Days)</div>
            <div className="table-container">
              <table className="table">
                <thead>
                  <tr>
                    <th>Start Time</th>
                    <th>Duration</th>
                    <th>IP Address</th>
                    <th>Download</th>
                    <th>Upload</th>
                  </tr>
                </thead>
                <tbody>
                  {sessions.length === 0 ? (
                    <tr>
                      <td colSpan="5" className="text-center py-4 text-gray-500 dark:text-[#aaa]">
                        No session history found
                      </td>
                    </tr>
                  ) : (
                    sessions.map((session, idx) => (
                      <tr key={idx}>
                        <td>{session.start_time ? formatDateTime(session.start_time) : '-'}</td>
                        <td>{formatDuration(session.duration)}</td>
                        <td className="font-mono">{session.ip_address || '-'}</td>
                        <td className="text-[#316AC5]">{formatBytes(session.bytes_out)}</td>
                        <td className="text-[#4CAF50]">{formatBytes(session.bytes_in)}</td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {activeTab === 'usage' && (
          <div className="wb-group">
            <div className="wb-group-title">Daily Usage History</div>
            <div className="table-container">
              <table className="table">
                <thead>
                  <tr>
                    <th>Date</th>
                    <th>Download</th>
                    <th>Upload</th>
                    <th>Sessions</th>
                  </tr>
                </thead>
                <tbody>
                  {usageHistory.length === 0 ? (
                    <tr>
                      <td colSpan="4" className="text-center py-4 text-gray-500 dark:text-[#aaa]">
                        No usage history found
                      </td>
                    </tr>
                  ) : (
                    usageHistory.map((usage, idx) => (
                      <tr key={idx}>
                        <td className="font-medium">{usage.date}</td>
                        <td className="text-[#316AC5]">{formatBytes(usage.download)}</td>
                        <td className="text-[#4CAF50]">{formatBytes(usage.upload)}</td>
                        <td>{usage.sessions}</td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {activeTab === 'invoices' && !viewInvoiceId && (
          <div className="space-y-3">
            <div className="card p-3">
              <h3 className="text-[12px] font-semibold text-gray-900 dark:text-white mb-3 pb-1 border-b border-[#ccc] dark:border-[#555]">My Invoices</h3>
              {invoices.length === 0 ? (
                <div className="text-center text-gray-500 dark:text-gray-400 py-8 text-[12px]">
                  No invoices found
                </div>
              ) : (
                <div className="table-container">
                  <table className="table">
                    <thead>
                      <tr>
                        <th>Date</th>
                        <th>Invoice #</th>
                        <th>Amount</th>
                        <th>Paid</th>
                        <th>Status</th>
                        <th>Due Date</th>
                        <th>Type</th>
                        <th style={{ textAlign: 'right' }}>Actions</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                      {invoices.map((inv) => (
                        <tr key={inv.id}>
                          <td className="text-[11px]">{inv.created_at ? new Date(inv.created_at).toLocaleDateString() : '-'}</td>
                          <td className="text-[11px] font-medium">{inv.invoice_number}</td>
                          <td className="text-[11px] font-medium">${(inv.total || 0).toFixed(2)}</td>
                          <td className="text-[11px]">${(inv.amount_paid || 0).toFixed(2)}</td>
                          <td>
                            <span className={`inline-block px-1.5 py-0.5 text-[9px] font-semibold rounded ${
                              inv.status === 'completed' ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300' :
                              inv.status === 'overdue' ? 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300' :
                              'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300'
                            }`}>
                              {inv.status}
                            </span>
                          </td>
                          <td className="text-[11px]">{inv.due_date ? new Date(inv.due_date).toLocaleDateString() : '-'}</td>
                          <td>
                            <span className={`inline-block px-1.5 py-0.5 text-[9px] font-semibold rounded ${
                              inv.auto_generated
                                ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300'
                                : 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300'
                            }`}>
                              {inv.auto_generated ? 'Auto' : 'Manual'}
                            </span>
                          </td>
                          <td style={{ textAlign: 'right' }}>
                            <button
                              onClick={() => openInvoice(inv.id)}
                              className="inline-block px-2 py-0.5 text-[10px] font-medium rounded border border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300"
                            >
                              View
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          </div>
        )}

        {activeTab === 'invoices' && viewInvoiceId && (
          <div className="space-y-3">
            <div className="card p-3">
              <div className="flex items-center justify-between mb-3 pb-1 border-b border-[#ccc] dark:border-[#555]">
                <button
                  onClick={closeInvoice}
                  className="text-[11px] text-blue-600 dark:text-blue-400 hover:underline"
                >
                  &larr; Back to Invoices
                </button>
                <button
                  onClick={() => window.print()}
                  className="no-print inline-block px-2 py-0.5 text-[10px] font-medium rounded border border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300"
                >
                  Print / Save PDF
                </button>
              </div>

              {viewLoading ? (
                <div className="flex items-center justify-center py-12">
                  <div className="animate-spin h-5 w-5 border-2 border-[#316AC5] border-t-transparent" style={{ borderRadius: '50%' }}></div>
                </div>
              ) : !viewInvoice ? (
                <div className="text-center text-gray-500 py-12 text-[12px]">Invoice not found</div>
              ) : (
                <div className="invoice-print-area bg-white text-black" style={{ padding: 24, fontSize: 12, fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif" }}>
                  {/* Header */}
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 20 }}>
                    <div>
                      <h1 style={{ fontSize: 20, fontWeight: 700, margin: 0, color: '#1a1a1a' }}>INVOICE</h1>
                      <p style={{ fontSize: 13, color: '#555', margin: '4px 0 0' }}>{viewInvoice.invoice_number}</p>
                    </div>
                    <div style={{ textAlign: 'right' }}>
                      <span style={{ display: 'inline-block', padding: '3px 10px', borderRadius: 4, fontSize: 11, fontWeight: 600, textTransform: 'uppercase', ...(viewInvoice.status === 'completed' ? { background: '#dcfce7', color: '#166534' } : viewInvoice.status === 'failed' ? { background: '#fee2e2', color: '#991b1b' } : { background: '#fef9c3', color: '#854d0e' }) }}>
                        {viewInvoice.status}
                      </span>
                    </div>
                  </div>

                  {/* Dates */}
                  <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 20 }}>
                    <div>
                      <div style={{ marginBottom: 4 }}>
                        <span style={{ fontSize: 10, color: '#888' }}>Created: </span>
                        <span style={{ fontSize: 11 }}>{viewInvoice.created_at ? new Date(viewInvoice.created_at).toLocaleDateString() : '-'}</span>
                      </div>
                      <div>
                        <span style={{ fontSize: 10, color: '#888' }}>Due Date: </span>
                        <span style={{ fontSize: 11, fontWeight: 600 }}>{viewInvoice.due_date ? new Date(viewInvoice.due_date).toLocaleDateString() : '-'}</span>
                      </div>
                      {viewInvoice.paid_date && (
                        <div style={{ marginTop: 4 }}>
                          <span style={{ fontSize: 10, color: '#888' }}>Paid: </span>
                          <span style={{ fontSize: 11, color: '#166534' }}>{new Date(viewInvoice.paid_date).toLocaleDateString()}</span>
                        </div>
                      )}
                    </div>
                    {viewInvoice.billing_period_start && viewInvoice.billing_period_end && (
                      <div style={{ textAlign: 'right' }}>
                        <span style={{ fontSize: 10, color: '#888' }}>Billing Period: </span>
                        <span style={{ fontSize: 11 }}>{new Date(viewInvoice.billing_period_start).toLocaleDateString()} — {new Date(viewInvoice.billing_period_end).toLocaleDateString()}</span>
                      </div>
                    )}
                  </div>

                  {/* Items Table */}
                  <table style={{ width: '100%', borderCollapse: 'collapse', marginBottom: 16 }}>
                    <thead>
                      <tr style={{ borderBottom: '2px solid #e5e7eb' }}>
                        <th style={{ textAlign: 'left', padding: '8px 6px', fontSize: 10, color: '#888', textTransform: 'uppercase', letterSpacing: 0.5 }}>Description</th>
                        <th style={{ textAlign: 'center', padding: '8px 6px', fontSize: 10, color: '#888', textTransform: 'uppercase', width: 60 }}>Qty</th>
                        <th style={{ textAlign: 'right', padding: '8px 6px', fontSize: 10, color: '#888', textTransform: 'uppercase', width: 90 }}>Unit Price</th>
                        <th style={{ textAlign: 'right', padding: '8px 6px', fontSize: 10, color: '#888', textTransform: 'uppercase', width: 90 }}>Total</th>
                      </tr>
                    </thead>
                    <tbody>
                      {(viewInvoice.items || viewInvoice.Items || []).map((item, i) => (
                        <tr key={item.id || i} style={{ borderBottom: '1px solid #f3f4f6' }}>
                          <td style={{ padding: '8px 6px', fontSize: 12 }}>{item.description}</td>
                          <td style={{ padding: '8px 6px', fontSize: 12, textAlign: 'center' }}>{item.quantity}</td>
                          <td style={{ padding: '8px 6px', fontSize: 12, textAlign: 'right' }}>${(item.unit_price || 0).toFixed(2)}</td>
                          <td style={{ padding: '8px 6px', fontSize: 12, textAlign: 'right' }}>${(item.total || item.unit_price * item.quantity || 0).toFixed(2)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>

                  {/* Summary */}
                  <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
                    <div style={{ width: 220 }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', padding: '4px 0', fontSize: 12 }}>
                        <span style={{ color: '#555' }}>Subtotal</span>
                        <span>${(viewInvoice.sub_total || viewInvoice.SubTotal || 0).toFixed(2)}</span>
                      </div>
                      {(viewInvoice.tax || 0) > 0 && (
                        <div style={{ display: 'flex', justifyContent: 'space-between', padding: '4px 0', fontSize: 12 }}>
                          <span style={{ color: '#555' }}>Tax</span>
                          <span>${(viewInvoice.tax).toFixed(2)}</span>
                        </div>
                      )}
                      <div style={{ display: 'flex', justifyContent: 'space-between', padding: '6px 0', fontSize: 14, fontWeight: 700, borderTop: '2px solid #1a1a1a', marginTop: 4 }}>
                        <span>Total</span>
                        <span>${(viewInvoice.total || 0).toFixed(2)}</span>
                      </div>
                      {(viewInvoice.amount_paid || 0) > 0 && (
                        <div style={{ display: 'flex', justifyContent: 'space-between', padding: '4px 0', fontSize: 12 }}>
                          <span style={{ color: '#166534' }}>Amount Paid</span>
                          <span style={{ color: '#166534' }}>-${(viewInvoice.amount_paid).toFixed(2)}</span>
                        </div>
                      )}
                      {(viewInvoice.total - (viewInvoice.amount_paid || 0)) > 0.01 && (
                        <div style={{ display: 'flex', justifyContent: 'space-between', padding: '6px 0', fontSize: 13, fontWeight: 700, borderTop: '1px solid #e5e7eb' }}>
                          <span style={{ color: '#991b1b' }}>Balance Due</span>
                          <span style={{ color: '#991b1b' }}>${(viewInvoice.total - (viewInvoice.amount_paid || 0)).toFixed(2)}</span>
                        </div>
                      )}
                    </div>
                  </div>

                  {viewInvoice.notes && (
                    <div style={{ marginTop: 16, padding: '10px 12px', background: '#f9fafb', borderRadius: 4, border: '1px solid #e5e7eb' }}>
                      <p style={{ fontSize: 10, color: '#888', textTransform: 'uppercase', margin: '0 0 4px' }}>Notes</p>
                      <p style={{ fontSize: 11, color: '#333', margin: 0, whiteSpace: 'pre-wrap' }}>{viewInvoice.notes}</p>
                    </div>
                  )}
                </div>
              )}
            </div>
            <style>{`
              @media print {
                body * { visibility: hidden !important; }
                .invoice-print-area, .invoice-print-area * { visibility: visible !important; }
                .invoice-print-area { position: fixed; left: 0; top: 0; width: 100%; background: white !important; padding: 40px !important; z-index: 99999; }
                .no-print { display: none !important; }
              }
            `}</style>
          </div>
        )}

        {activeTab === 'tickets' && (
          <div className="space-y-3">
            {/* Header with Create Button */}
            <div className="flex items-center justify-between">
              <span className="text-[13px] font-semibold text-gray-900 dark:text-[#e0e0e0]">Support Tickets</span>
              <button
                onClick={() => setShowCreateTicket(true)}
                className="btn btn-primary flex items-center gap-1"
              >
                <PlusIcon className="w-3.5 h-3.5" />
                New Ticket
              </button>
            </div>

            {/* Tickets List */}
            {!selectedTicket ? (
              <div className="wb-group">
                <div className="table-container">
                  <table className="table">
                    <thead>
                      <tr>
                        <th>Ticket #</th>
                        <th>Subject</th>
                        <th>Status</th>
                        <th>Category</th>
                        <th>Date</th>
                        <th>Action</th>
                      </tr>
                    </thead>
                    <tbody>
                      {tickets.length === 0 ? (
                        <tr>
                          <td colSpan="6" className="text-center py-4 text-gray-500 dark:text-[#aaa]">
                            No tickets found. Create your first support ticket!
                          </td>
                        </tr>
                      ) : (
                        tickets.map((ticket) => (
                          <tr key={ticket.id}>
                            <td>
                              <div className="flex items-center gap-1">
                                <span className="font-mono">{ticket.ticket_number}</span>
                                {ticket.has_admin_reply && (
                                  <span className="wb-status-dot bg-[#316AC5]" />
                                )}
                              </div>
                            </td>
                            <td>
                              <div className="flex items-center gap-1">
                                <span className="font-medium">{ticket.subject}</span>
                                {ticket.has_admin_reply && (
                                  <BellAlertIcon className="w-3.5 h-3.5 text-[#316AC5]" title="New reply from support" />
                                )}
                              </div>
                            </td>
                            <td>
                              <span className={
                                ticket.status === 'open' ? 'badge badge-success' :
                                ticket.status === 'pending' ? 'badge badge-warning' :
                                ticket.status === 'closed' ? 'badge badge-gray' :
                                'badge badge-info'
                              }>
                                {ticket.status}
                              </span>
                            </td>
                            <td className="capitalize">{ticket.category}</td>
                            <td>{formatDate(ticket.created_at)}</td>
                            <td>
                              <button
                                onClick={() => fetchTicketDetail(ticket.id)}
                                className="btn btn-sm btn-primary"
                              >
                                View
                              </button>
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </div>
            ) : (
              /* Ticket Detail View */
              <div className="wb-group">
                <div className="wb-group-title flex items-center justify-between">
                  <div>
                    <span className="font-semibold">{selectedTicket.subject}</span>
                    <span className="text-[11px] text-gray-500 dark:text-[#aaa] ml-2">{selectedTicket.ticket_number}</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className={
                      selectedTicket.status === 'open' ? 'badge badge-success' :
                      selectedTicket.status === 'pending' ? 'badge badge-warning' :
                      selectedTicket.status === 'closed' ? 'badge badge-gray' :
                      'badge badge-info'
                    }>
                      {selectedTicket.status}
                    </span>
                    <button
                      onClick={() => setSelectedTicket(null)}
                      className="btn btn-xs"
                    >
                      <XMarkIcon className="w-3.5 h-3.5" />
                    </button>
                  </div>
                </div>

                {/* Messages */}
                <div className="p-3 space-y-2 max-h-96 overflow-y-auto bg-white dark:bg-[#3a3a3a]">
                  {/* Original Message */}
                  <div className="p-2 border border-[#2196F3] bg-[#e3f2fd] dark:bg-[#1a2a4a] text-[12px]" style={{ borderRadius: '2px' }}>
                    <div className="text-[11px] text-gray-500 dark:text-[#aaa] mb-1">
                      You - {formatDateTime(selectedTicket.created_at)}
                    </div>
                    <p className="whitespace-pre-wrap text-gray-900 dark:text-[#e0e0e0]">{selectedTicket.description}</p>
                  </div>

                  {/* Replies */}
                  {selectedTicket.replies?.map((reply) => (
                    <div
                      key={reply.id}
                      className={`p-2 border text-[12px] ${reply.is_admin ? 'border-[#a0a0a0] bg-[#f0f0f0] dark:bg-[#444] dark:border-[#555]' : 'border-[#2196F3] bg-[#e3f2fd] dark:bg-[#1a2a4a]'}`}
                      style={{ borderRadius: '2px' }}
                    >
                      <div className="text-[11px] text-gray-500 dark:text-[#aaa] mb-1">
                        {reply.is_admin ? 'Support' : 'You'} - {formatDateTime(reply.created_at)}
                      </div>
                      <p className="whitespace-pre-wrap text-gray-900 dark:text-[#e0e0e0]">{reply.message}</p>
                    </div>
                  ))}
                </div>

                {/* Reply Form */}
                {selectedTicket.status !== 'closed' && (
                  <form onSubmit={handleReplyTicket} className="p-3 border-t border-[#a0a0a0] dark:border-[#555]">
                    <div className="flex gap-1">
                      <textarea
                        value={replyText}
                        onChange={(e) => setReplyText(e.target.value)}
                        placeholder="Type your reply..."
                        rows={2}
                        className="input flex-1 resize-none"
                      />
                      <button
                        type="submit"
                        disabled={!replyText.trim()}
                        className="btn btn-primary"
                      >
                        <PaperAirplaneIcon className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </form>
                )}
              </div>
            )}
          </div>
        )}

        {/* Create Ticket Modal */}
        {showCreateTicket && (
          <div className="modal-overlay">
            <div className="modal" style={{ maxWidth: '480px', width: '100%' }}>
              <div className="modal-header">
                <span>Create Support Ticket</span>
                <button onClick={() => setShowCreateTicket(false)} className="text-white hover:text-gray-200">
                  <XMarkIcon className="w-4 h-4" />
                </button>
              </div>
              <form onSubmit={handleCreateTicket}>
                <div className="modal-body space-y-3">
                  <div>
                    <label className="label">Subject</label>
                    <input
                      type="text"
                      value={ticketForm.subject}
                      onChange={(e) => setTicketForm({ ...ticketForm, subject: e.target.value })}
                      className="input w-full"
                      placeholder="Brief description of your issue"
                      required
                    />
                  </div>
                  <div>
                    <label className="label">Category</label>
                    <select
                      value={ticketForm.category}
                      onChange={(e) => setTicketForm({ ...ticketForm, category: e.target.value })}
                      className="input w-full"
                    >
                      <option value="general">General</option>
                      <option value="billing">Billing</option>
                      <option value="technical">Technical</option>
                      <option value="other">Other</option>
                    </select>
                  </div>
                  <div>
                    <label className="label">Description</label>
                    <textarea
                      value={ticketForm.description}
                      onChange={(e) => setTicketForm({ ...ticketForm, description: e.target.value })}
                      rows={4}
                      className="input w-full resize-none"
                      placeholder="Detailed description of your issue"
                      required
                    />
                  </div>
                </div>
                <div className="modal-footer">
                  <button
                    type="button"
                    onClick={() => setShowCreateTicket(false)}
                    className="btn"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    className="btn btn-primary"
                  >
                    Create Ticket
                  </button>
                </div>
              </form>
            </div>
          </div>
        )}

        {/* Wallet Tab */}
        {activeTab === 'wallet' && (
          <div className="space-y-3">
            {/* Balance Card */}
            <div className="card p-4 text-center">
              <p className="text-[11px] text-gray-500 dark:text-gray-400 mb-1">Available Balance</p>
              <p className={`text-3xl font-bold ${(dashboard?.balance || 0) > 0 ? 'text-[#4CAF50]' : 'text-gray-400'}`}>
                ${(dashboard?.balance || 0).toFixed(2)}
              </p>
            </div>

            {/* Transaction History */}
            <div className="card p-3">
              <h3 className="text-[12px] font-semibold text-gray-900 dark:text-white mb-2">Transaction History</h3>
              {walletTransactionsLoading ? (
                <div className="text-center py-4 text-gray-500 text-sm">Loading...</div>
              ) : walletTransactions.length === 0 ? (
                <div className="text-center py-6 text-gray-400 text-sm">No transactions yet</div>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-[11px]">
                    <thead>
                      <tr className="border-b border-gray-200 dark:border-gray-600">
                        <th className="text-left py-1.5 px-2 font-semibold text-gray-500 dark:text-gray-400">Date</th>
                        <th className="text-left py-1.5 px-2 font-semibold text-gray-500 dark:text-gray-400">Type</th>
                        <th className="text-left py-1.5 px-2 font-semibold text-gray-500 dark:text-gray-400">Description</th>
                        <th className="text-right py-1.5 px-2 font-semibold text-gray-500 dark:text-gray-400">Amount</th>
                        <th className="text-right py-1.5 px-2 font-semibold text-gray-500 dark:text-gray-400">Balance</th>
                      </tr>
                    </thead>
                    <tbody>
                      {walletTransactions.map((tx) => (
                        <tr key={tx.id} className="border-b border-gray-100 dark:border-gray-700">
                          <td className="py-1.5 px-2 text-gray-600 dark:text-gray-300 whitespace-nowrap">{formatDateTime(tx.created_at)}</td>
                          <td className="py-1.5 px-2">
                            <span className={`px-1.5 py-0.5 rounded text-[10px] font-semibold ${
                              tx.type === 'subscriber_topup'
                                ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                                : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                            }`}>
                              {tx.type === 'subscriber_topup' ? 'Top Up' : 'Purchase'}
                            </span>
                          </td>
                          <td className="py-1.5 px-2 text-gray-600 dark:text-gray-300">{tx.description}</td>
                          <td className={`py-1.5 px-2 text-right font-bold ${tx.amount >= 0 ? 'text-green-600' : 'text-red-600'}`}>
                            {tx.amount >= 0 ? '+' : ''}{tx.amount.toFixed(2)}
                          </td>
                          <td className="py-1.5 px-2 text-right font-medium text-gray-700 dark:text-gray-300">${tx.balance_after.toFixed(2)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          </div>
        )}

        {/* Public IP Tab */}
        {activeTab === 'public-ip' && (
          <div className="space-y-3">
            {publicIPLoading ? (
              <div className="flex justify-center py-8">
                <svg className="animate-spin h-8 w-8 text-[#316AC5]" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                </svg>
              </div>
            ) : publicIPData?.has_assignment ? (
              /* Current Assignment */
              <div className="card p-4">
                <h3 className="text-sm font-bold text-gray-900 dark:text-white mb-3 flex items-center gap-2">
                  <SignalIcon className="w-4 h-4 text-green-600" />
                  Your Public IP
                </h3>
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-4">
                  <div className="bg-blue-50 dark:bg-blue-900/30 rounded p-2 text-center">
                    <div className="text-[10px] text-gray-500 dark:text-gray-400">IP Address</div>
                    <div className="text-sm font-bold text-blue-700 dark:text-blue-300">{publicIPData.assignment.ip_address}</div>
                  </div>
                  <div className="bg-green-50 dark:bg-green-900/30 rounded p-2 text-center">
                    <div className="text-[10px] text-gray-500 dark:text-gray-400">Status</div>
                    <div className="text-sm font-bold text-green-700 dark:text-green-300 capitalize">{publicIPData.assignment.status}</div>
                  </div>
                  <div className="bg-purple-50 dark:bg-purple-900/30 rounded p-2 text-center">
                    <div className="text-[10px] text-gray-500 dark:text-gray-400">Monthly Cost</div>
                    <div className="text-sm font-bold text-purple-700 dark:text-purple-300">
                      {publicIPData.assignment.monthly_price > 0 ? `$${publicIPData.assignment.monthly_price.toFixed(2)}` : 'Free'}
                    </div>
                  </div>
                  <div className="bg-amber-50 dark:bg-amber-900/30 rounded p-2 text-center">
                    <div className="text-[10px] text-gray-500 dark:text-gray-400">Next Billing</div>
                    <div className="text-sm font-bold text-amber-700 dark:text-amber-300">
                      {publicIPData.assignment.next_billing_at ? formatDate(publicIPData.assignment.next_billing_at) : 'N/A'}
                    </div>
                  </div>
                </div>
                {publicIPData.assignment.pool && (
                  <div className="text-xs text-gray-500 dark:text-gray-400 mb-3">
                    Pool: {publicIPData.assignment.pool.name} ({publicIPData.assignment.pool.cidr})
                  </div>
                )}
                <p className="text-[10px] text-gray-500 dark:text-gray-400 italic">
                  Contact your administrator to release or change your public IP.
                </p>
              </div>
            ) : (
              /* No assignment — show info, balance, and available pools */
              <div className="card p-4">
                <h3 className="text-sm font-bold text-gray-900 dark:text-white mb-3 flex items-center gap-2">
                  <SignalIcon className="w-4 h-4 text-blue-500" />
                  Public IP Address
                </h3>

                {/* What is a Public IP */}
                <div className="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-3 mb-4 text-xs">
                  <p className="font-semibold text-blue-800 dark:text-blue-300 mb-1">What is a Public IP?</p>
                  <p className="text-blue-700 dark:text-blue-400 leading-relaxed">
                    A Public IP allows you to access your devices remotely — such as NVR cameras, servers, or smart home systems — from anywhere in the world. It gives your connection a unique address on the internet.
                  </p>
                </div>

                {/* How to get one */}
                <div className="bg-gray-50 dark:bg-gray-700/50 border border-gray-200 dark:border-gray-600 rounded-lg p-3 mb-4 text-xs">
                  <p className="font-semibold text-gray-800 dark:text-gray-200 mb-2">How to get a Public IP:</p>
                  <ol className="list-decimal list-inside space-y-1 text-gray-600 dark:text-gray-400">
                    <li><span className="font-medium text-gray-700 dark:text-gray-300">Add balance</span> to your wallet — contact your service provider to top up your account</li>
                    <li><span className="font-medium text-gray-700 dark:text-gray-300">Purchase</span> a Public IP from the available pools below</li>
                    <li>Your connection will <span className="font-medium text-gray-700 dark:text-gray-300">reconnect automatically</span> with the new IP</li>
                  </ol>
                </div>

                {/* Wallet Balance */}
                <div className={`rounded-lg p-3 mb-4 text-xs flex items-center justify-between ${(dashboard?.balance || 0) > 0 ? 'bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800' : 'bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800'}`}>
                  <div className="flex items-center gap-2">
                    <BanknotesIcon className={`w-5 h-5 ${(dashboard?.balance || 0) > 0 ? 'text-green-600' : 'text-yellow-600'}`} />
                    <div>
                      <p className="text-[10px] text-gray-500 dark:text-gray-400">Your Wallet Balance</p>
                      <p className={`text-lg font-bold ${(dashboard?.balance || 0) > 0 ? 'text-green-700 dark:text-green-400' : 'text-yellow-700 dark:text-yellow-400'}`}>${(dashboard?.balance || 0).toFixed(2)}</p>
                    </div>
                  </div>
                  {(dashboard?.balance || 0) <= 0 && (
                    <span className="text-[10px] text-yellow-700 dark:text-yellow-400 font-medium">Contact your provider to add balance</span>
                  )}
                </div>

                {/* Purchase */}
                {publicIPData?.pools?.length > 0 ? (
                  <div className="space-y-2">
                    {publicIPData.pools.map((pool) => (
                      <div key={pool.id} className="border border-gray-200 dark:border-gray-600 rounded-lg p-3 flex items-center justify-between">
                        <div>
                          <div className="text-sm font-semibold text-gray-900 dark:text-white">Public IP</div>
                          {pool.monthly_price > 0 && (
                            <div className="text-xs font-bold text-green-600 dark:text-green-400">${pool.monthly_price.toFixed(2)}/mo</div>
                          )}
                        </div>
                        {(dashboard?.balance || 0) >= pool.monthly_price && pool.available_ips > 0 ? (
                          <button
                            onClick={() => { if (confirm(`Buy a Public IP for $${pool.monthly_price.toFixed(2)}/mo? This will be deducted from your wallet.`)) handleBuyPublicIP(pool.id) }}
                            className="btn btn-primary btn-sm"
                          >
                            Buy
                          </button>
                        ) : pool.available_ips <= 0 ? (
                          <span className="text-[10px] text-red-500 font-medium">Not available</span>
                        ) : (
                          <span className="text-[10px] text-yellow-600 dark:text-yellow-400 font-medium text-right leading-tight">Insufficient balance<br/>Add ${(pool.monthly_price - (dashboard?.balance || 0)).toFixed(2)} more</span>
                        )}
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="text-xs text-gray-500 dark:text-gray-400">No public IP available at this time. Contact your service provider.</p>
                )}
              </div>
            )}
          </div>
        )}
      </main>
    </div>
  )
}
