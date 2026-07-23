import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { SaveIndexStatusBar } from './SaveIndexStatusBar';

describe('SaveIndexStatusBar', () => {
  it('keeps verbose player-save warnings collapsed behind a compact summary', () => {
    render(<SaveIndexStatusBar status={{
      enabled: true,
      state: 'ready',
      stale: false,
      source_path: 'Level.sav',
      updated_at: '2026-07-20T00:55:14Z',
      duration_ms: 10,
      parser: 'palpanel-sav-cli-go',
      counts: { players: 15, guilds: 8, bases: 29, pals: 3548, containers: 0, map_entities: 0 },
      warnings: [
        'player save warning: first.sav could not be parsed: utf16 fstring is too large',
        'player save warning: second.sav could not be parsed: ascii fstring is too large',
      ],
    }} onRefresh={vi.fn()} onRebuild={vi.fn()} />);

    expect(screen.getByText('2 个玩家存档解析失败')).toBeInTheDocument();
    expect(screen.getByText(/玩家 15 · 公会 8 · 基地 29 · 帕鲁 3548/)).toBeInTheDocument();
    const details = screen.getByText('查看解析警告详情（2）').closest('details');
    expect(details).not.toHaveAttribute('open');
    fireEvent.click(screen.getByText('查看解析警告详情（2）'));
    expect(details).toHaveAttribute('open');
  });

  it('shows stale cached data instead of a fatal parse failure', () => {
    render(<SaveIndexStatusBar status={{
      enabled: true,
      state: 'error',
      stale: true,
      source_path: 'Level.sav',
      updated_at: '2026-07-20T00:55:14Z',
      duration_ms: 10,
      parser: 'palpanel-sav-cli-go',
      error: 'save indexer failed; inspect the sav-cli text logs',
      error_code: 'parser_incompatible',
      counts: { players: 15, guilds: 8, bases: 29, pals: 3548, containers: 0, map_entities: 0 },
      warnings: ['returning stale save index after rebuild failed'],
    }} onRefresh={vi.fn()} onRebuild={vi.fn()} />);

    expect(screen.getByText('存档索引已过期')).toBeInTheDocument();
    expect(screen.queryByText('存档解析失败')).not.toBeInTheDocument();
    expect(screen.getByText('parser_incompatible')).toBeInTheDocument();
  });

  it('maps a fatal error code to an actionable Chinese hint', () => {
    render(<SaveIndexStatusBar status={{
      enabled: true,
      state: 'error',
      stale: false,
      source_path: 'Level.sav',
      updated_at: '2026-07-20T00:55:14Z',
      duration_ms: 10,
      parser: 'palpanel-sav-cli-go',
      error: 'save indexer failed (parser_incompatible); inspect the sav-cli text logs',
      error_code: 'parser_incompatible',
      error_detail: 'unknown property type StructProperty',
      oodle_available: true,
      counts: { players: 0, guilds: 0, bases: 0, pals: 0, containers: 0, map_entities: 0 },
      warnings: [],
    }} onRefresh={vi.fn()} onRebuild={vi.fn()} />);

    expect(screen.getByText(/存档格式不兼容/)).toBeInTheDocument();
    expect(screen.getByText('parser_incompatible')).toBeInTheDocument();
    expect(screen.getByText(/unknown property type StructProperty/)).toBeInTheDocument();
  });

  it.each([
    ['save_indexer_unavailable', /sav-cli 存档解析服务不可用/],
    ['save_index_timeout', /sav-cli 存档解析超时/],
  ])('distinguishes %s errors', (errorCode, message) => {
    render(<SaveIndexStatusBar status={{
      enabled: true,
      state: 'error',
      stale: false,
      source_path: 'Level.sav',
      updated_at: '',
      duration_ms: 0,
      error_code: errorCode,
      counts: { players: 0, guilds: 0, bases: 0, pals: 0, containers: 0, map_entities: 0 },
      warnings: [],
    }} onRefresh={vi.fn()} onRebuild={vi.fn()} />);

    expect(screen.getByText(message)).toBeInTheDocument();
  });

  it('reports missing Oodle support separately from parser incompatibility', () => {
    render(<SaveIndexStatusBar status={{
      enabled: true,
      state: 'error',
      stale: false,
      source_path: 'Level.sav',
      updated_at: '',
      duration_ms: 0,
      error_code: 'parser_incompatible',
      oodle_available: false,
      counts: { players: 0, guilds: 0, bases: 0, pals: 0, containers: 0, map_entities: 0 },
      warnings: [],
    }} onRefresh={vi.fn()} onRebuild={vi.fn()} />);

    expect(screen.getByText(/缺少 Oodle 解压能力/)).toBeInTheDocument();
    expect(screen.getByText('parser_incompatible')).toBeInTheDocument();
  });
});
