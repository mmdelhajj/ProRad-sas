import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { bandwidthCustomerApi, nasApi, publicIPApi } from '../services/api'
import toast from 'react-hot-toast'

export default function BandwidthCustomerEdit() {
  const { id } = useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const isNew = !id || id === 'new'

  const [formData, setFormData] = useState({
    name: '', contact_person: '', phone: '', email: '', address: '', notes: '',
    ip_address: '', subnet_mask: '255.255.255.0', gateway: '',
    nas_id: '', interface: '', vlan_id: '', queue_name: '',
    public_ip: '', public_subnet: '', public_gateway: '',
    ip_block_id: '', ip_allocation_id: '',
    download_speed: 2000, upload_speed: 2000,
    cdn_download_speed: 0, cdn_upload_speed: 0,
    speed_source: 'queue',
    burst_enabled: false,
    burst_download: 0, burst_upload: 0,
    burst_threshold_dl: 0, burst_threshold_ul: 0,
    burst_time: 10,
    fup_enabled: false, daily_quota: 0,
    fup1_threshold: 0, fup1_speed: 0,
    fup2_threshold: 0, fup2_speed: 0,
    fup3_threshold: 0, fup3_speed: 0,
    monthly_quota: 0,
    monthly_fup1_threshold: 0, monthly_fup1_speed: 0,
    monthly_fup2_threshold: 0, monthly_fup2_speed: 0,
    monthly_fup3_threshold: 0, monthly_fup3_speed: 0,
    price: 0, billing_cycle: 'monthly',
    start_date: new Date().toISOString().split('T')[0],
    expiry_date: '',
    auto_renew: true,
    status: 'active',
  })

  const [publicIPMode, setPublicIPMode] = useState('none') // 'none', 'block', 'manual'
  const [selectedBlockId, setSelectedBlockId] = useState('')
  const [selectedIP, setSelectedIP] = useState('')
  const [assignedIPs, setAssignedIPs] = useState([]) // multi-IP support
  const [showAddBlock, setShowAddBlock] = useState(false)
  const [newBlock, setNewBlock] = useState({ name: '', cidr: '', gateway: '', description: '' })

  const { data: customerData, isLoading } = useQuery({
    queryKey: ['bandwidth-customer', id],
    queryFn: () => bandwidthCustomerApi.get(id),
    enabled: !isNew,
  })

  const { data: nasData } = useQuery({
    queryKey: ['nas-list'],
    queryFn: () => nasApi.list({ limit: 100 }),
  })

  const { data: blocksData } = useQuery({
    queryKey: ['public-ip-pools'],
    queryFn: () => publicIPApi.listPools(),
  })

  const { data: availableIPsData } = useQuery({
    queryKey: ['public-ip-available', selectedBlockId],
    queryFn: () => publicIPApi.getAvailableIPs(selectedBlockId),
    enabled: !!selectedBlockId,
    staleTime: 0,
  })

  const { data: interfacesData } = useQuery({
    queryKey: ['nas-interfaces', formData.nas_id],
    queryFn: () => nasApi.getInterfaces(formData.nas_id),
    enabled: !!formData.nas_id,
  })

  const nasList = nasData?.data?.data || []
  const blocks = blocksData?.data?.data || []
  const availableIPs = availableIPsData?.data?.data || []
  const mikrotikInterfaces = interfacesData?.data?.data || []

  const createBlockMutation = useMutation({
    mutationFn: (data) => publicIPApi.createPool(data),
    onSuccess: (res) => {
      toast.success('IP Pool created')
      queryClient.invalidateQueries(['public-ip-pools'])
      const created = res?.data?.data
      if (created?.id) setSelectedBlockId(String(created.id))
      setShowAddBlock(false)
      setNewBlock({ name: '', cidr: '', gateway: '', description: '' })
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to create pool'),
  })

  useEffect(() => {
    if (customerData?.data?.data) {
      const c = customerData.data.data
      setFormData({
        name: c.name || '', contact_person: c.contact_person || '',
        phone: c.phone || '', email: c.email || '',
        address: c.address || '', notes: c.notes || '',
        ip_address: c.ip_address || '', subnet_mask: c.subnet_mask || '255.255.255.0',
        gateway: c.gateway || '', nas_id: c.nas_id || '',
        interface: c.interface || '', vlan_id: c.vlan_id || '',
        queue_name: c.queue_name || '',
        public_ip: c.public_ip || '', public_subnet: c.public_subnet || '',
        public_gateway: c.public_gateway || '',
        ip_block_id: c.ip_block_id || '', ip_allocation_id: c.ip_allocation_id || '',
        download_speed: c.download_speed || 2000, upload_speed: c.upload_speed || 2000,
        cdn_download_speed: c.cdn_download_speed || 0, cdn_upload_speed: c.cdn_upload_speed || 0,
        speed_source: c.speed_source || 'queue',
        burst_enabled: c.burst_enabled || false,
        burst_download: c.burst_download || 0, burst_upload: c.burst_upload || 0,
        burst_threshold_dl: c.burst_threshold_dl || 0, burst_threshold_ul: c.burst_threshold_ul || 0,
        burst_time: c.burst_time || 10,
        fup_enabled: c.fup_enabled || false,
        daily_quota: c.daily_quota || 0,
        fup1_threshold: c.fup1_threshold || 0, fup1_speed: c.fup1_speed || 0,
        fup2_threshold: c.fup2_threshold || 0, fup2_speed: c.fup2_speed || 0,
        fup3_threshold: c.fup3_threshold || 0, fup3_speed: c.fup3_speed || 0,
        monthly_quota: c.monthly_quota || 0,
        monthly_fup1_threshold: c.monthly_fup1_threshold || 0, monthly_fup1_speed: c.monthly_fup1_speed || 0,
        monthly_fup2_threshold: c.monthly_fup2_threshold || 0, monthly_fup2_speed: c.monthly_fup2_speed || 0,
        monthly_fup3_threshold: c.monthly_fup3_threshold || 0, monthly_fup3_speed: c.monthly_fup3_speed || 0,
        price: c.price || 0, billing_cycle: c.billing_cycle || 'monthly',
        start_date: c.start_date ? c.start_date.split('T')[0] : '',
        expiry_date: c.expiry_date ? c.expiry_date.split('T')[0] : '',
        auto_renew: c.auto_renew !== false,
        status: c.status || 'active',
      })
      if (c.ip_block_id) {
        setPublicIPMode('block')
        setSelectedBlockId(String(c.ip_block_id))
        // Load existing IPs (comma-separated) into array
        const ips = c.public_ip ? c.public_ip.split(',').map(ip => ip.trim()).filter(Boolean) : []
        setAssignedIPs(ips)
        setSelectedIP('')
      } else if (c.public_ip) {
        setPublicIPMode('manual')
      } else {
        setPublicIPMode('none')
      }
    }
  }, [customerData])

  const saveMutation = useMutation({
    mutationFn: (data) => isNew ? bandwidthCustomerApi.create(data) : bandwidthCustomerApi.update(id, data),
    onSuccess: (res) => {
      toast.success(isNew ? 'Customer created' : 'Customer updated')
      queryClient.invalidateQueries(['bandwidth-customers'])
      if (isNew && res?.data?.data?.id) {
        navigate(`/bandwidth-manager/${res.data.data.id}`)
      }
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to save'),
  })

  const handleChange = (e) => {
    const { name, value, type, checked } = e.target
    setFormData(prev => ({
      ...prev,
      [name]: type === 'checkbox' ? checked : (type === 'number' ? (value === '' ? 0 : Number(value)) : value),
    }))
  }

  const handleSubmit = (e) => {
    e.preventDefault()
    // Clean up types for backend
    const payload = {
      ...formData,
      nas_id: formData.nas_id ? Number(formData.nas_id) : null,
      vlan_id: formData.vlan_id ? Number(formData.vlan_id) : 0,
      ip_block_id: formData.ip_block_id ? Number(formData.ip_block_id) : null,
      ip_allocation_id: formData.ip_allocation_id ? Number(formData.ip_allocation_id) : null,
      download_speed: Number(formData.download_speed),
      upload_speed: Number(formData.upload_speed),
      cdn_download_speed: Number(formData.cdn_download_speed),
      cdn_upload_speed: Number(formData.cdn_upload_speed),
      burst_download: Number(formData.burst_download),
      burst_upload: Number(formData.burst_upload),
      burst_threshold_dl: Number(formData.burst_threshold_dl),
      burst_threshold_ul: Number(formData.burst_threshold_ul),
      burst_time: Number(formData.burst_time),
      daily_quota: Number(formData.daily_quota),
      fup1_threshold: Number(formData.fup1_threshold),
      fup1_speed: Number(formData.fup1_speed),
      fup2_threshold: Number(formData.fup2_threshold),
      fup2_speed: Number(formData.fup2_speed),
      fup3_threshold: Number(formData.fup3_threshold),
      fup3_speed: Number(formData.fup3_speed),
      monthly_quota: Number(formData.monthly_quota),
      monthly_fup1_threshold: Number(formData.monthly_fup1_threshold),
      monthly_fup1_speed: Number(formData.monthly_fup1_speed),
      monthly_fup2_threshold: Number(formData.monthly_fup2_threshold),
      monthly_fup2_speed: Number(formData.monthly_fup2_speed),
      monthly_fup3_threshold: Number(formData.monthly_fup3_threshold),
      monthly_fup3_speed: Number(formData.monthly_fup3_speed),
      price: Number(formData.price),
    }

    if (publicIPMode === 'none') {
      payload.public_ip = ''
      payload.public_subnet = ''
      payload.public_gateway = ''
      payload.ip_block_id = null
      payload.ip_allocation_id = null
    } else if (publicIPMode === 'block') {
      payload.public_ip = assignedIPs.join(',')
      payload.ip_block_id = selectedBlockId ? Number(selectedBlockId) : null
      const block = blocks.find(b => String(b.id) === selectedBlockId)
      if (block) {
        payload.public_gateway = block.gateway
        // Determine subnet prefix from number of assigned IPs
        const ipCount = assignedIPs.length
        let prefix = '/32'
        if (ipCount >= 13) prefix = '/28'       // 13 usable IPs
        else if (ipCount >= 5) prefix = '/29'    // 5 usable IPs
        else if (ipCount >= 2) prefix = '/30'    // 2 usable IPs
        else if (ipCount === 1) prefix = '/32'   // single IP
        payload.public_subnet = prefix
      }
    }
    // manual mode keeps formData values as-is

    // Go expects RFC3339 for *time.Time, not bare date strings
    payload.start_date = payload.start_date ? payload.start_date + 'T00:00:00Z' : null
    payload.expiry_date = payload.expiry_date ? payload.expiry_date + 'T00:00:00Z' : null

    saveMutation.mutate(payload)
  }

  const selectedBlock = blocks.find(b => String(b.id) === selectedBlockId)

  if (!isNew && isLoading) {
    return <div className="flex items-center justify-center h-64"><div className="animate-spin h-8 w-8 border-b-2 border-primary-600"></div></div>
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
          {isNew ? 'Add Bandwidth Customer' : `Edit: ${formData.name}`}
        </h1>
      </div>

      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {/* Customer Info */}
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white mb-3 pb-2 border-b border-gray-200 dark:border-gray-700">Customer Info</h3>
            <div className="space-y-3">
              <div>
                <label className="label">Name *</label>
                <input type="text" name="name" value={formData.name} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" required />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="label">Contact Person</label>
                  <input type="text" name="contact_person" value={formData.contact_person} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" />
                </div>
                <div>
                  <label className="label">Phone</label>
                  <input type="text" name="phone" value={formData.phone} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" />
                </div>
              </div>
              <div>
                <label className="label">Email</label>
                <input type="email" name="email" value={formData.email} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" />
              </div>
              <div>
                <label className="label">Address</label>
                <input type="text" name="address" value={formData.address} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" />
              </div>
              <div>
                <label className="label">Notes</label>
                <textarea name="notes" value={formData.notes} onChange={handleChange} rows={2} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" />
              </div>
            </div>
          </div>

          {/* Connection */}
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white mb-3 pb-2 border-b border-gray-200 dark:border-gray-700">Connection (Private Link)</h3>
            <p className="text-xs text-gray-400 dark:text-gray-500 mb-3">Private IP given to customer. Public IPs will be routed through this IP on MikroTik.</p>
            <div className="space-y-3">
              <div>
                <label className="label">Customer Private IP *</label>
                <input type="text" name="ip_address" value={formData.ip_address} onChange={handleChange} className="input font-mono dark:bg-gray-700 dark:text-white dark:border-gray-600" required placeholder="10.0.0.100" />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="label">Subnet Mask</label>
                  <input type="text" name="subnet_mask" value={formData.subnet_mask} onChange={handleChange} className="input font-mono dark:bg-gray-700 dark:text-white dark:border-gray-600" />
                </div>
                <div>
                  <label className="label">Gateway</label>
                  <input type="text" name="gateway" value={formData.gateway} onChange={handleChange} className="input font-mono dark:bg-gray-700 dark:text-white dark:border-gray-600" />
                </div>
              </div>
              <div>
                <label className="label">NAS / Router *</label>
                <select name="nas_id" value={formData.nas_id} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" required>
                  <option value="">Select NAS</option>
                  {nasList.map(nas => <option key={nas.id} value={nas.id}>{nas.name} ({nas.ip_address})</option>)}
                </select>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="label">Interface</label>
                  {mikrotikInterfaces.length > 0 ? (
                    <select name="interface" value={formData.interface} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600">
                      <option value="">Select Interface</option>
                      {mikrotikInterfaces.filter(i => i.disabled !== 'true').map(i => (
                        <option key={i.name} value={i.name}>{i.name} ({i.type}{i.running === 'true' ? '' : ' - down'})</option>
                      ))}
                    </select>
                  ) : (
                    <input type="text" name="interface" value={formData.interface} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" placeholder="ether1" />
                  )}
                </div>
                <div>
                  <label className="label">VLAN ID</label>
                  <input type="number" name="vlan_id" value={formData.vlan_id || ''} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" placeholder="100" min="1" max="4094" />
                  {formData.vlan_id > 0 && formData.interface && (
                    <p className="text-xs text-blue-500 dark:text-blue-400 mt-1">Will create vlan{formData.vlan_id} on {formData.interface}</p>
                  )}
                </div>
              </div>
              {!isNew && (
                <div>
                  <label className="label">Queue Name</label>
                  <input type="text" value={formData.queue_name} className="input bg-gray-100 dark:bg-gray-600 dark:text-gray-300 dark:border-gray-600 cursor-not-allowed" disabled />
                </div>
              )}
            </div>
          </div>

          {/* Public IP — Block-Based or Manual */}
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
            <div className="flex items-center justify-between mb-3 pb-2 border-b border-gray-200 dark:border-gray-700">
              <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Public IP</h3>
              <select
                value={publicIPMode}
                onChange={(e) => { setPublicIPMode(e.target.value); if (e.target.value === 'none') { setSelectedBlockId(''); setSelectedIP(''); } }}
                className="text-xs border rounded px-2 py-1 dark:bg-gray-700 dark:text-white dark:border-gray-600"
              >
                <option value="none">Disabled</option>
                <option value="block">From IP Block</option>
                <option value="manual">Manual Entry</option>
              </select>
            </div>
            {publicIPMode === 'block' ? (
              <div className="space-y-3">
                <div>
                  <div className="flex items-center justify-between mb-1">
                    <label className="label mb-0">IP Block</label>
                    <button type="button" onClick={() => setShowAddBlock(!showAddBlock)}
                      className="text-xs text-primary-600 hover:text-primary-700 dark:text-primary-400 font-medium">
                      {showAddBlock ? 'Cancel' : '+ Add Block'}
                    </button>
                  </div>
                  {showAddBlock ? (
                    <div className="border border-primary-200 dark:border-primary-800 rounded-lg p-3 bg-primary-50 dark:bg-primary-900/20 space-y-2">
                      <div className="grid grid-cols-2 gap-2">
                        <div>
                          <label className="text-xs text-gray-600 dark:text-gray-400">Name *</label>
                          <input type="text" value={newBlock.name} onChange={(e) => setNewBlock(p => ({ ...p, name: e.target.value }))}
                            className="input text-sm dark:bg-gray-700 dark:text-white dark:border-gray-600" placeholder="Office Block A" />
                        </div>
                        <div>
                          <label className="text-xs text-gray-600 dark:text-gray-400">CIDR *</label>
                          <input type="text" value={newBlock.cidr} onChange={(e) => setNewBlock(p => ({ ...p, cidr: e.target.value }))}
                            className="input text-sm font-mono dark:bg-gray-700 dark:text-white dark:border-gray-600" placeholder="185.111.161.0/28" />
                        </div>
                      </div>
                      <div>
                        <label className="text-xs text-gray-600 dark:text-gray-400">Gateway (optional)</label>
                        <input type="text" value={newBlock.gateway} onChange={(e) => setNewBlock(p => ({ ...p, gateway: e.target.value }))}
                          className="input text-sm font-mono dark:bg-gray-700 dark:text-white dark:border-gray-600" placeholder="185.111.161.1" />
                      </div>
                      <button type="button" onClick={() => createBlockMutation.mutate(newBlock)}
                        disabled={createBlockMutation.isLoading || !newBlock.name || !newBlock.cidr}
                        className="btn btn-primary btn-sm w-full">
                        {createBlockMutation.isLoading ? 'Creating...' : 'Create Block'}
                      </button>
                    </div>
                  ) : (
                    <select
                      value={selectedBlockId}
                      onChange={(e) => { setSelectedBlockId(e.target.value); setSelectedIP(''); }}
                      className="input dark:bg-gray-700 dark:text-white dark:border-gray-600"
                    >
                      <option value="">{blocks.filter(b => b.is_active).length === 0 ? 'No blocks — click "+ Add Block" above' : 'Select IP Block'}</option>
                      {blocks.filter(b => b.is_active).map(b => (
                        <option key={b.id} value={b.id}>{b.name} — {b.cidr} ({b.total_ips - b.used_ips} free)</option>
                      ))}
                    </select>
                  )}
                </div>
                {selectedBlock && (
                  <div className="text-xs text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-700 rounded p-2">
                    <span className="font-medium">CIDR:</span> {selectedBlock.cidr} &bull;{' '}
                    <span className="font-medium">Gateway:</span> {selectedBlock.gateway} &bull;{' '}
                    <span className="font-medium">Used:</span> {selectedBlock.used_ips}/{selectedBlock.total_ips}
                  </div>
                )}
                {/* Assigned IPs list */}
                {assignedIPs.length > 0 && (
                  <div>
                    <div className="flex items-center justify-between">
                      <label className="label mb-0">Assigned IPs ({assignedIPs.length})</label>
                      <button type="button" onClick={() => setAssignedIPs([])}
                        className="text-xs text-red-500 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 font-medium">Clear All</button>
                    </div>
                    <div className="flex flex-wrap gap-1.5">
                      {assignedIPs.map(ip => (
                        <span key={ip} className="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300 text-xs font-mono border border-green-200 dark:border-green-800">
                          {ip}
                          <button type="button" onClick={() => setAssignedIPs(prev => prev.filter(i => i !== ip))}
                            className="ml-0.5 text-green-600 dark:text-green-400 hover:text-red-600 dark:hover:text-red-400 font-bold">&times;</button>
                        </span>
                      ))}
                    </div>
                  </div>
                )}
                {/* Add IPs */}
                {selectedBlockId && (() => {
                  const freeIPs = availableIPs.filter(a => !assignedIPs.includes(a.ip_address))
                  const subnetOptions = [
                    { label: '/30 (2 IPs)', count: 2 },
                    { label: '/29 (5 IPs)', count: 5 },
                    { label: '/28 (13 IPs)', count: 13 },
                  ]
                  return (
                    <div className="space-y-2">
                      {/* Quick assign by subnet size */}
                      <div>
                        <label className="label">Quick Assign</label>
                        <div className="flex flex-wrap gap-1.5">
                          {subnetOptions.map(opt => (
                            <button key={opt.label} type="button"
                              disabled={freeIPs.length < opt.count}
                              onClick={() => {
                                const toAdd = freeIPs.slice(0, opt.count).map(a => a.ip_address)
                                setAssignedIPs(prev => [...prev, ...toAdd])
                              }}
                              className="px-2.5 py-1 text-xs font-medium rounded-md border border-blue-300 dark:border-blue-700 bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 hover:bg-blue-100 dark:hover:bg-blue-800/40 disabled:opacity-40 disabled:cursor-not-allowed"
                            >+ {opt.label}</button>
                          ))}
                          {freeIPs.length > 0 && (
                            <button type="button"
                              onClick={() => {
                                const toAdd = freeIPs.map(a => a.ip_address)
                                setAssignedIPs(prev => [...prev, ...toAdd])
                              }}
                              className="px-2.5 py-1 text-xs font-medium rounded-md border border-orange-300 dark:border-orange-700 bg-orange-50 dark:bg-orange-900/30 text-orange-700 dark:text-orange-300 hover:bg-orange-100 dark:hover:bg-orange-800/40"
                            >+ All ({freeIPs.length})</button>
                          )}
                        </div>
                        <p className="text-xs text-gray-400 dark:text-gray-500 mt-1">{freeIPs.length} IPs available in pool</p>
                      </div>
                      {/* Single IP add */}
                      <div>
                        <label className="label">Or pick individually</label>
                        <div className="flex gap-2">
                          <select
                            value={selectedIP}
                            onChange={(e) => setSelectedIP(e.target.value)}
                            className="input font-mono dark:bg-gray-700 dark:text-white dark:border-gray-600 flex-1 text-sm"
                          >
                            <option value="">Select IP</option>
                            {freeIPs.map(a => (
                              <option key={a.ip_address} value={a.ip_address}>{a.ip_address}</option>
                            ))}
                          </select>
                          <button type="button"
                            disabled={!selectedIP}
                            onClick={() => {
                              if (selectedIP && !assignedIPs.includes(selectedIP)) {
                                setAssignedIPs(prev => [...prev, selectedIP])
                                setSelectedIP('')
                              }
                            }}
                            className="btn btn-primary btn-sm px-3 whitespace-nowrap disabled:opacity-50"
                          >+ Add</button>
                        </div>
                      </div>
                    </div>
                  )
                })()}
              </div>
            ) : publicIPMode === 'manual' ? (
              <div className="space-y-3">
                <div>
                  <label className="label">Public IP</label>
                  <input type="text" name="public_ip" value={formData.public_ip} onChange={handleChange} className="input font-mono dark:bg-gray-700 dark:text-white dark:border-gray-600" />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="label">Subnet</label>
                    <input type="text" name="public_subnet" value={formData.public_subnet} onChange={handleChange} className="input font-mono dark:bg-gray-700 dark:text-white dark:border-gray-600" />
                  </div>
                  <div>
                    <label className="label">Gateway</label>
                    <input type="text" name="public_gateway" value={formData.public_gateway} onChange={handleChange} className="input font-mono dark:bg-gray-700 dark:text-white dark:border-gray-600" />
                  </div>
                </div>
              </div>
            ) : (
              <p className="text-sm text-gray-400 dark:text-gray-500">No public IP configured</p>
            )}
          </div>

          {/* Speed */}
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white mb-3 pb-2 border-b border-gray-200 dark:border-gray-700">Speed</h3>
            <div className="space-y-3">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="label">Download (kb)</label>
                  <input type="number" name="download_speed" value={formData.download_speed || ''} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" min="0" />
                </div>
                <div>
                  <label className="label">Upload (kb)</label>
                  <input type="number" name="upload_speed" value={formData.upload_speed || ''} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" min="0" />
                </div>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="label">CDN Download (kb)</label>
                  <input type="number" name="cdn_download_speed" value={formData.cdn_download_speed || ''} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" min="0" />
                </div>
                <div>
                  <label className="label">CDN Upload (kb)</label>
                  <input type="number" name="cdn_upload_speed" value={formData.cdn_upload_speed || ''} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" min="0" />
                </div>
              </div>
              <div>
                <label className="label">Speed Source</label>
                <div className="flex gap-4 mt-1">
                  <label className="flex items-center gap-2 cursor-pointer">
                    <input type="radio" name="speed_source" value="queue" checked={formData.speed_source === 'queue'} onChange={handleChange} />
                    <span className="text-sm text-gray-700 dark:text-gray-300">Simple Queue</span>
                  </label>
                  <label className="flex items-center gap-2 cursor-pointer">
                    <input type="radio" name="speed_source" value="radius" checked={formData.speed_source === 'radius'} onChange={handleChange} />
                    <span className="text-sm text-gray-700 dark:text-gray-300">RADIUS (CoA)</span>
                  </label>
                </div>
                <p className="text-xs text-gray-400 dark:text-gray-500 mt-1">Queue = MikroTik simple queue. RADIUS = radcheck/radreply + CoA disconnect.</p>
              </div>
            </div>
          </div>

          {/* Burst */}
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
            <div className="flex items-center justify-between mb-3 pb-2 border-b border-gray-200 dark:border-gray-700">
              <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Burst / Shaping</h3>
              <label className="flex items-center gap-2 cursor-pointer">
                <input type="checkbox" name="burst_enabled" checked={formData.burst_enabled} onChange={handleChange} className="rounded" />
                <span className="text-xs text-gray-500 dark:text-gray-400">Enable</span>
              </label>
            </div>
            {formData.burst_enabled ? (
              <div className="space-y-3">
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="label">Burst Download (kb)</label>
                    <input type="number" name="burst_download" value={formData.burst_download || ''} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" min="0" />
                  </div>
                  <div>
                    <label className="label">Burst Upload (kb)</label>
                    <input type="number" name="burst_upload" value={formData.burst_upload || ''} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" min="0" />
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="label">Threshold DL (kb)</label>
                    <input type="number" name="burst_threshold_dl" value={formData.burst_threshold_dl || ''} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" min="0" />
                  </div>
                  <div>
                    <label className="label">Threshold UL (kb)</label>
                    <input type="number" name="burst_threshold_ul" value={formData.burst_threshold_ul || ''} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" min="0" />
                  </div>
                </div>
                <div>
                  <label className="label">Burst Time (seconds)</label>
                  <input type="number" name="burst_time" value={formData.burst_time || ''} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600 w-32" min="1" />
                </div>
              </div>
            ) : (
              <p className="text-sm text-gray-400 dark:text-gray-500">Burst disabled</p>
            )}
          </div>

          {/* FUP */}
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
            <div className="flex items-center justify-between mb-3 pb-2 border-b border-gray-200 dark:border-gray-700">
              <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Fair Usage Policy (FUP)</h3>
              <label className="flex items-center gap-2 cursor-pointer">
                <input type="checkbox" name="fup_enabled" checked={formData.fup_enabled} onChange={handleChange} className="rounded" />
                <span className="text-xs text-gray-500 dark:text-gray-400">Enable</span>
              </label>
            </div>
            {formData.fup_enabled ? (
              <div className="space-y-4">
                {/* Daily FUP */}
                <div>
                  <h4 className="text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase mb-2">Daily</h4>
                  <div className="mb-2">
                    <label className="label">Daily Quota (GB)</label>
                    <input type="number" name="daily_quota" value={formData.daily_quota || ''} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600 w-32" min="0" step="0.1" />
                  </div>
                  {[1, 2, 3].map(tier => (
                    <div key={tier} className="grid grid-cols-2 gap-2 mb-1">
                      <div>
                        <label className="label text-xs">FUP{tier} Threshold (%)</label>
                        <input type="number" name={`fup${tier}_threshold`} value={formData[`fup${tier}_threshold`] || ''} onChange={handleChange} className="input text-xs dark:bg-gray-700 dark:text-white dark:border-gray-600" min="0" max="100" />
                      </div>
                      <div>
                        <label className="label text-xs">Speed (kb)</label>
                        <input type="number" name={`fup${tier}_speed`} value={formData[`fup${tier}_speed`] || ''} onChange={handleChange} className="input text-xs dark:bg-gray-700 dark:text-white dark:border-gray-600" min="0" />
                      </div>
                    </div>
                  ))}
                </div>
                {/* Monthly FUP */}
                <div>
                  <h4 className="text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase mb-2">Monthly</h4>
                  <div className="mb-2">
                    <label className="label">Monthly Quota (GB)</label>
                    <input type="number" name="monthly_quota" value={formData.monthly_quota || ''} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600 w-32" min="0" step="0.1" />
                  </div>
                  {[1, 2, 3].map(tier => (
                    <div key={tier} className="grid grid-cols-2 gap-2 mb-1">
                      <div>
                        <label className="label text-xs">FUP{tier} Threshold (%)</label>
                        <input type="number" name={`monthly_fup${tier}_threshold`} value={formData[`monthly_fup${tier}_threshold`] || ''} onChange={handleChange} className="input text-xs dark:bg-gray-700 dark:text-white dark:border-gray-600" min="0" max="100" />
                      </div>
                      <div>
                        <label className="label text-xs">Speed (kb)</label>
                        <input type="number" name={`monthly_fup${tier}_speed`} value={formData[`monthly_fup${tier}_speed`] || ''} onChange={handleChange} className="input text-xs dark:bg-gray-700 dark:text-white dark:border-gray-600" min="0" />
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            ) : (
              <p className="text-sm text-gray-400 dark:text-gray-500">FUP disabled — no daily/monthly quota enforcement</p>
            )}
          </div>

          {/* Billing */}
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white mb-3 pb-2 border-b border-gray-200 dark:border-gray-700">Billing</h3>
            <div className="space-y-3">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="label">Price ($)</label>
                  <input type="number" name="price" value={formData.price || ''} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" min="0" step="0.01" />
                </div>
                <div>
                  <label className="label">Billing Cycle</label>
                  <select name="billing_cycle" value={formData.billing_cycle} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600">
                    <option value="monthly">Monthly</option>
                    <option value="quarterly">Quarterly</option>
                    <option value="yearly">Yearly</option>
                  </select>
                </div>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="label">Start Date</label>
                  <input type="date" name="start_date" value={formData.start_date} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" />
                </div>
                <div>
                  <label className="label">Expiry Date</label>
                  <input type="date" name="expiry_date" value={formData.expiry_date} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" />
                </div>
              </div>
              <label className="flex items-center gap-2 cursor-pointer">
                <input type="checkbox" name="auto_renew" checked={formData.auto_renew} onChange={handleChange} className="rounded" />
                <span className="text-sm text-gray-700 dark:text-gray-300">Auto-renew</span>
              </label>
            </div>
          </div>

          {/* Status */}
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white mb-3 pb-2 border-b border-gray-200 dark:border-gray-700">Status</h3>
            <div>
              <label className="label">Status</label>
              <select name="status" value={formData.status} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600">
                <option value="active">Active</option>
                <option value="suspended">Suspended</option>
                <option value="expired">Expired</option>
              </select>
            </div>
          </div>
        </div>

        {/* Save Button */}
        <div className="flex items-center gap-3">
          <button type="submit" disabled={saveMutation.isLoading} className="btn btn-primary">
            {saveMutation.isLoading ? 'Saving...' : (isNew ? 'Create Customer' : 'Save Changes')}
          </button>
          <button type="button" onClick={() => navigate('/bandwidth-manager')} className="btn">Cancel</button>
        </div>
      </form>
    </div>
  )
}
