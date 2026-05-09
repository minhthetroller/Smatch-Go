import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { useApplications } from './hooks'
import { format } from 'date-fns'

const STATUSES = ['all', 'pending', 'approved', 'rejected', 'resubmit_requested']

export function ApplicationsListPage() {
  const navigate = useNavigate()
  const [status, setStatus] = useState('all')
  const [page, setPage] = useState(1)
  const limit = 20

  const { data, isLoading } = useApplications(status === 'all' ? '' : status, page, limit)

  if (isLoading) return <div className="p-4">Loading applications...</div>

  const applications = data?.data || []
  const meta = data?.meta.pagination

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold">Business Applications</h2>

      <div className="flex flex-wrap gap-1">
        {STATUSES.map((s) => (
          <Button
            key={s}
            variant={status === s ? 'default' : 'outline'}
            size="sm"
            onClick={() => {
              setStatus(s)
              setPage(1)
            }}
          >
            {s === 'resubmit_requested' ? 'Resubmit' : s.charAt(0).toUpperCase() + s.slice(1)}
          </Button>
        ))}
      </div>

      <div className="space-y-2">
        {applications.map((app) => (
          <Card
            key={app.id}
            className="cursor-pointer hover:bg-muted/50 transition-colors"
            onClick={() => navigate(`/admin/applications/${app.id}`)}
          >
            <CardContent className="p-4">
              <div className="flex items-center justify-between">
                <div>
                  <div className="font-medium">{app.legalRepresentativeName}</div>
                  <div className="text-xs text-muted-foreground">
                    Submitted {format(new Date(app.submittedAt), 'PPP')}
                  </div>
                </div>
                <Badge
                  variant={
                    app.status === 'approved'
                      ? 'default'
                      : app.status === 'rejected'
                      ? 'destructive'
                      : app.status === 'resubmit_requested'
                      ? 'outline'
                      : 'secondary'
                  }
                >
                  {app.status.replace('_', ' ')}
                </Badge>
              </div>
            </CardContent>
          </Card>
        ))}
        {applications.length === 0 && (
          <div className="text-center text-muted-foreground py-8">No applications found.</div>
        )}
      </div>

      {meta && meta.totalPages > 1 && (
        <div className="flex items-center justify-between">
          <Button variant="outline" size="sm" disabled={!meta.hasPrev} onClick={() => setPage((p) => p - 1)}>
            Previous
          </Button>
          <span className="text-xs text-muted-foreground">
            Page {meta.page} of {meta.totalPages}
          </span>
          <Button variant="outline" size="sm" disabled={!meta.hasNext} onClick={() => setPage((p) => p + 1)}>
            Next
          </Button>
        </div>
      )}
    </div>
  )
}
