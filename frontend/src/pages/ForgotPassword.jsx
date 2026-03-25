import { useState } from 'react'
import { Link } from 'react-router-dom'
import { EnvelopeIcon, ArrowLeftIcon } from '@heroicons/react/24/outline'
import api from '../services/api'
import { useBrandingStore } from '../store/brandingStore'

export default function ForgotPassword() {
  const [email, setEmail] = useState('')
  const [loading, setLoading] = useState(false)
  const [sent, setSent] = useState(false)
  const [error, setError] = useState('')
  const { primaryColor, companyName } = useBrandingStore()

  const handleSubmit = async (e) => {
    e.preventDefault()
    setLoading(true)
    setError('')

    try {
      await api.post('/auth/forgot-password', { email })
      setSent(true)
    } catch (err) {
      setError(err.response?.data?.message || 'Something went wrong. Please try again.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{
      minHeight: '100vh',
      background: `linear-gradient(135deg, ${primaryColor || '#4a7ab5'} 0%, #2d5a87 100%)`,
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      padding: '24px 20px',
      fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif",
    }}>
      <div style={{
        width: '100%', maxWidth: 400, background: '#fff', borderRadius: 12,
        padding: '32px 28px', boxShadow: '0 8px 32px rgba(0,0,0,0.2)',
      }}>
        {!sent ? (
          <>
            <div style={{ textAlign: 'center', marginBottom: 24 }}>
              <div style={{
                width: 56, height: 56, margin: '0 auto 12px', display: 'flex', alignItems: 'center', justifyContent: 'center',
                background: '#f0f4ff', borderRadius: 14,
              }}>
                <EnvelopeIcon style={{ width: 28, height: 28, color: primaryColor || '#4a7ab5' }} />
              </div>
              <h2 style={{ fontSize: 20, fontWeight: 700, color: '#1a1a2e', margin: '0 0 6px' }}>
                Forgot Password?
              </h2>
              <p style={{ fontSize: 14, color: '#666', margin: 0 }}>
                Enter your email address and we'll send you a link to reset your password.
              </p>
            </div>

            {error && (
              <div style={{ padding: '10px 14px', marginBottom: 16, fontSize: 13, borderRadius: 8, background: '#ffd8d8', color: '#600', border: '1px solid #e88' }}>
                {error}
              </div>
            )}

            <form onSubmit={handleSubmit}>
              <div style={{ marginBottom: 20 }}>
                <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: '#333', marginBottom: 6 }}>Email Address</label>
                <div style={{ position: 'relative' }}>
                  <EnvelopeIcon style={{ position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)', width: 18, height: 18, color: '#999' }} />
                  <input
                    type="email"
                    required
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="Enter your email"
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
                  display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
                }}
              >
                {loading ? 'Sending...' : 'Send Reset Link'}
              </button>
            </form>

            <div style={{ textAlign: 'center', marginTop: 16 }}>
              <Link to="/login" style={{ fontSize: 13, color: primaryColor || '#4a7ab5', textDecoration: 'none', display: 'inline-flex', alignItems: 'center', gap: 4 }}>
                <ArrowLeftIcon style={{ width: 14, height: 14 }} />
                Back to Sign In
              </Link>
            </div>
          </>
        ) : (
          <div style={{ textAlign: 'center' }}>
            <div style={{
              width: 56, height: 56, margin: '0 auto 16px', display: 'flex', alignItems: 'center', justifyContent: 'center',
              background: '#e8f5e9', borderRadius: 14,
            }}>
              <EnvelopeIcon style={{ width: 28, height: 28, color: '#2e7d32' }} />
            </div>
            <h2 style={{ fontSize: 20, fontWeight: 700, color: '#1a1a2e', margin: '0 0 8px' }}>
              Check Your Email
            </h2>
            <p style={{ fontSize: 14, color: '#666', margin: '0 0 20px', lineHeight: 1.5 }}>
              If an account with <strong>{email}</strong> exists, we've sent a password reset link. Please check your inbox.
            </p>
            <Link to="/login" style={{
              display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 14,
              color: primaryColor || '#4a7ab5', textDecoration: 'none', fontWeight: 600,
            }}>
              <ArrowLeftIcon style={{ width: 16, height: 16 }} />
              Back to Sign In
            </Link>
          </div>
        )}
      </div>
    </div>
  )
}
