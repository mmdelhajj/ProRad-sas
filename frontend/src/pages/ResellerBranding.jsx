import { useState, useEffect, useRef } from 'react'
import { resellerBrandingApi } from '../services/api'
import { useBrandingStore } from '../store/brandingStore'
import toast from 'react-hot-toast'
import {
  PhotoIcon,
  TrashIcon,
  PaintBrushIcon,
  BuildingOfficeIcon,
  CheckIcon,
  GlobeAltIcon,
  LockClosedIcon,
} from '@heroicons/react/24/outline'

const PRESET_COLORS = [
  { color: '#2563eb', name: 'Blue' },
  { color: '#16a34a', name: 'Green' },
  { color: '#7c3aed', name: 'Purple' },
  { color: '#d97706', name: 'Amber' },
  { color: '#dc2626', name: 'Red' },
  { color: '#0891b2', name: 'Cyan' },
]

export default function ResellerBranding() {
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [uploadingLogo, setUploadingLogo] = useState(false)
  const [formData, setFormData] = useState({
    company_name: '',
    primary_color: '#2563eb',
    footer_text: '',
    tagline: '',
  })
  const [logoUrl, setLogoUrl] = useState('')
  const [domain, setDomain] = useState('')
  const [domainSaving, setDomainSaving] = useState(false)
  const [sslRequesting, setSslRequesting] = useState(false)
  const [sslLog, setSslLog] = useState([])
  const [serverIp, setServerIp] = useState('')
  const [serverHasPublicIp, setServerHasPublicIp] = useState(true)
  const logoRef = useRef()
  const { fetchBranding } = useBrandingStore()

  useEffect(() => {
    loadBranding()
  }, [])

  const loadBranding = async () => {
    try {
      const res = await resellerBrandingApi.get()
      const d = res.data
      setFormData({
        company_name: d.company_name || '',
        primary_color: d.primary_color || '#2563eb',
        footer_text: d.footer_text || '',
        tagline: d.tagline || '',
      })
      setLogoUrl(d.logo_path || '')
      setServerIp(d.server_ip || '')
      setServerHasPublicIp(d.server_has_public_ip !== false)
      // Load domain from auth/me
      try {
        const meRes = await resellerBrandingApi.get()
        // domain comes back from reseller context via /auth/me
        const authRaw = localStorage.getItem('proisp-auth')
        if (authRaw) {
          const auth = JSON.parse(authRaw)
          const reseller = auth?.state?.user?.reseller
          if (reseller?.custom_domain !== undefined) {
            setDomain(reseller.custom_domain || '')
          }
        }
      } catch {}
    } catch (e) {
      toast.error('Failed to load branding')
    } finally {
      setLoading(false)
    }
  }

  const saveDomain = async () => {
    setDomainSaving(true)
    try {
      await resellerBrandingApi.updateDomain(domain.toLowerCase().trim())
      toast.success('Domain saved!')
    } catch (e) {
      toast.error('Failed to save domain')
    } finally {
      setDomainSaving(false)
    }
  }

  const requestSSL = async () => {
    setSslRequesting(true)
    setSslLog(['Starting SSL certificate request...'])
    try {
      const response = await fetch('/api/reseller/branding/ssl', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': 'Bearer ' + (JSON.parse(localStorage.getItem('proisp-auth') || '{}')?.state?.token || '')
        },
        body: JSON.stringify({ email: '' })
      })
      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        const text = decoder.decode(value)
        const lines = text.split('\n').filter(l => l.startsWith('data: ')).map(l => l.slice(6))
        setSslLog(prev => [...prev, ...lines])
        if (lines.some(l => l === 'DONE')) break
      }
    } catch (e) {
      setSslLog(prev => [...prev, 'Error: ' + e.message])
    } finally {
      setSslRequesting(false)
    }
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      await resellerBrandingApi.update(formData)
      toast.success('Branding saved!')
      fetchBranding() // Re-apply branding immediately
    } catch (e) {
      toast.error('Failed to save branding')
    } finally {
      setSaving(false)
    }
  }

  const handleLogoUpload = async (e) => {
    const file = e.target.files[0]
    if (!file) return
    const fd = new FormData()
    fd.append('logo', file)
    setUploadingLogo(true)
    try {
      const res = await resellerBrandingApi.uploadLogo(fd)
      setLogoUrl(res.data.logo_url)
      toast.success('Logo uploaded!')
      fetchBranding()
    } catch (e) {
      toast.error('Failed to upload logo')
    } finally {
      setUploadingLogo(false)
      e.target.value = ''
    }
  }

  const handleDeleteLogo = async () => {
    try {
      await resellerBrandingApi.deleteLogo()
      setLogoUrl('')
      toast.success('Logo removed')
      fetchBranding()
    } catch (e) {
      toast.error('Failed to delete logo')
    }
  }

  if (loading) return (
    <div className="flex items-center justify-center h-64">
      <svg className="animate-spin h-6 w-6 text-[#316AC5]" fill="none" viewBox="0 0 24 24">
        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
      </svg>
    </div>
  )

  return (
    <div className="max-w-2xl mx-auto space-y-3" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Toolbar */}
      <div className="wb-toolbar">
        <PaintBrushIcon className="w-4 h-4 text-[#316AC5]" />
        <span className="text-[13px] font-semibold text-gray-900">Branding</span>
      </div>

      {/* Company Logo */}
      <div className="wb-group">
        <div className="wb-group-title flex items-center gap-1">
          <PhotoIcon className="w-4 h-4 text-gray-500" />
          Company Logo
        </div>
        <div className="wb-group-body">
          <div className="flex items-center gap-3">
            <div className="h-14 w-36 border border-dashed border-[#a0a0a0] flex items-center justify-center bg-[#f0f0f0] overflow-hidden" style={{ borderRadius: '2px' }}>
              {logoUrl ? (
                <img src={logoUrl} alt="Logo" className="max-h-12 max-w-full object-contain" />
              ) : (
                <span className="text-[11px] text-gray-400">No logo</span>
              )}
            </div>
            <div className="flex gap-1">
              <input ref={logoRef} type="file" accept=".png,.jpg,.jpeg,.svg,.webp" className="hidden" onChange={handleLogoUpload} />
              <button
                onClick={() => logoRef.current.click()}
                disabled={uploadingLogo}
                className="btn btn-sm"
              >
                {uploadingLogo ? 'Uploading...' : 'Upload Logo'}
              </button>
              {logoUrl && (
                <button onClick={handleDeleteLogo} className="btn btn-danger btn-sm">
                  <TrashIcon className="h-3.5 w-3.5" />
                </button>
              )}
            </div>
          </div>
          <p className="text-[11px] text-gray-400 mt-1">PNG, JPG, SVG or WEBP -- max 2MB -- recommended 180x36px</p>
        </div>
      </div>

      {/* Company Name */}
      <div className="wb-group">
        <div className="wb-group-title flex items-center gap-1">
          <BuildingOfficeIcon className="w-4 h-4 text-gray-500" />
          Company Name
        </div>
        <div className="wb-group-body">
          <input
            type="text"
            value={formData.company_name}
            onChange={e => setFormData(p => ({ ...p, company_name: e.target.value }))}
            placeholder="Your Company Name"
            className="input w-full"
          />
          <p className="text-[11px] text-gray-400 mt-0.5">Shown in the sidebar when no logo is uploaded</p>
        </div>
      </div>

      {/* Primary Color */}
      <div className="wb-group">
        <div className="wb-group-title flex items-center gap-1">
          <PaintBrushIcon className="w-4 h-4 text-gray-500" />
          Primary Color
        </div>
        <div className="wb-group-body">
          <div className="flex items-center gap-2 mb-2">
            <input
              type="color"
              value={formData.primary_color}
              onChange={e => setFormData(p => ({ ...p, primary_color: e.target.value }))}
              className="h-7 w-7 cursor-pointer border border-[#a0a0a0]"
              style={{ borderRadius: '2px' }}
            />
            <input
              type="text"
              value={formData.primary_color}
              onChange={e => setFormData(p => ({ ...p, primary_color: e.target.value }))}
              placeholder="#2563eb"
              className="input w-28 font-mono"
            />
          </div>
          <div className="flex gap-1.5 flex-wrap">
            {PRESET_COLORS.map(({ color, name }) => (
              <button
                key={color}
                title={name}
                onClick={() => setFormData(p => ({ ...p, primary_color: color }))}
                className="h-6 w-6 border-2 flex items-center justify-center hover:opacity-80"
                style={{ backgroundColor: color, borderColor: formData.primary_color === color ? '#000' : 'transparent', borderRadius: '2px' }}
              >
                {formData.primary_color === color && <CheckIcon className="h-3.5 w-3.5 text-white" />}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Tagline & Footer */}
      <div className="wb-group">
        <div className="wb-group-title">Additional Text</div>
        <div className="wb-group-body space-y-3">
          <div>
            <label className="label">Tagline</label>
            <input
              type="text"
              value={formData.tagline}
              onChange={e => setFormData(p => ({ ...p, tagline: e.target.value }))}
              placeholder="Fast & Reliable Internet"
              className="input w-full"
            />
          </div>
          <div>
            <label className="label">Footer Text</label>
            <input
              type="text"
              value={formData.footer_text}
              onChange={e => setFormData(p => ({ ...p, footer_text: e.target.value }))}
              placeholder="(c) 2026 Your Company"
              className="input w-full"
            />
          </div>
        </div>
      </div>

      {/* Custom Domain */}
      <div className="wb-group">
        <div className="wb-group-title flex items-center gap-1">
          <GlobeAltIcon className="w-4 h-4 text-gray-500" />
          Custom Domain
        </div>
        <div className="wb-group-body space-y-3">
          <div className="flex gap-1">
            <input
              type="text"
              value={domain}
              onChange={e => setDomain(e.target.value.toLowerCase().trim())}
              placeholder="portal.myisp.com"
              className="input flex-1 font-mono"
            />
            <button onClick={saveDomain} disabled={domainSaving} className="btn whitespace-nowrap">
              {domainSaving ? 'Saving...' : 'Save Domain'}
            </button>
          </div>
          {domain && (
            <>
              <div className="p-2 border border-[#2196F3] bg-[#e3f2fd] text-[12px]" style={{ borderRadius: '2px' }}>
                <p className="font-medium text-[#1565c0] mb-1">DNS Setup Instructions</p>
                <p className="text-[#1976d2]">Add this A record to your domain's DNS:</p>
                <div className="font-mono bg-white p-1.5 text-[11px] border border-[#a0a0a0] mt-1" style={{ borderRadius: '2px' }}>
                  <span className="text-gray-500">Type:</span> A &nbsp;&nbsp;
                  <span className="text-gray-500">Name:</span> {domain.split('.').slice(0, -2).join('.') || '@'} &nbsp;&nbsp;
                  <span className="text-gray-500">Value:</span> <span className={`font-bold ${serverHasPublicIp ? 'text-[#4CAF50]' : 'text-[#f44336]'}`}>
                    {serverIp || 'YOUR_SERVER_IP'}
                  </span> &nbsp;&nbsp;
                  <span className="text-gray-500">TTL:</span> 3600
                </div>
                <p className="text-[#1565c0] text-[11px] mt-1">After DNS propagates (up to 24h), your portal will be available at <strong>http://{domain}</strong></p>
              </div>
              {serverIp && !serverHasPublicIp && (
                <div className="p-2 border border-[#f44336] bg-[#ffe0e0] text-[12px]" style={{ borderRadius: '2px' }}>
                  <p className="font-medium text-[#c62828]">Warning: This server does not have a public IP</p>
                  <p className="text-[#e53935] text-[11px] mt-0.5">
                    Custom domains require a public IP address. Your server IP ({serverIp}) is a private/internal IP.
                    Contact your hosting provider to assign a public IP.
                  </p>
                </div>
              )}
            </>
          )}
          {domain && (
            <div className="border-t border-[#a0a0a0] pt-3">
              <p className="text-[12px] font-semibold text-gray-800 flex items-center gap-1 mb-1">
                <LockClosedIcon className="w-3.5 h-3.5 text-[#4CAF50]" /> SSL Certificate (HTTPS)
              </p>
              <p className="text-[12px] text-gray-500 mb-2">
                After your DNS is set up, click below to automatically install a free Let's Encrypt SSL certificate.
                Make sure your domain is reachable on port 80 first.
              </p>
              <button
                onClick={requestSSL}
                disabled={sslRequesting}
                className="btn btn-sm flex items-center gap-1"
              >
                <LockClosedIcon className="w-3.5 h-3.5" />
                {sslRequesting ? 'Requesting certificate...' : 'Request SSL Certificate'}
              </button>
              {sslLog.length > 0 && (
                <div className="mt-2 bg-[#1a1a1a] p-2 font-mono text-[11px] text-[#4CAF50] max-h-48 overflow-y-auto border border-[#555]" style={{ borderRadius: '2px' }}>
                  {sslLog.map((line, i) => (
                    <div key={i}>{line}</div>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Live Preview */}
      <div className="wb-group">
        <div className="wb-group-title">Sidebar Preview</div>
        <div className="wb-group-body">
          <div className="w-48 border border-[#a0a0a0] overflow-hidden" style={{ borderRadius: '2px', background: formData.primary_color }}>
            <div className="p-2 flex items-center gap-2">
              {logoUrl ? (
                <img src={logoUrl} alt="logo" className="h-6 max-w-[120px] object-contain" />
              ) : (
                <span className="text-white font-bold text-[12px] truncate">{formData.company_name || 'Your Company'}</span>
              )}
            </div>
            <div className="bg-white/10 p-1.5 space-y-0.5">
              {['Dashboard', 'Subscribers', 'Services'].map(item => (
                <div key={item} className="text-white/80 text-[11px] px-1.5 py-0.5">{item}</div>
              ))}
            </div>
          </div>
        </div>
      </div>

      <button
        onClick={handleSave}
        disabled={saving}
        className="btn btn-primary w-full"
      >
        {saving ? 'Saving...' : 'Save Branding'}
      </button>
    </div>
  )
}
