import { useState, useEffect, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import api, { settingsApi, tunnelApi, maintenanceApi, apiKeysApi, resellerApi } from '../services/api'
import { copyToClipboard } from '../utils/clipboard'
import { useAuthStore } from '../store/authStore'
import { useBrandingStore } from '../store/brandingStore'
import { setTimezone } from '../utils/timezone'
import toast from 'react-hot-toast'
import { PhotoIcon, TrashIcon, SwatchIcon, CpuChipIcon, ServerIcon, ExclamationTriangleIcon, CheckCircleIcon, InformationCircleIcon, LockClosedIcon, GlobeAltIcon, KeyIcon, ClipboardDocumentIcon, EyeIcon, EyeSlashIcon } from '@heroicons/react/24/outline'
import ClusterTab from '../components/ClusterTab'
import APIKeysTab from '../components/APIKeysTab'
import NetworkConfiguration from '../components/NetworkConfiguration'
import { dashboardApi } from '../services/api'
import { QRCodeSVG } from 'qrcode.react'
import clsx from 'clsx'

export default function Settings() {
  const queryClient = useQueryClient()
  const { user, refreshUser, isReseller, hasPermission } = useAuthStore()
  const { companyName, companyLogo, loginBackground, favicon, footerText, primaryColor, fetchBranding, updateBranding } = useBrandingStore()
  const [searchParams, setSearchParams] = useSearchParams()

  // All valid tab IDs
  const validTabs = ['branding', 'general', 'billing', 'service_change', 'radius', 'notifications', 'security', 'account', 'license', 'cluster', 'system', 'ssl', 'api_keys']

  // Check if we should open a specific tab (from URL params)
  const urlTab = searchParams.get('tab')
  const initialTab = (urlTab && validTabs.includes(urlTab)) ? urlTab : 'branding'
  const [activeTab, setActiveTab] = useState(initialTab)

  // Update URL when tab changes (keeps tab in URL for refresh)
  const handleTabChange = (tabId) => {
    setActiveTab(tabId)
    setSearchParams({ tab: tabId }, { replace: true })
  }

  // Sync tab from URL on mount/URL change
  useEffect(() => {
    const tab = searchParams.get('tab')
    if (tab && validTabs.includes(tab) && tab !== activeTab) {
      setActiveTab(tab)
    }
  }, [searchParams])
  const [formData, setFormData] = useState({})
  const [hasChanges, setHasChanges] = useState(false)
  const fileInputRef = useRef(null)
  const backgroundInputRef = useRef(null)
  const faviconInputRef = useRef(null)
  const [uploadingLogo, setUploadingLogo] = useState(false)
  const [uploadingBackground, setUploadingBackground] = useState(false)
  const [uploadingFavicon, setUploadingFavicon] = useState(false)

  // 2FA state
  const [twoFASetup, setTwoFASetup] = useState(null)
  const [twoFACode, setTwoFACode] = useState('')
  const [disablePassword, setDisablePassword] = useState('')
  const [disableCode, setDisableCode] = useState('')

  // Notification test state
  const [testingSmtp, setTestingSmtp] = useState(false)
  const [testingSms, setTestingSms] = useState(false)
  const [testingWhatsapp, setTestingWhatsapp] = useState(false)
  const [testEmail, setTestEmail] = useState('')
  const [testPhone, setTestPhone] = useState('')

  // ProxRad WhatsApp state
  const [proxradPhone, setProxradPhone] = useState('')
  const [proxradQrUrl, setProxradQrUrl] = useState('')
  const [proxradInfoUrl, setProxradInfoUrl] = useState('')
  const [proxradLinking, setProxradLinking] = useState(false)
  const [proxradAccess, setProxradAccess] = useState(null) // { allowed, type, expires_at, trial_ends, trial_hours_left }
  const proxradPollRef = useRef(null)

  // Admin WhatsApp subscriber management
  const [waSubscribers, setWaSubscribers] = useState([])
  const [waSubsLoading, setWaSubsLoading] = useState(false)
  const [waSubSearch, setWaSubSearch] = useState('')
  const [waSelectedIDs, setWaSelectedIDs] = useState([])
  const [waSendAll, setWaSendAll] = useState(false)
  const [waMessage, setWaMessage] = useState('')
  const [waSending, setWaSending] = useState(false)
  const [waTogglingId, setWaTogglingId] = useState(null)

  // SSL state
  const [sslDomain, setSslDomain] = useState('')
  const [sslEmail, setSslEmail] = useState('')
  const [sslLog, setSslLog] = useState([])
  const [sslStreaming, setSslStreaming] = useState(false)
  const sslAbortRef = useRef(null)
  const [tunnelStatus, setTunnelStatus] = useState(null)
  const [tunnelLoading, setTunnelLoading] = useState(false)
  const [tunnelError, setTunnelError] = useState(null)

  // Maintenance window state
  const [maintenanceWindows, setMaintenanceWindows] = useState([])
  const [showMaintenanceModal, setShowMaintenanceModal] = useState(false)
  const [editingMaintenance, setEditingMaintenance] = useState(null)
  const [maintenanceForm, setMaintenanceForm] = useState({ title: '', message: '', start_time: '', end_time: '', notify_subscribers: false })

  const { data, isLoading } = useQuery({
    queryKey: ['settings'],
    queryFn: () => api.get('/settings').then(res => res.data.items || [])
  })

  // Fetch available timezones
  const { data: timezones } = useQuery({
    queryKey: ['timezones'],
    queryFn: () => api.get('/settings/timezones').then(res => res.data.data || [])
  })

  // Initialize form data when settings load
  useEffect(() => {
    if (data) {
      const initialData = {}
      data.forEach(s => {
        initialData[s.key] = s.value
      })
      setFormData(initialData)
      setHasChanges(false)
    }
  }, [data])

  // Fetch ProxRad access status when Notifications tab is active
  useEffect(() => {
    if (activeTab === 'notifications') {
      fetchProxRadAccess()
      fetchWaSubscribers()
    }
  }, [activeTab])

  // Fetch maintenance windows when general tab is active
  useEffect(() => {
    if (activeTab === 'general') fetchMaintenanceWindows()
  }, [activeTab])

  const fetchMaintenanceWindows = async () => {
    try {
      const res = await maintenanceApi.getWindows()
      setMaintenanceWindows(res.data?.data || [])
    } catch (e) { console.error('Failed to load maintenance windows', e) }
  }

  const handleSaveMaintenance = async () => {
    try {
      if (editingMaintenance) {
        await maintenanceApi.updateWindow(editingMaintenance.id, maintenanceForm)
        toast.success('Maintenance window updated')
      } else {
        await maintenanceApi.createWindow(maintenanceForm)
        toast.success('Maintenance window created')
      }
      setShowMaintenanceModal(false)
      setEditingMaintenance(null)
      setMaintenanceForm({ title: '', message: '', start_time: '', end_time: '', notify_subscribers: false })
      fetchMaintenanceWindows()
    } catch (err) {
      toast.error(err.response?.data?.message || 'Failed to save')
    }
  }

  const handleDeleteMaintenance = async (id) => {
    if (!confirm('Delete this maintenance window?')) return
    try {
      await maintenanceApi.deleteWindow(id)
      toast.success('Deleted')
      fetchMaintenanceWindows()
    } catch (err) {
      toast.error('Failed to delete')
    }
  }

  const updateMutation = useMutation({
    mutationFn: (settings) => api.put('/settings/bulk', { settings }),
    onSuccess: () => {
      queryClient.invalidateQueries(['settings'])
      setHasChanges(false)
      // Update timezone in the app if it was changed
      if (formData.system_timezone) {
        setTimezone(formData.system_timezone)
      }
      toast.success('Settings saved successfully')
    }
  })

  // 2FA queries and mutations
  const { data: twoFAStatus, refetch: refetchTwoFA } = useQuery({
    queryKey: ['2fa-status'],
    queryFn: () => api.get('/auth/2fa/status').then(res => res.data.data),
    enabled: activeTab === 'account'
  })

  const setupTwoFAMutation = useMutation({
    mutationFn: () => api.post('/auth/2fa/setup'),
    onSuccess: (res) => {
      setTwoFASetup(res.data.data)
    },
    onError: (err) => {
      toast.error(err.response?.data?.message || 'Failed to setup 2FA')
    }
  })

  const verifyTwoFAMutation = useMutation({
    mutationFn: (code) => api.post('/auth/2fa/verify', { code }),
    onSuccess: () => {
      toast.success('2FA enabled successfully!')
      setTwoFASetup(null)
      setTwoFACode('')
      refetchTwoFA()
      refreshUser()
    },
    onError: (err) => {
      toast.error(err.response?.data?.message || 'Invalid code')
    }
  })

  const disableTwoFAMutation = useMutation({
    mutationFn: (data) => api.post('/auth/2fa/disable', data),
    onSuccess: () => {
      toast.success('2FA disabled successfully')
      setDisablePassword('')
      setDisableCode('')
      refetchTwoFA()
      refreshUser()
    },
    onError: (err) => {
      toast.error(err.response?.data?.message || 'Failed to disable 2FA')
    }
  })

  const handleChange = (key, value) => {
    setFormData(prev => ({ ...prev, [key]: value }))
    setHasChanges(true)
  }

  const handleChangeAndSave = (key, value) => {
    setFormData(prev => ({ ...prev, [key]: value }))
    api.put('/settings/bulk', { settings: [{ key, value: String(value) }] })
      .then(() => queryClient.invalidateQueries(['settings']))
      .catch(() => {})
  }

  const handleSave = () => {
    const settings = Object.entries(formData).map(([key, value]) => ({
      key,
      value: String(value)
    }))
    updateMutation.mutate(settings)
  }

  const handleReset = () => {
    if (data) {
      const initialData = {}
      data.forEach(s => {
        initialData[s.key] = s.value
      })
      setFormData(initialData)
      setHasChanges(false)
    }
  }

  const handleInstallSSL = async () => {
    if (!sslDomain || !sslEmail) return
    setSslLog([])
    setSslStreaming(true)
    const controller = new AbortController()
    sslAbortRef.current = controller
    try {
      const token = useAuthStore.getState().token
      const formData = new FormData()
      formData.append('domain', sslDomain)
      formData.append('email', sslEmail)
      const response = await fetch('/api/settings/ssl-stream', {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}` },
        body: formData,
        signal: controller.signal,
      })
      if (!response.ok) {
        const text = await response.text()
        let msg = 'SSL installation failed'
        try { msg = JSON.parse(text).message || msg } catch {}
        setSslLog([`❌ ${msg}`])
        setSslStreaming(false)
        return
      }
      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      while (true) {
        const { value, done } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop()
        for (const line of lines) {
          if (!line.trim()) continue
          try {
            const data = JSON.parse(line)
            if (data.status === 'success') {
              refetchSSLStatus()
            } else if (data.status === 'error') {
              // error already shown in log
            } else if (data.msg) {
              setSslLog(prev => [...prev, data.msg])
            }
          } catch {}
        }
      }
    } catch (err) {
      if (err.name !== 'AbortError') {
        setSslLog(prev => [...prev, `❌ Error: ${err.message}`])
      }
    } finally {
      setSslStreaming(false)
      sslAbortRef.current = null
    }
  }

  const fetchTunnelStatus = async () => {
    try {
      setTunnelLoading(true)
      setTunnelError(null)
      const res = await tunnelApi.getStatus()
      setTunnelStatus(res.data)
    } catch (err) {
      // If route doesn't exist (old backend), show as inactive instead of error
      if (err?.response?.status === 404 || err?.response?.data?.message?.includes('Cannot GET')) {
        setTunnelStatus({ enabled: false, running: false, url: null, subdomain: '', credentials_set: false })
      } else {
        setTunnelError(err?.response?.data?.message || err?.message || 'Failed to get tunnel status')
      }
    } finally {
      setTunnelLoading(false)
    }
  }

  const handleEnableTunnel = async () => {
    try {
      setTunnelLoading(true)
      setTunnelError(null)
      await tunnelApi.enable()
      await fetchTunnelStatus()
      toast.success('Remote access enabled')
    } catch (err) {
      if (err?.response?.status === 404 || err?.response?.data?.message?.includes('Cannot')) {
        toast.error('Remote access not available on this server version')
      } else {
        const msg = err.response?.data?.message || 'Failed to enable remote access'
        setTunnelError(msg)
        toast.error(msg)
      }
    } finally {
      setTunnelLoading(false)
    }
  }

  const handleDisableTunnel = async () => {
    try {
      setTunnelLoading(true)
      setTunnelError(null)
      await tunnelApi.disable()
      await fetchTunnelStatus()
      toast.success('Remote access disabled')
    } catch (err) {
      if (err?.response?.status === 404 || err?.response?.data?.message?.includes('Cannot')) {
        toast.error('Remote access not available on this server version')
      } else {
        const msg = err.response?.data?.message || 'Failed to disable remote access'
        setTunnelError(msg)
        toast.error(msg)
      }
    } finally {
      setTunnelLoading(false)
    }
  }

  // License query
  const { data: licenseData, isLoading: licenseLoading, refetch: refetchLicense } = useQuery({
    queryKey: ['license', activeTab],
    queryFn: () => api.get('/license').then(res => res.data.data),
    enabled: activeTab === 'license',
    staleTime: 0,
    refetchOnMount: 'always'
  })

  // License status query (for WHMCS-style status)
  const { data: licenseStatus, refetch: refetchLicenseStatus } = useQuery({
    queryKey: ['license-status-detail', activeTab],
    queryFn: () => api.get('/license/status').then(res => res.data),
    enabled: activeTab === 'license',
    staleTime: 0,
    refetchOnMount: 'always'
  })

  // Revalidate license mutation
  const revalidateMutation = useMutation({
    mutationFn: () => api.post('/license/revalidate'),
    onSuccess: (res) => {
      if (res.data.success) {
        toast.success('License validated successfully')
      } else {
        toast.error(res.data.message || 'License validation failed')
      }
      refetchLicense()
      refetchLicenseStatus()
      queryClient.invalidateQueries(['license-status'])
    },
    onError: (err) => {
      toast.error(err.response?.data?.message || 'Failed to validate license')
    }
  })

  // SSL status query
  const { data: sslStatus, refetch: refetchSSLStatus } = useQuery({
    queryKey: ['ssl-status'],
    queryFn: () => settingsApi.getSSLStatus().then(res => res.data),
    enabled: activeTab === 'system' || activeTab === 'ssl',
    staleTime: 30000,
  })

  // Pre-fill domain from saved SSL status
  useEffect(() => {
    if (sslStatus?.domain && !sslDomain) {
      setSslDomain(sslStatus.domain)
    }
  }, [sslStatus])

  useEffect(() => {
    if (activeTab === 'ssl' || activeTab === 'general') {
      fetchTunnelStatus()
    }
  }, [activeTab])

  // Check for updates query
  const { data: updateData, refetch: refetchUpdate, isLoading: updateLoading } = useQuery({
    queryKey: ['system-update-check', activeTab],
    queryFn: () => api.get('/system/update/check').then(res => res.data),
    enabled: activeTab === 'license',
    staleTime: 0,
    refetchOnMount: 'always'
  })

  // Check for updates mutation (manual refresh)
  const checkUpdateMutation = useMutation({
    mutationFn: () => api.get('/system/update/check').then(res => res.data),
    onSuccess: (data) => {
      if (data.update_available) {
        toast.success(`Update available: v${data.new_version}`)
      } else {
        toast.success('You are running the latest version')
      }
      queryClient.invalidateQueries(['system-update-check'])
    },
    onError: (err) => {
      toast.error(err.response?.data?.message || 'Failed to check for updates')
    }
  })

  // System Info query
  const { data: systemInfo, isLoading: systemInfoLoading, refetch: refetchSystemInfo } = useQuery({
    queryKey: ['system-info', activeTab],
    queryFn: () => dashboardApi.systemInfo().then(res => res.data.data),
    enabled: activeTab === 'system',
    staleTime: 30000,
    refetchOnMount: 'always'
  })

  // Reseller WAN check settings (self-service)
  const { data: resellerWanData, refetch: refetchResellerWan } = useQuery({
    queryKey: ['reseller-wan-settings'],
    queryFn: () => resellerApi.getSelfWanSettings().then(res => res.data.data),
    enabled: activeTab === 'radius' && isReseller() && hasPermission('settings.wan_check'),
  })

  const saveResellerWanMutation = useMutation({
    mutationFn: (data) => resellerApi.updateSelfWanSettings(data),
    onSuccess: () => {
      toast.success('WAN check settings saved')
      refetchResellerWan()
    },
    onError: () => toast.error('Failed to save WAN check settings'),
  })

  // Test SMTP configuration
  const handleTestSmtp = async () => {
    setTestingSmtp(true)
    try {
      const res = await api.post('/notifications/test-smtp', {
        smtp_host: formData.smtp_host,
        smtp_port: formData.smtp_port,
        smtp_username: formData.smtp_username,
        smtp_password: formData.smtp_password,
        smtp_from_name: formData.smtp_from_name,
        smtp_from_email: formData.smtp_from_email,
        test_email: testEmail || formData.notification_email
      })
      toast.success(res.data.message || 'SMTP test successful!')
    } catch (err) {
      toast.error(err.response?.data?.message || 'SMTP test failed')
    } finally {
      setTestingSmtp(false)
    }
  }

  // Test SMS configuration
  const handleTestSms = async () => {
    setTestingSms(true)
    try {
      const res = await api.post('/notifications/test-sms', {
        sms_provider: formData.sms_provider,
        sms_twilio_sid: formData.sms_twilio_sid,
        sms_twilio_token: formData.sms_twilio_token,
        sms_twilio_from: formData.sms_twilio_from,
        sms_vonage_key: formData.sms_vonage_key,
        sms_vonage_secret: formData.sms_vonage_secret,
        sms_vonage_from: formData.sms_vonage_from,
        sms_custom_url: formData.sms_custom_url,
        sms_custom_method: formData.sms_custom_method,
        sms_custom_body: formData.sms_custom_body,
        test_phone: testPhone
      })
      toast.success(res.data.message || 'SMS test successful!')
    } catch (err) {
      toast.error(err.response?.data?.message || 'SMS test failed')
    } finally {
      setTestingSms(false)
    }
  }

  // Test ProxRad WhatsApp (uses the configured provider — proxrad or ultramsg)
  const handleTestProxRad = async () => {
    setTestingWhatsapp(true)
    try {
      const res = await api.post('/notifications/proxrad/test-send', {
        test_phone: testPhone
      })
      toast.success(res.data.message || 'Test message sent!')
    } catch (err) {
      toast.error(err.response?.data?.message || 'Test send failed')
    } finally {
      setTestingWhatsapp(false)
    }
  }

  // Test WhatsApp configuration
  const handleTestWhatsapp = async () => {
    setTestingWhatsapp(true)
    try {
      const res = await api.post('/notifications/test-whatsapp', {
        whatsapp_instance_id: formData.whatsapp_instance_id,
        whatsapp_token: formData.whatsapp_token,
        test_phone: testPhone
      })
      toast.success(res.data.message || 'WhatsApp test successful!')
    } catch (err) {
      toast.error(err.response?.data?.message || 'WhatsApp test failed')
    } finally {
      setTestingWhatsapp(false)
    }
  }

  // ProxRad: Start QR linking flow
  const handleProxRadCreateLink = async () => {
    setProxradLinking(true)
    setProxradQrUrl('')
    setProxradInfoUrl('')
    try {
      const res = await api.get('/notifications/proxrad/create-link')
      setProxradQrUrl(res.data.qr_image_url)
      setProxradInfoUrl(res.data.info_url)
      // Start polling for connection status
      proxradPollRef.current = setInterval(async () => {
        try {
          const status = await api.get(`/notifications/proxrad/link-status?info_url=${encodeURIComponent(res.data.info_url)}`)
          if (status.data.connected) {
            clearInterval(proxradPollRef.current)
            setProxradLinking(false)
            setProxradQrUrl('')
            setProxradInfoUrl('')
            handleChange('proxrad_account_unique', status.data.unique)
            setProxradPhone(status.data.phone || status.data.unique)
            fetchProxRadAccess()
            toast.success('WhatsApp account linked successfully!')
          }
        } catch {}
      }, 3000)
    } catch (err) {
      setProxradLinking(false)
      toast.error(err.response?.data?.message || 'Failed to create WhatsApp link')
    }
  }

  const handleProxRadCancelLink = () => {
    clearInterval(proxradPollRef.current)
    setProxradLinking(false)
    setProxradQrUrl('')
    setProxradInfoUrl('')
  }

  // ProxRad: Fetch access status (trial/subscribed/expired)
  const fetchProxRadAccess = async () => {
    try {
      const res = await api.get('/notifications/proxrad/access')
      setProxradAccess(res.data)
    } catch {}
  }

  // ProxRad: Unlink account (disconnects from proxsms + clears DB)
  const handleProxRadUnlink = async () => {
    try {
      await api.delete('/notifications/proxrad/unlink')
      handleChange('proxrad_account_unique', '')
      handleChange('proxrad_phone', '')
      setProxradPhone('')
      setProxradAccess(null)
      toast.success('WhatsApp account unlinked')
    } catch (err) {
      toast.error(err.response?.data?.message || 'Failed to unlink account')
    }
  }

  const fetchWaSubscribers = async (search = '') => {
    setWaSubsLoading(true)
    try {
      const res = await api.get('/notifications/whatsapp/subscribers', { params: { search } })
      if (res.data.success) setWaSubscribers(res.data.subscribers || [])
    } catch (e) {
      console.error(e)
    }
    setWaSubsLoading(false)
  }

  const waToggleNotifications = async (sub) => {
    setWaTogglingId(sub.id)
    try {
      const res = await api.post(`/notifications/whatsapp/subscribers/${sub.id}/toggle-notifications`)
      if (res.data.success) {
        setWaSubscribers(prev => prev.map(s =>
          s.id === sub.id ? { ...s, whatsapp_notifications: res.data.whatsapp_notifications } : s
        ))
      }
    } catch (e) {
      console.error(e)
    } finally {
      setWaTogglingId(null)
    }
  }

  const waEnableAll = async () => {
    try {
      await api.post('/notifications/whatsapp/notifications/set-all', { enabled: true })
      setWaSubscribers(prev => prev.map(s => ({ ...s, whatsapp_notifications: true })))
    } catch (e) { console.error(e) }
  }

  const waDisableAll = async () => {
    try {
      await api.post('/notifications/whatsapp/notifications/set-all', { enabled: false })
      setWaSubscribers(prev => prev.map(s => ({ ...s, whatsapp_notifications: false })))
    } catch (e) { console.error(e) }
  }

  const waHandleSend = async () => {
    if (!waMessage.trim()) { toast.error('Enter a message'); return }
    if (!waSendAll && waSelectedIDs.length === 0) { toast.error('Select at least one subscriber'); return }
    const count = waSendAll ? waSubscribers.length : waSelectedIDs.length
    if (!confirm(`Send message to ${count} subscriber${count !== 1 ? 's' : ''}?`)) return
    setWaSending(true)
    try {
      const res = await api.post('/notifications/whatsapp/send', {
        message: waMessage.trim(),
        send_all: waSendAll,
        subscriber_ids: waSendAll ? [] : waSelectedIDs,
      })
      if (res.data.success) {
        toast.success(`✅ Sent to ${res.data.sent} subscribers${res.data.failed > 0 ? `, ${res.data.failed} failed` : ''}`)
        if (res.data.failed === 0) {
          setWaMessage('')
          setWaSelectedIDs([])
          setWaSendAll(false)
        }
      } else {
        toast.error(res.data.message || 'Send failed')
      }
    } catch (e) {
      toast.error(e?.response?.data?.message || 'Failed to send')
    }
    setWaSending(false)
  }

  const tabs = [
    { id: 'branding', label: 'Branding' },
    { id: 'general', label: 'General' },
    { id: 'billing', label: 'Billing' },
    { id: 'service_change', label: 'Service Change' },
    { id: 'radius', label: 'RADIUS' },
    { id: 'notifications', label: 'Notifications' },
    { id: 'security', label: 'Security' },
    { id: 'account', label: 'My Account' },
    { id: 'license', label: 'License' },
    { id: 'cluster', label: 'HA Cluster' },
    { id: 'system', label: 'Network' },
    { id: 'ssl', label: 'SSL Certificate' },
    { id: 'api_keys', label: 'API Keys' },
  ]

  // Logo upload handler
  const handleLogoUpload = async (e) => {
    const file = e.target.files?.[0]
    if (!file) return

    // Validate file type
    const validTypes = ['image/png', 'image/jpeg', 'image/jpg', 'image/svg+xml', 'image/webp']
    if (!validTypes.includes(file.type)) {
      toast.error('Invalid file type. Use PNG, JPG, SVG, or WEBP')
      return
    }

    // Validate file size (2MB max)
    if (file.size > 2 * 1024 * 1024) {
      toast.error('File too large. Maximum size is 2MB')
      return
    }

    setUploadingLogo(true)
    const formData = new FormData()
    formData.append('logo', file)

    try {
      const response = await settingsApi.uploadLogo(formData)
      if (response.data.success) {
        toast.success('Logo uploaded successfully')
        updateBranding({ company_logo: response.data.data.url })
        queryClient.invalidateQueries(['settings'])
      }
    } catch (error) {
      toast.error(error.response?.data?.message || 'Failed to upload logo')
    } finally {
      setUploadingLogo(false)
      if (fileInputRef.current) {
        fileInputRef.current.value = ''
      }
    }
  }

  // Logo delete handler
  const handleLogoDelete = async () => {
    if (!companyLogo) return

    try {
      await settingsApi.deleteLogo()
      toast.success('Logo deleted')
      updateBranding({ company_logo: '' })
      queryClient.invalidateQueries(['settings'])
    } catch (error) {
      toast.error('Failed to delete logo')
    }
  }

  // Background upload handler
  const handleBackgroundUpload = async (e) => {
    const file = e.target.files?.[0]
    if (!file) return

    // Validate file size (max 5MB)
    if (file.size > 5 * 1024 * 1024) {
      toast.error('File too large. Maximum size is 5MB')
      return
    }

    setUploadingBackground(true)
    const formData = new FormData()
    formData.append('background', file)

    try {
      const response = await settingsApi.uploadLoginBackground(formData)
      if (response.data.success) {
        toast.success('Login background uploaded successfully')
        updateBranding({ login_background: response.data.data.url })
        queryClient.invalidateQueries(['settings'])
      }
    } catch (error) {
      toast.error(error.response?.data?.message || 'Failed to upload background')
    } finally {
      setUploadingBackground(false)
      if (backgroundInputRef.current) {
        backgroundInputRef.current.value = ''
      }
    }
  }

  // Background delete handler
  const handleBackgroundDelete = async () => {
    if (!loginBackground) return

    try {
      await settingsApi.deleteLoginBackground()
      toast.success('Login background deleted')
      updateBranding({ login_background: '' })
      queryClient.invalidateQueries(['settings'])
    } catch (error) {
      toast.error('Failed to delete background')
    }
  }

  // Favicon upload handler
  const handleFaviconUpload = async (e) => {
    const file = e.target.files?.[0]
    if (!file) return

    // Validate file size (max 500KB)
    if (file.size > 500 * 1024) {
      toast.error('File too large. Maximum size is 500KB')
      return
    }

    setUploadingFavicon(true)
    const formData = new FormData()
    formData.append('favicon', file)

    try {
      const response = await settingsApi.uploadFavicon(formData)
      if (response.data.success) {
        toast.success('Favicon uploaded successfully')
        updateBranding({ favicon: response.data.data.url })
        queryClient.invalidateQueries(['settings'])
      }
    } catch (error) {
      toast.error(error.response?.data?.message || 'Failed to upload favicon')
    } finally {
      setUploadingFavicon(false)
      if (faviconInputRef.current) {
        faviconInputRef.current.value = ''
      }
    }
  }

  // Favicon delete handler
  const handleFaviconDelete = async () => {
    if (!favicon) return

    try {
      await settingsApi.deleteFavicon()
      toast.success('Favicon deleted')
      updateBranding({ favicon: '' })
      queryClient.invalidateQueries(['settings'])
    } catch (error) {
      toast.error('Failed to delete favicon')
    }
  }

  const settingGroups = {
    general: [
      { key: 'company_name', label: 'Company Name', type: 'text', placeholder: 'Your Company Name' },
      { key: 'company_address', label: 'Company Address', type: 'textarea', placeholder: 'Full address' },
      { key: 'company_phone', label: 'Phone Number', type: 'text', placeholder: '+1 234 567 890' },
      { key: 'company_email', label: 'Email', type: 'email', placeholder: 'info@company.com' },
      { key: 'system_timezone', label: 'System Timezone', type: 'timezone', description: 'Used for FUP reset, bandwidth rules, and all time-based features' },
      { key: 'date_format', label: 'Date Format', type: 'select', options: [
        'YYYY-MM-DD', 'DD/MM/YYYY', 'MM/DD/YYYY', 'DD-MM-YYYY'
      ]},
    ],
    billing: [
      { key: 'currency', label: 'Currency Code', type: 'text', placeholder: 'USD' },
      { key: 'currency_symbol', label: 'Currency Symbol', type: 'text', placeholder: '$' },
      { key: 'tax_rate', label: 'Tax Rate (%)', type: 'number', placeholder: '0' },
      { key: 'invoice_prefix', label: 'Invoice Prefix', type: 'text', placeholder: 'INV-' },
      { key: 'payment_methods', label: 'Payment Methods', type: 'text', placeholder: 'cash,bank,mpesa' },
      { key: 'auto_generate_invoice', label: 'Auto Generate Invoice', type: 'toggle' },
      { key: 'invoice_due_days', label: 'Invoice Due Days', type: 'number', placeholder: '7' },
    ],
    service_change: [
      { key: 'upgrade_change_service_fee', label: 'Upgrade Fee ($)', type: 'number', placeholder: '0', description: 'Fee charged when subscriber upgrades to a higher-priced service' },
      { key: 'downgrade_change_service_fee', label: 'Downgrade Fee ($)', type: 'number', placeholder: '0', description: 'Fee charged when subscriber downgrades to a lower-priced service' },
      { key: 'allow_downgrade', label: 'Allow Downgrade', type: 'toggle', description: 'Allow subscribers to change to a lower-priced service' },
      { key: 'downgrade_refund', label: 'Refund on Downgrade', type: 'toggle', description: 'Refund the difference when downgrading (prorate credit)' },
    ],
    radius: [
      { key: 'daily_quota_reset_time', label: 'Daily Quota Reset Time', type: 'time', placeholder: '00:00' },
      { key: 'notification_send_time', label: 'Notification Send Time', type: 'time', placeholder: '08:00', description: 'Time to send expiry warning and expired notifications daily' },
      { key: 'invoice_days_before_expiry', label: 'Invoice Days Before Expiry', type: 'number', placeholder: '7', description: 'Auto-generate invoices this many days before subscriber expiry (requires Auto Invoice enabled per subscriber)' },
      { key: 'default_session_timeout', label: 'Default Session Timeout (sec)', type: 'number', placeholder: '86400' },
      { key: 'max_sessions_per_user', label: 'Max Sessions Per User', type: 'number', placeholder: '1' },
      { key: 'accounting_interval', label: 'Accounting Interval (sec)', type: 'number', placeholder: '300' },
      { key: 'idle_timeout', label: 'Idle Timeout (sec)', type: 'number', placeholder: '600' },
      { key: 'simultaneous_use', label: 'Allow Simultaneous Use', type: 'toggle', description: 'Allow subscribers to have multiple active PPPoE sessions at the same time. Each subscriber\'s session limit is set individually (default: 1).' },
      { key: 'mac_auth_enabled', label: 'MAC Authentication', type: 'toggle', description: 'Bind subscribers to their MAC address. Prevents connecting from a different device without resetting MAC first.' },
      { key: 'block_on_daily_quota_exceeded', label: 'Block Internet on Daily Quota Exceeded', type: 'toggle', description: 'When enabled, users will lose internet completely when daily quota is exceeded. When disabled, users get reduced FUP speed.' },
      { key: 'block_on_monthly_quota_exceeded', label: 'Block Internet on Monthly Quota Exceeded', type: 'toggle', description: 'When enabled, users will lose internet completely when monthly quota is exceeded. When disabled, users get reduced FUP speed.' },
      { key: 'wan_check_enabled', label: 'WAN Management Check', type: 'toggle', description: 'Block new subscribers until router ICMP (ping) and WAN management port are reachable. Existing subscribers are grandfathered.' },
      { key: 'wan_check_icmp_enabled', label: 'WAN Check — ICMP Ping', type: 'toggle', description: 'Enable ICMP ping check as part of WAN management. Disable to skip ping and only check port.' },
      { key: 'wan_check_port_enabled', label: 'WAN Check — Port Check', type: 'toggle', description: 'Enable TCP port check as part of WAN management. Disable to skip port check and only check ping.' },
      { key: 'wan_check_port', label: 'WAN Management Port', type: 'number', placeholder: '8084', description: 'TCP port to check on subscriber router (e.g. 8084 for MikroTik WAN management).' },
    ],
    notifications: [], // Custom rendering below
    security: [
      { key: 'session_timeout', label: 'Admin Session Timeout (min)', type: 'number', placeholder: '60' },
      { key: 'max_login_attempts', label: 'Max Login Attempts', type: 'number', placeholder: '5' },
      { key: 'password_min_length', label: 'Min Password Length', type: 'number', placeholder: '8' },
      { key: 'api_rate_limit', label: 'API Rate Limit (req/min)', type: 'number', placeholder: '100' },
      { key: 'allowed_ips', label: 'Allowed Admin IPs', type: 'text', placeholder: 'Leave empty for all' },
      { key: 'remote_support_enabled', label: 'Remote Support Access', type: 'toggle', description: 'Allow ProxPanel support team to access your server for troubleshooting. After enabling, run: proxpanel-support enable' },
    ],
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-[#316AC5]"></div>
      </div>
    )
  }

  const renderField = (field) => {
    const value = formData[field.key] || ''

    if (field.type === 'toggle') {
      const isChecked = value === 'true' || value === '1' || value === true
      return (
        <div>
          <label className="inline-flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={isChecked}
              onChange={() => handleChange(field.key, isChecked ? 'false' : 'true')}
              className="w-3.5 h-3.5 border border-[#a0a0a0] accent-[#316AC5]"
              style={{ borderRadius: '2px' }}
            />
            <span className="text-[12px] text-gray-700">{field.label}</span>
          </label>
          {field.description && (
            <p className="mt-1 text-[11px] text-gray-500">{field.description}</p>
          )}
        </div>
      )
    }

    if (field.type === 'timezone') {
      return (
        <div>
          <select
            value={value}
            onChange={(e) => handleChange(field.key, e.target.value)}
            className="input"
          >
            <option value="">Select timezone...</option>
            {(timezones || []).map(tz => (
              <option key={tz.value} value={tz.value}>{tz.label}</option>
            ))}
          </select>
          {field.description && (
            <p className="mt-1 text-[11px] text-gray-500 dark:text-gray-400">{field.description}</p>
          )}
        </div>
      )
    }

    if (field.type === 'select') {
      return (
        <select
          value={value}
          onChange={(e) => handleChange(field.key, e.target.value)}
          className="input"
        >
          <option value="">Select...</option>
          {field.options.map(opt => (
            <option key={opt} value={opt}>{opt}</option>
          ))}
        </select>
      )
    }

    if (field.type === 'textarea') {
      return (
        <textarea
          value={value}
          onChange={(e) => handleChange(field.key, e.target.value)}
          placeholder={field.placeholder}
          rows={3}
          className="input"
        />
      )
    }

    return (
      <div>
        <input
          type={field.type}
          value={value}
          onChange={(e) => handleChange(field.key, e.target.value)}
          placeholder={field.placeholder}
          step={field.type === 'number' ? '0.01' : undefined}
          className="input"
        />
        {field.description && (
          <p className="mt-1 text-[11px] text-gray-500 dark:text-gray-400">{field.description}</p>
        )}
      </div>
    )
  }

  return (
    <div className="space-y-1">
      <div className="wb-toolbar flex justify-between items-center">
        <h1 className="text-[14px] font-semibold text-gray-900 dark:text-[#e0e0e0]">Settings</h1>
        <div className="flex gap-1">
          {hasChanges && (
            <button
              onClick={handleReset}
              className="btn"
            >
              Reset
            </button>
          )}
          <button
            onClick={handleSave}
            disabled={!hasChanges || updateMutation.isPending}
            className={hasChanges ? 'btn-primary' : 'btn opacity-50 cursor-not-allowed'}
          >
            {updateMutation.isPending ? 'Saving...' : 'Save Changes'}
          </button>
        </div>
      </div>

      {updateMutation.isSuccess && (
        <div className="card border-green-500 bg-green-50 dark:bg-green-900/30 text-green-700 dark:text-green-300 px-3 py-2 text-[12px]">
          Settings saved successfully!
        </div>
      )}

      {updateMutation.isError && (
        <div className="card border-red-500 bg-red-50 dark:bg-red-900/30 text-red-700 dark:text-red-300 px-3 py-2 text-[12px]">
          Failed to save settings. Please try again.
        </div>
      )}

      <div className="card">
        {/* Tabs */}
        <div className="flex flex-wrap border-b border-[#a0a0a0] bg-[#f0f0f0] dark:bg-[#444] dark:border-[#555]">
          {tabs.map(tab => (
            <button
              key={tab.id}
              onClick={() => handleTabChange(tab.id)}
              className={`px-3 py-1.5 text-[12px] border border-[#a0a0a0] border-b-0 dark:border-[#555] ${
                activeTab === tab.id
                  ? 'bg-white dark:bg-[#333] font-semibold -mb-px'
                  : 'bg-[#e8e8e8] dark:bg-[#444] text-[#666] dark:text-[#aaa]'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {/* Form Fields */}
        <div className="p-3">
          {activeTab === 'branding' ? (
            <div className="space-y-4">
              {/* Company Name */}
              <div className="wb-group">
                <div className="wb-group-title">Company Name</div>
                <div className="wb-group-body">
                <input
                  type="text"
                  value={formData.company_name || ''}
                  onChange={(e) => handleChange('company_name', e.target.value)}
                  placeholder="Your Company Name"
                  className="input max-w-md"
                />
                <p className="mt-1 text-[11px] text-gray-500 dark:text-[#999]">This name appears in the sidebar and login page</p>
                </div>
              </div>

              {/* Logo Upload */}
              <div className="wb-group">
                <div className="wb-group-title">Company Logo</div>
                <div className="wb-group-body">
                <div className="flex items-start gap-4">
                  <div className="flex-shrink-0">
                    <div className="w-32 h-32 border border-[#a0a0a0] dark:border-[#555] rounded-sm flex items-center justify-center bg-white dark:bg-[#2a2a2a] overflow-hidden">
                      {companyLogo ? (
                        <img src={companyLogo} alt="Company Logo" className="max-w-full max-h-full object-contain" />
                      ) : (
                        <PhotoIcon className="w-12 h-12 text-gray-400" />
                      )}
                    </div>
                  </div>
                  <div className="flex-1">
                    <input ref={fileInputRef} type="file" accept="image/png,image/jpeg,image/jpg,image/svg+xml,image/webp" onChange={handleLogoUpload} className="hidden" />
                    <div className="flex flex-col gap-1">
                      <button type="button" onClick={() => fileInputRef.current?.click()} disabled={uploadingLogo} className="btn w-fit">
                        <PhotoIcon className="w-3.5 h-3.5 mr-1" />
                        {uploadingLogo ? 'Uploading...' : 'Upload Logo'}
                      </button>
                      {companyLogo && (
                        <button type="button" onClick={handleLogoDelete} className="btn-danger w-fit">
                          <TrashIcon className="w-3.5 h-3.5 mr-1" />
                          Remove Logo
                        </button>
                      )}
                    </div>
                    <p className="mt-2 text-[11px] text-gray-500 dark:text-[#999]">
                      Recommended: <strong>180 x 36 pixels</strong> (horizontal logo). PNG with transparent background, max 2MB.
                    </p>
                  </div>
                </div>
                </div>
              </div>

              {/* Primary Color */}
              <div className="wb-group">
                <div className="wb-group-title">
                  <SwatchIcon className="w-3.5 h-3.5 inline mr-1" />
                  Primary Color
                </div>
                <div className="wb-group-body">
                <div className="flex items-center gap-3">
                  <input
                    type="color"
                    value={formData.primary_color || '#2563eb'}
                    onChange={(e) => handleChange('primary_color', e.target.value)}
                    className="w-12 h-8 rounded-sm border border-[#a0a0a0] dark:border-[#555] cursor-pointer"
                  />
                  <input
                    type="text"
                    value={formData.primary_color || '#2563eb'}
                    onChange={(e) => handleChange('primary_color', e.target.value)}
                    placeholder="#2563eb"
                    className="input w-28"
                  />
                  <div className="flex gap-1.5">
                    {['#2563eb', '#10b981', '#8b5cf6', '#f59e0b', '#ef4444', '#06b6d4'].map((color) => (
                      <button
                        key={color}
                        type="button"
                        onClick={() => handleChange('primary_color', color)}
                        className="w-6 h-6 rounded-full border-2 border-white dark:border-[#555] hover:scale-110 transition-transform"
                        style={{ backgroundColor: color }}
                        title={color}
                      />
                    ))}
                  </div>
                </div>
                <p className="mt-1 text-[11px] text-gray-500 dark:text-[#999]">Used for buttons, links, and accent elements throughout the app</p>
                </div>
              </div>

              {/* Login Background Image */}
              <div className="wb-group">
                <div className="wb-group-title">Login Page Background</div>
                <div className="wb-group-body">
                <div className="flex items-start gap-4">
                  <div className="flex-shrink-0">
                    <div className="w-48 h-32 border border-[#a0a0a0] dark:border-[#555] rounded-sm flex items-center justify-center bg-gradient-to-br from-blue-600 to-indigo-800 overflow-hidden">
                      {loginBackground ? (
                        <img src={loginBackground} alt="Login Background" className="w-full h-full object-cover" />
                      ) : (
                        <span className="text-white text-[11px]">Default Gradient</span>
                      )}
                    </div>
                  </div>
                  <div className="flex-1">
                    <input ref={backgroundInputRef} type="file" accept="image/png,image/jpeg,image/jpg,image/webp" onChange={handleBackgroundUpload} className="hidden" />
                    <div className="flex flex-col gap-1">
                      <button type="button" onClick={() => backgroundInputRef.current?.click()} disabled={uploadingBackground} className="btn w-fit">
                        <PhotoIcon className="w-3.5 h-3.5 mr-1" />
                        {uploadingBackground ? 'Uploading...' : 'Upload Background'}
                      </button>
                      {loginBackground && (
                        <button type="button" onClick={handleBackgroundDelete} className="btn-danger w-fit">
                          <TrashIcon className="w-3.5 h-3.5 mr-1" />
                          Use Default Gradient
                        </button>
                      )}
                    </div>
                    <p className="mt-2 text-[11px] text-gray-500 dark:text-[#999]">
                      Recommended: <strong>1920 x 1080 pixels</strong>. JPG or PNG, max 5MB.<br />
                      This image appears on the left side of the login page.
                    </p>
                  </div>
                </div>
                </div>
              </div>

              {/* Favicon */}
              <div className="border-t dark:border-gray-700 pt-3">
                <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-2">
                  Favicon (Browser Tab Icon)
                </label>
                <div className="flex items-start gap-3">
                  <div className="flex-shrink-0">
                    <div className="w-16 h-16 border-2 border-dashed border-[#a0a0a0] flex items-center justify-center bg-gray-50 dark:bg-gray-700 overflow-hidden">
                      {favicon ? (
                        <img src={favicon} alt="Favicon" className="w-8 h-8 object-contain" />
                      ) : (
                        <span className="text-gray-400 text-xl">🌐</span>
                      )}
                    </div>
                  </div>
                  <div className="flex-1">
                    <input ref={faviconInputRef} type="file" accept="image/png,image/x-icon,image/svg+xml" onChange={handleFaviconUpload} className="hidden" />
                    <div className="flex flex-col gap-2">
                      <button type="button" onClick={() => faviconInputRef.current?.click()} disabled={uploadingFavicon} className="inline-flex items-center px-4 py-2 text-[12px] font-medium text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 border border-[#a0a0a0] hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50 w-fit">
                        <PhotoIcon className="w-4 h-4 mr-2" />
                        {uploadingFavicon ? 'Uploading...' : 'Upload Favicon'}
                      </button>
                      {favicon && (
                        <button type="button" onClick={handleFaviconDelete} className="inline-flex items-center px-4 py-2 text-[12px] font-medium text-red-600 bg-white dark:bg-gray-700 border border-red-300 dark:border-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 w-fit">
                          <TrashIcon className="w-4 h-4 mr-2" />
                          Remove Favicon
                        </button>
                      )}
                    </div>
                    <p className="mt-2 text-[11px] text-gray-500 dark:text-gray-400">
                      Recommended: <strong>32 x 32 pixels</strong>. PNG, ICO, or SVG, max 500KB.
                    </p>
                  </div>
                </div>
              </div>

              {/* Footer Text */}
              <div className="border-t dark:border-gray-700 pt-3">
                <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-2">
                  Footer Copyright Text
                </label>
                <input
                  type="text"
                  value={formData.footer_text || ''}
                  onChange={(e) => handleChange('footer_text', e.target.value)}
                  placeholder="© 2026 Your Company Name. All rights reserved."
                  className="block w-full max-w-lg border-[#a0a0a0] dark:bg-gray-700 dark:text-white focus:border-blue-500 focus:ring-blue-500 sm:text-[12px]"
                />
                <p className="mt-1 text-[11px] text-gray-500 dark:text-gray-400">Appears at the bottom of the login page</p>
              </div>

              {/* Login Page Features Section */}
              <div className="border-t dark:border-gray-700 pt-3">
                <h3 className="text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-3">Login Page Features</h3>

                {/* Tagline */}
                <div className="mb-3">
                  <label className="block text-[11px] font-medium text-gray-600 dark:text-gray-400 mb-1">
                    Tagline (bottom of left panel)
                  </label>
                  <input
                    type="text"
                    value={formData.login_tagline || ''}
                    onChange={(e) => handleChange('login_tagline', e.target.value)}
                    placeholder="High Performance ISP Management Solution"
                    className="block w-full max-w-lg border-[#a0a0a0] dark:bg-gray-700 dark:text-white focus:border-blue-500 focus:ring-blue-500 sm:text-[12px]"
                  />
                </div>

                {/* Show/Hide Features Toggle */}
                <div className="mb-3">
                  <label className="flex items-center gap-3 cursor-pointer">
                    <div className="relative">
                      <input
                        type="checkbox"
                        checked={formData.show_login_features !== 'false'}
                        onChange={(e) => handleChange('show_login_features', e.target.checked ? 'true' : 'false')}
                        className="sr-only"
                      />
                      <div className={`w-10 h-6 transition-colors ${formData.show_login_features !== 'false' ? 'bg-blue-600' : 'bg-gray-300 dark:bg-gray-600'}`}>
                        <div className={`absolute top-1 w-4 h-4 bg-white transition-transform ${formData.show_login_features !== 'false' ? 'translate-x-5' : 'translate-x-1'}`}></div>
                      </div>
                    </div>
                    <span className="text-[12px] text-gray-700 dark:text-gray-300">Show feature boxes on login page</span>
                  </label>
                </div>

                {/* Feature 1 */}
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-3 p-4 bg-gray-50 dark:bg-gray-700/50">
                  <div>
                    <label className="block text-[11px] font-medium text-gray-600 dark:text-gray-400 mb-1">Feature 1 Title</label>
                    <input
                      type="text"
                      value={formData.login_feature_1_title || ''}
                      onChange={(e) => handleChange('login_feature_1_title', e.target.value)}
                      placeholder="PPPoE Management"
                      className="block w-full border-[#a0a0a0] dark:bg-gray-700 dark:text-white focus:border-blue-500 focus:ring-blue-500 sm:text-[12px]"
                    />
                  </div>
                  <div>
                    <label className="block text-[11px] font-medium text-gray-600 dark:text-gray-400 mb-1">Feature 1 Description</label>
                    <input
                      type="text"
                      value={formData.login_feature_1_desc || ''}
                      onChange={(e) => handleChange('login_feature_1_desc', e.target.value)}
                      placeholder="Complete subscriber and session management..."
                      className="block w-full border-[#a0a0a0] dark:bg-gray-700 dark:text-white focus:border-blue-500 focus:ring-blue-500 sm:text-[12px]"
                    />
                  </div>
                </div>

                {/* Feature 2 */}
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-3 p-4 bg-gray-50 dark:bg-gray-700/50">
                  <div>
                    <label className="block text-[11px] font-medium text-gray-600 dark:text-gray-400 mb-1">Feature 2 Title</label>
                    <input
                      type="text"
                      value={formData.login_feature_2_title || ''}
                      onChange={(e) => handleChange('login_feature_2_title', e.target.value)}
                      placeholder="Bandwidth Control"
                      className="block w-full border-[#a0a0a0] dark:bg-gray-700 dark:text-white focus:border-blue-500 focus:ring-blue-500 sm:text-[12px]"
                    />
                  </div>
                  <div>
                    <label className="block text-[11px] font-medium text-gray-600 dark:text-gray-400 mb-1">Feature 2 Description</label>
                    <input
                      type="text"
                      value={formData.login_feature_2_desc || ''}
                      onChange={(e) => handleChange('login_feature_2_desc', e.target.value)}
                      placeholder="FUP quotas, time-based speed control..."
                      className="block w-full border-[#a0a0a0] dark:bg-gray-700 dark:text-white focus:border-blue-500 focus:ring-blue-500 sm:text-[12px]"
                    />
                  </div>
                </div>

                {/* Feature 3 */}
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4 p-4 bg-gray-50 dark:bg-gray-700/50">
                  <div>
                    <label className="block text-[11px] font-medium text-gray-600 dark:text-gray-400 mb-1">Feature 3 Title</label>
                    <input
                      type="text"
                      value={formData.login_feature_3_title || ''}
                      onChange={(e) => handleChange('login_feature_3_title', e.target.value)}
                      placeholder="MikroTik Integration"
                      className="block w-full border-[#a0a0a0] dark:bg-gray-700 dark:text-white focus:border-blue-500 focus:ring-blue-500 sm:text-[12px]"
                    />
                  </div>
                  <div>
                    <label className="block text-[11px] font-medium text-gray-600 dark:text-gray-400 mb-1">Feature 3 Description</label>
                    <input
                      type="text"
                      value={formData.login_feature_3_desc || ''}
                      onChange={(e) => handleChange('login_feature_3_desc', e.target.value)}
                      placeholder="Seamless RADIUS and API integration..."
                      className="block w-full border-[#a0a0a0] dark:bg-gray-700 dark:text-white focus:border-blue-500 focus:ring-blue-500 sm:text-[12px]"
                    />
                  </div>
                </div>
              </div>

              {/* Sidebar Preview */}
              <div className="border-t dark:border-gray-700 pt-3">
                <h3 className="text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-3">Sidebar Preview</h3>
                <div className="bg-gray-100 dark:bg-gray-700 p-3">
                  <div className="bg-white dark:bg-gray-800 shadow p-4 max-w-xs">
                    <div className="flex items-center gap-3">
                      {companyLogo ? (
                        <img src={companyLogo} alt="Logo" className="h-10 object-contain" />
                      ) : (
                        <span className="text-lg font-bold" style={{ color: formData.primary_color || '#2563eb' }}>
                          {formData.company_name || 'Your Company Name'}
                        </span>
                      )}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          ) : activeTab === 'account' ? (
            <div className="space-y-3">
              {/* User Info */}
              <div>
                <h3 className="text-[13px] font-semibold text-gray-900 dark:text-white mb-3">Account Information</h3>
                <div className="bg-gray-50 dark:bg-gray-700 p-3">
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <p className="text-[12px] text-gray-500 dark:text-gray-400">Username</p>
                      <p className="font-medium">{user?.username}</p>
                    </div>
                    <div>
                      <p className="text-[12px] text-gray-500 dark:text-gray-400">Email</p>
                      <p className="font-medium">{user?.email || '-'}</p>
                    </div>
                    <div>
                      <p className="text-[12px] text-gray-500 dark:text-gray-400">Full Name</p>
                      <p className="font-medium">{user?.full_name || '-'}</p>
                    </div>
                    <div>
                      <p className="text-[12px] text-gray-500 dark:text-gray-400">Role</p>
                      <p className="font-medium capitalize">{user?.user_type}</p>
                    </div>
                  </div>
                </div>
              </div>

              {/* Two-Factor Authentication */}
              <div>
                <h3 className="text-[13px] font-semibold text-gray-900 dark:text-white mb-3">Two-Factor Authentication</h3>

                {twoFAStatus?.enabled ? (
                  <div className="bg-green-50 dark:bg-green-900/30 border border-green-200 p-3">
                    <div className="flex items-center mb-3">
                      <svg className="w-5 h-5 text-green-600 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                      </svg>
                      <span className="font-medium text-green-800">2FA is enabled</span>
                    </div>
                    <p className="text-[12px] text-green-700 mb-3">Your account is protected with two-factor authentication.</p>

                    <div className="border-t border-green-200 pt-4 mt-4">
                      <p className="text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-2">Disable 2FA</p>
                      <div className="space-y-3">
                        <input
                          type="password"
                          placeholder="Current password"
                          value={disablePassword}
                          onChange={(e) => setDisablePassword(e.target.value)}
                          className="block w-full max-w-xs border-[#a0a0a0] dark:bg-gray-700 dark:text-white focus:border-blue-500 focus:ring-blue-500 sm:text-[12px]"
                        />
                        <input
                          type="text"
                          placeholder="2FA code"
                          value={disableCode}
                          onChange={(e) => setDisableCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                          maxLength={6}
                          className="block w-full max-w-xs border-[#a0a0a0] dark:bg-gray-700 dark:text-white focus:border-blue-500 focus:ring-blue-500 sm:text-[12px]"
                        />
                        <button
                          onClick={() => disableTwoFAMutation.mutate({ password: disablePassword, code: disableCode })}
                          disabled={disableTwoFAMutation.isPending || !disablePassword || disableCode.length !== 6}
                          className="px-4 py-2 text-[12px] font-medium text-white bg-red-600 hover:bg-red-700 disabled:opacity-50"
                        >
                          {disableTwoFAMutation.isPending ? 'Disabling...' : 'Disable 2FA'}
                        </button>
                      </div>
                    </div>
                  </div>
                ) : twoFASetup ? (
                  <div className="bg-blue-50 dark:bg-blue-900/30 border border-blue-200 p-3">
                    <h4 className="font-medium text-blue-900 mb-3">Setup Two-Factor Authentication</h4>

                    <div className="flex flex-col md:flex-row gap-3">
                      <div className="flex-shrink-0">
                        <p className="text-[12px] text-blue-800 mb-2">1. Scan this QR code with your authenticator app</p>
                        <img src={twoFASetup.qr_code} alt="2FA QR Code" className="w-48 h-48 border rounded" />
                      </div>

                      <div className="flex-1">
                        <p className="text-[12px] text-blue-800 mb-2">Or enter this code manually:</p>
                        <code className="block bg-white px-3 py-2 rounded border text-[12px] font-mono mb-3 break-all">
                          {twoFASetup.secret}
                        </code>

                        <p className="text-[12px] text-blue-800 mb-2">2. Enter the 6-digit code from your app:</p>
                        <div className="flex gap-2">
                          <input
                            type="text"
                            placeholder="000000"
                            value={twoFACode}
                            onChange={(e) => setTwoFACode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                            maxLength={6}
                            className="block w-32 border-[#a0a0a0] dark:bg-gray-700 dark:text-white focus:border-blue-500 focus:ring-blue-500 text-center text-lg tracking-widest"
                          />
                          <button
                            onClick={() => verifyTwoFAMutation.mutate(twoFACode)}
                            disabled={verifyTwoFAMutation.isPending || twoFACode.length !== 6}
                            className="px-4 py-2 text-[12px] font-medium text-white bg-blue-600 hover:bg-blue-700 disabled:opacity-50"
                          >
                            {verifyTwoFAMutation.isPending ? 'Verifying...' : 'Verify & Enable'}
                          </button>
                        </div>
                      </div>
                    </div>

                    <button
                      onClick={() => { setTwoFASetup(null); setTwoFACode('') }}
                      className="mt-4 text-[12px] text-blue-600 hover:text-blue-800"
                    >
                      Cancel setup
                    </button>
                  </div>
                ) : (
                  <div className="bg-gray-50 dark:bg-gray-700 border border-[#a0a0a0] p-3">
                    <div className="flex items-center mb-3">
                      <svg className="w-5 h-5 text-gray-400 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                      </svg>
                      <span className="font-medium text-gray-700 dark:text-gray-300 dark:text-gray-400">2FA is not enabled</span>
                    </div>
                    <p className="text-[12px] text-gray-600 mb-3">
                      Add an extra layer of security to your account by enabling two-factor authentication.
                      You'll need an authenticator app like Google Authenticator or Authy.
                    </p>
                    <button
                      onClick={() => setupTwoFAMutation.mutate()}
                      disabled={setupTwoFAMutation.isPending}
                      className="px-4 py-2 text-[12px] font-medium text-white bg-blue-600 hover:bg-blue-700 disabled:opacity-50"
                    >
                      {setupTwoFAMutation.isPending ? 'Setting up...' : 'Enable 2FA'}
                    </button>
                  </div>
                )}
              </div>
            </div>
          ) : activeTab === 'notifications' ? (
            <div className="space-y-3">
              {/* Email/SMTP Settings */}
              <div className="card p-3">
                <h3 className="text-[13px] font-semibold text-gray-900 dark:text-white mb-3">Email Notifications (SMTP)</h3>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">SMTP Host</label>
                    <input
                      type="text"
                      value={formData.smtp_host || ''}
                      onChange={(e) => handleChange('smtp_host', e.target.value)}
                      placeholder="smtp.gmail.com"
                      className="w-full px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white"
                    />
                  </div>
                  <div>
                    <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">SMTP Port</label>
                    <input
                      type="number"
                      value={formData.smtp_port || ''}
                      onChange={(e) => handleChange('smtp_port', e.target.value)}
                      placeholder="587"
                      className="w-full px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white"
                    />
                  </div>
                  <div>
                    <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">SMTP Username</label>
                    <input
                      type="text"
                      value={formData.smtp_username || ''}
                      onChange={(e) => handleChange('smtp_username', e.target.value)}
                      placeholder="user@gmail.com"
                      className="w-full px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white"
                    />
                  </div>
                  <div>
                    <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">SMTP Password</label>
                    <input
                      type="password"
                      value={formData.smtp_password || ''}
                      onChange={(e) => handleChange('smtp_password', e.target.value)}
                      placeholder="••••••••"
                      className="w-full px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white"
                    />
                  </div>
                  <div>
                    <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">From Name</label>
                    <input
                      type="text"
                      value={formData.smtp_from_name || ''}
                      onChange={(e) => handleChange('smtp_from_name', e.target.value)}
                      placeholder="Company Name"
                      className="w-full px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white"
                    />
                  </div>
                  <div>
                    <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">From Email</label>
                    <input
                      type="email"
                      value={formData.smtp_from_email || ''}
                      onChange={(e) => handleChange('smtp_from_email', e.target.value)}
                      placeholder="noreply@company.com"
                      className="w-full px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white"
                    />
                  </div>
                  <div className="md:col-span-2">
                    <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">Test Email Address</label>
                    <div className="flex gap-2">
                      <input
                        type="email"
                        value={testEmail}
                        onChange={(e) => setTestEmail(e.target.value)}
                        placeholder="test@example.com"
                        className="flex-1 px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white"
                      />
                      <button
                        onClick={handleTestSmtp}
                        disabled={testingSmtp || !formData.smtp_host}
                        className="px-4 py-2 text-[12px] font-medium text-white bg-green-600 hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed"
                      >
                        {testingSmtp ? 'Testing...' : 'Test SMTP'}
                      </button>
                    </div>
                  </div>
                </div>
              </div>

              {/* SMS Settings */}
              <div className="card p-3">
                <h3 className="text-[13px] font-semibold text-gray-900 dark:text-white mb-3">SMS Notifications</h3>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div className="md:col-span-2">
                    <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">SMS Provider</label>
                    <select
                      value={formData.sms_provider || ''}
                      onChange={(e) => handleChange('sms_provider', e.target.value)}
                      className="w-full px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white"
                    >
                      <option value="">Select Provider</option>
                      <option value="twilio">Twilio</option>
                      <option value="vonage">Vonage (Nexmo)</option>
                      <option value="custom">Custom API</option>
                    </select>
                  </div>

                  {formData.sms_provider === 'twilio' && (
                    <>
                      <div>
                        <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">Twilio Account SID</label>
                        <input
                          type="text"
                          value={formData.sms_twilio_sid || ''}
                          onChange={(e) => handleChange('sms_twilio_sid', e.target.value)}
                          placeholder="ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
                          className="w-full px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white"
                        />
                      </div>
                      <div>
                        <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">Twilio Auth Token</label>
                        <input
                          type="password"
                          value={formData.sms_twilio_token || ''}
                          onChange={(e) => handleChange('sms_twilio_token', e.target.value)}
                          placeholder="••••••••"
                          className="w-full px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white"
                        />
                      </div>
                      <div>
                        <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">Twilio Phone Number</label>
                        <input
                          type="text"
                          value={formData.sms_twilio_from || ''}
                          onChange={(e) => handleChange('sms_twilio_from', e.target.value)}
                          placeholder="+1234567890"
                          className="w-full px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white"
                        />
                      </div>
                    </>
                  )}

                  {formData.sms_provider === 'vonage' && (
                    <>
                      <div>
                        <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">Vonage API Key</label>
                        <input
                          type="text"
                          value={formData.sms_vonage_key || ''}
                          onChange={(e) => handleChange('sms_vonage_key', e.target.value)}
                          placeholder="API Key"
                          className="w-full px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white"
                        />
                      </div>
                      <div>
                        <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">Vonage API Secret</label>
                        <input
                          type="password"
                          value={formData.sms_vonage_secret || ''}
                          onChange={(e) => handleChange('sms_vonage_secret', e.target.value)}
                          placeholder="••••••••"
                          className="w-full px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white"
                        />
                      </div>
                      <div>
                        <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">Sender Name/Number</label>
                        <input
                          type="text"
                          value={formData.sms_vonage_from || ''}
                          onChange={(e) => handleChange('sms_vonage_from', e.target.value)}
                          placeholder="CompanyName or +1234567890"
                          className="w-full px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white"
                        />
                      </div>
                    </>
                  )}

                  {formData.sms_provider === 'custom' && (
                    <>
                      <div className="md:col-span-2">
                        <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">API URL</label>
                        <input
                          type="text"
                          value={formData.sms_custom_url || ''}
                          onChange={(e) => handleChange('sms_custom_url', e.target.value)}
                          placeholder="https://api.provider.com/sms/send"
                          className="w-full px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white"
                        />
                      </div>
                      <div>
                        <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">HTTP Method</label>
                        <select
                          value={formData.sms_custom_method || 'POST'}
                          onChange={(e) => handleChange('sms_custom_method', e.target.value)}
                          className="w-full px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white"
                        >
                          <option value="POST">POST</option>
                          <option value="GET">GET</option>
                        </select>
                      </div>
                      <div className="md:col-span-2">
                        <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">Request Body (JSON)</label>
                        <textarea
                          value={formData.sms_custom_body || ''}
                          onChange={(e) => handleChange('sms_custom_body', e.target.value)}
                          placeholder='{"to": "{{to}}", "message": "{{message}}"}'
                          rows={3}
                          className="w-full px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white font-mono text-[12px]"
                        />
                        <p className="mt-1 text-[11px] text-gray-500 dark:text-gray-400">Use {'{{to}}'} and {'{{message}}'} as placeholders</p>
                      </div>
                    </>
                  )}

                  {formData.sms_provider && (
                    <div className="md:col-span-2">
                      <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">Test Phone Number</label>
                      <div className="flex gap-2">
                        <input
                          type="text"
                          value={testPhone}
                          onChange={(e) => setTestPhone(e.target.value)}
                          placeholder="+1234567890"
                          className="flex-1 px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white"
                        />
                        <button
                          onClick={handleTestSms}
                          disabled={testingSms || !testPhone}
                          className="px-4 py-2 text-[12px] font-medium text-white bg-green-600 hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed"
                        >
                          {testingSms ? 'Testing...' : 'Test SMS'}
                        </button>
                      </div>
                    </div>
                  )}
                </div>
              </div>

              {/* WhatsApp Settings */}
              <div className="card p-3">
                <h3 className="text-[13px] font-semibold text-gray-900 dark:text-white mb-3">WhatsApp Notifications</h3>

                {/* Provider selector */}
                <div className="mb-3">
                  <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">Provider</label>
                  <select
                    value={formData.whatsapp_provider || 'ultramsg'}
                    onChange={(e) => handleChangeAndSave('whatsapp_provider', e.target.value)}
                    className="w-full sm:w-64 px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white"
                  >
                    <option value="ultramsg">Ultramsg</option>
                    <option value="proxrad">ProxRad WhatsApp</option>
                  </select>
                </div>

                {(formData.whatsapp_provider || 'ultramsg') === 'ultramsg' ? (
                  /* Ultramsg provider */
                  <>
                    <p className="text-[12px] text-gray-500 dark:text-gray-400 mb-3">
                      Get your Instance ID and Token from <a href="https://ultramsg.com" target="_blank" rel="noopener noreferrer" className="text-blue-600 hover:underline">ultramsg.com</a>
                    </p>
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <div>
                        <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">Instance ID</label>
                        <input
                          type="text"
                          value={formData.whatsapp_instance_id || ''}
                          onChange={(e) => handleChange('whatsapp_instance_id', e.target.value)}
                          placeholder="instanceXXXXX"
                          className="w-full px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white"
                        />
                      </div>
                      <div>
                        <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">Token</label>
                        <input
                          type="password"
                          value={formData.whatsapp_token || ''}
                          onChange={(e) => handleChange('whatsapp_token', e.target.value)}
                          placeholder="••••••••"
                          className="w-full px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white"
                        />
                      </div>
                      <div className="md:col-span-2">
                        <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">Test Phone Number</label>
                        <div className="flex gap-2">
                          <input
                            type="text"
                            value={testPhone}
                            onChange={(e) => setTestPhone(e.target.value)}
                            placeholder="+1234567890 (with country code)"
                            className="flex-1 px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white"
                          />
                          <button
                            onClick={handleTestWhatsapp}
                            disabled={testingWhatsapp || !formData.whatsapp_instance_id || !formData.whatsapp_token}
                            className="px-4 py-2 text-[12px] font-medium text-white bg-green-600 hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed"
                          >
                            {testingWhatsapp ? 'Testing...' : 'Test WhatsApp'}
                          </button>
                        </div>
                      </div>
                    </div>
                  </>
                ) : (
                  /* ProxRad provider */
                  <div className="space-y-4">
                    <p className="text-[12px] text-gray-500 dark:text-gray-400">
                      Link your WhatsApp number via ProxRad (proxsms.com). Scan the QR code below to connect your number.
                    </p>

                    {/* Subscription / Trial status banner */}
                    {proxradAccess && (
                      <div className={`flex items-start gap-3 p-3 border ${
                        proxradAccess.type === 'subscribed'
                          ? 'bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-700'
                          : proxradAccess.type === 'expired'
                          ? 'bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-700'
                          : 'bg-yellow-50 dark:bg-yellow-900/20 border-yellow-200 dark:border-yellow-700'
                      }`}>
                        <span className="text-lg mt-0.5">
                          {proxradAccess.type === 'subscribed' ? '✅' : proxradAccess.type === 'expired' ? '🔒' : '⏳'}
                        </span>
                        <div>
                          {proxradAccess.type === 'subscribed' && (
                            <>
                              <p className="text-[12px] font-medium text-green-800 dark:text-green-300">Subscribed</p>
                              <p className="text-[11px] text-green-600 dark:text-green-400">
                                Active until {new Date(proxradAccess.expires_at).toLocaleDateString()}
                              </p>
                            </>
                          )}
                          {proxradAccess.type === 'trial' && proxradAccess.trial_ends && (
                            <>
                              <p className="text-[12px] font-medium text-yellow-800 dark:text-yellow-300">Trial Mode</p>
                              <p className="text-[11px] text-yellow-600 dark:text-yellow-400">
                                {proxradAccess.trial_hours_left > 0
                                  ? `${proxradAccess.trial_hours_left} hour${proxradAccess.trial_hours_left !== 1 ? 's' : ''} remaining — contact your provider to subscribe`
                                  : 'Trial ending soon'}
                              </p>
                            </>
                          )}
                          {proxradAccess.type === 'trial' && !proxradAccess.trial_ends && (
                            <>
                              <p className="text-[12px] font-medium text-yellow-800 dark:text-yellow-300">Trial Mode</p>
                              <p className="text-[11px] text-yellow-600 dark:text-yellow-400">
                                2-day trial starts when you link your number
                              </p>
                            </>
                          )}
                          {proxradAccess.type === 'expired' && (
                            <>
                              <p className="text-[12px] font-medium text-red-800 dark:text-red-300">Trial / Subscription Expired</p>
                              <p className="text-[11px] text-red-600 dark:text-red-400">
                                Sending is blocked. Contact your provider to subscribe.
                              </p>
                            </>
                          )}
                        </div>
                      </div>
                    )}

                    {/* Currently linked account */}
                    {formData.proxrad_account_unique && !proxradLinking && (
                      <div className="flex items-center gap-3 p-3 bg-green-50 dark:bg-green-900/30 border border-green-200 dark:border-green-700">
                        <span className="text-green-600 dark:text-green-400 text-xl">✅</span>
                        <div className="flex-1">
                          <p className="text-[12px] font-medium text-green-800 dark:text-green-300">WhatsApp Account Linked</p>
                          <p className="text-[11px] text-green-600 dark:text-green-400">
                            {proxradPhone || formData.proxrad_phone || formData.proxrad_account_unique}
                          </p>
                        </div>
                        <button
                          onClick={handleProxRadUnlink}
                          className="text-[11px] text-red-500 hover:text-red-700 font-medium"
                        >
                          Unlink
                        </button>
                      </div>
                    )}

                    {/* QR Code display */}
                    {proxradLinking && (
                      <div className="flex flex-col items-center gap-3 p-4 border border-blue-200 dark:border-blue-700 bg-blue-50 dark:bg-blue-900/20">
                        <p className="text-[12px] font-medium text-blue-800 dark:text-blue-300">
                          📱 Scan with WhatsApp to link your number
                        </p>
                        {proxradQrUrl ? (
                          <img
                            src={proxradQrUrl}
                            alt="WhatsApp QR Code"
                            className="w-48 h-48 border border-[#a0a0a0] bg-white p-1"
                            onError={(e) => { e.target.style.display='none' }}
                          />
                        ) : (
                          <div className="w-48 h-48 flex items-center justify-center">
                            <span className="animate-spin inline-block w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full"></span>
                          </div>
                        )}
                        <p className="text-[11px] text-blue-600 dark:text-blue-400 text-center">
                          Open WhatsApp → Linked Devices → Link a Device → Scan this QR
                        </p>
                        <p className="text-[11px] text-gray-400 animate-pulse">Waiting for connection...</p>
                        <button
                          onClick={handleProxRadCancelLink}
                          className="text-[11px] text-red-500 hover:text-red-700 underline"
                        >
                          Cancel
                        </button>
                      </div>
                    )}

                    {/* Link via QR button */}
                    {!proxradLinking && !formData.proxrad_account_unique && (
                      <div className="flex gap-2 flex-wrap">
                        <button
                          onClick={handleProxRadCreateLink}
                          className="px-5 py-2 text-[12px] font-medium text-white bg-blue-600 hover:bg-blue-700 flex items-center gap-2"
                        >
                          📱 Link via QR Code
                        </button>
                      </div>
                    )}

                    {/* Test after selected */}
                    {formData.proxrad_account_unique && !proxradLinking && (
                      <div>
                        <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">Test Phone Number</label>
                        <div className="flex gap-2">
                          <input
                            type="text"
                            value={testPhone}
                            onChange={(e) => setTestPhone(e.target.value)}
                            placeholder="+1234567890"
                            className="flex-1 px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white"
                          />
                          <button
                            onClick={handleTestProxRad}
                            disabled={testingWhatsapp || !testPhone}
                            className="px-4 py-2 text-[12px] font-medium text-white bg-green-600 hover:bg-green-700 disabled:opacity-50"
                          >
                            {testingWhatsapp ? 'Testing...' : 'Test Send'}
                          </button>
                        </div>
                      </div>
                    )}
                  </div>
                )}
              </div>

              {/* WhatsApp Subscriber Management */}
              <div className="card p-3">
                <h3 className="text-[13px] font-semibold text-gray-900 dark:text-white mb-1 flex items-center gap-2">
                  📱 Send WhatsApp to Subscribers
                </h3>
                <p className="text-[12px] text-gray-500 dark:text-gray-400 mb-3">
                  Select subscribers and send a WhatsApp message directly, or manage auto-notification settings.
                </p>

                <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
                  {/* Subscriber Selector */}
                  <div>
                    <div className="flex items-center justify-between mb-3">
                      <h4 className="text-[12px] font-semibold text-gray-900 flex items-center gap-2">
                        Select Subscribers
                      </h4>
                      <span className="text-[11px] text-gray-500 dark:text-gray-400">
                        {waSubscribers.length} with phone
                        {waSubscribers.filter(s => s.whatsapp_notifications).length > 0 && (
                          <span className="ml-2 text-green-600 dark:text-green-400 font-medium">
                            🔔 {waSubscribers.filter(s => s.whatsapp_notifications).length} notif ON
                          </span>
                        )}
                      </span>
                    </div>

                    {/* Send All toggle */}
                    <label className="flex items-center gap-2 mb-3 cursor-pointer">
                      <div
                        onClick={() => { setWaSendAll(!waSendAll); setWaSelectedIDs([]) }}
                        className={`relative w-10 h-5 transition-colors ${waSendAll ? 'bg-green-500' : 'bg-gray-300 dark:bg-gray-600'}`}
                      >
                        <div className={`absolute top-0.5 w-4 h-4 bg-white transition-transform ${waSendAll ? 'translate-x-5' : 'translate-x-0.5'}`} />
                      </div>
                      <span className="text-[12px] text-gray-700 dark:text-gray-300">
                        Send to all subscribers ({waSubscribers.length})
                      </span>
                    </label>

                    {!waSendAll && (
                      <>
                        <div className="relative mb-2">
                          <input
                            type="text"
                            value={waSubSearch}
                            onChange={e => { setWaSubSearch(e.target.value); fetchWaSubscribers(e.target.value) }}
                            placeholder="Search subscribers..."
                            className="w-full px-3 py-2 pl-8 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white text-[12px]"
                          />
                          <svg className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/></svg>
                        </div>

                        <div className="flex gap-2 mb-2 flex-wrap items-center text-[11px]">
                          <button onClick={() => setWaSelectedIDs(waSubscribers.map(s => s.id))} className="text-blue-600 hover:underline">Select all</button>
                          <span className="text-gray-300 dark:text-gray-600">|</span>
                          <button onClick={() => setWaSelectedIDs([])} className="text-gray-500 hover:underline">Clear</button>
                          <span className="text-gray-300 dark:text-gray-600">|</span>
                          <button onClick={waEnableAll} className="text-green-600 dark:text-green-400 hover:underline font-medium">🔔 All notif ON</button>
                          <span className="text-gray-300 dark:text-gray-600">|</span>
                          <button onClick={waDisableAll} className="text-gray-500 dark:text-gray-400 hover:underline font-medium">🔕 All notif OFF</button>
                          {waSelectedIDs.length > 0 && <span className="text-green-600 font-medium">{waSelectedIDs.length} selected</span>}
                        </div>

                        <div className="max-h-64 overflow-y-auto space-y-1 border border-[#a0a0a0] p-2">
                          {waSubsLoading ? (
                            <div className="text-center py-4 text-gray-400 text-[12px]">Loading...</div>
                          ) : waSubscribers.length === 0 ? (
                            <p className="text-center py-4 text-gray-400 text-[12px]">No subscribers with phone numbers</p>
                          ) : (
                            waSubscribers.map(sub => (
                              <label key={sub.id} className={`flex items-center gap-2 px-2 py-1.5 rounded cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-600 ${waSelectedIDs.includes(sub.id) ? 'bg-blue-50 dark:bg-blue-900/20' : ''}`}>
                                <input
                                  type="checkbox"
                                  checked={waSelectedIDs.includes(sub.id)}
                                  onChange={() => setWaSelectedIDs(prev => prev.includes(sub.id) ? prev.filter(x => x !== sub.id) : [...prev, sub.id])}
                                  className="rounded border-[#a0a0a0]"
                                />
                                <div className="flex-1 min-w-0">
                                  <p className="text-[12px] text-[12px] font-semibold text-gray-900 truncate">{sub.username}</p>
                                  <p className="text-[11px] text-gray-500 dark:text-gray-400 truncate">{sub.phone}</p>
                                </div>
                                <button
                                  onClick={(e) => { e.stopPropagation(); waToggleNotifications(sub) }}
                                  disabled={waTogglingId === sub.id}
                                  title={sub.whatsapp_notifications ? 'Auto-notif ON — click to disable' : 'Auto-notif OFF — click to enable'}
                                  className={`shrink-0 p-1 transition-colors ${sub.whatsapp_notifications ? 'text-green-500 hover:bg-green-100 dark:hover:bg-green-900/30' : 'text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700'} ${waTogglingId === sub.id ? 'opacity-50 cursor-wait' : ''}`}
                                >
                                  {sub.whatsapp_notifications ? (
                                    <svg className="h-4 w-4" fill="currentColor" viewBox="0 0 24 24"><path d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"/></svg>
                                  ) : (
                                    <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9M3 3l18 18"/></svg>
                                  )}
                                </button>
                              </label>
                            ))
                          )}
                        </div>
                      </>
                    )}
                  </div>

                  {/* Message Composer */}
                  <div>
                    <h4 className="text-[12px] font-semibold text-gray-900 flex items-center gap-2 mb-3">
                      ✉️ Compose Message
                    </h4>
                    <textarea
                      value={waMessage}
                      onChange={e => setWaMessage(e.target.value)}
                      rows={8}
                      placeholder={"Type your message here...\n\nYou can use:\n{username} — subscriber username\n{full_name} — subscriber full name"}
                      className="w-full px-3 py-2 border border-[#a0a0a0] dark:bg-gray-800 dark:text-white resize-none text-[12px] mb-2"
                    />
                    <div className="flex items-center justify-between mb-3">
                      <p className="text-[11px] text-gray-400">{waMessage.length} characters</p>
                      <p className="text-[11px] text-gray-400">Recipients: {waSendAll ? `All (${waSubscribers.length})` : waSelectedIDs.length}</p>
                    </div>
                    <div className="flex flex-wrap gap-1 mb-3">
                      {['{username}', '{full_name}'].map(v => (
                        <button key={v} onClick={() => setWaMessage(m => m + v)} className="text-[11px] bg-gray-100 dark:bg-gray-600 text-gray-600 dark:text-gray-300 px-2 py-0.5 rounded hover:bg-gray-200 dark:hover:bg-gray-500">
                          {v}
                        </button>
                      ))}
                    </div>
                    <button
                      onClick={waHandleSend}
                      disabled={waSending || !waMessage.trim() || (!waSendAll && waSelectedIDs.length === 0)}
                      className="w-full px-4 py-2 text-[12px] font-medium text-white bg-green-600 hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
                    >
                      {waSending ? 'Sending...' : '📤 Send WhatsApp Message'}
                    </button>
                  </div>
                </div>
              </div>

              {/* Save Button */}
              <div className="flex justify-end">
                <button
                  onClick={handleSave}
                  disabled={!hasChanges || updateMutation.isPending}
                  className="px-6 py-2 text-[12px] font-medium text-white bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {updateMutation.isPending ? 'Saving...' : 'Save Notification Settings'}
                </button>
              </div>
            </div>
          ) : activeTab === 'license' ? (
            <div className="space-y-3">
              <div className="flex justify-between items-center">
                <h3 className="text-[13px] font-semibold text-gray-900 dark:text-white">License Information</h3>
                <div className="flex gap-2">
                  <button
                    onClick={() => checkUpdateMutation.mutate()}
                    disabled={checkUpdateMutation.isPending}
                    className="inline-flex items-center px-4 py-2 text-[12px] font-medium text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 border border-[#a0a0a0] hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50"
                  >
                    {checkUpdateMutation.isPending ? (
                      <>
                        <svg className="animate-spin -ml-1 mr-2 h-4 w-4 text-gray-700 dark:text-gray-300 dark:text-gray-400" fill="none" viewBox="0 0 24 24">
                          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                        </svg>
                        Checking...
                      </>
                    ) : (
                      <>
                        <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
                        </svg>
                        Check for Updates
                      </>
                    )}
                  </button>
                  <button
                    onClick={() => revalidateMutation.mutate()}
                    disabled={revalidateMutation.isPending}
                    className="inline-flex items-center px-4 py-2 text-[12px] font-medium text-white bg-blue-600 hover:bg-blue-700 disabled:opacity-50"
                  >
                    {revalidateMutation.isPending ? (
                      <>
                        <svg className="animate-spin -ml-1 mr-2 h-4 w-4 text-white" fill="none" viewBox="0 0 24 24">
                          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                        </svg>
                        Checking...
                      </>
                    ) : (
                      <>
                        <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                        </svg>
                        Check License
                      </>
                    )}
                  </button>
                </div>
              </div>

              {licenseLoading ? (
                <div className="flex items-center justify-center h-32">
                  <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-[#316AC5]"></div>
                </div>
              ) : (
                <div className="space-y-3">
                  {/* License Status Card */}
                  {(() => {
                    const status = licenseStatus?.license_status || (licenseData?.valid ? 'active' : 'blocked')
                    const statusConfig = {
                      active: { bg: 'bg-green-50 dark:bg-green-900/30 border-green-200', text: 'text-green-800', subtext: 'text-green-700', icon: 'text-green-600', label: 'License Active', desc: 'Your license is valid and active' },
                      warning: { bg: 'bg-yellow-50 dark:bg-yellow-900/30 border-yellow-200', text: 'text-yellow-800', subtext: 'text-yellow-700', icon: 'text-yellow-600', label: 'License Expiring Soon', desc: licenseStatus?.warning_message || 'Please renew soon' },
                      grace: { bg: 'bg-orange-50 border-orange-200', text: 'text-orange-800', subtext: 'text-orange-700', icon: 'text-orange-600', label: 'Grace Period', desc: 'License expired - renew now to avoid service interruption' },
                      readonly: { bg: 'bg-red-50 dark:bg-red-900/30 border-red-200', text: 'text-red-800', subtext: 'text-red-700', icon: 'text-red-600', label: 'Read-Only Mode', desc: 'License expired - system is read-only. Renew immediately!' },
                      blocked: { bg: 'bg-red-50 dark:bg-red-900/30 border-red-200', text: 'text-red-800', subtext: 'text-red-700', icon: 'text-red-600', label: 'License Blocked', desc: licenseData?.message || 'License invalid or expired' },
                      unknown: { bg: 'bg-gray-50 dark:bg-gray-700 border-[#a0a0a0]', text: 'text-gray-800', subtext: 'text-gray-700', icon: 'text-gray-600', label: 'Unknown Status', desc: 'Unable to determine license status' }
                    }
                    const config = statusConfig[status] || statusConfig.unknown

                    return (
                      <div className={`p-3 border ${config.bg}`}>
                        <div className="flex items-start">
                          <div className={`flex-shrink-0 ${config.icon}`}>
                            {status === 'active' ? (
                              <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                              </svg>
                            ) : status === 'warning' ? (
                              <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                              </svg>
                            ) : (
                              <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                              </svg>
                            )}
                          </div>
                          <div className="ml-3 flex-1">
                            <div className="flex items-center gap-2">
                              <p className={`font-medium ${config.text}`}>{config.label}</p>
                              {licenseStatus?.read_only && (
                                <span className="px-2 py-0.5 text-[11px] font-medium bg-red-100 text-red-800 rounded">READ-ONLY</span>
                              )}
                            </div>
                            <p className={`text-[12px] ${config.subtext}`}>{config.desc}</p>
                            {licenseStatus?.days_until_expiry !== undefined && licenseStatus?.days_until_expiry !== 0 && (
                              <p className={`text-[12px] mt-1 font-medium ${config.subtext}`}>
                                {licenseStatus.days_until_expiry > 0
                                  ? `${licenseStatus.days_until_expiry} days remaining`
                                  : `${Math.abs(licenseStatus.days_until_expiry)} days overdue`
                                }
                              </p>
                            )}
                          </div>
                        </div>
                      </div>
                    )
                  })()}

                  {/* System Update Card */}
                  {updateData && (
                    <div className={`p-3 border ${updateData.update_available ? 'bg-blue-50 dark:bg-blue-900/30 border-blue-200 dark:border-blue-800' : 'bg-gray-50 dark:bg-gray-700 border-[#a0a0a0]'}`}>
                      <div className="flex items-start justify-between">
                        <div className="flex items-start">
                          <div className={`flex-shrink-0 ${updateData.update_available ? 'text-blue-600' : 'text-gray-500'}`}>
                            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
                            </svg>
                          </div>
                          <div className="ml-3">
                            <p className={`font-medium ${updateData.update_available ? 'text-blue-800' : 'text-gray-800'}`}>
                              {updateData.update_available ? 'Update Available' : 'System Up to Date'}
                            </p>
                            <p className={`text-[12px] ${updateData.update_available ? 'text-blue-700' : 'text-gray-600'}`}>
                              Current version: v{updateData.current_version || '1.0.0'}
                              {updateData.update_available && ` → v${updateData.new_version}`}
                            </p>
                            {updateData.update_available && updateData.release_notes && (
                              <p className="text-[12px] text-blue-600 mt-1">{updateData.release_notes}</p>
                            )}
                          </div>
                        </div>
                        {updateData.update_available && (
                          <button
                            onClick={() => {
                              // Already on license tab, just trigger the update
                              window.location.href = '/settings?tab=license'
                            }}
                            className="inline-flex items-center px-3 py-1.5 text-[12px] font-medium text-white bg-blue-600 hover:bg-blue-700"
                          >
                            Update Now
                          </button>
                        )}
                      </div>
                    </div>
                  )}

                  {/* Service Management Card */}
                  <div className="card overflow-hidden">
                    <div className="px-4 py-3 bg-gray-50 dark:bg-gray-700 border-b border-[#a0a0a0]">
                      <h4 className="text-[12px] font-semibold text-gray-900">Service Management</h4>
                    </div>
                    <div className="p-4">
                      <p className="text-[12px] text-gray-600 mb-3">
                        Restart services if you experience issues after updates or configuration changes.
                      </p>
                      <div className="flex flex-wrap gap-2">
                        <button
                          onClick={async () => {
                            toast.loading('Restarting API service...', { id: 'restart' })
                            try {
                              await settingsApi.restartServices(['api'])
                            } catch (err) {
                              // 502/network error is expected - API restarts before responding
                              // This is actually success, not failure
                            }
                            toast.success('API service is restarting. Page will reload in 10 seconds.', { id: 'restart' })
                            setTimeout(() => window.location.reload(), 10000)
                          }}
                          className="inline-flex items-center px-3 py-2 text-[12px] font-medium text-white bg-orange-600 hover:bg-orange-700"
                        >
                          <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                          </svg>
                          Restart API
                        </button>
                        <button
                          onClick={async () => {
                            toast.loading('Restarting all services...', { id: 'restart' })
                            try {
                              await settingsApi.restartServices(['all'])
                            } catch (err) {
                              // 502/network error is expected - API restarts before responding
                              // This is actually success, not failure
                            }
                            toast.success('All services are restarting. Page will reload in 15 seconds.', { id: 'restart' })
                            setTimeout(() => window.location.reload(), 15000)
                          }}
                          className="inline-flex items-center px-3 py-2 text-[12px] font-medium text-white bg-red-600 hover:bg-red-700"
                        >
                          <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                          </svg>
                          Restart All Services
                        </button>
                      </div>
                    </div>
                  </div>

                  {/* License Details */}
                  <div className="card overflow-hidden">
                    <div className="px-4 py-3 bg-gray-50 dark:bg-gray-700 border-b border-[#a0a0a0]">
                      <h4 className="text-[12px] font-semibold text-gray-900">License Details</h4>
                    </div>
                    <div className="divide-y divide-gray-200 dark:divide-gray-700">
                      <div className="px-4 py-3 flex justify-between">
                        <span className="text-gray-500 dark:text-gray-400">License Key</span>
                        <span className="font-mono text-[12px]">{licenseData?.license_key || '-'}</span>
                      </div>
                      <div className="px-4 py-3 flex justify-between">
                        <span className="text-gray-500 dark:text-gray-400">Customer Name</span>
                        <span className="font-medium">{licenseData?.customer_name || '-'}</span>
                      </div>
                      <div className="px-4 py-3 flex justify-between">
                        <span className="text-gray-500 dark:text-gray-400">Plan / Tier</span>
                        <span className="font-medium capitalize">{licenseData?.tier || '-'}</span>
                      </div>
                      <div className="px-4 py-3 flex justify-between">
                        <span className="text-gray-500 dark:text-gray-400">Max Subscribers</span>
                        <span className="font-medium">{licenseData?.max_subscribers?.toLocaleString() || '-'}</span>
                      </div>
                      <div className="px-4 py-3 flex justify-between">
                        <span className="text-gray-500 dark:text-gray-400">License Type</span>
                        <span className="font-medium">{licenseData?.is_lifetime ? 'Lifetime' : 'Subscription'}</span>
                      </div>
                      {!licenseData?.is_lifetime && (
                        <>
                          <div className="px-4 py-3 flex justify-between">
                            <span className="text-gray-500 dark:text-gray-400">Expires At</span>
                            <span className="font-medium">
                              {licenseData?.expires_at ? new Date(licenseData.expires_at).toLocaleDateString('en-US', {
                                year: 'numeric',
                                month: 'long',
                                day: 'numeric',
                                hour: '2-digit',
                                minute: '2-digit'
                              }) : '-'}
                            </span>
                          </div>
                          <div className="px-4 py-3 flex justify-between">
                            <span className="text-gray-500 dark:text-gray-400">Days Remaining</span>
                            <span className={`font-medium ${
                              (licenseStatus?.days_until_expiry || licenseData?.days_remaining || 0) <= 0 ? 'text-red-600' :
                              (licenseStatus?.days_until_expiry || licenseData?.days_remaining || 0) <= 7 ? 'text-red-600' :
                              (licenseStatus?.days_until_expiry || licenseData?.days_remaining || 0) <= 14 ? 'text-yellow-600' :
                              'text-green-600'
                            }`}>
                              {licenseStatus?.days_until_expiry !== undefined
                                ? (licenseStatus.days_until_expiry > 0 ? `${licenseStatus.days_until_expiry} days` : `${Math.abs(licenseStatus.days_until_expiry)} days overdue`)
                                : (licenseData?.days_remaining !== undefined ? `${licenseData.days_remaining} days` : '-')
                              }
                            </span>
                          </div>
                        </>
                      )}
                    </div>
                  </div>

                  {/* Support Contact */}
                  <div className="bg-blue-50 dark:bg-blue-900/30 border border-blue-200 p-3">
                    <h4 className="font-medium text-blue-900 mb-2">Need to upgrade or renew?</h4>
                    <p className="text-[12px] text-blue-700">
                      Contact support to upgrade your plan or renew your license.
                    </p>
                  </div>
                </div>
              )}
            </div>
          ) : activeTab === 'cluster' ? (
            <ClusterTab />
          ) : activeTab === 'system' ? (
            <div className="space-y-3">
              {/* System Info Header */}
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-[13px] font-semibold text-gray-900 dark:text-white">System Information</h3>
                  <p className="text-[12px] text-gray-500 dark:text-gray-400">Hardware specifications and system health</p>
                </div>
                <button
                  onClick={() => refetchSystemInfo()}
                  disabled={systemInfoLoading}
                  className="inline-flex items-center px-4 py-2 text-[12px] font-medium text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 border border-[#a0a0a0] hover:bg-gray-50 dark:hover:bg-gray-600"
                >
                  {systemInfoLoading ? 'Loading...' : 'Refresh'}
                </button>
              </div>

              {systemInfoLoading ? (
                <div className="flex justify-center py-12">
                  <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600"></div>
                </div>
              ) : systemInfo ? (
                <>
                  {/* Hardware Specs Grid */}
                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
                    {/* CPU */}
                    <div className="card p-3">
                      <div className="flex items-center gap-3 mb-3">
                        <div className="p-2">
                          <CpuChipIcon className="w-5 h-5 text-blue-600 dark:text-blue-400" />
                        </div>
                        <h4 className="text-[12px] font-semibold text-gray-900">CPU</h4>
                      </div>
                      <p className="text-[16px] font-bold text-gray-900 dark:text-white">{systemInfo.cpu?.cores} Cores</p>
                      <p className="text-[12px] text-gray-500 dark:text-gray-400 truncate" title={systemInfo.cpu?.model}>{systemInfo.cpu?.model}</p>
                      <div className="mt-2">
                        <div className="flex justify-between text-[12px] mb-1">
                          <span className="text-gray-500 dark:text-gray-400">Usage</span>
                          <span className="text-[12px] font-semibold text-gray-900">{systemInfo.cpu?.usage}%</span>
                        </div>
                        <div className="w-full bg-[#e0e0e0] h-2 border border-[#c0c0c0]">
                          <div
                            className={`h-2 ${
                              systemInfo.cpu?.usage > 80 ? 'bg-red-500' :
                              systemInfo.cpu?.usage > 60 ? 'bg-yellow-500' : 'bg-green-500'
                            }`}
                            style={{ width: `${Math.min(systemInfo.cpu?.usage || 0, 100)}%` }}
                          ></div>
                        </div>
                      </div>
                    </div>

                    {/* Memory */}
                    <div className="card p-3">
                      <div className="flex items-center gap-3 mb-3">
                        <div className="p-2">
                          <svg className="w-5 h-5 text-purple-600 dark:text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
                          </svg>
                        </div>
                        <h4 className="text-[12px] font-semibold text-gray-900">Memory</h4>
                      </div>
                      <p className="text-[16px] font-bold text-gray-900 dark:text-white">{systemInfo.memory?.total_gb} GB</p>
                      <p className="text-[12px] text-gray-500 dark:text-gray-400">
                        {Math.round(systemInfo.memory?.used_mb / 1024 * 10) / 10} GB used
                      </p>
                      <div className="mt-2">
                        <div className="flex justify-between text-[12px] mb-1">
                          <span className="text-gray-500 dark:text-gray-400">Usage</span>
                          <span className="text-[12px] font-semibold text-gray-900">{systemInfo.memory?.usage}%</span>
                        </div>
                        <div className="w-full bg-[#e0e0e0] h-2 border border-[#c0c0c0]">
                          <div
                            className={`h-2 ${
                              systemInfo.memory?.usage > 80 ? 'bg-red-500' :
                              systemInfo.memory?.usage > 60 ? 'bg-yellow-500' : 'bg-green-500'
                            }`}
                            style={{ width: `${Math.min(systemInfo.memory?.usage || 0, 100)}%` }}
                          ></div>
                        </div>
                      </div>
                    </div>

                    {/* Disk */}
                    <div className="card p-3">
                      <div className="flex items-center gap-3 mb-3">
                        <div className="p-2">
                          <svg className="w-5 h-5 text-orange-600 dark:text-orange-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4" />
                          </svg>
                        </div>
                        <h4 className="text-[12px] font-semibold text-gray-900">Storage</h4>
                      </div>
                      <p className="text-[16px] font-bold text-gray-900 dark:text-white">{systemInfo.disk?.total_gb} GB</p>
                      <p className="text-[12px] text-gray-500 dark:text-gray-400">
                        {systemInfo.disk?.type?.toUpperCase()} • {systemInfo.disk?.free_gb} GB free
                      </p>
                      <div className="mt-2">
                        <div className="flex justify-between text-[12px] mb-1">
                          <span className="text-gray-500 dark:text-gray-400">Usage</span>
                          <span className="text-[12px] font-semibold text-gray-900">{systemInfo.disk?.usage}%</span>
                        </div>
                        <div className="w-full bg-[#e0e0e0] h-2 border border-[#c0c0c0]">
                          <div
                            className={`h-2 ${
                              systemInfo.disk?.usage > 80 ? 'bg-red-500' :
                              systemInfo.disk?.usage > 60 ? 'bg-yellow-500' : 'bg-green-500'
                            }`}
                            style={{ width: `${Math.min(systemInfo.disk?.usage || 0, 100)}%` }}
                          ></div>
                        </div>
                      </div>
                    </div>

                    {/* Capacity */}
                    <div className="card p-3">
                      <div className="flex items-center gap-3 mb-3">
                        <div className="p-2">
                          <svg className="w-5 h-5 text-green-600 dark:text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                          </svg>
                        </div>
                        <h4 className="text-[12px] font-semibold text-gray-900">Capacity</h4>
                      </div>
                      <p className="text-[16px] font-bold text-gray-900 dark:text-white">
                        {systemInfo.capacity?.current_subscribers?.toLocaleString()}
                      </p>
                      <p className="text-[12px] text-gray-500 dark:text-gray-400">
                        of {systemInfo.capacity?.estimated_max?.toLocaleString()} max subscribers
                      </p>
                      <div className="mt-2">
                        <div className="flex justify-between text-[12px] mb-1">
                          <span className="text-gray-500 dark:text-gray-400">Usage</span>
                          <span className={`font-medium ${
                            systemInfo.capacity?.status === 'critical' ? 'text-red-600' :
                            systemInfo.capacity?.status === 'warning' ? 'text-yellow-600' :
                            'text-green-600'
                          }`}>{systemInfo.capacity?.usage_percent}%</span>
                        </div>
                        <div className="w-full bg-[#e0e0e0] h-2 border border-[#c0c0c0]">
                          <div
                            className={`h-2 ${
                              systemInfo.capacity?.status === 'critical' ? 'bg-red-500' :
                              systemInfo.capacity?.status === 'warning' ? 'bg-yellow-500' :
                              systemInfo.capacity?.status === 'moderate' ? 'bg-blue-500' :
                              'bg-green-500'
                            }`}
                            style={{ width: `${Math.min(systemInfo.capacity?.usage_percent || 0, 100)}%` }}
                          ></div>
                        </div>
                      </div>
                    </div>
                  </div>

                  {/* OS Info */}
                  <div className="card p-3">
                    <h4 className="text-[12px] font-semibold text-gray-900 mb-3">Operating System</h4>
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                      <div>
                        <p className="text-[12px] text-gray-500 dark:text-gray-400">OS</p>
                        <p className="text-[12px] font-semibold text-gray-900">{systemInfo.os?.name}</p>
                      </div>
                      <div>
                        <p className="text-[12px] text-gray-500 dark:text-gray-400">Version</p>
                        <p className="text-[12px] font-semibold text-gray-900">{systemInfo.os?.version}</p>
                      </div>
                      <div>
                        <p className="text-[12px] text-gray-500 dark:text-gray-400">Uptime</p>
                        <p className="text-[12px] font-semibold text-gray-900">{systemInfo.os?.uptime}</p>
                      </div>
                      <div>
                        <p className="text-[12px] text-gray-500 dark:text-gray-400">CPU Speed</p>
                        <p className="text-[12px] font-semibold text-gray-900">{systemInfo.cpu?.speed || 'N/A'} MHz</p>
                      </div>
                    </div>
                  </div>

                  {/* Network Configuration */}
                  <NetworkConfiguration />

                </>
              ) : (
                <div className="text-center py-12 text-gray-500 dark:text-gray-400">
                  Failed to load system information
                </div>
              )}
            </div>
          ) : activeTab === 'ssl' ? (
            <div className="space-y-3">
              <div className="card p-3">
                <div className="flex items-center gap-3 mb-3">
                  <div className="p-2">
                    <LockClosedIcon className="w-5 h-5 text-green-600 dark:text-green-400" />
                  </div>
                  <div>
                    <h3 className="text-[13px] font-semibold text-gray-900 dark:text-white">Custom Domain & SSL Certificate</h3>
                    <p className="text-[12px] text-gray-500 dark:text-gray-400">Point your domain to this server and get a free HTTPS certificate from Let's Encrypt</p>
                  </div>
                </div>

                {sslStatus?.cert_exists && (
                  <div className="mb-3 p-3 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 flex items-center gap-2">
                    <CheckCircleIcon className="w-4 h-4 text-green-600 dark:text-green-400 flex-shrink-0" />
                    <p className="text-[12px] text-green-700 dark:text-green-300">
                      SSL active — panel accessible at <strong>https://{sslStatus.domain}</strong>
                    </p>
                  </div>
                )}
                {sslStatus?.domain && !sslStatus?.cert_exists && (
                  <div className="mb-3 p-3 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 flex items-center gap-2">
                    <ExclamationTriangleIcon className="w-4 h-4 text-yellow-600 dark:text-yellow-400 flex-shrink-0" />
                    <p className="text-[12px] text-yellow-700 dark:text-yellow-300">
                      Domain configured (<strong>{sslStatus.domain}</strong>) but no certificate found — run installation below.
                    </p>
                  </div>
                )}

                <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-3">
                  <div>
                    <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">Domain / Subdomain</label>
                    <input
                      type="text"
                      value={sslDomain}
                      onChange={e => setSslDomain(e.target.value.trim())}
                      placeholder="panel.yourisp.com"
                      className="input w-full"
                      disabled={sslStreaming}
                    />
                    <p className="mt-1 text-[11px] text-gray-500 dark:text-gray-400">Must point to this server's public IP address via DNS</p>
                  </div>
                  <div>
                    <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">Email (Let's Encrypt)</label>
                    <input
                      type="email"
                      value={sslEmail}
                      onChange={e => setSslEmail(e.target.value.trim())}
                      placeholder="admin@yourisp.com"
                      className="input w-full"
                      disabled={sslStreaming}
                    />
                    <p className="mt-1 text-[11px] text-gray-500 dark:text-gray-400">Used for certificate expiry notifications</p>
                  </div>
                </div>

                <button
                  onClick={handleInstallSSL}
                  disabled={sslStreaming || !sslDomain || !sslEmail}
                  className="btn btn-primary disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {sslStreaming ? (
                    <span className="flex items-center gap-2">
                      <svg className="animate-spin w-4 h-4" fill="none" viewBox="0 0 24 24">
                        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
                      </svg>
                      Installing SSL...
                    </span>
                  ) : sslStatus?.cert_exists ? 'Renew SSL Certificate' : 'Get SSL Certificate'}
                </button>

                {sslLog.length > 0 && (
                  <div className="mt-4 bg-gray-900 p-3 font-mono text-[11px] max-h-80 overflow-y-auto space-y-0.5">
                    {sslLog.map((line, i) => (
                      <div key={i} className={
                        line.includes('❌') ? 'text-red-400' :
                        line.includes('✅') || line.startsWith('✓') ? 'text-green-400' :
                        line.includes('⚠') ? 'text-yellow-400' :
                        'text-gray-300'
                      }>{line}</div>
                    ))}
                  </div>
                )}
              </div>

    {/* Remote Access Card */}
    <div className="card p-3">
      <div className="flex items-center gap-3 mb-3">
        <div className="p-2">
          <GlobeAltIcon className="w-5 h-5 text-blue-600 dark:text-blue-400" />
        </div>
        <div>
          <h3 className="text-[13px] font-semibold text-gray-900 dark:text-white">Remote Access</h3>
          <p className="text-[12px] text-gray-500 dark:text-gray-400">Access your panel from anywhere via a secure ProxRad tunnel — no port forwarding required</p>
        </div>
      </div>

      {tunnelError && (
        <div className="mb-3 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 flex items-center gap-2">
          <ExclamationTriangleIcon className="w-4 h-4 text-red-500 flex-shrink-0" />
          <p className="text-[12px] text-red-700 dark:text-red-300">{tunnelError}</p>
        </div>
      )}

      {tunnelLoading && !tunnelStatus ? (
        <div className="flex items-center gap-3 py-4">
          <svg className="animate-spin w-5 h-5 text-blue-500" fill="none" viewBox="0 0 24 24">
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
          </svg>
          <span className="text-[12px] text-gray-500 dark:text-gray-400">Loading tunnel status...</span>
        </div>
      ) : tunnelStatus && (
        <div className={`p-4 rounded-xl border ${tunnelStatus.running ? 'bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-800' : 'bg-gray-50 dark:bg-gray-700 border-[#a0a0a0]'}`}>
          <div className="flex items-center justify-between flex-wrap gap-3">
            <div className="flex items-center gap-3">
              <div className={`w-3 h-3 rounded-full ${tunnelStatus.running ? 'bg-green-500 animate-pulse' : 'bg-gray-400 dark:bg-gray-500'}`} />
              <div>
                <p className="text-[12px] font-semibold text-gray-900 text-[12px]">
                  {tunnelStatus.running ? 'Tunnel Active' : 'Tunnel Inactive'}
                </p>
                <p className="text-[11px] text-gray-500 dark:text-gray-400">
                  {tunnelStatus.running ? 'Remote access is enabled' : 'Click Enable to start remote access'}
                </p>
              </div>
            </div>
            <button
              onClick={tunnelStatus.running ? handleDisableTunnel : handleEnableTunnel}
              disabled={tunnelLoading}
              className={`btn text-[12px] disabled:opacity-50 disabled:cursor-not-allowed ${tunnelStatus.running ? 'btn-danger' : 'btn-primary'}`}
            >
              {tunnelLoading ? 'Please wait...' : tunnelStatus.running ? 'Disable' : 'Enable Remote Access'}
            </button>
          </div>

          {tunnelStatus.running && tunnelStatus.url && (
            <div className="mt-4 pt-4 border-t border-green-200 dark:border-green-800">
              <p className="text-[11px] font-medium text-gray-500 dark:text-gray-400 mb-2">Your Remote Access URL</p>
              <div className="flex items-center gap-2">
                <code className="flex-1 px-3 py-2 bg-white dark:bg-gray-800 border border-green-200 dark:border-green-700 text-[12px] text-green-700 dark:text-green-300 font-mono truncate">
                  {tunnelStatus.url}
                </code>
                <button
                  onClick={() => { copyToClipboard(tunnelStatus.url).then(() => toast.success('URL copied!')).catch(() => toast.error('Copy failed')) }}
                  className="btn btn-secondary text-[11px] px-3 py-2 whitespace-nowrap"
                >
                  Copy
                </button>
                <a
                  href={tunnelStatus.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="btn btn-secondary text-[11px] px-3 py-2 whitespace-nowrap"
                >
                  Open
                </a>
              </div>
              <p className="mt-2 text-[11px] text-gray-500 dark:text-gray-400">
                Share this URL with anyone who needs access. The connection is secured by Cloudflare.
              </p>
            </div>
          )}
        </div>
      )}

    </div>
            </div>
          ) : activeTab === 'api_keys' ? (
            <APIKeysTab />
          ) : (
            <>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
              {settingGroups[activeTab]?.map(field => (
                <div key={field.key} className={field.type === 'textarea' ? 'md:col-span-2' : ''}>
                  <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-2">
                    {field.label}
                  </label>
                  {renderField(field)}
                </div>
              ))}
            </div>

            {/* Reseller WAN Check Settings - RADIUS tab, reseller only */}
            {activeTab === 'radius' && isReseller() && hasPermission('settings.wan_check') && resellerWanData && (
            <div className="wb-group mt-3">
              <div className="wb-group-title">My WAN Management Check</div>
              <div className="wb-group-body space-y-3">
                <div className="text-[11px] text-gray-500 dark:text-gray-400 mb-2">
                  Override the global WAN management check for your subscribers.
                </div>
                <div>
                  <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">WAN Check Mode</label>
                  <select
                    value={resellerWanData.wan_check_enabled === null ? 'global' : resellerWanData.wan_check_enabled ? 'enabled' : 'disabled'}
                    onChange={e => {
                      const v = e.target.value
                      saveResellerWanMutation.mutate({
                        wan_check_enabled: v === 'global' ? null : v === 'enabled',
                        wan_check_icmp: resellerWanData.wan_check_icmp,
                        wan_check_port: resellerWanData.wan_check_port,
                      })
                    }}
                    className="input text-[12px] w-full max-w-xs"
                  >
                    <option value="global">Follow Global Setting</option>
                    <option value="enabled">Enabled</option>
                    <option value="disabled">Disabled</option>
                  </select>
                </div>
                {resellerWanData.wan_check_enabled !== false && (
                  <div className="flex gap-6 mt-1">
                    <label className="flex items-center gap-2 text-[12px]">
                      <input type="checkbox" checked={resellerWanData.wan_check_icmp}
                        onChange={e => saveResellerWanMutation.mutate({
                          wan_check_enabled: resellerWanData.wan_check_enabled,
                          wan_check_icmp: e.target.checked,
                          wan_check_port: resellerWanData.wan_check_port,
                        })} />
                      ICMP Ping Check
                    </label>
                    <label className="flex items-center gap-2 text-[12px]">
                      <input type="checkbox" checked={resellerWanData.wan_check_port}
                        onChange={e => saveResellerWanMutation.mutate({
                          wan_check_enabled: resellerWanData.wan_check_enabled,
                          wan_check_icmp: resellerWanData.wan_check_icmp,
                          wan_check_port: e.target.checked,
                        })} />
                      WAN Port Check
                    </label>
                  </div>
                )}
              </div>
            </div>
            )}

            {/* Subscriber Automation - Billing tab only */}
            {activeTab === 'billing' && (
            <div className="wb-group mt-3">
              <div className="wb-group-title">Subscriber Automation</div>
              <div className="wb-group-body space-y-3">
                <div className="flex items-center justify-between">
                  <div>
                    <div className="text-[12px] font-semibold dark:text-white">Overdue Auto-Suspend</div>
                    <div className="text-[10px] text-gray-500 dark:text-gray-400">Automatically suspend subscribers who are overdue by X days</div>
                  </div>
                  <div className="flex items-center gap-2">
                    <input
                      type="number"
                      value={formData.overdue_suspend_days || '7'}
                      onChange={e => { setFormData(p => ({...p, overdue_suspend_days: e.target.value})); setHasChanges(true) }}
                      className="input input-sm w-16 text-center"
                      min="1" max="90"
                      placeholder="7"
                    />
                    <span className="text-[10px] text-gray-500 dark:text-gray-400">days</span>
                    <label className="relative inline-flex items-center cursor-pointer">
                      <input
                        type="checkbox"
                        checked={formData.overdue_suspend_enabled === 'true' || formData.overdue_suspend_enabled === true}
                        onChange={e => { setFormData(p => ({...p, overdue_suspend_enabled: e.target.checked ? 'true' : 'false'})); setHasChanges(true) }}
                        className="sr-only peer"
                      />
                      <div className="w-8 h-4 bg-gray-200 peer-focus:outline-none rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-3 after:w-3 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
                    </label>
                  </div>
                </div>
                <div className="flex items-center justify-between">
                  <div>
                    <div className="text-[12px] font-semibold dark:text-white">Auto-Archive Expired</div>
                    <div className="text-[10px] text-gray-500 dark:text-gray-400">Automatically archive (soft-delete) subscribers expired for X days</div>
                  </div>
                  <div className="flex items-center gap-2">
                    <input
                      type="number"
                      value={formData.auto_archive_days || '30'}
                      onChange={e => { setFormData(p => ({...p, auto_archive_days: e.target.value})); setHasChanges(true) }}
                      className="input input-sm w-16 text-center"
                      min="1" max="365"
                      placeholder="30"
                    />
                    <span className="text-[10px] text-gray-500 dark:text-gray-400">days</span>
                    <label className="relative inline-flex items-center cursor-pointer">
                      <input
                        type="checkbox"
                        checked={formData.auto_archive_enabled === 'true' || formData.auto_archive_enabled === true}
                        onChange={e => { setFormData(p => ({...p, auto_archive_enabled: e.target.checked ? 'true' : 'false'})); setHasChanges(true) }}
                        className="sr-only peer"
                      />
                      <div className="w-8 h-4 bg-gray-200 peer-focus:outline-none rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-3 after:w-3 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
                    </label>
                  </div>
                </div>
              </div>
            </div>
            )}

            {/* Mobile App QR Code - General tab only */}
            {activeTab === 'general' && (() => {
              const companyName = formData.company_name || 'ProxPanel'
              const localUrl = window.location.origin
              const remoteUrl = tunnelStatus?.running && tunnelStatus?.url ? tunnelStatus.url : null
              const downloadQr = (elId, filename) => {
                const svg = document.querySelector(`#${elId} svg`)
                if (!svg) return
                const svgData = new XMLSerializer().serializeToString(svg)
                const canvas = document.createElement('canvas')
                canvas.width = 400; canvas.height = 400
                const ctx = canvas.getContext('2d')
                const img = new Image()
                img.onload = () => {
                  ctx.fillStyle = '#ffffff'
                  ctx.fillRect(0, 0, 400, 400)
                  ctx.drawImage(img, 0, 0, 400, 400)
                  const link = document.createElement('a')
                  link.download = filename
                  link.href = canvas.toDataURL('image/png')
                  link.click()
                }
                img.src = 'data:image/svg+xml;base64,' + btoa(unescape(encodeURIComponent(svgData)))
              }
              const printQr = (elId, label, url) => {
                const printWin = window.open('', '_blank', 'width=400,height=500')
                const svg = document.querySelector(`#${elId} svg`)
                if (!printWin || !svg) return
                printWin.document.write(`<html><head><title>${label}</title></head><body style="display:flex;flex-direction:column;align-items:center;justify-content:center;min-height:100vh;margin:0;font-family:sans-serif;"><div>${svg.outerHTML}</div><p style="margin-top:16px;font-size:14px;color:#666;">Scan with ProxPanel app</p><p style="font-size:12px;color:#999;">${url}</p></body></html>`)
                printWin.document.close()
                printWin.focus()
                setTimeout(() => printWin.print(), 300)
              }
              return (
              <div className="mt-8 border border-[#a0a0a0] p-6">
                <h3 className="text-[13px] font-semibold text-gray-900 dark:text-white mb-1">Mobile App QR Code</h3>
                <p className="text-[12px] text-gray-500 dark:text-gray-400 mb-3">
                  Customers scan this QR code with the ProxPanel mobile app to connect to your panel.
                </p>
                <div className={`grid gap-3 ${remoteUrl ? 'grid-cols-1 md:grid-cols-2' : 'grid-cols-1'}`}>
                  {/* Local / Direct URL QR */}
                  <div className="border border-[#a0a0a0] p-5">
                    <div className="flex items-center gap-2 mb-3">
                      <div className="w-2.5 h-2.5 rounded-full bg-blue-500" />
                      <h4 className="text-[12px] font-semibold text-gray-900 dark:text-white">Local Network</h4>
                    </div>
                    <div className="flex flex-col items-center">
                      <div className="bg-white p-3 border border-gray-100 mb-3">
                        <QRCodeSVG
                          value={JSON.stringify({ server: localUrl, name: companyName })}
                          size={180}
                          level="M"
                          includeMargin={false}
                        />
                      </div>
                      <code className="bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded text-[11px] text-gray-700 dark:text-gray-300 mb-3 max-w-full truncate">{localUrl}</code>
                      <p className="text-[11px] text-gray-500 dark:text-gray-400 text-center mb-3">For customers connected to your local network</p>
                      <div className="flex gap-2">
                        <button type="button" onClick={() => downloadQr('qr-local', 'proxpanel-local-qr.png')} className="inline-flex items-center px-3 py-1.5 text-[11px] font-medium border border-[#a0a0a0] dark:border-gray-500 text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600">Download</button>
                        <button type="button" onClick={() => printQr('qr-local', 'Local Network QR', localUrl)} className="inline-flex items-center px-3 py-1.5 text-[11px] font-medium border border-[#a0a0a0] dark:border-gray-500 text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600">Print</button>
                      </div>
                    </div>
                    <div id="qr-local" className="hidden">
                      <QRCodeSVG value={JSON.stringify({ server: localUrl, name: companyName })} size={400} level="M" includeMargin={true} />
                    </div>
                  </div>

                  {/* Remote Access URL QR */}
                  {remoteUrl && (
                  <div className="border border-green-200 dark:border-green-700 p-5 bg-green-50/50 dark:bg-green-900/10">
                    <div className="flex items-center gap-2 mb-3">
                      <div className="w-2.5 h-2.5 rounded-full bg-green-500 animate-pulse" />
                      <h4 className="text-[12px] font-semibold text-gray-900 dark:text-white">Remote Access</h4>
                    </div>
                    <div className="flex flex-col items-center">
                      <div className="bg-white p-3 border border-green-100 mb-3">
                        <QRCodeSVG
                          value={JSON.stringify({ server: remoteUrl, name: companyName })}
                          size={180}
                          level="M"
                          includeMargin={false}
                        />
                      </div>
                      <code className="bg-green-100 dark:bg-green-900/30 px-2 py-1 rounded text-[11px] text-green-800 dark:text-green-300 mb-3 max-w-full truncate">{remoteUrl}</code>
                      <p className="text-[11px] text-gray-500 dark:text-gray-400 text-center mb-3">For customers connecting from anywhere via internet</p>
                      <div className="flex gap-2">
                        <button type="button" onClick={() => downloadQr('qr-remote', 'proxpanel-remote-qr.png')} className="inline-flex items-center px-3 py-1.5 text-[11px] font-medium border border-green-300 dark:border-green-600 text-green-700 dark:text-green-300 bg-white dark:bg-green-900/20 hover:bg-green-50 dark:hover:bg-green-900/30">Download</button>
                        <button type="button" onClick={() => printQr('qr-remote', 'Remote Access QR', remoteUrl)} className="inline-flex items-center px-3 py-1.5 text-[11px] font-medium border border-green-300 dark:border-green-600 text-green-700 dark:text-green-300 bg-white dark:bg-green-900/20 hover:bg-green-50 dark:hover:bg-green-900/30">Print</button>
                      </div>
                    </div>
                    <div id="qr-remote" className="hidden">
                      <QRCodeSVG value={JSON.stringify({ server: remoteUrl, name: companyName })} size={400} level="M" includeMargin={true} />
                    </div>
                  </div>
                  )}
                </div>
              </div>
              )
            })()}

            {/* Maintenance Windows - General tab only */}
            {activeTab === 'general' && (
            <div className="wb-group mt-3">
              <div className="wb-group-title flex items-center justify-between">
                <span>Maintenance Windows</span>
                <button onClick={() => { setEditingMaintenance(null); setMaintenanceForm({ title: '', message: '', start_time: '', end_time: '', notify_subscribers: false }); setShowMaintenanceModal(true) }} className="btn btn-sm btn-primary">Add Window</button>
              </div>
              <div className="wb-group-body p-0">
                {maintenanceWindows.length === 0 ? (
                  <div className="text-center text-gray-500 dark:text-gray-400 py-4 text-[11px]">No maintenance windows configured</div>
                ) : (
                  <table className="table">
                    <thead>
                      <tr>
                        <th>Title</th>
                        <th>Start</th>
                        <th>End</th>
                        <th>Status</th>
                        <th>Actions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {maintenanceWindows.map(w => {
                        const now = new Date()
                        const start = new Date(w.start_time)
                        const end = new Date(w.end_time)
                        const status = now >= start && now <= end ? 'Active' : now < start ? 'Scheduled' : 'Past'
                        return (
                          <tr key={w.id}>
                            <td className="font-semibold">{w.title}</td>
                            <td className="text-[10px]">{new Date(w.start_time).toLocaleString()}</td>
                            <td className="text-[10px]">{new Date(w.end_time).toLocaleString()}</td>
                            <td>
                              <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${
                                status === 'Active' ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-300' :
                                status === 'Scheduled' ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300' :
                                'bg-gray-100 text-gray-500 dark:bg-gray-700 dark:text-gray-400'
                              }`}>{status}</span>
                            </td>
                            <td>
                              <div className="flex gap-1">
                                <button onClick={() => { setEditingMaintenance(w); setMaintenanceForm({ title: w.title, message: w.message || '', start_time: w.start_time?.slice(0, 16) || '', end_time: w.end_time?.slice(0, 16) || '', notify_subscribers: w.notify_subscribers || false }); setShowMaintenanceModal(true) }} className="btn btn-sm" style={{ padding: '1px 4px' }}>Edit</button>
                                <button onClick={() => handleDeleteMaintenance(w.id)} className="btn btn-sm btn-danger" style={{ padding: '1px 4px' }}>Del</button>
                              </div>
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                )}
              </div>
            </div>
            )}
            </>
          )}
        </div>
      </div>

      {/* Maintenance Window Modal */}
      {showMaintenanceModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="bg-white dark:bg-[#2b2b2b] border border-[#a0a0a0] dark:border-[#555] w-full max-w-md" style={{ borderRadius: 2 }}>
            <div className="flex items-center justify-between px-3 py-1.5 bg-[#f0f0f0] dark:bg-[#1f1f1f] border-b border-[#a0a0a0] dark:border-[#555]">
              <span className="text-[12px] font-semibold dark:text-white">{editingMaintenance ? 'Edit' : 'New'} Maintenance Window</span>
              <button onClick={() => setShowMaintenanceModal(false)} className="hover:bg-gray-200 dark:hover:bg-gray-600 p-0.5 rounded">
                <span className="text-[14px] dark:text-white">&times;</span>
              </button>
            </div>
            <div className="p-3 space-y-2">
              <div>
                <label className="text-[11px] text-gray-600 dark:text-gray-400">Title</label>
                <input type="text" value={maintenanceForm.title} onChange={e => setMaintenanceForm(p => ({...p, title: e.target.value}))} className="input input-sm w-full" placeholder="Scheduled maintenance" />
              </div>
              <div>
                <label className="text-[11px] text-gray-600 dark:text-gray-400">Message</label>
                <textarea value={maintenanceForm.message} onChange={e => setMaintenanceForm(p => ({...p, message: e.target.value}))} className="input input-sm w-full" rows={2} placeholder="Description of maintenance..." />
              </div>
              <div className="grid grid-cols-2 gap-2">
                <div>
                  <label className="text-[11px] text-gray-600 dark:text-gray-400">Start Time</label>
                  <input type="datetime-local" value={maintenanceForm.start_time} onChange={e => setMaintenanceForm(p => ({...p, start_time: e.target.value}))} className="input input-sm w-full" />
                </div>
                <div>
                  <label className="text-[11px] text-gray-600 dark:text-gray-400">End Time</label>
                  <input type="datetime-local" value={maintenanceForm.end_time} onChange={e => setMaintenanceForm(p => ({...p, end_time: e.target.value}))} className="input input-sm w-full" />
                </div>
              </div>
              <div className="flex items-center gap-2">
                <input type="checkbox" checked={maintenanceForm.notify_subscribers} onChange={e => setMaintenanceForm(p => ({...p, notify_subscribers: e.target.checked}))} className="rounded" />
                <label className="text-[11px] dark:text-gray-300">Notify subscribers via WhatsApp</label>
              </div>
              <button onClick={handleSaveMaintenance} disabled={!maintenanceForm.title || !maintenanceForm.start_time || !maintenanceForm.end_time} className="btn btn-primary btn-sm w-full">
                {editingMaintenance ? 'Update' : 'Create'} Window
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Quick Stats */}
      <div className="bg-gray-50 dark:bg-gray-700 p-3">
        <p className="text-[12px] text-gray-500 dark:text-gray-400">
          {data?.length || 0} settings configured •
          {hasChanges ? ' Unsaved changes' : ' All changes saved'}
        </p>
      </div>
    </div>
  )
}
