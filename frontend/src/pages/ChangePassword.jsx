import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store/authStore'
import api from '../services/api'
import toast from 'react-hot-toast'

export default function ChangePassword() {
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()
  const { user, refreshUser, getTenantSubdomain } = useAuthStore()

  // Check if this is forced password change (first login)
  const isForcedChange = user?.force_password_change === true
  const tenantSubdomain = getTenantSubdomain()

  const handleSubmit = async (e) => {
    e.preventDefault()

    if (newPassword !== confirmPassword) {
      toast.error('Passwords do not match')
      return
    }

    if (newPassword.length < 8) {
      toast.error('Password must be at least 8 characters')
      return
    }

    setLoading(true)
    try {
      const endpoint = tenantSubdomain ? '/saas/tenant-change-password' : '/auth/change-password'
      const response = await api.post(endpoint, {
        current_password: isForcedChange ? 'admin123' : currentPassword,
        new_password: newPassword
      })

      if (response.data.success) {
        toast.success('Password changed successfully')
        await refreshUser()
        navigate('/')
      } else {
        toast.error(response.data.message || 'Failed to change password')
      }
    } catch (error) {
      toast.error(error.response?.data?.message || 'Failed to change password')
    }
    setLoading(false)
  }

  return (
    <div className="min-h-screen bg-[#c0c0c0] flex items-center justify-center p-2" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      <div className="w-full max-w-md">
        <div className="card">
          {/* Header */}
          <div className="modal-header">
            <span>Change Password</span>
          </div>

          <div className="p-2 bg-[#f0f0f0]">
            {/* Warning message */}
            {isForcedChange && (
              <div className="flex items-center gap-2 mb-2 p-2 border border-[#FF9800] bg-[#fff8e1] text-[11px]" style={{ borderRadius: '2px' }}>
                <span className="text-gray-800">
                  For security reasons, you must change your password before continuing.
                </span>
              </div>
            )}

            <form onSubmit={handleSubmit} className="space-y-3">
              {/* Only show current password field if NOT forced change */}
              {!isForcedChange && (
                <div>
                  <label htmlFor="currentPassword" className="label">
                    Current Password
                  </label>
                  <input
                    id="currentPassword"
                    type="password"
                    required
                    value={currentPassword}
                    onChange={(e) => setCurrentPassword(e.target.value)}
                    className="input"
                    placeholder="Enter current password"
                  />
                </div>
              )}

              <div>
                <label htmlFor="newPassword" className="label">
                  New Password
                </label>
                <input
                  id="newPassword"
                  type="password"
                  required
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  className="input"
                  placeholder="Enter new password (min 8 characters)"
                  minLength={8}
                />
              </div>

              <div>
                <label htmlFor="confirmPassword" className="label">
                  Confirm New Password
                </label>
                <input
                  id="confirmPassword"
                  type="password"
                  required
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  className="input"
                  placeholder="Confirm new password"
                  minLength={8}
                />
              </div>

              <div className="pt-2 border-t border-[#a0a0a0] flex justify-end">
                <button
                  type="submit"
                  disabled={loading}
                  className="btn btn-primary"
                >
                  {loading ? 'Changing Password...' : 'Change Password & Continue'}
                </button>
              </div>
            </form>
          </div>
        </div>
      </div>
    </div>
  )
}
