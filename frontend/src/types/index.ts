import type { components } from '../api/generated/contracts';

export type RuntimeMode = 'wine_docker' | 'windows_steamcmd';
export type Role = 'admin' | 'operator' | 'viewer';
export type Permission =
  | 'read'
  | 'server:control'
  | 'config:write'
  | 'backup:write'
  | 'mods:write'
  | 'players:write'
  | 'security:write'
  | 'audit:read'
  | 'world:reset'
  | 'ai:config';

export interface SessionInfo {
  name: string;
  role: Role;
  permissions: Permission[];
}

export interface AuthStatus {
	initialized: boolean;
	authenticated: boolean;
	user?: SessionInfo;
}

export interface DevelopmentKey {
	id: string;
	name: string;
	prefix: string;
	created_at: string;
	last_used_at?: string;
	revoked_at?: string;
	token?: string;
}

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

export interface ServerVersionInfo {
  installed: boolean;
  current_build_id: string;
  latest_build_id: string;
  update_available: boolean;
  last_checked_at: string;
  source: string;
  manifest_path: string;
  game_version?: string;
  compatibility_target?: string;
  compatible?: boolean;
  compatibility_warnings: string[];
  error?: string;
}

export interface ServerMetrics {
  server_fps?: number;
  current_players?: number;
  max_players?: number;
  uptime?: number;
  total_pals?: number;
  active_bases?: number;
  frame_time?: number;
  days?: number;
}

export interface ServerLogResponse {
  logs: string;
  source: 'file' | 'docker' | 'none';
  available: boolean;
  reason?: string;
  updated_at?: string;
}

export interface WorldInfo {
  active_world_id: string;
  save_exists: boolean;
  last_modified?: string;
  server_running: boolean;
  reset_available: boolean;
  reset_unavailable_reason?: string;
}

export interface MonitorSample {
  id: string;
  created_at: string;
  cpu_available: boolean;
  cpu_percent: number;
  memory_available: boolean;
  memory_usage_bytes: number;
  memory_limit_bytes: number;
  disk_available: boolean;
  disk_free_bytes: number;
  disk_total_bytes: number;
  current_players: number;
  max_players: number;
  rest_healthy: boolean;
  rcon_healthy: boolean;
  game_port_healthy: boolean;
  query_port_healthy: boolean;
  unavailable_reason?: string;
}

export interface MonitorSnapshot {
  sample: MonitorSample;
}

export interface Prerequisite {
  id: string;
  label: string;
  ok: boolean;
  required: boolean;
  message?: string;
}

export interface DockerCapability {
  cli_installed: boolean;
  cli_path?: string;
  daemon_reachable: boolean;
  version?: string;
  error?: string;
}

export interface SudoCapability {
  is_root: boolean;
  sudo_installed: boolean;
  passwordless: boolean;
  can_elevate: boolean;
  needs_password: boolean;
  message?: string;
}

export interface HostCapabilities {
  os: string;
  arch: string;
  distro_id?: string;
  distro_name?: string;
  distro_version?: string;
  distro_codename?: string;
  package_manager?: string;
  systemd: boolean;
  supported: boolean;
  unsupported_reason?: string;
  recommended_runtime: RuntimeMode;
  docker: DockerCapability;
  sudo: SudoCapability;
  current_user?: string;
  current_user_in_docker_group: boolean;
  warnings?: string[];
}

export type DockerSourceID = 'auto' | 'official' | 'aliyun' | 'azurecn';
export type DockerMirrorID =
  | 'auto'
  | 'daocloud'
  | 'one_ms'
  | 'registry_cyou'
  | 'dockerproxy_net'
  | 'dockerproxy_link'
  | 'docker_jiaxin'
  | 'docker_xuanyuan'
  | 'free_hubfast';

export interface DockerInstallSource {
  id: string;
  name: string;
  url: string;
  probe_url: string;
  available: boolean;
  latency_ms?: number;
  error?: string;
  selected: boolean;
}

export interface DockerInstallPlan {
  host: HostCapabilities;
  source: string;
  source_url?: string;
  sources: DockerInstallSource[];
  supported: boolean;
  can_auto_install: boolean;
  requires_manual: boolean;
  docker_installed: boolean;
  docker_ready: boolean;
  error_code?: string;
  message?: string;
  manual_command?: string;
  script?: string;
  script_path?: string;
  warnings?: string[];
}

export interface DockerInstallRequest {
  source?: DockerSourceID;
  add_current_user_to_docker_group?: boolean;
}

export interface DockerRegistryMirror {
  id: string;
  name: string;
  url: string;
  probe_url: string;
  available: boolean;
  latency_ms?: number;
  error?: string;
  selected: boolean;
}

export interface DockerMirrorPlan {
  host: HostCapabilities;
  mirror: string;
  mirrors: DockerRegistryMirror[];
  selected_mirrors: string[];
  existing_mirrors?: string[];
  config_path: string;
  supported: boolean;
  can_auto_configure: boolean;
  requires_manual: boolean;
  docker_installed: boolean;
  docker_ready: boolean;
  error_code?: string;
  message?: string;
  manual_command?: string;
  script?: string;
  script_path?: string;
  warnings?: string[];
}

export interface DockerMirrorRequest {
  mirror?: DockerMirrorID;
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
  label?: string;
  group: string;
  type: FieldType;
  default?: string;
  enum?: string[];
  enum_labels?: Record<string, string>;
  min?: number;
  max?: number;
  requires_restart: boolean;
  risk?: string;
  description: string;
}

export type PalworldSettings = Record<string, string | number | boolean | undefined>;

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

type JobContract = components['schemas']['Job'];

export interface Job extends Omit<JobContract, 'type' | 'status' | 'message' | 'updated_at'> {
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
    | 'safe_restart'
    | 'restore'
    | 'version_check'
    | 'smart_update'
    | string;
  status: 'waiting' | 'running' | 'success' | 'failed';
  progress: number;
  message?: string;
  error?: string;
  error_code?: string;
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
  workshop_id?: string;
  preview_url?: string;
  steam_url?: string;
  summary?: string;
  tags?: string[];
  file_size?: number;
  subscriptions?: number;
  time_updated?: number;
  last_checked_at?: string;
  created_at?: string;
  updated_at?: string;
}

export type ImportCandidate = components['schemas']['ImportCandidate'];
export type ImportInspection = components['schemas']['ImportInspection'];
export type ModImportRequest = components['schemas']['ModImportRequest'];

export interface WorkshopItem {
  id: string;
  title: string;
  summary?: string;
  preview_url?: string;
  steam_url: string;
  tags: string[];
  file_size?: number;
  subscriptions?: number;
  time_created?: number;
  time_updated?: number;
  installed: boolean;
  enabled: boolean;
  update_available: boolean;
  mod_id?: string;
  translation?: AITranslation;
}

export interface AITranslation {
  text: string;
  target_language: string;
  model: string;
  generated_at: string;
  cached: boolean;
}

export interface AITranslationConfig {
  configured: boolean;
  base_url: string;
  model: string;
  api_key_present: boolean;
}

export interface AITranslationConfigUpdate {
  base_url?: string;
  model?: string;
  api_key?: string;
  clear_api_key?: boolean;
}

export interface AITranslationTestResult {
  ok: boolean;
  base_url: string;
  model: string;
  message: string;
}

export interface WorkshopSearchResponse {
  items: WorkshopItem[];
  next_cursor?: string;
  total: number;
  page_size: number;
}

export interface WorkshopStatus {
  configured: boolean;
  key_source?: 'environment' | 'bundled' | '';
  app_id: string;
}

export interface BackupInfo {
  name: string;
  path: string;
  size_bytes: number;
  created_at: string;
  reason?: string;
  status?: string;
}

export interface BackupVerifyResult {
  name: string;
  valid: boolean;
  format: string;
  checked_files: number;
  errors: string[];
}

export interface BackupRestoreRequest {
  name: string;
}

export interface SafeRestartRequest {
  waittime: number;
  message: string;
}

export interface AuditLog {
  id: string;
  actor: string;
  role: Role | string;
  action: string;
  target?: string;
  status: 'success' | 'failed' | string;
  message?: string;
  ip?: string;
  created_at: string;
}

export interface Alert {
  id: string;
  severity: 'info' | 'warning' | 'error' | string;
  title: string;
  message: string;
  source: string;
  status: 'open' | 'acked' | string;
  created_at: string;
  ack_at?: string;
}

export type ScheduleType = 'save' | 'backup' | 'safe_restart' | 'update' | 'version_check';

type ScheduleContract = components['schemas']['Schedule'];

export interface Schedule extends Omit<ScheduleContract, 'type'> {
  id: string;
  type: ScheduleType | string;
  enabled: boolean;
  interval_minutes?: number;
  time_of_day?: string;
  timezone: string;
  waittime?: number;
  message?: string;
  last_run_at?: string;
  next_run_at?: string;
  created_at: string;
  updated_at: string;
}

export interface ApiErrorShape {
  code?: string;
  message: string;
  status?: number;
}

export interface UnsupportedCapability {
  ok: false;
  unsupported: true;
  message: string;
}

export interface PlayerAccessEntry {
  steam_id: string;
  nickname?: string;
  reason?: string;
  created_at?: string;
  updated_at?: string;
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
  bundled: {
    version: string;
    sha256: string;
    size: number;
  };
  needs_first_start: boolean;
  files: Record<string, boolean>;
  paths: Record<string, string>;
  rest_api_enabled: boolean;
  warnings: string[];
}

export type PalDefenderGMStatus = components['schemas']['PalDefenderGMStatus'];
export type PalDefenderGMPlayer = components['schemas']['PalDefenderGMPlayer'];
export type PalDefenderGMPlayers = components['schemas']['PalDefenderGMPlayers'];
export type PalDefenderGMInventory = components['schemas']['PalDefenderGMInventory'];
export type PalDefenderInventory = components['schemas']['PalDefenderInventory'];
export type PalDefenderInventoryContainer = components['schemas']['PalDefenderInventoryContainer'];
export type PalDefenderInventorySlot = components['schemas']['PalDefenderInventorySlot'];
export type PalDefenderItemGrant = components['schemas']['PalDefenderItemGrant'];
export type PalDefenderItemCatalogEntry = components['schemas']['PalDefenderItemCatalogEntry'];
export type PalDefenderItemCatalog = components['schemas']['PalDefenderItemCatalog'];
export type PalDefenderMessageRequest = components['schemas']['PalDefenderMessageRequest'];
export type PalDefenderPunishmentRequest = components['schemas']['PalDefenderPunishmentRequest'];

export interface PalDefenderGiveItemsResult {
  Granted: {
    Items: number;
  };
}

export interface TokenResult {
  name: string;
  token: string;
  permissions: string[];
  path: string;
}

export interface Player {
  id: string;
  player_uid?: string;
  steam_id: string;
  nickname: string;
  level: number;
  guild_id?: string;
  guild_name: string;
  is_online: boolean;
  last_online_time: string;
  x: number;
  y: number;
  z: number;
  ping?: number;
  ip?: string;
  inventory_summary?: Record<string, unknown>;
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
  instance_id?: string;
  character_id?: string;
  species_name?: string;
  name: string;
  nickname?: string;
  level: number;
  rarity: 'Common' | 'Rare' | 'Boss';
  rarity_name?: string;
  owner_player_uid?: string;
  owner_nickname: string;
  owner_steam_id: string;
  guild_id?: string;
  container_id?: string;
  skills: PalSkill[];
  passives?: string[];
  raw_passives?: string[];
  raw_skills?: string[];
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
  guild_id?: string;
  guild_name: string;
  x: number;
  y: number;
  z: number;
  structures_count: number;
  pals_count: number;
  status: 'Safe' | 'Raid';
  online_members: string[];
  workers?: Array<{
    instance_id: string;
    character_id: string;
    species_name?: string;
    name?: string;
    nickname?: string;
    level?: number;
  }>;
  containers?: string[];
}

export interface GuildMember {
  player_uid: string;
  nickname: string;
  last_online_time?: string;
}

export interface Guild {
  id: string;
  name: string;
  owner_player_uid: string;
  members: GuildMember[];
  base_ids: string[];
  online_member_count: number;
}

export interface SaveIndexCounts {
  players: number;
  guilds: number;
  bases: number;
  pals: number;
  containers: number;
  map_entities: number;
}

export interface SaveIndexStatus {
  enabled: boolean;
  state: 'disabled' | 'missing' | 'not_indexed' | 'ready' | 'stale' | 'error' | string;
  stale: boolean;
  source_path: string;
  updated_at: string;
  duration_ms: number;
  error?: string;
  warnings: string[];
  counts: SaveIndexCounts;
  parser?: string;
  cache_path?: string;
}

export interface ListSummary {
  total: number;
  limit: number;
  offset: number;
  returned: number;
  page: number;
}

export interface EntityListResponse<T> {
  items: T[];
  status: SaveIndexStatus;
  summary: ListSummary;
}

export interface EntityListParams {
  limit?: number;
  offset?: number;
  page?: number;
  q?: string;
  online?: boolean;
  status?: string;
  owner_player_uid?: string;
  guild_id?: string;
  container_id?: string;
}

export type UnsupportedActionResult = {
  ok: false;
  unsupported: true;
  message: string;
};
