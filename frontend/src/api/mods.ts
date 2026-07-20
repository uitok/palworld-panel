import { apiClient, handleRequest } from './client';
import type {
  AITranslation,
  ImportCandidate,
  ImportInspection,
  Job,
  LocalModAction,
  LocalModActionResult,
  LocalModFinding,
  LocalScanResult,
  ModItem,
  SteamWorkshopAuthStatus,
  WorkshopItem,
  WorkshopSearchResponse,
  WorkshopStatus,
} from '../types';
import { createFallbackJob, mapJob } from './tasks';
import { AI_OPERATION_TIMEOUT_MS } from './requestTimeouts';

const STEAM_AUTH_OPERATION_TIMEOUT_MS = 180_000;

const stringArray = (raw: unknown): string[] => {
  if (!Array.isArray(raw)) return [];
  return raw.map((item) => String(item || '').trim()).filter(Boolean);
};

const numberValue = (raw: unknown) => {
  const value = Number(raw);
  return Number.isFinite(value) ? value : undefined;
};

const localOwnerships = ['managed', 'manual'] as const;
const localStates = ['present', 'missing_files', 'unknown', 'disabled', 'duplicate', 'incomplete'] as const;
const localSources = ['workshop', 'legacy_pak', 'ue4ss', 'database'] as const;
const localConfidences = ['high', 'medium', 'low'] as const;
const localClassifications = ['managed', 'manual', 'present', 'missing_files', 'unknown', 'disabled', 'duplicate', 'incomplete'] as const;
const localActions = ['import', 'repair', 'ignore', 'unignore', 'delete'] as const;

const enumValue = <T extends string>(raw: unknown, values: readonly T[], fallback: T): T => {
  const value = String(raw || '');
  return values.includes(value as T) ? (value as T) : fallback;
};

const mapMod = (raw: unknown): ModItem => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    id: String(data.id || ''),
    name: String(data.name || data.package_name || 'Unnamed Mod'),
    source: String(data.source || 'upload'),
    package_name: String(data.package_name || ''),
    path: String(data.path || ''),
    version: data.version ? String(data.version) : undefined,
    enabled: Boolean(data.enabled),
    workshop_id: data.workshop_id ? String(data.workshop_id) : undefined,
    preview_url: data.preview_url ? String(data.preview_url) : undefined,
    steam_url: data.steam_url ? String(data.steam_url) : undefined,
    summary: data.summary ? String(data.summary) : undefined,
    tags: stringArray(data.tags),
    file_size: numberValue(data.file_size),
    subscriptions: numberValue(data.subscriptions),
    time_updated: numberValue(data.time_updated),
    last_checked_at: data.last_checked_at ? String(data.last_checked_at) : undefined,
    created_at: data.created_at ? String(data.created_at) : undefined,
    updated_at: data.updated_at ? String(data.updated_at) : undefined,
  };
};
const mapMods = (raw: unknown): ModItem[] => {
  if (!Array.isArray(raw)) return [];
  return raw.map(mapMod);
};

const mapLocalFinding = (raw: unknown): LocalModFinding => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const databaseMods = Array.isArray(data.database_mods)
    ? data.database_mods
        .filter((item): item is Record<string, unknown> => Boolean(item && typeof item === 'object'))
        .map((item) => ({
          id: String(item.id || ''),
          name: String(item.name || item.package_name || 'Unnamed Mod'),
          source: String(item.source || 'unknown'),
          package_name: String(item.package_name || ''),
          path: String(item.path || ''),
          version: item.version ? String(item.version) : undefined,
          enabled: Boolean(item.enabled),
          workshop_id: item.workshop_id ? String(item.workshop_id) : undefined,
          preview_url: item.preview_url ? String(item.preview_url) : undefined,
          steam_url: item.steam_url ? String(item.steam_url) : undefined,
          summary: item.summary ? String(item.summary) : undefined,
          tags: stringArray(item.tags),
          file_size: numberValue(item.file_size),
          subscriptions: numberValue(item.subscriptions),
          time_updated: numberValue(item.time_updated),
          last_checked_at: item.last_checked_at ? String(item.last_checked_at) : undefined,
          created_at: String(item.created_at || ''),
          updated_at: String(item.updated_at || ''),
        }))
    : [];
  const classifications = stringArray(data.classifications)
    .map((item) => enumValue(item, localClassifications, 'unknown'));
  return {
    id: String(data.id || ''),
    revision: String(data.revision || ''),
    ownership: enumValue(data.ownership, localOwnerships, 'manual'),
    state: enumValue(data.state, localStates, 'unknown'),
    source: enumValue(data.source, localSources, 'database'),
    confidence: enumValue(data.confidence, localConfidences, 'low'),
    name: String(data.name || data.package_name || 'Unknown Mod'),
    package_name: data.package_name ? String(data.package_name) : undefined,
    version: data.version ? String(data.version) : undefined,
    enabled: Boolean(data.enabled),
    duplicate: Boolean(data.duplicate),
    paths: stringArray(data.paths),
    database_mods: databaseMods.length > 0 ? databaseMods : undefined,
    classifications,
    issues: stringArray(data.issues),
    ignored: Boolean(data.ignored),
    actions: Array.isArray(data.actions)
      ? data.actions.map((rawAction) => {
          const action = (rawAction && typeof rawAction === 'object' ? rawAction : {}) as Record<string, unknown>;
          return {
            action: enumValue(action.action, localActions, 'ignore'),
            available: Boolean(action.available),
            confirmation_required: Boolean(action.confirmation_required),
            reason: action.reason ? String(action.reason) : undefined,
          };
        })
      : [],
  };
};

export const mapLocalScanResult = (raw: unknown): LocalScanResult => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    server_dir: String(data.server_dir || ''),
    scanned_at: String(data.scanned_at || ''),
    findings: Array.isArray(data.findings) ? data.findings.map(mapLocalFinding) : [],
    skipped_paths: stringArray(data.skipped_paths),
    warnings: stringArray(data.warnings),
  };
};

const mapLocalActionResult = (raw: unknown): LocalModActionResult => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    action: enumValue(data.action, localActions, 'ignore'),
    finding_id: String(data.finding_id || ''),
    message: String(data.message || ''),
    mod: data.mod ? mapMod(data.mod) as LocalModActionResult['mod'] : undefined,
    scan: mapLocalScanResult(data.scan),
  };
};

const mapImportCandidate = (raw: unknown): ImportCandidate => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const sourceType = String(data.source_type || 'local_zip');
  const action = String(data.action || 'unknown');
  return {
    id: String(data.id || ''),
    source_type: (['workshop', 'github_asset', 'https_zip', 'local_zip'].includes(sourceType) ? sourceType : 'local_zip') as ImportCandidate['source_type'],
    file_name: data.file_name ? String(data.file_name) : undefined,
    file_size: numberValue(data.file_size),
    name: data.name ? String(data.name) : undefined,
    package_name: data.package_name ? String(data.package_name) : undefined,
    version: data.version ? String(data.version) : undefined,
    action: (['new', 'update', 'unknown'].includes(action) ? action : 'unknown') as ImportCandidate['action'],
    existing_mod_id: data.existing_mod_id ? String(data.existing_mod_id) : undefined,
    warnings: stringArray(data.warnings),
    ready: Boolean(data.ready),
  };
};

const mapImportInspection = (raw: unknown): ImportInspection => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const sourceType = String(data.source_type || 'local_zip');
  return {
    id: String(data.id || ''),
    source_type: (['workshop', 'github_release', 'https_zip', 'local_zip'].includes(sourceType) ? sourceType : 'local_zip') as ImportInspection['source_type'],
    source: String(data.source || ''),
    candidates: Array.isArray(data.candidates) ? data.candidates.map(mapImportCandidate) : [],
    selected_candidate_id: data.selected_candidate_id ? String(data.selected_candidate_id) : undefined,
    expires_at: String(data.expires_at || ''),
  };
};

export const mapWorkshopItem = (raw: unknown): WorkshopItem => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const id = String(data.id || data.publishedfileid || '');
  return {
    id,
    title: String(data.title || data.name || id || 'Untitled Workshop Item'),
    summary: data.summary ? String(data.summary) : undefined,
    preview_url: data.preview_url ? String(data.preview_url) : undefined,
    steam_url: String(data.steam_url || `https://steamcommunity.com/sharedfiles/filedetails/?id=${id}`),
    tags: stringArray(data.tags),
    file_size: numberValue(data.file_size),
    subscriptions: numberValue(data.subscriptions),
    time_created: numberValue(data.time_created),
    time_updated: numberValue(data.time_updated),
    installed: Boolean(data.installed),
    enabled: Boolean(data.enabled),
    update_available: Boolean(data.update_available),
    mod_id: data.mod_id ? String(data.mod_id) : undefined,
    translation: data.translation ? mapTranslation(data.translation) : undefined,
  };
};

const mapTranslation = (raw: unknown): AITranslation => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    text: String(data.text || ''),
    target_language: String(data.target_language || 'zh-CN'),
    model: String(data.model || ''),
    generated_at: String(data.generated_at || ''),
    cached: Boolean(data.cached),
  };
};

const mapWorkshopSearchResponse = (raw: unknown): WorkshopSearchResponse => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const items = Array.isArray(data.items) ? data.items.map(mapWorkshopItem) : [];
  return {
    items,
    next_cursor: data.next_cursor ? String(data.next_cursor) : undefined,
    total: Number(data.total) || items.length,
    page_size: Number(data.page_size) || items.length,
  };
};

export const mapWorkshopStatus = (raw: unknown): WorkshopStatus => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const keySource = data.key_source === 'environment' || data.key_source === 'bundled' ? data.key_source : '';
  return {
    configured: Boolean(data.configured) && keySource !== '',
    key_source: keySource,
    app_id: String(data.app_id || '1623730'),
  };
};

export const mapSteamWorkshopAuthStatus = (raw: unknown): SteamWorkshopAuthStatus => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    supported: Boolean(data.supported),
    steamcmd_installed: Boolean(data.steamcmd_installed),
    credentials_secure: Boolean(data.credentials_secure),
    login_in_progress: Boolean(data.login_in_progress),
    logged_in: Boolean(data.logged_in),
    verification_required: Boolean(data.verification_required),
    account_name: data.account_name ? String(data.account_name) : undefined,
    last_verified_at: data.last_verified_at ? String(data.last_verified_at) : undefined,
    password_configured: Boolean(data.password_configured),
    steam_guard_required: Boolean(data.steam_guard_required),
    message: data.message ? String(data.message) : undefined,
  };
};

const emptySteamWorkshopAuthStatus: SteamWorkshopAuthStatus = {
  supported: false,
  steamcmd_installed: false,
  credentials_secure: false,
  login_in_progress: false,
  logged_in: false,
  verification_required: false,
  password_configured: false,
  steam_guard_required: false,
};

export const modsApi = {
  list: () =>
    handleRequest<unknown, ModItem[]>(() => apiClient.get('/mods'), [], {
      map: mapMods,
      quiet: true,
    }),

  scanLocal: () =>
    handleRequest<unknown, LocalScanResult>(
      () => apiClient.post('/mods/local/scan', undefined, { timeout: 60_000 }),
      { server_dir: '', scanned_at: '', findings: [], skipped_paths: [], warnings: [] },
      { map: mapLocalScanResult, quiet: true, fallbackOnError: false },
    ),

  actOnLocalFinding: (finding: Pick<LocalModFinding, 'id' | 'revision'>, action: LocalModAction, confirm = false) =>
    handleRequest<unknown, LocalModActionResult>(
      () => apiClient.post(`/mods/local/findings/${encodeURIComponent(finding.id)}/actions`, {
        action,
        revision: finding.revision,
        confirm: confirm || undefined,
      }),
      { action, finding_id: finding.id, message: '', scan: { server_dir: '', scanned_at: '', findings: [], skipped_paths: [], warnings: [] } },
      { map: mapLocalActionResult, quiet: true, fallbackOnError: false },
    ),

  upload: (file: File, enable: boolean) => {
    const form = new FormData();
    form.append('file', file);
    form.append('enable', String(enable));

    return handleRequest<unknown, ModItem>(
      () =>
        apiClient.post('/mods/upload', form, {
          headers: { 'Content-Type': 'multipart/form-data' },
        }),
      {
        id: `upload_${Date.now()}`,
        name: file.name,
        source: 'upload',
        package_name: '',
        path: '',
        enabled: enable,
      },
      { map: mapMod, quiet: true, fallbackOnError: false },
    );
  },

  inspectImport: (input: { source?: string; file?: File }) => {
    if (input.file) {
      const form = new FormData();
      form.append('file', input.file);
      return handleRequest<unknown, ImportInspection>(
        () => apiClient.post('/mods/import/inspect', form, { headers: { 'Content-Type': 'multipart/form-data' } }),
        { id: '', source_type: 'local_zip', source: input.file.name, candidates: [], expires_at: '' },
        { map: mapImportInspection, quiet: true, fallbackOnError: false },
      );
    }
    return handleRequest<unknown, ImportInspection>(
      () => apiClient.post('/mods/import/inspect', { source: input.source || '' }),
      { id: '', source_type: 'https_zip', source: input.source || '', candidates: [], expires_at: '' },
      { map: mapImportInspection, quiet: true, fallbackOnError: false },
    );
  },

  selectImportCandidate: (inspectionId: string, candidateId: string) =>
    handleRequest<unknown, ImportInspection>(
      () => apiClient.post(`/mods/import/inspect/${encodeURIComponent(inspectionId)}/select`, { candidate_id: candidateId }),
      { id: inspectionId, source_type: 'github_release', source: '', candidates: [], expires_at: '' },
      { map: mapImportInspection, quiet: true, fallbackOnError: false },
    ),

  importInspected: (inspectionId: string, candidateId?: string) =>
    handleRequest<unknown, Job>(
      () => apiClient.post('/mods/import', { inspection_id: inspectionId, candidate_id: candidateId || undefined }),
      createFallbackJob('mod_import', '已提交 Mod 导入任务'),
      { map: mapJob, quiet: true, fallbackOnError: false },
    ),

  workshopStatus: () =>
    handleRequest<unknown, WorkshopStatus>(
      () => apiClient.get('/mods/workshop/status'),
      { configured: false, key_source: '', app_id: '1623730' },
      { map: mapWorkshopStatus, quiet: true, fallbackOnError: false },
    ),

  workshopAuthStatus: () =>
    handleRequest<unknown, SteamWorkshopAuthStatus>(
      () => apiClient.get('/mods/workshop/auth/status', { timeout: STEAM_AUTH_OPERATION_TIMEOUT_MS }),
      emptySteamWorkshopAuthStatus,
      { map: mapSteamWorkshopAuthStatus, quiet: true, fallbackOnError: false },
    ),

  startWorkshopAuth: (input: { accountName: string; password: string; steamGuardCode?: string }) =>
    handleRequest<unknown, SteamWorkshopAuthStatus>(
      () => apiClient.post(
        '/mods/workshop/auth/start',
        {
          account_name: input.accountName.trim(),
          password: input.password,
          steam_guard_code: input.steamGuardCode?.trim() || undefined,
        },
        { timeout: STEAM_AUTH_OPERATION_TIMEOUT_MS },
      ),
      emptySteamWorkshopAuthStatus,
      { map: mapSteamWorkshopAuthStatus, quiet: true, fallbackOnError: false },
    ),

  verifyWorkshopAuth: (accountName?: string, steamGuardCode?: string) =>
    handleRequest<unknown, SteamWorkshopAuthStatus>(
      () => apiClient.post(
        '/mods/workshop/auth/verify',
        {
          account_name: accountName?.trim() || undefined,
          steam_guard_code: steamGuardCode?.trim() || undefined,
        },
        { timeout: STEAM_AUTH_OPERATION_TIMEOUT_MS },
      ),
      emptySteamWorkshopAuthStatus,
      { map: mapSteamWorkshopAuthStatus, quiet: true, fallbackOnError: false },
    ),

  clearWorkshopAuth: () =>
    handleRequest<unknown, SteamWorkshopAuthStatus>(
      () => apiClient.delete('/mods/workshop/auth/credentials', { timeout: STEAM_AUTH_OPERATION_TIMEOUT_MS }),
      emptySteamWorkshopAuthStatus,
      { map: mapSteamWorkshopAuthStatus, quiet: true, fallbackOnError: false },
    ),

  searchWorkshop: (params: { q?: string; sort?: string; cursor?: string; page_size?: number; tags?: string[] }) =>
    handleRequest<unknown, WorkshopSearchResponse>(
      () =>
        apiClient.get('/mods/workshop/search', {
          params: {
            q: params.q || undefined,
            sort: params.sort || undefined,
            cursor: params.cursor || undefined,
            page_size: params.page_size || undefined,
            tags: params.tags && params.tags.length > 0 ? params.tags.join(',') : undefined,
          },
        }),
      { items: [], total: 0, page_size: params.page_size || 24 },
      { map: mapWorkshopSearchResponse, quiet: true, fallbackOnError: false },
    ),

  getWorkshopItem: (itemId: string) =>
    handleRequest<unknown, WorkshopItem>(
      () => apiClient.get(`/mods/workshop/${itemId}`),
      mapWorkshopItem({ id: itemId }),
      { map: mapWorkshopItem, quiet: true, fallbackOnError: false },
    ),

  translateWorkshop: (itemId: string, force = false) =>
    handleRequest<unknown, AITranslation>(
      () => apiClient.post(`/mods/workshop/${itemId}/translate`, { force }, { timeout: AI_OPERATION_TIMEOUT_MS }),
      { text: '', target_language: 'zh-CN', model: '', generated_at: '', cached: false },
      { map: mapTranslation, quiet: true, fallbackOnError: false },
    ),

  downloadWorkshop: (itemId: string, enable = false) =>
    handleRequest<unknown, Job>(
      () => apiClient.post('/mods/workshop', { item_id: itemId, enable }),
      createFallbackJob('workshop_download', '已提交 Workshop 下载任务'),
      { map: mapJob, quiet: true, fallbackOnError: false },
    ),

  setEnabled: (id: string, enabled: boolean) =>
    handleRequest<unknown, ModItem>(
      () => apiClient.post(`/mods/${id}/${enabled ? 'enable' : 'disable'}`),
      {
        id,
        name: id,
        source: 'upload',
        package_name: '',
        path: '',
        enabled,
      },
      { map: mapMod, quiet: true, fallbackOnError: false },
    ),

  delete: (id: string) =>
    handleRequest<unknown, { deleted: boolean }>(
      () => apiClient.delete(`/mods/${id}`),
      { deleted: true },
      { quiet: true, fallbackOnError: false },
    ),
};
