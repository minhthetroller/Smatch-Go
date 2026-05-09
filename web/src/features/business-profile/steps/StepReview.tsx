import { useFormContext } from 'react-hook-form'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Pencil } from 'lucide-react'
import type { ApplicationFormData } from '../schema'

interface Props {
  files: Record<string, File | null>
  onEditStep: (stepIndex: number) => void
}

function Section({ title, stepIndex, children, onEdit }: { title: string; stepIndex: number; children: React.ReactNode; onEdit: (i: number) => void }) {
  return (
    <details className="rounded-md border group" open>
      <summary className="flex cursor-pointer items-center justify-between px-4 py-3 text-sm font-medium hover:bg-muted/50 list-none">
        <span>{title}</span>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="h-7 gap-1 text-xs"
          onClick={(e) => {
            e.preventDefault()
            onEdit(stepIndex)
          }}
        >
          <Pencil className="h-3 w-3" />
          Edit
        </Button>
      </summary>
      <div className="border-t px-4 py-3 text-sm space-y-2">{children}</div>
    </details>
  )
}

export function StepReview({ files, onEditStep }: Props) {
  const { watch } = useFormContext<ApplicationFormData>()
  const data = watch()

  const fileNames = Object.entries(files)
    .filter(([, f]) => f !== null)
    .map(([k, f]) => ({ key: k, name: f!.name }))

  return (
    <div className="space-y-3">
      <Section title="Step 1: Representative Info" stepIndex={0} onEdit={onEditStep}>
        <div className="grid grid-cols-2 gap-2">
          <div className="text-muted-foreground">Name</div>
          <div>{data.legalRepresentativeName}</div>
          <div className="text-muted-foreground">Personal ID</div>
          <div>{data.personalIdNumber}</div>
        </div>
      </Section>

      <Section title="Step 2: Identity Documents" stepIndex={1} onEdit={onEditStep}>
        <div className="space-y-1">
          {files.personalIdFront && <div>Front: {files.personalIdFront.name}</div>}
          {files.personalIdBack && <div>Back: {files.personalIdBack.name}</div>}
          {!files.personalIdFront && !files.personalIdBack && (
            <span className="text-muted-foreground">Using previously uploaded documents</span>
          )}
        </div>
      </Section>

      <Section title="Step 3: Business Documents" stepIndex={2} onEdit={onEditStep}>
        <div className="grid grid-cols-2 gap-2">
          <div className="text-muted-foreground">Tax ID</div>
          <div>{data.taxIdNumber}</div>
        </div>
        <div className="space-y-1 mt-2">
          {files.businessRegistrationCert && <div>Business Reg: {files.businessRegistrationCert.name}</div>}
          {files.sportsBusinessEligibilityCert && <div>Sports Eligibility: {files.sportsBusinessEligibilityCert.name}</div>}
          {files.fireSafetyCert && <div>Fire Safety: {files.fireSafetyCert.name}</div>}
          {files.proofOfAddress && <div>Proof of Address: {files.proofOfAddress.name}</div>}
          {fileNames.length === 0 && (
            <span className="text-muted-foreground">Using previously uploaded documents</span>
          )}
        </div>
      </Section>

      <Section title="Step 4: Banking Information" stepIndex={3} onEdit={onEditStep}>
        <div className="grid grid-cols-2 gap-2">
          <div className="text-muted-foreground">Bank</div>
          <div>{data.bankName}</div>
          <div className="text-muted-foreground">Branch</div>
          <div>{data.bankBranch}</div>
          <div className="text-muted-foreground">Account Number</div>
          <div>{data.bankAccountNumber}</div>
          <div className="text-muted-foreground">Account Holder</div>
          <div>{data.bankAccountHolderName}</div>
        </div>
      </Section>

      <Section title="Step 5: Operational Specs" stepIndex={4} onEdit={onEditStep}>
        <div className="grid grid-cols-2 gap-2">
          <div className="text-muted-foreground">Subcourts</div>
          <div>{data.operationalSpecs?.subcourt_count}</div>
          <div className="text-muted-foreground">Surface</div>
          <div>{data.operationalSpecs?.surface_type}</div>
          <div className="text-muted-foreground">Hours</div>
          <div>
            {data.operationalSpecs?.operating_hours?.open} - {data.operationalSpecs?.operating_hours?.close}
          </div>
        </div>
        <div className="mt-2">
          <div className="text-muted-foreground mb-1">Pricing Rules ({data.operationalSpecs?.base_pricing?.length || 0})</div>
          <ul className="space-y-1">
            {data.operationalSpecs?.base_pricing?.map((rule, i) => (
              <li key={i} className="text-xs">
                {rule.day_type} | {rule.start_time} - {rule.end_time} | {rule.price_per_hour.toLocaleString()} VND/h
              </li>
            ))}
          </ul>
        </div>
      </Section>
    </div>
  )
}
