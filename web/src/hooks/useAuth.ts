import { useEffect } from 'react'
import { onAuthStateChanged } from 'firebase/auth'
import { auth } from '@/lib/firebase'
import api from '@/lib/api'
import { useAuthStore } from './useAuthStore'
import type { User } from '@/types/api'

export function useAuth() {
  const { setFirebaseUser, setUser, setLoading } = useAuthStore()

  useEffect(() => {
    const unsubscribe = onAuthStateChanged(auth, async (firebaseUser) => {
      setLoading(true)
      setFirebaseUser(firebaseUser)
      if (firebaseUser) {
        try {
          const res = await api.get('/api/auth/me')
          setUser(res.data as User)
        } catch {
          setUser(null)
        }
      } else {
        setUser(null)
      }
      setLoading(false)
    })

    return () => unsubscribe()
  }, [setFirebaseUser, setUser, setLoading])

  return useAuthStore()
}
