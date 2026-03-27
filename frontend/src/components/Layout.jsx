import { useState, useEffect, useRef, useCallback } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import api, { maintenanceApi } from '../services/api'
import { useAuthStore } from '../store/authStore'
import { useBrandingStore } from '../store/brandingStore'
import { useThemeStore } from '../store/themeStore'
import Clock from './Clock'
import LicenseBanner from './LicenseBanner'
import UpdateBanner from './UpdateBanner'
import NotificationBanner from './NotificationBanner'
import UpdateNotification from './UpdateNotification'
import {
  HomeIcon,
  UsersIcon,
  ServerIcon,
  CogIcon,
  ChartBarIcon,
  CreditCardIcon,
  SignalIcon,
  ArrowRightOnRectangleIcon,
  Bars3Icon,
  XMarkIcon,
  UserCircleIcon,
  BuildingOfficeIcon,
  DocumentTextIcon,
  TicketIcon,
  ClipboardDocumentListIcon,
  UserGroupIcon,
  BellAlertIcon,
  AdjustmentsHorizontalIcon,
  ChatBubbleLeftRightIcon,
  CloudArrowUpIcon,
  ShieldCheckIcon,
  QueueListIcon,
  ShieldExclamationIcon,
  GlobeAltIcon,
  WrenchScrewdriverIcon,
  BoltIcon,
  BanknotesIcon,
  DevicePhoneMobileIcon,
  PaintBrushIcon,
  QuestionMarkCircleIcon,
  ChevronRightIcon,
  ChevronDownIcon,
  ChevronUpIcon,
  StopIcon,
  SunIcon,
  MoonIcon,
  Bars2Icon,
  EyeIcon,
  EyeSlashIcon,
  ArrowUturnLeftIcon,
  CheckIcon,
} from '@heroicons/react/24/outline'
import clsx from 'clsx'

// Get saved menu order from localStorage
const getSavedMenuOrder = () => {
  try {
    const saved = localStorage.getItem('menuOrder')
    if (saved) return JSON.parse(saved)
  } catch (e) {
    console.error('Failed to load menu order:', e)
  }
  return null
}

// Save menu order to localStorage
const saveMenuOrder = (order) => {
  try {
    localStorage.setItem('menuOrder', JSON.stringify(order))
  } catch (e) {
    console.error('Failed to save menu order:', e)
  }
}

// Get saved hidden items from localStorage
const getSavedHiddenItems = () => {
  try {
    const saved = localStorage.getItem('menuHidden')
    if (saved) return new Set(JSON.parse(saved))
  } catch (e) {
    console.error('Failed to load hidden items:', e)
  }
  return new Set()
}

// Save hidden items to localStorage
const saveHiddenItems = (hiddenSet) => {
  try {
    localStorage.setItem('menuHidden', JSON.stringify([...hiddenSet]))
  } catch (e) {
    console.error('Failed to save hidden items:', e)
  }
}

// Apply saved order to navigation items
const applyMenuOrder = (navigation, savedOrder) => {
  if (!savedOrder || savedOrder.length === 0) return navigation
  const orderMap = new Map(savedOrder.map((name, index) => [name, index]))
  return [...navigation].sort((a, b) => {
    const orderA = orderMap.has(a.name) ? orderMap.get(a.name) : 999
    const orderB = orderMap.has(b.name) ? orderMap.get(b.name) : 999
    return orderA - orderB
  })
}

// Navigation items with permission requirements and tree structure
const allNavigation = [
  { name: 'Dashboard', href: '/', icon: HomeIcon, permission: 'dashboard.view' },
  { name: 'Subscribers', href: '/subscribers', icon: UsersIcon, permission: 'subscribers.view' },
  { name: 'Bandwidth Mgr', href: '/bandwidth-manager', icon: SignalIcon, permission: 'bandwidth_customers.view' },
  { name: 'Services', href: '/services', icon: CogIcon, permission: 'services.view' },
  {
    name: 'CDN', icon: GlobeAltIcon, permission: 'admin', children: [
      { name: 'CDN List', href: '/cdn', icon: GlobeAltIcon, permission: 'admin' },
      { name: 'Rules', href: '/cdn-bandwidth-rules', icon: AdjustmentsHorizontalIcon, permission: 'admin' },
      { name: 'Port Rules', href: '/cdn-port-rules', icon: BoltIcon, permission: 'admin' },
    ]
  },
  { name: 'Public IPs', href: '/public-ips', icon: GlobeAltIcon, permission: 'public_ips.view' },
  { name: 'NAS/Routers', href: '/nas', icon: ServerIcon, permission: 'nas.view', saasHidden: true },
  { name: 'Resellers', href: '/resellers', icon: BuildingOfficeIcon, permission: 'resellers.view' },
  { name: 'Sessions', href: '/sessions', icon: SignalIcon, permission: 'sessions.view' },
  { name: 'Speed Rules', href: '/bandwidth', icon: AdjustmentsHorizontalIcon, permission: 'bandwidth.view' },
  { name: 'Communication', href: '/communication', icon: BellAlertIcon, permission: 'communication.access_module' },
  { name: 'Notifications', href: '/notification-banners', icon: BellAlertIcon, permission: 'communication.notifications' },
  { name: 'Transactions', href: '/transactions', icon: CreditCardIcon, permission: 'transactions.view' },
  { name: 'Invoices', href: '/invoices', icon: DocumentTextIcon, permission: 'invoices.view' },
  { name: 'Prepaid Cards', href: '/prepaid', icon: TicketIcon, permission: 'prepaid.view' },
  { name: 'Reports', href: '/reports', icon: ChartBarIcon, permission: 'reports.view' },
  { name: 'Tickets', href: '/tickets', icon: ChatBubbleLeftRightIcon, permission: 'tickets.view' },
  { name: 'WhatsApp', href: '/whatsapp', icon: DevicePhoneMobileIcon, permission: 'notifications.whatsapp', resellerOnly: true },
  { name: 'Branding', href: '/reseller-branding', icon: PaintBrushIcon, permission: null, resellerOnly: true, rebrandOnly: true },
  { name: 'Users', href: '/users', icon: UserGroupIcon, permission: 'users.view' },
  { name: 'Permissions', href: '/permissions', icon: ShieldCheckIcon, permission: 'permissions.view' },
  { name: 'Audit Logs', href: '/audit', icon: ClipboardDocumentListIcon, permission: 'audit.view', saasHidden: true },
  { name: 'Logs', href: '/logs', icon: DocumentTextIcon, permission: 'logs.view', saasHidden: true },
  { name: 'Backups', href: '/backups', icon: CloudArrowUpIcon, permission: 'backups.view', saasHidden: true },
  { name: 'WAN Check', href: '/wan-check', icon: SignalIcon, permission: 'settings.wan_check', resellerOnly: true },
  // Account page hidden from sidebar — managed from license server admin panel
  // { name: 'Account', href: '/account', icon: UserCircleIcon, permission: null, saasOnly: true },
  { name: 'Settings', href: '/settings', icon: CogIcon, permission: 'settings.view' },
  { name: 'Change Bulk', href: '/change-bulk', icon: QueueListIcon, permission: 'subscribers.change_bulk' },
  { name: 'Collectors', href: '/collectors', icon: BanknotesIcon, permission: 'collectors.view' },
  { name: 'Sharing Detection', href: '/sharing', icon: ShieldExclamationIcon, permission: 'admin' },
  { name: 'Diagnostic Tools', href: '/diagnostic-tools', icon: WrenchScrewdriverIcon, permission: 'admin' },
  { name: 'Help / Docs', href: '/docs', icon: QuestionMarkCircleIcon, permission: 'admin', external: true },
  { name: 'My Collections', href: '/collector', icon: BanknotesIcon, permission: null, collectorOnly: true },
]

export default function Layout({ children }) {
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const [sidebarCollapsed, setSidebarCollapsed] = useState(() => {
    try { return localStorage.getItem('sidebarCollapsed') === 'true' } catch { return false }
  })
  const [expandedGroups, setExpandedGroups] = useState({ 'CDN': true })
  const [editMode, setEditMode] = useState(false)
  const [orderedNav, setOrderedNav] = useState([])
  const [hiddenItems, setHiddenItems] = useState(() => getSavedHiddenItems())

  const location = useLocation()
  const navigate = useNavigate()
  const { user, logout, hasPermission, isAdmin, isReseller, isCollector, refreshUser, isSaasMode, checkSaasMode } = useAuthStore()
  const { companyName, fetchBranding, loaded } = useBrandingStore()
  const { theme, toggleTheme } = useThemeStore()
  const queryClient = useQueryClient()
  const inactivityTimer = useRef(null)
  const sessionRemainingRef = useRef(null)
  const [sessionRemaining, setSessionRemaining] = useState(null)
  const [sessionTimeoutMin, setSessionTimeoutMin] = useState(10)
  const [maintenanceBanner, setMaintenanceBanner] = useState(null)
  const [maintenanceDismissed, setMaintenanceDismissed] = useState(false)
  const isSaaS = window.location.hostname.endsWith('.saas.proxrad.com') || window.location.hostname === 'saas.proxrad.com'

  // Fetch active maintenance windows (skip in SaaS)
  useEffect(() => {
    if (isSaaS) return
    const fetchMaintenance = () => {
      maintenanceApi.getActive().then(res => {
        const windows = res.data?.data
        if (windows && windows.length > 0) {
          setMaintenanceBanner(windows[0])
        } else {
          setMaintenanceBanner(null)
        }
      }).catch(() => {})
    }
    fetchMaintenance()
    const interval = setInterval(fetchMaintenance, 300000) // 5 min
    return () => clearInterval(interval)
  }, [])

  // Fetch session_timeout setting (skip in SaaS mode — no per-tenant settings route)
  useEffect(() => {
    if (isSaaS) return
    api.get('/settings/session_timeout').then(res => {
      const val = parseInt(res.data?.data?.value || res.data?.value, 10)
      if (val > 0) setSessionTimeoutMin(val)
    }).catch(() => {})
  }, [])

  // Fetch branding + SaaS mode on mount
  useEffect(() => {
    if (!loaded) {
      fetchBranding()
    }
    checkSaasMode()
  }, [loaded, fetchBranding, checkSaasMode])

  // Refresh reseller balance periodically
  useEffect(() => {
    if (!isReseller()) return
    const interval = setInterval(() => {
      refreshUser()
    }, 60000)
    return () => clearInterval(interval)
  }, [])

  // Auto-refresh all page data every 60 seconds
  useEffect(() => {
    const interval = setInterval(() => {
      queryClient.invalidateQueries()
    }, 60000)
    return () => clearInterval(interval)
  }, [queryClient])

  // Inactivity logout with configurable timeout + countdown
  useEffect(() => {
    const totalSec = sessionTimeoutMin * 60
    const TIMEOUT = totalSec * 1000
    setSessionRemaining(totalSec)
    sessionRemainingRef.current = totalSec

    const resetTimer = () => {
      if (inactivityTimer.current) clearTimeout(inactivityTimer.current)
      sessionRemainingRef.current = totalSec
      setSessionRemaining(totalSec)
      inactivityTimer.current = setTimeout(() => {
        logout()
        navigate('/login?reason=idle')
      }, TIMEOUT)
    }

    const countdownInterval = setInterval(() => {
      sessionRemainingRef.current = Math.max(0, sessionRemainingRef.current - 1)
      setSessionRemaining(sessionRemainingRef.current)
    }, 1000)

    const events = ['mousemove', 'mousedown', 'keydown', 'touchstart', 'scroll', 'click']
    events.forEach(e => window.addEventListener(e, resetTimer, { passive: true }))
    resetTimer()
    return () => {
      events.forEach(e => window.removeEventListener(e, resetTimer))
      if (inactivityTimer.current) clearTimeout(inactivityTimer.current)
      clearInterval(countdownInterval)
    }
  }, [logout, navigate, sessionTimeoutMin])

  // Handle session expired
  useEffect(() => {
    const handleExpired = () => {
      logout()
      navigate('/login?reason=expired')
    }
    window.addEventListener('auth:session-expired', handleExpired)
    return () => window.removeEventListener('auth:session-expired', handleExpired)
  }, [logout, navigate])

  // Filter and order navigation based on permissions + saved order
  useEffect(() => {
    const filtered = allNavigation.filter((item) => {
      // Hide items in SaaS mode
      if (item.saasHidden && isSaasMode) return false
      // Show saasOnly items only in SaaS mode
      if (item.saasOnly && !isSaasMode) return false
      // Collector users only see collectorOnly items
      if (isCollector()) return !!item.collectorOnly
      // Non-collector users never see collectorOnly items
      if (item.collectorOnly) return false
      if (item.rebrandOnly) return isReseller() && user?.reseller?.rebrand_enabled
      if (item.permission === null) return true
      if (item.permission === 'admin') return isAdmin()
      if (item.resellerOnly) return isReseller() && hasPermission(item.permission)
      return hasPermission(item.permission)
    })
    const savedOrder = getSavedMenuOrder()
    const ordered = applyMenuOrder(filtered, savedOrder)
    setOrderedNav(ordered)
  }, [hasPermission, isAdmin, isReseller, isCollector, user, isSaasMode])

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  // Move item up in menu order
  const moveUp = useCallback((index) => {
    if (index <= 0) return
    setOrderedNav(prev => {
      const newNav = [...prev]
      const temp = newNav[index]
      newNav[index] = newNav[index - 1]
      newNav[index - 1] = temp
      saveMenuOrder(newNav.map(item => item.name))
      return newNav
    })
  }, [])

  // Move item down in menu order
  const moveDown = useCallback((index) => {
    setOrderedNav(prev => {
      if (index >= prev.length - 1) return prev
      const newNav = [...prev]
      const temp = newNav[index]
      newNav[index] = newNav[index + 1]
      newNav[index + 1] = temp
      saveMenuOrder(newNav.map(item => item.name))
      return newNav
    })
  }, [])

  // Toggle item visibility
  const toggleHidden = useCallback((name) => {
    setHiddenItems(prev => {
      const next = new Set(prev)
      if (next.has(name)) {
        next.delete(name)
      } else {
        next.add(name)
      }
      saveHiddenItems(next)
      return next
    })
  }, [])

  // Show all hidden items
  const handleShowAll = useCallback(() => {
    setHiddenItems(new Set())
    saveHiddenItems(new Set())
  }, [])

  // Reset menu order and visibility
  const handleResetOrder = () => {
    localStorage.removeItem('menuOrder')
    localStorage.removeItem('menuHidden')
    setHiddenItems(new Set())
    const filtered = allNavigation.filter((item) => {
      if (item.saasHidden && isSaasMode) return false
      if (isCollector()) return !!item.collectorOnly
      if (item.collectorOnly) return false
      if (item.rebrandOnly) return isReseller() && user?.reseller?.rebrand_enabled
      if (item.permission === null) return true
      if (item.permission === 'admin') return isAdmin()
      if (item.resellerOnly) return isReseller() && hasPermission(item.permission)
      return hasPermission(item.permission)
    })
    setOrderedNav(filtered)
  }

  const toggleEditMode = () => {
    setEditMode(!editMode)
  }

  const toggleSidebarCollapsed = () => {
    setSidebarCollapsed(prev => {
      localStorage.setItem('sidebarCollapsed', String(!prev))
      return !prev
    })
  }

  // Edit mode controls bar
  const EditModeControls = () => (
    <div className="flex items-center gap-1 px-2 py-1.5 border-b border-[#a0a0a0] dark:border-[#374151] bg-amber-50 dark:bg-amber-900/30">
      <button
        onClick={handleResetOrder}
        className="flex items-center gap-1 px-1.5 py-0.5 text-[11px] text-gray-600 dark:text-gray-300 hover:bg-amber-100 dark:hover:bg-amber-900/50 rounded"
        title="Reset order and show all"
      >
        <ArrowUturnLeftIcon className="w-3 h-3" />
        Reset
      </button>
      {hiddenItems.size > 0 && (
        <button
          onClick={handleShowAll}
          className="flex items-center gap-1 px-1.5 py-0.5 text-[11px] text-blue-700 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/50 rounded"
          title="Show all hidden items"
        >
          <EyeIcon className="w-3 h-3" />
          Show All
        </button>
      )}
      <div className="flex-1" />
      <button
        onClick={toggleEditMode}
        className="flex items-center gap-1 px-1.5 py-0.5 text-[11px] text-green-700 dark:text-green-400 bg-green-100 dark:bg-green-900/50 hover:bg-green-200 dark:hover:bg-green-900/70 rounded font-medium"
      >
        <CheckIcon className="w-3 h-3" />
        Done
      </button>
    </div>
  )

  const toggleGroup = (name) => {
    setExpandedGroups(prev => ({ ...prev, [name]: !prev[name] }))
  }

  const isItemActive = (item) => {
    if (item.href) return location.pathname === item.href
    if (item.children) return item.children.some(c => location.pathname === c.href)
    return false
  }

  // Render tree navigation
  const renderTreeItems = (isMobile = false) => {
    return orderedNav
      .filter(item => editMode || !hiddenItems.has(item.name))
      .map((item, index) => {
      const Icon = item.icon
      const isActive = isItemActive(item)
      const hasChildren = item.children && item.children.length > 0
      const isExpanded = expandedGroups[item.name]
      const isHidden = hiddenItems.has(item.name)

      // Edit mode: show flat list with up/down arrows and eye toggle
      if (editMode) {
        return (
          <div
            key={item.name}
            className={clsx(
              'flex items-center gap-1 px-1 py-0.5 mx-1 my-0.5 text-[11px] rounded',
              isHidden
                ? 'bg-gray-100 dark:bg-gray-800 text-gray-400 dark:text-gray-500 line-through'
                : 'bg-gray-50 dark:bg-[#374151] text-gray-700 dark:text-gray-200'
            )}
          >
            <div className="flex flex-col">
              <button
                onClick={() => moveUp(index)}
                disabled={index === 0}
                className="p-0.5 hover:bg-gray-200 dark:hover:bg-gray-500 rounded disabled:opacity-30 disabled:cursor-not-allowed"
              >
                <ChevronUpIcon className="w-2.5 h-2.5" />
              </button>
              <button
                onClick={() => moveDown(index)}
                disabled={index === orderedNav.filter(i => editMode || !hiddenItems.has(i.name)).length - 1}
                className="p-0.5 hover:bg-gray-200 dark:hover:bg-gray-500 rounded disabled:opacity-30 disabled:cursor-not-allowed"
              >
                <ChevronDownIcon className="w-2.5 h-2.5" />
              </button>
            </div>
            <button
              onClick={() => toggleHidden(item.name)}
              className={clsx(
                'p-0.5 rounded hover:bg-gray-200 dark:hover:bg-gray-500',
                isHidden ? 'text-red-400 dark:text-red-500' : 'text-gray-500 dark:text-gray-400'
              )}
              title={isHidden ? 'Show item' : 'Hide item'}
            >
              {isHidden ? <EyeSlashIcon className="w-3 h-3" /> : <EyeIcon className="w-3 h-3" />}
            </button>
            <Icon className={clsx('w-3.5 h-3.5 flex-shrink-0', isHidden ? 'text-gray-400 dark:text-gray-600' : 'text-gray-500 dark:text-gray-400')} />
            <span className="truncate flex-1">{item.name}</span>
          </div>
        )
      }

      if (hasChildren) {
        // Filter children by permissions
        const visibleChildren = item.children.filter(child => {
          if (child.permission === 'admin') return isAdmin()
          return hasPermission(child.permission)
        })
        if (visibleChildren.length === 0) return null

        return (
          <div key={item.name}>
            <div
              className={clsx(
                'wb-tree-item',
                isActive && !isExpanded && 'active'
              )}
              onClick={() => toggleGroup(item.name)}
            >
              <span className="wb-tree-arrow">
                {isExpanded ? '▼' : '▶'}
              </span>
              <Icon className="wb-tree-icon" />
              <span className="truncate">{item.name}</span>
            </div>
            {isExpanded && visibleChildren.map(child => {
              const ChildIcon = child.icon
              const isChildActive = location.pathname === child.href
              return (
                <Link
                  key={child.href}
                  to={child.href}
                  onClick={isMobile ? () => setSidebarOpen(false) : undefined}
                  className={clsx('wb-tree-item pl-7', isChildActive && 'active')}
                >
                  <ChildIcon className="wb-tree-icon" />
                  <span className="truncate">{child.name}</span>
                </Link>
              )
            })}
          </div>
        )
      }

      if (item.external) {
        return (
          <a
            key={item.href}
            href={item.href}
            target="_blank"
            rel="noopener noreferrer"
            onClick={isMobile ? () => setSidebarOpen(false) : undefined}
            className={clsx('wb-tree-item')}
          >
            <span className="wb-tree-arrow" />
            <Icon className="wb-tree-icon" />
            <span className="truncate">{item.name}</span>
          </a>
        )
      }

      return (
        <Link
          key={item.href}
          to={item.href}
          onClick={isMobile ? () => setSidebarOpen(false) : undefined}
          className={clsx('wb-tree-item', isActive && 'active')}
        >
          <span className="wb-tree-arrow" />
          <Icon className="wb-tree-icon" />
          <span className="truncate">{item.name}</span>
        </Link>
      )
    })
  }

  const serverIP = window.location.hostname
  const username = user?.username || 'admin'
  const [appVersion, setAppVersion] = useState('')
  useEffect(() => {
    fetch('/health').then(r => r.json()).then(data => {
      const v = data?.version
      if (v && v !== 'unknown') setAppVersion('v' + v.replace(/^v/, ''))
    }).catch(() => {})
  }, [])
  const userType = user?.user_type === 4 ? 'Admin' : user?.user_type === 2 ? 'Reseller' : 'User'

  return (
    <div className="h-screen flex flex-col font-segoe text-[13px]">
      {/* ================================================================
          TITLE BAR - Blue gradient with connection info
          ================================================================ */}
      <div
        className="flex items-center justify-between h-7 px-3 text-white text-[12px] select-none flex-shrink-0"
        style={{ background: 'linear-gradient(to bottom, #4a7ab5, #2d5a87)' }}
      >
        <div className="flex items-center gap-2 min-w-0">
          {/* Mobile hamburger */}
          <button
            onClick={() => setSidebarOpen(true)}
            className="lg:hidden p-0.5 hover:bg-white/20 rounded-sm"
          >
            <Bars3Icon className="w-4 h-4" />
          </button>
          {/* Desktop sidebar toggle */}
          <button
            onClick={toggleSidebarCollapsed}
            className="hidden lg:flex p-0.5 hover:bg-white/20 rounded-sm"
            title={sidebarCollapsed ? 'Show sidebar' : 'Hide sidebar'}
          >
            <Bars3Icon className="w-4 h-4" />
          </button>
          <span className="truncate font-semibold">
            {username}@{serverIP} — {companyName || 'MES ISP Management'} {appVersion}
          </span>
        </div>
        <div className="flex items-center gap-3 flex-shrink-0">
          {isReseller() && (
            <span className={clsx(
              'font-mono text-[11px]',
              (user?.reseller?.balance ?? 0) >= 0 ? 'text-green-300' : 'text-red-300'
            )}>
              Balance: ${parseFloat(user?.reseller?.balance ?? 0).toFixed(2)}
            </span>
          )}
          <Clock sessionRemaining={sessionRemaining} />
          {!isSaaS && <UpdateNotification />}
          {/* Window controls — profile & logout visible on all screens, others desktop only */}
          <div className="flex items-center gap-0.5 ml-2">
            <Link
              to="/profile"
              className="w-5 h-4 flex items-center justify-center text-[11px] hover:bg-white/20 rounded-sm"
              title="My Profile"
            >
              <UserCircleIcon className="w-3 h-3" />
            </Link>
            {(isAdmin() || isReseller()) && (
              <button
                onClick={toggleEditMode}
                className="hidden lg:flex w-5 h-4 items-center justify-center text-[11px] hover:bg-white/20 rounded-sm"
                title="Customize Menu"
              >
                <Bars2Icon className="w-3 h-3" />
              </button>
            )}
            <button
              onClick={toggleTheme}
              className="w-5 h-4 flex items-center justify-center text-[11px] hover:bg-white/20 rounded-sm"
              title={theme === 'light' ? 'Dark mode' : 'Light mode'}
            >
              {theme === 'dark' ? <SunIcon className="w-3 h-3" /> : <MoonIcon className="w-3 h-3" />}
            </button>
            <button
              onClick={() => {
                if (document.fullscreenElement) {
                  document.exitFullscreen()
                } else {
                  document.documentElement.requestFullscreen()
                }
              }}
              className="hidden lg:flex w-5 h-4 items-center justify-center text-[11px] hover:bg-white/20 rounded-sm"
              title="Toggle Fullscreen"
            >
              <StopIcon className="w-3 h-3" />
            </button>
            <button
              onClick={handleLogout}
              className="w-5 h-4 flex items-center justify-center text-[11px] hover:bg-red-500 rounded-sm"
              title="Logout"
            >
              <XMarkIcon className="w-3 h-3" />
            </button>
          </div>
        </div>
      </div>


      {/* License/Update banners (skip in SaaS — managed by platform) */}
      {!isSaaS && <LicenseBanner />}
      {!isSaaS && <UpdateBanner />}
      {!isSaaS && <NotificationBanner />}
      {/* Maintenance banner */}
      {maintenanceBanner && !maintenanceDismissed && (
        <div className="flex items-center justify-between px-3 py-1.5 bg-yellow-100 dark:bg-yellow-900/40 border-b border-yellow-300 dark:border-yellow-700 text-yellow-800 dark:text-yellow-200 text-[11px]">
          <span>
            <span className="font-semibold mr-1">{maintenanceBanner.title}:</span>
            {maintenanceBanner.message}
          </span>
          <button onClick={() => setMaintenanceDismissed(true)} className="ml-2 hover:bg-yellow-200 dark:hover:bg-yellow-800 rounded p-0.5" title="Dismiss">
            <XMarkIcon className="w-3.5 h-3.5" />
          </button>
        </div>
      )}

      {/* ================================================================
          MAIN AREA - Sidebar + Content
          ================================================================ */}
      <div className="flex flex-1 min-h-0">

        {/* Mobile sidebar backdrop */}
        {sidebarOpen && (
          <div
            className="fixed inset-0 z-40 bg-black bg-opacity-50 lg:hidden"
            onClick={() => setSidebarOpen(false)}
          />
        )}

        {/* ============================================================
            SIDEBAR - Tree navigation
            ============================================================ */}
        {/* Mobile sidebar */}
        <div
          className={clsx(
            'fixed inset-y-0 left-0 z-50 w-52 bg-white dark:bg-[#1f2937] border-r border-[#a0a0a0] dark:border-[#374151] flex flex-col transform transition-transform lg:hidden',
            sidebarOpen ? 'translate-x-0' : '-translate-x-full'
          )}
        >
          <div className="flex items-center justify-between h-7 px-2 border-b border-[#a0a0a0] dark:border-[#374151] bg-[#f0f0f0] dark:bg-[#111827] flex-shrink-0">
            <span onClick={toggleTheme} className="text-[12px] font-semibold truncate cursor-pointer hover:text-[#316AC5] dark:text-[#f3f4f6]" title={theme === 'light' ? 'Dark Mode' : 'Light Mode'}>{companyName || 'MES ISP'}</span>
            <button onClick={() => setSidebarOpen(false)} className="p-0.5 hover:bg-gray-200 dark:hover:bg-[#374151] rounded-sm dark:text-[#f3f4f6]">
              <XMarkIcon className="w-4 h-4" />
            </button>
          </div>
          {editMode && <EditModeControls />}
          <nav className="flex-1 py-1 overflow-y-auto dark:text-[#f3f4f6]">
            {renderTreeItems(true)}
          </nav>
        </div>

        {/* Desktop sidebar */}
        {!sidebarCollapsed && (
        <div className="hidden lg:flex lg:flex-col lg:w-[185px] bg-white dark:bg-[#1f2937] border-r border-[#a0a0a0] dark:border-[#374151] flex-shrink-0">
          <div
            onClick={toggleTheme}
            className="flex items-center h-7 px-2 border-b border-[#a0a0a0] dark:border-[#374151] bg-[#f0f0f0] dark:bg-[#111827] cursor-pointer hover:bg-[#e0e0e0] dark:hover:bg-[#374151] flex-shrink-0"
            title={theme === 'light' ? 'Switch to Dark Mode' : 'Switch to Light Mode'}
          >
            <span className="text-[12px] font-semibold truncate dark:text-[#f3f4f6]">{companyName || 'MES ISP Management'}</span>
          </div>
          {editMode && <EditModeControls />}
          <nav className="flex-1 py-1 overflow-y-auto dark:text-[#f3f4f6]">
            {renderTreeItems(false)}
          </nav>
        </div>
        )}

        {/* ============================================================
            CONTENT AREA
            ============================================================ */}
        <div className="flex-1 flex flex-col min-w-0 bg-[#f0f0f0] dark:bg-[#111827]">
          {/* Page content - scrollable */}
          <main className="flex-1 overflow-auto p-2 lg:p-3">
            {children}
          </main>

          {/* ============================================================
              STATUS BAR
              ============================================================ */}
          <div className="wb-statusbar flex-shrink-0 text-[11px]">
            <span>{userType}: {username}</span>
            <span>
              {isReseller() && (
                <span className={clsx(
                  'mr-3',
                  (user?.reseller?.balance ?? 0) >= 0 ? 'text-green-700' : 'text-red-700'
                )}>
                  Balance: ${parseFloat(user?.reseller?.balance ?? 0).toFixed(2)}
                </span>
              )}
              {serverIP}
            </span>
          </div>
        </div>
      </div>
    </div>
  )
}
