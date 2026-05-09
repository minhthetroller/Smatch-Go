import { useFormContext, Controller } from 'react-hook-form'
import { FileDropzone } from '../FileDropzone'
import { Info, FileCheck } from 'lucide-react'
import type { ApplicationFormData } from '../schema'

interface Props {
  files: Record<string, File | null>
  onFileChange: (key: string, file: File | null) => void
}

export function StepIdentityDocs({ files, onFileChange }: Props) {
  const { control, formState: { errors } } = useFormContext<ApplicationFormData>()

  return (
    <div className="space-y-6">
      <div className="rounded-lg bg-blue-50 border border-blue-200 p-4">
        <div className="flex gap-3">
          <Info className="h-5 w-5 text-blue-600 shrink-0 mt-0.5" />
          <div className="text-sm text-blue-800 space-y-1">
            <p className="font-medium">Document Requirements</p>
            <p>
              Upload clear, color scans or photos of both sides of your Personal ID (CCCD/CMND/Passport).
              All documents must be valid and not expired.
            </p>
          </div>
        </div>
      </div>

      <div className="rounded-lg border bg-amber-50/50 p-4">
        <div className="flex gap-3">
          <FileCheck className="h-5 w-5 text-amber-600 shrink-0 mt-0.5" />
          <div className="text-sm text-amber-800 space-y-1">
            <p className="font-medium">Accepted formats</p>
            <ul className="list-disc list-inside space-y-0.5">
              <li>File types: JPG, PNG, PDF</li>
              <li>Maximum size: 5MB per file</li>
              <li>Ensure all corners are visible and text is readable</li>
              <li>Do not crop or obscure any information</li>
            </ul>
          </div>
        </div>
      </div>

      <div className="space-y-4">
        <Controller
          name="personalIdNumber"
          control={control}
          render={() => (
            <FileDropzone
              label="Personal ID Front *"
              file={files.personalIdFront}
              onChange={(f) => onFileChange('personalIdFront', f)}
            />
          )}
        />
        <Controller
          name="personalIdNumber"
          control={control}
          render={() => (
            <FileDropzone
              label="Personal ID Back *"
              file={files.personalIdBack}
              onChange={(f) => onFileChange('personalIdBack', f)}
            />
          )}
        />
        {(errors as unknown as Record<string, { message?: string }>)?.files?.personalIdFront && (
          <p className="text-sm text-destructive">ID front is required</p>
        )}
      </div>
    </div>
  )
}
