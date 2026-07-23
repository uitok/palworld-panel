import { apiClient, handleRequest } from './client';
import type {
  FieldSchema,
  PalworldConfigResponse,
  PalworldSchemaResponse,
  PalworldSettings,
  PalworldValidateResponse,
  ValidationIssue,
  Job,
} from '../types';
import { createFallbackJob, mapJob } from './tasks';

const fallbackConfig: PalworldConfigResponse = {
  settings: {},
  path: '',
  pending_restart: false,
  issues: [],
};
const mapIssues = (raw: unknown): ValidationIssue[] => {
  if (!Array.isArray(raw)) return [];
  return raw.map((item) => {
    const data = (item && typeof item === 'object' ? item : {}) as Record<string, unknown>;
    return {
      field: data.field ? String(data.field) : undefined,
      severity: String(data.severity || 'info'),
      message: String(data.message || ''),
    };
  });
};

const mapFormatIssues = (raw: unknown) => mapIssues(raw).map((issue, index) => ({
  ...issue,
  code: String((Array.isArray(raw) && raw[index] && typeof raw[index] === 'object'
    ? (raw[index] as Record<string, unknown>).code
    : '') || ''),
}));

export const mapPalworldConfig = (raw: unknown): PalworldConfigResponse => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const settings =
    data.settings && typeof data.settings === 'object' && !Array.isArray(data.settings)
      ? (data.settings as PalworldSettings)
      : {};

  return {
    settings,
    path: String(data.path || fallbackConfig.path),
    pending_restart: Boolean(data.pending_restart),
    revision_sha256: data.revision_sha256 ? String(data.revision_sha256) : undefined,
    secret_state: data.secret_state && typeof data.secret_state === 'object'
      ? data.secret_state as PalworldConfigResponse['secret_state']
      : undefined,
    format_issues: mapFormatIssues(data.format_issues),
    draft: data.draft && typeof data.draft === 'object'
      ? data.draft as PalworldConfigResponse['draft']
      : undefined,
    issues: mapIssues(data.issues),
  };
};

const mapEnumLabels = (raw: unknown): Record<string, string> | undefined => {
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return undefined;
  return Object.fromEntries(Object.entries(raw as Record<string, unknown>).map(([key, value]) => [key, String(value)]));
};

const mapFieldSchema = (raw: unknown): FieldSchema => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const field: FieldSchema = {
    key: String(data.key || ''),
    label: data.label ? String(data.label) : undefined,
    group: String(data.group || ''),
    type: String(data.type || 'string') as FieldSchema['type'],
    default: data.default == null ? undefined : String(data.default),
    enum: Array.isArray(data.enum) ? data.enum.map(String) : undefined,
    enum_labels: mapEnumLabels(data.enum_labels),
    min: typeof data.min === 'number' ? data.min : undefined,
    max: typeof data.max === 'number' ? data.max : undefined,
    requires_restart: Boolean(data.requires_restart),
    risk: data.risk ? String(data.risk) : undefined,
    description: String(data.description || ''),
  };
  return field;
};

export const mapSchema = (raw: unknown): PalworldSchemaResponse => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const fields = Array.isArray(data.fields) ? data.fields.map(mapFieldSchema) : [];
  return {
    version: String(data.version || '1.0.0'),
    fields,
  };
};

const compactSettings = (settings: Partial<PalworldSettings>): PalworldSettings => {
  return Object.fromEntries(
    Object.entries(settings).filter((entry): entry is [string, string | number | boolean] => entry[1] !== undefined),
  );
};

export const settingsApi = {
  getSettings: () =>
    handleRequest<unknown, PalworldConfigResponse>(
      () => apiClient.get('/config/palworld'),
      fallbackConfig,
      { map: mapPalworldConfig, quiet: true },
    ),

  getSchema: () =>
    handleRequest<unknown, PalworldSchemaResponse>(
      () => apiClient.get('/config/palworld/schema'),
      { version: '1.0.0', fields: [] },
      { map: mapSchema, quiet: true },
    ),

  validateSettings: (settings: Partial<PalworldSettings>) =>
    handleRequest<unknown, PalworldValidateResponse>(
      () => apiClient.post('/config/palworld/validate', { settings }),
      { valid: true, issues: [] },
      {
        map: (raw) => {
          const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
          return {
            valid: Boolean(data.valid ?? true),
            issues: mapIssues(data.issues),
          };
        },
        quiet: true,
        fallbackOnError: false,
      },
    ),

  updateSettings: (settings: Partial<PalworldSettings>, clearSecrets: string[] = []) =>
    handleRequest<unknown, PalworldConfigResponse>(
      () => apiClient.put('/config/palworld', { settings, ...(clearSecrets.length ? { clear_secrets: clearSecrets } : {}) }),
      { ...fallbackConfig, settings: compactSettings(settings), pending_restart: true },
      { map: mapPalworldConfig, quiet: true, fallbackOnError: false },
    ),

  applySettings: (draftId: string) =>
    handleRequest<unknown, Job>(
      () => apiClient.post('/config/palworld/apply', { draft_id: draftId }),
      createFallbackJob('palworld_config_apply', '已提交配置应用任务'),
      { map: mapJob, quiet: true, fallbackOnError: false },
    ),
};
