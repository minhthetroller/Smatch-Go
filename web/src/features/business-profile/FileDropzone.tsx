import { useCallback } from 'react'
import { useDropzone } from 'react-dropzone'
import { X, Upload } from 'lucide-react'
import { cn } from '@/lib/utils'

interface FileDropzoneProps {
  label: string
  file: File | null
  onChange: (file: File | null) => void
  accept?: Record<string, string[]>
}

export function FileDropzone({ label, file, onChange, accept }: FileDropzoneProps) {
  const onDrop = useCallback(
    (acceptedFiles: File[]) => {
      if (acceptedFiles.length > 0) {
        onChange(acceptedFiles[0])
      }
    },
    [onChange]
  )

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop,
    accept: accept ?? {
      'image/*': ['.png', '.jpg', '.jpeg'],
      'application/pdf': ['.pdf'],
    },
    multiple: false,
  })

  return (
    <div className="space-y-2">
      <div className="text-sm font-medium">{label}</div>
      {file ? (
        <div className="flex items-center justify-between rounded-md border px-3 py-2 text-sm">
          <span className="truncate">{file.name}</span>
          <button
            type="button"
            onClick={() => onChange(null)}
            className="ml-2 rounded-full p-1 hover:bg-muted"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      ) : (
        <div
          {...getRootProps()}
          className={cn(
            'flex cursor-pointer flex-col items-center justify-center rounded-md border-2 border-dashed border-muted-foreground/25 px-4 py-6 transition-colors hover:bg-muted/50',
            isDragActive && 'border-primary bg-muted/50'
          )}
        >
          <input {...getInputProps()} />
          <Upload className="mb-2 h-5 w-5 text-muted-foreground" />
          <p className="text-xs text-muted-foreground">Click or drag file here</p>
        </div>
      )}
    </div>
  )
}
