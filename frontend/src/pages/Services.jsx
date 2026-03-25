import { useState, useMemo, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { serviceApi, cdnApi, nasApi } from '../services/api'
import { useAuthStore } from '../store/authStore'
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  flexRender,
} from '@tanstack/react-table'
import {
  PlusIcon,
  PencilIcon,
  TrashIcon,
  XMarkIcon,
  GlobeAltIcon,
  ClockIcon,
  ChevronUpIcon,
  ChevronDownIcon,
  ChevronUpDownIcon,
} from '@heroicons/react/24/outline'
import toast from 'react-hot-toast'
import clsx from 'clsx'

export default function Services() {
  const queryClient = useQueryClient()
  const { hasPermission } = useAuthStore()
  const [showModal, setShowModal] = useState(false)
  const [editingService, setEditingService] = useState(null)
  const [expandedFUP, setExpandedFUP] = useState({})
  const [sorting, setSorting] = useState(() => {
    try {
      const saved = localStorage.getItem('services_sorting')
      return saved ? JSON.parse(saved) : []
    } catch { return [] }
  })

  useEffect(() => {
    localStorage.setItem('services_sorting', JSON.stringify(sorting))
  }, [sorting])
  const [formData, setFormData] = useState({
    name: '',
    description: '',
    download_speed: '',
    upload_speed: '',
    price: '',
    validity_days: '30',
    daily_quota: '',
    monthly_quota: '',
    burst_download: '',
    burst_upload: '',
    burst_threshold: '',
    burst_time: '',
    priority: '8',
    // Daily FUP (resets every day)
    fup1_threshold: '',
    fup1_download_speed: '',
    fup1_upload_speed: '',
    fup2_threshold: '',
    fup2_download_speed: '',
    fup2_upload_speed: '',
    fup3_threshold: '',
    fup3_download_speed: '',
    fup3_upload_speed: '',
    fup4_threshold: '',
    fup4_download_speed: '',
    fup4_upload_speed: '',
    fup5_threshold: '',
    fup5_download_speed: '',
    fup5_upload_speed: '',
    fup6_threshold: '',
    fup6_download_speed: '',
    fup6_upload_speed: '',
    // Monthly FUP (resets on renew)
    monthly_fup1_threshold: '',
    monthly_fup1_download_speed: '',
    monthly_fup1_upload_speed: '',
    monthly_fup2_threshold: '',
    monthly_fup2_download_speed: '',
    monthly_fup2_upload_speed: '',
    monthly_fup3_threshold: '',
    monthly_fup3_download_speed: '',
    monthly_fup3_upload_speed: '',
    monthly_fup4_threshold: '',
    monthly_fup4_download_speed: '',
    monthly_fup4_upload_speed: '',
    monthly_fup5_threshold: '',
    monthly_fup5_download_speed: '',
    monthly_fup5_upload_speed: '',
    monthly_fup6_threshold: '',
    monthly_fup6_download_speed: '',
    monthly_fup6_upload_speed: '',
    // CDN FUP
    cdn_fup_enabled: false,
    cdn_fup1_threshold: '',
    cdn_fup1_download_speed: '',
    cdn_fup1_upload_speed: '',
    cdn_fup2_threshold: '',
    cdn_fup2_download_speed: '',
    cdn_fup2_upload_speed: '',
    cdn_fup3_threshold: '',
    cdn_fup3_download_speed: '',
    cdn_fup3_upload_speed: '',
    cdn_monthly_fup1_threshold: '',
    cdn_monthly_fup1_download_speed: '',
    cdn_monthly_fup1_upload_speed: '',
    cdn_monthly_fup2_threshold: '',
    cdn_monthly_fup2_download_speed: '',
    cdn_monthly_fup2_upload_speed: '',
    cdn_monthly_fup3_threshold: '',
    cdn_monthly_fup3_download_speed: '',
    cdn_monthly_fup3_upload_speed: '',
    is_active: true,
    // Time-based speed control (12-hour format)
    time_based_speed_enabled: false,
    time_from_hour: '12',
    time_from_minute: '0',
    time_from_ampm: 'AM',
    time_to_hour: '12',
    time_to_minute: '0',
    time_to_ampm: 'AM',
    time_download_ratio: '100',
    time_upload_ratio: '100',
    // MikroTik/RADIUS settings
    nas_id: null,
    pool_name: '',
    address_list_in: '',
    address_list_out: '',
    queue_type: 'simple',
  })

  const [serviceCDNs, setServiceCDNs] = useState([])
  const [cdnsLoaded, setCdnsLoaded] = useState(false)

  // IP Pool state for RADIUS settings
  const [selectedNasId, setSelectedNasId] = useState(null)
  const [ipPools, setIpPools] = useState([])
  const [ipPoolsLoading, setIpPoolsLoading] = useState(false)

  const { data: services, isLoading } = useQuery({
    queryKey: ['services'],
    queryFn: () => serviceApi.list().then((r) => r.data.data),
  })

  const { data: cdnList } = useQuery({
    queryKey: ['cdns'],
    queryFn: () => cdnApi.list({ active: 'true' }).then((r) => r.data.data),
  })

  const { data: nasList } = useQuery({
    queryKey: ['nas-list'],
    queryFn: () => nasApi.list().then((r) => r.data.data),
  })

  // PCQ pool state: { [cdn_id]: { loading: bool, pools: [], selected: [] } }
  const [pcqPools, setPcqPools] = useState({})

  const fetchPoolsForCDN = async (cdnId, nasId) => {
    if (!nasId) return
    setPcqPools(prev => ({ ...prev, [cdnId]: { ...prev[cdnId], loading: true } }))
    try {
      const response = await nasApi.getPools(nasId)
      const pools = response.data.data || []
      setPcqPools(prev => ({ ...prev, [cdnId]: { loading: false, pools } }))
      // Auto-select pool matching service's pool_name if no real range selected yet
      const poolName = formData.pool_name
      if (poolName) {
        const match = pools.find(p => p.name === poolName)
        if (match) {
          setServiceCDNs(prev => prev.map(sc => {
            if (sc.cdn_id !== cdnId) return sc
            // Only override if current value is not already a CIDR range
            const hasRange = sc.pcq_target_pools && sc.pcq_target_pools.includes('/')
            if (!hasRange) {
              return { ...sc, pcq_target_pools: match.ranges }
            }
            return sc
          }))
        }
      }
    } catch (err) {
      console.error('Failed to fetch pools:', err)
      setPcqPools(prev => ({ ...prev, [cdnId]: { loading: false, pools: [] } }))
    }
  }

  // Fetch IP pools for RADIUS settings when NAS is selected
  const fetchIPPoolsForService = async (nasId) => {
    if (!nasId) {
      setIpPools([])
      return
    }
    setIpPoolsLoading(true)
    try {
      const response = await nasApi.getPools(nasId)
      const pools = response.data.data || []
      setIpPools(pools)
    } catch (err) {
      console.error('Failed to fetch IP pools:', err)
      setIpPools([])
      toast.error('Failed to fetch IP pools from NAS')
    } finally {
      setIpPoolsLoading(false)
    }
  }

  const saveMutation = useMutation({
    mutationFn: async (data) => {
      let result
      // Convert CDN time from 12h AM/PM to 24h format
      const cdnsFor24h = serviceCDNs.map(sc => {
        let fromHour24 = parseInt(sc.time_from_hour) || 12
        if (sc.time_from_ampm === 'PM' && fromHour24 !== 12) fromHour24 += 12
        if (sc.time_from_ampm === 'AM' && fromHour24 === 12) fromHour24 = 0

        let toHour24 = parseInt(sc.time_to_hour) || 12
        if (sc.time_to_ampm === 'PM' && toHour24 !== 12) toHour24 += 12
        if (sc.time_to_ampm === 'AM' && toHour24 === 12) toHour24 = 0

        return {
          cdn_id: sc.cdn_id,
          speed_limit: sc.speed_limit,
          bypass_quota: sc.bypass_quota,
          pcq_enabled: sc.pcq_enabled || false,
          pcq_limit: parseInt(sc.pcq_limit) || 50,
          pcq_total_limit: parseInt(sc.pcq_total_limit) || 2000,
          pcq_nas_id: sc.pcq_nas_id || null,
          pcq_target_pools: sc.pcq_target_pools || '',
          is_active: sc.is_active,
          time_based_speed_enabled: sc.time_based_speed_enabled || false,
          time_from_hour: fromHour24,
          time_from_minute: parseInt(sc.time_from_minute) || 0,
          time_to_hour: toHour24,
          time_to_minute: parseInt(sc.time_to_minute) || 0,
          time_speed_ratio: parseInt(sc.time_speed_ratio) || 0,
        }
      })

      if (editingService) {
        result = await serviceApi.update(editingService.id, data)
        // Save CDN configurations only if CDNs have been loaded (prevents accidental deletion)
        if (cdnsLoaded) {
          await cdnApi.updateServiceCDNs(editingService.id, { cdns: cdnsFor24h })
        }
      } else {
        result = await serviceApi.create(data)
        // Save CDN configurations for new service
        if (result.data?.data?.id) {
          await cdnApi.updateServiceCDNs(result.data.data.id, { cdns: cdnsFor24h })
        }
      }
      return result
    },
    onSuccess: () => {
      toast.success(editingService ? 'Service updated' : 'Service created')
      queryClient.invalidateQueries(['services'])
      closeModal()
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to save'),
  })

  const deleteMutation = useMutation({
    mutationFn: (id) => serviceApi.delete(id),
    onSuccess: () => {
      toast.success('Service deleted')
      queryClient.invalidateQueries(['services'])
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to delete'),
  })

  const openModal = (service = null) => {
    if (service) {
      setEditingService(service)
      setFormData({
        name: service.name || '',
        description: service.description || '',
        download_speed: service.download_speed || '',
        upload_speed: service.upload_speed || '',
        price: service.price || '',
        validity_days: service.validity_days || '30',
        daily_quota: service.daily_quota ? Math.round(service.daily_quota / (1024 * 1024 * 1024)) : '',
        monthly_quota: service.monthly_quota ? Math.round(service.monthly_quota / (1024 * 1024 * 1024)) : '',
        burst_download: service.burst_download || '',
        burst_upload: service.burst_upload || '',
        burst_threshold: service.burst_threshold || '',
        burst_time: service.burst_time || '',
        priority: service.priority || '8',
        // Daily FUP (resets every day)
        fup1_threshold: service.fup1_threshold ? Math.round(service.fup1_threshold / (1024 * 1024 * 1024)) : '',
        fup1_download_speed: service.fup1_download_speed || '',
        fup1_upload_speed: service.fup1_upload_speed || '',
        fup2_threshold: service.fup2_threshold ? Math.round(service.fup2_threshold / (1024 * 1024 * 1024)) : '',
        fup2_download_speed: service.fup2_download_speed || '',
        fup2_upload_speed: service.fup2_upload_speed || '',
        fup3_threshold: service.fup3_threshold ? Math.round(service.fup3_threshold / (1024 * 1024 * 1024)) : '',
        fup3_download_speed: service.fup3_download_speed || '',
        fup3_upload_speed: service.fup3_upload_speed || '',
        fup4_threshold: service.fup4_threshold ? Math.round(service.fup4_threshold / (1024 * 1024 * 1024)) : '',
        fup4_download_speed: service.fup4_download_speed || '',
        fup4_upload_speed: service.fup4_upload_speed || '',
        fup5_threshold: service.fup5_threshold ? Math.round(service.fup5_threshold / (1024 * 1024 * 1024)) : '',
        fup5_download_speed: service.fup5_download_speed || '',
        fup5_upload_speed: service.fup5_upload_speed || '',
        fup6_threshold: service.fup6_threshold ? Math.round(service.fup6_threshold / (1024 * 1024 * 1024)) : '',
        fup6_download_speed: service.fup6_download_speed || '',
        fup6_upload_speed: service.fup6_upload_speed || '',
        // Monthly FUP (resets on renew)
        monthly_fup1_threshold: service.monthly_fup1_threshold ? Math.round(service.monthly_fup1_threshold / (1024 * 1024 * 1024)) : '',
        monthly_fup1_download_speed: service.monthly_fup1_download_speed || '',
        monthly_fup1_upload_speed: service.monthly_fup1_upload_speed || '',
        monthly_fup2_threshold: service.monthly_fup2_threshold ? Math.round(service.monthly_fup2_threshold / (1024 * 1024 * 1024)) : '',
        monthly_fup2_download_speed: service.monthly_fup2_download_speed || '',
        monthly_fup2_upload_speed: service.monthly_fup2_upload_speed || '',
        monthly_fup3_threshold: service.monthly_fup3_threshold ? Math.round(service.monthly_fup3_threshold / (1024 * 1024 * 1024)) : '',
        monthly_fup3_download_speed: service.monthly_fup3_download_speed || '',
        monthly_fup3_upload_speed: service.monthly_fup3_upload_speed || '',
        monthly_fup4_threshold: service.monthly_fup4_threshold ? Math.round(service.monthly_fup4_threshold / (1024 * 1024 * 1024)) : '',
        monthly_fup4_download_speed: service.monthly_fup4_download_speed || '',
        monthly_fup4_upload_speed: service.monthly_fup4_upload_speed || '',
        monthly_fup5_threshold: service.monthly_fup5_threshold ? Math.round(service.monthly_fup5_threshold / (1024 * 1024 * 1024)) : '',
        monthly_fup5_download_speed: service.monthly_fup5_download_speed || '',
        monthly_fup5_upload_speed: service.monthly_fup5_upload_speed || '',
        monthly_fup6_threshold: service.monthly_fup6_threshold ? Math.round(service.monthly_fup6_threshold / (1024 * 1024 * 1024)) : '',
        monthly_fup6_download_speed: service.monthly_fup6_download_speed || '',
        monthly_fup6_upload_speed: service.monthly_fup6_upload_speed || '',
        // CDN FUP
        cdn_fup_enabled: service.cdn_fup_enabled ?? false,
        cdn_fup1_threshold: service.cdn_fup1_threshold ? Math.round(service.cdn_fup1_threshold / (1024 * 1024 * 1024)) : '',
        cdn_fup1_download_speed: service.cdn_fup1_download_speed || '',
        cdn_fup1_upload_speed: service.cdn_fup1_upload_speed || '',
        cdn_fup2_threshold: service.cdn_fup2_threshold ? Math.round(service.cdn_fup2_threshold / (1024 * 1024 * 1024)) : '',
        cdn_fup2_download_speed: service.cdn_fup2_download_speed || '',
        cdn_fup2_upload_speed: service.cdn_fup2_upload_speed || '',
        cdn_fup3_threshold: service.cdn_fup3_threshold ? Math.round(service.cdn_fup3_threshold / (1024 * 1024 * 1024)) : '',
        cdn_fup3_download_speed: service.cdn_fup3_download_speed || '',
        cdn_fup3_upload_speed: service.cdn_fup3_upload_speed || '',
        cdn_monthly_fup1_threshold: service.cdn_monthly_fup1_threshold ? Math.round(service.cdn_monthly_fup1_threshold / (1024 * 1024 * 1024)) : '',
        cdn_monthly_fup1_download_speed: service.cdn_monthly_fup1_download_speed || '',
        cdn_monthly_fup1_upload_speed: service.cdn_monthly_fup1_upload_speed || '',
        cdn_monthly_fup2_threshold: service.cdn_monthly_fup2_threshold ? Math.round(service.cdn_monthly_fup2_threshold / (1024 * 1024 * 1024)) : '',
        cdn_monthly_fup2_download_speed: service.cdn_monthly_fup2_download_speed || '',
        cdn_monthly_fup2_upload_speed: service.cdn_monthly_fup2_upload_speed || '',
        cdn_monthly_fup3_threshold: service.cdn_monthly_fup3_threshold ? Math.round(service.cdn_monthly_fup3_threshold / (1024 * 1024 * 1024)) : '',
        cdn_monthly_fup3_download_speed: service.cdn_monthly_fup3_download_speed || '',
        cdn_monthly_fup3_upload_speed: service.cdn_monthly_fup3_upload_speed || '',
        is_active: service.is_active ?? true,
        // Time-based speed control (convert 24h to 12h format)
        time_based_speed_enabled: service.time_based_speed_enabled ?? false,
        time_from_hour: (() => {
          const h = service.time_from_hour || 0
          if (h === 0) return '12'
          if (h > 12) return (h - 12).toString()
          return h.toString()
        })(),
        time_from_minute: service.time_from_minute?.toString() || '0',
        time_from_ampm: (service.time_from_hour || 0) >= 12 ? 'PM' : 'AM',
        time_to_hour: (() => {
          const h = service.time_to_hour || 0
          if (h === 0) return '12'
          if (h > 12) return (h - 12).toString()
          return h.toString()
        })(),
        time_to_minute: service.time_to_minute?.toString() || '0',
        time_to_ampm: (service.time_to_hour || 0) >= 12 ? 'PM' : 'AM',
        time_download_ratio: service.time_download_ratio?.toString() || '0',
        time_upload_ratio: service.time_upload_ratio?.toString() || '0',
        nas_id: service.nas_id || null,
        pool_name: service.pool_name || '',
        address_list_in: service.address_list_in || '',
        address_list_out: service.address_list_out || '',
        queue_type: service.queue_type || 'simple',
      })
    } else {
      setEditingService(null)
      setFormData({
        name: '',
        description: '',
        download_speed: '',
        upload_speed: '',
        price: '',
        validity_days: '30',
        daily_quota: '',
        monthly_quota: '',
        burst_download: '',
        burst_upload: '',
        burst_threshold: '',
        burst_time: '',
        priority: '8',
        // Daily FUP (resets every day)
        fup1_threshold: '',
        fup1_download_speed: '',
        fup1_upload_speed: '',
        fup2_threshold: '',
        fup2_download_speed: '',
        fup2_upload_speed: '',
        fup3_threshold: '',
        fup3_download_speed: '',
        fup3_upload_speed: '',
        // Monthly FUP (resets on renew)
        monthly_fup1_threshold: '',
        monthly_fup1_download_speed: '',
        monthly_fup1_upload_speed: '',
        monthly_fup2_threshold: '',
        monthly_fup2_download_speed: '',
        monthly_fup2_upload_speed: '',
        monthly_fup3_threshold: '',
        monthly_fup3_download_speed: '',
        monthly_fup3_upload_speed: '',
        is_active: true,
        // Time-based speed control (12-hour format)
        time_based_speed_enabled: false,
        time_from_hour: '12',
        time_from_minute: '0',
        time_from_ampm: 'AM',
        time_to_hour: '12',
        time_to_minute: '0',
        time_to_ampm: 'AM',
        time_download_ratio: '100',
        time_upload_ratio: '100',
        pool_name: '',
        address_list_in: '',
        address_list_out: '',
        queue_type: 'simple',
      })
    }
    // Reset IP pools state (restore NAS from service if editing)
    const restoredNasId = service?.nas_id || null
    setSelectedNasId(restoredNasId)
    setIpPools([])
    setIpPoolsLoading(false)
    if (restoredNasId) {
      fetchIPPoolsForService(restoredNasId)
    }

    // Load service CDNs if editing
    setCdnsLoaded(false)
    if (service) {
      cdnApi.listServiceCDNs(service.id).then((r) => {
        const cdns = r.data.data || []
        setServiceCDNs(cdns.map(sc => {
          // Convert 24h to 12h format
          const fromHour24 = sc.time_from_hour || 0
          const toHour24 = sc.time_to_hour || 0
          return {
            cdn_id: sc.cdn_id,
            speed_limit: sc.speed_limit || 0,
            bypass_quota: sc.bypass_quota || false,
            pcq_enabled: sc.pcq_enabled || false,
            pcq_limit: sc.pcq_limit || 50,
            pcq_total_limit: sc.pcq_total_limit || 2000,
            pcq_nas_id: sc.pcq_nas_id || null,
            pcq_target_pools: sc.pcq_target_pools || '',
            is_active: sc.is_active ?? true,
            time_based_speed_enabled: sc.time_based_speed_enabled || false,
            time_from_hour: fromHour24 === 0 ? 12 : (fromHour24 > 12 ? fromHour24 - 12 : fromHour24),
            time_from_minute: sc.time_from_minute || 0,
            time_from_ampm: fromHour24 >= 12 ? 'PM' : 'AM',
            time_to_hour: toHour24 === 0 ? 12 : (toHour24 > 12 ? toHour24 - 12 : toHour24),
            time_to_minute: sc.time_to_minute || 0,
            time_to_ampm: toHour24 >= 12 ? 'PM' : 'AM',
            time_speed_ratio: sc.time_speed_ratio || 0,
          }
        }))
        setCdnsLoaded(true)
      }).catch(() => {
        setServiceCDNs([])
        setCdnsLoaded(true)
      })
    } else {
      setServiceCDNs([])
      setCdnsLoaded(true)
    }
    setShowModal(true)
  }

  const closeModal = () => {
    setShowModal(false)
    setEditingService(null)
    setServiceCDNs([])
  }

  const [showDuplicateModal, setShowDuplicateModal] = useState(false)
  const [duplicatingService, setDuplicatingService] = useState(null)
  const [duplicateName, setDuplicateName] = useState('')

  const handleRowClick = (service) => {
    setDuplicatingService(service)
    setDuplicateName(service.name + ' (Copy)')
    setShowDuplicateModal(true)
  }

  const duplicateMutation = useMutation({
    mutationFn: async ({ originalService, newName }) => {
      const data = {
        name: newName,
        description: originalService.description || '',
        download_speed: originalService.download_speed || 0,
        upload_speed: originalService.upload_speed || 0,
        download_speed_str: originalService.download_speed ? `${originalService.download_speed}k` : '',
        upload_speed_str: originalService.upload_speed ? `${originalService.upload_speed}k` : '',
        price: originalService.price || 0,
        day_price: originalService.day_price || 0,
        validity_days: originalService.validity_days || 30,
        daily_quota: originalService.daily_quota || 0,
        monthly_quota: originalService.monthly_quota || 0,
        burst_download: originalService.burst_download || 0,
        burst_upload: originalService.burst_upload || 0,
        burst_threshold: originalService.burst_threshold || 0,
        burst_time: originalService.burst_time || 0,
        priority: originalService.priority || 8,
        fup1_threshold: originalService.fup1_threshold || 0,
        fup1_download_speed: originalService.fup1_download_speed || 0,
        fup1_upload_speed: originalService.fup1_upload_speed || 0,
        fup2_threshold: originalService.fup2_threshold || 0,
        fup2_download_speed: originalService.fup2_download_speed || 0,
        fup2_upload_speed: originalService.fup2_upload_speed || 0,
        fup3_threshold: originalService.fup3_threshold || 0,
        fup3_download_speed: originalService.fup3_download_speed || 0,
        fup3_upload_speed: originalService.fup3_upload_speed || 0,
        fup4_threshold: originalService.fup4_threshold || 0,
        fup4_download_speed: originalService.fup4_download_speed || 0,
        fup4_upload_speed: originalService.fup4_upload_speed || 0,
        fup5_threshold: originalService.fup5_threshold || 0,
        fup5_download_speed: originalService.fup5_download_speed || 0,
        fup5_upload_speed: originalService.fup5_upload_speed || 0,
        fup6_threshold: originalService.fup6_threshold || 0,
        fup6_download_speed: originalService.fup6_download_speed || 0,
        fup6_upload_speed: originalService.fup6_upload_speed || 0,
        monthly_fup1_threshold: originalService.monthly_fup1_threshold || 0,
        monthly_fup1_download_speed: originalService.monthly_fup1_download_speed || 0,
        monthly_fup1_upload_speed: originalService.monthly_fup1_upload_speed || 0,
        monthly_fup2_threshold: originalService.monthly_fup2_threshold || 0,
        monthly_fup2_download_speed: originalService.monthly_fup2_download_speed || 0,
        monthly_fup2_upload_speed: originalService.monthly_fup2_upload_speed || 0,
        monthly_fup3_threshold: originalService.monthly_fup3_threshold || 0,
        monthly_fup3_download_speed: originalService.monthly_fup3_download_speed || 0,
        monthly_fup3_upload_speed: originalService.monthly_fup3_upload_speed || 0,
        monthly_fup4_threshold: originalService.monthly_fup4_threshold || 0,
        monthly_fup4_download_speed: originalService.monthly_fup4_download_speed || 0,
        monthly_fup4_upload_speed: originalService.monthly_fup4_upload_speed || 0,
        monthly_fup5_threshold: originalService.monthly_fup5_threshold || 0,
        monthly_fup5_download_speed: originalService.monthly_fup5_download_speed || 0,
        monthly_fup5_upload_speed: originalService.monthly_fup5_upload_speed || 0,
        monthly_fup6_threshold: originalService.monthly_fup6_threshold || 0,
        monthly_fup6_download_speed: originalService.monthly_fup6_download_speed || 0,
        monthly_fup6_upload_speed: originalService.monthly_fup6_upload_speed || 0,
        time_based_speed_enabled: originalService.time_based_speed_enabled || false,
        time_from_hour: originalService.time_from_hour || 0,
        time_from_minute: originalService.time_from_minute || 0,
        time_to_hour: originalService.time_to_hour || 0,
        time_to_minute: originalService.time_to_minute || 0,
        time_download_ratio: originalService.time_download_ratio || 0,
        time_upload_ratio: originalService.time_upload_ratio || 0,
        nas_id: originalService.nas_id || null,
        pool_name: originalService.pool_name || '',
        address_list_in: originalService.address_list_in || '',
        address_list_out: originalService.address_list_out || '',
        queue_type: originalService.queue_type || 'simple',
        is_active: true,
      }
      const result = await serviceApi.create(data)
      // Copy CDN configurations
      if (result.data?.data?.id) {
        try {
          const cdnRes = await cdnApi.listServiceCDNs(originalService.id)
          const cdns = cdnRes.data.data || []
          if (cdns.length > 0) {
            await cdnApi.updateServiceCDNs(result.data.data.id, { cdns: cdns.map(sc => ({
              cdn_id: sc.cdn_id,
              speed_limit: sc.speed_limit,
              bypass_quota: sc.bypass_quota,
              pcq_enabled: sc.pcq_enabled || false,
              pcq_limit: sc.pcq_limit || 50,
              pcq_total_limit: sc.pcq_total_limit || 2000,
              pcq_nas_id: sc.pcq_nas_id || null,
              pcq_target_pools: sc.pcq_target_pools || '',
              is_active: sc.is_active ?? true,
              time_based_speed_enabled: sc.time_based_speed_enabled || false,
              time_from_hour: sc.time_from_hour || 0,
              time_from_minute: sc.time_from_minute || 0,
              time_to_hour: sc.time_to_hour || 0,
              time_to_minute: sc.time_to_minute || 0,
              time_speed_ratio: sc.time_speed_ratio || 0,
            }))})
          }
        } catch (err) {
          console.error('Failed to copy CDN configs:', err)
        }
      }
      return result
    },
    onSuccess: () => {
      toast.success('Service duplicated successfully')
      queryClient.invalidateQueries(['services'])
      setShowDuplicateModal(false)
      setDuplicatingService(null)
    },
    onError: (err) => toast.error(err.response?.data?.message || 'Failed to duplicate service'),
  })

  const handleDuplicate = (e) => {
    e.preventDefault()
    if (!duplicateName.trim()) {
      toast.error('Service name is required')
      return
    }
    duplicateMutation.mutate({ originalService: duplicatingService, newName: duplicateName.trim() })
  }

  const handleSubmit = (e) => {
    e.preventDefault()
    // Generate speed strings from kbps values (e.g., 1400 -> "1400k")
    const downloadSpeedKbps = parseInt(formData.download_speed) || 0
    const uploadSpeedKbps = parseInt(formData.upload_speed) || 0

    const data = {
      ...formData,
      download_speed: downloadSpeedKbps,
      upload_speed: uploadSpeedKbps,
      download_speed_str: downloadSpeedKbps > 0 ? `${downloadSpeedKbps}k` : '',
      upload_speed_str: uploadSpeedKbps > 0 ? `${uploadSpeedKbps}k` : '',
      price: parseFloat(formData.price) || 0,
      validity_days: parseInt(formData.validity_days) || 30,
      daily_quota: formData.daily_quota ? parseInt(formData.daily_quota) * 1024 * 1024 * 1024 : 0,
      monthly_quota: formData.monthly_quota ? parseInt(formData.monthly_quota) * 1024 * 1024 * 1024 : 0,
      burst_download: parseInt(formData.burst_download) || 0,
      burst_upload: parseInt(formData.burst_upload) || 0,
      burst_threshold: parseInt(formData.burst_threshold) || 0,
      burst_time: parseInt(formData.burst_time) || 0,
      priority: parseInt(formData.priority) || 8,
      // Multi-tier FUP with direct speeds (in Kbps)
      fup1_threshold: formData.fup1_threshold ? parseInt(formData.fup1_threshold) * 1024 * 1024 * 1024 : 0,
      fup1_download_speed: parseInt(formData.fup1_download_speed) || 0,
      fup1_upload_speed: parseInt(formData.fup1_upload_speed) || 0,
      fup2_threshold: formData.fup2_threshold ? parseInt(formData.fup2_threshold) * 1024 * 1024 * 1024 : 0,
      fup2_download_speed: parseInt(formData.fup2_download_speed) || 0,
      fup2_upload_speed: parseInt(formData.fup2_upload_speed) || 0,
      fup3_threshold: formData.fup3_threshold ? parseInt(formData.fup3_threshold) * 1024 * 1024 * 1024 : 0,
      fup3_download_speed: parseInt(formData.fup3_download_speed) || 0,
      fup3_upload_speed: parseInt(formData.fup3_upload_speed) || 0,
      fup4_threshold: formData.fup4_threshold ? parseInt(formData.fup4_threshold) * 1024 * 1024 * 1024 : 0,
      fup4_download_speed: parseInt(formData.fup4_download_speed) || 0,
      fup4_upload_speed: parseInt(formData.fup4_upload_speed) || 0,
      fup5_threshold: formData.fup5_threshold ? parseInt(formData.fup5_threshold) * 1024 * 1024 * 1024 : 0,
      fup5_download_speed: parseInt(formData.fup5_download_speed) || 0,
      fup5_upload_speed: parseInt(formData.fup5_upload_speed) || 0,
      fup6_threshold: formData.fup6_threshold ? parseInt(formData.fup6_threshold) * 1024 * 1024 * 1024 : 0,
      fup6_download_speed: parseInt(formData.fup6_download_speed) || 0,
      fup6_upload_speed: parseInt(formData.fup6_upload_speed) || 0,
      // Monthly FUP (resets on renewal)
      monthly_fup1_threshold: formData.monthly_fup1_threshold ? parseInt(formData.monthly_fup1_threshold) * 1024 * 1024 * 1024 : 0,
      monthly_fup1_download_speed: parseInt(formData.monthly_fup1_download_speed) || 0,
      monthly_fup1_upload_speed: parseInt(formData.monthly_fup1_upload_speed) || 0,
      monthly_fup2_threshold: formData.monthly_fup2_threshold ? parseInt(formData.monthly_fup2_threshold) * 1024 * 1024 * 1024 : 0,
      monthly_fup2_download_speed: parseInt(formData.monthly_fup2_download_speed) || 0,
      monthly_fup2_upload_speed: parseInt(formData.monthly_fup2_upload_speed) || 0,
      monthly_fup3_threshold: formData.monthly_fup3_threshold ? parseInt(formData.monthly_fup3_threshold) * 1024 * 1024 * 1024 : 0,
      monthly_fup3_download_speed: parseInt(formData.monthly_fup3_download_speed) || 0,
      monthly_fup3_upload_speed: parseInt(formData.monthly_fup3_upload_speed) || 0,
      monthly_fup4_threshold: formData.monthly_fup4_threshold ? parseInt(formData.monthly_fup4_threshold) * 1024 * 1024 * 1024 : 0,
      monthly_fup4_download_speed: parseInt(formData.monthly_fup4_download_speed) || 0,
      monthly_fup4_upload_speed: parseInt(formData.monthly_fup4_upload_speed) || 0,
      monthly_fup5_threshold: formData.monthly_fup5_threshold ? parseInt(formData.monthly_fup5_threshold) * 1024 * 1024 * 1024 : 0,
      monthly_fup5_download_speed: parseInt(formData.monthly_fup5_download_speed) || 0,
      monthly_fup5_upload_speed: parseInt(formData.monthly_fup5_upload_speed) || 0,
      monthly_fup6_threshold: formData.monthly_fup6_threshold ? parseInt(formData.monthly_fup6_threshold) * 1024 * 1024 * 1024 : 0,
      monthly_fup6_download_speed: parseInt(formData.monthly_fup6_download_speed) || 0,
      monthly_fup6_upload_speed: parseInt(formData.monthly_fup6_upload_speed) || 0,
      // CDN FUP
      cdn_fup_enabled: formData.cdn_fup_enabled,
      cdn_fup1_threshold: formData.cdn_fup1_threshold ? parseInt(formData.cdn_fup1_threshold) * 1024 * 1024 * 1024 : 0,
      cdn_fup1_download_speed: parseInt(formData.cdn_fup1_download_speed) || 0,
      cdn_fup1_upload_speed: parseInt(formData.cdn_fup1_upload_speed) || 0,
      cdn_fup2_threshold: formData.cdn_fup2_threshold ? parseInt(formData.cdn_fup2_threshold) * 1024 * 1024 * 1024 : 0,
      cdn_fup2_download_speed: parseInt(formData.cdn_fup2_download_speed) || 0,
      cdn_fup2_upload_speed: parseInt(formData.cdn_fup2_upload_speed) || 0,
      cdn_fup3_threshold: formData.cdn_fup3_threshold ? parseInt(formData.cdn_fup3_threshold) * 1024 * 1024 * 1024 : 0,
      cdn_fup3_download_speed: parseInt(formData.cdn_fup3_download_speed) || 0,
      cdn_fup3_upload_speed: parseInt(formData.cdn_fup3_upload_speed) || 0,
      cdn_monthly_fup1_threshold: formData.cdn_monthly_fup1_threshold ? parseInt(formData.cdn_monthly_fup1_threshold) * 1024 * 1024 * 1024 : 0,
      cdn_monthly_fup1_download_speed: parseInt(formData.cdn_monthly_fup1_download_speed) || 0,
      cdn_monthly_fup1_upload_speed: parseInt(formData.cdn_monthly_fup1_upload_speed) || 0,
      cdn_monthly_fup2_threshold: formData.cdn_monthly_fup2_threshold ? parseInt(formData.cdn_monthly_fup2_threshold) * 1024 * 1024 * 1024 : 0,
      cdn_monthly_fup2_download_speed: parseInt(formData.cdn_monthly_fup2_download_speed) || 0,
      cdn_monthly_fup2_upload_speed: parseInt(formData.cdn_monthly_fup2_upload_speed) || 0,
      cdn_monthly_fup3_threshold: formData.cdn_monthly_fup3_threshold ? parseInt(formData.cdn_monthly_fup3_threshold) * 1024 * 1024 * 1024 : 0,
      cdn_monthly_fup3_download_speed: parseInt(formData.cdn_monthly_fup3_download_speed) || 0,
      cdn_monthly_fup3_upload_speed: parseInt(formData.cdn_monthly_fup3_upload_speed) || 0,
      // Time-based speed control (convert 12h to 24h format)
      time_based_speed_enabled: formData.time_based_speed_enabled,
      time_from_hour: (() => {
        let h = parseInt(formData.time_from_hour) || 12
        if (formData.time_from_ampm === 'PM' && h !== 12) h += 12
        if (formData.time_from_ampm === 'AM' && h === 12) h = 0
        return h
      })(),
      time_from_minute: parseInt(formData.time_from_minute) || 0,
      time_to_hour: (() => {
        let h = parseInt(formData.time_to_hour) || 12
        if (formData.time_to_ampm === 'PM' && h !== 12) h += 12
        if (formData.time_to_ampm === 'AM' && h === 12) h = 0
        return h
      })(),
      time_to_minute: parseInt(formData.time_to_minute) || 0,
      time_download_ratio: parseInt(formData.time_download_ratio) || 0,
      time_upload_ratio: parseInt(formData.time_upload_ratio) || 0,
    }
    saveMutation.mutate(data)
  }

  const handleChange = (e) => {
    const { name, value, type, checked } = e.target
    setFormData((prev) => ({
      ...prev,
      [name]: type === 'checkbox' ? checked : value,
    }))
  }

  // CDN management functions
  const addCDNConfig = (cdnId) => {
    if (serviceCDNs.find(sc => sc.cdn_id === cdnId)) return
    // Auto-fill NAS and pool from RADIUS Settings if already selected
    const autoNasId = selectedNasId || formData.nas_id || null
    const autoPool = formData.pool_name || ''
    setServiceCDNs([...serviceCDNs, {
      cdn_id: cdnId,
      speed_limit: 0,
      bypass_quota: false,
      pcq_enabled: false,
      pcq_limit: 50,
      pcq_total_limit: 2000,
      pcq_nas_id: autoNasId,
      pcq_target_pools: autoPool,
      is_active: true,
      time_based_speed_enabled: false,
      time_from_hour: 12,
      time_from_minute: 0,
      time_from_ampm: 'AM',
      time_to_hour: 12,
      time_to_minute: 0,
      time_to_ampm: 'AM',
      time_speed_ratio: 100,
    }])
    // Auto-fetch pools for this CDN if NAS is already selected
    if (autoNasId) {
      fetchPoolsForCDN(cdnId, autoNasId)
    }
  }

  const removeCDNConfig = (cdnId) => {
    setServiceCDNs(serviceCDNs.filter(sc => sc.cdn_id !== cdnId))
  }

  const updateCDNConfig = (cdnId, field, value) => {
    setServiceCDNs(prev => prev.map(sc =>
      sc.cdn_id === cdnId ? { ...sc, [field]: value } : sc
    ))
  }

  const updateCDNConfigMultiple = (cdnId, updates) => {
    setServiceCDNs(prev => prev.map(sc =>
      sc.cdn_id === cdnId ? { ...sc, ...updates } : sc
    ))
  }

  const getCDNName = (cdnId) => {
    const cdn = cdnList?.find(c => c.id === cdnId)
    return cdn?.name || 'Unknown'
  }

  const columns = useMemo(
    () => [
      {
        accessorKey: 'name',
        header: 'Name',
        enableSorting: true,
        cell: ({ row }) => (
          <div>
            <div className="font-semibold text-[12px]">{row.original.name}</div>
            {row.original.description && (
              <div className="text-[11px] text-gray-500 dark:text-gray-400">{row.original.description}</div>
            )}
          </div>
        ),
      },
      {
        id: 'speed',
        header: 'Speed',
        enableSorting: true,
        accessorFn: (row) => row.download_speed || 0,
        sortingFn: 'basic',
        cell: ({ row }) => (
          <div className="text-[12px]">
            <div>DL {row.original.download_speed} kb</div>
            <div>UL {row.original.upload_speed} kb</div>
          </div>
        ),
      },
      {
        accessorKey: 'price',
        header: 'Price',
        enableSorting: true,
        sortingFn: 'basic',
        cell: ({ row }) => `$${row.original.price?.toFixed(2)}`,
      },
      {
        accessorKey: 'validity_days',
        header: 'Validity',
        enableSorting: false,
        cell: ({ row }) => `${row.original.validity_days}d`,
      },
      {
        accessorKey: 'quota',
        header: 'Quota',
        enableSorting: false,
        cell: ({ row }) => (
          <div className="text-[12px]">
            {row.original.daily_quota || row.original.monthly_quota ? (
              <>
                <div>D: {row.original.daily_quota ? (row.original.daily_quota / (1024 * 1024 * 1024)).toFixed(0) : '--'} GB</div>
                <div>M: {row.original.monthly_quota ? (row.original.monthly_quota / (1024 * 1024 * 1024)).toFixed(0) : '--'} GB</div>
              </>
            ) : (
              <span className="text-gray-400 dark:text-gray-500">Unlim</span>
            )}
          </div>
        ),
      },
      {
        accessorKey: 'pool_name',
        header: 'Pool',
        enableSorting: false,
        cell: ({ row }) => (
          <span className={clsx('text-[12px]', row.original.pool_name ? '' : 'text-gray-400 dark:text-gray-500')}>
            {row.original.pool_name || '-'}
          </span>
        ),
      },
      {
        accessorKey: 'fup',
        header: 'FUP',
        enableSorting: false,
        cell: ({ row }) => {
          const s = row.original
          const hasFUP = s.fup1_download_speed > 0 || s.fup2_download_speed > 0 || s.fup3_download_speed > 0 || s.fup4_download_speed > 0 || s.fup5_download_speed > 0 || s.fup6_download_speed > 0
          return (
            <div className="text-[11px]">
              {hasFUP ? (
                <>
                  {s.fup1_threshold > 0 && s.fup1_download_speed > 0 && (
                    <div><span className="fup-badge-1">1</span> {Math.round(s.fup1_threshold / 1024 / 1024 / 1024)}G&rarr;{s.fup1_download_speed}k</div>
                  )}
                  {s.fup2_threshold > 0 && s.fup2_download_speed > 0 && (
                    <div><span className="fup-badge-2">2</span> {Math.round(s.fup2_threshold / 1024 / 1024 / 1024)}G&rarr;{s.fup2_download_speed}k</div>
                  )}
                  {s.fup3_threshold > 0 && s.fup3_download_speed > 0 && (
                    <div><span className="fup-badge-3">3</span> {Math.round(s.fup3_threshold / 1024 / 1024 / 1024)}G&rarr;{s.fup3_download_speed}k</div>
                  )}
                  {s.fup4_threshold > 0 && s.fup4_download_speed > 0 && (
                    <div><span style={{display:'inline-block',width:16,height:16,borderRadius:'50%',backgroundColor:'#00897b',color:'#fff',textAlign:'center',fontSize:10,lineHeight:'16px',marginRight:4}}>4</span> {Math.round(s.fup4_threshold / 1024 / 1024 / 1024)}G&rarr;{s.fup4_download_speed}k</div>
                  )}
                  {s.fup5_threshold > 0 && s.fup5_download_speed > 0 && (
                    <div><span style={{display:'inline-block',width:16,height:16,borderRadius:'50%',backgroundColor:'#3949ab',color:'#fff',textAlign:'center',fontSize:10,lineHeight:'16px',marginRight:4}}>5</span> {Math.round(s.fup5_threshold / 1024 / 1024 / 1024)}G&rarr;{s.fup5_download_speed}k</div>
                  )}
                  {s.fup6_threshold > 0 && s.fup6_download_speed > 0 && (
                    <div><span style={{display:'inline-block',width:16,height:16,borderRadius:'50%',backgroundColor:'#455a64',color:'#fff',textAlign:'center',fontSize:10,lineHeight:'16px',marginRight:4}}>6</span> {Math.round(s.fup6_threshold / 1024 / 1024 / 1024)}G&rarr;{s.fup6_download_speed}k</div>
                  )}
                </>
              ) : (
                <span className="text-gray-400 dark:text-gray-500">-</span>
              )}
            </div>
          )
        },
      },
      {
        accessorKey: 'is_active',
        header: 'Status',
        enableSorting: false,
        cell: ({ row }) => (
          <span className={clsx(row.original.is_active ? 'badge-success' : 'badge-gray')}>
            {row.original.is_active ? 'Active' : 'Inactive'}
          </span>
        ),
      },
      {
        id: 'actions',
        header: 'Actions',
        enableSorting: false,
        cell: ({ row }) => (
          <div className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
            {hasPermission('services.edit') && (
              <button
                onClick={() => openModal(row.original)}
                className="btn btn-xs"
                title="Edit"
              >
                <PencilIcon className="w-3.5 h-3.5" />
              </button>
            )}
            {hasPermission('services.delete') && (
              <button
                onClick={() => {
                  if (confirm('Are you sure you want to delete this service?')) {
                    deleteMutation.mutate(row.original.id)
                  }
                }}
                className="btn btn-xs btn-danger"
                title="Delete"
              >
                <TrashIcon className="w-3.5 h-3.5" />
              </button>
            )}
          </div>
        ),
      },
    ],
    [deleteMutation, hasPermission]
  )

  const table = useReactTable({
    data: services || [],
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  })

  // Inline style helpers for compact modal form
  const inputStyle = { fontSize: 11, height: 24 }
  const sectionStyle = { borderTop: '1px solid #c0c0c0', paddingTop: 8, marginTop: 8 }

  return (
    <div style={{ fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif", fontSize: 11 }}>
      {/* Toolbar */}
      <div className="wb-toolbar" style={{ justifyContent: 'space-between' }}>
        <span className="text-[13px] font-semibold">Services</span>
        {hasPermission('services.create') && (
          <button onClick={() => openModal()} className="btn btn-sm btn-primary">
            <PlusIcon className="w-3.5 h-3.5 mr-1" />
            Add Service
          </button>
        )}
      </div>

      {/* Table */}
      <div className="table-container" style={{ borderTop: 0 }}>
        <table className="table">
          <thead>
            {table.getHeaderGroups().map((headerGroup) => (
              <tr key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <th key={header.id}>
                    {header.column.getCanSort() ? (
                      <button
                        type="button"
                        onClick={header.column.getToggleSortingHandler()}
                        className="flex items-center gap-1 cursor-pointer select-none"
                      >
                        {flexRender(header.column.columnDef.header, header.getContext())}
                        {{
                          asc: <ChevronUpIcon className="w-3 h-3" />,
                          desc: <ChevronDownIcon className="w-3 h-3" />,
                        }[header.column.getIsSorted()] ?? <ChevronUpDownIcon className="w-3 h-3 text-gray-400" />}
                      </button>
                    ) : (
                      flexRender(header.column.columnDef.header, header.getContext())
                    )}
                  </th>
                ))}
              </tr>
            ))}
          </thead>
          <tbody>
            {isLoading ? (
              <tr>
                <td colSpan={columns.length} className="text-center py-4 text-[12px] text-gray-500">
                  Loading services...
                </td>
              </tr>
            ) : table.getRowModel().rows.length === 0 ? (
              <tr>
                <td colSpan={columns.length} className="text-center py-4 text-[12px] text-gray-500">
                  No services found
                </td>
              </tr>
            ) : (
              table.getRowModel().rows.map((row) => (
                <tr
                  key={row.id}
                  className="cursor-pointer"
                  onClick={() => handleRowClick(row.original)}
                >
                  {row.getVisibleCells().map((cell) => (
                    <td key={cell.id}>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Statusbar */}
      <div className="wb-statusbar">
        <span>{services?.length || 0} service(s)</span>
        <span>{services?.filter(s => s.is_active).length || 0} active</span>
      </div>

      {/* Duplicate Modal */}
      {showDuplicateModal && duplicatingService && (
        <div className="modal-overlay">
          <div className="modal modal-sm">
            <div className="modal-header">
              <span>Duplicate Service</span>
              <button onClick={() => setShowDuplicateModal(false)} style={{ background: 'none', border: 'none', color: 'white', cursor: 'pointer', fontSize: 14 }}>X</button>
            </div>
            <div className="modal-body">
              <p className="text-[12px] text-gray-600 dark:text-gray-300 mb-2">
                Copy all settings from <strong>{duplicatingService.name}</strong>
              </p>
              <div className="text-[11px] text-gray-500 dark:text-gray-400 mb-3" style={{ lineHeight: '1.4' }}>
                <div>Speed: DL {duplicatingService.download_speed}kb / UL {duplicatingService.upload_speed}kb</div>
                <div>Price: ${duplicatingService.price?.toFixed(2)} | Validity: {duplicatingService.validity_days} days</div>
                {duplicatingService.pool_name && <div>Pool: {duplicatingService.pool_name}</div>}
              </div>
              <form onSubmit={handleDuplicate}>
                <label className="label">New Service Name</label>
                <input
                  type="text"
                  value={duplicateName}
                  onChange={(e) => setDuplicateName(e.target.value)}
                  className="input"
                  autoFocus
                  required
                />
                <div className="modal-footer" style={{ marginTop: 8, padding: 0, border: 'none' }}>
                  <button type="button" onClick={() => setShowDuplicateModal(false)} className="btn btn-sm">Cancel</button>
                  <button type="submit" disabled={duplicateMutation.isLoading} className="btn btn-sm btn-primary">
                    {duplicateMutation.isLoading ? 'Duplicating...' : 'Duplicate'}
                  </button>
                </div>
              </form>
            </div>
          </div>
        </div>
      )}

      {/* Edit/Create Modal */}
      {showModal && (
        <div className="modal-overlay">
          <div className="modal" style={{ maxWidth: 640, width: '100%', maxHeight: '90vh', overflow: 'auto' }}>
            <div className="modal-header">
              <span>{editingService ? 'Edit Service' : 'Add Service'}</span>
              <button onClick={closeModal} style={{ background: 'none', border: 'none', color: 'white', cursor: 'pointer', fontSize: 16 }}>X</button>
            </div>

            <form onSubmit={handleSubmit} className="modal-body" style={{ padding: 12 }}>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
                <div style={{ gridColumn: 'span 2' }}>
                  <label className="label">Service Name</label>
                  <input type="text" name="name" value={formData.name} onChange={handleChange} className="input" style={{ ...inputStyle, width: '100%' }} required />
                </div>
                <div style={{ gridColumn: 'span 2' }}>
                  <label className="label">Description</label>
                  <textarea name="description" value={formData.description} onChange={handleChange} className="input" rows={2} style={{ fontSize: 11, width: '100%' }} />
                </div>
              </div>

              {/* Speed Settings */}
              <div style={sectionStyle}>
                <div className="wb-group-title" style={{ fontSize: 11, marginBottom: 6 }}>Speed Settings</div>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
                  <div><label className="label">Download Speed (kb)</label><input type="number" name="download_speed" value={formData.download_speed} onChange={handleChange} className="input" style={{ ...inputStyle, width: '100%' }} placeholder="e.g., 4000" required /></div>
                  <div><label className="label">Upload Speed (kb)</label><input type="number" name="upload_speed" value={formData.upload_speed} onChange={handleChange} className="input" style={{ ...inputStyle, width: '100%' }} placeholder="e.g., 1400" required /></div>
                </div>
              </div>

              {/* Pricing & Validity */}
              <div style={sectionStyle}>
                <div className="wb-group-title" style={{ fontSize: 11, marginBottom: 6 }}>Pricing & Validity</div>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
                  <div><label className="label">Price ($)</label><input type="number" name="price" value={formData.price} onChange={handleChange} className="input" style={{ ...inputStyle, width: '100%' }} step="0.01" required /></div>
                  <div><label className="label">Validity (Days)</label><input type="number" name="validity_days" value={formData.validity_days} onChange={handleChange} className="input" style={{ ...inputStyle, width: '100%' }} required /></div>
                </div>
              </div>

              {/* Quota Settings */}
              <div style={sectionStyle}>
                <div className="wb-group-title" style={{ fontSize: 11, marginBottom: 6 }}>Quota Settings (0 = Unlimited)</div>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
                  <div><label className="label">Daily Quota (GB)</label><input type="number" name="daily_quota" value={formData.daily_quota} onChange={handleChange} className="input" style={{ ...inputStyle, width: '100%' }} /></div>
                  <div><label className="label">Monthly Quota (GB)</label><input type="number" name="monthly_quota" value={formData.monthly_quota} onChange={handleChange} className="input" style={{ ...inputStyle, width: '100%' }} /></div>
                </div>
              </div>

              {/* Burst Settings */}
              <div style={sectionStyle}>
                <div className="wb-group-title" style={{ fontSize: 11, marginBottom: 6 }}>Burst Settings (Mikrotik)</div>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
                  <div><label className="label">Burst Download (kb)</label><input type="number" name="burst_download" value={formData.burst_download} onChange={handleChange} className="input" style={{ ...inputStyle, width: '100%' }} /></div>
                  <div><label className="label">Burst Upload (kb)</label><input type="number" name="burst_upload" value={formData.burst_upload} onChange={handleChange} className="input" style={{ ...inputStyle, width: '100%' }} /></div>
                  <div><label className="label">Burst Threshold (%)</label><input type="number" name="burst_threshold" value={formData.burst_threshold} onChange={handleChange} className="input" style={{ ...inputStyle, width: '100%' }} /></div>
                  <div><label className="label">Burst Time (seconds)</label><input type="number" name="burst_time" value={formData.burst_time} onChange={handleChange} className="input" style={{ ...inputStyle, width: '100%' }} /></div>
                </div>
              </div>

              {/* RADIUS Settings */}
              <div style={sectionStyle}>
                <div className="wb-group-title" style={{ fontSize: 11, marginBottom: 6 }}>RADIUS Settings</div>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
                  <div>
                    <label className="label">Select NAS/Router</label>
                    <select value={selectedNasId || ''} onChange={(e) => { const nasId = e.target.value ? parseInt(e.target.value) : null; setSelectedNasId(nasId); setFormData(prev => ({ ...prev, nas_id: nasId })); if (nasId) { fetchIPPoolsForService(nasId) } else { setIpPools([]) } }} className="input" style={{ ...inputStyle, width: '100%' }}>
                      <option value="">-- Select NAS --</option>
                      {nasList?.filter(n => n.is_active).map(nas => (<option key={nas.id} value={nas.id}>{nas.name} ({nas.ip_address})</option>))}
                    </select>
                  </div>
                  <div>
                    <label className="label">IP Pool Name</label>
                    {ipPoolsLoading ? (
                      <div className="input" style={{ ...inputStyle, display: 'flex', alignItems: 'center', color: '#888' }}>Loading pools...</div>
                    ) : ipPools.length > 0 || formData.pool_name ? (
                      <select name="pool_name" value={formData.pool_name} onChange={handleChange} className="input" style={{ ...inputStyle, width: '100%' }}>
                        <option value="">-- Select Pool --</option>
                        {formData.pool_name && !ipPools.find(p => p.name === formData.pool_name) && (
                          <option key={formData.pool_name} value={formData.pool_name}>{formData.pool_name} (current)</option>
                        )}
                        {ipPools.map(pool => (<option key={pool.name} value={pool.name}>{pool.name} ({pool.ranges})</option>))}
                      </select>
                    ) : (
                      <input type="text" name="pool_name" value={formData.pool_name} onChange={handleChange} className="input" style={{ ...inputStyle, width: '100%' }}
                        placeholder={selectedNasId ? "No pools found" : "Select NAS first"} />
                    )}
                  </div>
                </div>
              </div>

              {/* Daily FUP - Accordion */}
              <div style={sectionStyle}>
                <div className="wb-group-title" style={{ fontSize: 11, marginBottom: 4 }}>Daily FUP (Resets Every Day)</div>
                <p className="text-[11px] text-gray-500 dark:text-gray-400 mb-2">Speed in Kbps (e.g., 700 = 700k). Click tier to expand.</p>
                {[
                  { n: 1, prefix: 'fup1', label: 'FUP 1', color: '#e65100', border: '#e0a060', bg: '#fff8f0', darkBg: '#3b2a1a' },
                  { n: 2, prefix: 'fup2', label: 'FUP 2', color: '#c62828', border: '#e06060', bg: '#fff0f0', darkBg: '#3b1a1a' },
                  { n: 3, prefix: 'fup3', label: 'FUP 3', color: '#6a1b9a', border: '#a060c0', bg: '#f8f0ff', darkBg: '#2a1a3b' },
                  { n: 4, prefix: 'fup4', label: 'FUP 4', color: '#00897b', border: '#60c0a0', bg: '#f0fff8', darkBg: '#1a3b2a' },
                  { n: 5, prefix: 'fup5', label: 'FUP 5', color: '#3949ab', border: '#6080e0', bg: '#f0f0ff', darkBg: '#1a1a3b' },
                  { n: 6, prefix: 'fup6', label: 'FUP 6', color: '#455a64', border: '#90a4ae', bg: '#f5f5f5', darkBg: '#2a2a2a' },
                ].map(tier => {
                  const t = formData[`${tier.prefix}_threshold`]
                  const d = formData[`${tier.prefix}_download_speed`]
                  const u = formData[`${tier.prefix}_upload_speed`]
                  const hasData = (t && parseInt(t) > 0) || (d && parseInt(d) > 0)
                  const isOpen = expandedFUP[`daily_${tier.n}`]
                  return (
                    <div key={tier.prefix} style={{ marginBottom: 4, borderLeft: `3px solid ${tier.color}`, borderRadius: 4, overflow: 'hidden' }}>
                      <div
                        onClick={() => setExpandedFUP(prev => ({ ...prev, [`daily_${tier.n}`]: !prev[`daily_${tier.n}`] }))}
                        style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '4px 8px', cursor: 'pointer', fontSize: 12, fontWeight: 500, color: tier.color }}
                        className="hover:bg-gray-50 dark:hover:bg-gray-700"
                      >
                        <span>{isOpen ? '▼' : '▶'} {tier.label} {hasData ? `— ${t || 0}GB → ${d || 0}k/${u || 0}k` : ''}</span>
                        {hasData && <span style={{ fontSize: 10, color: '#888' }}>configured</span>}
                      </div>
                      {isOpen && (
                        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 6, padding: 6, border: `1px solid ${tier.border}` }} className="dark:bg-gray-800">
                          <div><label className="label" style={{ color: tier.color }}>Threshold (GB)</label><input type="number" name={`${tier.prefix}_threshold`} value={formData[`${tier.prefix}_threshold`]} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" style={{ ...inputStyle, width: '100%' }} /></div>
                          <div><label className="label" style={{ color: tier.color }}>Download (Kbps)</label><input type="number" name={`${tier.prefix}_download_speed`} value={formData[`${tier.prefix}_download_speed`]} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" style={{ ...inputStyle, width: '100%' }} /></div>
                          <div><label className="label" style={{ color: tier.color }}>Upload (Kbps)</label><input type="number" name={`${tier.prefix}_upload_speed`} value={formData[`${tier.prefix}_upload_speed`]} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" style={{ ...inputStyle, width: '100%' }} /></div>
                        </div>
                      )}
                    </div>
                  )
                })}
              </div>

              {/* Monthly FUP - Accordion */}
              <div style={sectionStyle}>
                <div className="wb-group-title" style={{ fontSize: 11, marginBottom: 4 }}>Monthly FUP (Resets on Renewal)</div>
                {[
                  { n: 1, prefix: 'monthly_fup1', label: 'Monthly FUP 1', color: '#00695c', border: '#60c0a0' },
                  { n: 2, prefix: 'monthly_fup2', label: 'Monthly FUP 2', color: '#c62828', border: '#e06060' },
                  { n: 3, prefix: 'monthly_fup3', label: 'Monthly FUP 3', color: '#6a1b9a', border: '#a060c0' },
                  { n: 4, prefix: 'monthly_fup4', label: 'Monthly FUP 4', color: '#00897b', border: '#60c0a0' },
                  { n: 5, prefix: 'monthly_fup5', label: 'Monthly FUP 5', color: '#3949ab', border: '#6080e0' },
                  { n: 6, prefix: 'monthly_fup6', label: 'Monthly FUP 6', color: '#455a64', border: '#90a4ae' },
                ].map(tier => {
                  const t = formData[`${tier.prefix}_threshold`]
                  const d = formData[`${tier.prefix}_download_speed`]
                  const u = formData[`${tier.prefix}_upload_speed`]
                  const hasData = (t && parseInt(t) > 0) || (d && parseInt(d) > 0)
                  const isOpen = expandedFUP[`monthly_${tier.n}`]
                  return (
                    <div key={tier.prefix} style={{ marginBottom: 4, borderLeft: `3px solid ${tier.color}`, borderRadius: 4, overflow: 'hidden' }}>
                      <div
                        onClick={() => setExpandedFUP(prev => ({ ...prev, [`monthly_${tier.n}`]: !prev[`monthly_${tier.n}`] }))}
                        style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '4px 8px', cursor: 'pointer', fontSize: 12, fontWeight: 500, color: tier.color }}
                        className="hover:bg-gray-50 dark:hover:bg-gray-700"
                      >
                        <span>{isOpen ? '▼' : '▶'} {tier.label} {hasData ? `— ${t || 0}GB → ${d || 0}k/${u || 0}k` : ''}</span>
                        {hasData && <span style={{ fontSize: 10, color: '#888' }}>configured</span>}
                      </div>
                      {isOpen && (
                        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 6, padding: 6, border: `1px solid ${tier.border}` }} className="dark:bg-gray-800">
                          <div><label className="label" style={{ color: tier.color }}>Threshold (GB)</label><input type="number" name={`${tier.prefix}_threshold`} value={formData[`${tier.prefix}_threshold`]} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" style={{ ...inputStyle, width: '100%' }} /></div>
                          <div><label className="label" style={{ color: tier.color }}>Download (Kbps)</label><input type="number" name={`${tier.prefix}_download_speed`} value={formData[`${tier.prefix}_download_speed`]} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" style={{ ...inputStyle, width: '100%' }} /></div>
                          <div><label className="label" style={{ color: tier.color }}>Upload (Kbps)</label><input type="number" name={`${tier.prefix}_upload_speed`} value={formData[`${tier.prefix}_upload_speed`]} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" style={{ ...inputStyle, width: '100%' }} /></div>
                        </div>
                      )}
                    </div>
                  )
                })}
              </div>

              {/* CDN FUP (Fair Usage Policy) */}
              <div style={sectionStyle}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 4 }}>
                  <div className="wb-group-title" style={{ fontSize: 11 }}>CDN FUP (CDN Traffic Only)</div>
                  <label style={{ display: 'flex', alignItems: 'center', gap: 4, cursor: 'pointer' }}>
                    <input type="checkbox" name="cdn_fup_enabled" checked={formData.cdn_fup_enabled} onChange={handleChange} />
                    <span className="text-[11px]" style={{ color: formData.cdn_fup_enabled ? '#2e7d32' : '#888' }}>
                      {formData.cdn_fup_enabled ? 'Enabled' : 'Disabled'}
                    </span>
                  </label>
                </div>
                <p className="text-[11px] text-gray-500 dark:text-gray-400 mb-2">Limits CDN speed (YouTube, Netflix, etc.) independently of regular FUP. Internet stays full speed.</p>
                <div style={{ opacity: formData.cdn_fup_enabled ? 1 : 0.5, pointerEvents: formData.cdn_fup_enabled ? 'auto' : 'none' }}>
                  <p className="text-[10px] text-gray-400 mb-1">CDN Daily FUP (resets daily)</p>
                  {[
                    { n: 1, prefix: 'cdn_fup1', label: 'CDN FUP 1', color: '#f57c00', border: '#ffb74d' },
                    { n: 2, prefix: 'cdn_fup2', label: 'CDN FUP 2', color: '#e65100', border: '#ff8a65' },
                    { n: 3, prefix: 'cdn_fup3', label: 'CDN FUP 3', color: '#bf360c', border: '#ff7043' },
                  ].map(tier => {
                    const t = formData[`${tier.prefix}_threshold`]
                    const d = formData[`${tier.prefix}_download_speed`]
                    const u = formData[`${tier.prefix}_upload_speed`]
                    const hasData = (t && parseInt(t) > 0) || (d && parseInt(d) > 0)
                    const isOpen = expandedFUP[`cdn_daily_${tier.n}`]
                    return (
                      <div key={tier.prefix} style={{ marginBottom: 4, borderLeft: `3px solid ${tier.color}`, borderRadius: 4, overflow: 'hidden' }}>
                        <div
                          onClick={() => setExpandedFUP(prev => ({ ...prev, [`cdn_daily_${tier.n}`]: !prev[`cdn_daily_${tier.n}`] }))}
                          style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '4px 8px', cursor: 'pointer', fontSize: 12, fontWeight: 500, color: tier.color }}
                          className="hover:bg-gray-50 dark:hover:bg-gray-700"
                        >
                          <span>{isOpen ? '▼' : '▶'} {tier.label} {hasData ? `— ${t || 0}GB → ${d || 0}k/${u || 0}k` : ''}</span>
                          {hasData && <span style={{ fontSize: 10, color: '#888' }}>configured</span>}
                        </div>
                        {isOpen && (
                          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 6, padding: 6, border: `1px solid ${tier.border}` }} className="dark:bg-gray-800">
                            <div><label className="label" style={{ color: tier.color }}>Threshold (GB)</label><input type="number" name={`${tier.prefix}_threshold`} value={formData[`${tier.prefix}_threshold`]} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" style={{ ...inputStyle, width: '100%' }} /></div>
                            <div><label className="label" style={{ color: tier.color }}>Download (Kbps)</label><input type="number" name={`${tier.prefix}_download_speed`} value={formData[`${tier.prefix}_download_speed`]} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" style={{ ...inputStyle, width: '100%' }} /></div>
                            <div><label className="label" style={{ color: tier.color }}>Upload (Kbps)</label><input type="number" name={`${tier.prefix}_upload_speed`} value={formData[`${tier.prefix}_upload_speed`]} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" style={{ ...inputStyle, width: '100%' }} /></div>
                          </div>
                        )}
                      </div>
                    )
                  })}
                  <p className="text-[10px] text-gray-400 mb-1 mt-3">CDN Monthly FUP (resets on renewal)</p>
                  {[
                    { n: 1, prefix: 'cdn_monthly_fup1', label: 'CDN Monthly 1', color: '#00695c', border: '#4db6ac' },
                    { n: 2, prefix: 'cdn_monthly_fup2', label: 'CDN Monthly 2', color: '#004d40', border: '#26a69a' },
                    { n: 3, prefix: 'cdn_monthly_fup3', label: 'CDN Monthly 3', color: '#1b5e20', border: '#66bb6a' },
                  ].map(tier => {
                    const t = formData[`${tier.prefix}_threshold`]
                    const d = formData[`${tier.prefix}_download_speed`]
                    const u = formData[`${tier.prefix}_upload_speed`]
                    const hasData = (t && parseInt(t) > 0) || (d && parseInt(d) > 0)
                    const isOpen = expandedFUP[`cdn_monthly_${tier.n}`]
                    return (
                      <div key={tier.prefix} style={{ marginBottom: 4, borderLeft: `3px solid ${tier.color}`, borderRadius: 4, overflow: 'hidden' }}>
                        <div
                          onClick={() => setExpandedFUP(prev => ({ ...prev, [`cdn_monthly_${tier.n}`]: !prev[`cdn_monthly_${tier.n}`] }))}
                          style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '4px 8px', cursor: 'pointer', fontSize: 12, fontWeight: 500, color: tier.color }}
                          className="hover:bg-gray-50 dark:hover:bg-gray-700"
                        >
                          <span>{isOpen ? '▼' : '▶'} {tier.label} {hasData ? `— ${t || 0}GB → ${d || 0}k/${u || 0}k` : ''}</span>
                          {hasData && <span style={{ fontSize: 10, color: '#888' }}>configured</span>}
                        </div>
                        {isOpen && (
                          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 6, padding: 6, border: `1px solid ${tier.border}` }} className="dark:bg-gray-800">
                            <div><label className="label" style={{ color: tier.color }}>Threshold (GB)</label><input type="number" name={`${tier.prefix}_threshold`} value={formData[`${tier.prefix}_threshold`]} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" style={{ ...inputStyle, width: '100%' }} /></div>
                            <div><label className="label" style={{ color: tier.color }}>Download (Kbps)</label><input type="number" name={`${tier.prefix}_download_speed`} value={formData[`${tier.prefix}_download_speed`]} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" style={{ ...inputStyle, width: '100%' }} /></div>
                            <div><label className="label" style={{ color: tier.color }}>Upload (Kbps)</label><input type="number" name={`${tier.prefix}_upload_speed`} value={formData[`${tier.prefix}_upload_speed`]} onChange={handleChange} className="input dark:bg-gray-700 dark:text-white dark:border-gray-600" style={{ ...inputStyle, width: '100%' }} /></div>
                          </div>
                        )}
                      </div>
                    )
                  })}
                </div>
              </div>

              {/* Free Hours */}
              <div style={sectionStyle}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 4 }}>
                  <div className="wb-group-title" style={{ fontSize: 11 }}>Free Hours -- Quota Discount</div>
                  <label style={{ display: 'flex', alignItems: 'center', gap: 4, cursor: 'pointer' }}>
                    <input type="checkbox" name="time_based_speed_enabled" checked={formData.time_based_speed_enabled} onChange={handleChange} />
                    <span className="text-[11px]" style={{ color: formData.time_based_speed_enabled ? '#2e7d32' : '#888' }}>
                      {formData.time_based_speed_enabled ? 'Enabled' : 'Disabled'}
                    </span>
                  </label>
                </div>
                <p className="text-[11px] text-gray-500 mb-2">100% = fully free, 70% = 30% counted, 0% = no discount.</p>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8, opacity: formData.time_based_speed_enabled ? 1 : 0.5, pointerEvents: formData.time_based_speed_enabled ? 'auto' : 'none' }}>
                  <div style={{ padding: 6, border: '1px solid #a0a0c0', backgroundColor: '#f0f0ff' }}>
                    <label className="label" style={{ color: '#303f9f' }}>From Time</label>
                    <div style={{ display: 'flex', gap: 4, alignItems: 'center' }}>
                      <select name="time_from_hour" value={formData.time_from_hour} onChange={handleChange} className="input" style={{ ...inputStyle, width: 50 }}>
                        {[12,1,2,3,4,5,6,7,8,9,10,11].map(h => <option key={h} value={h}>{h}</option>)}
                      </select>
                      <span style={{ fontWeight: 'bold' }}>:</span>
                      <select name="time_from_minute" value={formData.time_from_minute} onChange={handleChange} className="input" style={{ ...inputStyle, width: 50 }}>
                        {[0,15,30,45].map(m => <option key={m} value={m}>{m.toString().padStart(2,'0')}</option>)}
                      </select>
                      <select name="time_from_ampm" value={formData.time_from_ampm} onChange={handleChange} className="input" style={{ ...inputStyle, width: 50, fontWeight: 600 }}>
                        <option value="AM">AM</option><option value="PM">PM</option>
                      </select>
                    </div>
                  </div>
                  <div style={{ padding: 6, border: '1px solid #a0a0c0', backgroundColor: '#f0f0ff' }}>
                    <label className="label" style={{ color: '#303f9f' }}>To Time</label>
                    <div style={{ display: 'flex', gap: 4, alignItems: 'center' }}>
                      <select name="time_to_hour" value={formData.time_to_hour} onChange={handleChange} className="input" style={{ ...inputStyle, width: 50 }}>
                        {[12,1,2,3,4,5,6,7,8,9,10,11].map(h => <option key={h} value={h}>{h}</option>)}
                      </select>
                      <span style={{ fontWeight: 'bold' }}>:</span>
                      <select name="time_to_minute" value={formData.time_to_minute} onChange={handleChange} className="input" style={{ ...inputStyle, width: 50 }}>
                        {[0,15,30,45].map(m => <option key={m} value={m}>{m.toString().padStart(2,'0')}</option>)}
                      </select>
                      <select name="time_to_ampm" value={formData.time_to_ampm} onChange={handleChange} className="input" style={{ ...inputStyle, width: 50, fontWeight: 600 }}>
                        <option value="AM">AM</option><option value="PM">PM</option>
                      </select>
                    </div>
                  </div>
                </div>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8, marginTop: 6, padding: 6, border: '1px solid #a0a0c0', backgroundColor: '#f0f0ff', opacity: formData.time_based_speed_enabled ? 1 : 0.5, pointerEvents: formData.time_based_speed_enabled ? 'auto' : 'none' }}>
                  <div><label className="label" style={{ color: '#303f9f' }}>Quota Free % (Download)</label><input type="number" name="time_download_ratio" value={formData.time_download_ratio} onChange={handleChange} className="input" style={{ ...inputStyle, width: '100%' }} min="0" max="100" /></div>
                  <div><label className="label" style={{ color: '#303f9f' }}>Quota Free % (Upload)</label><input type="number" name="time_upload_ratio" value={formData.time_upload_ratio} onChange={handleChange} className="input" style={{ ...inputStyle, width: '100%' }} min="0" max="100" /></div>
                </div>
              </div>

              {/* CDN Configuration */}
              <div style={sectionStyle}>
                <div className="wb-group-title" style={{ fontSize: 11, marginBottom: 4 }}>CDN Configuration</div>
                {cdnList && cdnList.length > 0 && (
                  <div style={{ marginBottom: 6 }}>
                    <select className="input" style={inputStyle} value="" onChange={(e) => { if (e.target.value) addCDNConfig(parseInt(e.target.value)) }}>
                      <option value="">+ Add CDN...</option>
                      {cdnList.filter(cdn => !serviceCDNs.find(sc => sc.cdn_id === cdn.id)).map(cdn => (<option key={cdn.id} value={cdn.id}>{cdn.name}</option>))}
                    </select>
                  </div>
                )}
                {serviceCDNs.length > 0 ? (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                    {serviceCDNs.map((sc) => (
                      <div key={sc.cdn_id} style={{ padding: 6, border: '1px solid #c0c0c0', backgroundColor: '#f8f8f8' }}>
                        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 4 }}>
                          <span className="font-medium text-[12px]">{getCDNName(sc.cdn_id)}</span>
                          <button type="button" onClick={() => removeCDNConfig(sc.cdn_id)} className="btn btn-xs btn-danger">Remove</button>
                        </div>
                        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 6 }}>
                          <div>
                            <label className="label" style={{ fontSize: 10 }}>Speed Limit (Mbps)</label>
                            <input type="number" value={sc.speed_limit === 0 ? '' : sc.speed_limit} onChange={(e) => updateCDNConfig(sc.cdn_id, 'speed_limit', e.target.value === '' ? 0 : parseInt(e.target.value) || 0)} className="input" style={{ ...inputStyle, width: '100%' }} min="0" placeholder="0=unlim" />
                          </div>
                          <div style={{ display: 'flex', alignItems: 'flex-end', paddingBottom: 2 }}>
                            <label style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                              <input type="checkbox" checked={sc.bypass_quota} onChange={(e) => updateCDNConfig(sc.cdn_id, 'bypass_quota', e.target.checked)} />
                              <span className="text-[11px]" style={{ color: '#2e7d32', fontWeight: 600 }}>Bypass Quota</span>
                            </label>
                          </div>
                          <div style={{ display: 'flex', alignItems: 'flex-end', paddingBottom: 2 }}>
                            <label style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                              <input type="checkbox" checked={sc.is_active} onChange={(e) => updateCDNConfig(sc.cdn_id, 'is_active', e.target.checked)} />
                              <span className="text-[11px]">Active</span>
                            </label>
                          </div>
                        </div>
                        {/* PCQ */}
                        <div style={{ marginTop: 4, paddingTop: 4, borderTop: '1px solid #d0d0d0' }}>
                          <label style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                            <input type="checkbox" checked={sc.pcq_enabled} onChange={(e) => {
                              const enabled = e.target.checked; const autoNasId = selectedNasId || formData.nas_id || null
                              if (enabled && autoNasId && !sc.pcq_nas_id) { updateCDNConfigMultiple(sc.cdn_id, { pcq_enabled: true, pcq_nas_id: autoNasId }); fetchPoolsForCDN(sc.cdn_id, autoNasId) }
                              else { updateCDNConfig(sc.cdn_id, 'pcq_enabled', enabled) }
                            }} />
                            <span className="text-[11px] font-medium" style={{ color: '#1565c0' }}>PCQ Mode</span>
                          </label>
                          {sc.pcq_enabled && (
                            <div style={{ marginTop: 4, padding: 4, backgroundColor: '#e8f0ff', border: '1px solid #a0c0e0' }}>
                              {(() => {
                                const serviceNasId = selectedNasId || formData.nas_id || null
                                const serviceNas = serviceNasId ? nasList?.find(n => n.id === serviceNasId) : null
                                if (serviceNas) return <div className="text-[11px]" style={{ color: '#1565c0' }}>NAS: <strong>{serviceNas.name}</strong> (from RADIUS Settings)</div>
                                return (
                                  <div><label className="label" style={{ fontSize: 10, color: '#1565c0' }}>Select NAS</label>
                                    <select value={sc.pcq_nas_id || ''} onChange={(e) => { const nasId = e.target.value ? parseInt(e.target.value) : null; updateCDNConfigMultiple(sc.cdn_id, { pcq_nas_id: nasId, pcq_target_pools: '' }); if (nasId) fetchPoolsForCDN(sc.cdn_id, nasId) }} className="input" style={{ ...inputStyle, width: '100%' }}>
                                      <option value="">-- Select NAS --</option>{nasList?.filter(n => n.is_active).map(nas => <option key={nas.id} value={nas.id}>{nas.name} ({nas.ip_address})</option>)}
                                    </select></div>
                                )
                              })()}
                              {sc.pcq_nas_id && (
                                <div style={{ marginTop: 4 }}>
                                  <label className="label" style={{ fontSize: 10, color: '#1565c0' }}>Select Pools (Target)</label>
                                  {pcqPools[sc.cdn_id]?.loading ? (
                                    <div className="text-[11px] text-gray-500">Loading pools...</div>
                                  ) : pcqPools[sc.cdn_id]?.pools?.length > 0 ? (
                                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 4 }}>
                                      {pcqPools[sc.cdn_id].pools.filter(pool => !formData.pool_name || pool.name === formData.pool_name).map(pool => {
                                        const selectedPools = sc.pcq_target_pools ? sc.pcq_target_pools.split(',') : []
                                        const isSelected = selectedPools.includes(pool.ranges)
                                        return (
                                          <label key={pool.name} style={{ display: 'flex', alignItems: 'center', gap: 4, padding: 4, backgroundColor: 'white', border: '1px solid #c0c0c0', cursor: 'pointer' }}>
                                            <input type="checkbox" checked={isSelected} onChange={(e) => {
                                              let newPools = [...selectedPools]; if (e.target.checked) newPools.push(pool.ranges); else newPools = newPools.filter(p => p !== pool.ranges)
                                              updateCDNConfig(sc.cdn_id, 'pcq_target_pools', newPools.filter(p => p).join(','))
                                            }} />
                                            <div><div className="text-[11px] font-medium">{pool.name}</div><div className="text-[10px] text-gray-500">{pool.ranges}</div></div>
                                          </label>
                                        )
                                      })}
                                    </div>
                                  ) : (
                                    <button type="button" onClick={() => fetchPoolsForCDN(sc.cdn_id, sc.pcq_nas_id)} className="text-[11px]" style={{ color: '#1565c0', cursor: 'pointer', background: 'none', border: 'none' }}>Click to fetch pools</button>
                                  )}
                                </div>
                              )}
                              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 6, marginTop: 4, paddingTop: 4, borderTop: '1px solid #a0c0e0' }}>
                                <div><label className="label" style={{ fontSize: 10, color: '#1565c0' }}>PCQ Limit (KiB)</label><input type="number" value={sc.pcq_limit} onChange={(e) => updateCDNConfig(sc.cdn_id, 'pcq_limit', parseInt(e.target.value) || 50)} className="input" style={{ ...inputStyle, width: '100%' }} min="1" /></div>
                                <div><label className="label" style={{ fontSize: 10, color: '#1565c0' }}>PCQ Total (KiB)</label><input type="number" value={sc.pcq_total_limit} onChange={(e) => updateCDNConfig(sc.cdn_id, 'pcq_total_limit', parseInt(e.target.value) || 2000)} className="input" style={{ ...inputStyle, width: '100%' }} min="1" /></div>
                              </div>
                              {sc.pcq_target_pools && <div className="text-[11px]" style={{ marginTop: 4, padding: 4, backgroundColor: '#d0e8ff', color: '#1565c0' }}><strong>Target:</strong> {sc.pcq_target_pools}</div>}
                            </div>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className="text-center text-[12px] text-gray-400" style={{ padding: 12 }}>No CDNs configured.</div>
                )}
              </div>

              {/* Active checkbox */}
              <div style={sectionStyle}>
                <label style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                  <input type="checkbox" name="is_active" checked={formData.is_active} onChange={handleChange} />
                  <span className="text-[12px]">Active Service</span>
                </label>
              </div>

              {/* Footer buttons */}
              <div className="modal-footer" style={{ marginTop: 12, display: 'flex', justifyContent: 'flex-end', gap: 4, paddingTop: 8, borderTop: '1px solid #c0c0c0' }}>
                <button type="button" onClick={closeModal} className="btn btn-sm">Cancel</button>
                <button type="submit" disabled={saveMutation.isLoading || (editingService && !cdnsLoaded)} className="btn btn-sm btn-primary">
                  {saveMutation.isLoading ? 'Saving...' : (!cdnsLoaded && editingService) ? 'Loading CDNs...' : editingService ? 'Update' : 'Create'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
