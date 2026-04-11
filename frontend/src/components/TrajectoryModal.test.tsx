import { screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { renderWithApp } from '../test/renderWithApp';
import { TrajectoryModal } from './TrajectoryModal';

describe('TrajectoryModal', () => {
  it('renders read-only trajectory mode when detail is unsupported', () => {
    renderWithApp(
      <TrajectoryModal
        detailSupported={false}
        detailError=""
        detailLoading={false}
        isLoading={false}
        items={[{ id: 'traj-1', raw: {}, status: 'running', title: 'demo', updatedAt: '2026-04-11T00:00:00Z' }]}
        listError=""
        onClose={vi.fn()}
        onRefresh={vi.fn()}
        onResume={vi.fn()}
        onRollback={vi.fn()}
        onSelect={vi.fn()}
        resumeSupported={false}
        resumeError=""
        resumeLoadingId=""
        resumeSuccess=""
        rollbackSupported={false}
        rollbackError=""
        rollbackStepId=""
        rollbackSuccess=""
        selectedDetail={null}
        selectedId="traj-1"
        steps={[]}
      />,
    );

    expect(screen.queryByRole('button', { name: '恢复此会话' })).not.toBeInTheDocument();
    expect(screen.getByText('当前内核仅支持查看轨迹列表，不支持详情、恢复或回滚。')).toBeInTheDocument();
    expect(screen.getByText('当前内核未暴露轨迹详情能力，仅可查看轨迹列表。')).toBeInTheDocument();
  });
});
