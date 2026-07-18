import React, { useEffect, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Crown, ExternalLink, ListChecks, LoaderCircle, Save, ShieldCheck, UserMinus, UserPlus } from 'lucide-react';
import { getErrorMessage } from '../../api/client';
import { palDefenderGMApi } from '../../api/paldefenderGM';
import type { PalDefenderAccessSettingsUpdate } from '../../types';

type ActionRunner = (key: string, action: () => Promise<unknown>, success: string) => Promise<boolean>;

const emptySettings: PalDefenderAccessSettingsUpdate = {
  use_whitelist: false,
  whitelist_message: '',
  use_admin_whitelist: false,
  admin_auto_login: false,
  admin_ips: [],
};

export const AccessWorkspace: React.FC<{
  identifier: string;
  playerName: string;
  canSecurityWrite: boolean;
  busy: boolean;
  pending: string;
  onRun: ActionRunner;
}> = ({ identifier, playerName, canSecurityWrite, busy, pending, onRun }) => {
  const queryClient = useQueryClient();
  const [settings, setSettings] = useState<PalDefenderAccessSettingsUpdate>(emptySettings);
  const [adminIPs, setAdminIPs] = useState('');

  const accessQuery = useQuery({
    queryKey: ['paldefender-gm', 'access-settings'],
    queryFn: palDefenderGMApi.accessSettings,
    enabled: canSecurityWrite,
  });
  const whitelistQuery = useQuery({
    queryKey: ['paldefender-gm', 'whitelist'],
    queryFn: palDefenderGMApi.whitelist,
    enabled: canSecurityWrite,
  });

  useEffect(() => {
    if (!accessQuery.data) return;
    setSettings({
      use_whitelist: accessQuery.data.use_whitelist,
      whitelist_message: accessQuery.data.whitelist_message,
      use_admin_whitelist: accessQuery.data.use_admin_whitelist,
      admin_auto_login: accessQuery.data.admin_auto_login,
      admin_ips: accessQuery.data.admin_ips,
    });
    setAdminIPs(accessQuery.data.admin_ips.join('\n'));
  }, [accessQuery.data]);

  const entries = whitelistQuery.data?.entries ?? [];
  const whitelisted = entries.some((entry) => entry.toLowerCase() === identifier.toLowerCase());
  const queryError = accessQuery.error || whitelistQuery.error;

  const changeWhitelist = async () => {
    const key = whitelisted ? 'whitelist-remove' : 'whitelist-add';
    await onRun(key, async () => {
      if (whitelisted) await palDefenderGMApi.whitelistRemove(identifier);
      else await palDefenderGMApi.whitelistAdd(identifier);
      await queryClient.invalidateQueries({ queryKey: ['paldefender-gm', 'whitelist'] });
    }, whitelisted ? `${playerName} 已移出白名单` : `${playerName} 已加入白名单`);
  };

  const toggleAdmin = async () => {
    await onRun('toggle-admin', () => palDefenderGMApi.toggleAdmin(identifier), `已切换 ${playerName} 的当前会话管理员状态`);
  };

  const saveSettings = async () => {
    const next = {
      ...settings,
      admin_ips: [...new Set(adminIPs.split(/[\n,，;；]+/).map((value) => value.trim()).filter(Boolean))],
    };
    await onRun('save-access', async () => {
      await palDefenderGMApi.putAccessSettings(next);
      await queryClient.invalidateQueries({ queryKey: ['paldefender-gm', 'access-settings'] });
    }, '访问控制配置已保存，请在安全页面重载 PalDefender 配置');
  };

  if (!canSecurityWrite) {
    return <div className="m-4 rounded-2xl border border-amber-200 bg-amber-50 px-5 py-4 text-xs font-semibold leading-5 text-amber-800">白名单、临时管理员和持久访问配置需要管理员的 <code>security:write</code> 权限。玩家踢出和封禁仍可在下方使用。</div>;
  }

  return (
    <div className="space-y-5 p-4 sm:p-5">
      {queryError && <div role="alert" className="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-xs font-semibold text-rose-700">{getErrorMessage(queryError)}。请检查 RCONEnabled、AdminPassword 与 RCONbase64=false。</div>}
      <section className="rounded-2xl border border-slate-100 bg-white p-4 shadow-sm">
        <div><h3 className="flex items-center gap-2 text-sm font-bold text-slate-800"><ShieldCheck size={16} className="text-emerald-500" />当前玩家访问权限</h3><p className="mt-1 text-[11px] font-semibold text-slate-400">以下操作都绑定当前选中的玩家，不需要再次填写 UserId。</p></div>
        <div className="mt-4 flex flex-wrap gap-2">
          <button type="button" onClick={() => void changeWhitelist()} disabled={busy || !identifier} className={`inline-flex items-center gap-2 rounded-xl px-4 py-2.5 text-xs font-bold disabled:opacity-40 ${whitelisted ? 'border border-rose-200 bg-rose-50 text-rose-700' : 'bg-emerald-600 text-white'}`}>
            {pending.startsWith('whitelist-') ? <LoaderCircle size={14} className="animate-spin" /> : whitelisted ? <UserMinus size={14} /> : <UserPlus size={14} />}{whitelisted ? '移出白名单' : '加入白名单'}
          </button>
          <button type="button" onClick={() => void toggleAdmin()} disabled={busy || !identifier} className="inline-flex items-center gap-2 rounded-xl border border-amber-200 bg-amber-50 px-4 py-2.5 text-xs font-bold text-amber-800 disabled:opacity-40">{pending === 'toggle-admin' ? <LoaderCircle size={14} className="animate-spin" /> : <Crown size={14} />}切换临时管理员</button>
        </div>
        <div className="mt-4 rounded-xl bg-slate-50 px-4 py-3 text-[11px] font-semibold leading-5 text-slate-500"><strong className="text-slate-700">注意：</strong> `/setadmin` 只对当前 Palworld 运行会话有效，服务端重启后不会保留。白名单列表来自 PalDefender RCON，而不是 PalPanel 本地备注列表。</div>
      </section>

      <section className="rounded-2xl border border-slate-100 bg-white p-4 shadow-sm">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between"><div><h3 className="flex items-center gap-2 text-sm font-bold text-slate-800"><ListChecks size={16} className="text-violet-500" />PalDefender 持久访问配置</h3><p className="mt-1 text-[11px] font-semibold text-slate-400">保存后需要在“安全防护”页面执行重载配置。</p></div>{accessQuery.data?.reference_url && <a href={accessQuery.data.reference_url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1.5 text-xs font-bold text-sky-600">配置文档 <ExternalLink size={12} /></a>}</div>
        <div className="mt-4 grid gap-4 lg:grid-cols-2">
          <div className="space-y-3">
            <Toggle label="启用玩家白名单" checked={settings.use_whitelist} onChange={(checked) => setSettings({ ...settings, use_whitelist: checked })} />
            <Toggle label="启用管理员 IP 白名单" checked={settings.use_admin_whitelist} onChange={(checked) => setSettings({ ...settings, use_admin_whitelist: checked })} />
            <Toggle label="管理员 IP 自动登录" checked={settings.admin_auto_login} onChange={(checked) => setSettings({ ...settings, admin_auto_login: checked })} />
          </div>
          <label className="text-xs font-bold text-slate-600">白名单拒绝提示<textarea aria-label="白名单提示" value={settings.whitelist_message} onChange={(event) => setSettings({ ...settings, whitelist_message: event.target.value })} rows={4} maxLength={512} className="mt-1.5 w-full resize-y rounded-xl border border-slate-200 p-3 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none" /></label>
          <label className="text-xs font-bold text-slate-600 lg:col-span-2">管理员 IP（每行一个，可使用 IPv4 通配）<textarea aria-label="管理员 IP" value={adminIPs} onChange={(event) => setAdminIPs(event.target.value)} rows={4} placeholder={'127.0.0.1\n192.168.*.*'} className="mt-1.5 w-full resize-y rounded-xl border border-slate-200 p-3 font-mono text-xs text-slate-700 focus:border-sky-500 focus:outline-none" /></label>
        </div>
        <button type="button" onClick={() => void saveSettings()} disabled={busy || (settings.use_admin_whitelist && !adminIPs.trim())} className="mt-4 inline-flex items-center gap-2 rounded-xl bg-slate-900 px-4 py-2.5 text-xs font-bold text-white disabled:opacity-40">{pending === 'save-access' ? <LoaderCircle size={14} className="animate-spin" /> : <Save size={14} />}保存访问配置</button>
      </section>
    </div>
  );
};

const Toggle: React.FC<{ label: string; checked: boolean; onChange: (checked: boolean) => void }> = ({ label, checked, onChange }) => (
  <label className="flex items-center justify-between gap-4 rounded-xl border border-slate-100 bg-slate-50 px-4 py-3 text-xs font-bold text-slate-600"><span>{label}</span><input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} className="h-4 w-4 accent-slate-900" /></label>
);
