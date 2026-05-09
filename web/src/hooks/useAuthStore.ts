import { create } from 'zustand'
import type { User } from '@/types/api'

interface AuthState {
  user: User | null
  firebaseUser: import('firebase/auth').User | null
  isLoading: boolean
  setUser: (user: User | null) => void
  setFirebaseUser: (user: import('firebase/auth').User | null) => void
  setLoading: (loading: boolean) => void
  logout: () => void
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  firebaseUser: null,
  isLoading: true,
  setUser: (user) => set({ user }),
  setFirebaseUser: (firebaseUser) => set({ firebaseUser }),
  setLoading: (isLoading) => set({ isLoading }),
  logout: () => set({ user: null, firebaseUser: null }),
}))
