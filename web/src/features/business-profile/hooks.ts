import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import api from '@/lib/api'
import type { BusinessProfileResponse, SubmitBusinessProfileRequest } from '@/types/api'

export function useBusinessProfile() {
  return useQuery<BusinessProfileResponse>({
    queryKey: ['business-profile'],
    queryFn: async () => {
      const res = await api.get('/api/owner/business-profile')
      return res.data
    },
    retry: false,
  })
}

export function useDeleteBusinessProfile() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async () => {
      await api.delete('/api/owner/business-profile')
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['business-profile'] })
    },
  })
}

export function useSubmitApplication() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (payload: {
      data: SubmitBusinessProfileRequest
      files: Record<string, File>
    }) => {
      const formData = new FormData()
      formData.append('data', JSON.stringify(payload.data))
      Object.entries(payload.files).forEach(([key, file]) => {
        formData.append(key, file)
      })
      return api.post('/api/owner/business-profile', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['business-profile'] })
    },
  })
}

export function useUpdateApplication() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (payload: {
      data: SubmitBusinessProfileRequest
      files: Record<string, File>
    }) => {
      const formData = new FormData()
      formData.append('data', JSON.stringify(payload.data))
      Object.entries(payload.files).forEach(([key, file]) => {
        formData.append(key, file)
      })
      return api.put('/api/owner/business-profile', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['business-profile'] })
    },
  })
}
