import { useState } from 'react'
import { useAuthStore } from '../store/authStore'
import api from '../services/api'
import toast from 'react-hot-toast'
import {
  UserCircleIcon,
  KeyIcon,
  EyeIcon,
  EyeSlashIcon,
  CheckCircleIcon,
} from '@heroicons/react/24/outline'

export default function Profile() {
  const { user, refreshUser, getTenantSubdomain } = useAuthStore()
  const tenantSubdomain = getTenantSubdomain()
  const [showPasswordForm, setShowPasswordForm] = useState(false)
  const [loading, setLoading] = useState(false)
  const [showCurrentPassword, setShowCurrentPassword] = useState(false)
  const [showNewPassword, setShowNewPassword] = useState(false)
  const [showConfirmPassword, setShowConfirmPassword] = useState(false)
  const [passwordForm, setPasswordForm] = useState({
    current_password: '',
    new_password: '',
    confirm_password: '',
  })

  const handlePasswordChange = async (e) => {
    e.preventDefault()

    if (passwordForm.new_password !== passwordForm.confirm_password) {
      toast.error('New passwords do not match')
      return
    }

    if (passwordForm.new_password.length < 6) {
      toast.error('Password must be at least 6 characters')
      return
    }

    setLoading(true)
    try {
      const endpoint = tenantSubdomain ? '/saas/tenant-change-password' : '/auth/change-password'
      const response = await api.post(endpoint, {
        current_password: passwordForm.current_password,
        new_password: passwordForm.new_password,
      })

      if (response.data.success) {
        toast.success('Password changed successfully')
        setPasswordForm({ current_password: '', new_password: '', confirm_password: '' })
        setShowPasswordForm(false)
        await refreshUser()
      } else {
        toast.error(response.data.message || 'Failed to change password')
      }
    } catch (error) {
      toast.error(error.response?.data?.message || 'Failed to change password')
    }
    setLoading(false)
  }

  return (
    <div className="space-y-3" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      <div className="wb-toolbar">
        <span className="text-[13px] font-semibold text-gray-800 dark:text-gray-100">My Profile</span>
        <span className="text-[11px] text-gray-500 dark:text-gray-400 ml-2">View and manage your account</span>
      </div>

      {/* Profile Info */}
      <div className="wb-group">
        <div className="wb-group-title flex items-center gap-2">
          <UserCircleIcon className="w-4 h-4 text-gray-600 dark:text-gray-400" />
          Account Information
        </div>
        <div className="wb-group-body">
          <div className="flex items-start gap-3">
            <UserCircleIcon className="w-10 h-10 text-[#316AC5] flex-shrink-0" />
            <div className="flex-1">
              <div className="text-[13px] font-semibold text-gray-900 dark:text-white">{user?.full_name || user?.username}</div>
              <div className="text-[12px] text-gray-500 dark:text-gray-400">@{user?.username}</div>
              <div className="mt-3 grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <div className="label">Email</div>
                  <div className="text-[12px] text-gray-900 dark:text-gray-100">{user?.email || '-'}</div>
                </div>
                <div>
                  <div className="label">Phone</div>
                  <div className="text-[12px] text-gray-900 dark:text-gray-100">{user?.phone || '-'}</div>
                </div>
                <div>
                  <div className="label">Account Type</div>
                  <div className="text-[12px] text-gray-900 dark:text-gray-100 capitalize">
                    {user?.user_type === 0 ? 'Administrator' : user?.user_type === 1 ? 'Staff' : 'Reseller'}
                  </div>
                </div>
                <div>
                  <div className="label">Status</div>
                  <span className={user?.is_active ? 'badge badge-success' : 'badge badge-danger'}>
                    {user?.is_active ? 'Active' : 'Inactive'}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Change Password */}
      <div className="wb-group">
        <div className="wb-group-title flex items-center justify-between">
          <div className="flex items-center gap-2">
            <KeyIcon className="w-4 h-4 text-gray-600 dark:text-gray-400" />
            Password
          </div>
          {!showPasswordForm && (
            <button
              onClick={() => setShowPasswordForm(true)}
              className="btn btn-primary btn-sm"
            >
              Change Password
            </button>
          )}
        </div>
        <div className="wb-group-body">
          {!showPasswordForm ? (
            <p className="text-[12px] text-gray-500 dark:text-gray-400">Click "Change Password" to update your password.</p>
          ) : (
            <form onSubmit={handlePasswordChange} className="space-y-3 max-w-sm">
              <div>
                <label className="label">Current Password</label>
                <div className="relative">
                  <input
                    type={showCurrentPassword ? 'text' : 'password'}
                    value={passwordForm.current_password}
                    onChange={(e) => setPasswordForm({ ...passwordForm, current_password: e.target.value })}
                    className="input pr-8"
                    placeholder="Enter current password"
                    required
                  />
                  <button
                    type="button"
                    onClick={() => setShowCurrentPassword(!showCurrentPassword)}
                    className="absolute right-1.5 top-1/2 -translate-y-1/2 p-0.5 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
                  >
                    {showCurrentPassword ? <EyeSlashIcon className="w-4 h-4" /> : <EyeIcon className="w-4 h-4" />}
                  </button>
                </div>
              </div>

              <div>
                <label className="label">New Password</label>
                <div className="relative">
                  <input
                    type={showNewPassword ? 'text' : 'password'}
                    value={passwordForm.new_password}
                    onChange={(e) => setPasswordForm({ ...passwordForm, new_password: e.target.value })}
                    className="input pr-8"
                    placeholder="Enter new password (min 6 characters)"
                    minLength={6}
                    required
                  />
                  <button
                    type="button"
                    onClick={() => setShowNewPassword(!showNewPassword)}
                    className="absolute right-1.5 top-1/2 -translate-y-1/2 p-0.5 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
                  >
                    {showNewPassword ? <EyeSlashIcon className="w-4 h-4" /> : <EyeIcon className="w-4 h-4" />}
                  </button>
                </div>
              </div>

              <div>
                <label className="label">Confirm New Password</label>
                <div className="relative">
                  <input
                    type={showConfirmPassword ? 'text' : 'password'}
                    value={passwordForm.confirm_password}
                    onChange={(e) => setPasswordForm({ ...passwordForm, confirm_password: e.target.value })}
                    className="input pr-8"
                    placeholder="Confirm new password"
                    minLength={6}
                    required
                  />
                  <button
                    type="button"
                    onClick={() => setShowConfirmPassword(!showConfirmPassword)}
                    className="absolute right-1.5 top-1/2 -translate-y-1/2 p-0.5 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
                  >
                    {showConfirmPassword ? <EyeSlashIcon className="w-4 h-4" /> : <EyeIcon className="w-4 h-4" />}
                  </button>
                </div>
              </div>

              <div className="flex gap-2 pt-1">
                <button
                  type="submit"
                  disabled={loading}
                  className="btn btn-primary flex items-center gap-1"
                >
                  {loading ? (
                    <>
                      <div className="animate-spin h-3 w-3 border-b-2 border-white" style={{ borderRadius: '50%' }}></div>
                      Saving...
                    </>
                  ) : (
                    <>
                      <CheckCircleIcon className="w-3.5 h-3.5" />
                      Save Password
                    </>
                  )}
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setShowPasswordForm(false)
                    setPasswordForm({ current_password: '', new_password: '', confirm_password: '' })
                  }}
                  className="btn"
                >
                  Cancel
                </button>
              </div>
            </form>
          )}
        </div>
      </div>
    </div>
  )
}
