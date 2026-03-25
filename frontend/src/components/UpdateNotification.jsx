import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import api from '../services/api'
import { ArrowDownTrayIcon, XMarkIcon } from '@heroicons/react/24/outline'

export default function UpdateNotification() {
  const [showPopup, setShowPopup] = useState(false)

  // Check for updates
  const { data: updateData } = useQuery({
    queryKey: ['update-check'],
    queryFn: () => api.get('/system/update/check').then(res => res.data),
    staleTime: 30 * 60 * 1000,
    refetchInterval: 30 * 60 * 1000,
    retry: false,
  })

  // Don't show if no update available
  if (!updateData?.update_available) return null

  const isCritical = updateData?.is_critical
  const newVersion = updateData?.new_version || updateData?.version

  const scrollToUpdateBanner = () => {
    setShowPopup(false)
    // Scroll to top where the banner is
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  return (
    <div className="relative">
      {/* Notification Icon */}
      <button
        onClick={() => setShowPopup(!showPopup)}
        className={`relative p-1 transition-colors ${
          isCritical
            ? 'text-[#FF9800] hover:bg-[#fff8e1] dark:hover:bg-[#2a2a1a]'
            : 'text-[#2196F3] hover:bg-[#e3f2fd] dark:hover:bg-[#1a2a3a]'
        }`}
        style={{ borderRadius: '2px' }}
        title={`Update available: v${newVersion}`}
      >
        <ArrowDownTrayIcon className="w-4 h-4" />
        {/* Badge */}
        <span className={`absolute -top-0.5 -right-0.5 w-2 h-2 ${
          isCritical ? 'bg-[#FF9800]' : 'bg-[#2196F3]'
        } animate-pulse`} style={{ borderRadius: '1px' }} />
      </button>

      {/* Popup */}
      {showPopup && (
        <>
          {/* Backdrop */}
          <div
            className="fixed inset-0 z-40"
            onClick={() => setShowPopup(false)}
          />

          {/* Popup content */}
          <div className="absolute right-0 mt-1 w-56 card border border-[#a0a0a0] dark:border-[#555] z-50" style={{ boxShadow: '2px 2px 6px rgba(0,0,0,0.2)' }}>
            <div className="p-2">
              <div className="flex items-start justify-between mb-1.5">
                <div className="flex items-center gap-1.5">
                  <ArrowDownTrayIcon className={`w-4 h-4 ${isCritical ? 'text-[#FF9800]' : 'text-[#2196F3]'}`} />
                  <span className="font-semibold text-[12px] text-gray-900 dark:text-[#e0e0e0]">
                    {isCritical ? 'Critical Update' : 'Update Available'}
                  </span>
                </div>
                <button
                  onClick={() => setShowPopup(false)}
                  className="text-gray-400 dark:text-[#888] hover:text-gray-600 dark:hover:text-[#ccc]"
                >
                  <XMarkIcon className="w-3.5 h-3.5" />
                </button>
              </div>

              <p className="text-[11px] text-gray-600 dark:text-[#aaa] mb-2">
                Version {newVersion} is available.
                {isCritical && (
                  <span className="block mt-0.5 text-[#e65100] dark:text-[#FF9800] font-semibold">
                    This is a critical security update.
                  </span>
                )}
              </p>

              <button
                onClick={scrollToUpdateBanner}
                className="btn btn-primary btn-xs w-full"
              >
                View Update Details
              </button>
            </div>
          </div>
        </>
      )}
    </div>
  )
}
