import axios from 'axios'
import { auth } from './firebase'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL,
  timeout: 30000,
  withCredentials: true,
})

api.interceptors.request.use(async (config) => {
  const user = auth.currentUser
  if (user) {
    const token = await user.getIdToken()
    config.headers.Authorization = `Bearer ${token}`
  }

  const adminSecret = import.meta.env.VITE_ADMIN_SECRET
  if (adminSecret) {
    config.headers['X-Admin-Secret'] = adminSecret
  }

  return config
})

api.interceptors.response.use(
  (response) => {
    const data = response.data
    if (data && typeof data === 'object' && 'success' in data) {
      if (!data.success) {
        const error = (data as { error: { message: string; code: string } }).error
        const err = new Error(error.message)
        ;(err as Error & { code: string; status: number }).code = error.code
        ;(err as Error & { code: string; status: number }).status = response.status
        throw err
      }
      return { ...response, data: data.data }
    }
    return response
  },
  (error) => {
    if (error.response?.status === 401 && window.location.pathname !== '/login') {
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

export default api
