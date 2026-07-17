import React, { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { LocateFixed, Map as MapIcon, Minus, Plus, Radio, RefreshCw, Search } from 'lucide-react';
import { getErrorMessage } from '../api/client';
import { saveIndexApi } from '../api/saveIndex';
import type { MapEntity, MapEntityType } from '../types';
import { SaveDataTabs } from '../components/ui/SaveDataTabs';
import { SaveIndexStatusBar } from '../components/ui/SaveIndexStatusBar';

type Bounds = { minX: number; minY: number; width: number; height: number };

const MAP_SIZE = 2048;
const MAP_BOUNDS: Bounds = { minX: 0, minY: 0, width: MAP_SIZE, height: MAP_SIZE };

const filterOptions: Array<{ type: MapEntityType; label: string; color: string }> = [
  { type: 'player', label: '玩家', color: '#38bdf8' },
  { type: 'base', label: '基地', color: '#f59e0b' },
  { type: 'pal', label: '帕鲁', color: '#b85443' },
  { type: 'map_object', label: '地图对象', color: '#94a3b8' },
];

const defaultFilters: Record<string, boolean> = {
  player: true,
  base: true,
  pal: false,
  map_object: false,
};

export const LiveMap: React.FC = () => {
  const queryClient = useQueryClient();
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [filters, setFilters] = useState(defaultFilters);
  const [search, setSearch] = useState('');
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [zoom, setZoom] = useState(1);

  const mapQuery = useQuery({
    queryKey: ['live-map'],
    queryFn: saveIndexApi.getMapEntities,
    refetchInterval: autoRefresh ? 2000 : false,
  });
  const rebuildMutation = useMutation({
    mutationFn: saveIndexApi.rebuild,
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['live-map'] }),
  });

  const allEntities = useMemo(() => mapQuery.data?.entities ?? [], [mapQuery.data]);
  const normalizedSearch = search.trim().toLowerCase();
  const visibleEntities = useMemo(
    () => allEntities.filter((entity) => {
      if (!filters[entity.type]) return false;
      if (!Number.isFinite(entity.x) || !Number.isFinite(entity.y)) return false;
      if (entity.x === 0 && entity.y === 0 && entity.z === 0) return false;
      return !normalizedSearch || `${entity.label} ${entity.id} ${entity.guild_name || ''}`.toLowerCase().includes(normalizedSearch);
    }),
    [allEntities, filters, normalizedSearch],
  );
  const onlinePlayers = allEntities.filter((entity) => entity.type === 'player' && entity.is_online);
  const selected = allEntities.find((entity) => entity.id === selectedID) ?? null;
  const bounds = scaleBounds(MAP_BOUNDS, zoom);
  const markerRadius = Math.max(bounds.width, bounds.height) / 140;
  const error = mapQuery.error ? getErrorMessage(mapQuery.error) : rebuildMutation.error ? getErrorMessage(rebuildMutation.error) : null;

  const toggleFilter = (type: string) => setFilters((current) => ({ ...current, [type]: !current[type] }));
  const resetView = () => {
    setZoom(1);
    setSearch('');
  };

  return (
    <div className="mx-auto flex w-full max-w-[1720px] flex-col gap-5 p-4 sm:p-6 lg:p-8">
      <SaveDataTabs />
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Metric label="在线玩家" value={mapQuery.data?.live.online_players ?? onlinePlayers.length} tone="blue" />
        <Metric label="地图实体" value={allEntities.length} tone="sky" />
        <Metric label="当前显示" value={visibleEntities.length} tone="terracotta" />
        <Metric label="实时来源" value={liveSourceLabel(mapQuery.data?.live.source, mapQuery.data?.live.available)} tone="amber" />
      </div>

      {error && <div className="rounded-2xl border border-rose-100 bg-rose-50 px-5 py-3 text-xs font-semibold text-rose-700">{error}</div>}

      <SaveIndexStatusBar
        status={mapQuery.data?.status ?? null}
        loading={mapQuery.isFetching}
        rebuilding={rebuildMutation.isPending}
        onRefresh={() => void mapQuery.refetch()}
        onRebuild={() => rebuildMutation.mutate()}
      />

      <section className="overflow-hidden rounded-3xl border border-slate-200 bg-white shadow-sm shadow-slate-200/60">
        <div className="flex flex-col gap-4 border-b border-slate-200 bg-slate-50/70 px-5 py-4 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h3 className="flex items-center gap-2 text-sm font-bold text-slate-800"><MapIcon size={17} className="text-sky-600" />Palpagos 实时地图</h3>
            <p className="mt-1 text-[11px] font-medium text-slate-500">在线玩家每 2 秒更新；其他对象来自最近一次存档索引。坐标按 Palworld 社区验证公式投影到游戏地图。</p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <label className="relative min-w-48 flex-1 lg:flex-none">
              <Search size={13} className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
              <input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="搜索玩家、基地或 ID" className="w-full rounded-xl border border-slate-200 bg-white py-2 pl-8 pr-3 text-xs font-semibold text-slate-700 outline-none placeholder:text-slate-400 focus:border-sky-500" />
            </label>
            <button type="button" onClick={() => setAutoRefresh((value) => !value)} className={`inline-flex items-center gap-2 rounded-xl border px-3 py-2 text-xs font-bold ${autoRefresh ? 'border-sky-200 bg-sky-50 text-sky-700' : 'border-slate-200 bg-white text-slate-500'}`}>
              <Radio size={13} className={autoRefresh ? 'animate-pulse' : ''} />{autoRefresh ? '自动刷新' : '已暂停'}
            </button>
            <button type="button" onClick={() => void mapQuery.refetch()} className="rounded-xl border border-slate-200 bg-white p-2 text-slate-600 hover:bg-slate-50" aria-label="刷新地图"><RefreshCw size={14} className={mapQuery.isFetching ? 'animate-spin' : ''} /></button>
          </div>
        </div>

        <div className="flex flex-wrap gap-2 border-b border-slate-200 px-5 py-3">
          {filterOptions.map((option) => (
            <button key={option.type} type="button" onClick={() => toggleFilter(option.type)} className={`inline-flex items-center gap-2 rounded-lg border px-3 py-1.5 text-[11px] font-bold ${filters[option.type] ? 'border-sky-200 bg-sky-50 text-sky-800' : 'border-slate-200 bg-white text-slate-500'}`}>
              <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: option.color }} />{option.label}
            </button>
          ))}
        </div>

        <div className="grid min-h-[620px] xl:grid-cols-[minmax(0,1fr)_300px]">
          <div className="relative min-h-[520px] overflow-hidden border-b border-slate-200 bg-slate-100 xl:border-b-0 xl:border-r">
            <svg viewBox={`${bounds.minX} ${bounds.minY} ${bounds.width} ${bounds.height}`} className="h-full min-h-[520px] w-full" role="img" aria-label="Palworld 实时坐标地图">
              <image href="/assets/maps/palworld-map.webp" x="0" y="0" width={MAP_SIZE} height={MAP_SIZE} preserveAspectRatio="xMidYMid meet" />
              {visibleEntities.map((entity) => (
                <MapMarker key={`${entity.type}:${entity.id}`} entity={entity} radius={markerRadius} selected={selected?.id === entity.id} onSelect={() => setSelectedID(entity.id)} />
              ))}
            </svg>
            <div className="absolute bottom-4 left-4 flex items-center gap-1 rounded-xl border border-slate-200 bg-white/90 p-1 shadow-lg backdrop-blur">
              <button type="button" onClick={() => setZoom((value) => Math.min(4, value * 1.35))} className="rounded-lg p-2 text-slate-600 hover:bg-slate-100" aria-label="放大"><Plus size={14} /></button>
              <button type="button" onClick={() => setZoom((value) => Math.max(1, value / 1.35))} className="rounded-lg p-2 text-slate-600 hover:bg-slate-100" aria-label="缩小"><Minus size={14} /></button>
              <button type="button" onClick={resetView} className="rounded-lg p-2 text-slate-600 hover:bg-slate-100" aria-label="适应全部"><LocateFixed size={14} /></button>
            </div>
            {visibleEntities.length === 0 && <div className="pointer-events-none absolute inset-x-0 bottom-5 flex justify-center"><span className="rounded-full border border-white/70 bg-white/85 px-4 py-2 text-xs font-semibold text-slate-600 shadow-sm backdrop-blur">当前筛选条件下没有可显示的坐标</span></div>}
          </div>

          <aside className="flex min-h-0 flex-col bg-slate-50/70 p-4">
            <div className="rounded-2xl border border-slate-200 bg-white p-4">
              <p className="text-[10px] font-bold uppercase tracking-wider text-slate-400">选中对象</p>
              {selected ? <EntityDetails entity={selected} /> : <p className="mt-3 text-xs font-semibold leading-5 text-slate-500">点击地图标记查看名称、坐标和实时状态。</p>}
            </div>
            <div className="mt-4 min-h-0 flex-1">
              <div className="mb-2 flex items-center justify-between"><p className="text-xs font-bold text-slate-700">在线玩家</p><span className="text-[10px] font-bold text-sky-700">{onlinePlayers.length}</span></div>
              <div className="flex max-h-[390px] flex-col gap-2 overflow-y-auto pr-1">
                {onlinePlayers.map((player) => (
                  <button key={player.id} type="button" onClick={() => setSelectedID(player.id)} className="flex items-center gap-3 rounded-xl border border-slate-200 bg-white px-3 py-2 text-left hover:bg-sky-50">
                    <span className="relative flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-sky-500/15 text-[10px] font-bold text-sky-300"><span className="absolute right-0 top-0 h-2.5 w-2.5 rounded-full border-2 border-slate-950 bg-sky-400" />{player.label.slice(0, 2).toUpperCase()}</span>
                    <span className="min-w-0"><span className="block truncate text-xs font-bold text-slate-700">{player.label}</span><span className="mt-0.5 block truncate font-mono text-[9px] text-slate-500">{formatCoordinates(player)}</span></span>
                  </button>
                ))}
                {onlinePlayers.length === 0 && <p className="rounded-xl border border-dashed border-slate-200 bg-white/70 px-3 py-5 text-center text-[11px] font-semibold text-slate-500">暂无在线玩家位置</p>}
              </div>
            </div>
          </aside>
        </div>
      </section>
    </div>
  );
};

const MapMarker: React.FC<{ entity: MapEntity; radius: number; selected: boolean; onSelect: () => void }> = ({ entity, radius, selected, onSelect }) => {
  const color = markerColor(entity);
  const position = projectWorldToMap(entity.x, entity.y);
  const { x, y } = position;
  const size = entity.type === 'base' ? radius * 1.3 : entity.type === 'player' ? radius : radius * 0.72;
  return (
    <g onClick={onSelect} className="cursor-pointer" role="button" aria-label={`${entity.label} ${formatCoordinates(entity)}`}>
      {entity.is_online && <circle cx={x} cy={y} r={size * 2.2} fill={color} fillOpacity="0.12" stroke={color} strokeOpacity="0.35" strokeWidth={radius / 7} />}
      {entity.type === 'base' ? <rect x={x - size} y={y - size} width={size * 2} height={size * 2} rx={size * 0.2} transform={`rotate(45 ${x} ${y})`} fill={color} stroke={selected ? '#fff' : '#34495e'} strokeWidth={selected ? radius / 3 : radius / 6} /> : <circle cx={x} cy={y} r={selected ? size * 1.25 : size} fill={color} stroke={selected ? '#fff' : '#34495e'} strokeWidth={selected ? radius / 3 : radius / 6} />}
      {(entity.is_online || entity.type === 'base' || selected) && <text x={x + size * 1.5} y={y - size * 1.3} fill="#fff" fontSize={radius * 1.15} fontWeight="700" paintOrder="stroke" stroke="#34495e" strokeWidth={radius / 3}>{entity.label}</text>}
    </g>
  );
};

const EntityDetails: React.FC<{ entity: MapEntity }> = ({ entity }) => (
  <div className="mt-3">
    <div className="flex items-center gap-2"><span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: markerColor(entity) }} /><p className="truncate text-sm font-bold text-slate-800">{entity.label}</p></div>
    <p className="mt-2 break-all font-mono text-[9px] text-slate-500">{entity.id}</p>
    <div className="mt-3 grid grid-cols-2 gap-2 text-[10px] font-semibold text-slate-400"><span>类型：{entityTypeLabel(entity.type)}</span><span>来源：{entity.live ? '实时' : '存档'}</span><span className="col-span-2 font-mono">坐标：{formatCoordinates(entity)}</span>{entity.guild_name && <span className="col-span-2 truncate">公会：{entity.guild_name}</span>}{entity.level != null && <span>等级：Lv.{entity.level}</span>}{entity.ping != null && <span>Ping：{entity.ping} ms</span>}</div>
  </div>
);

const Metric: React.FC<{ label: string; value: string | number; tone: 'blue' | 'sky' | 'terracotta' | 'amber' }> = ({ label, value, tone }) => {
  const colors = { blue: 'bg-blue-500', sky: 'bg-sky-500', terracotta: 'bg-rose-500', amber: 'bg-amber-500' };
  return <div className="rounded-2xl border border-slate-100 bg-white px-4 py-3 shadow-sm"><p className="text-[10px] font-bold uppercase tracking-wider text-slate-400">{label}</p><div className="mt-1 flex items-center gap-2"><span className={`h-2 w-2 rounded-full ${colors[tone]}`} /><p className="truncate text-sm font-bold text-slate-800">{value}</p></div></div>;
};

const scaleBounds = (bounds: Bounds, zoom: number): Bounds => {
  const width = bounds.width / zoom;
  const height = bounds.height / zoom;
  return { minX: bounds.minX + (bounds.width - width) / 2, minY: bounds.minY + (bounds.height - height) / 2, width, height };
};

// Coordinate conversion follows fa0311/palworld-map, which maps Palworld
// REST/save world coordinates onto the 256-unit CRS used by the game map.
const projectWorldToMap = (worldX: number, worldY: number) => {
  const ratio = 458.355;
  const mapRatio = 7.8;
  const leafletSize = 256;
  const adjustedX = worldX + 122500;
  const adjustedY = worldY - 158100;
  const gameX = adjustedX / ratio + (adjustedX > 0 ? 0 : 1);
  const gameY = adjustedY / ratio + (adjustedY > 0 ? 0 : 1);
  const markerLatitude = (gameX - (gameX > 0 ? 0 : 1)) / mapRatio - leafletSize / 2;
  const markerLongitude = (gameY - (gameY > 0 ? 0 : 1)) / mapRatio + leafletSize / 2;
  return {
    x: (markerLongitude / leafletSize) * MAP_SIZE,
    y: (-markerLatitude / leafletSize) * MAP_SIZE,
  };
};

const markerColor = (entity: MapEntity) => entity.type === 'player' ? (entity.is_online ? '#2f73b7' : '#6499bd') : entity.type === 'base' ? '#f59e0b' : entity.type === 'pal' ? '#b85443' : '#94a3b8';
const entityTypeLabel = (type: string) => ({ player: '玩家', base: '基地', pal: '帕鲁', map_object: '地图对象' }[type] || type);
const formatCoordinates = (entity: MapEntity) => `${entity.x.toFixed(0)}, ${entity.y.toFixed(0)}, ${entity.z.toFixed(0)}`;
const liveSourceLabel = (source?: string, available?: boolean) => !available ? '仅存档' : source === 'paldefender' ? 'PalDefender' : source === 'palworld_rest' ? '官方 REST' : source || '实时';
