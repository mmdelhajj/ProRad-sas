import { useState, useEffect, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import api from '../services/api'
import { formatDate } from '../utils/timezone'

const STATUS_BADGE = {
  pending: 'badge-warning',
  completed: 'badge-success',
  failed: 'badge-danger',
  refunded: 'badge-gray'
}

function InvoiceDetailModal({ invoiceId, onClose, apiPrefix = '' }) {
  const [invoice, setInvoice] = useState(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!invoiceId) return
    setLoading(true)
    api.get(`${apiPrefix}/invoices/${invoiceId}`)
      .then(res => {
        setInvoice(res.data?.data || null)
        setLoading(false)
      })
      .catch(() => setLoading(false))
  }, [invoiceId, apiPrefix])

  const handlePrint = () => {
    window.print()
  }

  if (!invoiceId) return null

  const statusColor = {
    pending: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300',
    completed: 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300',
    failed: 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300',
    refunded: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300',
  }

  return (
    <div className="modal-overlay no-print" onClick={onClose}>
      <div className="modal modal-lg" onClick={e => e.stopPropagation()} style={{ maxWidth: 700, width: '95%' }}>
        <div className="modal-header no-print">
          <span>Invoice Details</span>
          <div className="flex items-center gap-2">
            <button
              onClick={handlePrint}
              className="text-white hover:text-gray-200 text-[11px] px-2 py-0.5 border border-white/30 rounded"
            >
              Print / Save PDF
            </button>
            <button onClick={onClose} className="text-white hover:text-gray-200 text-[13px] leading-none">&times;</button>
          </div>
        </div>
        <div className="modal-body" style={{ padding: 0 }}>
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <div className="animate-spin h-5 w-5 border-2 border-[#316AC5] border-t-transparent" style={{ borderRadius: '50%' }}></div>
            </div>
          ) : !invoice ? (
            <div className="text-center text-gray-500 py-12">Invoice not found</div>
          ) : (
            <div className="invoice-print-area bg-white text-black" style={{ padding: 32, fontSize: 12, fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif" }}>
              {/* Header */}
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 24 }}>
                <div>
                  <h1 style={{ fontSize: 22, fontWeight: 700, margin: 0, color: '#1a1a1a' }}>INVOICE</h1>
                  <p style={{ fontSize: 13, color: '#555', margin: '4px 0 0' }}>{invoice.invoice_number}</p>
                </div>
                <div style={{ textAlign: 'right' }}>
                  <span style={{ display: 'inline-block', padding: '3px 10px', borderRadius: 4, fontSize: 11, fontWeight: 600, textTransform: 'uppercase', ...(invoice.status === 'completed' ? { background: '#dcfce7', color: '#166534' } : invoice.status === 'failed' ? { background: '#fee2e2', color: '#991b1b' } : { background: '#fef9c3', color: '#854d0e' }) }}>
                    {invoice.status}
                  </span>
                  {invoice.auto_generated && (
                    <span style={{ display: 'inline-block', marginLeft: 6, padding: '3px 8px', borderRadius: 4, fontSize: 10, fontWeight: 600, background: '#dbeafe', color: '#1e40af' }}>Auto</span>
                  )}
                </div>
              </div>

              {/* Subscriber + Dates */}
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 24, gap: 24 }}>
                <div>
                  <p style={{ fontSize: 10, color: '#888', textTransform: 'uppercase', letterSpacing: 0.5, margin: '0 0 4px' }}>Bill To</p>
                  <p style={{ fontWeight: 600, fontSize: 13, margin: 0 }}>{invoice.subscriber?.full_name || invoice.subscriber?.username || 'N/A'}</p>
                  {invoice.subscriber?.username && invoice.subscriber?.full_name && (
                    <p style={{ color: '#555', margin: '2px 0 0', fontSize: 11 }}>{invoice.subscriber.username}</p>
                  )}
                  {invoice.subscriber?.phone && (
                    <p style={{ color: '#555', margin: '2px 0 0', fontSize: 11 }}>{invoice.subscriber.phone}</p>
                  )}
                  {invoice.subscriber?.address && (
                    <p style={{ color: '#555', margin: '2px 0 0', fontSize: 11 }}>{invoice.subscriber.address}</p>
                  )}
                </div>
                <div style={{ textAlign: 'right' }}>
                  <div style={{ marginBottom: 6 }}>
                    <span style={{ fontSize: 10, color: '#888' }}>Created: </span>
                    <span style={{ fontSize: 11 }}>{invoice.created_at ? new Date(invoice.created_at).toLocaleDateString() : '-'}</span>
                  </div>
                  <div style={{ marginBottom: 6 }}>
                    <span style={{ fontSize: 10, color: '#888' }}>Due Date: </span>
                    <span style={{ fontSize: 11, fontWeight: 600 }}>{invoice.due_date ? new Date(invoice.due_date).toLocaleDateString() : '-'}</span>
                  </div>
                  {invoice.paid_date && (
                    <div>
                      <span style={{ fontSize: 10, color: '#888' }}>Paid: </span>
                      <span style={{ fontSize: 11, color: '#166534' }}>{new Date(invoice.paid_date).toLocaleDateString()}</span>
                    </div>
                  )}
                  {invoice.billing_period_start && invoice.billing_period_end && (
                    <div style={{ marginTop: 6 }}>
                      <span style={{ fontSize: 10, color: '#888' }}>Period: </span>
                      <span style={{ fontSize: 11 }}>{new Date(invoice.billing_period_start).toLocaleDateString()} — {new Date(invoice.billing_period_end).toLocaleDateString()}</span>
                    </div>
                  )}
                </div>
              </div>

              {/* Items Table */}
              <table style={{ width: '100%', borderCollapse: 'collapse', marginBottom: 16 }}>
                <thead>
                  <tr style={{ borderBottom: '2px solid #e5e7eb' }}>
                    <th style={{ textAlign: 'left', padding: '8px 6px', fontSize: 10, color: '#888', textTransform: 'uppercase', letterSpacing: 0.5 }}>Description</th>
                    <th style={{ textAlign: 'center', padding: '8px 6px', fontSize: 10, color: '#888', textTransform: 'uppercase', width: 60 }}>Qty</th>
                    <th style={{ textAlign: 'right', padding: '8px 6px', fontSize: 10, color: '#888', textTransform: 'uppercase', width: 90 }}>Unit Price</th>
                    <th style={{ textAlign: 'right', padding: '8px 6px', fontSize: 10, color: '#888', textTransform: 'uppercase', width: 90 }}>Total</th>
                  </tr>
                </thead>
                <tbody>
                  {(invoice.items || invoice.Items || []).map((item, i) => (
                    <tr key={item.id || i} style={{ borderBottom: '1px solid #f3f4f6' }}>
                      <td style={{ padding: '8px 6px', fontSize: 12 }}>{item.description}</td>
                      <td style={{ padding: '8px 6px', fontSize: 12, textAlign: 'center' }}>{item.quantity}</td>
                      <td style={{ padding: '8px 6px', fontSize: 12, textAlign: 'right' }}>${(item.unit_price || 0).toFixed(2)}</td>
                      <td style={{ padding: '8px 6px', fontSize: 12, textAlign: 'right' }}>${(item.total || item.unit_price * item.quantity || 0).toFixed(2)}</td>
                    </tr>
                  ))}
                  {(!invoice.items || invoice.items.length === 0) && (!invoice.Items || invoice.Items.length === 0) && (
                    <tr><td colSpan={4} style={{ padding: '12px 6px', color: '#999', textAlign: 'center' }}>No items</td></tr>
                  )}
                </tbody>
              </table>

              {/* Summary */}
              <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
                <div style={{ width: 240 }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', padding: '4px 0', fontSize: 12 }}>
                    <span style={{ color: '#555' }}>Subtotal</span>
                    <span>${(invoice.sub_total || invoice.SubTotal || 0).toFixed(2)}</span>
                  </div>
                  {(invoice.discount || 0) > 0 && (
                    <div style={{ display: 'flex', justifyContent: 'space-between', padding: '4px 0', fontSize: 12 }}>
                      <span style={{ color: '#555' }}>Discount</span>
                      <span>-${(invoice.discount).toFixed(2)}</span>
                    </div>
                  )}
                  {(invoice.tax || 0) > 0 && (
                    <div style={{ display: 'flex', justifyContent: 'space-between', padding: '4px 0', fontSize: 12 }}>
                      <span style={{ color: '#555' }}>Tax</span>
                      <span>${(invoice.tax).toFixed(2)}</span>
                    </div>
                  )}
                  <div style={{ display: 'flex', justifyContent: 'space-between', padding: '6px 0', fontSize: 14, fontWeight: 700, borderTop: '2px solid #1a1a1a', marginTop: 4 }}>
                    <span>Total</span>
                    <span>${(invoice.total || 0).toFixed(2)}</span>
                  </div>
                  {(invoice.amount_paid || 0) > 0 && (
                    <div style={{ display: 'flex', justifyContent: 'space-between', padding: '4px 0', fontSize: 12 }}>
                      <span style={{ color: '#166534' }}>Amount Paid</span>
                      <span style={{ color: '#166534' }}>-${(invoice.amount_paid).toFixed(2)}</span>
                    </div>
                  )}
                  {(invoice.total - (invoice.amount_paid || 0)) > 0.01 && (
                    <div style={{ display: 'flex', justifyContent: 'space-between', padding: '6px 0', fontSize: 13, fontWeight: 700, borderTop: '1px solid #e5e7eb' }}>
                      <span style={{ color: '#991b1b' }}>Balance Due</span>
                      <span style={{ color: '#991b1b' }}>${(invoice.total - (invoice.amount_paid || 0)).toFixed(2)}</span>
                    </div>
                  )}
                </div>
              </div>

              {/* Notes */}
              {invoice.notes && (
                <div style={{ marginTop: 20, padding: '10px 12px', background: '#f9fafb', borderRadius: 4, border: '1px solid #e5e7eb' }}>
                  <p style={{ fontSize: 10, color: '#888', textTransform: 'uppercase', margin: '0 0 4px' }}>Notes</p>
                  <p style={{ fontSize: 11, color: '#333', margin: 0, whiteSpace: 'pre-wrap' }}>{invoice.notes}</p>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Print CSS */}
      <style>{`
        @media print {
          body * { visibility: hidden !important; }
          .invoice-print-area, .invoice-print-area * { visibility: visible !important; }
          .invoice-print-area { position: fixed; left: 0; top: 0; width: 100%; background: white !important; padding: 40px !important; z-index: 99999; }
          .no-print { display: none !important; }
          .modal-overlay { background: transparent !important; }
          .modal { box-shadow: none !important; border: none !important; }
        }
      `}</style>
    </div>
  )
}

export { InvoiceDetailModal }

export default function Invoices() {
  const queryClient = useQueryClient()
  const [showModal, setShowModal] = useState(false)
  const [showPaymentModal, setShowPaymentModal] = useState(false)
  const [selectedInvoice, setSelectedInvoice] = useState(null)
  const [viewInvoiceId, setViewInvoiceId] = useState(null)
  const [page, setPage] = useState(1)
  const [status, setStatus] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['invoices', page, status],
    queryFn: () => api.get('/invoices', { params: { page, status } }).then(res => res.data)
  })

  const [formData, setFormData] = useState({
    subscriber_id: '',
    due_date: '',
    notes: '',
    items: [{ description: '', quantity: 1, unit_price: 0 }]
  })

  // Subscriber search for Create Invoice
  const [subscriberSearch, setSubscriberSearch] = useState('')
  const [subscriberResults, setSubscriberResults] = useState([])
  const [selectedSubscriber, setSelectedSubscriber] = useState(null)
  const [showSubscriberDropdown, setShowSubscriberDropdown] = useState(false)
  const [searchLoading, setSearchLoading] = useState(false)
  const searchRef = useRef(null)
  const dropdownRef = useRef(null)

  useEffect(() => {
    if (subscriberSearch.length < 2) {
      setSubscriberResults([])
      setShowSubscriberDropdown(false)
      return
    }
    setSearchLoading(true)
    const timer = setTimeout(() => {
      api.get('/subscribers', { params: { search: subscriberSearch, limit: 10, page: 1 } })
        .then(res => {
          setSubscriberResults(res.data?.data || [])
          setShowSubscriberDropdown(true)
          setSearchLoading(false)
        })
        .catch(() => setSearchLoading(false))
    }, 300)
    return () => clearTimeout(timer)
  }, [subscriberSearch])

  // Close dropdown on outside click
  useEffect(() => {
    const handler = (e) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target) &&
          searchRef.current && !searchRef.current.contains(e.target)) {
        setShowSubscriberDropdown(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  const selectSubscriber = (sub) => {
    setSelectedSubscriber(sub)
    setSubscriberSearch(sub.username + (sub.full_name ? ` — ${sub.full_name}` : ''))
    setShowSubscriberDropdown(false)
    const price = sub.override_price ? sub.price : (sub.service?.price || 0)
    const serviceName = sub.service?.name || 'Internet Service'
    setFormData(prev => ({
      ...prev,
      subscriber_id: sub.id,
      due_date: sub.expiry_date ? sub.expiry_date.split('T')[0] : '',
      items: [{ description: serviceName, quantity: 1, unit_price: price }]
    }))
  }

  const [paymentData, setPaymentData] = useState({
    amount: 0,
    method: 'cash',
    reference: '',
    notes: ''
  })

  const createMutation = useMutation({
    mutationFn: (data) => api.post('/invoices', data),
    onSuccess: () => {
      queryClient.invalidateQueries(['invoices'])
      setShowModal(false)
    }
  })

  const paymentMutation = useMutation({
    mutationFn: ({ id, data }) => api.post(`/invoices/${id}/payment`, data),
    onSuccess: () => {
      queryClient.invalidateQueries(['invoices'])
      setShowPaymentModal(false)
    }
  })

  const deleteMutation = useMutation({
    mutationFn: (id) => api.delete(`/invoices/${id}`),
    onSuccess: () => queryClient.invalidateQueries(['invoices'])
  })

  const addItem = () => {
    setFormData({
      ...formData,
      items: [...formData.items, { description: '', quantity: 1, unit_price: 0 }]
    })
  }

  const removeItem = (index) => {
    setFormData({
      ...formData,
      items: formData.items.filter((_, i) => i !== index)
    })
  }

  const updateItem = (index, field, value) => {
    const items = [...formData.items]
    items[index][field] = field === 'quantity' || field === 'unit_price' ? parseFloat(value) || 0 : value
    setFormData({ ...formData, items })
  }

  const calculateTotal = () => {
    return formData.items.reduce((sum, item) => sum + (item.quantity * item.unit_price), 0)
  }

  const handleSubmit = (e) => {
    e.preventDefault()
    createMutation.mutate({
      ...formData,
      subscriber_id: parseInt(formData.subscriber_id)
    })
  }

  const handlePayment = (invoice) => {
    setSelectedInvoice(invoice)
    setPaymentData({
      amount: invoice.total - invoice.amount_paid,
      method: 'cash',
      reference: '',
      notes: ''
    })
    setShowPaymentModal(true)
  }

  const submitPayment = (e) => {
    e.preventDefault()
    paymentMutation.mutate({ id: selectedInvoice.id, data: paymentData })
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin h-5 w-5 border-2 border-[#316AC5] border-t-transparent" style={{ borderRadius: '50%' }}></div>
      </div>
    )
  }

  const invoices = data?.data || []
  const meta = data?.meta || {}

  return (
    <div className="space-y-2" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Toolbar */}
      <div className="wb-toolbar justify-between">
        <span className="text-[13px] font-semibold">Invoices</span>
        <button
          onClick={() => { setShowModal(true); setSubscriberSearch(''); setSelectedSubscriber(null); setFormData({ subscriber_id: '', due_date: '', notes: '', items: [{ description: '', quantity: 1, unit_price: 0 }] }) }}
          className="btn btn-primary"
        >
          Create Invoice
        </button>
      </div>

      {/* Filter */}
      <div className="wb-toolbar">
        <select
          value={status}
          onChange={(e) => { setStatus(e.target.value); setPage(1) }}
          className="input"
          style={{ width: 'auto', minWidth: 140 }}
        >
          <option value="">All Status</option>
          <option value="pending">Pending</option>
          <option value="completed">Completed</option>
          <option value="failed">Failed</option>
        </select>
      </div>

      {/* Table */}
      <div className="table-container">
        <table className="table">
          <thead>
            <tr>
              <th>Invoice #</th>
              <th>Subscriber</th>
              <th>Total</th>
              <th>Paid</th>
              <th>Status</th>
              <th>Due Date</th>
              <th style={{ textAlign: 'right' }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {invoices.map(invoice => (
              <tr key={invoice.id}>
                <td className="font-semibold">
                  {invoice.invoice_number}
                  {invoice.auto_generated && (
                    <span className="ml-1 inline-block px-1.5 py-0.5 text-[9px] font-bold bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300 rounded">Auto</span>
                  )}
                </td>
                <td>{invoice.subscriber?.username || 'N/A'}</td>
                <td>${invoice.total?.toFixed(2)}</td>
                <td>${invoice.amount_paid?.toFixed(2)}</td>
                <td>
                  <span className={STATUS_BADGE[invoice.status] || 'badge-gray'}>
                    {invoice.status}
                  </span>
                </td>
                <td>{formatDate(invoice.due_date)}</td>
                <td style={{ textAlign: 'right' }}>
                  <button
                    onClick={() => setViewInvoiceId(invoice.id)}
                    className="btn btn-xs mr-1"
                    title="View Invoice"
                  >
                    View
                  </button>
                  {invoice.status !== 'completed' && (
                    <button
                      onClick={() => handlePayment(invoice)}
                      className="btn btn-success btn-xs mr-1"
                    >
                      Add Payment
                    </button>
                  )}
                  {invoice.status !== 'completed' && (
                    <button
                      onClick={() => deleteMutation.mutate(invoice.id)}
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
          <span>
            Page {page} of {meta.totalPages} ({meta.total} total)
          </span>
          <div className="flex gap-1">
            <button
              onClick={() => setPage(p => Math.max(1, p - 1))}
              disabled={page === 1}
              className="btn btn-sm"
            >
              Previous
            </button>
            <button
              onClick={() => setPage(p => p + 1)}
              disabled={page >= meta.totalPages}
              className="btn btn-sm"
            >
              Next
            </button>
          </div>
        </div>
      )}

      {/* View Invoice Modal */}
      {viewInvoiceId && (
        <InvoiceDetailModal
          invoiceId={viewInvoiceId}
          onClose={() => setViewInvoiceId(null)}
        />
      )}

      {/* Create Invoice Modal */}
      {showModal && (
        <div className="modal-overlay">
          <div className="modal modal-lg">
            <div className="modal-header">
              <span>Create Invoice</span>
              <button onClick={() => setShowModal(false)} className="text-white hover:text-gray-200 text-[13px] leading-none">&times;</button>
            </div>
            <form onSubmit={handleSubmit}>
              <div className="modal-body space-y-2">
                <div className="grid grid-cols-2 gap-2">
                  <div className="relative">
                    <label className="label">Subscriber</label>
                    <input
                      ref={searchRef}
                      type="text"
                      value={subscriberSearch}
                      onChange={(e) => { setSubscriberSearch(e.target.value); setSelectedSubscriber(null); setFormData(f => ({ ...f, subscriber_id: '' })) }}
                      onFocus={() => subscriberResults.length > 0 && setShowSubscriberDropdown(true)}
                      placeholder="Search by username or name..."
                      className="input"
                      required={!formData.subscriber_id}
                      autoComplete="off"
                    />
                    {searchLoading && <span className="absolute right-2 top-7 text-[10px] text-gray-400">...</span>}
                    {showSubscriberDropdown && subscriberResults.length > 0 && (
                      <div ref={dropdownRef} className="absolute z-50 left-0 right-0 top-full mt-0.5 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded shadow-lg max-h-48 overflow-y-auto">
                        {subscriberResults.map(sub => (
                          <div
                            key={sub.id}
                            onClick={() => selectSubscriber(sub)}
                            className="px-2 py-1.5 cursor-pointer hover:bg-blue-50 dark:hover:bg-gray-700 text-[11px] border-b border-gray-100 dark:border-gray-700 last:border-0"
                          >
                            <span className="font-semibold">{sub.username}</span>
                            {sub.full_name && <span className="text-gray-500 dark:text-gray-400 ml-1">— {sub.full_name}</span>}
                            {sub.service?.name && <span className="text-gray-400 dark:text-gray-500 ml-1">({sub.service.name})</span>}
                          </div>
                        ))}
                      </div>
                    )}
                    {selectedSubscriber && (
                      <p className="text-[10px] text-green-600 dark:text-green-400 mt-0.5">
                        ID: {selectedSubscriber.id} | Service: {selectedSubscriber.service?.name || 'N/A'} | Price: ${(selectedSubscriber.override_price ? selectedSubscriber.price : (selectedSubscriber.service?.price || 0)).toFixed(2)}
                      </p>
                    )}
                  </div>
                  <div>
                    <label className="label">Due Date</label>
                    <input
                      type="date"
                      value={formData.due_date}
                      onChange={(e) => setFormData({ ...formData, due_date: e.target.value })}
                      className="input"
                    />
                  </div>
                </div>

                <div>
                  <label className="label">Items</label>
                  {formData.items.map((item, index) => (
                    <div key={index} className="flex gap-1 mb-1">
                      <input
                        type="text"
                        placeholder="Description"
                        value={item.description}
                        onChange={(e) => updateItem(index, 'description', e.target.value)}
                        className="input flex-1"
                      />
                      <input
                        type="number"
                        placeholder="Qty"
                        value={item.quantity}
                        onChange={(e) => updateItem(index, 'quantity', e.target.value)}
                        className="input"
                        style={{ width: 60 }}
                      />
                      <input
                        type="number"
                        step="0.01"
                        placeholder="Price"
                        value={item.unit_price}
                        onChange={(e) => updateItem(index, 'unit_price', e.target.value)}
                        className="input"
                        style={{ width: 80 }}
                      />
                      {formData.items.length > 1 && (
                        <button
                          type="button"
                          onClick={() => removeItem(index)}
                          className="btn btn-danger btn-xs"
                        >
                          X
                        </button>
                      )}
                    </div>
                  ))}
                  <button
                    type="button"
                    onClick={addItem}
                    className="btn btn-sm mt-1"
                  >
                    + Add Item
                  </button>
                </div>

                <div className="text-right text-[13px] font-semibold">
                  Total: ${calculateTotal().toFixed(2)}
                </div>

                <div>
                  <label className="label">Notes</label>
                  <textarea
                    value={formData.notes}
                    onChange={(e) => setFormData({ ...formData, notes: e.target.value })}
                    className="input"
                    rows={3}
                  />
                </div>
              </div>
              <div className="modal-footer">
                <button
                  type="button"
                  onClick={() => setShowModal(false)}
                  className="btn"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={createMutation.isPending}
                  className="btn btn-primary"
                >
                  Create Invoice
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Payment Modal */}
      {showPaymentModal && selectedInvoice && (
        <div className="modal-overlay">
          <div className="modal" style={{ width: 400 }}>
            <div className="modal-header">
              <span>Add Payment</span>
              <button onClick={() => setShowPaymentModal(false)} className="text-white hover:text-gray-200 text-[13px] leading-none">&times;</button>
            </div>
            <form onSubmit={submitPayment}>
              <div className="modal-body space-y-2">
                <p className="text-[11px] text-gray-600 dark:text-gray-400">
                  Invoice: {selectedInvoice.invoice_number} |
                  Balance: ${(selectedInvoice.total - selectedInvoice.amount_paid).toFixed(2)}
                </p>
                <div>
                  <label className="label">Amount</label>
                  <input
                    type="number"
                    step="0.01"
                    value={paymentData.amount}
                    onChange={(e) => setPaymentData({ ...paymentData, amount: parseFloat(e.target.value) || 0 })}
                    className="input"
                    required
                  />
                </div>
                <div>
                  <label className="label">Method</label>
                  <select
                    value={paymentData.method}
                    onChange={(e) => setPaymentData({ ...paymentData, method: e.target.value })}
                    className="input"
                  >
                    <option value="cash">Cash</option>
                    <option value="card">Card</option>
                    <option value="bank_transfer">Bank Transfer</option>
                    <option value="online">Online</option>
                  </select>
                </div>
                <div>
                  <label className="label">Reference</label>
                  <input
                    type="text"
                    value={paymentData.reference}
                    onChange={(e) => setPaymentData({ ...paymentData, reference: e.target.value })}
                    className="input"
                  />
                </div>
              </div>
              <div className="modal-footer">
                <button
                  type="button"
                  onClick={() => setShowPaymentModal(false)}
                  className="btn"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={paymentMutation.isPending}
                  className="btn btn-success"
                >
                  Add Payment
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
