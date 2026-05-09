import { useState } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/hooks/useAuthStore'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Sheet, SheetContent, SheetTrigger } from '@/components/ui/sheet'
import { auth } from '@/lib/firebase'
import {
  LogOut,
  Menu,
  LayoutDashboard,
  FileText,
  Building2,
  ClipboardList,
  ChevronDown,
  ChevronRight,
  Activity,
  UserCircle,
  CreditCard,
  Briefcase,
  Settings,
  CheckCircle,
} from 'lucide-react'

const BP_STEPS = [
  { id: 'representative', label: 'Representative Info', icon: UserCircle },
  { id: 'identity', label: 'Identity Documents', icon: FileText },
  { id: 'business', label: 'Business Documents', icon: Briefcase },
  { id: 'banking', label: 'Banking Information', icon: CreditCard },
  { id: 'operational', label: 'Operational Specs', icon: Settings },
  { id: 'review', label: 'Review & Submit', icon: CheckCircle },
]

function SidebarContent() {
  const { user } = useAuthStore()
  const location = useLocation()
  const navigate = useNavigate()
  const isAdmin = user?.roles?.includes('admin')
  const isOwner = user?.roles?.includes('court_owner')
  const [bpOpen, setBpOpen] = useState(true)

  const adminLinks = [
    { to: '/admin/dashboard', label: 'Dashboard', icon: LayoutDashboard },
    { to: '/admin/applications', label: 'Applications', icon: ClipboardList },
  ]

  const ownerLinks = [
    { to: '/owner/courts', label: 'Courts', icon: Building2 },
  ]

  return (
    <div className="flex flex-col h-full">
      <div className="px-4 py-3 border-b flex items-center gap-2">
        <img src="/logo.jpg" alt="Smatch Badminton" className="h-8 w-8 rounded object-cover" />
        <span className="font-semibold text-lg">Smatch Badminton</span>
      </div>
      <ScrollArea className="flex-1 py-2">
        <nav className="space-y-1 px-2">
          {isAdmin && adminLinks.map((link) => {
            const Icon = link.icon
            const active = location.pathname.startsWith(link.to)
            return (
              <button
                key={link.to}
                onClick={() => navigate(link.to)}
                className={`w-full flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition-colors ${
                  active
                    ? 'bg-primary text-primary-foreground'
                    : 'hover:bg-accent hover:text-accent-foreground'
                }`}
              >
                <Icon className="h-4 w-4" />
                {link.label}
              </button>
            )
          })}

          {isOwner && ownerLinks.map((link) => {
            const Icon = link.icon
            const active = location.pathname.startsWith(link.to)
            return (
              <button
                key={link.to}
                onClick={() => navigate(link.to)}
                className={`w-full flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition-colors ${
                  active
                    ? 'bg-primary text-primary-foreground'
                    : 'hover:bg-accent hover:text-accent-foreground'
                }`}
              >
                <Icon className="h-4 w-4" />
                {link.label}
              </button>
            )
          })}

          {/* Business Profile Accordion */}
          <div>
            <button
              onClick={() => setBpOpen((v) => !v)}
              className={`w-full flex items-center justify-between rounded-md px-3 py-2 text-sm font-medium transition-colors ${
                location.pathname.startsWith('/owner/business-profile')
                  ? 'bg-primary text-primary-foreground'
                  : 'hover:bg-accent hover:text-accent-foreground'
              }`}
            >
              <div className="flex items-center gap-2">
                <FileText className="h-4 w-4" />
                Business Profile
              </div>
              {bpOpen ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
            </button>
            {bpOpen && (
              <div className="ml-4 mt-1 space-y-0.5 border-l pl-2">
                {BP_STEPS.map((step) => {
                  const Icon = step.icon
                  const to = `/owner/business-profile?step=${step.id}`
                  const active = location.pathname === '/owner/business-profile' && location.search.includes(`step=${step.id}`)
                  return (
                    <button
                      key={step.id}
                      onClick={() => navigate(to)}
                      className={`w-full flex items-center gap-2 rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
                        active
                          ? 'bg-primary/10 text-primary'
                          : 'hover:bg-accent hover:text-accent-foreground text-muted-foreground'
                      }`}
                    >
                      <Icon className="h-3.5 w-3.5" />
                      {step.label}
                    </button>
                  )
                })}
              </div>
            )}
          </div>

          {/* Application Status */}
          <button
            onClick={() => navigate('/owner/application-status')}
            className={`w-full flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition-colors ${
              location.pathname === '/owner/application-status'
                ? 'bg-primary text-primary-foreground'
                : 'hover:bg-accent hover:text-accent-foreground'
            }`}
          >
            <Activity className="h-4 w-4" />
            Status
          </button>
        </nav>
      </ScrollArea>
      <div className="border-t p-3 space-y-2">
        <div className="text-xs text-muted-foreground truncate">{user?.email}</div>
        <Button
          variant="outline"
          size="sm"
          className="w-full justify-start gap-2"
          onClick={() => auth.signOut()}
        >
          <LogOut className="h-4 w-4" />
          Logout
        </Button>
      </div>
    </div>
  )
}

export function AppShell() {
  return (
    <div className="flex h-screen w-full bg-background">
      <aside className="hidden md:flex w-60 border-r flex-col">
        <SidebarContent />
      </aside>
      <div className="flex flex-col flex-1 overflow-hidden">
        <header className="flex items-center justify-between border-b px-4 py-3 md:px-6">
          <div className="flex items-center gap-2">
            <Sheet>
              <SheetTrigger asChild className="md:hidden">
                <Button variant="ghost" size="icon">
                  <Menu className="h-5 w-5" />
                </Button>
              </SheetTrigger>
              <SheetContent side="left" className="w-60 p-0">
                <SidebarContent />
              </SheetContent>
            </Sheet>
            <span className="text-sm text-muted-foreground hidden sm:inline">
              Smatch Badminton
            </span>
          </div>
        </header>
        <main className="flex-1 overflow-auto p-4 md:p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
