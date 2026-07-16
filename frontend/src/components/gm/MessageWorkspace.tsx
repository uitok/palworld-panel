import React from 'react';
import { BellRing, LoaderCircle, Megaphone, MessageSquareText, Send } from 'lucide-react';
import type { PalDefenderMessageRequest } from '../../types';

export type MessageMode = 'player' | 'broadcast' | 'alert';

const messageTypes: Array<{ value: NonNullable<PalDefenderMessageRequest['SendType']>; label: string }> = [
  { value: 'PlayerChat', label: '玩家聊天' }, { value: 'PlayerGlobalChat', label: '全局聊天' }, { value: 'PlayerGuildChat', label: '公会聊天' },
  { value: 'PlayerLogNormal', label: '普通通知' }, { value: 'PlayerLogImportant', label: '重要通知' }, { value: 'PlayerLogVeryImportant', label: '紧急通知' },
];

export const MessageWorkspace: React.FC<{
  mode: MessageMode; onModeChange: (mode: MessageMode) => void;
  messageType: NonNullable<PalDefenderMessageRequest['SendType']>; onMessageTypeChange: (type: NonNullable<PalDefenderMessageRequest['SendType']>) => void;
  message: string; onMessageChange: (message: string) => void; canWrite: boolean; online: boolean; busy: boolean; onSubmit: () => void;
}> = ({ mode, onModeChange, messageType, onMessageTypeChange, message, onMessageChange, canWrite, online, busy, onSubmit }) => (
  <form className="max-w-3xl p-4 sm:p-5" onSubmit={(event) => { event.preventDefault(); onSubmit(); }}>
    <div className="flex w-fit max-w-full overflow-x-auto rounded-xl border border-slate-200 bg-slate-100 p-0.5" aria-label="消息目标">
      {([['player', '当前玩家', MessageSquareText], ['broadcast', '全服广播', Megaphone], ['alert', '全服警报', BellRing]] as const).map(([id, label, Icon]) => <button type="button" key={id} onClick={() => onModeChange(id)} className={`inline-flex shrink-0 items-center gap-1.5 rounded-lg px-3 py-2 text-xs font-bold ${mode === id ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500'}`}><Icon size={13} />{label}</button>)}
    </div>
    {mode === 'player' && <label className="mt-4 flex max-w-sm flex-col gap-1.5 text-xs font-bold text-slate-600">消息类型<select value={messageType} onChange={(event) => onMessageTypeChange(event.target.value as NonNullable<PalDefenderMessageRequest['SendType']>)} className="rounded-xl border border-slate-200 bg-white px-3 py-2.5 text-xs font-semibold">{messageTypes.map((type) => <option key={type.value} value={type.value}>{type.label}</option>)}</select></label>}
    <label className="mt-4 flex flex-col gap-1.5 text-xs font-bold text-slate-600">消息内容<textarea aria-label="消息内容" value={message} onChange={(event) => onMessageChange(event.target.value)} maxLength={4096} rows={8} className="resize-y rounded-xl border border-slate-200 p-3 text-sm font-medium text-slate-700 focus:border-sky-500 focus:outline-none" /></label>
    <div className="mt-2 text-right text-[10px] font-semibold text-slate-400">{message.length}/4096</div>
    <button type="submit" disabled={!canWrite || busy || !message.trim() || (mode === 'player' && !online)} className="mt-3 inline-flex items-center gap-2 rounded-xl bg-slate-900 px-4 py-2.5 text-xs font-bold text-white disabled:opacity-40">{busy ? <LoaderCircle size={14} className="animate-spin" /> : <Send size={14} />}发送</button>
  </form>
);
