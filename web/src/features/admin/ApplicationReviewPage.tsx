import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { useApplication, useReviewApplication } from './hooks'
import { format } from 'date-fns'
import { toast } from 'sonner'
import { CheckCircle, XCircle, RotateCcw, ExternalLink } from 'lucide-react'

export function ApplicationReviewPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { data: app, isLoading } = useApplication(id!)
  const review = useReviewApplication()

  const [action, setAction] = useState<'approve' | 'reject' | 'request_resubmit' | null>(null)
  const [notes, setNotes] = useState('')

  if (isLoading) return <div className="p-4">Loading application...</div>
  if (!app) return <div className="p-4 text-muted-foreground">Application not found.</div>

  const handleReview = async () => {
    if (!action) return
    try {
      await review.mutateAsync({
        id: id!,
        action,
        data: { action, adminNotes: notes },
      })
      toast.success(`Application ${action.replace('_', ' ')}`)
      navigate('/admin/applications')
    } catch (err: any) {
      toast.error(err?.message || 'Review failed')
    }
  }

  const docFields = [
    { label: 'Personal ID Front', url: app.personalIdFrontImageUrl },
    { label: 'Personal ID Back', url: app.personalIdBackImageUrl },
    { label: 'Business Registration', url: app.businessRegistrationCertUrl },
    { label: 'Sports Eligibility', url: app.sportsBusinessEligibilityCertUrl },
    { label: 'Fire Safety', url: app.fireSafetyCertUrl },
    { label: 'Proof of Address', url: app.proofOfAddressUrl },
  ]

  return (
    <div className="space-y-4 max-w-3xl">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">Application Review</h2>
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

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Representative</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-2 gap-2 text-sm">
          <div className="text-muted-foreground">Name</div>
          <div>{app.legalRepresentativeName}</div>
          <div className="text-muted-foreground">Personal ID</div>
          <div>{app.personalIdNumber}</div>
          <div className="text-muted-foreground">Tax ID</div>
          <div>{app.taxIdNumber}</div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Banking Information</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-2 gap-2 text-sm">
          <div className="text-muted-foreground">Bank</div>
          <div>{app.bankName} — {app.bankBranch}</div>
          <div className="text-muted-foreground">Account Number</div>
          <div>{app.bankAccountNumber}</div>
          <div className="text-muted-foreground">Account Holder</div>
          <div>{app.bankAccountHolderName}</div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Operational Specs</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-2 gap-2 text-sm">
          <div className="text-muted-foreground">Subcourts</div>
          <div>{app.operationalSpecs.subcourt_count}</div>
          <div className="text-muted-foreground">Surface</div>
          <div>{app.operationalSpecs.surface_type}</div>
          <div className="text-muted-foreground">Hours</div>
          <div>{app.operationalSpecs.operating_hours.open} - {app.operationalSpecs.operating_hours.close}</div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Documents</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          {docFields.map((doc) =>
            doc.url ? (
              <a
                key={doc.label}
                href={doc.url}
                target="_blank"
                rel="noreferrer"
                className="flex items-center gap-2 text-sm text-primary hover:underline"
              >
                <ExternalLink className="h-3 w-3" />
                {doc.label}
              </a>
            ) : (
              <div key={doc.label} className="text-sm text-muted-foreground">
                {doc.label}: Not uploaded
              </div>
            )
          )}
        </CardContent>
      </Card>

      {app.adminNotes && (
        <div className="rounded-md border border-destructive/50 bg-destructive/5 p-3 text-sm">
          <div className="font-medium text-destructive">Previous Admin Notes</div>
          <div className="mt-1 whitespace-pre-wrap">{app.adminNotes}</div>
        </div>
      )}

      {app.status === 'pending' && (
        <div className="flex gap-2 pt-2">
          <Dialog>
            <DialogTrigger asChild>
              <Button variant="default" className="gap-1" onClick={() => setAction('approve')}>
                <CheckCircle className="h-4 w-4" />
                Approve
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Approve Application</DialogTitle>
              </DialogHeader>
              <div className="space-y-3 pt-2">
                <Label>Admin Notes (optional)</Label>
                <Textarea value={notes} onChange={(e) => setNotes(e.target.value)} />
                <Button className="w-full" onClick={handleReview} disabled={review.isPending}>
                  Confirm Approval
                </Button>
              </div>
            </DialogContent>
          </Dialog>

          <Dialog>
            <DialogTrigger asChild>
              <Button variant="destructive" className="gap-1" onClick={() => setAction('reject')}>
                <XCircle className="h-4 w-4" />
                Reject
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Reject Application</DialogTitle>
              </DialogHeader>
              <div className="space-y-3 pt-2">
                <Label>Admin Notes (optional)</Label>
                <Textarea value={notes} onChange={(e) => setNotes(e.target.value)} />
                <Button className="w-full" variant="destructive" onClick={handleReview} disabled={review.isPending}>
                  Confirm Rejection
                </Button>
              </div>
            </DialogContent>
          </Dialog>

          <Dialog>
            <DialogTrigger asChild>
              <Button variant="outline" className="gap-1" onClick={() => setAction('request_resubmit')}>
                <RotateCcw className="h-4 w-4" />
                Request Resubmit
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Request Resubmission</DialogTitle>
              </DialogHeader>
              <div className="space-y-3 pt-2">
                <Label>Admin Notes (required)</Label>
                <Textarea value={notes} onChange={(e) => setNotes(e.target.value)} />
                <Button className="w-full" variant="outline" onClick={handleReview} disabled={review.isPending}>
                  Confirm Request
                </Button>
              </div>
            </DialogContent>
          </Dialog>
        </div>
      )}
    </div>
  )
}
