import { useFormContext } from 'react-hook-form'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import type { ApplicationFormData } from '../schema'

export function StepLegalInfo() {
  const {
    register,
    formState: { errors },
  } = useFormContext<ApplicationFormData>()

  return (
    <div className="space-y-4">
      <div>
        <Label htmlFor="legalRepresentativeName">Legal Representative Name</Label>
        <Input
          id="legalRepresentativeName"
          {...register('legalRepresentativeName')}
          placeholder="Full name"
          className="mt-1"
        />
        {errors.legalRepresentativeName && (
          <p className="text-sm text-destructive mt-1">{errors.legalRepresentativeName.message}</p>
        )}
      </div>
      <div>
        <Label htmlFor="personalIdNumber">Personal ID Number</Label>
        <Input
          id="personalIdNumber"
          {...register('personalIdNumber')}
          placeholder="ID number"
          className="mt-1"
        />
        {errors.personalIdNumber && (
          <p className="text-sm text-destructive mt-1">{errors.personalIdNumber.message}</p>
        )}
      </div>
    </div>
  )
}
