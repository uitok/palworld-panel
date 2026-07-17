import React from 'react';
import { ArrowDownRight, ArrowUpRight, Minus } from 'lucide-react';

interface StatCardProps {
  title: string;
  value: string | number;
  icon: React.ReactNode;
  trend?: string;
  trendType?: 'up' | 'down' | 'neutral' | 'info';
  color?: string;
}

const iconTones: Record<string, string> = {
  emerald: 'border-emerald-100 bg-emerald-50 text-emerald-600',
  amber: 'border-amber-100 bg-amber-50 text-amber-600',
  rose: 'border-rose-100 bg-rose-50 text-rose-600',
  blue: 'border-blue-100 bg-blue-50 text-blue-600',
  sky: 'border-sky-100 bg-sky-50 text-sky-700',
};

const trendTones: Record<NonNullable<StatCardProps['trendType']>, string> = {
  up: 'text-emerald-700',
  down: 'text-rose-700',
  info: 'text-blue-700',
  neutral: 'text-slate-500',
};

export const StatCard: React.FC<StatCardProps> = ({
  title,
  value,
  icon,
  trend,
  trendType = 'neutral',
  color = 'sky',
}) => {
  const TrendIcon = trendType === 'up' ? ArrowUpRight : trendType === 'down' ? ArrowDownRight : Minus;

  return (
    <div className="relative flex min-h-[150px] flex-col justify-between overflow-hidden rounded-2xl border border-slate-200/80 bg-white p-5 shadow-sm">
      <span className="absolute inset-x-5 top-0 h-0.5 rounded-b-full bg-gradient-to-r from-sky-400/70 via-blue-400/30 to-transparent" />
      <div className="flex items-start justify-between gap-4">
        <span className="text-[13px] font-semibold leading-5 text-slate-500">{title}</span>
        <span className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border ${iconTones[color] || iconTones.sky}`}>
          {icon}
        </span>
      </div>

      <div className="mt-5 min-w-0">
        <strong className="block truncate text-[28px] font-bold leading-none tracking-[-0.035em] text-slate-900 sm:text-[30px]">
          {value}
        </strong>
        {trend && (
          <span className={`mt-3 flex min-w-0 items-center gap-1.5 text-xs font-semibold ${trendTones[trendType]}`}>
            <TrendIcon size={13} className="shrink-0" />
            <span className="truncate">{trend}</span>
          </span>
        )}
      </div>
    </div>
  );
};
