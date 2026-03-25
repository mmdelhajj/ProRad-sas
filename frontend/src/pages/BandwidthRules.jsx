import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { PlayIcon } from '@heroicons/react/24/outline'
import api from '../services/api'
import toast from 'react-hot-toast'

export default function BandwidthRules() {
  const queryClient = useQueryClient()
  const [showModal, setShowModal] = useState(false)
  const [editingRule, setEditingRule] = useState(null)
  const [formData, setFormData] = useState({
    name: '',
    trigger_type: 'time',
    start_time: '00:00',
    end_time: '06:00',
    days_of_week: [0, 1, 2, 3, 4, 5, 6],
    upload_multiplier: 100,
    download_multiplier: 100,
    service_ids: [],
    priority: 10,
    enabled: true,
    auto_apply: true,
  })

  const { data: rules, isLoading } = useQuery({
    queryKey: ['bandwidth-rules'],
    queryFn: () => api.get('/bandwidth/rules').then(res => res.data.data || [])
  })

  const { data: services } = useQuery({
    queryKey: ['services'],
    queryFn: () => api.get('/services').then(res => res.data.data || [])
  })

  const createMutation = useMutation({
    mutationFn: (data) => api.post('/bandwidth/rules', data),
    onSuccess: () => {
      queryClient.invalidateQueries(['bandwidth-rules'])
      closeModal()
    }
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, ...data }) => api.put(`/bandwidth/rules/${id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries(['bandwidth-rules'])
      closeModal()
    }
  })

  const deleteMutation = useMutation({
    mutationFn: (id) => api.delete(`/bandwidth/rules/${id}`),
    onSuccess: () => queryClient.invalidateQueries(['bandwidth-rules'])
  })

  const toggleMutation = useMutation({
    mutationFn: ({ id, enabled }) => api.put(`/bandwidth/rules/${id}`, { enabled }),
    onSuccess: () => queryClient.invalidateQueries(['bandwidth-rules'])
  })

  const applyNowMutation = useMutation({
    mutationFn: (id) => api.post(`/bandwidth/rules/${id}/apply`),
    onSuccess: (res) => {
      toast.success(`Rule applied to ${res.data.applied_count} subscribers`)
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
      trigger_type: 'time',
      start_time: '00:00',
      end_time: '06:00',
      days_of_week: [0, 1, 2, 3, 4, 5, 6],
      upload_multiplier: 100,
      download_multiplier: 100,
      service_ids: [],
      priority: 10,
      enabled: true,
      auto_apply: false,
    })
  }

  const openEdit = (rule) => {
    setEditingRule(rule)
    setFormData({
      name: rule.name,
      trigger_type: rule.trigger_type,
      start_time: rule.start_time || '00:00',
      end_time: rule.end_time || '06:00',
      days_of_week: rule.days_of_week || [0, 1, 2, 3, 4, 5, 6],
      upload_multiplier: rule.upload_multiplier || 100,
      download_multiplier: rule.download_multiplier || 100,
      service_ids: rule.service_ids || [],
      priority: rule.priority || 10,
      enabled: rule.enabled,
      auto_apply: rule.auto_apply || false,
    })
    setShowModal(true)
  }

  const handleSubmit = (e) => {
    e.preventDefault()
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
    if (JSON.stringify(days.sort()) === JSON.stringify([1, 2, 3, 4, 5])) return 'Weekdays'
    if (JSON.stringify(days.sort()) === JSON.stringify([0, 6])) return 'Weekends'
    return days.map(d => daysOfWeek.find(dw => dw.value === d)?.label).join(', ')
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-4">
        <span className="text-[11px] text-gray-500 dark:text-[#aaa]">Loading...</span>
      </div>
    )
  }

  return (
    <div className="space-y-2" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Header */}
      <div className="wb-toolbar flex justify-between items-center">
        <div>
          <div className="text-[13px] font-semibold">Speed Rules</div>
          <div className="text-[10px] text-gray-500 dark:text-[#aaa]">Configure time-based bandwidth adjustments</div>
        </div>
        <button onClick={() => setShowModal(true)} className="btn btn-primary btn-sm">
          Add Rule
        </button>
      </div>

      {/* Info Box */}
      <div className="wb-group">
        <div className="wb-group-body">
          <p className="text-[11px] text-gray-700 dark:text-[#ccc]">
            <strong>Time-Based Speed Rules:</strong> Create rules to adjust speed during specific hours (e.g., night boost). Enable Auto Apply to have rules activate automatically on schedule.
          </p>
        </div>
      </div>

      {/* Rules Table */}
      <div className="table-container">
        <table className="table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Schedule</th>
              <th>Speed Adjustment</th>
              <th>Status</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {(rules || []).map(rule => (
              <tr key={rule.id}>
                <td>
                  <span className="font-medium">{rule.name}</span>
                  {rule.service_ids?.length > 0 && (
                    <div className="text-[10px] text-gray-500 dark:text-[#aaa]">{rule.service_ids.length} service(s)</div>
                  )}
                </td>
                <td>
                  <div>{rule.start_time} - {rule.end_time}</div>
                  <div className="text-[10px] text-gray-500 dark:text-[#aaa]">{getDaysLabel(rule.days_of_week)}</div>
                </td>
                <td>
                  <span className={rule.upload_multiplier > 100 ? 'text-green-700' : rule.upload_multiplier < 100 ? 'text-red-700' : ''}>
                    {rule.upload_multiplier > 100 ? '+' : ''}{rule.upload_multiplier - 100}% UP
                  </span>
                  {' | '}
                  <span className={rule.download_multiplier > 100 ? 'text-green-700' : rule.download_multiplier < 100 ? 'text-red-700' : ''}>
                    {rule.download_multiplier > 100 ? '+' : ''}{rule.download_multiplier - 100}% DN
                  </span>
                </td>
                <td>
                  <div className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={rule.enabled}
                      onChange={() => toggleMutation.mutate({ id: rule.id, enabled: !rule.enabled })}
                      className="border-[#a0a0a0]"
                    />
                    <span className="text-[10px]">{rule.enabled ? 'On' : 'Off'}</span>
                    {rule.auto_apply && (
                      <span className="badge-success">AUTO</span>
                    )}
                  </div>
                </td>
                <td>
                  <div className="flex items-center gap-1">
                    <button
                      onClick={() => applyNowMutation.mutate(rule.id)}
                      disabled={applyNowMutation.isPending}
                      className="btn btn-success btn-sm"
                      title="Apply Now"
                    >
                      <PlayIcon className="w-3 h-3 mr-1" />
                      Apply
                    </button>
                    <button onClick={() => openEdit(rule)} className="btn btn-sm">
                      Edit
                    </button>
                    <button
                      onClick={() => {
                        if (confirm('Delete this rule?')) {
                          deleteMutation.mutate(rule.id)
                        }
                      }}
                      className="btn btn-danger btn-sm"
                    >
                      Delete
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {(rules || []).length === 0 && (
        <div className="text-center py-4 text-[11px] text-gray-500 dark:text-[#aaa]">
          No bandwidth rules configured. Click "Add Rule" to create one.
        </div>
      )}

      {/* Add/Edit Modal */}
      {showModal && (
        <div className="modal-overlay">
          <div className="modal" style={{ width: '480px' }}>
            <div className="modal-header">
              <span>{editingRule ? 'Edit Rule' : 'Add Bandwidth Rule'}</span>
              <button onClick={closeModal} className="text-white hover:text-gray-200 text-[16px] leading-none">&times;</button>
            </div>
            <form onSubmit={handleSubmit}>
              <div className="modal-body space-y-2">
                {/* Rule Name */}
                <div>
                  <label className="label">Rule Name</label>
                  <input
                    type="text"
                    value={formData.name}
                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                    className="input input-sm w-full"
                    placeholder="e.g., Night Boost, FUP Limit"
                    required
                  />
                </div>

                {/* Times */}
                <div className="grid grid-cols-2 gap-2">
                  <div>
                    <label className="label">Start Time</label>
                    <input
                      type="time"
                      value={formData.start_time}
                      onChange={(e) => setFormData({ ...formData, start_time: e.target.value })}
                      className="input input-sm w-full"
                    />
                  </div>
                  <div>
                    <label className="label">End Time</label>
                    <input
                      type="time"
                      value={formData.end_time}
                      onChange={(e) => setFormData({ ...formData, end_time: e.target.value })}
                      className="input input-sm w-full"
                    />
                  </div>
                </div>

                {/* Days of Week */}
                <div>
                  <label className="label">Days of Week</label>
                  <div className="flex flex-wrap gap-1">
                    {daysOfWeek.map(day => (
                      <button
                        key={day.value}
                        type="button"
                        onClick={() => toggleDay(day.value)}
                        className={`btn btn-sm ${formData.days_of_week.includes(day.value) ? 'btn-primary' : ''}`}
                      >
                        {day.label}
                      </button>
                    ))}
                  </div>
                </div>

                {/* Speed info */}
                <div className="wb-group">
                  <div className="wb-group-body text-[10px] text-gray-600 dark:text-[#bbb]">
                    Speed multiplier: 100% = same speed, 200% = double speed, 50% = half speed
                  </div>
                </div>

                {/* Speed sliders */}
                <div className="grid grid-cols-2 gap-2">
                  <div>
                    <label className="label">
                      Upload Speed: <span className={`font-bold ${formData.upload_multiplier > 100 ? 'text-green-700' : formData.upload_multiplier < 100 ? 'text-red-700' : ''}`}>{formData.upload_multiplier}%</span>
                    </label>
                    <input
                      type="range"
                      min="10"
                      max="500"
                      step="10"
                      value={formData.upload_multiplier}
                      onChange={(e) => setFormData({ ...formData, upload_multiplier: parseInt(e.target.value) })}
                      className="w-full h-1.5 bg-[#e0e0e0] dark:bg-[#555] appearance-none cursor-pointer accent-[#316AC5]"
                      style={{ borderRadius: '1px' }}
                    />
                    <div className="flex justify-between text-[9px] text-gray-400 dark:text-[#888] mt-0.5">
                      <span>10%</span><span>100%</span><span>300%</span><span>500%</span>
                    </div>
                  </div>
                  <div>
                    <label className="label">
                      Download Speed: <span className={`font-bold ${formData.download_multiplier > 100 ? 'text-green-700' : formData.download_multiplier < 100 ? 'text-red-700' : ''}`}>{formData.download_multiplier}%</span>
                    </label>
                    <input
                      type="range"
                      min="10"
                      max="500"
                      step="10"
                      value={formData.download_multiplier}
                      onChange={(e) => setFormData({ ...formData, download_multiplier: parseInt(e.target.value) })}
                      className="w-full h-1.5 bg-[#e0e0e0] dark:bg-[#555] appearance-none cursor-pointer accent-[#316AC5]"
                      style={{ borderRadius: '1px' }}
                    />
                    <div className="flex justify-between text-[9px] text-gray-400 dark:text-[#888] mt-0.5">
                      <span>10%</span><span>100%</span><span>300%</span><span>500%</span>
                    </div>
                  </div>
                </div>

                {/* Apply to Services */}
                <div>
                  <label className="label">Apply to Services</label>
                  <div className="border border-[#a0a0a0] dark:border-[#555] bg-white dark:bg-[#333] max-h-24 overflow-y-auto p-1" style={{ borderRadius: '2px' }}>
                    {(services || []).map(service => (
                      <label key={service.id} className="flex items-center py-0.5 px-1 text-[11px] hover:bg-[#e8e8f0] dark:hover:bg-[#444] cursor-pointer">
                        <input
                          type="checkbox"
                          checked={formData.service_ids.includes(service.id)}
                          onChange={(e) => {
                            const ids = e.target.checked
                              ? [...formData.service_ids, service.id]
                              : formData.service_ids.filter(id => id !== service.id)
                            setFormData({ ...formData, service_ids: ids })
                          }}
                          className="border-[#a0a0a0] mr-1.5"
                        />
                        {service.name}
                      </label>
                    ))}
                  </div>
                  <p className="text-[10px] text-gray-500 dark:text-[#aaa] mt-0.5">Leave empty to apply to all services</p>
                </div>

                {/* Checkboxes */}
                <div className="space-y-0.5">
                  <label className="flex items-center text-[11px] cursor-pointer">
                    <input
                      type="checkbox"
                      checked={formData.enabled}
                      onChange={(e) => setFormData({ ...formData, enabled: e.target.checked })}
                      className="border-[#a0a0a0] mr-1.5"
                    />
                    Enabled
                  </label>
                  <label className="flex items-center text-[11px] cursor-pointer">
                    <input
                      type="checkbox"
                      checked={formData.auto_apply}
                      onChange={(e) => setFormData({ ...formData, auto_apply: e.target.checked })}
                      className="border-[#a0a0a0] mr-1.5"
                    />
                    Auto Apply (apply automatically on schedule)
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
