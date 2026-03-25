import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'react-hot-toast'
import { clusterApi } from '../services/api'
import { copyToClipboard } from '../utils/clipboard'

const ClusterTab = () => {
  const queryClient = useQueryClient()
  const [setupMode, setSetupMode] = useState(null) // null, 'main', 'secondary', 'recover'
  const [mainServerIP, setMainServerIP] = useState('')
  const [clusterSecret, setClusterSecret] = useState('')
  const [serverName, setServerName] = useState('')
  const [serverIP, setServerIP] = useState('')
  const [serverRole, setServerRole] = useState('secondary')
  const [testResult, setTestResult] = useState(null)
  const [testing, setTesting] = useState(false)
  const [promoting, setPromoting] = useState(false)
  const [recovering, setRecovering] = useState(false)
  const [sourceServerIP, setSourceServerIP] = useState('')
  const [sourcePassword, setSourcePassword] = useState('')
  const [sourceTestResult, setSourceTestResult] = useState(null)
  const [testingSource, setTestingSource] = useState(false)

  // Fetch cluster config
  const { data: configData, isLoading: configLoading } = useQuery({
    queryKey: ['cluster-config'],
    queryFn: async () => {
      const res = await clusterApi.getConfig()
      return res.data.data
    },
  })

  // Fetch cluster status
  const { data: statusData, isLoading: statusLoading, refetch: refetchStatus } = useQuery({
    queryKey: ['cluster-status'],
    queryFn: async () => {
      const res = await clusterApi.getStatus()
      return res.data.data
    },
    refetchInterval: 10000, // Refresh every 10 seconds
    enabled: configData?.is_active,
  })

  // Check main server status (for secondary servers)
  const { data: mainStatus, refetch: refetchMainStatus } = useQuery({
    queryKey: ['cluster-main-status'],
    queryFn: async () => {
      const res = await clusterApi.checkMainStatus()
      return res.data.data
    },
    refetchInterval: 30000, // Check every 30 seconds
    enabled: configData?.is_active && configData?.server_role !== 'main',
  })

  // Setup main mutation
  const setupMainMutation = useMutation({
    mutationFn: (data) => clusterApi.setupMain(data),
    onSuccess: (res) => {
      toast.success('Main server configured successfully')
      setSetupMode(null)
      queryClient.invalidateQueries(['cluster-config'])
      queryClient.invalidateQueries(['cluster-status'])
    },
    onError: (err) => {
      toast.error(err.response?.data?.message || 'Failed to configure main server')
    },
  })

  // Setup secondary mutation
  const setupSecondaryMutation = useMutation({
    mutationFn: (data) => clusterApi.setupSecondary(data),
    onSuccess: (res) => {
      toast.success('Successfully joined cluster')
      setSetupMode(null)
      queryClient.invalidateQueries(['cluster-config'])
      queryClient.invalidateQueries(['cluster-status'])
    },
    onError: (err) => {
      toast.error(err.response?.data?.message || 'Failed to join cluster')
    },
  })

  // Leave cluster mutation
  const leaveClusterMutation = useMutation({
    mutationFn: () => clusterApi.leaveCluster(),
    onSuccess: () => {
      toast.success('Left cluster successfully')
      queryClient.invalidateQueries(['cluster-config'])
      queryClient.invalidateQueries(['cluster-status'])
    },
    onError: (err) => {
      toast.error(err.response?.data?.message || 'Failed to leave cluster')
    },
  })

  // Remove node mutation
  const removeNodeMutation = useMutation({
    mutationFn: (id) => clusterApi.removeNode(id),
    onSuccess: () => {
      toast.success('Node removed from cluster')
      queryClient.invalidateQueries(['cluster-status'])
    },
    onError: (err) => {
      toast.error(err.response?.data?.message || 'Failed to remove node')
    },
  })

  // Test connection
  const handleTestConnection = async () => {
    setTesting(true)
    setTestResult(null)
    try {
      const res = await clusterApi.testConnection({
        main_server_ip: mainServerIP,
        cluster_secret: clusterSecret,
      })
      setTestResult(res.data)
    } catch (err) {
      setTestResult({
        success: false,
        message: err.response?.data?.message || 'Connection failed',
      })
    } finally {
      setTesting(false)
    }
  }

  // Handle setup main
  const handleSetupMain = () => {
    setupMainMutation.mutate({
      server_name: serverName || 'Main Server',
      server_ip: serverIP || configData?.server_ip,
    })
  }

  // Handle setup secondary
  const handleSetupSecondary = () => {
    if (!mainServerIP || !clusterSecret) {
      toast.error('Main server IP and cluster secret are required')
      return
    }
    setupSecondaryMutation.mutate({
      main_server_ip: mainServerIP,
      cluster_secret: clusterSecret,
      server_name: serverName || 'Secondary Server',
      server_ip: serverIP || configData?.server_ip,
      server_role: serverRole,
    })
  }

  // Handle promote to main (one-click failover)
  const handlePromoteToMain = async () => {
    if (!confirm('Are you sure you want to promote this server to MAIN?\n\nThis will:\n• Make this server the primary database\n• Allow all write operations\n• Stop replication from old main\n\nOnly do this if the main server is offline!')) {
      return
    }

    setPromoting(true)
    try {
      const res = await clusterApi.promoteToMain()
      if (res.data.success) {
        toast.success('Successfully promoted to main server!')
        queryClient.invalidateQueries(['cluster-config'])
        queryClient.invalidateQueries(['cluster-status'])
        queryClient.invalidateQueries(['cluster-main-status'])
      } else {
        toast.error(res.data.message || 'Failed to promote')
      }
    } catch (err) {
      toast.error(err.response?.data?.message || 'Failed to promote to main')
    } finally {
      setPromoting(false)
    }
  }

  // Test source server connection (for recovery)
  const handleTestSourceConnection = async () => {
    setTestingSource(true)
    setSourceTestResult(null)
    try {
      const res = await clusterApi.testSourceConnection({
        source_server_ip: sourceServerIP,
        root_password: sourcePassword,
      })
      setSourceTestResult(res.data)
    } catch (err) {
      setSourceTestResult({
        success: false,
        message: err.response?.data?.message || 'Connection failed',
      })
    } finally {
      setTestingSource(false)
    }
  }

  // Handle recover from server
  const handleRecoverFromServer = async () => {
    if (!sourceTestResult?.success) {
      toast.error('Please test the connection first')
      return
    }

    if (!confirm(`Are you sure you want to recover data from ${sourceServerIP}?\n\nThis will:\n• Download the full database from the source server\n• Replace all data on this server\n• Configure this server as the new main\n\nThis process may take a few minutes.`)) {
      return
    }

    setRecovering(true)
    try {
      const res = await clusterApi.recoverFromServer({
        source_server_ip: sourceServerIP,
        root_password: sourcePassword,
        become_main: true,
      })
      if (res.data.success) {
        toast.success('Recovery complete! Refreshing page...')
        setTimeout(() => {
          window.location.reload()
        }, 2000)
      } else {
        toast.error(res.data.message || 'Recovery failed')
      }
    } catch (err) {
      toast.error(err.response?.data?.message || 'Recovery failed')
    } finally {
      setRecovering(false)
    }
  }

  // Get status color
  const getStatusColor = (status) => {
    switch (status) {
      case 'online': return 'badge-success'
      case 'syncing': return 'badge-warning'
      case 'offline': return 'badge-gray'
      case 'error': return 'badge-danger'
      default: return 'badge-gray'
    }
  }

  // Get status icon
  const getStatusIcon = (status) => {
    switch (status) {
      case 'online': return '*'
      case 'syncing': return '~'
      case 'offline': return '-'
      case 'error': return '!'
      default: return '?'
    }
  }

  if (configLoading) {
    return (
      <div className="flex items-center justify-center py-8">
        <span className="text-[12px] text-gray-600 dark:text-[#aaa]">Loading cluster configuration...</span>
      </div>
    )
  }

  // Not configured - show setup options
  if (!configData?.is_active || configData?.server_role === 'standalone') {
    return (
      <div className="space-y-4">
        <div className="wb-group">
          <div className="wb-group-title">HA Cluster Configuration</div>
          <div className="wb-group-body">
            <p className="text-[12px] text-gray-600 dark:text-[#aaa] mb-4">
              Set up High Availability clustering to improve performance, redundancy, and backup capabilities.
            </p>

            {!setupMode ? (
              <div className="space-y-4">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  {/* Main Server Option */}
                  <div
                    className="card p-3 cursor-pointer hover:bg-[#e8e8f0] dark:hover:bg-[#4a4a4a] transition-colors"
                    onClick={() => setSetupMode('main')}
                  >
                    <div className="flex items-center mb-2">
                      <div className="w-8 h-8 bg-[#e3f2fd] dark:bg-[#2d5a87] flex items-center justify-center border border-[#a0a0a0] dark:border-[#555]" style={{ borderRadius: '2px' }}>
                        <svg className="w-4 h-4 text-[#316AC5]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
                        </svg>
                      </div>
                      <div className="ml-2">
                        <span className="text-[12px] font-semibold text-gray-900 dark:text-[#e0e0e0]">Main Server</span>
                        <span className="text-[11px] text-gray-500 dark:text-[#aaa] ml-1">(Primary node)</span>
                      </div>
                    </div>
                    <p className="text-[12px] text-gray-600 dark:text-[#aaa] mb-2">
                      Configure this server as the main (primary) server. Other servers will replicate from this server.
                    </p>
                    <ul className="text-[11px] text-gray-600 dark:text-[#aaa] space-y-0.5">
                      <li>- Database primary (all writes)</li>
                      <li>- Redis primary</li>
                      <li>- RADIUS primary</li>
                      <li>- API active</li>
                    </ul>
                  </div>

                  {/* Secondary Server Option */}
                  <div
                    className="card p-3 cursor-pointer hover:bg-[#e8e8f0] dark:hover:bg-[#4a4a4a] transition-colors"
                    onClick={() => setSetupMode('secondary')}
                  >
                    <div className="flex items-center mb-2">
                      <div className="w-8 h-8 bg-[#e8f5e9] dark:bg-[#1e7e34] flex items-center justify-center border border-[#a0a0a0] dark:border-[#555]" style={{ borderRadius: '2px' }}>
                        <svg className="w-4 h-4 text-[#4CAF50]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
                        </svg>
                      </div>
                      <div className="ml-2">
                        <span className="text-[12px] font-semibold text-gray-900 dark:text-[#e0e0e0]">Secondary Server</span>
                        <span className="text-[11px] text-gray-500 dark:text-[#aaa] ml-1">(Replica node)</span>
                      </div>
                    </div>
                    <p className="text-[12px] text-gray-600 dark:text-[#aaa] mb-2">
                      Join an existing cluster as a secondary server. Data will be replicated from the main server.
                    </p>
                    <ul className="text-[11px] text-gray-600 dark:text-[#aaa] space-y-0.5">
                      <li>- Database replica (real-time sync)</li>
                      <li>- Redis replica</li>
                      <li>- RADIUS backup</li>
                      <li>- API standby (auto-failover)</li>
                    </ul>
                  </div>
                </div>

                {/* Recovery Option */}
                <div className="border-t border-[#a0a0a0] dark:border-[#555] pt-4">
                  <div
                    className="border-l-4 border-l-[#FF9800] bg-[#fff8e1] dark:bg-[#2a2a2a] p-3 cursor-pointer hover:bg-[#fff3cd] dark:hover:bg-[#3a3a2a] transition-colors"
                    onClick={() => setSetupMode('recover')}
                  >
                    <div className="flex items-center mb-2">
                      <div className="w-8 h-8 bg-[#fff3e0] dark:bg-[#8a5a00] flex items-center justify-center border border-[#a0a0a0] dark:border-[#555]" style={{ borderRadius: '2px' }}>
                        <svg className="w-4 h-4 text-[#FF9800]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
                        </svg>
                      </div>
                      <div className="ml-2">
                        <span className="text-[12px] font-semibold text-gray-900 dark:text-[#e0e0e0]">Recover from Existing Server</span>
                        <span className="text-[11px] text-[#FF9800] ml-1">(Disaster Recovery)</span>
                      </div>
                    </div>
                    <p className="text-[12px] text-gray-600 dark:text-[#aaa] mb-2">
                      Restore data from an existing server. Use this if you're replacing a failed main server or migrating to new hardware.
                    </p>
                    <ul className="text-[11px] text-gray-600 dark:text-[#aaa] space-y-0.5">
                      <li>- Download full database backup</li>
                      <li>- Restore all subscribers and settings</li>
                      <li>- Sync uploads (logo, favicon)</li>
                      <li>- Become the new main server</li>
                    </ul>
                  </div>
                </div>
              </div>
            ) : setupMode === 'recover' ? (
              /* Recovery Form */
              <div className="max-w-lg">
                <div className="text-[12px] font-semibold text-gray-900 dark:text-[#e0e0e0] mb-3">
                  Recover Data from Existing Server
                </div>

                <div className="border-l-4 border-l-[#FF9800] bg-[#fff8e1] dark:bg-[#2a2a2a] p-3 mb-3">
                  <div className="text-[12px] font-semibold text-[#e65100] dark:text-[#FF9800] mb-1">Important</div>
                  <p className="text-[12px] text-gray-700 dark:text-[#aaa]">
                    This will download all data from the source server and replace any existing data on this server.
                    Make sure the source server is running and accessible.
                  </p>
                </div>

                <div className="space-y-3">
                  <div>
                    <label className="label">Source Server IP Address *</label>
                    <input
                      type="text"
                      value={sourceServerIP}
                      onChange={(e) => setSourceServerIP(e.target.value)}
                      placeholder="10.0.0.219"
                      className="input"
                    />
                    <p className="text-[11px] text-gray-500 dark:text-[#aaa] mt-0.5">The IP of your existing ProISP server with the data</p>
                  </div>

                  <div>
                    <label className="label">Root Password *</label>
                    <input
                      type="password"
                      value={sourcePassword}
                      onChange={(e) => setSourcePassword(e.target.value)}
                      placeholder="********"
                      className="input"
                    />
                    <p className="text-[11px] text-gray-500 dark:text-[#aaa] mt-0.5">SSH root password for the source server</p>
                  </div>

                  {/* Test Connection */}
                  <div className="card p-3">
                    <button
                      onClick={handleTestSourceConnection}
                      disabled={testingSource || !sourceServerIP || !sourcePassword}
                      className="btn btn-sm"
                    >
                      {testingSource ? 'Testing...' : 'Test Connection'}
                    </button>

                    {sourceTestResult && (
                      <div className={`mt-2 p-2 text-[12px] ${sourceTestResult.success ? 'border-l-4 border-l-[#4CAF50] bg-[#e8f5e9] dark:bg-[#1a3a1a]' : 'border-l-4 border-l-[#f44336] bg-[#ffebee] dark:bg-[#3a1a1a]'}`}>
                        <span className={`font-semibold ${sourceTestResult.success ? 'text-[#2e7d32] dark:text-[#4CAF50]' : 'text-[#c62828] dark:text-[#f44336]'}`}>
                          {sourceTestResult.success ? 'Connection successful' : 'Connection failed'}
                        </span>
                        {sourceTestResult.success && sourceTestResult.data && (
                          <div className="mt-1 text-[11px] text-[#2e7d32] dark:text-[#4CAF50]">
                            <p>SSH: {sourceTestResult.data.ssh_ok ? 'OK' : 'FAIL'}</p>
                            <p>Database: {sourceTestResult.data.database_ok ? 'OK' : 'FAIL'}</p>
                            <p>Subscribers: {sourceTestResult.data.subscribers?.toLocaleString() || 0}</p>
                          </div>
                        )}
                        {sourceTestResult.message && !sourceTestResult.success && (
                          <p className="mt-1 text-[11px] text-[#c62828] dark:text-[#f44336]">{sourceTestResult.message}</p>
                        )}
                      </div>
                    )}
                  </div>

                  <div className="flex gap-2">
                    <button
                      onClick={handleRecoverFromServer}
                      disabled={recovering || !sourceTestResult?.success}
                      className="btn btn-primary btn-sm"
                    >
                      {recovering ? 'Recovering... Please wait' : 'Recover Data'}
                    </button>
                    <button
                      onClick={() => {
                        setSetupMode(null)
                        setSourceTestResult(null)
                      }}
                      className="btn btn-sm"
                    >
                      Cancel
                    </button>
                  </div>
                </div>
              </div>
            ) : setupMode === 'main' ? (
              /* Main Server Setup Form */
              <div className="max-w-lg">
                <div className="text-[12px] font-semibold text-gray-900 dark:text-[#e0e0e0] mb-3">
                  Configure as Main Server
                </div>

                <div className="space-y-3">
                  <div>
                    <label className="label">Server Name</label>
                    <input
                      type="text"
                      value={serverName}
                      onChange={(e) => setServerName(e.target.value)}
                      placeholder="Main Server"
                      className="input"
                    />
                  </div>

                  <div>
                    <label className="label">Server IP Address</label>
                    <input
                      type="text"
                      value={serverIP || configData?.server_ip || ''}
                      onChange={(e) => setServerIP(e.target.value)}
                      placeholder={configData?.server_ip || 'Auto-detect'}
                      className="input"
                    />
                    <p className="text-[11px] text-gray-500 dark:text-[#aaa] mt-0.5">Leave empty to auto-detect</p>
                  </div>

                  <div className="border-l-4 border-l-[#2196F3] bg-[#e3f2fd] dark:bg-[#1a2a3a] p-3">
                    <div className="text-[12px] font-semibold text-[#1565c0] dark:text-[#64b5f6] mb-1">What happens next?</div>
                    <ul className="text-[11px] text-[#1976d2] dark:text-[#90caf9] space-y-0.5">
                      <li>- A unique Cluster ID and Secret will be generated</li>
                      <li>- PostgreSQL will be configured for replication</li>
                      <li>- Redis will be configured as primary</li>
                      <li>- You'll receive the cluster secret to share with secondary servers</li>
                    </ul>
                  </div>

                  <div className="flex gap-2">
                    <button
                      onClick={handleSetupMain}
                      disabled={setupMainMutation.isPending}
                      className="btn btn-primary btn-sm"
                    >
                      {setupMainMutation.isPending ? 'Configuring...' : 'Configure as Main'}
                    </button>
                    <button
                      onClick={() => setSetupMode(null)}
                      className="btn btn-sm"
                    >
                      Cancel
                    </button>
                  </div>
                </div>
              </div>
            ) : (
              /* Secondary Server Setup Form */
              <div className="max-w-lg">
                <div className="text-[12px] font-semibold text-gray-900 dark:text-[#e0e0e0] mb-3">
                  Join Cluster as Secondary
                </div>

                <div className="space-y-3">
                  <div>
                    <label className="label">Main Server IP Address *</label>
                    <input
                      type="text"
                      value={mainServerIP}
                      onChange={(e) => setMainServerIP(e.target.value)}
                      placeholder="192.168.1.10"
                      className="input"
                    />
                  </div>

                  <div>
                    <label className="label">Cluster Secret Key *</label>
                    <input
                      type="text"
                      value={clusterSecret}
                      onChange={(e) => setClusterSecret(e.target.value)}
                      placeholder="xxxx-xxxx-xxxx-xxxx"
                      className="input font-mono"
                    />
                    <p className="text-[11px] text-gray-500 dark:text-[#aaa] mt-0.5">Get this from the main server's cluster settings</p>
                  </div>

                  <div>
                    <label className="label">Server Role</label>
                    <select
                      value={serverRole}
                      onChange={(e) => setServerRole(e.target.value)}
                      className="input"
                    >
                      <option value="secondary">Secondary (Failover + RADIUS backup)</option>
                      <option value="server3">Server 3 (Read-only + Reports)</option>
                    </select>
                  </div>

                  <div>
                    <label className="label">Server Name</label>
                    <input
                      type="text"
                      value={serverName}
                      onChange={(e) => setServerName(e.target.value)}
                      placeholder="Secondary Server"
                      className="input"
                    />
                  </div>

                  {/* Test Connection */}
                  <div className="card p-3">
                    <button
                      onClick={handleTestConnection}
                      disabled={testing || !mainServerIP}
                      className="btn btn-sm"
                    >
                      {testing ? 'Testing...' : 'Test Connection'}
                    </button>

                    {testResult && (
                      <div className={`mt-2 p-2 text-[12px] ${testResult.success ? 'border-l-4 border-l-[#4CAF50] bg-[#e8f5e9] dark:bg-[#1a3a1a]' : 'border-l-4 border-l-[#f44336] bg-[#ffebee] dark:bg-[#3a1a1a]'}`}>
                        <span className={`font-semibold ${testResult.success ? 'text-[#2e7d32] dark:text-[#4CAF50]' : 'text-[#c62828] dark:text-[#f44336]'}`}>
                          {testResult.success ? 'Connection successful' : 'Connection failed'}
                        </span>
                        {testResult.success && (
                          <div className="mt-1 text-[11px] text-[#2e7d32] dark:text-[#4CAF50]">
                            <p>API: {testResult.api_ok ? 'OK' : 'FAIL'}</p>
                            <p>Database: {testResult.db_ok ? 'OK' : 'FAIL'}</p>
                            <p>Redis: {testResult.redis_ok ? 'OK' : 'FAIL'}</p>
                          </div>
                        )}
                        {testResult.message && !testResult.success && (
                          <p className="mt-1 text-[11px] text-[#c62828] dark:text-[#f44336]">{testResult.message}</p>
                        )}
                      </div>
                    )}
                  </div>

                  <div className="flex gap-2">
                    <button
                      onClick={handleSetupSecondary}
                      disabled={setupSecondaryMutation.isPending || !mainServerIP || !clusterSecret}
                      className="btn btn-success btn-sm"
                    >
                      {setupSecondaryMutation.isPending ? 'Joining...' : 'Join Cluster'}
                    </button>
                    <button
                      onClick={() => setSetupMode(null)}
                      className="btn btn-sm"
                    >
                      Cancel
                    </button>
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Current Server Info */}
        <div className="wb-group">
          <div className="wb-group-title">Current Server Information</div>
          <div className="wb-group-body">
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 text-[12px]">
              <div>
                <span className="text-gray-500 dark:text-[#aaa]">Server IP:</span>
                <p className="font-mono text-gray-900 dark:text-[#e0e0e0]">{configData?.server_ip || 'Unknown'}</p>
              </div>
              <div>
                <span className="text-gray-500 dark:text-[#aaa]">Hardware ID:</span>
                <p className="font-mono text-gray-900 dark:text-[#e0e0e0] truncate">{configData?.hardware_id || 'Unknown'}</p>
              </div>
              <div>
                <span className="text-gray-500 dark:text-[#aaa]">Database ID:</span>
                <p className="font-mono text-gray-900 dark:text-[#e0e0e0] truncate">{configData?.database_id || 'Unknown'}</p>
              </div>
              <div>
                <span className="text-gray-500 dark:text-[#aaa]">Status:</span>
                <p className="text-gray-900 dark:text-[#e0e0e0]">Standalone</p>
              </div>
            </div>
          </div>
        </div>
      </div>
    )
  }

  // Cluster is active - show status dashboard
  return (
    <div className="space-y-4">
      {/* Failover Alert - Show when secondary and main is offline */}
      {configData?.server_role !== 'main' && mainStatus?.can_promote && (
        <div className="border-l-4 border-l-[#f44336] bg-[#ffebee] dark:bg-[#3a1a1a] p-4">
          <div className="flex items-start">
            <div className="flex-shrink-0">
              <svg className="h-5 w-5 text-[#f44336]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
            </div>
            <div className="ml-3 flex-1">
              <div className="text-[12px] font-semibold text-[#c62828] dark:text-[#ef9a9a]">
                Main Server Offline
              </div>
              <p className="mt-1 text-[12px] text-[#c62828] dark:text-[#ef9a9a]">
                Main server ({mainStatus?.main_server_ip}) has been offline for {mainStatus?.offline_minutes} minutes.
                {mainStatus?.main_last_seen && (
                  <span className="block mt-0.5">Last seen: {mainStatus.main_last_seen}</span>
                )}
              </p>
              <p className="mt-1 text-[12px] text-[#c62828] dark:text-[#ef9a9a]">
                Your data is safe on this server. You can promote this server to become the new main server.
              </p>
              <div className="mt-3">
                <button
                  onClick={handlePromoteToMain}
                  disabled={promoting}
                  className="btn btn-danger"
                >
                  {promoting ? (
                    <span className="flex items-center">
                      <svg className="animate-spin -ml-1 mr-2 h-3 w-3 text-white" fill="none" viewBox="0 0 24 24">
                        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                      </svg>
                      Promoting...
                    </span>
                  ) : (
                    'Promote to Main Server'
                  )}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Secondary Server Notice - Read Only Mode */}
      {configData?.server_role !== 'main' && mainStatus?.is_main_online && (
        <div className="border-l-4 border-l-[#2196F3] bg-[#e3f2fd] dark:bg-[#1a2a3a] p-3">
          <div className="flex items-center">
            <svg className="h-4 w-4 text-[#2196F3] mr-2 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span className="text-[12px] text-[#1565c0] dark:text-[#90caf9]">
              This is a <strong>secondary server</strong> (read-only). To create or edit data, use the main server ({mainStatus?.main_server_ip}).
            </span>
          </div>
        </div>
      )}

      {/* Cluster Overview */}
      <div className="wb-group">
        <div className="wb-group-title flex items-center justify-between">
          <span>HA Cluster Status</span>
          <div className="flex items-center gap-2">
            <span className="badge-success">Active</span>
            <button
              onClick={() => refetchStatus()}
              className="btn btn-xs"
            >
              Refresh
            </button>
          </div>
        </div>
        <div className="wb-group-body">
          <p className="text-[12px] text-gray-500 dark:text-[#aaa] mb-3">
            Cluster ID: <span className="font-mono">{configData?.cluster_id}</span>
          </p>

          {/* Role and Secret (for main only) */}
          {configData?.server_role === 'main' && (
            <div className="border-l-4 border-l-[#2196F3] bg-[#e3f2fd] dark:bg-[#1a2a3a] p-3 mb-3">
              <div className="flex items-center justify-between">
                <div>
                  <div className="text-[12px] font-semibold text-[#1565c0] dark:text-[#64b5f6]">Cluster Secret Key</div>
                  <p className="text-[11px] text-[#1976d2] dark:text-[#90caf9] mt-0.5">
                    Share this key with secondary servers to join the cluster
                  </p>
                </div>
                <button
                  onClick={() => {
                    copyToClipboard(configData?.cluster_secret || '').then(() => toast.success('Copied to clipboard')).catch(() => toast.error('Copy failed'))
                  }}
                  className="btn btn-primary btn-xs"
                >
                  Copy
                </button>
              </div>
              <div className="mt-2 p-2 bg-white dark:bg-[#3a3a3a] border border-[#a0a0a0] dark:border-[#555] font-mono text-[12px] text-gray-900 dark:text-[#e0e0e0]" style={{ borderRadius: '2px' }}>
                {configData?.cluster_secret}
              </div>
            </div>
          )}

          {/* Nodes Table */}
          <div className="table-container">
            <table className="table">
              <thead>
                <tr>
                  <th>Server</th>
                  <th>Role</th>
                  <th>IP Address</th>
                  <th>Version</th>
                  <th>Status</th>
                  <th>DB Sync</th>
                  <th>Resources</th>
                  <th>Last Seen</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {statusData?.nodes?.map((node) => (
                  <tr key={node.id}>
                    <td>
                      <span className="font-semibold">{node.server_name}</span>
                    </td>
                    <td>
                      <span className={node.server_role === 'main' ? 'badge-primary' : 'badge-gray'}>
                        {node.server_role}
                      </span>
                    </td>
                    <td className="font-mono">{node.server_ip}</td>
                    <td>
                      <span className="badge-purple">{node.version || 'Unknown'}</span>
                    </td>
                    <td>
                      <span className={getStatusColor(node.status)}>
                        {node.status}
                      </span>
                    </td>
                    <td>
                      <span className={getStatusColor(node.db_sync_status)}>
                        {node.db_sync_status}
                        {node.db_replication_lag > 0 && (
                          <span className="ml-1">({node.db_replication_lag}s lag)</span>
                        )}
                      </span>
                    </td>
                    <td>
                      <span className="text-[11px]">
                        CPU {node.cpu_usage?.toFixed(0)}% | MEM {node.memory_usage?.toFixed(0)}%
                      </span>
                    </td>
                    <td>
                      {node.last_heartbeat
                        ? new Date(node.last_heartbeat).toLocaleTimeString()
                        : 'Never'
                      }
                    </td>
                    <td>
                      {node.server_role !== 'main' && configData?.server_role === 'main' && (
                        <button
                          onClick={() => {
                            if (confirm(`Remove ${node.server_name} from cluster?`)) {
                              removeNodeMutation.mutate(node.id)
                            }
                          }}
                          className="btn btn-danger btn-xs"
                        >
                          Remove
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Cluster Stats */}
          <div className="mt-3 grid grid-cols-3 gap-3">
            <div className="stat-card text-center">
              <div className="text-[18px] font-bold text-gray-900 dark:text-[#e0e0e0]">
                {statusData?.online_nodes || 0}/{statusData?.total_nodes || 0}
              </div>
              <div className="text-[11px] text-gray-500 dark:text-[#aaa]">Nodes Online</div>
            </div>
            <div className="stat-card text-center">
              <div className="text-[18px] font-bold text-gray-900 dark:text-[#e0e0e0]">
                {statusData?.db_replication_ok ? 'OK' : 'FAIL'}
              </div>
              <div className="text-[11px] text-gray-500 dark:text-[#aaa]">DB Replication</div>
            </div>
            <div className="stat-card text-center">
              <div className="text-[18px] font-bold text-gray-900 dark:text-[#e0e0e0] capitalize">
                {configData?.server_role}
              </div>
              <div className="text-[11px] text-gray-500 dark:text-[#aaa]">This Server Role</div>
            </div>
          </div>
        </div>
      </div>

      {/* Recent Events */}
      {statusData?.events?.length > 0 && (
        <div className="wb-group">
          <div className="wb-group-title">Recent Cluster Events</div>
          <div className="wb-group-body space-y-1">
            {statusData.events.map((event) => (
              <div key={event.id} className={`p-2 text-[12px] ${
                event.severity === 'error' ? 'border-l-4 border-l-[#f44336] bg-[#ffebee] dark:bg-[#3a1a1a]' :
                event.severity === 'warning' ? 'border-l-4 border-l-[#FF9800] bg-[#fff8e1] dark:bg-[#2a2a1a]' :
                'bg-[#f7f7f7] dark:bg-[#333]'
              }`}>
                <div className="flex items-center justify-between">
                  <span className="font-semibold text-gray-900 dark:text-[#e0e0e0]">{event.event_type}</span>
                  <span className="text-[11px] text-gray-500 dark:text-[#aaa]">
                    {new Date(event.created_at).toLocaleString()}
                  </span>
                </div>
                <p className="text-gray-600 dark:text-[#aaa] mt-0.5">{event.description}</p>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Actions */}
      <div className="wb-group">
        <div className="wb-group-title">Cluster Actions</div>
        <div className="wb-group-body flex flex-wrap gap-2">
          <button
            onClick={() => refetchStatus()}
            className="btn btn-sm"
          >
            Force Sync
          </button>
          {configData?.server_role !== 'main' && (
            <button
              onClick={() => {
                if (confirm('Are you sure you want to leave the cluster?')) {
                  leaveClusterMutation.mutate()
                }
              }}
              disabled={leaveClusterMutation.isPending}
              className="btn btn-danger btn-sm"
            >
              {leaveClusterMutation.isPending ? 'Leaving...' : 'Leave Cluster'}
            </button>
          )}
          {configData?.server_role === 'main' && statusData?.total_nodes === 1 && (
            <button
              onClick={() => {
                if (confirm('Dissolve the cluster and return to standalone mode?')) {
                  leaveClusterMutation.mutate()
                }
              }}
              disabled={leaveClusterMutation.isPending}
              className="btn btn-danger btn-sm"
            >
              {leaveClusterMutation.isPending ? 'Dissolving...' : 'Dissolve Cluster'}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}

export default ClusterTab
