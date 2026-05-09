import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type { BusinessProfileResponse } from '@/types/api'
import { format } from 'date-fns'

function StatusBadge({ status }: { status: string }) {
  const variants: Record<string, 'default' | 'secondary' | 'destructive' | 'outline'> = {
    pending: 'secondary',
    approved: 'default',
    rejected: 'destructive',
    resubmit_requested: 'outline',
  }
  return <Badge variant={variants[status] ?? 'secondary'}>{status.replace('_', ' ')}</Badge>
}

export function StatusDashboard({ profile }: { profile: BusinessProfileResponse }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          Business Application <StatusBadge status={profile.status} />
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-sm">
          <div>
            <div className="text-muted-foreground">Legal Representative</div>
            <div className="font-medium">{profile.legalRepresentativeName}</div>
          </div>
          <div>
            <div className="text-muted-foreground">Personal ID</div>
            <div className="font-medium">{profile.personalIdNumber}</div>
          </div>
          <div>
            <div className="text-muted-foreground">Tax ID</div>
            <div className="font-medium">{profile.taxIdNumber}</div>
          </div>
          <div>
            <div className="text-muted-foreground">Bank</div>
            <div className="font-medium">{profile.bankName} - {profile.bankBranch}</div>
          </div>
          <div>
            <div className="text-muted-foreground">Account Holder</div>
            <div className="font-medium">{profile.bankAccountHolderName}</div>
          </div>
          <div>
            <div className="text-muted-foreground">Submitted At</div>
            <div className="font-medium">{format(new Date(profile.submittedAt), 'PPP p')}</div>
          </div>
        </div>

        {profile.adminNotes && (
          <div className="rounded-md border border-destructive/50 bg-destructive/5 p-3 text-sm">
            <div className="font-medium text-destructive">Admin Notes</div>
            <div className="mt-1 whitespace-pre-wrap">{profile.adminNotes}</div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
