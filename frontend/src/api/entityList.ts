import type { EntityListParams, ListSummary } from '../types';

export const emptySummary: ListSummary = {
  total: 0,
  limit: 0,
  offset: 0,
  returned: 0,
  page: 1,
};

export const entityListQuery = (params: EntityListParams = {}) => {
  const query = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value === undefined || value === null || value === '') return;
    query.set(key, String(value));
  });
  const text = query.toString();
  return text ? `?${text}` : '';
};

export const mapSummary = (raw: unknown): ListSummary => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    total: Number(data.total || 0),
    limit: Number(data.limit || 0),
    offset: Number(data.offset || 0),
    returned: Number(data.returned || 0),
    page: Number(data.page || 1),
  };
};
