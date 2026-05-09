import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import api from '@/lib/api'
import { toast } from 'sonner'

interface ProfileCompletionModalProps {
  open: boolean
  onComplete: () => void
}

export function ProfileCompletionModal({ open, onComplete }: ProfileCompletionModalProps) {
  const [loading, setLoading] = useState(false)
  const [phone, setPhone] = useState('')
  const [gender, setGender] = useState('')
  const [street, setStreet] = useState('')
  const [ward, setWard] = useState('')
  const [district, setDistrict] = useState('')
  const [city, setCity] = useState('Hà Nội')

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    try {
      await api.put('/api/auth/me', {
        phoneNumber: phone || undefined,
        gender: gender || undefined,
        addressStreet: street || undefined,
        addressWard: ward || undefined,
        addressDistrict: district || undefined,
        addressCity: city || undefined,
      })
      toast.success('Profile updated')
      onComplete()
    } catch (err: any) {
      toast.error(err.message || 'Failed to update profile')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={() => {}}>
      <DialogContent className="sm:max-w-md" onPointerDownOutside={(e) => e.preventDefault()}>
        <DialogHeader>
          <DialogTitle>Complete Your Profile</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4 pt-2">
          <div>
            <Label htmlFor="pc-phone">Phone Number</Label>
            <Input id="pc-phone" type="tel" value={phone} onChange={(e) => setPhone(e.target.value)} placeholder="+84..." required />
          </div>
          <div>
            <Label htmlFor="pc-gender">Gender</Label>
            <Select value={gender} onValueChange={setGender} required>
              <SelectTrigger id="pc-gender">
                <SelectValue placeholder="Select gender" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="male">Male</SelectItem>
                <SelectItem value="female">Female</SelectItem>
                <SelectItem value="other">Other</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div>
            <Label htmlFor="pc-street">Street Address</Label>
            <Input id="pc-street" value={street} onChange={(e) => setStreet(e.target.value)} required />
          </div>
          <div className="grid grid-cols-3 gap-3">
            <div>
              <Label htmlFor="pc-ward">Ward</Label>
              <Input id="pc-ward" value={ward} onChange={(e) => setWard(e.target.value)} />
            </div>
            <div>
              <Label htmlFor="pc-district">District</Label>
              <Input id="pc-district" value={district} onChange={(e) => setDistrict(e.target.value)} />
            </div>
            <div>
              <Label htmlFor="pc-city">City</Label>
              <Input id="pc-city" value={city} onChange={(e) => setCity(e.target.value)} />
            </div>
          </div>
          <Button type="submit" className="w-full" disabled={loading}>
            {loading ? 'Saving...' : 'Continue'}
          </Button>
        </form>
      </DialogContent>
    </Dialog>
  )
}
