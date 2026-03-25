import { useQuery, useMutation } from '@tanstack/react-query'
import { resellerApi } from '../services/api'
import toast from 'react-hot-toast'

export default function WanCheck() {
  const { data, refetch } = useQuery({
    queryKey: ['reseller-wan-settings'],
    queryFn: () => resellerApi.getSelfWanSettings().then(res => res.data.data),
  })

  const saveMutation = useMutation({
    mutationFn: (d) => resellerApi.updateSelfWanSettings(d),
    onSuccess: () => { toast.success('WAN check settings saved'); refetch() },
    onError: () => toast.error('Failed to save'),
  })

  if (!data) return <div className="p-6 text-gray-500">Loading...</div>

  return (
    <div className="p-4 sm:p-6 max-w-lg">
      <h1 className="text-lg font-bold text-gray-900 dark:text-white mb-4">WAN Management Check</h1>
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-4 space-y-4">
        <p className="text-[11px] text-gray-500 dark:text-gray-400">
          Override the global WAN management check for your subscribers.
        </p>

        <div>
          <label className="block text-[12px] font-semibold text-gray-700 dark:text-gray-300 mb-1">WAN Check Mode</label>
          <select
            value={data.wan_check_enabled === null ? 'global' : data.wan_check_enabled ? 'enabled' : 'disabled'}
            onChange={e => {
              const v = e.target.value
              saveMutation.mutate({
                wan_check_enabled: v === 'global' ? null : v === 'enabled',
                wan_check_icmp: data.wan_check_icmp,
                wan_check_port: data.wan_check_port,
              })
            }}
            className="input text-[12px] w-full"
          >
            <option value="global">Follow Global Setting</option>
            <option value="enabled">Enabled</option>
            <option value="disabled">Disabled</option>
          </select>
        </div>

        {data.wan_check_enabled !== false && (
          <div className="flex gap-6">
            <label className="flex items-center gap-2 text-[12px] text-gray-700 dark:text-gray-300">
              <input type="checkbox" checked={data.wan_check_icmp}
                onChange={e => saveMutation.mutate({
                  wan_check_enabled: data.wan_check_enabled,
                  wan_check_icmp: e.target.checked,
                  wan_check_port: data.wan_check_port,
                })} />
              ICMP Ping Check
            </label>
            <label className="flex items-center gap-2 text-[12px] text-gray-700 dark:text-gray-300">
              <input type="checkbox" checked={data.wan_check_port}
                onChange={e => saveMutation.mutate({
                  wan_check_enabled: data.wan_check_enabled,
                  wan_check_icmp: data.wan_check_icmp,
                  wan_check_port: e.target.checked,
                })} />
              WAN Port Check
            </label>
          </div>
        )}
      </div>
    </div>
  )
}
