export type RuntimeMode = 'wine_docker' | 'windows_steamcmd';

export type ServerProcessStatus = 'running' | 'stopped' | 'starting' | 'stopping' | 'updating' | 'error';

export interface ContainerStatus {
  exists: boolean;
  status: string;
}

export interface ServerStatus {
  status: ServerProcessStatus;
  installed: boolean;
  pending_restart: boolean;
  runtime_mode: RuntimeMode;
  setup_step: 'prerequisites' | 'installed' | 'config_initialized' | 'configured' | 'started' | 'ready' | string;
  config_exists: boolean;
  container: ContainerStatus;
  startup_args: string[];
  ports: Record<string, number>;
  warnings: string[];
  paths: Record<string, string>;
  cpu_percent?: number;
  memory_usage_bytes?: number;
  port?: number;
  settings_path?: string;
  pid?: number;
  version?: string;
}

export interface ServerMetrics {
  server_fps?: number;
  current_players?: number;
  max_players?: number;
  uptime?: number;
  total_pals?: number;
  active_bases?: number;
  frame_time?: number;
}

export interface Prerequisite {
  id: string;
  label: string;
  ok: boolean;
  required: boolean;
  message?: string;
}

export interface ValidationIssue {
  field?: string;
  severity: 'error' | 'warning' | 'info' | string;
  message: string;
}

export interface StartupConfig {
  port: number;
  players: number;
  public_lobby: boolean;
  public_ip?: string;
  public_port?: number;
  log_format: 'text' | 'json' | 'Text' | 'Json' | string;
  use_perf_threads: boolean;
  no_async_loading_thread: boolean;
  use_multithread_for_ds: boolean;
  number_of_worker_threads_server?: number;
  workshop_dir?: string;
  no_mods: boolean;
}

export interface StartupResponse {
  startup: StartupConfig;
  args: string[];
  issues: ValidationIssue[];
}

export type FieldType = 'string' | 'bool' | 'int' | 'float' | 'enum' | 'list';

export interface FieldSchema {
  key: string;
  group: string;
  type: FieldType;
  default: string;
  enum?: string[];
  min?: number;
  max?: number;
  requires_restart: boolean;
  risk?: string;
  description: string;
}

export type PalworldSettings = Record<string, string | number | boolean>;

export interface PalworldConfigResponse {
  settings: PalworldSettings;
  path: string;
  pending_restart: boolean;
  issues?: ValidationIssue[];
}

export interface PalworldSchemaResponse {
  version: string;
  fields: FieldSchema[];
}

export interface PalworldValidateResponse {
  valid: boolean;
  issues: ValidationIssue[];
}

export interface Job {
  id: string;
  type:
    | 'backup'
    | 'restart'
    | 'broadcast'
    | 'install'
    | 'update'
    | 'save'
    | 'shutdown'
    | 'bootstrap'
    | 'workshop_download'
    | 'paldefender_install'
    | 'paldefender_update'
    | string;
  status: 'waiting' | 'running' | 'success' | 'failed';
  progress: number;
  message?: string;
  error?: string;
  created_at: string;
  updated_at?: string;
  finished_at?: string;
}

export interface ModItem {
  id: string;
  name: string;
  source: 'upload' | 'workshop' | string;
  package_name: string;
  path: string;
  version?: string;
  enabled: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface BackupInfo {
  name: string;
  path: string;
  size_bytes: number;
  created_at: string;
}

export interface PalDefenderAsset {
  name: string;
  size: number;
  digest?: string;
  browser_download_url: string;
}

export interface PalDefenderRelease {
  tag_name: string;
  name: string;
  published_at: string;
  assets: PalDefenderAsset[];
}

export interface PalDefenderStatus {
  installed: boolean;
  version?: string;
  needs_first_start: boolean;
  files: Record<string, boolean>;
  paths: Record<string, string>;
  rest_api_enabled: boolean;
  warnings: string[];
}

export interface TokenResult {
  name: string;
  token: string;
  permissions: string[];
  path: string;
}

export interface Player {
  id: string;
  steam_id: string;
  nickname: string;
  level: number;
  guild_name: string;
  is_online: boolean;
  last_online_time: string;
  x: number;
  y: number;
  z: number;
  ping?: number;
  ip?: string;
}

export interface PalSkill {
  name: string;
  type: string;
  power: number;
}

export interface WorkSuitability {
  type:
    | 'Handiwork'
    | 'Transport'
    | 'Watering'
    | 'Planting'
    | 'Generating'
    | 'Gathering'
    | 'Lumbering'
    | 'Mining'
    | 'Cooling'
    | 'Farming'
    | 'Medicine';
  level: number;
}

export interface Pal {
  id: string;
  name: string;
  level: number;
  rarity: 'Common' | 'Rare' | 'Boss';
  owner_nickname: string;
  owner_steam_id: string;
  skills: PalSkill[];
  work_suitability: WorkSuitability[];
  health: number;
  max_health: number;
  status: 'Healthy' | 'Injured' | 'Working' | 'Battling' | 'Dead';
  x: number;
  y: number;
  z: number;
}

export interface Base {
  id: string;
  name: string;
  guild_name: string;
  x: number;
  y: number;
  z: number;
  structures_count: number;
  pals_count: number;
  status: 'Safe' | 'Raid';
  online_members: string[];
}

export type UnsupportedActionResult = {
  ok: false;
  unsupported: true;
  message: string;
};
