import { useNavigate } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { useBusinessProfile, useDeleteBusinessProfile } from './hooks'
import { format } from 'date-fns'
import { toast } from 'sonner'
import {
  UserCircle,
  FileText,
  CreditCard,
  Settings,
  AlertTriangle,
  RotateCcw,
  Trash2,
  ExternalLink,
  CheckCircle,
  XCircle,
  Clock,
} from 'lucide-react'

function StatusBadge({ status }: { status: string }) {
  const config: Record<string, { variant: 'default' | 'secondary' | 'destructive' | 'outline'; icon: any; label: string }> = {
    pending: { variant: 'secondary', icon: Clock, label: 'Pending Review' },
    approved: { variant: 'default', icon: CheckCircle, label: 'Approved' },
    rejected: { variant: 'destructive', icon: XCircle, label: 'Rejected' },
    resubmit_requested: { variant: 'outline', icon: RotateCcw, label: 'Resubmission Requested' },
  }
  const c = config[status] || config.pending
  const Icon = c.icon
  return (
    <Badge variant={c.variant} className="gap-1 text-sm px-3 py-1">
      <Icon className="h-3.5 w-3.5" />
      {c.label}
    </Badge>
  )
}

function DetailRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex flex-col sm:flex-row sm:items-center gap-1 sm:gap-4 py-2 border-b last:border-0">
      <div className="text-sm text-muted-foreground sm:w-48 shrink-0">{label}</div>
      <div className="text-sm font-medium">{value || <span className="text-muted-foreground italic">Not provided</span>}</div>
    </div>
  )
}

function DocImage({ label, url }: { label: string; url?: string }) {
  if (!url) {
    return (
      <div className="rounded-lg border border-dashed p-4 text-center text-sm text-muted-foreground">
        {label}: Not uploaded
      </div>
    )
  }
  return (
    <div className="space-y-2">
      <div className="text-xs font-medium text-muted-foreground">{label}</div>
      <a href={url} target="_blank" rel="noreferrer" className="block group">
        <div className="rounded-lg border overflow-hidden bg-muted relative">
          <img src={url} alt={label} className="w-full h-40 object-contain group-hover:opacity-90 transition-opacity" />
          <div className="absolute top-2 right-2 bg-background/80 rounded p-1 opacity-0 group-hover:opacity-100 transition-opacity">
            <ExternalLink className="h-3.5 w-3.5" />
          </div>
        </div>
      </a>
    </div>
  )
}

export function ApplicationStatusPage() {
  const navigate = useNavigate()
  const { data: profile, isLoading } = useBusinessProfile()
  const deleteProfile = useDeleteBusinessProfile()

  if (isLoading) return <div className="p-4">Loading status...</div>

  if (!profile) {
    return (
      <div className="space-y-4">
        <h2 className="text-lg font-semibold">Application Status</h2>
        <Card>
          <CardContent className="p-8 text-center space-y-4">
            <FileText className="h-12 w-12 text-muted-foreground mx-auto" />
            <div className="text-muted-foreground">You have not submitted a business profile yet.</div>
            <Button onClick={() => navigate('/owner/business-profile')}>
              Start Application
            </Button>
          </CardContent>
        </Card>
      </div>
    )
  }

  const isRejected = profile.status === 'rejected'
  const hasNotes = profile.adminNotes && profile.adminNotes.trim().length > 0
  const canEditResubmit = isRejected && hasNotes
  const canDeleteAndResubmit = isRejected && !hasNotes

  const handleDeleteAndResubmit = async () => {
    try {
      await deleteProfile.mutateAsync()
      toast.success('Previous application removed. You can now submit a new one.')
      navigate('/owner/business-profile')
    } catch (err: any) {
      toast.error(err?.message || 'Failed to remove application')
    }
  }

  return (
    <div className="space-y-6 max-w-4xl">
      <div className="flex items-center justify-between flex-wrap gap-3">
        <h2 className="text-lg font-semibold">Application Status</h2>
        <StatusBadge status={profile.status} />
      </div>

      {/* Status Banner */}
      {profile.status === 'pending' && (
        <div className="rounded-lg border bg-blue-50/50 p-4 text-sm flex items-start gap-3">
          <Clock className="h-5 w-5 text-blue-500 shrink-0 mt-0.5" />
          <div>
            <div className="font-medium">Under Review</div>
            <div className="text-muted-foreground">Your application is being reviewed by our team. We will notify you once a decision is made.</div>
          </div>
        </div>
      )}
      {profile.status === 'approved' && (
        <div className="rounded-lg border bg-green-50/50 p-4 text-sm flex items-start gap-3">
          <CheckCircle className="h-5 w-5 text-green-500 shrink-0 mt-0.5" />
          <div>
            <div className="font-medium">Approved</div>
            <div className="text-muted-foreground">Congratulations! Your application has been approved. You can now manage your courts.</div>
          </div>
        </div>
      )}
      {isRejected && hasNotes && (
        <div className="rounded-lg border bg-amber-50/50 p-4 text-sm flex items-start gap-3">
          <AlertTriangle className="h-5 w-5 text-amber-500 shrink-0 mt-0.5" />
          <div>
            <div className="font-medium">Resubmission Required</div>
            <div className="text-muted-foreground">Your application was rejected with feedback. Please review the admin notes below and update the required sections.</div>
          </div>
        </div>
      )}
      {isRejected && !hasNotes && (
        <div className="rounded-lg border bg-red-50/50 p-4 text-sm flex items-start gap-3">
          <XCircle className="h-5 w-5 text-red-500 shrink-0 mt-0.5" />
          <div>
            <div className="font-medium">Application Rejected</div>
            <div className="text-muted-foreground">This application was rejected as invalid and cannot be edited. You must submit a new application.</div>
          </div>
        </div>
      )}

      {/* Admin Notes */}
      {profile.adminNotes && (
        <Card className="border-destructive/30">
          <CardHeader>
            <CardTitle className="text-sm text-destructive flex items-center gap-2">
              <AlertTriangle className="h-4 w-4" />
              Admin Notes
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-sm whitespace-pre-wrap">{profile.adminNotes}</div>
          </CardContent>
        </Card>
      )}

      {/* Representative Info */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm flex items-center gap-2">
            <UserCircle className="h-4 w-4" />
            Representative Information
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-0">
          <DetailRow label="Legal Representative Name" value={profile.legalRepresentativeName} />
          <DetailRow label="Personal ID Number" value={profile.personalIdNumber} />
          <DetailRow label="Tax ID Number" value={profile.taxIdNumber} />
        </CardContent>
      </Card>

      {/* Identity Documents */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm flex items-center gap-2">
            <FileText className="h-4 w-4" />
            Identity Documents
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <DocImage label="Personal ID Front" url={profile.personalIdFrontImageUrl} />
            <DocImage label="Personal ID Back" url={profile.personalIdBackImageUrl} />
          </div>
        </CardContent>
      </Card>

      {/* Business Documents */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm flex items-center gap-2">
            <FileText className="h-4 w-4" />
            Business Documents
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <DocImage label="Business Registration Certificate" url={profile.businessRegistrationCertUrl} />
            <DocImage label="Sports Business Eligibility Certificate" url={profile.sportsBusinessEligibilityCertUrl} />
            <DocImage label="Fire Safety Certificate" url={profile.fireSafetyCertUrl} />
            <DocImage label="Proof of Address" url={profile.proofOfAddressUrl} />
          </div>
        </CardContent>
      </Card>

      {/* Banking Information */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm flex items-center gap-2">
            <CreditCard className="h-4 w-4" />
            Banking Information
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-0">
          <DetailRow label="Bank Name" value={profile.bankName} />
          <DetailRow label="Bank Branch" value={profile.bankBranch} />
          <DetailRow label="Account Number" value={profile.bankAccountNumber} />
          <DetailRow label="Account Holder" value={profile.bankAccountHolderName} />
        </CardContent>
      </Card>

      {/* Operational Specs */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm flex items-center gap-2">
            <Settings className="h-4 w-4" />
            Operational Specifications
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <div className="text-muted-foreground">Subcourts</div>
              <div className="font-medium">{profile.operationalSpecs.subcourt_count}</div>
            </div>
            <div>
              <div className="text-muted-foreground">Surface Type</div>
              <div className="font-medium capitalize">{profile.operationalSpecs.surface_type}</div>
            </div>
            <div>
              <div className="text-muted-foreground">Opening Hours</div>
              <div className="font-medium">{profile.operationalSpecs.operating_hours.open} - {profile.operationalSpecs.operating_hours.close}</div>
            </div>
          </div>
          <Separator />
          <div>
            <div className="text-sm font-medium mb-2">Base Pricing Rules</div>
            <div className="rounded-md border overflow-hidden">
              <table className="w-full text-sm">
                <thead className="bg-muted">
                  <tr>
                    <th className="px-3 py-2 text-left font-medium text-muted-foreground">Day Type</th>
                    <th className="px-3 py-2 text-left font-medium text-muted-foreground">Start</th>
                    <th className="px-3 py-2 text-left font-medium text-muted-foreground">End</th>
                    <th className="px-3 py-2 text-right font-medium text-muted-foreground">Price/hr</th>
                  </tr>
                </thead>
                <tbody>
                  {profile.operationalSpecs.base_pricing.map((rule, idx) => (
                    <tr key={idx} className="border-t">
                      <td className="px-3 py-2 capitalize">{rule.day_type}</td>
                      <td className="px-3 py-2">{rule.start_time}</td>
                      <td className="px-3 py-2">{rule.end_time}</td>
                      <td className="px-3 py-2 text-right font-medium">{rule.price_per_hour.toLocaleString()} VND</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Submission Metadata */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Submission Details</CardTitle>
        </CardHeader>
        <CardContent className="space-y-0">
          <DetailRow label="Submitted At" value={format(new Date(profile.submittedAt), 'PPP p')} />
          {profile.reviewedAt && (
            <DetailRow label="Reviewed At" value={format(new Date(profile.reviewedAt), 'PPP p')} />
          )}
          {profile.reviewedBy && <DetailRow label="Reviewed By" value={profile.reviewedBy} />}
        </CardContent>
      </Card>

      {/* Actions */}
      <div className="flex gap-3 pt-2">
        {canEditResubmit && (
          <Button onClick={() => navigate('/owner/business-profile')} className="gap-2">
            <RotateCcw className="h-4 w-4" />
            Edit and Resubmit
          </Button>
        )}
        {canDeleteAndResubmit && (
          <Button variant="destructive" onClick={handleDeleteAndResubmit} disabled={deleteProfile.isPending} className="gap-2">
            <Trash2 className="h-4 w-4" />
            {deleteProfile.isPending ? 'Removing...' : 'Submit New Application'}
          </Button>
        )}
      </div>
    </div>
  )
}
