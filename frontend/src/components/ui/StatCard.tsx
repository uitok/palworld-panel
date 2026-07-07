import React from 'react';
import { ArrowUpRight, ArrowDownRight, Minus } from 'lucide-react';

interface StatCardProps {
  title: string;
  value: string | number;
  icon: React.ReactNode;
  trend?: string;
  trendType?: 'up' | 'down' | 'neutral' | 'info';
  color?: string; // sky, blue, emerald, amber, rose, slate
}

export const StatCard: React.FC<StatCardProps> = ({
  title,
  value,
  icon,
  trend,
  trendType = 'neutral',
  color = 'sky',
}) => {
  const getTrendColor = () => {
    switch (trendType) {
      case 'up':
        return 'text-emerald-600 bg-emerald-50 border-emerald-100';
      case 'down':
        return 'text-rose-600 bg-rose-50 border-rose-100';
      case 'info':
        return 'text-sky-600 bg-sky-50 border-sky-100';
      default:
        return 'text-slate-500 bg-slate-50 border-slate-100';
    }
  };

  const getTrendIcon = () => {
    switch (trendType) {
      case 'up':
        return <ArrowUpRight size={12} className="shrink-0" />;
      case 'down':
        return <ArrowDownRight size={12} className="shrink-0" />;
      default:
        return <Minus size={12} className="shrink-0" />;
    }
  };

  const getIconContainerColor = () => {
    switch (color) {
      case 'emerald':
        return 'bg-emerald-50 text-emerald-500 border-emerald-100/50';
      case 'amber':
        return 'bg-amber-50 text-amber-500 border-amber-100/50';
      case 'rose':
        return 'bg-rose-50 text-rose-500 border-rose-100/50';
      case 'blue':
        return 'bg-blue-50 text-blue-500 border-blue-100/50';
      default:
        return 'bg-sky-50 text-sky-500 border-sky-100/50';
    }
  };

  return (
    <div className="bg-white border border-slate-100 rounded-2xl p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.03)] hover:shadow-[0_8px_24px_-4px_rgba(15,23,42,0.06)] hover:border-slate-200/60 transition-all duration-300 flex flex-col justify-between gap-3 select-none">
      <div className="flex items-center gap-3">
        <div className={`w-8 h-8 rounded-xl border flex items-center justify-center ${getIconContainerColor()}`}>
          {icon}
        </div>
        <span className="text-[12px] font-semibold text-slate-400 tracking-wide">{title}</span>
      </div>

      <div className="flex flex-col gap-1.5 mt-1">
        <span className="text-2xl font-bold text-slate-800 tracking-tight leading-none">
          {value}
        </span>
        
        {trend && (
          <div className="flex items-center mt-0.5">
            <span className={`flex items-center gap-0.5 px-2 py-0.5 rounded-lg border text-[10px] font-bold ${getTrendColor()}`}>
              {getTrendIcon()}
              {trend}
            </span>
          </div>
        )}
      </div>
    </div>
  );
};
