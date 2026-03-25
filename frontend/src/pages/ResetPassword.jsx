import { useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { LockClosedIcon, ArrowLeftIcon, CheckCircleIcon } from '@heroicons/react/24/outline'
import api from '../services/api'
import { useBrandingStore } from '../store/brandingStore'

export default function ResetPassword() {
  const [searchParams] = useSearchParams()
  const token = searchParams.get('token') || ''
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [success, setSuccess] = useState(false)
  const [error, setError] = useState('')
  const { primaryColor } = useBrandingStore()

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError('')

    if (newPassword.length < 6) {
      setError('Password must be at least 6 characters.')
      return
    }
    if (newPassword !== confirmPassword) {
      setError('Passwords do not match.')
      return
    }

    setLoading(true)
    try {
      const res = await api.post('/auth/reset-password', { token, new_password: newPassword })
      if (res.data.success) {
        setSuccess(true)
      } else {
        setError(res.data.message || 'Failed to reset password.')
      }
    } catch (err) {
      setError(err.response?.data?.message || 'Something went wrong. The link may have expired.')
    } finally {
      setLoading(false)
    }
  }

  if (!token) {
    return (
      <div style={{
        minHeight: '100vh',
        background: `linear-gradient(135deg, ${primaryColor || '#4a7ab5'} 0%, #2d5a87 100%)`,
        display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 24,
        fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif",
      }}>
        <div style={{ width: '100%', maxWidth: 400, background: '#fff', borderRadius: 12, padding: '32px 28px', boxShadow: '0 8px 32px rgba(0,0,0,0.2)', textAlign: 'center' }}>
          <h2 style={{ fontSize: 20, fontWeight: 700, color: '#1a1a2e', margin: '0 0 8px' }}>Invalid Reset Link</h2>
          <p style={{ fontSize: 14, color: '#666', margin: '0 0 20px' }}>This password reset link is invalid or has expired.</p>
          <Link to="/forgot-password" style={{ fontSize: 14, color: primaryColor || '#4a7ab5', textDecoration: 'none', fontWeight: 600 }}>
            Request a new reset link
          </Link>
        </div>
      </div>
    )
  }

  return (
    <div style={{
      minHeight: '100vh',
      background: `linear-gradient(135deg, ${primaryColor || '#4a7ab5'} 0%, #2d5a87 100%)`,
      display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
      padding: '24px 20px',
      fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif",
    }}>
      <div style={{
        width: '100%', maxWidth: 400, background: '#fff', borderRadius: 12,
        padding: '32px 28px', boxShadow: '0 8px 32px rgba(0,0,0,0.2)',
      }}>
        {!success ? (
          <>
            <div style={{ textAlign: 'center', marginBottom: 24 }}>
              <div style={{
                width: 56, height: 56, margin: '0 auto 12px', display: 'flex', alignItems: 'center', justifyContent: 'center',
                background: '#f0f4ff', borderRadius: 14,
              }}>
                <LockClosedIcon style={{ width: 28, height: 28, color: primaryColor || '#4a7ab5' }} />
              </div>
              <h2 style={{ fontSize: 20, fontWeight: 700, color: '#1a1a2e', margin: '0 0 6px' }}>
                Reset Password
              </h2>
              <p style={{ fontSize: 14, color: '#666', margin: 0 }}>
                Enter your new password below.
              </p>
            </div>

            {error && (
              <div style={{ padding: '10px 14px', marginBottom: 16, fontSize: 13, borderRadius: 8, background: '#ffd8d8', color: '#600', border: '1px solid #e88' }}>
                {error}
              </div>
            )}

            <form onSubmit={handleSubmit}>
              <div style={{ marginBottom: 16 }}>
                <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: '#333', marginBottom: 6 }}>New Password</label>
                <div style={{ position: 'relative' }}>
                  <LockClosedIcon style={{ position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)', width: 18, height: 18, color: '#999' }} />
                  <input
                    type="password"
                    required
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                    placeholder="Enter new password"
                    minLength={6}
                    style={{
                      width: '100%', boxSizing: 'border-box', padding: '12px 12px 12px 40px',
                      fontSize: 15, fontFamily: 'inherit', border: '1.5px solid #ddd', borderRadius: 8,
                      background: '#f8f9fa', color: '#000', outline: 'none',
                    }}
                    onFocus={(e) => { e.target.style.borderColor = primaryColor || '#4a7ab5'; e.target.style.background = '#fff' }}
                    onBlur={(e) => { e.target.style.borderColor = '#ddd'; e.target.style.background = '#f8f9fa' }}
                  />
                </div>
              </div>

              <div style={{ marginBottom: 24 }}>
                <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: '#333', marginBottom: 6 }}>Confirm Password</label>
                <div style={{ position: 'relative' }}>
                  <LockClosedIcon style={{ position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)', width: 18, height: 18, color: '#999' }} />
                  <input
                    type="password"
                    required
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    placeholder="Confirm new password"
                    minLength={6}
                    style={{
                      width: '100%', boxSizing: 'border-box', padding: '12px 12px 12px 40px',
                      fontSize: 15, fontFamily: 'inherit', border: '1.5px solid #ddd', borderRadius: 8,
                      background: '#f8f9fa', color: '#000', outline: 'none',
                    }}
                    onFocus={(e) => { e.target.style.borderColor = primaryColor || '#4a7ab5'; e.target.style.background = '#fff' }}
                    onBlur={(e) => { e.target.style.borderColor = '#ddd'; e.target.style.background = '#f8f9fa' }}
                  />
                </div>
              </div>

              <button
                type="submit"
                disabled={loading}
                style={{
                  width: '100%', padding: '13px', fontSize: 15, fontWeight: 600, fontFamily: 'inherit',
                  color: '#fff', background: primaryColor || '#4a7ab5', border: 'none', borderRadius: 8,
                  cursor: loading ? 'not-allowed' : 'pointer', opacity: loading ? 0.7 : 1,
                }}
              >
                {loading ? 'Resetting...' : 'Reset Password'}
              </button>
            </form>
          </>
        ) : (
          <div style={{ textAlign: 'center' }}>
            <div style={{
              width: 56, height: 56, margin: '0 auto 16px', display: 'flex', alignItems: 'center', justifyContent: 'center',
              background: '#e8f5e9', borderRadius: 14,
            }}>
              <CheckCircleIcon style={{ width: 28, height: 28, color: '#2e7d32' }} />
            </div>
            <h2 style={{ fontSize: 20, fontWeight: 700, color: '#1a1a2e', margin: '0 0 8px' }}>
              Password Reset!
            </h2>
            <p style={{ fontSize: 14, color: '#666', margin: '0 0 20px', lineHeight: 1.5 }}>
              Your password has been reset successfully. You can now sign in with your new password.
            </p>
            <Link to="/login" style={{
              display: 'inline-block', padding: '12px 32px', fontSize: 15, fontWeight: 600,
              color: '#fff', background: primaryColor || '#4a7ab5', borderRadius: 8, textDecoration: 'none',
            }}>
              Sign In
            </Link>
          </div>
        )}
      </div>
    </div>
  )
}
