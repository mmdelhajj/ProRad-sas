import axios from 'axios'

const api = axios.create({
  baseURL: '/api',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Token refresh state
let isRefreshing = false
let refreshSubscribers = []

const onRefreshed = (token) => {
  refreshSubscribers.forEach((callback) => callback(token))
  refreshSubscribers = []
}

const addRefreshSubscriber = (callback) => {
  refreshSubscribers.push(callback)
}

// Parse JWT to get expiration
const parseJwt = (token) => {
  try {
    const base64Url = token.split('.')[1]
    const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/')
    const jsonPayload = decodeURIComponent(
      atob(base64)
        .split('')
        .map((c) => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2))
        .join('')
    )
    return JSON.parse(jsonPayload)
  } catch (e) {
    return null
  }
}

// Check if token needs refresh (within 1 hour of expiry)
const shouldRefreshToken = (token) => {
  if (!token) return false
  const payload = parseJwt(token)
  if (!payload || !payload.exp) return false
  const expiresAt = payload.exp * 1000 // Convert to milliseconds
  const now = Date.now()
  const oneHour = 60 * 60 * 1000
  return expiresAt - now < oneHour && expiresAt > now
}

// Request interceptor - auto refresh token if needed
api.interceptors.request.use(
  async (config) => {
    // Skip refresh for auth endpoints
    if (config.url?.includes('/auth/')) {
      return config
    }

    // Check impersonated session first (sessionStorage), then normal session (localStorage)
    const impersonateData = sessionStorage.getItem('proisp-impersonate')
    const authData = impersonateData || localStorage.getItem('proisp-auth')
    if (!authData) return config

    try {
      const parsed = JSON.parse(authData)
      const token = parsed?.token

      if (token && shouldRefreshToken(token)) {
        if (!isRefreshing) {
          isRefreshing = true
          try {
            const response = await axios.post('/api/auth/refresh', null, {
              headers: { Authorization: `Bearer ${token}` },
            })
            if (response.data.success && response.data.token) {
              const newToken = response.data.token
              // Update stored token in the correct storage
              parsed.token = newToken
              if (impersonateData) {
                sessionStorage.setItem('proisp-impersonate', JSON.stringify(parsed))
              } else {
                localStorage.setItem('proisp-auth', JSON.stringify(parsed))
              }
              api.defaults.headers.common['Authorization'] = `Bearer ${newToken}`
              config.headers['Authorization'] = `Bearer ${newToken}`
              onRefreshed(newToken)
            } else {
              // Refresh didn't return new token, use old token
              config.headers['Authorization'] = `Bearer ${token}`
              onRefreshed(token)
            }
          } catch (err) {
            // Refresh failed, continue with old token and notify subscribers
            config.headers['Authorization'] = `Bearer ${token}`
            onRefreshed(token)
          } finally {
            isRefreshing = false
          }
        } else {
          // Wait for ongoing refresh with timeout
          return new Promise((resolve) => {
            const timeout = setTimeout(() => {
              // Timeout after 10 seconds, continue with old token
              config.headers['Authorization'] = `Bearer ${token}`
              resolve(config)
            }, 10000)
            addRefreshSubscriber((newToken) => {
              clearTimeout(timeout)
              config.headers['Authorization'] = `Bearer ${newToken}`
              resolve(config)
            })
          })
        }
      } else if (token) {
        // Token doesn't need refresh, just add it to the request
        config.headers['Authorization'] = `Bearer ${token}`
      }
    } catch (e) {
      // Parse error, continue without refresh
    }

    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// Response interceptor
api.interceptors.response.use(
  (response) => {
    return response
  },
  (error) => {
    // Handle 401 Unauthorized - token expired or invalid
    // Skip for auth/login endpoints where 401 is expected (wrong credentials)
    const requestUrl = error.config?.url || ''
    if (error.response?.status === 401 && !requestUrl.includes('/auth/') && !requestUrl.includes('/customer/login')) {
      const authData = localStorage.getItem('proisp-auth')
      if (authData) {
        try {
          const data = JSON.parse(authData)
          if (data.token) {
            // Token exists but server rejected it → session expired
            localStorage.removeItem('proisp-auth')
            window.dispatchEvent(new CustomEvent('auth:session-expired'))
          }
        } catch (e) { /* ignore */ }
      }
    }

    // Handle license-related errors
    if (error.response?.status === 403) {
      const code = error.response.data?.code
      if (code === 'LICENSE_READONLY') {
        // System is in read-only mode - enhance error message
        error.licenseReadOnly = true
        error.response.data.message = 'System is in read-only mode due to expired license. Please renew your license to make changes.'
      } else if (code === 'LICENSE_GRACE_PERIOD') {
        // Grace period - can't create new records
        error.licenseGracePeriod = true
        error.response.data.message = 'License expired. Creating new records is disabled during grace period. Please renew your license.'
      } else if (code === 'LICENSE_INVALID') {
        // License is blocked
        error.licenseBlocked = true
      }
    }

    // Handle 402 Payment Required (license blocked)
    if (error.response?.status === 402) {
      error.licenseBlocked = true
      error.response.data.message = 'License expired or invalid. Please contact support.'
    }

    return Promise.reject(error)
  }
)

export default api

// API helper functions
export const bandwidthCustomerApi = {
  list: (params) => api.get('/bandwidth-customers', { params }),
  get: (id) => api.get(`/bandwidth-customers/${id}`),
  create: (data) => api.post('/bandwidth-customers', data),
  update: (id, data) => api.put(`/bandwidth-customers/${id}`, data),
  delete: (id) => api.delete(`/bandwidth-customers/${id}`),
  suspend: (id) => api.post(`/bandwidth-customers/${id}/suspend`),
  unsuspend: (id) => api.post(`/bandwidth-customers/${id}/unsuspend`),
  resetFup: (id) => api.post(`/bandwidth-customers/${id}/reset-fup`),
  changeSpeed: (id, data) => api.post(`/bandwidth-customers/${id}/change-speed`, data),
  getBandwidth: (id) => api.get(`/bandwidth-customers/${id}/bandwidth`),
  getUsage: (id, days = 30) => api.get(`/bandwidth-customers/${id}/usage?days=${days}`),
  getStats: () => api.get('/bandwidth-customers/stats'),
  getHourlyUsage: (id, days = 7) => api.get(`/bandwidth-customers/${id}/hourly-usage?days=${days}`),
  getSessions: (id, days = 30) => api.get(`/bandwidth-customers/${id}/sessions?days=${days}`),
  getHeatmap: (id, days = 30) => api.get(`/bandwidth-customers/${id}/heatmap?days=${days}`),
}

export const bwIPBlockApi = {
  list: () => api.get('/bw-ip-blocks'),
  create: (data) => api.post('/bw-ip-blocks', data),
  get: (id) => api.get(`/bw-ip-blocks/${id}`),
  delete: (id) => api.delete(`/bw-ip-blocks/${id}`),
  getAvailableIPs: (id) => api.get(`/bw-ip-blocks/${id}/available-ips`),
  assignIP: (blockId, data) => api.post(`/bw-ip-blocks/${blockId}/assign`, data),
  releaseIP: (blockId, allocId) => api.post(`/bw-ip-blocks/${blockId}/release/${allocId}`),
}

export const subscriberApi = {
  list: (params) => api.get('/subscribers', { params }),
  listArchived: (params) => api.get('/subscribers/archived', { params }),
  getArchivedStats: () => api.get('/subscribers/archived', { params: { page: 1, limit: 1 } }),
  get: (id) => api.get(`/subscribers/${id}`),
  create: (data) => api.post('/subscribers', data),
  update: (id, data) => api.put(`/subscribers/${id}`, data),
  delete: (id) => api.delete(`/subscribers/${id}`),
  renew: (id, data) => api.post(`/subscribers/${id}/renew`, data),
  disconnect: (id) => api.post(`/subscribers/${id}/disconnect`),
  resetFup: (id) => api.post(`/subscribers/${id}/reset-fup`),
  resetMac: (id, data) => api.post(`/subscribers/${id}/reset-mac`, data),
  restore: (id) => api.post(`/subscribers/${id}/restore`),
  permanentDelete: (id) => api.delete(`/subscribers/${id}/permanent`),
  // New action endpoints
  rename: (id, data) => api.post(`/subscribers/${id}/rename`, data),
  addDays: (id, data) => api.post(`/subscribers/${id}/add-days`, data),
  calculateChangeServicePrice: (id, serviceId) => api.get(`/subscribers/${id}/calculate-change-service-price?service_id=${serviceId}`),
  changeService: (id, data) => api.post(`/subscribers/${id}/change-service`, data),
  activate: (id) => api.post(`/subscribers/${id}/activate`),
  deactivate: (id) => api.post(`/subscribers/${id}/deactivate`),
  refill: (id, data) => api.post(`/subscribers/${id}/refill`, data),
  addBalance: (id, data) => api.post(`/subscribers/${id}/add-balance`, data),
  ping: (id) => api.post(`/subscribers/${id}/ping`),
  portCheck: (id, port) => api.post(`/subscribers/${id}/port-check`, { port }),
  getPassword: (id) => api.get(`/subscribers/${id}/password`),
  getBandwidth: (id) => api.get(`/subscribers/${id}/bandwidth`),
  getTorch: (id, duration = 3) => api.get(`/subscribers/${id}/torch?duration=${duration}`),
  bulkImport: (formData) => api.post('/subscribers/bulk-import', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  }),
  importExcel: (data) => api.post('/subscribers/import-excel', data),
  bulkUpdate: (data) => api.post('/subscribers/bulk-update', data),
  bulkAction: (data) => api.post('/subscribers/bulk-action', data),
  // Bandwidth rules
  getBandwidthRules: (id) => api.get(`/subscribers/${id}/bandwidth-rules`),
  createBandwidthRule: (id, data) => api.post(`/subscribers/${id}/bandwidth-rules`, data),
  updateBandwidthRule: (id, ruleId, data) => api.put(`/subscribers/${id}/bandwidth-rules/${ruleId}`, data),
  deleteBandwidthRule: (id, ruleId) => api.delete(`/subscribers/${id}/bandwidth-rules/${ruleId}`),
  getCDNUpgrades: (id) => api.get(`/subscribers/${id}/cdn-upgrades`),
  // WAN Management Check
  wanCheckSkip: (id) => api.post(`/subscribers/${id}/wan-check-skip`),
  wanCheckRecheck: (id) => api.post(`/subscribers/${id}/wan-check-recheck`),
}

export const serviceApi = {
  list: (params) => api.get('/services', { params }),
  get: (id) => api.get(`/services/${id}`),
  create: (data) => api.post('/services', data),
  update: (id, data) => api.put(`/services/${id}`, data),
  delete: (id) => api.delete(`/services/${id}`),
}

export const nasApi = {
  list: () => api.get('/nas'),
  get: (id) => api.get(`/nas/${id}`),
  create: (data) => api.post('/nas', data),
  update: (id, data) => api.put(`/nas/${id}`, data),
  delete: (id) => api.delete(`/nas/${id}`),
  sync: (id) => api.post(`/nas/${id}/sync`),
  test: (id) => api.post(`/nas/${id}/test`),
  getPools: (id) => api.get(`/nas/${id}/pools`),
  updatePools: (id, data) => api.put(`/nas/${id}/pools`, data),
  getDashboard: (id) => api.get(`/nas/${id}/dashboard`),
  getNetworkMap: () => api.get('/nas/network-map'),
  getInterfaces: (id) => api.get(`/nas/${id}/interfaces`),
}

export const resellerApi = {
  list: (params) => api.get('/resellers', { params }),
  get: (id) => api.get(`/resellers/${id}`),
  create: (data) => api.post('/resellers', data),
  update: (id, data) => api.put(`/resellers/${id}`, data),
  delete: (id) => api.delete(`/resellers/${id}`),
  permanentDelete: (id) => api.delete(`/resellers/${id}/permanent`),
  transfer: (id, data) => api.post(`/resellers/${id}/transfer`, data),
  withdraw: (id, data) => api.post(`/resellers/${id}/withdraw`, data),
  impersonate: (id) => api.post(`/resellers/${id}/impersonate`),
  getImpersonateToken: (id) => api.post(`/resellers/${id}/impersonate-token`), // Get temp token for new tab
  // NAS and Service assignments
  getAssignedNAS: (id) => api.get(`/resellers/${id}/assigned-nas`),
  updateAssignedNAS: (id, nasIds) => api.put(`/resellers/${id}/assigned-nas`, { nas_ids: nasIds }),
  getAssignedServices: (id) => api.get(`/resellers/${id}/assigned-services`),
  updateAssignedServices: (id, services) => api.put(`/resellers/${id}/assigned-services`, { services }),
  getServiceLimits: (id) => api.get(`/resellers/${id}/service-limits`),
  setServiceLimits: (id, limits) => api.put(`/resellers/${id}/service-limits`, { limits }),
  // Sub-reseller service limits (reseller context)
  getSubResellerServiceLimits: (id) => api.get(`/reseller/sub-resellers/${id}/service-limits`),
  setSubResellerServiceLimits: (id, limits) => api.put(`/reseller/sub-resellers/${id}/service-limits`, { limits }),
  // Self-service WAN check settings (reseller context)
  getSelfWanSettings: () => api.get('/reseller/wan-settings'),
  updateSelfWanSettings: (data) => api.put('/reseller/wan-settings', data),
}

export const dashboardApi = {
  stats: () => api.get('/dashboard/stats'),
  chart: (params) => api.get('/dashboard/chart', { params }),
  transactions: (params) => api.get('/dashboard/transactions', { params }),
  resellers: (params) => api.get('/dashboard/resellers', { params }),
  sessions: (params) => api.get('/dashboard/sessions', { params }),
  systemMetrics: () => api.get('/dashboard/system-metrics'),
  systemCapacity: () => api.get('/dashboard/system-capacity'),
  systemInfo: () => api.get('/dashboard/system-info'),
  getBandwidthHeatmap: () => api.get('/dashboard/bandwidth-heatmap'),
  getSubnetMap: () => api.get('/dashboard/subnet-map'),
}

export const sessionApi = {
  list: (params) => api.get('/sessions', { params }),
  get: (id) => api.get(`/sessions/${id}`),
  disconnect: (id) => api.post(`/sessions/${id}/disconnect`),
  exportCSV: (params) => api.get('/sessions/export', { params, responseType: 'blob' }),
}

export const sharingApi = {
  list: (params) => api.get('/sharing', { params }),
  stats: () => api.get('/sharing/stats'),
  getSubscriberDetails: (id) => api.get(`/sharing/subscriber/${id}`),
  getNasRuleStatus: () => api.get('/sharing/nas-rules'),
  generateTTLRules: (nasId) => api.post(`/sharing/nas/${nasId}/rules`),
  removeTTLRules: (nasId) => api.delete(`/sharing/nas/${nasId}/rules`),
  getHistory: (params) => api.get('/sharing/history', { params }),
  getTrends: (params) => api.get('/sharing/trends', { params }),
  getRepeatOffenders: (params) => api.get('/sharing/repeat-offenders', { params }),
  getSettings: () => api.get('/sharing/settings'),
  updateSettings: (data) => api.put('/sharing/settings', data),
  runManualScan: () => api.post('/sharing/scan'),
  getMonthlyScores: (params) => api.get('/sharing/scores', { params }),
  getSubscriberScoreHistory: (id) => api.get(`/sharing/scores/subscriber/${id}`),
  toggleWhitelist: (id, data) => api.post(`/sharing/whitelist/${id}`, data),
  getWhitelistedSubscribers: () => api.get('/sharing/whitelist'),
  getActionLogs: (params) => api.get('/sharing/action-logs', { params }),
}

export const ticketApi = {
  list: (params) => api.get('/tickets', { params }),
  stats: () => api.get('/tickets/stats'),
  get: (id) => api.get(`/tickets/${id}`),
  create: (data) => api.post('/tickets', data),
  update: (id, data) => api.put(`/tickets/${id}`, data),
  delete: (id) => api.delete(`/tickets/${id}`),
  addReply: (id, data) => api.post(`/tickets/${id}/reply`, data),
}

export const backupApi = {
  list: () => api.get('/backups'),
  create: (data) => api.post('/backups', data),
  createMikrotik: (nasIds) => api.post('/backups/mikrotik', { nas_ids: nasIds }),
  upload: (formData) => api.post('/backups/upload', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  }),
  download: (filename) => api.get(`/backups/${filename}/download`, { responseType: 'blob' }),
  getDownloadToken: (filename) => api.get(`/backups/${filename}/token`),
  restore: (filename, sourceLicenseKey) =>
    api.post(`/backups/${filename}/restore`, { source_license_key: sourceLicenseKey }),
  delete: (filename) => api.delete(`/backups/${filename}`),
  // Schedules
  listSchedules: () => api.get('/backups/schedules'),
  getSchedule: (id) => api.get(`/backups/schedules/${id}`),
  createSchedule: (data) => api.post('/backups/schedules', data),
  updateSchedule: (id, data) => api.put(`/backups/schedules/${id}`, data),
  deleteSchedule: (id) => api.delete(`/backups/schedules/${id}`),
  toggleSchedule: (id) => api.post(`/backups/schedules/${id}/toggle`),
  runScheduleNow: (id) => api.post(`/backups/schedules/${id}/run`),
  testFTP: (data) => api.post('/backups/test-ftp', data),
  listLogs: (params) => api.get('/backups/logs', { params }),
  // Cloud backup
  cloudList: () => api.get('/backups/cloud/list'),
  cloudUsage: () => api.get('/backups/cloud/usage'),
  cloudUpload: (filename) => api.post(`/backups/${filename}/cloud-upload`),
  cloudDownload: (backupId) => api.get(`/backups/cloud/download/${backupId}`, { responseType: 'blob' }),
  cloudDelete: (backupId) => api.delete(`/backups/cloud/${backupId}`),
  cloudDownloadToken: (backupId) => api.get(`/backups/cloud/${backupId}/token`),
}

// Cloud backup download helper (handles blob response + file save)
export const downloadCloudBackup = async (backupId, filename) => {
  const response = await api.get(`/backups/cloud/download/${backupId}`, {
    responseType: 'blob',
  })
  const url = window.URL.createObjectURL(new Blob([response.data]))
  const link = document.createElement('a')
  link.href = url
  link.setAttribute('download', filename || `cloud-backup-${backupId}.proisp.bak`)
  document.body.appendChild(link)
  link.click()
  link.remove()
  window.URL.revokeObjectURL(url)
}

export const logsApi = {
  listRadius: (params) => api.get('/logs/radius', { params }),
  listAuth: (params) => api.get('/logs/auth', { params }),
  listSystem: (params) => api.get('/logs/system', { params }),
}

export const permissionApi = {
  list: () => api.get('/permissions'),
  seed: () => api.post('/permissions/seed'),
  listGroups: () => api.get('/permissions/groups'),
  getGroup: (id) => api.get(`/permissions/groups/${id}`),
  createGroup: (data) => api.post('/permissions/groups', data),
  updateGroup: (id, data) => api.put(`/permissions/groups/${id}`, data),
  deleteGroup: (id) => api.delete(`/permissions/groups/${id}`),
}

export const invoiceExtApi = {
  calculateProrate: (data) => api.post('/invoices/prorate', data),
  getCommissions: (params) => api.get('/invoices/commissions', { params }),
}

export const reportApi = {
  getRevenueForecast: () => api.get('/reports/revenue-forecast'),
  getResellerPerformance: (params) => api.get('/reports/reseller-performance', { params }),
  getChurnReport: (params) => api.get('/reports/churn', { params }),
}

export const maintenanceApi = {
  getWindows: () => api.get('/settings/maintenance'),
  createWindow: (data) => api.post('/settings/maintenance', data),
  updateWindow: (id, data) => api.put(`/settings/maintenance/${id}`, data),
  deleteWindow: (id) => api.delete(`/settings/maintenance/${id}`),
  getActive: () => axios.get('/api/maintenance/active'),
}

export const settingsApi = {
  list: () => api.get('/settings'),
  get: (key) => api.get(`/settings/${key}`),
  update: (key, value) => api.put(`/settings/${key}`, { key, value }),
  bulkUpdate: (settings) => api.put('/settings/bulk', { settings }),
  getTimezones: () => api.get('/settings/timezones'),
  uploadLogo: (formData) => api.post('/settings/logo', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  }),
  deleteLogo: () => api.delete('/settings/logo'),
  uploadLoginBackground: (formData) => api.post('/settings/login-background', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  }),
  deleteLoginBackground: () => api.delete('/settings/login-background'),
  uploadFavicon: (formData) => api.post('/settings/favicon', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  }),
  deleteFavicon: () => api.delete('/settings/favicon'),
  restartServices: (services) => api.post('/system/restart-services', { services }),
  getSSLStatus: () => api.get('/settings/ssl-status'),
}

export const resellerBrandingApi = {
  get: () => api.get('/reseller/branding'),
  update: (data) => api.put('/reseller/branding', data),
  uploadLogo: (formData) => api.post('/reseller/branding/logo', formData, {
    headers: { 'Content-Type': 'multipart/form-data' }
  }),
  deleteLogo: () => api.delete('/reseller/branding/logo'),
  updateDomain: (domain) => api.put('/reseller/branding/domain', { custom_domain: domain }),
  requestSSL: (email) => api.post('/reseller/branding/ssl', { email }),
}

export const clusterApi = {
  getConfig: () => api.get('/cluster/config'),
  getStatus: () => api.get('/cluster/status'),
  setupMain: (data) => api.post('/cluster/setup-main', data),
  setupSecondary: (data) => api.post('/cluster/setup-secondary', data),
  joinCluster: (data) => api.post('/cluster/join', data),
  leaveCluster: () => api.post('/cluster/leave'),
  removeNode: (id) => api.delete(`/cluster/nodes/${id}`),
  manualFailover: (targetNodeId) => api.post('/cluster/failover', { target_node_id: targetNodeId }),
  testConnection: (data) => api.post('/cluster/test-connection', data),
  checkMainStatus: () => api.get('/cluster/check-main-status'),
  promoteToMain: () => api.post('/cluster/promote-to-main'),
  testSourceConnection: (data) => api.post('/cluster/test-source-connection', data),
  recoverFromServer: (data) => api.post('/cluster/recover-from-server', data),
}

export const cdnApi = {
  list: (params) => api.get('/cdns', { params }),
  get: (id) => api.get(`/cdns/${id}`),
  getSpeeds: () => api.get('/cdns/speeds'), // Get all CDN speeds from services
  create: (data) => api.post('/cdns', data),
  update: (id, data) => api.put(`/cdns/${id}`, data),
  delete: (id) => api.delete(`/cdns/${id}`),
  syncToNAS: (id) => api.post(`/cdns/${id}/sync`),
  syncAllToNAS: () => api.post('/cdns/sync-all'),
  // Service CDN configurations
  listServiceCDNs: (serviceId) => api.get(`/services/${serviceId}/cdns`),
  updateServiceCDNs: (serviceId, data) => api.put(`/services/${serviceId}/cdns`, data),
  addServiceCDN: (serviceId, data) => api.post(`/services/${serviceId}/cdns`, data),
  deleteServiceCDN: (serviceId, cdnId) => api.delete(`/services/${serviceId}/cdns/${cdnId}`),
  // Port Rules
  listPortRules: () => api.get('/cdn-port-rules'),
  createPortRule: (data) => api.post('/cdn-port-rules', data),
  updatePortRule: (id, data) => api.put(`/cdn-port-rules/${id}`, data),
  deletePortRule: (id) => api.delete(`/cdn-port-rules/${id}`),
  syncPortRule: (id) => api.post(`/cdn-port-rules/${id}/sync`),
  syncAllPortRules: () => api.post('/cdn-port-rules/sync-all'),
}

export const notificationBannerApi = {
  getActive: () => api.get('/active-banners'),
  list: () => api.get('/notification-banners'),
  getSubResellers: () => api.get('/notification-banners/sub-resellers'),
  create: (data) => api.post('/notification-banners', data),
  update: (id, data) => api.put(`/notification-banners/${id}`, data),
  delete: (id) => api.delete(`/notification-banners/${id}`),
}

export const notificationApi = {
  // Update notifications
  getPending: () => api.get('/notifications/updates/pending'),
  markRead: (id) => api.post(`/notifications/updates/${id}/read`),
  getSettings: () => api.get('/notifications/updates/settings'),
  updateSettings: (settings) => api.put('/notifications/updates/settings', settings),
  // Test notifications (admin only)
  testSMTP: (data) => api.post('/notifications/test-smtp', data),
  testSMS: (data) => api.post('/notifications/test-sms', data),
  testWhatsApp: (data) => api.post('/notifications/test-whatsapp', data),
  getWhatsAppStatus: () => api.get('/notifications/whatsapp-status'),
}

// Public API - no auth required
export const publicApi = {
  getBranding: () => axios.get('/api/branding'),
  exchangeImpersonateToken: (token) => axios.post('/api/auth/impersonate-exchange', { token }),
}

// Diagnostic Tools API
export const diagnosticApi = {
  ping: (data) => api.post('/diagnostic/ping', data),
  traceroute: (data) => api.post('/diagnostic/traceroute', data),
  nslookup: (data) => api.post('/diagnostic/nslookup', data),
  searchSubscribers: (nasId, query) => api.get(`/diagnostic/search-subscribers?nas_id=${nasId}&q=${query}`),
}

// Network Configuration API
export const networkApi = {
  getCurrentConfig: () => api.get('/system/network/current'),
  detectDNSMethod: () => api.get('/system/network/detect-dns'),
  testConfig: (data) => api.post('/system/network/test', data),
  applyConfig: (data) => api.post('/system/network/apply', data),
}

// Remote Access Tunnel API
export const tunnelApi = {
  getStatus: () => api.get('/system/tunnel/status'),
  enable: () => api.post('/system/tunnel/enable'),
  disable: () => api.post('/system/tunnel/disable'),
  saveCredentials: (data) => api.post('/system/tunnel/credentials', data),
}

// Collector management (admin/reseller)
export const collectorApi = {
  list: () => api.get('/collectors'),
  get: (id) => api.get(`/collectors/${id}`),
  getAssignments: (id, params) => api.get(`/collectors/${id}/assignments`, { params }),
  createAssignment: (data) => api.post('/collectors/assignments', data),
  deleteAssignment: (id) => api.delete(`/collectors/assignments/${id}`),
  getReport: (params) => api.get('/collectors/report', { params }),
}

// Collector self-service
export const collectionApi = {
  dashboard: () => api.get('/collector/dashboard'),
  listAssignments: (params) => api.get('/collector/assignments', { params }),
  getAssignment: (id) => api.get(`/collector/assignments/${id}`),
  markCollected: (id, data) => api.post(`/collector/assignments/${id}/collect`, data),
  markFailed: (id, data) => api.post(`/collector/assignments/${id}/fail`, data),
}

// API Key management (admin)
export const apiKeysApi = {
  list: () => api.get('/api-keys'),
  create: (data) => api.post('/api-keys', data),
  revoke: (id) => api.delete(`/api-keys/${id}`),
  getLogs: (id, params) => api.get(`/api-keys/${id}/logs`, { params }),
  getStats: () => api.get('/api-keys/stats'),
}

// Public IP Management
export const publicIPApi = {
  listPools: (params) => api.get('/public-ips/pools', { params }),
  createPool: (data) => api.post('/public-ips/pools', data),
  updatePool: (id, data) => api.put(`/public-ips/pools/${id}`, data),
  deletePool: (id) => api.delete(`/public-ips/pools/${id}`),
  listAssignments: (params) => api.get('/public-ips/assignments', { params }),
  assignIP: (data) => api.post('/public-ips/assign', data),
  releaseIP: (id) => api.post(`/public-ips/release/${id}`),
  reserveIP: (data) => api.post('/public-ips/reserve', data),
  getSubscriberPublicIP: (subscriberId) => api.get(`/subscribers/${subscriberId}/public-ip`),
  getAvailableIPs: (poolId) => api.get(`/public-ips/pools/${poolId}/available-ips`),
}

// Customer Portal Public IP
export const customerPublicIPApi = {
  get: () => api.get('/customer/public-ip'),
  buy: (data) => api.post('/customer/public-ip/buy', data),
  release: () => api.post('/customer/public-ip/release'),
}
