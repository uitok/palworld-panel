import React, { useEffect, useState } from 'react';
import { AlertTriangle, CheckCircle, Info, Megaphone, Save, Send, ServerCrash, X } from 'lucide-react';
import { getErrorMessage } from '../../api/client';
import { serverApi } from '../../api/server';
import { Header } from './Header';
import { Sidebar } from './Sidebar';

interface AppLayoutProps {
  children: React.ReactNode;
}

type Toast = { text: string; type: 'success' | 'error' | 'info' };

export const AppLayout: React.FC<AppLayoutProps> = ({ children }) => {
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const [announceOpen, setAnnounceOpen] = useState(false);
  const [restartOpen, setRestartOpen] = useState(false);
  const [saveOpen, setSaveOpen] = useState(false);
  const [announceMsg, setAnnounceMsg] = useState('');
  const [restartDelay, setRestartDelay] = useState(10);
  const [restartReason, setRestartReason] = useState('服务器例行维护重启');
  const [toast, setToast] = useState<Toast | null>(null);

  useEffect(() => {
    if (!toast) return;
    const timer = setTimeout(() => setToast(null), 3000);
    return () => clearTimeout(timer);
  }, [toast]);

  useEffect(() => {
    document.body.style.overflow = mobileNavOpen ? 'hidden' : '';
    return () => {
      document.body.style.overflow = '';
    };
  }, [mobileNavOpen]);

  const showToast = (text: string, type: Toast['type'] = 'success') => {
    setToast({ text, type });
  };

  const handleAnnounce = async () => {
    if (!announceMsg.trim()) return;
    try {
      await serverApi.announce(announceMsg);
      showToast('公告请求已发送');
      setAnnounceOpen(false);
      setAnnounceMsg('');
    } catch (error) {
      showToast(getErrorMessage(error), 'error');
    }
  };

  const handleSave = async () => {
    try {
      showToast('正在保存世界...', 'info');
      await serverApi.save();
      showToast('世界保存请求已发送');
      setSaveOpen(false);
    } catch (error) {
      showToast(getErrorMessage(error), 'error');
    }
  };

  const handleRestart = async () => {
    try {
      showToast('正在提交安全重启任务...', 'info');
      await serverApi.safeRestart(restartDelay, restartReason);
      showToast(`安全重启任务已提交，倒计时 ${restartDelay} 秒`);
      setRestartOpen(false);
    } catch (error) {
      showToast(getErrorMessage(error), 'error');
    }
  };

  return (
    <div className="min-h-dvh w-full font-sans text-slate-800">
      <div className="pp-shell relative overflow-hidden lg:h-dvh lg:min-h-0">
        <div className="hidden lg:block">
          <Sidebar />
        </div>

        <div className="pp-shell__content">
          <Header
            onMenuClick={() => setMobileNavOpen(true)}
            onAnnounceClick={() => setAnnounceOpen(true)}
            onSaveClick={() => setSaveOpen(true)}
            onRestartClick={() => setRestartOpen(true)}
          />

          <main id="app-main">{children}</main>

          <div className="pp-mobile-actions shrink-0 px-4 pb-[calc(0.75rem+env(safe-area-inset-bottom))] pt-3 lg:hidden">
            <div className="mx-auto grid max-w-md grid-cols-3 gap-2">
              <button
                type="button"
                onClick={() => setSaveOpen(true)}
                className="flex min-h-12 flex-col items-center justify-center gap-1 rounded-xl border border-slate-200 bg-white py-2 text-[11px] font-bold text-slate-600 transition-colors hover:bg-slate-50"
              >
                <Save size={16} />
                保存
              </button>
              <button
                type="button"
                onClick={() => setAnnounceOpen(true)}
                className="flex min-h-12 flex-col items-center justify-center gap-1 rounded-xl bg-sky-500 py-2 text-[11px] font-bold text-white shadow-sm shadow-sky-500/20 transition-colors hover:bg-sky-600"
              >
                <Megaphone size={16} />
                广播
              </button>
              <button
                type="button"
                onClick={() => setRestartOpen(true)}
                className="flex min-h-12 flex-col items-center justify-center gap-1 rounded-xl border border-rose-200 bg-white py-2 text-[11px] font-bold text-rose-600 transition-colors hover:bg-rose-50"
              >
                <ServerCrash size={16} />
                重启
              </button>
            </div>
          </div>
        </div>
      </div>

      {mobileNavOpen && (
        <div className="fixed inset-0 z-50 lg:hidden">
          <button
            type="button"
            aria-label="关闭导航"
            className="pp-dialog-backdrop absolute inset-0"
            onClick={() => setMobileNavOpen(false)}
          />
          <div className="absolute bottom-0 left-0 top-0 w-[86vw] max-w-[300px] shadow-2xl">
            <Sidebar mobile onNavigate={() => setMobileNavOpen(false)} />
          </div>
        </div>
      )}

      {toast && (
        <div className="animate-slide-in fixed left-1/2 top-5 z-[70] flex max-w-[calc(100vw-2rem)] -translate-x-1/2 items-center gap-2.5 rounded-xl border border-slate-200/80 bg-white/96 px-4 py-3 shadow-[0_18px_50px_-24px_rgba(8,17,31,0.55)] backdrop-blur-xl">
          {toast.type === 'success' && <CheckCircle className="shrink-0 text-emerald-500" size={18} />}
          {toast.type === 'error' && <AlertTriangle className="shrink-0 text-rose-500" size={18} />}
          {toast.type === 'info' && <Info className="shrink-0 text-sky-500" size={18} />}
          <span className="text-sm font-semibold text-slate-700">{toast.text}</span>
        </div>
      )}

      {announceOpen && (
        <Dialog onClose={() => setAnnounceOpen(false)} title="广播服务器公告">
          <p className="mb-4 text-xs font-medium leading-relaxed text-slate-400">
            向所有在线玩家发送服务器公告。后端会代理 Palworld 官方 REST API。
          </p>
          <textarea
            value={announceMsg}
            onChange={(event) => setAnnounceMsg(event.target.value)}
            placeholder="例如：服务器将在 10 分钟后保存并重启维护。"
            rows={4}
            className="w-full resize-none rounded-xl border border-slate-200 p-3.5 text-xs font-medium text-slate-700 placeholder:text-slate-400 focus:border-sky-500 focus:outline-none focus:ring-1 focus:ring-sky-500"
          />
          <div className="mt-4 flex justify-end gap-3">
            <button
              type="button"
              onClick={() => setAnnounceOpen(false)}
              className="rounded-xl border border-slate-200 px-4 py-2 text-xs font-semibold text-slate-500 hover:bg-slate-50"
            >
              取消
            </button>
            <button
              type="button"
              onClick={handleAnnounce}
              className="flex items-center gap-2 rounded-xl bg-sky-500 px-5 py-2 text-xs font-semibold text-white hover:bg-sky-600"
            >
              <Send size={14} />
              发送
            </button>
          </div>
        </Dialog>
      )}

      {saveOpen && (
        <ConfirmDialog
          icon={<Info size={24} />}
          title="保存世界数据？"
          description="这会请求 Palworld 服务端将当前世界状态写入存档。服务器未启动时后端会返回降级状态。"
          confirmText="确认保存"
          confirmClass="bg-sky-500 hover:bg-sky-600"
          onCancel={() => setSaveOpen(false)}
          onConfirm={handleSave}
        />
      )}

      {restartOpen && (
        <Dialog onClose={() => setRestartOpen(false)} title="确认重启服务器">
          <p className="mb-4 text-xs font-medium leading-relaxed text-slate-400">
            在线玩家会断开连接，请确认已经保存世界或提前广播通知。
          </p>
          <div className="flex flex-col gap-4">
            <label className="flex flex-col gap-1.5 text-xs font-semibold text-slate-500">
              倒计时通知（秒）
              <input
                type="number"
                value={restartDelay}
                onChange={(event) => setRestartDelay(Number(event.target.value))}
                min={5}
                max={300}
                className="rounded-xl border border-slate-200 p-3 text-xs font-medium text-slate-700 focus:border-sky-500 focus:outline-none"
              />
            </label>
            <label className="flex flex-col gap-1.5 text-xs font-semibold text-slate-500">
              停机原因
              <input
                type="text"
                value={restartReason}
                onChange={(event) => setRestartReason(event.target.value)}
                className="rounded-xl border border-slate-200 p-3 text-xs font-medium text-slate-700 focus:border-sky-500 focus:outline-none"
              />
            </label>
            <div className="flex justify-end gap-3 pt-2">
              <button
                type="button"
                onClick={() => setRestartOpen(false)}
                className="rounded-xl border border-slate-200 px-4 py-2 text-xs font-semibold text-slate-500 hover:bg-slate-50"
              >
                取消
              </button>
              <button
                type="button"
                onClick={handleRestart}
                className="rounded-xl bg-rose-500 px-5 py-2 text-xs font-semibold text-white hover:bg-rose-600"
              >
                安全重启
              </button>
            </div>
          </div>
        </Dialog>
      )}
    </div>
  );
};

const Dialog: React.FC<React.PropsWithChildren<{ title: string; onClose: () => void }>> = ({
  title,
  onClose,
  children,
}) => (
  <div className="pp-dialog-backdrop fixed inset-0 z-50 flex items-end justify-center p-0 sm:items-center sm:p-4">
    <div
      role="dialog"
      aria-modal="true"
      aria-label={title}
      className="pp-dialog-panel animate-scale-up max-h-[92dvh] w-full max-w-md overflow-y-auto rounded-t-2xl p-5 sm:rounded-2xl sm:p-6"
    >
      <div className="mb-4 flex items-center justify-between">
        <h3 className="text-base font-bold text-slate-900">{title}</h3>
        <button type="button" onClick={onClose} className="rounded-lg p-2 text-slate-400 hover:bg-slate-100 hover:text-slate-700" aria-label="关闭">
          <X size={18} />
        </button>
      </div>
      {children}
    </div>
  </div>
);

const ConfirmDialog: React.FC<{
  icon: React.ReactNode;
  title: string;
  description: string;
  confirmText: string;
  confirmClass: string;
  onCancel: () => void;
  onConfirm: () => void;
}> = ({ icon, title, description, confirmText, confirmClass, onCancel, onConfirm }) => (
  <div className="pp-dialog-backdrop fixed inset-0 z-50 flex items-end justify-center p-0 sm:items-center sm:p-4">
    <div
      role="dialog"
      aria-modal="true"
      aria-label={title}
      className="pp-dialog-panel animate-scale-up w-full max-w-sm rounded-t-2xl p-6 text-center sm:rounded-2xl"
    >
      <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-sky-50 text-sky-500">
        {icon}
      </div>
      <h3 className="text-base font-bold text-slate-900">{title}</h3>
      <p className="mt-2 text-sm font-medium leading-6 text-slate-500">{description}</p>
      <div className="mt-5 flex gap-3">
        <button
          type="button"
          onClick={onCancel}
          className="flex-1 rounded-xl border border-slate-200 py-2.5 text-xs font-semibold text-slate-500 hover:bg-slate-50"
        >
          取消
        </button>
        <button
          type="button"
          onClick={onConfirm}
          className={`flex-1 rounded-xl py-2.5 text-xs font-semibold text-white ${confirmClass}`}
        >
          {confirmText}
        </button>
      </div>
    </div>
  </div>
);
