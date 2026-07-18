import React from 'react';
import { AlertTriangle, RefreshCw } from 'lucide-react';
import { isPageResourceLoadError } from './pageResourceError';

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

    const resourceLoadFailed = isPageResourceLoadError(this.state.error);

    return (
      <div className="flex h-full min-h-[420px] items-center justify-center p-6 sm:p-8">
        <div className="w-full max-w-md rounded-2xl border border-slate-200 bg-white p-6 text-center shadow-lg sm:p-8">
          <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-rose-50 text-rose-600 ring-1 ring-rose-100">
            <AlertTriangle size={22} />
          </div>
          <h2 className="text-lg font-bold text-slate-900">{resourceLoadFailed ? '页面资源加载中断' : '页面加载失败'}</h2>
          <p className="mt-2 text-sm font-medium leading-6 text-slate-500">
            {resourceLoadFailed
              ? 'PalPanel 后端或访问链路曾短暂不可用，页面脚本没有完整载入。重新载入后会自动恢复，正在运行的任务不会因切换页面而取消。'
              : '当前页面遇到渲染错误，可以恢复页面或切换到其他功能继续操作。'}
          </p>
          <button
            type="button"
            onClick={() => {
              if (resourceLoadFailed) {
                window.location.reload();
                return;
              }
              this.setState({ error: null });
            }}
            className="mx-auto mt-5 flex h-10 items-center gap-2 rounded-lg bg-slate-900 px-4 text-sm font-semibold text-white transition-colors hover:bg-slate-800"
          >
            <RefreshCw size={14} />
            {resourceLoadFailed ? '重新载入页面' : '恢复页面'}
          </button>
        </div>
      </div>
    );
  }
}
