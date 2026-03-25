import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import api from '../services/api'

export default function Prepaid() {
  const queryClient = useQueryClient()
  const [showGenerateModal, setShowGenerateModal] = useState(false)
  const [showRedeemModal, setShowRedeemModal] = useState(false)
  const [page, setPage] = useState(1)
  const [isUsed, setIsUsed] = useState('')
  const [batchFilter, setBatchFilter] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['prepaid', page, isUsed, batchFilter],
    queryFn: () => api.get('/prepaid', { params: { page, is_used: isUsed, batch_id: batchFilter } }).then(res => res.data)
  })

  const { data: batches } = useQuery({
    queryKey: ['prepaid-batches'],
    queryFn: () => api.get('/prepaid/batches').then(res => res.data.data)
  })

  const { data: services } = useQuery({
    queryKey: ['services'],
    queryFn: () => api.get('/services').then(res => res.data.data)
  })

  const [generateForm, setGenerateForm] = useState({
    service_id: '',
    count: 10,
    value: 0,
    days: 30,
    quota_refill: 0,
    prefix: '',
    code_length: 12,
    pin_length: 4
  })

  const [redeemForm, setRedeemForm] = useState({
    code: '',
    pin: '',
    subscriber_id: ''
  })

  const generateMutation = useMutation({
    mutationFn: (data) => api.post('/prepaid/generate', data),
    onSuccess: () => {
      queryClient.invalidateQueries(['prepaid'])
      queryClient.invalidateQueries(['prepaid-batches'])
      setShowGenerateModal(false)
    }
  })

  const redeemMutation = useMutation({
    mutationFn: (data) => api.post('/prepaid/use', data),
    onSuccess: () => {
      queryClient.invalidateQueries(['prepaid'])
      setShowRedeemModal(false)
      alert('Card redeemed successfully!')
    },
    onError: (error) => {
      alert(error.response?.data?.message || 'Failed to redeem card')
    }
  })

  const deleteMutation = useMutation({
    mutationFn: (id) => api.delete(`/prepaid/${id}`),
    onSuccess: () => queryClient.invalidateQueries(['prepaid'])
  })

  const deleteBatchMutation = useMutation({
    mutationFn: (batchId) => api.delete(`/prepaid/batch/${batchId}`),
    onSuccess: () => {
      queryClient.invalidateQueries(['prepaid'])
      queryClient.invalidateQueries(['prepaid-batches'])
    }
  })

  const handleGenerate = (e) => {
    e.preventDefault()
    generateMutation.mutate({
      ...generateForm,
      service_id: parseInt(generateForm.service_id) || 0
    })
  }

  const handleRedeem = (e) => {
    e.preventDefault()
    redeemMutation.mutate({
      ...redeemForm,
      subscriber_id: parseInt(redeemForm.subscriber_id)
    })
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin h-5 w-5 border-2 border-[#316AC5] border-t-transparent" style={{ borderRadius: '50%' }}></div>
      </div>
    )
  }

  const cards = data?.data || []
  const meta = data?.meta || {}

  return (
    <div className="space-y-2" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Toolbar */}
      <div className="wb-toolbar justify-between">
        <span className="text-[13px] font-semibold">Prepaid Cards</span>
        <div className="flex gap-1">
          <button
            onClick={() => setShowRedeemModal(true)}
            className="btn btn-success"
          >
            Redeem Card
          </button>
          <button
            onClick={() => setShowGenerateModal(true)}
            className="btn btn-primary"
          >
            Generate Cards
          </button>
        </div>
      </div>

      {/* Batches Overview */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-2">
        {(batches || []).slice(0, 4).map(batch => (
          <div key={batch.batch_id} className="stat-card">
            <div className="text-[11px] font-medium text-gray-500 dark:text-gray-400">{batch.batch_id}</div>
            <div className="mt-1 flex justify-between items-baseline">
              <span className="text-[14px] font-bold text-[#4CAF50]">{batch.active}</span>
              <span className="text-[10px] text-gray-500 dark:text-gray-400">/ {batch.total} total</span>
            </div>
            <div className="mt-1 flex justify-between items-center">
              <span className="text-[10px] text-gray-500 dark:text-gray-400">Used: {batch.used}</span>
              {batch.active > 0 && (
                <button
                  onClick={() => {
                    if (confirm(`Delete all ${batch.active} unused cards from ${batch.batch_id}?`)) {
                      deleteBatchMutation.mutate(batch.batch_id)
                    }
                  }}
                  className="btn btn-danger btn-xs"
                >
                  Delete Unused
                </button>
              )}
            </div>
          </div>
        ))}
      </div>

      {/* Filters */}
      <div className="wb-toolbar gap-2">
        <select
          value={isUsed}
          onChange={(e) => { setIsUsed(e.target.value); setPage(1) }}
          className="input"
          style={{ width: 'auto', minWidth: 120 }}
        >
          <option value="">All Cards</option>
          <option value="false">Available</option>
          <option value="true">Used</option>
        </select>
        <select
          value={batchFilter}
          onChange={(e) => { setBatchFilter(e.target.value); setPage(1) }}
          className="input"
          style={{ width: 'auto', minWidth: 140 }}
        >
          <option value="">All Batches</option>
          {(batches || []).map(b => (
            <option key={b.batch_id} value={b.batch_id}>{b.batch_id}</option>
          ))}
        </select>
      </div>

      {/* Cards Table */}
      <div className="table-container">
        <table className="table">
          <thead>
            <tr>
              <th>Code</th>
              <th>PIN</th>
              <th>Value</th>
              <th>Days</th>
              <th>Service</th>
              <th>Status</th>
              <th>Batch</th>
              <th style={{ textAlign: 'right' }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {cards.map(card => (
              <tr key={card.id}>
                <td className="font-mono">{card.code}</td>
                <td className="font-mono">{card.pin}</td>
                <td>${card.value?.toFixed(2)}</td>
                <td>{card.days}</td>
                <td>{card.service?.name || '-'}</td>
                <td>
                  <span className={card.is_used ? 'badge-gray' : 'badge-success'}>
                    {card.is_used ? 'Used' : 'Available'}
                  </span>
                </td>
                <td>{card.batch_id}</td>
                <td style={{ textAlign: 'right' }}>
                  {!card.is_used && (
                    <button
                      onClick={() => deleteMutation.mutate(card.id)}
                      className="btn btn-danger btn-xs"
                    >
                      Delete
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {meta.totalPages > 1 && (
        <div className="wb-statusbar">
          <span>Page {page} of {meta.totalPages}</span>
          <div className="flex gap-1">
            <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1} className="btn btn-sm">Previous</button>
            <button onClick={() => setPage(p => p + 1)} disabled={page >= meta.totalPages} className="btn btn-sm">Next</button>
          </div>
        </div>
      )}

      {/* Generate Modal */}
      {showGenerateModal && (
        <div className="modal-overlay">
          <div className="modal" style={{ width: 420 }}>
            <div className="modal-header">
              <span>Generate Prepaid Cards</span>
              <button onClick={() => setShowGenerateModal(false)} className="text-white hover:text-gray-200 text-[13px] leading-none">&times;</button>
            </div>
            <form onSubmit={handleGenerate}>
              <div className="modal-body space-y-2">
                <div className="grid grid-cols-2 gap-2">
                  <div>
                    <label className="label">Count</label>
                    <input
                      type="number"
                      value={generateForm.count}
                      onChange={(e) => setGenerateForm({ ...generateForm, count: parseInt(e.target.value) || 0 })}
                      className="input"
                      min="1" max="1000" required
                    />
                  </div>
                  <div>
                    <label className="label">Days</label>
                    <input
                      type="number"
                      value={generateForm.days}
                      onChange={(e) => setGenerateForm({ ...generateForm, days: parseInt(e.target.value) || 0 })}
                      className="input"
                    />
                  </div>
                </div>
                <div>
                  <label className="label">Service</label>
                  <select
                    value={generateForm.service_id}
                    onChange={(e) => setGenerateForm({ ...generateForm, service_id: e.target.value })}
                    className="input"
                  >
                    <option value="">No service change</option>
                    {(services || []).map(s => (
                      <option key={s.id} value={s.id}>{s.name}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="label">Prefix (optional)</label>
                  <input
                    type="text"
                    value={generateForm.prefix}
                    onChange={(e) => setGenerateForm({ ...generateForm, prefix: e.target.value.toUpperCase() })}
                    className="input"
                    maxLength="6"
                  />
                </div>
              </div>
              <div className="modal-footer">
                <button type="button" onClick={() => setShowGenerateModal(false)} className="btn">Cancel</button>
                <button type="submit" disabled={generateMutation.isPending} className="btn btn-primary">
                  Generate {generateForm.count} Cards
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Redeem Modal */}
      {showRedeemModal && (
        <div className="modal-overlay">
          <div className="modal" style={{ width: 400 }}>
            <div className="modal-header">
              <span>Redeem Prepaid Card</span>
              <button onClick={() => setShowRedeemModal(false)} className="text-white hover:text-gray-200 text-[13px] leading-none">&times;</button>
            </div>
            <form onSubmit={handleRedeem}>
              <div className="modal-body space-y-2">
                <div>
                  <label className="label">Card Code</label>
                  <input
                    type="text"
                    value={redeemForm.code}
                    onChange={(e) => setRedeemForm({ ...redeemForm, code: e.target.value })}
                    className="input"
                    required
                  />
                </div>
                <div>
                  <label className="label">PIN</label>
                  <input
                    type="text"
                    value={redeemForm.pin}
                    onChange={(e) => setRedeemForm({ ...redeemForm, pin: e.target.value })}
                    className="input"
                    required
                  />
                </div>
                <div>
                  <label className="label">Subscriber ID</label>
                  <input
                    type="number"
                    value={redeemForm.subscriber_id}
                    onChange={(e) => setRedeemForm({ ...redeemForm, subscriber_id: e.target.value })}
                    className="input"
                    required
                  />
                </div>
              </div>
              <div className="modal-footer">
                <button type="button" onClick={() => setShowRedeemModal(false)} className="btn">Cancel</button>
                <button type="submit" disabled={redeemMutation.isPending} className="btn btn-success">Redeem</button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
