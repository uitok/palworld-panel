import axios, { AxiosError, AxiosResponse } from 'axios';
import {
  CONFIGURED_BACKEND_URL,
  DEFAULT_BACKEND_PORT,
  appEvents,
  readAppStorage,
  removeAppStorage,
  writeAppStorage,
} from '../config/defaults';

interface ApiEnvelope<T = unknown> {
  ok: boolean;
  data?: T | null;
  error?: {
    code?: string;
    message?: string;
  } | string;
}

interface HandleRequestOptions<R> {
  map?: (data: unknown) => R;
  quiet?: boolean;
  fallbackOnError?: boolean;
}

const fallbackDelayMs = 120;
const sameOriginApiBaseUrl = () => import.meta.env.VITE_API_BASE_URL || '/api';

const loopbackHosts = new Set(['localhost', '127.0.0.1', '0.0.0.0', '::1']);

const readToken = () => {
  if (typeof localStorage === 'undefined') {
    return '';
  }
  return readAppStorage('token') || '';
};

const currentPageHostname = () => {
  if (typeof window === 'undefined') return '';
  return window.location.hostname;
};

const isLoopbackHost = (host: string) => loopbackHosts.has(host.toLowerCase());

const withProtocol = (value: string) => (
  value.startsWith('/') || /^[a-z][a-z\d+\-.]*:\/\//i.test(value) ? value : `http://${value}`
);

const adaptLoopbackToCurrentHost = (backendUrl: string) => {
  const value = backendUrl.trim();
  if (!value || value.startsWith('/')) return value;
  const normalizedValue = withProtocol(value);
  const pageHost = currentPageHostname();
  if (!pageHost || isLoopbackHost(pageHost)) return normalizedValue.replace(/\/+$/, '');

  try {
    const parsed = new URL(normalizedValue);
    if (isLoopbackHost(parsed.hostname)) {
      parsed.hostname = pageHost;
    }
    return parsed.toString().replace(/\/+$/, '');
  } catch {
    return normalizedValue.replace(/\/+$/, '');
  }
};

export const defaultBackendUrl = (
  isDev = import.meta.env.DEV,
  configuredBackendUrl = CONFIGURED_BACKEND_URL,
) => {
  if (configuredBackendUrl) {
    return adaptLoopbackToCurrentHost(configuredBackendUrl);
  }
  if (!isDev) return '';
  const pageHost = currentPageHostname();
  const host = pageHost && !isLoopbackHost(pageHost) ? pageHost : '127.0.0.1';
  return `http://${host}:${DEFAULT_BACKEND_PORT}`;
};

export const readBackendUrl = (
  isDev = import.meta.env.DEV,
  configuredBackendUrl = CONFIGURED_BACKEND_URL,
) => {
  // A relative deployment URL is an explicit same-origin proxy choice. Do not
  // let an absolute URL left in browser storage bypass that proxy.
  if (configuredBackendUrl.trim().startsWith('/')) {
    return adaptLoopbackToCurrentHost(configuredBackendUrl);
  }
  if (typeof localStorage === 'undefined') {
    return defaultBackendUrl(isDev, configuredBackendUrl);
  }
  const stored = readAppStorage('backendUrl');
  return stored ? adaptLoopbackToCurrentHost(stored) : defaultBackendUrl(isDev, configuredBackendUrl);
};

export const writeBackendUrl = (value: string) => {
  if (typeof localStorage === 'undefined') return;
  const nextValue = value.trim();
  if (nextValue) {
    writeAppStorage('backendUrl', nextValue);
  } else {
    removeAppStorage('backendUrl');
  }
};

const apiBaseUrlFor = (backendUrl: string) => {
  const value = backendUrl.trim();
  if (!value) return import.meta.env.VITE_API_BASE_URL || '/api';
  const normalizedValue = withProtocol(value);
  const withoutTrailingSlash = normalizedValue.replace(/\/+$/, '');
  return withoutTrailingSlash.endsWith('/api') ? withoutTrailingSlash : `${withoutTrailingSlash}/api`;
};

export const currentApiBaseUrl = (
  isDev = import.meta.env.DEV,
  configuredBackendUrl = CONFIGURED_BACKEND_URL,
) => apiBaseUrlFor(readBackendUrl(isDev, configuredBackendUrl));

interface RetryableAxiosConfig {
  baseURL?: string;
  url?: string;
  headers?: unknown;
  _palpanelProxyRetry?: boolean;
  _palpanelUseProxyBase?: boolean;
}

export class ApiError extends Error {
  status?: number;
  code?: string;

  constructor(message: string, status?: number, code?: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

export const apiClient = axios.create({
  baseURL: currentApiBaseUrl(),
  timeout: 8000,
  headers: {
    'Content-Type': 'application/json',
  },
});

apiClient.interceptors.request.use((config) => {
  const retryConfig = config as typeof config & RetryableAxiosConfig;
  config.baseURL = retryConfig._palpanelUseProxyBase ? sameOriginApiBaseUrl() : currentApiBaseUrl();
  const token = readToken();
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

const shouldRetryWithDevProxy = (error: AxiosError, config?: RetryableAxiosConfig) => {
  if (!import.meta.env.DEV || !config || config._palpanelProxyRetry) return false;
  if (error.response) return false;
  if (!['ERR_NETWORK', 'ECONNABORTED', 'ETIMEDOUT'].includes(String(error.code || ''))) return false;
  const currentBase = currentApiBaseUrl();
  const proxyBase = sameOriginApiBaseUrl();
  return /^[a-z][a-z\d+\-.]*:\/\//i.test(currentBase) && proxyBase.startsWith('/');
};

apiClient.interceptors.response.use(undefined, async (error: AxiosError) => {
  const config = error.config as (typeof error.config & RetryableAxiosConfig) | undefined;
  if (!config || !shouldRetryWithDevProxy(error, config)) {
    return Promise.reject(error);
  }
  config._palpanelProxyRetry = true;
  config._palpanelUseProxyBase = true;
  config.baseURL = sameOriginApiBaseUrl();
  return apiClient.request(config);
});

const isAxiosResponse = (value: unknown): value is AxiosResponse => {
  return Boolean(value && typeof value === 'object' && 'data' in value && 'status' in value);
};

const isApiEnvelope = (value: unknown): value is ApiEnvelope => {
  return Boolean(value && typeof value === 'object' && 'ok' in value);
};

const fallbackFor = <T>(fallback: T): T => {
  return (Array.isArray(fallback) ? [] : fallback) as T;
};

export const demoDataEnabled = () => {
  return Boolean(import.meta.env.DEV && import.meta.env.VITE_ENABLE_DEMO_DATA === 'true');
};

export const unwrapApiData = <T>(response: unknown, fallback: T): T => {
  const body = isAxiosResponse(response) ? response.data : response;

  if (isApiEnvelope(body)) {
    if (!body.ok) {
      return fallbackFor(fallback);
    }
    return body.data == null ? fallbackFor(fallback) : (body.data as T);
  }

  return body == null ? fallbackFor(fallback) : (body as T);
};

const apiErrorFrom = (error: unknown): ApiError => {
  const axiosError = error as AxiosError<ApiEnvelope> | undefined;
  const status = axiosError?.response?.status;
  const payload = axiosError?.response?.data;
  if (!status && axiosError?.code === 'ERR_NETWORK') {
    return new ApiError(`无法连接后端 ${readBackendUrl()}。请确认后端已启动、地址可从当前浏览器访问，并检查 CORS 配置。`);
  }
  if (!status && (axiosError?.code === 'ECONNABORTED' || axiosError?.code === 'ETIMEDOUT')) {
    return new ApiError(`连接后端 ${readBackendUrl()} 超时。请确认后端正在响应。`);
  }
  if (status === 401) {
    return new ApiError('面板 Token 无效或权限不足，请重新输入。', status, 'unauthorized');
  }
  if (payload?.error) {
    if (typeof payload.error === 'string') {
      return new ApiError(payload.error, status);
    }
    return new ApiError(payload.error.message || payload.error.code || 'API request failed', status, payload.error.code);
  }
  if (error instanceof ApiError) {
    return error;
  }
  if (error instanceof Error) {
    return new ApiError(error.message, status);
  }
  return new ApiError('API request failed', status);
};

const getStatusCode = (error: unknown) => {
  const axiosError = error as AxiosError | undefined;
  return axiosError?.response?.status;
};

const shouldSuppressLog = (error: unknown, quiet?: boolean) => {
  if (quiet) return true;
  const statusCode = getStatusCode(error);
  if (!statusCode) return false;
  return [400, 401, 403, 404, 409, 422, 500, 502, 503, 504].includes(statusCode);
};

const logFallback = (error: unknown, quiet?: boolean) => {
  if (!import.meta.env.DEV) return;
  if (shouldSuppressLog(error, quiet)) {
    const statusCode = getStatusCode(error);
    console.debug('API request used fallback', statusCode ? `HTTP ${statusCode}` : error);
    return;
  }

  console.warn('API request failed, falling back to local data:', error);
};

const notifyAuthError = (error: unknown) => {
  if (getStatusCode(error) === 401 && typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent(appEvents.authError));
  }
};

export const handleRequest = async <T, R = T>(
  requestFn: () => Promise<unknown>,
  fallback: R,
  options: HandleRequestOptions<R> = {},
): Promise<R> => {
  const fallbackOnError = options.fallbackOnError ?? demoDataEnabled();
  try {
    const response = await requestFn();
    const body = isAxiosResponse(response) ? response.data : response;
    if (!fallbackOnError && isApiEnvelope(body) && !body.ok) {
      const error = body.error;
      if (typeof error === 'string') {
        throw new ApiError(error);
      }
      throw new ApiError(error?.message || error?.code || 'API request failed', undefined, error?.code);
    }
    const data = unwrapApiData<unknown>(response, fallback);
    const mapped = options.map ? options.map(data) : (data as R);
    return mapped == null ? fallbackFor(fallback) : mapped;
  } catch (error) {
    notifyAuthError(error);
    if (!fallbackOnError) {
      throw apiErrorFrom(error);
    }
    logFallback(error, options.quiet);
    await new Promise((resolve) => setTimeout(resolve, fallbackDelayMs));
    return fallbackFor(fallback);
  }
};

export const getErrorMessage = (error: unknown) => {
  if (error instanceof ApiError) return error.message;
  if (error instanceof Error) return error.message;
  return '操作失败，请检查后端状态';
};
