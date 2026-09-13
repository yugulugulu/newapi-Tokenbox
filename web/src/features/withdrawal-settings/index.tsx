import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Pencil, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  createWithdrawalConfig,
  deleteWithdrawalConfig,
  getWithdrawalConfigs,
  updateWithdrawalConfig,
} from '@/features/commissions/api'
import type { WithdrawalConfig } from '@/features/commissions/types'
import { formatTimestampToDate } from '@/lib/format'

function formatUsdt(minor: number) {
  return `${new Intl.NumberFormat(undefined, {
    maximumFractionDigits: 2,
  }).format(minor / 100)} USDT`
}

function formatExchangeRate(minor: number) {
  return new Intl.NumberFormat(undefined, {
    maximumFractionDigits: 6,
  }).format(minor / 1_000_000)
}

interface ConfigFormState {
  id?: number
  exchangeRate: string
  feeUsdt: string
  minAmountUsdt: string
  enabled: boolean
}

const emptyForm: ConfigFormState = {
  exchangeRate: '7.2',
  feeUsdt: '1',
  minAmountUsdt: '10',
  enabled: false,
}

function toForm(config: WithdrawalConfig): ConfigFormState {
  return {
    id: config.id,
    exchangeRate: String(config.exchange_rate_minor / 1_000_000),
    feeUsdt: String(config.fee_usdt_minor / 100),
    minAmountUsdt: String(config.min_amount_usdt_minor / 100),
    enabled: config.enabled,
  }
}

export function WithdrawalSettings() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [form, setForm] = useState<ConfigFormState>(emptyForm)

  const configsQuery = useQuery({
    queryKey: ['withdrawal-configs'],
    queryFn: getWithdrawalConfigs,
  })

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ['withdrawal-configs'] })

  const saveMutation = useMutation({
    mutationFn: () => {
      const exchangeRate = Number(form.exchangeRate)
      const feeUsdt = Number(form.feeUsdt)
      const minAmountUsdt = Number(form.minAmountUsdt)
      if (
        !Number.isFinite(exchangeRate) ||
        exchangeRate <= 0 ||
        !Number.isFinite(feeUsdt) ||
        feeUsdt < 0 ||
        !Number.isFinite(minAmountUsdt) ||
        minAmountUsdt <= 0
      ) {
        throw new Error('invalid withdrawal config')
      }
      const payload = {
        exchange_rate: exchangeRate,
        fee_usdt: feeUsdt,
        min_amount_usdt: minAmountUsdt,
        enabled: form.enabled,
      }
      return form.id
        ? updateWithdrawalConfig(form.id, payload)
        : createWithdrawalConfig(payload)
    },
    onSuccess: () => {
      toast.success(t('Saved'))
      setDialogOpen(false)
      void invalidate()
    },
    onError: () => toast.error(t('Save failed')),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteWithdrawalConfig(id),
    onSuccess: () => {
      toast.success(t('Deleted'))
      void invalidate()
    },
    onError: () => toast.error(t('Delete failed')),
  })

  const openCreate = () => {
    setForm(emptyForm)
    setDialogOpen(true)
  }

  const openEdit = (config: WithdrawalConfig) => {
    setForm(toForm(config))
    setDialogOpen(true)
  }

  const configs = configsQuery.data?.data?.items ?? []

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Withdrawal Settings')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button size='sm' onClick={openCreate}>
            <Plus />
            {t('Add configuration')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='rounded-lg border'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Currency')}</TableHead>
                  <TableHead>{t('Network')}</TableHead>
                  <TableHead>{t('Exchange rate')}</TableHead>
                  <TableHead>{t('Withdrawal fee')}</TableHead>
                  <TableHead>{t('Minimum withdrawal amount')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead>{t('Updated at')}</TableHead>
                  <TableHead>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {configs.length === 0 ? (
                  <TableRow>
                    <TableCell
                      colSpan={8}
                      className='text-muted-foreground text-center'
                    >
                      {t('No records')}
                    </TableCell>
                  </TableRow>
                ) : (
                  configs.map((config) => (
                    <TableRow key={config.id}>
                      <TableCell>{config.currency}</TableCell>
                      <TableCell>{config.network}</TableCell>
                      <TableCell>
                        {formatExchangeRate(config.exchange_rate_minor)}
                      </TableCell>
                      <TableCell>{formatUsdt(config.fee_usdt_minor)}</TableCell>
                      <TableCell>
                        {formatUsdt(config.min_amount_usdt_minor)}
                      </TableCell>
                      <TableCell>
                        <Badge variant={config.enabled ? 'default' : 'outline'}>
                          {config.enabled ? t('Enabled') : t('Disabled')}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        {formatTimestampToDate(config.updated_at)}
                      </TableCell>
                      <TableCell>
                        <div className='flex gap-2'>
                          <Button
                            size='icon-sm'
                            variant='outline'
                            onClick={() => openEdit(config)}
                            aria-label={t('Edit')}
                            title={t('Edit')}
                          >
                            <Pencil />
                          </Button>
                          <Button
                            size='icon-sm'
                            variant='destructive'
                            onClick={() => {
                              if (
                                window.confirm(t('Delete this configuration?'))
                              ) {
                                deleteMutation.mutate(config.id)
                              }
                            }}
                            disabled={deleteMutation.isPending}
                            aria-label={t('Delete')}
                            title={t('Delete')}
                          >
                            <Trash2 />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <Dialog
        open={dialogOpen}
        onOpenChange={(open) => {
          setDialogOpen(open)
          if (!open) setForm(emptyForm)
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {form.id
                ? t('Edit withdrawal configuration')
                : t('Add configuration')}
            </DialogTitle>
          </DialogHeader>
          <div className='grid gap-4'>
            <div className='grid gap-2'>
              <Label>{t('Currency')}</Label>
              <Input value='USDT' disabled />
            </div>
            <div className='grid gap-2'>
              <Label>{t('Network')}</Label>
              <Input value='BSC' disabled />
            </div>
            <div className='grid gap-2'>
              <Label>{t('Exchange rate')}</Label>
              <Input
                type='number'
                min='0'
                step='0.000001'
                value={form.exchangeRate}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    exchangeRate: event.target.value,
                  }))
                }
              />
            </div>
            <div className='grid gap-2'>
              <Label>{t('Withdrawal fee')}</Label>
              <Input
                type='number'
                min='0'
                step='0.01'
                value={form.feeUsdt}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    feeUsdt: event.target.value,
                  }))
                }
              />
            </div>
            <div className='grid gap-2'>
              <Label>{t('Minimum withdrawal amount')}</Label>
              <Input
                type='number'
                min='0'
                step='0.01'
                value={form.minAmountUsdt}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    minAmountUsdt: event.target.value,
                  }))
                }
              />
            </div>
            <div className='flex items-center justify-between gap-3'>
              <Label>{t('Enabled')}</Label>
              <Switch
                checked={form.enabled}
                onCheckedChange={(checked) =>
                  setForm((current) => ({ ...current, enabled: checked }))
                }
                aria-label={t('Enabled')}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setDialogOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button
              onClick={() => saveMutation.mutate()}
              disabled={saveMutation.isPending}
            >
              {t('Confirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
