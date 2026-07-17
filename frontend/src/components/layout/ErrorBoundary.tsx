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
      <div className="flex h-full min-h-[420px] items-center justify-center p-6 sm:p-8">
        <div className="w-full max-w-md rounded-2xl border border-slate-200 bg-white p-6 text-center shadow-lg sm:p-8">
          <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-rose-50 text-rose-600 ring-1 ring-rose-100">
            <AlertTriangle size={22} />
          </div>
          <h2 className="text-lg font-bold text-slate-900">页面加载失败</h2>
          <p className="mt-2 text-sm font-medium leading-6 text-slate-500">
            当前页面遇到渲染错误，可以恢复页面或切换到其他功能继续操作。
          </p>
          <button
            type="button"
            onClick={() => this.setState({ error: null })}
            className="mx-auto mt-5 flex h-10 items-center gap-2 rounded-lg bg-slate-900 px-4 text-sm font-semibold text-white transition-colors hover:bg-slate-800"
          >
            <RefreshCw size={14} />
            恢复页面
          </button>
        </div>
      </div>
    );
  }
}
