import { apiClient, handleRequest } from './client';
import type {
  FieldSchema,
  PalworldConfigResponse,
  PalworldSchemaResponse,
  PalworldSettings,
  PalworldValidateResponse,
  ValidationIssue,
} from '../types';

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
    issues: mapIssues(data.issues),
  };
};

export const mapSchema = (raw: unknown): PalworldSchemaResponse => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const fields = Array.isArray(data.fields) ? (data.fields as FieldSchema[]) : [];
  return {
    version: String(data.version || '0.7.2'),
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
      { version: '0.7.2', fields: [] },
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

  updateSettings: (settings: Partial<PalworldSettings>) =>
    handleRequest<unknown, PalworldConfigResponse>(
      () => apiClient.put('/config/palworld', { settings }),
      { ...fallbackConfig, settings: compactSettings(settings), pending_restart: true },
      { map: mapPalworldConfig, quiet: true, fallbackOnError: false },
    ),
};
