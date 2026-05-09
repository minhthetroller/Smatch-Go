import { useFormContext } from 'react-hook-form'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Info } from 'lucide-react'
import type { ApplicationFormData } from '../schema'

export function StepRepresentative() {
  const {
    register,
    formState: { errors },
  } = useFormContext<ApplicationFormData>()

  return (
    <div className="space-y-6">
      <div className="rounded-lg bg-blue-50 border border-blue-200 p-4">
        <div className="flex gap-3">
          <Info className="h-5 w-5 text-blue-600 shrink-0 mt-0.5" />
          <div className="text-sm text-blue-800 space-y-1">
            <p className="font-medium">Why we need this</p>
            <p>
              The legal representative is the individual authorized to operate the badminton facility
              on behalf of the business. This name must match your government-issued ID exactly.
              Payouts and legal notices will be addressed to this person.
            </p>
          </div>
        </div>
      </div>

      <div className="space-y-4">
        <div>
          <Label htmlFor="legalRepresentativeName">Legal Representative Name</Label>
          <p className="text-xs text-muted-foreground mt-1">
            Enter the full name as it appears on your government-issued ID.
          </p>
          <Input
            id="legalRepresentativeName"
            {...register('legalRepresentativeName')}
            placeholder="e.g. Nguyễn Văn A"
            className="mt-1.5"
          />
          {errors.legalRepresentativeName && (
            <p className="text-sm text-destructive mt-1">{errors.legalRepresentativeName.message}</p>
          )}
        </div>

        <div>
          <Label htmlFor="personalIdNumber">Personal ID Number</Label>
          <p className="text-xs text-muted-foreground mt-1">
            Your CCCD, CMND, or passport number. Must be 9–12 digits.
          </p>
          <Input
            id="personalIdNumber"
            {...register('personalIdNumber')}
            placeholder="e.g. 001203000123"
            className="mt-1.5"
          />
          {errors.personalIdNumber && (
            <p className="text-sm text-destructive mt-1">{errors.personalIdNumber.message}</p>
          )}
        </div>
      </div>
    </div>
  )
}
