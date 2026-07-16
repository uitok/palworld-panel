import { apiClient, handleRequest } from './client';
import { isJobDone, mapJob } from './tasks';
import type { BreedingCatalog, BreedingSolveResult, Job } from '../types';

export interface BreedSessionPrincipal {
  subject: string;
  qq_id: string;
  player_uid: string;
  nickname?: string;
  balance?: number;
}

export interface BreedingSubmitInput {
  owner_player_uid?: string;
  custom_container_ids?: string[];
  target: {
    pal_id: string;
    gender: string;
    required_passives: string[];
    optional_passives: string[];
    iv_hp: number;
    iv_attack: number;
    iv_defense: number;
  };
  settings: {
    max_breeding_steps: number;
    max_solver_iterations: number;
    max_wild_pals: number;
    max_input_irrelevant_passives: number;
    max_bred_irrelevant_passives: number;
    max_threads: number;
    max_gold_cost: number;
    use_gender_reversers: boolean;
    allowed_wild_pals?: string[];
    banned_wild_pals?: string[];
    banned_bred_pals?: string[];
    allowed_surgery_passives?: string[];
    banned_surgery_passives?: string[];
  };
  game_settings: {
    breeding_time_seconds: number;
    massive_egg_incubation_minutes: number;
    multiple_breeding_farms: boolean;
    multiple_incubators: boolean;
  };
  result_limit: number;
}

export interface BreedingResultResponse {
  job_id: string;
  status: string;
  fingerprint?: string;
  stale?: boolean;
  result?: { save_fingerprint: string; results: BreedingSolveResult[] };
}

export interface BreedSessionSubmitResponse {
  job: Job;
  balance: number;
}

export type BreedingAccess = 'admin' | 'session';

export interface CustomPalContainerSummary {
  id: string;
  name: string;
  pals: Array<Record<string, unknown>>;
}

export interface BreedingPresetSummary {
  id: string;
  name: string;
  config: Record<string, unknown>;
}

const mapCatalog = (raw: unknown): BreedingCatalog => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const items = (value: unknown) => (Array.isArray(value) ? value : []).map((entry) => {
    const item = (entry && typeof entry === 'object' ? entry : {}) as Record<string, unknown>;
    return { id: String(item.id || ''), name: String(item.name || item.id || '') };
  });
  return {
    version: String(data.version || ''),
    pals: items(data.pals),
    passives: (Array.isArray(data.passives) ? data.passives : []).map((entry) => {
      const item = (entry && typeof entry === 'object' ? entry : {}) as Record<string, unknown>;
      return { id: String(item.id || ''), name: String(item.name || item.id || ''), supports_surgery: Boolean(item.supports_surgery), surgery_cost: Number(item.surgery_cost || 0) };
    }),
    active_skills: items(data.active_skills),
  };
};

export const breedingApi = {
  catalog: (access: BreedingAccess = 'admin') => handleRequest<unknown, BreedingCatalog>(() => apiClient.get(access === 'session' ? '/breed/catalog' : '/breeding/catalog'), { version: '', pals: [], passives: [], active_skills: [] }, { map: mapCatalog, fallbackOnError: false }),
  customContainers: (access: BreedingAccess = 'admin') => handleRequest<unknown, CustomPalContainerSummary[]>(
    () => apiClient.get(access === 'session' ? '/breed/custom-containers' : '/breeding/custom-containers'),
    [],
    { fallbackOnError: false, map: (raw) => (Array.isArray(raw) ? raw : []).map((entry) => { const item = entry as Record<string, unknown>; return { id: String(item.id || ''), name: String(item.name || item.id || ''), pals: Array.isArray(item.pals) ? item.pals as Array<Record<string, unknown>> : [] }; }) },
  ),
  presets: (access: BreedingAccess = 'admin') => handleRequest<unknown, BreedingPresetSummary[]>(
    () => apiClient.get(access === 'session' ? '/breed/presets' : '/breeding/presets'),
    [],
    { fallbackOnError: false, map: (raw) => (Array.isArray(raw) ? raw : []).map((entry) => { const item = entry as Record<string, unknown>; return { id: String(item.id || ''), name: String(item.name || item.id || ''), config: item.config && typeof item.config === 'object' ? item.config as Record<string, unknown> : {} }; }) },
  ),
  savePreset: (access: BreedingAccess, name: string, config: Record<string, unknown>) => handleRequest(
    () => apiClient.post(access === 'session' ? '/breed/presets' : '/breeding/presets', { name, config }),
    {},
    { fallbackOnError: false },
  ),
  submit: (input: BreedingSubmitInput, access: BreedingAccess = 'admin') => handleRequest<unknown, BreedSessionSubmitResponse>(
    () => apiClient.post(access === 'session' ? '/breed/jobs' : '/breeding/jobs', input),
    { job: mapJob({}), balance: 0 },
    {
      map: (raw) => {
        const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
        return access === 'session'
          ? { job: mapJob(data.job), balance: Number(data.balance || 0) }
          : { job: mapJob(data), balance: 0 };
      },
      fallbackOnError: false,
    },
  ),
  job: (jobId: string, access: BreedingAccess = 'admin') => handleRequest<unknown, Job>(
    () => apiClient.get(access === 'session' ? `/breed/jobs/${encodeURIComponent(jobId)}` : `/jobs/${encodeURIComponent(jobId)}`),
    mapJob({ id: jobId }),
    { map: mapJob, fallbackOnError: false },
  ),
  waitForJob: async (jobId: string, access: BreedingAccess, onUpdate?: (job: Job) => void) => {
    let current = await breedingApi.job(jobId, access);
    onUpdate?.(current);
    for (let attempt = 0; attempt < 1800 && !isJobDone(current); attempt += 1) {
      await new Promise((resolve) => setTimeout(resolve, 1000));
      current = await breedingApi.job(jobId, access);
      onUpdate?.(current);
    }
    return current;
  },
  result: (jobId: string, access: BreedingAccess = 'admin') => handleRequest<unknown, BreedingResultResponse>(() => apiClient.get(`${access === 'session' ? '/breed' : '/breeding'}/jobs/${encodeURIComponent(jobId)}/result`), { job_id: jobId, status: 'waiting' }, { fallbackOnError: false }),
  pause: (jobId: string, access: BreedingAccess = 'admin') => handleRequest(() => apiClient.post(`${access === 'session' ? '/breed' : '/breeding'}/jobs/${encodeURIComponent(jobId)}/pause`), {}, { fallbackOnError: false }),
  resume: (jobId: string, access: BreedingAccess = 'admin') => handleRequest(() => apiClient.post(`${access === 'session' ? '/breed' : '/breeding'}/jobs/${encodeURIComponent(jobId)}/resume`), {}, { fallbackOnError: false }),
  cancel: (jobId: string, access: BreedingAccess = 'admin') => handleRequest(() => apiClient.post(`${access === 'session' ? '/breed' : '/breeding'}/jobs/${encodeURIComponent(jobId)}/cancel`), {}, { fallbackOnError: false }),
};

export const breedSessionApi = {
  exchange: (ticket: string) => handleRequest<unknown, BreedSessionPrincipal>(
    () => apiClient.post('/breed/session/exchange', { ticket }),
    { subject: '', qq_id: '', player_uid: '' },
    { fallbackOnError: false },
  ),
  me: () => handleRequest<unknown, BreedSessionPrincipal>(
    () => apiClient.get('/breed/me'),
    { subject: '', qq_id: '', player_uid: '' },
    { fallbackOnError: false },
  ),
};
