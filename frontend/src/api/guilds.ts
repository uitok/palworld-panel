import { apiClient, handleRequest } from './client';
import { emptySummary, entityListQuery, mapSummary } from './entityList';
import { emptySaveIndexStatus, mapSaveIndexStatus } from './saveIndex';
import type { EntityListParams, EntityListResponse, Guild } from '../types';

const mapGuild = (raw: unknown): Guild => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    id: String(data.id || data.owner_player_uid || data.name || ''),
    name: String(data.name || 'Unknown Guild'),
    owner_player_uid: String(data.owner_player_uid || ''),
    members: Array.isArray(data.members)
      ? data.members.map((item) => {
          const member = (item && typeof item === 'object' ? item : {}) as Record<string, unknown>;
          return {
            player_uid: String(member.player_uid || ''),
            nickname: String(member.nickname || member.player_name || ''),
            last_online_time: member.last_online_time ? String(member.last_online_time) : undefined,
          };
        })
      : [],
    base_ids: Array.isArray(data.base_ids) ? data.base_ids.map(String) : [],
    online_member_count: Number(data.online_member_count || 0),
  };
};

const mapGuilds = (raw: unknown): Guild[] => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const list = Array.isArray(raw) ? raw : Array.isArray(data.guilds) ? data.guilds : [];
  return list.map(mapGuild);
};

const mapGuildsList = (raw: unknown): EntityListResponse<Guild> => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const items = mapGuilds(raw);
  return {
    items,
    status: data.status ? mapSaveIndexStatus(data.status) : emptySaveIndexStatus,
    summary: data.summary ? mapSummary(data.summary) : { ...emptySummary, total: items.length, returned: items.length },
  };
};

export const guildsApi = {
  getGuildsList: (params: EntityListParams = {}) =>
    handleRequest<unknown, EntityListResponse<Guild>>(
      () => apiClient.get(`/guilds${entityListQuery(params)}`),
      { items: [], status: emptySaveIndexStatus, summary: emptySummary },
      {
        map: mapGuildsList,
        quiet: true,
      },
    ),

  getGuilds: () =>
    handleRequest<unknown, Guild[]>(() => apiClient.get('/guilds'), [], {
      map: mapGuilds,
      quiet: true,
    }),
};
