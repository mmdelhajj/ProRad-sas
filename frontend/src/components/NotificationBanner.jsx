import { useState, useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import { notificationBannerApi } from '../services/api'
import { XMarkIcon, InformationCircleIcon, ExclamationTriangleIcon, ExclamationCircleIcon, CheckCircleIcon } from '@heroicons/react/24/outline'

// Dismissed banners kept in memory only — reappear on page refresh

const typeConfig = {
  info: {
    bg: 'bg-blue-600 dark:bg-blue-700',
    icon: InformationCircleIcon,
  },
  warning: {
    bg: 'bg-amber-500 dark:bg-amber-600',
    icon: ExclamationTriangleIcon,
  },
  error: {
    bg: 'bg-red-600 dark:bg-red-700',
    icon: ExclamationCircleIcon,
  },
  success: {
    bg: 'bg-green-600 dark:bg-green-700',
    icon: CheckCircleIcon,
  },
}

export default function NotificationBanner() {
  const [dismissed, setDismissed] = useState([])

  const { data } = useQuery({
    queryKey: ['notification-banners-active'],
    queryFn: () => notificationBannerApi.getActive(),
    refetchInterval: 5 * 60 * 1000,
    staleTime: 60 * 1000,
  })

  const banners = data?.data?.data || []

  const handleDismiss = useCallback((id) => {
    setDismissed(prev => [...prev, id])
  }, [])

  const visibleBanners = banners.filter(b => !dismissed.includes(b.id))

  if (visibleBanners.length === 0) return null

  return (
    <div className="flex flex-col">
      {visibleBanners.map(banner => {
        const config = typeConfig[banner.banner_type] || typeConfig.info
        const Icon = config.icon

        return (
          <div
            key={banner.id}
            className={`${config.bg} text-white text-[12px] px-3 py-1.5 flex items-center gap-2`}
          >
            <Icon className="w-4 h-4 flex-shrink-0" />
            <span className="font-semibold flex-shrink-0">{banner.title}</span>
            <span className="truncate">{banner.message}</span>
            <div className="flex-1" />
            {banner.dismissible && (
              <button
                onClick={() => handleDismiss(banner.id)}
                className="p-0.5 hover:bg-white/20 rounded flex-shrink-0"
                title="Dismiss"
              >
                <XMarkIcon className="w-3.5 h-3.5" />
              </button>
            )}
          </div>
        )
      })}
    </div>
  )
}
