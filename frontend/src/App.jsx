import { useEffect, Suspense, lazy } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuthStore } from './store/authStore'
import Layout from './components/Layout'

// Eager load critical pages (login, dashboard)
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import CustomerPortal from './pages/CustomerPortal'
import ChangePassword from './pages/ChangePassword'
import Impersonate from './pages/Impersonate'

// Lazy load all other pages for code splitting
const Subscribers = lazy(() => import('./pages/Subscribers'))
const SubscriberEdit = lazy(() => import('./pages/SubscriberEdit'))
const BandwidthManager = lazy(() => import('./pages/BandwidthManager'))
const BandwidthCustomerEdit = lazy(() => import('./pages/BandwidthCustomerEdit'))
const BandwidthCustomerDetail = lazy(() => import('./pages/BandwidthCustomerDetail'))
const SubscriberImport = lazy(() => import('./pages/SubscriberImport'))
const Services = lazy(() => import('./pages/Services'))
const Nas = lazy(() => import('./pages/Nas'))
const Resellers = lazy(() => import('./pages/Resellers'))
const Sessions = lazy(() => import('./pages/Sessions'))
const Transactions = lazy(() => import('./pages/Transactions'))
const Settings = lazy(() => import('./pages/Settings'))
const Users = lazy(() => import('./pages/Users'))
const Invoices = lazy(() => import('./pages/Invoices'))
const Prepaid = lazy(() => import('./pages/Prepaid'))
const Reports = lazy(() => import('./pages/Reports'))
const AuditLogs = lazy(() => import('./pages/AuditLogs'))
const Logs = lazy(() => import('./pages/Logs'))
const CommunicationRules = lazy(() => import('./pages/CommunicationRules'))
const BandwidthRules = lazy(() => import('./pages/BandwidthRules'))
const FUPCounters = lazy(() => import('./pages/FUPCounters'))
const Tickets = lazy(() => import('./pages/Tickets'))
const Backups = lazy(() => import('./pages/Backups'))
const Permissions = lazy(() => import('./pages/Permissions'))
const ChangeBulk = lazy(() => import('./pages/ChangeBulk'))
const SharingDetection = lazy(() => import('./pages/SharingDetection'))
const CDNList = lazy(() => import('./pages/CDNList'))
const CDNBandwidthRules = lazy(() => import('./pages/CDNBandwidthRules'))
const CDNPortRules = lazy(() => import('./pages/CDNPortRules'))
const Profile = lazy(() => import('./pages/Profile'))
const DiagnosticTools = lazy(() => import('./pages/DiagnosticTools'))
const WhatsAppSettings = lazy(() => import('./pages/WhatsAppSettings'))
const ResellerBranding = lazy(() => import('./pages/ResellerBranding'))
const Collectors = lazy(() => import('./pages/Collectors'))
const CollectorView = lazy(() => import('./pages/CollectorView'))
const NotificationBanners = lazy(() => import('./pages/NotificationBanners'))
const ApiDocs = lazy(() => import('./pages/ApiDocs'))
const WanCheck = lazy(() => import('./pages/WanCheck'))
const SystemDocs = lazy(() => import('./pages/SystemDocs'))
const PublicIPs = lazy(() => import('./pages/PublicIPs'))
const SuperAdmin = lazy(() => import('./pages/SuperAdmin'))
const Signup = lazy(() => import('./pages/Signup'))
const ForgotPassword = lazy(() => import('./pages/ForgotPassword'))
const ResetPassword = lazy(() => import('./pages/ResetPassword'))

// Loading fallback component
function PageLoader() {
  return (
    <div className="flex items-center justify-center min-h-[60vh]">
      <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-600"></div>
    </div>
  )
}

// Admin/Reseller private route - redirects customers to portal, collectors to their page
function PrivateRoute({ children }) {
  const { isAuthenticated, isCustomer, isCollector, refreshUser } = useAuthStore()

  // Refresh user data (including permissions) on mount and periodically
  useEffect(() => {
    if (isAuthenticated && !isCustomer) {
      // Refresh immediately on mount
      refreshUser()

      // Set up periodic refresh every 2 minutes to get updated permissions
      // This allows resellers to see new permissions without logout/login
      const intervalId = setInterval(() => {
        refreshUser()
      }, 2 * 60 * 1000) // 2 minutes

      return () => clearInterval(intervalId)
    }
  }, [isAuthenticated, isCustomer, refreshUser])

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }

  // If logged in as customer, redirect to customer portal
  if (isCustomer) {
    return <Navigate to="/portal" replace />
  }

  return children
}

// Route that redirects collectors to their own page
function DashboardRoute({ children }) {
  const { isCollector } = useAuthStore()
  if (isCollector()) {
    return <Navigate to="/collector" replace />
  }
  return children
}

// Permission-protected route - checks if user has required permission
function PermissionRoute({ children, permission, adminOnly = false }) {
  const { hasPermission, isAdmin } = useAuthStore()

  // Admin-only routes
  if (adminOnly && !isAdmin()) {
    return <AccessDenied />
  }

  // Permission-protected routes
  if (permission && !hasPermission(permission)) {
    return <AccessDenied />
  }

  return children
}

// Access Denied component
function AccessDenied() {
  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh] text-center">
      <div className="text-6xl mb-4">🚫</div>
      <h1 className="text-2xl font-bold text-gray-900 mb-2">Access Denied</h1>
      <p className="text-gray-600 mb-4">You don't have permission to access this page.</p>
      <a href="/" className="btn btn-primary">Go to Dashboard</a>
    </div>
  )
}

function App() {
  return (
    <Routes>
      <Route path="/portal" element={<CustomerPortal />} />
      <Route path="/login" element={<Login />} />
      <Route path="/impersonate" element={<Impersonate />} />
      <Route path="/change-password" element={<ChangePassword />} />
      <Route path="/api-docs" element={<Suspense fallback={<PageLoader />}><ApiDocs /></Suspense>} />
      <Route path="/docs" element={<Suspense fallback={<PageLoader />}><SystemDocs /></Suspense>} />
      <Route path="/super-admin" element={<Suspense fallback={<PageLoader />}><SuperAdmin /></Suspense>} />
      <Route path="/signup" element={<Suspense fallback={<PageLoader />}><Signup /></Suspense>} />
      <Route path="/forgot-password" element={<Suspense fallback={<PageLoader />}><ForgotPassword /></Suspense>} />
      <Route path="/reset-password" element={<Suspense fallback={<PageLoader />}><ResetPassword /></Suspense>} />
      <Route
        path="/*"
        element={
          <PrivateRoute>
            <Layout>
              <Suspense fallback={<PageLoader />}>
                <Routes>
                  <Route path="/" element={<DashboardRoute><Dashboard /></DashboardRoute>} />
                  <Route path="/profile" element={<Profile />} />
                  <Route path="/subscribers" element={<PermissionRoute permission="subscribers.view"><Subscribers /></PermissionRoute>} />
                  <Route path="/subscribers/new" element={<PermissionRoute permission="subscribers.create"><SubscriberEdit /></PermissionRoute>} />
                  <Route path="/subscribers/:id" element={<PermissionRoute permission="subscribers.view"><SubscriberEdit /></PermissionRoute>} />
                  <Route path="/subscribers/import" element={<PermissionRoute permission="subscribers.create"><SubscriberImport /></PermissionRoute>} />
                  <Route path="/bandwidth-manager" element={<PermissionRoute permission="bandwidth_customers.view"><BandwidthManager /></PermissionRoute>} />
                  <Route path="/bandwidth-manager/new" element={<PermissionRoute permission="bandwidth_customers.create"><BandwidthCustomerEdit /></PermissionRoute>} />
                  <Route path="/bandwidth-manager/:id" element={<PermissionRoute permission="bandwidth_customers.view"><BandwidthCustomerDetail /></PermissionRoute>} />
                  <Route path="/bandwidth-manager/:id/edit" element={<PermissionRoute permission="bandwidth_customers.edit"><BandwidthCustomerEdit /></PermissionRoute>} />
                  <Route path="/services" element={<PermissionRoute permission="services.view"><Services /></PermissionRoute>} />
                  <Route path="/nas" element={<PermissionRoute adminOnly><Nas /></PermissionRoute>} />
                  <Route path="/resellers" element={<PermissionRoute permission="resellers.view"><Resellers /></PermissionRoute>} />
                  <Route path="/sessions" element={<PermissionRoute permission="sessions.view"><Sessions /></PermissionRoute>} />
                  <Route path="/transactions" element={<PermissionRoute permission="transactions.view"><Transactions /></PermissionRoute>} />
                  <Route path="/settings" element={<PermissionRoute adminOnly><Settings /></PermissionRoute>} />
                  <Route path="/users" element={<PermissionRoute adminOnly><Users /></PermissionRoute>} />
                  <Route path="/invoices" element={<PermissionRoute permission="invoices.view"><Invoices /></PermissionRoute>} />
                  <Route path="/prepaid" element={<PermissionRoute permission="prepaid.view"><Prepaid /></PermissionRoute>} />
                  <Route path="/reports" element={<PermissionRoute permission="reports.view"><Reports /></PermissionRoute>} />
                  <Route path="/audit" element={<PermissionRoute permission="audit.view"><AuditLogs /></PermissionRoute>} />
                  <Route path="/logs" element={<PermissionRoute permission="logs.view"><Logs /></PermissionRoute>} />
                  <Route path="/communication" element={<PermissionRoute permission="communication.access_module"><CommunicationRules /></PermissionRoute>} />
                  <Route path="/bandwidth" element={<PermissionRoute permission="bandwidth.view"><BandwidthRules /></PermissionRoute>} />
                  <Route path="/fup" element={<PermissionRoute adminOnly><FUPCounters /></PermissionRoute>} />
                  <Route path="/tickets" element={<PermissionRoute permission="tickets.view"><Tickets /></PermissionRoute>} />
                  <Route path="/backups" element={<PermissionRoute adminOnly><Backups /></PermissionRoute>} />
                  <Route path="/permissions" element={<PermissionRoute adminOnly><Permissions /></PermissionRoute>} />
                  <Route path="/change-bulk" element={<PermissionRoute permission="subscribers.change_bulk"><ChangeBulk /></PermissionRoute>} />
                  <Route path="/sharing" element={<PermissionRoute adminOnly><SharingDetection /></PermissionRoute>} />
                  <Route path="/cdn" element={<PermissionRoute adminOnly><CDNList /></PermissionRoute>} />
                  <Route path="/cdn-bandwidth-rules" element={<PermissionRoute adminOnly><CDNBandwidthRules /></PermissionRoute>} />
                  <Route path="/cdn-port-rules" element={<PermissionRoute adminOnly><CDNPortRules /></PermissionRoute>} />
                  <Route path="/diagnostic-tools" element={<PermissionRoute adminOnly><DiagnosticTools /></PermissionRoute>} />
                  <Route path="/whatsapp" element={<PermissionRoute permission="notifications.whatsapp"><WhatsAppSettings /></PermissionRoute>} />
                  <Route path="/reseller-branding" element={<ResellerBranding />} />
                  <Route path="/wan-check" element={<PermissionRoute permission="settings.wan_check"><WanCheck /></PermissionRoute>} />
                  <Route path="/collectors" element={<PermissionRoute permission="collectors.view"><Collectors /></PermissionRoute>} />
                  <Route path="/notification-banners" element={<PermissionRoute permission="communication.notifications"><NotificationBanners /></PermissionRoute>} />
                  <Route path="/collector" element={<CollectorView />} />
                  <Route path="/public-ips" element={<PermissionRoute permission="public_ips.view"><PublicIPs /></PermissionRoute>} />
                  <Route path="*" element={<Navigate to="/" replace />} />
                </Routes>
              </Suspense>
            </Layout>
          </PrivateRoute>
        }
      />
    </Routes>
  )
}

export default App
