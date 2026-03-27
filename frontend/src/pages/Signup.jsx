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
      desc: 'Complete subscriber lifecycle — create, renew, suspend. PPPoE auth, MAC binding, IP management.',
    },
    {
      Icon: WifiIcon,
      title: 'MikroTik & RADIUS',
      desc: 'Native MikroTik integration. FreeRADIUS with MS-CHAPv2. CoA for instant speed changes.',
    },
    {
      Icon: ChartBarIcon,
      title: 'Real-time Analytics',
      desc: 'Live bandwidth via Torch, FUP enforcement, CDN management, usage reports and graphs.',
    },
    {
      Icon: ShieldCheckIcon,
      title: 'Billing & Resellers',
      desc: 'Automated invoicing, prepaid cards, multi-tier reseller hierarchy with balance management.',
    },
  ]

  const stats = [
    { value: '30,000+', label: 'Subscribers Managed' },
    { value: '99.9%', label: 'Uptime' },
    { value: '50+', label: 'ISPs Trust Us' },
  ]

  return (
    <div style={{ minHeight: '100vh', background: COLORS.bg }}>
      {/* Nav */}
      <nav style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '20px 32px', maxWidth: 1200, margin: '0 auto' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <div style={{ width: 36, height: 36, borderRadius: 8, background: COLORS.primary, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <WifiIcon style={{ width: 20, height: 20, color: '#fff' }} />
          </div>
          <span style={{ color: '#fff', fontWeight: 700, fontSize: 20 }}>ProxRad</span>
          <span style={{ color: COLORS.primaryLight, fontSize: 11, fontWeight: 600, padding: '2px 8px', borderRadius: 4, background: 'rgba(79,70,229,0.2)', marginLeft: -4 }}>Cloud</span>
        </div>
        <div style={{ display: 'flex', gap: 16, alignItems: 'center' }}>
          <Link to="/login" style={{ color: COLORS.textMuted, textDecoration: 'none', fontSize: 14, fontWeight: 500, padding: '8px 16px', borderRadius: 8, border: `1px solid ${COLORS.border}`, transition: 'all 0.2s' }}>Login</Link>
          <button
            onClick={onGetStarted}
            style={{
              padding: '8px 20px', fontSize: 14, fontWeight: 600, color: '#fff',
              background: COLORS.primary, border: 'none', borderRadius: 8, cursor: 'pointer',
            }}
          >
            Start Free Trial
          </button>
        </div>
      </nav>

      {/* Hero */}
      <div style={{ textAlign: 'center', padding: '80px 24px 40px', maxWidth: 800, margin: '0 auto' }}>
        <div style={{ display: 'inline-block', padding: '6px 16px', borderRadius: 20, background: 'rgba(79,70,229,0.2)', border: '1px solid rgba(79,70,229,0.4)', color: COLORS.primaryLight, fontSize: 13, fontWeight: 500, marginBottom: 24 }}>
          The Complete ISP Management Platform
        </div>
        <h1 style={{ color: '#fff', fontSize: 'clamp(32px, 5vw, 56px)', fontWeight: 800, lineHeight: 1.1, margin: '0 0 20px' }}>
          Manage Your ISP<br />
          <span style={{ background: 'linear-gradient(135deg, #818cf8, #06b6d4)', WebkitBackgroundClip: 'text', WebkitTextFillColor: 'transparent' }}>
            From One Dashboard
          </span>
        </h1>
        <p style={{ color: COLORS.textMuted, fontSize: 18, lineHeight: 1.6, maxWidth: 580, margin: '0 auto 40px' }}>
          Subscribers, billing, bandwidth monitoring, MikroTik integration — all automated. Start your free trial in 60 seconds.
        </p>
        <div style={{ display: 'flex', gap: 16, justifyContent: 'center', flexWrap: 'wrap' }}>
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
            Start Free Trial
            <ArrowRightIcon style={{ width: 20, height: 20 }} />
          </button>
          <Link
            to="/login"
            style={{
              padding: '16px 32px', fontSize: 17, fontWeight: 600, color: COLORS.textMuted,
              background: 'transparent', border: `1px solid ${COLORS.border}`, borderRadius: 12,
              textDecoration: 'none', display: 'inline-flex', alignItems: 'center', gap: 8,
              transition: 'all 0.2s',
            }}
          >
            Login to Dashboard
          </Link>
        </div>
      </div>

      {/* Stats */}
      <div style={{ display: 'flex', justifyContent: 'center', gap: 48, padding: '32px 24px', flexWrap: 'wrap' }}>
        {stats.map(({ value, label }) => (
          <div key={label} style={{ textAlign: 'center' }}>
            <div style={{ color: '#fff', fontSize: 28, fontWeight: 800 }}>{value}</div>
            <div style={{ color: COLORS.textMuted, fontSize: 13, marginTop: 4 }}>{label}</div>
          </div>
        ))}
      </div>

      {/* Features */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))', gap: 20, maxWidth: 1100, margin: '0 auto', padding: '48px 24px 80px' }}>
        {features.map(({ Icon, title, desc }, i) => (
          <div key={i} style={{
            background: 'rgba(30,41,59,0.6)', border: '1px solid rgba(71,85,105,0.3)', borderRadius: 16,
            padding: '28px 24px', backdropFilter: 'blur(8px)', transition: 'transform 0.2s, border-color 0.2s',
          }}
            onMouseOver={e => { e.currentTarget.style.transform = 'translateY(-4px)'; e.currentTarget.style.borderColor = 'rgba(79,70,229,0.4)' }}
            onMouseOut={e => { e.currentTarget.style.transform = 'translateY(0)'; e.currentTarget.style.borderColor = 'rgba(71,85,105,0.3)' }}
          >
            <div style={{
              width: 44, height: 44, borderRadius: 10, marginBottom: 14,
              background: 'rgba(79,70,229,0.15)', display: 'flex', alignItems: 'center', justifyContent: 'center',
            }}>
              <Icon style={{ width: 22, height: 22, color: COLORS.primaryLight }} />
            </div>
            <h3 style={{ color: '#fff', fontSize: 17, fontWeight: 600, margin: '0 0 8px' }}>{title}</h3>
            <p style={{ color: COLORS.textMuted, fontSize: 14, lineHeight: 1.6, margin: 0 }}>{desc}</p>
          </div>
        ))}
      </div>

      {/* How it works */}
      <div style={{ background: 'rgba(30,41,59,0.3)', padding: '60px 24px', borderTop: '1px solid rgba(71,85,105,0.2)', borderBottom: '1px solid rgba(71,85,105,0.2)' }}>
        <div style={{ textAlign: 'center', marginBottom: 40 }}>
          <h2 style={{ color: '#fff', fontSize: 28, fontWeight: 700, margin: '0 0 8px' }}>How It Works</h2>
          <p style={{ color: COLORS.textMuted, fontSize: 15 }}>Get started in three simple steps</p>
        </div>
        <div style={{ display: 'flex', justifyContent: 'center', gap: 32, maxWidth: 900, margin: '0 auto', flexWrap: 'wrap' }}>
          {[
            { step: '1', title: 'Sign Up', desc: 'Create your account in 60 seconds. Free 14-day trial, no credit card required.' },
            { step: '2', title: 'Connect MikroTik', desc: 'Paste 3 lines of RADIUS commands into your router terminal.' },
            { step: '3', title: 'Start Managing', desc: 'Subscribers authenticate automatically. Monitor everything from one dashboard.' },
          ].map(({ step, title, desc }) => (
            <div key={step} style={{ flex: '1 1 240px', textAlign: 'center', maxWidth: 280 }}>
              <div style={{
                width: 48, height: 48, borderRadius: '50%', margin: '0 auto 16px',
                background: COLORS.primary, color: '#fff', fontSize: 20, fontWeight: 700,
                display: 'flex', alignItems: 'center', justifyContent: 'center',
              }}>{step}</div>
              <h3 style={{ color: '#fff', fontSize: 17, fontWeight: 600, margin: '0 0 8px' }}>{title}</h3>
              <p style={{ color: COLORS.textMuted, fontSize: 14, lineHeight: 1.6, margin: 0 }}>{desc}</p>
            </div>
          ))}
        </div>
      </div>

      {/* Bottom CTA */}
      <div style={{ textAlign: 'center', padding: '64px 24px 80px' }}>
        <h2 style={{ color: '#fff', fontSize: 30, fontWeight: 700, margin: '0 0 12px' }}>Ready to get started?</h2>
        <p style={{ color: COLORS.textMuted, fontSize: 16, margin: '0 0 32px' }}>Join 50+ ISPs already using ProxRad Cloud</p>
        <button
          onClick={onGetStarted}
          style={{
            padding: '16px 48px', fontSize: 17, fontWeight: 600, color: '#fff',
            background: `linear-gradient(135deg, ${COLORS.primary}, ${COLORS.primaryDark})`,
            border: 'none', borderRadius: 12, cursor: 'pointer',
            boxShadow: '0 4px 24px rgba(79,70,229,0.4)',
          }}
        >
          Start Free Trial
        </button>
        <div style={{ marginTop: 12, color: COLORS.textMuted, fontSize: 13 }}>14-day free trial &middot; No credit card required</div>
      </div>

      {/* Footer */}
      <div style={{ borderTop: '1px solid rgba(71,85,105,0.2)', padding: '24px 32px', display: 'flex', justifyContent: 'space-between', alignItems: 'center', maxWidth: 1200, margin: '0 auto', flexWrap: 'wrap', gap: 12 }}>
        <span style={{ color: COLORS.textMuted, fontSize: 13 }}>&copy; 2026 ProxRad. All rights reserved.</span>
        <div style={{ display: 'flex', gap: 20 }}>
          <a href="https://proxrad.com" target="_blank" rel="noopener noreferrer" style={{ color: COLORS.textMuted, fontSize: 13, textDecoration: 'none' }}>Website</a>
          <a href="https://proxrad.com/pricing.html" target="_blank" rel="noopener noreferrer" style={{ color: COLORS.textMuted, fontSize: 13, textDecoration: 'none' }}>Pricing</a>
          <a href="mailto:support@proxrad.com" style={{ color: COLORS.textMuted, fontSize: 13, textDecoration: 'none' }}>Contact</a>
        </div>
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
  const [copied, setCopied] = useState(false)
  const panelUrl = data.panel_url || `https://${data.subdomain}.saas.proxrad.com`

  const copyField = (text) => {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  const fieldRow = (label, value, mono) => (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '12px 16px', background: 'rgba(255,255,255,0.04)', borderRadius: 8, marginBottom: 8 }}>
      <div>
        <div style={{ color: COLORS.textMuted, fontSize: 12, fontWeight: 500, marginBottom: 2 }}>{label}</div>
        <div style={{ color: COLORS.text, fontSize: 15, fontWeight: 600, fontFamily: mono ? 'monospace' : 'inherit', wordBreak: 'break-all' }}>{value}</div>
      </div>
      <button onClick={() => copyField(value)} style={{ background: 'rgba(99,102,241,0.15)', border: 'none', borderRadius: 6, padding: '6px 10px', cursor: 'pointer', color: COLORS.primaryLight, fontSize: 12, fontWeight: 500, whiteSpace: 'nowrap' }}>
        {copied ? 'Copied!' : 'Copy'}
      </button>
    </div>
  )

  return (
    <div style={{ minHeight: '100vh', background: COLORS.bg, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 24 }}>
      <div style={{ width: '100%', maxWidth: 480 }}>
        {/* Success Icon */}
        <div style={{ textAlign: 'center', marginBottom: 24 }}>
          <div style={{
            width: 72, height: 72, borderRadius: 18, margin: '0 auto 16px',
            background: 'rgba(34,197,94,0.15)', display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}>
            <CheckCircleIcon style={{ width: 40, height: 40, color: COLORS.success }} />
          </div>
          <h2 style={{ color: '#fff', fontSize: 24, fontWeight: 700, margin: '0 0 6px' }}>Account Created!</h2>
          <p style={{ color: COLORS.textMuted, fontSize: 14, margin: 0 }}>Your panel is ready. Use the details below to log in.</p>
        </div>

        {/* Credentials Card */}
        <div style={{
          background: COLORS.bgCard, borderRadius: 16, padding: 24,
          border: `1px solid ${COLORS.border}`, marginBottom: 20,
        }}>
          {fieldRow('Panel URL', panelUrl, false)}
          {fieldRow('Username', data.admin_username || 'admin', true)}
          {fieldRow('Password', data.password || '', true)}
        </div>

        {/* Login Button */}
        <a
          href={panelUrl}
          style={{
            display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 10,
            width: '100%', padding: '16px 0', fontSize: 16, fontWeight: 600, color: '#fff',
            background: `linear-gradient(135deg, ${COLORS.primary}, ${COLORS.primaryDark})`,
            border: 'none', borderRadius: 12, cursor: 'pointer', textDecoration: 'none',
            boxShadow: '0 4px 20px rgba(79,70,229,0.3)',
          }}
        >
          Go to Login Page <ArrowRightIcon style={{ width: 18, height: 18 }} />
        </a>

        <p style={{ textAlign: 'center', color: COLORS.textMuted, fontSize: 13, marginTop: 16 }}>
          A confirmation email has been sent to <strong style={{ color: COLORS.text }}>{data.email}</strong>
        </p>

      </div>
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
