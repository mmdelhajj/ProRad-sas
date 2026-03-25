import { useState, useEffect, useRef, lazy, Suspense, useCallback } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { bandwidthCustomerApi } from '../services/api'
import { useAuthStore } from '../store/authStore'
import toast from 'react-hot-toast'
const ReactECharts = lazy(() => import('echarts-for-react'))

function formatSpeed(kb) {
  if (!kb) return '0k'
  if (kb >= 1000000) return (kb / 1000000).toFixed(1) + 'G'
  if (kb >= 1000) return (kb / 1000).toFixed(0) + 'M'
  return kb + 'k'
}

function formatBytes(bytes) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let val = bytes
  while (val >= 1024 && i < units.length - 1) { val /= 1024; i++ }
  return val.toFixed(i > 1 ? 1 : 0) + ' ' + units[i]
}

function formatDuration(seconds) {
  if (!seconds) return '0s'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = seconds % 60
  if (h > 0) return `${h}h ${m}m`
  if (m > 0) return `${m}m ${s}s`
  return `${s}s`
}

function formatDateTime(dateStr) {
  if (!dateStr) return '—'
  const d = new Date(dateStr)
  return d.toLocaleString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false })
}

const statusColors = {
  active: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
  suspended: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
  expired: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
}

const TABS = [
  { id: 'overview', label: 'Overview' },
  { id: 'live', label: 'Live Graph' },
  { id: 'historical', label: 'Historical' },
  { id: 'sessions', label: 'Sessions' },
  { id: 'heatmap', label: 'Heatmap' },
]

export default function BandwidthCustomerDetail() {
  const { id } = useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { hasPermission } = useAuthStore()
  const [activeTab, setActiveTab] = useState('overview')
  const [showSpeedModal, setShowSpeedModal] = useState(false)
  const [newSpeed, setNewSpeed] = useState({ download: 0, upload: 0 })
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)

  // Live graph state
  const downloadDataRef = useRef(Array(30).fill(0))
  const uploadDataRef = useRef(Array(30).fill(0))
  const [currentBandwidth, setCurrentBandwidth] = useState({ download: '0.00', upload: '0.00' })
  const [graphActive, setGraphActive] = useState(false)
  const timeLabels = Array.from({ length: 30 }, (_, i) => `${(30 - i) * 2}s`)

  // Historical state
  const [historyDays, setHistoryDays] = useState(7)
  // Sessions state
  const [sessionDays, setSessionDays] = useState(30)
  // Heatmap state
  const [heatmapDays, setHeatmapDays] = useState(30)
  const [heatmapMode, setHeatmapMode] = useState('combined') // download, upload, combined

  // Chart refs for PNG export
  const historicalChartRef = useRef(null)
  const sessionsChartRef = useRef(null)
  const heatmapChartRef = useRef(null)

  const { data: customerData, isLoading, refetch } = useQuery({
    queryKey: ['bandwidth-customer', id],
    queryFn: () => bandwidthCustomerApi.get(id),
  })

  const { data: usageData } = useQuery({
    queryKey: ['bandwidth-customer-usage', id],
    queryFn: () => bandwidthCustomerApi.getUsage(id, 30),
  })

  const { data: hourlyData } = useQuery({
    queryKey: ['bandwidth-customer-hourly', id, historyDays],
    queryFn: () => bandwidthCustomerApi.getHourlyUsage(id, historyDays),
    enabled: activeTab === 'historical',
  })

  const { data: sessionsData } = useQuery({
    queryKey: ['bandwidth-customer-sessions', id, sessionDays],
    queryFn: () => bandwidthCustomerApi.getSessions(id, sessionDays),
    enabled: activeTab === 'sessions',
  })

  const { data: heatmapData } = useQuery({
    queryKey: ['bandwidth-customer-heatmap', id, heatmapDays],
    queryFn: () => bandwidthCustomerApi.getHeatmap(id, heatmapDays),
    enabled: activeTab === 'heatmap',
  })

  const customer = customerData?.data?.data || null
  const usageHistory = usageData?.data?.data || []
  const hourlyUsage = hourlyData?.data?.data || []
  const sessions = sessionsData?.data?.data || []
  const heatmap = heatmapData?.data?.data || []

  // Live graph polling
  useEffect(() => {
    if (!graphActive || !customer) return
    let isMounted = true

    const fetchBandwidth = async () => {
      try {
        const response = await bandwidthCustomerApi.getBandwidth(id)
        if (response.data.success && isMounted) {
          const data = response.data.data
          const dl = data.download || 0
          const ul = data.upload || 0
          setCurrentBandwidth({ download: dl.toFixed(2), upload: ul.toFixed(2) })
          downloadDataRef.current = [...downloadDataRef.current.slice(1), dl]
          uploadDataRef.current = [...uploadDataRef.current.slice(1), ul]
        }
      } catch { /* ignore polling errors */ }
    }

    fetchBandwidth()
    const interval = setInterval(fetchBandwidth, 2000)
    return () => { isMounted = false; clearInterval(interval) }
  }, [graphActive, id, customer])

  // Action mutations
  const suspendMutation = useMutation({
    mutationFn: () => bandwidthCustomerApi.suspend(id),
    onSuccess: () => { toast.success('Customer suspended'); refetch(); queryClient.invalidateQueries(['bandwidth-customers']) },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed'),
  })

  const unsuspendMutation = useMutation({
    mutationFn: () => bandwidthCustomerApi.unsuspend(id),
    onSuccess: () => { toast.success('Customer unsuspended'); refetch(); queryClient.invalidateQueries(['bandwidth-customers']) },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed'),
  })

  const resetFupMutation = useMutation({
    mutationFn: () => bandwidthCustomerApi.resetFup(id),
    onSuccess: () => { toast.success('FUP reset'); refetch() },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed'),
  })

  const changeSpeedMutation = useMutation({
    mutationFn: (data) => bandwidthCustomerApi.changeSpeed(id, data),
    onSuccess: () => { toast.success('Speed changed'); setShowSpeedModal(false); refetch() },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed'),
  })

  const deleteMutation = useMutation({
    mutationFn: () => bandwidthCustomerApi.delete(id),
    onSuccess: () => { toast.success('Customer deleted'); navigate('/bandwidth-manager') },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed'),
  })

  const exportChart = useCallback((chartRef, filename) => {
    if (!chartRef.current) return
    const instance = chartRef.current.getEchartsInstance()
    const url = instance.getDataURL({ type: 'png', pixelRatio: 2, backgroundColor: '#fff' })
    const link = document.createElement('a')
    link.download = `${filename}.png`
    link.href = url
    link.click()
  }, [])

  // ---- Chart Options ----

  const liveChartOption = {
    animation: false,
    tooltip: {
      trigger: 'axis',
      formatter: (params) => {
        let result = params[0].name + '<br/>'
        params.forEach((p) => {
          if (p.value != null) result += `${p.marker} ${p.seriesName}: ${p.value.toFixed(2)} Mbps<br/>`
        })
        return result
      },
    },
    legend: { data: ['Download', 'Upload'], top: 0 },
    grid: { left: '3%', right: '4%', bottom: '3%', top: '40px', containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, data: timeLabels, axisLine: { lineStyle: { color: '#ccc' } } },
    yAxis: {
      type: 'value', min: 0,
      max: (v) => Math.max(3, Math.ceil(Math.max(v.max, 0.1) * 1.2)),
      axisLabel: { formatter: '{value} Mbps' },
      axisLine: { lineStyle: { color: '#ccc' } },
      splitLine: { lineStyle: { color: '#eee' } },
    },
    series: [
      {
        name: 'Download', type: 'line', smooth: true, data: downloadDataRef.current,
        lineStyle: { color: '#10B981', width: 2 }, itemStyle: { color: '#10B981' },
        areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: 'rgba(16,185,129,0.4)' }, { offset: 1, color: 'rgba(16,185,129,0.05)' }] } },
        showSymbol: false,
      },
      {
        name: 'Upload', type: 'line', smooth: true, data: uploadDataRef.current,
        lineStyle: { color: '#3B82F6', width: 2 }, itemStyle: { color: '#3B82F6' },
        areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: 'rgba(59,130,246,0.4)' }, { offset: 1, color: 'rgba(59,130,246,0.05)' }] } },
        showSymbol: false,
      },
    ],
  }

  const historicalChartOption = {
    tooltip: {
      trigger: 'axis',
      formatter: (params) => {
        let result = params[0].axisValueLabel + '<br/>'
        params.forEach((p) => {
          if (p.value != null) {
            const label = p.seriesName.includes('Peak') ? `${p.marker} ${p.seriesName}: ${(p.value / 1000).toFixed(1)} Mbps` : `${p.marker} ${p.seriesName}: ${formatBytes(p.value)}`
            result += label + '<br/>'
          }
        })
        return result
      },
    },
    legend: { data: ['Download', 'Upload', 'Peak Download', 'Peak Upload'], top: 0 },
    grid: { left: '3%', right: '4%', bottom: '3%', top: '50px', containLabel: true },
    dataZoom: [{ type: 'inside', start: 0, end: 100 }, { type: 'slider', start: 0, end: 100, height: 20 }],
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: hourlyUsage.map(u => {
        const d = new Date(u.hour)
        return d.toLocaleString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false })
      }),
      axisLabel: { rotate: 45, fontSize: 10 },
    },
    yAxis: [
      { type: 'value', name: 'Bytes', axisLabel: { formatter: (v) => formatBytes(v) }, splitLine: { lineStyle: { color: '#eee' } } },
      { type: 'value', name: 'Kbps', position: 'right', axisLabel: { formatter: (v) => `${(v / 1000).toFixed(0)}M` }, splitLine: { show: false } },
    ],
    series: [
      {
        name: 'Download', type: 'line', smooth: true, data: hourlyUsage.map(u => u.download_bytes || 0),
        lineStyle: { color: '#10B981', width: 2 }, itemStyle: { color: '#10B981' },
        areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: 'rgba(16,185,129,0.3)' }, { offset: 1, color: 'rgba(16,185,129,0.05)' }] } },
        showSymbol: false, yAxisIndex: 0,
      },
      {
        name: 'Upload', type: 'line', smooth: true, data: hourlyUsage.map(u => u.upload_bytes || 0),
        lineStyle: { color: '#3B82F6', width: 2 }, itemStyle: { color: '#3B82F6' },
        areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: 'rgba(59,130,246,0.3)' }, { offset: 1, color: 'rgba(59,130,246,0.05)' }] } },
        showSymbol: false, yAxisIndex: 0,
      },
      {
        name: 'Peak Download', type: 'line', smooth: true, data: hourlyUsage.map(u => u.peak_download_kbps || 0),
        lineStyle: { color: '#10B981', width: 1, type: 'dashed' }, itemStyle: { color: '#10B981' },
        showSymbol: false, yAxisIndex: 1,
      },
      {
        name: 'Peak Upload', type: 'line', smooth: true, data: hourlyUsage.map(u => u.peak_upload_kbps || 0),
        lineStyle: { color: '#3B82F6', width: 1, type: 'dashed' }, itemStyle: { color: '#3B82F6' },
        showSymbol: false, yAxisIndex: 1,
      },
    ],
  }

  const sessionsChartOption = (() => {
    if (!sessions.length) return null
    // Build Gantt-style data
    const onlineData = []
    const categories = []
    sessions.forEach((s, i) => {
      const start = new Date(s.started_at).getTime()
      const end = s.ended_at ? new Date(s.ended_at).getTime() : Date.now()
      categories.push(`#${i + 1}`)
      onlineData.push({
        name: `Session ${i + 1}`,
        value: [i, start, end, s.download_bytes, s.upload_bytes],
        itemStyle: { color: s.ended_at ? '#10B981' : '#3B82F6' },
      })
    })

    return {
      tooltip: {
        formatter: (params) => {
          const v = params.value
          const start = new Date(v[1]).toLocaleString()
          const end = v[2] === Date.now() ? 'Now (Online)' : new Date(v[2]).toLocaleString()
          const dur = formatDuration(Math.round((v[2] - v[1]) / 1000))
          return `<b>${params.name}</b><br/>Start: ${start}<br/>End: ${end}<br/>Duration: ${dur}<br/>DL: ${formatBytes(v[3])}<br/>UL: ${formatBytes(v[4])}`
        },
      },
      grid: { left: '60px', right: '20px', top: '20px', bottom: '40px' },
      xAxis: {
        type: 'time',
        axisLabel: { formatter: (v) => new Date(v).toLocaleString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false }) },
      },
      yAxis: {
        type: 'category',
        data: categories,
        inverse: true,
        axisLabel: { fontSize: 10 },
      },
      series: [{
        type: 'custom',
        renderItem: (params, api) => {
          const catIdx = api.value(0)
          const start = api.coord([api.value(1), catIdx])
          const end = api.coord([api.value(2), catIdx])
          const height = api.size([0, 1])[1] * 0.6
          return {
            type: 'rect',
            shape: { x: start[0], y: start[1] - height / 2, width: Math.max(end[0] - start[0], 2), height },
            style: api.style(),
          }
        },
        encode: { x: [1, 2], y: 0 },
        data: onlineData,
      }],
    }
  })()

  const heatmapChartOption = (() => {
    if (!heatmap.length) return null
    const dayNames = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
    const hours = Array.from({ length: 24 }, (_, i) => `${i}:00`)

    const data = heatmap.map(h => {
      let val = 0
      if (heatmapMode === 'download') val = h.download_bytes || 0
      else if (heatmapMode === 'upload') val = h.upload_bytes || 0
      else val = (h.download_bytes || 0) + (h.upload_bytes || 0)
      return [h.hour, h.day, val]
    })

    const maxVal = Math.max(...data.map(d => d[2]), 1)

    return {
      tooltip: {
        formatter: (params) => {
          const [hour, day, val] = params.value
          return `${dayNames[day]} ${hours[hour]}<br/>${heatmapMode === 'combined' ? 'Total' : heatmapMode}: ${formatBytes(val)}`
        },
      },
      grid: { left: '60px', right: '80px', top: '20px', bottom: '40px' },
      xAxis: {
        type: 'category',
        data: hours,
        splitArea: { show: true },
        axisLabel: { fontSize: 10 },
      },
      yAxis: {
        type: 'category',
        data: dayNames,
        splitArea: { show: true },
      },
      visualMap: {
        min: 0,
        max: maxVal,
        calculable: true,
        orient: 'vertical',
        right: '0',
        top: 'center',
        inRange: {
          color: heatmapMode === 'upload' ? ['#EFF6FF', '#3B82F6', '#1E40AF'] : ['#ECFDF5', '#10B981', '#065F46'],
        },
        formatter: (v) => formatBytes(v),
      },
      series: [{
        type: 'heatmap',
        data: data,
        label: { show: false },
        emphasis: { itemStyle: { shadowBlur: 10, shadowColor: 'rgba(0,0,0,0.5)' } },
      }],
    }
  })()

  const usageChartOption = {
    tooltip: { trigger: 'axis' },
    legend: { data: ['Download', 'Upload'], top: 0 },
    grid: { left: '3%', right: '4%', bottom: '3%', top: '40px', containLabel: true },
    xAxis: { type: 'category', data: usageHistory.map(u => u.date) },
    yAxis: { type: 'value', axisLabel: { formatter: (v) => formatBytes(v) } },
    series: [
      { name: 'Download', type: 'bar', data: usageHistory.map(u => u.download_bytes || 0), color: '#10B981' },
      { name: 'Upload', type: 'bar', data: usageHistory.map(u => u.upload_bytes || 0), color: '#3B82F6' },
    ],
  }

  if (isLoading) {
    return <div className="flex items-center justify-center h-64"><div className="animate-spin h-8 w-8 border-b-2 border-primary-600"></div></div>
  }

  if (!customer) {
    return <div className="text-center py-12 text-gray-500 dark:text-gray-400">Customer not found</div>
  }

  // Compute public IP display: show subnet notation like "109.110.185.8/29 (5 IPs)" instead of listing all
  const publicIPDisplay = (() => {
    if (!customer.public_ip) return null
    const ips = customer.public_ip.split(',').map(ip => ip.trim()).filter(Boolean)
    if (ips.length === 0) return null
    if (ips.length === 1) return { label: ips[0], count: 1, subnet: customer.public_subnet || '' }
    // Multiple IPs — find common prefix and show as subnet
    const parts0 = ips[0].split('.')
    const prefix = parts0.slice(0, 3).join('.')
    const firstOctet = Math.min(...ips.map(ip => parseInt(ip.split('.')[3]) || 0))
    const subnet = customer.public_subnet || `/${Math.max(28, 32 - Math.ceil(Math.log2(ips.length + 2)))}`
    return { label: `${prefix}.${firstOctet}${subnet}`, count: ips.length, subnet, ips }
  })()

  return (
    <div className="space-y-4">
      {/* Header Card */}
      <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-5">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            {/* Status indicator */}
            <div className={`w-12 h-12 rounded-xl flex items-center justify-center ${customer.is_online ? 'bg-green-100 dark:bg-green-900/30' : 'bg-gray-100 dark:bg-gray-700'}`}>
              <div className={`w-4 h-4 rounded-full ${customer.is_online ? 'bg-green-500 animate-pulse' : 'bg-gray-400'}`} />
            </div>
            <div>
              <h1 className="text-xl font-bold text-gray-900 dark:text-white">{customer.name}</h1>
              <div className="flex items-center gap-2 mt-0.5">
                <span className="text-sm font-mono text-gray-500 dark:text-gray-400">{customer.ip_address}</span>
                {customer.vlan_id > 0 && <span className="px-1.5 py-0.5 text-xs font-medium bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400 rounded">VLAN {customer.vlan_id}</span>}
                {customer.is_online ? (
                  <span className="px-1.5 py-0.5 text-xs font-medium bg-green-50 text-green-700 dark:bg-green-900/30 dark:text-green-400 rounded">Online</span>
                ) : (
                  <span className="px-1.5 py-0.5 text-xs font-medium bg-gray-50 text-gray-500 dark:bg-gray-700 dark:text-gray-400 rounded">Offline</span>
                )}
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <span className={`px-3 py-1.5 text-sm rounded-lg font-semibold ${statusColors[customer.status] || 'bg-gray-100 text-gray-800'}`}>
              {customer.status?.charAt(0).toUpperCase() + customer.status?.slice(1)}
            </span>
            {customer.fup_level > 0 && (
              <span className="px-2.5 py-1.5 text-xs font-bold bg-orange-500 text-white rounded-lg">FUP {customer.fup_level}</span>
            )}
            {hasPermission('bandwidth_customers.edit') && (
              <Link to={`/bandwidth-manager/${id}/edit`} className="px-3 py-1.5 text-sm font-medium rounded-lg bg-blue-600 text-white hover:bg-blue-700 transition-colors">Edit</Link>
            )}
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700">
        <nav className="flex gap-0 overflow-x-auto px-2 pt-2">
          {TABS.map(tab => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`px-5 py-2.5 text-sm font-medium whitespace-nowrap rounded-t-lg transition-colors ${
                activeTab === tab.id
                  ? 'bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-400 border-b-2 border-blue-600'
                  : 'text-gray-500 hover:text-gray-700 hover:bg-gray-50 dark:text-gray-400 dark:hover:text-gray-300 dark:hover:bg-gray-700/50'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </nav>
      </div>

      {/* Tab Content */}
      {activeTab === 'overview' && (
        <div className="space-y-4">
          {/* Stat Cards — colored top borders */}
          <div className="grid grid-cols-2 lg:grid-cols-5 gap-3">
            <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-4 border-t-4 border-t-blue-500">
              <div className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">Speed</div>
              <div className="text-xl font-bold text-gray-900 dark:text-white mt-1">
                <span className="text-green-600">{formatSpeed(customer.download_speed)}</span>
                <span className="text-gray-400 mx-0.5">/</span>
                <span className="text-blue-600">{formatSpeed(customer.upload_speed)}</span>
              </div>
              {customer.cdn_download_speed > 0 && <div className="text-xs text-gray-400 dark:text-gray-500 mt-1">CDN: {formatSpeed(customer.cdn_download_speed)}/{formatSpeed(customer.cdn_upload_speed)}</div>}
            </div>
            <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-4 border-t-4 border-t-green-500">
              <div className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">Daily Used</div>
              <div className="text-xl font-bold text-green-600 mt-1">{formatBytes(customer.daily_download_used)}</div>
              <div className="text-xs text-blue-500 mt-1">Up: {formatBytes(customer.daily_upload_used)}</div>
            </div>
            <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-4 border-t-4 border-t-emerald-500">
              <div className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">Monthly Used</div>
              <div className="text-xl font-bold text-emerald-600 mt-1">{formatBytes(customer.monthly_download_used)}</div>
              <div className="text-xs text-blue-500 mt-1">Up: {formatBytes(customer.monthly_upload_used)}</div>
            </div>
            <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-4 border-t-4 border-t-purple-500">
              <div className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">Price</div>
              <div className="text-xl font-bold text-purple-600 mt-1">${customer.price || 0}</div>
              <div className="text-xs text-gray-400 dark:text-gray-500 capitalize mt-1">{customer.billing_cycle || 'monthly'}</div>
            </div>
            {publicIPDisplay && (
              <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-4 border-t-4 border-t-amber-500">
                <div className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">Public IP</div>
                <div className="text-base font-bold font-mono text-amber-600 mt-1">{publicIPDisplay.label}</div>
                {publicIPDisplay.count > 1 && (
                  <div className="text-xs text-gray-400 dark:text-gray-500 mt-1">{publicIPDisplay.count} IPs assigned</div>
                )}
              </div>
            )}
          </div>

          {/* Two-column layout: Connection + Customer Info */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            {/* Connection Details */}
            <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700">
              <div className="px-4 py-3 border-b border-gray-100 dark:border-gray-700">
                <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Connection Details</h3>
              </div>
              <div className="px-4 py-3 space-y-2.5">
                {[
                  { label: 'IP Address', value: customer.ip_address, mono: true },
                  { label: 'Subnet Mask', value: customer.subnet_mask },
                  { label: 'Gateway', value: customer.gateway, mono: true },
                  { label: 'NAS / Router', value: customer.nas?.name || (customer.nas_id ? `ID: ${customer.nas_id}` : null) },
                  { label: 'Interface', value: customer.interface_name },
                  { label: 'Queue Name', value: customer.queue_name, mono: true },
                  { label: 'Speed Source', value: customer.speed_source, capitalize: true },
                  { label: 'VLAN', value: customer.vlan_id > 0 ? customer.vlan_id : null },
                ].filter(r => r.value).map(row => (
                  <div key={row.label} className="flex items-center justify-between text-sm">
                    <span className="text-gray-500 dark:text-gray-400">{row.label}</span>
                    <span className={`text-gray-900 dark:text-white ${row.mono ? 'font-mono' : ''} ${row.capitalize ? 'capitalize' : ''}`}>{row.value}</span>
                  </div>
                ))}
              </div>
            </div>

            {/* Customer Info + Public IP */}
            <div className="space-y-4">
              {/* Customer Info */}
              {(customer.contact_person || customer.phone || customer.email || customer.address) && (
                <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700">
                  <div className="px-4 py-3 border-b border-gray-100 dark:border-gray-700">
                    <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Customer Info</h3>
                  </div>
                  <div className="px-4 py-3 space-y-2.5">
                    {[
                      { label: 'Contact Person', value: customer.contact_person },
                      { label: 'Phone', value: customer.phone },
                      { label: 'Email', value: customer.email },
                      { label: 'Address', value: customer.address },
                    ].filter(r => r.value).map(row => (
                      <div key={row.label} className="flex items-center justify-between text-sm">
                        <span className="text-gray-500 dark:text-gray-400">{row.label}</span>
                        <span className="text-gray-900 dark:text-white">{row.value}</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Public IP Block Card */}
              {publicIPDisplay && (
                <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700">
                  <div className="px-4 py-3 border-b border-gray-100 dark:border-gray-700">
                    <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Public IP Block</h3>
                  </div>
                  <div className="px-4 py-3 space-y-2.5">
                    <div className="flex items-center justify-between text-sm">
                      <span className="text-gray-500 dark:text-gray-400">Subnet</span>
                      <span className="font-mono font-semibold text-amber-600 dark:text-amber-400">{publicIPDisplay.label}</span>
                    </div>
                    {publicIPDisplay.count > 1 && (
                      <div className="flex items-center justify-between text-sm">
                        <span className="text-gray-500 dark:text-gray-400">IP Count</span>
                        <span className="text-gray-900 dark:text-white">{publicIPDisplay.count} IPs</span>
                      </div>
                    )}
                    {customer.public_gateway && (
                      <div className="flex items-center justify-between text-sm">
                        <span className="text-gray-500 dark:text-gray-400">Gateway</span>
                        <span className="font-mono text-gray-900 dark:text-white">{customer.public_gateway}</span>
                      </div>
                    )}
                    {publicIPDisplay.ips && publicIPDisplay.ips.length > 1 && (
                      <div className="pt-2 border-t border-gray-100 dark:border-gray-700">
                        <div className="text-xs text-gray-500 dark:text-gray-400 mb-1.5">Assigned IPs</div>
                        <div className="flex flex-wrap gap-1.5">
                          {publicIPDisplay.ips.map(ip => (
                            <span key={ip} className="px-2 py-0.5 text-xs font-mono bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-400 rounded-md border border-amber-200 dark:border-amber-800">
                              .{ip.split('.')[3]}
                            </span>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Action Buttons */}
          <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-4">
            <div className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-3">Actions</div>
            <div className="flex flex-wrap gap-2">
              {customer.status === 'active' && hasPermission('bandwidth_customers.suspend') && (
                <button onClick={() => suspendMutation.mutate()} disabled={suspendMutation.isLoading} className="px-4 py-2 text-sm font-medium rounded-lg bg-red-600 text-white hover:bg-red-700 transition-colors disabled:opacity-50">
                  {suspendMutation.isLoading ? 'Suspending...' : 'Suspend'}
                </button>
              )}
              {customer.status === 'suspended' && hasPermission('bandwidth_customers.suspend') && (
                <button onClick={() => unsuspendMutation.mutate()} disabled={unsuspendMutation.isLoading} className="px-4 py-2 text-sm font-medium rounded-lg bg-green-600 text-white hover:bg-green-700 transition-colors disabled:opacity-50">
                  {unsuspendMutation.isLoading ? 'Unsuspending...' : 'Unsuspend'}
                </button>
              )}
              {hasPermission('bandwidth_customers.reset_fup') && customer.fup_level > 0 && (
                <button onClick={() => resetFupMutation.mutate()} disabled={resetFupMutation.isLoading} className="px-4 py-2 text-sm font-medium rounded-lg bg-orange-500 text-white hover:bg-orange-600 transition-colors disabled:opacity-50">
                  {resetFupMutation.isLoading ? 'Resetting...' : 'Reset FUP'}
                </button>
              )}
              {hasPermission('bandwidth_customers.change_speed') && (
                <button onClick={() => { setNewSpeed({ download: customer.download_speed, upload: customer.upload_speed }); setShowSpeedModal(true) }} className="px-4 py-2 text-sm font-medium rounded-lg bg-blue-600 text-white hover:bg-blue-700 transition-colors">
                  Change Speed
                </button>
              )}
              {hasPermission('bandwidth_customers.delete') && (
                <button onClick={() => setShowDeleteConfirm(true)} className="px-4 py-2 text-sm font-medium rounded-lg bg-gray-200 text-gray-700 hover:bg-gray-300 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600 transition-colors">
                  Delete
                </button>
              )}
            </div>
          </div>

          {/* Daily Usage Chart */}
          {usageHistory.length > 0 && (
            <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-4">
              <h3 className="text-sm font-semibold text-gray-900 dark:text-white mb-3">Daily Usage (Last 30 Days)</h3>
              <Suspense fallback={<div className="h-64 flex items-center justify-center text-gray-400">Loading chart...</div>}>
                <ReactECharts option={usageChartOption} style={{ height: '300px' }} />
              </Suspense>
            </div>
          )}
        </div>
      )}

      {activeTab === 'live' && (
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
          <div className="flex items-center justify-between mb-3">
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Live Bandwidth</h3>
            <div className="flex items-center gap-3">
              {graphActive && (
                <div className="flex items-center gap-3 text-xs">
                  <span className="text-green-600">DL: {currentBandwidth.download} Mbps</span>
                  <span className="text-blue-600">UL: {currentBandwidth.upload} Mbps</span>
                </div>
              )}
              <button onClick={() => setGraphActive(!graphActive)} className={`btn btn-sm ${graphActive ? 'bg-red-600 text-white hover:bg-red-700' : 'bg-green-600 text-white hover:bg-green-700'}`}>
                {graphActive ? 'Stop' : 'Start'}
              </button>
            </div>
          </div>
          {graphActive ? (
            <Suspense fallback={<div className="h-64 flex items-center justify-center text-gray-400">Loading chart...</div>}>
              <ReactECharts option={liveChartOption} style={{ height: '350px' }} />
            </Suspense>
          ) : (
            <div className="h-64 flex items-center justify-center text-gray-400 dark:text-gray-500">
              Click "Start" to begin live monitoring
            </div>
          )}
        </div>
      )}

      {activeTab === 'historical' && (
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
          <div className="flex items-center justify-between mb-3">
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Hourly Bandwidth History</h3>
            <div className="flex items-center gap-2">
              {[{ label: '24h', val: 1 }, { label: '7d', val: 7 }, { label: '30d', val: 30 }].map(p => (
                <button key={p.val} onClick={() => setHistoryDays(p.val)}
                  className={`px-3 py-1 text-xs rounded-full ${historyDays === p.val ? 'bg-primary-600 text-white' : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'}`}
                >{p.label}</button>
              ))}
              <button onClick={() => exportChart(historicalChartRef, `bandwidth-history-${customer.name}`)}
                className="px-3 py-1 text-xs rounded-full bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600"
                title="Export as PNG"
              >Export PNG</button>
            </div>
          </div>
          {hourlyUsage.length > 0 ? (
            <Suspense fallback={<div className="h-80 flex items-center justify-center text-gray-400">Loading chart...</div>}>
              <ReactECharts ref={historicalChartRef} option={historicalChartOption} style={{ height: '400px' }} />
            </Suspense>
          ) : (
            <div className="h-64 flex items-center justify-center text-gray-400 dark:text-gray-500">
              No historical data available yet. Data is collected every 30 seconds.
            </div>
          )}
        </div>
      )}

      {activeTab === 'sessions' && (
        <div className="space-y-4">
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
            <div className="flex items-center justify-between mb-3">
              <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Session Timeline</h3>
              <div className="flex items-center gap-2">
                {[{ label: '7d', val: 7 }, { label: '30d', val: 30 }, { label: '90d', val: 90 }].map(p => (
                  <button key={p.val} onClick={() => setSessionDays(p.val)}
                    className={`px-3 py-1 text-xs rounded-full ${sessionDays === p.val ? 'bg-primary-600 text-white' : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'}`}
                  >{p.label}</button>
                ))}
                <button onClick={() => exportChart(sessionsChartRef, `sessions-${customer.name}`)}
                  className="px-3 py-1 text-xs rounded-full bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600"
                  title="Export as PNG"
                >Export PNG</button>
              </div>
            </div>
            {sessions.length > 0 && sessionsChartOption ? (
              <Suspense fallback={<div className="h-80 flex items-center justify-center text-gray-400">Loading chart...</div>}>
                <ReactECharts ref={sessionsChartRef} option={sessionsChartOption} style={{ height: Math.max(200, sessions.length * 30 + 60) + 'px' }} />
              </Suspense>
            ) : (
              <div className="h-64 flex items-center justify-center text-gray-400 dark:text-gray-500">
                No session data available yet. Sessions are tracked automatically.
              </div>
            )}
          </div>

          {/* Sessions Table */}
          {sessions.length > 0 && (
            <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                  <thead className="bg-gray-50 dark:bg-gray-700">
                    <tr>
                      <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">#</th>
                      <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Start</th>
                      <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">End</th>
                      <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Duration</th>
                      <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Download</th>
                      <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Upload</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                    {sessions.map((s, i) => (
                      <tr key={s.id || i} className="hover:bg-gray-50 dark:hover:bg-gray-700/50">
                        <td className="px-4 py-2 text-sm text-gray-900 dark:text-white">{i + 1}</td>
                        <td className="px-4 py-2 text-sm text-gray-600 dark:text-gray-300">{formatDateTime(s.started_at)}</td>
                        <td className="px-4 py-2 text-sm">
                          {s.ended_at ? (
                            <span className="text-gray-600 dark:text-gray-300">{formatDateTime(s.ended_at)}</span>
                          ) : (
                            <span className="text-green-600 font-medium">Online</span>
                          )}
                        </td>
                        <td className="px-4 py-2 text-sm text-gray-900 dark:text-white">{formatDuration(s.duration_sec || (s.ended_at ? Math.round((new Date(s.ended_at) - new Date(s.started_at)) / 1000) : Math.round((Date.now() - new Date(s.started_at).getTime()) / 1000)))}</td>
                        <td className="px-4 py-2 text-sm text-green-600">{formatBytes(s.download_bytes)}</td>
                        <td className="px-4 py-2 text-sm text-blue-600">{formatBytes(s.upload_bytes)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </div>
      )}

      {activeTab === 'heatmap' && (
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
          <div className="flex items-center justify-between mb-3">
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Usage Heatmap (Day x Hour)</h3>
            <div className="flex items-center gap-2">
              {['download', 'upload', 'combined'].map(m => (
                <button key={m} onClick={() => setHeatmapMode(m)}
                  className={`px-3 py-1 text-xs rounded-full capitalize ${heatmapMode === m ? 'bg-primary-600 text-white' : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'}`}
                >{m}</button>
              ))}
              <span className="text-gray-300 dark:text-gray-600">|</span>
              {[{ label: '7d', val: 7 }, { label: '30d', val: 30 }, { label: '90d', val: 90 }].map(p => (
                <button key={p.val} onClick={() => setHeatmapDays(p.val)}
                  className={`px-3 py-1 text-xs rounded-full ${heatmapDays === p.val ? 'bg-primary-600 text-white' : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'}`}
                >{p.label}</button>
              ))}
              <button onClick={() => exportChart(heatmapChartRef, `heatmap-${customer.name}`)}
                className="px-3 py-1 text-xs rounded-full bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600"
                title="Export as PNG"
              >Export PNG</button>
            </div>
          </div>
          {heatmap.length > 0 && heatmapChartOption ? (
            <Suspense fallback={<div className="h-80 flex items-center justify-center text-gray-400">Loading chart...</div>}>
              <ReactECharts ref={heatmapChartRef} option={heatmapChartOption} style={{ height: '350px' }} />
            </Suspense>
          ) : (
            <div className="h-64 flex items-center justify-center text-gray-400 dark:text-gray-500">
              No heatmap data available yet. Data is collected every 30 seconds.
            </div>
          )}
        </div>
      )}

      {/* Change Speed Modal */}
      {showSpeedModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setShowSpeedModal(false)}>
          <div className="bg-white dark:bg-gray-800 rounded-lg p-6 w-full max-w-sm" onClick={(e) => e.stopPropagation()}>
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Change Speed</h3>
            <div className="space-y-3">
              <div>
                <label className="label">Download Speed (kb)</label>
                <input type="number" value={newSpeed.download} onChange={(e) => setNewSpeed(prev => ({ ...prev, download: Number(e.target.value) }))} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" min="0" />
              </div>
              <div>
                <label className="label">Upload Speed (kb)</label>
                <input type="number" value={newSpeed.upload} onChange={(e) => setNewSpeed(prev => ({ ...prev, upload: Number(e.target.value) }))} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" min="0" />
              </div>
            </div>
            <div className="flex justify-end gap-2 mt-4">
              <button onClick={() => setShowSpeedModal(false)} className="btn">Cancel</button>
              <button
                onClick={() => changeSpeedMutation.mutate({ download_speed: newSpeed.download, upload_speed: newSpeed.upload })}
                disabled={changeSpeedMutation.isLoading}
                className="btn btn-primary"
              >
                {changeSpeedMutation.isLoading ? 'Applying...' : 'Apply'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete Confirmation */}
      {showDeleteConfirm && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setShowDeleteConfirm(false)}>
          <div className="bg-white dark:bg-gray-800 rounded-lg p-6 w-full max-w-sm" onClick={(e) => e.stopPropagation()}>
            <h3 className="text-lg font-semibold text-red-600 mb-2">Delete Customer</h3>
            <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
              Are you sure you want to delete <strong>{customer.name}</strong>? This will also remove the MikroTik queue.
            </p>
            <div className="flex justify-end gap-2">
              <button onClick={() => setShowDeleteConfirm(false)} className="btn">Cancel</button>
              <button onClick={() => deleteMutation.mutate()} disabled={deleteMutation.isLoading} className="btn bg-red-600 text-white hover:bg-red-700">
                {deleteMutation.isLoading ? 'Deleting...' : 'Delete'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
