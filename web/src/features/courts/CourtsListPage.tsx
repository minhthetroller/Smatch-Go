import { useNavigate } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { useOwnerCourts } from './hooks'
import { BarChart3, MapPin, ChevronRight } from 'lucide-react'

export function CourtsListPage() {
  const navigate = useNavigate()
  const { data: courts, isLoading } = useOwnerCourts()

  if (isLoading) return <div className="p-4">Loading courts...</div>
  if (!courts || courts.length === 0) {
    return (
      <div className="p-4 text-muted-foreground">
        No courts found. Your approved business profile should auto-create a court.
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold">My Courts</h2>
      <div className="grid gap-3">
        {courts.map((court) => (
          <Card key={court.id} className="cursor-pointer hover:shadow-sm transition-shadow">
            <CardContent className="p-4">
              <div className="flex items-start justify-between">
                <div className="space-y-1">
                  <div className="font-medium">{court.name}</div>
                  <div className="flex items-center text-xs text-muted-foreground">
                    <MapPin className="mr-1 h-3 w-3" />
                    {[court.addressStreet, court.addressDistrict, court.addressCity]
                      .filter(Boolean)
                      .join(', ') || 'No address'}
                  </div>
                  <div className="flex gap-2 pt-1">
                    <Badge variant={court.isActive ? 'default' : 'secondary'}>
                      {court.isActive ? 'Active' : 'Inactive'}
                    </Badge>
                  </div>
                </div>
                <div className="flex gap-1">
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => navigate(`/owner/courts/${court.id}/stats`)}
                  >
                    <BarChart3 className="mr-1 h-4 w-4" />
                    Stats
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => navigate(`/owner/courts/${court.id}/subcourts`)}
                  >
                    Manage
                    <ChevronRight className="ml-1 h-4 w-4" />
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  )
}
