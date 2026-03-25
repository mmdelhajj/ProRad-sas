import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { PlayIcon } from '@heroicons/react/24/outline'
import api from '../services/api'
import toast from 'react-hot-toast'
import clsx from 'clsx'

export default function CDNBandwidthRules() {
  const queryClient = useQueryClient()
  const [showModal, setShowModal] = useState(false)
  const [editingRule, setEditingRule] = useState(null)
  const [formData, setFormData] = useState({
    name: '',
    start_time: '16:00',
    end_time: '23:00',
    days_of_week: [0, 1, 2, 3, 4, 5, 6],
    speed_multiplier: 100,
    cdn_ids: [],
    priority: 10,
    enabled: true,
    auto_apply: true,
  })

  const { data: rules, isLoading } = useQuery({
    queryKey: ['cdn-bandwidth-rules'],
    queryFn: () => api.get('/cdn-bandwidth-rules').then(res => res.data.data || [])
  })

  const { data: cdns } = useQuery({
    queryKey: ['cdns'],
    queryFn: () => api.get('/cdns').then(res => res.data.data || [])
  })

  const createMutation = useMutation({
    mutationFn: (data) => api.post('/cdn-bandwidth-rules', data),
    onSuccess: () => {
      queryClient.invalidateQueries(['cdn-bandwidth-rules'])
      toast.success('CDN bandwidth rule created')
      closeModal()
    },
    onError: (err) => {
      toast.error(err.response?.data?.message || 'Failed to create rule')
    }
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, ...data }) => api.put(`/cdn-bandwidth-rules/${id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries(['cdn-bandwidth-rules'])
      toast.success('CDN bandwidth rule updated')
      closeModal()
    },
    onError: (err) => {
      toast.error(err.response?.data?.message || 'Failed to update rule')
    }
  })

  const deleteMutation = useMutation({
    mutationFn: (id) => api.delete(`/cdn-bandwidth-rules/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries(['cdn-bandwidth-rules'])
      toast.success('Rule deleted')
    }
  })

  const toggleMutation = useMutation({
    mutationFn: ({ id, enabled }) => api.put(`/cdn-bandwidth-rules/${id}`, { enabled }),
    onSuccess: () => queryClient.invalidateQueries(['cdn-bandwidth-rules'])
  })

  const applyNowMutation = useMutation({
    mutationFn: (id) => api.post(`/cdn-bandwidth-rules/${id}/apply`),
    onSuccess: (res) => {
      toast.success(`CDN rule applied to ${res.data.applied_count} queues`)
    },
    onError: (err) => {
      toast.error(err.response?.data?.message || 'Failed to apply rule')
    }
  })

  const daysOfWeek = [
    { value: 0, label: 'Sun' },
    { value: 1, label: 'Mon' },
    { value: 2, label: 'Tue' },
    { value: 3, label: 'Wed' },
    { value: 4, label: 'Thu' },
    { value: 5, label: 'Fri' },
    { value: 6, label: 'Sat' },
  ]

  const closeModal = () => {
    setShowModal(false)
    setEditingRule(null)
    setFormData({
      name: '',
      start_time: '16:00',
      end_time: '23:00',
      days_of_week: [0, 1, 2, 3, 4, 5, 6],
      speed_multiplier: 100,
      cdn_ids: [],
      priority: 10,
      enabled: true,
      auto_apply: true,
    })
  }

  const openEdit = (rule) => {
    setEditingRule(rule)
    setFormData({
      name: rule.name,
      start_time: rule.start_time || '16:00',
      end_time: rule.end_time || '23:00',
      days_of_week: rule.days_of_week || [0, 1, 2, 3, 4, 5, 6],
      speed_multiplier: rule.speed_multiplier || 100,
      cdn_ids: rule.cdn_ids || [],
      priority: rule.priority || 10,
      enabled: rule.enabled,
      auto_apply: rule.auto_apply || false,
    })
    setShowModal(true)
  }

  const handleSubmit = (e) => {
    e.preventDefault()
    if (formData.cdn_ids.length === 0) {
      toast.error('Please select at least one CDN')
      return
    }
    if (editingRule) {
      updateMutation.mutate({ id: editingRule.id, ...formData })
    } else {
      createMutation.mutate(formData)
    }
  }

  const toggleDay = (day) => {
    const days = formData.days_of_week.includes(day)
      ? formData.days_of_week.filter(d => d !== day)
      : [...formData.days_of_week, day].sort()
    setFormData({ ...formData, days_of_week: days })
  }

  const getDaysLabel = (days) => {
    if (!days || days.length === 0) return 'Never'
    if (days.length === 7) return 'Every day'
    if (JSON.stringify([...days].sort()) === JSON.stringify([1, 2, 3, 4, 5])) return 'Weekdays'
    if (JSON.stringify([...days].sort()) === JSON.stringify([0, 6])) return 'Weekends'
    return days.map(d => daysOfWeek.find(dw => dw.value === d)?.label).join(', ')
  }

  const getCDNNames = (cdnIds) => {
    if (!cdnIds || cdnIds.length === 0) return 'No CDNs selected'
    const names = cdnIds.map(id => cdns?.find(c => c.id === id)?.name).filter(Boolean)
    return names.length > 0 ? names.join(', ') : 'No CDNs selected'
  }

  if (isLoading) {
    return <div className="text-center py-4 text-[11px] text-gray-500 dark:text-[#aaa]">Loading...</div>
  }

  return (
    <div style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Header */}
      <div className="wb-toolbar flex justify-between items-center mb-2">
        <div className="text-[13px] font-semibold">CDN Speed Rules</div>
        <button
          onClick={() => setShowModal(true)}
          className="btn btn-primary btn-sm"
        >
          Add Rule
        </button>
      </div>

      {/* Info Box */}
      <div className="wb-group mb-2">
        <div className="wb-group-title">Time-Based CDN Speed Rules</div>
        <div className="wb-group-body text-[11px] text-gray-700 dark:text-[#ccc]">
          Create rules to adjust CDN speeds during peak hours. For example, reduce CDN traffic to 50% during evening hours (16:00-23:00) to prioritize regular user traffic.
          Enable Auto Apply to have rules activate automatically on schedule.
        </div>
      </div>

      {/* Rules Table */}
      <div className="table-container">
        <table className="table">
          <thead>
            <tr>
              <th>Name</th>
              <th>CDNs</th>
              <th>Schedule</th>
              <th>Speed</th>
              <th>Status</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {(rules || []).length === 0 ? (
              <tr>
                <td colSpan={6} className="text-center py-4 text-gray-500 dark:text-[#aaa]">
                  No CDN bandwidth rules configured. Click "Add Rule" to create one.
                </td>
              </tr>
            ) : (
              (rules || []).map(rule => (
                <tr key={rule.id}>
                  <td className="font-semibold">{rule.name}</td>
                  <td>{getCDNNames(rule.cdn_ids)}</td>
                  <td>
                    <div>
                      <div>{rule.start_time} - {rule.end_time}</div>
                      <div className="text-[10px] text-gray-500 dark:text-[#aaa]">{getDaysLabel(rule.days_of_week)}</div>
                    </div>
                  </td>
                  <td>
                    <span className={clsx('font-semibold', rule.speed_multiplier > 100 ? 'text-[#4CAF50]' : rule.speed_multiplier < 100 ? 'text-[#FF9800]' : '')}>
                      {rule.speed_multiplier}%
                    </span>
                    {rule.speed_multiplier < 100 && (
                      <span className="text-[10px] text-gray-400 dark:text-[#888] ml-1">
                        ({100 - rule.speed_multiplier}% reduction)
                      </span>
                    )}
                  </td>
                  <td>
                    <div className="flex items-center gap-1">
                      <span className={clsx('badge', rule.enabled ? 'badge-success' : 'badge-gray')}>
                        {rule.enabled ? 'ON' : 'OFF'}
                      </span>
                      {rule.auto_apply && (
                        <span className="badge badge-info">AUTO</span>
                      )}
                    </div>
                  </td>
                  <td>
                    <div className="flex items-center gap-0.5">
                      <button
                        onClick={() => applyNowMutation.mutate(rule.id)}
                        disabled={applyNowMutation.isPending}
                        className="btn btn-sm btn-success flex items-center gap-0.5"
                        title="Apply Now"
                        style={{ padding: '1px 4px' }}
                      >
                        <PlayIcon className="w-3 h-3" />
                        <span className="text-[10px]">Apply</span>
                      </button>
                      <button
                        onClick={() => toggleMutation.mutate({ id: rule.id, enabled: !rule.enabled })}
                        className={clsx('btn btn-sm', rule.enabled ? 'btn-warning' : 'btn-success')}
                        style={{ padding: '1px 4px' }}
                        title={rule.enabled ? 'Disable' : 'Enable'}
                      >
                        <span className="text-[10px]">{rule.enabled ? 'Disable' : 'Enable'}</span>
                      </button>
                      <button
                        onClick={() => openEdit(rule)}
                        className="btn btn-sm btn-primary"
                        style={{ padding: '1px 4px' }}
                      >
                        <span className="text-[10px]">Edit</span>
                      </button>
                      <button
                        onClick={() => {
                          if (confirm('Delete this rule?')) {
                            deleteMutation.mutate(rule.id)
                          }
                        }}
                        className="btn btn-sm btn-danger"
                        style={{ padding: '1px 4px' }}
                      >
                        <span className="text-[10px]">Delete</span>
                      </button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Add/Edit Modal */}
      {showModal && (
        <div className="modal-overlay">
          <div className="modal" style={{ maxWidth: '500px', width: '100%' }}>
            <div className="modal-header">
              {editingRule ? 'Edit CDN Bandwidth Rule' : 'Add CDN Bandwidth Rule'}
            </div>
            <form onSubmit={handleSubmit}>
              <div className="modal-body space-y-2" style={{ maxHeight: '70vh', overflowY: 'auto' }}>
                <div>
                  <label className="label block mb-0.5">Rule Name</label>
                  <input
                    type="text"
                    value={formData.name}
                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                    className="input input-sm w-full"
                    placeholder="e.g., Peak Hours CDN Limit"
                    required
                  />
                </div>

                <div className="grid grid-cols-2 gap-2">
                  <div>
                    <label className="label block mb-0.5">Start Time</label>
                    <input
                      type="time"
                      value={formData.start_time}
                      onChange={(e) => setFormData({ ...formData, start_time: e.target.value })}
                      className="input input-sm w-full"
                    />
                  </div>
                  <div>
                    <label className="label block mb-0.5">End Time</label>
                    <input
                      type="time"
                      value={formData.end_time}
                      onChange={(e) => setFormData({ ...formData, end_time: e.target.value })}
                      className="input input-sm w-full"
                    />
                  </div>
                </div>

                <div>
                  <label className="label block mb-1">Days of Week</label>
                  <div className="flex flex-wrap gap-1">
                    {daysOfWeek.map(day => (
                      <button
                        key={day.value}
                        type="button"
                        onClick={() => toggleDay(day.value)}
                        className={clsx(
                          'btn btn-sm',
                          formData.days_of_week.includes(day.value) ? 'btn-primary' : ''
                        )}
                        style={{ padding: '2px 8px' }}
                      >
                        {day.label}
                      </button>
                    ))}
                  </div>
                </div>

                <div className="wb-group">
                  <div className="wb-group-body text-[10px] text-gray-700 dark:text-[#ccc]">
                    Speed multiplier: 100% = same speed, 200% = double speed, 50% = half speed
                  </div>
                </div>

                <div>
                  <label className="label block mb-0.5">
                    CDN Speed: <span className={clsx('font-bold', formData.speed_multiplier > 100 ? 'text-[#4CAF50]' : formData.speed_multiplier < 100 ? 'text-[#FF9800]' : '')}>{formData.speed_multiplier}%</span>
                    {formData.speed_multiplier < 100 && (
                      <span className="font-normal text-gray-500 dark:text-[#aaa] ml-1">({100 - formData.speed_multiplier}% reduction)</span>
                    )}
                  </label>
                  <input
                    type="range"
                    min="10"
                    max="500"
                    step="10"
                    value={formData.speed_multiplier}
                    onChange={(e) => setFormData({ ...formData, speed_multiplier: parseInt(e.target.value) })}
                    className="mt-1 w-full h-1.5 bg-[#e0e0e0] dark:bg-[#555] appearance-none cursor-pointer accent-[#316AC5]"
                    style={{ borderRadius: '1px' }}
                  />
                  <div className="flex justify-between text-[9px] text-gray-400 dark:text-[#888] mt-0.5">
                    <span>10%</span>
                    <span>50%</span>
                    <span>100%</span>
                    <span>200%</span>
                    <span>300%</span>
                    <span>500%</span>
                  </div>
                </div>

                <div>
                  <label className="label block mb-0.5">
                    Apply to CDNs <span className="text-[#f44336]">*</span>
                  </label>
                  <div className={clsx(
                    'max-h-24 overflow-y-auto border p-1.5',
                    formData.cdn_ids.length === 0 ? 'border-[#f44336] bg-[#fff0f0] dark:bg-[#3a2020]' : 'border-[#a0a0a0] dark:border-[#555] bg-white dark:bg-[#333]'
                  )} style={{ borderRadius: '2px' }}>
                    {(cdns || []).map(cdn => (
                      <label key={cdn.id} className="flex items-center py-0.5 text-[11px] cursor-pointer hover:bg-[#e8e8f0] dark:hover:bg-[#444]">
                        <input
                          type="checkbox"
                          checked={formData.cdn_ids.includes(cdn.id)}
                          onChange={(e) => {
                            const ids = e.target.checked
                              ? [...formData.cdn_ids, cdn.id]
                              : formData.cdn_ids.filter(id => id !== cdn.id)
                            setFormData({ ...formData, cdn_ids: ids })
                          }}
                          className="mr-2"
                        />
                        <span
                          className="w-2.5 h-2.5 mr-1.5 border border-[#999] flex-shrink-0"
                          style={{ backgroundColor: cdn.color || '#EF4444', borderRadius: '1px' }}
                        />
                        {cdn.name}
                      </label>
                    ))}
                  </div>
                  <p className="text-[10px] text-[#f44336] mt-0.5">Select at least one CDN (required)</p>
                </div>

                <div className="space-y-0.5">
                  <label className="flex items-center gap-2 text-[11px] cursor-pointer">
                    <input
                      type="checkbox"
                      checked={formData.enabled}
                      onChange={(e) => setFormData({ ...formData, enabled: e.target.checked })}
                    />
                    <span className="font-semibold">Enabled</span>
                  </label>
                  <label className="flex items-center gap-2 text-[11px] cursor-pointer">
                    <input
                      type="checkbox"
                      checked={formData.auto_apply}
                      onChange={(e) => setFormData({ ...formData, auto_apply: e.target.checked })}
                    />
                    <span className="font-semibold">Auto Apply</span>
                    <span className="text-[10px] text-gray-500 dark:text-[#aaa]">(apply automatically on schedule)</span>
                  </label>
                </div>
              </div>

              <div className="modal-footer">
                <button type="button" onClick={closeModal} className="btn btn-sm">Cancel</button>
                <button
                  type="submit"
                  disabled={createMutation.isPending || updateMutation.isPending}
                  className="btn btn-primary btn-sm"
                >
                  {editingRule ? 'Update' : 'Create'} Rule
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
