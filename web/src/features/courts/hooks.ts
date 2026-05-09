import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import api from '@/lib/api'
import type {
  CourtOwnerCourtResponse,
  CourtOwnerCourtDetailResponse,
  CourtStatsDetailResponse,
  CloseCourtRequest,
  CloseSubCourtRequest,
} from '@/types/api'

export function useOwnerCourts() {
  return useQuery<CourtOwnerCourtResponse[]>({
    queryKey: ['owner-courts'],
    queryFn: async () => {
      const res = await api.get('/api/owner/courts')
      return res.data
    },
  })
}

export function useOwnerCourt(courtId: string) {
  return useQuery<CourtOwnerCourtDetailResponse>({
    queryKey: ['owner-court', courtId],
    queryFn: async () => {
      const res = await api.get(`/api/owner/courts/${courtId}`)
      return res.data
    },
    enabled: !!courtId,
  })
}

export function useCourtStats(courtId: string, period: string, granularity?: string) {
  return useQuery<CourtStatsDetailResponse>({
    queryKey: ['court-stats', courtId, period, granularity],
    queryFn: async () => {
      const res = await api.get(`/api/owner/courts/${courtId}/stats`, {
        params: { period, granularity },
      })
      return res.data
    },
    enabled: !!courtId,
  })
}

export function useCloseCourt() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ courtId, data }: { courtId: string; data: CloseCourtRequest }) => {
      const res = await api.post(`/api/owner/courts/${courtId}/close`, data)
      return res.data
    },
    onSuccess: (_, vars) => {
      queryClient.invalidateQueries({ queryKey: ['owner-court', vars.courtId] })
      queryClient.invalidateQueries({ queryKey: ['owner-courts'] })
    },
  })
}

export function useOpenCourt() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ courtId, date }: { courtId: string; date: string }) => {
      const res = await api.post(`/api/owner/courts/${courtId}/open?date=${date}`)
      return res.data
    },
    onSuccess: (_, vars) => {
      queryClient.invalidateQueries({ queryKey: ['owner-court', vars.courtId] })
      queryClient.invalidateQueries({ queryKey: ['owner-courts'] })
    },
  })
}

export function useCloseSubCourt() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({
      courtId,
      subCourtId,
      data,
    }: {
      courtId: string
      subCourtId: string
      data: CloseSubCourtRequest
    }) => {
      const res = await api.post(`/api/owner/courts/${courtId}/subcourts/${subCourtId}/close`, data)
      return res.data
    },
    onSuccess: (_, vars) => {
      queryClient.invalidateQueries({ queryKey: ['owner-court', vars.courtId] })
    },
  })
}

export function useOpenSubCourt() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({
      courtId,
      subCourtId,
      date,
    }: {
      courtId: string
      subCourtId: string
      date: string
    }) => {
      const res = await api.post(`/api/owner/courts/${courtId}/subcourts/${subCourtId}/open?date=${date}`)
      return res.data
    },
    onSuccess: (_, vars) => {
      queryClient.invalidateQueries({ queryKey: ['owner-court', vars.courtId] })
    },
  })
}
