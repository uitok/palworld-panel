import React, { useEffect, useState } from 'react';
import { AlertTriangle, CheckCircle, Info, Megaphone, Save, Send, ServerCrash, X } from 'lucide-react';
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
    await serverApi.announce(announceMsg);
    showToast('公告请求已发送');
    setAnnounceOpen(false);
    setAnnounceMsg('');
  };

  const handleSave = async () => {
    showToast('正在保存世界...', 'info');
    await serverApi.save();
    showToast('世界保存请求已发送');
    setSaveOpen(false);
  };

  const handleRestart = async () => {
    showToast('正在提交重启请求...', 'info');
    await serverApi.shutdown(restartDelay, restartReason);
    await serverApi.restart();
    showToast(`服务器重启流程已提交，倒计时 ${restartDelay} 秒`);
    setRestartOpen(false);
  };

  return (
    <div className="min-h-dvh w-full bg-slate-100 font-sans text-slate-800 lg:p-4">
      <div className="mx-auto flex min-h-dvh w-full overflow-hidden bg-white shadow-[0_24px_70px_rgba(15,23,42,0.06)] lg:h-[calc(100dvh-2rem)] lg:min-h-0 lg:rounded-[28px] lg:border lg:border-slate-100/80">
        <div className="hidden lg:block">
          <Sidebar />
        </div>

        <div className="flex min-w-0 flex-1 flex-col">
          <Header
            onMenuClick={() => setMobileNavOpen(true)}
            onAnnounceClick={() => setAnnounceOpen(true)}
            onSaveClick={() => setSaveOpen(true)}
            onRestartClick={() => setRestartOpen(true)}
          />

          <main className="min-h-0 flex-1 overflow-y-auto bg-white">{children}</main>

          <div className="shrink-0 border-t border-slate-200 bg-white/95 px-4 pb-[calc(0.75rem+env(safe-area-inset-bottom))] pt-3 shadow-[0_-12px_30px_rgba(15,23,42,0.08)] backdrop-blur lg:hidden">
            <div className="mx-auto grid max-w-md grid-cols-3 gap-2">
              <button
                type="button"
                onClick={() => setSaveOpen(true)}
                className="flex flex-col items-center gap-1 rounded-2xl border border-slate-200 py-2 text-[11px] font-bold text-slate-600"
              >
                <Save size={16} />
                保存
              </button>
              <button
                type="button"
                onClick={() => setAnnounceOpen(true)}
                className="flex flex-col items-center gap-1 rounded-2xl bg-sky-500 py-2 text-[11px] font-bold text-white"
              >
                <Megaphone size={16} />
                广播
              </button>
              <button
                type="button"
                onClick={() => setRestartOpen(true)}
                className="flex flex-col items-center gap-1 rounded-2xl border border-rose-200 py-2 text-[11px] font-bold text-rose-600"
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
            className="absolute inset-0 bg-slate-900/40 backdrop-blur-[2px]"
            onClick={() => setMobileNavOpen(false)}
          />
          <div className="absolute bottom-0 left-0 top-0 w-[84vw] max-w-[320px] bg-white shadow-2xl">
            <Sidebar mobile onNavigate={() => setMobileNavOpen(false)} />
          </div>
        </div>
      )}

      {toast && (
        <div className="fixed left-1/2 top-5 z-[70] flex max-w-[calc(100vw-2rem)] -translate-x-1/2 items-center gap-2.5 rounded-2xl border border-slate-100 bg-white px-5 py-3 shadow-[0_12px_30px_rgba(15,23,42,0.08)]">
          {toast.type === 'success' && <CheckCircle className="shrink-0 text-emerald-500" size={18} />}
          {toast.type === 'error' && <AlertTriangle className="shrink-0 text-rose-500" size={18} />}
          {toast.type === 'info' && <Info className="shrink-0 text-sky-500" size={18} />}
          <span className="text-xs font-semibold text-slate-700">{toast.text}</span>
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
  <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40 p-4 backdrop-blur-[2px]">
    <div className="max-h-[90dvh] w-full max-w-md overflow-y-auto rounded-3xl border border-slate-100 bg-white p-6 shadow-2xl">
      <div className="mb-4 flex items-center justify-between">
        <h3 className="text-[16px] font-bold text-slate-800">{title}</h3>
        <button type="button" onClick={onClose} className="text-slate-400 hover:text-slate-600" aria-label="关闭">
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
  <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40 p-4 backdrop-blur-[2px]">
    <div className="w-full max-w-sm rounded-3xl border border-slate-100 bg-white p-6 text-center shadow-2xl">
      <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-sky-50 text-sky-500">
        {icon}
      </div>
      <h3 className="text-[16px] font-bold text-slate-800">{title}</h3>
      <p className="mt-2 text-xs font-medium leading-relaxed text-slate-400">{description}</p>
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
