import { useFormContext } from 'react-hook-form'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { FileDropzone } from '../FileDropzone'
import { Info, FileCheck, Building2 } from 'lucide-react'
import type { ApplicationFormData } from '../schema'

interface Props {
  files: Record<string, File | null>
  onFileChange: (key: string, file: File | null) => void
}

export function StepBusinessDocs({ files, onFileChange }: Props) {
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
            <p className="font-medium">Business Registration Requirements</p>
            <p>
              Vietnamese sports business regulations require these four certificates for court
              owners. If you do not yet have a Sports Business Eligibility Certificate, contact
              your local Department of Culture and Sports to apply.
            </p>
          </div>
        </div>
      </div>

      <div className="rounded-lg border bg-amber-50/50 p-4">
        <div className="flex gap-3">
          <Building2 className="h-5 w-5 text-amber-600 shrink-0 mt-0.5" />
          <div className="text-sm text-amber-800 space-y-2">
            <p className="font-medium">Required documents checklist</p>
            <ul className="space-y-1.5">
              <li className="flex items-start gap-2">
                <FileCheck className="h-4 w-4 text-amber-600 shrink-0 mt-0.5" />
                <span><strong>Business Registration Certificate</strong> — Giấy chứng nhận đăng ký doanh nghiệp</span>
              </li>
              <li className="flex items-start gap-2">
                <FileCheck className="h-4 w-4 text-amber-600 shrink-0 mt-0.5" />
                <span><strong>Sports Business Eligibility Certificate</strong> — Giấy phép kinh doanh thể thao</span>
              </li>
              <li className="flex items-start gap-2">
                <FileCheck className="h-4 w-4 text-amber-600 shrink-0 mt-0.5" />
                <span><strong>Fire Safety Certificate</strong> — Giấy chứng nhận an toàn phòng cháy chữa cháy</span>
              </li>
              <li className="flex items-start gap-2">
                <FileCheck className="h-4 w-4 text-amber-600 shrink-0 mt-0.5" />
                <span><strong>Proof of Address</strong> — Hóa đơn điện/nước or lease agreement for the facility</span>
              </li>
            </ul>
          </div>
        </div>
      </div>

      <div className="space-y-4">
        <div>
          <Label htmlFor="taxIdNumber">Tax ID Number (MST)</Label>
          <p className="text-xs text-muted-foreground mt-1">
            10-digit tax code issued by the General Department of Taxation.
          </p>
          <Input
            id="taxIdNumber"
            {...register('taxIdNumber')}
            placeholder="e.g. 0101234567"
            className="mt-1.5"
          />
          {errors.taxIdNumber && (
            <p className="text-sm text-destructive mt-1">{errors.taxIdNumber.message}</p>
          )}
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <FileDropzone
            label="Business Registration Cert *"
            file={files.businessRegistrationCert}
            onChange={(f) => onFileChange('businessRegistrationCert', f)}
          />
          <FileDropzone
            label="Sports Business Eligibility Cert *"
            file={files.sportsBusinessEligibilityCert}
            onChange={(f) => onFileChange('sportsBusinessEligibilityCert', f)}
          />
          <FileDropzone
            label="Fire Safety Cert *"
            file={files.fireSafetyCert}
            onChange={(f) => onFileChange('fireSafetyCert', f)}
          />
          <FileDropzone
            label="Proof of Address *"
            file={files.proofOfAddress}
            onChange={(f) => onFileChange('proofOfAddress', f)}
          />
        </div>
      </div>
    </div>
  )
}
