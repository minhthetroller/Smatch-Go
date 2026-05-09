import { z } from 'zod'

export const basePricingRuleSchema = z.object({
  day_type: z.enum(['weekday', 'weekend', 'holiday']),
  start_time: z.string().regex(/^([01]\d|2[0-3]):([0-5]\d)$/, 'Invalid time format'),
  end_time: z.string().regex(/^([01]\d|2[0-3]):([0-5]\d)$/, 'Invalid time format'),
  price_per_hour: z.number().min(0, 'Price must be >= 0'),
})

export const operationalSpecsSchema = z.object({
  subcourt_count: z.number().min(1, 'At least 1 subcourt'),
  surface_type: z.enum(['synthetic', 'wood', 'concrete', 'rubber', 'other']),
  operating_hours: z.object({
    open: z.string().regex(/^([01]\d|2[0-3]):([0-5]\d)$/, 'Invalid time'),
    close: z.string().regex(/^([01]\d|2[0-3]):([0-5]\d)$/, 'Invalid time'),
  }),
  base_pricing: z.array(basePricingRuleSchema).min(1, 'At least one pricing rule'),
})

export const applicationSchema = z.object({
  legalRepresentativeName: z.string().min(1, 'Required'),
  personalIdNumber: z.string().regex(/^[0-9]{9,12}$/, 'ID must be 9-12 digits'),
  taxIdNumber: z.string().min(1, 'Required'),
  bankAccountNumber: z.string().min(1, 'Required'),
  bankName: z.string().min(1, 'Required'),
  bankBranch: z.string().min(1, 'Required'),
  bankAccountHolderName: z.string().min(1, 'Required'),
  operationalSpecs: operationalSpecsSchema,
})

export type ApplicationFormData = z.infer<typeof applicationSchema>
