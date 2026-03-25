import { useState, useEffect } from 'react'
import { fetchTimezone, getTimezone, onTimezoneChange } from '../utils/timezone'
import api from '../services/api'

export default function Clock({ sessionRemaining }) {
  const [time, setTime] = useState('')
  const [date, setDate] = useState('')
  const [timezone, setTimezone] = useState(getTimezone() || '')
  const [uptime, setUptime] = useState(null)

  // Fetch uptime every 60 seconds
  useEffect(() => {
    const fetchUptime = () => {
      api.get('/dashboard/system-info').then(res => {
        if (res.data?.data?.os?.uptime_seconds) {
          setUptime(res.data.data.os.uptime_seconds)
        }
      }).catch(() => {})
    }
    fetchUptime()
    const uptimeInterval = setInterval(fetchUptime, 60000)
    return () => clearInterval(uptimeInterval)
  }, [])

  // Increment uptime locally every second
  useEffect(() => {
    if (uptime === null) return
    const interval = setInterval(() => {
      setUptime(prev => prev + 1)
    }, 1000)
    return () => clearInterval(interval)
  }, [uptime !== null])

  const formatUptime = (seconds) => {
    if (!seconds) return ''
    const days = Math.floor(seconds / 86400)
    const hours = Math.floor((seconds % 86400) / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)
    const secs = seconds % 60
    if (days > 0) return `${days}d ${String(hours).padStart(2,'0')}:${String(minutes).padStart(2,'0')}:${String(secs).padStart(2,'0')}`
    return `${String(hours).padStart(2,'0')}:${String(minutes).padStart(2,'0')}:${String(secs).padStart(2,'0')}`
  }

  // Subscribe to timezone changes
  useEffect(() => {
    if (!timezone) {
      fetchTimezone().then(tz => {
        if (tz) setTimezone(tz)
      })
    }
    const unsubscribe = onTimezoneChange((tz) => {
      setTimezone(tz)
    })
    return () => unsubscribe()
  }, [])

  // Update clock every second using server timezone
  useEffect(() => {
    const updateClock = () => {
      const now = new Date()
      const tzOptions = timezone ? { timeZone: timezone } : {}
      setTime(now.toLocaleTimeString('en-US', {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false,
        ...tzOptions
      }))
      setDate(now.toLocaleDateString('en-US', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        ...tzOptions
      }))
    }

    updateClock()
    const interval = setInterval(updateClock, 1000)
    return () => clearInterval(interval)
  }, [timezone])

  const formatSessionRemaining = (seconds) => {
    if (seconds == null || seconds < 0) return ''
    const m = Math.floor(seconds / 60)
    const s = seconds % 60
    return `${m}:${String(s).padStart(2, '0')}`
  }

  const sessionColor = sessionRemaining != null
    ? sessionRemaining <= 30 ? 'text-red-400' : sessionRemaining <= 120 ? 'text-yellow-300' : 'text-green-300'
    : ''

  return (
    <>
      {uptime !== null && (
        <span className="hidden lg:inline">
          Uptime: <span className="font-mono">{formatUptime(uptime)}</span>
        </span>
      )}
      {sessionRemaining != null && (
        <span className={`hidden lg:inline ${sessionColor}`}>
          Session: <span className="font-mono">{formatSessionRemaining(sessionRemaining)}</span>
        </span>
      )}
      <span className="hidden sm:inline">
        Time: <span className="font-mono">{time || '--:--:--'}</span>
      </span>
      <span className="hidden sm:inline font-mono">{date || '--'}</span>
      {timezone && (
        <span className="hidden md:inline text-[11px] opacity-80">{timezone}</span>
      )}
    </>
  )
}
