import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { publicApi } from '../services/api'
import api from '../services/api'

export default function Impersonate() {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const [error, setError] = useState(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const token = searchParams.get('token')

    if (!token) {
      setError('No impersonation token provided')
      setLoading(false)
      return
    }

    // Exchange the temporary token for a real session
    const exchangeToken = async () => {
      try {
        const response = await publicApi.exchangeImpersonateToken(token)

        if (response.data.success) {
          const { token: jwtToken, user } = response.data.data

          // Store session in sessionStorage with special impersonate key
          // This key is used by authStore to detect impersonated sessions
          // Format must match what authStore.loadInitialState() expects (flat structure)
          const authState = {
            user: user,
            token: jwtToken,
            isCustomer: false,
            customerData: null,
          }
          // Use the special impersonate key - authStore checks for this on load
          sessionStorage.setItem('proisp-impersonate', JSON.stringify(authState))

          // Set the API header
          api.defaults.headers.common['Authorization'] = `Bearer ${jwtToken}`

          // Redirect to dashboard
          window.location.href = '/'
        } else {
          setError(response.data.message || 'Failed to exchange token')
          setLoading(false)
        }
      } catch (err) {
        console.error('Impersonate error:', err)
        setError(err.response?.data?.message || 'Failed to authenticate. Token may have expired.')
        setLoading(false)
      }
    }

    exchangeToken()
  }, [searchParams, navigate])

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[#c0c0c0] dark:bg-[#2d2d2d]" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
        <div className="card p-2 text-center">
          <svg className="animate-spin h-6 w-6 text-[#316AC5] mx-auto mb-2" fill="none" viewBox="0 0 24 24">
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
          </svg>
          <p className="text-[11px] text-gray-600 dark:text-[#ccc]">Authenticating as reseller...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[#c0c0c0] dark:bg-[#2d2d2d]" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
        <div className="card" style={{ maxWidth: '400px', width: '100%' }}>
          <div className="modal-header">
            <span>Authentication Failed</span>
          </div>
          <div className="p-2 bg-[#f0f0f0] dark:bg-[#3a3a3a] text-center">
            <div className="mx-auto flex items-center justify-center h-8 w-8 border border-[#f44336] bg-[#ffe0e0] dark:bg-[#4a2020] mb-2" style={{ borderRadius: '2px' }}>
              <svg className="h-5 w-5 text-[#f44336]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </div>
            <p className="text-[12px] font-semibold text-gray-900 dark:text-[#e0e0e0] mb-1">Authentication Failed</p>
            <p className="text-[11px] text-gray-600 dark:text-[#ccc] mb-2">{error}</p>
            <button
              onClick={() => window.close()}
              className="btn btn-primary"
            >
              Close This Tab
            </button>
          </div>
        </div>
      </div>
    )
  }

  return null
}
