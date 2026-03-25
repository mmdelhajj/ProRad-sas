import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { notificationBannerApi, resellerApi } from '../services/api'
import { useAuthStore } from '../store/authStore'
import {
  PlusIcon,
  PencilSquareIcon,
  TrashIcon,
  BellAlertIcon,
  InformationCircleIcon,
  ExclamationTriangleIcon,
  ExclamationCircleIcon,
  CheckCircleIcon,
} from '@heroicons/react/24/outline'

const typeOptions = [
  { value: 'info', label: 'Info', icon: InformationCircleIcon, color: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300' },
  { value: 'warning', label: 'Warning', icon: ExclamationTriangleIcon, color: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300' },
  { value: 'error', label: 'Error', icon: ExclamationCircleIcon, color: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300' },
  { value: 'success', label: 'Success', icon: CheckCircleIcon, color: 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300' },
]

const adminTargetOptions = [
  { value: 'all', label: 'All Users' },
  { value: 'resellers', label: 'Resellers Only' },
  { value: 'subscribers', label: 'Subscribers Only' },
  { value: 'sub_resellers', label: 'Sub-Resellers' },
]

const resellerTargetOptions = [
  { value: 'subscribers', label: 'My Subscribers' },
  { value: 'sub_resellers', label: 'My Sub-Resellers' },
]

function formatDate(dateStr) {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  return d.toLocaleString('en-US', { month: 'short', day: 'numeric', year: 'numeric', hour: '2-digit', minute: '2-digit' })
}

function isActive(banner) {
  const now = new Date()
  return banner.enabled && new Date(banner.start_date) <= now && new Date(banner.end_date) >= now
}

export default function NotificationBanners() {
  const queryClient = useQueryClient()
  const { isAdmin, isReseller } = useAuthStore()
  const [showModal, setShowModal] = useState(false)
  const [editing, setEditing] = useState(null)
  const [form, setForm] = useState({
    title: '',
    message: '',
    banner_type: 'info',
    target: 'all',
    target_ids: '',
    start_date: '',
    end_date: '',
    dismissible: true,
    enabled: true,
  })

  const { data, isLoading } = useQuery({
    queryKey: ['notification-banners'],
    queryFn: () => notificationBannerApi.list().then(res => res.data.data || []),
  })

  // Admin: fetch all resellers for targeting
  const { data: resellers } = useQuery({
    queryKey: ['resellers-list'],
    queryFn: () => resellerApi.list({ limit: 500 }).then(res => res.data.data || res.data.resellers || []),
    enabled: isAdmin(),
  })

  // Reseller: fetch sub-resellers for targeting
  const { data: subResellers } = useQuery({
    queryKey: ['sub-resellers-list'],
    queryFn: () => notificationBannerApi.getSubResellers().then(res => res.data.data || []),
    enabled: isReseller(),
  })

  const createMutation = useMutation({
    mutationFn: (data) => notificationBannerApi.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries(['notification-banners'])
      queryClient.invalidateQueries(['notification-banners-active'])
      closeModal()
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, ...data }) => notificationBannerApi.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries(['notification-banners'])
      queryClient.invalidateQueries(['notification-banners-active'])
      closeModal()
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id) => notificationBannerApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries(['notification-banners'])
      queryClient.invalidateQueries(['notification-banners-active'])
    },
  })

  const closeModal = () => {
    setShowModal(false)
    setEditing(null)
    setForm({
      title: '',
      message: '',
      banner_type: 'info',
      target: 'all',
      target_ids: '',
      start_date: '',
      end_date: '',
      dismissible: true,
      enabled: true,
    })
  }

  const openCreate = () => {
    setEditing(null)
    const now = new Date()
    const nextWeek = new Date(now.getTime() + 7 * 24 * 60 * 60 * 1000)
    setForm({
      title: '',
      message: '',
      banner_type: 'info',
      target: isAdmin() ? 'all' : 'subscribers',
      target_ids: '',
      start_date: now.toISOString().slice(0, 16),
      end_date: nextWeek.toISOString().slice(0, 16),
      dismissible: true,
      enabled: true,
    })
    setShowModal(true)
  }

  const openEdit = (banner) => {
    setEditing(banner)
    setForm({
      title: banner.title,
      message: banner.message,
      banner_type: banner.banner_type,
      target: banner.target,
      target_ids: banner.target_ids || '',
      start_date: banner.start_date ? new Date(banner.start_date).toISOString().slice(0, 16) : '',
      end_date: banner.end_date ? new Date(banner.end_date).toISOString().slice(0, 16) : '',
      dismissible: banner.dismissible,
      enabled: banner.enabled,
    })
    setShowModal(true)
  }

  const handleSubmit = (e) => {
    e.preventDefault()
    if (editing) {
      updateMutation.mutate({ id: editing.id, ...form })
    } else {
      createMutation.mutate(form)
    }
  }

  const handleDelete = (id) => {
    if (window.confirm('Delete this notification banner?')) {
      deleteMutation.mutate(id)
    }
  }

  const banners = data || []
  const isSaving = createMutation.isPending || updateMutation.isPending

  // Selected reseller IDs for multi-select
  const selectedResellerIds = form.target_ids ? form.target_ids.split(',').map(s => s.trim()).filter(Boolean) : []

  const toggleReseller = (id) => {
    const idStr = String(id)
    const current = new Set(selectedResellerIds)
    if (current.has(idStr)) {
      current.delete(idStr)
    } else {
      current.add(idStr)
    }
    setForm({ ...form, target_ids: [...current].join(',') })
  }

  return (
    <div>
      {/* Header */}
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <BellAlertIcon className="w-5 h-5 text-blue-600 dark:text-blue-400" />
          <h1 className="text-lg font-bold text-gray-900 dark:text-white">Notification Banners</h1>
        </div>
        <button onClick={openCreate} className="btn btn-primary btn-sm">
          <PlusIcon className="w-4 h-4 mr-1" />
          New Banner
        </button>
      </div>

      {/* Table */}
      <div className="bg-white dark:bg-gray-800 rounded border border-gray-200 dark:border-gray-700 overflow-x-auto">
        <table className="min-w-full text-[12px]">
          <thead>
            <tr className="bg-gray-50 dark:bg-gray-700 border-b border-gray-200 dark:border-gray-600">
              <th className="px-3 py-2 text-left font-semibold text-gray-700 dark:text-gray-300">Title</th>
              <th className="px-3 py-2 text-left font-semibold text-gray-700 dark:text-gray-300">Type</th>
              <th className="px-3 py-2 text-left font-semibold text-gray-700 dark:text-gray-300">Target</th>
              {isAdmin() && <th className="px-3 py-2 text-left font-semibold text-gray-700 dark:text-gray-300">Created By</th>}
              <th className="px-3 py-2 text-left font-semibold text-gray-700 dark:text-gray-300">Start</th>
              <th className="px-3 py-2 text-left font-semibold text-gray-700 dark:text-gray-300">End</th>
              <th className="px-3 py-2 text-center font-semibold text-gray-700 dark:text-gray-300">Status</th>
              <th className="px-3 py-2 text-center font-semibold text-gray-700 dark:text-gray-300">Actions</th>
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <tr><td colSpan={isAdmin() ? 8 : 7} className="px-3 py-8 text-center text-gray-500 dark:text-gray-400">Loading...</td></tr>
            ) : banners.length === 0 ? (
              <tr><td colSpan={isAdmin() ? 8 : 7} className="px-3 py-8 text-center text-gray-500 dark:text-gray-400">No notification banners yet</td></tr>
            ) : banners.map(banner => {
              const typeOpt = typeOptions.find(t => t.value === banner.banner_type) || typeOptions[0]
              const active = isActive(banner)
              return (
                <tr key={banner.id} className="border-b border-gray-100 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50">
                  <td className="px-3 py-2 text-gray-900 dark:text-gray-100 font-medium max-w-[200px] truncate">{banner.title}</td>
                  <td className="px-3 py-2">
                    <span className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[11px] font-medium ${typeOpt.color}`}>
                      <typeOpt.icon className="w-3 h-3" />
                      {typeOpt.label}
                    </span>
                  </td>
                  <td className="px-3 py-2 text-gray-700 dark:text-gray-300 capitalize">{banner.target}</td>
                  {isAdmin() && (
                    <td className="px-3 py-2">
                      <span className={`inline-block px-1.5 py-0.5 rounded text-[11px] font-medium ${banner.reseller_id === 0 ? 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300' : 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/40 dark:text-cyan-300'}`}>
                        {banner.created_by_name || 'admin'}
                      </span>
                    </td>
                  )}
                  <td className="px-3 py-2 text-gray-600 dark:text-gray-400">{formatDate(banner.start_date)}</td>
                  <td className="px-3 py-2 text-gray-600 dark:text-gray-400">{formatDate(banner.end_date)}</td>
                  <td className="px-3 py-2 text-center">
                    {active ? (
                      <span className="inline-block px-1.5 py-0.5 rounded text-[11px] font-medium bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300">Active</span>
                    ) : banner.enabled ? (
                      <span className="inline-block px-1.5 py-0.5 rounded text-[11px] font-medium bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400">Scheduled</span>
                    ) : (
                      <span className="inline-block px-1.5 py-0.5 rounded text-[11px] font-medium bg-red-100 text-red-600 dark:bg-red-900/40 dark:text-red-400">Disabled</span>
                    )}
                  </td>
                  <td className="px-3 py-2 text-center">
                    <div className="flex items-center justify-center gap-1">
                      <button onClick={() => openEdit(banner)} className="p-1 hover:bg-gray-100 dark:hover:bg-gray-600 rounded" title="Edit">
                        <PencilSquareIcon className="w-3.5 h-3.5 text-blue-600 dark:text-blue-400" />
                      </button>
                      <button onClick={() => handleDelete(banner.id)} className="p-1 hover:bg-gray-100 dark:hover:bg-gray-600 rounded" title="Delete">
                        <TrashIcon className="w-3.5 h-3.5 text-red-600 dark:text-red-400" />
                      </button>
                    </div>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>

      {/* Create/Edit Modal */}
      {showModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-lg mx-4 max-h-[90vh] overflow-y-auto">
            <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-700">
              <h3 className="text-sm font-bold text-gray-900 dark:text-white">
                {editing ? 'Edit Banner' : 'Create Banner'}
              </h3>
              <button onClick={closeModal} className="text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200">
                &times;
              </button>
            </div>

            <form onSubmit={handleSubmit} className="p-4 space-y-3">
              {/* Title */}
              <div>
                <label className="block text-[12px] font-medium text-gray-700 dark:text-gray-300 mb-1">Title</label>
                <input
                  type="text"
                  value={form.title}
                  onChange={e => setForm({ ...form, title: e.target.value })}
                  className="input w-full"
                  required
                  placeholder="Banner title"
                />
              </div>

              {/* Message */}
              <div>
                <label className="block text-[12px] font-medium text-gray-700 dark:text-gray-300 mb-1">Message</label>
                <textarea
                  value={form.message}
                  onChange={e => setForm({ ...form, message: e.target.value })}
                  className="input w-full"
                  rows={3}
                  required
                  placeholder="Banner message text"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                {/* Type */}
                <div>
                  <label className="block text-[12px] font-medium text-gray-700 dark:text-gray-300 mb-1">Type</label>
                  <select
                    value={form.banner_type}
                    onChange={e => setForm({ ...form, banner_type: e.target.value })}
                    className="input w-full"
                  >
                    {typeOptions.map(t => (
                      <option key={t.value} value={t.value}>{t.label}</option>
                    ))}
                  </select>
                </div>

                {/* Target */}
                <div>
                  <label className="block text-[12px] font-medium text-gray-700 dark:text-gray-300 mb-1">Target</label>
                  <select
                      value={form.target}
                      onChange={e => setForm({ ...form, target: e.target.value, target_ids: '' })}
                      className="input w-full"
                    >
                      {(isAdmin() ? adminTargetOptions : resellerTargetOptions)
                        .filter(t => {
                          // Hide sub_resellers option if no sub-resellers exist
                          if (t.value === 'sub_resellers' && !isAdmin() && (!subResellers || subResellers.length === 0)) return false
                          return true
                        })
                        .map(t => (
                        <option key={t.value} value={t.value}>{t.label}</option>
                      ))}
                    </select>
                </div>
              </div>

              {/* Target Resellers (multi-select when admin targets resellers or subscribers) */}
              {isAdmin() && (form.target === 'resellers' || form.target === 'subscribers') && resellers && resellers.length > 0 && (
                <div>
                  <label className="block text-[12px] font-medium text-gray-700 dark:text-gray-300 mb-1">
                    {form.target === 'subscribers' ? 'Subscribers of Resellers' : 'Target Resellers'}{' '}
                    <span className="text-gray-400 font-normal">(leave empty for all)</span>
                  </label>
                  <div className="border border-gray-200 dark:border-gray-600 rounded max-h-32 overflow-y-auto p-1">
                    {resellers.map(r => (
                      <label key={r.id} className="flex items-center gap-2 px-2 py-1 hover:bg-gray-50 dark:hover:bg-gray-700 rounded cursor-pointer text-[12px]">
                        <input
                          type="checkbox"
                          checked={selectedResellerIds.includes(String(r.id))}
                          onChange={() => toggleReseller(r.id)}
                          className="accent-blue-600"
                        />
                        <span className="text-gray-800 dark:text-gray-200">{r.name || r.username}</span>
                      </label>
                    ))}
                  </div>
                </div>
              )}

              {/* Target Sub-Resellers (for both admin and reseller when target=sub_resellers) */}
              {form.target === 'sub_resellers' && (
                <div>
                  {isAdmin() && resellers && resellers.length > 0 ? (
                    <>
                      <label className="block text-[12px] font-medium text-gray-700 dark:text-gray-300 mb-1">
                        Target Resellers & Sub-Resellers <span className="text-gray-400 font-normal">(select which ones)</span>
                      </label>
                      <div className="border border-gray-200 dark:border-gray-600 rounded max-h-32 overflow-y-auto p-1">
                        {resellers.map(r => (
                          <label key={r.id} className="flex items-center gap-2 px-2 py-1 hover:bg-gray-50 dark:hover:bg-gray-700 rounded cursor-pointer text-[12px]">
                            <input
                              type="checkbox"
                              checked={selectedResellerIds.includes(String(r.id))}
                              onChange={() => toggleReseller(r.id)}
                              className="accent-blue-600"
                            />
                            <span className="text-gray-800 dark:text-gray-200">{r.name || r.username}</span>
                          </label>
                        ))}
                      </div>
                    </>
                  ) : subResellers && subResellers.length > 0 ? (
                    <>
                      <label className="block text-[12px] font-medium text-gray-700 dark:text-gray-300 mb-1">
                        Target Sub-Resellers <span className="text-gray-400 font-normal">(leave empty for all)</span>
                      </label>
                      <div className="border border-gray-200 dark:border-gray-600 rounded max-h-32 overflow-y-auto p-1">
                        {subResellers.map(r => (
                          <label key={r.id} className="flex items-center gap-2 px-2 py-1 hover:bg-gray-50 dark:hover:bg-gray-700 rounded cursor-pointer text-[12px]">
                            <input
                              type="checkbox"
                              checked={selectedResellerIds.includes(String(r.id))}
                              onChange={() => toggleReseller(r.id)}
                              className="accent-blue-600"
                            />
                            <span className="text-gray-800 dark:text-gray-200">{r.name || r.username}</span>
                          </label>
                        ))}
                      </div>
                    </>
                  ) : null}
                </div>
              )}

              <div className="grid grid-cols-2 gap-3">
                {/* Start Date */}
                <div>
                  <label className="block text-[12px] font-medium text-gray-700 dark:text-gray-300 mb-1">Start Date</label>
                  <input
                    type="datetime-local"
                    value={form.start_date}
                    onChange={e => setForm({ ...form, start_date: e.target.value })}
                    className="input w-full"
                    required
                  />
                </div>

                {/* End Date */}
                <div>
                  <label className="block text-[12px] font-medium text-gray-700 dark:text-gray-300 mb-1">End Date</label>
                  <input
                    type="datetime-local"
                    value={form.end_date}
                    onChange={e => setForm({ ...form, end_date: e.target.value })}
                    className="input w-full"
                    required
                  />
                </div>
              </div>

              <div className="flex items-center gap-6">
                {/* Dismissible */}
                <label className="flex items-center gap-2 cursor-pointer text-[12px] text-gray-700 dark:text-gray-300">
                  <input
                    type="checkbox"
                    checked={form.dismissible}
                    onChange={e => setForm({ ...form, dismissible: e.target.checked })}
                    className="accent-blue-600"
                  />
                  Dismissible
                </label>

                {/* Enabled */}
                <label className="flex items-center gap-2 cursor-pointer text-[12px] text-gray-700 dark:text-gray-300">
                  <input
                    type="checkbox"
                    checked={form.enabled}
                    onChange={e => setForm({ ...form, enabled: e.target.checked })}
                    className="accent-blue-600"
                  />
                  Enabled
                </label>
              </div>

              {/* Preview */}
              {form.title && (
                <div>
                  <label className="block text-[12px] font-medium text-gray-700 dark:text-gray-300 mb-1">Preview</label>
                  <div className={`${
                    form.banner_type === 'warning' ? 'bg-amber-500' :
                    form.banner_type === 'error' ? 'bg-red-600' :
                    form.banner_type === 'success' ? 'bg-green-600' : 'bg-blue-600'
                  } text-white text-[12px] px-3 py-1.5 rounded flex items-center gap-2`}>
                    <span className="font-semibold">{form.title}</span>
                    <span className="truncate">{form.message}</span>
                  </div>
                </div>
              )}

              {/* Buttons */}
              <div className="flex justify-end gap-2 pt-2 border-t border-gray-200 dark:border-gray-700">
                <button type="button" onClick={closeModal} className="btn btn-sm">Cancel</button>
                <button type="submit" className="btn btn-primary btn-sm" disabled={isSaving}>
                  {isSaving ? 'Saving...' : editing ? 'Update' : 'Create'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
