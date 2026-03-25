// Timezone utility for consistent date/time formatting across the app
import api from '../services/api'

let systemTimezone = null
let timezoneListeners = []

// Fetch timezone from server
export const fetchTimezone = async () => {
  try {
    const res = await api.get('/server-time')
    if (res.data.success && res.data.timezone) {
      systemTimezone = res.data.timezone
      // Notify all listeners
      timezoneListeners.forEach(fn => fn(systemTimezone))
    }
  } catch (err) {
    console.error('Failed to fetch timezone', err)
  }
  return systemTimezone
}

// Get current timezone (returns null if not fetched yet)
export const getTimezone = () => systemTimezone

// Subscribe to timezone changes
export const onTimezoneChange = (callback) => {
  timezoneListeners.push(callback)
  // Return unsubscribe function
  return () => {
    timezoneListeners = timezoneListeners.filter(fn => fn !== callback)
  }
}

// Set timezone (used when user changes it in settings)
export const setTimezone = (tz) => {
  systemTimezone = tz
  timezoneListeners.forEach(fn => fn(systemTimezone))
}

// Format date with system timezone
export const formatDate = (dateStr, options = {}) => {
  if (!dateStr) return '-'
  try {
    const date = new Date(dateStr)
    if (isNaN(date.getTime())) return '-'

    const defaultOptions = {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      ...(systemTimezone ? { timeZone: systemTimezone } : {})
    }
    return date.toLocaleDateString('en-US', { ...defaultOptions, ...options })
  } catch (e) {
    return '-'
  }
}

// Format time with system timezone
export const formatTime = (dateStr, options = {}) => {
  if (!dateStr) return '-'
  try {
    const date = new Date(dateStr)
    if (isNaN(date.getTime())) return '-'

    const defaultOptions = {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
      ...(systemTimezone ? { timeZone: systemTimezone } : {})
    }
    return date.toLocaleTimeString('en-US', { ...defaultOptions, ...options })
  } catch (e) {
    return '-'
  }
}

// Format datetime with system timezone
export const formatDateTime = (dateStr, options = {}) => {
  if (!dateStr) return '-'
  try {
    const date = new Date(dateStr)
    if (isNaN(date.getTime())) return '-'

    const defaultOptions = {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
      ...(systemTimezone ? { timeZone: systemTimezone } : {})
    }
    return date.toLocaleString('en-US', { ...defaultOptions, ...options })
  } catch (e) {
    return '-'
  }
}

// Format datetime short (without seconds)
export const formatDateTimeShort = (dateStr) => {
  if (!dateStr) return '-'
  try {
    const date = new Date(dateStr)
    if (isNaN(date.getTime())) return '-'

    const options = {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
      ...(systemTimezone ? { timeZone: systemTimezone } : {})
    }
    return date.toLocaleString('en-US', options)
  } catch (e) {
    return '-'
  }
}

// Format relative time (e.g., "2 hours ago")
export const formatRelativeTime = (dateStr) => {
  if (!dateStr) return '-'
  try {
    const date = new Date(dateStr)
    if (isNaN(date.getTime())) return '-'

    const now = new Date()
    const diffMs = now - date
    const diffSec = Math.floor(diffMs / 1000)
    const diffMin = Math.floor(diffSec / 60)
    const diffHour = Math.floor(diffMin / 60)
    const diffDay = Math.floor(diffHour / 24)

    if (diffSec < 60) return 'Just now'
    if (diffMin < 60) return `${diffMin}m ago`
    if (diffHour < 24) return `${diffHour}h ago`
    if (diffDay < 7) return `${diffDay}d ago`

    return formatDate(dateStr)
  } catch (e) {
    return '-'
  }
}

// Initialize timezone on app start
fetchTimezone()
