import { useState, useRef } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { subscriberApi, serviceApi, nasApi } from '../services/api'
import * as XLSX from 'xlsx'
import {
  ArrowDownTrayIcon,
  ArrowUpTrayIcon,
  DocumentArrowUpIcon,
  CheckCircleIcon,
  ArrowLeftIcon,
} from '@heroicons/react/24/outline'
import toast from 'react-hot-toast'

export default function SubscriberImport() {
  const navigate = useNavigate()
  const fileInputRef = useRef(null)
  const [parsedData, setParsedData] = useState([])
  const [fileName, setFileName] = useState('')
  const [selectedNasId, setSelectedNasId] = useState('')
  const [importResults, setImportResults] = useState(null)
  const [isImporting, setIsImporting] = useState(false)

  // Fetch services for validation
  const { data: servicesData } = useQuery({
    queryKey: ['services'],
    queryFn: () => serviceApi.list(),
  })

  // Fetch NAS devices
  const { data: nasData } = useQuery({
    queryKey: ['nas'],
    queryFn: () => nasApi.list(),
  })

  const services = servicesData?.data?.data || []
  const nasDevices = nasData?.data?.data || []

  // Import mutation
  const importMutation = useMutation({
    mutationFn: (data) => subscriberApi.importExcel(data),
    onSuccess: (response) => {
      setIsImporting(false)
      setImportResults(response.data.data)
      toast.success(response.data.message)
    },
    onError: (error) => {
      setIsImporting(false)
      toast.error(error.response?.data?.message || 'Import failed')
    },
  })

  const handleFileChange = (e) => {
    const file = e.target.files[0]
    if (!file) return

    setFileName(file.name)
    setImportResults(null)

    const reader = new FileReader()
    reader.onload = (evt) => {
      try {
        const bstr = evt.target.result
        const wb = XLSX.read(bstr, { type: 'binary' })
        const wsname = wb.SheetNames[0]
        const ws = wb.Sheets[wsname]
        const data = XLSX.utils.sheet_to_json(ws, { header: 1 })

        // Skip header row (row 0), auto-detect if row 1 is description or data
        const headers = data[0] || []

        // Check if row 1 looks like a description row (contains words like "required", "optional", etc.)
        const row1 = data[1] || []
        const row1Text = row1.join(' ').toLowerCase()
        const isDescriptionRow = row1Text.includes('required') || row1Text.includes('optional') ||
                                  row1Text.includes('format') || row1Text.includes('example')

        const dataStartRow = isDescriptionRow ? 2 : 1
        const rows = data.slice(dataStartRow).filter(row => row.some(cell => cell !== undefined && cell !== ''))

        // Map column names to indices (case-insensitive)
        const colMap = {}
        headers.forEach((h, i) => {
          if (h) colMap[h.toLowerCase().replace(/[*\s]/g, '')] = i
        })

        // Parse rows into objects
        const parsed = rows.map((row, idx) => {
          const getCell = (names) => {
            for (const name of names) {
              const cleanName = name.toLowerCase().replace(/[*\s]/g, '')
              if (colMap[cleanName] !== undefined && row[colMap[cleanName]] !== undefined) {
                return String(row[colMap[cleanName]]).trim()
              }
            }
            return ''
          }

          return {
            row: idx + dataStartRow + 1, // Excel row number (1-indexed)
            username: getCell(['username', 'user']),
            full_name: getCell(['fullname', 'name', 'full_name']),
            password: getCell(['password', 'pass']),
            service: getCell(['service', 'plan', 'package']),
            expiry: getCell(['expiry', 'expiry_date', 'expires', 'exp']),
            phone: getCell(['phone', 'mobile', 'tel']),
            address: getCell(['address', 'addr']),
            region: getCell(['region', 'area']),
            building: getCell(['building', 'bldg']),
            nationality: getCell(['nationality', 'nation']),
            country: getCell(['country']),
            mac_address: getCell(['macaddress', 'mac', 'mac_address']),
            note: getCell(['note', 'notes', 'comment']),
            reseller: getCell(['reseller']),
            blocked: getCell(['blocked', 'block', 'status']),
          }
        })

        setParsedData(parsed)
        toast.success(`Parsed ${parsed.length} rows from Excel file`)
      } catch (err) {
        console.error('Error parsing Excel:', err)
        toast.error('Failed to parse Excel file')
        setParsedData([])
      }
    }
    reader.readAsBinaryString(file)
  }

  const handleImport = () => {
    if (parsedData.length === 0) {
      toast.error('No data to import')
      return
    }

    // Validate required fields
    const invalidRows = parsedData.filter(row => !row.username || !row.password || !row.service)
    if (invalidRows.length > 0) {
      toast.error(`${invalidRows.length} rows are missing required fields (Username, Password, Service)`)
      return
    }

    setIsImporting(true)
    importMutation.mutate({
      data: parsedData,
      nas_id: selectedNasId ? parseInt(selectedNasId) : 0,
    })
  }

  const downloadSample = () => {
    window.open('/import_subscribers_sample.xlsx', '_blank')
  }

  const getServiceValidation = (serviceName) => {
    if (!serviceName) return { valid: false, message: 'Required' }
    const found = services.find(s => s.name.toLowerCase() === serviceName.toLowerCase())
    return found ? { valid: true, message: found.name } : { valid: false, message: 'Not found' }
  }

  return (
    <div className="space-y-3" style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Toolbar */}
      <div className="wb-toolbar">
        <button
          onClick={() => navigate('/subscribers')}
          className="btn btn-sm flex items-center gap-1"
        >
          <ArrowLeftIcon className="h-3 w-3" />
          Back
        </button>
        <div className="wb-toolbar-separator" />
        <span className="text-[13px] font-semibold text-gray-900">Import Subscribers</span>
      </div>

      {/* Instructions */}
      <div className="wb-group">
        <div className="wb-group-title">Instructions</div>
        <div className="wb-group-body text-[12px] text-gray-700">
          <ol className="list-decimal list-inside space-y-0.5">
            <li>Download the sample Excel file to see the required format</li>
            <li>Fill in subscriber data starting from row 3 (row 1 is headers, row 2 is description)</li>
            <li>Required fields: Username, Password, Service (must match existing service name)</li>
            <li>Upload your Excel file and review the preview</li>
            <li>Click Import to add all subscribers</li>
          </ol>
        </div>
      </div>

      {/* Step 1: Download Sample */}
      <div className="wb-group">
        <div className="wb-group-title">Step 1: Download Sample File</div>
        <div className="wb-group-body">
          <button
            onClick={downloadSample}
            className="btn btn-primary flex items-center gap-1"
          >
            <ArrowDownTrayIcon className="h-3.5 w-3.5" />
            Download Sample Excel
          </button>
        </div>
      </div>

      {/* Step 2: Upload File */}
      <div className="wb-group">
        <div className="wb-group-title">Step 2: Upload Excel File</div>
        <div className="wb-group-body space-y-3">
          <div className="flex items-center gap-2">
            <input
              type="file"
              ref={fileInputRef}
              onChange={handleFileChange}
              accept=".xlsx,.xls"
              className="hidden"
            />
            <button
              onClick={() => fileInputRef.current?.click()}
              className="btn flex items-center gap-1"
            >
              <ArrowUpTrayIcon className="h-3.5 w-3.5" />
              Choose File
            </button>
            {fileName && (
              <span className="text-[12px] text-gray-600 flex items-center gap-1">
                <DocumentArrowUpIcon className="h-3.5 w-3.5" />
                {fileName}
              </span>
            )}
          </div>

          {/* NAS Selection */}
          <div>
            <label className="label">Default NAS (optional)</label>
            <select
              value={selectedNasId}
              onChange={(e) => setSelectedNasId(e.target.value)}
              className="input max-w-xs"
            >
              <option value="">No default NAS</option>
              {nasDevices.map((nas) => (
                <option key={nas.id} value={nas.id}>
                  {nas.name} ({nas.ip_address})
                </option>
              ))}
            </select>
          </div>
        </div>
      </div>

      {/* Step 3: Preview & Import */}
      {parsedData.length > 0 && (
        <div className="wb-group">
          <div className="wb-group-title flex items-center justify-between">
            <span>Step 3: Review & Import ({parsedData.length} subscribers)</span>
            <button
              onClick={handleImport}
              disabled={isImporting}
              className="btn btn-success btn-sm flex items-center gap-1"
            >
              {isImporting ? (
                <>
                  <svg className="animate-spin h-3 w-3 text-white" fill="none" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                  </svg>
                  Importing...
                </>
              ) : (
                <>
                  <CheckCircleIcon className="h-3.5 w-3.5" />
                  Import All
                </>
              )}
            </button>
          </div>
          <div className="table-container">
            <table className="table">
              <thead>
                <tr>
                  <th style={{ padding: '3px 8px', fontSize: 11 }}>Row</th>
                  <th style={{ padding: '3px 8px', fontSize: 11 }}>Username</th>
                  <th style={{ padding: '3px 8px', fontSize: 11 }}>Name</th>
                  <th style={{ padding: '3px 8px', fontSize: 11 }}>Password</th>
                  <th style={{ padding: '3px 8px', fontSize: 11 }}>Service</th>
                  <th style={{ padding: '3px 8px', fontSize: 11 }}>Expiry</th>
                  <th style={{ padding: '3px 8px', fontSize: 11 }}>Phone</th>
                  <th style={{ padding: '3px 8px', fontSize: 11 }}>MAC</th>
                </tr>
              </thead>
              <tbody>
                {parsedData.slice(0, 100).map((row, idx) => {
                  const serviceValidation = getServiceValidation(row.service)
                  return (
                    <tr key={idx} className={!row.username || !row.password || !serviceValidation.valid ? 'bg-[#ffe0e0]' : ''}>
                      <td style={{ padding: '3px 8px', fontSize: 11 }}>{row.row}</td>
                      <td style={{ padding: '3px 8px', fontSize: 11 }}>
                        {row.username || <span className="text-[#f44336] font-semibold">Missing</span>}
                      </td>
                      <td style={{ padding: '3px 8px', fontSize: 11 }}>{row.full_name}</td>
                      <td style={{ padding: '3px 8px', fontSize: 11 }}>
                        {row.password ? '****' : <span className="text-[#f44336] font-semibold">Missing</span>}
                      </td>
                      <td style={{ padding: '3px 8px', fontSize: 11 }}>
                        {serviceValidation.valid ? (
                          <span className="text-[#4CAF50]">{serviceValidation.message}</span>
                        ) : (
                          <span className="text-[#f44336]">{row.service || 'Missing'} ({serviceValidation.message})</span>
                        )}
                      </td>
                      <td style={{ padding: '3px 8px', fontSize: 11 }}>{row.expiry}</td>
                      <td style={{ padding: '3px 8px', fontSize: 11 }}>{row.phone}</td>
                      <td style={{ padding: '3px 8px', fontSize: 11 }}>{row.mac_address}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
          {parsedData.length > 100 && (
            <div className="wb-statusbar">
              Showing first 100 of {parsedData.length} rows
            </div>
          )}
        </div>
      )}

      {/* Import Results */}
      {importResults && (
        <div className="wb-group">
          <div className="wb-group-title">
            Import Results: {importResults.success} success, {importResults.failed} failed
          </div>
          <div className="table-container" style={{ maxHeight: '384px', overflowY: 'auto' }}>
            <table className="table">
              <thead>
                <tr>
                  <th style={{ padding: '3px 8px', fontSize: 11 }}>Row</th>
                  <th style={{ padding: '3px 8px', fontSize: 11 }}>Username</th>
                  <th style={{ padding: '3px 8px', fontSize: 11 }}>Status</th>
                  <th style={{ padding: '3px 8px', fontSize: 11 }}>Message</th>
                </tr>
              </thead>
              <tbody>
                {importResults.results?.map((result, idx) => (
                  <tr key={idx} className={result.status === 'failed' ? 'bg-[#ffe0e0]' : 'bg-[#e0ffe0]'}>
                    <td style={{ padding: '3px 8px', fontSize: 11 }}>{result.row}</td>
                    <td style={{ padding: '3px 8px', fontSize: 11 }}>{result.username}</td>
                    <td style={{ padding: '3px 8px', fontSize: 11 }}>
                      {result.status === 'success' ? (
                        <span className="badge badge-success">Success</span>
                      ) : (
                        <span className="badge badge-danger">Failed</span>
                      )}
                    </td>
                    <td style={{ padding: '3px 8px', fontSize: 11 }}>{result.message}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="p-2 flex gap-2 border-t border-[#a0a0a0]">
            <button
              onClick={() => {
                setParsedData([])
                setFileName('')
                setImportResults(null)
                if (fileInputRef.current) fileInputRef.current.value = ''
              }}
              className="btn"
            >
              Import More
            </button>
            <button
              onClick={() => navigate('/subscribers')}
              className="btn btn-primary"
            >
              Go to Subscribers
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
