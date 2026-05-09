import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { useOwnerCourt, useCloseSubCourt, useOpenSubCourt } from './hooks'
import { DoorOpen, DoorClosed, Clock, AlertCircle } from 'lucide-react'
import { toast } from 'sonner'

export function SubCourtManagerPage() {
  const { id } = useParams<{ id: string }>()
  const { data: court, isLoading } = useOwnerCourt(id!)
  const closeSub = useCloseSubCourt()
  const openSub = useOpenSubCourt()

  const [closeForm, setCloseForm] = useState<Record<string, { date: string; startTime: string; endTime: string; reason: string }>>({})

  if (isLoading) return <div className="p-4">Loading...</div>
  if (!court) return <div className="p-4 text-muted-foreground">Court not found.</div>

  const handleClose = async (subCourtId: string) => {
    const form = closeForm[subCourtId]
    if (!form?.date || !form?.reason) {
      toast.error('Date and reason are required')
      return
    }
    try {
      await closeSub.mutateAsync({
        courtId: id!,
        subCourtId,
        data: {
          date: form.date,
          startTime: form.startTime || undefined,
          endTime: form.endTime || undefined,
          reason: form.reason,
        },
      })
      toast.success('Subcourt closed')
      setCloseForm((prev) => ({ ...prev, [subCourtId]: { date: '', startTime: '', endTime: '', reason: '' } }))
    } catch (err: any) {
      toast.error(err?.message || 'Failed to close')
    }
  }

  const handleOpen = async (subCourtId: string, date: string) => {
    try {
      await openSub.mutateAsync({ courtId: id!, subCourtId, date })
      toast.success('Subcourt opened')
    } catch (err: any) {
      toast.error(err?.message || 'Failed to open')
    }
  }

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold">{court.name} — Subcourts</h2>

      <div className="grid gap-3">
        {court.subCourts.map((sub) => (
          <Card key={sub.id}>
            <CardContent className="p-4">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  {sub.isActive ? (
                    <DoorOpen className="h-5 w-5 text-green-600" />
                  ) : (
                    <DoorClosed className="h-5 w-5 text-muted-foreground" />
                  )}
                  <div>
                    <div className="font-medium">{sub.name}</div>
                    <div className="text-xs text-muted-foreground">
                      {sub.isActive ? 'Active' : 'Inactive'}
                    </div>
                  </div>
                </div>

                <Dialog>
                  <DialogTrigger asChild>
                    <Button variant="outline" size="sm">Close</Button>
                  </DialogTrigger>
                  <DialogContent>
                    <DialogHeader>
                      <DialogTitle>Close {sub.name}</DialogTitle>
                    </DialogHeader>
                    <div className="space-y-3 pt-2">
                      <div>
                        <Label>Date</Label>
                        <Input
                          type="date"
                          value={closeForm[sub.id]?.date || ''}
                          onChange={(e) =>
                            setCloseForm((prev) => ({
                              ...prev,
                              [sub.id]: { ...prev[sub.id], date: e.target.value },
                            }))
                          }
                        />
                      </div>
                      <div className="grid grid-cols-2 gap-2">
                        <div>
                          <Label>Start Time (optional)</Label>
                          <Input
                            type="time"
                            value={closeForm[sub.id]?.startTime || ''}
                            onChange={(e) =>
                              setCloseForm((prev) => ({
                                ...prev,
                                [sub.id]: { ...prev[sub.id], startTime: e.target.value },
                              }))
                            }
                          />
                        </div>
                        <div>
                          <Label>End Time (optional)</Label>
                          <Input
                            type="time"
                            value={closeForm[sub.id]?.endTime || ''}
                            onChange={(e) =>
                              setCloseForm((prev) => ({
                                ...prev,
                                [sub.id]: { ...prev[sub.id], endTime: e.target.value },
                              }))
                            }
                          />
                        </div>
                      </div>
                      <div>
                        <Label>Reason</Label>
                        <Input
                          value={closeForm[sub.id]?.reason || ''}
                          onChange={(e) =>
                            setCloseForm((prev) => ({
                              ...prev,
                              [sub.id]: { ...prev[sub.id], reason: e.target.value },
                            }))
                          }
                          placeholder="Maintenance, event, etc."
                        />
                      </div>
                      <Button
                        className="w-full"
                        onClick={() => handleClose(sub.id)}
                        disabled={closeSub.isPending}
                      >
                        Confirm Close
                      </Button>
                    </div>
                  </DialogContent>
                </Dialog>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      {court.upcomingClosures.length > 0 && (
        <div className="space-y-2">
          <h3 className="text-sm font-medium flex items-center gap-2">
            <AlertCircle className="h-4 w-4" />
            Upcoming Closures
          </h3>
          <div className="space-y-2">
            {court.upcomingClosures.map((cl) => (
              <div key={cl.id} className="flex items-center justify-between rounded-md border px-3 py-2 text-sm">
                <div className="flex items-center gap-2">
                  <Clock className="h-4 w-4 text-muted-foreground" />
                  <span>
                    {cl.date}
                    {cl.startTime && cl.endTime
                      ? ` (${cl.startTime} - ${cl.endTime})`
                      : ' (Full day)'}
                    {' — '}
                    {cl.reason}
                  </span>
                </div>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => handleOpen(cl.subCourtId, cl.date)}
                  disabled={openSub.isPending}
                >
                  Open
                </Button>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
