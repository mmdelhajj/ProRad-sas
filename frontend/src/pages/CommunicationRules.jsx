import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import api from '../services/api'

export default function CommunicationRules() {
  const queryClient = useQueryClient()
  const [showModal, setShowModal] = useState(false)
  const [editingRule, setEditingRule] = useState(null)
  const [formData, setFormData] = useState({
    name: '',
    trigger_event: 'expiry_warning',
    channel: 'sms',
    days_before: 3,
    template: '',
    enabled: true,
    send_to_reseller: false,
    fup_levels: ['1', '2', '3', '4', '5', '6'],
  })

  const { data, isLoading } = useQuery({
    queryKey: ['communication-rules'],
    queryFn: () => api.get('/communication/rules').then(res => res.data.data || [])
  })

  const { data: templates } = useQuery({
    queryKey: ['communication-templates'],
    queryFn: () => api.get('/communication/templates').then(res => res.data.data || [])
  })

  const createMutation = useMutation({
    mutationFn: (data) => api.post('/communication/rules', data),
    onSuccess: () => {
      queryClient.invalidateQueries(['communication-rules'])
      closeModal()
    }
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, ...data }) => api.put(`/communication/rules/${id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries(['communication-rules'])
      closeModal()
    }
  })

  const deleteMutation = useMutation({
    mutationFn: (id) => api.delete(`/communication/rules/${id}`),
    onSuccess: () => queryClient.invalidateQueries(['communication-rules'])
  })

  const toggleMutation = useMutation({
    mutationFn: ({ id, enabled }) => api.put(`/communication/rules/${id}`, { enabled }),
    onSuccess: () => queryClient.invalidateQueries(['communication-rules'])
  })

  const triggerEvents = [
    { value: 'expiry_warning', label: 'Expiry Warning', description: 'Send X days before expiry' },
    { value: 'expired', label: 'Account Expired', description: 'When subscription expires' },
    { value: 'quota_warning', label: 'Quota Warning', description: 'When quota reaches threshold' },
    { value: 'quota_exceeded', label: 'Quota Exceeded', description: 'When quota is fully used' },
    { value: 'payment_received', label: 'Payment Received', description: 'After successful payment' },
    { value: 'account_created', label: 'Account Created', description: 'When new account is created' },
    { value: 'account_renewed', label: 'Account Renewed', description: 'When subscription is renewed' },
    { value: 'password_changed', label: 'Password Changed', description: 'When password is updated' },
    { value: 'session_started', label: 'Session Started', description: 'When user connects' },
    { value: 'fup_applied', label: 'FUP Applied', description: 'When FUP limit is reached' },
  ]

  const channels = [
    { value: 'sms', label: 'SMS' },
    { value: 'email', label: 'Email' },
    { value: 'whatsapp', label: 'WhatsApp' },
  ]

  const closeModal = () => {
    setShowModal(false)
    setEditingRule(null)
    setFormData({
      name: '',
      trigger_event: 'expiry_warning',
      channel: 'sms',
      days_before: 3,
      template: '',
      enabled: true,
      send_to_reseller: false,
      fup_levels: ['1', '2', '3', '4', '5', '6'],
    })
  }

  const openEdit = (rule) => {
    setEditingRule(rule)
    setFormData({
      name: rule.name,
      trigger_event: rule.trigger_event,
      channel: rule.channel,
      days_before: rule.days_before || 0,
      template: rule.template,
      enabled: rule.enabled,
      send_to_reseller: rule.send_to_reseller || false,
      fup_levels: rule.fup_levels ? rule.fup_levels.split(',') : ['1', '2', '3'],
    })
    setShowModal(true)
  }

  const handleSubmit = (e) => {
    e.preventDefault()
    const data = {
      ...formData,
      fup_levels: Array.isArray(formData.fup_levels) ? formData.fup_levels.join(',') : formData.fup_levels,
    }
    if (editingRule) {
      updateMutation.mutate({ id: editingRule.id, ...data })
    } else {
      createMutation.mutate(data)
    }
  }

  const getChannelBadge = (channel) => {
    const map = {
      sms: 'badge-success',
      email: 'badge-info',
      whatsapp: 'badge-success',
    }
    return map[channel] || 'badge-gray'
  }

  const getTriggerLabel = (trigger) => {
    const event = triggerEvents.find(e => e.value === trigger)
    return event ? event.label : trigger
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
          <div className="text-[13px] font-semibold">Communication Rules</div>
          <div className="text-[10px] text-gray-500 dark:text-[#aaa]">Configure automated SMS, Email, and WhatsApp notifications</div>
        </div>
        <button onClick={() => setShowModal(true)} className="btn btn-primary btn-sm">
          Add Rule
        </button>
      </div>

      {/* Summary Stats */}
      <div className="grid grid-cols-4 gap-2">
        <div className="stat-card">
          <div className="text-[14px] font-bold text-gray-900 dark:text-[#e0e0e0]">{(data || []).length}</div>
          <div className="text-[10px] text-gray-500 dark:text-[#aaa]">Total Rules</div>
        </div>
        <div className="stat-card">
          <div className="text-[14px] font-bold text-green-700 dark:text-green-400">{(data || []).filter(r => r.enabled).length}</div>
          <div className="text-[10px] text-gray-500 dark:text-[#aaa]">Active Rules</div>
        </div>
        <div className="stat-card">
          <div className="text-[14px] font-bold text-[#316AC5]">{(data || []).filter(r => r.channel === 'sms').length}</div>
          <div className="text-[10px] text-gray-500 dark:text-[#aaa]">SMS Rules</div>
        </div>
        <div className="stat-card">
          <div className="text-[14px] font-bold text-[#9C27B0]">{(data || []).filter(r => r.channel === 'email').length}</div>
          <div className="text-[10px] text-gray-500 dark:text-[#aaa]">Email Rules</div>
        </div>
      </div>

      {/* Rules Table */}
      <div className="table-container">
        <table className="table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Trigger</th>
              <th>Channel</th>
              <th>Days Before</th>
              <th>Status</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {(data || []).map(rule => (
              <tr key={rule.id}>
                <td style={{ padding: '3px 8px', fontSize: 11 }}>
                  <span className="font-medium">{rule.name}</span>
                  {rule.send_to_reseller && (
                    <div className="text-[10px] text-gray-500 dark:text-[#aaa]">+ Send to Reseller</div>
                  )}
                </td>
                <td style={{ padding: '3px 8px', fontSize: 11 }}>
                  <div>{getTriggerLabel(rule.trigger_event)}</div>
                  {rule.trigger_event === 'fup_applied' && rule.fup_levels && (
                    <div className="text-[10px] text-gray-500 dark:text-[#aaa]">
                      Levels: {rule.fup_levels.split(',').map(l => `FUP ${l}`).join(', ')}
                    </div>
                  )}
                </td>
                <td style={{ padding: '3px 8px', fontSize: 11 }}>
                  <span className={`badge ${getChannelBadge(rule.channel)}`}>
                    {rule.channel.toUpperCase()}
                  </span>
                </td>
                <td style={{ padding: '3px 8px', fontSize: 11 }}>
                  {rule.trigger_event === 'quota_warning'
                    ? (rule.days_before > 0 ? `${rule.days_before}%` : '-')
                    : (rule.days_before > 0 ? `${rule.days_before} days` : '-')
                  }
                </td>
                <td style={{ padding: '3px 8px', fontSize: 11 }}>
                  <input
                    type="checkbox"
                    checked={rule.enabled}
                    onChange={() => toggleMutation.mutate({ id: rule.id, enabled: !rule.enabled })}
                    className="border-[#a0a0a0]"
                  />
                  <span className="ml-1 text-[10px]">{rule.enabled ? 'On' : 'Off'}</span>
                </td>
                <td style={{ padding: '3px 8px', fontSize: 11 }}>
                  <div className="flex items-center gap-0.5">
                    <button onClick={() => openEdit(rule)} className="btn btn-sm" style={{ padding: '1px 4px' }}>
                      <span className="text-[11px]">Edit</span>
                    </button>
                    <button
                      onClick={() => {
                        if (confirm('Delete this rule?')) {
                          deleteMutation.mutate(rule.id)
                        }
                      }}
                      className="btn btn-danger btn-sm"
                      style={{ padding: '1px 4px' }}
                    >
                      <span className="text-[11px]">Delete</span>
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {(data || []).length === 0 && (
        <div className="text-center py-2 text-[11px] text-gray-500 dark:text-[#aaa]">
          No communication rules configured. Click "Add Rule" to create one.
        </div>
      )}

      {/* Add/Edit Modal */}
      {showModal && (
        <div className="modal-overlay">
          <div className="modal" style={{ maxWidth: '480px', width: '100%' }}>
            <div className="modal-header">
              <span>{editingRule ? 'Edit Rule' : 'Add Communication Rule'}</span>
              <button onClick={closeModal} className="text-white hover:text-gray-200 text-[16px] leading-none">&times;</button>
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
                    required
                  />
                </div>

                <div>
                  <label className="label block mb-0.5">Trigger Event</label>
                  <select
                    value={formData.trigger_event}
                    onChange={(e) => {
                      const newTrigger = e.target.value
                      const updates = { trigger_event: newTrigger }
                      if (newTrigger === 'quota_warning') updates.days_before = 80
                      else if (newTrigger === 'expiry_warning') updates.days_before = 3
                      setFormData({ ...formData, ...updates })
                    }}
                    className="input input-sm w-full"
                  >
                    {triggerEvents.map(event => (
                      <option key={event.value} value={event.value}>
                        {event.label} - {event.description}
                      </option>
                    ))}
                  </select>
                </div>

                <div>
                  <label className="label block mb-0.5">Channel</label>
                  <select
                    value={formData.channel}
                    onChange={(e) => setFormData({ ...formData, channel: e.target.value })}
                    className="input input-sm w-full"
                  >
                    {channels.map(ch => (
                      <option key={ch.value} value={ch.value}>{ch.label}</option>
                    ))}
                  </select>
                </div>

                {formData.trigger_event === 'expiry_warning' && (
                  <div>
                    <label className="label block mb-0.5">Days Before Expiry</label>
                    <input
                      type="number"
                      min="1"
                      max="30"
                      value={formData.days_before}
                      onChange={(e) => setFormData({ ...formData, days_before: parseInt(e.target.value) })}
                      className="input input-sm"
                      style={{ width: '120px' }}
                    />
                    <p className="text-[10px] text-gray-500 dark:text-[#aaa] mt-0.5">Send notification this many days before expiry</p>
                  </div>
                )}

                {formData.trigger_event === 'quota_warning' && (
                  <div>
                    <label className="label block mb-0.5">Quota Threshold (%)</label>
                    <input
                      type="number"
                      min="1"
                      max="99"
                      value={formData.days_before}
                      onChange={(e) => setFormData({ ...formData, days_before: parseInt(e.target.value) })}
                      className="input input-sm"
                      style={{ width: '120px' }}
                    />
                    <p className="text-[10px] text-gray-500 dark:text-[#aaa] mt-0.5">Send when monthly quota usage reaches this % (e.g. 80 = notify at 80% used)</p>
                  </div>
                )}

                {formData.trigger_event === 'fup_applied' && (
                  <div>
                    <label className="label block mb-0.5">Trigger on FUP Level</label>
                    <div className="flex gap-4 mt-1">
                      {[
                        { value: '1', label: 'FUP 1' },
                        { value: '2', label: 'FUP 2' },
                        { value: '3', label: 'FUP 3' },
                        { value: '4', label: 'FUP 4' },
                        { value: '5', label: 'FUP 5' },
                        { value: '6', label: 'FUP 6' },
                      ].map(({ value, label }) => (
                        <label key={value} className="flex items-center text-[11px] cursor-pointer">
                          <input
                            type="checkbox"
                            checked={formData.fup_levels.includes(value)}
                            onChange={(e) => {
                              const newLevels = e.target.checked
                                ? [...formData.fup_levels, value].sort()
                                : formData.fup_levels.filter(l => l !== value)
                              setFormData({ ...formData, fup_levels: newLevels })
                            }}
                            className="border-[#a0a0a0] mr-1"
                          />
                          {label}
                        </label>
                      ))}
                    </div>
                    <p className="text-[10px] text-gray-500 dark:text-[#aaa] mt-0.5">
                      Select which FUP levels trigger this rule. You can create separate rules for each level.
                    </p>
                  </div>
                )}

                <div>
                  <label className="label block mb-0.5">Message Template</label>
                  <textarea
                    value={formData.template}
                    onChange={(e) => setFormData({ ...formData, template: e.target.value })}
                    rows={3}
                    className="input input-sm w-full"
                    style={{ minHeight: '70px' }}
                    placeholder="Use variables: {username}, {full_name}, {expiry_date}, {service_name}, {balance}"
                  />
                  <p className="text-[10px] text-gray-500 dark:text-[#aaa] mt-0.5">
                    Available variables: {'{username}'}, {'{full_name}'}, {'{service_name}'}, {'{balance}'}
                    {['expiry_warning', 'expired'].includes(formData.trigger_event) && <>, {'{expiry_date}'}, <strong>{'{days_before}'}</strong> (days until expiry)</>}
                    {formData.trigger_event === 'fup_applied' && <>, {'{quota_used}'}, {'{quota_total}'}, <strong>{'{fup_level}'}</strong> (1, 2, or 3)</>}
                    {formData.trigger_event === 'quota_warning' && <>, <strong>{'{quota_used}'}</strong> (GB used), <strong>{'{quota_total}'}</strong> (GB total), <strong>{'{quota_percent}'}</strong> (% used)</>}
                  </p>
                </div>

                <div className="flex items-center gap-4">
                  <label className="flex items-center text-[11px] cursor-pointer">
                    <input
                      type="checkbox"
                      checked={formData.enabled}
                      onChange={(e) => setFormData({ ...formData, enabled: e.target.checked })}
                      className="border-[#a0a0a0] mr-1"
                    />
                    Enabled
                  </label>
                  <label className="flex items-center text-[11px] cursor-pointer">
                    <input
                      type="checkbox"
                      checked={formData.send_to_reseller}
                      onChange={(e) => setFormData({ ...formData, send_to_reseller: e.target.checked })}
                      className="border-[#a0a0a0] mr-1"
                    />
                    Also notify Reseller
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
