import { useState, useEffect, useRef } from 'react'
import { copyToClipboard } from '../utils/clipboard'

const SECTIONS = [
  { id: 'getting-started', label: 'Getting Started' },
  { id: 'authentication', label: 'Authentication' },
  { id: 'response-format', label: 'Response Format' },
  { id: 'subscribers', label: 'Subscribers' },
  { id: 'services', label: 'Services' },
  { id: 'nas', label: 'NAS Devices' },
  { id: 'transactions', label: 'Transactions' },
  { id: 'system', label: 'System' },
]

const METHOD_COLORS = {
  GET: 'bg-green-500',
  POST: 'bg-blue-500',
  PUT: 'bg-orange-500',
  DELETE: 'bg-red-500',
}

function MethodBadge({ method }) {
  return (
    <span className={`inline-block px-2 py-0.5 text-[11px] font-bold text-white rounded ${METHOD_COLORS[method] || 'bg-gray-500'}`}>
      {method}
    </span>
  )
}

function CodeBlock({ children, lang }) {
  const [copied, setCopied] = useState(false)
  const copy = () => {
    copyToClipboard(children).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    }).catch(() => {})
  }
  return (
    <div className="relative group">
      <pre className="bg-gray-900 text-gray-100 p-4 rounded-lg text-[12px] overflow-x-auto leading-relaxed">
        <code>{children}</code>
      </pre>
      <button onClick={copy} className="absolute top-2 right-2 px-2 py-1 text-[10px] bg-gray-700 text-gray-300 rounded opacity-0 group-hover:opacity-100 transition-opacity hover:bg-gray-600">
        {copied ? 'Copied!' : 'Copy'}
      </button>
    </div>
  )
}

function ParamTable({ params }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-[12px] border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
        <thead>
          <tr className="bg-gray-50 dark:bg-gray-800">
            <th className="text-left px-3 py-2 font-semibold text-gray-700 dark:text-gray-300">Parameter</th>
            <th className="text-left px-3 py-2 font-semibold text-gray-700 dark:text-gray-300">Type</th>
            <th className="text-left px-3 py-2 font-semibold text-gray-700 dark:text-gray-300">Required</th>
            <th className="text-left px-3 py-2 font-semibold text-gray-700 dark:text-gray-300">Description</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
          {params.map(p => (
            <tr key={p.name}>
              <td className="px-3 py-2 font-mono text-[11px] text-blue-600 dark:text-blue-400">{p.name}</td>
              <td className="px-3 py-2 text-gray-600 dark:text-gray-400">{p.type}</td>
              <td className="px-3 py-2">{p.required ? <span className="text-red-500 font-semibold">Yes</span> : <span className="text-gray-400">No</span>}</td>
              <td className="px-3 py-2 text-gray-600 dark:text-gray-400">{p.desc}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function Endpoint({ method, path, description, params, curlExample, jsExample, responseExample, scope }) {
  const [tab, setTab] = useState('curl')
  return (
    <div className="mb-8 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
      <div className="p-4 bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700">
        <div className="flex items-center gap-3 mb-1">
          <MethodBadge method={method} />
          <code className="text-[13px] font-semibold text-gray-900 dark:text-white font-mono">{path}</code>
          {scope && <span className="text-[10px] px-1.5 py-0.5 bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-400 rounded font-medium">scope: {scope}</span>}
        </div>
        <p className="text-[12px] text-gray-600 dark:text-gray-400 mt-1">{description}</p>
      </div>
      <div className="p-4 space-y-4 bg-gray-50 dark:bg-gray-900/50">
        {params && params.length > 0 && (
          <div>
            <h4 className="text-[11px] font-semibold text-gray-700 dark:text-gray-300 mb-2 uppercase tracking-wider">Parameters</h4>
            <ParamTable params={params} />
          </div>
        )}
        <div>
          <div className="flex gap-1 mb-2">
            <button onClick={() => setTab('curl')} className={`px-3 py-1 text-[11px] rounded ${tab === 'curl' ? 'bg-gray-900 text-white' : 'bg-gray-200 dark:bg-gray-700 text-gray-600 dark:text-gray-400'}`}>cURL</button>
            <button onClick={() => setTab('js')} className={`px-3 py-1 text-[11px] rounded ${tab === 'js' ? 'bg-gray-900 text-white' : 'bg-gray-200 dark:bg-gray-700 text-gray-600 dark:text-gray-400'}`}>JavaScript</button>
          </div>
          <CodeBlock>{tab === 'curl' ? curlExample : jsExample}</CodeBlock>
        </div>
        {responseExample && (
          <div>
            <h4 className="text-[11px] font-semibold text-gray-700 dark:text-gray-300 mb-2 uppercase tracking-wider">Response</h4>
            <CodeBlock>{responseExample}</CodeBlock>
          </div>
        )}
      </div>
    </div>
  )
}

export default function ApiDocs() {
  const [activeSection, setActiveSection] = useState('getting-started')
  const contentRef = useRef(null)

  useEffect(() => {
    const hash = window.location.hash.slice(1)
    if (hash && SECTIONS.find(s => s.id === hash)) {
      setActiveSection(hash)
      document.getElementById(hash)?.scrollIntoView({ behavior: 'smooth' })
    }
  }, [])

  const scrollTo = (id) => {
    setActiveSection(id)
    document.getElementById(id)?.scrollIntoView({ behavior: 'smooth' })
    window.history.replaceState(null, '', `#${id}`)
  }

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-950">
      {/* Header */}
      <div className="bg-white dark:bg-gray-900 border-b border-gray-200 dark:border-gray-800 sticky top-0 z-20">
        <div className="max-w-7xl mx-auto px-4 py-3 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 bg-blue-600 rounded-lg flex items-center justify-center">
              <span className="text-white font-bold text-[14px]">P</span>
            </div>
            <div>
              <h1 className="text-[15px] font-bold text-gray-900 dark:text-white">ProxPanel API</h1>
              <p className="text-[11px] text-gray-500 dark:text-gray-400">v1 Documentation</p>
            </div>
          </div>
          <a href="/" className="text-[12px] text-blue-600 dark:text-blue-400 hover:underline">Back to Panel</a>
        </div>
      </div>

      <div className="max-w-7xl mx-auto flex">
        {/* Sidebar */}
        <nav className="w-56 flex-shrink-0 hidden md:block sticky top-[53px] h-[calc(100vh-53px)] overflow-y-auto border-r border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 p-4">
          <ul className="space-y-1">
            {SECTIONS.map(s => (
              <li key={s.id}>
                <button
                  onClick={() => scrollTo(s.id)}
                  className={`w-full text-left px-3 py-1.5 text-[12px] rounded transition-colors ${
                    activeSection === s.id
                      ? 'bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-400 font-semibold'
                      : 'text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-800'
                  }`}
                >
                  {s.label}
                </button>
              </li>
            ))}
          </ul>
        </nav>

        {/* Main Content */}
        <main ref={contentRef} className="flex-1 min-w-0 p-6 md:p-8 max-w-4xl">
          {/* Getting Started */}
          <section id="getting-started" className="mb-12">
            <h2 className="text-xl font-bold text-gray-900 dark:text-white mb-3">Getting Started</h2>
            <p className="text-[13px] text-gray-600 dark:text-gray-400 mb-4 leading-relaxed">
              The ProxPanel API allows you to integrate your ISP management system with external applications such as CRM, billing, and accounting software.
              All API endpoints are available under the <code className="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-800 rounded text-[12px] font-mono">/api/v1/external/</code> base path.
            </p>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <div className="card p-4">
                <h3 className="text-[12px] font-semibold text-gray-900 dark:text-white mb-1">Base URL</h3>
                <code className="text-[11px] text-blue-600 dark:text-blue-400 font-mono">https://your-server/api/v1/external</code>
              </div>
              <div className="card p-4">
                <h3 className="text-[12px] font-semibold text-gray-900 dark:text-white mb-1">Rate Limit</h3>
                <p className="text-[11px] text-gray-600 dark:text-gray-400">60 requests per minute per key</p>
              </div>
              <div className="card p-4">
                <h3 className="text-[12px] font-semibold text-gray-900 dark:text-white mb-1">Format</h3>
                <p className="text-[11px] text-gray-600 dark:text-gray-400">JSON request & response bodies</p>
              </div>
            </div>
          </section>

          {/* Authentication */}
          <section id="authentication" className="mb-12">
            <h2 className="text-xl font-bold text-gray-900 dark:text-white mb-3">Authentication</h2>
            <p className="text-[13px] text-gray-600 dark:text-gray-400 mb-4 leading-relaxed">
              All API requests require an API key passed via the <code className="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-800 rounded text-[12px] font-mono">X-API-Key</code> header.
              Generate keys from <strong>Settings &rarr; API Keys</strong> in your admin panel.
            </p>
            <CodeBlock>{`curl -H "X-API-Key: pk_live_your_key_here" \\
  https://your-server/api/v1/external/subscribers`}</CodeBlock>
            <div className="mt-4 space-y-2">
              <h3 className="text-[13px] font-semibold text-gray-900 dark:text-white">Scopes</h3>
              <p className="text-[12px] text-gray-600 dark:text-gray-400">Each API key has one or more scopes that control access:</p>
              <div className="flex gap-2 flex-wrap">
                <span className="px-2 py-1 text-[11px] bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 rounded font-medium">read — List and retrieve resources</span>
                <span className="px-2 py-1 text-[11px] bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400 rounded font-medium">write — Create and update resources</span>
                <span className="px-2 py-1 text-[11px] bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400 rounded font-medium">delete — Delete resources</span>
              </div>
            </div>
          </section>

          {/* Response Format */}
          <section id="response-format" className="mb-12">
            <h2 className="text-xl font-bold text-gray-900 dark:text-white mb-3">Response Format</h2>
            <p className="text-[13px] text-gray-600 dark:text-gray-400 mb-4">All responses follow a consistent JSON envelope format.</p>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <h4 className="text-[12px] font-semibold text-green-600 mb-2">Success Response</h4>
                <CodeBlock>{`{
  "success": true,
  "data": { ... },
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 150,
    "pages": 8
  },
  "timestamp": "2026-03-20T12:00:00Z"
}`}</CodeBlock>
              </div>
              <div>
                <h4 className="text-[12px] font-semibold text-red-600 mb-2">Error Response</h4>
                <CodeBlock>{`{
  "success": false,
  "error": {
    "code": "INVALID_PARAMETER",
    "message": "subscriber_id is required"
  },
  "timestamp": "2026-03-20T12:00:00Z"
}`}</CodeBlock>
              </div>
            </div>
            <div className="mt-4">
              <h4 className="text-[12px] font-semibold text-gray-900 dark:text-white mb-2">HTTP Status Codes</h4>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-2 text-[11px]">
                <div className="card p-2"><span className="font-mono text-green-600">200</span> — Success</div>
                <div className="card p-2"><span className="font-mono text-green-600">201</span> — Created</div>
                <div className="card p-2"><span className="font-mono text-red-600">400</span> — Bad Request</div>
                <div className="card p-2"><span className="font-mono text-red-600">401</span> — Unauthorized</div>
                <div className="card p-2"><span className="font-mono text-red-600">403</span> — Forbidden</div>
                <div className="card p-2"><span className="font-mono text-red-600">404</span> — Not Found</div>
                <div className="card p-2"><span className="font-mono text-orange-600">429</span> — Rate Limited</div>
                <div className="card p-2"><span className="font-mono text-red-600">500</span> — Server Error</div>
              </div>
            </div>
          </section>

          {/* Subscribers */}
          <section id="subscribers" className="mb-12">
            <h2 className="text-xl font-bold text-gray-900 dark:text-white mb-3">Subscribers</h2>
            <p className="text-[13px] text-gray-600 dark:text-gray-400 mb-6">Manage PPPoE subscribers — list, create, update, suspend, and more.</p>

            <Endpoint
              method="GET" path="/subscribers" scope="read"
              description="List subscribers with pagination and optional filters."
              params={[
                { name: 'page', type: 'integer', required: false, desc: 'Page number (default: 1)' },
                { name: 'limit', type: 'integer', required: false, desc: 'Results per page (1-100, default: 20)' },
                { name: 'username', type: 'string', required: false, desc: 'Filter by username (partial match)' },
                { name: 'status', type: 'integer', required: false, desc: 'Filter by status (1=Active, 2=Inactive, 3=Expired, 4=Stopped)' },
                { name: 'service_id', type: 'integer', required: false, desc: 'Filter by service ID' },
                { name: 'nas_id', type: 'integer', required: false, desc: 'Filter by NAS device ID' },
                { name: 'is_online', type: 'boolean', required: false, desc: 'Filter by online status' },
              ]}
              curlExample={`curl -H "X-API-Key: pk_live_your_key" \\
  "https://your-server/api/v1/external/subscribers?page=1&limit=20&status=1"`}
              jsExample={`const response = await fetch(
  'https://your-server/api/v1/external/subscribers?page=1&limit=20',
  { headers: { 'X-API-Key': 'pk_live_your_key' } }
);
const data = await response.json();`}
              responseExample={`{
  "success": true,
  "data": [
    {
      "id": 1,
      "username": "user@domain",
      "full_name": "John Doe",
      "phone": "+961123456",
      "status": 1,
      "service_id": 3,
      "is_online": true,
      "ip_address": "10.0.0.100",
      "expiry_date": "2026-04-20T00:00:00Z",
      "created_at": "2025-01-15T10:00:00Z"
    }
  ],
  "pagination": { "page": 1, "limit": 20, "total": 350, "pages": 18 },
  "timestamp": "2026-03-20T12:00:00Z"
}`}
            />

            <Endpoint
              method="GET" path="/subscribers/:id" scope="read"
              description="Get a single subscriber by ID with full details."
              params={[{ name: 'id', type: 'integer', required: true, desc: 'Subscriber ID' }]}
              curlExample={`curl -H "X-API-Key: pk_live_your_key" \\
  https://your-server/api/v1/external/subscribers/42`}
              jsExample={`const response = await fetch(
  'https://your-server/api/v1/external/subscribers/42',
  { headers: { 'X-API-Key': 'pk_live_your_key' } }
);
const data = await response.json();`}
              responseExample={`{
  "success": true,
  "data": {
    "id": 42,
    "username": "user@domain",
    "full_name": "John Doe",
    "phone": "+961123456",
    "status": 1,
    "service_id": 3,
    "service_name": "8MB-20GB",
    "is_online": true,
    "ip_address": "10.0.0.100",
    "mac_address": "AA:BB:CC:DD:EE:FF",
    "expiry_date": "2026-04-20T00:00:00Z",
    "balance": 25.50,
    "daily_download_used": 1073741824,
    "monthly_download_used": 10737418240,
    "created_at": "2025-01-15T10:00:00Z"
  },
  "timestamp": "2026-03-20T12:00:00Z"
}`}
            />

            <Endpoint
              method="GET" path="/subscribers/by-username/:username" scope="read"
              description="Look up a subscriber by their PPPoE username."
              params={[{ name: 'username', type: 'string', required: true, desc: 'PPPoE username (e.g. user@domain)' }]}
              curlExample={`curl -H "X-API-Key: pk_live_your_key" \\
  https://your-server/api/v1/external/subscribers/by-username/john@isp.com`}
              jsExample={`const response = await fetch(
  'https://your-server/api/v1/external/subscribers/by-username/john@isp.com',
  { headers: { 'X-API-Key': 'pk_live_your_key' } }
);
const data = await response.json();`}
            />

            <Endpoint
              method="POST" path="/subscribers" scope="write"
              description="Create a new subscriber."
              params={[
                { name: 'username', type: 'string', required: true, desc: 'PPPoE username' },
                { name: 'password', type: 'string', required: true, desc: 'PPPoE password' },
                { name: 'service_id', type: 'integer', required: true, desc: 'Service plan ID' },
                { name: 'full_name', type: 'string', required: false, desc: 'Full name' },
                { name: 'phone', type: 'string', required: false, desc: 'Phone number' },
                { name: 'address', type: 'string', required: false, desc: 'Address' },
                { name: 'expiry_date', type: 'string', required: false, desc: 'Expiry date (YYYY-MM-DD). Defaults to 1 month.' },
              ]}
              curlExample={`curl -X POST -H "X-API-Key: pk_live_your_key" \\
  -H "Content-Type: application/json" \\
  -d '{"username":"newuser@isp.com","password":"secret123","service_id":3,"full_name":"Jane Doe"}' \\
  https://your-server/api/v1/external/subscribers`}
              jsExample={`const response = await fetch(
  'https://your-server/api/v1/external/subscribers',
  {
    method: 'POST',
    headers: {
      'X-API-Key': 'pk_live_your_key',
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      username: 'newuser@isp.com',
      password: 'secret123',
      service_id: 3,
      full_name: 'Jane Doe',
    }),
  }
);
const data = await response.json();`}
              responseExample={`{
  "success": true,
  "data": {
    "id": 351,
    "username": "newuser@isp.com",
    "status": 1,
    "service_id": 3,
    "expiry_date": "2026-04-20T00:00:00Z",
    "created_at": "2026-03-20T12:00:00Z"
  },
  "timestamp": "2026-03-20T12:00:00Z"
}`}
            />

            <Endpoint
              method="PUT" path="/subscribers/:id" scope="write"
              description="Update an existing subscriber's details."
              params={[
                { name: 'id', type: 'integer', required: true, desc: 'Subscriber ID (URL)' },
                { name: 'full_name', type: 'string', required: false, desc: 'Full name' },
                { name: 'phone', type: 'string', required: false, desc: 'Phone number' },
                { name: 'address', type: 'string', required: false, desc: 'Address' },
                { name: 'service_id', type: 'integer', required: false, desc: 'Service plan ID' },
                { name: 'password', type: 'string', required: false, desc: 'New PPPoE password' },
                { name: 'expiry_date', type: 'string', required: false, desc: 'New expiry date (YYYY-MM-DD)' },
              ]}
              curlExample={`curl -X PUT -H "X-API-Key: pk_live_your_key" \\
  -H "Content-Type: application/json" \\
  -d '{"full_name":"Jane Smith","phone":"+961999888"}' \\
  https://your-server/api/v1/external/subscribers/42`}
              jsExample={`const response = await fetch(
  'https://your-server/api/v1/external/subscribers/42',
  {
    method: 'PUT',
    headers: {
      'X-API-Key': 'pk_live_your_key',
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      full_name: 'Jane Smith',
      phone: '+961999888',
    }),
  }
);
const data = await response.json();`}
            />

            <Endpoint
              method="DELETE" path="/subscribers/:id" scope="delete"
              description="Soft-delete a subscriber. The subscriber will be marked as deleted but not permanently removed."
              params={[{ name: 'id', type: 'integer', required: true, desc: 'Subscriber ID' }]}
              curlExample={`curl -X DELETE -H "X-API-Key: pk_live_your_key" \\
  https://your-server/api/v1/external/subscribers/42`}
              jsExample={`const response = await fetch(
  'https://your-server/api/v1/external/subscribers/42',
  {
    method: 'DELETE',
    headers: { 'X-API-Key': 'pk_live_your_key' },
  }
);
const data = await response.json();`}
            />

            <Endpoint
              method="POST" path="/subscribers/:id/suspend" scope="write"
              description="Suspend an active subscriber. Sets status to Inactive."
              params={[{ name: 'id', type: 'integer', required: true, desc: 'Subscriber ID' }]}
              curlExample={`curl -X POST -H "X-API-Key: pk_live_your_key" \\
  https://your-server/api/v1/external/subscribers/42/suspend`}
              jsExample={`const response = await fetch(
  'https://your-server/api/v1/external/subscribers/42/suspend',
  {
    method: 'POST',
    headers: { 'X-API-Key': 'pk_live_your_key' },
  }
);
const data = await response.json();`}
            />

            <Endpoint
              method="POST" path="/subscribers/:id/activate" scope="write"
              description="Activate a suspended subscriber. Sets status to Active."
              params={[{ name: 'id', type: 'integer', required: true, desc: 'Subscriber ID' }]}
              curlExample={`curl -X POST -H "X-API-Key: pk_live_your_key" \\
  https://your-server/api/v1/external/subscribers/42/activate`}
              jsExample={`const response = await fetch(
  'https://your-server/api/v1/external/subscribers/42/activate',
  {
    method: 'POST',
    headers: { 'X-API-Key': 'pk_live_your_key' },
  }
);
const data = await response.json();`}
            />

            <Endpoint
              method="GET" path="/subscribers/:id/usage" scope="read"
              description="Get current bandwidth usage statistics for a subscriber."
              params={[{ name: 'id', type: 'integer', required: true, desc: 'Subscriber ID' }]}
              curlExample={`curl -H "X-API-Key: pk_live_your_key" \\
  https://your-server/api/v1/external/subscribers/42/usage`}
              jsExample={`const response = await fetch(
  'https://your-server/api/v1/external/subscribers/42/usage',
  { headers: { 'X-API-Key': 'pk_live_your_key' } }
);
const data = await response.json();`}
              responseExample={`{
  "success": true,
  "data": {
    "daily_download_used": 1073741824,
    "daily_upload_used": 268435456,
    "monthly_download_used": 10737418240,
    "monthly_upload_used": 2684354560,
    "daily_quota": 21474836480,
    "monthly_quota": 107374182400,
    "fup_level": 0
  },
  "timestamp": "2026-03-20T12:00:00Z"
}`}
            />
          </section>

          {/* Services */}
          <section id="services" className="mb-12">
            <h2 className="text-xl font-bold text-gray-900 dark:text-white mb-3">Services</h2>
            <p className="text-[13px] text-gray-600 dark:text-gray-400 mb-6">Retrieve available service plans and their details.</p>

            <Endpoint
              method="GET" path="/services" scope="read"
              description="List all available service plans."
              curlExample={`curl -H "X-API-Key: pk_live_your_key" \\
  https://your-server/api/v1/external/services`}
              jsExample={`const response = await fetch(
  'https://your-server/api/v1/external/services',
  { headers: { 'X-API-Key': 'pk_live_your_key' } }
);
const data = await response.json();`}
              responseExample={`{
  "success": true,
  "data": [
    {
      "id": 3,
      "name": "8MB-20GB",
      "download_speed": 12000,
      "upload_speed": 6000,
      "price": 25.00,
      "daily_quota": 21474836480,
      "monthly_quota": 107374182400
    }
  ],
  "timestamp": "2026-03-20T12:00:00Z"
}`}
            />

            <Endpoint
              method="GET" path="/services/:id" scope="read"
              description="Get a single service plan by ID."
              params={[{ name: 'id', type: 'integer', required: true, desc: 'Service ID' }]}
              curlExample={`curl -H "X-API-Key: pk_live_your_key" \\
  https://your-server/api/v1/external/services/3`}
              jsExample={`const response = await fetch(
  'https://your-server/api/v1/external/services/3',
  { headers: { 'X-API-Key': 'pk_live_your_key' } }
);
const data = await response.json();`}
            />
          </section>

          {/* NAS Devices */}
          <section id="nas" className="mb-12">
            <h2 className="text-xl font-bold text-gray-900 dark:text-white mb-3">NAS Devices</h2>
            <p className="text-[13px] text-gray-600 dark:text-gray-400 mb-6">Retrieve NAS (MikroTik router) information.</p>

            <Endpoint
              method="GET" path="/nas" scope="read"
              description="List all NAS devices."
              curlExample={`curl -H "X-API-Key: pk_live_your_key" \\
  https://your-server/api/v1/external/nas`}
              jsExample={`const response = await fetch(
  'https://your-server/api/v1/external/nas',
  { headers: { 'X-API-Key': 'pk_live_your_key' } }
);
const data = await response.json();`}
              responseExample={`{
  "success": true,
  "data": [
    {
      "id": 1,
      "name": "Main Router",
      "ip_address": "10.0.0.1",
      "is_online": true,
      "active_sessions": 285
    }
  ],
  "timestamp": "2026-03-20T12:00:00Z"
}`}
            />

            <Endpoint
              method="GET" path="/nas/:id" scope="read"
              description="Get a single NAS device by ID, including active session count."
              params={[{ name: 'id', type: 'integer', required: true, desc: 'NAS device ID' }]}
              curlExample={`curl -H "X-API-Key: pk_live_your_key" \\
  https://your-server/api/v1/external/nas/1`}
              jsExample={`const response = await fetch(
  'https://your-server/api/v1/external/nas/1',
  { headers: { 'X-API-Key': 'pk_live_your_key' } }
);
const data = await response.json();`}
            />
          </section>

          {/* Transactions */}
          <section id="transactions" className="mb-12">
            <h2 className="text-xl font-bold text-gray-900 dark:text-white mb-3">Transactions</h2>
            <p className="text-[13px] text-gray-600 dark:text-gray-400 mb-6">View and create billing transactions (payments, charges, renewals).</p>

            <Endpoint
              method="GET" path="/transactions" scope="read"
              description="List transactions with pagination and optional filters."
              params={[
                { name: 'page', type: 'integer', required: false, desc: 'Page number (default: 1)' },
                { name: 'limit', type: 'integer', required: false, desc: 'Results per page (1-100, default: 20)' },
                { name: 'subscriber_id', type: 'integer', required: false, desc: 'Filter by subscriber ID' },
                { name: 'type', type: 'string', required: false, desc: 'Filter by type (renewal, new, payment, etc.)' },
                { name: 'date_from', type: 'string', required: false, desc: 'Start date (YYYY-MM-DD)' },
                { name: 'date_to', type: 'string', required: false, desc: 'End date (YYYY-MM-DD)' },
              ]}
              curlExample={`curl -H "X-API-Key: pk_live_your_key" \\
  "https://your-server/api/v1/external/transactions?subscriber_id=42&date_from=2026-03-01"`}
              jsExample={`const response = await fetch(
  'https://your-server/api/v1/external/transactions?subscriber_id=42',
  { headers: { 'X-API-Key': 'pk_live_your_key' } }
);
const data = await response.json();`}
              responseExample={`{
  "success": true,
  "data": [
    {
      "id": 1050,
      "subscriber_id": 42,
      "type": "renewal",
      "amount": 25.00,
      "balance_before": 50.00,
      "balance_after": 25.00,
      "description": "Service renewal - 8MB-20GB",
      "created_at": "2026-03-15T10:00:00Z"
    }
  ],
  "pagination": { "page": 1, "limit": 20, "total": 5, "pages": 1 },
  "timestamp": "2026-03-20T12:00:00Z"
}`}
            />

            <Endpoint
              method="POST" path="/transactions" scope="write"
              description="Create a new transaction (payment, charge, etc.)."
              params={[
                { name: 'subscriber_id', type: 'integer', required: true, desc: 'Subscriber ID' },
                { name: 'type', type: 'string', required: true, desc: 'Transaction type (payment, charge, renewal, refund)' },
                { name: 'amount', type: 'number', required: true, desc: 'Transaction amount' },
                { name: 'description', type: 'string', required: false, desc: 'Description' },
              ]}
              curlExample={`curl -X POST -H "X-API-Key: pk_live_your_key" \\
  -H "Content-Type: application/json" \\
  -d '{"subscriber_id":42,"type":"payment","amount":25.00,"description":"Monthly payment"}' \\
  https://your-server/api/v1/external/transactions`}
              jsExample={`const response = await fetch(
  'https://your-server/api/v1/external/transactions',
  {
    method: 'POST',
    headers: {
      'X-API-Key': 'pk_live_your_key',
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      subscriber_id: 42,
      type: 'payment',
      amount: 25.00,
      description: 'Monthly payment',
    }),
  }
);
const data = await response.json();`}
            />
          </section>

          {/* System */}
          <section id="system" className="mb-12">
            <h2 className="text-xl font-bold text-gray-900 dark:text-white mb-3">System</h2>
            <p className="text-[13px] text-gray-600 dark:text-gray-400 mb-6">System-wide statistics and health checks.</p>

            <Endpoint
              method="GET" path="/system/stats" scope="read"
              description="Get system-wide statistics: online users, total subscribers, NAS count."
              curlExample={`curl -H "X-API-Key: pk_live_your_key" \\
  https://your-server/api/v1/external/system/stats`}
              jsExample={`const response = await fetch(
  'https://your-server/api/v1/external/system/stats',
  { headers: { 'X-API-Key': 'pk_live_your_key' } }
);
const data = await response.json();`}
              responseExample={`{
  "success": true,
  "data": {
    "online_users": 285,
    "total_subscribers": 1500,
    "active_subscribers": 1200,
    "nas_count": 3
  },
  "timestamp": "2026-03-20T12:00:00Z"
}`}
            />

            <Endpoint
              method="GET" path="/system/health"
              description="API health check. No authentication required for this endpoint within the API key group, but the key header is still needed."
              curlExample={`curl -H "X-API-Key: pk_live_your_key" \\
  https://your-server/api/v1/external/system/health`}
              jsExample={`const response = await fetch(
  'https://your-server/api/v1/external/system/health',
  { headers: { 'X-API-Key': 'pk_live_your_key' } }
);
const data = await response.json();`}
              responseExample={`{
  "success": true,
  "data": {
    "status": "healthy",
    "database": "connected",
    "timestamp": "2026-03-20T12:00:00Z"
  },
  "timestamp": "2026-03-20T12:00:00Z"
}`}
            />
          </section>

          {/* Footer */}
          <div className="mt-16 pt-6 border-t border-gray-200 dark:border-gray-800 text-center">
            <p className="text-[11px] text-gray-400 dark:text-gray-500">
              ProxPanel API Documentation &bull; Need help? Contact your system administrator.
            </p>
          </div>
        </main>
      </div>
    </div>
  )
}
