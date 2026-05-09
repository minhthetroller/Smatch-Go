import { useSearchParams } from 'react-router-dom'
import { useBusinessProfile } from './hooks'
import { StatusDashboard } from './StatusDashboard'
import { ApplicationWizard } from './ApplicationWizard'

export function BusinessProfilePage() {
  const { data, isLoading } = useBusinessProfile()
  const [searchParams] = useSearchParams()
  const stepParam = searchParams.get('step') || undefined

  if (isLoading) return <div className="p-4">Loading...</div>

  if (!data) {
    return <ApplicationWizard mode="new" initialStep={stepParam} />
  }

  const editableStatuses: Array<string> = ['rejected', 'resubmit_requested']
  if (editableStatuses.includes(data.status)) {
    return (
      <div className="space-y-4">
        <StatusDashboard profile={data} />
        <ApplicationWizard mode="resubmit" initialData={data} initialStep={stepParam} />
      </div>
    )
  }

  return <StatusDashboard profile={data} />
}
