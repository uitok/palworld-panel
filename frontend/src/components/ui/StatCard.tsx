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
  info: 'text-slate-700',
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
    <div className="pp-card relative flex min-h-[132px] flex-col justify-between p-4 sm:p-5">
      <div className="flex items-start justify-between gap-4">
        <span className="text-[12px] font-semibold leading-5 text-slate-500">{title}</span>
        <span className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border ${iconTones[color] || iconTones.sky}`}>
          {icon}
        </span>
      </div>

      <div className="mt-4 min-w-0">
        <strong className="pp-num block truncate text-[26px] font-bold leading-none tracking-[-0.035em] text-slate-900 sm:text-[28px]">
          {value}
        </strong>
        {trend && (
          <span className={`mt-2.5 flex min-w-0 items-center gap-1.5 text-[11px] font-semibold ${trendTones[trendType]}`}>
            <TrendIcon size={13} className="shrink-0" />
            <span className="truncate">{trend}</span>
          </span>
        )}
      </div>
    </div>
  );
};
