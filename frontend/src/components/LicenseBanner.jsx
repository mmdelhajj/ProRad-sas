import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import api from '../services/api'
import { XMarkIcon } from '@heroicons/react/24/outline'
import { useState, useEffect } from 'react'

export default function LicenseBanner() {
  const [dismissed, setDismissed] = useState(false)

  const { data: licenseData } = useQuery({
    queryKey: ['license-status'],
    queryFn: () => api.get('/license/status').then(res => res.data),
    staleTime: 60 * 1000, // Cache for 1 minute
    refetchInterval: 60 * 1000, // Refetch every minute
    retry: false,
  })

  // Reset dismissed state when status changes to something critical
  useEffect(() => {
    if (licenseData?.license_status === 'blocked' || licenseData?.license_status === 'readonly') {
      setDismissed(false)
    }
  }, [licenseData?.license_status])

  // Don't show banner if dismissed or no data
  if (dismissed || !licenseData) return null

  const status = licenseData.license_status
  const daysLeft = licenseData.days_until_expiry
  const warningMessage = licenseData.warning_message
  const readOnly = licenseData.read_only

  // Determine what type of warning to show based on WHMCS-style status
  let bannerType = null
  let message = ''
  let submessage = ''
  let canDismiss = true

  switch (status) {
    case 'blocked':
      // License is fully blocked
      bannerType = 'error'
      canDismiss = false
      message = 'License Blocked'
      submessage = 'Your license has expired and the grace period has ended. Please contact support or renew your license immediately.'
      break

    case 'readonly':
      // System is in read-only mode
      bannerType = 'error'
      canDismiss = false
      message = 'Read-Only Mode'
      submessage = `License expired. You can view data but cannot make changes. ${Math.abs(daysLeft)} days overdue. Please renew immediately.`
      break

    case 'grace':
      // License expired but in grace period
      bannerType = 'warning'
      canDismiss = false
      message = 'License Grace Period'
      submessage = `Your license has expired. ${Math.abs(daysLeft)} days overdue. System will enter read-only mode soon. Renew now!`
      break

    case 'warning':
      // License expiring soon
      bannerType = 'warning'
      canDismiss = true
      if (daysLeft <= 7) {
        message = `License expires in ${daysLeft} day${daysLeft !== 1 ? 's' : ''}!`
        submessage = warningMessage || 'Please renew immediately to avoid service interruption.'
      } else {
        message = `License expires in ${daysLeft} days`
        submessage = warningMessage || 'Please renew soon to ensure uninterrupted service.'
      }
      break

    case 'active':
    default:
      // No warning needed
      return null
  }

  // No warning needed
  if (!bannerType) return null

  const isError = bannerType === 'error'

  return (
    <div className={`${isError ? 'border-l-4 border-l-[#f44336] bg-[#ffebee] dark:bg-[#3a1a1a]' : 'border-l-4 border-l-[#FF9800] bg-[#fff8e1] dark:bg-[#2a2a1a]'} px-3 py-2`}>
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2 min-w-0">
          <svg className={`w-4 h-4 flex-shrink-0 ${isError ? 'text-[#f44336]' : 'text-[#FF9800]'}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
            {isError ? (
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            ) : (
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            )}
          </svg>
          <div className="text-[12px] min-w-0">
            <span className={`font-semibold ${isError ? 'text-[#c62828] dark:text-[#ef9a9a]' : 'text-[#e65100] dark:text-[#FFB74D]'}`}>
              {message}
            </span>
            {submessage && (
              <span className={`ml-2 ${isError ? 'text-[#c62828] dark:text-[#ef9a9a]' : 'text-[#bf360c] dark:text-[#FFB74D]'}`}>
                {submessage}
              </span>
            )}
            {readOnly && status !== 'blocked' && (
              <span className="ml-2 badge-danger">READ-ONLY</span>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2 flex-shrink-0 ml-2">
          <Link
            to="/settings?tab=license"
            className={`text-[12px] font-medium underline hover:no-underline ${isError ? 'text-[#c62828] dark:text-[#ef9a9a]' : 'text-[#e65100] dark:text-[#FFB74D]'}`}
          >
            View License
          </Link>
          {canDismiss && (
            <button
              onClick={() => setDismissed(true)}
              className={`p-0.5 hover:bg-black/10 ${isError ? 'text-[#c62828] dark:text-[#ef9a9a]' : 'text-[#e65100] dark:text-[#FFB74D]'}`}
              style={{ borderRadius: '2px' }}
            >
              <XMarkIcon className="w-3.5 h-3.5" />
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
