import { apiClient, handleRequest } from './client';
import type {
  PalDefenderGiveItemsResult,
  PalDefenderGMInventory,
  PalDefenderGMPlayer,
  PalDefenderGMPlayers,
  PalDefenderGMStatus,
  PalDefenderInventoryContainer,
  PalDefenderInventorySlot,
  PalDefenderItemCatalog,
  PalDefenderItemGrant,
  PalDefenderMessageRequest,
  PalDefenderPunishmentRequest,
} from '../types';

const emptyLocation = { x: 0, y: 0, z: 0 };

const asRecord = (raw: unknown): Record<string, unknown> =>
  raw && typeof raw === 'object' && !Array.isArray(raw) ? (raw as Record<string, unknown>) : {};

const mapLocation = (raw: unknown) => {
  const data = asRecord(raw);
  return { x: Number(data.x || 0), y: Number(data.y || 0), z: Number(data.z || 0) };
};

const mapStatus = (raw: unknown): PalDefenderGMStatus => {
  const data = asRecord(raw);
  const version = asRecord(data.version);
  return {
    configured: Boolean(data.configured),
    available: Boolean(data.available),
    error: data.error ? String(data.error) : undefined,
    version:
      Object.keys(version).length > 0
        ? {
            Major: Number(version.Major || 0),
            Minor: Number(version.Minor || 0),
            Patch: Number(version.Patch || 0),
            Build: Number(version.Build || 0),
            Version: String(version.Version || ''),
            VersionLong: String(version.VersionLong || ''),
            Beta: Boolean(version.Beta),
          }
        : undefined,
  };
};

const mapPlayer = (raw: unknown): PalDefenderGMPlayer => {
  const data = asRecord(raw);
  return {
    Name: String(data.Name || '未知玩家'),
    IP: String(data.IP || ''),
    PlayerUID: String(data.PlayerUID || ''),
    UserId: String(data.UserId || ''),
    GuildName: String(data.GuildName || ''),
    GuildUUID: String(data.GuildUUID || ''),
    Status: String(data.Status || 'Unknown'),
    WorldLocation: data.WorldLocation ? mapLocation(data.WorldLocation) : { ...emptyLocation },
    MapLocation: data.MapLocation ? mapLocation(data.MapLocation) : { ...emptyLocation },
  };
};

const mapPlayers = (raw: unknown): PalDefenderGMPlayers => {
  const data = asRecord(raw);
  const meta = asRecord(data.Meta);
  const players = Array.isArray(data.Players) ? data.Players.map(mapPlayer) : [];
  return {
    Meta: {
      PlayerCount: Number(meta.PlayerCount ?? players.length),
      OnlineCount: Number(meta.OnlineCount ?? players.filter((player) => player.Status.toLowerCase() === 'online').length),
    },
    Players: players,
  };
};

const emptyContainer = (): PalDefenderInventoryContainer => ({
  Available: false,
  ContainerID: '',
  UsedSlots: 0,
  MaxSlots: 0,
  FreeSlots: 0,
  Slots: {},
});

const mapContainer = (raw: unknown): PalDefenderInventoryContainer => {
  const data = asRecord(raw);
  const slots: Record<string, PalDefenderInventorySlot> = {};
  for (const [slot, rawValue] of Object.entries(asRecord(data.Slots))) {
    const value = asRecord(rawValue);
    const itemID = String(value.ItemID || '').trim();
    if (!itemID) continue;
    slots[slot] = { ItemID: itemID, Count: Number(value.Count || 0) };
  }
  return {
    Available: Boolean(data.Available),
    ContainerID: String(data.ContainerID || ''),
    UsedSlots: Number(data.UsedSlots ?? Object.keys(slots).length),
    MaxSlots: Number(data.MaxSlots || 0),
    FreeSlots: Number(data.FreeSlots || 0),
    Slots: slots,
  };
};

const mapInventory = (raw: unknown): PalDefenderGMInventory => {
  const data = asRecord(raw);
  const meta = asRecord(data.Meta);
  const inventory = asRecord(data.Inventory);
  return {
    Meta: { PlayerUID: String(meta.PlayerUID || ''), Player: String(meta.Player || '') },
    Inventory: {
      Items: inventory.Items ? mapContainer(inventory.Items) : emptyContainer(),
      KeyItems: inventory.KeyItems ? mapContainer(inventory.KeyItems) : emptyContainer(),
      Weapons: inventory.Weapons ? mapContainer(inventory.Weapons) : emptyContainer(),
      Armor: inventory.Armor ? mapContainer(inventory.Armor) : emptyContainer(),
      Food: inventory.Food ? mapContainer(inventory.Food) : emptyContainer(),
      DropSlot: inventory.DropSlot ? mapContainer(inventory.DropSlot) : emptyContainer(),
    },
  };
};

const mapItemCatalog = (raw: unknown): PalDefenderItemCatalog => {
  const data = asRecord(raw);
  const items = Array.isArray(data.items)
    ? data.items.flatMap((rawItem) => {
        const item = asRecord(rawItem);
        const id = String(item.id || '').trim();
        const name = String(item.name || '').trim();
        if (!id || !name) return [];
        return [{ id, name, icon: item.icon ? String(item.icon) : undefined }];
      })
    : [];
  return { items, returned: Number(data.returned ?? items.length) };
};

const playerPath = (identifier: string) =>
  `/security/paldefender/gm/players/${encodeURIComponent(identifier)}`;

export const palDefenderGMApi = {
  status: () =>
    handleRequest<unknown, PalDefenderGMStatus>(
      () => apiClient.get('/security/paldefender/gm/status'),
      { configured: false, available: false },
      { map: mapStatus, quiet: true, fallbackOnError: false },
    ),

  players: () =>
    handleRequest<unknown, PalDefenderGMPlayers>(
      () => apiClient.get('/security/paldefender/gm/players'),
      { Meta: { PlayerCount: 0, OnlineCount: 0 }, Players: [] },
      { map: mapPlayers, quiet: true, fallbackOnError: false },
    ),

  items: (query = '', limit = 5000) => {
    const params = new URLSearchParams();
    if (query.trim()) params.set('q', query.trim());
    params.set('limit', String(limit));
    return handleRequest<unknown, PalDefenderItemCatalog>(
      () => apiClient.get(`/security/paldefender/gm/items?${params.toString()}`),
      { items: [], returned: 0 },
      { map: mapItemCatalog, quiet: true, fallbackOnError: false },
    );
  },

  inventory: (identifier: string) =>
    handleRequest<unknown, PalDefenderGMInventory>(
      () => apiClient.get(`${playerPath(identifier)}/inventory`),
      { Meta: { PlayerUID: '', Player: identifier }, Inventory: {
        Items: emptyContainer(), KeyItems: emptyContainer(), Weapons: emptyContainer(),
        Armor: emptyContainer(), Food: emptyContainer(), DropSlot: emptyContainer(),
      } },
      { map: mapInventory, quiet: true, fallbackOnError: false },
    ),

  giveItems: (identifier: string, items: PalDefenderItemGrant[]) =>
    handleRequest<unknown, PalDefenderGiveItemsResult>(
      () => apiClient.post(`${playerPath(identifier)}/items`, { Items: items }),
      { Granted: { Items: 0 } },
      { quiet: true, fallbackOnError: false },
    ),

  sendMessage: (identifier: string, request: PalDefenderMessageRequest) =>
    handleRequest<unknown, Record<string, unknown>>(
      () => apiClient.post(`${playerPath(identifier)}/message`, request),
      {},
      { quiet: true, fallbackOnError: false },
    ),

  broadcast: (message: string, alert = false) =>
    handleRequest<unknown, Record<string, unknown>>(
      () => apiClient.post('/security/paldefender/gm/broadcast', { message, alert }),
      {},
      { quiet: true, fallbackOnError: false },
    ),

  kick: (identifier: string, request: PalDefenderPunishmentRequest = {}) =>
    handleRequest<unknown, Record<string, unknown>>(
      () => apiClient.post(`${playerPath(identifier)}/kick`, request),
      {},
      { quiet: true, fallbackOnError: false },
    ),

  ban: (identifier: string, request: PalDefenderPunishmentRequest = {}) =>
    handleRequest<unknown, Record<string, unknown>>(
      () => apiClient.post(`${playerPath(identifier)}/ban`, request),
      {},
      { quiet: true, fallbackOnError: false },
    ),

  unban: (identifier: string, request: PalDefenderPunishmentRequest = {}) =>
    handleRequest<unknown, Record<string, unknown>>(
      () => apiClient.post(`${playerPath(identifier)}/unban`, request),
      {},
      { quiet: true, fallbackOnError: false },
    ),
};
