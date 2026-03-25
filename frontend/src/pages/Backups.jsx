import { useState, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { backupApi, nasApi } from '../services/api'
import { formatDate, formatDateTime } from '../utils/timezone'
import {
  ArrowPathIcon,
  ArrowDownTrayIcon,
  ArrowUpTrayIcon,
  TrashIcon,
  CloudArrowUpIcon,
  CloudIcon,
  DocumentArrowUpIcon,
  ExclamationTriangleIcon,
  CalendarDaysIcon,
  ClockIcon,
  PlayIcon,
  PencilIcon,
  ServerIcon,
  CheckCircleIcon,
  XCircleIcon,
  PlusIcon,
} from '@heroicons/react/24/outline'
import clsx from 'clsx'
import toast from 'react-hot-toast'

function formatBytes(bytes) {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

const DAYS_OF_WEEK = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']

export default function Backups() {
  const queryClient = useQueryClient()
  const fileInputRef = useRef(null)
  const uploadRestoreInputRef = useRef(null)
  const [activeTab, setActiveTab] = useState('manual')
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [showRestoreConfirm, setShowRestoreConfirm] = useState(null)
  const [sourceLicenseKey, setSourceLicenseKey] = useState('')
  const [backupType, setBackupType] = useState('full')
  const [uploadRestoreFile, setUploadRestoreFile] = useState(null)
  // MikroTik backup state
  const [selectedNasIds, setSelectedNasIds] = useState([])
  const [mikrotikCreating, setMikrotikCreating] = useState(false)

  // Schedule modal state
  const [showScheduleModal, setShowScheduleModal] = useState(false)
  const [editingSchedule, setEditingSchedule] = useState(null)
  const [scheduleForm, setScheduleForm] = useState({
    name: '',
    backup_type: 'full',
    frequency: 'daily',
    day_of_week: 0,
    day_of_month: 1,
    time_of_day: '02:00',
    retention: 7,
    storage_type: 'local',
    ftp_enabled: false,
    ftp_host: '',
    ftp_port: 21,
    ftp_username: '',
    ftp_password: '',
    ftp_path: '/backups',
    is_enabled: true,
    cloud_enabled: false,
    include_mikrotik: false,
  })
  const [testingFTP, setTestingFTP] = useState(false)

  // Cloud backup state
  const [cloudDeleteConfirm, setCloudDeleteConfirm] = useState(null)
  const [cloudUploadConfirm, setCloudUploadConfirm] = useState(null)

  // Manual backups query
  const { data, isLoading, refetch } = useQuery({
    queryKey: ['backups'],
    queryFn: () => backupApi.list().then((r) => r.data),
  })

  // Schedules query
  const { data: schedulesData, isLoading: schedulesLoading, refetch: refetchSchedules } = useQuery({
    queryKey: ['backup-schedules'],
    queryFn: () => backupApi.listSchedules().then((r) => r.data),
  })

  // Backup logs query
  const { data: logsData, isLoading: logsLoading } = useQuery({
    queryKey: ['backup-logs'],
    queryFn: () => backupApi.listLogs({ limit: 50 }).then((r) => r.data),
    enabled: activeTab === 'logs',
  })

  // Cloud backup queries
  const { data: cloudBackupsData, isLoading: cloudLoading, refetch: refetchCloud } = useQuery({
    queryKey: ['cloud-backups'],
    queryFn: () => backupApi.cloudList().then((r) => r.data),
    enabled: activeTab === 'cloud',
  })

  const { data: cloudUsageData } = useQuery({
    queryKey: ['cloud-usage'],
    queryFn: () => backupApi.cloudUsage().then((r) => r.data),
    enabled: activeTab === 'cloud',
  })

  // NAS list for MikroTik backup tab
  const { data: nasData } = useQuery({
    queryKey: ['nas-list'],
    queryFn: () => nasApi.list().then((r) => r.data),
    enabled: activeTab === 'mikrotik',
  })

  const createMutation = useMutation({
    mutationFn: ({ type }) => backupApi.create({ type }),
    onSuccess: (res) => {
      toast.success(res.data.message)
      setShowCreateModal(false)
      queryClient.invalidateQueries(['backups'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to create backup'),
  })

  const deleteMutation = useMutation({
    mutationFn: (filename) => backupApi.delete(filename),
    onSuccess: () => {
      toast.success('Backup deleted')
      queryClient.invalidateQueries(['backups'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to delete'),
  })

  const restoreMutation = useMutation({
    mutationFn: ({ filename, sourceLicenseKey }) => backupApi.restore(filename, sourceLicenseKey),
    onSuccess: () => {
      toast.success('Backup restored successfully')
      setShowRestoreConfirm(null)
      setSourceLicenseKey('')
      queryClient.invalidateQueries(['backups'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to restore'),
  })

  const uploadMutation = useMutation({
    mutationFn: ({ file, restoreAfter }) => {
      const formData = new FormData()
      formData.append('file', file)
      return backupApi.upload(formData).then(res => ({ ...res, restoreAfter, filename: res.data?.data?.filename }))
    },
    onSuccess: (res) => {
      queryClient.invalidateQueries(['backups'])
      if (res.restoreAfter && res.filename) {
        toast.success('Backup uploaded -- opening restore...')
        setShowRestoreConfirm(res.filename)
        setSourceLicenseKey('')
      } else {
        toast.success(res.data?.message || 'Backup uploaded')
      }
      setUploadRestoreFile(null)
    },
    onError: (err) => {
      toast.error(err.response?.data?.message || 'Failed to upload')
      setUploadRestoreFile(null)
    },
  })

  // Schedule mutations
  const createScheduleMutation = useMutation({
    mutationFn: (data) => backupApi.createSchedule(data),
    onSuccess: () => {
      toast.success('Schedule created successfully')
      setShowScheduleModal(false)
      resetScheduleForm()
      queryClient.invalidateQueries(['backup-schedules'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to create schedule'),
  })

  const updateScheduleMutation = useMutation({
    mutationFn: ({ id, data }) => backupApi.updateSchedule(id, data),
    onSuccess: () => {
      toast.success('Schedule updated successfully')
      setShowScheduleModal(false)
      setEditingSchedule(null)
      resetScheduleForm()
      queryClient.invalidateQueries(['backup-schedules'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to update schedule'),
  })

  const deleteScheduleMutation = useMutation({
    mutationFn: (id) => backupApi.deleteSchedule(id),
    onSuccess: () => {
      toast.success('Schedule deleted')
      queryClient.invalidateQueries(['backup-schedules'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to delete schedule'),
  })

  const toggleScheduleMutation = useMutation({
    mutationFn: (id) => backupApi.toggleSchedule(id),
    onSuccess: (res) => {
      toast.success(res.data.message)
      queryClient.invalidateQueries(['backup-schedules'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to toggle schedule'),
  })

  const runNowMutation = useMutation({
    mutationFn: (id) => backupApi.runScheduleNow(id),
    onSuccess: () => {
      toast.success('Backup started')
      queryClient.invalidateQueries(['backup-logs'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to run backup'),
  })

  // Cloud backup mutations
  const cloudUploadMutation = useMutation({
    mutationFn: (filename) => backupApi.cloudUpload(filename),
    onSuccess: (res) => {
      toast.success(res.data?.message || 'Backup uploaded to cloud')
      setCloudUploadConfirm(null)
      queryClient.invalidateQueries(['cloud-backups'])
      queryClient.invalidateQueries(['cloud-usage'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to upload to cloud'),
  })

  const cloudDeleteMutation = useMutation({
    mutationFn: (backupId) => backupApi.cloudDelete(backupId),
    onSuccess: () => {
      toast.success('Cloud backup deleted')
      setCloudDeleteConfirm(null)
      queryClient.invalidateQueries(['cloud-backups'])
      queryClient.invalidateQueries(['cloud-usage'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to delete cloud backup'),
  })

  const handleCloudDownload = async (backupId) => {
    try {
      const { data } = await backupApi.cloudDownloadToken(backupId)
      if (data.success && data.url) {
        window.open(data.url, '_blank')
      } else {
        toast.error('Failed to get download token')
      }
    } catch (err) {
      toast.error(err.response?.data?.message || 'Failed to download cloud backup')
    }
  }

  const handleDownload = async (filename) => {
    try {
      const { data } = await backupApi.getDownloadToken(filename)
      if (data.success && data.url) {
        window.open(data.url, '_blank')
      } else {
        toast.error('Failed to get download token')
      }
    } catch (err) {
      toast.error(err.response?.data?.message || 'Failed to download backup')
    }
  }

  const handleUpload = (e) => {
    const file = e.target.files?.[0]
    if (file) {
      uploadMutation.mutate({ file, restoreAfter: false })
    }
    e.target.value = ''
  }

  const handleUploadAndRestore = (e) => {
    const file = e.target.files?.[0]
    if (file) {
      setUploadRestoreFile(file.name)
      uploadMutation.mutate({ file, restoreAfter: true })
    }
    e.target.value = ''
  }

  const resetScheduleForm = () => {
    setScheduleForm({
      name: '',
      backup_type: 'full',
      frequency: 'daily',
      day_of_week: 0,
      day_of_month: 1,
      time_of_day: '02:00',
      retention: 7,
      storage_type: 'local',
      ftp_enabled: false,
      ftp_host: '',
      ftp_port: 21,
      ftp_username: '',
      ftp_password: '',
      ftp_path: '/backups',
      is_enabled: true,
      cloud_enabled: false,
      include_mikrotik: false,
    })
  }

  const openScheduleModal = (schedule = null) => {
    if (schedule) {
      setEditingSchedule(schedule)
      setScheduleForm({
        name: schedule.name || '',
        backup_type: schedule.backup_type || 'full',
        frequency: schedule.frequency || 'daily',
        day_of_week: schedule.day_of_week || 0,
        day_of_month: schedule.day_of_month || 1,
        time_of_day: schedule.time_of_day || '02:00',
        retention: schedule.retention || 7,
        storage_type: schedule.storage_type || 'local',
        ftp_enabled: schedule.ftp_enabled || false,
        ftp_host: schedule.ftp_host || '',
        ftp_port: schedule.ftp_port || 21,
        ftp_username: schedule.ftp_username || '',
        ftp_password: schedule.ftp_password || '',
        ftp_path: schedule.ftp_path || '/backups',
        is_enabled: schedule.is_enabled !== false,
        cloud_enabled: schedule.cloud_enabled || false,
        include_mikrotik: schedule.include_mikrotik || false,
      })
    } else {
      setEditingSchedule(null)
      resetScheduleForm()
    }
    setShowScheduleModal(true)
  }

  const handleScheduleSubmit = (e) => {
    e.preventDefault()
    if (!scheduleForm.name) {
      toast.error('Schedule name is required')
      return
    }
    if (editingSchedule) {
      updateScheduleMutation.mutate({ id: editingSchedule.id, data: scheduleForm })
    } else {
      createScheduleMutation.mutate(scheduleForm)
    }
  }

  const handleTestFTP = async () => {
    if (!scheduleForm.ftp_host || !scheduleForm.ftp_username) {
      toast.error('FTP host and username are required')
      return
    }
    setTestingFTP(true)
    try {
      const res = await backupApi.testFTP({
        host: scheduleForm.ftp_host,
        port: scheduleForm.ftp_port,
        username: scheduleForm.ftp_username,
        password: scheduleForm.ftp_password,
        path: scheduleForm.ftp_path,
      })
      if (res.data.success) {
        toast.success('FTP connection successful')
      } else {
        toast.error(res.data.message || 'FTP connection failed')
      }
    } catch (err) {
      toast.error(err.response?.data?.message || 'FTP connection failed')
    } finally {
      setTestingFTP(false)
    }
  }

  const allBackups = data?.data || []
  const backups = allBackups.filter(b => b.type !== 'mikrotik')
  const schedules = schedulesData?.data || []
  const logs = logsData?.data || []
  const cloudBackups = cloudBackupsData?.data || []
  const cloudUsage = cloudUsageData?.data || null
  const usagePercent = cloudUsage ? Math.round((cloudUsage.used_bytes / cloudUsage.quota_bytes) * 100) : 0

  const tabDefs = [
    { id: 'manual', label: 'Manual Backups' },
    { id: 'scheduled', label: 'Scheduled' },
    { id: 'logs', label: 'Logs' },
    { id: 'cloud', label: 'Cloud Backup' },
    { id: 'mikrotik', label: 'MikroTik Backup' },
  ]

  return (
    <div style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Header toolbar */}
      <div className="wb-toolbar justify-between mb-2">
        <span className="text-[13px] font-semibold">Backup Management</span>
        <div className="flex items-center gap-1">
          {activeTab === 'manual' && (
            <>
              <button
                onClick={() => uploadRestoreInputRef.current?.click()}
                disabled={uploadMutation.isPending}
                className="btn btn-primary btn-sm flex items-center gap-1"
                title="Upload a backup file and immediately restore it"
              >
                <DocumentArrowUpIcon className="w-3.5 h-3.5" />
                {uploadMutation.isPending && uploadRestoreFile ? 'Uploading...' : 'Upload & Restore'}
              </button>
              <input
                ref={uploadRestoreInputRef}
                type="file"
                accept=".proisp.bak,.sql"
                onChange={handleUploadAndRestore}
                className="hidden"
              />
              <button
                onClick={() => fileInputRef.current?.click()}
                disabled={uploadMutation.isPending}
                className="btn btn-sm flex items-center gap-1"
                title="Upload a backup file to the server list"
              >
                <ArrowUpTrayIcon className="w-3.5 h-3.5" />
                Upload Only
              </button>
              <input
                ref={fileInputRef}
                type="file"
                accept=".proisp.bak,.sql"
                onChange={handleUpload}
                className="hidden"
              />
              <button
                onClick={() => refetch()}
                className="btn btn-sm flex items-center gap-1"
              >
                <ArrowPathIcon className="w-3.5 h-3.5" />
                Refresh
              </button>
              <button
                onClick={() => setShowCreateModal(true)}
                className="btn btn-success btn-sm flex items-center gap-1"
              >
                <CloudArrowUpIcon className="w-3.5 h-3.5" />
                Create Backup
              </button>
            </>
          )}
          {activeTab === 'scheduled' && (
            <>
              <button
                onClick={() => refetchSchedules()}
                className="btn btn-sm flex items-center gap-1"
              >
                <ArrowPathIcon className="w-3.5 h-3.5" />
                Refresh
              </button>
              <button
                onClick={() => openScheduleModal()}
                className="btn btn-primary btn-sm flex items-center gap-1"
              >
                <PlusIcon className="w-3.5 h-3.5" />
                Add Schedule
              </button>
            </>
          )}
          {activeTab === 'cloud' && (
            <button
              onClick={() => { refetchCloud(); queryClient.invalidateQueries(['cloud-usage']) }}
              className="btn btn-sm flex items-center gap-1"
            >
              <ArrowPathIcon className="w-3.5 h-3.5" />
              Refresh
            </button>
          )}
          {activeTab === 'mikrotik' && (
            <button
              onClick={() => refetch()}
              className="btn btn-sm flex items-center gap-1"
            >
              <ArrowPathIcon className="w-3.5 h-3.5" />
              Refresh
            </button>
          )}
        </div>
      </div>

      {/* WinBox Tabs */}
      <div className="flex items-end gap-0 mb-0">
        {tabDefs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={activeTab === tab.id ? 'wb-tab active' : 'wb-tab'}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab content area */}
      <div className="border border-[#a0a0a0] dark:border-[#555] bg-white dark:bg-[#2b2b2b] p-3" style={{ borderRadius: '0 2px 2px 2px' }}>

        {/* Manual Backups Tab */}
        {activeTab === 'manual' && (
          <div className="space-y-3">
            {/* Info row */}
            <div className="grid grid-cols-3 gap-2">
              <div className="wb-group">
                <div className="wb-group-title">Total Backups</div>
                <div className="wb-group-body">
                  <div className="text-[18px] font-bold text-gray-900 dark:text-[#e0e0e0]">{backups.length}</div>
                </div>
              </div>
              <div className="wb-group">
                <div className="wb-group-title">Total Size</div>
                <div className="wb-group-body">
                  <div className="text-[18px] font-bold text-gray-900 dark:text-[#e0e0e0]">
                    {formatBytes(backups.reduce((acc, b) => acc + (b.size || 0), 0))}
                  </div>
                </div>
              </div>
              <div className="wb-group">
                <div className="wb-group-title">Latest Backup</div>
                <div className="wb-group-body">
                  <div className="text-[18px] font-bold text-gray-900 dark:text-[#e0e0e0]">
                    {backups[0] ? formatDate(backups[0].created_at) : 'None'}
                  </div>
                </div>
              </div>
            </div>

            {/* Backups Table */}
            <div className="table-container">
              <table className="table">
                <thead>
                  <tr>
                    <th>Filename</th>
                    <th>Type</th>
                    <th>Size</th>
                    <th>Created At</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {isLoading ? (
                    <tr>
                      <td colSpan={5} className="text-center py-4">Loading...</td>
                    </tr>
                  ) : backups.length === 0 ? (
                    <tr>
                      <td colSpan={5} className="text-center py-4 text-gray-500 dark:text-[#aaa]">
                        No backups found. Create your first backup to get started.
                      </td>
                    </tr>
                  ) : (
                    backups.map((backup) => (
                      <tr key={backup.filename}>
                        <td className="font-semibold">{backup.filename}</td>
                        <td>
                          <span className={clsx('badge', backup.type === 'full' ? 'badge-success' : backup.type === 'data' ? 'badge-info' : backup.type === 'mikrotik' ? 'badge-purple' : 'badge-warning')}>
                            {backup.type === 'mikrotik' ? 'MikroTik' : (backup.type || 'full')}
                          </span>
                        </td>
                        <td>{formatBytes(backup.size)}</td>
                        <td>{formatDateTime(backup.created_at)}</td>
                        <td>
                          <div className="flex items-center gap-0.5">
                            <button onClick={() => handleDownload(backup.filename)} className="btn btn-sm btn-primary" title="Download" style={{ padding: '1px 4px' }}>
                              <ArrowDownTrayIcon className="w-3.5 h-3.5" />
                            </button>
                            {backup.type !== 'mikrotik' && (
                              <button onClick={() => setShowRestoreConfirm(backup.filename)} className="btn btn-sm btn-success" title="Restore" style={{ padding: '1px 4px' }}>
                                <ArrowPathIcon className="w-3.5 h-3.5" />
                              </button>
                            )}
                            {backup.type !== 'mikrotik' && (
                              <button onClick={() => setCloudUploadConfirm(backup.filename)} className="btn btn-sm" title="Upload to Cloud" style={{ padding: '1px 4px' }}>
                                <CloudArrowUpIcon className="w-3.5 h-3.5" />
                              </button>
                            )}
                            <button
                              onClick={() => {
                                if (confirm('Are you sure you want to delete this backup?')) {
                                  deleteMutation.mutate(backup.filename)
                                }
                              }}
                              className="btn btn-sm btn-danger"
                              title="Delete"
                              style={{ padding: '1px 4px' }}
                            >
                              <TrashIcon className="w-3.5 h-3.5" />
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {/* Scheduled Backups Tab */}
        {activeTab === 'scheduled' && (
          <div className="space-y-2">
            {schedulesLoading ? (
              <div className="text-center py-4 text-[12px] text-gray-500 dark:text-[#aaa]">Loading...</div>
            ) : schedules.length === 0 ? (
              <div className="text-center py-6 text-[12px] text-gray-500 dark:text-[#aaa]">
                <p>No backup schedules configured.</p>
                <p className="text-[11px] mt-1">Create a schedule to automate your backups.</p>
                <button
                  onClick={() => openScheduleModal()}
                  className="btn btn-primary btn-sm mt-2"
                >
                  Create Schedule
                </button>
              </div>
            ) : (
              <div className="table-container">
                <table className="table">
                  <thead>
                    <tr>
                      <th>Name</th>
                      <th>Type</th>
                      <th>Frequency</th>
                      <th>Retention</th>
                      <th>Storage</th>
                      <th>Status</th>
                      <th>Last Run</th>
                      <th>Next Run</th>
                      <th>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {schedules.map((schedule) => (
                      <tr key={schedule.id}>
                        <td className="font-semibold">{schedule.name}</td>
                        <td>
                          {schedule.backup_type === 'mikrotik' ? (
                            <span className="badge badge-purple">MikroTik</span>
                          ) : (
                            <>
                              <span className={clsx('badge', schedule.backup_type === 'full' ? 'badge-info' : schedule.backup_type === 'data' ? 'badge-warning' : 'badge-secondary')}>
                                {schedule.backup_type}
                              </span>
                              {schedule.include_mikrotik && <span className="badge badge-purple ml-1">+MT</span>}
                            </>
                          )}
                          {schedule.cloud_enabled && <span className="badge badge-cyan ml-1">Cloud</span>}
                        </td>
                        <td>
                          {schedule.frequency === 'daily'
                            ? 'Daily'
                            : schedule.frequency === 'weekly'
                            ? `Weekly (${DAYS_OF_WEEK[schedule.day_of_week]?.substring(0,3)})`
                            : `Monthly (${schedule.day_of_month})`}
                          {' '}{schedule.time_of_day || '02:00'}
                        </td>
                        <td>{schedule.retention}d</td>
                        <td>
                          {schedule.storage_type === 'both' ? 'Local+FTP'
                            : schedule.storage_type === 'ftp' ? 'FTP'
                            : schedule.storage_type === 'cloud' ? 'Cloud'
                            : schedule.storage_type === 'local+cloud' ? 'Local+Cloud'
                            : 'Local'}
                        </td>
                        <td>
                          <span className={clsx('badge', schedule.is_enabled ? 'badge-success' : 'badge-secondary')}>
                            {schedule.is_enabled ? 'Active' : 'Disabled'}
                          </span>
                        </td>
                        <td>
                          {schedule.last_run_at ? (
                            <span>
                              {formatDateTime(schedule.last_run_at)}
                              {schedule.last_status && (
                                <span className={clsx('ml-1', schedule.last_status === 'success' ? 'text-[#4CAF50]' : schedule.last_status === 'running' ? 'text-[#2196F3]' : 'text-[#f44336]')}>
                                  ({schedule.last_status})
                                </span>
                              )}
                            </span>
                          ) : '-'}
                        </td>
                        <td>{schedule.next_run_at ? formatDateTime(schedule.next_run_at) : '-'}</td>
                        <td>
                          <div className="flex items-center gap-0.5">
                            <button onClick={() => runNowMutation.mutate(schedule.id)} disabled={runNowMutation.isLoading} className="btn btn-sm btn-success" title="Run Now" style={{ padding: '1px 4px' }}>
                              <PlayIcon className="w-3.5 h-3.5" />
                            </button>
                            <button onClick={() => openScheduleModal(schedule)} className="btn btn-sm btn-primary" title="Edit" style={{ padding: '1px 4px' }}>
                              <PencilIcon className="w-3.5 h-3.5" />
                            </button>
                            <button
                              onClick={() => toggleScheduleMutation.mutate(schedule.id)}
                              disabled={toggleScheduleMutation.isLoading}
                              className={clsx('btn btn-sm', schedule.is_enabled ? 'btn-success' : '')}
                              title={schedule.is_enabled ? 'Disable' : 'Enable'}
                              style={{ padding: '1px 4px' }}
                            >
                              {schedule.is_enabled ? <CheckCircleIcon className="w-3.5 h-3.5" /> : <XCircleIcon className="w-3.5 h-3.5" />}
                            </button>
                            <button
                              onClick={() => {
                                if (confirm('Are you sure you want to delete this schedule?')) {
                                  deleteScheduleMutation.mutate(schedule.id)
                                }
                              }}
                              className="btn btn-sm btn-danger"
                              title="Delete"
                              style={{ padding: '1px 4px' }}
                            >
                              <TrashIcon className="w-3.5 h-3.5" />
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )}

        {/* Backup Logs Tab */}
        {activeTab === 'logs' && (
          <div className="table-container">
            <table className="table">
              <thead>
                <tr>
                  <th>Schedule</th>
                  <th>Type</th>
                  <th>Status</th>
                  <th>File</th>
                  <th>Size</th>
                  <th>Duration</th>
                  <th>Started At</th>
                </tr>
              </thead>
              <tbody>
                {logsLoading ? (
                  <tr>
                    <td colSpan={7} className="text-center py-4">Loading...</td>
                  </tr>
                ) : logs.length === 0 ? (
                  <tr>
                    <td colSpan={7} className="text-center py-4 text-gray-500 dark:text-[#aaa]">
                      No backup logs found.
                    </td>
                  </tr>
                ) : (
                  logs.map((log) => (
                    <tr key={log.id}>
                      <td>{log.schedule_name || 'Manual'}</td>
                      <td><span className="badge badge-info">{log.backup_type}</span></td>
                      <td>
                        <span className={clsx('badge', log.status === 'success' ? 'badge-success' : log.status === 'running' ? 'badge-info' : 'badge-danger')}>
                          {log.status}
                        </span>
                      </td>
                      <td>{log.filename || '-'}</td>
                      <td>{log.file_size ? formatBytes(log.file_size) : '-'}</td>
                      <td>{log.duration ? `${log.duration}s` : '-'}</td>
                      <td>{formatDateTime(log.started_at)}</td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        )}

        {/* Cloud Backup Tab */}
        {activeTab === 'cloud' && (
          <div className="space-y-3">
            {/* Storage Usage */}
            <div className="wb-group">
              <div className="wb-group-title flex items-center justify-between">
                <span>ProxPanel Cloud Storage</span>
                <span className="badge badge-info">{cloudUsage?.tier?.toUpperCase() || 'FREE'}</span>
              </div>
              <div className="wb-group-body">
                <div className="flex items-center justify-between text-[12px] mb-1">
                  <span className="text-gray-700 dark:text-[#ccc]">Usage</span>
                  <span className="text-gray-500 dark:text-[#aaa]">
                    {formatBytes(cloudUsage?.used_bytes || 0)} / {formatBytes(cloudUsage?.quota_bytes || 524288000)}
                  </span>
                </div>
                <div className="wb-usage-bar">
                  <div
                    className="wb-usage-bar-fill"
                    style={{
                      width: `${Math.min(usagePercent, 100)}%`,
                      backgroundColor: usagePercent > 90 ? '#f44336' : usagePercent > 70 ? '#FF9800' : '#2196F3',
                    }}
                  />
                </div>
                <div className="text-[11px] text-gray-500 dark:text-[#aaa] mt-1">
                  {cloudUsage?.backup_count || cloudBackups.length} backups stored - Free tier: 500 MB
                </div>
              </div>
            </div>

            {/* Cloud Backups Table */}
            <div className="table-container">
              <table className="table">
                <thead>
                  <tr>
                    <th>Filename</th>
                    <th>Type</th>
                    <th>Size</th>
                    <th>Uploaded</th>
                    <th>Expires</th>
                    <th>Downloads</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {cloudLoading ? (
                    <tr>
                      <td colSpan={7} className="text-center py-4">Loading...</td>
                    </tr>
                  ) : cloudBackups.length === 0 ? (
                    <tr>
                      <td colSpan={7} className="text-center py-4 text-gray-500 dark:text-[#aaa]">
                        No cloud backups yet. Upload a local backup to get started.
                      </td>
                    </tr>
                  ) : (
                    cloudBackups.map((backup) => (
                      <tr key={backup.id}>
                        <td className="font-semibold">{backup.filename}</td>
                        <td>
                          <span className={clsx('badge', backup.type === 'full' ? 'badge-success' : backup.type === 'data' ? 'badge-info' : backup.type === 'mikrotik' ? 'badge-purple' : 'badge-warning')}>
                            {backup.type === 'mikrotik' ? 'MikroTik' : (backup.type || 'full')}
                          </span>
                        </td>
                        <td>{formatBytes(backup.size || 0)}</td>
                        <td>{backup.uploaded_at ? formatDateTime(backup.uploaded_at) : formatDateTime(backup.created_at)}</td>
                        <td>
                          {backup.expires_at ? (
                            <span className={new Date(backup.expires_at) < new Date() ? 'text-[#f44336]' : ''}>
                              {formatDate(backup.expires_at)}
                            </span>
                          ) : 'Never'}
                        </td>
                        <td>{backup.download_count || 0}</td>
                        <td>
                          <div className="flex items-center gap-0.5">
                            <button onClick={() => handleCloudDownload(backup.backup_id)} className="btn btn-sm btn-primary" title="Download" style={{ padding: '1px 4px' }}>
                              <ArrowDownTrayIcon className="w-3.5 h-3.5" />
                            </button>
                            <button onClick={() => setCloudDeleteConfirm(backup)} className="btn btn-sm btn-danger" title="Delete" style={{ padding: '1px 4px' }}>
                              <TrashIcon className="w-3.5 h-3.5" />
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>

            {/* Upload from Local */}
            {backups.length > 0 && (
              <div className="wb-group">
                <div className="wb-group-title">Upload Local Backup to Cloud</div>
                <div className="wb-group-body space-y-1">
                  {backups.map((backup) => (
                    <div key={backup.filename} className="flex items-center justify-between py-1 px-2 border border-[#ddd] dark:border-[#555] bg-[#f9f9f9] dark:bg-[#333]" style={{ borderRadius: '2px' }}>
                      <div className="flex items-center gap-2 text-[12px] min-w-0">
                        <span className="font-semibold text-gray-700 dark:text-[#ccc] truncate">{backup.filename}</span>
                        <span className="text-gray-400 dark:text-[#888] flex-shrink-0">{formatBytes(backup.size || 0)}</span>
                      </div>
                      <button
                        onClick={() => setCloudUploadConfirm(backup.filename)}
                        className="btn btn-sm flex items-center gap-1 flex-shrink-0 ml-2"
                      >
                        <CloudArrowUpIcon className="w-3 h-3" />
                        Upload
                      </button>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}

        {/* MikroTik Backup Tab */}
        {activeTab === 'mikrotik' && (
          <div className="space-y-3">
            {/* FTP Requirement Notice */}
            <div className="flex items-start gap-2 p-2 bg-amber-50 dark:bg-amber-900/30 border border-amber-300 dark:border-amber-700 text-[12px]" style={{ borderRadius: '2px' }}>
              <ExclamationTriangleIcon className="w-4 h-4 text-amber-600 dark:text-amber-400 flex-shrink-0 mt-0.5" />
              <div>
                <p className="font-semibold text-amber-800 dark:text-amber-300">FTP service must be enabled on MikroTik</p>
                <p className="text-amber-700 dark:text-amber-400 mt-0.5">
                  This feature uses FTP to download the router configuration. Enable FTP on your MikroTik:
                </p>
                <code className="block mt-1 px-1.5 py-0.5 bg-amber-100 dark:bg-amber-900/50 text-amber-900 dark:text-amber-200 text-[11px]" style={{ borderRadius: '2px' }}>
                  /ip service enable ftp
                </code>
                <p className="text-amber-700 dark:text-amber-400 mt-1">
                  If your FTP port is not the default (21), you can change it in <strong>NAS/Routers</strong> settings (FTP Port field).
                </p>
              </div>
            </div>

            {/* NAS Selection */}
            <div className="wb-group">
              <div className="wb-group-title flex items-center justify-between">
                <span>Select NAS Devices</span>
                <div className="flex gap-2">
                  <button
                    onClick={() => {
                      const allNas = (nasData?.data || []).filter(n => n.api_username || n.has_api_password)
                      setSelectedNasIds(allNas.map(n => n.id))
                    }}
                    className="text-[10px] text-[#316AC5] hover:underline cursor-pointer"
                  >
                    Select All
                  </button>
                  <button
                    onClick={() => setSelectedNasIds([])}
                    className="text-[10px] text-[#316AC5] hover:underline cursor-pointer"
                  >
                    Clear
                  </button>
                </div>
              </div>
              <div className="wb-group-body">
                {(() => {
                  const nasList = (nasData?.data || []).filter(n => n.api_username || n.has_api_password)
                  if (nasList.length === 0) {
                    return (
                      <p className="text-[12px] text-gray-500 dark:text-[#aaa]">
                        No NAS devices with API credentials found. Configure API credentials in NAS settings first.
                      </p>
                    )
                  }
                  return (
                    <div className="space-y-1">
                      {nasList.map((nas) => (
                        <label
                          key={nas.id}
                          className={clsx(
                            'flex items-center gap-2 p-1.5 border cursor-pointer text-[12px]',
                            selectedNasIds.includes(nas.id)
                              ? 'border-[#316AC5] bg-[#e8eef8] dark:bg-[#2a3a5a]'
                              : 'border-[#ddd] dark:border-[#555] hover:bg-[#f5f5f5] dark:hover:bg-[#333]'
                          )}
                          style={{ borderRadius: '2px' }}
                        >
                          <input
                            type="checkbox"
                            checked={selectedNasIds.includes(nas.id)}
                            onChange={(e) => {
                              if (e.target.checked) {
                                setSelectedNasIds([...selectedNasIds, nas.id])
                              } else {
                                setSelectedNasIds(selectedNasIds.filter(id => id !== nas.id))
                              }
                            }}
                            className="w-3.5 h-3.5 accent-[#316AC5]"
                          />
                          <ServerIcon className="w-3.5 h-3.5 text-gray-500 dark:text-[#aaa]" />
                          <span className="font-semibold">{nas.name}</span>
                          <span className="text-gray-400 dark:text-[#888]">({nas.ip_address})</span>
                        </label>
                      ))}
                    </div>
                  )
                })()}
                <button
                  onClick={async () => {
                    setMikrotikCreating(true)
                    try {
                      const res = await backupApi.createMikrotik(selectedNasIds.length > 0 ? selectedNasIds : undefined)
                      toast.success(res.data.message || 'MikroTik backup created')
                      queryClient.invalidateQueries(['backups'])
                      setSelectedNasIds([])
                    } catch (err) {
                      toast.error(err.response?.data?.message || 'Failed to create MikroTik backup')
                    } finally {
                      setMikrotikCreating(false)
                    }
                  }}
                  disabled={mikrotikCreating}
                  className="btn btn-primary btn-sm mt-2 flex items-center gap-1"
                >
                  <ServerIcon className="w-3.5 h-3.5" />
                  {mikrotikCreating ? 'Creating...' : 'Create MikroTik Backup'}
                </button>
              </div>
            </div>

            {/* MikroTik Schedules */}
            {(() => {
              const mtSchedules = (schedulesData?.data || []).filter(s => s.backup_type === 'mikrotik' || s.include_mikrotik)
              return (
                <div className="wb-group">
                  <div className="wb-group-title flex items-center justify-between">
                    <span>Scheduled MikroTik Backups</span>
                    <div className="flex gap-2">
                      <button
                        onClick={() => {
                          resetScheduleForm()
                          setScheduleForm(prev => ({ ...prev, backup_type: 'mikrotik', name: 'MikroTik Backup' }))
                          setEditingSchedule(null)
                          setShowScheduleModal(true)
                        }}
                        className="text-[10px] text-[#316AC5] hover:underline cursor-pointer"
                      >
                        + Create Schedule
                      </button>
                      <button
                        onClick={() => { setActiveTab('scheduled') }}
                        className="text-[10px] text-[#316AC5] hover:underline cursor-pointer"
                      >
                        Manage Schedules
                      </button>
                    </div>
                  </div>
                  <div className="wb-group-body">
                    {mtSchedules.length === 0 ? (
                      <p className="text-[12px] text-gray-500 dark:text-[#aaa]">
                        No MikroTik backup schedules yet. Click "+ Create Schedule" above to create one.
                      </p>
                    ) : (
                      <div className="space-y-1">
                        {mtSchedules.map((schedule) => (
                          <div
                            key={schedule.id}
                            className="flex items-center justify-between p-1.5 border border-[#ddd] dark:border-[#555] text-[12px]"
                            style={{ borderRadius: '2px' }}
                          >
                            <div className="flex items-center gap-2">
                              <CalendarDaysIcon className="w-3.5 h-3.5 text-gray-500 dark:text-[#aaa]" />
                              <span className="font-semibold">{schedule.name}</span>
                              {schedule.backup_type === 'mikrotik' ? (
                                <span className="badge badge-purple">MikroTik</span>
                              ) : (
                                <>
                                  <span className="badge badge-info">{schedule.backup_type}</span>
                                  <span className="badge badge-purple">+MT</span>
                                </>
                              )}
                              {schedule.cloud_enabled && <span className="badge badge-cyan">Cloud</span>}
                              <span className="text-gray-400 dark:text-[#888]">
                                {schedule.frequency === 'daily' ? 'Daily' : schedule.frequency === 'weekly' ? `Weekly` : `Monthly`}
                                {' '}{schedule.time_of_day || '02:00'}
                              </span>
                            </div>
                            <div className="flex items-center gap-2">
                              <span className={clsx('badge', schedule.is_enabled ? 'badge-success' : 'badge-secondary')}>
                                {schedule.is_enabled ? 'Active' : 'Disabled'}
                              </span>
                              {schedule.next_run_at && (
                                <span className="text-gray-400 dark:text-[#888] text-[10px]">
                                  Next: {formatDateTime(schedule.next_run_at)}
                                </span>
                              )}
                              <button onClick={() => openScheduleModal(schedule)} className="btn btn-sm btn-primary" title="Edit" style={{ padding: '1px 4px' }}>
                                <PencilIcon className="w-3 h-3" />
                              </button>
                              <button onClick={() => runNowMutation.mutate(schedule.id)} disabled={runNowMutation.isLoading} className="btn btn-sm btn-success" title="Run Now" style={{ padding: '1px 4px' }}>
                                <PlayIcon className="w-3 h-3" />
                              </button>
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                </div>
              )
            })()}

            {/* MikroTik Backups Table */}
            <div className="table-container">
              <table className="table">
                <thead>
                  <tr>
                    <th>Filename</th>
                    <th>Size</th>
                    <th>Created At</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {isLoading ? (
                    <tr>
                      <td colSpan={4} className="text-center py-4">Loading...</td>
                    </tr>
                  ) : allBackups.filter(b => b.type === 'mikrotik').length === 0 ? (
                    <tr>
                      <td colSpan={4} className="text-center py-4 text-gray-500 dark:text-[#aaa]">
                        No MikroTik backups yet. Select NAS devices above and create a backup.
                      </td>
                    </tr>
                  ) : (
                    allBackups.filter(b => b.type === 'mikrotik').map((backup) => (
                      <tr key={backup.filename}>
                        <td className="font-semibold">{backup.filename}</td>
                        <td>{formatBytes(backup.size)}</td>
                        <td>{formatDateTime(backup.created_at)}</td>
                        <td>
                          <div className="flex items-center gap-0.5">
                            <button onClick={() => handleDownload(backup.filename)} className="btn btn-sm btn-primary" title="Download" style={{ padding: '1px 4px' }}>
                              <ArrowDownTrayIcon className="w-3.5 h-3.5" />
                            </button>
                            <button onClick={() => setCloudUploadConfirm(backup.filename)} className="btn btn-sm" title="Upload to Cloud" style={{ padding: '1px 4px' }}>
                              <CloudArrowUpIcon className="w-3.5 h-3.5" />
                            </button>
                            <button
                              onClick={() => {
                                if (confirm('Are you sure you want to delete this MikroTik backup?')) {
                                  deleteMutation.mutate(backup.filename)
                                }
                              }}
                              className="btn btn-sm btn-danger"
                              title="Delete"
                              style={{ padding: '1px 4px' }}
                            >
                              <TrashIcon className="w-3.5 h-3.5" />
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </div>

      {/* Cloud Upload Confirmation Modal */}
      {cloudUploadConfirm && (
        <div className="modal-overlay">
          <div className="modal modal-sm">
            <div className="modal-header">Upload to Cloud</div>
            <div className="modal-body">
              <p className="mb-2">Upload this backup to ProxPanel Cloud Storage?</p>
              <p className="font-mono bg-[#e0e0e0] dark:bg-[#444] px-2 py-1 text-[11px]" style={{ borderRadius: '2px' }}>{cloudUploadConfirm}</p>
            </div>
            <div className="modal-footer">
              <button onClick={() => setCloudUploadConfirm(null)} className="btn btn-sm">Cancel</button>
              <button
                onClick={() => cloudUploadMutation.mutate(cloudUploadConfirm)}
                disabled={cloudUploadMutation.isLoading}
                className="btn btn-primary btn-sm"
              >
                {cloudUploadMutation.isLoading ? 'Uploading...' : 'Upload to Cloud'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Cloud Delete Confirmation Modal */}
      {cloudDeleteConfirm && (
        <div className="modal-overlay">
          <div className="modal modal-sm">
            <div className="modal-header" style={{ background: 'linear-gradient(to bottom, #c44, #922)' }}>Delete Cloud Backup</div>
            <div className="modal-body">
              <p className="mb-2">Are you sure you want to permanently delete this cloud backup?</p>
              <p className="font-mono bg-[#e0e0e0] dark:bg-[#444] px-2 py-1 text-[11px]" style={{ borderRadius: '2px' }}>{cloudDeleteConfirm.filename}</p>
            </div>
            <div className="modal-footer">
              <button onClick={() => setCloudDeleteConfirm(null)} className="btn btn-sm">Cancel</button>
              <button
                onClick={() => cloudDeleteMutation.mutate(cloudDeleteConfirm.backup_id)}
                disabled={cloudDeleteMutation.isLoading}
                className="btn btn-danger btn-sm"
              >
                {cloudDeleteMutation.isLoading ? 'Deleting...' : 'Delete from Cloud'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Create Backup Modal */}
      {showCreateModal && (
        <div className="modal-overlay">
          <div className="modal modal-sm">
            <div className="modal-header">Create Backup</div>
            <div className="modal-body">
              <p className="mb-2">Select backup type:</p>
              <div className="space-y-1">
                {[
                  { value: 'full', label: 'Full Backup', desc: 'Complete database backup (all tables)' },
                  { value: 'data', label: 'Data Only', desc: 'Subscribers, services, transactions, sessions' },
                  { value: 'config', label: 'Config Only', desc: 'Users, settings, templates, rules' },
                ].map((type) => (
                  <label
                    key={type.value}
                    className={clsx(
                      'flex items-start p-2 border cursor-pointer',
                      backupType === type.value
                        ? 'border-[#316AC5] bg-[#e8eef8] dark:bg-[#2a3a5a]'
                        : 'border-[#ccc] dark:border-[#555] hover:bg-[#f5f5f5] dark:hover:bg-[#333]'
                    )}
                    style={{ borderRadius: '2px' }}
                  >
                    <input
                      type="radio"
                      name="backupType"
                      value={type.value}
                      checked={backupType === type.value}
                      onChange={(e) => setBackupType(e.target.value)}
                      className="mt-0.5 mr-2"
                    />
                    <div>
                      <p className="font-semibold text-[12px]">{type.label}</p>
                      <p className="text-[11px] text-gray-500 dark:text-[#aaa]">{type.desc}</p>
                    </div>
                  </label>
                ))}
              </div>
            </div>
            <div className="modal-footer">
              <button onClick={() => setShowCreateModal(false)} className="btn btn-sm">Cancel</button>
              <button
                onClick={() => createMutation.mutate({ type: backupType })}
                disabled={createMutation.isLoading}
                className="btn btn-primary btn-sm"
              >
                {createMutation.isLoading ? 'Creating...' : 'Create Backup'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Restore Confirmation Modal */}
      {showRestoreConfirm && (
        <div className="modal-overlay">
          <div className="modal" style={{ maxWidth: '450px' }}>
            <div className="modal-header" style={{ background: 'linear-gradient(to bottom, #b8860b, #8b6914)' }}>Confirm Restore</div>
            <div className="modal-body">
              <p className="mb-2">Are you sure you want to restore from this backup? This will overwrite existing data.</p>
              <p className="mb-3">
                File: <span className="font-mono font-semibold">{showRestoreConfirm}</span>
              </p>
              <div className="mb-2">
                <label className="block text-[12px] font-semibold text-gray-700 dark:text-[#ccc] mb-1">
                  Source License Key (optional - auto-detected)
                </label>
                <input
                  type="text"
                  value={sourceLicenseKey}
                  onChange={(e) => setSourceLicenseKey(e.target.value)}
                  placeholder="Auto-detected from backup file (leave empty)"
                  className="input w-full"
                />
                <p className="text-[11px] text-[#4CAF50] mt-1">
                  System automatically reads the license key from the backup file - no manual input needed!
                </p>
                <p className="text-[11px] text-gray-500 dark:text-[#aaa] mt-0.5">
                  Only fill this if you want to override the auto-detected license key.
                </p>
              </div>
            </div>
            <div className="modal-footer">
              <button
                onClick={() => { setShowRestoreConfirm(null); setSourceLicenseKey('') }}
                className="btn btn-sm"
              >
                Cancel
              </button>
              <button
                onClick={() => restoreMutation.mutate({
                  filename: showRestoreConfirm,
                  sourceLicenseKey: sourceLicenseKey.trim()
                })}
                disabled={restoreMutation.isLoading}
                className="btn btn-danger btn-sm"
              >
                {restoreMutation.isLoading ? 'Restoring...' : 'Restore Backup'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Schedule Modal */}
      {showScheduleModal && (
        <div className="modal-overlay">
          <div className="modal modal-lg" style={{ maxWidth: '600px' }}>
            <div className="modal-header">
              {editingSchedule ? 'Edit Schedule' : 'Create Backup Schedule'}
            </div>
            <div className="modal-body" style={{ maxHeight: '70vh', overflowY: 'auto' }}>
              <form id="scheduleForm" onSubmit={handleScheduleSubmit} className="space-y-3">
                {/* Basic Settings */}
                <div className="wb-group">
                  <div className="wb-group-title">Basic Settings</div>
                  <div className="wb-group-body space-y-2">
                    <div>
                      <label className="block text-[12px] font-semibold text-gray-700 dark:text-[#ccc] mb-0.5">Schedule Name *</label>
                      <input
                        type="text"
                        value={scheduleForm.name}
                        onChange={(e) => setScheduleForm({ ...scheduleForm, name: e.target.value })}
                        placeholder="e.g., Daily Full Backup"
                        className="input w-full"
                        required
                      />
                    </div>

                    <div className="grid grid-cols-2 gap-2">
                      {scheduleForm.backup_type !== 'mikrotik' ? (
                      <div>
                        <label className="block text-[12px] font-semibold text-gray-700 dark:text-[#ccc] mb-0.5">Backup Type</label>
                        <select
                          value={scheduleForm.backup_type}
                          onChange={(e) => setScheduleForm({ ...scheduleForm, backup_type: e.target.value })}
                          className="input w-full"
                        >
                          <option value="full">Full Backup</option>
                          <option value="data">Data Only</option>
                          <option value="config">Config Only</option>
                        </select>
                      </div>
                      ) : (
                      <div>
                        <label className="block text-[12px] font-semibold text-gray-700 dark:text-[#ccc] mb-0.5">Backup Type</label>
                        <div className="input w-full bg-[#f0f0f0] dark:bg-[#444] flex items-center gap-1">
                          <ServerIcon className="w-3.5 h-3.5 text-purple-600" />
                          <span className="font-semibold text-purple-700 dark:text-purple-300">MikroTik Only</span>
                        </div>
                      </div>
                      )}
                      <div>
                        <label className="block text-[12px] font-semibold text-gray-700 dark:text-[#ccc] mb-0.5">Frequency</label>
                        <select
                          value={scheduleForm.frequency}
                          onChange={(e) => setScheduleForm({ ...scheduleForm, frequency: e.target.value })}
                          className="input w-full"
                        >
                          <option value="daily">Daily</option>
                          <option value="weekly">Weekly</option>
                          <option value="monthly">Monthly</option>
                        </select>
                      </div>
                    </div>

                    {scheduleForm.frequency === 'weekly' && (
                      <div>
                        <label className="block text-[12px] font-semibold text-gray-700 dark:text-[#ccc] mb-0.5">Day of Week</label>
                        <select
                          value={scheduleForm.day_of_week}
                          onChange={(e) => setScheduleForm({ ...scheduleForm, day_of_week: parseInt(e.target.value) })}
                          className="input w-full"
                        >
                          {DAYS_OF_WEEK.map((day, idx) => (
                            <option key={idx} value={idx}>{day}</option>
                          ))}
                        </select>
                      </div>
                    )}

                    {scheduleForm.frequency === 'monthly' && (
                      <div>
                        <label className="block text-[12px] font-semibold text-gray-700 dark:text-[#ccc] mb-0.5">Day of Month</label>
                        <select
                          value={scheduleForm.day_of_month}
                          onChange={(e) => setScheduleForm({ ...scheduleForm, day_of_month: parseInt(e.target.value) })}
                          className="input w-full"
                        >
                          {Array.from({ length: 28 }, (_, i) => i + 1).map((day) => (
                            <option key={day} value={day}>{day}</option>
                          ))}
                        </select>
                      </div>
                    )}

                    <div className="grid grid-cols-2 gap-2">
                      <div>
                        <label className="block text-[12px] font-semibold text-gray-700 dark:text-[#ccc] mb-0.5">Time of Day</label>
                        <input
                          type="time"
                          value={scheduleForm.time_of_day}
                          onChange={(e) => setScheduleForm({ ...scheduleForm, time_of_day: e.target.value })}
                          className="input w-full"
                        />
                      </div>
                      <div>
                        <label className="block text-[12px] font-semibold text-gray-700 dark:text-[#ccc] mb-0.5">Retention (days)</label>
                        <input
                          type="number"
                          min="1"
                          max="365"
                          value={scheduleForm.retention}
                          onChange={(e) => setScheduleForm({ ...scheduleForm, retention: parseInt(e.target.value) })}
                          className="input w-full"
                        />
                      </div>
                    </div>
                  </div>
                </div>

                {/* Storage Settings */}
                <div className="wb-group">
                  <div className="wb-group-title">Storage Settings</div>
                  <div className="wb-group-body space-y-2">
                    <div>
                      <label className="block text-[12px] font-semibold text-gray-700 dark:text-[#ccc] mb-0.5">Storage Type</label>
                      <select
                        value={scheduleForm.storage_type}
                        onChange={(e) => {
                          const val = e.target.value
                          setScheduleForm({
                            ...scheduleForm,
                            storage_type: val,
                            ftp_enabled: val === 'ftp' || val === 'both'
                          })
                        }}
                        className="input w-full"
                      >
                        <option value="local">Local Only</option>
                        <option value="ftp">FTP Only</option>
                        <option value="both">Local + FTP</option>
                        <option value="cloud">ProxPanel Cloud</option>
                        <option value="local+cloud">Local + Cloud</option>
                      </select>
                    </div>

                    {/* FTP Settings */}
                    {(scheduleForm.storage_type === 'ftp' || scheduleForm.storage_type === 'both') && (
                      <div className="border border-[#ccc] dark:border-[#555] bg-[#f9f9f9] dark:bg-[#333] p-2 space-y-2" style={{ borderRadius: '2px' }}>
                        <div className="flex items-center justify-between">
                          <span className="text-[12px] font-semibold text-gray-700 dark:text-[#ccc]">FTP Settings</span>
                          <button
                            type="button"
                            onClick={handleTestFTP}
                            disabled={testingFTP}
                            className="btn btn-sm"
                          >
                            {testingFTP ? 'Testing...' : 'Test Connection'}
                          </button>
                        </div>

                        <div className="grid grid-cols-2 gap-2">
                          <div>
                            <label className="block text-[11px] font-semibold text-gray-600 dark:text-[#bbb] mb-0.5">FTP Host *</label>
                            <input
                              type="text"
                              value={scheduleForm.ftp_host}
                              onChange={(e) => setScheduleForm({ ...scheduleForm, ftp_host: e.target.value })}
                              placeholder="ftp.example.com"
                              className="input w-full"
                            />
                          </div>
                          <div>
                            <label className="block text-[11px] font-semibold text-gray-600 dark:text-[#bbb] mb-0.5">FTP Port</label>
                            <input
                              type="number"
                              value={scheduleForm.ftp_port}
                              onChange={(e) => setScheduleForm({ ...scheduleForm, ftp_port: parseInt(e.target.value) })}
                              className="input w-full"
                            />
                          </div>
                        </div>

                        <div className="grid grid-cols-2 gap-2">
                          <div>
                            <label className="block text-[11px] font-semibold text-gray-600 dark:text-[#bbb] mb-0.5">Username *</label>
                            <input
                              type="text"
                              value={scheduleForm.ftp_username}
                              onChange={(e) => setScheduleForm({ ...scheduleForm, ftp_username: e.target.value })}
                              className="input w-full"
                            />
                          </div>
                          <div>
                            <label className="block text-[11px] font-semibold text-gray-600 dark:text-[#bbb] mb-0.5">Password</label>
                            <input
                              type="password"
                              value={scheduleForm.ftp_password}
                              onChange={(e) => setScheduleForm({ ...scheduleForm, ftp_password: e.target.value })}
                              placeholder={editingSchedule ? '(unchanged)' : ''}
                              className="input w-full"
                            />
                          </div>
                        </div>

                        <div>
                          <label className="block text-[11px] font-semibold text-gray-600 dark:text-[#bbb] mb-0.5">Remote Path</label>
                          <input
                            type="text"
                            value={scheduleForm.ftp_path}
                            onChange={(e) => setScheduleForm({ ...scheduleForm, ftp_path: e.target.value })}
                            placeholder="/backups"
                            className="input w-full"
                          />
                        </div>
                      </div>
                    )}
                  </div>
                </div>

                {/* Options */}
                <div className="wb-group">
                  <div className="wb-group-title">Options</div>
                  <div className="wb-group-body space-y-2">
                    <label className="flex items-center gap-2 text-[12px] cursor-pointer">
                      <input
                        type="checkbox"
                        checked={scheduleForm.is_enabled}
                        onChange={() => setScheduleForm({ ...scheduleForm, is_enabled: !scheduleForm.is_enabled })}
                      />
                      <span className="font-semibold">Enable Schedule</span>
                      <span className="text-gray-500 dark:text-[#aaa]">- Backups will run automatically when enabled</span>
                    </label>
                    <label className="flex items-center gap-2 text-[12px] cursor-pointer">
                      <input
                        type="checkbox"
                        checked={scheduleForm.cloud_enabled}
                        onChange={() => setScheduleForm({ ...scheduleForm, cloud_enabled: !scheduleForm.cloud_enabled })}
                      />
                      <span className="font-semibold">Upload to ProxPanel Cloud</span>
                      <span className="text-gray-500 dark:text-[#aaa]">- Auto-upload after each scheduled run</span>
                    </label>
                    {scheduleForm.backup_type !== 'mikrotik' && (
                    <label className="flex items-center gap-2 text-[12px] cursor-pointer">
                      <input
                        type="checkbox"
                        checked={scheduleForm.include_mikrotik}
                        onChange={() => setScheduleForm({ ...scheduleForm, include_mikrotik: !scheduleForm.include_mikrotik })}
                      />
                      <span className="font-semibold">Include MikroTik Configs</span>
                      <span className="text-gray-500 dark:text-[#aaa]">- Export router configs alongside DB backup</span>
                    </label>
                    )}
                  </div>
                </div>
              </form>
            </div>
            <div className="modal-footer">
              <button
                type="button"
                onClick={() => { setShowScheduleModal(false); setEditingSchedule(null); resetScheduleForm() }}
                className="btn btn-sm"
              >
                Cancel
              </button>
              <button
                type="submit"
                form="scheduleForm"
                disabled={createScheduleMutation.isLoading || updateScheduleMutation.isLoading}
                className="btn btn-primary btn-sm"
              >
                {(createScheduleMutation.isLoading || updateScheduleMutation.isLoading) ? 'Saving...' : editingSchedule ? 'Update Schedule' : 'Create Schedule'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
