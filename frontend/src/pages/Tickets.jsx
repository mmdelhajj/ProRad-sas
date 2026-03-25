import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import api from '../services/api'
import { formatDate, formatDateTime } from '../utils/timezone'
import {
  PlusIcon,
  TicketIcon,
  ChatBubbleLeftRightIcon,
  CheckCircleIcon,
  ClockIcon,
  ExclamationCircleIcon,
  XMarkIcon,
  BellAlertIcon,
} from '@heroicons/react/24/outline'

const priorityBadge = {
  low: 'badge-gray',
  normal: 'badge-info',
  high: 'badge-orange',
  urgent: 'badge-danger',
}

const statusBadge = {
  open: 'badge-success',
  pending: 'badge-warning',
  in_progress: 'badge-info',
  resolved: 'badge-purple',
  closed: 'badge-gray',
}

const statusIcons = {
  open: ExclamationCircleIcon,
  pending: ClockIcon,
  in_progress: ClockIcon,
  resolved: CheckCircleIcon,
  closed: CheckCircleIcon,
}

export default function Tickets() {
  const queryClient = useQueryClient()
  const [showModal, setShowModal] = useState(false)
  const [showDetailModal, setShowDetailModal] = useState(false)
  const [selectedTicket, setSelectedTicket] = useState(null)
  const [replyText, setReplyText] = useState('')
  const [formData, setFormData] = useState({
    subject: '',
    description: '',
    priority: 'normal',
    category: 'general',
  })
  const [filters, setFilters] = useState({
    status: '',
    priority: '',
  })

  // Fetch tickets
  const { data: ticketsData, isLoading } = useQuery({
    queryKey: ['tickets', filters],
    queryFn: () => api.get('/tickets', { params: filters }).then(res => res.data),
  })

  // Fetch stats
  const { data: statsData } = useQuery({
    queryKey: ['ticket-stats'],
    queryFn: () => api.get('/tickets/stats').then(res => res.data.data),
  })

  // Create ticket
  const createMutation = useMutation({
    mutationFn: (data) => api.post('/tickets', data),
    onSuccess: () => {
      queryClient.invalidateQueries(['tickets'])
      queryClient.invalidateQueries(['ticket-stats'])
      setShowModal(false)
      setFormData({ subject: '', description: '', priority: 'normal', category: 'general' })
    },
  })

  // Update ticket
  const updateMutation = useMutation({
    mutationFn: ({ id, data }) => api.put(`/tickets/${id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries(['tickets'])
      queryClient.invalidateQueries(['ticket-stats'])
    },
  })

  // Add reply
  const replyMutation = useMutation({
    mutationFn: ({ id, message }) => api.post(`/tickets/${id}/reply`, { message }),
    onSuccess: () => {
      queryClient.invalidateQueries(['tickets'])
      setReplyText('')
      // Refresh ticket detail
      if (selectedTicket) {
        api.get(`/tickets/${selectedTicket.id}`).then(res => {
          setSelectedTicket(res.data.data)
        })
      }
    },
  })

  const handleViewTicket = async (ticket) => {
    const res = await api.get(`/tickets/${ticket.id}`)
    setSelectedTicket(res.data.data)
    setShowDetailModal(true)
  }

  const stats = statsData || { open: 0, pending: 0, closed: 0, total: 0 }

  return (
    <div className="space-y-2" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Toolbar */}
      <div className="wb-toolbar justify-between">
        <span className="text-[13px] font-semibold">Support Tickets</span>
        <button
          onClick={() => setShowModal(true)}
          className="btn btn-primary"
        >
          <PlusIcon className="w-3 h-3 mr-1" />
          New Ticket
        </button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-2">
        <div className="stat-card">
          <div className="flex items-center">
            <ExclamationCircleIcon className="w-4 h-4 text-[#4CAF50] mr-2" />
            <div>
              <div className="text-[11px] text-gray-500">Open</div>
              <div className="text-[16px] font-bold text-[#4CAF50]">{stats.open}</div>
            </div>
          </div>
        </div>
        <div className="stat-card">
          <div className="flex items-center">
            <ClockIcon className="w-4 h-4 text-[#FF9800] mr-2" />
            <div>
              <div className="text-[11px] text-gray-500">Pending</div>
              <div className="text-[16px] font-bold text-[#FF9800]">{stats.pending}</div>
            </div>
          </div>
        </div>
        <div className="stat-card">
          <div className="flex items-center">
            <CheckCircleIcon className="w-4 h-4 text-gray-500 mr-2" />
            <div>
              <div className="text-[11px] text-gray-500">Closed</div>
              <div className="text-[16px] font-bold text-gray-500">{stats.closed}</div>
            </div>
          </div>
        </div>
        <div className="stat-card">
          <div className="flex items-center">
            <TicketIcon className="w-4 h-4 text-[#2196F3] mr-2" />
            <div>
              <div className="text-[11px] text-gray-500">Total</div>
              <div className="text-[16px] font-bold">{stats.total}</div>
            </div>
          </div>
        </div>
      </div>

      {/* Filters */}
      <div className="wb-toolbar gap-2">
        <select
          value={filters.status}
          onChange={(e) => setFilters({ ...filters, status: e.target.value })}
          className="input"
          style={{ width: 'auto', minWidth: 130 }}
        >
          <option value="">All Status</option>
          <option value="open">Open</option>
          <option value="pending">Pending</option>
          <option value="in_progress">In Progress</option>
          <option value="resolved">Resolved</option>
          <option value="closed">Closed</option>
        </select>
        <select
          value={filters.priority}
          onChange={(e) => setFilters({ ...filters, priority: e.target.value })}
          className="input"
          style={{ width: 'auto', minWidth: 130 }}
        >
          <option value="">All Priority</option>
          <option value="low">Low</option>
          <option value="normal">Normal</option>
          <option value="high">High</option>
          <option value="urgent">Urgent</option>
        </select>
      </div>

      {/* Tickets Table */}
      <div className="table-container">
        <table className="table">
          <thead>
            <tr>
              <th>Ticket</th>
              <th>Subject</th>
              <th>Status</th>
              <th>Priority</th>
              <th>Category</th>
              <th>Created</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <tr>
                <td colSpan={7} className="text-center py-4 text-gray-500">Loading...</td>
              </tr>
            ) : ticketsData?.data?.length === 0 ? (
              <tr>
                <td colSpan={7} className="text-center py-4 text-gray-500">No tickets found</td>
              </tr>
            ) : (
              ticketsData?.data?.map((ticket) => {
                const StatusIcon = statusIcons[ticket.status] || ExclamationCircleIcon
                return (
                  <tr key={ticket.id}>
                    <td>
                      <div className="flex items-center gap-1">
                        <span className="font-mono">{ticket.ticket_number}</span>
                        {ticket.has_customer_reply && (
                          <span className="inline-block w-2 h-2 bg-red-500" style={{ borderRadius: '50%' }}></span>
                        )}
                      </div>
                    </td>
                    <td>
                      <div className="flex items-center gap-1">
                        <span className="font-semibold">{ticket.subject}</span>
                        {ticket.has_customer_reply && (
                          <BellAlertIcon className="w-3 h-3 text-red-500" title="New customer reply" />
                        )}
                      </div>
                      {ticket.subscriber && (
                        <div className="text-[11px] text-gray-500">{ticket.subscriber.username}</div>
                      )}
                    </td>
                    <td>
                      <span className={statusBadge[ticket.status] || 'badge-gray'}>
                        <StatusIcon className="w-3 h-3 mr-0.5 inline" />
                        {ticket.status}
                      </span>
                    </td>
                    <td>
                      <span className={priorityBadge[ticket.priority] || 'badge-gray'}>
                        {ticket.priority}
                      </span>
                    </td>
                    <td>{ticket.category}</td>
                    <td>{formatDate(ticket.created_at)}</td>
                    <td>
                      <button
                        onClick={() => handleViewTicket(ticket)}
                        className="btn btn-sm"
                      >
                        View
                      </button>
                    </td>
                  </tr>
                )
              })
            )}
          </tbody>
        </table>
      </div>

      {/* Create Modal */}
      {showModal && (
        <div className="modal-overlay">
          <div className="modal" style={{ width: 480 }}>
            <div className="modal-header">
              <span>Create New Ticket</span>
              <button onClick={() => setShowModal(false)} className="text-white hover:text-gray-200 text-[16px] leading-none">&times;</button>
            </div>
            <div className="modal-body space-y-3">
              <div>
                <label className="label">Subject</label>
                <input
                  type="text"
                  value={formData.subject}
                  onChange={(e) => setFormData({ ...formData, subject: e.target.value })}
                  className="input"
                  placeholder="Brief description of the issue"
                />
              </div>
              <div>
                <label className="label">Description</label>
                <textarea
                  value={formData.description}
                  onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                  rows={4}
                  className="input"
                  placeholder="Detailed description of the issue"
                />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="label">Priority</label>
                  <select
                    value={formData.priority}
                    onChange={(e) => setFormData({ ...formData, priority: e.target.value })}
                    className="input"
                  >
                    <option value="low">Low</option>
                    <option value="normal">Normal</option>
                    <option value="high">High</option>
                    <option value="urgent">Urgent</option>
                  </select>
                </div>
                <div>
                  <label className="label">Category</label>
                  <select
                    value={formData.category}
                    onChange={(e) => setFormData({ ...formData, category: e.target.value })}
                    className="input"
                  >
                    <option value="general">General</option>
                    <option value="billing">Billing</option>
                    <option value="technical">Technical</option>
                    <option value="other">Other</option>
                  </select>
                </div>
              </div>
            </div>
            <div className="modal-footer">
              <button
                onClick={() => setShowModal(false)}
                className="btn"
              >
                Cancel
              </button>
              <button
                onClick={() => createMutation.mutate(formData)}
                disabled={createMutation.isPending || !formData.subject || !formData.description}
                className="btn btn-primary"
              >
                {createMutation.isPending ? 'Creating...' : 'Create Ticket'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Detail Modal */}
      {showDetailModal && selectedTicket && (
        <div className="modal-overlay">
          <div className="modal modal-lg" style={{ maxHeight: '90vh' }}>
            <div className="modal-header">
              <div>
                <span>{selectedTicket.subject}</span>
                <span className="ml-2 text-[11px] font-normal opacity-80">{selectedTicket.ticket_number}</span>
              </div>
              <button onClick={() => setShowDetailModal(false)} className="text-white hover:text-gray-200 text-[16px] leading-none">&times;</button>
            </div>
            <div className="modal-body space-y-3" style={{ maxHeight: '60vh', overflowY: 'auto' }}>
              {/* Status and Actions */}
              <div className="flex items-center gap-2">
                <span className={statusBadge[selectedTicket.status] || 'badge-gray'}>
                  {selectedTicket.status}
                </span>
                <span className={priorityBadge[selectedTicket.priority] || 'badge-gray'}>
                  {selectedTicket.priority}
                </span>
                <div className="flex-1" />
                <select
                  value={selectedTicket.status}
                  onChange={(e) => {
                    updateMutation.mutate({ id: selectedTicket.id, data: { status: e.target.value } })
                    setSelectedTicket({ ...selectedTicket, status: e.target.value })
                  }}
                  className="input"
                  style={{ width: 'auto', minWidth: 120 }}
                >
                  <option value="open">Open</option>
                  <option value="pending">Pending</option>
                  <option value="in_progress">In Progress</option>
                  <option value="resolved">Resolved</option>
                  <option value="closed">Closed</option>
                </select>
              </div>

              {/* Original Description */}
              <div className="wb-group">
                <div className="wb-group-title text-[11px] text-gray-500">
                  Created on {formatDateTime(selectedTicket.created_at)}
                </div>
                <div className="wb-group-body">
                  <p className="whitespace-pre-wrap text-[12px]">{selectedTicket.description}</p>
                </div>
              </div>

              {/* Replies */}
              {selectedTicket.replies?.map((reply) => (
                <div key={reply.id} className="wb-group" style={reply.is_internal ? { borderColor: '#FF9800' } : {}}>
                  <div className="wb-group-title flex items-center gap-2">
                    <span className="font-semibold text-[12px]">{reply.user?.username || 'User'}</span>
                    {reply.is_internal && (
                      <span className="badge-warning">Internal</span>
                    )}
                    <span className="text-[11px] text-gray-500 ml-auto">
                      {formatDateTime(reply.created_at)}
                    </span>
                  </div>
                  <div className="wb-group-body">
                    <p className="whitespace-pre-wrap text-[12px]">{reply.message}</p>
                  </div>
                </div>
              ))}
            </div>

            {/* Reply Form */}
            <div className="modal-footer flex-col items-stretch gap-2">
              <div className="flex gap-1">
                <textarea
                  value={replyText}
                  onChange={(e) => setReplyText(e.target.value)}
                  rows={2}
                  className="input flex-1"
                  placeholder="Type your reply..."
                />
                <button
                  onClick={() => replyMutation.mutate({ id: selectedTicket.id, message: replyText })}
                  disabled={replyMutation.isPending || !replyText}
                  className="btn btn-primary"
                >
                  <ChatBubbleLeftRightIcon className="w-4 h-4" />
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
