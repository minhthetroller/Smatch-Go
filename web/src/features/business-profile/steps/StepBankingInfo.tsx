import { useState } from 'react'
import { useFormContext } from 'react-hook-form'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command'
import { Button } from '@/components/ui/button'
import { Check, ChevronsUpDown, Info, Wallet } from 'lucide-react'
import { cn } from '@/lib/utils'
import { BANK_LIST } from '../constants'
import type { ApplicationFormData } from '../schema'

export function StepBankingInfo() {
  const {
    register,
    setValue,
    watch,
    formState: { errors },
  } = useFormContext<ApplicationFormData>()

  const [open, setOpen] = useState(false)
  const bankName = watch('bankName')

  return (
    <div className="space-y-6">
      <div className="rounded-lg bg-blue-50 border border-blue-200 p-4">
        <div className="flex gap-3">
          <Info className="h-5 w-5 text-blue-600 shrink-0 mt-0.5" />
          <div className="text-sm text-blue-800 space-y-1">
            <p className="font-medium">Payout Information</p>
            <p>
              Revenue from court bookings will be transferred to this account weekly.
              For security and regulatory compliance, the account holder name must exactly
              match the Legal Representative name entered in Step 1.
            </p>
          </div>
        </div>
      </div>

      <div className="rounded-lg border bg-amber-50/50 p-4">
        <div className="flex gap-3">
          <Wallet className="h-5 w-5 text-amber-600 shrink-0 mt-0.5" />
          <div className="text-sm text-amber-800 space-y-1">
            <p className="font-medium">Important notes</p>
            <ul className="list-disc list-inside space-y-0.5">
              <li>We currently support most Vietnamese banks</li>
              <li>Business accounts are preferred but personal accounts are accepted</li>
              <li>Bank account holder name must match legal representative name</li>
              <li>Payouts are processed every Monday for the previous week's revenue</li>
            </ul>
          </div>
        </div>
      </div>

      <div className="space-y-4">
        <div>
          <Label>Bank Name</Label>
          <p className="text-xs text-muted-foreground mt-1">
            Select your bank from the list below.
          </p>
          <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
              <Button
                variant="outline"
                role="combobox"
                aria-expanded={open}
                className="w-full justify-between mt-1.5 font-normal"
              >
                {bankName || 'Select bank...'}
                <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-full p-0">
              <Command>
                <CommandInput placeholder="Search bank..." />
                <CommandList>
                  <CommandEmpty>No bank found.</CommandEmpty>
                  <CommandGroup className="max-h-64 overflow-auto">
                    {BANK_LIST.map((bank) => (
                      <CommandItem
                        key={bank}
                        value={bank}
                        onSelect={(currentValue) => {
                          setValue('bankName', currentValue, { shouldValidate: true })
                          setOpen(false)
                        }}
                      >
                        <Check
                          className={cn(
                            'mr-2 h-4 w-4',
                            bankName === bank ? 'opacity-100' : 'opacity-0'
                          )}
                        />
                        {bank}
                      </CommandItem>
                    ))}
                  </CommandGroup>
                </CommandList>
              </Command>
            </PopoverContent>
          </Popover>
          {errors.bankName && (
            <p className="text-sm text-destructive mt-1">{errors.bankName.message}</p>
          )}
        </div>

        <div>
          <Label htmlFor="bankBranch">Bank Branch</Label>
          <p className="text-xs text-muted-foreground mt-1">
            The branch where your account was opened.
          </p>
          <Input id="bankBranch" {...register('bankBranch')} placeholder="e.g. Hà Nội" className="mt-1.5" />
          {errors.bankBranch && (
            <p className="text-sm text-destructive mt-1">{errors.bankBranch.message}</p>
          )}
        </div>

        <div>
          <Label htmlFor="bankAccountNumber">Account Number</Label>
          <p className="text-xs text-muted-foreground mt-1">
            Your bank account number for receiving payouts.
          </p>
          <Input id="bankAccountNumber" {...register('bankAccountNumber')} placeholder="e.g. 1234567890" className="mt-1.5" />
          {errors.bankAccountNumber && (
            <p className="text-sm text-destructive mt-1">{errors.bankAccountNumber.message}</p>
          )}
        </div>

        <div>
          <Label htmlFor="bankAccountHolderName">Account Holder Name</Label>
          <p className="text-xs text-muted-foreground mt-1">
            Must exactly match the Legal Representative name from Step 1.
          </p>
          <Input
            id="bankAccountHolderName"
            {...register('bankAccountHolderName')}
            placeholder="e.g. Nguyễn Văn A"
            className="mt-1.5"
          />
          {errors.bankAccountHolderName && (
            <p className="text-sm text-destructive mt-1">{errors.bankAccountHolderName.message}</p>
          )}
        </div>
      </div>
    </div>
  )
}
