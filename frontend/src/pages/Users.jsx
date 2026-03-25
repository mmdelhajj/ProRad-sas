import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import api, { permissionApi } from '../services/api'
import { formatDateTime } from '../utils/timezone'
import { EyeIcon, EyeSlashIcon } from '@heroicons/react/24/outline'

const USER_TYPES = {
  'reseller': 'Reseller',
  'support': 'Support',
  'admin': 'Admin',
  'collector': 'Collector',
  'readonly': 'Read Only'
}

// Map for select dropdown (string value -> display label)
const USER_TYPE_OPTIONS = [
  { value: 'reseller', label: 'Reseller' },
  { value: 'support', label: 'Support' },
  { value: 'admin', label: 'Admin' },
  { value: 'collector', label: 'Collector' },
  { value: 'readonly', label: 'Read Only' }
]

export default function Users() {
  const queryClient = useQueryClient()
  const [showModal, setShowModal] = useState(false)
  const [editUser, setEditUser] = useState(null)
  const [showPassword, setShowPassword] = useState(false)
  const [formData, setFormData] = useState({
    username: '',
    password: '',
    email: '',
    phone: '',
    full_name: '',
    user_type: 'support',
    is_active: true,
    permission_group: ''
  })

  const { data, isLoading } = useQuery({
    queryKey: ['users'],
    queryFn: () => api.get('/users').then(res => res.data.data)
  })

  const { data: permissionGroups } = useQuery({
    queryKey: ['permissionGroups'],
    queryFn: () => permissionApi.listGroups().then(r => r.data.data || [])
  })

  const createMutation = useMutation({
    mutationFn: (data) => api.post('/users', data),
    onSuccess: () => {
      queryClient.invalidateQueries(['users'])
      setShowModal(false)
      resetForm()
    }
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }) => api.put(`/users/${id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries(['users'])
      setShowModal(false)
      resetForm()
    }
  })

  const deleteMutation = useMutation({
    mutationFn: (id) => api.delete(`/users/${id}`),
    onSuccess: () => queryClient.invalidateQueries(['users'])
  })

  const resetForm = () => {
    setEditUser(null)
    setShowPassword(false)
    setFormData({
      username: '',
      password: '',
      email: '',
      phone: '',
      full_name: '',
      user_type: 'support',
      is_active: true,
      permission_group: ''
    })
  }

  const handleEdit = (user) => {
    setEditUser(user)
    setFormData({
      username: user.username,
      password: '',
      email: user.email,
      phone: user.phone,
      full_name: user.full_name,
      user_type: user.user_type,
      is_active: user.is_active,
      permission_group: user.permission_group || ''
    })
    setShowModal(true)
  }

  const handleSubmit = (e) => {
    e.preventDefault()
    // Clean up data - convert empty string to null for permission_group
    const submitData = {
      ...formData,
      permission_group: formData.permission_group === '' ? null : formData.permission_group
    }
    if (editUser) {
      updateMutation.mutate({ id: editUser.id, data: submitData })
    } else {
      createMutation.mutate(submitData)
    }
  }

  const handleDelete = (user) => {
    if (confirm(`Delete user ${user.username}?`)) {
      deleteMutation.mutate(user.id)
    }
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin h-5 w-5 border-2 border-[#316AC5] border-t-transparent" style={{ borderRadius: '50%' }}></div>
      </div>
    )
  }

  return (
    <div className="space-y-2" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Toolbar */}
      <div className="wb-toolbar justify-between">
        <span className="text-[13px] font-semibold">Admin Users</span>
        <button
          onClick={() => { resetForm(); setShowModal(true) }}
          className="btn btn-primary"
        >
          Add User
        </button>
      </div>

      {/* Table */}
      <div className="table-container">
        <table className="table">
          <thead>
            <tr>
              <th>Username</th>
              <th>Full Name</th>
              <th>Email</th>
              <th>Type</th>
              <th>Status</th>
              <th>Last Login</th>
              <th style={{ textAlign: 'right' }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {(data || []).map(user => (
              <tr key={user.id}>
                <td className="font-semibold">{user.username}</td>
                <td>{user.full_name}</td>
                <td>{user.email}</td>
                <td>{USER_TYPES[user.user_type] || 'Unknown'}</td>
                <td>
                  <span className={user.is_active ? 'badge-success' : 'badge-danger'}>
                    {user.is_active ? 'Active' : 'Inactive'}
                  </span>
                </td>
                <td>{user.last_login ? formatDateTime(user.last_login) : 'Never'}</td>
                <td style={{ textAlign: 'right' }}>
                  <button onClick={() => handleEdit(user)} className="btn btn-sm mr-1">
                    Edit
                  </button>
                  <button onClick={() => handleDelete(user)} className="btn btn-danger btn-sm">
                    Delete
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Modal */}
      {showModal && (
        <div className="modal-overlay">
          <div className="modal" style={{ width: 420 }}>
            <div className="modal-header">
              <span>{editUser ? 'Edit User' : 'Add User'}</span>
              <button onClick={() => { setShowModal(false); resetForm() }} className="text-white hover:text-gray-200 text-[16px] leading-none">&times;</button>
            </div>
            <form onSubmit={handleSubmit}>
              <div className="modal-body space-y-3">
                {!editUser && (
                  <div>
                    <label className="label">Username</label>
                    <input
                      type="text"
                      value={formData.username}
                      onChange={(e) => setFormData({ ...formData, username: e.target.value })}
                      className="input"
                      required
                    />
                  </div>
                )}
                <div>
                  <label className="label">
                    Password {editUser && '(leave blank to keep current)'}
                  </label>
                  <div className="relative">
                    <input
                      type={showPassword ? 'text' : 'password'}
                      value={formData.password}
                      onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                      className="input pr-8"
                      required={!editUser}
                      placeholder={editUser ? 'Enter new password' : 'Enter password'}
                    />
                    <button
                      type="button"
                      onClick={() => setShowPassword(!showPassword)}
                      className="absolute inset-y-0 right-0 pr-2 flex items-center text-gray-400 hover:text-gray-600"
                    >
                      {showPassword ? (
                        <EyeSlashIcon className="h-4 w-4" />
                      ) : (
                        <EyeIcon className="h-4 w-4" />
                      )}
                    </button>
                  </div>
                </div>
                <div>
                  <label className="label">Full Name</label>
                  <input
                    type="text"
                    value={formData.full_name}
                    onChange={(e) => setFormData({ ...formData, full_name: e.target.value })}
                    className="input"
                  />
                </div>
                <div>
                  <label className="label">Email</label>
                  <input
                    type="email"
                    value={formData.email}
                    onChange={(e) => setFormData({ ...formData, email: e.target.value })}
                    className="input"
                  />
                </div>
                <div>
                  <label className="label">Phone</label>
                  <input
                    type="text"
                    value={formData.phone}
                    onChange={(e) => setFormData({ ...formData, phone: e.target.value })}
                    className="input"
                  />
                </div>
                <div>
                  <label className="label">User Type</label>
                  <select
                    value={formData.user_type}
                    onChange={(e) => setFormData({ ...formData, user_type: e.target.value })}
                    className="input"
                  >
                    {USER_TYPE_OPTIONS.map((option) => (
                      <option key={option.value} value={option.value}>{option.label}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="label">Permission Group</label>
                  <select
                    value={formData.permission_group || ''}
                    onChange={(e) => setFormData({ ...formData, permission_group: e.target.value ? parseInt(e.target.value) : null })}
                    className="input"
                  >
                    <option value="">All Permissions (No Restriction)</option>
                    {(permissionGroups || []).map((group) => (
                      <option key={group.id} value={group.id}>{group.name}</option>
                    ))}
                  </select>
                  <p className="mt-0.5 text-[11px] text-gray-500">Leave empty to grant all permissions</p>
                </div>
                <div className="flex items-center">
                  <input
                    type="checkbox"
                    id="is_active"
                    checked={formData.is_active}
                    onChange={(e) => setFormData({ ...formData, is_active: e.target.checked })}
                    className="mr-1"
                  />
                  <label htmlFor="is_active" className="text-[12px]">Active</label>
                </div>
              </div>
              <div className="modal-footer">
                <button
                  type="button"
                  onClick={() => { setShowModal(false); resetForm() }}
                  className="btn"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={createMutation.isPending || updateMutation.isPending}
                  className="btn btn-primary"
                >
                  {editUser ? 'Update' : 'Create'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
