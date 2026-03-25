import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { collectorApi, subscriberApi } from '../services/api'
import { useAuthStore } from '../store/authStore'
import {
  BanknotesIcon,
  PlusIcon,
  TrashIcon,
  ChartBarIcon,
  UserGroupIcon,
  XMarkIcon,
  CheckCircleIcon,
  ClockIcon,
  ExclamationTriangleIcon,
  MagnifyingGlassIcon,
  MapPinIcon,
} from '@heroicons/react/24/outline'
import toast from 'react-hot-toast'

export default function Collectors() {
  const [activeTab, setActiveTab] = useState('collectors')
  const [showAssignModal, setShowAssignModal] = useState(false)
  const [selectedCollector, setSelectedCollector] = useState(null)
  const [viewAssignments, setViewAssignments] = useState(null)
  const [startDate, setStartDate] = useState(() => {
    const d = new Date()
    d.setMonth(d.getMonth() - 1)
    return d.toISOString().split('T')[0]
  })
  const [endDate, setEndDate] = useState(() => new Date().toISOString().split('T')[0])
  const queryClient = useQueryClient()

  // Fetch collectors
  const { data: collectorsData, isLoading } = useQuery({
    queryKey: ['collectors'],
    queryFn: () => collectorApi.list(),
    select: (res) => res.data?.data || [],
  })

  // Fetch report
  const { data: reportData } = useQuery({
    queryKey: ['collector-report', startDate, endDate],
    queryFn: () => collectorApi.getReport({ start_date: startDate, end_date: endDate }),
    select: (res) => res.data?.data || [],
    enabled: activeTab === 'reports',
  })

  // Fetch assignments for a collector
  const { data: assignmentsData } = useQuery({
    queryKey: ['collector-assignments', viewAssignments],
    queryFn: () => collectorApi.getAssignments(viewAssignments),
    select: (res) => res.data?.data || [],
    enabled: !!viewAssignments,
  })

  const deleteAssignment = useMutation({
    mutationFn: (id) => collectorApi.deleteAssignment(id),
    onSuccess: () => {
      toast.success('Assignment cancelled')
      queryClient.invalidateQueries(['collector-assignments'])
      queryClient.invalidateQueries(['collectors'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to cancel assignment'),
  })

  const collectors = collectorsData || []
  const report = reportData || []
  const assignments = assignmentsData || []

  const statusBadge = (status) => {
    const colors = {
      pending: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
      collected: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
      failed: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
      cancelled: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300',
    }
    return (
      <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${colors[status] || colors.pending}`}>
        {status}
      </span>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
            <BanknotesIcon className="h-7 w-7 text-green-600" />
            Collectors
          </h1>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">Manage payment collectors and assignments</p>
        </div>
        <button
          onClick={() => setShowAssignModal(true)}
          className="btn btn-primary flex items-center gap-2"
        >
          <PlusIcon className="h-5 w-5" />
          <span>Create Assignment</span>
        </button>
      </div>

      {/* Tabs */}
      <div className="border-b border-gray-200 dark:border-gray-700">
        <nav className="flex gap-6">
          {[
            { id: 'collectors', label: 'Collectors', icon: UserGroupIcon },
            { id: 'reports', label: 'Reports', icon: ChartBarIcon },
          ].map((tab) => (
            <button
              key={tab.id}
              onClick={() => { setActiveTab(tab.id); setViewAssignments(null) }}
              className={`flex items-center gap-2 py-3 px-1 border-b-2 text-sm font-medium transition-colors ${
                activeTab === tab.id
                  ? 'border-blue-500 text-blue-600 dark:text-blue-400'
                  : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
              }`}
            >
              <tab.icon className="h-4 w-4" />
              {tab.label}
            </button>
          ))}
        </nav>
      </div>

      {/* Collectors Tab */}
      {activeTab === 'collectors' && !viewAssignments && (
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow overflow-hidden">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-700">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Collector</th>
                <th className="px-4 py-3 text-center text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Assigned</th>
                <th className="px-4 py-3 text-center text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Pending</th>
                <th className="px-4 py-3 text-center text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Collected</th>
                <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Amount</th>
                <th className="px-4 py-3 text-center text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
              {isLoading ? (
                <tr><td colSpan="6" className="px-4 py-8 text-center text-gray-400">Loading...</td></tr>
              ) : collectors.length === 0 ? (
                <tr><td colSpan="6" className="px-4 py-8 text-center text-gray-400">No collectors found. Create a user with type "Collector" from the Users page.</td></tr>
              ) : collectors.map((col) => (
                <tr key={col.id} className="hover:bg-gray-50 dark:hover:bg-gray-700/50">
                  <td className="px-4 py-3">
                    <div className="font-medium text-gray-900 dark:text-white">{col.full_name || col.username}</div>
                    <div className="text-xs text-gray-500 dark:text-gray-400">{col.username}</div>
                  </td>
                  <td className="px-4 py-3 text-center text-sm text-gray-700 dark:text-gray-300">{col.total_assigned}</td>
                  <td className="px-4 py-3 text-center">
                    <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400">
                      {col.total_pending}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-center">
                    <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400">
                      {col.total_collected}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-right text-sm font-medium text-gray-900 dark:text-white">
                    ${(col.total_amount || 0).toFixed(2)}
                  </td>
                  <td className="px-4 py-3 text-center">
                    <button
                      onClick={() => setViewAssignments(col.id)}
                      className="text-blue-600 dark:text-blue-400 hover:text-blue-800 text-sm font-medium"
                    >
                      View Assignments
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Assignments View */}
      {activeTab === 'collectors' && viewAssignments && (
        <div>
          <div className="flex items-center gap-3 mb-4">
            <button
              onClick={() => setViewAssignments(null)}
              className="text-sm text-blue-600 dark:text-blue-400 hover:underline"
            >
              &larr; Back to Collectors
            </button>
            <span className="text-gray-400">|</span>
            <span className="text-sm text-gray-600 dark:text-gray-300">
              {assignments.length} assignment(s)
            </span>
          </div>
          <div className="bg-white dark:bg-gray-800 rounded-lg shadow overflow-hidden">
            <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
              <thead className="bg-gray-50 dark:bg-gray-700">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Subscriber</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Amount</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Status</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Notes</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Auto-Renew</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Created</th>
                  <th className="px-4 py-3 text-center text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                {assignments.length === 0 ? (
                  <tr><td colSpan="7" className="px-4 py-8 text-center text-gray-400">No assignments</td></tr>
                ) : assignments.map((a) => (
                  <tr key={a.id} className="hover:bg-gray-50 dark:hover:bg-gray-700/50">
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <div>
                          <div className="font-medium text-gray-900 dark:text-white">{a.subscriber?.full_name || 'Unknown'}</div>
                          <div className="text-xs text-gray-500 dark:text-gray-400">{a.subscriber?.phone}</div>
                          {a.subscriber?.address && (
                            <div className="text-xs text-gray-400 dark:text-gray-500">{a.subscriber.address}</div>
                          )}
                        </div>
                        {a.subscriber?.latitude && a.subscriber?.longitude && a.subscriber.latitude !== 0 && (
                          <button
                            onClick={() => window.open(`https://www.google.com/maps?q=${a.subscriber.latitude},${a.subscriber.longitude}`, '_blank')}
                            className="p-1.5 rounded-lg text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900/30"
                            title="Open Map"
                          >
                            <MapPinIcon className="h-4 w-4" />
                          </button>
                        )}
                      </div>
                    </td>
                    <td className="px-4 py-3 text-sm text-gray-900 dark:text-white">${(a.amount || 0).toFixed(2)}</td>
                    <td className="px-4 py-3">{statusBadge(a.status)}</td>
                    <td className="px-4 py-3">
                      {a.notes ? (
                        <div className={`text-sm ${a.status === 'failed' ? 'text-red-600 dark:text-red-400 font-medium' : 'text-gray-600 dark:text-gray-400'}`}>
                          {a.notes}
                        </div>
                      ) : (
                        <span className="text-xs text-gray-400">—</span>
                      )}
                      {a.collected_at && (
                        <div className="text-xs text-gray-400 mt-0.5">
                          {a.status === 'collected' ? 'Collected' : a.status === 'failed' ? 'Failed' : ''}: {new Date(a.collected_at).toLocaleString()}
                        </div>
                      )}
                    </td>
                    <td className="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">{a.auto_renew ? 'Yes' : 'No'}</td>
                    <td className="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
                      {new Date(a.created_at).toLocaleDateString()}
                    </td>
                    <td className="px-4 py-3 text-center">
                      {a.status === 'pending' && (
                        <button
                          onClick={() => { if (confirm('Cancel this assignment?')) deleteAssignment.mutate(a.id) }}
                          className="text-red-600 dark:text-red-400 hover:text-red-800"
                          title="Cancel"
                        >
                          <TrashIcon className="h-4 w-4" />
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Reports Tab */}
      {activeTab === 'reports' && (
        <div className="space-y-4">
          <div className="flex flex-wrap gap-4 items-end">
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Start Date</label>
              <input type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)}
                className="input" />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">End Date</label>
              <input type="date" value={endDate} onChange={(e) => setEndDate(e.target.value)}
                className="input" />
            </div>
          </div>
          <div className="bg-white dark:bg-gray-800 rounded-lg shadow overflow-hidden">
            <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
              <thead className="bg-gray-50 dark:bg-gray-700">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Collector</th>
                  <th className="px-4 py-3 text-center text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Assigned</th>
                  <th className="px-4 py-3 text-center text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Collected</th>
                  <th className="px-4 py-3 text-center text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Failed</th>
                  <th className="px-4 py-3 text-center text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Pending</th>
                  <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Amount</th>
                  <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Success %</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                {report.length === 0 ? (
                  <tr><td colSpan="7" className="px-4 py-8 text-center text-gray-400">No data for selected period</td></tr>
                ) : report.map((r) => (
                  <tr key={r.collector_id} className="hover:bg-gray-50 dark:hover:bg-gray-700/50">
                    <td className="px-4 py-3 font-medium text-gray-900 dark:text-white">{r.collector_name}</td>
                    <td className="px-4 py-3 text-center text-sm text-gray-700 dark:text-gray-300">{r.total_assigned}</td>
                    <td className="px-4 py-3 text-center text-sm text-green-600 dark:text-green-400 font-medium">{r.total_collected}</td>
                    <td className="px-4 py-3 text-center text-sm text-red-600 dark:text-red-400">{r.total_failed}</td>
                    <td className="px-4 py-3 text-center text-sm text-yellow-600 dark:text-yellow-400">{r.total_pending}</td>
                    <td className="px-4 py-3 text-right text-sm font-medium text-gray-900 dark:text-white">${(r.total_amount || 0).toFixed(2)}</td>
                    <td className="px-4 py-3 text-right text-sm text-gray-700 dark:text-gray-300">{(r.success_rate || 0).toFixed(1)}%</td>
                  </tr>
                ))}
                {report.length > 1 && (
                  <tr className="bg-gray-50 dark:bg-gray-700/50 font-semibold">
                    <td className="px-4 py-3 text-gray-900 dark:text-white">Total</td>
                    <td className="px-4 py-3 text-center text-sm text-gray-900 dark:text-white">{report.reduce((s, r) => s + r.total_assigned, 0)}</td>
                    <td className="px-4 py-3 text-center text-sm text-green-600 dark:text-green-400">{report.reduce((s, r) => s + r.total_collected, 0)}</td>
                    <td className="px-4 py-3 text-center text-sm text-red-600 dark:text-red-400">{report.reduce((s, r) => s + r.total_failed, 0)}</td>
                    <td className="px-4 py-3 text-center text-sm text-yellow-600 dark:text-yellow-400">{report.reduce((s, r) => s + r.total_pending, 0)}</td>
                    <td className="px-4 py-3 text-right text-sm text-gray-900 dark:text-white">${report.reduce((s, r) => s + (r.total_amount || 0), 0).toFixed(2)}</td>
                    <td className="px-4 py-3 text-right text-sm text-gray-700 dark:text-gray-300">-</td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Create Assignment Modal */}
      {showAssignModal && (
        <CreateAssignmentModal
          collectors={collectors}
          onClose={() => setShowAssignModal(false)}
          onCreated={() => {
            queryClient.invalidateQueries(['collectors'])
            setShowAssignModal(false)
          }}
        />
      )}
    </div>
  )
}

// Create Assignment Modal Component
function CreateAssignmentModal({ collectors, onClose, onCreated }) {
  const [selectedCollectorId, setSelectedCollectorId] = useState('')
  const [selectedSubscribers, setSelectedSubscribers] = useState([])
  const [autoRenew, setAutoRenew] = useState(false)
  const [sendNotification, setSendNotification] = useState(true)
  const [notes, setNotes] = useState('')
  const [searchTerm, setSearchTerm] = useState('')

  const { data: subscribersData, isLoading: loadingSubs } = useQuery({
    queryKey: ['subscribers-for-assign', searchTerm],
    queryFn: () => subscriberApi.list({ search: searchTerm || undefined, limit: 500 }),
    select: (res) => res.data?.data || [],
  })

  const createMutation = useMutation({
    mutationFn: (data) => collectorApi.createAssignment(data),
    onSuccess: (res) => {
      toast.success(res.data?.message || 'Assignments created')
      onCreated()
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to create assignments'),
  })

  const subscribers = subscribersData || []

  const toggleSubscriber = (id) => {
    setSelectedSubscribers((prev) =>
      prev.includes(id) ? prev.filter((s) => s !== id) : [...prev, id]
    )
  }

  const handleSubmit = () => {
    if (!selectedCollectorId || selectedSubscribers.length === 0) {
      toast.error('Select a collector and at least one subscriber')
      return
    }
    createMutation.mutate({
      collector_id: parseInt(selectedCollectorId),
      subscriber_ids: selectedSubscribers,
      auto_renew: autoRenew,
      send_notification: sendNotification,
      notes,
    })
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-2xl max-h-[90vh] overflow-hidden flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Create Collection Assignment</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
            <XMarkIcon className="h-5 w-5" />
          </button>
        </div>

        {/* Body */}
        <div className="flex-1 overflow-y-auto px-6 py-4 space-y-4">
          {/* Collector Select */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Collector</label>
            <select
              value={selectedCollectorId}
              onChange={(e) => setSelectedCollectorId(e.target.value)}
              className="input w-full"
            >
              <option value="">-- Select Collector --</option>
              {collectors.map((col) => (
                <option key={col.id} value={col.id}>
                  {col.full_name || col.username}
                </option>
              ))}
            </select>
          </div>

          {/* Subscriber Search & Select */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Subscribers ({selectedSubscribers.length} selected)
            </label>
            <div className="relative mb-2">
              <MagnifyingGlassIcon className="h-4 w-4 absolute left-3 top-2.5 text-gray-400" />
              <input
                type="text"
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                placeholder="Search subscribers..."
                className="input w-full pl-9"
              />
            </div>
            <div className="border border-gray-200 dark:border-gray-600 rounded-lg max-h-48 overflow-y-auto">
              {loadingSubs ? (
                <div className="px-4 py-3 text-sm text-gray-400">Loading...</div>
              ) : subscribers.length === 0 ? (
                <div className="px-4 py-3 text-sm text-gray-400">No subscribers found</div>
              ) : subscribers.map((sub) => (
                <label
                  key={sub.id}
                  className="flex items-center gap-3 px-4 py-2 hover:bg-gray-50 dark:hover:bg-gray-700/50 cursor-pointer border-b border-gray-100 dark:border-gray-700 last:border-0"
                >
                  <input
                    type="checkbox"
                    checked={selectedSubscribers.includes(sub.id)}
                    onChange={() => toggleSubscriber(sub.id)}
                    className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                  />
                  <div className="flex-1 min-w-0">
                    <div className="text-sm font-medium text-gray-900 dark:text-white truncate">{sub.full_name || sub.username}</div>
                    <div className="text-xs text-gray-500 dark:text-gray-400">{sub.phone} {sub.address ? `| ${sub.address}` : ''}</div>
                  </div>
                  <div className="text-sm text-gray-600 dark:text-gray-300">
                    ${(sub.price || sub.service?.price || 0).toFixed(2)}
                  </div>
                </label>
              ))}
            </div>
          </div>

          {/* Options */}
          <div className="flex flex-wrap gap-6">
            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={autoRenew}
                onChange={(e) => setAutoRenew(e.target.checked)}
                className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
              />
              <span className="text-sm text-gray-700 dark:text-gray-300">Auto-renew on collection</span>
            </label>
            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={sendNotification}
                onChange={(e) => setSendNotification(e.target.checked)}
                className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
              />
              <span className="text-sm text-gray-700 dark:text-gray-300">Send notification</span>
            </label>
          </div>

          {/* Notes */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Notes (optional)</label>
            <textarea
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              rows={2}
              className="input w-full"
              placeholder="Instructions for the collector..."
            />
          </div>
        </div>

        {/* Footer */}
        <div className="flex justify-end gap-3 px-6 py-4 border-t border-gray-200 dark:border-gray-700">
          <button onClick={onClose} className="btn btn-secondary">Cancel</button>
          <button
            onClick={handleSubmit}
            disabled={createMutation.isLoading || !selectedCollectorId || selectedSubscribers.length === 0}
            className="btn btn-primary"
          >
            {createMutation.isLoading ? 'Creating...' : `Assign ${selectedSubscribers.length} Subscriber(s)`}
          </button>
        </div>
      </div>
    </div>
  )
}
