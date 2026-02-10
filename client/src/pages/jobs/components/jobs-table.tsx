import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { VirtualTable } from '@/components/ui/virtual-table';
import { Clock, Trash2 } from 'lucide-react';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';

type JobStatus = 'pending' | 'running' | 'done' | 'failed';

interface Job {
  id: string;
  action_type: string;
  scheduled_for: string;
  status: JobStatus;
  error_message?: string | null;
}

interface StatusBadgeConfig {
  label: string;
  variant: 'default' | 'secondary' | 'destructive' | 'outline';
}

interface JobsTableProps {
  jobs: Job[];
  onEditJob: (job: Job) => void;
  onDeleteJob: (jobId: string) => void;
  getStatusBadgeConfig: (status: JobStatus) => StatusBadgeConfig;
  getActionLabel: (action: string) => string;
}

const colors = {
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
};

const VIRTUALIZATION_THRESHOLD = 50;

/**
 * Tabela de jobs para desktop com virtualização condicional.
 */
export function JobsTable({
  jobs,
  onEditJob,
  onDeleteJob,
  getStatusBadgeConfig,
  getActionLabel,
}: JobsTableProps) {
  const { t } = useTranslation();
  const shouldVirtualize = jobs.length > VIRTUALIZATION_THRESHOLD;

  const columns = useMemo(
    () => [
      {
        key: 'action',
        header: t('pages.jobs.table.action'),
        render: (job: Job) => (
          <span className="font-medium">{getActionLabel(job.action_type)}</span>
        ),
      },
      {
        key: 'dateTime',
        header: t('pages.jobs.table.dateTime'),
        render: (job: Job) => (
          <div className="flex items-center gap-2">
            <Clock className="h-4 w-4" style={{ color: colors.wineTextMuted }} />
            {new Date(job.scheduled_for).toLocaleString('pt-BR')}
          </div>
        ),
      },
      {
        key: 'status',
        header: t('pages.jobs.table.status'),
        render: (job: Job) => {
          const config = getStatusBadgeConfig(job.status);
          return (
            <div className="space-y-1">
              <Badge variant={config.variant}>{config.label}</Badge>
              {job.status === 'failed' && job.error_message && (
                <p className="text-xs text-destructive">{job.error_message}</p>
              )}
            </div>
          );
        },
      },
      {
        key: 'actions',
        header: <span className="text-right block">{t('pages.jobs.table.actions')}</span>,
        render: (job: Job) => (
          <div className="flex items-center justify-end gap-1">
            <Button variant="ghost" size="sm" onClick={() => onEditJob(job)}>
              {t('pages.jobs.table.edit')}
            </Button>
            {(job.status === 'pending' || job.status === 'failed') && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => onDeleteJob(job.id)}
                className="text-destructive hover:text-destructive"
              >
                <Trash2 className="h-4 w-4" />
              </Button>
            )}
          </div>
        ),
      },
    ],
    [t, getActionLabel, getStatusBadgeConfig, onEditJob, onDeleteJob]
  );

  if (shouldVirtualize) {
    return (
      <VirtualTable
        data={jobs}
        columns={columns}
        getRowKey={(job) => job.id}
        estimateSize={48}
        maxHeight={500}
      />
    );
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t('pages.jobs.table.action')}</TableHead>
          <TableHead>{t('pages.jobs.table.dateTime')}</TableHead>
          <TableHead>{t('pages.jobs.table.status')}</TableHead>
          <TableHead className="text-right">{t('pages.jobs.table.actions')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {jobs.map((job) => {
          const config = getStatusBadgeConfig(job.status);
          return (
            <TableRow key={job.id}>
              <TableCell className="font-medium">{getActionLabel(job.action_type)}</TableCell>
              <TableCell>
                <div className="flex items-center gap-2">
                  <Clock className="h-4 w-4" style={{ color: colors.wineTextMuted }} />
                  {new Date(job.scheduled_for).toLocaleString('pt-BR')}
                </div>
              </TableCell>
              <TableCell>
                <div className="space-y-1">
                  <Badge variant={config.variant}>{config.label}</Badge>
                  {job.status === 'failed' && job.error_message && (
                    <p className="text-xs text-destructive">{job.error_message}</p>
                  )}
                </div>
              </TableCell>
              <TableCell className="text-right">
                <div className="flex items-center justify-end gap-1">
                  <Button variant="ghost" size="sm" onClick={() => onEditJob(job)}>
                    {t('pages.jobs.table.edit')}
                  </Button>
                  {(job.status === 'pending' || job.status === 'failed') && (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => onDeleteJob(job.id)}
                      className="text-destructive hover:text-destructive"
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  )}
                </div>
              </TableCell>
            </TableRow>
          );
        })}
      </TableBody>
    </Table>
  );
}
