import React, { useEffect, useMemo, useState } from 'react';
import { AlertTriangle, CheckCircle2, Info, KeyRound, Languages, Plus, RefreshCw, Save, Shield } from 'lucide-react';
import { getErrorMessage } from '../api/client';
import { aiTranslationApi } from '../api/aiTranslation';
import { serverApi } from '../api/server';
import { settingsApi } from '../api/settings';
import { useServerStore } from '../store/useServerStore';
import { storageKeys } from '../config/defaults';
import type { AITranslationConfig, AITranslationConfigUpdate, FieldSchema, PalworldSettings, ServerVersionInfo, ValidationIssue } from '../types';

const groupLabels: Record<string, string> = {
  server_management: '服务器管理',
  performance: '性能',
  game_balance: '游戏平衡',
  features: '功能开关',
  pvp: 'PvP 对战',
  technology: '科技限制',
};

const coerceInitialValue = (field: FieldSchema, value: unknown) => {
  const raw = value ?? field.default;
  if (raw === undefined) return undefined;
  if (field.type === 'bool') {
    return String(raw).toLowerCase() === 'true';
  }
  if (field.type === 'int' || field.type === 'float') {
    const number = Number(raw);
    return Number.isFinite(number) ? number : undefined;
  }
  return String(raw);
};

export const Settings: React.FC = () => {
  const { panelToken, setPanelToken, triggerRefresh, session } = useServerStore();
  const [fields, setFields] = useState<FieldSchema[]>([]);
  const [draft, setDraft] = useState<PalworldSettings>({});
  const [path, setPath] = useState('');
  const [version, setVersion] = useState('1.0.0');
  const [versionInfo, setVersionInfo] = useState<ServerVersionInfo | null>(null);
  const [originalKeys, setOriginalKeys] = useState<Set<string>>(new Set());
  const [dirtyKeys, setDirtyKeys] = useState<Set<string>>(new Set());
  const [issues, setIssues] = useState<ValidationIssue[]>([]);
  const [pendingRestart, setPendingRestart] = useState(false);
  const [activeGroup, setActiveGroup] = useState('server_management');
  const [tokenInput, setTokenInput] = useState(panelToken);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState<string | null>(null);
  const [aiConfig, setAIConfig] = useState<AITranslationConfig | null>(null);
  const [aiBaseURL, setAIBaseURL] = useState('');
  const [aiModel, setAIModel] = useState('');
  const [aiAPIKey, setAIAPIKey] = useState('');
  const [clearAIAPIKey, setClearAIAPIKey] = useState(false);
  const [aiBusy, setAIBusy] = useState(false);
  const canConfigureAI = Boolean(session?.permissions.includes('ai:config'));

  const load = async () => {
    setLoading(true);
    try {
      const [schema, status, serverVersion] = await Promise.all([
        settingsApi.getSchema(),
        serverApi.getStatus(),
        serverApi.getVersion(),
      ]);
      const config = status.config_exists
        ? await settingsApi.getSettings()
        : { settings: {}, path: status.settings_path || '', pending_restart: status.pending_restart, issues: [] };
      const nextDraft: PalworldSettings = {};
      schema.fields.forEach((field) => {
        nextDraft[field.key] = coerceInitialValue(field, config.settings[field.key]);
      });
      Object.entries(config.settings).forEach(([key, value]) => {
        if (!(key in nextDraft)) nextDraft[key] = value;
      });
      setFields(schema.fields);
      setVersion(schema.version);
      setVersionInfo(serverVersion);
      setDraft(nextDraft);
      setOriginalKeys(new Set(Object.keys(config.settings)));
      setDirtyKeys(new Set());
      setPath(config.path);
      setIssues(config.issues || []);
      setPendingRestart(config.pending_restart);
      setActiveGroup(schema.fields[0]?.group || 'server_management');
    } catch (error) {
      setFields([]);
      setDraft({});
      setIssues([]);
      setMessage(getErrorMessage(error));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  useEffect(() => {
    if (!canConfigureAI) return;
    let active = true;
    void aiTranslationApi.getConfig()
      .then((config) => {
        if (!active) return;
        setAIConfig(config);
        setAIBaseURL(config.base_url);
        setAIModel(config.model);
        setAIAPIKey('');
        setClearAIAPIKey(false);
      })
      .catch((error) => {
        if (active) setMessage(getErrorMessage(error));
      });
    return () => {
      active = false;
    };
  }, [canConfigureAI]);

  const groups = useMemo(() => {
    const ids = Array.from(new Set(fields.map((field) => field.group)));
    return ids.map((id) => ({ id, label: groupLabels[id] || id, fields: fields.filter((field) => field.group === id) }));
  }, [fields]);

  const activeFields = groups.find((group) => group.id === activeGroup)?.fields || [];
  const errorCount = issues.filter((issue) => issue.severity === 'error').length;
  const warningCount = issues.filter((issue) => issue.severity === 'warning').length;

  const updateField = (key: string, value: string | number | boolean) => {
    setDraft((prev) => ({ ...prev, [key]: value }));
    setDirtyKeys((prev) => new Set(prev).add(key));
  };

  const submission = () =>
    Object.fromEntries(
      Object.entries(draft).filter(
        ([key, value]) => value !== undefined && (originalKeys.has(key) || dirtyKeys.has(key)),
      ),
    ) as PalworldSettings;

  const validate = async () => {
    const result = await settingsApi.validateSettings(submission());
    setIssues(result.issues);
    setMessage(result.valid ? '配置校验通过' : '配置存在错误，请修正后再保存');
    return result.valid;
  };

  const save = async () => {
    const valid = await validate();
    if (!valid) return;
    try {
      setPanelToken(tokenInput);
      const saved = await settingsApi.updateSettings(submission());
      setPendingRestart(saved.pending_restart);
      setIssues(saved.issues || []);
      setOriginalKeys(new Set(Object.keys(saved.settings)));
      setDirtyKeys(new Set());
      setMessage('配置已保存，重启服务器后生效');
      triggerRefresh();
    } catch (error) {
      setMessage(getErrorMessage(error));
    }
  };

  const aiUpdate = (): AITranslationConfigUpdate => ({
    base_url: aiBaseURL.trim(),
    model: aiModel.trim(),
    ...(aiAPIKey.trim() ? { api_key: aiAPIKey.trim() } : {}),
    ...(clearAIAPIKey ? { clear_api_key: true } : {}),
  });

  const saveAIConfig = async () => {
    setAIBusy(true);
    try {
      const saved = await aiTranslationApi.updateConfig(aiUpdate());
      setAIConfig(saved);
      setAIBaseURL(saved.base_url);
      setAIModel(saved.model);
      setAIAPIKey('');
      setClearAIAPIKey(false);
      setMessage(saved.configured ? 'AI 翻译配置已保存' : 'AI 翻译配置已保存，但尚未完整配置');
    } catch (error) {
      setMessage(getErrorMessage(error));
    } finally {
      setAIBusy(false);
    }
  };

  const testAIConfig = async () => {
    setAIBusy(true);
    try {
      await aiTranslationApi.testConfig(aiUpdate());
      setMessage('AI 翻译连接测试通过');
    } catch (error) {
      setMessage(getErrorMessage(error));
    } finally {
      setAIBusy(false);
    }
  };

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center p-12 text-xs font-semibold text-slate-400">
        <RefreshCw className="mr-2 animate-spin text-sky-500" size={16} />
        正在读取服务器配置...
      </div>
    );
  }

  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 p-4 sm:p-6 lg:p-8">
      {message && (
        <div className="flex items-center gap-2.5 rounded-2xl border border-sky-100 bg-sky-50 px-5 py-3.5 text-xs font-semibold text-sky-700">
          <Info size={16} />
          {message}
        </div>
      )}

      {pendingRestart && (
        <div className="rounded-2xl border border-amber-100 bg-amber-50 px-5 py-4">
          <div className="flex items-center gap-2 text-xs font-bold text-amber-800">
            <AlertTriangle size={16} />
            配置已写入，等待重启生效
          </div>
          <p className="mt-1 text-[11px] font-medium leading-relaxed text-amber-700">
            PalWorldSettings.ini 修改后需要重启 Palworld 服务端。启动后的状态接口会清除待重启标记。
          </p>
        </div>
      )}

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-[260px_minmax(0,1fr)]">
        <aside className="rounded-3xl border border-slate-100 bg-white p-4 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)]">
          <div className="mb-4 rounded-2xl bg-slate-50 p-4">
            <p className="text-[11px] font-semibold text-slate-400">配置规范</p>
            <p className="mt-1 text-sm font-bold text-slate-800">配置规范 {version}</p>
            <dl className="mt-3 grid gap-1.5 text-[10px] font-semibold text-slate-500">
              <div className="flex justify-between gap-3"><dt>游戏版本</dt><dd>{versionInfo?.game_version || '离线未知'}</dd></div>
              <div className="flex justify-between gap-3"><dt>当前 Build</dt><dd>{versionInfo?.current_build_id || '未知'}</dd></div>
              <div className="flex justify-between gap-3"><dt>最新 Build</dt><dd>{versionInfo?.latest_build_id || '未检查'}</dd></div>
              <div className="flex justify-between gap-3">
                <dt>兼容状态</dt>
                <dd>{versionInfo?.compatible === true ? '兼容' : versionInfo?.compatible === false ? '不匹配' : '待运行确认'}</dd>
              </div>
            </dl>
            <p className="mt-2 break-all text-[10px] font-medium leading-relaxed text-slate-400">
              {path || '配置文件尚未初始化'}
            </p>
          </div>

          {versionInfo?.compatibility_warnings && versionInfo.compatibility_warnings.length > 0 && (
            <div className="mb-4 rounded-2xl border border-amber-100 bg-amber-50 p-3 text-[10px] font-semibold leading-relaxed text-amber-800">
              {versionInfo.compatibility_warnings.join(' / ')}
            </div>
          )}

          <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 xl:flex xl:flex-col">
            {groups.length > 0 ? (
              groups.map((group) => (
                <button
                  type="button"
                  key={group.id}
                  onClick={() => setActiveGroup(group.id)}
                  className={`shrink-0 rounded-xl px-4 py-3 text-left text-xs font-bold transition-all ${
                    activeGroup === group.id
                      ? 'bg-slate-900 text-white'
                      : 'bg-slate-50 text-slate-500 hover:bg-slate-100 hover:text-slate-800'
                  }`}
                >
                  {group.label}
                </button>
              ))
            ) : (
              <p className="rounded-2xl bg-slate-50 p-4 text-xs font-semibold text-slate-400">
                后端未返回配置 schema。请先完成开服初始化，或检查 `/config/palworld/schema`。
              </p>
            )}
          </div>
        </aside>

        <section className="min-w-0 rounded-3xl border border-slate-100 bg-white p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)]">
          <div className="mb-5 flex flex-col gap-3 border-b border-slate-100 pb-5 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <h3 className="text-[16px] font-bold text-slate-800">{groupLabels[activeGroup] || activeGroup}</h3>
              <p className="mt-1 text-xs font-medium text-slate-400">
                字段来自后端 `/config/palworld/schema`，保存前会执行后端校验。
              </p>
            </div>
            <div className="flex flex-wrap gap-2">
              <button
                type="button"
                onClick={validate}
                className="flex items-center gap-2 rounded-xl border border-slate-200 px-4 py-2 text-xs font-semibold text-slate-600 hover:bg-slate-50"
              >
                <CheckCircle2 size={14} />
                校验
              </button>
              <button
                type="button"
                onClick={save}
                className="flex items-center gap-2 rounded-xl bg-sky-500 px-4 py-2 text-xs font-semibold text-white hover:bg-sky-600"
              >
                <Save size={14} />
                保存
              </button>
            </div>
          </div>

          {issues.length > 0 && (
            <div className="mb-5 rounded-2xl border border-slate-100 bg-slate-50 p-4">
              <div className="mb-2 flex items-center gap-2 text-xs font-bold text-slate-700">
                <Shield size={15} className={errorCount > 0 ? 'text-rose-500' : 'text-amber-500'} />
                校验结果：{errorCount} 个错误，{warningCount} 个警告
              </div>
              <div className="grid gap-2">
                {issues.map((issue, index) => (
                  <p
                    key={index}
                    className={`rounded-xl px-3 py-2 text-[11px] font-semibold ${
                      issue.severity === 'error'
                        ? 'bg-rose-50 text-rose-700'
                        : issue.severity === 'warning'
                          ? 'bg-amber-50 text-amber-700'
                          : 'bg-sky-50 text-sky-700'
                    }`}
                  >
                    {issue.field ? `${issue.field}: ` : ''}
                    {issue.message}
                  </p>
                ))}
              </div>
            </div>
          )}

          {activeFields.length > 0 ? (
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
              {activeFields.map((field) => (
                <FieldControl
                  key={field.key}
                  field={field}
                  value={draft[field.key]}
                  isSet={originalKeys.has(field.key) || dirtyKeys.has(field.key)}
                  onChange={(value) => updateField(field.key, value)}
                />
              ))}
            </div>
          ) : (
            <div className="rounded-2xl border border-dashed border-slate-200 bg-slate-50/70 px-4 py-12 text-center text-xs font-semibold text-slate-400">
              暂无可编辑配置字段
            </div>
          )}

          <div className="mt-6 rounded-2xl border border-slate-100 bg-slate-50 p-4">
            <label className="flex flex-col gap-1.5 text-xs font-semibold text-slate-500">
              面板 API 令牌
              <input
                type="password"
                value={tokenInput}
                onChange={(event) => setTokenInput(event.target.value)}
                className="rounded-xl border border-slate-200 bg-white p-3 font-mono text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none"
              />
            </label>
            <p className="mt-2 text-[10px] font-medium text-slate-400">
              保存设置时同步写入本机 localStorage.{storageKeys.token}。
            </p>
          </div>

          <div className="sticky bottom-20 mt-6 flex justify-end border-t border-slate-100 bg-white/95 pt-4 backdrop-blur lg:bottom-0">
            <button
              type="button"
              onClick={save}
              className="flex w-full items-center justify-center gap-2.5 rounded-2xl bg-sky-500 px-6 py-3.5 text-xs font-bold text-white hover:bg-sky-600 sm:w-auto"
            >
              <Save size={15} />
              保存全部参数
            </button>
          </div>
        </section>
      </div>

      {canConfigureAI && (
        <section className="rounded-lg border border-slate-100 bg-white p-5 sm:p-6">
          <div className="flex flex-col gap-3 border-b border-slate-100 pb-4 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex items-center gap-3">
              <div className="rounded-lg bg-emerald-50 p-2 text-emerald-600"><Languages size={18} /></div>
              <div>
                <h3 className="text-[15px] font-bold text-slate-800">AI 翻译</h3>
                <p className="mt-0.5 text-[11px] font-semibold text-slate-400">{aiConfig?.configured ? '已配置' : '未完整配置'}</p>
              </div>
            </div>
            <div className="flex gap-2">
              <button type="button" onClick={() => void testAIConfig()} disabled={aiBusy} className="inline-flex items-center gap-2 rounded-lg border border-slate-200 px-4 py-2 text-xs font-bold text-slate-600 hover:bg-slate-50 disabled:opacity-40">
                <RefreshCw className={aiBusy ? 'animate-spin' : ''} size={14} />连接测试
              </button>
              <button type="button" onClick={() => void saveAIConfig()} disabled={aiBusy} className="inline-flex items-center gap-2 rounded-lg bg-emerald-600 px-4 py-2 text-xs font-bold text-white hover:bg-emerald-700 disabled:opacity-40">
                <Save size={14} />保存 AI 配置
              </button>
            </div>
          </div>

          <div className="mt-5 grid grid-cols-1 gap-4 md:grid-cols-2">
            <label className="flex flex-col gap-2 text-xs font-bold text-slate-600 md:col-span-2">
              OpenAI-compatible Base URL
              <input type="url" value={aiBaseURL} onChange={(event) => setAIBaseURL(event.target.value)} placeholder="https://api.example.com/v1" className="rounded-lg border border-slate-200 px-3 py-2.5 font-mono text-xs font-semibold text-slate-700 focus:border-emerald-500 focus:outline-none" />
            </label>
            <label className="flex flex-col gap-2 text-xs font-bold text-slate-600">
              Model
              <input type="text" value={aiModel} onChange={(event) => setAIModel(event.target.value)} placeholder="model-name" className="rounded-lg border border-slate-200 px-3 py-2.5 font-mono text-xs font-semibold text-slate-700 focus:border-emerald-500 focus:outline-none" />
            </label>
            <label className="flex flex-col gap-2 text-xs font-bold text-slate-600">
              <span className="flex items-center gap-2"><KeyRound size={13} />API Key {aiConfig?.api_key_present ? '（已保存）' : ''}</span>
              <input type="password" value={aiAPIKey} onChange={(event) => { setAIAPIKey(event.target.value); if (event.target.value) setClearAIAPIKey(false); }} placeholder={aiConfig?.api_key_present ? '留空以保留现有密钥' : '输入 API Key'} className="rounded-lg border border-slate-200 px-3 py-2.5 font-mono text-xs font-semibold text-slate-700 focus:border-emerald-500 focus:outline-none" />
            </label>
          </div>
          <label className="mt-4 inline-flex items-center gap-2 text-xs font-semibold text-rose-600">
            <input type="checkbox" checked={clearAIAPIKey} onChange={(event) => { setClearAIAPIKey(event.target.checked); if (event.target.checked) setAIAPIKey(''); }} className="h-4 w-4 rounded border-slate-300 text-rose-500 focus:ring-rose-500" />
            删除已保存的 API Key
          </label>
        </section>
      )}
    </div>
  );
};

const FieldControl: React.FC<{
  field: FieldSchema;
  value: string | number | boolean | undefined;
  isSet: boolean;
  onChange: (value: string | number | boolean) => void;
}> = ({ field, value, isSet, onChange }) => {
  const commonLabel = (
    <div className="flex items-start justify-between gap-3">
      <div className="min-w-0">
        <span className="break-all text-xs font-bold text-slate-700">{field.label || field.key}</span>
        <p className="mt-0.5 break-all font-mono text-[9px] font-semibold leading-relaxed text-slate-400">
          {field.key}
        </p>
        <p className="mt-0.5 text-[10px] font-medium leading-relaxed text-slate-400">{field.description}</p>
      </div>
      {field.requires_restart && (
        <span className="shrink-0 rounded-lg bg-amber-50 px-2 py-0.5 text-[9px] font-bold text-amber-600">需重启</span>
      )}
    </div>
  );

  if (!isSet) {
    const initialValue =
      coerceInitialValue(field, field.default) ??
      (field.type === 'bool' ? false : field.type === 'int' || field.type === 'float' ? 0 : '');
    return (
      <div className="flex min-h-[104px] flex-col justify-between rounded-2xl border border-dashed border-slate-200 bg-slate-50/70 p-4">
        {commonLabel}
        <div className="mt-3 flex items-center justify-between gap-3">
          <span className="text-[10px] font-semibold text-slate-400">
            {field.default === undefined ? '当前未设置' : `当前未设置，服务端默认 ${field.default}`}
          </span>
          <button
            type="button"
            onClick={() => onChange(initialValue)}
            className="flex shrink-0 items-center gap-1 rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-[10px] font-bold text-slate-600 hover:bg-slate-100"
          >
            <Plus size={12} />
            设置
          </button>
        </div>
      </div>
    );
  }

  if (field.type === 'bool') {
    return (
      <label className="flex min-h-[104px] flex-col justify-between rounded-2xl border border-slate-100 bg-slate-50/70 p-4">
        {commonLabel}
        <div className="mt-3 flex items-center gap-2 text-xs font-semibold text-slate-600">
          <input
            type="checkbox"
            checked={Boolean(value)}
            onChange={(event) => onChange(event.target.checked)}
            className="h-4 w-4 rounded border-slate-300 text-sky-500 focus:ring-sky-500"
          />
          {value ? '开启' : '关闭'}
        </div>
      </label>
    );
  }

  if (field.type === 'enum') {
    return (
      <label className="flex min-h-[104px] flex-col gap-3 rounded-2xl border border-slate-100 bg-slate-50/70 p-4">
        {commonLabel}
        <select
          value={String(value ?? '')}
          onChange={(event) => onChange(event.target.value)}
          className="rounded-xl border border-slate-200 bg-white p-3 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none"
        >
          {(field.enum || []).map((item) => (
            <option key={item} value={item}>
              {field.enum_labels?.[item] || item}
            </option>
          ))}
        </select>
      </label>
    );
  }

  const inputType = field.type === 'int' || field.type === 'float' ? 'number' : 'text';
  const step = field.type === 'float' ? '0.1' : '1';
  const isLongText = field.key.includes('Description') || field.type === 'list';

  return (
    <label className={`flex flex-col gap-3 rounded-2xl border border-slate-100 bg-slate-50/70 p-4 ${isLongText ? 'md:col-span-2' : ''}`}>
      {commonLabel}
      {isLongText ? (
        <textarea
          value={String(value ?? '')}
          onChange={(event) => onChange(event.target.value)}
          rows={3}
          className="resize-none rounded-xl border border-slate-200 bg-white p-3 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none"
        />
      ) : (
        <input
          type={inputType}
          step={step}
          min={field.min}
          max={field.max}
          value={String(value ?? '')}
          onChange={(event) => {
            if (field.type === 'int' || field.type === 'float') {
              onChange(Number(event.target.value));
            } else {
              onChange(event.target.value);
            }
          }}
          className="rounded-xl border border-slate-200 bg-white p-3 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none"
        />
      )}
    </label>
  );
};
