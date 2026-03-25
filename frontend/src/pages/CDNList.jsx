import { useState, useMemo } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { cdnApi, nasApi } from '../services/api'
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
  GlobeAltIcon,
  ArrowPathIcon,
} from '@heroicons/react/24/outline'
import toast from 'react-hot-toast'
import clsx from 'clsx'

export default function CDNList() {
  const queryClient = useQueryClient()
  const [showModal, setShowModal] = useState(false)
  const [editingCDN, setEditingCDN] = useState(null)
  const [formData, setFormData] = useState({
    name: '',
    description: '',
    subnets: '',
    color: '#EF4444',
    nas_ids: '',
    is_active: true,
  })

  // Predefined colors for CDN (excluding blue #3B82F6 and green #22C55E used for download/upload)
  const presetColors = [
    '#EF4444', // Red
    '#F97316', // Orange
    '#F59E0B', // Amber
    '#8B5CF6', // Purple
    '#EC4899', // Pink
    '#06B6D4', // Cyan
    '#84CC16', // Lime
    '#6366F1', // Indigo
    '#14B8A6', // Teal
    '#D946EF', // Fuchsia
  ]

  const { data: cdns, isLoading } = useQuery({
    queryKey: ['cdns'],
    queryFn: () => cdnApi.list().then((r) => r.data.data),
  })

  const { data: nasList } = useQuery({
    queryKey: ['nas'],
    queryFn: () => nasApi.list().then((r) => r.data.data),
  })

  const saveMutation = useMutation({
    mutationFn: async (data) => {
      try {
        const response = editingCDN
          ? await cdnApi.update(editingCDN.id, data)
          : await cdnApi.create(data)
        return response
      } catch (err) {
        throw err
      }
    },
    onSuccess: () => {
      toast.success(editingCDN ? 'CDN updated' : 'CDN created')
      queryClient.invalidateQueries({ queryKey: ['cdns'] })
      closeModal()
    },
    onError: (err) => {
      toast.error(err.response?.data?.message || 'Failed to save')
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id) => cdnApi.delete(id),
    onSuccess: () => {
      toast.success('CDN deleted')
      queryClient.invalidateQueries({ queryKey: ['cdns'] })
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to delete'),
  })

  const syncMutation = useMutation({
    mutationFn: (id) => cdnApi.syncToNAS(id),
    onSuccess: (res) => {
      toast.success(res.data?.message || 'CDN synced to all NAS devices')
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to sync'),
  })

  const syncAllMutation = useMutation({
    mutationFn: () => cdnApi.syncAllToNAS(),
    onSuccess: (res) => {
      toast.success(res.data?.message || 'All CDNs synced to NAS devices')
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to sync'),
  })

  const openModal = (cdn = null) => {
    if (cdn) {
      setEditingCDN(cdn)
      setFormData({
        name: cdn.name || '',
        description: cdn.description || '',
        subnets: cdn.subnets || '',
        color: cdn.color || '#EF4444',
        nas_ids: cdn.nas_ids || '',
        is_active: cdn.is_active ?? true,
      })
    } else {
      setEditingCDN(null)
      setFormData({
        name: '',
        description: '',
        subnets: '',
        color: '#EF4444',
        nas_ids: '',
        is_active: true,
      })
    }
    setShowModal(true)
  }

  const closeModal = () => {
    setShowModal(false)
    setEditingCDN(null)
  }

  const handleSubmit = (e) => {
    e.preventDefault()
    saveMutation.mutate(formData)
  }

  const handleChange = (e) => {
    const { name, value, type, checked } = e.target
    setFormData((prev) => ({
      ...prev,
      [name]: type === 'checkbox' ? checked : value,
    }))
  }

  // Parse subnets (handles comma, newline, or mixed separators)
  const parseSubnets = (subnets) => {
    if (!subnets) return []
    return subnets.replace(/\r?\n/g, ',').split(',').map(s => s.trim()).filter(s => s)
  }

  const columns = useMemo(
    () => [
      {
        accessorKey: 'name',
        header: 'Name',
        cell: ({ row }) => (
          <div className="flex items-center gap-2">
            <div
              className="w-3 h-3 flex-shrink-0 border border-[#999]"
              style={{ backgroundColor: row.original.color || '#EF4444', borderRadius: '1px' }}
              title={`Graph color: ${row.original.color || '#EF4444'}`}
            />
            <div>
              <div className="font-semibold">{row.original.name}</div>
              {row.original.description && (
                <div className="text-[11px] text-gray-500 dark:text-[#aaa]">{row.original.description}</div>
              )}
            </div>
          </div>
        ),
      },
      {
        accessorKey: 'subnets',
        header: 'Subnets',
        cell: ({ row }) => {
          const subnetList = parseSubnets(row.original.subnets)
          const count = subnetList.length
          const displayList = subnetList.slice(0, 3)
          return (
            <div>
              <div className="font-semibold">{count} subnet{count !== 1 ? 's' : ''}</div>
              <div className="text-[11px] text-gray-500 dark:text-[#aaa] font-mono">
                {displayList.join(', ')}
                {count > 3 && ` +${count - 3} more`}
              </div>
            </div>
          )
        },
      },
      {
        accessorKey: 'nas_ids',
        header: 'Target NAS',
        cell: ({ row }) => {
          const nasIds = row.original.nas_ids
          if (!nasIds) {
            return <span className="text-gray-500 dark:text-[#aaa]">All NAS</span>
          }
          const idList = nasIds.split(',').map(id => id.trim()).filter(id => id)
          const nasNames = idList.map(id => {
            const nas = nasList?.find(n => n.id === parseInt(id))
            return nas?.name || `NAS #${id}`
          })
          return (
            <div>
              <div>{nasNames.slice(0, 2).join(', ')}</div>
              {nasNames.length > 2 && (
                <div className="text-[11px] text-gray-500 dark:text-[#aaa]">+{nasNames.length - 2} more</div>
              )}
            </div>
          )
        },
      },
      {
        accessorKey: 'is_active',
        header: 'Status',
        cell: ({ row }) => (
          <span className={clsx('badge', row.original.is_active ? 'badge-success' : 'badge-gray')}>
            {row.original.is_active ? 'Active' : 'Inactive'}
          </span>
        ),
      },
      {
        id: 'actions',
        header: 'Actions',
        cell: ({ row }) => (
          <div className="flex items-center gap-0.5">
            <button
              onClick={() => syncMutation.mutate(row.original.id)}
              disabled={!row.original.is_active || syncMutation.isPending}
              className={clsx('btn btn-sm btn-success', (!row.original.is_active) && 'opacity-40 cursor-not-allowed')}
              title={row.original.is_active ? 'Sync to MikroTik' : 'CDN is inactive'}
              style={{ padding: '1px 4px' }}
            >
              <ArrowPathIcon className={clsx('w-3.5 h-3.5', syncMutation.isPending && 'animate-spin')} />
            </button>
            <button
              onClick={() => openModal(row.original)}
              className="btn btn-sm btn-primary"
              title="Edit"
              style={{ padding: '1px 4px' }}
            >
              <PencilIcon className="w-3.5 h-3.5" />
            </button>
            <button
              onClick={() => {
                if (confirm('Are you sure you want to delete this CDN?')) {
                  deleteMutation.mutate(row.original.id)
                }
              }}
              className="btn btn-sm btn-danger"
              title="Delete"
              style={{ padding: '1px 4px' }}
            >
              <TrashIcon className="w-3.5 h-3.5" />
            </button>
          </div>
        ),
      },
    ],
    [deleteMutation, syncMutation, nasList]
  )

  const table = useReactTable({
    data: cdns || [],
    columns,
    getCoreRowModel: getCoreRowModel(),
  })

  return (
    <div style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Header */}
      <div className="wb-toolbar flex items-center justify-between mb-2">
        <div className="text-[13px] font-semibold">CDN List</div>
        <div className="flex gap-1">
          <button
            onClick={() => syncAllMutation.mutate()}
            disabled={syncAllMutation.isPending}
            className="btn btn-sm flex items-center gap-1"
          >
            <ArrowPathIcon className={clsx('w-3.5 h-3.5', syncAllMutation.isPending && 'animate-spin')} />
            {syncAllMutation.isPending ? 'Syncing...' : 'Sync All to MikroTik'}
          </button>
          <button onClick={() => openModal()} className="btn btn-primary btn-sm flex items-center gap-1">
            <PlusIcon className="w-3.5 h-3.5" />
            Add CDN
          </button>
        </div>
      </div>

      {/* Info Box */}
      <div className="wb-group mb-2">
        <div className="wb-group-title">CDN Configuration</div>
        <div className="wb-group-body text-[11px] text-gray-700 dark:text-[#ccc]">
          Add CDN providers (GGC, Akamai, Cloudflare, etc.) with their IP subnets.
          Then assign them to services with custom speed limits and bypass options.
        </div>
      </div>

      {/* Table */}
      <div className="table-container">
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
                <td colSpan={columns.length} className="text-center py-4">Loading...</td>
              </tr>
            ) : table.getRowModel().rows.length === 0 ? (
              <tr>
                <td colSpan={columns.length} className="text-center py-4 text-gray-500 dark:text-[#aaa]">
                  No CDNs found. Click "Add CDN" to create one.
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

      {/* Modal */}
      {showModal && (
        <div className="modal-overlay">
          <div className="modal" style={{ maxWidth: '500px', width: '100%' }}>
            <div className="modal-header">
              <span>{editingCDN ? 'Edit CDN' : 'Add CDN'}</span>
              <button onClick={closeModal} className="text-white hover:text-gray-200">
                <XMarkIcon className="w-4 h-4" />
              </button>
            </div>

            <form onSubmit={handleSubmit}>
              <div className="modal-body space-y-2">
                <div>
                  <label className="label block mb-0.5">CDN Name</label>
                  <input
                    type="text"
                    name="name"
                    value={formData.name}
                    onChange={handleChange}
                    className="input w-full"
                    placeholder="e.g., GGC, Akamai, Cloudflare"
                    required
                  />
                </div>

                <div>
                  <label className="label block mb-0.5">Description</label>
                  <input
                    type="text"
                    name="description"
                    value={formData.description}
                    onChange={handleChange}
                    className="input w-full"
                    placeholder="e.g., Google Global Cache"
                  />
                </div>

                <div>
                  <label className="label block mb-0.5">Subnets</label>
                  <textarea
                    name="subnets"
                    value={formData.subnets}
                    onChange={handleChange}
                    className="input w-full font-mono"
                    rows={4}
                    placeholder={"Enter subnets separated by commas or new lines:\n185.82.96.0/24\n185.82.97.0/24\n34.104.35.0/24"}
                  />
                  <p className="text-[10px] text-gray-500 dark:text-[#aaa] mt-0.5">
                    Enter IP subnets in CIDR notation, separated by commas or new lines
                  </p>
                </div>

                <div>
                  <label className="label block mb-0.5">Sync to NAS</label>
                  <p className="text-[10px] text-gray-500 dark:text-[#aaa] mb-1">
                    Select which NAS devices to sync (leave empty for all NAS)
                  </p>
                  <div className="border border-[#a0a0a0] dark:border-[#555] p-1.5 max-h-28 overflow-y-auto bg-white dark:bg-[#333]" style={{ borderRadius: '2px' }}>
                    {nasList && nasList.length > 0 ? (
                      <div className="space-y-0.5">
                        {nasList.filter(nas => nas.is_active).map((nas) => {
                          const selectedIds = formData.nas_ids ? formData.nas_ids.split(',').map(id => id.trim()) : []
                          const isSelected = selectedIds.includes(String(nas.id))
                          return (
                            <label key={nas.id} className="flex items-center gap-2 cursor-pointer text-[11px] py-0.5 px-1 hover:bg-[#e8e8f0] dark:hover:bg-[#444]" style={{ borderRadius: '1px' }}>
                              <input
                                type="checkbox"
                                checked={isSelected}
                                onChange={(e) => {
                                  let newIds = selectedIds.filter(id => id !== '')
                                  if (e.target.checked) {
                                    newIds.push(String(nas.id))
                                  } else {
                                    newIds = newIds.filter(id => id !== String(nas.id))
                                  }
                                  setFormData(prev => ({ ...prev, nas_ids: newIds.join(',') }))
                                }}
                              />
                              <span>{nas.name}</span>
                              <span className="text-[10px] text-gray-400 dark:text-[#888]">({nas.ip_address})</span>
                            </label>
                          )
                        })}
                      </div>
                    ) : (
                      <p className="text-[11px] text-gray-400 dark:text-[#888]">No NAS devices available</p>
                    )}
                  </div>
                  {formData.nas_ids && (
                    <p className="text-[10px] text-gray-500 dark:text-[#aaa] mt-0.5">
                      Selected: {formData.nas_ids.split(',').filter(id => id).length} NAS device(s)
                    </p>
                  )}
                </div>

                <div>
                  <label className="label block mb-0.5">Graph Color</label>
                  <p className="text-[10px] text-gray-500 dark:text-[#aaa] mb-1">
                    Choose a color for the live bandwidth graph (blue and green are reserved for download/upload)
                  </p>
                  <div className="flex items-center gap-3">
                    <div className="flex flex-wrap gap-1">
                      {presetColors.map((color) => (
                        <button
                          key={color}
                          type="button"
                          onClick={() => setFormData((prev) => ({ ...prev, color }))}
                          className={clsx(
                            'w-6 h-6 border transition-all',
                            formData.color === color
                              ? 'border-[#316AC5] border-2 scale-110'
                              : 'border-[#999] hover:scale-105'
                          )}
                          style={{ backgroundColor: color, borderRadius: '2px' }}
                          title={color}
                        />
                      ))}
                    </div>
                    <div className="flex items-center gap-1">
                      <input
                        type="color"
                        name="color"
                        value={formData.color}
                        onChange={handleChange}
                        className="w-7 h-7 cursor-pointer border border-[#999]"
                        style={{ borderRadius: '2px', padding: 0 }}
                        title="Custom color"
                      />
                      <span className="text-[10px] text-gray-500 dark:text-[#aaa] font-mono">{formData.color}</span>
                    </div>
                  </div>
                </div>

                <div>
                  <label className="flex items-center gap-2 text-[11px] cursor-pointer">
                    <input
                      type="checkbox"
                      name="is_active"
                      checked={formData.is_active}
                      onChange={handleChange}
                    />
                    <span className="font-semibold">Active</span>
                  </label>
                </div>
              </div>

              <div className="modal-footer">
                <button type="button" onClick={closeModal} className="btn btn-sm">Cancel</button>
                <button type="submit" disabled={saveMutation.isPending} className="btn btn-primary btn-sm">
                  {saveMutation.isPending ? 'Saving...' : editingCDN ? 'Update' : 'Create'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
