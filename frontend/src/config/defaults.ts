type StorageKeyName = 'sidebarCollapsed';

const envString = (value: unknown, fallback: string) => {
  const trimmed = String(value || '').trim();
  return trimmed || fallback;
};

const storagePrefix = envString(import.meta.env.VITE_STORAGE_PREFIX, 'palpanel').replace(/[^a-z0-9_-]/gi, '_');

export const appConfig = {
  brand: envString(import.meta.env.VITE_APP_BRAND, 'PalPanel'),
  storagePrefix,
  devPort: envString(import.meta.env.VITE_DEV_PORT, '63107'),
} as const;

export const DEV_PORT = appConfig.devPort;

export const storageKeys: Record<StorageKeyName, string> = {
  sidebarCollapsed: `${appConfig.storagePrefix}_sidebar_collapsed`,
};

export const legacyStorageKeys: Record<StorageKeyName, string> = {
  sidebarCollapsed: 'palsphere_sidebar_collapsed',
};

export const appEvents = {
  authError: `${appConfig.storagePrefix}:auth-error`,
  legacyAuthError: 'palsphere:auth-error',
} as const;

export const readAppStorage = (key: StorageKeyName) => {
  if (typeof localStorage === 'undefined') return null;
  const primaryKey = storageKeys[key];
  const primaryValue = localStorage.getItem(primaryKey);
  if (primaryValue !== null) return primaryValue;

  const legacyKey = legacyStorageKeys[key];
  if (legacyKey === primaryKey) return null;
  const legacyValue = localStorage.getItem(legacyKey);
  if (legacyValue !== null) {
    localStorage.setItem(primaryKey, legacyValue);
    localStorage.removeItem(legacyKey);
  }
  return legacyValue;
};

export const writeAppStorage = (key: StorageKeyName, value: string) => {
  if (typeof localStorage === 'undefined') return;
  localStorage.setItem(storageKeys[key], value);
  if (legacyStorageKeys[key] !== storageKeys[key]) {
    localStorage.removeItem(legacyStorageKeys[key]);
  }
};

export const removeAppStorage = (key: StorageKeyName) => {
  if (typeof localStorage === 'undefined') return;
  localStorage.removeItem(storageKeys[key]);
  if (legacyStorageKeys[key] !== storageKeys[key]) {
    localStorage.removeItem(legacyStorageKeys[key]);
  }
};
