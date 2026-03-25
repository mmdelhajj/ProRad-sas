import { useState, useEffect, useRef, lazy, Suspense } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import api, { subscriberApi, serviceApi, nasApi, resellerApi, cdnApi, publicIPApi } from '../services/api'
import { useAuthStore } from '../store/authStore'
import { formatDateTime } from '../utils/timezone'
import toast from 'react-hot-toast'
import clsx from 'clsx'
const ReactECharts = lazy(() => import('echarts-for-react'))
import {
  UserIcon,
  ChartBarIcon,
  DocumentTextIcon,
  ClockIcon,
  ArrowPathIcon,
  EyeIcon,
  EyeSlashIcon,
  CircleStackIcon,
  SignalIcon,
  MapPinIcon,
  GlobeAltIcon,
  XMarkIcon,
  BanknotesIcon,
} from '@heroicons/react/24/outline'
import { InvoiceDetailModal } from './Invoices'
import 'leaflet/dist/leaflet.css'
import L from 'leaflet'

// Fix leaflet default marker icons in Vite/webpack builds
delete L.Icon.Default.prototype._getIconUrl
L.Icon.Default.mergeOptions({
  iconUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon.png',
  iconRetinaUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon-2x.png',
  shadowUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-shadow.png',
})

// Vanilla Leaflet map component (avoids react-leaflet version conflicts)
function LocationMap({ lat, lng, onLocationChange, flyTarget }) {
  const containerRef = useRef(null)
  const mapRef = useRef(null)
  const markerRef = useRef(null)
  const cbRef = useRef(onLocationChange)
  useEffect(() => { cbRef.current = onLocationChange })

  // Initialize map once
  useEffect(() => {
    if (!containerRef.current) return
    const map = L.map(containerRef.current, {
      center: [lat || 33.8938, lng || 35.5018],
      zoom: (lat && lng) ? 15 : 9,
    })
    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      attribution: '© <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
    }).addTo(map)
    if (lat && lng) {
      const marker = L.marker([lat, lng], { draggable: true }).addTo(map)
      marker.on('dragend', (e) => { const p = e.target.getLatLng(); cbRef.current(p.lat, p.lng) })
      markerRef.current = marker
    }
    map.on('click', (e) => cbRef.current(e.latlng.lat, e.latlng.lng))
    mapRef.current = map
    return () => { map.remove(); mapRef.current = null; markerRef.current = null }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  // Update marker when lat/lng changes
  useEffect(() => {
    const map = mapRef.current
    if (!map) return
    if (lat && lng) {
      if (markerRef.current) {
        markerRef.current.setLatLng([lat, lng])
      } else {
        const marker = L.marker([lat, lng], { draggable: true }).addTo(map)
        marker.on('dragend', (e) => { const p = e.target.getLatLng(); cbRef.current(p.lat, p.lng) })
        markerRef.current = marker
      }
    } else if (markerRef.current) {
      markerRef.current.remove()
      markerRef.current = null
    }
  }, [lat, lng])

  // Fly to location on demand
  useEffect(() => {
    if (mapRef.current && flyTarget) mapRef.current.flyTo(flyTarget, 15, { duration: 0.5 })
  }, [flyTarget])

  return <div ref={containerRef} style={{ height: '280px', width: '100%' }} />
}

const tabs = [
  { id: 'info', name: 'Info', icon: UserIcon },
  { id: 'usage', name: 'Usage', icon: CircleStackIcon },
  { id: 'graph', name: 'Live Graph', icon: ChartBarIcon },
  { id: 'invoices', name: 'Invoices', icon: DocumentTextIcon },
  { id: 'logs', name: 'Logs', icon: ClockIcon },
]

export default function SubscriberEdit() {
  const { id } = useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { hasPermission } = useAuthStore()
  const isNew = !id || id === 'new'
  const [showPassword, setShowPassword] = useState(false)

  // Get saved tab from localStorage or default to 'info'
  const getInitialTab = () => {
    if (isNew) return 'info'
    const saved = localStorage.getItem(`subscriber-tab-${id}`)
    const validTabs = ['info', 'usage', 'graph', 'invoices', 'logs']
    return validTabs.includes(saved) ? saved : 'info'
  }

  const [activeTab, setActiveTab] = useState(getInitialTab)
  const [viewInvoiceId, setViewInvoiceId] = useState(null)

  // Save tab to localStorage when it changes
  const handleTabChange = (tabId) => {
    setActiveTab(tabId)
    if (!isNew) {
      localStorage.setItem(`subscriber-tab-${id}`, tabId)
    }
  }

  // Live bandwidth state
  const [currentBandwidth, setCurrentBandwidth] = useState({
    download: 0,
    upload: 0,
    uptime: '',
    ipAddress: '',
    cdnTraffic: [], // Array of { cdn_id, cdn_name, bytes, color }
    portRuleTraffic: [], // Array of { rule_id, rule_name, bytes, color }
  })
  const chartRef = useRef(null) // Reference to ECharts instance
  const downloadDataRef = useRef(Array(30).fill(0))
  const uploadDataRef = useRef(Array(30).fill(0))
  const cdnDataRefs = useRef({}) // { cdn_id: [30 data points] }
  const cdnPrevBytesRef = useRef({}) // { cdn_id: previous_bytes } - for calculating delta
  const [cdnList, setCdnList] = useState([]) // List of CDNs with their colors
  const portRuleDataRefs = useRef({}) // { rule_id: [30 data points] }
  const portRulePrevBytesRef = useRef({}) // { rule_id: previous_bytes }
  const [portRuleList, setPortRuleList] = useState([]) // List of port rules with their colors
  const [livePing, setLivePing] = useState({ ms: 0, ok: false }) // Live ping to subscriber
  const pingDataRef = useRef(Array(30).fill(null)) // RTT history (null = timeout)

  const [formData, setFormData] = useState({
    username: '',
    password: '',
    full_name: '',
    email: '',
    phone: '',
    address: '',
    region: '',
    building: '',
    nationality: '',
    country: '',
    service_id: '',
    nas_id: '',
    reseller_id: '',
    status: 1,
    auto_renew: false,
    auto_invoice: false,
    mac_address: '',
    save_mac: true,
    static_ip: '',
    simultaneous_sessions: 1,
    expiry_date: '',
    note: '',
    price: '',
    override_price: false,
    latitude: 0,
    longitude: 0,
  })

  const { data: subscriberResponse, isLoading } = useQuery({
    queryKey: ['subscriber', id],
    queryFn: () => subscriberApi.get(id).then((r) => r.data),
    enabled: !isNew,
  })

  // Extract subscriber data, quota info, and sessions
  const subscriber = subscriberResponse?.data
  const subscriberPassword = subscriberResponse?.password || ''
  const dailyQuota = subscriberResponse?.daily_quota
  const monthlyQuota = subscriberResponse?.monthly_quota
  const sessions = subscriberResponse?.sessions || []


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

  // Bandwidth rules
  const { data: bandwidthRulesResponse, refetch: refetchBandwidthRules } = useQuery({
    queryKey: ['bandwidth-rules', id],
    queryFn: () => subscriberApi.getBandwidthRules(id).then((r) => r.data),
    enabled: !isNew && !!id,
  })
  const bandwidthRules = bandwidthRulesResponse?.data || []

  // Wallet / Add Balance
  const [showAddBalanceModal, setShowAddBalanceModal] = useState(false)
  const [addBalanceAmount, setAddBalanceAmount] = useState('')
  const [addBalanceReason, setAddBalanceReason] = useState('')

  const addBalanceMutation = useMutation({
    mutationFn: (data) => subscriberApi.addBalance(id, data),
    onSuccess: (res) => {
      const data = res.data?.data
      toast.success(data ? `Added $${data.amount.toFixed(2)} to wallet` : 'Balance added')
      setShowAddBalanceModal(false)
      setAddBalanceAmount('')
      setAddBalanceReason('')
      queryClient.invalidateQueries(['subscriber', id])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to add balance'),
  })

  // Public IP assignment
  const [showAssignPublicIPModal, setShowAssignPublicIPModal] = useState(false)
  const [publicIPPoolId, setPublicIPPoolId] = useState('')
  const [publicIPAddress, setPublicIPAddress] = useState('')
  const [publicIPNotes, setPublicIPNotes] = useState('')

  const { data: publicIPResponse, refetch: refetchPublicIP } = useQuery({
    queryKey: ['subscriber-public-ip', id],
    queryFn: () => publicIPApi.getSubscriberPublicIP(id).then(r => r.data),
    enabled: !isNew && !!id,
  })
  const subscriberPublicIP = publicIPResponse?.assignment

  const { data: publicIPPoolsResponse } = useQuery({
    queryKey: ['public-ip-pools'],
    queryFn: () => publicIPApi.listPools().then(r => r.data),
    enabled: showAssignPublicIPModal,
  })
  const publicIPPools = publicIPPoolsResponse?.data || []

  const assignPublicIPMutation = useMutation({
    mutationFn: (data) => publicIPApi.assignIP(data),
    onSuccess: (res) => {
      toast.success(res.data.message || 'Public IP assigned')
      refetchPublicIP()
      setShowAssignPublicIPModal(false)
      setPublicIPPoolId('')
      setPublicIPAddress('')
      setPublicIPNotes('')
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to assign public IP'),
  })

  const releasePublicIPMutation = useMutation({
    mutationFn: (assignmentId) => publicIPApi.releaseIP(assignmentId),
    onSuccess: (res) => {
      toast.success(res.data.message || 'Public IP released')
      refetchPublicIP()
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to release public IP'),
  })

  // Invoices for this subscriber
  const { data: invoicesResponse, refetch: refetchInvoices } = useQuery({
    queryKey: ['subscriber-invoices', id],
    queryFn: () => api.get(`/invoices?subscriber_id=${id}&limit=50`).then((r) => r.data),
    enabled: !isNew && !!id && activeTab === 'invoices',
  })
  const subscriberInvoices = invoicesResponse?.data || []

  // CDN upgrades
  const { data: cdnUpgradesResponse } = useQuery({
    queryKey: ['cdn-upgrades', id],
    queryFn: () => subscriberApi.getCDNUpgrades(id).then((r) => r.data),
    enabled: !isNew && !!id,
  })
  const cdnUpgrades = cdnUpgradesResponse?.available_upgrades || []
  const currentCDNs = cdnUpgradesResponse?.current_cdns || []

  // All CDNs with their available speeds (for bandwidth rules)
  const { data: cdnSpeedsResponse } = useQuery({
    queryKey: ['cdn-speeds'],
    queryFn: () => cdnApi.getSpeeds().then((r) => r.data),
  })
  const cdnSpeedsData = cdnSpeedsResponse?.data || []

  // Filter CDNs by subscriber's NAS - only show CDNs that sync to this NAS
  const filteredCDNsForNAS = cdnSpeedsData.filter(cdn => {
    // If subscriber has no NAS selected, show all CDNs
    if (!formData.nas_id) return true
    // If CDN has no NAS restriction (empty nas_ids), it syncs to all NAS
    if (!cdn.nas_ids || cdn.nas_ids === '') return true
    // Check if subscriber's NAS ID is in the CDN's nas_ids list
    const nasIdsList = cdn.nas_ids.split(',').map(id => id.trim())
    return nasIdsList.includes(String(formData.nas_id))
  })

  // Get speeds for the selected CDN
  const getSpeedsForCDN = (cdnId) => {
    const cdn = cdnSpeedsData.find(c => c.cdn_id === parseInt(cdnId))
    return cdn?.speeds || []
  }

  // Bandwidth rule modal state
  const [mapFlyTarget, setMapFlyTarget] = useState(null)
  const [gettingLocation, setGettingLocation] = useState(false)
  const [showBandwidthRuleModal, setShowBandwidthRuleModal] = useState(false)
  const [editingBandwidthRule, setEditingBandwidthRule] = useState(null)
  const [bandwidthRuleForm, setBandwidthRuleForm] = useState({
    rule_type: 'internet',
    enabled: true,
    download_speed: '',
    upload_speed: '',
    duration: 'permanent',
    priority: 0,
    cdn_id: '',
    cdn_speed: '',
  })

  const resetBandwidthRuleForm = () => {
    setBandwidthRuleForm({
      rule_type: 'internet',
      enabled: true,
      download_speed: '',
      upload_speed: '',
      duration: 'permanent',
      priority: 0,
      cdn_id: '',
      cdn_speed: '',
    })
    setEditingBandwidthRule(null)
  }

  const openBandwidthRuleModal = (rule = null) => {
    if (rule) {
      setEditingBandwidthRule(rule)
      // For CDN rules, convert kbps back to Mbps for display
      const isCDN = rule.rule_type === 'cdn'
      setBandwidthRuleForm({
        rule_type: rule.rule_type || 'internet',
        enabled: rule.enabled ?? true,
        download_speed: isCDN ? Math.round((rule.download_speed || 0) / 1000) : (rule.download_speed || ''),
        upload_speed: isCDN ? Math.round((rule.upload_speed || 0) / 1000) : (rule.upload_speed || ''),
        duration: rule.duration || 'permanent',
        priority: rule.priority || 0,
        cdn_id: rule.cdn_id || '',
        cdn_speed: rule.cdn_speed || '',
      })
    } else {
      resetBandwidthRuleForm()
    }
    setShowBandwidthRuleModal(true)
  }

  // Handle CDN upgrade selection
  const handleCDNUpgradeSelect = (value) => {
    if (!value) {
      setBandwidthRuleForm({ ...bandwidthRuleForm, cdn_id: '', cdn_speed: '', download_speed: '', upload_speed: '' })
      return
    }
    const [cdnId, speed] = value.split('-')
    const speedKbps = parseInt(speed) * 1000 // Convert Mbps to kbps
    setBandwidthRuleForm({
      ...bandwidthRuleForm,
      cdn_id: cdnId,
      cdn_speed: speed,
      download_speed: speedKbps,
      upload_speed: speedKbps,
    })
  }

  // Helper function to format time remaining for bandwidth rules
  const formatTimeRemaining = (rule) => {
    if (!rule.expires_at) return 'Permanent'
    const expiresAt = new Date(rule.expires_at)
    const now = new Date()
    if (expiresAt <= now) return 'Expired'
    const diffMs = expiresAt - now
    const diffHours = Math.floor(diffMs / (1000 * 60 * 60))
    const diffMins = Math.floor((diffMs % (1000 * 60 * 60)) / (1000 * 60))
    if (diffHours >= 24) {
      const days = Math.floor(diffHours / 24)
      const hours = diffHours % 24
      return `${days}d ${hours}h remaining`
    }
    return `${diffHours}h ${diffMins}m remaining`
  }

  useEffect(() => {
    if (subscriber) {
      setFormData({
        username: subscriber.username || '',
        password: subscriberPassword || '',
        full_name: subscriber.full_name || '',
        email: subscriber.email || '',
        phone: subscriber.phone || '',
        address: subscriber.address || '',
        region: subscriber.region || '',
        building: subscriber.building || '',
        nationality: subscriber.nationality || '',
        country: subscriber.country || '',
        service_id: subscriber.service_id || '',
        nas_id: subscriber.nas_id || '',
        reseller_id: subscriber.reseller_id || '',
        status: subscriber.status ?? 1,
        auto_renew: subscriber.auto_renew ?? false,
        auto_invoice: subscriber.auto_invoice ?? false,
        mac_address: subscriber.mac_address || '',
        save_mac: subscriber.save_mac ?? false,
        static_ip: subscriber.static_ip || '',
        simultaneous_sessions: subscriber.simultaneous_sessions || 1,
        expiry_date: subscriber.expiry_date ? subscriber.expiry_date.split('T')[0] : '',
        note: subscriber.note || '',
        price: subscriber.price || '',
        override_price: subscriber.override_price || false,
        latitude: subscriber.latitude || 0,
        longitude: subscriber.longitude || 0,
      })

    }
  }, [subscriber, subscriberPassword, id, isNew])

  // Start/stop polling when graph tab is active
  useEffect(() => {
    // Only poll when on graph tab, subscriber is online, and not creating new
    if (activeTab !== 'graph' || !subscriber?.is_online || isNew) {
      return
    }

    let isMounted = true

    const fetchBandwidthData = async () => {
      try {
        const response = await subscriberApi.getBandwidth(id)

        if (response.data.success && isMounted) {
          const data = response.data.data
          const downloadValue = data.download || 0
          const uploadValue = data.upload || 0
          const cdnTraffic = data.cdn_traffic || []
          const cdnIsRate = data.cdn_is_rate || false // true = Torch (bytes/sec), false = connection tracking (cumulative)
          const portRuleTraffic = data.port_rule_traffic || []

          // Update CDN data arrays first to calculate total CDN rate
          const updatedCdnList = []
          let totalCdnMbps = 0

          cdnTraffic.forEach(cdn => {
            if (!cdnDataRefs.current[cdn.cdn_id]) {
              cdnDataRefs.current[cdn.cdn_id] = Array(30).fill(0)
            }

            const currentBytes = cdn.bytes || 0
            let cdnMbps = 0

            if (cdnIsRate) {
              cdnMbps = (currentBytes * 8 / 1000000) || 0
            } else {
              const prevBytes = cdnPrevBytesRef.current[cdn.cdn_id]
              if (prevBytes !== undefined && currentBytes >= prevBytes) {
                const deltaBytes = currentBytes - prevBytes
                cdnMbps = (deltaBytes * 8 / 1000000 / 2) || 0
              }
              cdnPrevBytesRef.current[cdn.cdn_id] = currentBytes
            }

            cdnDataRefs.current[cdn.cdn_id] = [...cdnDataRefs.current[cdn.cdn_id].slice(1), cdnMbps]
            totalCdnMbps += cdnMbps
            updatedCdnList.push({ id: cdn.cdn_id, name: cdn.cdn_name, color: cdn.color })
          })
          setCdnList(updatedCdnList)

          // Update Port Rule data arrays
          const updatedPortRuleList = []
          portRuleTraffic.forEach(pr => {
            if (!portRuleDataRefs.current[pr.rule_id]) {
              portRuleDataRefs.current[pr.rule_id] = Array(30).fill(0)
            }
            const currentBytes = pr.bytes || 0
            // Port rules always use Torch (bytes/sec rate)
            const prMbps = (currentBytes * 8 / 1000000) || 0
            portRuleDataRefs.current[pr.rule_id] = [...portRuleDataRefs.current[pr.rule_id].slice(1), prMbps]
            updatedPortRuleList.push({ id: pr.rule_id, name: pr.rule_name, color: pr.color })
          })
          setPortRuleList(updatedPortRuleList)

          // Subtract CDN traffic from download to show only regular internet
          const regularDownload = Math.max(0, downloadValue - totalCdnMbps)

          setCurrentBandwidth({
            download: regularDownload.toFixed(2),
            upload: uploadValue.toFixed(2),
            uptime: data.uptime || '',
            ipAddress: data.ip_address || '',
            cdnTraffic: cdnTraffic,
            portRuleTraffic: portRuleTraffic,
          })

          // Update live ping
          if (data.ping_ok) {
            setLivePing({ ms: data.ping_ms, ok: true })
            pingDataRef.current = [...pingDataRef.current.slice(1), data.ping_ms]
          } else {
            setLivePing(prev => ({ ...prev, ok: false }))
            pingDataRef.current = [...pingDataRef.current.slice(1), null] // null = gap in line
          }

          // Update data arrays
          downloadDataRef.current = [...downloadDataRef.current.slice(1), regularDownload]
          uploadDataRef.current = [...uploadDataRef.current.slice(1), uploadValue]

          // Directly update the chart instance for smooth animation
          if (chartRef.current) {
            const chartInstance = chartRef.current.getEchartsInstance()
            if (chartInstance) {
              const legendData = ['Download', 'Upload', 'Ping (ms)', ...updatedCdnList.map(c => c.name), ...updatedPortRuleList.map(pr => pr.name)]
              const seriesConfig = [
                { name: 'Download', data: downloadDataRef.current },
                { name: 'Upload', data: uploadDataRef.current },
                {
                  name: 'Ping (ms)',
                  type: 'line',
                  yAxisIndex: 1,
                  smooth: false,
                  data: pingDataRef.current,
                  lineStyle: { color: '#F59E0B', width: 1.5, type: 'dotted' },
                  itemStyle: { color: '#F59E0B' },
                  showSymbol: false,
                  connectNulls: false,
                  z: 10,
                },
                ...updatedCdnList.map(cdn => ({
                  name: cdn.name,
                  type: 'line',
                  smooth: true,
                  data: cdnDataRefs.current[cdn.id] || Array(30).fill(0),
                  lineStyle: { color: cdn.color, width: 2 },
                  itemStyle: { color: cdn.color },
                  areaStyle: {
                    color: {
                      type: 'linear',
                      x: 0, y: 0, x2: 0, y2: 1,
                      colorStops: [
                        { offset: 0, color: cdn.color + '66' },
                        { offset: 1, color: cdn.color + '0D' }
                      ]
                    }
                  },
                  showSymbol: false,
                })),
                ...updatedPortRuleList.map(pr => ({
                  name: pr.name,
                  type: 'line',
                  smooth: true,
                  data: portRuleDataRefs.current[pr.id] || Array(30).fill(0),
                  lineStyle: { color: pr.color, width: 2, type: 'dashed' },
                  itemStyle: { color: pr.color },
                  areaStyle: {
                    color: {
                      type: 'linear',
                      x: 0, y: 0, x2: 0, y2: 1,
                      colorStops: [
                        { offset: 0, color: pr.color + '55' },
                        { offset: 1, color: pr.color + '0D' }
                      ]
                    }
                  },
                  showSymbol: false,
                }))
              ]

              chartInstance.setOption({
                legend: { data: legendData },
                series: seriesConfig
              })
            }
          }
        }
      } catch (error) {
        console.error('Failed to fetch bandwidth:', error)
      }
    }

    // Initial fetch
    fetchBandwidthData()

    // Poll every 2 seconds
    const intervalId = setInterval(fetchBandwidthData, 2000)

    return () => {
      isMounted = false
      clearInterval(intervalId)
    }
  }, [activeTab, subscriber?.is_online, isNew, id])

  const saveMutation = useMutation({
    mutationFn: (data) =>
      isNew ? subscriberApi.create(data) : subscriberApi.update(id, data),
    onSuccess: (res) => {
      toast.success(isNew ? 'Subscriber created' : 'Subscriber updated')
      queryClient.invalidateQueries(['subscribers'])
      if (isNew) {
        navigate(`/subscribers/${res.data.data?.id || res.data.id}`)
      } else {
        queryClient.invalidateQueries(['subscriber', id])
      }
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to save'),
  })

  // Bandwidth rule mutations
  const saveBandwidthRuleMutation = useMutation({
    mutationFn: (data) =>
      editingBandwidthRule
        ? subscriberApi.updateBandwidthRule(id, editingBandwidthRule.id, data)
        : subscriberApi.createBandwidthRule(id, data),
    onSuccess: () => {
      toast.success(editingBandwidthRule ? 'Rule updated' : 'Rule created')
      refetchBandwidthRules()
      setShowBandwidthRuleModal(false)
      resetBandwidthRuleForm()
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to save rule'),
  })

  const deleteBandwidthRuleMutation = useMutation({
    mutationFn: (ruleId) => subscriberApi.deleteBandwidthRule(id, ruleId),
    onSuccess: () => {
      toast.success('Rule deleted')
      refetchBandwidthRules()
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to delete rule'),
  })

  const handleSaveBandwidthRule = () => {
    // For CDN rules, convert Mbps to kbps (user enters 30, we send 30000)
    const isCDN = bandwidthRuleForm.rule_type === 'cdn'
    const downloadSpeed = parseInt(bandwidthRuleForm.download_speed) || 0
    const uploadSpeed = parseInt(bandwidthRuleForm.upload_speed) || 0

    const data = {
      ...bandwidthRuleForm,
      download_speed: isCDN ? downloadSpeed * 1000 : downloadSpeed,
      upload_speed: isCDN ? uploadSpeed * 1000 : uploadSpeed,
      priority: parseInt(bandwidthRuleForm.priority) || 0,
      cdn_id: bandwidthRuleForm.cdn_id ? parseInt(bandwidthRuleForm.cdn_id) : 0,
    }
    saveBandwidthRuleMutation.mutate(data)
  }

  const handleDeleteBandwidthRule = (ruleId) => {
    if (confirm('Are you sure you want to delete this rule?')) {
      deleteBandwidthRuleMutation.mutate(ruleId)
    }
  }

  const handleSubmit = (e) => {
    e.preventDefault()
    const data = { ...formData }
    if (!data.password) delete data.password
    if (data.override_price && data.price !== '') {
      data.price = parseFloat(data.price)
    } else {
      data.price = 0
      data.override_price = false
    }
    if (data.service_id) data.service_id = parseInt(data.service_id)
    if (data.nas_id) data.nas_id = parseInt(data.nas_id)
    else delete data.nas_id
    if (data.reseller_id) data.reseller_id = parseInt(data.reseller_id)
    else delete data.reseller_id
    data.status = parseInt(data.status)
    data.simultaneous_sessions = parseInt(data.simultaneous_sessions) || 1
    saveMutation.mutate(data)
  }

  const handleChange = (e) => {
    const { name, value, type, checked } = e.target
    setFormData((prev) => ({
      ...prev,
      [name]: type === 'checkbox' ? checked : value,
    }))
  }

  // Generate time labels (30s ago to now)
  const timeLabels = Array.from({ length: 30 }, (_, i) => `${30 - i}s`)

  const bandwidthChartOption = {
    animation: true,
    animationDuration: 300,
    animationEasing: 'linear',
    tooltip: {
      trigger: 'axis',
      formatter: (params) => {
        let result = params[0].name + '<br/>'
        params.forEach((param) => {
          if (param.value === null || param.value === undefined) return
          const isPing = param.seriesName === 'Ping (ms)'
          const val = typeof param.value === 'number' ? param.value.toFixed(isPing ? 1 : 2) : '—'
          const unit = isPing ? 'ms' : 'Mbps'
          result += `${param.marker} ${param.seriesName}: ${val} ${unit}<br/>`
        })
        return result
      },
    },
    legend: {
      data: ['Download', 'Upload', 'Ping (ms)'],
      top: 0,
    },
    grid: {
      left: '3%',
      right: '8%',
      bottom: '3%',
      top: '40px',
      containLabel: true,
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: timeLabels,
      axisLine: { lineStyle: { color: '#ccc' } },
    },
    yAxis: [
      {
        type: 'value',
        min: 0,
        max: (value) => {
          const maxVal = Math.max(value.max, 0.1)
          return Math.max(3, Math.ceil(maxVal * 1.2))
        },
        axisLabel: { formatter: '{value} Mbps' },
        axisLine: { lineStyle: { color: '#ccc' } },
        splitLine: { lineStyle: { color: '#eee' } },
      },
      {
        type: 'value',
        name: 'ms',
        min: 0,
        max: (value) => Math.max(100, Math.ceil(value.max * 1.3)),
        position: 'right',
        axisLabel: { formatter: '{value} ms', color: '#F59E0B' },
        axisLine: { lineStyle: { color: '#F59E0B33' } },
        splitLine: { show: false },
      },
    ],
    series: [
      {
        name: 'Download',
        type: 'line',
        yAxisIndex: 0,
        smooth: true,
        data: downloadDataRef.current,
        lineStyle: { color: '#10B981', width: 2 },
        itemStyle: { color: '#10B981' },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(16, 185, 129, 0.4)' },
              { offset: 1, color: 'rgba(16, 185, 129, 0.05)' }
            ]
          }
        },
        showSymbol: false,
      },
      {
        name: 'Upload',
        type: 'line',
        yAxisIndex: 0,
        smooth: true,
        data: uploadDataRef.current,
        lineStyle: { color: '#3B82F6', width: 2 },
        itemStyle: { color: '#3B82F6' },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(59, 130, 246, 0.4)' },
              { offset: 1, color: 'rgba(59, 130, 246, 0.05)' }
            ]
          }
        },
        showSymbol: false,
      },
      {
        name: 'Ping (ms)',
        type: 'line',
        yAxisIndex: 1,
        smooth: false,
        data: pingDataRef.current,
        lineStyle: { color: '#F59E0B', width: 1.5, type: 'dotted' },
        itemStyle: { color: '#F59E0B' },
        showSymbol: false,
        connectNulls: false,
        z: 10,
      },
    ],
  }

  if (!isNew && isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin h-8 w-8 border-b-2 border-[#4a6984]"></div>
      </div>
    )
  }

  return (
    <div className="space-y-3" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-[13px] font-semibold text-gray-900 dark:text-white">
            {isNew ? 'Add Subscriber' : `Edit: ${subscriber?.username}`}
          </h1>
          <p className="text-[12px] text-gray-500 dark:text-gray-400">
            {isNew ? 'Create a new PPPoE subscriber' : 'Manage subscriber details'}
          </p>
        </div>
        <div className="flex items-center gap-3">
          {!isNew && subscriber?.is_online && (
            <span className="badge badge-success">Online</span>
          )}
          {!isNew && !subscriber?.is_online && (
            <span className="badge badge-gray">Offline</span>
          )}
          {!isNew && subscriber?.fup_level > 0 && (
            <span className="px-2 py-0.5 text-[11px] font-bold bg-red-500 text-white animate-pulse">
              FUP {subscriber.fup_level === 1 ? '(Daily)' : '(Monthly)'}
            </span>
          )}
        </div>
      </div>

      {/* Tabs */}
      {!isNew && (
        <div className="flex border-b border-[#a0a0a0] bg-[#f0f0f0] dark:bg-[#444] dark:border-[#555]">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => handleTabChange(tab.id)}
              className={clsx(
                activeTab === tab.id
                  ? 'px-3 py-1.5 text-[12px] border border-[#a0a0a0] border-b-0 bg-white font-semibold -mb-px dark:bg-[#333] dark:border-[#555]'
                  : 'px-3 py-1.5 text-[12px] bg-[#e8e8e8] text-[#666] dark:bg-[#444] dark:text-[#aaa]'
              )}
            >
              <span className="flex items-center gap-1.5">
                <tab.icon className="w-3.5 h-3.5" />
                {tab.name}
              </span>
            </button>
          ))}
        </div>
      )}

      {/* Info Tab */}
      {(isNew || activeTab === 'info') && (
        <form onSubmit={handleSubmit} className="space-y-3">
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
            {/* Account Info */}
            <div className="card p-3">
              <h3 className="text-[12px] font-semibold text-gray-900 dark:text-white mb-3 pb-1 border-b border-[#ccc] dark:border-[#555]">Account Information</h3>
              <div className="space-y-2">
                <div>
                  <label className="label">Username (PPPoE) {!isNew && <span className="text-[11px] text-gray-400 ml-1">(locked)</span>}</label>
                  <input
                    type="text"
                    name="username"
                    value={formData.username}
                    onChange={handleChange}
                    className={`input ${!isNew ? 'bg-gray-100 cursor-not-allowed' : ''}`}
                    required
                    autoComplete="off"
                    disabled={!isNew}
                  />
                </div>
                <div>
                  <label className="label">Password</label>
                  <div className="flex gap-2">
                    <div className="relative flex-1">
                      <input
                        type={showPassword ? 'text' : 'password'}
                        name="password"
                        value={formData.password}
                        onChange={handleChange}
                        className="input pr-10"
                        placeholder={isNew ? '' : 'Leave blank to keep current'}
                        required={isNew}
                        autoComplete="new-password"
                      />
                      <button
                        type="button"
                        onClick={() => setShowPassword(!showPassword)}
                        className="absolute inset-y-0 right-0 flex items-center pr-3 text-gray-400 hover:text-gray-600 dark:text-gray-400 dark:text-gray-500 dark:text-gray-400"
                      >
                        {showPassword ? (
                          <EyeSlashIcon className="h-5 w-5" />
                        ) : (
                          <EyeIcon className="h-5 w-5" />
                        )}
                      </button>
                    </div>
                    <button
                      type="button"
                      onClick={() => {
                        const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789'
                        const length = 8 + Math.floor(Math.random() * 3)
                        let password = ''
                        for (let i = 0; i < length; i++) {
                          password += chars.charAt(Math.floor(Math.random() * chars.length))
                        }
                        setFormData(prev => ({ ...prev, password }))
                        setShowPassword(true)
                      }}
                      className="btn btn-secondary whitespace-nowrap"
                    >
                      Generate
                    </button>
                  </div>
                </div>
                {(isNew || hasPermission('subscribers.change_service')) ? (
                  <div>
                    <label className="label">Service Plan</label>
                    <select
                      name="service_id"
                      value={formData.service_id}
                      onChange={handleChange}
                      className="input"
                      required
                    >
                      <option value="">Select Service</option>
                      {services?.map((s) => (
                        <option key={s.id} value={s.id}>
                          {s.name} - ${s.price}
                        </option>
                      ))}
                    </select>
                  </div>
                ) : (
                  <div>
                    <label className="label">Service Plan</label>
                    <input
                      type="text"
                      value={services?.find(s => s.id === parseInt(formData.service_id))?.name || 'N/A'}
                      className="input bg-gray-100 dark:bg-gray-700"
                      disabled
                    />
                  </div>
                )}
                {/* Override Price */}
                <div>
                  <label className="label">Price</label>
                  <div className="flex items-center gap-3 mb-2">
                    <input
                      type="checkbox"
                      id="override_price"
                      name="override_price"
                      checked={formData.override_price}
                      onChange={handleChange}
                      className="w-4 h-4 border-gray-300 text-primary-600 focus:ring-primary-500"
                    />
                    <label htmlFor="override_price" className="text-[12px] text-gray-700 dark:text-gray-300 cursor-pointer">
                      Override service price for this subscriber
                    </label>
                  </div>
                  {formData.override_price ? (
                    <input
                      type="number"
                      name="price"
                      value={formData.price}
                      onChange={handleChange}
                      placeholder="Enter custom price"
                      step="0.01"
                      min="0"
                      className="input"
                    />
                  ) : (
                    <div className="text-[12px] text-gray-500 dark:text-gray-400 px-2 py-1.5 bg-gray-50 dark:bg-gray-700 border border-[#a0a0a0] dark:border-[#555]">
                      Using service default: ${services?.find(s => s.id == formData.service_id)?.price ?? '—'}
                    </div>
                  )}
                </div>
                <div>
                  <label className="label">NAS</label>
                  <select
                    name="nas_id"
                    value={formData.nas_id}
                    onChange={handleChange}
                    className="input"
                  >
                    <option value="">Select NAS</option>
                    {nasList?.map((n) => (
                      <option key={n.id} value={n.id}>
                        {n.name} ({n.ip_address})
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="label">Reseller</label>
                  <select
                    name="reseller_id"
                    value={formData.reseller_id}
                    onChange={handleChange}
                    className="input"
                  >
                    <option value="">No Reseller (Admin)</option>
                    {resellers?.map((r) => (
                      <option key={r.id} value={r.id}>
                        {r.name || r.username} (Balance: ${r.balance})
                      </option>
                    ))}
                  </select>
                </div>
                {!isNew && subscriber?.created_at && (
                  <div>
                    <label className="label">Created At</label>
                    <div className="input bg-gray-50 dark:bg-gray-700 cursor-default text-gray-700 dark:text-gray-200 font-medium">
                      {formatDateTime(subscriber.created_at)}
                    </div>
                  </div>
                )}
              </div>
            </div>

            {/* Personal Info */}
            <div className="card p-3">
              <h3 className="text-[12px] font-semibold text-gray-900 dark:text-white mb-3 pb-1 border-b border-[#ccc] dark:border-[#555]">Personal Information</h3>
              <div className="space-y-2">
                <div>
                  <label className="label">Full Name</label>
                  <input
                    type="text"
                    name="full_name"
                    value={formData.full_name}
                    onChange={handleChange}
                    className="input"
                  />
                </div>
                <div>
                  <label className="label">Email</label>
                  <input
                    type="email"
                    name="email"
                    value={formData.email}
                    onChange={handleChange}
                    className="input"
                  />
                </div>
                <div>
                  <label className="label">Phone</label>
                  <input
                    type="tel"
                    name="phone"
                    value={formData.phone}
                    onChange={handleChange}
                    className="input"
                  />
                </div>
                <div>
                  <label className="label">Address</label>
                  <textarea
                    name="address"
                    value={formData.address}
                    onChange={handleChange}
                    className="input"
                    rows={3}
                  />
                </div>
                <div className="grid grid-cols-2 gap-2">
                  <div>
                    <label className="label">Region</label>
                    <input
                      type="text"
                      name="region"
                      value={formData.region}
                      onChange={handleChange}
                      className="input"
                    />
                  </div>
                  <div>
                    <label className="label">Building</label>
                    <input
                      type="text"
                      name="building"
                      value={formData.building}
                      onChange={handleChange}
                      className="input"
                    />
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-2">
                  <div>
                    <label className="label">Nationality</label>
                    <select
                      name="nationality"
                      value={formData.nationality}
                      onChange={handleChange}
                      className="input"
                    >
                      <option value="">Select Nationality</option>
                      <option value="Afghan">Afghan</option>
                      <option value="Albanian">Albanian</option>
                      <option value="Algerian">Algerian</option>
                      <option value="Argentine">Argentine</option>
                      <option value="Australian">Australian</option>
                      <option value="Austrian">Austrian</option>
                      <option value="Bahraini">Bahraini</option>
                      <option value="Bangladeshi">Bangladeshi</option>
                      <option value="Belgian">Belgian</option>
                      <option value="Brazilian">Brazilian</option>
                      <option value="Canadian">Canadian</option>
                      <option value="Chinese">Chinese</option>
                      <option value="Colombian">Colombian</option>
                      <option value="Czech">Czech</option>
                      <option value="Danish">Danish</option>
                      <option value="Egyptian">Egyptian</option>
                      <option value="Finnish">Finnish</option>
                      <option value="French">French</option>
                      <option value="German">German</option>
                      <option value="Greek">Greek</option>
                      <option value="Hong Konger">Hong Konger</option>
                      <option value="Hungarian">Hungarian</option>
                      <option value="Indian">Indian</option>
                      <option value="Indonesian">Indonesian</option>
                      <option value="Iranian">Iranian</option>
                      <option value="Iraqi">Iraqi</option>
                      <option value="Irish">Irish</option>
                      <option value="Israeli">Israeli</option>
                      <option value="Italian">Italian</option>
                      <option value="Japanese">Japanese</option>
                      <option value="Jordanian">Jordanian</option>
                      <option value="Kuwaiti">Kuwaiti</option>
                      <option value="Lebanese">Lebanese</option>
                      <option value="Libyan">Libyan</option>
                      <option value="Malaysian">Malaysian</option>
                      <option value="Mexican">Mexican</option>
                      <option value="Moroccan">Moroccan</option>
                      <option value="Dutch">Dutch</option>
                      <option value="New Zealander">New Zealander</option>
                      <option value="Nigerian">Nigerian</option>
                      <option value="Norwegian">Norwegian</option>
                      <option value="Omani">Omani</option>
                      <option value="Pakistani">Pakistani</option>
                      <option value="Palestinian">Palestinian</option>
                      <option value="Filipino">Filipino</option>
                      <option value="Polish">Polish</option>
                      <option value="Portuguese">Portuguese</option>
                      <option value="Qatari">Qatari</option>
                      <option value="Romanian">Romanian</option>
                      <option value="Russian">Russian</option>
                      <option value="Saudi">Saudi</option>
                      <option value="Singaporean">Singaporean</option>
                      <option value="South African">South African</option>
                      <option value="South Korean">South Korean</option>
                      <option value="Spanish">Spanish</option>
                      <option value="Sudanese">Sudanese</option>
                      <option value="Swedish">Swedish</option>
                      <option value="Swiss">Swiss</option>
                      <option value="Syrian">Syrian</option>
                      <option value="Taiwanese">Taiwanese</option>
                      <option value="Thai">Thai</option>
                      <option value="Tunisian">Tunisian</option>
                      <option value="Turkish">Turkish</option>
                      <option value="Ukrainian">Ukrainian</option>
                      <option value="Emirati">Emirati</option>
                      <option value="British">British</option>
                      <option value="American">American</option>
                      <option value="Vietnamese">Vietnamese</option>
                      <option value="Yemeni">Yemeni</option>
                    </select>
                  </div>
                  <div>
                    <label className="label">Country</label>
                    <select
                      name="country"
                      value={formData.country}
                      onChange={handleChange}
                      className="input"
                    >
                      <option value="">Select Country</option>
                      <option value="Afghanistan">Afghanistan</option>
                      <option value="Albania">Albania</option>
                      <option value="Algeria">Algeria</option>
                      <option value="Argentina">Argentina</option>
                      <option value="Australia">Australia</option>
                      <option value="Austria">Austria</option>
                      <option value="Bahrain">Bahrain</option>
                      <option value="Bangladesh">Bangladesh</option>
                      <option value="Belgium">Belgium</option>
                      <option value="Brazil">Brazil</option>
                      <option value="Canada">Canada</option>
                      <option value="China">China</option>
                      <option value="Colombia">Colombia</option>
                      <option value="Czech Republic">Czech Republic</option>
                      <option value="Denmark">Denmark</option>
                      <option value="Egypt">Egypt</option>
                      <option value="Finland">Finland</option>
                      <option value="France">France</option>
                      <option value="Germany">Germany</option>
                      <option value="Greece">Greece</option>
                      <option value="Hong Kong">Hong Kong</option>
                      <option value="Hungary">Hungary</option>
                      <option value="India">India</option>
                      <option value="Indonesia">Indonesia</option>
                      <option value="Iran">Iran</option>
                      <option value="Iraq">Iraq</option>
                      <option value="Ireland">Ireland</option>
                      <option value="Israel">Israel</option>
                      <option value="Italy">Italy</option>
                      <option value="Japan">Japan</option>
                      <option value="Jordan">Jordan</option>
                      <option value="Kuwait">Kuwait</option>
                      <option value="Lebanon">Lebanon</option>
                      <option value="Libya">Libya</option>
                      <option value="Malaysia">Malaysia</option>
                      <option value="Mexico">Mexico</option>
                      <option value="Morocco">Morocco</option>
                      <option value="Netherlands">Netherlands</option>
                      <option value="New Zealand">New Zealand</option>
                      <option value="Nigeria">Nigeria</option>
                      <option value="Norway">Norway</option>
                      <option value="Oman">Oman</option>
                      <option value="Pakistan">Pakistan</option>
                      <option value="Palestine">Palestine</option>
                      <option value="Philippines">Philippines</option>
                      <option value="Poland">Poland</option>
                      <option value="Portugal">Portugal</option>
                      <option value="Qatar">Qatar</option>
                      <option value="Romania">Romania</option>
                      <option value="Russia">Russia</option>
                      <option value="Saudi Arabia">Saudi Arabia</option>
                      <option value="Singapore">Singapore</option>
                      <option value="South Africa">South Africa</option>
                      <option value="South Korea">South Korea</option>
                      <option value="Spain">Spain</option>
                      <option value="Sudan">Sudan</option>
                      <option value="Sweden">Sweden</option>
                      <option value="Switzerland">Switzerland</option>
                      <option value="Syria">Syria</option>
                      <option value="Taiwan">Taiwan</option>
                      <option value="Thailand">Thailand</option>
                      <option value="Tunisia">Tunisia</option>
                      <option value="Turkey">Turkey</option>
                      <option value="Ukraine">Ukraine</option>
                      <option value="United Arab Emirates">United Arab Emirates</option>
                      <option value="United Kingdom">United Kingdom</option>
                      <option value="United States">United States</option>
                      <option value="Vietnam">Vietnam</option>
                      <option value="Yemen">Yemen</option>
                    </select>
                  </div>
                </div>
              </div>
            </div>

            {/* Connection Settings */}
            <div className="card p-3">
              <h3 className="text-[12px] font-semibold text-gray-900 dark:text-white mb-3 pb-1 border-b border-[#ccc] dark:border-[#555]">Connection Settings</h3>
              <div className="space-y-2">
                <div>
                  <label className="label">MAC Address {!isNew && <span className="text-[11px] text-gray-400 ml-1">(locked)</span>}</label>
                  <input
                    type="text"
                    name="mac_address"
                    value={formData.mac_address}
                    onChange={(e) => {
                      const val = e.target.value.toUpperCase()
                      if (val === '' || /^[0-9A-F:-]*$/.test(val)) {
                        setFormData(prev => ({ ...prev, mac_address: val }))
                      }
                    }}
                    className={`input ${!isNew ? 'bg-gray-100 cursor-not-allowed' : ''}`}
                    placeholder="Leave empty - auto-saves on first connect"
                    disabled={!isNew}
                  />
                  <p className="text-[11px] text-gray-500 dark:text-gray-400 mt-1">Leave empty to auto-capture MAC on first connection</p>
                </div>
                <label className="flex items-center gap-3">
                  <input
                    type="checkbox"
                    name="save_mac"
                    checked={formData.save_mac}
                    onChange={handleChange}
                    className="border-gray-300 text-primary-600 focus:ring-primary-500"
                  />
                  <div>
                    <span className="font-medium">Save MAC</span>
                    <p className="text-[12px] text-gray-500 dark:text-gray-400">Lock to current MAC address (reject other devices)</p>
                  </div>
                </label>
                <div>
                  <label className="label">Static IP</label>
                  <input
                    type="text"
                    name="static_ip"
                    value={formData.static_ip}
                    onChange={handleChange}
                    className="input"
                    placeholder="Leave blank for dynamic"
                  />
                </div>
                <div>
                  <label className="label">Simultaneous Sessions</label>
                  <input
                    type="number"
                    name="simultaneous_sessions"
                    value={formData.simultaneous_sessions}
                    onChange={handleChange}
                    className="input"
                    min={1}
                    max={10}
                  />
                </div>
                <div>
                  <label className="label">Expiry Date</label>
                  <input
                    type="date"
                    name="expiry_date"
                    value={formData.expiry_date}
                    onChange={handleChange}
                    className="input"
                  />
                </div>
              </div>
            </div>

            {/* Bandwidth Rules - Only show for existing subscribers with permission */}
            {!isNew && hasPermission('subscribers.bandwidth_rules') && (
              <div className="card p-3">
                <div className="flex items-center justify-between mb-3">
                  <h3 className="text-[12px] font-semibold text-gray-900 dark:text-white">Speed Rules</h3>
                  <button
                    type="button"
                    onClick={() => openBandwidthRuleModal()}
                    className="btn btn-primary btn-sm"
                  >
                    Add Rule
                  </button>
                </div>
                <p className="text-[12px] text-gray-500 mb-3">
                  Custom speed overrides for this subscriber. Set duration for temporary speed changes.
                </p>

                {bandwidthRules.length === 0 ? (
                  <p className="text-gray-500 text-center py-4">No bandwidth rules configured</p>
                ) : (
                  <div className="space-y-3">
                    {bandwidthRules.map((rule) => {
                      const timeRemaining = formatTimeRemaining(rule)
                      const isExpired = timeRemaining === 'Expired'
                      return (
                        <div
                          key={rule.id}
                          className={clsx(
                            'border p-3',
                            !rule.enabled || isExpired ? 'border-[#ccc] bg-gray-50 opacity-60' : 'border-[#a0a0a0] bg-white'
                          )}
                        >
                          <div className="flex items-center justify-between">
                            <div className="flex items-center gap-3">
                              <span className={clsx(
                                'px-1.5 py-0.5 text-[11px] font-medium border',
                                rule.rule_type === 'internet'
                                  ? 'bg-blue-50 text-blue-800 border-blue-300'
                                  : 'bg-purple-50 dark:bg-purple-900/50 text-purple-800 dark:text-purple-300 border-purple-300'
                              )}>
                                {rule.rule_type === 'internet' ? 'Internet' : (rule.cdn_name || 'CDN')}
                              </span>
                              <div>
                                <p className="font-medium">
                                  {rule.rule_type === 'cdn' ? `${Math.round(rule.download_speed / 1000)}M` : `${rule.download_speed}k / ${rule.upload_speed}k`}
                                </p>
                                <p className={clsx(
                                  'text-[12px]',
                                  isExpired ? 'text-red-500' : 'text-gray-500'
                                )}>
                                  {timeRemaining}
                                </p>
                              </div>
                            </div>
                            <div className="flex items-center gap-2">
                              <button
                                type="button"
                                onClick={() => openBandwidthRuleModal(rule)}
                                className="text-primary-600 hover:text-primary-800 text-[12px]"
                              >
                                Edit
                              </button>
                              <button
                                type="button"
                                onClick={() => handleDeleteBandwidthRule(rule.id)}
                                className="text-red-600 hover:text-red-800 text-[12px]"
                              >
                                Delete
                              </button>
                            </div>
                          </div>
                        </div>
                      )
                    })}
                  </div>
                )}
              </div>
            )}

            {/* Wallet Balance */}
            {!isNew && (
              <div className="card p-3">
                <div className="flex items-center justify-between mb-3">
                  <h3 className="text-[12px] font-semibold text-gray-900 dark:text-white flex items-center gap-1.5">
                    <BanknotesIcon className="w-4 h-4" />
                    Wallet Balance
                  </h3>
                  {hasPermission('subscribers.refill_quota') && (
                    <button
                      type="button"
                      onClick={() => setShowAddBalanceModal(true)}
                      className="btn btn-primary btn-sm"
                    >
                      Add Balance
                    </button>
                  )}
                </div>
                <div className="text-center py-3">
                  <div className={`text-2xl font-bold ${(subscriber?.balance || 0) > 0 ? 'text-green-600 dark:text-green-400' : 'text-gray-400 dark:text-gray-500'}`}>
                    ${(subscriber?.balance || 0).toFixed(2)}
                  </div>
                  <div className="text-xs text-gray-500 dark:text-gray-400 mt-1">Available Balance</div>
                </div>
              </div>
            )}

            {/* Public IP - Only show for existing subscribers */}
            {!isNew && hasPermission('public_ips.view') && (
              <div className="card p-3">
                <div className="flex items-center justify-between mb-3">
                  <h3 className="text-[12px] font-semibold text-gray-900 dark:text-white flex items-center gap-1.5">
                    <GlobeAltIcon className="w-4 h-4" />
                    Public IP
                  </h3>
                  {!subscriberPublicIP && hasPermission('public_ips.manage') && (
                    <button
                      type="button"
                      onClick={() => setShowAssignPublicIPModal(true)}
                      className="btn btn-primary btn-sm"
                    >
                      Assign IP
                    </button>
                  )}
                </div>
                {subscriberPublicIP ? (
                  <div className="bg-blue-50 dark:bg-blue-900/30 rounded-lg p-3 space-y-2">
                    <div className="flex items-center justify-between">
                      <span className="font-mono font-bold text-blue-700 dark:text-blue-300 text-sm">
                        {subscriberPublicIP.ip_address}
                      </span>
                      <span className="px-2 py-0.5 rounded text-xs font-semibold bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200">
                        Active
                      </span>
                    </div>
                    <div className="text-xs text-gray-600 dark:text-gray-400 space-y-0.5">
                      {subscriberPublicIP.pool && (
                        <div>Pool: <span className="font-medium">{subscriberPublicIP.pool.name}</span> ({subscriberPublicIP.pool.cidr})</div>
                      )}
                      {subscriberPublicIP.monthly_price > 0 && (
                        <div>Price: <span className="font-medium text-green-600 dark:text-green-400">${subscriberPublicIP.monthly_price.toFixed(2)}/mo</span></div>
                      )}
                      {subscriberPublicIP.next_billing_at && (
                        <div>Next billing: {new Date(subscriberPublicIP.next_billing_at).toLocaleDateString()}</div>
                      )}
                    </div>
                    {hasPermission('public_ips.manage') && (
                      <button
                        type="button"
                        onClick={() => { if (confirm(`Release public IP ${subscriberPublicIP.ip_address}?`)) releasePublicIPMutation.mutate(subscriberPublicIP.id) }}
                        disabled={releasePublicIPMutation.isPending}
                        className="text-xs text-red-600 hover:text-red-800 dark:text-red-400 font-semibold"
                      >
                        {releasePublicIPMutation.isPending ? 'Releasing...' : 'Release IP'}
                      </button>
                    )}
                  </div>
                ) : (
                  <p className="text-gray-500 text-center py-4">No public IP assigned</p>
                )}
              </div>
            )}

            {/* Status & Options */}
            <div className="card p-3">
              <h3 className="text-[12px] font-semibold text-gray-900 dark:text-white mb-3 pb-1 border-b border-[#ccc] dark:border-[#555]">Status & Options</h3>
              <div className="space-y-2">
                <div>
                  <label className="label">Status</label>
                  <select
                    name="status"
                    value={formData.status}
                    onChange={handleChange}
                    className="input"
                  >
                    <option value={1}>Active</option>
                    <option value={0}>Inactive</option>
                    <option value={2}>Suspended</option>
                  </select>
                </div>
                <label className="flex items-center gap-3">
                  <input
                    type="checkbox"
                    name="auto_renew"
                    checked={formData.auto_renew}
                    onChange={handleChange}
                    className="border-gray-300 text-primary-600 focus:ring-primary-500"
                  />
                  <span>Auto Renew</span>
                </label>
                <label className="flex items-center gap-3">
                  <input
                    type="checkbox"
                    name="auto_invoice"
                    checked={formData.auto_invoice}
                    onChange={handleChange}
                    className="border-gray-300 text-primary-600 focus:ring-primary-500"
                  />
                  <span>Auto Generate Invoice</span>
                </label>
                <div>
                  <label className="label">Notes</label>
                  <textarea
                    name="note"
                    value={formData.note}
                    onChange={handleChange}
                    className="input"
                    rows={4}
                    placeholder="Internal notes about this subscriber..."
                  />
                </div>

                {/* Location */}
                <div>
                  <label className="label flex items-center justify-between">
                    <span className="flex items-center gap-1.5">
                      <MapPinIcon className="w-4 h-4" />
                      Location
                    </span>
                    {(formData.latitude !== 0 || formData.longitude !== 0) && (
                      <button
                        type="button"
                        onClick={() => { setFormData(prev => ({ ...prev, latitude: 0, longitude: 0 })); setMapFlyTarget(null) }}
                        className="text-[11px] text-red-500 hover:text-red-700 font-normal"
                      >
                        Clear
                      </button>
                    )}
                  </label>
                  <div className="flex gap-2 mb-2">
                    <input
                      type="number"
                      step="any"
                      placeholder="Latitude (e.g. 34.4324)"
                      value={formData.latitude || ''}
                      onChange={(e) => setFormData(prev => ({ ...prev, latitude: parseFloat(e.target.value) || 0 }))}
                      onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); setMapFlyTarget([formData.latitude, formData.longitude]) } }}
                      className="input flex-1 text-[12px]"
                    />
                    <input
                      type="number"
                      step="any"
                      placeholder="Longitude (e.g. 35.8328)"
                      value={formData.longitude || ''}
                      onChange={(e) => setFormData(prev => ({ ...prev, longitude: parseFloat(e.target.value) || 0 }))}
                      onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); setMapFlyTarget([formData.latitude, formData.longitude]) } }}
                      className="input flex-1 text-[12px]"
                    />
                    <button
                      type="button"
                      onClick={() => setMapFlyTarget([formData.latitude, formData.longitude])}
                      className="btn btn-secondary text-[12px] px-3"
                      title="Go to coordinates"
                    >
                      Go
                    </button>
                    <button
                      type="button"
                      disabled={gettingLocation}
                      onClick={() => {
                        if (!navigator.geolocation) {
                          alert('Geolocation is not supported by your browser')
                          return
                        }
                        setGettingLocation(true)
                        navigator.geolocation.getCurrentPosition(
                          (pos) => {
                            const lat = pos.coords.latitude
                            const lng = pos.coords.longitude
                            setFormData(prev => ({ ...prev, latitude: lat, longitude: lng }))
                            setMapFlyTarget([lat, lng])
                            setGettingLocation(false)
                          },
                          (err) => {
                            setGettingLocation(false)
                            if (err.code === 1) {
                              const ua = navigator.userAgent || ''
                              const isIOS = /iPhone|iPad|iPod/i.test(ua)
                              const isAndroid = /Android/i.test(ua)
                              if (isIOS) {
                                alert('Location blocked by Safari.\n\nTo enable location:\n\n1. Open iPhone Settings\n2. Go to Safari → Advanced → Website Data\n3. Find this website and swipe to delete it\n4. Reload the page — Safari will ask permission again\n5. Tap "Allow"\n\nAlternatively:\nSettings → Privacy & Security → Location Services → Safari Websites → set to "While Using"')
                              } else if (isAndroid) {
                                alert('Location permission denied.\n\nTo enable location on Android:\n\n1. Tap the lock/tune icon in the browser address bar\n2. Tap "Permissions" or "Site settings"\n3. Set Location to "Allow"\n4. Reload the page\n\nOr in Android Settings:\nSettings → Apps → Chrome (or your browser) → Permissions → Location → Allow')
                              } else {
                                alert('Location permission denied.\n\nPlease allow location access in your browser settings and try again.')
                              }
                            } else if (err.code === 2) {
                              alert('GPS unavailable. Make sure Location Services is ON in your device settings.')
                            } else {
                              alert('Location timed out. Make sure GPS is enabled and try again.')
                            }
                          },
                          { enableHighAccuracy: true, timeout: 10000 }
                        )
                      }}
                      className="btn btn-secondary text-[12px] px-3 flex items-center gap-1 whitespace-nowrap"
                      title="Use my current location"
                    >
                      {gettingLocation ? (
                        <svg className="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
                          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"/>
                          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"/>
                        </svg>
                      ) : (
                        <MapPinIcon className="w-4 h-4" />
                      )}
                      {gettingLocation ? 'Getting...' : 'My Location'}
                    </button>
                  </div>
                  {!isLoading && (
                    <div className="overflow-hidden border border-[#a0a0a0] dark:border-[#555]">
                      <LocationMap
                        lat={formData.latitude}
                        lng={formData.longitude}
                        onLocationChange={(lat, lng) => {
                          setFormData(prev => ({ ...prev, latitude: lat, longitude: lng }))
                          setMapFlyTarget([lat, lng])
                        }}
                        flyTarget={mapFlyTarget}
                      />
                    </div>
                  )}
                  {formData.latitude !== 0 && formData.longitude !== 0 && (
                    <div className="flex items-center justify-between mt-1">
                      <p className="text-[11px] text-gray-400">
                        📍 {formData.latitude.toFixed(6)}, {formData.longitude.toFixed(6)}
                      </p>
                      <a
                        href={`https://www.google.com/maps/dir/?api=1&destination=${formData.latitude},${formData.longitude}`}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="btn btn-secondary text-[11px] px-3 py-1 flex items-center gap-1"
                      >
                        <MapPinIcon className="w-3.5 h-3.5" />
                        Navigate
                      </a>
                    </div>
                  )}
                </div>

              </div>
            </div>
          </div>

          <div className="flex items-center justify-end gap-2">
            <Link to="/subscribers" className="btn btn-secondary">
              Cancel
            </Link>
            {hasPermission(isNew ? 'subscribers.create' : 'subscribers.edit') && (
              <button
                type="submit"
                disabled={saveMutation.isLoading}
                className="btn btn-primary"
              >
                {saveMutation.isLoading ? 'Saving...' : isNew ? 'Create Subscriber' : 'Save Changes'}
              </button>
            )}
          </div>
        </form>
      )}

      {/* Usage Tab */}
      {!isNew && activeTab === 'usage' && (
        <div className="space-y-3">
          {/* Monthly Summary */}
          <div className="card p-3">
            <h3 className="text-[12px] font-semibold text-gray-900 dark:text-white mb-2 pb-1 border-b border-[#ccc] dark:border-[#555]">Monthly</h3>
            <div className="flex items-end gap-4 h-48">
              {/* Download Bar */}
              <div className="flex flex-col items-center flex-1">
                <div className="w-full max-w-[120px] bg-gray-100 border border-[#a0a0a0] relative h-40 flex items-end">
                  <div
                    className="w-full transition-all duration-500"
                    style={{
                      height: (() => {
                        const used = monthlyQuota?.download_used || 0
                        const limit = monthlyQuota?.download_limit || 0
                        if (limit > 0) {
                          return `${Math.min(used / limit * 100, 100)}%`
                        }
                        // For unlimited: scale based on 100GB reference
                        if (used > 0) {
                          return `${Math.min(used / 107374182400 * 100, 100)}%`
                        }
                        return '0%'
                      })(),
                      backgroundColor: '#14B8A6',
                      minHeight: (monthlyQuota?.download_used || 0) > 0 ? '8px' : '0'
                    }}
                  />
                </div>
                <div className="mt-2 text-center">
                  <div className="text-[13px] font-bold text-teal-500">
                    {((monthlyQuota?.download_used || 0) / 1073741824).toFixed(2)} GB
                  </div>
                  <div className="text-[12px] text-gray-500 dark:text-gray-400">
                    {monthlyQuota?.download_limit > 0 ? `/ ${(monthlyQuota.download_limit / 1073741824).toFixed(0)} GB` : 'Unlimited'}
                  </div>
                  <div className="text-[11px] text-gray-400 dark:text-gray-500 mt-0.5">Download</div>
                </div>
              </div>

              {/* Upload Bar */}
              <div className="flex flex-col items-center flex-1">
                <div className="w-full max-w-[120px] bg-gray-100 border border-[#a0a0a0] relative h-40 flex items-end">
                  <div
                    className="w-full transition-all duration-500"
                    style={{
                      height: (() => {
                        const used = monthlyQuota?.upload_used || 0
                        const limit = monthlyQuota?.upload_limit || 0
                        if (limit > 0) {
                          return `${Math.min(used / limit * 100, 100)}%`
                        }
                        // For unlimited: scale based on 100GB reference
                        if (used > 0) {
                          return `${Math.min(used / 107374182400 * 100, 100)}%`
                        }
                        return '0%'
                      })(),
                      backgroundColor: '#F97316',
                      minHeight: (monthlyQuota?.upload_used || 0) > 0 ? '8px' : '0'
                    }}
                  />
                </div>
                <div className="mt-2 text-center">
                  <div className="text-[13px] font-bold text-orange-500">
                    {((monthlyQuota?.upload_used || 0) / 1073741824).toFixed(2)} GB
                  </div>
                  <div className="text-[12px] text-gray-500 dark:text-gray-400">
                    {monthlyQuota?.upload_limit > 0 ? `/ ${(monthlyQuota.upload_limit / 1073741824).toFixed(0)} GB` : 'Unlimited'}
                  </div>
                  <div className="text-[11px] text-gray-400 dark:text-gray-500 mt-0.5">Upload</div>
                </div>
              </div>
            </div>
          </div>

          {/* Daily Usage Chart */}
          <div className="card p-3">
            <h3 className="text-[12px] font-semibold text-gray-900 dark:text-white mb-3 pb-1 border-b border-[#ccc] dark:border-[#555]">Daily</h3>
            <Suspense fallback={<div style={{ height: '300px' }} />}>
            <ReactECharts
              option={{
                tooltip: {
                  trigger: 'axis',
                  axisPointer: { type: 'shadow' },
                  formatter: (params) => {
                    let result = `Day ${params[0].name}<br/>`
                    params.forEach((param) => {
                      const val = (param.value / 1073741824).toFixed(2)
                      result += `${param.marker} ${param.seriesName}: ${val} GB<br/>`
                    })
                    return result
                  }
                },
                legend: {
                  data: ['Download', 'Upload'],
                  top: 0,
                },
                grid: {
                  left: '3%',
                  right: '4%',
                  bottom: '3%',
                  top: '40px',
                  containLabel: true,
                },
                xAxis: {
                  type: 'category',
                  data: Array.from({ length: dailyQuota?.daily_download?.length || new Date(new Date().getFullYear(), new Date().getMonth() + 1, 0).getDate() }, (_, i) => i + 1),
                  axisLine: { lineStyle: { color: '#ccc' } },
                  axisLabel: { color: '#666' },
                },
                yAxis: {
                  type: 'value',
                  axisLabel: {
                    formatter: (val) => (val / 1073741824).toFixed(1) + ' GB',
                    color: '#666',
                  },
                  axisLine: { lineStyle: { color: '#ccc' } },
                  splitLine: { lineStyle: { color: '#eee' } },
                },
                series: [
                  {
                    name: 'Download',
                    type: 'bar',
                    data: dailyQuota?.daily_download || [],
                    itemStyle: { color: '#14B8A6' },
                    barWidth: '35%',
                  },
                  {
                    name: 'Upload',
                    type: 'bar',
                    data: dailyQuota?.daily_upload || [],
                    itemStyle: { color: '#F97316' },
                    barWidth: '35%',
                  },
                ],
              }}
              style={{ height: '300px' }}
            />
            </Suspense>
          </div>

          {/* Daily Quota Summary */}
          <div className="card p-3">
            <h3 className="text-[12px] font-semibold text-gray-900 dark:text-white mb-3 pb-1 border-b border-[#ccc] dark:border-[#555]">Today's Usage</h3>
            <div className="grid grid-cols-2 gap-3">
              <div className="text-center p-3 bg-teal-50 border border-[#a0a0a0]">
                <div className="text-[13px] font-bold text-teal-600">
                  {((dailyQuota?.download_used || 0) / 1073741824).toFixed(2)} GB
                </div>
                <div className="text-[12px] text-gray-500 dark:text-gray-400 mt-1">
                  Download {dailyQuota?.download_limit > 0 && `/ ${(dailyQuota.download_limit / 1073741824).toFixed(0)} GB`}
                </div>
              </div>
              <div className="text-center p-3 bg-orange-50 border border-[#a0a0a0]">
                <div className="text-[13px] font-bold text-orange-600">
                  {((dailyQuota?.upload_used || 0) / 1073741824).toFixed(2)} GB
                </div>
                <div className="text-[12px] text-gray-500 dark:text-gray-400 mt-1">
                  Upload {dailyQuota?.upload_limit > 0 && `/ ${(dailyQuota.upload_limit / 1073741824).toFixed(0)} GB`}
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Live Graph Tab */}
      {!isNew && activeTab === 'graph' && (
        <div className="space-y-3">
          {/* Current Stats */}
          {subscriber?.is_online && (
            <div className="space-y-2">
              <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-2">
                <div className="card p-2 text-center">
                  <div className="text-[13px] font-bold text-green-500">{currentBandwidth.download}</div>
                  <div className="text-[11px] text-gray-500 dark:text-gray-400">Download (Mbps)</div>
                </div>
                <div className="card p-2 text-center">
                  <div className="text-[13px] font-bold text-blue-500">{currentBandwidth.upload}</div>
                  <div className="text-[11px] text-gray-500 dark:text-gray-400">Upload (Mbps)</div>
                </div>
                <div className="card p-2 text-center" style={{ borderTop: '2px solid #F59E0B' }}>
                  <div className={`text-[13px] font-bold ${
                    !livePing.ok             ? 'text-gray-400 dark:text-gray-500' :
                    livePing.ms < 20         ? 'text-green-500' :
                    livePing.ms < 80         ? 'text-yellow-500' :
                                               'text-red-500'
                  }`}>
                    {livePing.ok ? `${livePing.ms.toFixed(1)}` : '—'}
                  </div>
                  <div className="text-[11px] text-gray-500 dark:text-gray-400">Latency (ms)</div>
                </div>
                <div className="card p-2 text-center">
                  <div className="text-[12px] font-semibold text-gray-700 dark:text-gray-300">{currentBandwidth.ipAddress || '-'}</div>
                  <div className="text-[11px] text-gray-500 dark:text-gray-400">IP Address</div>
                </div>
                <div className="card p-2 text-center">
                  <div className="text-[12px] font-semibold text-gray-700 dark:text-gray-300">{currentBandwidth.uptime || '-'}</div>
                  <div className="text-[11px] text-gray-500 dark:text-gray-400">Uptime</div>
                </div>
              </div>

              {/* CDN Traffic Stats - Show live rate in Mbps */}
              {cdnList.length > 0 && (
                <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-2">
                  {cdnList.map(cdn => {
                    const cdnRateData = cdnDataRefs.current[cdn.id] || []
                    const currentRate = cdnRateData.length > 0 ? cdnRateData[cdnRateData.length - 1] : 0
                    return (
                      <div key={cdn.id} className="card p-2 text-center" style={{ borderTop: `2px solid ${cdn.color}` }}>
                        <div className="text-[12px] font-bold" style={{ color: cdn.color }}>{currentRate.toFixed(2)} Mbps</div>
                        <div className="text-[11px] text-gray-500 dark:text-gray-400">{cdn.name}</div>
                      </div>
                    )
                  })}
                </div>
              )}

              {/* Port Rule Traffic Stats - Show live rate in Mbps */}
              {portRuleList.length > 0 && (
                <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-2">
                  {portRuleList.map(pr => {
                    const prRateData = portRuleDataRefs.current[pr.id] || []
                    const currentRate = prRateData.length > 0 ? prRateData[prRateData.length - 1] : 0
                    return (
                      <div key={pr.id} className="card p-2 text-center" style={{ borderTop: `2px dashed ${pr.color}` }}>
                        <div className="text-[12px] font-bold" style={{ color: pr.color }}>{currentRate.toFixed(2)} Mbps</div>
                        <div className="text-[11px] text-gray-500 dark:text-gray-400">Port: {pr.name}</div>
                      </div>
                    )
                  })}
                </div>
              )}
            </div>
          )}

          {/* Chart */}
          <div className="card p-3">
            <div className="flex items-center justify-between mb-3">
              <h3 className="text-[12px] font-semibold text-gray-900 dark:text-white">Live Bandwidth Graph</h3>
              {subscriber?.is_online && (
                <div className="flex items-center gap-3">
                  <div className="flex items-center gap-1.5 text-[11px] text-green-600">
                    <span className="relative flex h-2.5 w-2.5">
                      <span className="animate-ping absolute inline-flex h-full w-full bg-green-400 opacity-75"></span>
                      <span className="relative inline-flex h-2.5 w-2.5 bg-green-500"></span>
                    </span>
                    Live (updating every 2s)
                  </div>
                  {livePing.ok ? (
                    <div className={`flex items-center gap-1 text-[11px] font-mono px-1.5 py-0.5 border border-[#a0a0a0] ${
                      livePing.ms < 20  ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-400' :
                      livePing.ms < 80  ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-400' :
                                          'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-400'
                    }`}>
                      <SignalIcon className="h-3 w-3" />
                      {livePing.ms.toFixed(1)} ms
                    </div>
                  ) : (
                    <div className="flex items-center gap-1 text-[11px] font-mono px-1.5 py-0.5 border border-[#a0a0a0] bg-gray-100 text-gray-400 dark:bg-gray-700 dark:text-gray-500">
                      <SignalIcon className="h-3 w-3" />
                      — ms
                    </div>
                  )}
                </div>
              )}
            </div>
            {subscriber?.is_online ? (
              <Suspense fallback={<div style={{ height: '400px' }} />}>
              <ReactECharts
                ref={chartRef}
                option={bandwidthChartOption}
                notMerge={false}
                lazyUpdate={true}
                style={{ height: '400px' }}
              />
              </Suspense>
            ) : (
              <div className="flex items-center justify-center h-64 text-gray-500 dark:text-gray-400">
                Subscriber is offline. Live graph is only available when connected.
              </div>
            )}
          </div>
        </div>
      )}

      {/* Invoices Tab */}
      {!isNew && activeTab === 'invoices' && (
        <div className="card p-3">
          <h3 className="text-[12px] font-semibold text-gray-900 dark:text-white mb-3 pb-1 border-b border-[#ccc] dark:border-[#555]">Invoices & Payments</h3>
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
                  <th style={{ textAlign: 'right' }}>Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                {subscriberInvoices.length === 0 ? (
                  <tr>
                    <td colSpan={7} className="text-center text-gray-500 py-8">
                      No invoices found
                    </td>
                  </tr>
                ) : (
                  subscriberInvoices.map((inv) => (
                    <tr key={inv.id}>
                      <td className="text-[11px]">{inv.created_at ? new Date(inv.created_at).toLocaleDateString() : '-'}</td>
                      <td className="text-[11px] font-medium">
                        {inv.invoice_number}
                        {inv.auto_generated && (
                          <span className="ml-1 px-1.5 py-0.5 text-[9px] font-semibold bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300 rounded">Auto</span>
                        )}
                      </td>
                      <td className="text-[11px] font-medium">${(inv.total || 0).toFixed(2)}</td>
                      <td className="text-[11px]">${(inv.amount_paid || 0).toFixed(2)}</td>
                      <td>
                        <span className={`badge ${
                          inv.status === 'completed' ? 'badge-success' :
                          inv.status === 'overdue' ? 'badge-danger' :
                          'badge-warning'
                        }`}>
                          {inv.status}
                        </span>
                      </td>
                      <td className="text-[11px]">{inv.due_date ? new Date(inv.due_date).toLocaleDateString() : '-'}</td>
                      <td style={{ textAlign: 'right' }}>
                        <button
                          onClick={() => setViewInvoiceId(inv.id)}
                          className="btn btn-xs"
                          title="View Invoice"
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
          {viewInvoiceId && (
            <InvoiceDetailModal
              invoiceId={viewInvoiceId}
              onClose={() => setViewInvoiceId(null)}
            />
          )}
        </div>
      )}

      {/* Logs Tab */}
      {!isNew && activeTab === 'logs' && (
        <div className="card p-3">
          <h3 className="text-[12px] font-semibold text-gray-900 dark:text-white mb-3 pb-1 border-b border-[#ccc] dark:border-[#555]">Session & Activity Logs</h3>
          <div className="table-container">
            <table className="table">
              <thead>
                <tr>
                  <th>Session ID</th>
                  <th>Start Time</th>
                  <th>End Time</th>
                  <th>Duration</th>
                  <th>IP Address</th>
                  <th>MAC Address</th>
                  <th>Download</th>
                  <th>Upload</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                {sessions.length === 0 ? (
                  <tr>
                    <td colSpan={8} className="text-center text-gray-500 py-8">
                      No session logs found
                    </td>
                  </tr>
                ) : (
                  sessions.map((session, idx) => {
                    const startTime = session.acctstarttime ? new Date(session.acctstarttime) : null
                    const endTime = session.acctstoptime ? new Date(session.acctstoptime) : null
                    const isActive = !session.acctstoptime

                    // Calculate duration
                    let duration = '-'
                    if (session.acctsessiontime > 0) {
                      const hours = Math.floor(session.acctsessiontime / 3600)
                      const mins = Math.floor((session.acctsessiontime % 3600) / 60)
                      duration = `${hours}h ${mins}m`
                    } else if (startTime && !endTime) {
                      const now = new Date()
                      const diffMs = now - startTime
                      const hours = Math.floor(diffMs / 3600000)
                      const mins = Math.floor((diffMs % 3600000) / 60000)
                      duration = `${hours}h ${mins}m`
                    }

                    // Format bytes
                    const formatBytes = (bytes) => {
                      if (!bytes || bytes === 0) return '0 B'
                      const units = ['B', 'KB', 'MB', 'GB']
                      const i = Math.floor(Math.log(bytes) / Math.log(1024))
                      return `${(bytes / Math.pow(1024, i)).toFixed(2)} ${units[i]}`
                    }

                    return (
                      <tr key={session.acctsessionid || idx} className={isActive ? 'bg-[#e8ffe8]' : ''}>
                        <td className="font-mono text-[12px]">
                          {session.acctsessionid}
                          {isActive && (
                            <span className="ml-2 px-1.5 py-0.5 text-[11px] bg-green-100 text-green-800 border border-green-300">
                              Active
                            </span>
                          )}
                        </td>
                        <td className="text-[12px]">
                          {formatDateTime(session.acctstarttime)}
                        </td>
                        <td className="text-[12px]">
                          {formatDateTime(session.acctstoptime)}
                        </td>
                        <td className="text-[12px]">{duration}</td>
                        <td className="font-mono text-[12px]">{session.framedipaddress || '-'}</td>
                        <td className="font-mono text-[12px] text-gray-500 dark:text-gray-400">{session.callingstationid || '-'}</td>
                        <td className="text-[12px] text-green-600">{formatBytes(session.acctoutputoctets)}</td>
                        <td className="text-[12px] text-blue-600">{formatBytes(session.acctinputoctets)}</td>
                      </tr>
                    )
                  })
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Bandwidth Rule Modal */}
      {showBandwidthRuleModal && (
        <div className="modal-overlay">
          <div className="modal w-full max-w-md mx-4">
            <div className="modal-header">
              <h3 className="text-[12px] font-semibold">
                {editingBandwidthRule ? 'Edit Bandwidth Rule' : 'Add Bandwidth Rule'}
              </h3>
            </div>

            <div className="modal-body space-y-2">
              <div>
                <label className="label">Rule Type</label>
                <select
                  value={bandwidthRuleForm.rule_type}
                  onChange={(e) => setBandwidthRuleForm({ ...bandwidthRuleForm, rule_type: e.target.value })}
                  className="input"
                >
                  <option value="internet">Internet Speed</option>
                  <option value="cdn">CDN Speed</option>
                </select>
              </div>

              {bandwidthRuleForm.rule_type === 'cdn' ? (
                <div className="space-y-2">
                  <div>
                    <label className="label">Select CDN</label>
                    {filteredCDNsForNAS.length > 0 ? (
                      <select
                        value={bandwidthRuleForm.cdn_id || ''}
                        onChange={(e) => setBandwidthRuleForm({ ...bandwidthRuleForm, cdn_id: e.target.value, download_speed: '', upload_speed: '' })}
                        className="input"
                      >
                        <option value="">Select CDN...</option>
                        {filteredCDNsForNAS.map((cdn) => (
                          <option key={cdn.cdn_id} value={cdn.cdn_id}>
                            {cdn.cdn_name}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <p className="text-[12px] text-gray-500 italic">No CDNs available for this NAS</p>
                    )}
                  </div>
                  {bandwidthRuleForm.cdn_id && (
                    <div>
                      <label className="label">Select Speed</label>
                      {getSpeedsForCDN(bandwidthRuleForm.cdn_id).length > 0 ? (
                        <select
                          value={bandwidthRuleForm.download_speed || ''}
                          onChange={(e) => setBandwidthRuleForm({ ...bandwidthRuleForm, download_speed: e.target.value, upload_speed: e.target.value })}
                          className="input"
                        >
                          <option value="">Select speed...</option>
                          {getSpeedsForCDN(bandwidthRuleForm.cdn_id).map((speed, idx) => (
                            <option key={idx} value={speed.speed_limit}>
                              {speed.speed_limit}M ({speed.service_name})
                            </option>
                          ))}
                        </select>
                      ) : (
                        <p className="text-[12px] text-gray-500 italic">No speeds configured for this CDN</p>
                      )}
                    </div>
                  )}
                  {currentCDNs.length > 0 && (
                    <p className="text-[11px] text-gray-500 dark:text-gray-400">
                      Current service CDNs: {currentCDNs.map(c => `${c.cdn_name} ${c.speed_limit}M`).join(', ')}
                    </p>
                  )}
                </div>
              ) : (
                <div className="grid grid-cols-2 gap-2">
                  <div>
                    <label className="label">Download (kbps)</label>
                    <input
                      type="number"
                      value={bandwidthRuleForm.download_speed}
                      onChange={(e) => setBandwidthRuleForm({ ...bandwidthRuleForm, download_speed: e.target.value })}
                      className="input"
                      placeholder="e.g., 50000"
                    />
                  </div>
                  <div>
                    <label className="label">Upload (kbps)</label>
                    <input
                      type="number"
                      value={bandwidthRuleForm.upload_speed}
                      onChange={(e) => setBandwidthRuleForm({ ...bandwidthRuleForm, upload_speed: e.target.value })}
                      className="input"
                      placeholder="e.g., 10000"
                    />
                  </div>
                </div>
              )}

              <div>
                <label className="label">Duration</label>
                <select
                  value={bandwidthRuleForm.duration}
                  onChange={(e) => setBandwidthRuleForm({ ...bandwidthRuleForm, duration: e.target.value })}
                  className="input"
                >
                  <option value="permanent">Permanent</option>
                  <option value="1h">1 Hour</option>
                  <option value="2h">2 Hours</option>
                  <option value="6h">6 Hours</option>
                  <option value="12h">12 Hours</option>
                  <option value="1d">1 Day</option>
                  <option value="2d">2 Days</option>
                  <option value="7d">7 Days</option>
                  <option value="14d">14 Days</option>
                  <option value="30d">30 Days</option>
                </select>
                <p className="text-[11px] text-gray-500 dark:text-gray-400 mt-1">
                  {bandwidthRuleForm.duration === 'permanent'
                    ? 'Rule will apply until manually disabled or deleted'
                    : 'After duration expires, subscriber returns to normal service speed'}
                </p>
              </div>

              <label className="flex items-center gap-3">
                <input
                  type="checkbox"
                  checked={bandwidthRuleForm.enabled}
                  onChange={(e) => setBandwidthRuleForm({ ...bandwidthRuleForm, enabled: e.target.checked })}
                  className="border-gray-300 text-primary-600 focus:ring-primary-500"
                />
                <span>Enabled</span>
              </label>
            </div>

            <div className="modal-footer">
              <button
                type="button"
                onClick={() => {
                  setShowBandwidthRuleModal(false)
                  resetBandwidthRuleForm()
                }}
                className="btn btn-secondary"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleSaveBandwidthRule}
                disabled={saveBandwidthRuleMutation.isPending}
                className="btn btn-primary"
              >
                {saveBandwidthRuleMutation.isPending ? 'Saving...' : 'Save'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Add Balance Modal */}
      {showAddBalanceModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-gray-800 rounded-lg p-6 w-full max-w-sm shadow-xl">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-bold text-gray-900 dark:text-white">Add Balance</h2>
              <button onClick={() => setShowAddBalanceModal(false)} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                <XMarkIcon className="w-5 h-5" />
              </button>
            </div>
            <div className="space-y-3">
              <div className="text-sm text-gray-600 dark:text-gray-400">
                Current balance: <span className="font-bold text-green-600 dark:text-green-400">${(subscriber?.balance || 0).toFixed(2)}</span>
              </div>
              <div>
                <label className="label">Amount *</label>
                <input
                  type="number"
                  step="0.01"
                  min="0"
                  value={addBalanceAmount}
                  onChange={(e) => setAddBalanceAmount(e.target.value)}
                  className="input"
                  placeholder="Enter amount"
                />
              </div>
              <div>
                <label className="label">Reason</label>
                <select value={addBalanceReason} onChange={(e) => setAddBalanceReason(e.target.value)} className="input">
                  <option value="">Select reason...</option>
                  <option value="cash_payment">Cash Payment</option>
                  <option value="bank_transfer">Bank Transfer</option>
                  <option value="prepaid_card">Prepaid Card</option>
                  <option value="credit">Credit</option>
                  <option value="other">Other</option>
                </select>
              </div>
            </div>
            <div className="flex justify-end gap-2 mt-4">
              <button onClick={() => setShowAddBalanceModal(false)} className="btn">Cancel</button>
              <button
                onClick={() => addBalanceMutation.mutate({ amount: parseFloat(addBalanceAmount), reason: addBalanceReason })}
                disabled={!addBalanceAmount || parseFloat(addBalanceAmount) <= 0 || addBalanceMutation.isPending}
                className="btn btn-primary"
              >
                {addBalanceMutation.isPending ? 'Adding...' : 'Add Balance'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Assign Public IP Modal */}
      {showAssignPublicIPModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-gray-800 rounded-lg p-6 w-full max-w-lg shadow-xl">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-bold text-gray-900 dark:text-white">Assign Public IP</h2>
              <button onClick={() => setShowAssignPublicIPModal(false)} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                <XMarkIcon className="w-5 h-5" />
              </button>
            </div>
            <form onSubmit={e => {
              e.preventDefault()
              assignPublicIPMutation.mutate({
                pool_id: parseInt(publicIPPoolId),
                subscriber_id: parseInt(id),
                ip_address: publicIPAddress || undefined,
                notes: publicIPNotes,
              })
            }}>
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">Pool *</label>
                  <select value={publicIPPoolId}
                    onChange={e => setPublicIPPoolId(e.target.value)}
                    className="input w-full" required>
                    <option value="">-- Select Pool --</option>
                    {publicIPPools.filter(p => p.is_active).map(p => (
                      <option key={p.id} value={p.id}>
                        {p.name} ({p.cidr}) - {p.used_ips}/{p.total_ips} used {p.monthly_price > 0 ? `- $${p.monthly_price}/mo` : '- Free'}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">Specific IP (optional)</label>
                  <input type="text" value={publicIPAddress}
                    onChange={e => setPublicIPAddress(e.target.value)}
                    className="input w-full font-mono" placeholder="Leave empty for auto-assign" />
                </div>
                <div>
                  <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">Notes</label>
                  <input type="text" value={publicIPNotes}
                    onChange={e => setPublicIPNotes(e.target.value)}
                    className="input w-full" placeholder="Optional notes" />
                </div>
              </div>
              <div className="flex gap-2 mt-6">
                <button type="submit" disabled={assignPublicIPMutation.isPending || !publicIPPoolId}
                  className="flex-1 btn btn-primary">
                  {assignPublicIPMutation.isPending ? 'Assigning...' : 'Assign IP'}
                </button>
                <button type="button" onClick={() => setShowAssignPublicIPModal(false)} className="flex-1 btn btn-secondary">Cancel</button>
              </div>
            </form>
          </div>
        </div>
      )}

    </div>
  )
}
