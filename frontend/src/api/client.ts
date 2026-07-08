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
  fallbackOnError?: boolean;
}

const fallbackDelayMs = 120;

const readToken = () => {
  if (typeof localStorage === 'undefined') {
    return import.meta.env.VITE_PANEL_TOKEN || '';
  }
  return localStorage.getItem('palsphere_token') || import.meta.env.VITE_PANEL_TOKEN || '';
};

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
  if (shouldSuppressLog(error, quiet)) {
    if (import.meta.env.DEV) {
      const statusCode = getStatusCode(error);
      console.debug('API request used fallback', statusCode ? `HTTP ${statusCode}` : error);
    }
    return;
  }

  console.warn('API request failed, falling back to local data:', error);
};

const notifyAuthError = (error: unknown) => {
  if (getStatusCode(error) === 401 && typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent('palsphere:auth-error'));
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
