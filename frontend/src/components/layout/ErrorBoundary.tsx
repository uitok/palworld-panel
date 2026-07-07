import React from 'react';
import { AlertTriangle, RefreshCw } from 'lucide-react';

interface ErrorBoundaryState {
  error: Error | null;
}

export class ErrorBoundary extends React.Component<React.PropsWithChildren, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    if (import.meta.env.DEV) {
      console.error('Page rendering failed', error, errorInfo);
    }
  }

  render() {
    if (!this.state.error) {
      return this.props.children;
    }

    return (
      <div className="flex h-full min-h-[420px] items-center justify-center p-8">
        <div className="w-full max-w-md rounded-2xl border border-rose-100 bg-white p-6 text-center shadow-[0_16px_40px_rgba(15,23,42,0.06)]">
          <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-rose-50 text-rose-500">
            <AlertTriangle size={22} />
          </div>
          <h2 className="text-base font-bold text-slate-800">页面加载失败</h2>
          <p className="mt-2 text-xs font-medium leading-6 text-slate-500">
            当前页面遇到渲染错误，可以恢复页面或切换到其他功能继续操作。
          </p>
          <button
            type="button"
            onClick={() => this.setState({ error: null })}
            className="mx-auto mt-5 flex items-center gap-2 rounded-xl border border-slate-200 px-4 py-2 text-xs font-semibold text-slate-600 transition-all hover:bg-slate-50"
          >
            <RefreshCw size={14} />
            恢复页面
          </button>
        </div>
      </div>
    );
  }
}
