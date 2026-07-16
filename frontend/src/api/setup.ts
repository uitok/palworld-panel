import { apiClient, handleRequest } from './client';
import type {
  DockerCapability,
  DockerInstallPlan,
  DockerInstallRequest,
  DockerInstallSource,
  DockerMirrorID,
  DockerMirrorPlan,
  DockerMirrorRequest,
  DockerRegistryMirror,
  DockerSourceID,
  HostCapabilities,
  Job,
  Prerequisite,
  RuntimeMode,
  ServerImportResult,
  SudoCapability,
  StartupConfig,
  StartupResponse,
  ValidationIssue,
} from '../types';
import { mapJob } from './tasks';

const defaultStartup: StartupConfig = {
  port: 8211,
  players: 32,
  public_lobby: false,
  public_ip: '',
  public_port: 8211,
  log_format: 'text',
  use_perf_threads: true,
  no_async_loading_thread: true,
  use_multithread_for_ds: true,
  number_of_worker_threads_server: 0,
  workshop_dir: '',
  no_mods: false,
};
const mapPrerequisites = (raw: unknown): Prerequisite[] => {
  if (!Array.isArray(raw)) return [];
  return raw.map((item) => {
    const data = (item && typeof item === 'object' ? item : {}) as Record<string, unknown>;
    return {
      id: String(data.id || ''),
      label: String(data.label || data.id || ''),
      ok: Boolean(data.ok),
      required: Boolean(data.required),
      message: data.message ? String(data.message) : undefined,
    };
  });
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

const emptyDockerCapability: DockerCapability = {
  cli_installed: false,
  daemon_reachable: false,
};

const emptySudoCapability: SudoCapability = {
  is_root: false,
  sudo_installed: false,
  passwordless: false,
  can_elevate: false,
  needs_password: false,
};

const mapRuntimeMode = (value: unknown): RuntimeMode => (value === 'windows_steamcmd' ? 'windows_steamcmd' : 'wine_docker');

const mapServerImportResult = (raw: unknown): ServerImportResult => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    path: String(data.path || ''),
    manifest_path: String(data.manifest_path || ''),
    build_id: String(data.build_id || ''),
    config_exists: Boolean(data.config_exists),
    already_bound: Boolean(data.already_bound),
    original_input: String(data.original_input || ''),
  };
};

const mapDockerCapability = (raw: unknown): DockerCapability => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    cli_installed: Boolean(data.cli_installed),
    cli_path: data.cli_path ? String(data.cli_path) : undefined,
    daemon_reachable: Boolean(data.daemon_reachable),
    version: data.version ? String(data.version) : undefined,
    error: data.error ? String(data.error) : undefined,
  };
};

const mapSudoCapability = (raw: unknown): SudoCapability => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    is_root: Boolean(data.is_root),
    sudo_installed: Boolean(data.sudo_installed),
    passwordless: Boolean(data.passwordless),
    can_elevate: Boolean(data.can_elevate),
    needs_password: Boolean(data.needs_password),
    message: data.message ? String(data.message) : undefined,
  };
};

export const mapHostCapabilities = (raw: unknown): HostCapabilities => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    os: String(data.os || ''),
    arch: String(data.arch || ''),
    distro_id: data.distro_id ? String(data.distro_id) : undefined,
    distro_name: data.distro_name ? String(data.distro_name) : undefined,
    distro_version: data.distro_version ? String(data.distro_version) : undefined,
    distro_codename: data.distro_codename ? String(data.distro_codename) : undefined,
    package_manager: data.package_manager ? String(data.package_manager) : undefined,
    systemd: Boolean(data.systemd),
    supported: Boolean(data.supported),
    unsupported_reason: data.unsupported_reason ? String(data.unsupported_reason) : undefined,
    recommended_runtime: mapRuntimeMode(data.recommended_runtime),
    docker: mapDockerCapability(data.docker),
    sudo: mapSudoCapability(data.sudo),
    current_user: data.current_user ? String(data.current_user) : undefined,
    current_user_in_docker_group: Boolean(data.current_user_in_docker_group),
    warnings: Array.isArray(data.warnings) ? data.warnings.map(String) : [],
  };
};

const emptyHost: HostCapabilities = {
  os: '',
  arch: '',
  systemd: false,
  supported: false,
  recommended_runtime: 'wine_docker',
  docker: emptyDockerCapability,
  sudo: emptySudoCapability,
  current_user_in_docker_group: false,
  warnings: [],
};

const mapDockerSource = (raw: unknown): DockerInstallSource => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const latency = Number(data.latency_ms);
  return {
    id: String(data.id || ''),
    name: String(data.name || data.id || ''),
    url: String(data.url || ''),
    probe_url: String(data.probe_url || ''),
    available: Boolean(data.available),
    latency_ms: Number.isFinite(latency) ? latency : undefined,
    error: data.error ? String(data.error) : undefined,
    selected: Boolean(data.selected),
  };
};

const mapDockerRegistryMirror = (raw: unknown): DockerRegistryMirror => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const latency = Number(data.latency_ms);
  return {
    id: String(data.id || ''),
    name: String(data.name || data.id || ''),
    url: String(data.url || ''),
    probe_url: String(data.probe_url || ''),
    available: Boolean(data.available),
    latency_ms: Number.isFinite(latency) ? latency : undefined,
    error: data.error ? String(data.error) : undefined,
    selected: Boolean(data.selected),
  };
};

export const mapDockerInstallPlan = (raw: unknown): DockerInstallPlan => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    host: data.host ? mapHostCapabilities(data.host) : emptyHost,
    source: String(data.source || 'auto'),
    source_url: data.source_url ? String(data.source_url) : undefined,
    sources: Array.isArray(data.sources) ? data.sources.map(mapDockerSource) : [],
    supported: Boolean(data.supported),
    can_auto_install: Boolean(data.can_auto_install),
    requires_manual: Boolean(data.requires_manual),
    docker_installed: Boolean(data.docker_installed),
    docker_ready: Boolean(data.docker_ready),
    error_code: data.error_code ? String(data.error_code) : undefined,
    message: data.message ? String(data.message) : undefined,
    manual_command: data.manual_command ? String(data.manual_command) : undefined,
    script: data.script ? String(data.script) : undefined,
    script_path: data.script_path ? String(data.script_path) : undefined,
    warnings: Array.isArray(data.warnings) ? data.warnings.map(String) : [],
  };
};

export const mapDockerMirrorPlan = (raw: unknown): DockerMirrorPlan => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    host: data.host ? mapHostCapabilities(data.host) : emptyHost,
    mirror: String(data.mirror || 'auto'),
    mirrors: Array.isArray(data.mirrors) ? data.mirrors.map(mapDockerRegistryMirror) : [],
    selected_mirrors: Array.isArray(data.selected_mirrors) ? data.selected_mirrors.map(String) : [],
    existing_mirrors: Array.isArray(data.existing_mirrors) ? data.existing_mirrors.map(String) : [],
    config_path: String(data.config_path || '/etc/docker/daemon.json'),
    supported: Boolean(data.supported),
    can_auto_configure: Boolean(data.can_auto_configure),
    requires_manual: Boolean(data.requires_manual),
    docker_installed: Boolean(data.docker_installed),
    docker_ready: Boolean(data.docker_ready),
    error_code: data.error_code ? String(data.error_code) : undefined,
    message: data.message ? String(data.message) : undefined,
    manual_command: data.manual_command ? String(data.manual_command) : undefined,
    script: data.script ? String(data.script) : undefined,
    script_path: data.script_path ? String(data.script_path) : undefined,
    warnings: Array.isArray(data.warnings) ? data.warnings.map(String) : [],
  };
};

export const mapStartup = (raw: unknown): StartupResponse => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    startup: { ...defaultStartup, ...((data.startup as Partial<StartupConfig>) || data) },
    args: Array.isArray(data.args) ? data.args.map(String) : [],
    issues: mapIssues(data.issues),
  };
};

const fallbackJob = (type: string, message: string): Job => ({
  id: `local_${Date.now()}`,
  type,
  status: 'waiting',
  progress: 0,
  message,
  created_at: new Date().toISOString(),
});

export const setupApi = {
  getPrerequisites: () =>
    handleRequest<unknown, Prerequisite[]>(
      () => apiClient.get('/server/prerequisites'),
      [],
      { map: mapPrerequisites, quiet: true },
    ),

  getRuntime: () =>
    handleRequest<unknown, { mode: RuntimeMode }>(
      () => apiClient.get('/server/runtime'),
      { mode: 'wine_docker' },
      {
        map: (raw) => {
          const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
          return { mode: mapRuntimeMode(data.mode) };
        },
        quiet: true,
      },
    ),

  getHost: () =>
    handleRequest<unknown, HostCapabilities>(
      () => apiClient.get('/server/host'),
      emptyHost,
      { map: mapHostCapabilities, quiet: true },
    ),

  getDockerPlan: (source: DockerSourceID = 'auto') =>
    handleRequest<unknown, DockerInstallPlan>(
      () => apiClient.get(`/server/docker/plan?source=${encodeURIComponent(source)}`),
      {
        host: emptyHost,
        source,
        sources: [],
        supported: false,
        can_auto_install: false,
        requires_manual: false,
        docker_installed: false,
        docker_ready: false,
        warnings: [],
      },
      { map: mapDockerInstallPlan, quiet: true },
    ),

  getDockerMirrorPlan: (mirror: DockerMirrorID = 'auto') =>
    handleRequest<unknown, DockerMirrorPlan>(
      () => apiClient.get(`/server/docker/mirrors/plan?mirror=${encodeURIComponent(mirror)}`),
      {
        host: emptyHost,
        mirror,
        mirrors: [],
        selected_mirrors: [],
        existing_mirrors: [],
        config_path: '/etc/docker/daemon.json',
        supported: false,
        can_auto_configure: false,
        requires_manual: false,
        docker_installed: false,
        docker_ready: false,
        warnings: [],
      },
      { map: mapDockerMirrorPlan, quiet: true },
    ),

  installDocker: (request: DockerInstallRequest) =>
    handleRequest<unknown, Job>(
      () => apiClient.post('/server/docker/install', request),
      fallbackJob('docker_install', '已提交 Docker 安装任务'),
      { map: mapJob, quiet: true, fallbackOnError: false },
    ),

  configureDockerMirrors: (request: DockerMirrorRequest) =>
    handleRequest<unknown, Job>(
      () => apiClient.post('/server/docker/mirrors/configure', request),
      fallbackJob('docker_mirror_configure', '已提交 Docker 镜像加速配置任务'),
      { map: mapJob, quiet: true, fallbackOnError: false },
    ),

  setRuntime: (mode: RuntimeMode) =>
    handleRequest<unknown, { mode: RuntimeMode }>(
      () => apiClient.put('/server/runtime', { mode }),
      { mode },
      { quiet: true, fallbackOnError: false },
    ),

  importServerDirectory: (path: string) =>
    handleRequest<unknown, ServerImportResult>(
      () => apiClient.post('/server/import', { path }),
      { path: '', manifest_path: '', build_id: '', config_exists: false, already_bound: false, original_input: path },
      { map: mapServerImportResult, quiet: true, fallbackOnError: false },
    ),

  getStartup: () =>
    handleRequest<unknown, StartupResponse>(
      () => apiClient.get('/server/startup'),
      { startup: defaultStartup, args: [], issues: [] },
      { map: mapStartup, quiet: true },
    ),

  setStartup: (startup: StartupConfig) =>
    handleRequest<unknown, StartupResponse>(
      () => apiClient.put('/server/startup', startup),
      { startup, args: [], issues: [] },
      { map: mapStartup, quiet: true, fallbackOnError: false },
    ),

  bootstrap: () =>
    handleRequest<unknown, Job>(
      () => apiClient.post('/server/bootstrap'),
      fallbackJob('bootstrap', '已提交开服初始化任务'),
      { map: mapJob, quiet: true, fallbackOnError: false },
    ),

  install: () =>
    handleRequest<unknown, Job>(
      () => apiClient.post('/server/install'),
      fallbackJob('install', '已提交安装任务'),
      { map: mapJob, quiet: true, fallbackOnError: false },
    ),

  initializeConfig: () =>
    handleRequest<unknown, { path?: string }>(
      () => apiClient.post('/server/initialize-config'),
      {},
      { quiet: true, fallbackOnError: false },
    ),
};
