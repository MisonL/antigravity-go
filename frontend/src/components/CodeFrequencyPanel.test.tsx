import { fireEvent, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { renderWithApp } from '../test/renderWithApp';
import { CodeFrequencyPanel } from './CodeFrequencyPanel';

describe('CodeFrequencyPanel', () => {
  it('renders summary metrics and triggers refresh', () => {
    const onRefresh = vi.fn();

    renderWithApp(
      <CodeFrequencyPanel
        loading={false}
        onRefresh={onRefresh}
        summary={{
          code_frequency: [
            {
              lines_added: 12,
              lines_deleted: 3,
              num_commits: 2,
              record_end_time: '2026-04-11T11:00:00Z',
              record_start_time: '2026-04-11T10:00:00Z',
            },
            {
              lines_added: 20,
              lines_deleted: 5,
              num_commits: 1,
              record_end_time: '2026-04-11T12:00:00Z',
              record_start_time: '2026-04-11T11:00:00Z',
            },
          ],
          generated_at: '2026-04-11T12:00:00Z',
          repo_uri: 'file:///tmp/project',
          workspace_root: '/tmp/project',
        }}
      />,
    );

    expect(screen.getByText('代码频率')).toBeInTheDocument();
    expect(screen.getByText('+32')).toBeInTheDocument();
    expect(screen.getByText('-8')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '刷新代码频率' }));
    expect(onRefresh).toHaveBeenCalledTimes(1);
  });
});
