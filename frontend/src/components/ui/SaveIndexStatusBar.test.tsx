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
});
