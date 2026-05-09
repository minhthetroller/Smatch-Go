import { useFormContext, useFieldArray } from 'react-hook-form'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Plus, Trash2, Info, Clock } from 'lucide-react'
import { SURFACE_TYPES } from '../constants'
import type { ApplicationFormData } from '../schema'

export function StepOperationalSpecs() {
  const {
    register,
    control,
    setValue,
    watch,
    formState: { errors },
  } = useFormContext<ApplicationFormData>()

  const { fields, append, remove } = useFieldArray({
    control,
    name: 'operationalSpecs.base_pricing',
  })

  const surfaceType = watch('operationalSpecs.surface_type')

  return (
    <div className="space-y-6">
      <div className="rounded-lg bg-blue-50 border border-blue-200 p-4">
        <div className="flex gap-3">
          <Info className="h-5 w-5 text-blue-600 shrink-0 mt-0.5" />
          <div className="text-sm text-blue-800 space-y-1">
            <p className="font-medium">Facility Operations</p>
            <p>
              Define how your facility operates. Subcourts are individual playable courts within
              your location (e.g., Court A, Court B). Add at least one pricing rule per day type
              to define your hourly rates.
            </p>
          </div>
        </div>
      </div>

      <div className="rounded-lg border bg-amber-50/50 p-4">
        <div className="flex gap-3">
          <Clock className="h-5 w-5 text-amber-600 shrink-0 mt-0.5" />
          <div className="text-sm text-amber-800 space-y-1">
            <p className="font-medium">Example pricing setup</p>
            <p>
              A typical badminton center might charge <strong>120,000 VND/hour</strong> on weekdays
              and <strong>150,000 VND/hour</strong> on weekends. You can add multiple time blocks
              for the same day type if your rates vary by hour.
            </p>
          </div>
        </div>
      </div>

      <div className="space-y-4">
        <div>
          <Label htmlFor="subcourt_count">Subcourt Count</Label>
          <p className="text-xs text-muted-foreground mt-1">
            How many individual playable courts do you have? Each court can be booked independently.
          </p>
          <Input
            id="subcourt_count"
            type="number"
            min={1}
            {...register('operationalSpecs.subcourt_count', { valueAsNumber: true })}
            className="mt-1.5"
          />
          {errors.operationalSpecs?.subcourt_count && (
            <p className="text-sm text-destructive mt-1">{errors.operationalSpecs.subcourt_count.message}</p>
          )}
        </div>

        <div>
          <Label>Surface Type</Label>
          <p className="text-xs text-muted-foreground mt-1">
            The primary playing surface of your courts.
          </p>
          <Select
            value={surfaceType || ''}
            onValueChange={(v) => setValue('operationalSpecs.surface_type', v, { shouldValidate: true })}
          >
            <SelectTrigger className="mt-1.5">
              <SelectValue placeholder="Select surface type" />
            </SelectTrigger>
            <SelectContent>
              {SURFACE_TYPES.map((t) => (
                <SelectItem key={t} value={t}>
                  {t.charAt(0).toUpperCase() + t.slice(1)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {errors.operationalSpecs?.surface_type && (
            <p className="text-sm text-destructive mt-1">{errors.operationalSpecs.surface_type.message}</p>
          )}
        </div>

        <div>
          <Label>Operating Hours</Label>
          <p className="text-xs text-muted-foreground mt-1">
            The daily opening and closing times for your facility.
          </p>
          <div className="grid grid-cols-2 gap-4 mt-1.5">
            <div>
              <Label htmlFor="open" className="text-xs text-muted-foreground">Open</Label>
              <Input
                id="open"
                type="time"
                {...register('operationalSpecs.operating_hours.open')}
                className="mt-1"
              />
            </div>
            <div>
              <Label htmlFor="close" className="text-xs text-muted-foreground">Close</Label>
              <Input
                id="close"
                type="time"
                {...register('operationalSpecs.operating_hours.close')}
                className="mt-1"
              />
            </div>
          </div>
          {errors.operationalSpecs?.operating_hours && (
            <p className="text-sm text-destructive mt-1">Invalid operating hours</p>
          )}
        </div>

        <div>
          <div className="flex items-center justify-between">
            <Label>Base Pricing Rules</Label>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() =>
                append({ day_type: 'weekday', start_time: '06:00', end_time: '22:00', price_per_hour: 120000 })
              }
            >
              <Plus className="mr-1 h-3 w-3" />
              Add Row
            </Button>
          </div>
          <p className="text-xs text-muted-foreground mt-1">
            Define hourly rates by day type and time range.
          </p>
          <div className="mt-2 rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Day Type</TableHead>
                  <TableHead>Start</TableHead>
                  <TableHead>End</TableHead>
                  <TableHead>Price/hour (VND)</TableHead>
                  <TableHead className="w-10"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {fields.map((field, index) => (
                  <TableRow key={field.id}>
                    <TableCell>
                      <Select
                        value={watch(`operationalSpecs.base_pricing.${index}.day_type`)}
                        onValueChange={(v) =>
                          setValue(`operationalSpecs.base_pricing.${index}.day_type`, v, { shouldValidate: true })
                        }
                      >
                        <SelectTrigger className="h-8 text-xs">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="weekday">Weekday</SelectItem>
                          <SelectItem value="weekend">Weekend</SelectItem>
                          <SelectItem value="holiday">Holiday</SelectItem>
                        </SelectContent>
                      </Select>
                    </TableCell>
                    <TableCell>
                      <Input
                        type="time"
                        className="h-8 text-xs"
                        {...register(`operationalSpecs.base_pricing.${index}.start_time`)}
                      />
                    </TableCell>
                    <TableCell>
                      <Input
                        type="time"
                        className="h-8 text-xs"
                        {...register(`operationalSpecs.base_pricing.${index}.end_time`)}
                      />
                    </TableCell>
                    <TableCell>
                      <Input
                        type="number"
                        min={0}
                        step={1000}
                        className="h-8 text-xs"
                        placeholder="120000"
                        {...register(`operationalSpecs.base_pricing.${index}.price_per_hour`, { valueAsNumber: true })}
                      />
                    </TableCell>
                    <TableCell>
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7"
                        onClick={() => remove(index)}
                      >
                        <Trash2 className="h-3 w-3" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
                {fields.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={5} className="text-center text-sm text-muted-foreground py-4">
                      No pricing rules. Click "Add Row" to add one.
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
          {errors.operationalSpecs?.base_pricing && (
            <p className="text-sm text-destructive mt-1">{errors.operationalSpecs.base_pricing.message}</p>
          )}
        </div>
      </div>
    </div>
  )
}
