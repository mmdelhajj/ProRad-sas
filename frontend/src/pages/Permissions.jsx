import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { permissionApi } from '../services/api'
import { PlusIcon, ArrowPathIcon, PencilIcon, TrashIcon, ArrowLeftIcon, MagnifyingGlassIcon } from '@heroicons/react/24/outline'
import toast from 'react-hot-toast'

export default function Permissions() {
  const queryClient = useQueryClient()
  const [view, setView] = useState('list') // 'list' or 'edit'
  const [editingGroup, setEditingGroup] = useState(null)
  const [groupForm, setGroupForm] = useState({
    name: '',
    description: '',
    permissions: []
  })
  const [searchTerm, setSearchTerm] = useState('')
  const [permissionSearch, setPermissionSearch] = useState('')

  // Fetch permissions
  const { data: permissionsData, isLoading: permissionsLoading, refetch: refetchPermissions } = useQuery({
    queryKey: ['permissions'],
    queryFn: async () => {
      const res = await permissionApi.list()
      return res.data
    }
  })

  // Fetch permission groups
  const { data: groupsData, isLoading: groupsLoading, refetch: refetchGroups } = useQuery({
    queryKey: ['permission-groups'],
    queryFn: async () => {
      const res = await permissionApi.listGroups()
      return res.data
    }
  })

  // Seed permissions mutation
  const seedMutation = useMutation({
    mutationFn: () => permissionApi.seed(),
    onSuccess: () => {
      queryClient.invalidateQueries(['permissions'])
      toast.success('Default permissions seeded successfully')
    }
  })

  // Create/Update group mutation
  const groupMutation = useMutation({
    mutationFn: (data) => {
      if (editingGroup) {
        return permissionApi.updateGroup(editingGroup.id, data)
      }
      return permissionApi.createGroup(data)
    },
    onSuccess: () => {
      queryClient.invalidateQueries(['permission-groups'])
      toast.success(editingGroup ? 'Permission group updated' : 'Permission group created')
      setView('list')
      setEditingGroup(null)
      setGroupForm({ name: '', description: '', permissions: [] })
    },
    onError: (err) => {
      toast.error(err.response?.data?.message || 'Failed to save permission group')
    }
  })

  // Delete group mutation
  const deleteGroupMutation = useMutation({
    mutationFn: (id) => permissionApi.deleteGroup(id),
    onSuccess: () => {
      queryClient.invalidateQueries(['permission-groups'])
      toast.success('Permission group deleted')
    }
  })

  const permissions = permissionsData?.data || []
  const groups = groupsData?.data || []

  // Filter groups by search term
  const filteredGroups = groups.filter(group =>
    group.name.toLowerCase().includes(searchTerm.toLowerCase())
  )

  // Filter permissions by search term
  const filteredPermissions = permissionSearch
    ? permissions.filter(perm =>
        perm.name?.toLowerCase().includes(permissionSearch.toLowerCase()) ||
        perm.description?.toLowerCase().includes(permissionSearch.toLowerCase())
      )
    : permissions

  // Group permissions by category
  const permissionsByCategory = filteredPermissions.reduce((acc, perm) => {
    const category = perm.name?.split('.')[0] || 'Other'
    if (!acc[category]) acc[category] = []
    acc[category].push(perm)
    return acc
  }, {})

  // Get all categories sorted
  const categories = Object.keys(permissionsByCategory).sort()

  const handleEditGroup = (group) => {
    setEditingGroup(group)
    setGroupForm({
      name: group.name,
      description: group.description || '',
      permissions: group.permissions?.map(p => p.id) || []
    })
    setPermissionSearch('')
    setView('edit')
  }

  const handleAddGroup = () => {
    setEditingGroup(null)
    setGroupForm({ name: '', description: '', permissions: [] })
    setPermissionSearch('')
    setView('edit')
  }

  const handlePermissionToggle = (permId) => {
    setGroupForm(prev => ({
      ...prev,
      permissions: prev.permissions.includes(permId)
        ? prev.permissions.filter(id => id !== permId)
        : [...prev.permissions, permId]
    }))
  }

  const handleSelectAllCategory = (category, selected) => {
    const categoryPermIds = permissionsByCategory[category].map(p => p.id)
    setGroupForm(prev => ({
      ...prev,
      permissions: selected
        ? [...new Set([...prev.permissions, ...categoryPermIds])]
        : prev.permissions.filter(id => !categoryPermIds.includes(id))
    }))
  }

  const handleSubmitGroup = (e) => {
    e.preventDefault()
    if (!groupForm.name.trim()) {
      toast.error('Group name is required')
      return
    }
    groupMutation.mutate({
      name: groupForm.name,
      description: groupForm.description,
      permission_ids: groupForm.permissions
    })
  }

  const handleRefresh = () => {
    refetchGroups()
    refetchPermissions()
    toast.success('Refreshed')
  }

  if (permissionsLoading || groupsLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin h-5 w-5 border-2 border-[#316AC5] border-t-transparent" style={{ borderRadius: '50%' }}></div>
      </div>
    )
  }

  // Edit View
  if (view === 'edit') {
    return (
      <div className="space-y-2" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
        {/* Title */}
        <div className="wb-toolbar">
          <span className="text-[13px] font-semibold">
            {editingGroup ? 'EDIT PERMISSIONS' : 'ADD PERMISSION GROUP'}
          </span>
        </div>

        <div className="wb-group">
          <form onSubmit={handleSubmitGroup}>
            <div className="wb-group-body space-y-3">
              {/* Group Name and Search */}
              <div className="flex items-end gap-3">
                <div className="flex-1" style={{ maxWidth: 300 }}>
                  <label className="label">GroupName*</label>
                  <input
                    type="text"
                    value={groupForm.name}
                    onChange={(e) => setGroupForm({ ...groupForm, name: e.target.value })}
                    className="input"
                    placeholder="Enter group name"
                    required
                  />
                </div>
                <div>
                  <label className="label">Search Permissions</label>
                  <div className="relative">
                    <MagnifyingGlassIcon className="absolute left-2 top-1/2 transform -translate-y-1/2 h-3 w-3 text-gray-400" />
                    <input
                      type="text"
                      value={permissionSearch}
                      onChange={(e) => setPermissionSearch(e.target.value)}
                      className="input pl-6"
                      style={{ width: 220 }}
                      placeholder="Search permissions..."
                    />
                  </div>
                </div>
              </div>

              {/* Search Results Info */}
              {permissionSearch && (
                <div className="text-[12px] text-gray-600">
                  Found {filteredPermissions.length} permission{filteredPermissions.length !== 1 ? 's' : ''} matching "{permissionSearch}"
                  {filteredPermissions.length === 0 && (
                    <span className="text-[#c0392b] ml-2">- No results found</span>
                  )}
                </div>
              )}

              {/* Permissions */}
              <div>
                <div className="px-3 py-1.5 text-[12px] font-semibold text-white" style={{ background: 'linear-gradient(to bottom, #5a8abf, #3a6a9f)', borderRadius: '2px 2px 0 0' }}>
                  Permissions
                </div>
                <div className="border border-[#a0a0a0] border-t-0 p-3 space-y-3" style={{ maxHeight: 400, overflowY: 'auto', borderRadius: '0 0 2px 2px' }}>
                  {categories.length === 0 && (
                    <p className="text-[12px] text-gray-500">No permissions found</p>
                  )}
                  {categories.map(category => (
                    <div key={category} className="space-y-1">
                      <div className="flex items-center justify-between">
                        <span className="font-semibold text-[12px] capitalize">{category}</span>
                        <label className="flex items-center text-[11px] text-gray-500 cursor-pointer">
                          <input
                            type="checkbox"
                            checked={permissionsByCategory[category].every(p => groupForm.permissions.includes(p.id))}
                            onChange={(e) => handleSelectAllCategory(category, e.target.checked)}
                            className="mr-1"
                          />
                          All
                        </label>
                      </div>
                      <div className="space-y-0.5 pl-2">
                        {permissionsByCategory[category].map(perm => (
                          <label key={perm.id} className="flex items-center cursor-pointer py-0.5">
                            <input
                              type="checkbox"
                              checked={groupForm.permissions.includes(perm.id)}
                              onChange={() => handlePermissionToggle(perm.id)}
                              className="mr-1.5"
                            />
                            <span className="text-[12px]">
                              {perm.description || perm.name}
                            </span>
                          </label>
                        ))}
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              {/* Action Buttons */}
              <div className="flex gap-2 pt-2">
                <button
                  type="submit"
                  disabled={groupMutation.isPending}
                  className="btn btn-primary"
                >
                  {groupMutation.isPending ? 'Saving...' : (editingGroup ? 'Update' : 'Create')}
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setView('list')
                    setEditingGroup(null)
                  }}
                  className="btn"
                >
                  <ArrowLeftIcon className="h-3 w-3 mr-1" />
                  Back to List
                </button>
              </div>
            </div>
          </form>
        </div>
      </div>
    )
  }

  // List View
  return (
    <div className="space-y-2" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Title */}
      <div className="wb-toolbar">
        <span className="text-[13px] font-semibold">PERMISSIONS</span>
      </div>

      <div className="wb-group">
        <div className="wb-group-body">
          {/* Action Buttons */}
          <div className="flex items-center justify-between mb-2">
            <div className="flex gap-1">
              <button
                onClick={handleRefresh}
                className="btn"
              >
                <ArrowPathIcon className="h-3 w-3 mr-1" />
                Refresh
              </button>
              <button
                onClick={handleAddGroup}
                className="btn"
              >
                <PlusIcon className="h-3 w-3 mr-1" />
                Add
              </button>
              {permissions.length === 0 && (
                <button
                  onClick={() => seedMutation.mutate()}
                  disabled={seedMutation.isPending}
                  className="btn btn-primary"
                >
                  {seedMutation.isPending ? 'Seeding...' : 'Seed Default Permissions'}
                </button>
              )}
            </div>

            {/* Search */}
            <div className="relative">
              <MagnifyingGlassIcon className="absolute left-2 top-1/2 transform -translate-y-1/2 h-3 w-3 text-gray-400" />
              <input
                type="text"
                placeholder="Search groups..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="input pl-6"
                style={{ width: 180 }}
              />
            </div>
          </div>

          {/* Table */}
          <div className="table-container">
            <table className="table">
              <thead>
                <tr>
                  <th style={{ width: 60 }}>ID</th>
                  <th>Name</th>
                  <th style={{ width: 100 }}>Permissions</th>
                  <th>Resellers</th>
                  <th style={{ width: 100 }}>Actions</th>
                </tr>
              </thead>
              <tbody>
                {filteredGroups.length === 0 ? (
                  <tr>
                    <td colSpan="5" className="text-center py-4 text-gray-500">
                      No permission groups found
                    </td>
                  </tr>
                ) : (
                  filteredGroups.map((group) => (
                    <tr key={group.id}>
                      <td>{group.id}</td>
                      <td className="font-semibold">{group.name}</td>
                      <td>{group.permissions?.length || 0}</td>
                      <td>
                        {group.resellers && group.resellers.length > 0 ? (
                          <div className="flex flex-wrap gap-0.5">
                            {group.resellers.map(reseller => (
                              <span
                                key={reseller.id}
                                className="badge-info"
                                title={reseller.full_name || reseller.username}
                              >
                                {reseller.username}
                              </span>
                            ))}
                          </div>
                        ) : (
                          <span className="text-gray-400">-</span>
                        )}
                      </td>
                      <td>
                        <div className="flex gap-1">
                          <button
                            onClick={() => handleEditGroup(group)}
                            className="btn btn-xs"
                            title="Edit"
                          >
                            <PencilIcon className="h-3 w-3" />
                          </button>
                          <button
                            onClick={() => {
                              if (confirm('Delete this permission group?')) {
                                deleteGroupMutation.mutate(group.id)
                              }
                            }}
                            className="btn btn-danger btn-xs"
                            title="Delete"
                          >
                            <TrashIcon className="h-3 w-3" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>

          {/* Footer */}
          <div className="wb-statusbar mt-2" style={{ border: '1px solid #a0a0a0', borderRadius: 2 }}>
            <span>total: {filteredGroups.length}</span>
            <div className="flex items-center gap-1">
              <span>Page size:</span>
              <select className="input" style={{ width: 'auto', minWidth: 60, minHeight: 20, padding: '0 4px' }}>
                <option>10</option>
                <option>25</option>
                <option>50</option>
              </select>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
