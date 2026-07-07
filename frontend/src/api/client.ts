import axios, { AxiosError, AxiosResponse } from 'axios';

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
}

const fallbackDelayMs = 120;

const readToken = () => {
  if (typeof localStorage === 'undefined') {
    return import.meta.env.VITE_PANEL_TOKEN || 'change-me';
  }
  return localStorage.getItem('palsphere_token') || import.meta.env.VITE_PANEL_TOKEN || 'change-me';
};

export const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
  timeout: 8000,
  headers: {
    'Content-Type': 'application/json',
  },
});

apiClient.interceptors.request.use((config) => {
  const token = readToken();
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
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
  if (shouldSuppressLog(error, quiet)) {
    if (import.meta.env.DEV) {
      const statusCode = getStatusCode(error);
      console.debug('API request used fallback', statusCode ? `HTTP ${statusCode}` : error);
    }
    return;
  }

  console.warn('API request failed, falling back to local data:', error);
};

export const handleRequest = async <T, R = T>(
  requestFn: () => Promise<unknown>,
  fallback: R,
  options: HandleRequestOptions<R> = {},
): Promise<R> => {
  try {
    const response = await requestFn();
    const data = unwrapApiData<unknown>(response, fallback);
    const mapped = options.map ? options.map(data) : (data as R);
    return mapped == null ? fallbackFor(fallback) : mapped;
  } catch (error) {
    logFallback(error, options.quiet);
    await new Promise((resolve) => setTimeout(resolve, fallbackDelayMs));
    return fallbackFor(fallback);
  }
};
