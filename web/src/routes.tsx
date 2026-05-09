import { createBrowserRouter, Navigate, Outlet } from 'react-router-dom'
import { useAuthStore } from '@/hooks/useAuthStore'
import { AppShell } from '@/components/layout/AppShell'
import { LoginPage } from '@/features/auth/LoginPage'
import { RegisterPage } from '@/features/auth/RegisterPage'
import { BusinessProfilePage } from '@/features/business-profile/BusinessProfilePage'
import { ApplicationStatusPage } from '@/features/business-profile/ApplicationStatusPage'
import { CourtsListPage } from '@/features/courts/CourtsListPage'
import { CourtStatsPage } from '@/features/courts/CourtStatsPage'
import { SubCourtManagerPage } from '@/features/courts/SubCourtManagerPage'
import { AdminDashboard } from '@/features/admin/AdminDashboard'
import { ApplicationsListPage } from '@/features/admin/ApplicationsListPage'
import { ApplicationReviewPage } from '@/features/admin/ApplicationReviewPage'

function AuthGuard() {
  const { user, isLoading } = useAuthStore()
  if (isLoading) return <div className="p-8">Loading...</div>
  if (!user) return <Navigate to="/login" replace />
  return <Outlet />
}

function OwnerGuard() {
  const { user } = useAuthStore()
  if (!user?.roles?.includes('court_owner')) return <Navigate to="/admin" replace />
  return <Outlet />
}

function AdminGuard() {
  const { user } = useAuthStore()
  if (!user?.roles?.includes('admin')) return <Navigate to="/owner" replace />
  return <Outlet />
}

function RoleRedirect() {
  const { user } = useAuthStore()
  if (user?.roles?.includes('admin')) return <Navigate to="/admin" replace />
  if (user?.roles?.includes('court_owner')) return <Navigate to="/owner" replace />
  // Regular authenticated users land on business profile to apply
  return <Navigate to="/owner/business-profile" replace />
}

export const router = createBrowserRouter([
  { path: '/login', element: <LoginPage /> },
  { path: '/register', element: <RegisterPage /> },
  {
    path: '/',
    element: <AuthGuard />,
    children: [
      { index: true, element: <RoleRedirect /> },
      {
        path: 'owner',
        element: <AppShell />,
        children: [
          { index: true, element: <Navigate to="/owner/courts" replace /> },
          { path: 'business-profile', element: <BusinessProfilePage /> },
          { path: 'application-status', element: <ApplicationStatusPage /> },
          { element: <OwnerGuard />, children: [
            { path: 'courts', element: <CourtsListPage /> },
            { path: 'courts/:id/stats', element: <CourtStatsPage /> },
            { path: 'courts/:id/subcourts', element: <SubCourtManagerPage /> },
          ]},
        ],
      },
      {
        path: 'admin',
        element: <AppShell />,
        children: [
          { index: true, element: <Navigate to="/admin/dashboard" replace /> },
          { element: <AdminGuard />, children: [
            { path: 'dashboard', element: <AdminDashboard /> },
            { path: 'applications', element: <ApplicationsListPage /> },
            { path: 'applications/:id', element: <ApplicationReviewPage /> },
          ]},
        ],
      },
    ],
  },
])
