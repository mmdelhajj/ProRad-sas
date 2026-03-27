import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { tenantAccountApi } from '../services/api'
import {
  BuildingOfficeIcon,
  CreditCardIcon,
  ArrowUpIcon,
  ArrowDownIcon,
  CheckCircleIcon,
  ClockIcon,
  XCircleIcon,
  SwatchIcon,
  PhotoIcon,
  TrashIcon,
} from '@heroicons/react/24/outline'

export default function TenantAccount() {
  const [activeTab, setActiveTab] = useState('overview')
  const queryClient = useQueryClient()

  const tabs = [
    { id: 'overview', name: 'Overview', icon: BuildingOfficeIcon },
    { id: 'plans', name: 'Plans', icon: ArrowUpIcon },
    { id: 'billing', name: 'Billing', icon: CreditCardIcon },
  ]

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Account</h1>

      {/* Tabs */}
      <div className="border-b border-gray-200 dark:border-gray-700">
        <nav className="flex space-x-8">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`flex items-center gap-2 py-3 px-1 border-b-2 text-sm font-medium transition-colors ${
                activeTab === tab.id
                  ? 'border-blue-600 text-blue-600 dark:text-blue-400'
                  : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'
              }`}
            >
              <tab.icon className="w-4 h-4" />
              {tab.name}
            </button>
          ))}
        </nav>
      </div>

      {activeTab === 'overview' && <OverviewTab />}
      {activeTab === 'plans' && <PlansTab />}
      {activeTab === 'billing' && <BillingTab />}
    </div>
  )
}

function OverviewTab() {
  const { data, isLoading } = useQuery({
    queryKey: ['tenant-overview'],
    queryFn: () => tenantAccountApi.getOverview().then(r => r.data.data),
  })

  if (isLoading) return <div className="animate-pulse h-48 bg-gray-100 dark:bg-gray-800 rounded-lg" />

  if (!data) return null

  const usagePercent = data.max_subscribers > 0 ? Math.round((data.subscriber_count / data.max_subscribers) * 100) : 0

  return (
    <div className="space-y-6">
      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5">
          <p className="text-sm text-gray-500 dark:text-gray-400">Company</p>
          <p className="text-lg font-semibold text-gray-900 dark:text-white mt-1">{data.tenant_name}</p>
          <p className="text-xs text-gray-400 mt-1">{data.subdomain}.saas.proxrad.com</p>
        </div>
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5">
          <p className="text-sm text-gray-500 dark:text-gray-400">Current Plan</p>
          <p className="text-lg font-semibold text-blue-600 dark:text-blue-400 mt-1">{data.plan_display_name || data.plan_name}</p>
          {data.plan_price > 0 && <p className="text-xs text-gray-400 mt-1">${data.plan_price}/month</p>}
        </div>
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5">
          <p className="text-sm text-gray-500 dark:text-gray-400">Subscribers</p>
          <p className="text-lg font-semibold text-gray-900 dark:text-white mt-1">{data.subscriber_count} / {data.max_subscribers}</p>
          <div className="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2 mt-2">
            <div className={`h-2 rounded-full ${usagePercent > 90 ? 'bg-red-500' : usagePercent > 70 ? 'bg-yellow-500' : 'bg-green-500'}`} style={{ width: `${Math.min(usagePercent, 100)}%` }} />
          </div>
          <p className="text-xs text-gray-400 mt-1">{usagePercent}% used</p>
        </div>
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5">
          <p className="text-sm text-gray-500 dark:text-gray-400">Status</p>
          <span className={`inline-flex items-center mt-1 px-2.5 py-0.5 rounded-full text-xs font-medium ${
            data.status === 'active' ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
            : data.status === 'trial' ? 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400'
            : 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400'
          }`}>{data.status}</span>
          {data.is_trial_active && <p className="text-xs text-yellow-600 mt-2">{data.trial_days_left} days left in trial</p>}
        </div>
      </div>

      {/* Plan Limits */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Plan Limits</h3>
        <div className="grid grid-cols-3 gap-6">
          <div>
            <p className="text-sm text-gray-500 dark:text-gray-400">Max Subscribers</p>
            <p className="text-xl font-bold text-gray-900 dark:text-white">{data.max_subscribers === -1 ? 'Unlimited' : data.max_subscribers}</p>
          </div>
          <div>
            <p className="text-sm text-gray-500 dark:text-gray-400">Max Resellers</p>
            <p className="text-xl font-bold text-gray-900 dark:text-white">{data.max_resellers === -1 ? 'Unlimited' : data.max_resellers}</p>
          </div>
          <div>
            <p className="text-sm text-gray-500 dark:text-gray-400">Max Routers</p>
            <p className="text-xl font-bold text-gray-900 dark:text-white">{data.max_routers === -1 ? 'Unlimited' : data.max_routers}</p>
          </div>
        </div>
      </div>

      {/* Plan Change Requests */}
      <PlanChangeRequestsList />
    </div>
  )
}

function PlansTab() {
  const queryClient = useQueryClient()
  const [confirmPlan, setConfirmPlan] = useState(null)
  const [notes, setNotes] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['tenant-plans'],
    queryFn: () => tenantAccountApi.getPlans().then(r => r.data),
  })

  const requestMutation = useMutation({
    mutationFn: (planData) => tenantAccountApi.requestPlanChange(planData),
    onSuccess: () => {
      queryClient.invalidateQueries(['tenant-plans'])
      queryClient.invalidateQueries(['tenant-plan-changes'])
      setConfirmPlan(null)
      setNotes('')
    },
  })

  if (isLoading) return <div className="animate-pulse h-48 bg-gray-100 dark:bg-gray-800 rounded-lg" />

  const plans = data?.data || []
  const currentPlan = data?.current_plan

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {plans.map((plan) => {
          const isCurrent = plan.name === currentPlan
          let features = []
          try { features = JSON.parse(plan.features || '[]') } catch {}

          return (
            <div key={plan.id} className={`relative bg-white dark:bg-gray-800 rounded-lg border-2 p-6 ${
              isCurrent ? 'border-blue-500 ring-2 ring-blue-200 dark:ring-blue-900' : 'border-gray-200 dark:border-gray-700'
            }`}>
              {isCurrent && (
                <span className="absolute -top-3 left-4 bg-blue-600 text-white text-xs font-bold px-3 py-1 rounded-full">Current Plan</span>
              )}
              <h3 className="text-lg font-bold text-gray-900 dark:text-white">{plan.display_name}</h3>
              <p className="text-3xl font-bold text-gray-900 dark:text-white mt-2">
                ${plan.price_monthly}<span className="text-sm font-normal text-gray-500">/mo</span>
              </p>
              <p className="text-sm text-gray-500 dark:text-gray-400 mt-2">
                {plan.max_subscribers === -1 ? 'Unlimited' : plan.max_subscribers} subscribers
              </p>
              <ul className="mt-4 space-y-2">
                {features.map((f, i) => (
                  <li key={i} className="flex items-start gap-2 text-sm text-gray-600 dark:text-gray-300">
                    <CheckCircleIcon className="w-4 h-4 text-green-500 mt-0.5 flex-shrink-0" />
                    {f}
                  </li>
                ))}
              </ul>
              {!isCurrent && (
                <button
                  onClick={() => setConfirmPlan(plan)}
                  className="mt-4 w-full py-2 px-4 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 transition-colors"
                >
                  {plan.price_monthly > 0 ? 'Request Upgrade' : 'Request Downgrade'}
                </button>
              )}
            </div>
          )
        })}
      </div>

      {/* Confirm Modal */}
      {confirmPlan && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-gray-800 rounded-lg p-6 max-w-md w-full mx-4">
            <h3 className="text-lg font-bold text-gray-900 dark:text-white">Request Plan Change</h3>
            <p className="text-sm text-gray-600 dark:text-gray-400 mt-2">
              Request to change to <strong>{confirmPlan.display_name}</strong> (${confirmPlan.price_monthly}/mo)?
            </p>
            <textarea
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              placeholder="Optional notes..."
              className="mt-3 w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white p-3 text-sm"
              rows={3}
            />
            <div className="flex justify-end gap-3 mt-4">
              <button onClick={() => { setConfirmPlan(null); setNotes('') }} className="px-4 py-2 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-900">Cancel</button>
              <button
                onClick={() => requestMutation.mutate({ plan_name: confirmPlan.name, notes })}
                disabled={requestMutation.isPending}
                className="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 disabled:opacity-50"
              >
                {requestMutation.isPending ? 'Submitting...' : 'Submit Request'}
              </button>
            </div>
            {requestMutation.isError && (
              <p className="text-sm text-red-600 mt-2">{requestMutation.error?.response?.data?.message || 'Failed to submit request'}</p>
            )}
          </div>
        </div>
      )}

      <PlanChangeRequestsList />
    </div>
  )
}

function PlanChangeRequestsList() {
  const { data } = useQuery({
    queryKey: ['tenant-plan-changes'],
    queryFn: () => tenantAccountApi.getPlanChanges().then(r => r.data.data),
  })

  if (!data || data.length === 0) return null

  const statusBadge = (status) => {
    const styles = {
      pending: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
      approved: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
      rejected: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
    }
    const icons = {
      pending: ClockIcon,
      approved: CheckCircleIcon,
      rejected: XCircleIcon,
    }
    const Icon = icons[status] || ClockIcon
    return (
      <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${styles[status] || styles.pending}`}>
        <Icon className="w-3 h-3" />{status}
      </span>
    )
  }

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
      <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Plan Change Requests</h3>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-gray-500 dark:text-gray-400 border-b dark:border-gray-700">
              <th className="pb-2">Date</th>
              <th className="pb-2">From</th>
              <th className="pb-2">To</th>
              <th className="pb-2">Status</th>
              <th className="pb-2">Notes</th>
            </tr>
          </thead>
          <tbody>
            {data.map((req) => (
              <tr key={req.id} className="border-b dark:border-gray-700">
                <td className="py-2 text-gray-900 dark:text-white">{new Date(req.created_at).toLocaleDateString()}</td>
                <td className="py-2 text-gray-600 dark:text-gray-300">{req.current_plan}</td>
                <td className="py-2 text-gray-600 dark:text-gray-300">{req.requested_plan}</td>
                <td className="py-2">{statusBadge(req.status)}</td>
                <td className="py-2 text-gray-500 dark:text-gray-400 text-xs">{req.admin_notes || req.notes || '-'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function BillingTab() {
  const { data, isLoading } = useQuery({
    queryKey: ['tenant-billing'],
    queryFn: () => tenantAccountApi.getBilling().then(r => r.data.data),
  })

  if (isLoading) return <div className="animate-pulse h-48 bg-gray-100 dark:bg-gray-800 rounded-lg" />

  if (!data || data.length === 0) {
    return (
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-12 text-center">
        <CreditCardIcon className="w-12 h-12 text-gray-400 mx-auto" />
        <p className="text-gray-500 dark:text-gray-400 mt-3">No billing events yet</p>
      </div>
    )
  }

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
      <div className="p-6 border-b dark:border-gray-700">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Billing History</h3>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-gray-500 dark:text-gray-400 border-b dark:border-gray-700">
              <th className="p-4">Date</th>
              <th className="p-4">Type</th>
              <th className="p-4">Description</th>
              <th className="p-4">Plan</th>
              <th className="p-4 text-right">Amount</th>
            </tr>
          </thead>
          <tbody>
            {data.map((event) => (
              <tr key={event.id} className="border-b dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50">
                <td className="p-4 text-gray-900 dark:text-white">{new Date(event.created_at).toLocaleDateString()}</td>
                <td className="p-4">
                  <span className="px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400">
                    {event.event_type}
                  </span>
                </td>
                <td className="p-4 text-gray-600 dark:text-gray-300">{event.description}</td>
                <td className="p-4 text-gray-600 dark:text-gray-300">{event.plan_name || '-'}</td>
                <td className="p-4 text-right text-gray-900 dark:text-white font-medium">
                  {event.amount > 0 ? `$${event.amount.toFixed(2)}` : '-'}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function BrandingTab() {
  const queryClient = useQueryClient()
  const [form, setForm] = useState({ company_name: '', primary_color: '#2563eb', tagline: '', footer_text: '' })
  const [logoPreview, setLogoPreview] = useState(null)

  const { data, isLoading } = useQuery({
    queryKey: ['tenant-branding'],
    queryFn: () => tenantAccountApi.getBranding().then(r => {
      const d = r.data.data
      setForm({ company_name: d.company_name || '', primary_color: d.primary_color || '#2563eb', tagline: d.tagline || '', footer_text: d.footer_text || '' })
      setLogoPreview(d.logo_url || null)
      return d
    }),
  })

  const updateMutation = useMutation({
    mutationFn: (data) => tenantAccountApi.updateBranding(data),
    onSuccess: () => queryClient.invalidateQueries(['tenant-branding']),
  })

  const logoMutation = useMutation({
    mutationFn: (formData) => tenantAccountApi.uploadLogo(formData),
    onSuccess: (res) => {
      setLogoPreview(res.data.data.logo_url)
      queryClient.invalidateQueries(['tenant-branding'])
    },
  })

  const deleteLogoMutation = useMutation({
    mutationFn: () => tenantAccountApi.deleteLogo(),
    onSuccess: () => {
      setLogoPreview(null)
      queryClient.invalidateQueries(['tenant-branding'])
    },
  })

  const handleLogoUpload = (e) => {
    const file = e.target.files[0]
    if (!file) return
    const fd = new FormData()
    fd.append('logo', file)
    logoMutation.mutate(fd)
  }

  const colorPresets = ['#2563eb', '#7c3aed', '#059669', '#dc2626', '#ea580c', '#0891b2']

  if (isLoading) return <div className="animate-pulse h-48 bg-gray-100 dark:bg-gray-800 rounded-lg" />

  return (
    <div className="max-w-2xl space-y-6">
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Brand Settings</h3>
        <p className="text-sm text-gray-500 dark:text-gray-400 mb-6">Customize how your panel looks for your team</p>

        {/* Logo */}
        <div className="mb-6">
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Logo</label>
          <div className="flex items-center gap-4">
            {logoPreview ? (
              <div className="relative">
                <img src={logoPreview} alt="Logo" className="h-16 w-auto rounded border dark:border-gray-600" />
                <button
                  onClick={() => deleteLogoMutation.mutate()}
                  className="absolute -top-2 -right-2 bg-red-500 text-white rounded-full p-1 hover:bg-red-600"
                >
                  <TrashIcon className="w-3 h-3" />
                </button>
              </div>
            ) : (
              <div className="h-16 w-16 bg-gray-100 dark:bg-gray-700 rounded border dark:border-gray-600 flex items-center justify-center">
                <PhotoIcon className="w-8 h-8 text-gray-400" />
              </div>
            )}
            <label className="cursor-pointer px-4 py-2 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 text-sm rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600">
              Upload Logo
              <input type="file" accept="image/*" onChange={handleLogoUpload} className="hidden" />
            </label>
          </div>
        </div>

        {/* Company Name */}
        <div className="mb-4">
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Company Name</label>
          <input
            type="text"
            value={form.company_name}
            onChange={(e) => setForm({ ...form, company_name: e.target.value })}
            placeholder="Your company name"
            className="w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white p-2.5 text-sm"
          />
        </div>

        {/* Primary Color */}
        <div className="mb-4">
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Primary Color</label>
          <div className="flex items-center gap-3">
            <input
              type="color"
              value={form.primary_color}
              onChange={(e) => setForm({ ...form, primary_color: e.target.value })}
              className="w-10 h-10 rounded border-0 cursor-pointer"
            />
            <input
              type="text"
              value={form.primary_color}
              onChange={(e) => setForm({ ...form, primary_color: e.target.value })}
              className="w-28 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white p-2 text-sm"
            />
            <div className="flex gap-1.5">
              {colorPresets.map((c) => (
                <button
                  key={c}
                  onClick={() => setForm({ ...form, primary_color: c })}
                  className={`w-6 h-6 rounded-full border-2 ${form.primary_color === c ? 'border-gray-900 dark:border-white' : 'border-transparent'}`}
                  style={{ backgroundColor: c }}
                />
              ))}
            </div>
          </div>
        </div>

        {/* Tagline */}
        <div className="mb-4">
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Login Tagline</label>
          <input
            type="text"
            value={form.tagline}
            onChange={(e) => setForm({ ...form, tagline: e.target.value })}
            placeholder="Shown on login page"
            className="w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white p-2.5 text-sm"
          />
        </div>

        {/* Footer Text */}
        <div className="mb-6">
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Footer Text</label>
          <input
            type="text"
            value={form.footer_text}
            onChange={(e) => setForm({ ...form, footer_text: e.target.value })}
            placeholder="Footer copyright text"
            className="w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white p-2.5 text-sm"
          />
        </div>

        <button
          onClick={() => updateMutation.mutate(form)}
          disabled={updateMutation.isPending}
          className="px-6 py-2.5 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 disabled:opacity-50"
        >
          {updateMutation.isPending ? 'Saving...' : 'Save Branding'}
        </button>
        {updateMutation.isSuccess && <span className="ml-3 text-sm text-green-600">Saved!</span>}
        {updateMutation.isError && <span className="ml-3 text-sm text-red-600">Failed to save</span>}
      </div>

      {/* Preview */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Preview</h3>
        <div className="rounded-lg overflow-hidden border dark:border-gray-600">
          <div className="h-10 flex items-center px-4 gap-2" style={{ backgroundColor: form.primary_color }}>
            {logoPreview ? (
              <img src={logoPreview} alt="" className="h-6 w-auto" />
            ) : (
              <span className="text-white text-sm font-bold">{form.company_name || 'Your Company'}</span>
            )}
          </div>
          <div className="bg-gray-50 dark:bg-gray-900 p-8 text-center">
            <p className="text-gray-500 dark:text-gray-400 text-sm">{form.tagline || 'Your login tagline'}</p>
          </div>
          {form.footer_text && (
            <div className="bg-gray-100 dark:bg-gray-800 px-4 py-2 text-center">
              <p className="text-gray-400 text-xs">{form.footer_text}</p>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
