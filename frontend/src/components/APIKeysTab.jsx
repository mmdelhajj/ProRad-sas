import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiKeysApi } from '../services/api'
import toast from 'react-hot-toast'
import { copyToClipboard as clipboardCopy } from '../utils/clipboard'
import { KeyIcon, ClipboardDocumentIcon, TrashIcon, EyeIcon, ChevronDownIcon, ChevronUpIcon } from '@heroicons/react/24/outline'

export default function APIKeysTab() {
  const queryClient = useQueryClient()
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [newKeyName, setNewKeyName] = useState('API Key')
  const [newKeyScopes, setNewKeyScopes] = useState(['read'])
  const [newKeyExpiry, setNewKeyExpiry] = useState('')
  const [createdKey, setCreatedKey] = useState(null)
  const [expandedLogs, setExpandedLogs] = useState(null)

  const { data: keysData, isLoading } = useQuery({
    queryKey: ['api-keys'],
    queryFn: () => apiKeysApi.list().then(r => r.data),
  })

  const { data: statsData } = useQuery({
    queryKey: ['api-keys-stats'],
    queryFn: () => apiKeysApi.getStats().then(r => r.data),
  })

  const { data: logsData } = useQuery({
    queryKey: ['api-key-logs', expandedLogs],
    queryFn: () => apiKeysApi.getLogs(expandedLogs, { limit: 20 }).then(r => r.data),
    enabled: !!expandedLogs,
  })

  const createMutation = useMutation({
    mutationFn: (data) => apiKeysApi.create(data),
    onSuccess: (res) => {
      setCreatedKey(res.data.data)
      queryClient.invalidateQueries(['api-keys'])
      queryClient.invalidateQueries(['api-keys-stats'])
      toast.success('API key created')
    },
    onError: () => toast.error('Failed to create API key'),
  })

  const revokeMutation = useMutation({
    mutationFn: (id) => apiKeysApi.revoke(id),
    onSuccess: () => {
      queryClient.invalidateQueries(['api-keys'])
      queryClient.invalidateQueries(['api-keys-stats'])
      toast.success('API key revoked')
    },
    onError: () => toast.error('Failed to revoke key'),
  })

  const handleCreate = () => {
    createMutation.mutate({
      name: newKeyName,
      scopes: newKeyScopes.join(','),
      expires_at: newKeyExpiry || undefined,
    })
  }

  const copyToClipboard = (text) => {
    clipboardCopy(text)
      .then(() => toast.success('Copied to clipboard'))
      .catch(() => toast.error('Failed to copy — please select and copy manually'))
  }

  const toggleScope = (scope) => {
    setNewKeyScopes(prev =>
      prev.includes(scope) ? prev.filter(s => s !== scope) : [...prev, scope]
    )
  }

  const keys = keysData?.data || []
  const stats = statsData?.data || {}

  return (
    <div className="space-y-4">
      {/* Stats Cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        <div className="card p-3">
          <div className="text-[11px] text-gray-500 dark:text-gray-400">Total Keys</div>
          <div className="text-lg font-bold text-gray-900 dark:text-white">{stats.total_keys || 0}</div>
        </div>
        <div className="card p-3">
          <div className="text-[11px] text-gray-500 dark:text-gray-400">Active Keys</div>
          <div className="text-lg font-bold text-green-600">{stats.active_keys || 0}</div>
        </div>
        <div className="card p-3">
          <div className="text-[11px] text-gray-500 dark:text-gray-400">Requests (24h)</div>
          <div className="text-lg font-bold text-blue-600">{stats.requests_last_24h || 0}</div>
        </div>
        <div className="card p-3">
          <div className="text-[11px] text-gray-500 dark:text-gray-400">Rate Limit</div>
          <div className="text-lg font-bold text-gray-900 dark:text-white">{stats.rate_limit || '60/min'}</div>
        </div>
      </div>

      {/* Header + Create Button */}
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-[13px] font-semibold text-gray-900 dark:text-white">API Keys</h3>
          <p className="text-[11px] text-gray-500 dark:text-gray-400">Generate keys for external API access. Keys are shown only once.</p>
        </div>
        <button onClick={() => { setShowCreateModal(true); setCreatedKey(null); setNewKeyName('API Key'); setNewKeyScopes(['read']); setNewKeyExpiry('') }} className="btn btn-primary btn-sm">
          <KeyIcon className="w-4 h-4 mr-1" /> Generate Key
        </button>
      </div>

      {/* API Docs Link */}
      <div className="card p-3 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800">
        <p className="text-[12px] text-blue-700 dark:text-blue-300">
          View the <a href="/api-docs" target="_blank" rel="noopener noreferrer" className="font-semibold underline">API Documentation</a> for endpoint details, examples, and authentication guide.
        </p>
      </div>

      {/* Keys Table */}
      {isLoading ? (
        <div className="card p-6 text-center text-[12px] text-gray-500 dark:text-gray-400">Loading...</div>
      ) : keys.length === 0 ? (
        <div className="card p-6 text-center">
          <KeyIcon className="w-8 h-8 text-gray-300 dark:text-gray-600 mx-auto mb-2" />
          <p className="text-[12px] text-gray-500 dark:text-gray-400">No API keys yet. Generate one to get started.</p>
        </div>
      ) : (
        <div className="card overflow-hidden">
          <table className="w-full text-[12px]">
            <thead>
              <tr className="bg-gray-50 dark:bg-gray-700 text-left">
                <th className="px-3 py-2 font-semibold text-gray-600 dark:text-gray-300">Name</th>
                <th className="px-3 py-2 font-semibold text-gray-600 dark:text-gray-300">Key</th>
                <th className="px-3 py-2 font-semibold text-gray-600 dark:text-gray-300">Scopes</th>
                <th className="px-3 py-2 font-semibold text-gray-600 dark:text-gray-300">Last Used</th>
                <th className="px-3 py-2 font-semibold text-gray-600 dark:text-gray-300">Created</th>
                <th className="px-3 py-2 font-semibold text-gray-600 dark:text-gray-300">Status</th>
                <th className="px-3 py-2 font-semibold text-gray-600 dark:text-gray-300">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 dark:divide-gray-700">
              {keys.map(key => (
                <>
                  <tr key={key.id} className="hover:bg-gray-50 dark:hover:bg-gray-800">
                    <td className="px-3 py-2 font-medium text-gray-900 dark:text-white">{key.name}</td>
                    <td className="px-3 py-2 font-mono text-gray-500 dark:text-gray-400">{key.key_prefix}...</td>
                    <td className="px-3 py-2">
                      {key.scopes?.split(',').map(s => (
                        <span key={s} className={`inline-block px-1.5 py-0.5 mr-1 text-[10px] font-medium rounded ${
                          s.trim() === 'read' ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400' :
                          s.trim() === 'write' ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400' :
                          'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
                        }`}>{s.trim()}</span>
                      ))}
                    </td>
                    <td className="px-3 py-2 text-gray-500 dark:text-gray-400">
                      {key.last_used_at ? new Date(key.last_used_at).toLocaleDateString() : 'Never'}
                    </td>
                    <td className="px-3 py-2 text-gray-500 dark:text-gray-400">
                      {new Date(key.created_at).toLocaleDateString()}
                    </td>
                    <td className="px-3 py-2">
                      {key.is_active ? (
                        <span className="inline-block px-1.5 py-0.5 text-[10px] font-medium rounded bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400">Active</span>
                      ) : (
                        <span className="inline-block px-1.5 py-0.5 text-[10px] font-medium rounded bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400">Revoked</span>
                      )}
                    </td>
                    <td className="px-3 py-2 flex items-center gap-1">
                      <button onClick={() => setExpandedLogs(expandedLogs === key.id ? null : key.id)} className="p-1 hover:bg-gray-100 dark:hover:bg-gray-700 rounded" title="View Logs">
                        {expandedLogs === key.id ? <ChevronUpIcon className="w-4 h-4 text-gray-500" /> : <ChevronDownIcon className="w-4 h-4 text-gray-500" />}
                      </button>
                      {key.is_active && (
                        <button onClick={() => { if (confirm('Revoke this API key? This cannot be undone.')) revokeMutation.mutate(key.id) }} className="p-1 hover:bg-red-50 dark:hover:bg-red-900/20 rounded" title="Revoke">
                          <TrashIcon className="w-4 h-4 text-red-500" />
                        </button>
                      )}
                    </td>
                  </tr>
                  {expandedLogs === key.id && (
                    <tr key={`logs-${key.id}`}>
                      <td colSpan={7} className="px-3 py-2 bg-gray-50 dark:bg-gray-800">
                        <div className="text-[11px] font-semibold text-gray-700 dark:text-gray-300 mb-1">Recent Requests</div>
                        {logsData?.data?.length > 0 ? (
                          <table className="w-full text-[11px]">
                            <thead>
                              <tr className="text-left text-gray-500 dark:text-gray-400">
                                <th className="pr-3 py-1">Method</th>
                                <th className="pr-3 py-1">Path</th>
                                <th className="pr-3 py-1">Status</th>
                                <th className="pr-3 py-1">IP</th>
                                <th className="pr-3 py-1">Duration</th>
                                <th className="pr-3 py-1">Time</th>
                              </tr>
                            </thead>
                            <tbody>
                              {logsData.data.map(log => (
                                <tr key={log.id} className="border-t border-gray-200 dark:border-gray-700">
                                  <td className="pr-3 py-1"><span className={`font-mono font-medium ${log.method === 'GET' ? 'text-green-600' : log.method === 'POST' ? 'text-blue-600' : log.method === 'PUT' ? 'text-orange-600' : 'text-red-600'}`}>{log.method}</span></td>
                                  <td className="pr-3 py-1 font-mono text-gray-600 dark:text-gray-400">{log.path}</td>
                                  <td className="pr-3 py-1"><span className={log.status_code < 400 ? 'text-green-600' : 'text-red-600'}>{log.status_code}</span></td>
                                  <td className="pr-3 py-1 text-gray-500 dark:text-gray-400">{log.ip_address}</td>
                                  <td className="pr-3 py-1 text-gray-500 dark:text-gray-400">{log.duration_ms}ms</td>
                                  <td className="pr-3 py-1 text-gray-500 dark:text-gray-400">{new Date(log.created_at).toLocaleString()}</td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        ) : (
                          <p className="text-gray-400 dark:text-gray-500">No requests logged yet.</p>
                        )}
                      </td>
                    </tr>
                  )}
                </>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Create Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4" onClick={() => !createdKey && setShowCreateModal(false)}>
          <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-md" onClick={e => e.stopPropagation()}>
            <div className="p-4 border-b border-gray-200 dark:border-gray-700">
              <h3 className="text-[14px] font-semibold text-gray-900 dark:text-white">
                {createdKey ? 'API Key Created' : 'Generate New API Key'}
              </h3>
            </div>
            <div className="p-4 space-y-3">
              {createdKey ? (
                <>
                  <div className="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 p-3 rounded">
                    <p className="text-[11px] font-semibold text-yellow-700 dark:text-yellow-300 mb-1">Copy this key now — it won't be shown again!</p>
                  </div>
                  <div className="relative">
                    <input type="text" readOnly value={createdKey.key} className="input w-full font-mono text-[12px] pr-10" />
                    <button onClick={() => copyToClipboard(createdKey.key)} className="absolute right-2 top-1/2 -translate-y-1/2 p-1 hover:bg-gray-100 dark:hover:bg-gray-700 rounded">
                      <ClipboardDocumentIcon className="w-4 h-4 text-gray-500" />
                    </button>
                  </div>
                  <div className="text-[11px] text-gray-500 dark:text-gray-400 space-y-1">
                    <p><strong>Name:</strong> {createdKey.name}</p>
                    <p><strong>Scopes:</strong> {createdKey.scopes}</p>
                    <p><strong>Prefix:</strong> {createdKey.key_prefix}</p>
                  </div>
                </>
              ) : (
                <>
                  <div>
                    <label className="block text-[11px] font-semibold text-gray-700 dark:text-gray-300 mb-1">Key Name</label>
                    <input type="text" value={newKeyName} onChange={e => setNewKeyName(e.target.value)} className="input w-full" placeholder="My Integration" />
                  </div>
                  <div>
                    <label className="block text-[11px] font-semibold text-gray-700 dark:text-gray-300 mb-1">Scopes</label>
                    <div className="flex gap-3">
                      {['read', 'write', 'delete'].map(scope => (
                        <label key={scope} className="flex items-center gap-1.5 cursor-pointer">
                          <input type="checkbox" checked={newKeyScopes.includes(scope)} onChange={() => toggleScope(scope)} className="rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
                          <span className="text-[12px] text-gray-700 dark:text-gray-300 capitalize">{scope}</span>
                        </label>
                      ))}
                    </div>
                  </div>
                  <div>
                    <label className="block text-[11px] font-semibold text-gray-700 dark:text-gray-300 mb-1">Expires (optional)</label>
                    <input type="date" value={newKeyExpiry} onChange={e => setNewKeyExpiry(e.target.value)} className="input w-full" />
                  </div>
                </>
              )}
            </div>
            <div className="p-4 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-2">
              {createdKey ? (
                <button onClick={() => setShowCreateModal(false)} className="btn btn-primary btn-sm">Done</button>
              ) : (
                <>
                  <button onClick={() => setShowCreateModal(false)} className="btn btn-sm">Cancel</button>
                  <button onClick={handleCreate} disabled={createMutation.isPending || newKeyScopes.length === 0} className="btn btn-primary btn-sm">
                    {createMutation.isPending ? 'Creating...' : 'Generate Key'}
                  </button>
                </>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
