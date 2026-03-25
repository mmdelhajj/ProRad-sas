import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import api from '../services/api'

// SaaS Super Admin API
const saasApi = {
  login: (data) => api.post('/saas/login', data),
  getTenants: () => api.get('/saas/tenants'),
  getTenant: (id) => api.get(`/saas/tenants/${id}`),
  createTenant: (data) => api.post('/saas/tenants', data),
  updateTenant: (id, data) => api.put(`/saas/tenants/${id}`, data),
  deleteTenant: (id) => api.delete(`/saas/tenants/${id}`),
  getScript: (id) => api.get(`/saas/tenants/${id}/script`),
  getStats: () => api.get('/saas/tenants/stats'),
}

function SuperAdminLogin({ onLogin }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')

  const loginMutation = useMutation({
    mutationFn: () => saasApi.login({ username, password }),
    onSuccess: (res) => {
      if (res.data.success) {
        localStorage.setItem('saas_token', res.data.token)
        api.defaults.headers.common['Authorization'] = `Bearer ${res.data.token}`
        onLogin()
      }
    },
    onError: (err) => setError(err.response?.data?.message || 'Login failed'),
  })

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-900">
      <div className="bg-gray-800 p-8 rounded-lg shadow-xl w-full max-w-md">
        <h1 className="text-2xl font-bold text-white mb-6">ProxPanel SaaS Admin</h1>
        {error && <div className="bg-red-500/20 text-red-400 p-3 rounded mb-4">{error}</div>}
        <input
          type="text"
          placeholder="Username"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          className="w-full p-3 mb-4 bg-gray-700 text-white rounded border border-gray-600"
        />
        <input
          type="password"
          placeholder="Password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && loginMutation.mutate()}
          className="w-full p-3 mb-4 bg-gray-700 text-white rounded border border-gray-600"
        />
        <button
          onClick={() => loginMutation.mutate()}
          disabled={loginMutation.isPending}
          className="w-full p-3 bg-blue-600 hover:bg-blue-700 text-white rounded font-medium"
        >
          {loginMutation.isPending ? 'Logging in...' : 'Login'}
        </button>
      </div>
    </div>
  )
}

function CreateTenantModal({ onClose, onCreated }) {
  const [form, setForm] = useState({
    name: '', subdomain: '', admin_username: '', admin_password: '', admin_email: '',
    plan: 'free', max_subscribers: 50, max_routers: 1,
  })
  const [script, setScript] = useState('')

  const createMutation = useMutation({
    mutationFn: () => saasApi.createTenant(form),
    onSuccess: (res) => {
      if (res.data.mikrotik_script) setScript(res.data.mikrotik_script)
      onCreated()
    },
  })

  if (script) {
    return (
      <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
        <div className="bg-gray-800 p-6 rounded-lg w-full max-w-2xl max-h-[80vh] overflow-y-auto">
          <h2 className="text-xl font-bold text-white mb-4">Tenant Created - MikroTik Script</h2>
          <p className="text-gray-400 mb-3">Paste this script into the MikroTik terminal to connect:</p>
          <pre className="bg-gray-900 text-green-400 p-4 rounded text-sm overflow-x-auto whitespace-pre-wrap">{script}</pre>
          <div className="flex gap-3 mt-4">
            <button onClick={() => navigator.clipboard.writeText(script)} className="px-4 py-2 bg-blue-600 text-white rounded">Copy Script</button>
            <button onClick={onClose} className="px-4 py-2 bg-gray-600 text-white rounded">Close</button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
      <div className="bg-gray-800 p-6 rounded-lg w-full max-w-lg">
        <h2 className="text-xl font-bold text-white mb-4">Create New Tenant</h2>
        <div className="space-y-3">
          <input placeholder="Company Name" value={form.name} onChange={(e) => setForm({...form, name: e.target.value})} className="w-full p-2 bg-gray-700 text-white rounded border border-gray-600" />
          <input placeholder="Subdomain (e.g. acme)" value={form.subdomain} onChange={(e) => setForm({...form, subdomain: e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '')})} className="w-full p-2 bg-gray-700 text-white rounded border border-gray-600" />
          <p className="text-gray-400 text-sm">{form.subdomain ? `${form.subdomain}.saas.proxrad.com` : 'Enter subdomain'}</p>
          <input placeholder="Admin Username" value={form.admin_username} onChange={(e) => setForm({...form, admin_username: e.target.value})} className="w-full p-2 bg-gray-700 text-white rounded border border-gray-600" />
          <input placeholder="Admin Password" type="password" value={form.admin_password} onChange={(e) => setForm({...form, admin_password: e.target.value})} className="w-full p-2 bg-gray-700 text-white rounded border border-gray-600" />
          <input placeholder="Admin Email" type="email" value={form.admin_email} onChange={(e) => setForm({...form, admin_email: e.target.value})} className="w-full p-2 bg-gray-700 text-white rounded border border-gray-600" />
          <select value={form.plan} onChange={(e) => setForm({...form, plan: e.target.value})} className="w-full p-2 bg-gray-700 text-white rounded border border-gray-600">
            <option value="free">Free (50 subscribers)</option>
            <option value="starter">Starter (200 subscribers)</option>
            <option value="pro">Pro (1000 subscribers)</option>
            <option value="enterprise">Enterprise (unlimited)</option>
          </select>
        </div>
        <div className="flex gap-3 mt-4">
          <button onClick={() => createMutation.mutate()} disabled={createMutation.isPending} className="px-4 py-2 bg-green-600 hover:bg-green-700 text-white rounded">
            {createMutation.isPending ? 'Creating...' : 'Create Tenant'}
          </button>
          <button onClick={onClose} className="px-4 py-2 bg-gray-600 text-white rounded">Cancel</button>
        </div>
        {createMutation.isError && <p className="text-red-400 mt-2">{createMutation.error?.response?.data?.message || 'Failed'}</p>}
      </div>
    </div>
  )
}

function TenantList() {
  const queryClient = useQueryClient()
  const [showCreate, setShowCreate] = useState(false)
  const [selectedTenant, setSelectedTenant] = useState(null)

  const { data: tenants, isLoading } = useQuery({
    queryKey: ['saas-tenants'],
    queryFn: () => saasApi.getTenants().then(r => r.data.data || r.data.tenants || []),
  })

  const { data: stats } = useQuery({
    queryKey: ['saas-stats'],
    queryFn: () => saasApi.getStats().then(r => r.data.data || r.data),
  })

  const scriptQuery = useQuery({
    queryKey: ['saas-script', selectedTenant],
    queryFn: () => saasApi.getScript(selectedTenant).then(r => r.data.data?.script || r.data.script),
    enabled: !!selectedTenant,
  })

  const statusColors = {
    active: 'bg-green-500/20 text-green-400',
    trial: 'bg-yellow-500/20 text-yellow-400',
    suspended: 'bg-red-500/20 text-red-400',
  }

  return (
    <div className="min-h-screen bg-gray-900 text-white p-6">
      {/* Header */}
      <div className="flex justify-between items-center mb-8">
        <div>
          <h1 className="text-3xl font-bold">ProxPanel SaaS Dashboard</h1>
          <p className="text-gray-400 mt-1">Manage ISP tenants</p>
        </div>
        <button onClick={() => setShowCreate(true)} className="px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded font-medium">
          + New Tenant
        </button>
      </div>

      {/* Stats */}
      {stats && (
        <div className="grid grid-cols-4 gap-4 mb-8">
          <div className="bg-gray-800 p-4 rounded-lg">
            <p className="text-gray-400 text-sm">Total Tenants</p>
            <p className="text-2xl font-bold">{stats.total_tenants || 0}</p>
          </div>
          <div className="bg-gray-800 p-4 rounded-lg">
            <p className="text-gray-400 text-sm">Active Tenants</p>
            <p className="text-2xl font-bold text-green-400">{stats.active_tenants || 0}</p>
          </div>
          <div className="bg-gray-800 p-4 rounded-lg">
            <p className="text-gray-400 text-sm">Total Subscribers</p>
            <p className="text-2xl font-bold">{stats.total_subscribers || 0}</p>
          </div>
          <div className="bg-gray-800 p-4 rounded-lg">
            <p className="text-gray-400 text-sm">VPN Connected</p>
            <p className="text-2xl font-bold text-blue-400">{stats.connected_vpns || 0}</p>
          </div>
        </div>
      )}

      {/* Tenants Table */}
      <div className="bg-gray-800 rounded-lg overflow-hidden">
        <table className="w-full">
          <thead className="bg-gray-700">
            <tr>
              <th className="text-left p-3">Name</th>
              <th className="text-left p-3">Subdomain</th>
              <th className="text-left p-3">Plan</th>
              <th className="text-left p-3">Status</th>
              <th className="text-left p-3">Subscribers</th>
              <th className="text-left p-3">VPN</th>
              <th className="text-left p-3">Actions</th>
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <tr><td colSpan="7" className="p-4 text-center text-gray-400">Loading...</td></tr>
            ) : tenants?.length === 0 ? (
              <tr><td colSpan="7" className="p-4 text-center text-gray-400">No tenants yet. Create one to get started.</td></tr>
            ) : tenants?.map((t) => (
              <tr key={t.id} className="border-t border-gray-700 hover:bg-gray-700/50">
                <td className="p-3 font-medium">{t.name}</td>
                <td className="p-3"><span className="text-blue-400">{t.subdomain}.saas.proxrad.com</span></td>
                <td className="p-3 capitalize">{t.plan}</td>
                <td className="p-3"><span className={`px-2 py-1 rounded text-sm ${statusColors[t.status] || ''}`}>{t.status}</span></td>
                <td className="p-3">{t.current_subscriber_count || 0} / {t.max_subscribers}</td>
                <td className="p-3">{t.wg_client_ip || '-'}</td>
                <td className="p-3">
                  <button onClick={() => setSelectedTenant(t.id)} className="text-blue-400 hover:text-blue-300 mr-3">Script</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Create Modal */}
      {showCreate && (
        <CreateTenantModal
          onClose={() => setShowCreate(false)}
          onCreated={() => { setShowCreate(false); queryClient.invalidateQueries(['saas-tenants']); }}
        />
      )}

      {/* Script Modal */}
      {selectedTenant && scriptQuery.data && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
          <div className="bg-gray-800 p-6 rounded-lg w-full max-w-2xl max-h-[80vh] overflow-y-auto">
            <h2 className="text-xl font-bold mb-4">MikroTik Connection Script</h2>
            <pre className="bg-gray-900 text-green-400 p-4 rounded text-sm overflow-x-auto whitespace-pre-wrap">{scriptQuery.data}</pre>
            <div className="flex gap-3 mt-4">
              <button onClick={() => navigator.clipboard.writeText(scriptQuery.data)} className="px-4 py-2 bg-blue-600 text-white rounded">Copy</button>
              <button onClick={() => setSelectedTenant(null)} className="px-4 py-2 bg-gray-600 text-white rounded">Close</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default function SuperAdmin() {
  const [isLoggedIn, setIsLoggedIn] = useState(() => {
    const token = localStorage.getItem('saas_token')
    if (token) {
      api.defaults.headers.common['Authorization'] = `Bearer ${token}`
      return true
    }
    return false
  })

  if (!isLoggedIn) {
    return <SuperAdminLogin onLogin={() => setIsLoggedIn(true)} />
  }

  return <TenantList />
}
