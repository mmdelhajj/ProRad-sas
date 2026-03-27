import { useState, useEffect } from 'react'
import { useNavigate, useSearchParams, Navigate } from 'react-router-dom'
import { useAuthStore } from '../store/authStore'
import { useBrandingStore } from '../store/brandingStore'
import toast from 'react-hot-toast'
import {
  UserIcon,
  LockClosedIcon,
  ShieldCheckIcon,
  WifiIcon,
  ChartBarIcon,
  CogIcon,
  ArrowLeftIcon
} from '@heroicons/react/24/outline'

const winStyles = {
  /* ── full-screen wrapper ── */
  page: {
    minHeight: '100vh',
    display: 'flex',
    fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif",
    fontSize: 11,
    margin: 0,
    padding: 0,
    background: '#c0c0c0',
  },

  /* ── left branding panel ── */
  leftPanel: (bg, primaryColor) => ({
    display: 'flex',
    flexDirection: 'column',
    justifyContent: 'space-between',
    position: 'relative',
    overflow: 'hidden',
    padding: '24px',
    ...(bg
      ? { backgroundImage: `url(${bg})`, backgroundSize: 'cover', backgroundPosition: 'center' }
      : { background: `linear-gradient(135deg, ${primaryColor || '#4a7ab5'} 0%, #2d5a87 100%)` }),
  }),

  leftOverlay: {
    position: 'absolute',
    inset: 0,
    background: 'rgba(0,0,0,0.45)',
  },

  /* ── feature rows on the left panel ── */
  featureIcon: {
    width: 36,
    height: 36,
    minWidth: 36,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    border: '1px solid rgba(255,255,255,0.35)',
    background: 'rgba(255,255,255,0.12)',
    borderRadius: '2px',
    marginRight: 10,
  },

  /* ── right side wrapper ── */
  rightPanel: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    padding: '24px',
    background: '#c0c0c0',
  },

  /* ── the dialog box ── */
  dialog: {
    width: '100%',
    maxWidth: 380,
    border: '2px solid',
    borderColor: '#dfdfdf #808080 #808080 #dfdfdf',
    background: '#c0c0c0',
    borderRadius: '0px',
  },

  /* ── title bar ── */
  titleBar: {
    background: 'linear-gradient(to right, #4a7ab5, #2d5a87)',
    padding: '6px 10px',
    display: 'flex',
    alignItems: 'center',
    gap: 8,
  },

  titleText: {
    color: '#fff',
    fontWeight: 600,
    fontSize: '12px',
    letterSpacing: '0.2px',
    flex: 1,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  },

  /* ── body area ── */
  body: {
    padding: '16px 18px 14px',
  },

  /* ── classic label ── */
  label: {
    display: 'block',
    fontSize: '11px',
    color: '#000',
    marginBottom: 3,
    fontWeight: 400,
  },

  /* ── classic input ── */
  input: {
    width: '100%',
    boxSizing: 'border-box',
    padding: '4px 6px',
    fontSize: '12px',
    fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif",
    border: '1px solid #a0a0a0',
    borderRadius: '1px',
    background: '#fff',
    color: '#000',
    outline: 'none',
  },

  inputFocused: {
    borderColor: '#4a7ab5',
  },

  /* ── primary (blue) button ── */
  btnPrimary: (disabled) => ({
    width: '100%',
    padding: '5px 12px',
    fontSize: '12px',
    fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif",
    fontWeight: 600,
    color: '#fff',
    background: disabled
      ? 'linear-gradient(to bottom, #8db4d6, #6c97b9)'
      : 'linear-gradient(to bottom, #5b8ec2, #3a6fa0)',
    border: '1px solid',
    borderColor: disabled ? '#8db4d6 #6c97b9 #6c97b9 #8db4d6' : '#4a7ab5 #2d5a87 #2d5a87 #4a7ab5',
    borderRadius: '1px',
    cursor: disabled ? 'not-allowed' : 'pointer',
    opacity: disabled ? 0.7 : 1,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 6,
    textShadow: '0 1px 1px rgba(0,0,0,0.25)',
  }),

  /* ── secondary (gray) button ── */
  btnSecondary: {
    width: '100%',
    padding: '4px 10px',
    fontSize: '11px',
    fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif",
    color: '#000',
    background: 'linear-gradient(to bottom, #fff, #e8e8e8)',
    border: '1px solid',
    borderColor: '#dfdfdf #808080 #808080 #dfdfdf',
    borderRadius: '1px',
    cursor: 'pointer',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 4,
  },

  /* ── sunken well (status / info) ── */
  infoWell: (type) => ({
    padding: '6px 8px',
    marginBottom: 10,
    fontSize: '11px',
    border: '1px solid',
    borderRadius: '1px',
    ...(type === 'warning'
      ? { borderColor: '#c0a000', background: '#fff8d0', color: '#665200' }
      : { borderColor: '#c00000', background: '#ffd8d8', color: '#600' }),
  }),

  /* ── horizontal separator ── */
  separator: {
    borderTop: '1px solid #808080',
    borderBottom: '1px solid #dfdfdf',
    margin: '10px 0',
  },

  /* ── footer text ── */
  footer: {
    textAlign: 'center',
    fontSize: '11px',
    color: '#555',
    marginTop: 10,
    padding: '0 18px 12px',
  },

  /* ── 2FA section ── */
  twoFAIcon: {
    width: 48,
    height: 48,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    border: '2px solid',
    borderColor: '#dfdfdf #808080 #808080 #dfdfdf',
    background: '#d8d8d8',
    borderRadius: '1px',
    margin: '0 auto 8px',
  },

  twoFAInput: {
    width: '100%',
    boxSizing: 'border-box',
    padding: '8px 6px',
    fontSize: '20px',
    fontFamily: "'Consolas', 'Courier New', monospace",
    textAlign: 'center',
    letterSpacing: '0.4em',
    border: '1px solid #a0a0a0',
    borderRadius: '1px',
    background: '#fff',
    color: '#000',
    outline: 'none',
  },
}

export default function Login() {
  const saved = JSON.parse(localStorage.getItem('rememberMe') || '{}')
  const [username, setUsername] = useState(saved.username || '')
  const [password, setPassword] = useState(saved.password || '')
  const [rememberMe, setRememberMe] = useState(!!saved.username)
  const [twoFACode, setTwoFACode] = useState('')
  const [requires2FA, setRequires2FA] = useState(false)
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const sessionReason = searchParams.get('reason')
  const { login, isAuthenticated, isCustomer } = useAuthStore()
  const {
    companyName, companyLogo, loginBackground, footerText, primaryColor,
    loginTagline, showLoginFeatures,
    loginFeature1Title, loginFeature1Desc,
    loginFeature2Title, loginFeature2Desc,
    loginFeature3Title, loginFeature3Desc,
    fetchBranding, loaded
  } = useBrandingStore()

  useEffect(() => {
    if (!loaded) {
      fetchBranding()
    }
  }, [loaded, fetchBranding])

  const handleSubmit = async (e) => {
    e.preventDefault()
    setLoading(true)

    const result = await login(username, password, twoFACode)

    if (result.success) {
      if (rememberMe) {
        localStorage.setItem('rememberMe', JSON.stringify({ username, password }))
      } else {
        localStorage.removeItem('rememberMe')
      }
      toast.success('Login successful')
      if (result.force_password_change) {
        toast('Please change your password to continue', { icon: '🔐' })
        navigate('/change-password')
      } else if (result.userType === 'customer') {
        navigate('/portal')
      } else {
        // Full reload ensures all queries start with token already in axios defaults
        window.location.href = '/'
      }
    } else if (result.requires_2fa) {
      setRequires2FA(true)
      toast('Please enter your 2FA code', { icon: '🔐' })
    } else {
      toast.error(result.message || 'Login failed')
    }

    setLoading(false)
  }

  const handleBack = () => {
    setRequires2FA(false)
    setTwoFACode('')
  }

  // If user navigated to /login manually while authenticated, log them out first
  // This allows switching accounts (e.g., admin -> reseller)
  useEffect(() => {
    if (isAuthenticated && !sessionReason) {
      const { logout } = useAuthStore.getState()
      logout()
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  /* ── Mobile-friendly login (clean, simple) ── */
  const mobileLogin = (
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
      {/* Logo / Company Name */}
      <div style={{ textAlign: 'center', marginBottom: 32 }}>
        {companyLogo ? (
          <img src={companyLogo} alt={companyName || 'Logo'} style={{ height: 56, objectFit: 'contain' }} />
        ) : (
          <>
            <div style={{
              width: 64, height: 64, margin: '0 auto 12px', display: 'flex', alignItems: 'center', justifyContent: 'center',
              background: 'rgba(255,255,255,0.15)', borderRadius: 16, border: '1px solid rgba(255,255,255,0.25)',
            }}>
              <WifiIcon style={{ width: 32, height: 32, color: '#fff' }} />
            </div>
            <div style={{ fontSize: 22, fontWeight: 700, color: '#fff', letterSpacing: '0.3px' }}>
              {companyName || 'ISP Management'}
            </div>
          </>
        )}
      </div>

      {/* Login Card */}
      <div style={{
        width: '100%', maxWidth: 340, background: '#fff', borderRadius: 12,
        padding: '28px 24px', boxShadow: '0 8px 32px rgba(0,0,0,0.2)',
      }}>
        {!requires2FA ? (
          <>
            {/* Session warnings */}
            {sessionReason === 'idle' && (
              <div style={{ padding: '8px 12px', marginBottom: 16, fontSize: 13, borderRadius: 8, background: '#fff8d0', color: '#665200', border: '1px solid #e8d44d' }}>
                Logged out due to inactivity.
              </div>
            )}
            {sessionReason === 'expired' && (
              <div style={{ padding: '8px 12px', marginBottom: 16, fontSize: 13, borderRadius: 8, background: '#ffd8d8', color: '#600', border: '1px solid #e88' }}>
                Session expired. Please sign in again.
              </div>
            )}

            <form onSubmit={handleSubmit}>
              {/* Username */}
              <div style={{ marginBottom: 16 }}>
                <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: '#333', marginBottom: 6 }}>Username</label>
                <div style={{ position: 'relative' }}>
                  <UserIcon style={{ position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)', width: 18, height: 18, color: '#999' }} />
                  <input
                    type="text"
                    required
                    value={username}
                    onChange={(e) => setUsername(e.target.value)}
                    placeholder="Enter your username"
                    autoComplete="username"
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

              {/* Password */}
              <div style={{ marginBottom: 24 }}>
                <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: '#333', marginBottom: 6 }}>Password</label>
                <div style={{ position: 'relative' }}>
                  <LockClosedIcon style={{ position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)', width: 18, height: 18, color: '#999' }} />
                  <input
                    type="password"
                    required
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder="Enter your password"
                    autoComplete="current-password"
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

              {/* Remember Me */}
              <div style={{ marginBottom: 20, display: 'flex', alignItems: 'center', gap: 8 }}>
                <input
                  type="checkbox"
                  id="rememberMe"
                  checked={rememberMe}
                  onChange={(e) => setRememberMe(e.target.checked)}
                  style={{ width: 16, height: 16, cursor: 'pointer', accentColor: primaryColor || '#4a7ab5' }}
                />
                <label htmlFor="rememberMe" style={{ fontSize: 13, color: '#555', cursor: 'pointer', userSelect: 'none' }}>
                  Remember me
                </label>
              </div>

              {/* Sign In button */}
              <button
                type="submit"
                disabled={loading}
                style={{
                  width: '100%', padding: '13px', fontSize: 16, fontWeight: 600, fontFamily: 'inherit',
                  color: '#fff', background: primaryColor || '#4a7ab5', border: 'none', borderRadius: 8,
                  cursor: loading ? 'not-allowed' : 'pointer', opacity: loading ? 0.7 : 1,
                  display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
                }}
              >
                {loading ? (
                  <>
                    <svg style={{ width: 18, height: 18, animation: 'spin 1s linear infinite' }} viewBox="0 0 24 24">
                      <circle style={{ opacity: 0.25 }} cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                      <path style={{ opacity: 0.75 }} fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                    </svg>
                    Signing in...
                  </>
                ) : (
                  'Sign In'
                )}
              </button>
            </form>
          </>
        ) : (
          <>
            {/* 2FA */}
            <div style={{ textAlign: 'center', marginBottom: 16 }}>
              <div style={{
                width: 52, height: 52, margin: '0 auto 10px', display: 'flex', alignItems: 'center', justifyContent: 'center',
                background: '#f0f4ff', borderRadius: 12,
              }}>
                <ShieldCheckIcon style={{ width: 28, height: 28, color: primaryColor || '#4a7ab5' }} />
              </div>
              <div style={{ fontWeight: 600, fontSize: 15, color: '#000', marginBottom: 4 }}>Verification Required</div>
              <div style={{ fontSize: 13, color: '#666' }}>Enter the 6-digit code from your authenticator app</div>
            </div>

            <form onSubmit={handleSubmit}>
              <div style={{ marginBottom: 16 }}>
                <input
                  type="text"
                  required
                  value={twoFACode}
                  onChange={(e) => setTwoFACode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                  placeholder="000000"
                  maxLength={6}
                  autoComplete="one-time-code"
                  autoFocus
                  style={{
                    width: '100%', boxSizing: 'border-box', padding: '14px', fontSize: 24,
                    fontFamily: "'Consolas', 'Courier New', monospace", textAlign: 'center', letterSpacing: '0.5em',
                    border: '1.5px solid #ddd', borderRadius: 8, background: '#f8f9fa', color: '#000', outline: 'none',
                  }}
                  onFocus={(e) => { e.target.style.borderColor = primaryColor || '#4a7ab5' }}
                  onBlur={(e) => { e.target.style.borderColor = '#ddd' }}
                />
              </div>

              <button
                type="submit"
                disabled={loading || twoFACode.length !== 6}
                style={{
                  width: '100%', padding: '13px', fontSize: 16, fontWeight: 600, fontFamily: 'inherit',
                  color: '#fff', background: primaryColor || '#4a7ab5', border: 'none', borderRadius: 8,
                  cursor: (loading || twoFACode.length !== 6) ? 'not-allowed' : 'pointer',
                  opacity: (loading || twoFACode.length !== 6) ? 0.7 : 1,
                  display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
                }}
              >
                {loading ? (
                  <>
                    <svg style={{ width: 18, height: 18, animation: 'spin 1s linear infinite' }} viewBox="0 0 24 24">
                      <circle style={{ opacity: 0.25 }} cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                      <path style={{ opacity: 0.75 }} fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                    </svg>
                    Verifying...
                  </>
                ) : (
                  'Verify & Sign In'
                )}
              </button>

              <button
                type="button"
                onClick={handleBack}
                style={{
                  width: '100%', marginTop: 10, padding: '10px', fontSize: 14, fontFamily: 'inherit',
                  color: '#666', background: '#f0f0f0', border: '1px solid #ddd', borderRadius: 8, cursor: 'pointer',
                  display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 6,
                }}
              >
                <ArrowLeftIcon style={{ width: 14, height: 14 }} />
                Back to login
              </button>
            </form>
          </>
        )}
      </div>

      {/* Footer */}
      <div style={{ marginTop: 20, fontSize: 12, color: 'rgba(255,255,255,0.6)', textAlign: 'center' }}>
        {footerText || (companyName ? `${companyName}` : 'ISP Management System')}
      </div>
    </div>
  )

  /* ── Desktop login (unchanged) ── */
  const desktopLogin = (
    <div style={{ ...winStyles.page, flexDirection: 'row' }}>
      {/* ─── Left Side: Branding & Features ─── */}
      <div
        className="lg:w-1/2"
        style={{ ...winStyles.leftPanel(loginBackground, primaryColor), display: 'flex' }}
      >
        {loginBackground && <div style={winStyles.leftOverlay} />}

        {/* Logo / Name */}
        <div style={{ position: 'relative', zIndex: 1 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            {companyLogo ? (
              <img src={companyLogo} alt={companyName || 'Logo'} style={{ height: 48, objectFit: 'contain' }} />
            ) : (
              <>
                <div style={{
                  width: 44, height: 44, display: 'flex', alignItems: 'center', justifyContent: 'center',
                  background: 'rgba(255,255,255,0.18)', border: '1px solid rgba(255,255,255,0.3)',
                  borderRadius: '2px',
                }}>
                  <WifiIcon style={{ width: 24, height: 24, color: '#fff' }} />
                </div>
                <div>
                  <div style={{ fontSize: 22, fontWeight: 700, color: '#fff' }}>
                    {companyName || 'ISP Management'}
                  </div>
                  <div style={{ fontSize: 11, color: 'rgba(255,255,255,0.7)' }}>
                    ISP Management System
                  </div>
                </div>
              </>
            )}
          </div>
        </div>

        {/* Features */}
        {showLoginFeatures && (
          <div style={{ position: 'relative', zIndex: 1, display: 'flex', flexDirection: 'column', gap: 20 }}>
            {[
              { Icon: WifiIcon, title: loginFeature1Title, desc: loginFeature1Desc },
              { Icon: ChartBarIcon, title: loginFeature2Title, desc: loginFeature2Desc },
              { Icon: CogIcon, title: loginFeature3Title, desc: loginFeature3Desc },
            ].map(({ Icon, title, desc }, i) => (
              <div key={i} style={{ display: 'flex', alignItems: 'flex-start' }}>
                <div style={winStyles.featureIcon}>
                  <Icon style={{ width: 18, height: 18, color: '#fff' }} />
                </div>
                <div>
                  <div style={{ color: '#fff', fontWeight: 600, fontSize: 13 }}>{title}</div>
                  <div style={{ color: 'rgba(255,255,255,0.65)', fontSize: 11 }}>{desc}</div>
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Tagline */}
        <div style={{ position: 'relative', zIndex: 1, color: 'rgba(255,255,255,0.65)', fontSize: 11 }}>
          {loginTagline}
        </div>
      </div>

      {/* ─── Right Side: Login Dialog ─── */}
      <div className="lg:w-1/2" style={winStyles.rightPanel}>
        <div style={{ width: '100%', maxWidth: 400 }}>

          {/* ── Dialog window ── */}
          <div style={winStyles.dialog}>

            {!requires2FA ? (
              <>
                {/* Title bar */}
                <div style={winStyles.titleBar}>
                  <LockClosedIcon style={{ width: 14, height: 14, color: '#fff' }} />
                  <span style={winStyles.titleText}>
                    {companyName ? `${companyName} - Sign In` : 'Sign In'}
                  </span>
                </div>

                {/* Body */}
                <div style={winStyles.body}>
                  {/* Session warnings */}
                  {sessionReason === 'idle' && (
                    <div style={winStyles.infoWell('warning')}>
                      You were logged out due to inactivity. Please sign in again.
                    </div>
                  )}
                  {sessionReason === 'expired' && (
                    <div style={winStyles.infoWell('error')}>
                      Your session has expired. Please sign in again.
                    </div>
                  )}

                  <form onSubmit={handleSubmit}>
                    {/* Username */}
                    <div style={{ marginBottom: 10 }}>
                      <label htmlFor="username-desktop" style={winStyles.label}>Username:</label>
                      <div style={{ position: 'relative' }}>
                        <UserIcon style={{
                          position: 'absolute', left: 5, top: '50%', transform: 'translateY(-50%)',
                          width: 14, height: 14, color: '#808080',
                        }} />
                        <input
                          id="username-desktop"
                          type="text"
                          required
                          value={username}
                          onChange={(e) => setUsername(e.target.value)}
                          style={{ ...winStyles.input, paddingLeft: 24 }}
                          onFocus={(e) => e.target.style.borderColor = '#4a7ab5'}
                          onBlur={(e) => e.target.style.borderColor = '#a0a0a0'}
                          placeholder="Enter your username"
                          autoComplete="username"
                        />
                      </div>
                    </div>

                    {/* Password */}
                    <div style={{ marginBottom: 14 }}>
                      <label htmlFor="password-desktop" style={winStyles.label}>Password:</label>
                      <div style={{ position: 'relative' }}>
                        <LockClosedIcon style={{
                          position: 'absolute', left: 5, top: '50%', transform: 'translateY(-50%)',
                          width: 14, height: 14, color: '#808080',
                        }} />
                        <input
                          id="password-desktop"
                          type="password"
                          required
                          value={password}
                          onChange={(e) => setPassword(e.target.value)}
                          style={{ ...winStyles.input, paddingLeft: 24 }}
                          onFocus={(e) => e.target.style.borderColor = '#4a7ab5'}
                          onBlur={(e) => e.target.style.borderColor = '#a0a0a0'}
                          placeholder="Enter your password"
                          autoComplete="current-password"
                        />
                      </div>
                    </div>

                    {/* Remember Me */}
                    <div style={{ marginBottom: 10, display: 'flex', alignItems: 'center', gap: 6 }}>
                      <input
                        type="checkbox"
                        id="rememberMe-desktop"
                        checked={rememberMe}
                        onChange={(e) => setRememberMe(e.target.checked)}
                        style={{ width: 14, height: 14, cursor: 'pointer', accentColor: '#4a7ab5' }}
                      />
                      <label htmlFor="rememberMe-desktop" style={{ fontSize: 11, color: '#555', cursor: 'pointer', userSelect: 'none' }}>
                        Remember me
                      </label>
                    </div>

                    <div style={winStyles.separator} />

                    {/* Sign In button */}
                    <button
                      type="submit"
                      disabled={loading}
                      style={winStyles.btnPrimary(loading)}
                    >
                      {loading ? (
                        <>
                          <svg style={{ width: 14, height: 14, animation: 'spin 1s linear infinite' }} viewBox="0 0 24 24">
                            <circle style={{ opacity: 0.25 }} cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                            <path style={{ opacity: 0.75 }} fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                          </svg>
                          Signing in...
                        </>
                      ) : (
                        'Sign In'
                      )}
                    </button>
                  </form>

                  <div style={{ textAlign: 'center', marginTop: 10, fontSize: 11, color: '#666' }}>
                    Admin, Reseller, or PPPoE Customer
                  </div>
                </div>
              </>
            ) : (
              <>
                {/* 2FA Title bar */}
                <div style={winStyles.titleBar}>
                  <ShieldCheckIcon style={{ width: 14, height: 14, color: '#fff' }} />
                  <span style={winStyles.titleText}>Two-Factor Authentication</span>
                </div>

                {/* 2FA Body */}
                <div style={winStyles.body}>
                  <div style={{ textAlign: 'center', marginBottom: 12 }}>
                    <div style={winStyles.twoFAIcon}>
                      <ShieldCheckIcon style={{ width: 24, height: 24, color: '#4a7ab5' }} />
                    </div>
                    <div style={{ fontWeight: 600, fontSize: 13, color: '#000', marginBottom: 2 }}>
                      Verification Required
                    </div>
                    <div style={{ fontSize: 11, color: '#555' }}>
                      Enter the 6-digit code from your authenticator app
                    </div>
                  </div>

                  <form onSubmit={handleSubmit}>
                    <div style={{ marginBottom: 10 }}>
                      <label htmlFor="twoFACode-desktop" style={winStyles.label}>Authentication Code:</label>
                      <input
                        id="twoFACode-desktop"
                        type="text"
                        required
                        value={twoFACode}
                        onChange={(e) => setTwoFACode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                        style={winStyles.twoFAInput}
                        onFocus={(e) => e.target.style.borderColor = '#4a7ab5'}
                        onBlur={(e) => e.target.style.borderColor = '#a0a0a0'}
                        placeholder="000000"
                        maxLength={6}
                        autoComplete="one-time-code"
                        autoFocus
                      />
                    </div>

                    <div style={winStyles.separator} />

                    {/* Verify button */}
                    <button
                      type="submit"
                      disabled={loading || twoFACode.length !== 6}
                      style={winStyles.btnPrimary(loading || twoFACode.length !== 6)}
                    >
                      {loading ? (
                        <>
                          <svg style={{ width: 14, height: 14, animation: 'spin 1s linear infinite' }} viewBox="0 0 24 24">
                            <circle style={{ opacity: 0.25 }} cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                            <path style={{ opacity: 0.75 }} fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                          </svg>
                          Verifying...
                        </>
                      ) : (
                        'Verify & Sign In'
                      )}
                    </button>

                    <div style={{ marginTop: 8 }}>
                      <button
                        type="button"
                        onClick={handleBack}
                        style={winStyles.btnSecondary}
                      >
                        <ArrowLeftIcon style={{ width: 12, height: 12 }} />
                        Back to login
                      </button>
                    </div>
                  </form>
                </div>
              </>
            )}
          </div>

          {/* Footer */}
          <div style={winStyles.footer}>
            {footerText || (companyName ? `${companyName} - ISP Management System` : 'ISP Management System')}
          </div>
        </div>
      </div>
    </div>
  )

  return (
    <>
      {/* Mobile: clean simple login */}
      <div className="lg:hidden">
        {mobileLogin}
      </div>
      {/* Desktop: original Windows-style login */}
      <div className="hidden lg:block">
        {desktopLogin}
      </div>
      {/* Keyframe for spinner */}
      <style>{`
        @keyframes spin {
          from { transform: rotate(0deg); }
          to { transform: rotate(360deg); }
        }
      `}</style>
    </>
  )
}
