import { useState, useEffect, useRef, useCallback } from 'react'
import { Link } from 'react-router-dom'
import axios from 'axios'
import {
  WifiIcon,
  ChartBarIcon,
  CogIcon,
  CheckCircleIcon,
  XCircleIcon,
  ClipboardDocumentIcon,
  ArrowRightIcon,
  ArrowLeftIcon,
  RocketLaunchIcon,
  ServerIcon,
  SignalIcon,
  ShieldCheckIcon,
  CommandLineIcon,
  CheckIcon,
  ArrowTopRightOnSquareIcon,
} from '@heroicons/react/24/outline'

const COLORS = {
  primary: '#4f46e5',
  primaryDark: '#3730a3',
  primaryLight: '#818cf8',
  bg: '#0f172a',
  bgCard: '#1e293b',
  bgInput: '#334155',
  border: '#475569',
  text: '#f8fafc',
  textMuted: '#94a3b8',
  success: '#22c55e',
  error: '#ef4444',
  warning: '#f59e0b',
}

// Spinner component
function Spinner({ size = 20, color = '#fff' }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" style={{ animation: 'spin 1s linear infinite' }}>
      <circle cx="12" cy="12" r="10" stroke={color} strokeWidth="3" fill="none" opacity={0.25} />
      <path fill={color} opacity={0.75} d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
    </svg>
  )
}

// ──────────────────────────────────────
// Landing Page
// ──────────────────────────────────────
function LandingView({ onGetStarted }) {
  const features = [
    {
      Icon: ServerIcon,
      title: 'ISP Management',
      desc: 'Complete subscriber management, billing, invoicing, and service plans in one platform.',
    },
    {
      Icon: WifiIcon,
      title: 'MikroTik Integration',
      desc: 'Auto-connect your MikroTik router via VPN. RADIUS, queues, and CoA work out of the box.',
    },
    {
      Icon: ChartBarIcon,
      title: 'Real-time Analytics',
      desc: 'Live bandwidth graphs, usage reports, FUP enforcement, and sharing detection.',
    },
  ]

  return (
    <div style={{ minHeight: '100vh', background: `linear-gradient(135deg, ${COLORS.bg} 0%, #1a1a2e 50%, ${COLORS.primaryDark} 100%)` }}>
      {/* Nav */}
      <nav style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '20px 32px', maxWidth: 1200, margin: '0 auto' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <div style={{ width: 36, height: 36, borderRadius: 8, background: COLORS.primary, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <WifiIcon style={{ width: 20, height: 20, color: '#fff' }} />
          </div>
          <span style={{ color: '#fff', fontWeight: 700, fontSize: 20 }}>ProxRad</span>
        </div>
        <div style={{ display: 'flex', gap: 16, alignItems: 'center' }}>
          <Link to="/login" style={{ color: COLORS.textMuted, textDecoration: 'none', fontSize: 14 }}>Login</Link>
          <Link to="/super-admin" style={{ color: COLORS.textMuted, textDecoration: 'none', fontSize: 12, opacity: 0.6 }}>Super Admin</Link>
        </div>
      </nav>

      {/* Hero */}
      <div style={{ textAlign: 'center', padding: '80px 24px 40px', maxWidth: 800, margin: '0 auto' }}>
        <div style={{ display: 'inline-block', padding: '6px 16px', borderRadius: 20, background: 'rgba(79,70,229,0.2)', border: '1px solid rgba(79,70,229,0.4)', color: COLORS.primaryLight, fontSize: 13, fontWeight: 500, marginBottom: 24 }}>
          14-day free trial &middot; No credit card required
        </div>
        <h1 style={{ color: '#fff', fontSize: 'clamp(32px, 5vw, 56px)', fontWeight: 800, lineHeight: 1.1, margin: '0 0 20px' }}>
          Launch Your ISP<br />in Minutes
        </h1>
        <p style={{ color: COLORS.textMuted, fontSize: 18, lineHeight: 1.6, maxWidth: 560, margin: '0 auto 40px' }}>
          Complete ISP management platform. Connect your MikroTik, start billing subscribers. Everything automated.
        </p>
        <button
          onClick={onGetStarted}
          style={{
            padding: '16px 40px', fontSize: 17, fontWeight: 600, color: '#fff',
            background: `linear-gradient(135deg, ${COLORS.primary}, ${COLORS.primaryDark})`,
            border: 'none', borderRadius: 12, cursor: 'pointer',
            display: 'inline-flex', alignItems: 'center', gap: 10,
            boxShadow: '0 4px 24px rgba(79,70,229,0.4)',
            transition: 'transform 0.2s, box-shadow 0.2s',
          }}
          onMouseOver={e => { e.currentTarget.style.transform = 'translateY(-2px)'; e.currentTarget.style.boxShadow = '0 8px 32px rgba(79,70,229,0.5)' }}
          onMouseOut={e => { e.currentTarget.style.transform = 'translateY(0)'; e.currentTarget.style.boxShadow = '0 4px 24px rgba(79,70,229,0.4)' }}
        >
          Get Started Free
          <ArrowRightIcon style={{ width: 20, height: 20 }} />
        </button>
      </div>

      {/* Features */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: 24, maxWidth: 1000, margin: '0 auto', padding: '40px 24px 80px' }}>
        {features.map(({ Icon, title, desc }, i) => (
          <div key={i} style={{
            background: 'rgba(30,41,59,0.6)', border: '1px solid rgba(71,85,105,0.4)', borderRadius: 16,
            padding: '32px 24px', backdropFilter: 'blur(8px)',
          }}>
            <div style={{
              width: 48, height: 48, borderRadius: 12, marginBottom: 16,
              background: `rgba(79,70,229,0.15)`, display: 'flex', alignItems: 'center', justifyContent: 'center',
            }}>
              <Icon style={{ width: 24, height: 24, color: COLORS.primaryLight }} />
            </div>
            <h3 style={{ color: '#fff', fontSize: 18, fontWeight: 600, margin: '0 0 8px' }}>{title}</h3>
            <p style={{ color: COLORS.textMuted, fontSize: 14, lineHeight: 1.6, margin: 0 }}>{desc}</p>
          </div>
        ))}
      </div>
    </div>
  )
}

// ──────────────────────────────────────
// Signup Form
// ──────────────────────────────────────
function SignupForm({ onSuccess, onBack }) {
  const [form, setForm] = useState({ name: '', subdomain: '', admin_username: 'admin', email: '', password: '', confirmPassword: '' })
  const [subdomainStatus, setSubdomainStatus] = useState(null) // null | 'checking' | 'available' | 'taken' | 'invalid'
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const debounceRef = useRef(null)

  const handleChange = (field, value) => {
    setForm(prev => ({ ...prev, [field]: value }))
    if (field === 'subdomain') {
      const clean = value.toLowerCase().replace(/[^a-z0-9-]/g, '')
      setForm(prev => ({ ...prev, subdomain: clean }))
      checkSubdomain(clean)
    }
  }

  const checkSubdomain = useCallback((name) => {
    if (debounceRef.current) clearTimeout(debounceRef.current)
    if (!name || name.length < 3) {
      setSubdomainStatus(name ? 'invalid' : null)
      return
    }
    setSubdomainStatus('checking')
    debounceRef.current = setTimeout(async () => {
      try {
        const res = await axios.get(`/api/saas/check-subdomain/${name}`)
        setSubdomainStatus(res.data.available ? 'available' : 'taken')
      } catch {
        setSubdomainStatus('invalid')
      }
    }, 500)
  }, [])

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError('')
    if (form.password !== form.confirmPassword) {
      setError('Passwords do not match')
      return
    }
    if (form.password.length < 6) {
      setError('Password must be at least 6 characters')
      return
    }
    if (subdomainStatus !== 'available') {
      setError('Please choose an available subdomain')
      return
    }
    setLoading(true)
    try {
      const res = await axios.post('/api/saas/signup', {
        name: form.name,
        subdomain: form.subdomain,
        admin_username: form.admin_username,
        email: form.email,
        password: form.password,
      })
      if (res.data.success) {
        onSuccess({ ...res.data.data, admin_username: form.admin_username, email: form.email, password: form.password })
      } else {
        setError(res.data.message || 'Signup failed')
      }
    } catch (err) {
      setError(err.response?.data?.message || 'Something went wrong. Please try again.')
    } finally {
      setLoading(false)
    }
  }

  const inputStyle = {
    width: '100%', boxSizing: 'border-box', padding: '12px 14px', fontSize: 15,
    background: COLORS.bgInput, color: COLORS.text, border: `1px solid ${COLORS.border}`,
    borderRadius: 8, outline: 'none', fontFamily: 'inherit',
  }

  const labelStyle = { display: 'block', color: COLORS.textMuted, fontSize: 13, fontWeight: 500, marginBottom: 6 }

  return (
    <div style={{ minHeight: '100vh', background: COLORS.bg, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 24 }}>
      <div style={{ width: '100%', maxWidth: 480 }}>
        {/* Header */}
        <div style={{ textAlign: 'center', marginBottom: 32 }}>
          <button
            onClick={onBack}
            style={{ background: 'none', border: 'none', color: COLORS.textMuted, cursor: 'pointer', display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 14, marginBottom: 20 }}
          >
            <ArrowLeftIcon style={{ width: 16, height: 16 }} /> Back
          </button>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 10, marginBottom: 8 }}>
            <div style={{ width: 36, height: 36, borderRadius: 8, background: COLORS.primary, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <WifiIcon style={{ width: 20, height: 20, color: '#fff' }} />
            </div>
            <span style={{ color: '#fff', fontWeight: 700, fontSize: 20 }}>ProxRad</span>
          </div>
          <h2 style={{ color: '#fff', fontSize: 24, fontWeight: 700, margin: '12px 0 4px' }}>Create your ISP account</h2>
          <p style={{ color: COLORS.textMuted, fontSize: 14, margin: 0 }}>Start your 14-day free trial</p>
        </div>

        {/* Form Card */}
        <div style={{ background: COLORS.bgCard, border: `1px solid ${COLORS.border}`, borderRadius: 16, padding: 32 }}>
          {error && (
            <div style={{ padding: '10px 14px', borderRadius: 8, background: 'rgba(239,68,68,0.15)', border: '1px solid rgba(239,68,68,0.3)', color: COLORS.error, fontSize: 13, marginBottom: 20 }}>
              {error}
            </div>
          )}

          <form onSubmit={handleSubmit}>
            {/* Company Name */}
            <div style={{ marginBottom: 18 }}>
              <label style={labelStyle}>Company Name</label>
              <input
                type="text" required value={form.name}
                onChange={e => handleChange('name', e.target.value)}
                placeholder="My ISP Company"
                style={inputStyle}
                onFocus={e => e.target.style.borderColor = COLORS.primary}
                onBlur={e => e.target.style.borderColor = COLORS.border}
              />
            </div>

            {/* Subdomain */}
            <div style={{ marginBottom: 18 }}>
              <label style={labelStyle}>Panel Subdomain</label>
              <div style={{ display: 'flex', alignItems: 'center', gap: 0 }}>
                <input
                  type="text" required value={form.subdomain}
                  onChange={e => handleChange('subdomain', e.target.value)}
                  placeholder="myisp"
                  style={{ ...inputStyle, borderRadius: '8px 0 0 8px', borderRight: 'none' }}
                  onFocus={e => e.target.style.borderColor = COLORS.primary}
                  onBlur={e => e.target.style.borderColor = COLORS.border}
                />
                <div style={{
                  padding: '12px 14px', fontSize: 14, color: COLORS.textMuted, whiteSpace: 'nowrap',
                  background: 'rgba(51,65,85,0.6)', border: `1px solid ${COLORS.border}`, borderRadius: '0 8px 8px 0',
                }}>
                  .saas.proxrad.com
                </div>
              </div>
              {/* Status badge */}
              <div style={{ marginTop: 6, minHeight: 20 }}>
                {subdomainStatus === 'checking' && (
                  <span style={{ fontSize: 12, color: COLORS.textMuted, display: 'flex', alignItems: 'center', gap: 4 }}>
                    <Spinner size={14} color={COLORS.textMuted} /> Checking...
                  </span>
                )}
                {subdomainStatus === 'available' && (
                  <span style={{ fontSize: 12, color: COLORS.success, display: 'flex', alignItems: 'center', gap: 4 }}>
                    <CheckCircleIcon style={{ width: 16, height: 16 }} /> Available
                  </span>
                )}
                {subdomainStatus === 'taken' && (
                  <span style={{ fontSize: 12, color: COLORS.error, display: 'flex', alignItems: 'center', gap: 4 }}>
                    <XCircleIcon style={{ width: 16, height: 16 }} /> Already taken
                  </span>
                )}
                {subdomainStatus === 'invalid' && (
                  <span style={{ fontSize: 12, color: COLORS.warning, display: 'flex', alignItems: 'center', gap: 4 }}>
                    Min 3 characters, lowercase letters, numbers, hyphens
                  </span>
                )}
              </div>
            </div>

            {/* Email */}
            <div style={{ marginBottom: 18 }}>
              <label style={labelStyle}>Email</label>
              <input
                type="email" required value={form.email}
                onChange={e => handleChange('email', e.target.value)}
                placeholder="you@company.com"
                style={inputStyle}
                onFocus={e => e.target.style.borderColor = COLORS.primary}
                onBlur={e => e.target.style.borderColor = COLORS.border}
              />
            </div>

            {/* Password */}
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12, marginBottom: 24 }}>
              <div>
                <label style={labelStyle}>Password</label>
                <input
                  type="password" required value={form.password}
                  onChange={e => handleChange('password', e.target.value)}
                  placeholder="Min 6 characters"
                  style={inputStyle}
                  onFocus={e => e.target.style.borderColor = COLORS.primary}
                  onBlur={e => e.target.style.borderColor = COLORS.border}
                />
              </div>
              <div>
                <label style={labelStyle}>Confirm Password</label>
                <input
                  type="password" required value={form.confirmPassword}
                  onChange={e => handleChange('confirmPassword', e.target.value)}
                  placeholder="Repeat password"
                  style={inputStyle}
                  onFocus={e => e.target.style.borderColor = COLORS.primary}
                  onBlur={e => e.target.style.borderColor = COLORS.border}
                />
              </div>
            </div>

            {/* Submit */}
            <button
              type="submit"
              disabled={loading || subdomainStatus !== 'available'}
              style={{
                width: '100%', padding: '14px', fontSize: 16, fontWeight: 600, color: '#fff',
                background: (loading || subdomainStatus !== 'available') ? COLORS.bgInput : COLORS.primary,
                border: 'none', borderRadius: 10, cursor: (loading || subdomainStatus !== 'available') ? 'not-allowed' : 'pointer',
                display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
                opacity: (loading || subdomainStatus !== 'available') ? 0.6 : 1,
              }}
            >
              {loading ? <><Spinner size={18} /> Creating account...</> : <>Create Account <RocketLaunchIcon style={{ width: 18, height: 18 }} /></>}
            </button>
          </form>
        </div>

        {/* Footer links */}
        <div style={{ textAlign: 'center', marginTop: 20, fontSize: 14, color: COLORS.textMuted }}>
          Already have an account?{' '}
          <Link to="/login" style={{ color: COLORS.primaryLight, textDecoration: 'none' }}>Login</Link>
        </div>
      </div>

      <style>{`@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }`}</style>
    </div>
  )
}

// ──────────────────────────────────────
// Setup Wizard
// ──────────────────────────────────────
function SetupWizard({ data }) {
  const [wizardStep, setWizardStep] = useState(1)
  const [copied, setCopied] = useState(false)
  const [connectionStatus, setConnectionStatus] = useState({ vpn_connected: false, mikrotik_reachable: false, radius_ready: false })
  const [polling, setPolling] = useState(false)
  const pollRef = useRef(null)

  const steps = [
    { num: 1, label: 'Welcome' },
    { num: 2, label: 'MikroTik' },
    { num: 3, label: 'Verify' },
    { num: 4, label: 'Done' },
  ]

  const copyScript = () => {
    navigator.clipboard.writeText(data.mikrotik_script || '').then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  // Poll for connection verification
  useEffect(() => {
    if (wizardStep === 3 && !polling) {
      setPolling(true)
      const poll = async () => {
        try {
          const res = await axios.post(`/api/saas/verify-connection/${data.tenant_id}`)
          if (res.data.success) {
            const s = res.data.data
            setConnectionStatus(s)
            if (s.vpn_connected) {
              setWizardStep(4)
              return
            }
          }
        } catch { /* ignore */ }
        pollRef.current = setTimeout(poll, 5000)
      }
      poll()
    }
    return () => { if (pollRef.current) clearTimeout(pollRef.current) }
  }, [wizardStep, data.tenant_id, polling])

  // Clean up polling when leaving step 3
  useEffect(() => {
    if (wizardStep !== 3) {
      setPolling(false)
      if (pollRef.current) clearTimeout(pollRef.current)
    }
  }, [wizardStep])

  const panelUrl = data.panel_url || `https://${data.subdomain}.saas.proxrad.com`

  const StatusDot = ({ active, label }) => (
    <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '14px 0' }}>
      {active ? (
        <CheckCircleIcon style={{ width: 24, height: 24, color: COLORS.success, flexShrink: 0 }} />
      ) : (
        <div style={{ width: 24, height: 24, flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <Spinner size={20} color={COLORS.primaryLight} />
        </div>
      )}
      <span style={{ color: active ? COLORS.success : COLORS.textMuted, fontSize: 15, fontWeight: active ? 600 : 400 }}>{label}</span>
    </div>
  )

  return (
    <div style={{ minHeight: '100vh', background: COLORS.bg, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 24 }}>
      <div style={{ width: '100%', maxWidth: 600 }}>
        {/* Progress Steps */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 0, marginBottom: 40 }}>
          {steps.map((s, i) => (
            <div key={s.num} style={{ display: 'flex', alignItems: 'center' }}>
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 6 }}>
                <div style={{
                  width: 36, height: 36, borderRadius: '50%', display: 'flex', alignItems: 'center', justifyContent: 'center',
                  fontSize: 14, fontWeight: 700,
                  background: wizardStep > s.num ? COLORS.success : wizardStep === s.num ? COLORS.primary : COLORS.bgInput,
                  color: '#fff',
                  border: wizardStep === s.num ? `2px solid ${COLORS.primaryLight}` : '2px solid transparent',
                }}>
                  {wizardStep > s.num ? <CheckIcon style={{ width: 18, height: 18 }} /> : s.num}
                </div>
                <span style={{ fontSize: 11, color: wizardStep >= s.num ? COLORS.text : COLORS.textMuted, fontWeight: wizardStep === s.num ? 600 : 400 }}>
                  {s.label}
                </span>
              </div>
              {i < steps.length - 1 && (
                <div style={{
                  width: 60, height: 2, margin: '0 8px', marginBottom: 20,
                  background: wizardStep > s.num ? COLORS.success : COLORS.bgInput,
                }} />
              )}
            </div>
          ))}
        </div>

        {/* Card */}
        <div style={{ background: COLORS.bgCard, border: `1px solid ${COLORS.border}`, borderRadius: 16, padding: 32 }}>

          {/* Step 1: Welcome */}
          {wizardStep === 1 && (
            <div style={{ textAlign: 'center' }}>
              <div style={{
                width: 64, height: 64, borderRadius: 16, margin: '0 auto 20px',
                background: 'rgba(34,197,94,0.15)', display: 'flex', alignItems: 'center', justifyContent: 'center',
              }}>
                <CheckCircleIcon style={{ width: 36, height: 36, color: COLORS.success }} />
              </div>
              <h2 style={{ color: '#fff', fontSize: 24, fontWeight: 700, margin: '0 0 8px' }}>Your ISP panel is ready!</h2>
              <p style={{ color: COLORS.textMuted, fontSize: 15, margin: '0 0 24px' }}>
                Your panel has been created and is ready to connect.
              </p>

              <div style={{ background: COLORS.bgInput, borderRadius: 12, padding: 20, textAlign: 'left', marginBottom: 24 }}>
                <div style={{ display: 'grid', gap: 12 }}>
                  <div>
                    <span style={{ color: COLORS.textMuted, fontSize: 12, textTransform: 'uppercase', letterSpacing: 1 }}>Panel URL</span>
                    <div style={{ marginTop: 4 }}>
                      <a href={panelUrl} target="_blank" rel="noopener noreferrer" style={{ color: COLORS.primaryLight, fontSize: 15, textDecoration: 'none', display: 'flex', alignItems: 'center', gap: 6 }}>
                        {panelUrl} <ArrowTopRightOnSquareIcon style={{ width: 14, height: 14 }} />
                      </a>
                    </div>
                  </div>
                  <div>
                    <span style={{ color: COLORS.textMuted, fontSize: 12, textTransform: 'uppercase', letterSpacing: 1 }}>Login Credentials</span>
                    <div style={{ color: COLORS.text, fontSize: 14, marginTop: 4 }}>
                      Email: <strong>{data.email}</strong>
                    </div>
                    <div style={{ color: COLORS.text, fontSize: 14, marginTop: 4 }}>
                      Password: <strong>{data.password}</strong>
                    </div>
                  </div>
                  <div style={{ padding: '10px 14px', borderRadius: 8, background: 'rgba(245,158,11,0.1)', border: '1px solid rgba(245,158,11,0.3)' }}>
                    <span style={{ color: COLORS.warning, fontSize: 13 }}>14-day free trial &middot; Up to 50 subscribers</span>
                  </div>
                </div>
              </div>

              <button
                onClick={() => setWizardStep(2)}
                style={{
                  padding: '14px 32px', fontSize: 16, fontWeight: 600, color: '#fff',
                  background: COLORS.primary, border: 'none', borderRadius: 10, cursor: 'pointer',
                  display: 'inline-flex', alignItems: 'center', gap: 8,
                }}
              >
                Next: Connect MikroTik <ArrowRightIcon style={{ width: 18, height: 18 }} />
              </button>
            </div>
          )}

          {/* Step 2: MikroTik Script */}
          {wizardStep === 2 && (
            <div>
              <div style={{ textAlign: 'center', marginBottom: 24 }}>
                <div style={{
                  width: 56, height: 56, borderRadius: 14, margin: '0 auto 16px',
                  background: 'rgba(79,70,229,0.15)', display: 'flex', alignItems: 'center', justifyContent: 'center',
                }}>
                  <CommandLineIcon style={{ width: 28, height: 28, color: COLORS.primaryLight }} />
                </div>
                <h2 style={{ color: '#fff', fontSize: 22, fontWeight: 700, margin: '0 0 8px' }}>Connect Your MikroTik</h2>
                <p style={{ color: COLORS.textMuted, fontSize: 14 }}>
                  Open your MikroTik terminal (WebFig or Winbox) and paste this script:
                </p>
              </div>

              {/* Script Block */}
              <div style={{ position: 'relative', marginBottom: 20 }}>
                <pre style={{
                  background: '#0d1117', border: '1px solid #30363d', borderRadius: 10, padding: 16,
                  color: '#c9d1d9', fontSize: 13, lineHeight: 1.6, overflow: 'auto', maxHeight: 300,
                  fontFamily: "'Consolas', 'Monaco', monospace', monospace",
                  whiteSpace: 'pre-wrap', wordBreak: 'break-all', margin: 0,
                }}>
                  {data.mikrotik_script || '# No script generated - WireGuard setup may have failed.\n# Please contact support.'}
                </pre>
                <button
                  onClick={copyScript}
                  style={{
                    position: 'absolute', top: 10, right: 10, padding: '6px 14px', fontSize: 13, fontWeight: 500,
                    color: copied ? COLORS.success : '#fff', background: copied ? 'rgba(34,197,94,0.15)' : 'rgba(79,70,229,0.8)',
                    border: 'none', borderRadius: 6, cursor: 'pointer',
                    display: 'flex', alignItems: 'center', gap: 6,
                  }}
                >
                  {copied ? <><CheckIcon style={{ width: 14, height: 14 }} /> Copied!</> : <><ClipboardDocumentIcon style={{ width: 14, height: 14 }} /> Copy</>}
                </button>
              </div>

              <div style={{ background: 'rgba(79,70,229,0.08)', border: '1px solid rgba(79,70,229,0.2)', borderRadius: 10, padding: 14, marginBottom: 24 }}>
                <p style={{ color: COLORS.textMuted, fontSize: 13, margin: 0, lineHeight: 1.6 }}>
                  <strong style={{ color: COLORS.text }}>Instructions:</strong><br />
                  1. Open MikroTik Winbox or WebFig<br />
                  2. Go to <strong style={{ color: COLORS.text }}>Terminal</strong><br />
                  3. Paste the script above and press Enter<br />
                  4. Wait for the VPN connection to establish
                </p>
              </div>

              <div style={{ display: 'flex', gap: 12 }}>
                <button
                  onClick={() => setWizardStep(1)}
                  style={{
                    padding: '12px 20px', fontSize: 14, fontWeight: 500, color: COLORS.textMuted,
                    background: COLORS.bgInput, border: `1px solid ${COLORS.border}`, borderRadius: 8, cursor: 'pointer',
                    display: 'flex', alignItems: 'center', gap: 6,
                  }}
                >
                  <ArrowLeftIcon style={{ width: 16, height: 16 }} /> Back
                </button>
                <button
                  onClick={() => setWizardStep(3)}
                  style={{
                    flex: 1, padding: '12px 20px', fontSize: 15, fontWeight: 600, color: '#fff',
                    background: COLORS.primary, border: 'none', borderRadius: 8, cursor: 'pointer',
                    display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
                  }}
                >
                  I've pasted the script <ArrowRightIcon style={{ width: 16, height: 16 }} />
                </button>
              </div>
            </div>
          )}

          {/* Step 3: Verify Connection */}
          {wizardStep === 3 && (
            <div>
              <div style={{ textAlign: 'center', marginBottom: 24 }}>
                <div style={{
                  width: 56, height: 56, borderRadius: 14, margin: '0 auto 16px',
                  background: 'rgba(79,70,229,0.15)', display: 'flex', alignItems: 'center', justifyContent: 'center',
                }}>
                  <SignalIcon style={{ width: 28, height: 28, color: COLORS.primaryLight }} />
                </div>
                <h2 style={{ color: '#fff', fontSize: 22, fontWeight: 700, margin: '0 0 8px' }}>Verifying Connection</h2>
                <p style={{ color: COLORS.textMuted, fontSize: 14 }}>
                  Checking your MikroTik VPN connection...
                </p>
              </div>

              <div style={{ background: COLORS.bgInput, borderRadius: 12, padding: 20, marginBottom: 24 }}>
                <StatusDot active={connectionStatus.vpn_connected} label="VPN Tunnel Connected" />
                <div style={{ borderTop: `1px solid ${COLORS.border}` }} />
                <StatusDot active={connectionStatus.mikrotik_reachable} label="MikroTik Router Reachable" />
                <div style={{ borderTop: `1px solid ${COLORS.border}` }} />
                <StatusDot active={connectionStatus.radius_ready} label="RADIUS Service Ready" />
              </div>

              <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
                <button
                  onClick={() => setWizardStep(2)}
                  style={{
                    padding: '12px 20px', fontSize: 14, fontWeight: 500, color: COLORS.textMuted,
                    background: COLORS.bgInput, border: `1px solid ${COLORS.border}`, borderRadius: 8, cursor: 'pointer',
                    display: 'flex', alignItems: 'center', gap: 6,
                  }}
                >
                  <ArrowLeftIcon style={{ width: 16, height: 16 }} /> Back
                </button>
                <div style={{ flex: 1 }} />
                <button
                  onClick={() => setWizardStep(4)}
                  style={{
                    padding: '10px 16px', fontSize: 13, color: COLORS.textMuted,
                    background: 'none', border: 'none', cursor: 'pointer', textDecoration: 'underline',
                  }}
                >
                  Skip this step
                </button>
              </div>
            </div>
          )}

          {/* Step 4: All Done */}
          {wizardStep === 4 && (
            <div style={{ textAlign: 'center' }}>
              <div style={{
                width: 72, height: 72, borderRadius: 18, margin: '0 auto 20px',
                background: 'rgba(34,197,94,0.15)', display: 'flex', alignItems: 'center', justifyContent: 'center',
              }}>
                <ShieldCheckIcon style={{ width: 40, height: 40, color: COLORS.success }} />
              </div>
              <h2 style={{ color: '#fff', fontSize: 26, fontWeight: 700, margin: '0 0 8px' }}>You're all set!</h2>
              <p style={{ color: COLORS.textMuted, fontSize: 15, margin: '0 0 32px', lineHeight: 1.6 }}>
                Your ISP panel is ready. Log in with your credentials to start managing subscribers.
              </p>

              <a
                href={panelUrl}
                target="_blank"
                rel="noopener noreferrer"
                style={{
                  display: 'inline-flex', alignItems: 'center', gap: 10,
                  padding: '16px 40px', fontSize: 17, fontWeight: 600, color: '#fff',
                  background: `linear-gradient(135deg, ${COLORS.success}, #16a34a)`,
                  border: 'none', borderRadius: 12, cursor: 'pointer', textDecoration: 'none',
                  boxShadow: '0 4px 20px rgba(34,197,94,0.3)',
                }}
              >
                Go to My Panel <ArrowTopRightOnSquareIcon style={{ width: 18, height: 18 }} />
              </a>

              <div style={{ marginTop: 24 }}>
                <Link to="/super-admin" style={{ color: COLORS.textMuted, fontSize: 13, textDecoration: 'none' }}>
                  Back to Super Admin
                </Link>
              </div>
            </div>
          )}
        </div>
      </div>

      <style>{`@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }`}</style>
    </div>
  )
}

// ──────────────────────────────────────
// Main Signup Component
// ──────────────────────────────────────
export default function Signup() {
  const [step, setStep] = useState('landing') // 'landing' | 'form' | 'wizard'
  const [signupData, setSignupData] = useState(null)

  const handleSignupSuccess = (data) => {
    setSignupData(data)
    setStep('wizard')
  }

  if (step === 'form') {
    return <SignupForm onSuccess={handleSignupSuccess} onBack={() => setStep('landing')} />
  }

  if (step === 'wizard' && signupData) {
    return <SetupWizard data={signupData} />
  }

  return <LandingView onGetStarted={() => setStep('form')} />
}
