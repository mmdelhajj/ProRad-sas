import { useState, useEffect, useRef } from 'react'
import { useAuthStore } from '../store/authStore'
import api from '../services/api'
import toast from 'react-hot-toast'
import {
  DevicePhoneMobileIcon,
  CheckCircleIcon,
  XCircleIcon,
  ArrowPathIcon,
  PaperAirplaneIcon,
  UserGroupIcon,
  MagnifyingGlassIcon,
  QrCodeIcon,
} from '@heroicons/react/24/outline'

export default function WhatsAppSettings() {
  const { user } = useAuthStore()

  // Connection state
  const [settings, setSettings] = useState(null)
  const [loadingSettings, setLoadingSettings] = useState(true)

  // QR linking state
  const [linking, setLinking] = useState(false)
  const [qrImageUrl, setQrImageUrl] = useState('')
  const [infoUrl, setInfoUrl] = useState('')
  const pollRef = useRef(null)

  // Subscribers state
  const [subscribers, setSubscribers] = useState([])
  const [loadingSubs, setLoadingSubs] = useState(false)
  const [subSearch, setSubSearch] = useState('')
  const [selectedIDs, setSelectedIDs] = useState([])
  const [sendAll, setSendAll] = useState(false)

  // Message state
  const [message, setMessage] = useState('')
  const [sending, setSending] = useState(false)
  const [testPhone, setTestPhone] = useState('')
  const [testSending, setTestSending] = useState(false)

  const [togglingId, setTogglingId] = useState(null)

  // Load settings on mount
  useEffect(() => {
    fetchSettings()
    fetchSubscribers()
  }, [])

  const fetchSettings = async () => {
    setLoadingSettings(true)
    try {
      const res = await api.get('/reseller/whatsapp/settings')
      if (res.data.success) setSettings(res.data)
    } catch (e) {
      console.error(e)
    }
    setLoadingSettings(false)
  }

  const fetchSubscribers = async (search = '') => {
    setLoadingSubs(true)
    try {
      const res = await api.get('/reseller/whatsapp/subscribers', { params: { search } })
      if (res.data.success) setSubscribers(res.data.subscribers || [])
    } catch (e) {
      console.error(e)
    }
    setLoadingSubs(false)
  }

  // ── QR Linking ──────────────────────────────────────────────
  const handleCreateLink = async () => {
    setLinking(true)
    setQrImageUrl('')
    setInfoUrl('')
    try {
      const res = await api.get('/reseller/whatsapp/proxrad/create-link')
      if (res.data.success) {
        setQrImageUrl(res.data.qr_image_url)
        setInfoUrl(res.data.info_url)
        // Start polling
        pollRef.current = setInterval(() => pollLinkStatus(res.data.info_url), 3000)
      } else {
        toast.error(res.data.message || 'Failed to create link')
        setLinking(false)
      }
    } catch (e) {
      toast.error('Failed to create WhatsApp link')
      setLinking(false)
    }
  }

  const pollLinkStatus = async (url) => {
    try {
      const res = await api.get('/reseller/whatsapp/proxrad/link-status', { params: { info_url: url } })
      if (res.data.connected) {
        clearInterval(pollRef.current)
        setLinking(false)
        setQrImageUrl('')
        toast.success('WhatsApp connected successfully!')
        fetchSettings()
      }
    } catch (e) {
      console.error('Poll error:', e)
    }
  }

  const handleCancelLink = () => {
    clearInterval(pollRef.current)
    setLinking(false)
    setQrImageUrl('')
    setInfoUrl('')
  }

  const handleUnlink = async () => {
    if (!confirm('Disconnect your WhatsApp account?')) return
    try {
      const res = await api.delete('/reseller/whatsapp/proxrad/unlink')
      if (res.data.success) {
        toast.success('WhatsApp disconnected')
        fetchSettings()
      }
    } catch (e) {
      toast.error('Failed to unlink')
    }
  }

  // ── Test Send ────────────────────────────────────────────────
  const handleTestSend = async () => {
    if (!testPhone.trim()) { toast.error('Enter a phone number'); return }
    setTestSending(true)
    try {
      const res = await api.post('/reseller/whatsapp/proxrad/test-send', { test_phone: testPhone.trim() })
      if (res.data.success) toast.success(res.data.message)
      else toast.error(res.data.message)
    } catch (e) {
      toast.error(e?.response?.data?.message || 'Failed to send test')
    }
    setTestSending(false)
  }

  // ── Notification toggle ──────────────────────────────────────
  const toggleNotifications = async (sub) => {
    setTogglingId(sub.id)
    try {
      const res = await api.post(`/reseller/whatsapp/subscribers/${sub.id}/toggle-notifications`)
      if (res.data.success) {
        setSubscribers(prev => prev.map(s =>
          s.id === sub.id ? { ...s, whatsapp_notifications: res.data.whatsapp_notifications } : s
        ))
      }
    } catch (e) {
      console.error('Toggle failed', e)
    } finally {
      setTogglingId(null)
    }
  }

  const enableAllNotifications = async () => {
    try {
      await api.post('/reseller/whatsapp/notifications/set-all', { enabled: true })
      setSubscribers(prev => prev.map(s => ({ ...s, whatsapp_notifications: true })))
    } catch (e) {
      console.error('Enable all failed', e)
    }
  }

  const disableAllNotifications = async () => {
    try {
      await api.post('/reseller/whatsapp/notifications/set-all', { enabled: false })
      setSubscribers(prev => prev.map(s => ({ ...s, whatsapp_notifications: false })))
    } catch (e) {
      console.error('Disable all failed', e)
    }
  }

  // ── Subscriber selection ─────────────────────────────────────
  const toggleSubscriber = (id) => {
    setSelectedIDs(prev => prev.includes(id) ? prev.filter(x => x !== id) : [...prev, id])
  }

  const handleSearchChange = (v) => {
    setSubSearch(v)
    fetchSubscribers(v)
  }

  // ── Send Message ─────────────────────────────────────────────
  const handleSend = async () => {
    if (!message.trim()) { toast.error('Enter a message'); return }
    if (!sendAll && selectedIDs.length === 0) { toast.error('Select at least one subscriber'); return }

    const count = sendAll ? subscribers.length : selectedIDs.length
    if (!confirm(`Send message to ${count} subscriber${count !== 1 ? 's' : ''}?`)) return

    setSending(true)
    try {
      const res = await api.post('/reseller/whatsapp/send', {
        message: message.trim(),
        send_all: sendAll,
        subscriber_ids: sendAll ? [] : selectedIDs,
      })
      if (res.data.success) {
        toast.success(`Sent to ${res.data.sent} subscribers${res.data.failed > 0 ? `, ${res.data.failed} failed` : ''}`)
        if (res.data.failed === 0) {
          setMessage('')
          setSelectedIDs([])
          setSendAll(false)
        }
      } else {
        toast.error(res.data.message || 'Send failed')
      }
    } catch (e) {
      toast.error(e?.response?.data?.message || 'Failed to send')
    }
    setSending(false)
  }

  // ── Render ───────────────────────────────────────────────────
  const connected = settings?.connected
  const phone = settings?.phone
  const subCanUse = settings?.sub_can_use !== false // fail-open default
  const subType = settings?.sub_type || 'trial'
  const subDaysLeft = settings?.sub_days_left || 0
  const subTrialEnd = settings?.sub_trial_end
  const subExpiresAt = settings?.sub_expires_at

  const SubStatusBadge = () => {
    if (!settings) return null
    if (subType === 'active') {
      return (
        <span className="badge badge-success flex items-center gap-0.5">
          <CheckCircleIcon className="w-3 h-3" /> Active - {subDaysLeft}d left
        </span>
      )
    }
    if (subType === 'trial' && subCanUse) {
      return (
        <span className="badge badge-info flex items-center gap-0.5">
          <ArrowPathIcon className="w-3 h-3" /> Trial - {subDaysLeft}d left
        </span>
      )
    }
    return (
      <span className="badge badge-danger flex items-center gap-0.5">
        <XCircleIcon className="w-3 h-3" /> {subType === 'cancelled' ? 'Cancelled' : 'Expired'}
      </span>
    )
  }

  return (
    <div className="space-y-3" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Toolbar */}
      <div className="wb-toolbar">
        <DevicePhoneMobileIcon className="w-4 h-4 text-[#4CAF50]" />
        <span className="text-[13px] font-semibold text-gray-900">WhatsApp Notifications</span>
      </div>

      {/* Subscription Status Banner */}
      {settings && (
        <div className={`p-2 flex items-center justify-between text-[12px] border ${
          !subCanUse
            ? 'border-[#f44336] bg-[#ffe0e0]'
            : subType === 'trial'
            ? 'border-[#2196F3] bg-[#e3f2fd]'
            : 'border-[#4CAF50] bg-[#e8f5e9]'
        }`} style={{ borderRadius: '2px' }}>
          <div>
            <p className="font-medium text-gray-800">
              {!subCanUse
                ? 'WhatsApp subscription expired -- contact your provider to activate'
                : subType === 'trial'
                ? `Free trial -- ${subDaysLeft} day${subDaysLeft !== 1 ? 's' : ''} remaining`
                : `Subscription active -- ${subDaysLeft} day${subDaysLeft !== 1 ? 's' : ''} remaining`}
            </p>
            {subType === 'trial' && subTrialEnd && (
              <p className="text-[11px] text-gray-600 mt-0.5">
                Trial ends: {new Date(subTrialEnd).toLocaleDateString()}
              </p>
            )}
            {subType === 'active' && subExpiresAt && (
              <p className="text-[11px] text-gray-600 mt-0.5">
                Expires: {new Date(subExpiresAt).toLocaleDateString()}
              </p>
            )}
          </div>
          <SubStatusBadge />
        </div>
      )}

      {/* Connection Card */}
      <div className="wb-group">
        <div className="wb-group-title flex items-center justify-between">
          <span className="flex items-center gap-1">
            <QrCodeIcon className="w-4 h-4 text-[#4CAF50]" />
            WhatsApp Connection
          </span>
          {connected && (
            <span className="badge badge-success flex items-center gap-0.5">
              <CheckCircleIcon className="w-3 h-3" /> Connected
            </span>
          )}
        </div>
        <div className="wb-group-body">
          {loadingSettings ? (
            <div className="flex items-center gap-1 text-[12px] text-gray-500"><ArrowPathIcon className="w-3.5 h-3.5 animate-spin" /> Loading...</div>
          ) : connected ? (
            <div className="space-y-3">
              {/* Connected info */}
              <div className="flex items-center gap-2 p-2 border border-[#4CAF50] bg-[#e8f5e9]" style={{ borderRadius: '2px' }}>
                <DevicePhoneMobileIcon className="w-5 h-5 text-[#4CAF50] flex-shrink-0" />
                <div>
                  <p className="text-[12px] font-medium text-[#2e7d32]">Connected</p>
                  {phone && <p className="text-[12px] text-[#4CAF50]">{phone}</p>}
                </div>
              </div>

              {/* Test Send */}
              <div>
                <label className="label">Send Test Message</label>
                <div className="flex gap-1">
                  <input
                    type="text"
                    value={testPhone}
                    onChange={e => setTestPhone(e.target.value)}
                    placeholder="Phone number (e.g. 96170123456)"
                    className="input flex-1"
                  />
                  <button
                    onClick={handleTestSend}
                    disabled={testSending}
                    className="btn btn-primary whitespace-nowrap"
                  >
                    {testSending ? <ArrowPathIcon className="w-3.5 h-3.5 animate-spin" /> : 'Test Send'}
                  </button>
                </div>
                <p className="text-[11px] text-gray-400 mt-0.5">Include country code, no + sign (e.g. 96170123456)</p>
              </div>

              {/* Disconnect */}
              <button onClick={handleUnlink} className="btn btn-danger btn-sm flex items-center gap-1">
                <XCircleIcon className="w-3.5 h-3.5" /> Disconnect WhatsApp
              </button>
            </div>
          ) : linking ? (
            <div className="space-y-3">
              <p className="text-[12px] text-gray-600">
                Scan the QR code below with your WhatsApp app to connect your number.
              </p>
              {qrImageUrl ? (
                <div className="flex flex-col items-center gap-2">
                  <img src={qrImageUrl} alt="WhatsApp QR Code" className="w-48 h-48 border border-[#a0a0a0]" style={{ borderRadius: '2px' }} />
                  <div className="flex items-center gap-1 text-[12px] text-gray-500">
                    <ArrowPathIcon className="w-3.5 h-3.5 animate-spin" />
                    Waiting for scan...
                  </div>
                </div>
              ) : (
                <div className="flex items-center gap-1 text-[12px] text-gray-500"><ArrowPathIcon className="w-3.5 h-3.5 animate-spin" /> Generating QR code...</div>
              )}
              <button onClick={handleCancelLink} className="btn btn-sm">Cancel</button>
            </div>
          ) : (
            <div className="space-y-2">
              {!subCanUse ? (
                <div className="p-2 border border-[#f44336] bg-[#ffe0e0] text-[12px]" style={{ borderRadius: '2px' }}>
                  <p className="font-medium text-[#c62828]">
                    Subscription {subType === 'cancelled' ? 'cancelled' : 'expired'}
                  </p>
                  <p className="text-[11px] text-[#e53935] mt-0.5">
                    Please contact your service provider to activate your WhatsApp subscription.
                  </p>
                </div>
              ) : (
                <>
                  <p className="text-[12px] text-gray-600">
                    Link your WhatsApp number via <strong>ProxRad</strong> (proxsms.com). Scan the QR code to connect your number.
                  </p>
                  <button onClick={handleCreateLink} className="btn btn-primary flex items-center gap-1">
                    <QrCodeIcon className="w-3.5 h-3.5" />
                    Connect WhatsApp
                  </button>
                </>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Send Message (only if connected) */}
      {connected && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
          {/* Subscriber Selector */}
          <div className="wb-group">
            <div className="wb-group-title flex items-center justify-between">
              <span className="flex items-center gap-1">
                <UserGroupIcon className="w-4 h-4 text-[#2196F3]" />
                Select Subscribers
              </span>
              <span className="text-[11px] text-gray-500 flex items-center gap-1">
                {subscribers.length} with phone
                {subscribers.filter(s => s.whatsapp_notifications).length > 0 && (
                  <span className="ml-1 badge badge-success">
                    {subscribers.filter(s => s.whatsapp_notifications).length} notif on
                  </span>
                )}
              </span>
            </div>
            <div className="wb-group-body space-y-2">
              {/* Send All toggle */}
              <label className="flex items-center gap-2 cursor-pointer text-[12px]">
                <input
                  type="checkbox"
                  checked={sendAll}
                  onChange={() => { setSendAll(!sendAll); setSelectedIDs([]) }}
                  className="border-[#a0a0a0]"
                />
                <span className="text-gray-700">
                  Send to all subscribers ({subscribers.length})
                </span>
              </label>

              {!sendAll && (
                <>
                  {/* Search */}
                  <div className="relative">
                    <MagnifyingGlassIcon className="absolute left-1.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-gray-400" />
                    <input
                      type="text"
                      value={subSearch}
                      onChange={e => handleSearchChange(e.target.value)}
                      placeholder="Search subscribers..."
                      className="input pl-6 w-full"
                    />
                  </div>

                  {/* Select/Deselect all visible */}
                  <div className="flex gap-1 flex-wrap items-center text-[11px]">
                    <button onClick={() => setSelectedIDs(subscribers.map(s => s.id))} className="text-[#316AC5] hover:underline">Select all</button>
                    <span className="text-gray-400">|</span>
                    <button onClick={() => setSelectedIDs([])} className="text-gray-500 hover:underline">Clear</button>
                    <span className="text-gray-400">|</span>
                    <button
                      onClick={enableAllNotifications}
                      className="text-[#4CAF50] hover:underline font-medium"
                    >
                      All notif ON
                    </button>
                    <span className="text-gray-400">|</span>
                    <button
                      onClick={disableAllNotifications}
                      className="text-gray-500 hover:underline font-medium"
                    >
                      All notif OFF
                    </button>
                    {selectedIDs.length > 0 && <span className="text-[#4CAF50] font-medium ml-1">{selectedIDs.length} selected</span>}
                  </div>

                  {/* List */}
                  <div className="max-h-64 overflow-y-auto border border-[#a0a0a0]" style={{ borderRadius: '2px' }}>
                    {loadingSubs ? (
                      <div className="text-center py-4 text-[12px] text-gray-400"><ArrowPathIcon className="w-3.5 h-3.5 animate-spin inline" /></div>
                    ) : subscribers.length === 0 ? (
                      <p className="text-center py-4 text-[12px] text-gray-400">No subscribers with phone numbers</p>
                    ) : (
                      subscribers.map(sub => (
                        <label key={sub.id} className={`flex items-center gap-2 px-2 py-1 cursor-pointer hover:bg-[#e8e8f0] ${selectedIDs.includes(sub.id) ? 'bg-[#316AC5] text-white' : ''}`}>
                          <input
                            type="checkbox"
                            checked={selectedIDs.includes(sub.id)}
                            onChange={() => toggleSubscriber(sub.id)}
                            className="border-[#a0a0a0]"
                          />
                          <div className="flex-1 min-w-0">
                            <p className={`text-[12px] font-medium truncate ${selectedIDs.includes(sub.id) ? 'text-white' : 'text-gray-900'}`}>{sub.username}</p>
                            <p className={`text-[11px] truncate ${selectedIDs.includes(sub.id) ? 'text-white/80' : 'text-gray-500'}`}>{sub.phone}</p>
                          </div>
                          <button
                            onClick={(e) => { e.stopPropagation(); toggleNotifications(sub); }}
                            disabled={togglingId === sub.id}
                            title={sub.whatsapp_notifications ? 'Auto-notifications ON -- click to disable' : 'Auto-notifications OFF -- click to enable'}
                            className={`ml-auto shrink-0 p-0.5 transition-colors ${
                              sub.whatsapp_notifications
                                ? 'text-[#4CAF50]'
                                : selectedIDs.includes(sub.id) ? 'text-white/50' : 'text-gray-400'
                            } ${togglingId === sub.id ? 'opacity-50 cursor-wait' : ''}`}
                          >
                            {togglingId === sub.id ? (
                              <svg className="animate-spin h-3.5 w-3.5" fill="none" viewBox="0 0 24 24">
                                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"/>
                                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
                              </svg>
                            ) : sub.whatsapp_notifications ? (
                              <svg className="h-3.5 w-3.5" fill="currentColor" viewBox="0 0 24 24">
                                <path d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"/>
                              </svg>
                            ) : (
                              <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9M3 3l18 18"/>
                              </svg>
                            )}
                          </button>
                        </label>
                      ))
                    )}
                  </div>
                </>
              )}
            </div>
          </div>

          {/* Message Composer */}
          <div className="wb-group">
            <div className="wb-group-title flex items-center gap-1">
              <PaperAirplaneIcon className="w-4 h-4 text-[#4CAF50]" />
              Compose Message
            </div>
            <div className="wb-group-body space-y-2">
              <textarea
                value={message}
                onChange={e => setMessage(e.target.value)}
                rows={8}
                placeholder={`Type your message here...\n\nYou can use:\n{username} -- subscriber username\n{full_name} -- subscriber full name\n{reseller_name} -- your name`}
                className="input w-full resize-none"
              />

              <div className="flex items-center justify-between text-[11px] text-gray-400">
                <span>{message.length} characters</span>
                <span>Recipients: {sendAll ? `All (${subscribers.length})` : selectedIDs.length}</span>
              </div>

              {/* Variable hints */}
              <div className="flex flex-wrap gap-1">
                {['{username}', '{full_name}', '{reseller_name}'].map(v => (
                  <button
                    key={v}
                    onClick={() => setMessage(m => m + v)}
                    className="btn btn-xs text-[11px]"
                  >
                    {v}
                  </button>
                ))}
              </div>

              <button
                onClick={handleSend}
                disabled={sending || !message.trim() || (!sendAll && selectedIDs.length === 0)}
                className="btn btn-primary w-full flex items-center justify-center gap-1"
              >
                {sending ? (
                  <><ArrowPathIcon className="w-3.5 h-3.5 animate-spin" /> Sending...</>
                ) : (
                  <><PaperAirplaneIcon className="w-3.5 h-3.5" /> Send Message</>
                )}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
