import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import api from '@/lib/api'
import type {
  AdminStatsResponse,
  AdminTimeseriesResponse,
  BusinessProfileResponse,
  ReviewBusinessProfileRequest,
  PaginatedResponse,
} from '@/types/api'

export function useAdminStats() {
  return useQuery<AdminStatsResponse>({
    queryKey: ['admin-stats'],
    queryFn: async () => {
      const res = await api.get('/api/admin/stats')
      return res.data
    },
  })
}

export function useAdminTimeseries(range: string) {
  return useQuery<AdminTimeseriesResponse>({
    queryKey: ['admin-timeseries', range],
    queryFn: async () => {
      const res = await api.get('/api/admin/stats/timeseries', { params: { range } })
      return res.data
    },
  })
}

export function useApplications(status: string, page: number, limit: number) {
  return useQuery<PaginatedResponse<BusinessProfileResponse>>({
    queryKey: ['admin-applications', status, page, limit],
    queryFn: async () => {
      const res = await api.get('/api/admin/business-profiles', {
        params: { status, page, limit },
      })
      return res.data
    },
  })
}

export function useApplication(id: string) {
  return useQuery<BusinessProfileResponse>({
    queryKey: ['admin-application', id],
    queryFn: async () => {
      const res = await api.get(`/api/admin/business-profiles/${id}`)
      return res.data
    },
    enabled: !!id,
  })
}

export function useReviewApplication() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({
      id,
      action,
      data,
    }: {
      id: string
      action: 'approve' | 'reject' | 'request_resubmit'
      data: ReviewBusinessProfileRequest
    }) => {
      const res = await api.post(`/api/admin/business-profiles/${id}/${action}`, data)
      return res.data
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-applications'] })
      queryClient.invalidateQueries({ queryKey: ['admin-stats'] })
    },
  })
}
