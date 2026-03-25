import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate } from 'react-router-dom'
import { bandwidthCustomerApi, nasApi } from '../services/api'
import {
  PlusIcon, SignalIcon, MagnifyingGlassIcon,
  UsersIcon, CurrencyDollarIcon, WifiIcon
} from '@heroicons/react/24/outline'

function formatSpeed(kb) {
  if (!kb) return '0k'
  if (kb >= 1000000) return (kb / 1000000).toFixed(1) + 'G'
  if (kb >= 1000) return (kb / 1000).toFixed(0) + 'M'
  return kb + 'k'
}

function formatBytes(bytes) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let val = bytes
  while (val >= 1024 && i < units.length - 1) { val /= 1024; i++ }
  return val.toFixed(i > 1 ? 1 : 0) + ' ' + units[i]
}

const statusColors = {
  active: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
  suspended: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
  expired: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
}

export default function BandwidthManager() {
  const navigate = useNavigate()
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [nasFilter, setNasFilter] = useState('')
  const limit = 25

  const { data, isLoading } = useQuery({
    queryKey: ['bandwidth-customers', page, search, statusFilter, nasFilter],
    queryFn: () => bandwidthCustomerApi.list({ page, limit, search, status: statusFilter, nas_id: nasFilter || undefined }),
    keepPreviousData: true,
  })

  const { data: nasData } = useQuery({
    queryKey: ['nas-list'],
    queryFn: () => nasApi.list({ limit: 100 }),
  })

  const customers = data?.data?.data || []
  const total = data?.data?.total || 0
  const stats = data?.data?.stats || {}
  const nasList = nasData?.data?.data || []
  const totalPages = Math.ceil(total / limit)

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Bandwidth Manager</h1>
        <Link to="/bandwidth-manager/new" className="btn btn-primary flex items-center gap-2">
          <PlusIcon className="w-5 h-5" />
          <span className="hidden sm:inline">Add Customer</span>
        </Link>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-2 sm:grid-cols-5 gap-3">
        <div className="bg-white dark:bg-gray-800 rounded-lg p-3 border border-gray-200 dark:border-gray-700">
          <div className="text-sm text-gray-500 dark:text-gray-400">Total</div>
          <div className="text-xl font-bold text-gray-900 dark:text-white">{stats.total || 0}</div>
        </div>
        <div className="bg-white dark:bg-gray-800 rounded-lg p-3 border border-gray-200 dark:border-gray-700">
          <div className="text-sm text-gray-500 dark:text-gray-400">Active</div>
          <div className="text-xl font-bold text-green-600">{stats.active || 0}</div>
        </div>
        <div className="bg-white dark:bg-gray-800 rounded-lg p-3 border border-gray-200 dark:border-gray-700">
          <div className="text-sm text-gray-500 dark:text-gray-400">Suspended</div>
          <div className="text-xl font-bold text-red-600">{stats.suspended || 0}</div>
        </div>
        <div className="bg-white dark:bg-gray-800 rounded-lg p-3 border border-gray-200 dark:border-gray-700">
          <div className="text-sm text-gray-500 dark:text-gray-400">Online</div>
          <div className="text-xl font-bold text-blue-600">{stats.online || 0}</div>
        </div>
        <div className="bg-white dark:bg-gray-800 rounded-lg p-3 border border-gray-200 dark:border-gray-700">
          <div className="text-sm text-gray-500 dark:text-gray-400">Revenue</div>
          <div className="text-xl font-bold text-purple-600">${(stats.revenue || 0).toLocaleString()}</div>
        </div>
      </div>

      {/* Filters */}
      <div className="flex flex-col sm:flex-row gap-3">
        <div className="relative flex-1">
          <MagnifyingGlassIcon className="w-5 h-5 absolute left-3 top-2.5 text-gray-400" />
          <input
            type="text"
            placeholder="Search name, IP, contact..."
            value={search}
            onChange={(e) => { setSearch(e.target.value); setPage(1) }}
            className="input pl-10 w-full dark:bg-gray-700 dark:text-white dark:border-gray-600"
          />
        </div>
        <select
          value={statusFilter}
          onChange={(e) => { setStatusFilter(e.target.value); setPage(1) }}
          className="input w-full sm:w-40 dark:bg-gray-700 dark:text-white dark:border-gray-600"
        >
          <option value="">All Status</option>
          <option value="active">Active</option>
          <option value="suspended">Suspended</option>
          <option value="expired">Expired</option>
        </select>
        <select
          value={nasFilter}
          onChange={(e) => { setNasFilter(e.target.value); setPage(1) }}
          className="input w-full sm:w-40 dark:bg-gray-700 dark:text-white dark:border-gray-600"
        >
          <option value="">All NAS</option>
          {nasList.map(nas => <option key={nas.id} value={nas.id}>{nas.name}</option>)}
        </select>
      </div>

      {/* Table */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-x-auto">
        <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
          <thead className="bg-gray-50 dark:bg-gray-700">
            <tr>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Name</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">IP Address</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase hidden sm:table-cell">VLAN</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Speed</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase hidden md:table-cell">CDN</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase hidden md:table-cell">Daily Used</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase hidden lg:table-cell">Price</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Status</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
            {isLoading ? (
              <tr><td colSpan={8} className="px-4 py-8 text-center text-gray-500 dark:text-gray-400">Loading...</td></tr>
            ) : customers.length === 0 ? (
              <tr><td colSpan={8} className="px-4 py-8 text-center text-gray-500 dark:text-gray-400">No bandwidth customers found</td></tr>
            ) : customers.map(cust => (
              <tr
                key={cust.id}
                onClick={() => navigate(`/bandwidth-manager/${cust.id}`)}
                className="hover:bg-gray-50 dark:hover:bg-gray-700 cursor-pointer"
              >
                <td className="px-4 py-3">
                  <div className="flex items-center gap-2">
                    {cust.is_online && <span className="w-2 h-2 bg-green-500 rounded-full flex-shrink-0" />}
                    <span className="font-medium text-gray-900 dark:text-white">{cust.name}</span>
                  </div>
                  {cust.contact_person && <div className="text-xs text-gray-500 dark:text-gray-400">{cust.contact_person}</div>}
                </td>
                <td className="px-4 py-3 text-sm text-gray-700 dark:text-gray-300 font-mono">{cust.ip_address}</td>
                <td className="px-4 py-3 text-sm text-gray-700 dark:text-gray-300 hidden sm:table-cell">{cust.vlan_id || '—'}</td>
                <td className="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">
                  {formatSpeed(cust.download_speed)}/{formatSpeed(cust.upload_speed)}
                </td>
                <td className="px-4 py-3 text-sm text-gray-700 dark:text-gray-300 hidden md:table-cell">
                  {cust.cdn_download_speed ? formatSpeed(cust.cdn_download_speed) : '—'}
                </td>
                <td className="px-4 py-3 text-sm text-gray-700 dark:text-gray-300 hidden md:table-cell">
                  {formatBytes(cust.daily_download_used)}
                  {cust.fup_level > 0 && <span className="ml-1 text-xs text-orange-500">FUP{cust.fup_level}</span>}
                </td>
                <td className="px-4 py-3 text-sm hidden lg:table-cell text-gray-700 dark:text-gray-300">${cust.price}</td>
                <td className="px-4 py-3">
                  <span className={`px-2 py-0.5 text-xs rounded-full font-medium ${statusColors[cust.status] || 'bg-gray-100 text-gray-800 dark:bg-gray-600 dark:text-gray-300'}`}>
                    {cust.status}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between text-sm text-gray-500 dark:text-gray-400">
          <span>Showing {(page - 1) * limit + 1} to {Math.min(page * limit, total)} of {total}</span>
          <div className="flex gap-1">
            <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1} className="btn btn-sm">Prev</button>
            <button onClick={() => setPage(p => Math.min(totalPages, p + 1))} disabled={page === totalPages} className="btn btn-sm">Next</button>
          </div>
        </div>
      )}
    </div>
  )
}
