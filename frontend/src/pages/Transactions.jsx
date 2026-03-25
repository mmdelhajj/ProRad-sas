import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { dashboardApi } from '../services/api'
import { formatDate, formatTime } from '../utils/timezone'
import {
  useReactTable,
  getCoreRowModel,
  flexRender,
} from '@tanstack/react-table'
import {
  ArrowPathIcon,
  MagnifyingGlassIcon,
  FunnelIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  ArrowTrendingUpIcon,
  ArrowTrendingDownIcon,
  BanknotesIcon,
} from '@heroicons/react/24/outline'
import clsx from 'clsx'

const typeFilters = [
  { value: '', label: 'All Types' },
  { value: 'new', label: 'New Subscription' },
  { value: 'renewal', label: 'Renewal' },
  { value: 'change_service', label: 'Change Service' },
  { value: 'refund', label: 'Refund' },
  { value: 'transfer', label: 'Transfer' },
  { value: 'withdraw', label: 'Withdrawal' },
  { value: 'reset_fup', label: 'Reset FUP' },
  { value: 'refill', label: 'Refill' },
]

export default function Transactions() {
  const [page, setPage] = useState(1)
  const [limit] = useState(25)
  const [search, setSearch] = useState('')
  const [type, setType] = useState('')
  const [dateFrom, setDateFrom] = useState('')
  const [dateTo, setDateTo] = useState('')
  const [showFilters, setShowFilters] = useState(false)

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['transactions', page, limit, search, type, dateFrom, dateTo],
    queryFn: () =>
      dashboardApi
        .transactions({ page, limit, search, type, date_from: dateFrom, date_to: dateTo })
        .then((r) => r.data),
  })

  const columns = useMemo(
    () => [
      {
        accessorKey: 'created_at',
        header: 'Date',
        cell: ({ row }) => (
          <div className="text-[12px]">
            <div>{formatDate(row.original.created_at)}</div>
            <div className="text-gray-500">
              {formatTime(row.original.created_at)}
            </div>
          </div>
        ),
      },
      {
        accessorKey: 'type',
        header: 'Type',
        cell: ({ row }) => {
          const typeColors = {
            new: 'badge-success',
            renewal: 'badge-info',
            change_service: 'badge-purple',
            refund: 'badge-warning',
            transfer: 'badge-primary',
            withdraw: 'badge-danger',
            reset_fup: 'badge-orange',
            refill: 'badge-success',
            adjustment: 'badge-gray',
          }
          const typeLabels = {
            new: 'New',
            renewal: 'Renewal',
            change_service: 'Change Service',
            refund: 'Refund',
            transfer: 'Transfer',
            withdraw: 'Withdrawal',
            reset_fup: 'Reset FUP',
            refill: 'Refill',
          }
          return (
            <span className={clsx('badge', typeColors[row.original.type] || 'badge-gray')}>
              {typeLabels[row.original.type] || row.original.type}
            </span>
          )
        },
      },
      {
        accessorKey: 'subscriber',
        header: 'User',
        cell: ({ row }) => (
          <div className="text-[12px]">
            <div className="font-medium">{row.original.subscriber?.username || '-'}</div>
            <div className="text-gray-500">
              {row.original.subscriber?.fullname || row.original.reseller?.username || ''}
            </div>
          </div>
        ),
      },
      {
        accessorKey: 'service',
        header: 'Service',
        cell: ({ row }) => {
          const t = row.original
          if (t.type === 'change_service' && (t.old_service_name || t.new_service_name)) {
            return (
              <div className="text-[12px]">
                <span className="text-gray-500">{t.old_service_name || '-'}</span>
                <span className="mx-1 text-gray-400">{'->'}</span>
                <span className="font-medium text-blue-700">{t.new_service_name || '-'}</span>
              </div>
            )
          }
          if (t.service_name) {
            return <span className="text-[12px]">{t.service_name}</span>
          }
          return <span className="text-[12px] text-gray-400">-</span>
        },
      },
      {
        accessorKey: 'amount',
        header: 'Amount',
        cell: ({ row }) => (
          <div className={clsx('flex items-center gap-1 font-semibold text-[12px]', row.original.amount >= 0 ? 'text-green-700' : 'text-red-700')}>
            {row.original.amount >= 0 ? (
              <ArrowTrendingUpIcon className="w-3.5 h-3.5" />
            ) : (
              <ArrowTrendingDownIcon className="w-3.5 h-3.5" />
            )}
            ${Math.abs(row.original.amount).toFixed(2)}
          </div>
        ),
      },
      {
        accessorKey: 'balance_after',
        header: 'Balance After',
        cell: ({ row }) => (
          <span className="text-[12px]">
            {row.original.balance_after !== undefined
              ? `$${row.original.balance_after?.toFixed(2)}`
              : '-'}
          </span>
        ),
      },
      {
        accessorKey: 'description',
        header: 'Description',
        cell: ({ row }) => (
          <div className="max-w-[200px] truncate text-[12px] text-gray-600">
            {row.original.description || '-'}
          </div>
        ),
      },
      {
        accessorKey: 'reference',
        header: 'Reference',
        cell: ({ row }) => (
          <code className="text-[11px] px-1 py-0.5 bg-[#f0f0f0] border border-[#d0d0d0]" style={{ borderRadius: 2 }}>
            {row.original.reference || '-'}
          </code>
        ),
      },
    ],
    []
  )

  const table = useReactTable({
    data: data?.data || [],
    columns,
    getCoreRowModel: getCoreRowModel(),
  })

  const totalPages = Math.ceil((data?.meta?.total || 0) / limit)

  // Calculate summary stats
  const totalIncome = data?.data?.reduce((sum, t) => (t.amount > 0 ? sum + t.amount : sum), 0) || 0
  const totalExpense = data?.data?.reduce((sum, t) => (t.amount < 0 ? sum + Math.abs(t.amount) : sum), 0) || 0

  return (
    <div style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Page Title + Toolbar */}
      <div className="wb-toolbar" style={{ marginBottom: 4, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <span className="text-[13px] font-semibold">Transactions</span>
        <button
          onClick={() => refetch()}
          className="btn btn-sm"
          style={{ display: 'flex', alignItems: 'center', gap: 4 }}
        >
          <ArrowPathIcon className="w-3.5 h-3.5" />
          Refresh
        </button>
      </div>

      {/* Summary Stats */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 4, marginBottom: 4 }}>
        <div className="wb-group" style={{ margin: 0 }}>
          <div className="wb-group-body" style={{ padding: '6px 8px', display: 'flex', alignItems: 'center', gap: 8 }}>
            <BanknotesIcon className="w-4 h-4 text-blue-600" />
            <div>
              <div className="text-[11px] text-gray-600">Total Transactions</div>
              <div className="text-[16px] font-bold">{data?.meta?.total || 0}</div>
            </div>
          </div>
        </div>
        <div className="wb-group" style={{ margin: 0 }}>
          <div className="wb-group-body" style={{ padding: '6px 8px', display: 'flex', alignItems: 'center', gap: 8 }}>
            <ArrowTrendingUpIcon className="w-4 h-4 text-green-600" />
            <div>
              <div className="text-[11px] text-gray-600">Page Income</div>
              <div className="text-[16px] font-bold text-green-700">${totalIncome.toFixed(2)}</div>
            </div>
          </div>
        </div>
        <div className="wb-group" style={{ margin: 0 }}>
          <div className="wb-group-body" style={{ padding: '6px 8px', display: 'flex', alignItems: 'center', gap: 8 }}>
            <ArrowTrendingDownIcon className="w-4 h-4 text-red-600" />
            <div>
              <div className="text-[11px] text-gray-600">Page Expense</div>
              <div className="text-[16px] font-bold text-red-700">${totalExpense.toFixed(2)}</div>
            </div>
          </div>
        </div>
        <div className="wb-group" style={{ margin: 0 }}>
          <div className="wb-group-body" style={{ padding: '6px 8px', display: 'flex', alignItems: 'center', gap: 8 }}>
            <BanknotesIcon className="w-4 h-4 text-purple-600" />
            <div>
              <div className="text-[11px] text-gray-600">Page Net</div>
              <div className={clsx('text-[16px] font-bold', totalIncome - totalExpense >= 0 ? 'text-green-700' : 'text-red-700')}>
                ${(totalIncome - totalExpense).toFixed(2)}
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Search and Filters */}
      <div className="wb-group" style={{ margin: 0, marginBottom: 4 }}>
        <div className="wb-group-body" style={{ padding: '4px 8px' }}>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
            <div style={{ flex: 1, position: 'relative', minWidth: 200 }}>
              <MagnifyingGlassIcon className="w-3.5 h-3.5 text-gray-500" style={{ position: 'absolute', left: 6, top: '50%', transform: 'translateY(-50%)' }} />
              <input
                type="text"
                placeholder="Search by username, description, reference..."
                value={search}
                onChange={(e) => {
                  setSearch(e.target.value)
                  setPage(1)
                }}
                className="input"
                style={{ paddingLeft: 24, fontSize: 11, height: 24, width: '100%' }}
              />
            </div>
            <select
              value={type}
              onChange={(e) => {
                setType(e.target.value)
                setPage(1)
              }}
              className="input"
              style={{ fontSize: 11, height: 24 }}
            >
              {typeFilters.map((f) => (
                <option key={f.value} value={f.value}>
                  {f.label}
                </option>
              ))}
            </select>
            <button
              onClick={() => setShowFilters(!showFilters)}
              className={clsx('btn btn-sm', showFilters && 'btn-primary')}
              style={{ display: 'flex', alignItems: 'center', gap: 4 }}
            >
              <FunnelIcon className="w-3.5 h-3.5" />
              Filters
            </button>
          </div>

          {showFilters && (
            <div className="mt-1.5 pt-1.5 border-t border-[#d0d0d0] dark:border-[#374151] grid grid-cols-[1fr_1fr_auto] gap-2 items-end">
              <div>
                <label className="label" style={{ fontSize: 11, marginBottom: 2 }}>Date From</label>
                <input
                  type="date"
                  value={dateFrom}
                  onChange={(e) => {
                    setDateFrom(e.target.value)
                    setPage(1)
                  }}
                  className="input"
                  style={{ fontSize: 11, height: 24 }}
                />
              </div>
              <div>
                <label className="label" style={{ fontSize: 11, marginBottom: 2 }}>Date To</label>
                <input
                  type="date"
                  value={dateTo}
                  onChange={(e) => {
                    setDateTo(e.target.value)
                    setPage(1)
                  }}
                  className="input"
                  style={{ fontSize: 11, height: 24 }}
                />
              </div>
              <button
                onClick={() => {
                  setSearch('')
                  setType('')
                  setDateFrom('')
                  setDateTo('')
                  setPage(1)
                }}
                className="btn btn-sm"
              >
                Clear Filters
              </button>
            </div>
          )}
        </div>
      </div>

      {/* Table */}
      <div className="table-container">
        <table className="table">
          <thead>
            {table.getHeaderGroups().map((headerGroup) => (
              <tr key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <th key={header.id} style={{ fontSize: 11, padding: '4px 8px' }}>
                    {flexRender(header.column.columnDef.header, header.getContext())}
                  </th>
                ))}
              </tr>
            ))}
          </thead>
          <tbody>
            {isLoading ? (
              <tr>
                <td colSpan={columns.length} className="text-center" style={{ padding: 24 }}>
                  Loading...
                </td>
              </tr>
            ) : table.getRowModel().rows.length === 0 ? (
              <tr>
                <td colSpan={columns.length} className="text-center text-gray-500" style={{ padding: 24 }}>
                  No transactions found
                </td>
              </tr>
            ) : (
              table.getRowModel().rows.map((row, idx) => (
                <tr key={row.id} className={idx % 2 === 0 ? 'bg-white dark:bg-[#1f2937]' : 'bg-[#f7f7f7] dark:bg-[#1a2332]'}>
                  {row.getVisibleCells().map((cell) => (
                    <td key={cell.id} style={{ padding: '3px 8px', fontSize: 11 }}>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      <div className="flex items-center justify-between px-2 py-1.5 border-t border-[#d0d0d0] dark:border-[#374151] bg-[#f5f5f5] dark:bg-[#1f2937] text-[11px]">
        <div className="text-gray-600 dark:text-gray-400">
          Showing {((page - 1) * limit) + 1} to {Math.min(page * limit, data?.meta?.total || 0)} of{' '}
          {data?.meta?.total || 0} results
        </div>
        <div className="flex items-center gap-1">
          <button
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={page === 1}
            className="btn btn-sm"
            style={{ padding: '2px 6px', opacity: page === 1 ? 0.5 : 1 }}
          >
            <ChevronLeftIcon className="w-3.5 h-3.5" />
          </button>
          <span className="px-2 dark:text-gray-300">
            Page {page} of {totalPages || 1}
          </span>
          <button
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            disabled={page >= totalPages}
            className="btn btn-sm"
            style={{ padding: '2px 6px', opacity: page >= totalPages ? 0.5 : 1 }}
          >
            <ChevronRightIcon className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>
    </div>
  )
}
