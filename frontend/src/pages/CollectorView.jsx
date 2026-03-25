import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { collectionApi } from '../services/api'
import {
  BanknotesIcon,
  CheckCircleIcon,
  XCircleIcon,
  MapPinIcon,
  PhoneIcon,
  XMarkIcon,
  ExclamationTriangleIcon,
} from '@heroicons/react/24/outline'
import toast from 'react-hot-toast'

export default function CollectorView() {
  const [statusFilter, setStatusFilter] = useState('')
  const [collectModal, setCollectModal] = useState(null)
  const [failModal, setFailModal] = useState(null)
  const queryClient = useQueryClient()

  const { data: dashboardData } = useQuery({
    queryKey: ['collector-dashboard'],
    queryFn: () => collectionApi.dashboard(),
    select: (res) => res.data?.data,
    refetchInterval: 30000,
  })

  const { data: assignmentsData, isLoading } = useQuery({
    queryKey: ['my-assignments', statusFilter],
    queryFn: () => collectionApi.listAssignments({ status: statusFilter || undefined }),
    select: (res) => res.data?.data || [],
  })

  const d = dashboardData || {}
  const assignments = assignmentsData || []

  const filterTabs = [
    { id: '', label: 'All', count: null },
    { id: 'pending', label: 'Pending', count: d.pending_count },
    { id: 'collected', label: 'Collected', count: d.total_collected },
    { id: 'failed', label: 'Failed', count: null },
  ]

  const openMap = (lat, lng) => {
    window.open(`https://www.google.com/maps?q=${lat},${lng}`, '_blank')
  }

  const invalidate = () => {
    queryClient.invalidateQueries(['my-assignments'])
    queryClient.invalidateQueries(['collector-dashboard'])
  }

  return (
    <div className="space-y-3">
      {/* Header + Stats inline */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <h1 className="text-lg font-bold text-gray-900 dark:text-white flex items-center gap-2">
          <BanknotesIcon className="h-5 w-5 text-green-600" />
          My Collections
        </h1>
        <div className="flex items-center gap-3 text-sm">
          <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-yellow-50 dark:bg-yellow-900/20">
            <span className="font-bold text-yellow-700 dark:text-yellow-400">{d.pending_count || 0}</span>
            <span className="text-yellow-600 dark:text-yellow-500 text-xs">Pending</span>
          </div>
          <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-blue-50 dark:bg-blue-900/20">
            <span className="font-bold text-blue-700 dark:text-blue-400">{d.collected_today || 0}</span>
            <span className="text-blue-600 dark:text-blue-500 text-xs">Today</span>
          </div>
          <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-green-50 dark:bg-green-900/20">
            <span className="font-bold text-green-700 dark:text-green-400">{d.total_collected || 0}</span>
            <span className="text-green-600 dark:text-green-500 text-xs">Collected</span>
          </div>
          <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-emerald-50 dark:bg-emerald-900/20">
            <span className="font-bold text-emerald-700 dark:text-emerald-400">${(d.total_amount || 0).toFixed(0)}</span>
            <span className="text-emerald-600 dark:text-emerald-500 text-xs">Total</span>
          </div>
        </div>
      </div>

      {/* Filter Tabs */}
      <div className="flex gap-1 border-b border-gray-200 dark:border-gray-700">
        {filterTabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setStatusFilter(tab.id)}
            className={`px-3 py-1.5 text-xs font-medium border-b-2 transition-colors ${
              statusFilter === tab.id
                ? 'border-blue-500 text-blue-600 dark:text-blue-400'
                : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
            }`}
          >
            {tab.label}
            {tab.count != null && tab.count > 0 && (
              <span className="ml-1 px-1.5 py-0.5 rounded-full bg-gray-200 dark:bg-gray-600 text-[10px]">{tab.count}</span>
            )}
          </button>
        ))}
      </div>

      {/* Assignments Table */}
      {isLoading ? (
        <div className="text-center py-8 text-gray-400 text-sm">Loading...</div>
      ) : assignments.length === 0 ? (
        <div className="text-center py-8">
          <BanknotesIcon className="h-8 w-8 text-gray-300 dark:text-gray-600 mx-auto mb-2" />
          <p className="text-sm text-gray-500 dark:text-gray-400">No assignments found</p>
        </div>
      ) : (
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow overflow-hidden">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-700/50">
              <tr>
                <th className="px-3 py-2 text-left text-[11px] font-medium text-gray-500 dark:text-gray-400 uppercase">Subscriber</th>
                <th className="px-3 py-2 text-left text-[11px] font-medium text-gray-500 dark:text-gray-400 uppercase hidden sm:table-cell">Address</th>
                <th className="px-3 py-2 text-right text-[11px] font-medium text-gray-500 dark:text-gray-400 uppercase">Amount</th>
                <th className="px-3 py-2 text-center text-[11px] font-medium text-gray-500 dark:text-gray-400 uppercase">Status</th>
                <th className="px-3 py-2 text-center text-[11px] font-medium text-gray-500 dark:text-gray-400 uppercase">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 dark:divide-gray-700/50">
              {assignments.map((a) => {
                const sub = a.subscriber_info || a.subscriber || {}
                const isPending = a.status === 'pending'
                const hasMap = sub.latitude && sub.longitude && sub.latitude !== 0
                return (
                  <tr key={a.id} className={`hover:bg-gray-50 dark:hover:bg-gray-700/30 ${
                    isPending ? '' : 'opacity-75'
                  }`}>
                    {/* Subscriber */}
                    <td className="px-3 py-2">
                      <div className="text-sm font-medium text-gray-900 dark:text-white leading-tight">
                        {sub.full_name || 'Unknown'}
                      </div>
                      {sub.phone && (
                        <a href={`tel:${sub.phone}`} className="inline-flex items-center gap-0.5 text-xs text-blue-600 dark:text-blue-400">
                          <PhoneIcon className="h-3 w-3" />
                          {sub.phone}
                        </a>
                      )}
                      {/* Address on mobile (hidden on sm+) */}
                      <div className="sm:hidden text-xs text-gray-400 dark:text-gray-500 mt-0.5 leading-tight">
                        {[sub.building, sub.address, sub.region].filter(Boolean).join(', ')}
                      </div>
                    </td>

                    {/* Address - desktop */}
                    <td className="px-3 py-2 hidden sm:table-cell">
                      <div className="text-xs text-gray-600 dark:text-gray-400 leading-tight max-w-[200px]">
                        {[sub.building, sub.address, sub.region].filter(Boolean).join(', ') || '—'}
                      </div>
                      {a.invoice && (
                        <div className="text-[10px] text-gray-400 mt-0.5">Inv #{a.invoice.invoice_number}</div>
                      )}
                    </td>

                    {/* Amount */}
                    <td className="px-3 py-2 text-right">
                      <div className="text-sm font-semibold text-gray-900 dark:text-white">${(a.amount || 0).toFixed(2)}</div>
                      {a.auto_renew && (
                        <div className="text-[10px] text-blue-600 dark:text-blue-400">auto-renew</div>
                      )}
                    </td>

                    {/* Status */}
                    <td className="px-3 py-2 text-center">
                      <span className={`inline-block px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase tracking-wide ${
                        a.status === 'pending' ? 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400' :
                        a.status === 'collected' ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400' :
                        a.status === 'failed' ? 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400' :
                        'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'
                      }`}>
                        {a.status}
                      </span>
                      {a.notes && a.status !== 'pending' && (
                        <div className="text-[10px] text-gray-400 dark:text-gray-500 mt-0.5 italic max-w-[80px] mx-auto truncate" title={a.notes}>
                          {a.notes}
                        </div>
                      )}
                      {a.collected_at && (
                        <div className="text-[10px] text-gray-400 mt-0.5">
                          {new Date(a.collected_at).toLocaleDateString()}
                        </div>
                      )}
                    </td>

                    {/* Actions */}
                    <td className="px-3 py-2">
                      <div className="flex items-center justify-center gap-1">
                        {hasMap && (
                          <button
                            onClick={() => openMap(sub.latitude, sub.longitude)}
                            className="p-1 rounded text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900/30"
                            title="Open Map"
                          >
                            <MapPinIcon className="h-4 w-4" />
                          </button>
                        )}
                        {isPending && (
                          <>
                            <button
                              onClick={() => setCollectModal(a)}
                              className="p-1 rounded text-green-600 dark:text-green-400 hover:bg-green-50 dark:hover:bg-green-900/30"
                              title="Mark Collected"
                            >
                              <CheckCircleIcon className="h-4 w-4" />
                            </button>
                            <button
                              onClick={() => setFailModal(a)}
                              className="p-1 rounded text-red-500 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/30"
                              title="Mark Failed"
                            >
                              <XCircleIcon className="h-4 w-4" />
                            </button>
                          </>
                        )}
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Modals */}
      {collectModal && (
        <CollectModal
          assignment={collectModal}
          onClose={() => setCollectModal(null)}
          onSuccess={() => { setCollectModal(null); invalidate() }}
        />
      )}
      {failModal && (
        <FailModal
          assignment={failModal}
          onClose={() => setFailModal(null)}
          onSuccess={() => { setFailModal(null); invalidate() }}
        />
      )}
    </div>
  )
}

function CollectModal({ assignment, onClose, onSuccess }) {
  const [amount, setAmount] = useState(assignment.amount || 0)
  const [notes, setNotes] = useState('')
  const [reference, setReference] = useState('')

  const mutation = useMutation({
    mutationFn: (data) => collectionApi.markCollected(assignment.id, data),
    onSuccess: () => { toast.success('Payment collected!'); onSuccess() },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed'),
  })

  const sub = assignment.subscriber_info || assignment.subscriber || {}

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-sm">
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-700">
          <h2 className="text-sm font-semibold text-gray-900 dark:text-white flex items-center gap-1.5">
            <CheckCircleIcon className="h-4 w-4 text-green-600" />
            Collect Payment
          </h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600">
            <XMarkIcon className="h-4 w-4" />
          </button>
        </div>
        <div className="px-4 py-3 space-y-3">
          <div className="text-xs text-gray-500 dark:text-gray-400">
            {sub.full_name} {sub.phone ? `· ${sub.phone}` : ''}
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-600 dark:text-gray-300 mb-1">Amount</label>
            <input type="number" step="0.01" value={amount} onChange={(e) => setAmount(parseFloat(e.target.value) || 0)}
              className="input w-full text-sm" />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-600 dark:text-gray-300 mb-1">Reference</label>
            <input type="text" value={reference} onChange={(e) => setReference(e.target.value)}
              className="input w-full text-sm" placeholder="Receipt #" />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-600 dark:text-gray-300 mb-1">Notes</label>
            <textarea value={notes} onChange={(e) => setNotes(e.target.value)} rows={2}
              className="input w-full text-sm" placeholder="Optional notes..." />
          </div>
        </div>
        <div className="flex justify-end gap-2 px-4 py-3 border-t border-gray-200 dark:border-gray-700">
          <button onClick={onClose} className="btn btn-secondary text-xs px-3 py-1.5">Cancel</button>
          <button
            onClick={() => mutation.mutate({ amount, notes, reference })}
            disabled={mutation.isLoading || amount <= 0}
            className="btn text-xs px-3 py-1.5 bg-green-600 text-white hover:bg-green-700 disabled:opacity-50"
          >
            {mutation.isLoading ? 'Saving...' : 'Confirm'}
          </button>
        </div>
      </div>
    </div>
  )
}

function FailModal({ assignment, onClose, onSuccess }) {
  const [notes, setNotes] = useState('')

  const mutation = useMutation({
    mutationFn: (data) => collectionApi.markFailed(assignment.id, data),
    onSuccess: () => { toast.success('Marked as failed'); onSuccess() },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed'),
  })

  const sub = assignment.subscriber_info || assignment.subscriber || {}

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-sm">
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-700">
          <h2 className="text-sm font-semibold text-gray-900 dark:text-white flex items-center gap-1.5">
            <ExclamationTriangleIcon className="h-4 w-4 text-red-500" />
            Mark Failed
          </h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600">
            <XMarkIcon className="h-4 w-4" />
          </button>
        </div>
        <div className="px-4 py-3 space-y-3">
          <div className="text-xs text-gray-500 dark:text-gray-400">
            {sub.full_name} · ${(assignment.amount || 0).toFixed(2)}
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-600 dark:text-gray-300 mb-1">Reason</label>
            <textarea value={notes} onChange={(e) => setNotes(e.target.value)} rows={2}
              className="input w-full text-sm" placeholder="Not at home, refused, etc." />
          </div>
        </div>
        <div className="flex justify-end gap-2 px-4 py-3 border-t border-gray-200 dark:border-gray-700">
          <button onClick={onClose} className="btn btn-secondary text-xs px-3 py-1.5">Cancel</button>
          <button
            onClick={() => mutation.mutate({ notes })}
            disabled={mutation.isLoading}
            className="btn text-xs px-3 py-1.5 bg-red-600 text-white hover:bg-red-700 disabled:opacity-50"
          >
            {mutation.isLoading ? 'Saving...' : 'Mark Failed'}
          </button>
        </div>
      </div>
    </div>
  )
}
