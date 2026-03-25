import { useState, useEffect, useRef, useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { nasApi, diagnosticApi } from '../services/api'
import { useAuthStore } from '../store/authStore'
import {
  PlayIcon,
  ArrowPathIcon,
  StopIcon,
  MagnifyingGlassIcon,
} from '@heroicons/react/24/outline'

const PACKET_SIZES = [64, 500, 1000, 1400, 1500, 8000, 16000, 32000, 64000]

export default function DiagnosticTools() {
  const [searchParams] = useSearchParams()
  const [activeTab, setActiveTab] = useState('ping')
  const [selectedNasId, setSelectedNasId] = useState(searchParams.get('nas_id') || '')

  // Ping state
  const [pingTarget, setPingTarget] = useState('')
  const [pingSize, setPingSize] = useState(64)
  const [pingCount, setPingCount] = useState(50)
  const [pingLines, setPingLines] = useState([])
  const [pingStreaming, setPingStreaming] = useState(false)
  const abortRef = useRef(null)
  const scrollRef = useRef(null)

  // Subscriber search state
  const [searchQuery, setSearchQuery] = useState('')
  const [searchResults, setSearchResults] = useState([])
  const [showDropdown, setShowDropdown] = useState(false)
  const [searchLoading, setSearchLoading] = useState(false)
  const searchTimeout = useRef(null)
  const dropdownRef = useRef(null)

  // Traceroute state
  const [traceTarget, setTraceTarget] = useState('')
  const [traceResult, setTraceResult] = useState(null)
  const [traceLoading, setTraceLoading] = useState(false)

  // NSLookup state
  const [nslookupDomain, setNslookupDomain] = useState('')
  const [nslookupResult, setNslookupResult] = useState(null)
  const [nslookupLoading, setNslookupLoading] = useState(false)

  // Fetch NAS list
  const { data: nasData } = useQuery({
    queryKey: ['nas-list'],
    queryFn: () => nasApi.list(),
    select: (res) => res.data?.data || [],
  })

  const nasList = nasData || []

  // Auto-select NAS: prefer URL param, fall back to first NAS if only one
  useEffect(() => {
    if (!selectedNasId && nasList.length > 0) {
      const paramId = searchParams.get('nas_id')
      if (paramId && nasList.find(n => String(n.id) === paramId)) {
        setSelectedNasId(paramId)
      } else if (nasList.length === 1) {
        setSelectedNasId(String(nasList[0].id))
      }
    }
  }, [nasList])

  // Close dropdown on outside click
  useEffect(() => {
    const handleClick = (e) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target)) {
        setShowDropdown(false)
      }
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [])

  // Auto-scroll ping terminal
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [pingLines])

  // Subscriber search with debounce
  const handleSearchChange = useCallback((value) => {
    setSearchQuery(value)
    setPingTarget(value)

    if (searchTimeout.current) clearTimeout(searchTimeout.current)

    if (value.length < 2) {
      setSearchResults([])
      setShowDropdown(false)
      return
    }

    if (/^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$/.test(value)) {
      setShowDropdown(false)
      return
    }

    setSearchLoading(true)
    searchTimeout.current = setTimeout(async () => {
      try {
        const res = await diagnosticApi.searchSubscribers(selectedNasId || 0, value)
        const data = res.data?.data || []
        setSearchResults(data)
        setShowDropdown(data.length > 0)
      } catch {
        setSearchResults([])
        setShowDropdown(false)
      } finally {
        setSearchLoading(false)
      }
    }, 300)
  }, [selectedNasId])

  const selectSubscriber = (sub) => {
    const ip = sub.static_ip || sub.ip_address || ''
    setPingTarget(ip)
    setSearchQuery(ip)
    setShowDropdown(false)
  }

  // Live streaming ping handler
  const handlePing = async () => {
    if (!selectedNasId || !pingTarget) return
    if (pingStreaming) {
      // Stop current ping
      if (abortRef.current) abortRef.current.abort()
      return
    }

    setPingLines([])
    setPingStreaming(true)
    const controller = new AbortController()
    abortRef.current = controller

    try {
      const token = useAuthStore.getState().token
      const response = await fetch('/api/diagnostic/ping-stream', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({
          nas_id: Number(selectedNasId),
          target: pingTarget,
          size: pingSize,
          count: pingCount,
        }),
        signal: controller.signal,
      })

      if (!response.ok) {
        const text = await response.text()
        let msg = 'Ping failed'
        try { msg = JSON.parse(text).message || msg } catch {}
        setPingLines([{ text: `Error: ${msg}`, color: 'red' }])
        setPingStreaming(false)
        return
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { value, done } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop()
        for (const line of lines) {
          if (!line.trim()) continue
          try {
            const data = JSON.parse(line)
            handlePingEvent(data)
          } catch {}
        }
      }
      // Process remaining buffer
      if (buffer.trim()) {
        try {
          handlePingEvent(JSON.parse(buffer))
        } catch {}
      }
    } catch (err) {
      if (err.name !== 'AbortError') {
        setPingLines(prev => [...prev, { text: `Error: ${err.message}`, color: 'red' }])
      }
    } finally {
      setPingStreaming(false)
      abortRef.current = null
    }
  }

  const handlePingEvent = (data) => {
    switch (data.type) {
      case 'start':
        setPingLines(prev => [...prev,
          { text: `Pinging ${data.target} via ${data.nas} (size=${data.size}, count=${data.count}):`, color: 'white' },
          { text: '', color: 'white' },
        ])
        break
      case 'reply':
        setPingLines(prev => [...prev,
          { text: `  seq=${data.seq}  Reply from ${data.host}: bytes=${data.size} time=${data.time.toFixed(2)}ms TTL=${data.ttl}`, color: 'green' },
        ])
        break
      case 'timeout':
        setPingLines(prev => [...prev,
          { text: `  seq=${data.seq}  Request timed out.`, color: 'red' },
        ])
        break
      case 'error':
        setPingLines(prev => [...prev,
          { text: `Error: ${data.message}`, color: 'red' },
        ])
        break
      case 'stats':
        setPingLines(prev => [...prev,
          { text: '', color: 'white' },
          { text: `Ping statistics for ${data.target}:`, color: 'cyan' },
          { text: `    Packets: Sent = ${data.sent}, Received = ${data.received}, Lost = ${data.lost} (${data.loss}% loss)`, color: data.lost > 0 ? 'yellow' : 'white' },
          ...(data.received > 0 ? [
            { text: `Approximate round trip times in milli-seconds:`, color: 'cyan' },
            { text: `    Minimum = ${data.min.toFixed(2)}ms, Maximum = ${data.max.toFixed(2)}ms, Average = ${data.avg.toFixed(2)}ms`, color: 'white' },
          ] : []),
        ])
        break
    }
  }

  // Traceroute handler
  const handleTraceroute = async () => {
    if (!traceTarget) return
    setTraceLoading(true)
    setTraceResult(null)
    try {
      const res = await diagnosticApi.traceroute({
        target: traceTarget,
      })
      setTraceResult(res.data?.data)
    } catch (err) {
      setTraceResult({ error: err.response?.data?.message || err.message, hops: [] })
    } finally {
      setTraceLoading(false)
    }
  }

  // NSLookup handler
  const handleNslookup = async () => {
    if (!nslookupDomain) return
    setNslookupLoading(true)
    setNslookupResult(null)
    try {
      const res = await diagnosticApi.nslookup({ domain: nslookupDomain })
      setNslookupResult(res.data?.data)
    } catch (err) {
      setNslookupResult({ error: err.response?.data?.message || err.message })
    } finally {
      setNslookupLoading(false)
    }
  }

  return (
    <div className="space-y-3" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Header */}
      <div className="wb-toolbar">
        <span className="text-[13px] font-semibold text-gray-800 dark:text-gray-100">Diagnostic Tools</span>
        <span className="text-[11px] text-gray-500 dark:text-gray-400 ml-2">Network diagnostic tools via MikroTik routers</span>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-[#a0a0a0]">
        <button
          onClick={() => setActiveTab('ping')}
          className={`wb-tab ${activeTab === 'ping' ? 'active' : ''}`}
        >
          Ping
        </button>
        <button
          onClick={() => setActiveTab('traceroute')}
          className={`wb-tab ${activeTab === 'traceroute' ? 'active' : ''}`}
        >
          Traceroute
        </button>
        <button
          onClick={() => setActiveTab('nslookup')}
          className={`wb-tab ${activeTab === 'nslookup' ? 'active' : ''}`}
        >
          NSLookup
        </button>
      </div>

      {/* Tab Content */}
      {activeTab === 'ping' && (
        <PingTab
          nasList={nasList}
          selectedNasId={selectedNasId}
          setSelectedNasId={setSelectedNasId}
          pingTarget={pingTarget}
          searchQuery={searchQuery}
          handleSearchChange={handleSearchChange}
          searchResults={searchResults}
          showDropdown={showDropdown}
          setShowDropdown={setShowDropdown}
          searchLoading={searchLoading}
          selectSubscriber={selectSubscriber}
          dropdownRef={dropdownRef}
          pingSize={pingSize}
          setPingSize={setPingSize}
          pingCount={pingCount}
          setPingCount={setPingCount}
          pingLines={pingLines}
          pingStreaming={pingStreaming}
          handlePing={handlePing}
          scrollRef={scrollRef}
        />
      )}

      {activeTab === 'traceroute' && (
        <TracerouteTab
          traceTarget={traceTarget}
          setTraceTarget={setTraceTarget}
          traceResult={traceResult}
          traceLoading={traceLoading}
          handleTraceroute={handleTraceroute}
        />
      )}

      {activeTab === 'nslookup' && (
        <NslookupTab
          nslookupDomain={nslookupDomain}
          setNslookupDomain={setNslookupDomain}
          nslookupResult={nslookupResult}
          nslookupLoading={nslookupLoading}
          handleNslookup={handleNslookup}
        />
      )}
    </div>
  )
}

function PingTab({
  nasList, selectedNasId, setSelectedNasId,
  pingTarget, searchQuery, handleSearchChange,
  searchResults, showDropdown, setShowDropdown, searchLoading, selectSubscriber, dropdownRef,
  pingSize, setPingSize, pingCount, setPingCount,
  pingLines, pingStreaming, handlePing, scrollRef,
}) {
  const lineColors = {
    green: 'text-green-400',
    red: 'text-red-400',
    yellow: 'text-yellow-400',
    cyan: 'text-cyan-400',
    white: 'text-gray-300',
  }

  return (
    <div className="space-y-3">
      <div className="wb-group">
        <div className="wb-group-title">Ping Configuration</div>
        <div className="wb-group-body space-y-3">
          {/* Row 1: NAS + Target */}
          <div className="grid grid-cols-2 gap-3">
            {/* NAS Dropdown */}
            <div>
              <label className="label">NAS / Router</label>
              <select
                value={selectedNasId}
                onChange={(e) => setSelectedNasId(e.target.value)}
                className="input"
              >
                <option value="">-- Select NAS --</option>
                {nasList.map((nas) => (
                  <option key={nas.id} value={nas.id}>{nas.name} ({nas.ip_address})</option>
                ))}
              </select>
            </div>

            {/* Target IP / User Search */}
            <div className="relative" ref={dropdownRef}>
              <label className="label">Target (IP or Username)</label>
              <div className="relative">
                <input
                  type="text"
                  value={searchQuery || pingTarget}
                  onChange={(e) => handleSearchChange(e.target.value)}
                  onFocus={() => searchResults.length > 0 && setShowDropdown(true)}
                  placeholder="IP or search user..."
                  className="input w-full pr-6"
                  onKeyDown={(e) => e.key === 'Enter' && handlePing()}
                />
                {searchLoading && (
                  <ArrowPathIcon className="absolute right-1.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-gray-400 animate-spin" />
                )}
              </div>
              {/* Autocomplete Dropdown */}
              {showDropdown && searchResults.length > 0 && (
                <div className="absolute z-50 mt-0 w-full bg-white dark:bg-gray-800 border border-[#a0a0a0] dark:border-gray-600 max-h-48 overflow-y-auto" style={{ borderRadius: '2px' }}>
                  {searchResults.map((sub) => (
                    <button
                      key={sub.id}
                      onClick={() => selectSubscriber(sub)}
                      className="w-full text-left px-2 py-1 hover:bg-[#e8e8f0] dark:hover:bg-gray-700 flex items-center justify-between border-b border-[#eee] dark:border-gray-700 last:border-0 text-[12px]"
                    >
                      <div className="min-w-0 flex-1">
                        <span className="font-medium text-gray-900 dark:text-gray-100">{sub.username}</span>
                        <span className={`ml-1 ${sub.is_online ? 'badge-success' : 'badge-gray'}`}>
                          {sub.is_online ? 'Online' : 'Offline'}
                        </span>
                      </div>
                      <span className="text-[12px] text-gray-500 dark:text-gray-400 font-mono flex-shrink-0 ml-2">
                        {sub.static_ip || sub.ip_address || 'No IP'}
                      </span>
                    </button>
                  ))}
                </div>
              )}
            </div>
          </div>

          {/* Row 2: Size + Count + Button */}
          <div className="grid grid-cols-3 gap-3">
            {/* Packet Size */}
            <div>
              <label className="label">Packet Size</label>
              <select
                value={pingSize}
                onChange={(e) => setPingSize(Number(e.target.value))}
                className="input"
              >
                {PACKET_SIZES.map((size) => (
                  <option key={size} value={size}>{size} bytes</option>
                ))}
              </select>
            </div>

            {/* Count */}
            <div>
              <label className="label">Count</label>
              <input
                type="number"
                value={pingCount}
                onChange={(e) => setPingCount(Math.min(100, Math.max(1, Number(e.target.value))))}
                min="1"
                max="100"
                className="input"
              />
            </div>

            {/* Run/Stop Ping Button */}
            <div className="flex items-end">
              <button
                onClick={handlePing}
                disabled={!pingStreaming && (!selectedNasId || !pingTarget)}
                className={pingStreaming ? 'btn btn-danger w-full' : 'btn btn-primary w-full'}
              >
                {pingStreaming ? (
                  <>
                    <StopIcon className="h-3.5 w-3.5 mr-1" />
                    Stop
                  </>
                ) : (
                  <>
                    <PlayIcon className="h-3.5 w-3.5 mr-1" />
                    Run Ping
                  </>
                )}
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Live Terminal Output */}
      {(pingLines.length > 0 || pingStreaming) && (
        <div className="border border-[#a0a0a0]" style={{ borderRadius: '2px' }}>
          <div className="flex items-center justify-between px-2 py-1 bg-[#2d2d2d] border-b border-[#555]">
            <span className="text-[11px] text-gray-400">Ping Output</span>
            {pingStreaming && (
              <div className="flex items-center gap-1 text-[11px] text-green-400">
                <div className="w-2 h-2 bg-green-400 animate-pulse" style={{ borderRadius: '1px' }}></div>
                Live
              </div>
            )}
          </div>
          <div
            ref={scrollRef}
            className="p-2 font-mono text-[11px] max-h-[500px] overflow-y-auto bg-[#1e1e1e]"
          >
            {pingLines.map((line, i) => (
              <div key={i} className={lineColors[line.color] || 'text-gray-300'}>
                {line.text || '\u00A0'}
              </div>
            ))}
            {pingStreaming && (
              <div className="text-green-400 animate-pulse inline-block">_</div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

function TracerouteTab({
  traceTarget, setTraceTarget,
  traceResult, traceLoading, handleTraceroute,
}) {
  return (
    <div className="space-y-3">
      <div className="wb-group">
        <div className="wb-group-title">Traceroute Configuration</div>
        <div className="wb-group-body space-y-2">
          <p className="text-[11px] text-gray-500 dark:text-gray-400">Runs from server. Public IPs and hostnames only.</p>
          <div className="flex gap-3 items-end">
            <div className="flex-1">
              <label className="label">Target (Public IP or Hostname)</label>
              <input
                type="text"
                value={traceTarget}
                onChange={(e) => setTraceTarget(e.target.value)}
                placeholder="e.g., 8.8.8.8 or google.com"
                className="input"
                onKeyDown={(e) => e.key === 'Enter' && handleTraceroute()}
              />
            </div>
            <button
              onClick={handleTraceroute}
              disabled={traceLoading || !traceTarget}
              className="btn btn-primary"
            >
              {traceLoading ? (
                <ArrowPathIcon className="h-3.5 w-3.5 mr-1 animate-spin" />
              ) : (
                <PlayIcon className="h-3.5 w-3.5 mr-1" />
              )}
              Run Traceroute
            </button>
          </div>
        </div>
      </div>

      {/* Loading */}
      {traceLoading && (
        <div className="wb-group">
          <div className="wb-group-body text-center py-2">
            <ArrowPathIcon className="h-6 w-6 animate-spin mx-auto text-[#316AC5] mb-2" />
            <p className="text-[12px] text-gray-500 dark:text-gray-400">Running traceroute... this may take up to 30 seconds</p>
          </div>
        </div>
      )}

      {/* Results */}
      {traceResult && !traceLoading && (
        <div className="wb-group">
          <div className="wb-group-title">
            Traceroute to {traceResult.target} from {traceResult.source || 'Server'}
          </div>
          <div className="wb-group-body">
            {traceResult.error && (
              <p className="text-[12px] text-red-600 dark:text-red-400 mb-2">{traceResult.error}</p>
            )}

            {traceResult.hops && traceResult.hops.length > 0 ? (
              <div className="table-container">
                <table className="table">
                  <thead>
                    <tr>
                      <th>Hop</th>
                      <th>Address</th>
                      <th>Loss</th>
                      <th>Last</th>
                      <th>Avg</th>
                      <th>Best</th>
                      <th>Worst</th>
                    </tr>
                  </thead>
                  <tbody>
                    {traceResult.hops.map((hop) => (
                      <tr key={hop.hop}>
                        <td className="font-medium">{hop.hop}</td>
                        <td className="font-mono">{hop.address || '*'}</td>
                        <td>{hop.loss || '0%'}</td>
                        <td className="font-mono">{hop.last ? `${hop.last.toFixed(1)} ms` : '-'}</td>
                        <td className="font-mono">{hop.avg ? `${hop.avg.toFixed(1)} ms` : '-'}</td>
                        <td className="font-mono text-green-700 dark:text-green-400">{hop.best ? `${hop.best.toFixed(1)} ms` : '-'}</td>
                        <td className="font-mono text-red-700 dark:text-red-400">{hop.worst ? `${hop.worst.toFixed(1)} ms` : '-'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              !traceResult.error && <p className="text-[12px] text-gray-500 dark:text-gray-400">No hops returned</p>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

function NslookupTab({
  nslookupDomain, setNslookupDomain,
  nslookupResult, nslookupLoading, handleNslookup,
}) {
  return (
    <div className="space-y-3">
      <div className="wb-group">
        <div className="wb-group-title">DNS Lookup</div>
        <div className="wb-group-body">
          <div className="flex gap-3 items-end">
            <div className="flex-1">
              <label className="label">Domain Name</label>
              <input
                type="text"
                value={nslookupDomain}
                onChange={(e) => setNslookupDomain(e.target.value)}
                placeholder="e.g., google.com"
                className="input"
                onKeyDown={(e) => e.key === 'Enter' && handleNslookup()}
              />
            </div>
            <button
              onClick={handleNslookup}
              disabled={nslookupLoading || !nslookupDomain}
              className="btn btn-primary"
            >
              {nslookupLoading ? (
                <ArrowPathIcon className="h-3.5 w-3.5 mr-1 animate-spin" />
              ) : (
                <MagnifyingGlassIcon className="h-3.5 w-3.5 mr-1" />
              )}
              Lookup
            </button>
          </div>
        </div>
      </div>

      {nslookupResult && (
        <div className="wb-group">
          <div className="wb-group-title">
            DNS Records for {nslookupResult.domain}
          </div>
          <div className="wb-group-body space-y-3">
            {nslookupResult.error && (
              <p className="text-[12px] text-red-600 dark:text-red-400">{nslookupResult.error}</p>
            )}

            {nslookupResult.records && (
              <>
                {nslookupResult.records.a && nslookupResult.records.a.length > 0 && (
                  <RecordSection title="A Records (IPv4)" items={nslookupResult.records.a} />
                )}
                {nslookupResult.records.aaaa && nslookupResult.records.aaaa.length > 0 && (
                  <RecordSection title="AAAA Records (IPv6)" items={nslookupResult.records.aaaa} />
                )}
                {nslookupResult.records.cname && (
                  <RecordSection title="CNAME" items={[nslookupResult.records.cname]} />
                )}
                {nslookupResult.records.mx && nslookupResult.records.mx.length > 0 && (
                  <div>
                    <div className="text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">MX Records (Mail)</div>
                    <div className="border border-[#a0a0a0] bg-[#f7f7f7] dark:bg-gray-800 dark:border-gray-600 p-2 space-y-0.5" style={{ borderRadius: '2px' }}>
                      {nslookupResult.records.mx.map((mx, i) => (
                        <div key={i} className="font-mono text-[12px] text-gray-800 dark:text-gray-200 break-all">
                          Priority: {mx.priority} &mdash; {mx.host}
                        </div>
                      ))}
                    </div>
                  </div>
                )}
                {nslookupResult.records.ns && nslookupResult.records.ns.length > 0 && (
                  <RecordSection title="NS Records (Nameservers)" items={nslookupResult.records.ns} />
                )}
                {nslookupResult.records.txt && nslookupResult.records.txt.length > 0 && (
                  <div>
                    <div className="text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">TXT Records</div>
                    <div className="border border-[#a0a0a0] bg-[#f7f7f7] dark:bg-gray-800 dark:border-gray-600 p-2 space-y-0.5" style={{ borderRadius: '2px' }}>
                      {nslookupResult.records.txt.map((txt, i) => (
                        <div key={i} className="font-mono text-[11px] text-gray-800 dark:text-gray-200 break-all">{txt}</div>
                      ))}
                    </div>
                  </div>
                )}
                {!nslookupResult.records.a?.length && !nslookupResult.records.aaaa?.length && !nslookupResult.records.cname && !nslookupResult.records.mx?.length && !nslookupResult.records.ns?.length && !nslookupResult.records.txt?.length && (
                  <p className="text-[12px] text-gray-500 dark:text-gray-400">No DNS records found for this domain.</p>
                )}
              </>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

function RecordSection({ title, items }) {
  return (
    <div>
      <div className="text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">{title}</div>
      <div className="border border-[#a0a0a0] bg-[#f7f7f7] dark:bg-gray-800 dark:border-gray-600 p-2 space-y-0.5" style={{ borderRadius: '2px' }}>
        {items.map((item, i) => (
          <div key={i} className="font-mono text-[12px] text-gray-800 dark:text-gray-200 break-all">{item}</div>
        ))}
      </div>
    </div>
  )
}
