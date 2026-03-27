import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import api from '../services/api'
import { XMarkIcon, ArrowDownTrayIcon, ArrowPathIcon, CheckCircleIcon, ExclamationTriangleIcon } from '@heroicons/react/24/outline'
import { useState, useEffect } from 'react'

export default function UpdateBanner() {
  const [dismissed, setDismissed] = useState(false)
  const [showModal, setShowModal] = useState(false)
  const [updating, setUpdating] = useState(false)
  const queryClient = useQueryClient()
  const isSaaS = window.location.hostname.endsWith('.saas.proxrad.com') || window.location.hostname === 'saas.proxrad.com'

  // Check for updates (skip in SaaS mode — updates managed by platform)
  const { data: updateData } = useQuery({
    queryKey: ['update-check'],
    queryFn: () => api.get('/system/update/check').then(res => res.data),
    staleTime: 30 * 60 * 1000,
    refetchInterval: 30 * 60 * 1000,
    retry: false,
    enabled: !isSaaS,
  })

  // Get update status (poll when updating)
  const { data: statusData, refetch: refetchStatus } = useQuery({
    queryKey: ['update-status'],
    queryFn: () => api.get('/system/update/status').then(res => res.data.data),
    enabled: updating,
    refetchInterval: updating ? 2000 : false,
  })

  // Start update mutation
  const startUpdateMutation = useMutation({
    mutationFn: (version) => api.post('/system/update/start', { version }),
    onSuccess: () => {
      setUpdating(true)
    },
    onError: (err) => {
      alert(err.response?.data?.message || 'Failed to start update')
    }
  })

  // Handle update completion
  useEffect(() => {
    if (statusData?.step === 'complete' && statusData?.needs_restart) {
      // Update complete, will reload shortly
      setTimeout(() => {
        window.location.reload()
      }, 3000)
    }
  }, [statusData])

  // Don't show banner if dismissed or no update available
  if (dismissed || (!updateData?.update_available && !updating)) return null

  const isCritical = updateData?.is_critical
  const currentVersion = updateData?.current_version || '1.0.0'
  const newVersion = updateData?.new_version || updateData?.version

  const handleStartUpdate = () => {
    if (confirm(`Are you sure you want to update from v${currentVersion} to v${newVersion}?\n\nThe system will restart after the update.`)) {
      startUpdateMutation.mutate(newVersion)
    }
  }

  return (
    <>
      <div className={`${isCritical ? 'border-l-4 border-l-[#FF9800] bg-[#fff8e1] dark:bg-[#2a2a1a]' : 'border-l-4 border-l-[#2196F3] bg-[#e3f2fd] dark:bg-[#1a2a3a]'} px-3 py-2`}>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 min-w-0">
            {updating ? (
              <ArrowPathIcon className={`w-4 h-4 flex-shrink-0 animate-spin ${isCritical ? 'text-[#FF9800]' : 'text-[#2196F3]'}`} />
            ) : (
              <ArrowDownTrayIcon className={`w-4 h-4 flex-shrink-0 ${isCritical ? 'text-[#FF9800]' : 'text-[#2196F3]'}`} />
            )}
            <div className="text-[12px] min-w-0">
              {updating ? (
                <span className={`font-semibold ${isCritical ? 'text-[#e65100] dark:text-[#FFB74D]' : 'text-[#1565c0] dark:text-[#90caf9]'}`}>
                  Updating to v{newVersion}... {statusData?.message || 'Please wait'}
                </span>
              ) : (
                <>
                  <span className={`font-semibold ${isCritical ? 'text-[#e65100] dark:text-[#FFB74D]' : 'text-[#1565c0] dark:text-[#90caf9]'}`}>
                    {isCritical ? 'Critical Update Available: ' : 'Update Available: '}
                    v{newVersion}
                  </span>
                  <span className={`ml-2 ${isCritical ? 'text-[#bf360c] dark:text-[#FFB74D]' : 'text-[#1976d2] dark:text-[#90caf9]'}`}>
                    (Current: v{currentVersion})
                  </span>
                </>
              )}
            </div>
          </div>
          <div className="flex items-center gap-2 flex-shrink-0 ml-2">
            {!updating && (
              <>
                <button
                  onClick={() => setShowModal(true)}
                  className={`text-[12px] underline hover:no-underline ${isCritical ? 'text-[#e65100] dark:text-[#FFB74D]' : 'text-[#1565c0] dark:text-[#90caf9]'}`}
                >
                  View details
                </button>
                <button
                  onClick={handleStartUpdate}
                  disabled={startUpdateMutation.isPending}
                  className="btn btn-primary btn-xs"
                >
                  {startUpdateMutation.isPending ? 'Starting...' : 'Update Now'}
                </button>
                {!isCritical && (
                  <button
                    onClick={() => setDismissed(true)}
                    className={`p-0.5 hover:bg-black/10 ${isCritical ? 'text-[#e65100] dark:text-[#FFB74D]' : 'text-[#1565c0] dark:text-[#90caf9]'}`}
                    style={{ borderRadius: '2px' }}
                  >
                    <XMarkIcon className="w-3.5 h-3.5" />
                  </button>
                )}
              </>
            )}
            {updating && statusData?.progress > 0 && (
              <span className={`text-[12px] font-semibold ${isCritical ? 'text-[#e65100] dark:text-[#FFB74D]' : 'text-[#1565c0] dark:text-[#90caf9]'}`}>
                {statusData.progress}%
              </span>
            )}
          </div>
        </div>
        {/* Progress bar when updating */}
        {updating && (
          <div className="mt-1.5">
            <div className="w-full h-1 bg-[#e0e0e0] dark:bg-[#555]" style={{ borderRadius: '1px' }}>
              <div
                className={`h-1 transition-all duration-300 ${isCritical ? 'bg-[#FF9800]' : 'bg-[#2196F3]'}`}
                style={{ width: `${statusData?.progress || 0}%`, borderRadius: '1px' }}
              />
            </div>
          </div>
        )}
      </div>

      {/* Update Details Modal */}
      {showModal && (
        <div className="modal-overlay">
          <div className="modal" style={{ maxWidth: '500px', width: '100%' }}>
            <div className="modal-header">
              <span>
                Update to v{newVersion}
                {isCritical && (
                  <span className="ml-2 badge-warning">Critical</span>
                )}
              </span>
              <button
                onClick={() => setShowModal(false)}
                className="text-white/80 hover:text-white"
              >
                <XMarkIcon className="w-4 h-4" />
              </button>
            </div>

            <div className="modal-body" style={{ maxHeight: '300px' }}>
              <div className="flex items-center gap-2 mb-3 text-[12px] text-gray-500 dark:text-[#aaa]">
                <span className="font-mono bg-[#e0e0e0] dark:bg-[#444] px-1.5 py-0.5" style={{ borderRadius: '2px' }}>v{currentVersion}</span>
                <span>-&gt;</span>
                <span className="font-mono bg-[#e8f5e9] dark:bg-[#1e7e34] text-[#2e7d32] dark:text-white px-1.5 py-0.5" style={{ borderRadius: '2px' }}>v{newVersion}</span>
              </div>

              {updateData?.release_notes && (
                <>
                  <div className="text-[12px] font-semibold text-gray-700 dark:text-[#ccc] mb-1">What's New</div>
                  <div className="text-[12px] text-gray-600 dark:text-[#aaa] whitespace-pre-wrap bg-[#f7f7f7] dark:bg-[#333] p-2 border border-[#a0a0a0] dark:border-[#555]" style={{ borderRadius: '2px' }}>
                    {updateData.release_notes}
                  </div>
                </>
              )}

              {updateData?.released_at && (
                <p className="mt-3 text-[11px] text-gray-400 dark:text-[#888]">
                  Released: {new Date(updateData.released_at).toLocaleDateString('en-US', {
                    year: 'numeric',
                    month: 'long',
                    day: 'numeric'
                  })}
                </p>
              )}
            </div>

            <div className="px-3 py-2 border-l-4 border-l-[#FF9800] bg-[#fff8e1] dark:bg-[#2a2a1a] mx-0">
              <div className="flex items-start gap-2">
                <ExclamationTriangleIcon className="w-4 h-4 text-[#FF9800] flex-shrink-0 mt-0.5" />
                <div className="text-[12px] text-[#e65100] dark:text-[#FFB74D]">
                  <p className="font-semibold">Before updating:</p>
                  <ul className="mt-0.5 list-disc list-inside text-[11px]">
                    <li>A backup will be created automatically</li>
                    <li>The system will restart after update</li>
                    <li>Users may experience brief disconnection</li>
                  </ul>
                </div>
              </div>
            </div>

            <div className="modal-footer">
              <button
                onClick={() => setShowModal(false)}
                className="btn btn-sm"
              >
                Cancel
              </button>
              <button
                onClick={() => {
                  setShowModal(false)
                  handleStartUpdate()
                }}
                disabled={startUpdateMutation.isPending}
                className="btn btn-primary btn-sm"
              >
                <ArrowDownTrayIcon className="w-3.5 h-3.5 mr-1" />
                Update Now
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Update Complete Modal */}
      {statusData?.step === 'complete' && (
        <div className="modal-overlay">
          <div className="modal modal-sm">
            <div className="modal-header">
              <span>Update Complete</span>
            </div>
            <div className="modal-body text-center py-6">
              <CheckCircleIcon className="w-12 h-12 text-[#4CAF50] mx-auto mb-3" />
              <div className="text-[13px] font-semibold text-gray-900 dark:text-[#e0e0e0] mb-1">Update Complete!</div>
              <p className="text-[12px] text-gray-600 dark:text-[#aaa] mb-3">
                ProxPanel has been updated to v{newVersion}
              </p>
              <p className="text-[11px] text-gray-500 dark:text-[#888]">
                The page will reload automatically...
              </p>
              <div className="mt-3">
                <ArrowPathIcon className="w-5 h-5 text-gray-400 dark:text-[#888] mx-auto animate-spin" />
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Update Error Modal */}
      {statusData?.error && (
        <div className="modal-overlay">
          <div className="modal modal-sm">
            <div className="modal-header">
              <span>Update Failed</span>
            </div>
            <div className="modal-body">
              <div className="flex items-center gap-2 mb-3">
                <ExclamationTriangleIcon className="w-5 h-5 text-[#f44336]" />
                <span className="text-[13px] font-semibold text-gray-900 dark:text-[#e0e0e0]">Update Failed</span>
              </div>
              <p className="text-[12px] text-gray-600 dark:text-[#aaa] mb-2">{statusData.error}</p>
              <p className="text-[11px] text-gray-500 dark:text-[#888]">
                The system has been restored from backup. Please try again or contact support.
              </p>
            </div>
            <div className="modal-footer">
              <button
                onClick={() => {
                  setUpdating(false)
                  queryClient.invalidateQueries(['update-status'])
                }}
                className="btn btn-sm"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  )
}
