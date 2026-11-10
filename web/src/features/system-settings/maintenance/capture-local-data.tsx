/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { StatusBadge } from '@/components/status-badge'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatTimestampToDate } from '@/lib/format'

import {
  deleteCaptureLocalData,
  getCaptureLocalStats,
  getCaptureUploads,
  getCurrentCaptureTask,
  getSystemTask,
  startCapturePackTask,
  startCaptureUploadTask,
} from '../api'
import type {
  CaptureLocalStatsItem,
  CapturePackResult,
  CaptureUploadResult,
  SystemTask,
  SystemTaskResponse,
} from '../types'

const STATS_REFRESH_MS = 10_000
const TASK_REFRESH_MS = 2_000
const UPLOAD_PAGE_SIZE = 10

type CaptureTaskType = 'capture_pack' | 'capture_upload'
type CaptureTask = SystemTask<
  null,
  { processed: number; total: number; progress: number },
  CapturePackResult | CaptureUploadResult
>

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const exponent = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    units.length - 1
  )
  const value = bytes / 1024 ** exponent
  return `${value.toFixed(exponent === 0 ? 0 : 2)} ${units[exponent]}`
}

function isActiveTask(task?: CaptureTask | null) {
  return task?.status === 'pending' || task?.status === 'running'
}

/**
 * Tracks one capture system task: recovers an active task after a reload,
 * polls it while it runs and reports the outcome once it finishes.
 */
function useCaptureTask(
  type: CaptureTaskType,
  start: () => Promise<SystemTaskResponse<CaptureTask>>,
  onFinished: (task: CaptureTask) => void
) {
  const [taskId, setTaskId] = useState<string>()
  const [starting, setStarting] = useState(false)
  const reportedRef = useRef<string | undefined>(undefined)

  const { data: current } = useQuery({
    queryKey: ['capture-task-current', type],
    queryFn: () => getCurrentCaptureTask(type),
    enabled: !taskId,
  })
  useEffect(() => {
    if (!taskId && current?.data?.task_id) setTaskId(current.data.task_id)
  }, [current, taskId])

  const { data: polled } = useQuery({
    queryKey: ['capture-task', taskId],
    queryFn: () => getSystemTask(taskId as string),
    enabled: Boolean(taskId),
    refetchInterval: (query) => {
      const task = query.state.data?.data as CaptureTask | undefined
      return task && !isActiveTask(task) ? false : TASK_REFRESH_MS
    },
  })
  const task = polled?.data as CaptureTask | undefined

  useEffect(() => {
    if (!task || isActiveTask(task) || reportedRef.current === task.task_id) {
      return
    }
    reportedRef.current = task.task_id
    onFinished(task)
  }, [task, onFinished])

  const run = async () => {
    setStarting(true)
    try {
      const res = await start()
      if (!res.success || !res.data) throw new Error(res.message)
      setTaskId(res.data.task_id)
    } catch (error) {
      toast.error(error instanceof Error && error.message ? error.message : '')
    } finally {
      setStarting(false)
    }
  }

  return { task, running: starting || isActiveTask(task), run }
}

function itemStatus(item: CaptureLocalStatsItem) {
  if (item.raw_files > 0 && !item.packaged) return 'unpacked'
  if (item.uploaded && item.raw_files === 0) return 'uploaded'
  if (item.packaged && !item.uploaded) return 'pending'
  return 'partial'
}

export function CaptureLocalData() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [uploadPage, setUploadPage] = useState(1)
  const [deleting, setDeleting] = useState(false)
  const [confirmDeleteOpen, setConfirmDeleteOpen] = useState(false)

  const statsQuery = useQuery({
    queryKey: ['capture-local-stats'],
    queryFn: getCaptureLocalStats,
    refetchInterval: STATS_REFRESH_MS,
  })
  const stats = statsQuery.data?.data

  const uploadsQuery = useQuery({
    queryKey: ['capture-uploads', uploadPage],
    queryFn: () => getCaptureUploads(uploadPage, UPLOAD_PAGE_SIZE),
  })
  const uploads = uploadsQuery.data?.data
  const uploadPageCount = Math.max(
    1,
    Math.ceil((uploads?.total ?? 0) / UPLOAD_PAGE_SIZE)
  )

  const refreshAll = () => {
    void queryClient.invalidateQueries({ queryKey: ['capture-local-stats'] })
    void queryClient.invalidateQueries({ queryKey: ['capture-uploads'] })
  }

  const pack = useCaptureTask(
    'capture_pack',
    startCapturePackTask as () => Promise<SystemTaskResponse<CaptureTask>>,
    (task) => {
      refreshAll()
      if (task.status === 'failed') {
        toast.error(task.error || t('Packing failed'))
        return
      }
      const result = task.result as CapturePackResult | undefined
      toast.success(
        t('Packed {{records}} records into {{packages}} packages', {
          records: result?.records ?? 0,
          packages: result?.packages ?? 0,
        })
      )
    }
  )

  const upload = useCaptureTask(
    'capture_upload',
    startCaptureUploadTask as () => Promise<SystemTaskResponse<CaptureTask>>,
    (task) => {
      refreshAll()
      if (task.status === 'failed') {
        toast.error(task.error || t('Upload failed'))
        return
      }
      const result = task.result as CaptureUploadResult | undefined
      const message = t(
        'Uploaded {{succeeded}} packages ({{size}}), {{failed}} failed',
        {
          succeeded: result?.succeeded ?? 0,
          failed: result?.failed ?? 0,
          size: formatBytes(result?.bytes ?? 0),
        }
      )
      if (result?.failed) toast.warning(message)
      else toast.success(message)
    }
  )

  const handleDelete = async () => {
    setDeleting(true)
    try {
      const res = await deleteCaptureLocalData()
      if (!res.success) throw new Error(res.message)
      setConfirmDeleteOpen(false)
      toast.success(t('Local capture data deleted'))
      refreshAll()
    } catch (error) {
      toast.error(
        error instanceof Error && error.message
          ? error.message
          : t('Failed to delete local capture data')
      )
    } finally {
      setDeleting(false)
    }
  }

  const busy = pack.running || upload.running || deleting
  const activeTask = pack.running ? pack.task : upload.task

  const summary = [
    { label: t('Total local data'), value: stats?.total_bytes ?? 0 },
    {
      label: t('Not yet uploaded to S3'),
      value: stats?.pending_upload_bytes ?? 0,
    },
    { label: t('Uploaded packages'), value: stats?.uploaded_bytes ?? 0 },
    { label: t('Unpacked raw records'), value: stats?.raw_bytes ?? 0 },
  ]

  const statusLabel: Record<ReturnType<typeof itemStatus>, string> = {
    unpacked: t('Not packed'),
    pending: t('Not uploaded'),
    uploaded: t('Uploaded'),
    partial: t('New records pending'),
  }

  return (
    <div className='space-y-4'>
      <div>
        <h4 className='font-medium'>{t('Local capture data')}</h4>
        <p className='text-muted-foreground mt-1 text-xs'>
          {t(
            'Records accumulate per Beijing-time day. Pack finished days into <day>/<model>.zip, upload the packages to S3, then delete the local backlog.'
          )}
        </p>
        {stats?.root ? (
          <p className='text-muted-foreground mt-1 font-mono text-xs'>
            {stats.root}
          </p>
        ) : null}
      </div>

      <div className='grid grid-cols-2 gap-3 md:grid-cols-4'>
        {summary.map((entry) => (
          <div key={entry.label} className='rounded-lg border p-3'>
            <div className='text-muted-foreground text-xs'>{entry.label}</div>
            <div className='mt-1 text-lg font-semibold tabular-nums'>
              {formatBytes(entry.value)}
            </div>
          </div>
        ))}
      </div>
      {stats && stats.legacy_files > 0 ? (
        <p className='text-muted-foreground text-xs'>
          {t(
            'Includes {{size}} in {{files}} files from the old real-time layout, which were already uploaded to S3 and are not packed.',
            {
              size: formatBytes(stats.legacy_bytes),
              files: stats.legacy_files,
            }
          )}
        </p>
      ) : null}

      <div className='flex flex-wrap items-center gap-2'>
        <Button type='button' size='sm' disabled={busy} onClick={pack.run}>
          {pack.running ? t('Packing...') : t('Pack finished days')}
        </Button>
        <Button
          type='button'
          size='sm'
          variant='outline'
          disabled={busy}
          onClick={upload.run}
        >
          {upload.running ? t('Uploading...') : t('Upload to S3')}
        </Button>
        <AlertDialog
          open={confirmDeleteOpen}
          onOpenChange={setConfirmDeleteOpen}
        >
          <AlertDialogTrigger
            render={
              <Button
                type='button'
                size='sm'
                variant='destructive'
                disabled={busy}
              />
            }
          >
            {t('Delete all local data')}
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>{t('Delete all local data?')}</AlertDialogTitle>
              <AlertDialogDescription>
                {t(
                  'This permanently deletes {{size}} of raw records, packages and legacy files on the server, including {{pending}} not yet uploaded to S3. Upload records are kept. This cannot be undone.',
                  {
                    size: formatBytes(stats?.total_bytes ?? 0),
                    pending: formatBytes(stats?.pending_upload_bytes ?? 0),
                  }
                )}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
              <AlertDialogAction
                variant='destructive'
                disabled={deleting}
                onClick={handleDelete}
              >
                {t('Delete')}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>

      {activeTask && isActiveTask(activeTask) ? (
        <div className='max-w-md space-y-1'>
          <Progress value={activeTask.state?.progress ?? 0} />
          <p className='text-muted-foreground text-xs tabular-nums'>
            {activeTask.state?.processed ?? 0} / {activeTask.state?.total ?? 0}
          </p>
        </div>
      ) : null}

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Date')}</TableHead>
            <TableHead>{t('Model')}</TableHead>
            <TableHead className='text-right'>{t('Raw records')}</TableHead>
            <TableHead className='text-right'>{t('Package')}</TableHead>
            <TableHead>{t('Status')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {(stats?.items ?? []).length === 0 ? (
            <TableRow>
              <TableCell
                colSpan={5}
                className='text-muted-foreground text-center'
              >
                {t('No local capture data')}
              </TableCell>
            </TableRow>
          ) : (
            stats?.items.map((item) => {
              const status = itemStatus(item)
              return (
                <TableRow key={`${item.day}/${item.model}`}>
                  <TableCell className='tabular-nums'>{item.day}</TableCell>
                  <TableCell className='font-mono text-xs'>
                    {item.model}
                  </TableCell>
                  <TableCell className='text-right tabular-nums'>
                    {item.raw_files > 0
                      ? `${item.raw_files} · ${formatBytes(item.raw_bytes)}`
                      : '-'}
                  </TableCell>
                  <TableCell className='text-right tabular-nums'>
                    {item.packaged
                      ? `${item.package_files} · ${formatBytes(item.package_bytes)}`
                      : '-'}
                  </TableCell>
                  <TableCell>
                    <StatusBadge
                      label={statusLabel[status]}
                      variant={status === 'uploaded' ? 'success' : 'neutral'}
                      size='sm'
                      copyable={false}
                    />
                  </TableCell>
                </TableRow>
              )
            })
          )}
        </TableBody>
      </Table>

      <div>
        <h4 className='font-medium'>{t('Upload records')}</h4>
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Time')}</TableHead>
            <TableHead>{t('Package')}</TableHead>
            <TableHead className='text-right'>{t('Records')}</TableHead>
            <TableHead className='text-right'>{t('Size')}</TableHead>
            <TableHead>{t('S3 key')}</TableHead>
            <TableHead>{t('Status')}</TableHead>
            <TableHead className='text-right'>{t('Duration')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {(uploads?.items ?? []).length === 0 ? (
            <TableRow>
              <TableCell
                colSpan={7}
                className='text-muted-foreground text-center'
              >
                {t('No uploads yet')}
              </TableCell>
            </TableRow>
          ) : (
            uploads?.items.map((record) => (
              <TableRow key={record.id}>
                <TableCell className='tabular-nums'>
                  {formatTimestampToDate(record.finished_at)}
                </TableCell>
                <TableCell className='font-mono text-xs'>
                  {record.capture_date}/{record.model_name}.zip
                </TableCell>
                <TableCell className='text-right tabular-nums'>
                  {record.file_count}
                </TableCell>
                <TableCell className='text-right tabular-nums'>
                  {formatBytes(record.zip_size)}
                </TableCell>
                <TableCell className='max-w-64 truncate font-mono text-xs'>
                  {record.s3_key}
                </TableCell>
                <TableCell>
                  <StatusBadge
                    label={
                      record.status === 'success' ? t('Success') : t('Failed')
                    }
                    variant={record.status === 'success' ? 'success' : 'danger'}
                    size='sm'
                    copyable={false}
                  />
                  {record.error ? (
                    <p className='text-destructive mt-1 max-w-64 text-xs break-all'>
                      {record.error}
                    </p>
                  ) : null}
                </TableCell>
                <TableCell className='text-right tabular-nums'>
                  {(record.duration_ms / 1000).toFixed(1)}s
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
      {uploadPageCount > 1 ? (
        <div className='flex items-center justify-end gap-2'>
          <Button
            type='button'
            size='sm'
            variant='outline'
            disabled={uploadPage <= 1}
            onClick={() => setUploadPage((page) => page - 1)}
          >
            {t('Previous page')}
          </Button>
          <span className='text-muted-foreground text-xs tabular-nums'>
            {uploadPage} / {uploadPageCount}
          </span>
          <Button
            type='button'
            size='sm'
            variant='outline'
            disabled={uploadPage >= uploadPageCount}
            onClick={() => setUploadPage((page) => page + 1)}
          >
            {t('Next page')}
          </Button>
        </div>
      ) : null}
    </div>
  )
}
