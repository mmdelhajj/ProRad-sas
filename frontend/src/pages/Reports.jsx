import { useState, lazy, Suspense, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import api, { reportApi, dashboardApi } from '../services/api'
import { formatDate } from '../utils/timezone'
import { ArrowTrendingUpIcon, ArrowTrendingDownIcon, ArrowDownTrayIcon } from '@heroicons/react/24/outline'

const ReactECharts = lazy(() => import('echarts-for-react'))

const typeLabels = {
  renewal: 'Renewals',
  new: 'New Subscriptions',
  prepaid_card: 'Prepaid Cards',
  addon: 'Add-ons',
  refill: 'Refills',
  static_ip: 'Static IP',
  change_service: 'Service Changes',
  reset_fup: 'FUP Resets',
  rename: 'Renames',
}

export default function Reports() {
  const [activeTab, setActiveTab] = useState('subscribers')
  const [period, setPeriod] = useState('month')
  const [customFrom, setCustomFrom] = useState('')
  const [customTo, setCustomTo] = useState('')

  const { data: subscriberStats } = useQuery({
    queryKey: ['reports', 'subscribers'],
    queryFn: () => api.get('/reports/subscribers').then(res => res.data.data)
  })

  const revenueParams = useMemo(() => {
    const p = { period }
    if (period === 'custom') {
      if (customFrom) p.date_from = customFrom
      if (customTo) p.date_to = customTo
    }
    return p
  }, [period, customFrom, customTo])

  const { data: revenueStats } = useQuery({
    queryKey: ['reports', 'revenue', revenueParams],
    queryFn: () => api.get('/reports/revenue', { params: revenueParams }).then(res => res.data.data),
    enabled: activeTab === 'revenue'
  })

  const { data: serviceStats } = useQuery({
    queryKey: ['reports', 'services'],
    queryFn: () => api.get('/reports/services').then(res => res.data.data)
  })

  const { data: resellerStats } = useQuery({
    queryKey: ['reports', 'resellers'],
    queryFn: () => api.get('/reports/resellers').then(res => res.data.data)
  })

  const { data: expiryReport } = useQuery({
    queryKey: ['reports', 'expiry'],
    queryFn: () => api.get('/reports/expiry', { params: { days: 7 } }).then(res => res.data.data)
  })

  const { data: forecastData } = useQuery({
    queryKey: ['reports', 'forecast'],
    queryFn: () => reportApi.getRevenueForecast().then(res => res.data.data),
    enabled: activeTab === 'forecast'
  })

  const { data: resellerPerfData } = useQuery({
    queryKey: ['reports', 'reseller-performance'],
    queryFn: () => reportApi.getResellerPerformance().then(res => res.data.data),
    enabled: activeTab === 'reseller_kpis'
  })

  const { data: heatmapRaw } = useQuery({
    queryKey: ['reports', 'heatmap'],
    queryFn: () => dashboardApi.getBandwidthHeatmap().then(res => res.data.data),
    enabled: activeTab === 'heatmap'
  })

  const { data: churnData } = useQuery({
    queryKey: ['reports', 'churn'],
    queryFn: () => reportApi.getChurnReport().then(res => res.data.data),
    enabled: activeTab === 'churn'
  })

  const tabs = [
    { id: 'subscribers', label: 'Subscribers' },
    { id: 'revenue', label: 'Revenue' },
    { id: 'services', label: 'Services' },
    { id: 'resellers', label: 'Resellers' },
    { id: 'expiry', label: 'Expiry Report' },
    { id: 'forecast', label: 'Forecast' },
    { id: 'reseller_kpis', label: 'Reseller KPIs' },
    { id: 'heatmap', label: 'Bandwidth Heatmap' },
    { id: 'churn', label: 'Churn Prediction' },
  ]

  const formatBytes = (bytes) => {
    if (!bytes) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
  }

  const fmtMoney = (v) => {
    if (v == null) return '$0.00'
    return '$' + Number(v).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })
  }

  const ChangeIndicator = ({ change }) => {
    if (change == null || change === 0) return null
    const up = change > 0
    return (
      <span className={`inline-flex items-center gap-0.5 text-[11px] font-medium ${up ? 'text-[#4CAF50]' : 'text-[#f44336]'}`}>
        {up ? <ArrowTrendingUpIcon className="w-3 h-3" /> : <ArrowTrendingDownIcon className="w-3 h-3" />}
        {Math.abs(change).toFixed(1)}%
      </span>
    )
  }

  const handleExportCSV = () => {
    if (!revenueStats?.by_type?.length) return
    const headers = ['Type', 'Transactions', 'Revenue']
    const rows = revenueStats.by_type.map(r => [
      typeLabels[r.type] || r.type,
      r.count,
      r.amount?.toFixed(2)
    ])
    const csv = [headers, ...rows].map(row => row.map(cell => `"${cell || ''}"`).join(',')).join('\n')
    const blob = new Blob([csv], { type: 'text/csv' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `revenue_${new Date().toISOString().split('T')[0]}.csv`
    a.click()
    URL.revokeObjectURL(url)
  }

  // ECharts options
  const trendChartOption = useMemo(() => {
    if (!revenueStats?.daily_revenue?.length) return null
    return {
      tooltip: {
        trigger: 'axis',
        formatter: (params) => {
          const p = params[0]
          return `${p.name}<br/>Revenue: $${Number(p.value).toLocaleString(undefined, { minimumFractionDigits: 2 })}<br/>Transactions: ${revenueStats.daily_revenue[p.dataIndex]?.count || 0}`
        }
      },
      grid: { left: 60, right: 20, top: 20, bottom: 50 },
      xAxis: {
        type: 'category',
        data: revenueStats.daily_revenue.map(d => d.date),
        axisLabel: { rotate: 45, fontSize: 10, color: '#888' }
      },
      yAxis: {
        type: 'value',
        axisLabel: {
          formatter: (v) => '$' + (v >= 1000 ? (v / 1000).toFixed(1) + 'k' : v),
          fontSize: 10,
          color: '#888'
        },
        splitLine: { lineStyle: { color: '#e0e0e0', type: 'dashed' } }
      },
      series: [{
        type: 'bar',
        data: revenueStats.daily_revenue.map(d => d.amount),
        itemStyle: { color: '#4285F4', borderRadius: [3, 3, 0, 0] },
        barMaxWidth: 30
      }]
    }
  }, [revenueStats?.daily_revenue])

  const typeChartOption = useMemo(() => {
    if (!revenueStats?.by_type?.length) return null
    const sorted = [...revenueStats.by_type].sort((a, b) => a.amount - b.amount)
    return {
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'shadow' },
        formatter: (params) => {
          const p = params[0]
          return `${p.name}<br/>$${Number(p.value).toLocaleString(undefined, { minimumFractionDigits: 2 })}`
        }
      },
      grid: { left: 130, right: 30, top: 10, bottom: 10 },
      xAxis: {
        type: 'value',
        axisLabel: {
          formatter: (v) => '$' + (v >= 1000 ? (v / 1000).toFixed(1) + 'k' : v),
          fontSize: 10,
          color: '#888'
        },
        splitLine: { lineStyle: { color: '#e0e0e0', type: 'dashed' } }
      },
      yAxis: {
        type: 'category',
        data: sorted.map(d => typeLabels[d.type] || d.type),
        axisLabel: { fontSize: 10, color: '#555' }
      },
      series: [{
        type: 'bar',
        data: sorted.map(d => d.amount),
        itemStyle: { color: '#FB8C00', borderRadius: [0, 3, 3, 0] },
        barMaxWidth: 22
      }]
    }
  }, [revenueStats?.by_type])

  const forecastChartOption = useMemo(() => {
    if (!forecastData) return null
    const months = [...(forecastData.history || []).map(h => h.month), ...(forecastData.forecast || []).map(f => f.month)]
    const historyValues = (forecastData.history || []).map(h => h.revenue)
    const forecastValues = new Array((forecastData.history || []).length).fill(null).concat((forecastData.forecast || []).map(f => f.projected))
    return {
      tooltip: { trigger: 'axis', formatter: (params) => params.map(p => `${p.seriesName}: $${Number(p.value || 0).toLocaleString()}`).join('<br/>') },
      legend: { data: ['History', 'Forecast'], bottom: 0 },
      grid: { left: 60, right: 20, top: 20, bottom: 40 },
      xAxis: { type: 'category', data: months, axisLabel: { fontSize: 10, color: '#888' } },
      yAxis: { type: 'value', axisLabel: { formatter: (v) => '$' + (v >= 1000 ? (v/1000).toFixed(0) + 'k' : v), fontSize: 10, color: '#888' } },
      series: [
        { name: 'History', type: 'line', data: historyValues.concat(new Array((forecastData.forecast || []).length).fill(null)), itemStyle: { color: '#4285F4' }, lineStyle: { width: 2 } },
        { name: 'Forecast', type: 'line', data: forecastValues, itemStyle: { color: '#4CAF50' }, lineStyle: { width: 2, type: 'dashed' } }
      ]
    }
  }, [forecastData])

  const heatmapChartOption = useMemo(() => {
    const cells = heatmapRaw?.cells
    if (!cells?.length) return null
    const days = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']
    const hours = Array.from({length: 24}, (_, i) => {
      if (i === 0) return '12 AM'
      if (i < 12) return `${i} AM`
      if (i === 12) return '12 PM'
      return `${i - 12} PM`
    })
    const data = cells.map(d => [d.hour, d.day_of_week, d.sessions || 0])
    const max = Math.max(...data.map(d => d[2]), 1)
    return {
      tooltip: {
        backgroundColor: 'rgba(0,0,0,0.85)',
        borderWidth: 0,
        textStyle: { color: '#fff', fontSize: 12 },
        formatter: (p) => {
          const cell = cells.find(c => c.hour === p.value[0] && c.day_of_week === p.value[1])
          const dl = cell ? (cell.download / 1073741824).toFixed(2) : '0'
          const ul = cell ? (cell.upload / 1073741824).toFixed(2) : '0'
          return `<div style="font-weight:600;margin-bottom:4px">${days[p.value[1]]} at ${hours[p.value[0]]}</div>` +
            `<div>Active Sessions: <b>${p.value[2]}</b></div>` +
            `<div style="color:#60a5fa">Download: <b>${dl} GB</b></div>` +
            `<div style="color:#34d399">Upload: <b>${ul} GB</b></div>`
        }
      },
      grid: { left: 80, right: 20, top: 10, bottom: 60 },
      xAxis: {
        type: 'category',
        data: hours,
        axisLabel: { fontSize: 10, color: '#888', interval: 1, rotate: 45 },
        axisTick: { show: false },
        axisLine: { lineStyle: { color: '#ddd' } },
        splitArea: { show: true, areaStyle: { color: ['rgba(0,0,0,0.01)', 'rgba(0,0,0,0.03)'] } }
      },
      yAxis: {
        type: 'category',
        data: days,
        axisLabel: { fontSize: 11, color: '#555', fontWeight: 500 },
        axisTick: { show: false },
        axisLine: { show: false }
      },
      visualMap: {
        min: 0,
        max,
        calculable: true,
        orient: 'horizontal',
        left: 'center',
        bottom: 0,
        itemWidth: 12,
        itemHeight: 140,
        textStyle: { fontSize: 10, color: '#888' },
        text: ['High', 'Low'],
        inRange: { color: ['#e8edf3', '#a8c4e6', '#5b9bd5', '#2e75b6', '#1a4e8a', '#0d3563'] }
      },
      series: [{
        type: 'heatmap',
        data,
        itemStyle: { borderColor: '#fff', borderWidth: 2, borderRadius: 3 },
        emphasis: { itemStyle: { borderColor: '#333', borderWidth: 2, shadowBlur: 8, shadowColor: 'rgba(0,0,0,0.3)' } },
        label: { show: max > 0, fontSize: 9, color: '#333',
          formatter: (p) => p.value[2] > max * 0.3 ? p.value[2] : ''
        }
      }]
    }
  }, [heatmapRaw])

  return (
    <div style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      <div className="wb-toolbar mb-2">
        <span className="text-[13px] font-semibold">Reports</span>
      </div>

      {/* WinBox Tabs */}
      <div className="flex items-end gap-0 mb-0">
        {tabs.map(tab => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={
              tab.id === activeTab
                ? 'wb-tab active'
                : 'wb-tab'
            }
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab content area */}
      <div className="border border-[#a0a0a0] dark:border-[#555] bg-white dark:bg-[#2b2b2b] p-3" style={{ borderRadius: '0 2px 2px 2px' }}>

        {/* Subscribers Tab */}
        {activeTab === 'subscribers' && subscriberStats && (
          <div className="space-y-3">
            <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
              <div className="wb-group">
                <div className="wb-group-title">Total Subscribers</div>
                <div className="wb-group-body">
                  <div className="text-[20px] font-bold text-gray-900 dark:text-[#e0e0e0]">{subscriberStats.total}</div>
                </div>
              </div>
              <div className="wb-group">
                <div className="wb-group-title">Active</div>
                <div className="wb-group-body">
                  <div className="text-[20px] font-bold text-[#4CAF50]">{subscriberStats.active}</div>
                </div>
              </div>
              <div className="wb-group">
                <div className="wb-group-title">Expired</div>
                <div className="wb-group-body">
                  <div className="text-[20px] font-bold text-[#f44336]">{subscriberStats.expired}</div>
                </div>
              </div>
              <div className="wb-group">
                <div className="wb-group-title">Online Now</div>
                <div className="wb-group-body">
                  <div className="text-[20px] font-bold text-[#2196F3]">{subscriberStats.online}</div>
                </div>
              </div>
            </div>
            <div className="grid grid-cols-2 gap-2">
              <div className="wb-group">
                <div className="wb-group-title">New This Month</div>
                <div className="wb-group-body">
                  <div className="text-[20px] font-bold text-gray-900 dark:text-[#e0e0e0]">{subscriberStats.newThisMonth}</div>
                </div>
              </div>
              <div className="wb-group">
                <div className="wb-group-title">Expiring Soon (7 days)</div>
                <div className="wb-group-body">
                  <div className="text-[20px] font-bold text-[#FF9800]">{subscriberStats.expiringSoon}</div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Revenue Tab */}
        {activeTab === 'revenue' && (
          <div className="space-y-3">
            {/* Period buttons + Export */}
            <div className="flex items-center justify-between flex-wrap gap-2">
              <div className="flex items-center gap-1 flex-wrap">
                {['day', 'week', 'month', 'year', 'custom'].map(p => (
                  <button
                    key={p}
                    onClick={() => setPeriod(p)}
                    className={period === p ? 'btn btn-primary btn-sm' : 'btn btn-sm'}
                  >
                    {p.charAt(0).toUpperCase() + p.slice(1)}
                  </button>
                ))}
                {period === 'custom' && (
                  <>
                    <input
                      type="date"
                      value={customFrom}
                      onChange={e => setCustomFrom(e.target.value)}
                      className="px-2 py-1 border border-[#a0a0a0] dark:border-[#555] bg-white dark:bg-[#333] dark:text-white text-[11px]"
                      style={{ borderRadius: '2px' }}
                    />
                    <span className="text-gray-500 dark:text-gray-400 text-[11px]">to</span>
                    <input
                      type="date"
                      value={customTo}
                      onChange={e => setCustomTo(e.target.value)}
                      className="px-2 py-1 border border-[#a0a0a0] dark:border-[#555] bg-white dark:bg-[#333] dark:text-white text-[11px]"
                      style={{ borderRadius: '2px' }}
                    />
                  </>
                )}
              </div>
              <button
                onClick={handleExportCSV}
                className="btn btn-sm inline-flex items-center gap-1"
                title="Export CSV"
              >
                <ArrowDownTrayIcon className="w-3.5 h-3.5" />
                <span className="hidden sm:inline">Export CSV</span>
              </button>
            </div>

            {revenueStats && (
              <>
                {/* 4 Summary Cards */}
                <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
                  {[
                    { label: 'Today', value: revenueStats.today_revenue, change: revenueStats.today_change, color: '#4CAF50', border: 'border-t-[#4CAF50]' },
                    { label: 'This Week', value: revenueStats.week_revenue, change: revenueStats.week_change, color: '#2196F3', border: 'border-t-[#2196F3]' },
                    { label: 'This Month', value: revenueStats.month_revenue, change: revenueStats.month_change, color: '#FF9800', border: 'border-t-[#FF9800]' },
                    { label: 'This Year', value: revenueStats.year_revenue, change: revenueStats.year_change, color: '#9C27B0', border: 'border-t-[#9C27B0]' },
                  ].map(card => (
                    <div key={card.label} className={`wb-group border-t-2 ${card.border}`}>
                      <div className="wb-group-title">{card.label}</div>
                      <div className="wb-group-body">
                        <div className="text-[18px] font-bold" style={{ color: card.color }}>
                          {fmtMoney(card.value)}
                        </div>
                        <ChangeIndicator change={card.change} />
                      </div>
                    </div>
                  ))}
                </div>

                {/* Transaction count */}
                <div className="text-[11px] text-gray-500 dark:text-gray-400">
                  {revenueStats.transaction_count || 0} transactions in selected period
                </div>

                {/* Revenue Trend Chart */}
                {trendChartOption && (
                  <div className="wb-group">
                    <div className="wb-group-title">Revenue Trend</div>
                    <div className="wb-group-body">
                      <Suspense fallback={<div className="h-[220px] flex items-center justify-center text-gray-400">Loading chart...</div>}>
                        <ReactECharts option={trendChartOption} style={{ height: '220px' }} />
                      </Suspense>
                    </div>
                  </div>
                )}

                {/* Two-column: By Type chart + By Service table */}
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
                  {/* Revenue by Type */}
                  {typeChartOption && (
                    <div className="wb-group">
                      <div className="wb-group-title">Revenue by Type</div>
                      <div className="wb-group-body">
                        <Suspense fallback={<div className="h-[200px] flex items-center justify-center text-gray-400">Loading chart...</div>}>
                          <ReactECharts option={typeChartOption} style={{ height: Math.max(120, (revenueStats.by_type?.length || 1) * 28) + 'px' }} />
                        </Suspense>
                      </div>
                    </div>
                  )}

                  {/* Revenue by Service */}
                  {revenueStats.by_service && revenueStats.by_service.length > 0 && (
                    <div className="wb-group">
                      <div className="wb-group-title">Revenue by Service</div>
                      <div className="wb-group-body p-0">
                        <div className="table-container" style={{ maxHeight: 300, overflowY: 'auto' }}>
                          <table className="table">
                            <thead>
                              <tr>
                                <th>Service</th>
                                <th className="text-right">Transactions</th>
                                <th className="text-right">Revenue</th>
                              </tr>
                            </thead>
                            <tbody>
                              {revenueStats.by_service.map((s, i) => (
                                <tr key={i}>
                                  <td className="font-semibold">{s.service_name}</td>
                                  <td className="text-right">{s.count}</td>
                                  <td className="text-right">{fmtMoney(s.amount)}</td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        </div>
                      </div>
                    </div>
                  )}
                </div>

                {/* Revenue by Reseller */}
                {revenueStats.by_reseller && revenueStats.by_reseller.length > 0 && (
                  <div className="wb-group">
                    <div className="wb-group-title">Revenue by Reseller (Top 20)</div>
                    <div className="wb-group-body p-0">
                      <div className="table-container" style={{ maxHeight: 300, overflowY: 'auto' }}>
                        <table className="table">
                          <thead>
                            <tr>
                              <th>Reseller</th>
                              <th className="text-right">Transactions</th>
                              <th className="text-right">Revenue</th>
                            </tr>
                          </thead>
                          <tbody>
                            {revenueStats.by_reseller.map((r, i) => (
                              <tr key={i}>
                                <td className="font-semibold">{r.reseller_name}</td>
                                <td className="text-right">{r.count}</td>
                                <td className="text-right">{fmtMoney(r.amount)}</td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    </div>
                  </div>
                )}
              </>
            )}
          </div>
        )}

        {/* Services Tab */}
        {activeTab === 'services' && serviceStats && (
          <div className="table-container">
            <table className="table">
              <thead>
                <tr>
                  <th>Service</th>
                  <th>Subscribers</th>
                  <th>Revenue</th>
                </tr>
              </thead>
              <tbody>
                {serviceStats.map(s => (
                  <tr key={s.id}>
                    <td className="font-semibold">{s.name}</td>
                    <td>{s.subscriber_count}</td>
                    <td>${s.revenue?.toFixed(2)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Resellers Tab */}
        {activeTab === 'resellers' && resellerStats && (
          <div className="table-container">
            <table className="table">
              <thead>
                <tr>
                  <th>Reseller</th>
                  <th>Balance</th>
                  <th>Total Subscribers</th>
                  <th>Active</th>
                </tr>
              </thead>
              <tbody>
                {resellerStats.map(r => (
                  <tr key={r.id}>
                    <td className="font-semibold">{r.name}</td>
                    <td>${r.balance?.toFixed(2)}</td>
                    <td>{r.subscriber_count}</td>
                    <td>{r.active_count}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Expiry Report Tab */}
        {activeTab === 'expiry' && expiryReport && (
          <div className="space-y-3">
            <div className="wb-group">
              <div className="wb-group-title">
                {expiryReport.total} subscribers expiring in the next 7 days
              </div>
              <div className="wb-group-body">
                {expiryReport.byDay && (
                  <div className="grid grid-cols-7 gap-1">
                    {expiryReport.byDay.map(d => (
                      <div key={d.day} className="text-center border border-[#ccc] dark:border-[#555] p-2 bg-[#fff8f0] dark:bg-[#3a3020]" style={{ borderRadius: '2px' }}>
                        <div className="text-[16px] font-bold text-[#FF9800]">{d.count}</div>
                        <div className="text-[11px] text-gray-600 dark:text-[#aaa]">Day {d.day}</div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
            {expiryReport.subscribers && expiryReport.subscribers.length > 0 && (
              <div className="table-container">
                <table className="table">
                  <thead>
                    <tr>
                      <th>Username</th>
                      <th>Full Name</th>
                      <th>Service</th>
                      <th>Expiry Date</th>
                    </tr>
                  </thead>
                  <tbody>
                    {expiryReport.subscribers.slice(0, 20).map(s => (
                      <tr key={s.id}>
                        <td className="font-semibold">{s.username}</td>
                        <td>{s.full_name}</td>
                        <td>{s.service?.name}</td>
                        <td>{formatDate(s.expiry_date)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )}

        {/* Forecast Tab */}
        {activeTab === 'forecast' && forecastData && (
          <div className="space-y-3">
            <div className="grid grid-cols-2 gap-2">
              <div className="wb-group border-t-2 border-t-[#4CAF50]">
                <div className="wb-group-title">Growth Rate</div>
                <div className="wb-group-body">
                  <div className="text-[18px] font-bold text-[#4CAF50]">{((forecastData.growth_rate || 0) * 100).toFixed(1)}%</div>
                  <div className="text-[10px] text-gray-500 dark:text-gray-400">Monthly average</div>
                </div>
              </div>
              <div className="wb-group border-t-2 border-t-[#2196F3]">
                <div className="wb-group-title">Next Month Projected</div>
                <div className="wb-group-body">
                  <div className="text-[18px] font-bold text-[#2196F3]">{fmtMoney(forecastData.forecast?.[0]?.projected)}</div>
                </div>
              </div>
            </div>
            {forecastChartOption && (
              <div className="wb-group">
                <div className="wb-group-title">Revenue History & Forecast</div>
                <div className="wb-group-body">
                  <Suspense fallback={<div className="h-[250px] flex items-center justify-center text-gray-400">Loading chart...</div>}>
                    <ReactECharts option={forecastChartOption} style={{ height: '250px' }} />
                  </Suspense>
                </div>
              </div>
            )}
          </div>
        )}

        {/* Reseller KPIs Tab */}
        {activeTab === 'reseller_kpis' && resellerPerfData && (
          <div className="table-container">
            <table className="table">
              <thead>
                <tr>
                  <th>Reseller</th>
                  <th className="text-right">Subscribers</th>
                  <th className="text-right">Active %</th>
                  <th className="text-right">Revenue</th>
                  <th className="text-right">Commission</th>
                  <th className="text-right">Avg Lifetime</th>
                  <th className="text-right">Tickets</th>
                </tr>
              </thead>
              <tbody>
                {(resellerPerfData || []).map((r, i) => (
                  <tr key={i} className={i === 0 ? 'bg-green-50 dark:bg-green-900/20' : ''}>
                    <td className="font-semibold">{r.reseller_name}</td>
                    <td className="text-right">{r.total_subscribers}</td>
                    <td className="text-right">{(r.active_percent || 0).toFixed(1)}%</td>
                    <td className="text-right">{fmtMoney(r.revenue)}</td>
                    <td className="text-right">{fmtMoney(r.commission)}</td>
                    <td className="text-right">{(r.avg_lifetime_days || 0).toFixed(0)}d</td>
                    <td className="text-right">{r.ticket_count || 0}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Heatmap Tab */}
        {activeTab === 'heatmap' && (
          <div className="space-y-3">
            {/* Summary Cards */}
            {heatmapRaw?.summary && (() => {
              const s = heatmapRaw.summary
              const days = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']
              const fmtHour = (h) => h === 0 ? '12 AM' : h < 12 ? `${h} AM` : h === 12 ? '12 PM' : `${h-12} PM`
              return (
                <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
                  <div className="wb-group border-t-2 border-t-[#2e75b6]">
                    <div className="wb-group-title">Peak Sessions</div>
                    <div className="wb-group-body text-center">
                      <div className="text-[20px] font-bold text-[#2e75b6]">{s.peak_sessions}</div>
                      <div className="text-[10px] text-gray-500">{days[s.peak_day]} at {fmtHour(s.peak_hour)}</div>
                    </div>
                  </div>
                  <div className="wb-group border-t-2 border-t-[#4CAF50]">
                    <div className="wb-group-title">Busiest Day</div>
                    <div className="wb-group-body text-center">
                      <div className="text-[16px] font-bold text-[#4CAF50]">{days[s.busiest_day]}</div>
                      <div className="text-[10px] text-gray-500">Most active day of the week</div>
                    </div>
                  </div>
                  <div className="wb-group border-t-2 border-t-[#FF9800]">
                    <div className="wb-group-title">Peak Hour</div>
                    <div className="wb-group-body text-center">
                      <div className="text-[16px] font-bold text-[#FF9800]">{fmtHour(s.busiest_hour)}</div>
                      <div className="text-[10px] text-gray-500">Busiest hour overall</div>
                    </div>
                  </div>
                  <div className="wb-group border-t-2 border-t-[#9C27B0]">
                    <div className="wb-group-title">Total Sessions (7d)</div>
                    <div className="wb-group-body text-center">
                      <div className="text-[20px] font-bold text-[#9C27B0]">{(s.total_sessions || 0).toLocaleString()}</div>
                      <div className="text-[10px] text-gray-500">Across all hours</div>
                    </div>
                  </div>
                </div>
              )
            })()}

            {/* Heatmap Chart */}
            {heatmapChartOption ? (
              <div className="wb-group">
                <div className="wb-group-title">Active Sessions Heatmap — Last 7 Days</div>
                <div className="wb-group-body" style={{ padding: '8px 4px' }}>
                  <Suspense fallback={<div className="h-[360px] flex items-center justify-center text-gray-400">Loading chart...</div>}>
                    <ReactECharts option={heatmapChartOption} style={{ height: '360px' }} />
                  </Suspense>
                </div>
              </div>
            ) : (
              <div className="wb-group">
                <div className="wb-group-title">Active Sessions Heatmap — Last 7 Days</div>
                <div className="wb-group-body">
                  <div className="text-center text-gray-500 dark:text-gray-400 py-12">
                    <div className="text-[14px] mb-1">No session data available</div>
                    <div className="text-[11px]">Heatmap will populate as sessions are recorded</div>
                  </div>
                </div>
              </div>
            )}
          </div>
        )}

        {/* Churn Tab */}
        {activeTab === 'churn' && churnData && (
          <div className="space-y-3">
            <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
              {[
                { label: 'Low Risk', count: churnData.summary?.low || 0, color: '#4CAF50', border: 'border-t-[#4CAF50]' },
                { label: 'Medium Risk', count: churnData.summary?.medium || 0, color: '#FF9800', border: 'border-t-[#FF9800]' },
                { label: 'High Risk', count: churnData.summary?.high || 0, color: '#f44336', border: 'border-t-[#f44336]' },
                { label: 'Critical', count: churnData.summary?.critical || 0, color: '#9C27B0', border: 'border-t-[#9C27B0]' },
              ].map(c => (
                <div key={c.label} className={`wb-group border-t-2 ${c.border}`}>
                  <div className="wb-group-title">{c.label}</div>
                  <div className="wb-group-body">
                    <div className="text-[20px] font-bold" style={{ color: c.color }}>{c.count}</div>
                  </div>
                </div>
              ))}
            </div>
            {churnData.subscribers?.length > 0 && (
              <div className="table-container" style={{ maxHeight: 400, overflowY: 'auto' }}>
                <table className="table">
                  <thead>
                    <tr>
                      <th>Subscriber</th>
                      <th>Score</th>
                      <th>Risk</th>
                      <th>Days to Expiry</th>
                      <th>Usage Trend</th>
                      <th>Tickets</th>
                    </tr>
                  </thead>
                  <tbody>
                    {churnData.subscribers.map((s, i) => (
                      <tr key={i}>
                        <td className="font-semibold">{s.username}</td>
                        <td>
                          <div className="flex items-center gap-1">
                            <div className="w-16 h-2 bg-gray-200 dark:bg-gray-600 rounded-full overflow-hidden">
                              <div className="h-full rounded-full" style={{ width: `${s.score}%`, backgroundColor: s.score > 75 ? '#9C27B0' : s.score > 50 ? '#f44336' : s.score > 25 ? '#FF9800' : '#4CAF50' }} />
                            </div>
                            <span className="text-[10px]">{s.score}</span>
                          </div>
                        </td>
                        <td>
                          <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${
                            s.risk_level === 'critical' ? 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300' :
                            s.risk_level === 'high' ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300' :
                            s.risk_level === 'medium' ? 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300' :
                            'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
                          }`}>{s.risk_level}</span>
                        </td>
                        <td>{s.days_until_expiry}</td>
                        <td>
                          <span className={`text-[10px] ${s.usage_trend === 'declining' ? 'text-red-500' : s.usage_trend === 'growing' ? 'text-green-500' : 'text-gray-500'}`}>
                            {s.usage_trend === 'declining' ? '\u2193' : s.usage_trend === 'growing' ? '\u2191' : '\u2192'} {s.usage_trend}
                          </span>
                        </td>
                        <td>{s.ticket_count}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
