import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import type { RecurrenceType } from '@/hooks/use-jobs';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { T } from './handoff-tokens';

interface JobScheduleFieldsProps {
  recurrence: RecurrenceType;
  weekdays: number[];
  intervalDays: number;
  times: string[];
  scheduledDate: string;
  repeatUntil: string;
  onRecurrenceChange: (value: RecurrenceType) => void;
  onToggleWeekday: (day: number) => void;
  onIntervalChange: (value: number) => void;
  onAddTime: (time: string) => void;
  onRemoveTime: (time: string) => void;
  onScheduledDateChange: (value: string) => void;
  onRepeatUntilChange: (value: string) => void;
}

const pad = (n: number) => String(n).padStart(2, '0');
const toIso = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
const fromIso = (iso: string) => {
  const [y, m, d] = iso.split('-').map(Number);
  return new Date(y, m - 1, d);
};
const fmtBr = (iso: string) => (iso ? iso.split('-').reverse().join('/') : '');
const MESES = [
  'janeiro',
  'fevereiro',
  'março',
  'abril',
  'maio',
  'junho',
  'julho',
  'agosto',
  'setembro',
  'outubro',
  'novembro',
  'dezembro',
];
const DOW = ['D', 'S', 'T', 'Q', 'Q', 'S', 'S'];

const overline = {
  fontSize: 11,
  fontWeight: 700,
  letterSpacing: '0.06em',
  color: T.ink,
  opacity: 0.45,
  textTransform: 'uppercase' as const,
};

/** Calendário pt-BR do handoff: navegação de mês, grade de ano e atalhos. */
function HandoffCalendar({
  value,
  onPick,
  showTodayShortcuts,
}: {
  value: string;
  onPick: (iso: string) => void;
  showTodayShortcuts: boolean;
}) {
  const { t } = useTranslation();
  const base = value ? fromIso(value) : new Date();
  const [calY, setCalY] = useState(base.getFullYear());
  const [calM, setCalM] = useState(base.getMonth());
  const [showYears, setShowYears] = useState(false);

  const todayIso = toIso(new Date());
  const first = new Date(calY, calM, 1);
  const daysIn = new Date(calY, calM + 1, 0).getDate();

  const prev = () => {
    setCalM((m) => (m === 0 ? 11 : m - 1));
    setCalY((y) => (calM === 0 ? y - 1 : y));
  };
  const next = () => {
    setCalM((m) => (m === 11 ? 0 : m + 1));
    setCalY((y) => (calM === 11 ? y + 1 : y));
  };

  const navBtn = {
    width: 28,
    height: 28,
    borderRadius: 8,
    border: `1px solid ${T.border}`,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    color: T.ink,
  };

  return (
    <div style={{ width: 268 }}>
      <div className="mb-2.5 flex items-center justify-between">
        <button type="button" onClick={prev} className="hf-nav cursor-pointer" style={navBtn}>
          ‹
        </button>
        <button
          type="button"
          onClick={() => setShowYears((v) => !v)}
          className="hf-yeartitle cursor-pointer select-none capitalize"
          style={{
            fontSize: 13.5,
            fontWeight: 700,
            color: T.ink,
            padding: '4px 10px',
            borderRadius: 8,
          }}
        >
          {MESES[calM]} {calY} ▾
        </button>
        <button type="button" onClick={next} className="hf-nav cursor-pointer" style={navBtn}>
          ›
        </button>
      </div>

      {showYears ? (
        <div className="grid grid-cols-3 gap-2">
          {Array.from({ length: 12 }, (_, i) => calY - 4 + i).map((y) => {
            const on = y === calY;
            return (
              <button
                type="button"
                key={y}
                onClick={() => {
                  setCalY(y);
                  setShowYears(false);
                }}
                className="cursor-pointer"
                style={{
                  fontSize: 13.5,
                  fontWeight: on ? 700 : 500,
                  color: on ? T.white : T.ink,
                  background: on ? T.wine : T.white,
                  border: `1.5px solid ${on ? T.wine : T.border}`,
                  borderRadius: 10,
                  padding: '10px 0',
                }}
              >
                {y}
              </button>
            );
          })}
        </div>
      ) : (
        <>
          <div className="mb-1 grid grid-cols-7 gap-0.5">
            {DOW.map((d, i) => (
              <span
                key={`${d}-${i}`}
                className="text-center"
                style={{
                  fontSize: 10.5,
                  fontWeight: 700,
                  color: T.ink,
                  opacity: 0.4,
                  padding: '4px 0',
                }}
              >
                {d}
              </span>
            ))}
          </div>
          <div className="grid grid-cols-7 gap-0.5">
            {Array.from({ length: first.getDay() }, (_, i) => (
              <span key={`pad-${i}`} />
            ))}
            {Array.from({ length: daysIn }, (_, i) => i + 1).map((d) => {
              const dIso = toIso(new Date(calY, calM, d));
              const isSel = dIso === value;
              const isToday = dIso === todayIso;
              return (
                <button
                  type="button"
                  key={d}
                  onClick={() => onPick(dIso)}
                  className="hf-day flex cursor-pointer items-center justify-center"
                  style={{
                    height: 34,
                    borderRadius: 9,
                    fontSize: 13,
                    fontWeight: isSel || isToday ? 700 : 500,
                    color: isSel ? T.white : T.ink,
                    background: isSel ? T.wine : 'transparent',
                    border: `1.5px solid ${isToday && !isSel ? T.borderSel : 'transparent'}`,
                  }}
                >
                  {d}
                </button>
              );
            })}
          </div>
        </>
      )}

      {showTodayShortcuts && (
        <div className="mt-3 flex gap-2">
          <button
            type="button"
            onClick={() => onPick(todayIso)}
            className="hf-btn-wine flex-1 cursor-pointer"
            style={{
              fontSize: 12.5,
              fontWeight: 600,
              color: T.wine,
              background: T.rose,
              border: `1px solid ${T.wineBtnBorder}`,
              borderRadius: 9,
              padding: 8,
            }}
          >
            {t('pages.jobs.schedule.today')}
          </button>
          <button
            type="button"
            onClick={() => {
              const tm = new Date();
              tm.setDate(tm.getDate() + 1);
              onPick(toIso(tm));
            }}
            className="hf-btn-wine flex-1 cursor-pointer"
            style={{
              fontSize: 12.5,
              fontWeight: 600,
              color: T.wine,
              background: T.rose,
              border: `1px solid ${T.wineBtnBorder}`,
              borderRadius: 9,
              padding: 8,
            }}
          >
            {t('pages.jobs.schedule.tomorrow')}
          </button>
        </div>
      )}
    </div>
  );
}

/** Timepicker do handoff: presets rápidos + colunas de hora/minuto. */
function TimePickerContent({ onAdd }: { onAdd: (time: string) => void }) {
  const { t } = useTranslation();
  const [pickH, setPickH] = useState('09');
  const [pickM, setPickM] = useState('00');
  const value = `${pickH}:${pickM}`;
  const col = {
    maxHeight: 130,
    overflowY: 'auto' as const,
    overscrollBehavior: 'contain' as const,
    touchAction: 'pan-y' as const,
    border: `1px solid ${T.divider}`,
    borderRadius: 10,
  };

  return (
    <div style={{ width: 272 }}>
      <div className="mb-3 flex flex-wrap gap-2">
        {['08:00', '12:00', '18:00', '20:00'].map((qt) => (
          <button
            type="button"
            key={qt}
            onClick={() => onAdd(qt)}
            className="cursor-pointer"
            style={{
              fontSize: 12.5,
              fontWeight: 600,
              color: T.wine,
              background: T.rose,
              border: `1px solid ${T.wineBtnBorder}`,
              borderRadius: 8,
              padding: '6px 10px',
            }}
          >
            {qt}
          </button>
        ))}
      </div>
      <div className="grid grid-cols-2 gap-2.5">
        <div>
          <div style={{ ...overline, marginBottom: 5 }}>{t('pages.jobs.schedule.hour')}</div>
          <div style={col}>
            {Array.from({ length: 24 }, (_, h) => pad(h)).map((h) => {
              const on = pickH === h;
              return (
                <button
                  type="button"
                  key={h}
                  onClick={() => setPickH(h)}
                  className="hf-timecell block w-full cursor-pointer text-center"
                  style={{
                    padding: '7px 12px',
                    fontSize: 13.5,
                    fontWeight: 600,
                    color: on ? T.white : T.ink,
                    background: on ? T.wine : T.white,
                  }}
                >
                  {h}
                </button>
              );
            })}
          </div>
        </div>
        <div>
          <div style={{ ...overline, marginBottom: 5 }}>{t('pages.jobs.schedule.minute')}</div>
          <div style={col}>
            {Array.from({ length: 12 }, (_, i) => pad(i * 5)).map((m) => {
              const on = pickM === m;
              return (
                <button
                  type="button"
                  key={m}
                  onClick={() => setPickM(m)}
                  className="hf-timecell block w-full cursor-pointer text-center"
                  style={{
                    padding: '7px 12px',
                    fontSize: 13.5,
                    fontWeight: 600,
                    color: on ? T.white : T.ink,
                    background: on ? T.wine : T.white,
                  }}
                >
                  {m}
                </button>
              );
            })}
          </div>
        </div>
      </div>
      <button
        type="button"
        onClick={() => onAdd(value)}
        className="hf-cta mt-3 w-full cursor-pointer"
        style={{
          fontSize: 13.5,
          fontWeight: 700,
          color: T.white,
          background: T.wine,
          border: 'none',
          borderRadius: 10,
          padding: 10,
        }}
      >
        {t('pages.jobs.schedule.addTimeValue', { time: value })}
      </button>
    </div>
  );
}

/** Card "Quando executar": recorrência, dias, intervalo, horários e datas. */
export function JobScheduleFields({
  recurrence,
  weekdays,
  intervalDays,
  times,
  scheduledDate,
  repeatUntil,
  onRecurrenceChange,
  onToggleWeekday,
  onIntervalChange,
  onAddTime,
  onRemoveTime,
  onScheduledDateChange,
  onRepeatUntilChange,
}: JobScheduleFieldsProps) {
  const { t } = useTranslation();
  const [openPicker, setOpenPicker] = useState<'time' | 'date' | 'until' | null>(null);
  const isRecurring = recurrence !== 'once';

  const recurOpts: { id: RecurrenceType; label: string }[] = [
    { id: 'once', label: t('pages.jobs.schedule.recurrenceOnce') },
    { id: 'daily', label: t('pages.jobs.schedule.recurrenceDaily') },
    { id: 'weekly', label: t('pages.jobs.schedule.recurrenceWeekly') },
    { id: 'interval', label: t('pages.jobs.schedule.recurrenceInterval') },
  ];

  const pill = (on: boolean) => ({
    fontSize: 13,
    fontWeight: 600,
    color: on ? T.white : T.ink,
    background: on ? T.wine : T.white,
    border: `1.5px solid ${on ? T.wine : T.border}`,
    borderRadius: 999,
    padding: '8px 15px',
    transition: 'background .12s',
  });

  const sortedTimes = [...times].sort();

  return (
    <div>
      <div className="mb-2.5" style={{ fontSize: 14, fontWeight: 700, color: T.ink }}>
        {t('pages.jobs.schedule.whenToExecute')} <span style={{ color: T.wine }}>*</span>
      </div>
      <div
        className="flex flex-col gap-4"
        style={{ border: `1px solid ${T.border}`, borderRadius: 16, padding: '16px 18px' }}
      >
        {/* Recorrência */}
        <div>
          <div className="mb-2" style={overline}>
            {t('pages.jobs.schedule.recurrence')}
          </div>
          <div className="flex flex-wrap gap-2">
            {recurOpts.map((r) => (
              <button
                type="button"
                key={r.id}
                onClick={() => onRecurrenceChange(r.id)}
                className="cursor-pointer whitespace-nowrap"
                style={pill(recurrence === r.id)}
              >
                {r.label}
              </button>
            ))}
          </div>
        </div>

        {/* Dias da semana */}
        {recurrence === 'weekly' && (
          <div>
            <div className="mb-2" style={overline}>
              {t('pages.jobs.schedule.weekdaysLabel')}
            </div>
            <div className="flex flex-wrap gap-2">
              {DOW.map((_, i) => {
                const on = weekdays.includes(i);
                return (
                  <button
                    type="button"
                    key={i}
                    onClick={() => onToggleWeekday(i)}
                    className="cursor-pointer"
                    style={{
                      width: 44,
                      height: 44,
                      fontSize: 13,
                      fontWeight: 700,
                      color: on ? T.white : T.ink,
                      background: on ? T.wine : T.white,
                      border: `1.5px solid ${on ? T.wine : T.border}`,
                      borderRadius: 12,
                    }}
                  >
                    {t(`pages.jobs.schedule.weekdays.${i}`)}
                  </button>
                );
              })}
            </div>
          </div>
        )}

        {/* Intervalo */}
        {recurrence === 'interval' && (
          <div className="flex items-center gap-2.5">
            <span style={{ fontSize: 13.5, fontWeight: 600, color: T.ink }}>
              {t('pages.jobs.schedule.repeatEvery')}
            </span>
            <input
              type="number"
              min={2}
              max={30}
              value={intervalDays}
              onChange={(e) => onIntervalChange(Number(e.target.value))}
              className="text-center outline-none"
              style={{
                width: 64,
                fontSize: 14,
                fontWeight: 600,
                color: T.ink,
                padding: '8px 10px',
                borderRadius: 10,
                border: `1.5px solid ${T.border}`,
              }}
            />
            <span style={{ fontSize: 13.5, fontWeight: 600, color: T.ink }}>
              {t('pages.jobs.schedule.days')}
            </span>
          </div>
        )}

        {/* Horários */}
        <div>
          <div className="mb-2" style={overline}>
            {t('pages.jobs.schedule.timesLabel')}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            {sortedTimes.map((tm) => (
              <span
                key={tm}
                className="flex items-center gap-2"
                style={{
                  fontSize: 13.5,
                  fontWeight: 700,
                  color: T.ink,
                  background: T.tagWine,
                  borderRadius: 999,
                  padding: '8px 8px 8px 14px',
                }}
              >
                {tm}
                <button
                  type="button"
                  onClick={() => onRemoveTime(tm)}
                  title={t('pages.jobs.schedule.removeTime')}
                  className="flex cursor-pointer items-center justify-center"
                  style={{
                    width: 19,
                    height: 19,
                    borderRadius: '50%',
                    background: T.white,
                    color: T.wine,
                    fontSize: 11,
                    fontWeight: 700,
                  }}
                >
                  ✕
                </button>
              </span>
            ))}
            {/* modal: o Dialog cancela o touchmove de tudo fora dele (react-remove-scroll),
                o que travava a rolagem das colunas de hora/minuto no celular. */}
            <Popover
              modal
              open={openPicker === 'time'}
              onOpenChange={(o) => setOpenPicker(o ? 'time' : null)}
            >
              <PopoverTrigger asChild>
                <button
                  type="button"
                  className="hf-addtime cursor-pointer whitespace-nowrap"
                  style={{
                    fontSize: 13,
                    fontWeight: 600,
                    color: T.wine,
                    background: T.white,
                    border: `1.5px dashed ${T.borderSel}`,
                    borderRadius: 999,
                    padding: '8px 15px',
                  }}
                >
                  + {t('pages.jobs.schedule.addTime')}
                </button>
              </PopoverTrigger>
              <PopoverContent align="start" className="w-auto rounded-2xl p-3.5">
                <TimePickerContent
                  onAdd={(tm) => {
                    onAddTime(tm);
                    setOpenPicker(null);
                  }}
                />
              </PopoverContent>
            </Popover>
          </div>
        </div>

        {/* Data / Repetir até */}
        <div className="grid grid-cols-2 gap-4">
          <div>
            <div className="mb-2" style={overline}>
              {isRecurring ? t('pages.jobs.schedule.startOptional') : t('pages.jobs.schedule.date')}
            </div>
            <Popover
              open={openPicker === 'date'}
              onOpenChange={(o) => setOpenPicker(o ? 'date' : null)}
            >
              <PopoverTrigger asChild>
                <button
                  type="button"
                  className="flex w-full cursor-pointer items-center justify-between text-left"
                  style={{
                    fontSize: 14,
                    fontWeight: 600,
                    color: scheduledDate ? T.ink : T.placeholder,
                    padding: '11px 14px',
                    borderRadius: 12,
                    border: `1.5px solid ${scheduledDate ? T.borderSel : T.border}`,
                    background: T.white,
                  }}
                >
                  <span>
                    {scheduledDate
                      ? fmtBr(scheduledDate)
                      : t('pages.jobs.schedule.datePlaceholder')}
                  </span>
                  <span style={{ opacity: 0.5 }}>🗓</span>
                </button>
              </PopoverTrigger>
              <PopoverContent align="start" className="w-auto rounded-2xl p-3.5">
                <HandoffCalendar
                  value={scheduledDate}
                  showTodayShortcuts={!isRecurring}
                  onPick={(iso) => {
                    onScheduledDateChange(iso);
                    setOpenPicker(null);
                  }}
                />
              </PopoverContent>
            </Popover>
          </div>

          {isRecurring && (
            <div>
              <div className="mb-2" style={overline}>
                {t('pages.jobs.schedule.repeatUntil')}{' '}
                <span style={{ textTransform: 'none', fontWeight: 600 }}>
                  {t('pages.jobs.schedule.optional')}
                </span>
              </div>
              <div className="flex gap-1.5">
                <Popover
                  open={openPicker === 'until'}
                  onOpenChange={(o) => setOpenPicker(o ? 'until' : null)}
                >
                  <PopoverTrigger asChild>
                    <button
                      type="button"
                      className="flex flex-1 cursor-pointer items-center justify-between text-left"
                      style={{
                        fontSize: 14,
                        fontWeight: 600,
                        color: repeatUntil ? T.ink : T.placeholder,
                        padding: '11px 14px',
                        borderRadius: 12,
                        border: `1.5px solid ${T.border}`,
                        background: T.white,
                      }}
                    >
                      <span>
                        {repeatUntil ? fmtBr(repeatUntil) : t('pages.jobs.schedule.noEndDate')}
                      </span>
                      <span style={{ opacity: 0.5 }}>🗓</span>
                    </button>
                  </PopoverTrigger>
                  <PopoverContent align="end" className="w-auto rounded-2xl p-3.5">
                    <HandoffCalendar
                      value={repeatUntil}
                      showTodayShortcuts={false}
                      onPick={(iso) => {
                        onRepeatUntilChange(iso);
                        setOpenPicker(null);
                      }}
                    />
                  </PopoverContent>
                </Popover>
                {repeatUntil && (
                  <button
                    type="button"
                    onClick={() => onRepeatUntilChange('')}
                    title={t('pages.jobs.schedule.noEndDate')}
                    className="hf-btn-neutral flex-none cursor-pointer"
                    style={{
                      width: 44,
                      borderRadius: 12,
                      border: `1.5px solid ${T.border}`,
                      background: T.white,
                      color: T.ink,
                      opacity: 0.6,
                    }}
                  >
                    ✕
                  </button>
                )}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
