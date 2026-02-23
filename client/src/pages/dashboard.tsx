import { ErrorBoundary } from '@/components/error-boundary';
import { Skeleton } from '@/components/ui/skeleton';
import { useDashboard } from '@/hooks/use-dashboard';
import { useToast } from '@/hooks/use-toast';
import { AlertCircle, type LucideIcon } from 'lucide-react';

const QRCodeIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    className={className}
    {...props}
  >
    <path d="M3 6C3 4.58579 3 3.87868 3.43934 3.43934C3.87868 3 4.58579 3 6 3C7.41421 3 8.12132 3 8.56066 3.43934C9 3.87868 9 4.58579 9 6C9 7.41421 9 8.12132 8.56066 8.56066C8.12132 9 7.41421 9 6 9C4.58579 9 3.87868 9 3.43934 8.56066C3 8.12132 3 7.41421 3 6Z" />
    <path d="M3 18C3 16.5858 3 15.8787 3.43934 15.4393C3.87868 15 4.58579 15 6 15C7.41421 15 8.12132 15 8.56066 15.4393C9 15.8787 9 16.5858 9 18C9 19.4142 9 20.1213 8.56066 20.5607C8.12132 21 7.41421 21 6 21C4.58579 21 3.87868 21 3.43934 20.5607C3 20.1213 3 19.4142 3 18Z" />
    <path d="M3 12L9 12" />
    <path d="M12 3V8" />
    <path d="M15 6C15 4.58579 15 3.87868 15.4393 3.43934C15.8787 3 16.5858 3 18 3C19.4142 3 20.1213 3 20.5607 3.43934C21 3.87868 21 4.58579 21 6C21 7.41421 21 8.12132 20.5607 8.56066C20.1213 9 19.4142 9 18 9C16.5858 9 15.8787 9 15.4393 8.56066C15 8.12132 15 7.41421 15 6Z" />
    <path d="M21 12H15C13.5858 12 12.8787 12 12.4393 12.4393C12 12.8787 12 13.5858 12 15M12 17.7692V20.5385M15 15V16.5C15 17.9464 15.7837 18 17 18C17.5523 18 18 18.4477 18 19M16 21H15M18 15C19.4142 15 20.1213 15 20.5607 15.44C21 15.8799 21 16.5881 21 18.0043C21 19.4206 21 20.1287 20.5607 20.5687C20.24 20.8898 19.7767 20.9766 19 21" />
  </svg>
);

const ImportGroupsIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    {...props}
  >
    <path d="M16.0001 12C16.0001 12 13.0542 16 12.0001 16C10.946 16 8.00012 12 8.00012 12M12.0001 15.5L12.0001 3" />
    <path d="M17.0001 8C19.2093 8 21.0001 9.79086 21.0001 12V14.5C21.0001 16.8346 21.0001 18.0019 20.5278 18.8856C20.1549 19.5833 19.5834 20.1547 18.8857 20.5277C18.0021 21 16.8348 21 14.5001 21H9.50052C7.16551 21 5.99801 21 5.11426 20.5275C4.41677 20.1546 3.84547 19.5834 3.47258 18.8859C3.00012 18.0021 3.00012 16.8346 3.00012 14.4996V11.999C3.00067 9.79114 4.78999 8.00125 6.99785 8H7.00012" />
  </svg>
);

const RedirectSettingsIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinejoin="round"
    className={className}
    {...props}
  >
    <path d="M2.5 12C2.5 7.52166 2.5 5.28249 3.89124 3.89124C5.28249 2.5 7.52166 2.5 12 2.5C16.4783 2.5 18.7175 2.5 20.1088 3.89124C21.5 5.28249 21.5 7.52166 21.5 12C21.5 16.4783 21.5 18.7175 20.1088 20.1088C18.7175 21.5 16.4783 21.5 12 21.5C7.52166 21.5 5.28249 21.5 3.89124 20.1088C2.5 18.7175 2.5 16.4783 2.5 12Z" />
    <path d="M8.5 10C7.67157 10 7 9.32843 7 8.5C7 7.67157 7.67157 7 8.5 7C9.32843 7 10 7.67157 10 8.5C10 9.32843 9.32843 10 8.5 10Z" />
    <path d="M15.5 17C16.3284 17 17 16.3284 17 15.5C17 14.6716 16.3284 14 15.5 14C14.6716 14 14 14.6716 14 15.5C14 16.3284 14.6716 17 15.5 17Z" />
    <path d="M10 8.5L17 8.5" />
    <path d="M14 15.5L7 15.5" />
  </svg>
);

const DashboardIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    {...props}
  >
    <path d="M18 11H18.009" />
    <path d="M20 4H20.009" />
    <path d="M11 6H11.009" />
    <path d="M11 11H11.009" />
    <path d="M9 16H9.00898" />
    <path d="M21 21H10C6.70017 21 5.05025 21 4.02513 19.9749C3 18.9497 3 17.2998 3 14V3" />
  </svg>
);

const GroupsIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    {...props}
  >
    <path d="M15.5 11C15.5 9.067 13.933 7.5 12 7.5C10.067 7.5 8.5 9.067 8.5 11C8.5 12.933 10.067 14.5 12 14.5C13.933 14.5 15.5 12.933 15.5 11Z" />
    <path d="M15.4827 11.3499C15.8047 11.4475 16.1462 11.5 16.5 11.5C18.433 11.5 20 9.933 20 8C20 6.067 18.433 4.5 16.5 4.5C14.6851 4.5 13.1928 5.8814 13.0173 7.65013" />
    <path d="M10.9827 7.65013C10.8072 5.8814 9.31492 4.5 7.5 4.5C5.567 4.5 4 6.067 4 8C4 9.933 5.567 11.5 7.5 11.5C7.85381 11.5 8.19535 11.4475 8.51727 11.3499" />
    <path d="M22 16.5C22 13.7386 19.5376 11.5 16.5 11.5" />
    <path d="M17.5 19.5C17.5 16.7386 15.0376 14.5 12 14.5C8.96243 14.5 6.5 16.7386 6.5 19.5" />
    <path d="M7.5 11.5C4.46243 11.5 2 13.7386 2 16.5" />
  </svg>
);

const FullGroupsIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    className={className}
    {...props}
  >
    <path d="M12.9994 2H9.99936C6.22812 2 4.34251 2 3.17093 3.17157C1.99936 4.34315 1.99936 6.22876 1.99936 10C1.99936 13.7712 1.99936 15.6569 3.17093 16.8284C4.34251 18 6.22812 18 9.99936 18H13.9994C17.7706 18 19.6563 18 20.8278 16.8284C21.481 16.1752 21.77 15.3001 21.8979 14" />
    <path d="M11.9994 18V22" />
    <path d="M7.99936 22H15.9994" />
    <path d="M10.9994 15H12.9994" />
    <path d="M16.7501 5.37603C16.7501 5.31603 16.7487 4.55715 16.7501 4.12257C16.7514 3.72548 16.7165 3.34257 16.9061 2.99217C17.6165 1.57857 19.6565 1.72257 20.1605 3.16257C20.2478 3.39964 20.2531 3.77549 20.2505 4.12257C20.2472 4.56606 20.2565 5.37603 20.2565 5.37603M16.7501 5.37603C15.6701 5.37603 15.2165 6.15603 15.0965 6.63603C14.9765 7.11603 14.9765 8.85603 15.0485 9.57603C15.2885 10.476 15.8885 10.848 16.4765 10.968C17.0165 11.016 19.2965 10.998 19.9565 10.998C20.9165 11.016 21.6365 10.656 21.9365 9.57603C21.9965 9.21603 22.0565 7.23603 21.9065 6.63603C21.5885 5.67603 20.8565 5.37603 20.2565 5.37603M16.7501 5.37603H20.2565" />
  </svg>
);

const ActiveLinksIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    className={className}
    {...props}
  >
    <path d="M10 13.229C10.1416 13.4609 10.3097 13.6804 10.5042 13.8828C11.7117 15.1395 13.5522 15.336 14.9576 14.4722C15.218 14.3121 15.4634 14.1157 15.6872 13.8828L18.9266 10.5114C20.3578 9.02184 20.3578 6.60676 18.9266 5.11718C17.4953 3.6276 15.1748 3.62761 13.7435 5.11718L13.03 5.85978" />
    <path d="M10.9703 18.14L10.2565 18.8828C8.82526 20.3724 6.50471 20.3724 5.07345 18.8828C3.64218 17.3932 3.64218 14.9782 5.07345 13.4886L8.31287 10.1172C9.74413 8.62761 12.0647 8.6276 13.4959 10.1172C13.6904 10.3195 13.8584 10.539 14 10.7708" />
  </svg>
);

const WeeklyGrowthIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    {...props}
  >
    <path d="M7 18V16M12 18V15M17 18V13M2.5 12C2.5 7.52166 2.5 5.28249 3.89124 3.89124C5.28249 2.5 7.52166 2.5 12 2.5C16.4783 2.5 18.7175 2.5 20.1088 3.89124C21.5 5.28249 21.5 7.52166 21.5 12C21.5 16.4783 21.5 18.7175 20.1088 20.1088C18.7175 21.5 16.4783 21.5 12 21.5C7.52166 21.5 5.28249 21.5 3.89124 20.1088C2.5 18.7175 2.5 16.4783 2.5 12Z" />
    <path d="M5.99219 11.4863C8.14729 11.5581 13.0341 11.2328 15.8137 6.82132M13.9923 6.28835L15.8678 5.98649C16.0964 5.95738 16.432 6.13785 16.5145 6.35298L17.0104 7.99142" />
  </svg>
);

const WelcomeIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    {...props}
  >
    <circle cx="12" cy="12" r="10" />
    <path d="M12 8V12.5" />
    <path d="M12 15.9883V15.9983" />
  </svg>
);

const TotalMembersIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    {...props}
  >
    <path d="M12.06 21.5124C11.5445 21.8375 11.2868 22 11 22C10.7132 22 10.4554 21.8374 9.94 21.5124L8.02913 20.3073C7.54415 20.0014 7.30166 19.8485 7.03253 19.8397C6.74172 19.8301 6.49493 19.9768 5.97087 20.3073C5.38395 20.6774 4.21687 21.6971 3.46195 21.2108C3 20.9133 3 20.1575 3 18.6458V8.00002C3 5.17158 3 3.75736 3.82699 2.87868C4.65399 2 5.98501 2 8.64706 2H13.3529C16.015 2 17.346 2 18.173 2.87868C19 3.75736 19 5.17158 19 8.00002V12" />
    <path d="M11 11H7" />
    <path d="M17 14V22M21 18L13 18" />
    <path d="M15 7L7 7" />
  </svg>
);

import { type HistoryPeriod, useDashboardHistory } from '@/hooks/use-dashboard-connect';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';

// Wine/burgundy color palette based on #8D2D53
const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBgHover: 'rgba(141, 45, 83, 0.12)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
  iconBgHover: 'rgba(141, 45, 83, 0.15)',
};

const PERIODS: HistoryPeriod[] = ['weekly', 'monthly', 'yearly'];

/**
 * Dashboard page component showing overview statistics.
 * All logic is delegated to useDashboard hook.
 */
export default function Dashboard() {
  const { t } = useTranslation();
  const { toast } = useToast();
  const { announcement, stats, isLoading, isError } = useDashboard();

  // Chart period state
  const [chartPeriod, setChartPeriod] = useState<HistoryPeriod>('weekly');
  const { data: historyData, isLoading: historyLoading } = useDashboardHistory(chartPeriod);

  // Responsive chart font size
  const [chartFontSize, setChartFontSize] = useState(12);

  const updateFontSize = useCallback(() => {
    const w = window.innerWidth;
    setChartFontSize(w < 480 ? 9 : w < 768 ? 10 : 12);
  }, []);

  useEffect(() => {
    updateFontSize();
    window.addEventListener('resize', updateFontSize);
    return () => window.removeEventListener('resize', updateFontSize);
  }, [updateFontSize]);

  // Transform history data for Recharts
  const chartData = useMemo(() => {
    if (!historyData || historyData.length === 0) {
      return [];
    }
    return historyData.map((dp) => ({
      label: dp.label,
      members: dp.members,
      groups: dp.groups,
    }));
  }, [historyData]);

  // Calculate chart stats
  const chartStats = useMemo(() => {
    if (!historyData || historyData.length === 0) {
      return { growthRate: 0, peakMembers: 0, activeGroups: 0 };
    }

    const peakMembers = Math.max(...historyData.map((dp) => dp.members));
    const activeGroups = historyData.length > 0 ? historyData[historyData.length - 1].groups : 0;

    // Calculate growth rate
    const firstMembers = historyData[0]?.members || 0;
    const lastMembers = historyData[historyData.length - 1]?.members || 0;
    const growthRate =
      firstMembers > 0 ? Math.round(((lastMembers - firstMembers) / firstMembers) * 100) : 0;

    return { growthRate, peakMembers, activeGroups };
  }, [historyData]);

  // Get period-specific subtitle
  const chartSubtitle = useMemo(() => {
    switch (chartPeriod) {
      case 'weekly':
        return t('pages.dashboard.chart.subtitleWeekly');
      case 'monthly':
        return t('pages.dashboard.chart.subtitleMonthly');
      case 'yearly':
        return t('pages.dashboard.chart.subtitleYearly');
      default:
        return t('pages.dashboard.chart.subtitle');
    }
  }, [chartPeriod, t]);

  useEffect(() => {
    if (isError) {
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: t('pages.dashboard.error.loadFailed'),
      });
    }
  }, [isError, toast, t]);

  return (
    <ErrorBoundary>
      <div className="flex flex-col w-full h-full 2xl:max-w-[1600px] 2xl:mx-auto">
        {/* Main Dashboard Card */}
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl lg:rounded-3xl p-4 xs:p-5 sm:p-6 md:p-6 lg:p-8 xl:p-10"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          {/* Header */}
          <div className="flex items-center mb-5 xs:mb-6 sm:mb-8 lg:mb-10">
            <div className="flex items-center gap-2 xs:gap-3">
              <div
                className="p-1.5 xs:p-2 rounded-lg xs:rounded-xl border"
                style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
              >
                <DashboardIcon className="w-5 h-5 xs:w-6 xs:h-6" style={{ color: colors.wine }} />
              </div>
              <h2
                className="text-xl xs:text-2xl md:text-3xl xl:text-4xl font-semibold tracking-tight"
                style={{ color: '#18181b' }}
              >
                {t('pages.dashboard.title')}
              </h2>
            </div>
          </div>

          {/* Announcement */}
          {announcement.enabled && (
            <div
              className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6 lg:p-8 mb-5 xs:mb-6 sm:mb-8 lg:mb-10"
              style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
            >
              <div className="flex items-center gap-2 xs:gap-3 mb-2 xs:mb-3">
                <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
                  <AlertCircle className="w-4 h-4 xs:w-5 xs:h-5" style={{ color: colors.wine }} />
                </div>
                <span
                  className="text-base xs:text-lg font-semibold tracking-tight"
                  style={{ color: '#18181b' }}
                >
                  {announcement.title}
                </span>
              </div>
              <div className="space-y-1.5 xs:space-y-2">
                {announcement.body
                  .split('\n')
                  .map((l) => l.trim())
                  .filter(Boolean)
                  .map((line) => (
                    <p
                      key={line}
                      className="text-xs xs:text-sm leading-relaxed"
                      style={{ color: '#71717a' }}
                    >
                      {line}
                    </p>
                  ))}
              </div>
            </div>
          )}

          {/* Stats Grid */}
          <div className="grid grid-cols-1 xs:grid-cols-2 lg:grid-cols-4 gap-3 xs:gap-4 sm:gap-5 lg:gap-5 xl:gap-6 mb-5 xs:mb-6 sm:mb-8 lg:mb-10">
            {isLoading ? (
              Array.from({ length: 4 }).map((_, index) => (
                <div
                  key={`skeleton-${index}`}
                  className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6"
                  style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
                >
                  <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4">
                    <Skeleton className="h-8 w-8 xs:h-9 xs:w-9 rounded-lg" />
                    <Skeleton className="h-3 w-20 xs:w-24" />
                  </div>
                  <Skeleton className="h-6 xs:h-7 w-14 xs:w-16 mb-2 xs:mb-3" />
                  <Skeleton className="h-3 xs:h-4 w-28 xs:w-32" />
                </div>
              ))
            ) : (
              <>
                {/* Total Groups */}
                <div
                  className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6 transition-all cursor-default group"
                  style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.backgroundColor = colors.wineBgHover;
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.backgroundColor = colors.wineBg;
                  }}
                >
                  <div className="flex items-center gap-2 xs:gap-3 text-sm mb-3 xs:mb-4">
                    <div
                      className="p-1.5 xs:p-2 rounded-lg transition-all"
                      style={{ backgroundColor: colors.iconBg }}
                    >
                      <GroupsIcon
                        className="w-4 h-4 xs:w-5 xs:h-5"
                        style={{ color: colors.wine }}
                      />
                    </div>
                    <span
                      className="uppercase text-[10px] xs:text-xs tracking-[0.1em] xs:tracking-[0.12em] line-clamp-1"
                      style={{ color: colors.wineTextLight }}
                    >
                      {t('pages.dashboard.stats.totalGroups')}
                    </span>
                  </div>
                  <div className="flex items-baseline gap-1.5 xs:gap-2 mb-2 xs:mb-3">
                    <span
                      className="text-xl xs:text-2xl font-semibold tracking-tight"
                      style={{ color: '#18181b' }}
                    >
                      {stats.totalGroups.toLocaleString()}
                    </span>
                    <span
                      className="text-xs xs:text-sm tracking-tight"
                      style={{ color: colors.wine }}
                    >
                      {t('pages.dashboard.stats.groups')}
                    </span>
                  </div>
                  <div
                    className="text-xs xs:text-sm tracking-tight line-clamp-2"
                    style={{ color: colors.wineTextMuted }}
                  >
                    {t('pages.dashboard.stats.totalGroupsDesc')}
                  </div>
                </div>

                {/* Full Groups */}
                <div
                  className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6 transition-all cursor-default"
                  style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.backgroundColor = colors.wineBgHover;
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.backgroundColor = colors.wineBg;
                  }}
                >
                  <div className="flex items-center gap-2 xs:gap-3 text-sm mb-3 xs:mb-4">
                    <div
                      className="p-1.5 xs:p-2 rounded-lg transition-all"
                      style={{ backgroundColor: colors.iconBg }}
                    >
                      <FullGroupsIcon
                        className="w-4 h-4 xs:w-5 xs:h-5"
                        style={{ color: colors.wine }}
                      />
                    </div>
                    <span
                      className="uppercase text-[10px] xs:text-xs tracking-[0.1em] xs:tracking-[0.12em] line-clamp-1"
                      style={{ color: colors.wineTextLight }}
                    >
                      {t('pages.dashboard.stats.fullGroups')}
                    </span>
                  </div>
                  <div className="flex items-center gap-1.5 xs:gap-2 mb-2 xs:mb-3 flex-wrap">
                    <span
                      className="text-xl xs:text-2xl font-semibold tracking-tight"
                      style={{ color: '#18181b' }}
                    >
                      {stats.fullGroups.toLocaleString()}
                    </span>
                    <span
                      className="px-1.5 xs:px-2 py-0.5 xs:py-1 rounded-full text-[10px] xs:text-xs tracking-tight"
                      style={{ backgroundColor: 'rgba(141, 45, 83, 0.15)', color: colors.wine }}
                    >
                      {t('pages.dashboard.stats.full')}
                    </span>
                  </div>
                  <div
                    className="text-xs xs:text-sm tracking-tight line-clamp-2"
                    style={{ color: colors.wineTextMuted }}
                  >
                    {t('pages.dashboard.stats.fullGroupsDesc')}
                  </div>
                </div>

                {/* Total Members */}
                <div
                  className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6 transition-all cursor-default"
                  style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.backgroundColor = colors.wineBgHover;
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.backgroundColor = colors.wineBg;
                  }}
                >
                  <div className="flex items-center gap-2 xs:gap-3 text-sm mb-3 xs:mb-4">
                    <div
                      className="p-1.5 xs:p-2 rounded-lg transition-all"
                      style={{ backgroundColor: colors.iconBg }}
                    >
                      <TotalMembersIcon
                        className="w-4 h-4 xs:w-5 xs:h-5"
                        style={{ color: colors.wine }}
                      />
                    </div>
                    <span
                      className="uppercase text-[10px] xs:text-xs tracking-[0.1em] xs:tracking-[0.12em] line-clamp-1"
                      style={{ color: colors.wineTextLight }}
                    >
                      {t('pages.dashboard.stats.totalMembers')}
                    </span>
                  </div>
                  <div className="flex items-baseline gap-1.5 xs:gap-2 mb-2 xs:mb-3">
                    <span
                      className="text-xl xs:text-2xl font-semibold tracking-tight"
                      style={{ color: '#18181b' }}
                    >
                      {stats.totalMembers.toLocaleString()}
                    </span>
                    <span
                      className="text-xs xs:text-sm tracking-tight"
                      style={{ color: colors.wine }}
                    >
                      {t('pages.dashboard.stats.members')}
                    </span>
                  </div>
                  <div
                    className="text-xs xs:text-sm tracking-tight line-clamp-2"
                    style={{ color: colors.wineTextMuted }}
                  >
                    {t('pages.dashboard.stats.totalMembersDesc')}
                  </div>
                </div>

                {/* Active Links */}
                <div
                  className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6 transition-all cursor-default"
                  style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.backgroundColor = colors.wineBgHover;
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.backgroundColor = colors.wineBg;
                  }}
                >
                  <div className="flex items-center gap-2 xs:gap-3 text-sm mb-3 xs:mb-4">
                    <div
                      className="p-1.5 xs:p-2 rounded-lg transition-all"
                      style={{ backgroundColor: colors.iconBg }}
                    >
                      <ActiveLinksIcon
                        className="w-4 h-4 xs:w-5 xs:h-5"
                        style={{ color: colors.wine }}
                      />
                    </div>
                    <span
                      className="uppercase text-[10px] xs:text-xs tracking-[0.1em] xs:tracking-[0.12em] line-clamp-1"
                      style={{ color: colors.wineTextLight }}
                    >
                      {t('pages.dashboard.stats.activeLinks')}
                    </span>
                  </div>
                  <div className="flex items-center gap-1.5 xs:gap-2 mb-2 xs:mb-3 flex-wrap">
                    <span
                      className="text-xl xs:text-2xl font-semibold tracking-tight"
                      style={{ color: '#18181b' }}
                    >
                      {stats.activeLinks.toLocaleString()}
                    </span>
                    <span className="flex items-center gap-1 xs:gap-1.5">
                      <span
                        className="h-2 w-2 xs:h-2.5 xs:w-2.5 rounded-full"
                        style={{
                          backgroundColor: colors.wine,
                          boxShadow: '0 0 8px rgba(141, 45, 83, 0.6)',
                        }}
                      />
                      <span
                        className="text-[10px] xs:text-xs tracking-tight"
                        style={{ color: colors.wine }}
                      >
                        {t('pages.dashboard.stats.active')}
                      </span>
                    </span>
                  </div>
                  <div
                    className="text-xs xs:text-sm tracking-tight line-clamp-2"
                    style={{ color: colors.wineTextMuted }}
                  >
                    {t('pages.dashboard.stats.activeLinksDesc')}
                  </div>
                </div>
              </>
            )}
          </div>

          {/* Welcome Panel */}
          <div
            className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-3 xs:p-5 sm:p-6 lg:p-8 transition-all"
            style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
          >
            <div className="flex items-start xs:items-center gap-2 xs:gap-3 mb-3 xs:mb-5 sm:mb-6 lg:mb-8">
              <div
                className="p-1.5 xs:p-2 rounded-lg border shrink-0 mt-0.5 xs:mt-0"
                style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
              >
                <WelcomeIcon className="w-4 h-4 xs:w-5 xs:h-5" style={{ color: colors.wine }} />
              </div>
              <div className="min-w-0">
                <div
                  className="text-sm xs:text-base sm:text-lg font-semibold tracking-tight"
                  style={{ color: '#18181b' }}
                >
                  {t('pages.dashboard.welcome.title')}
                </div>
                <div
                  className="text-[11px] xs:text-xs sm:text-sm tracking-tight leading-snug"
                  style={{ color: colors.wineTextMuted }}
                >
                  {t('pages.dashboard.welcome.description')}
                </div>
              </div>
            </div>

            <div
              className="h-px mb-3 xs:mb-5 sm:mb-6 lg:mb-8"
              style={{ backgroundColor: colors.wineBorder }}
            />

            <div className="grid gap-3 xs:gap-4 lg:gap-6 grid-cols-1 md:grid-cols-3">
              <div
                className="p-3 xs:p-4 lg:p-6 rounded-lg xs:rounded-xl lg:rounded-2xl border transition-all hover:opacity-90"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="flex items-start justify-between mb-1.5 xs:mb-2">
                  <h3 className="font-semibold text-sm xs:text-base" style={{ color: colors.wine }}>
                    {t('pages.dashboard.welcome.step1.title')}
                  </h3>
                  <div
                    className="p-1 xs:p-1.5 rounded-md xs:rounded-lg shrink-0 ml-2"
                    style={{ backgroundColor: colors.iconBg }}
                  >
                    <QRCodeIcon className="w-4 h-4 xs:w-5 xs:h-5" style={{ color: colors.wine }} />
                  </div>
                </div>
                <p className="text-xs xs:text-sm" style={{ color: '#71717a' }}>
                  {t('pages.dashboard.welcome.step1.description')}
                </p>
              </div>
              <div
                className="p-3 xs:p-4 lg:p-6 rounded-lg xs:rounded-xl lg:rounded-2xl border transition-all hover:opacity-90"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="flex items-start justify-between mb-1.5 xs:mb-2">
                  <h3 className="font-semibold text-sm xs:text-base" style={{ color: colors.wine }}>
                    {t('pages.dashboard.welcome.step2.title')}
                  </h3>
                  <div
                    className="p-1 xs:p-1.5 rounded-md xs:rounded-lg shrink-0 ml-2"
                    style={{ backgroundColor: colors.iconBg }}
                  >
                    <ImportGroupsIcon
                      className="w-4 h-4 xs:w-5 xs:h-5"
                      style={{ color: colors.wine }}
                    />
                  </div>
                </div>
                <p className="text-xs xs:text-sm" style={{ color: '#71717a' }}>
                  {t('pages.dashboard.welcome.step2.description')}
                </p>
              </div>
              <div
                className="p-3 xs:p-4 lg:p-6 rounded-lg xs:rounded-xl lg:rounded-2xl border transition-all hover:opacity-90"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="flex items-start justify-between mb-1.5 xs:mb-2">
                  <h3 className="font-semibold text-sm xs:text-base" style={{ color: colors.wine }}>
                    {t('pages.dashboard.welcome.step3.title')}
                  </h3>
                  <div
                    className="p-1 xs:p-1.5 rounded-md xs:rounded-lg shrink-0 ml-2"
                    style={{ backgroundColor: colors.iconBg }}
                  >
                    <RedirectSettingsIcon
                      className="w-4 h-4 xs:w-5 xs:h-5"
                      style={{ color: colors.wine }}
                    />
                  </div>
                </div>
                <p className="text-xs xs:text-sm" style={{ color: '#71717a' }}>
                  {t('pages.dashboard.welcome.step3.description')}
                </p>
              </div>
            </div>
          </div>

          {/* Growth Chart */}
          <div
            className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6 lg:p-8 mt-5 xs:mt-6 sm:mt-8 lg:mt-10"
            style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
          >
            {/* Chart Header */}
            <div className="flex flex-col gap-3 xs:gap-4 lg:gap-5 mb-4 xs:mb-5 sm:mb-6 lg:mb-8">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2 xs:gap-3">
                  <div
                    className="p-1.5 xs:p-2 rounded-lg border"
                    style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
                  >
                    <WeeklyGrowthIcon
                      className="w-4 h-4 xs:w-5 xs:h-5"
                      style={{ color: colors.wine }}
                    />
                  </div>
                  <div>
                    <div
                      className="text-base xs:text-lg font-semibold tracking-tight"
                      style={{ color: '#18181b' }}
                    >
                      {t('pages.dashboard.chart.title')}
                    </div>
                    <div
                      className="text-xs xs:text-sm tracking-tight hidden xs:block"
                      style={{ color: colors.wineTextMuted }}
                    >
                      {chartSubtitle}
                    </div>
                  </div>
                </div>
                {/* Legend - Hidden on very small screens, shown on xs+ */}
                <div className="hidden sm:flex flex-col gap-1 xs:gap-1.5">
                  <div className="flex items-center gap-1.5 xs:gap-2">
                    <span
                      className="w-2 h-2 xs:w-2.5 xs:h-2.5 rounded-full"
                      style={{ backgroundColor: '#8D2D53' }}
                    />
                    <span className="text-[10px] xs:text-xs" style={{ color: '#8D2D53' }}>
                      {t('pages.dashboard.chart.members')}
                    </span>
                  </div>
                  <div className="flex items-center gap-1.5 xs:gap-2">
                    <span
                      className="w-2 h-2 xs:w-2.5 xs:h-2.5 rounded-full"
                      style={{ backgroundColor: '#C4A4B0' }}
                    />
                    <span className="text-[10px] xs:text-xs" style={{ color: '#9B7A87' }}>
                      {t('pages.dashboard.chart.groups')}
                    </span>
                  </div>
                </div>
              </div>
              {/* Period Selector - Full width on mobile */}
              <div className="flex flex-col xs:flex-row items-start xs:items-center gap-2 xs:gap-4">
                <div
                  className="flex items-center p-0.5 xs:p-1 rounded-lg xs:rounded-xl w-full xs:w-auto"
                  style={{ backgroundColor: 'rgba(141, 45, 83, 0.06)' }}
                >
                  {PERIODS.map((period) => (
                    <button
                      key={period}
                      type="button"
                      onClick={() => setChartPeriod(period)}
                      className="flex-1 xs:flex-none px-2.5 xs:px-4 lg:px-5 py-1 xs:py-1.5 lg:py-2 rounded-md xs:rounded-lg lg:rounded-xl text-xs xs:text-sm lg:text-base font-medium transition-all"
                      style={{
                        backgroundColor: chartPeriod === period ? colors.wine : 'transparent',
                        color: chartPeriod === period ? '#ffffff' : colors.wineTextMuted,
                      }}
                    >
                      {t(`pages.dashboard.chart.period.${period}`)}
                    </button>
                  ))}
                </div>
                {/* Mobile Legend */}
                <div className="flex sm:hidden items-center gap-3">
                  <div className="flex items-center gap-1">
                    <span className="w-2 h-2 rounded-full" style={{ backgroundColor: '#8D2D53' }} />
                    <span className="text-[10px]" style={{ color: '#8D2D53' }}>
                      {t('pages.dashboard.chart.members')}
                    </span>
                  </div>
                  <div className="flex items-center gap-1">
                    <span className="w-2 h-2 rounded-full" style={{ backgroundColor: '#C4A4B0' }} />
                    <span className="text-[10px]" style={{ color: '#9B7A87' }}>
                      {t('pages.dashboard.chart.groups')}
                    </span>
                  </div>
                </div>
              </div>
            </div>

            {/* Chart */}
            <div className="h-[200px] xs:h-[250px] sm:h-[300px] lg:h-[350px] xl:h-[400px] 2xl:h-[450px] w-full [&_.recharts-wrapper]:outline-none [&_.recharts-surface]:outline-none">
              {historyLoading ? (
                <div className="h-full flex items-center justify-center">
                  <div className="text-sm" style={{ color: colors.wineTextMuted }}>
                    {t('pages.dashboard.chart.loading')}
                  </div>
                </div>
              ) : chartData.length === 0 ? (
                <div className="h-full flex items-center justify-center">
                  <div className="text-sm" style={{ color: colors.wineTextMuted }}>
                    {t('common.noData')}
                  </div>
                </div>
              ) : (
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
                    <defs>
                      <linearGradient id="colorMembers" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="5%" stopColor="#8D2D53" stopOpacity={0.3} />
                        <stop offset="95%" stopColor="#8D2D53" stopOpacity={0} />
                      </linearGradient>
                      <linearGradient id="colorGroups" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="5%" stopColor="#C4A4B0" stopOpacity={0.4} />
                        <stop offset="95%" stopColor="#C4A4B0" stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <CartesianGrid
                      strokeDasharray="3 3"
                      stroke="rgba(141, 45, 83, 0.1)"
                      vertical={false}
                    />
                    <XAxis
                      dataKey="label"
                      axisLine={false}
                      tickLine={false}
                      tick={{ fill: 'rgba(141, 45, 83, 0.6)', fontSize: chartFontSize }}
                      dy={10}
                    />
                    <YAxis
                      axisLine={false}
                      tickLine={false}
                      tick={{ fill: 'rgba(141, 45, 83, 0.6)', fontSize: chartFontSize }}
                      dx={-10}
                    />
                    <Tooltip
                      contentStyle={{
                        backgroundColor: '#ffffff',
                        border: '1px solid rgba(141, 45, 83, 0.2)',
                        borderRadius: '12px',
                        boxShadow: '0 4px 12px rgba(0, 0, 0, 0.1)',
                      }}
                      labelStyle={{ color: '#18181b', fontWeight: 600, marginBottom: 4 }}
                      itemStyle={{ color: '#8D2D53', fontSize: 13 }}
                    />
                    <Area
                      type="monotone"
                      dataKey="members"
                      name={t('pages.dashboard.chart.members')}
                      stroke="#8D2D53"
                      strokeWidth={2.5}
                      fillOpacity={1}
                      fill="url(#colorMembers)"
                      dot={false}
                      activeDot={{
                        r: 6,
                        fill: '#8D2D53',
                        stroke: '#ffffff',
                        strokeWidth: 2,
                      }}
                    />
                    <Area
                      type="monotone"
                      dataKey="groups"
                      name={t('pages.dashboard.chart.groups')}
                      stroke="#C4A4B0"
                      strokeWidth={2}
                      fillOpacity={1}
                      fill="url(#colorGroups)"
                      dot={false}
                      activeDot={{
                        r: 5,
                        fill: '#C4A4B0',
                        stroke: '#ffffff',
                        strokeWidth: 2,
                      }}
                    />
                  </AreaChart>
                </ResponsiveContainer>
              )}
            </div>

            {/* Chart Footer Stats */}
            <div
              className="h-px my-4 xs:my-5 sm:my-6 lg:my-8"
              style={{ backgroundColor: colors.wineBorder }}
            />
            <div className="grid grid-cols-1 xs:grid-cols-3 gap-2 xs:gap-3 sm:gap-4 lg:gap-6">
              <div className="text-center">
                <div
                  className="text-lg xs:text-xl sm:text-2xl lg:text-3xl font-semibold"
                  style={{ color: '#18181b' }}
                >
                  {chartStats.growthRate >= 0 ? '+' : ''}
                  {chartStats.growthRate}%
                </div>
                <div className="text-[10px] xs:text-xs sm:text-sm" style={{ color: colors.wineTextMuted }}>
                  {t('pages.dashboard.chart.growthRate')}
                </div>
              </div>
              <div className="text-center">
                <div
                  className="text-lg xs:text-xl sm:text-2xl lg:text-3xl font-semibold"
                  style={{ color: '#18181b' }}
                >
                  {chartStats.peakMembers.toLocaleString()}
                </div>
                <div className="text-[10px] xs:text-xs sm:text-sm" style={{ color: colors.wineTextMuted }}>
                  {t('pages.dashboard.chart.peakMembers')}
                </div>
              </div>
              <div className="text-center">
                <div
                  className="text-lg xs:text-xl sm:text-2xl lg:text-3xl font-semibold"
                  style={{ color: '#18181b' }}
                >
                  {chartStats.activeGroups.toLocaleString()}
                </div>
                <div className="text-[10px] xs:text-xs sm:text-sm" style={{ color: colors.wineTextMuted }}>
                  {t('pages.dashboard.chart.activeGroups')}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </ErrorBoundary>
  );
}
