import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { useAdminStats, useAdminTimeseries } from './hooks'
import { Users, Building2, ClipboardList, Wallet, TrendingUp } from 'lucide-react'
import {
  AreaChart,
  Area,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts'

const RANGE_OPTIONS = [
  { value: '1h', label: '1 Hour' },
  { value: '1d', label: '1 Day' },
  { value: '1m', label: '1 Month' },
]

export function AdminDashboard() {
  const navigate = useNavigate()
  const [range, setRange] = useState('1d')
  const { data: stats, isLoading: statsLoading } = useAdminStats()
  const { data: timeseries, isLoading: tsLoading } = useAdminTimeseries(range)

  if (statsLoading) return <div className="p-4">Loading dashboard...</div>
  if (!stats) return <div className="p-4 text-muted-foreground">Failed to load stats.</div>

  const items = [
    { label: 'Active Users', value: stats.totalActiveUsers, icon: Users, href: null as string | null },
    { label: 'Court Owners', value: stats.totalCourtOwners, icon: Building2, href: null },
    { label: 'Courts', value: stats.totalCourts, icon: Building2, href: null },
    { label: 'Total Revenue', value: stats.totalRevenue.toLocaleString(), icon: Wallet, href: null },
    { label: 'Pending Applications', value: stats.pendingApplications, icon: ClipboardList, href: '/admin/applications?status=pending' },
    { label: 'Recent Signups', value: stats.recentSignups, icon: TrendingUp, href: null },
  ]

  const ChartSkeleton = () => (
    <div className="h-64 flex items-center justify-center text-muted-foreground text-sm">Loading chart...</div>
  )

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">Platform Dashboard</h2>
        <div className="flex gap-1">
          {RANGE_OPTIONS.map((r) => (
            <Button
              key={r.value}
              variant={range === r.value ? 'default' : 'outline'}
              size="sm"
              onClick={() => setRange(r.value)}
            >
              {r.label}
            </Button>
          ))}
        </div>
      </div>

      <div className="grid grid-cols-2 lg:grid-cols-3 gap-3">
        {items.map((item) => {
          const Icon = item.icon
          const content = (
            <CardContent className="p-4">
              <div className="flex items-center gap-2 text-muted-foreground text-xs">
                <Icon className="h-4 w-4" />
                {item.label}
              </div>
              <div className="text-2xl font-semibold mt-1">{item.value}</div>
            </CardContent>
          )
          return item.href ? (
            <Card
              key={item.label}
              className="cursor-pointer hover:bg-muted/50 transition-colors"
              onClick={() => navigate(item.href!)}
            >
              {content}
            </Card>
          ) : (
            <Card key={item.label}>{content}</Card>
          )
        })}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <Card className="lg:col-span-2">
          <CardHeader>
            <CardTitle className="text-sm">Revenue</CardTitle>
          </CardHeader>
          <CardContent>
            {tsLoading || !timeseries ? (
              <ChartSkeleton />
            ) : (
              <ResponsiveContainer width="100%" height={240}>
                <AreaChart data={timeseries.revenue}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="label" tick={{ fontSize: 12 }} />
                  <YAxis tick={{ fontSize: 12 }} />
                  <Tooltip />
                  <Area type="monotone" dataKey="value" stroke="#10b981" fill="#10b981" fillOpacity={0.2} />
                </AreaChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Active Users</CardTitle>
          </CardHeader>
          <CardContent>
            {tsLoading || !timeseries ? (
              <ChartSkeleton />
            ) : (
              <ResponsiveContainer width="100%" height={240}>
                <AreaChart data={timeseries.activeUsers}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="label" tick={{ fontSize: 12 }} />
                  <YAxis tick={{ fontSize: 12 }} />
                  <Tooltip />
                  <Area type="monotone" dataKey="value" stroke="#3b82f6" fill="#3b82f6" fillOpacity={0.2} />
                </AreaChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>

        <Card className="lg:col-span-3">
          <CardHeader>
            <CardTitle className="text-sm">Sign-ups</CardTitle>
          </CardHeader>
          <CardContent>
            {tsLoading || !timeseries ? (
              <ChartSkeleton />
            ) : (
              <ResponsiveContainer width="100%" height={240}>
                <BarChart data={timeseries.signups}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="label" tick={{ fontSize: 12 }} />
                  <YAxis tick={{ fontSize: 12 }} />
                  <Tooltip />
                  <Bar dataKey="value" fill="#6366f1" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
