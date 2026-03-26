import { useQuery } from '@tanstack/react-query'
import { dashboardApi } from '../services/api'
import { formatDate } from '../utils/timezone'
import { useBrandingStore } from '../store/brandingStore'
import { useAuthStore } from '../store/authStore'
import { lazy, Suspense, useState } from 'react'
const ReactECharts = lazy(() => import('echarts-for-react'))
import {
  UsersIcon,
  SignalIcon,
  CurrencyDollarIcon,
  ServerIcon,
  ArrowTrendingUpIcon,
  ArrowTrendingDownIcon,
  ClockIcon,
  CpuChipIcon,
  CircleStackIcon,
  ExclamationTriangleIcon,
  CalendarDaysIcon,
  BanknotesIcon,
  UserGroupIcon,
  WifiIcon,
  BookOpenIcon,
  CodeBracketIcon,
  QuestionMarkCircleIcon,
} from '@heroicons/react/24/outline'
import clsx from 'clsx'

function SystemMetricBar({ label, value }) {
  const percentage = value || 0
  const barColor =
    percentage < 50
      ? '#4CAF50'
      : percentage < 80
        ? '#FF9800'
        : '#f44336'

  return (
    <div className="flex items-center gap-2 py-0.5">
      <span className="text-[11px] text-gray-700 dark:text-[#ccc] w-[50px] flex-shrink-0">{label}</span>
      <div className="flex-1 h-[4px] bg-[#e0e0e0] dark:bg-[#555]" style={{ borderRadius: '1px' }}>
        <div
          className="h-full transition-all duration-500"
          style={{
            width: `${Math.min(percentage, 100)}%`,
            backgroundColor: barColor,
            borderRadius: '1px',
          }}
        />
      </div>
      <span className="text-[11px] font-semibold w-[36px] text-right" style={{ color: barColor }}>
        {percentage}%
      </span>
    </div>
  )
}

function StatBox({ label, value, trend, icon: Icon, iconColor }) {
  return (
    <div className="stat-card flex items-center gap-2 px-3 py-2">
      {Icon && (
        <div className="flex-shrink-0">
          <Icon className="w-4 h-4" style={{ color: iconColor || '#4a7ab5' }} />
        </div>
      )}
      <div className="min-w-0">
        <div className="text-[16px] font-bold text-gray-900 dark:text-[#e0e0e0] leading-tight">{value}</div>
        <div className="text-[10px] text-gray-500 dark:text-[#aaa]">{label}</div>
        {trend !== undefined && (
          <div className={clsx('flex items-center mt-0.5 text-[10px]', trend >= 0 ? 'text-[#4CAF50]' : 'text-[#f44336]')}>
            {trend >= 0 ? (
              <ArrowTrendingUpIcon className="w-3 h-3 mr-0.5" />
            ) : (
              <ArrowTrendingDownIcon className="w-3 h-3 mr-0.5" />
            )}
            {Math.abs(trend)}%
          </div>
        )}
      </div>
    </div>
  )
}

export default function Dashboard() {
  const { companyName } = useBrandingStore()
  const { isAdmin, isSaasMode } = useAuthStore()
  const [showConfig, setShowConfig] = useState(false)
  const [configData, setConfigData] = useState(null)
  const [configLoading, setConfigLoading] = useState(false)
  const [copied, setCopied] = useState(false)

  const loadConfig = async () => {
    setConfigLoading(true)
    try {
      const res = await dashboardApi.mikrotikConfig()
      setConfigData(res.data)
      setShowConfig(true)
    } catch (e) {
      console.error('Failed to load config', e)
    }
    setConfigLoading(false)
  }

  const copyScript = (text) => {
    navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const { data: stats, isLoading } = useQuery({
    queryKey: ['dashboard-stats'],
    queryFn: () => dashboardApi.stats().then((r) => r.data.data),
    refetchInterval: 30000, // Refresh every 30 seconds
  })

  const { data: chartData } = useQuery({
    queryKey: ['dashboard-chart'],
    queryFn: () => dashboardApi.chart({ type: 'new_expired', days: 30 }).then((r) => r.data.data),
  })

  const { data: serviceData } = useQuery({
    queryKey: ['dashboard-services'],
    queryFn: () => dashboardApi.chart({ type: 'services' }).then((r) => r.data.data),
  })

  const { data: transactions } = useQuery({
    queryKey: ['dashboard-transactions'],
    queryFn: () => dashboardApi.transactions({ limit: 5 }).then((r) => r.data.data),
  })

  const { data: systemMetrics } = useQuery({
    queryKey: ['dashboard-system-metrics'],
    queryFn: () => dashboardApi.systemMetrics().then((r) => r.data.data),
    refetchInterval: 10000, // Refresh every 10 seconds for real-time monitoring
    enabled: isAdmin() && !isSaasMode,
  })

  const { data: systemCapacityResponse } = useQuery({
    queryKey: ['dashboard-system-capacity'],
    queryFn: () => dashboardApi.systemCapacity().then((r) => r.data),
    refetchInterval: 30000, // Refresh every 30 seconds
    enabled: isAdmin() && !isSaasMode,
  })

  // Don't show capacity on secondary/replica servers
  // API returns { success, is_replica, data } - extract data only for main server
  const systemCapacity = systemCapacityResponse?.is_replica ? null : systemCapacityResponse?.data

  // RADIUS connection status (SaaS mode only)
  const { data: radiusStatus } = useQuery({
    queryKey: ['radius-status'],
    queryFn: () => dashboardApi.radiusStatus().then(r => r.data),
    refetchInterval: 10000,
    enabled: isSaasMode,
  })


  const lineChartOption = {
    tooltip: {
      trigger: 'axis',
      textStyle: { fontSize: 10 },
    },
    legend: {
      data: ['New', 'Expired'],
      textStyle: { fontSize: 10 },
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      containLabel: true,
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: chartData?.new?.map((d) => d.date) || [],
    },
    yAxis: {
      type: 'value',
    },
    series: [
      {
        name: 'New',
        type: 'line',
        smooth: true,
        data: chartData?.new?.map((d) => d.count) || [],
        lineStyle: { color: '#10B981' },
        itemStyle: { color: '#10B981' },
        areaStyle: { color: 'rgba(16, 185, 129, 0.1)' },
      },
      {
        name: 'Expired',
        type: 'line',
        smooth: true,
        data: chartData?.expired?.map((d) => d.count) || [],
        lineStyle: { color: '#EF4444' },
        itemStyle: { color: '#EF4444' },
        areaStyle: { color: 'rgba(239, 68, 68, 0.1)' },
      },
    ],
  }

  const pieChartOption = {
    tooltip: {
      trigger: 'item',
    },
    legend: {
      orient: 'vertical',
      left: 'left',
      textStyle: { fontSize: 10 },
    },
    series: [
      {
        name: 'Subscribers',
        type: 'pie',
        radius: ['40%', '70%'],
        avoidLabelOverlap: false,
        itemStyle: {
          borderRadius: 2,
          borderColor: '#fff',
          borderWidth: 1,
        },
        label: {
          show: false,
        },
        emphasis: {
          label: {
            show: true,
            fontSize: 11,
            fontWeight: 'bold',
          },
        },
        data: serviceData?.map((s) => ({ value: s.count, name: s.name })) || [],
      },
    ],
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
        <div className="text-[11px] text-gray-500 dark:text-[#aaa]">Loading dashboard...</div>
      </div>
    )
  }

  return (
    <div className="space-y-2" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Page Header */}
      <div className="wb-toolbar">
        <span className="text-[13px] font-semibold">Dashboard - {companyName || 'ISP'} Management System</span>
      </div>

      {/* RADIUS Connection Status - SaaS mode only */}
      {isSaasMode && (
        <div className={clsx(
          'wb-group p-3 flex items-center gap-3 border-l-4',
          radiusStatus?.status === 'connected' ? 'border-green-500' :
          radiusStatus?.status === 'configured' ? 'border-blue-500' :
          radiusStatus?.status === 'ready' ? 'border-yellow-500' :
          radiusStatus?.status === 'waiting' ? 'border-yellow-500' :
          'border-red-500'
        )}>
          <SignalIcon className={clsx('w-5 h-5',
            radiusStatus?.status === 'connected' ? 'text-green-500' :
            radiusStatus?.status === 'configured' ? 'text-blue-500' :
            radiusStatus?.status === 'ready' || radiusStatus?.status === 'waiting' ? 'text-yellow-500' :
            'text-red-500'
          )} />
          <div className="flex-1">
            <div className="text-[12px] font-semibold text-gray-900 dark:text-[#e0e0e0]">
              {radiusStatus?.status === 'connected'
                ? `RADIUS Connected \u2014 ${radiusStatus.active_sessions} active session${radiusStatus.active_sessions !== 1 ? 's' : ''}`
                : radiusStatus?.status === 'configured'
                ? 'RADIUS Configured \u2014 No active sessions'
                : radiusStatus?.status === 'ready'
                ? 'Ready \u2014 Create a subscriber and connect via PPPoE'
                : radiusStatus?.status === 'waiting'
                ? 'Waiting \u2014 Create a subscriber and connect via PPPoE'
                : 'RADIUS Not Connected'}
            </div>
            {radiusStatus?.status === 'not_configured' && (
              <div className="text-[10px] text-gray-500 dark:text-[#aaa]">
                Run the MikroTik script from your welcome email to connect
              </div>
            )}
            {radiusStatus?.nas_configured && radiusStatus?.connected && (
              <div className="text-[10px] text-gray-500 dark:text-[#aaa]">
                NAS IP: {radiusStatus.nas_ip}
              </div>
            )}
          </div>
          {/* Router VPN Status */}
          <div className={clsx(
            'flex items-center gap-1.5 px-2.5 py-1 rounded text-[11px] font-medium',
            radiusStatus?.router_online
              ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
              : 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
          )}>
            <span className={clsx(
              'w-2 h-2 rounded-full',
              radiusStatus?.router_online ? 'bg-green-500 animate-pulse' : 'bg-red-500'
            )} />
            {radiusStatus?.router_online ? 'Router Online' : 'Router Offline'}
          </div>
          {/* MikroTik Config Button */}
          <button
            onClick={loadConfig}
            disabled={configLoading}
            className="flex items-center gap-1.5 px-2.5 py-1 rounded text-[11px] font-medium bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400 hover:bg-blue-200 dark:hover:bg-blue-900/50 transition-colors"
          >
            <CodeBracketIcon className="w-3.5 h-3.5" />
            {configLoading ? 'Loading...' : 'MikroTik Config'}
          </button>
        </div>
      )}

      {/* MikroTik Config Modal */}
      {showConfig && configData && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => setShowConfig(false)}>
          <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-2xl mx-4 max-h-[80vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
            <div className="flex items-center justify-between p-4 border-b border-gray-200 dark:border-gray-700">
              <h3 className="text-[14px] font-semibold text-gray-900 dark:text-white">MikroTik Configuration</h3>
              <button onClick={() => setShowConfig(false)} className="text-gray-500 hover:text-gray-700 dark:hover:text-gray-300 text-xl">&times;</button>
            </div>
            <div className="p-4 space-y-4">
              {configData.mikrotik_script && (
                <div>
                  <div className="flex items-center justify-between mb-2">
                    <label className="text-[12px] font-semibold text-gray-700 dark:text-gray-300">Full Setup Script (VPN + RADIUS)</label>
                    <button
                      onClick={() => copyScript(configData.mikrotik_script)}
                      className="text-[11px] px-2 py-0.5 rounded bg-blue-500 text-white hover:bg-blue-600"
                    >
                      {copied ? 'Copied!' : 'Copy'}
                    </button>
                  </div>
                  <p className="text-[10px] text-gray-500 dark:text-gray-400 mb-2">
                    Paste this entire command in your MikroTik terminal. It configures VPN + RADIUS automatically.
                  </p>
                  <pre className="bg-gray-100 dark:bg-gray-900 p-3 rounded text-[10px] text-gray-800 dark:text-gray-200 overflow-x-auto whitespace-pre-wrap break-all font-mono leading-relaxed">
                    {configData.mikrotik_script}
                  </pre>
                </div>
              )}
              {configData.wg_client_ip && (
                <div className="grid grid-cols-2 gap-2 text-[11px]">
                  <div className="bg-gray-50 dark:bg-gray-700/50 p-2 rounded">
                    <span className="text-gray-500 dark:text-gray-400">VPN IP:</span>{' '}
                    <span className="font-mono font-medium text-gray-900 dark:text-white">{configData.wg_client_ip}</span>
                  </div>
                  <div className="bg-gray-50 dark:bg-gray-700/50 p-2 rounded">
                    <span className="text-gray-500 dark:text-gray-400">Server IP:</span>{' '}
                    <span className="font-mono font-medium text-gray-900 dark:text-white">{configData.wg_server_ip}</span>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Trial / Plan Info - SaaS mode only */}
      {isSaasMode && radiusStatus && (
        <div className="wb-group p-3">
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            {/* Trial Status */}
            <div className={clsx(
              'flex items-center gap-3 p-2.5 rounded border',
              radiusStatus.tenant_status === 'trial'
                ? radiusStatus.trial_days_left <= 3
                  ? 'border-red-300 bg-red-50 dark:border-red-800 dark:bg-red-900/20'
                  : 'border-yellow-300 bg-yellow-50 dark:border-yellow-800 dark:bg-yellow-900/20'
                : 'border-green-300 bg-green-50 dark:border-green-800 dark:bg-green-900/20'
            )}>
              <ClockIcon className={clsx('w-5 h-5 flex-shrink-0',
                radiusStatus.tenant_status === 'trial'
                  ? radiusStatus.trial_days_left <= 3 ? 'text-red-500' : 'text-yellow-500'
                  : 'text-green-500'
              )} />
              <div>
                <div className="text-[12px] font-semibold text-gray-900 dark:text-[#e0e0e0]">
                  {radiusStatus.tenant_status === 'trial'
                    ? radiusStatus.trial_days_left > 0
                      ? `Trial \u2014 ${radiusStatus.trial_days_left} day${radiusStatus.trial_days_left !== 1 ? 's' : ''} left`
                      : 'Trial Expired'
                    : radiusStatus.tenant_status === 'active'
                    ? 'Active Plan'
                    : radiusStatus.tenant_status || 'Unknown'}
                </div>
                <div className="text-[10px] text-gray-500 dark:text-[#aaa]">
                  {radiusStatus.tenant_status === 'trial'
                    ? radiusStatus.trial_days_left > 0
                      ? 'Upgrade to keep your panel after trial ends'
                      : 'Your panel is read-only. Upgrade to continue.'
                    : 'Your subscription is active'}
                </div>
              </div>
            </div>

            {/* Subscriber Limit */}
            <div className="flex items-center gap-3 p-2.5 rounded border border-gray-200 bg-gray-50 dark:border-gray-700 dark:bg-gray-800">
              <UsersIcon className="w-5 h-5 text-blue-500 flex-shrink-0" />
              <div className="flex-1">
                <div className="text-[12px] font-semibold text-gray-900 dark:text-[#e0e0e0]">
                  Subscribers: {radiusStatus.subscriber_count || 0} / {radiusStatus.max_subscribers || 0}
                </div>
                <div className="w-full h-1.5 bg-gray-200 dark:bg-gray-600 rounded-full mt-1">
                  <div
                    className={clsx('h-full rounded-full transition-all',
                      (radiusStatus.subscriber_count / radiusStatus.max_subscribers) >= 0.9 ? 'bg-red-500' :
                      (radiusStatus.subscriber_count / radiusStatus.max_subscribers) >= 0.7 ? 'bg-yellow-500' :
                      'bg-blue-500'
                    )}
                    style={{ width: `${Math.min((radiusStatus.subscriber_count / (radiusStatus.max_subscribers || 1)) * 100, 100)}%` }}
                  />
                </div>
              </div>
            </div>

            {/* Reseller Limit */}
            <div className="flex items-center gap-3 p-2.5 rounded border border-gray-200 bg-gray-50 dark:border-gray-700 dark:bg-gray-800">
              <UserGroupIcon className="w-5 h-5 text-purple-500 flex-shrink-0" />
              <div className="flex-1">
                <div className="text-[12px] font-semibold text-gray-900 dark:text-[#e0e0e0]">
                  Resellers: {radiusStatus.reseller_count || 0} / {radiusStatus.max_resellers || 0}
                </div>
                <div className="w-full h-1.5 bg-gray-200 dark:bg-gray-600 rounded-full mt-1">
                  <div
                    className={clsx('h-full rounded-full transition-all',
                      (radiusStatus.reseller_count / radiusStatus.max_resellers) >= 0.9 ? 'bg-red-500' :
                      (radiusStatus.reseller_count / radiusStatus.max_resellers) >= 0.7 ? 'bg-yellow-500' :
                      'bg-purple-500'
                    )}
                    style={{ width: `${Math.min((radiusStatus.reseller_count / (radiusStatus.max_resellers || 1)) * 100, 100)}%` }}
                  />
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Subscriber Stats */}
      <div className="wb-group">
        <div className="wb-group-title text-[11px]">Subscribers</div>
        <div className="wb-group-body p-2">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-1">
            <StatBox
              label="Total Subscribers"
              value={stats?.total_subscribers?.toLocaleString() || 0}
              icon={UsersIcon}
              iconColor="#4a7ab5"
            />
            <StatBox
              label="Online Now"
              value={stats?.online_subscribers?.toLocaleString() || 0}
              icon={WifiIcon}
              iconColor="#4CAF50"
            />
            <StatBox
              label="Expired"
              value={stats?.expired_subscribers?.toLocaleString() || 0}
              icon={ExclamationTriangleIcon}
              iconColor="#f44336"
            />
            <StatBox
              label="Expiring Soon"
              value={stats?.expiring_subscribers?.toLocaleString() || 0}
              icon={CalendarDaysIcon}
              iconColor="#FF9800"
            />
          </div>
        </div>
      </div>

      {/* Revenue & Sessions Stats */}
      <div className="wb-group">
        <div className="wb-group-title text-[11px]">Revenue & Sessions</div>
        <div className="wb-group-body p-2">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-1">
            <StatBox
              label="Today's Revenue"
              value={`$${stats?.today_revenue?.toFixed(2) || '0.00'}`}
              icon={BanknotesIcon}
              iconColor="#4CAF50"
            />
            <StatBox
              label="Month Revenue"
              value={`$${stats?.month_revenue?.toFixed(2) || '0.00'}`}
              icon={CurrencyDollarIcon}
              iconColor="#2196F3"
            />
            <StatBox
              label="Active Sessions"
              value={stats?.active_sessions?.toLocaleString() || 0}
              icon={SignalIcon}
              iconColor="#FF9800"
            />
            {isAdmin() && (
              <StatBox
                label="Total Resellers"
                value={stats?.total_resellers?.toLocaleString() || 0}
                icon={UserGroupIcon}
                iconColor="#9C27B0"
              />
            )}
          </div>
        </div>
      </div>

      {/* System Metrics - Admin Only, hidden in SaaS mode */}
      {isAdmin() && !isSaasMode && (
        <div className="wb-group">
          <div className="wb-group-title text-[11px]">System Metrics</div>
          <div className="wb-group-body p-2">
            <SystemMetricBar label="CPU" value={systemMetrics?.cpu_percent} />
            <SystemMetricBar label="Memory" value={systemMetrics?.memory_percent} />
            <SystemMetricBar label="HDD" value={systemMetrics?.disk_percent} />
          </div>
        </div>
      )}

      {/* Server Capacity & Cluster - Admin Only, hidden in SaaS mode */}
      {isAdmin() && !isSaasMode && systemCapacity && (
        <div className="wb-group">
          <div className="wb-group-title text-[11px] flex items-center justify-between">
            <div className="flex items-center gap-2">
              <span>Server Capacity</span>
              {systemCapacity.cluster_enabled ? (
                <span className="badge-info">
                  Cluster: {systemCapacity.online_nodes}/{systemCapacity.total_nodes} Online
                </span>
              ) : (
                <span className="badge-gray">Standalone</span>
              )}
            </div>
            <span className={clsx(
              'badge',
              systemCapacity.status === 'healthy' && 'badge-success',
              systemCapacity.status === 'warning' && 'badge-warning',
              systemCapacity.status === 'critical' && 'badge-danger'
            )}>
              {systemCapacity.status === 'healthy' ? 'Healthy' : systemCapacity.status === 'warning' ? 'Warning' : 'Critical'}
            </span>
          </div>
          <div className="wb-group-body p-2 space-y-2">
            {/* Capacity Stats Row */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-1">
              <StatBox
                label="Online Users"
                value={systemCapacity.online_users?.toLocaleString()}
                icon={WifiIcon}
                iconColor="#4CAF50"
              />
              <StatBox
                label="Recommended (70%)"
                value={systemCapacity.recommended_capacity?.toLocaleString()}
                icon={ServerIcon}
                iconColor="#2196F3"
              />
              <StatBox
                label="Maximum"
                value={systemCapacity.maximum_capacity?.toLocaleString()}
                icon={CpuChipIcon}
                iconColor="#FF9800"
              />
              <StatBox
                label="Usage"
                value={`${systemCapacity.usage_percent}%`}
                icon={CircleStackIcon}
                iconColor="#9C27B0"
              />
            </div>

            {/* Capacity Usage Bar */}
            <div>
              <div className="flex justify-between text-[10px] mb-0.5">
                <span className="text-gray-600 dark:text-[#aaa]">Capacity Usage</span>
                <span className="font-semibold text-gray-800 dark:text-[#e0e0e0]">{systemCapacity.online_users?.toLocaleString()} / {systemCapacity.maximum_capacity?.toLocaleString()} users</span>
              </div>
              <div className="w-full h-[4px] bg-[#e0e0e0] dark:bg-[#555]" style={{ borderRadius: '1px' }}>
                <div
                  className="h-full transition-all duration-500"
                  style={{
                    width: `${Math.min(systemCapacity.usage_percent, 100)}%`,
                    backgroundColor: systemCapacity.usage_percent < 70 ? '#4CAF50' : systemCapacity.usage_percent < 90 ? '#FF9800' : '#f44336',
                    borderRadius: '1px',
                  }}
                />
              </div>
            </div>

            {/* Cluster Nodes */}
            {systemCapacity.nodes && systemCapacity.nodes.length > 0 && (
              <div>
                <div className="text-[11px] font-semibold text-gray-700 dark:text-[#ccc] mb-1">
                  {systemCapacity.cluster_enabled ? 'Cluster Nodes' : 'Server Specs'}
                </div>
                <div className="table-container">
                  <table className="table table-compact">
                    <thead>
                      <tr>
                        <th>Server</th>
                        <th>Role</th>
                        <th>Status</th>
                        <th>CPU</th>
                        <th>RAM</th>
                        <th>Capacity</th>
                        {systemCapacity.cluster_enabled && <th>CPU%</th>}
                        {systemCapacity.cluster_enabled && <th>MEM%</th>}
                      </tr>
                    </thead>
                    <tbody>
                      {systemCapacity.nodes.map((node, idx) => (
                        <tr key={idx}>
                          <td>
                            <div className="font-semibold text-[11px] text-gray-900 dark:text-[#e0e0e0]">{node.name}</div>
                            <div className="text-[9px] text-gray-500 dark:text-[#aaa]">{node.ip}</div>
                          </td>
                          <td>
                            <span className={clsx(
                              'badge',
                              node.role === 'main' && 'badge-info',
                              node.role === 'secondary' && 'badge-purple',
                              node.role === 'standalone' && 'badge-gray'
                            )}>
                              {node.role}
                            </span>
                          </td>
                          <td>
                            <span className={clsx(
                              'badge',
                              node.status === 'online' && 'badge-success',
                              node.status === 'offline' && 'badge-danger'
                            )}>
                              {node.status}
                            </span>
                          </td>
                          <td>{node.cpu_cores} cores</td>
                          <td>{node.ram_gb} GB</td>
                          <td className="font-semibold">{node.capacity?.toLocaleString()}</td>
                          {systemCapacity.cluster_enabled && (
                            <td>{node.cpu_usage?.toFixed(1)}%</td>
                          )}
                          {systemCapacity.cluster_enabled && (
                            <td>{node.mem_usage?.toFixed(1)}%</td>
                          )}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}

            {/* Capacity Formula Explanation */}
            <div className="border border-[#a0a0a0] dark:border-[#555] bg-[#f7f7f7] dark:bg-[#333] p-2" style={{ borderRadius: '2px' }}>
              <div className="text-[10px] font-semibold text-gray-700 dark:text-[#ccc] mb-1">Capacity Formula</div>
              <div className="text-[10px] text-gray-600 dark:text-[#aaa] space-y-0.5">
                <div><span className="font-mono bg-[#e0e0e0] dark:bg-[#444] px-0.5" style={{ borderRadius: '1px' }}>{systemCapacity.total_cpu_cores} cores x 2000</span> = {(systemCapacity.total_cpu_cores * 2000).toLocaleString()} base users/CPU</div>
                <div><span className="font-mono bg-[#e0e0e0] dark:bg-[#444] px-0.5" style={{ borderRadius: '1px' }}>x {systemCapacity.storage_multiplier}</span> storage factor ({systemCapacity.storage_type?.toUpperCase()})</div>
                <div><span className="font-mono bg-[#e0e0e0] dark:bg-[#444] px-0.5" style={{ borderRadius: '1px' }}>x {systemCapacity.interim_factor}</span> interim factor ({systemCapacity.interim_interval} min)</div>
                <div><span className="font-mono bg-[#e0e0e0] dark:bg-[#444] px-0.5" style={{ borderRadius: '1px' }}>x {systemCapacity.safety_margin}</span> safety margin (15% reserve)</div>
                <div className="pt-1 border-t border-[#a0a0a0] dark:border-[#555]">
                  <span className="font-semibold">= {systemCapacity.maximum_capacity?.toLocaleString()} max users</span>
                  {systemCapacity.limiting_factor && (
                    <span className="ml-2 text-gray-500 dark:text-[#aaa]">(limited by {systemCapacity.limiting_factor?.toUpperCase()})</span>
                  )}
                </div>
              </div>
            </div>

            {/* Server Details */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-x-4 gap-y-0.5 text-[10px] text-gray-500 dark:text-[#aaa]">
              <div><span className="font-semibold text-gray-700 dark:text-[#ccc]">CPU Model:</span> {systemCapacity.cpu_model?.split('@')[0]?.trim() || 'N/A'}</div>
              <div><span className="font-semibold text-gray-700 dark:text-[#ccc]">DB Writes/sec:</span> {systemCapacity.db_writes_per_sec}</div>
              <div><span className="font-semibold text-gray-700 dark:text-[#ccc]">NAS Routers:</span> {systemCapacity.nas_count}</div>
              <div><span className="font-semibold text-gray-700 dark:text-[#ccc]">Total Subs:</span> {systemCapacity.total_subscribers?.toLocaleString()}</div>
            </div>
          </div>
        </div>
      )}

      {/* Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-2">
        <div className="lg:col-span-2 wb-group">
          <div className="wb-group-title text-[11px]">New vs Expired Users</div>
          <div className="wb-group-body p-2">
            <Suspense fallback={<div style={{ height: '240px' }} />}>
              <ReactECharts option={lineChartOption} style={{ height: '240px' }} />
            </Suspense>
          </div>
        </div>
        <div className="wb-group">
          <div className="wb-group-title text-[11px]">Users by Service</div>
          <div className="wb-group-body p-2">
            <Suspense fallback={<div style={{ height: '240px' }} />}>
              <ReactECharts option={pieChartOption} style={{ height: '240px' }} />
            </Suspense>
          </div>
        </div>
      </div>

      {/* Recent Transactions */}
      <div className="wb-group">
        <div className="wb-group-title text-[11px]">Recent Transactions</div>
        <div className="wb-group-body p-0">
          <div className="table-container" style={{ border: 'none' }}>
            <table className="table">
              <thead>
                <tr>
                  <th>Date</th>
                  <th>Type</th>
                  <th>User</th>
                  <th>Amount</th>
                  <th>Description</th>
                </tr>
              </thead>
              <tbody>
                {transactions?.map((tx) => (
                  <tr key={tx.id}>
                    <td>{formatDate(tx.created_at)}</td>
                    <td>
                      <span className={clsx('badge', tx.type === 'renewal' ? 'badge-success' : tx.type === 'new' ? 'badge-info' : 'badge-gray')}>
                        {tx.type}
                      </span>
                    </td>
                    <td>{tx.subscriber?.username || '-'}</td>
                    <td style={{ color: tx.amount >= 0 ? '#4CAF50' : '#f44336' }}>
                      ${Math.abs(tx.amount).toFixed(2)}
                    </td>
                    <td style={{ maxWidth: '200px', overflow: 'hidden', textOverflow: 'ellipsis' }}>{tx.description}</td>
                  </tr>
                )) || (
                  <tr>
                    <td colSpan={5} className="text-center text-gray-500 dark:text-[#aaa]">No transactions</td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      </div>

      {/* Help & Documentation */}
      <div className="wb-group">
        <div className="wb-group-title text-[11px]">Help & Documentation</div>
        <div className="wb-group-body p-3">
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <a
              href="/docs"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-3 p-3 rounded-lg border border-gray-200 dark:border-gray-700 hover:bg-blue-50 dark:hover:bg-blue-900/20 hover:border-blue-300 dark:hover:border-blue-700 transition-colors group"
            >
              <div className="w-9 h-9 rounded-lg bg-blue-100 dark:bg-blue-900/40 flex items-center justify-center flex-shrink-0">
                <BookOpenIcon className="w-5 h-5 text-blue-600 dark:text-blue-400" />
              </div>
              <div>
                <p className="text-[12px] font-semibold text-gray-900 dark:text-white group-hover:text-blue-700 dark:group-hover:text-blue-400">System Documentation</p>
                <p className="text-[11px] text-gray-500 dark:text-gray-400">Complete user guide for all features</p>
              </div>
            </a>
            <a
              href="/api-docs"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-3 p-3 rounded-lg border border-gray-200 dark:border-gray-700 hover:bg-green-50 dark:hover:bg-green-900/20 hover:border-green-300 dark:hover:border-green-700 transition-colors group"
            >
              <div className="w-9 h-9 rounded-lg bg-green-100 dark:bg-green-900/40 flex items-center justify-center flex-shrink-0">
                <CodeBracketIcon className="w-5 h-5 text-green-600 dark:text-green-400" />
              </div>
              <div>
                <p className="text-[12px] font-semibold text-gray-900 dark:text-white group-hover:text-green-700 dark:group-hover:text-green-400">API Documentation</p>
                <p className="text-[11px] text-gray-500 dark:text-gray-400">REST API reference for integrations</p>
              </div>
            </a>
            <a
              href="/docs#settings"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-3 p-3 rounded-lg border border-gray-200 dark:border-gray-700 hover:bg-purple-50 dark:hover:bg-purple-900/20 hover:border-purple-300 dark:hover:border-purple-700 transition-colors group"
            >
              <div className="w-9 h-9 rounded-lg bg-purple-100 dark:bg-purple-900/40 flex items-center justify-center flex-shrink-0">
                <QuestionMarkCircleIcon className="w-5 h-5 text-purple-600 dark:text-purple-400" />
              </div>
              <div>
                <p className="text-[12px] font-semibold text-gray-900 dark:text-white group-hover:text-purple-700 dark:group-hover:text-purple-400">Setup Guide</p>
                <p className="text-[11px] text-gray-500 dark:text-gray-400">Settings, configuration & troubleshooting</p>
              </div>
            </a>
          </div>
        </div>
      </div>
    </div>
  )
}
