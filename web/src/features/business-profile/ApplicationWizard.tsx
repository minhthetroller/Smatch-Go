import { useState, useCallback, useEffect } from 'react'
import { useForm, FormProvider } from 'react-hook-form'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { zodResolver } from '@hookform/resolvers/zod'
import { Button } from '@/components/ui/button'
import { toast } from 'sonner'
import { applicationSchema, type ApplicationFormData } from './schema'
import { useSubmitApplication, useUpdateApplication } from './hooks'
import { StepRepresentative } from './steps/StepRepresentative'
import { StepIdentityDocs } from './steps/StepIdentityDocs'
import { StepBusinessDocs } from './steps/StepBusinessDocs'
import { StepBankingInfo } from './steps/StepBankingInfo'
import { StepOperationalSpecs } from './steps/StepOperationalSpecs'
import { StepReview } from './steps/StepReview'
import type { BusinessProfileResponse } from '@/types/api'

const STEPS = [
  {
    id: 'representative',
    title: 'Representative Info',
    description: 'Legal representative details',
    component: StepRepresentative,
  },
  {
    id: 'identity',
    title: 'Identity Documents',
    description: 'ID card or passport uploads',
    component: StepIdentityDocs,
  },
  {
    id: 'business',
    title: 'Business Documents',
    description: 'Certificates and registrations',
    component: StepBusinessDocs,
  },
  {
    id: 'banking',
    title: 'Banking Information',
    description: 'Payout account details',
    component: StepBankingInfo,
  },
  {
    id: 'operational',
    title: 'Operational Specs',
    description: 'Courts, hours, and pricing',
    component: StepOperationalSpecs,
  },
]

function getDefaultValues(initial?: BusinessProfileResponse): ApplicationFormData {
  return {
    legalRepresentativeName: initial?.legalRepresentativeName || '',
    personalIdNumber: initial?.personalIdNumber || '',
    taxIdNumber: initial?.taxIdNumber || '',
    bankAccountNumber: initial?.bankAccountNumber || '',
    bankName: initial?.bankName || '',
    bankBranch: initial?.bankBranch || '',
    bankAccountHolderName: initial?.bankAccountHolderName || '',
    operationalSpecs: {
      subcourt_count: initial?.operationalSpecs?.subcourt_count || 1,
      surface_type: initial?.operationalSpecs?.surface_type || 'synthetic',
      operating_hours: {
        open: initial?.operationalSpecs?.operating_hours?.open || '06:00',
        close: initial?.operationalSpecs?.operating_hours?.close || '22:00',
      },
      base_pricing: initial?.operationalSpecs?.base_pricing?.length
        ? initial.operationalSpecs.base_pricing.map((r) => ({
            day_type: r.day_type as 'weekday' | 'weekend' | 'holiday',
            start_time: r.start_time,
            end_time: r.end_time,
            price_per_hour: r.price_per_hour,
          }))
        : [{ day_type: 'weekday', start_time: '06:00', end_time: '22:00', price_per_hour: 0 }],
    },
  }
}

function stepIdToIndex(id?: string): number {
  if (!id) return 0
  const idx = STEPS.findIndex((s) => s.id === id)
  return idx >= 0 ? idx : 0
}

export function ApplicationWizard({
  mode,
  initialData,
  initialStep,
}: {
  mode: 'new' | 'resubmit'
  initialData?: BusinessProfileResponse
  initialStep?: string
}) {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const stepFromUrl = searchParams.get('step') || initialStep
  const initialIndex = stepIdToIndex(stepFromUrl)

  const [activeStep, setActiveStep] = useState(initialIndex)
  const [visitedSteps, setVisitedSteps] = useState<Set<number>>(new Set([initialIndex]))
  const [completedSteps, setCompletedSteps] = useState<Set<number>>(new Set())
  const [showReview, setShowReview] = useState(stepFromUrl === 'review')

  // React to URL changes from sidebar navigation
  useEffect(() => {
    const newIndex = stepIdToIndex(stepFromUrl)
    setActiveStep(newIndex)
    setShowReview(stepFromUrl === 'review')
    setVisitedSteps((prev) => new Set([...prev, newIndex]))
  }, [stepFromUrl])

  const syncUrl = useCallback((stepIndex: number, review: boolean) => {
    if (review) {
      navigate('/owner/business-profile?step=review', { replace: true })
    } else {
      const id = STEPS[stepIndex]?.id
      if (id) {
        navigate(`/owner/business-profile?step=${id}`, { replace: true })
      }
    }
  }, [navigate])

  const [files, setFiles] = useState<Record<string, File | null>>({
    personalIdFront: null,
    personalIdBack: null,
    businessRegistrationCert: null,
    sportsBusinessEligibilityCert: null,
    fireSafetyCert: null,
    proofOfAddress: null,
  })

  const methods = useForm<ApplicationFormData>({
    resolver: zodResolver(applicationSchema),
    defaultValues: getDefaultValues(initialData),
    mode: 'onBlur',
  })

  const submitApp = useSubmitApplication()
  const updateApp = useUpdateApplication()

  const validateStep = async (stepIndex: number): Promise<boolean> => {
    const stepFields: Record<number, string[]> = {
      0: ['legalRepresentativeName', 'personalIdNumber'],
      1: [],
      2: ['taxIdNumber'],
      3: ['bankName', 'bankBranch', 'bankAccountNumber', 'bankAccountHolderName'],
      4: [
        'operationalSpecs.subcourt_count',
        'operationalSpecs.surface_type',
        'operationalSpecs.operating_hours.open',
        'operationalSpecs.operating_hours.close',
        'operationalSpecs.base_pricing',
      ],
    }

    const fieldsToValidate = stepFields[stepIndex]
    const valid =
      fieldsToValidate && fieldsToValidate.length > 0
        ? await methods.trigger(fieldsToValidate as any)
        : true

    if (!valid) return false

    const isNewMode = mode === 'new'

    if (stepIndex === 1) {
      const hasFront = isNewMode
        ? !!files.personalIdFront
        : !!(files.personalIdFront || initialData?.personalIdFrontImageUrl)
      const hasBack = isNewMode
        ? !!files.personalIdBack
        : !!(files.personalIdBack || initialData?.personalIdBackImageUrl)
      if (!hasFront || !hasBack) {
        toast.error('Please upload both ID documents')
        return false
      }
    }

    if (stepIndex === 2) {
      const hasBizReg = isNewMode
        ? !!files.businessRegistrationCert
        : !!(files.businessRegistrationCert || initialData?.businessRegistrationCertUrl)
      const hasSports = isNewMode
        ? !!files.sportsBusinessEligibilityCert
        : !!(files.sportsBusinessEligibilityCert || initialData?.sportsBusinessEligibilityCertUrl)
      const hasFire = isNewMode
        ? !!files.fireSafetyCert
        : !!(files.fireSafetyCert || initialData?.fireSafetyCertUrl)
      const hasAddr = isNewMode
        ? !!files.proofOfAddress
        : !!(files.proofOfAddress || initialData?.proofOfAddressUrl)
      if (!hasBizReg || !hasSports || !hasFire || !hasAddr) {
        toast.error('Please upload all business documents')
        return false
      }
    }

    if (stepIndex === 4) {
      const pricing = methods.getValues('operationalSpecs.base_pricing')
      if (!pricing || pricing.length === 0) {
        toast.error('Please add at least one pricing rule')
        return false
      }
    }

    return true
  }

  const handleStepClick = async (stepIndex: number) => {
    // Can always go backwards
    if (stepIndex < activeStep) {
      setActiveStep(stepIndex)
      setShowReview(false)
      syncUrl(stepIndex, false)
      return
    }

    // Can only go forwards if all previous steps are completed
    for (let i = activeStep; i < stepIndex; i++) {
      if (!completedSteps.has(i)) {
        const valid = await validateStep(i)
        if (!valid) {
          setActiveStep(i)
          setShowReview(false)
          syncUrl(i, false)
          return
        }
        setCompletedSteps((prev) => new Set([...prev, i]))
      }
    }

    setActiveStep(stepIndex)
    setVisitedSteps((prev) => new Set([...prev, stepIndex]))
    setShowReview(false)
    syncUrl(stepIndex, false)
  }

  const handleSaveContinue = async () => {
    const valid = await validateStep(activeStep)
    if (!valid) {
      toast.error('Please fix the errors before continuing')
      return
    }

    setCompletedSteps((prev) => new Set([...prev, activeStep]))

    if (activeStep === STEPS.length - 1) {
      setShowReview(true)
      syncUrl(activeStep, true)
    } else {
      const nextStep = activeStep + 1
      setActiveStep(nextStep)
      setVisitedSteps((prev) => new Set([...prev, nextStep]))
      syncUrl(nextStep, false)
    }
  }

  const handlePrevious = () => {
    if (showReview) {
      setShowReview(false)
      setActiveStep(STEPS.length - 1)
      syncUrl(STEPS.length - 1, false)
    } else if (activeStep > 0) {
      const prev = activeStep - 1
      setActiveStep(prev)
      syncUrl(prev, false)
    }
  }

  const handleSubmit = async () => {
    const data = methods.getValues()
    const payload = {
      data,
      files: Object.fromEntries(
        Object.entries(files).filter(([, f]) => f !== null)
      ) as Record<string, File>,
    }

    try {
      if (mode === 'new') {
        await submitApp.mutateAsync(payload)
      } else {
        await updateApp.mutateAsync(payload)
      }
      toast.success('Application submitted successfully')
      navigate('/owner/application-status')
    } catch (err: any) {
      toast.error(err?.message || 'Submission failed')
    }
  }

  const ActiveComponent = STEPS[activeStep].component

  const getStepStatus = (index: number) => {
    if (completedSteps.has(index)) return 'completed'
    if (index === activeStep && !showReview) return 'active'
    if (visitedSteps.has(index)) return 'visited'
    return 'pending'
  }

  return (
    <FormProvider {...methods}>
      <div className="w-full">
        {showReview ? (
          <div className="space-y-6">
            <div className="rounded-lg border bg-card p-6">
              <h2 className="text-lg font-semibold">Review Your Application</h2>
              <p className="text-sm text-muted-foreground mt-1">
                Please review all information before submitting. You can click any section below to go back and edit.
              </p>
              <div className="mt-6">
                <StepReview
                  files={files}
                  onEditStep={(stepIndex) => {
                    setShowReview(false)
                    setActiveStep(stepIndex)
                    syncUrl(stepIndex, false)
                  }}
                />
              </div>
              <div className="flex justify-between mt-6 pt-4 border-t">
                <Button type="button" variant="outline" onClick={handlePrevious}>
                  Previous
                </Button>
                <Button
                  type="button"
                  onClick={handleSubmit}
                  disabled={submitApp.isPending || updateApp.isPending}
                >
                  {submitApp.isPending || updateApp.isPending
                    ? 'Submitting...'
                    : 'Submit Application'}
                </Button>
              </div>
            </div>
          </div>
        ) : (
          <div className="space-y-6">
            <div className="rounded-lg border bg-card">
              <div className="border-b px-6 py-4">
                <div className="flex items-center gap-2">
                  <span className="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-xs font-medium text-primary-foreground">
                    {activeStep + 1}
                  </span>
                  <h2 className="text-lg font-semibold">{STEPS[activeStep].title}</h2>
                </div>
              </div>
              <div className="p-6">
                {activeStep === 0 && <ActiveComponent />}
                {activeStep === 1 && (
                  <ActiveComponent
                    files={files}
                    onFileChange={(key: string, file: File | null) =>
                      setFiles((prev) => ({ ...prev, [key]: file }))
                    }
                  />
                )}
                {activeStep === 2 && (
                  <ActiveComponent
                    files={files}
                    onFileChange={(key: string, file: File | null) =>
                      setFiles((prev) => ({ ...prev, [key]: file }))
                    }
                  />
                )}
                {activeStep === 3 && <ActiveComponent />}
                {activeStep === 4 && <ActiveComponent />}

                <div className="flex justify-between mt-8 pt-4 border-t">
                  <Button
                    type="button"
                    variant="outline"
                    onClick={handlePrevious}
                    disabled={activeStep === 0 && !showReview}
                  >
                    Previous
                  </Button>
                  <Button type="button" onClick={handleSaveContinue}>
                    {activeStep === STEPS.length - 1 ? 'Review Application' : 'Save & Continue'}
                  </Button>
                </div>
              </div>
            </div>
          </div>
        )}
      </div>
    </FormProvider>
  )
}
