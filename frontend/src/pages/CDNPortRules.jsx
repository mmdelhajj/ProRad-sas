import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { cdnApi, nasApi } from '../services/api'
import {
  PlusIcon,
  PencilIcon,
  TrashIcon,
  XMarkIcon,
  ArrowPathIcon,
  BoltIcon,
  ChartBarIcon,
} from '@heroicons/react/24/outline'
import toast from 'react-hot-toast'
import clsx from 'clsx'

const DIRECTIONS = [
  { value: 'src', label: 'Source Port Only (src-port)' },
  { value: 'dst', label: 'Destination Port Only (dst-port)' },
  { value: 'both', label: 'Both (src-port + dst-port)' },
  { value: 'dscp', label: 'DSCP Only (no port)' },
]

const GRAPH_COLORS = ['#8B5CF6', '#EF4444', '#3B82F6', '#10B981', '#F59E0B', '#EC4899', '#06B6D4', '#84CC16']

const defaultForm = {
  name: '',
  port: '',
  direction: 'both',
  dscp_value: 63,
  speed_mbps: 5,
  nas_id: null,
  is_active: true,
  show_in_graph: false,
  color: '#8B5CF6',
}

export default function CDNPortRules() {
  const queryClient = useQueryClient()
  const [showModal, setShowModal] = useState(false)
  const [editingRule, setEditingRule] = useState(null)
  const [formData, setFormData] = useState(defaultForm)

  const { data: rules, isLoading } = useQuery({
    queryKey: ['cdn-port-rules'],
    queryFn: () => cdnApi.listPortRules().then((r) => r.data.data),
  })

  const { data: nasList } = useQuery({
    queryKey: ['nas'],
    queryFn: () => nasApi.list().then((r) => r.data.data),
  })

  const saveMutation = useMutation({
    mutationFn: (data) =>
      editingRule ? cdnApi.updatePortRule(editingRule.id, data) : cdnApi.createPortRule(data),
    onSuccess: () => {
      toast.success(editingRule ? 'Port rule updated' : 'Port rule created')
      queryClient.invalidateQueries({ queryKey: ['cdn-port-rules'] })
      closeModal()
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to save'),
  })

  const deleteMutation = useMutation({
    mutationFn: (id) => cdnApi.deletePortRule(id),
    onSuccess: () => {
      toast.success('Port rule deleted')
      queryClient.invalidateQueries({ queryKey: ['cdn-port-rules'] })
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to delete'),
  })

  const syncMutation = useMutation({
    mutationFn: (id) => cdnApi.syncPortRule(id),
    onSuccess: (res) => toast.success(res.data?.message || 'Syncing to MikroTik...'),
    onError: (err) => toast.error(err.response?.data?.message || 'Sync failed'),
  })

  const syncAllMutation = useMutation({
    mutationFn: () => cdnApi.syncAllPortRules(),
    onSuccess: (res) => toast.success(res.data?.message || 'Syncing all to MikroTik...'),
    onError: (err) => toast.error(err.response?.data?.message || 'Sync failed'),
  })

  const openModal = (rule = null) => {
    if (rule) {
      setEditingRule(rule)
      setFormData({
        name: rule.name || '',
        port: rule.port || '',
        direction: rule.direction || 'both',
        dscp_value: rule.dscp_value ?? 63,
        speed_mbps: rule.speed_mbps || 5,
        nas_id: rule.nas_id || null,
        is_active: rule.is_active ?? true,
        show_in_graph: rule.show_in_graph ?? false,
        color: rule.color || '#8B5CF6',
      })
    } else {
      setEditingRule(null)
      setFormData(defaultForm)
    }
    setShowModal(true)
  }

  const closeModal = () => {
    setShowModal(false)
    setEditingRule(null)
  }

  const handleSubmit = (e) => {
    e.preventDefault()
    const payload = {
      ...formData,
      speed_mbps: parseInt(formData.speed_mbps) || 5,
      nas_id: formData.nas_id ? parseInt(formData.nas_id) : null,
      show_in_graph: formData.show_in_graph,
      color: formData.color || '#8B5CF6',
    }
    if (formData.direction === 'dscp') {
      payload.dscp_value = parseInt(formData.dscp_value) || 0
      payload.port = ''
      payload.show_in_graph = false // DSCP can't be matched by port in Torch
    } else {
      payload.dscp_value = null
    }
    saveMutation.mutate(payload)
  }

  const handleChange = (e) => {
    const { name, value, type, checked } = e.target
    setFormData((prev) => ({
      ...prev,
      [name]: type === 'checkbox' ? checked : value,
    }))
  }

  const directionLabel = (d, rule) => {
    if (d === 'src') return 'src-port'
    if (d === 'dst') return 'dst-port'
    if (d === 'dscp') return `dscp=${rule?.dscp_value ?? ''}`
    return 'src + dst'
  }

  const directionBadge = (d) => {
    if (d === 'src') return 'badge-info'
    if (d === 'dst') return 'badge-purple'
    if (d === 'dscp') return 'badge-orange'
    return 'badge-success'
  }

  return (
    <div style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Header */}
      <div className="wb-toolbar flex items-center justify-between mb-2">
        <div className="text-[13px] font-semibold">CDN Port Rules</div>
        <div className="flex gap-1">
          <button
            onClick={() => syncAllMutation.mutate()}
            disabled={syncAllMutation.isPending}
            className="btn btn-sm flex items-center gap-1"
          >
            <ArrowPathIcon className={clsx('w-3.5 h-3.5', syncAllMutation.isPending && 'animate-spin')} />
            {syncAllMutation.isPending ? 'Syncing...' : 'Sync All to MikroTik'}
          </button>
          <button onClick={() => openModal()} className="btn btn-primary btn-sm flex items-center gap-1">
            <PlusIcon className="w-3.5 h-3.5" />
            Add Port Rule
          </button>
        </div>
      </div>

      {/* Info Box */}
      <div className="wb-group mb-2">
        <div className="wb-group-title">Port-Based Speed Control</div>
        <div className="wb-group-body text-[11px] text-gray-700 dark:text-[#ccc]">
          <p>
            Create PCQ speed rules based on TCP port numbers. Each rule creates a queue type, mangle rules
            (src-port, dst-port, or both), and a simple queue on MikroTik. No IP subnets needed -- rules match by port only.
          </p>
          <p className="font-mono text-[10px] text-gray-500 dark:text-[#aaa] mt-1">
            Example: Port 8080 -&gt; mark-packet PORT-SP -&gt; PCQ queue 5Mbps
          </p>
        </div>
      </div>

      {/* Table */}
      <div className="table-container">
        <table className="table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Port</th>
              <th>Direction</th>
              <th>Speed</th>
              <th>NAS</th>
              <th>Status</th>
              <th>Graph</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <tr>
                <td colSpan={8} className="text-center py-4">Loading...</td>
              </tr>
            ) : !rules || rules.length === 0 ? (
              <tr>
                <td colSpan={8} className="text-center py-4 text-gray-500 dark:text-[#aaa]">
                  No port rules found. Click "Add Port Rule" to create one.
                </td>
              </tr>
            ) : (
              rules.map((rule) => (
                <tr key={rule.id}>
                  <td className="font-semibold">{rule.name}</td>
                  <td>
                    {rule.direction === 'dscp' ? (
                      <span className="badge badge-orange">DSCP {rule.dscp_value}</span>
                    ) : (
                      <span className="font-mono">:{rule.port}</span>
                    )}
                  </td>
                  <td>
                    <span className={clsx('badge', directionBadge(rule.direction))}>
                      {directionLabel(rule.direction, rule)}
                    </span>
                  </td>
                  <td className="font-semibold">{rule.speed_mbps} Mbps</td>
                  <td>
                    {rule.nas_id
                      ? nasList?.find((n) => n.id === rule.nas_id)?.name || `NAS #${rule.nas_id}`
                      : 'All NAS'}
                  </td>
                  <td>
                    <span className={clsx('badge', rule.is_active ? 'badge-success' : 'badge-gray')}>
                      {rule.is_active ? 'Active' : 'Inactive'}
                    </span>
                  </td>
                  <td>
                    {rule.show_in_graph && rule.direction !== 'dscp' ? (
                      <div className="flex items-center gap-1">
                        <div className="w-2.5 h-2.5 flex-shrink-0 border border-[#999]" style={{ backgroundColor: rule.color || '#8B5CF6', borderRadius: '1px' }} />
                        <ChartBarIcon className="w-3.5 h-3.5" style={{ color: rule.color || '#8B5CF6' }} title="Shows in live graph" />
                      </div>
                    ) : (
                      <span className="text-gray-400 dark:text-[#666]">--</span>
                    )}
                  </td>
                  <td>
                    <div className="flex items-center gap-0.5">
                      <button
                        onClick={() => syncMutation.mutate(rule.id)}
                        disabled={!rule.is_active || syncMutation.isPending}
                        className={clsx('btn btn-sm btn-success', !rule.is_active && 'opacity-40 cursor-not-allowed')}
                        title={rule.is_active ? 'Sync to MikroTik' : 'Rule is inactive'}
                        style={{ padding: '1px 4px' }}
                      >
                        <ArrowPathIcon className={clsx('w-3.5 h-3.5', syncMutation.isPending && 'animate-spin')} />
                      </button>
                      <button
                        onClick={() => openModal(rule)}
                        className="btn btn-sm btn-primary"
                        title="Edit"
                        style={{ padding: '1px 4px' }}
                      >
                        <PencilIcon className="w-3.5 h-3.5" />
                      </button>
                      <button
                        onClick={() => {
                          if (confirm('Delete this port rule?')) deleteMutation.mutate(rule.id)
                        }}
                        className="btn btn-sm btn-danger"
                        title="Delete"
                        style={{ padding: '1px 4px' }}
                      >
                        <TrashIcon className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Modal */}
      {showModal && (
        <div className="modal-overlay">
          <div className="modal" style={{ maxWidth: '460px', width: '100%' }}>
            <div className="modal-header">
              <span>{editingRule ? 'Edit Port Rule' : 'Add Port Rule'}</span>
              <button onClick={closeModal} className="text-white hover:text-gray-200">
                <XMarkIcon className="w-4 h-4" />
              </button>
            </div>

            <form onSubmit={handleSubmit}>
              <div className="modal-body space-y-2" style={{ maxHeight: '70vh', overflowY: 'auto' }}>
                <div>
                  <label className="label block mb-0.5">Rule Name</label>
                  <input
                    type="text"
                    name="name"
                    value={formData.name}
                    onChange={handleChange}
                    className="input input-sm w-full"
                    placeholder="e.g. SP, YouTube, Cache"
                    required
                  />
                </div>

                <div>
                  <label className="label block mb-0.5">Direction</label>
                  <select
                    name="direction"
                    value={formData.direction}
                    onChange={handleChange}
                    className="input input-sm w-full"
                  >
                    {DIRECTIONS.map((d) => (
                      <option key={d.value} value={d.value}>{d.label}</option>
                    ))}
                  </select>
                  <p className="text-[10px] text-gray-500 dark:text-[#aaa] mt-0.5">
                    {formData.direction === 'src' && 'Generates: src-port mangle rule (chain=forward)'}
                    {formData.direction === 'dst' && 'Generates: dst-port mangle rule (chain=forward)'}
                    {formData.direction === 'both' && 'Generates: src-port + dst-port mangle rules (chain=forward)'}
                    {formData.direction === 'dscp' && 'Generates: DSCP mangle rule (chain=postrouting, no port)'}
                  </p>
                </div>

                {formData.direction !== 'dscp' ? (
                  <div>
                    <label className="label block mb-0.5">Port</label>
                    <input
                      type="text"
                      name="port"
                      value={formData.port}
                      onChange={handleChange}
                      className="input input-sm w-full font-mono"
                      placeholder="e.g. 8080"
                      required
                    />
                    <p className="text-[10px] text-gray-500 dark:text-[#aaa] mt-0.5">TCP port number to match</p>
                  </div>
                ) : (
                  <div>
                    <label className="label block mb-0.5">DSCP Value (0-63)</label>
                    <input
                      type="number"
                      name="dscp_value"
                      value={formData.dscp_value}
                      onChange={handleChange}
                      className="input input-sm w-full font-mono"
                      placeholder="e.g. 63"
                      min="0"
                      max="63"
                      required
                    />
                    <p className="text-[10px] text-gray-500 dark:text-[#aaa] mt-0.5">DSCP value (0-63). Common: 46=EF, 34=AF41, 63=custom</p>
                  </div>
                )}

                <div>
                  <label className="label block mb-0.5">Speed Limit (Mbps)</label>
                  <input
                    type="number"
                    name="speed_mbps"
                    value={formData.speed_mbps}
                    onChange={handleChange}
                    className="input input-sm w-full"
                    min="1"
                    required
                  />
                </div>

                <div>
                  <label className="label block mb-0.5">Apply to NAS</label>
                  <select
                    name="nas_id"
                    value={formData.nas_id || ''}
                    onChange={(e) => setFormData((prev) => ({ ...prev, nas_id: e.target.value || null }))}
                    className="input input-sm w-full"
                  >
                    <option value="">All NAS</option>
                    {nasList?.filter((n) => n.is_active).map((nas) => (
                      <option key={nas.id} value={nas.id}>{nas.name} ({nas.ip_address})</option>
                    ))}
                  </select>
                </div>

                <div>
                  <label className="flex items-center gap-2 text-[11px] cursor-pointer">
                    <input
                      type="checkbox"
                      name="is_active"
                      checked={formData.is_active}
                      onChange={handleChange}
                    />
                    <span className="font-semibold">Active</span>
                  </label>
                </div>

                {/* Show in Live Graph toggle -- only for port-based rules */}
                {formData.direction !== 'dscp' && (
                  <div className="border-t border-[#a0a0a0] dark:border-[#555] pt-2 space-y-2">
                    <label className="flex items-start gap-2 text-[11px] cursor-pointer">
                      <input
                        type="checkbox"
                        name="show_in_graph"
                        checked={formData.show_in_graph}
                        onChange={handleChange}
                        className="mt-0.5"
                      />
                      <div>
                        <span className="font-semibold">Show in Live Graph</span>
                        <p className="text-[10px] text-gray-500 dark:text-[#aaa]">When enabled, traffic matching this port will appear as a colored bar in the subscriber live bandwidth graph</p>
                      </div>
                    </label>
                    {formData.show_in_graph && (
                      <div>
                        <label className="label block mb-0.5">Graph Color</label>
                        <div className="flex items-center gap-1 flex-wrap">
                          {GRAPH_COLORS.map(color => (
                            <button
                              key={color}
                              type="button"
                              onClick={() => setFormData(prev => ({ ...prev, color }))}
                              className={clsx(
                                'w-6 h-6 border',
                                formData.color === color ? 'border-[#316AC5] border-2 scale-110' : 'border-[#999] hover:scale-105'
                              )}
                              style={{ backgroundColor: color, borderRadius: '2px' }}
                              title={color}
                            />
                          ))}
                          <input
                            type="color"
                            value={formData.color}
                            onChange={(e) => setFormData(prev => ({ ...prev, color: e.target.value }))}
                            className="w-6 h-6 cursor-pointer border border-[#999]"
                            style={{ borderRadius: '2px', padding: 0 }}
                            title="Custom color"
                          />
                          <span className="text-[10px] text-gray-500 dark:text-[#aaa] font-mono">{formData.color}</span>
                        </div>
                      </div>
                    )}
                  </div>
                )}

                {/* Preview */}
                {formData.name && (formData.direction === 'dscp' ? true : formData.port) && (
                  <div className="bg-[#f0f0f0] dark:bg-[#333] border border-[#a0a0a0] dark:border-[#555] p-1.5 font-mono text-[10px] text-gray-600 dark:text-[#bbb] space-y-0.5" style={{ borderRadius: '2px' }}>
                    <div className="text-gray-400 dark:text-[#888] mb-0.5">MikroTik preview:</div>
                    <div>queue type: PORT-{formData.name}-{formData.speed_mbps} ({formData.speed_mbps}M PCQ)</div>
                    {formData.direction === 'dscp' ? (
                      <div>mangle: chain=postrouting dscp={formData.dscp_value} new-packet-mark=PORT-{formData.name} passthrough=no</div>
                    ) : (
                      <>
                        {(formData.direction === 'src' || formData.direction === 'both') && (
                          <div>mangle: chain=forward protocol=tcp src-port={formData.port} mark=PORT-{formData.name}</div>
                        )}
                        {(formData.direction === 'dst' || formData.direction === 'both') && (
                          <div>mangle: chain=forward protocol=tcp dst-port={formData.port} mark=PORT-{formData.name}</div>
                        )}
                      </>
                    )}
                    <div>queue: PORT-{formData.name}-{formData.speed_mbps}M (packet-mark=PORT-{formData.name})</div>
                  </div>
                )}
              </div>

              <div className="modal-footer">
                <button type="button" onClick={closeModal} className="btn btn-sm">Cancel</button>
                <button type="submit" disabled={saveMutation.isPending} className="btn btn-primary btn-sm">
                  {saveMutation.isPending ? 'Saving...' : editingRule ? 'Update' : 'Create'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
