import { useState, useMemo, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { resellerApi, permissionApi, nasApi, serviceApi } from '../services/api'
import { useAuthStore } from '../store/authStore'
import api from '../services/api'
import {
  useReactTable,
  getCoreRowModel,
  flexRender,
} from '@tanstack/react-table'
import {
  PlusIcon,
  PencilIcon,
  TrashIcon,
  XMarkIcon,
  XCircleIcon,
  BanknotesIcon,
  ArrowUpIcon,
  ArrowDownIcon,
  UserGroupIcon,
  ArrowRightOnRectangleIcon,
  EyeIcon,
  EyeSlashIcon,
  ServerIcon,
  CubeIcon,
  Cog6ToothIcon,
} from '@heroicons/react/24/outline'
import toast from 'react-hot-toast'
import clsx from 'clsx'

export default function Resellers() {
  const queryClient = useQueryClient()
  const [showModal, setShowModal] = useState(false)
  const [showTransferModal, setShowTransferModal] = useState(false)
  const [showWithdrawModal, setShowWithdrawModal] = useState(false)
  const [editingReseller, setEditingReseller] = useState(null)
  const [selectedReseller, setSelectedReseller] = useState(null)
  const [transferAmount, setTransferAmount] = useState('')
  const [withdrawAmount, setWithdrawAmount] = useState('')
  const [visiblePasswords, setVisiblePasswords] = useState({})
  const [activeTab, setActiveTab] = useState('general')
  const [assignedNAS, setAssignedNAS] = useState([])
  const [assignedServices, setAssignedServices] = useState([])
  const [serviceLimits, setServiceLimits] = useState([])

  const [formData, setFormData] = useState({
    username: '',
    password: '',
    fullname: '',
    email: '',
    phone: '',
    address: '',
    company: '',
    balance: '0',
    credit_limit: '0',
    discount: '0',
    is_active: true,
    parent_id: '',
    permission_group: '',
    notes: '',
    rebrand_enabled: false,
    customer_change_plan: false,
    custom_domain: '',
    wan_check_enabled: null,
    wan_check_icmp: true,
    wan_check_port: true,
    wan_check_port_number: 0,
  })

  const { data: resellers, isLoading } = useQuery({
    queryKey: ['resellers'],
    queryFn: () => resellerApi.list().then((r) => r.data.data),
  })

  const { data: permissionGroups } = useQuery({
    queryKey: ['permissionGroups'],
    queryFn: () => permissionApi.listGroups().then((r) => r.data.data || []),
  })

  // Fetch all NAS for assignment (admin view)
  const { data: allNAS } = useQuery({
    queryKey: ['allNAS'],
    queryFn: () => nasApi.list().then((r) => r.data.data || []),
    enabled: showModal && !!editingReseller,
  })

  // Fetch all services for assignment (admin view)
  const { data: allServices } = useQuery({
    queryKey: ['allServices'],
    queryFn: () => serviceApi.list().then((r) => r.data.data || []),
    enabled: showModal && !!editingReseller,
  })

  // Fetch assigned NAS for the reseller
  const { data: resellerAssignedNAS, refetch: refetchAssignedNAS } = useQuery({
    queryKey: ['resellerAssignedNAS', editingReseller?.id],
    queryFn: () => resellerApi.getAssignedNAS(editingReseller.id).then((r) => r.data.data || []),
    enabled: showModal && !!editingReseller,
  })

  // Fetch assigned services for the reseller
  const { data: resellerAssignedServices, refetch: refetchAssignedServices } = useQuery({
    queryKey: ['resellerAssignedServices', editingReseller?.id],
    queryFn: () => resellerApi.getAssignedServices(editingReseller.id).then((r) => r.data.data || []),
    enabled: showModal && !!editingReseller,
  })

  // Fetch service limits for the reseller
  const { data: resellerServiceLimits, refetch: refetchServiceLimits } = useQuery({
    queryKey: ['resellerServiceLimits', editingReseller?.id],
    queryFn: () => resellerApi.getServiceLimits(editingReseller.id).then((r) => r.data.data || []),
    enabled: showModal && !!editingReseller,
  })

  // Update local state when data is fetched
  useEffect(() => {
    if (resellerAssignedNAS) {
      setAssignedNAS(resellerAssignedNAS.filter(n => n.assigned).map(n => n.id))
    }
  }, [resellerAssignedNAS])

  useEffect(() => {
    if (resellerAssignedServices) {
      setAssignedServices(resellerAssignedServices.map(s => ({
        service_id: s.id,
        enabled: s.is_enabled || false,
        custom_price: s.custom_price != null ? String(s.custom_price) : '',
        custom_day_price: s.custom_day_price != null ? String(s.custom_day_price) : '',
      })))
    }
  }, [resellerAssignedServices])

  useEffect(() => {
    if (resellerServiceLimits) {
      setServiceLimits(resellerServiceLimits.map(s => ({
        service_id: s.service_id,
        service_name: s.service_name,
        max_subscribers: s.max_subscribers || 0,
        current_count: s.current_count || 0,
      })))
    }
  }, [resellerServiceLimits])

  const saveMutation = useMutation({
    mutationFn: (data) =>
      editingReseller
        ? resellerApi.update(editingReseller.id, data)
        : resellerApi.create(data),
    onSuccess: () => {
      toast.success(editingReseller ? 'Reseller updated' : 'Reseller created')
      queryClient.invalidateQueries(['resellers'])
      closeModal()
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to save'),
  })

  const deleteMutation = useMutation({
    mutationFn: (id) => resellerApi.delete(id),
    onSuccess: () => {
      toast.success('Reseller deleted')
      queryClient.invalidateQueries(['resellers'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to delete'),
  })

  const permanentDeleteMutation = useMutation({
    mutationFn: (id) => resellerApi.permanentDelete(id),
    onSuccess: () => {
      toast.success('Reseller permanently deleted. Username can be reused.')
      queryClient.invalidateQueries(['resellers'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to permanently delete'),
  })

  const transferMutation = useMutation({
    mutationFn: ({ id, amount }) => resellerApi.transfer(id, { amount: parseFloat(amount) }),
    onSuccess: () => {
      toast.success('Balance transferred successfully')
      queryClient.invalidateQueries(['resellers'])
      setShowTransferModal(false)
      setTransferAmount('')
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Transfer failed'),
  })

  const withdrawMutation = useMutation({
    mutationFn: ({ id, amount }) => resellerApi.withdraw(id, { amount: parseFloat(amount) }),
    onSuccess: () => {
      toast.success('Balance withdrawn successfully')
      queryClient.invalidateQueries(['resellers'])
      setShowWithdrawModal(false)
      setWithdrawAmount('')
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Withdrawal failed'),
  })

  const saveNASAssignmentsMutation = useMutation({
    mutationFn: ({ id, nasIds }) => resellerApi.updateAssignedNAS(id, nasIds),
    onSuccess: () => {
      toast.success('NAS assignments updated')
      refetchAssignedNAS()
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to update NAS assignments'),
  })

  const saveServiceAssignmentsMutation = useMutation({
    mutationFn: ({ id, services }) => resellerApi.updateAssignedServices(id, services),
    onSuccess: () => {
      toast.success('Service assignments updated')
      refetchAssignedServices()
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to update service assignments'),
  })

  const saveServiceLimitsMutation = useMutation({
    mutationFn: ({ id, limits }) => resellerApi.setServiceLimits(id, limits),
    onSuccess: () => {
      toast.success('Service limits updated')
      refetchServiceLimits()
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to update service limits'),
  })

  const impersonateMutation = useMutation({
    mutationFn: (id) => resellerApi.getImpersonateToken(id),
    onSuccess: (response) => {
      const { token } = response.data
      // Open new tab with impersonation token
      const newWindow = window.open(`/impersonate?token=${token}`, '_blank')
      if (newWindow) {
        toast.success('Opening reseller session in new tab...')
      } else {
        toast.error('Popup blocked! Please allow popups for this site.')
      }
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to get impersonation token'),
  })

  const togglePasswordVisibility = (id) => {
    setVisiblePasswords(prev => ({ ...prev, [id]: !prev[id] }))
  }

  const openModal = (reseller = null) => {
    if (reseller) {
      setEditingReseller(reseller)
      setFormData({
        username: reseller.user?.username || reseller.username || '',
        password: '',
        fullname: reseller.user?.full_name || reseller.fullname || '',
        email: reseller.user?.email || reseller.email || '',
        phone: reseller.user?.phone || reseller.phone || '',
        address: reseller.address || '',
        company: reseller.name || reseller.company || '',
        balance: reseller.balance || '0',
        credit_limit: reseller.credit || reseller.credit_limit || '0',
        discount: reseller.discount || '0',
        is_active: reseller.is_active ?? true,
        parent_id: reseller.parent_id || '',
        permission_group: reseller.permission_group || '',
        notes: reseller.notes || '',
        rebrand_enabled: reseller.rebrand_enabled || false,
        customer_change_plan: reseller.customer_change_plan || false,
        custom_domain: reseller.custom_domain || '',
        wan_check_enabled: reseller.wan_check_enabled ?? null,
        wan_check_icmp: reseller.wan_check_icmp ?? true,
        wan_check_port: reseller.wan_check_port ?? true,
        wan_check_port_number: reseller.wan_check_port_number || 0,
      })
    } else {
      setEditingReseller(null)
      setFormData({
        username: '',
        password: '',
        fullname: '',
        email: '',
        phone: '',
        address: '',
        company: '',
        balance: '0',
        credit_limit: '0',
        discount: '0',
        is_active: true,
        parent_id: '',
        permission_group: '',
        notes: '',
        rebrand_enabled: false,
        customer_change_plan: false,
        custom_domain: '',
        wan_check_enabled: null,
        wan_check_icmp: true,
        wan_check_port: true,
        wan_check_port_number: 0,
      })
    }
    setShowModal(true)
  }

  const closeModal = () => {
    setShowModal(false)
    setEditingReseller(null)
    setActiveTab('general')
    setAssignedNAS([])
    setAssignedServices([])
    setServiceLimits([])
  }

  const handleSubmit = (e) => {
    e.preventDefault()
    const data = {
      ...formData,
      balance: parseFloat(formData.balance) || 0,
      credit_limit: parseFloat(formData.credit_limit) || 0,
      discount: parseFloat(formData.discount) || 0,
      parent_id: formData.parent_id ? parseInt(formData.parent_id) : null,
      permission_group: formData.permission_group ? parseInt(formData.permission_group) : null,
    }
    if (!data.password && editingReseller) delete data.password
    saveMutation.mutate(data)
  }

  const handleChange = (e) => {
    const { name, value, type, checked } = e.target
    setFormData((prev) => ({
      ...prev,
      [name]: type === 'checkbox' ? checked : value,
    }))
  }

  const handleNASToggle = (nasId) => {
    setAssignedNAS(prev =>
      prev.includes(nasId)
        ? prev.filter(id => id !== nasId)
        : [...prev, nasId]
    )
  }

  const handleServiceToggle = (serviceId) => {
    setAssignedServices(prev => {
      const existing = prev.find(s => s.service_id === serviceId)
      if (existing) {
        return prev.map(s =>
          s.service_id === serviceId
            ? { ...s, enabled: !s.enabled }
            : s
        )
      }
      return [...prev, { service_id: serviceId, enabled: true, custom_price: '', custom_day_price: '' }]
    })
  }

  const handleServicePriceChange = (serviceId, field, value) => {
    setAssignedServices(prev => {
      const exists = prev.find(s => s.service_id === serviceId)
      if (exists) {
        return prev.map(s =>
          s.service_id === serviceId
            ? { ...s, [field]: value }
            : s
        )
      }
      // If service doesn't exist in the list, add it
      return [...prev, { service_id: serviceId, enabled: false, custom_price: '', custom_day_price: '', [field]: value }]
    })
  }

  const saveNASAssignments = () => {
    if (editingReseller) {
      saveNASAssignmentsMutation.mutate({ id: editingReseller.id, nasIds: assignedNAS })
    }
  }

  const saveServiceAssignments = () => {
    if (editingReseller) {
      const services = assignedServices
        .filter(s => s.enabled)
        .map(s => ({
          service_id: s.service_id,
          custom_price: s.custom_price ? parseFloat(s.custom_price) : null,
          custom_day_price: s.custom_day_price ? parseFloat(s.custom_day_price) : null,
        }))
      saveServiceAssignmentsMutation.mutate({ id: editingReseller.id, services })
    }
  }

  const handleServiceLimitChange = (serviceId, value) => {
    setServiceLimits(prev => prev.map(s =>
      s.service_id === serviceId ? { ...s, max_subscribers: parseInt(value) || 0 } : s
    ))
  }

  const saveServiceLimits = () => {
    if (editingReseller) {
      const limits = serviceLimits
        .filter(s => s.max_subscribers > 0)
        .map(s => ({ service_id: s.service_id, max_subscribers: s.max_subscribers }))
      saveServiceLimitsMutation.mutate({ id: editingReseller.id, limits })
    }
  }

  const columns = useMemo(
    () => [
      {
        accessorKey: 'username',
        header: 'Username',
        cell: ({ row }) => (
          <div>
            <div className="font-semibold text-[12px]">{row.original.user?.username || row.original.username}</div>
            <div className="text-[11px] text-gray-500 dark:text-gray-400">{row.original.name || row.original.company}</div>
          </div>
        ),
      },
      {
        accessorKey: 'password',
        header: 'Password',
        cell: ({ row }) => {
          const password = row.original.user?.password_plain
          const isVisible = visiblePasswords[row.original.id]
          return (
            <div className="flex items-center gap-1">
              <span className="font-mono text-[12px]">
                {password ? (isVisible ? password : '********') : '-'}
              </span>
              {password && (
                <button
                  onClick={() => togglePasswordVisibility(row.original.id)}
                  className="btn btn-xs"
                  title={isVisible ? 'Hide' : 'Show'}
                  style={{ padding: '0 2px', minHeight: 16 }}
                >
                  {isVisible ? <EyeSlashIcon className="w-3 h-3" /> : <EyeIcon className="w-3 h-3" />}
                </button>
              )}
            </div>
          )
        },
      },
      {
        accessorKey: 'contact',
        header: 'Contact',
        cell: ({ row }) => (
          <div className="text-[12px]">
            <div>{row.original.user?.email || row.original.email}</div>
            <div className="text-gray-500 dark:text-gray-400">{row.original.user?.phone || row.original.phone}</div>
          </div>
        ),
      },
      {
        accessorKey: 'balance',
        header: 'Balance',
        cell: ({ row }) => (
          <span className={clsx('font-semibold text-[12px]', row.original.balance >= 0 ? 'text-green-600' : 'text-red-600')}>
            ${row.original.balance?.toFixed(2)}
          </span>
        ),
      },
      {
        accessorKey: 'credit',
        header: 'Credit',
        cell: ({ row }) => <span className="text-[12px]">${(row.original.credit || 0).toFixed(2)}</span>,
      },
      {
        accessorKey: 'subscriber_count',
        header: 'Subs',
        cell: ({ row }) => <span className="text-[12px]">{row.original.subscriber_count || 0}</span>,
      },
      {
        accessorKey: 'is_active',
        header: 'Status',
        cell: ({ row }) => (
          <div className="flex flex-wrap items-center gap-1">
            <span className={clsx(row.original.is_active ? 'badge-success' : 'badge-gray')}>
              {row.original.is_active ? 'Active' : 'Inactive'}
            </span>
            {row.original.rebrand_enabled && (
              <span className="badge-info">Rebrand</span>
            )}
            {row.original.custom_domain && (
              <span className="badge-purple" title={row.original.custom_domain}>
                {row.original.custom_domain}
              </span>
            )}
          </div>
        ),
      },
      {
        id: 'actions',
        header: 'Actions',
        cell: ({ row }) => (
          <div className="flex items-center gap-0.5">
            <button
              onClick={() => {
                if (confirm(`Login as ${row.original.user?.username || row.original.username}?`)) {
                  impersonateMutation.mutate(row.original.id)
                }
              }}
              className="btn btn-xs"
              title="Login as Reseller"
            >
              <ArrowRightOnRectangleIcon className="w-3.5 h-3.5" />
            </button>
            <button
              onClick={() => {
                setSelectedReseller(row.original)
                setShowTransferModal(true)
              }}
              className="btn btn-xs btn-success"
              title="Transfer Balance"
            >
              <ArrowUpIcon className="w-3.5 h-3.5" />
            </button>
            <button
              onClick={() => {
                setSelectedReseller(row.original)
                setShowWithdrawModal(true)
              }}
              className="btn btn-xs"
              title="Withdraw Balance"
            >
              <ArrowDownIcon className="w-3.5 h-3.5" />
            </button>
            <button
              onClick={() => openModal(row.original)}
              className="btn btn-xs btn-primary"
              title="Edit"
            >
              <PencilIcon className="w-3.5 h-3.5" />
            </button>
            <button
              onClick={() => {
                if (confirm('Are you sure you want to delete this reseller?')) {
                  deleteMutation.mutate(row.original.id)
                }
              }}
              className="btn btn-xs btn-danger"
              title="Delete (can restore)"
            >
              <TrashIcon className="w-3.5 h-3.5" />
            </button>
            <button
              onClick={() => {
                if (confirm('PERMANENT DELETE\n\nThis will permanently remove the reseller from the database.\nThe username can be reused after this.\n\nTHIS CANNOT BE UNDONE!\n\nAre you sure?')) {
                  permanentDeleteMutation.mutate(row.original.id)
                }
              }}
              className="btn btn-xs btn-danger"
              title="Permanent Delete (cannot undo)"
            >
              <XCircleIcon className="w-3.5 h-3.5" />
            </button>
          </div>
        ),
      },
    ],
    [deleteMutation, permanentDeleteMutation, impersonateMutation, visiblePasswords]
  )

  const table = useReactTable({
    data: resellers || [],
    columns,
    getCoreRowModel: getCoreRowModel(),
  })

  return (
    <div>
      {/* Toolbar */}
      <div className="wb-toolbar" style={{ justifyContent: 'space-between' }}>
        <span className="text-[13px] font-semibold">Resellers</span>
        <button onClick={() => openModal()} className="btn btn-sm btn-primary">
          <PlusIcon className="w-3.5 h-3.5 mr-1" />
          Add Reseller
        </button>
      </div>

      {/* Summary Stats */}
      <div className="flex items-center gap-3 px-2 py-1.5 border-b border-[#a0a0a0] dark:border-[#374151] bg-[#f8f8f8] dark:bg-[#1f2937]">
        <div className="stat-card" style={{ padding: '4px 10px', minWidth: 0 }}>
          <div className="text-[11px] text-gray-500 dark:text-gray-400">Total</div>
          <div className="text-[14px] font-bold dark:text-gray-200">{resellers?.length || 0}</div>
        </div>
        <div className="stat-card" style={{ padding: '4px 10px', minWidth: 0 }}>
          <div className="text-[11px] text-gray-500 dark:text-gray-400">Balance</div>
          <div className="text-[14px] font-bold text-green-600 dark:text-green-400">
            ${resellers?.reduce((sum, r) => sum + (r.balance || 0), 0).toFixed(2)}
          </div>
        </div>
        <div className="stat-card" style={{ padding: '4px 10px', minWidth: 0 }}>
          <div className="text-[11px] text-gray-500 dark:text-gray-400">Active</div>
          <div className="text-[14px] font-bold dark:text-gray-200">{resellers?.filter((r) => r.is_active).length || 0}</div>
        </div>
        <div className="stat-card" style={{ padding: '4px 10px', minWidth: 0 }}>
          <div className="text-[11px] text-gray-500 dark:text-gray-400">Subscribers</div>
          <div className="text-[14px] font-bold dark:text-gray-200">{resellers?.reduce((sum, r) => sum + (r.subscriber_count || 0), 0)}</div>
        </div>
      </div>

      {/* Table */}
      <div className="table-container" style={{ borderTop: 0 }}>
        <table className="table">
          <thead>
            {table.getHeaderGroups().map((headerGroup) => (
              <tr key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <th key={header.id}>
                    {flexRender(header.column.columnDef.header, header.getContext())}
                  </th>
                ))}
              </tr>
            ))}
          </thead>
          <tbody>
            {isLoading ? (
              <tr>
                <td colSpan={columns.length} className="text-center py-4 text-[12px] text-gray-500">
                  Loading resellers...
                </td>
              </tr>
            ) : table.getRowModel().rows.length === 0 ? (
              <tr>
                <td colSpan={columns.length} className="text-center py-4 text-[12px] text-gray-500">
                  No resellers found
                </td>
              </tr>
            ) : (
              table.getRowModel().rows.map((row) => (
                <tr key={row.id}>
                  {row.getVisibleCells().map((cell) => (
                    <td key={cell.id}>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Statusbar */}
      <div className="wb-statusbar">
        <span>{resellers?.length || 0} reseller(s)</span>
        <span>{resellers?.filter(r => r.is_active).length || 0} active</span>
      </div>

      {/* Add/Edit Modal */}
      {showModal && (
        <div className="modal-overlay">
          <div className="modal modal-lg" style={{ maxHeight: '90vh', overflow: 'hidden', display: 'flex', flexDirection: 'column' }}>
            <div className="modal-header">
              <span>{editingReseller ? 'Edit Reseller' : 'Add Reseller'}</span>
              <button onClick={closeModal} style={{ background: 'none', border: 'none', color: 'white', cursor: 'pointer', fontSize: 16 }}>X</button>
            </div>

            {/* Tabs - only show when editing */}
            {editingReseller && (
              <div className="flex items-center border-b border-[#a0a0a0] bg-[#f0f0f0]">
                <button
                  type="button"
                  onClick={() => setActiveTab('general')}
                  className={clsx('wb-tab', activeTab === 'general' && 'active')}
                >
                  <Cog6ToothIcon className="w-3.5 h-3.5 inline mr-1" />
                  General
                </button>
                <button
                  type="button"
                  onClick={() => setActiveTab('nas')}
                  className={clsx('wb-tab', activeTab === 'nas' && 'active')}
                >
                  <ServerIcon className="w-3.5 h-3.5 inline mr-1" />
                  NAS ({assignedNAS.length})
                </button>
                <button
                  type="button"
                  onClick={() => setActiveTab('services')}
                  className={clsx('wb-tab', activeTab === 'services' && 'active')}
                >
                  <CubeIcon className="w-3.5 h-3.5 inline mr-1" />
                  Services ({assignedServices.filter(s => s.enabled).length})
                </button>
                <button
                  type="button"
                  onClick={() => setActiveTab('limits')}
                  className={clsx('wb-tab', activeTab === 'limits' && 'active')}
                >
                  <UserGroupIcon className="w-3.5 h-3.5 inline mr-1" />
                  Service Limits
                </button>
              </div>
            )}

            {/* General Tab */}
            {(activeTab === 'general' || !editingReseller) && (
              <form onSubmit={handleSubmit} className="modal-body" style={{ overflow: 'auto', flex: 1 }}>
                <div className="wb-group mb-3">
                  <div className="wb-group-title">Account</div>
                  <div className="wb-group-body">
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
                      <div>
                        <label className="label">Username</label>
                        <input type="text" name="username" value={formData.username} onChange={handleChange} className="input" required />
                      </div>
                      <div>
                        <label className="label">Password</label>
                        <input type="password" name="password" value={formData.password} onChange={handleChange} className="input"
                          placeholder={editingReseller ? 'Leave blank to keep current' : ''}
                          required={!editingReseller} />
                      </div>
                      <div>
                        <label className="label">Full Name</label>
                        <input type="text" name="fullname" value={formData.fullname} onChange={handleChange} className="input" />
                      </div>
                      <div>
                        <label className="label">Company</label>
                        <input type="text" name="company" value={formData.company} onChange={handleChange} className="input" />
                      </div>
                      <div>
                        <label className="label">Email</label>
                        <input type="email" name="email" value={formData.email} onChange={handleChange} className="input" />
                      </div>
                      <div>
                        <label className="label">Phone</label>
                        <input type="tel" name="phone" value={formData.phone} onChange={handleChange} className="input" />
                      </div>
                    </div>
                    <div className="mt-2">
                      <label className="label">Address</label>
                      <textarea name="address" value={formData.address} onChange={handleChange} className="input" rows={2} />
                    </div>
                  </div>
                </div>

                <div className="wb-group mb-3">
                  <div className="wb-group-title">Billing</div>
                  <div className="wb-group-body">
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 8 }}>
                      <div>
                        <label className="label">Initial Balance ($)</label>
                        <input type="number" name="balance" value={formData.balance} onChange={handleChange} className="input" step="0.01" disabled={!!editingReseller} />
                      </div>
                      <div>
                        <label className="label">Credit Limit ($)</label>
                        <input type="number" name="credit_limit" value={formData.credit_limit} onChange={handleChange} className="input" step="0.01" />
                      </div>
                      <div>
                        <label className="label">Discount (%)</label>
                        <input type="number" name="discount" value={formData.discount} onChange={handleChange} className="input" min="0" max="100" />
                      </div>
                    </div>
                  </div>
                </div>

                <div className="wb-group mb-3">
                  <div className="wb-group-title">Settings</div>
                  <div className="wb-group-body">
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
                      <div>
                        <label className="label">Parent Reseller</label>
                        <select name="parent_id" value={formData.parent_id} onChange={handleChange} className="input">
                          <option value="">No Parent (Direct)</option>
                          {resellers
                            ?.filter((r) => r.id !== editingReseller?.id)
                            .map((r) => (
                              <option key={r.id} value={r.id}>
                                {r.user?.username || r.username} - {r.name || r.company}
                              </option>
                            ))}
                        </select>
                      </div>
                      <div>
                        <label className="label">Permission Group</label>
                        <select name="permission_group" value={formData.permission_group} onChange={handleChange} className="input">
                          <option value="">No Permission Group</option>
                          {permissionGroups?.map((g) => (
                            <option key={g.id} value={g.id}>{g.name}</option>
                          ))}
                        </select>
                      </div>
                    </div>
                    <div className="mt-2">
                      <label className="label">Notes</label>
                      <textarea name="notes" value={formData.notes} onChange={handleChange} className="input" rows={2} />
                    </div>
                    <div className="mt-2 flex flex-col gap-2">
                      <label className="flex items-center gap-2 text-[12px]">
                        <input type="checkbox" name="is_active" checked={formData.is_active} onChange={handleChange} />
                        Active Reseller
                      </label>
                      <label className="flex items-center gap-2 text-[12px]">
                        <input type="checkbox" id="rebrand_enabled" checked={formData.rebrand_enabled || false}
                          onChange={e => setFormData(p => ({ ...p, rebrand_enabled: e.target.checked }))} />
                        Enable Rebranding
                      </label>
                      <label className="flex items-center gap-2 text-[12px]">
                        <input type="checkbox" id="customer_change_plan" checked={formData.customer_change_plan || false}
                          onChange={e => setFormData(p => ({ ...p, customer_change_plan: e.target.checked }))} />
                        Allow Customer Self-Service Plan Change
                      </label>
                    </div>
                    <div className="mt-2">
                      <label className="label">Custom Domain</label>
                      <input type="text" value={formData.custom_domain}
                        onChange={e => setFormData(p => ({ ...p, custom_domain: e.target.value.toLowerCase().trim() }))}
                        placeholder="portal.myisp.com" className="input font-mono" />
                      <div className="text-[11px] text-gray-500 mt-0.5">A record must point to this server's IP.</div>
                    </div>

                    {/* WAN Management Check */}
                    <div className="mt-3 p-2.5 rounded-lg border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-700/50">
                      <div className="text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-2">WAN Management Check</div>
                      <div className="mb-2">
                        <select
                          value={formData.wan_check_enabled === null ? 'global' : formData.wan_check_enabled ? 'enabled' : 'disabled'}
                          onChange={e => {
                            const v = e.target.value
                            setFormData(p => ({
                              ...p,
                              wan_check_enabled: v === 'global' ? null : v === 'enabled',
                            }))
                          }}
                          className="input text-[12px]"
                        >
                          <option value="global">Follow Global Setting</option>
                          <option value="enabled">Enabled</option>
                          <option value="disabled">Disabled</option>
                        </select>
                      </div>
                      {formData.wan_check_enabled !== false && (
                        <div className="flex flex-col gap-1.5 pl-1">
                          <label className="flex items-center gap-2 text-[12px]">
                            <input type="checkbox" checked={formData.wan_check_icmp}
                              onChange={e => setFormData(p => ({ ...p, wan_check_icmp: e.target.checked }))} />
                            ICMP Ping Check
                          </label>
                          <label className="flex items-center gap-2 text-[12px]">
                            <input type="checkbox" checked={formData.wan_check_port}
                              onChange={e => setFormData(p => ({ ...p, wan_check_port: e.target.checked }))} />
                            WAN Port Check
                          </label>
                          <div className="mt-1">
                            <label className="text-[11px] text-gray-500">Custom Port (0 = use global)</label>
                            <input type="number" min="0" max="65535" value={formData.wan_check_port_number || 0}
                              onChange={e => setFormData(p => ({ ...p, wan_check_port_number: parseInt(e.target.value) || 0 }))}
                              className="input text-[12px] mt-0.5" placeholder="0" />
                          </div>
                        </div>
                      )}
                    </div>
                  </div>
                </div>

                <div className="modal-footer">
                  <button type="button" onClick={closeModal} className="btn btn-sm">Cancel</button>
                  <button type="submit" disabled={saveMutation.isLoading} className="btn btn-sm btn-primary">
                    {saveMutation.isLoading ? 'Saving...' : editingReseller ? 'Update' : 'Create'}
                  </button>
                </div>
              </form>
            )}

            {/* NAS Tab */}
            {activeTab === 'nas' && editingReseller && (
              <div className="modal-body" style={{ overflow: 'auto', flex: 1 }}>
                <p className="text-[12px] text-gray-600 dark:text-gray-400 mb-2">
                  Select the NAS devices this reseller can manage.
                </p>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 6 }} className="max-h-80 overflow-y-auto">
                  {allNAS?.map((nas) => (
                    <label
                      key={nas.id}
                      className={clsx(
                        'flex items-center gap-2 p-2 border cursor-pointer text-[12px]',
                        assignedNAS.includes(nas.id)
                          ? 'border-[#316AC5] bg-[#e8f0ff]'
                          : 'border-[#a0a0a0] bg-white'
                      )}
                      style={{ borderRadius: 2 }}
                    >
                      <input
                        type="checkbox"
                        checked={assignedNAS.includes(nas.id)}
                        onChange={() => handleNASToggle(nas.id)}
                      />
                      <div>
                        <div className="font-semibold">{nas.name}</div>
                        <div className="text-[11px] text-gray-500">{nas.ip_address}</div>
                      </div>
                    </label>
                  ))}
                </div>
                {(!allNAS || allNAS.length === 0) && (
                  <div className="text-center py-4 text-[12px] text-gray-500">No NAS devices found</div>
                )}
                <div className="modal-footer" style={{ marginTop: 8 }}>
                  <button type="button" onClick={closeModal} className="btn btn-sm">Cancel</button>
                  <button type="button" onClick={saveNASAssignments} disabled={saveNASAssignmentsMutation.isLoading} className="btn btn-sm btn-primary">
                    {saveNASAssignmentsMutation.isLoading ? 'Saving...' : 'Save NAS Assignments'}
                  </button>
                </div>
              </div>
            )}

            {/* Services Tab */}
            {activeTab === 'services' && editingReseller && (
              <div className="modal-body" style={{ overflow: 'auto', flex: 1 }}>
                <p className="text-[12px] text-gray-600 dark:text-gray-400 mb-2">
                  Select which services this reseller can sell. Set custom prices per service.
                </p>
                <div className="table-container max-h-80 overflow-y-auto">
                  <table className="table table-compact">
                    <thead className="sticky top-0">
                      <tr>
                        <th>Service</th>
                        <th>Base Price</th>
                        <th>Custom Price</th>
                        <th>Custom Day Price</th>
                      </tr>
                    </thead>
                    <tbody>
                      {allServices?.map((service) => {
                        const assignment = assignedServices.find(s => s.service_id === service.id)
                        const isEnabled = assignment?.enabled || false
                        return (
                          <tr key={service.id} className={clsx(isEnabled && 'bg-[#e8f0ff]')}>
                            <td>
                              <label className="flex items-center gap-2 cursor-pointer text-[12px]">
                                <input type="checkbox" checked={isEnabled} onChange={() => handleServiceToggle(service.id)} />
                                <span className="font-semibold">{service.name}</span>
                              </label>
                            </td>
                            <td className="text-[12px] text-gray-600 dark:text-gray-400">${service.price?.toFixed(2)}</td>
                            <td>
                              <input type="number" value={assignment?.custom_price || ''}
                                onChange={(e) => handleServicePriceChange(service.id, 'custom_price', e.target.value)}
                                placeholder="Same" className="input input-sm" style={{ width: 80 }} step="0.01" disabled={!isEnabled} />
                            </td>
                            <td>
                              <input type="number" value={assignment?.custom_day_price || ''}
                                onChange={(e) => handleServicePriceChange(service.id, 'custom_day_price', e.target.value)}
                                placeholder="Auto" className="input input-sm" style={{ width: 80 }} step="0.01" disabled={!isEnabled} />
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                </div>
                {(!allServices || allServices.length === 0) && (
                  <div className="text-center py-4 text-[12px] text-gray-500">No services found</div>
                )}
                <div className="modal-footer" style={{ marginTop: 8 }}>
                  <button type="button" onClick={closeModal} className="btn btn-sm">Cancel</button>
                  <button type="button" onClick={saveServiceAssignments} disabled={saveServiceAssignmentsMutation.isLoading} className="btn btn-sm btn-primary">
                    {saveServiceAssignmentsMutation.isLoading ? 'Saving...' : 'Save Service Assignments'}
                  </button>
                </div>
              </div>
            )}

            {/* Service Limits Tab */}
            {activeTab === 'limits' && editingReseller && (
              <div className="modal-body" style={{ overflow: 'auto', flex: 1 }}>
                <p className="text-[12px] text-gray-600 dark:text-gray-400 mb-2">
                  Set maximum subscriber limits per service for this reseller. 0 or empty = unlimited.
                </p>
                <div className="table-container max-h-80 overflow-y-auto">
                  <table className="table table-compact">
                    <thead className="sticky top-0">
                      <tr>
                        <th>Service</th>
                        <th>Current Subscribers</th>
                        <th>Max Limit</th>
                      </tr>
                    </thead>
                    <tbody>
                      {serviceLimits.map((svc) => {
                        const isOverLimit = svc.max_subscribers > 0 && svc.current_count >= svc.max_subscribers
                        return (
                          <tr key={svc.service_id} className={clsx(isOverLimit && 'bg-yellow-50 dark:bg-yellow-900/20')}>
                            <td className="text-[12px] font-semibold">{svc.service_name}</td>
                            <td className="text-[12px]">
                              <span className={clsx(isOverLimit ? 'text-red-600 font-bold' : '')}>
                                {svc.current_count}
                              </span>
                              <span className="text-gray-400"> / </span>
                              <span className="text-gray-500">{svc.max_subscribers > 0 ? svc.max_subscribers : '∞'}</span>
                              {isOverLimit && (
                                <span className="ml-1 text-[10px] text-yellow-600 dark:text-yellow-400 font-medium">
                                  (limit reached)
                                </span>
                              )}
                            </td>
                            <td>
                              <input
                                type="number"
                                value={svc.max_subscribers || ''}
                                onChange={(e) => handleServiceLimitChange(svc.service_id, e.target.value)}
                                placeholder="0 (unlimited)"
                                className="input input-sm"
                                style={{ width: 120 }}
                                min="0"
                              />
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                </div>
                {serviceLimits.length === 0 && (
                  <div className="text-center py-4 text-[12px] text-gray-500">No services found</div>
                )}
                <div className="modal-footer" style={{ marginTop: 8 }}>
                  <button type="button" onClick={closeModal} className="btn btn-sm">Cancel</button>
                  <button type="button" onClick={saveServiceLimits} disabled={saveServiceLimitsMutation.isLoading} className="btn btn-sm btn-primary">
                    {saveServiceLimitsMutation.isLoading ? 'Saving...' : 'Save Service Limits'}
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Transfer Modal */}
      {showTransferModal && selectedReseller && (
        <div className="modal-overlay">
          <div className="modal modal-sm">
            <div className="modal-header">
              <span>Transfer Balance</span>
              <button onClick={() => setShowTransferModal(false)} style={{ background: 'none', border: 'none', color: 'white', cursor: 'pointer', fontSize: 14 }}>X</button>
            </div>
            <div className="modal-body">
              <p className="text-[12px] text-gray-700 dark:text-gray-300 mb-1">
                Transfer to <strong>{selectedReseller.username}</strong>
              </p>
              <p className="text-[11px] text-gray-500 mb-3">
                Current Balance: <span className="font-semibold text-green-600">${selectedReseller.balance?.toFixed(2)}</span>
              </p>
              <label className="label">Amount ($)</label>
              <input type="number" value={transferAmount} onChange={(e) => setTransferAmount(e.target.value)}
                className="input" step="0.01" min="0.01" required />
            </div>
            <div className="modal-footer">
              <button onClick={() => setShowTransferModal(false)} className="btn btn-sm">Cancel</button>
              <button onClick={() => transferMutation.mutate({ id: selectedReseller.id, amount: transferAmount })}
                disabled={!transferAmount || transferMutation.isLoading} className="btn btn-sm btn-success">
                {transferMutation.isLoading ? 'Transferring...' : 'Transfer'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Withdraw Modal */}
      {showWithdrawModal && selectedReseller && (
        <div className="modal-overlay">
          <div className="modal modal-sm">
            <div className="modal-header">
              <span>Withdraw Balance</span>
              <button onClick={() => setShowWithdrawModal(false)} style={{ background: 'none', border: 'none', color: 'white', cursor: 'pointer', fontSize: 14 }}>X</button>
            </div>
            <div className="modal-body">
              <p className="text-[12px] text-gray-700 dark:text-gray-300 mb-1">
                Withdraw from <strong>{selectedReseller.username}</strong>
              </p>
              <p className="text-[11px] text-gray-500 mb-3">
                Current Balance: <span className="font-semibold text-green-600">${selectedReseller.balance?.toFixed(2)}</span>
              </p>
              <label className="label">Amount ($)</label>
              <input type="number" value={withdrawAmount} onChange={(e) => setWithdrawAmount(e.target.value)}
                className="input" step="0.01" min="0.01" max={selectedReseller.balance} required />
            </div>
            <div className="modal-footer">
              <button onClick={() => setShowWithdrawModal(false)} className="btn btn-sm">Cancel</button>
              <button onClick={() => withdrawMutation.mutate({ id: selectedReseller.id, amount: withdrawAmount })}
                disabled={!withdrawAmount || withdrawMutation.isLoading} className="btn btn-sm btn-danger">
                {withdrawMutation.isLoading ? 'Withdrawing...' : 'Withdraw'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
