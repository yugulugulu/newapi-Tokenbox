/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { GitBranch, Loader2, Search, X } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

import { searchUsers, setUserParent } from '../../api'
import {
  USER_ROLE,
  USER_ROLES,
  USER_STATUS,
  isCommissionParticipantRole,
} from '../../constants'
import type { User } from '../../types'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  user: User
  onSuccess: () => void
}

export function SetUserParentDialog(props: Props) {
  const { t } = useTranslation()
  const [keyword, setKeyword] = useState('')
  const [users, setUsers] = useState<User[]>([])
  const [selectedParentId, setSelectedParentId] = useState<number | null>(null)
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!props.open) return
    setKeyword('')
    setUsers([])
    setSelectedParentId(
      props.user.parent_user_id && props.user.parent_user_id > 0
        ? props.user.parent_user_id
        : null
    )
  }, [props.open, props.user])

  useEffect(() => {
    if (!props.open) return
    const timer = window.setTimeout(async () => {
      setLoading(true)
      try {
        const response = await searchUsers({
          keyword: keyword.trim(),
          status: String(USER_STATUS.ENABLED),
          p: 1,
          page_size: 50,
        })
        setUsers(response.data?.items ?? [])
      } catch {
        toast.error(t('Failed to search users'))
      } finally {
        setLoading(false)
      }
    }, 250)
    return () => window.clearTimeout(timer)
  }, [keyword, props.open, t])

  const eligibleUsers = useMemo(
    () =>
      users.filter(
        (candidate) =>
          candidate.id !== props.user.id &&
          candidate.status === USER_STATUS.ENABLED &&
          isCommissionParticipantRole(candidate.role)
      ),
    [props.user.id, users]
  )

  const selectedParent = eligibleUsers.find(
    (candidate) => candidate.id === selectedParentId
  )
  let selectedParentLabel = t('No parent user')
  if (selectedParent) {
    selectedParentLabel = `${selectedParent.email || selectedParent.username} (UID: ${selectedParent.id})`
  } else if (selectedParentId) {
    selectedParentLabel = `UID: ${selectedParentId}`
  }

  let candidateContent
  if (loading) {
    candidateContent = (
      <div className='text-muted-foreground flex justify-center py-6'>
        <Loader2 className='animate-spin' />
      </div>
    )
  } else if (eligibleUsers.length === 0) {
    candidateContent = (
      <p className='text-muted-foreground py-6 text-center text-sm'>
        {t('No eligible parent users found')}
      </p>
    )
  } else {
    candidateContent = eligibleUsers.map((candidate) => {
      const role = USER_ROLES[candidate.role as keyof typeof USER_ROLES]
      const isSelected = candidate.id === selectedParentId
      return (
        <button
          key={candidate.id}
          type='button'
          className={`hover:bg-muted flex w-full items-center justify-between rounded-md border px-3 py-2 text-left ${isSelected ? 'border-primary bg-primary/5' : ''}`}
          onClick={() => setSelectedParentId(candidate.id)}
          aria-pressed={isSelected}
        >
          <span className='min-w-0'>
            <span className='block truncate font-medium'>
              {candidate.email || candidate.username}
            </span>
            <span className='text-muted-foreground block truncate text-xs'>
              {candidate.display_name || candidate.username} · UID:{' '}
              {candidate.id}
            </span>
          </span>
          <Badge variant='outline' className='ml-2 shrink-0'>
            {role ? t(role.labelKey) : candidate.role}
          </Badge>
        </button>
      )
    })
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      const response = await setUserParent(props.user.id, selectedParentId ?? 0)
      if (!response.success) {
        toast.error(response.message || t('Failed to update parent user'))
        return
      }
      toast.success(t('Parent user updated successfully'))
      props.onOpenChange(false)
      props.onSuccess()
    } catch {
      toast.error(t('Failed to update parent user'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={
        <span className='flex items-center gap-2'>
          <GitBranch className='h-5 w-5' />
          {t('Set parent user')}
        </span>
      }
      description={t('Only enabled agents and admins can be selected')}
      contentClassName='sm:max-w-lg'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            variant='outline'
            onClick={() => props.onOpenChange(false)}
            disabled={saving}
          >
            {t('Cancel')}
          </Button>
          <Button onClick={() => void handleSave()} disabled={saving}>
            {saving && <Loader2 className='animate-spin' />}
            {t('Save')}
          </Button>
        </>
      }
    >
      <div className='space-y-3'>
        <div className='bg-muted/50 rounded-md border p-3 text-sm'>
          <p>
            <span className='text-muted-foreground'>{t('Target user')}: </span>
            {props.user.email || props.user.username} (UID: {props.user.id})
          </p>
          <p className='mt-1'>
            <span className='text-muted-foreground'>{t('New parent')}: </span>
            {selectedParentLabel}
          </p>
        </div>

        <div className='relative'>
          <Search className='text-muted-foreground absolute top-1/2 left-2.5 h-4 w-4 -translate-y-1/2' />
          <Input
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
            placeholder={t('Search parent user')}
            className='pr-8 pl-8'
            aria-label={t('Search parent user')}
          />
          {keyword && (
            <Button
              type='button'
              variant='ghost'
              size='icon-xs'
              className='absolute top-1/2 right-1 -translate-y-1/2'
              onClick={() => setKeyword('')}
              aria-label={t('Clear search')}
            >
              <X />
            </Button>
          )}
        </div>

        <div className='max-h-64 space-y-1 overflow-y-auto'>
          {candidateContent}
        </div>

        <Button
          type='button'
          variant='outline'
          className='w-full'
          onClick={() => setSelectedParentId(null)}
          disabled={selectedParentId === null}
        >
          {t('Remove parent user')}
        </Button>

        {props.user.role === USER_ROLE.ROOT && (
          <p className='text-destructive text-sm'>
            {t('Root users cannot have a parent')}
          </p>
        )}
      </div>
    </Dialog>
  )
}
