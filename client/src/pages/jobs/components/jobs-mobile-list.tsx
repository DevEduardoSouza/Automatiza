import { Badge } from '@/components/ui/badge';
import { Clock, Trash2 } from 'lucide-react';
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

interface JobsMobileListProps {
  jobs: Job[];
  onEditJob: (job: Job) => void;
  onDeleteJob: (jobId: string) => void;
  getStatusBadgeConfig: (status: JobStatus) => StatusBadgeConfig;
  getActionLabel: (action: string) => string;
}

const colors = {
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
};

/**
 * Lista de cards de jobs para mobile.
 */
export function JobsMobileList({
  jobs,
  onEditJob,
  onDeleteJob,
  getStatusBadgeConfig,
  getActionLabel,
}: JobsMobileListProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-3 xs:space-y-4">
      {jobs.map((job) => {
        const config = getStatusBadgeConfig(job.status);
        return (
          <div
            key={job.id}
            className="rounded-xl xs:rounded-2xl border p-3 xs:p-4 sm:p-5 space-y-3"
            style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
          >
            <div className="flex items-start justify-between">
              <div className="space-y-1 flex-1">
                <p className="font-semibold text-sm xs:text-base">{getActionLabel(job.action_type)}</p>
                <div
                  className="flex items-center gap-1.5 xs:gap-2 text-xs xs:text-sm"
                  style={{ color: colors.wineTextMuted }}
                >
                  <Clock className="h-4 w-4" style={{ color: colors.wineTextMuted }} />
                  {new Date(job.scheduled_for).toLocaleString('pt-BR', {
                    day: '2-digit',
                    month: '2-digit',
                    year: 'numeric',
                    hour: '2-digit',
                    minute: '2-digit',
                  })}
                </div>
              </div>
              <div className="space-y-1">
                <Badge variant={config.variant}>{config.label}</Badge>
                {job.status === 'failed' && job.error_message && (
                  <p className="text-xs text-destructive">{job.error_message}</p>
                )}
              </div>
            </div>
            <div className="flex flex-col xs:flex-row gap-2">
              <button
                type="button"
                onClick={() => onEditJob(job)}
                className="flex-1 inline-flex cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold transition-all border border-gray-300 bg-white text-[#520B27] shadow-sm hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md"
              >
                {t('pages.jobs.table.editSchedule')}
              </button>
              {(job.status === 'pending' || job.status === 'failed') && (
                <button
                  type="button"
                  onClick={() => onDeleteJob(job.id)}
                  className="inline-flex cursor-pointer items-center justify-center rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold transition-all border border-red-300 bg-white text-destructive shadow-sm hover:-translate-y-0.5 hover:border-red-400 hover:shadow-md"
                >
                  <Trash2 className="h-4 w-4" />
                </button>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}
