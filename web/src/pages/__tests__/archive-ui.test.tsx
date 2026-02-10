import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ArchivePage } from '../ArchivePage';
import { api } from '../../lib/api';

vi.mock('../../lib/api', () => ({
  api: {
    getArchive: vi.fn(),
  },
}));

describe('ArchivePage UI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders page title and inputs', () => {
    render(<ArchivePage />);
    expect(screen.getByTestId('archive-page')).toBeInTheDocument();
    expect(screen.getByTestId('archive-filter-from')).toBeInTheDocument();
    expect(screen.getByTestId('archive-filter-to')).toBeInTheDocument();
    expect(screen.getByTestId('archive-filter-apply')).toBeInTheDocument();
  });

  it('fetches data on mount', async () => {
    (api.getArchive as any).mockResolvedValue({ txt2img: [], img2img: [] });
    render(<ArchivePage />);
    await waitFor(() => {
      expect(api.getArchive).toHaveBeenCalledWith({ from: undefined, to: undefined });
    });
  });

  it('shows empty state when no data', async () => {
    (api.getArchive as any).mockResolvedValue({ txt2img: [], img2img: [] });
    render(<ArchivePage />);
    await waitFor(() => {
      expect(screen.getByTestId('archive-empty')).toBeInTheDocument();
    });
  });

  it('shows data groups when data exists', async () => {
    const mockData = {
      txt2img: [
        { id: '1', domain: 'txt2img', archived_at: '2023-01-01T12:00:00Z', asset: { file_path: 'foo.png' } }
      ],
      img2img: [
        { id: '2', domain: 'img2img', archived_at: '2023-01-02T12:00:00Z', asset: { file_path: 'bar.png' } }
      ]
    };
    (api.getArchive as any).mockResolvedValue(mockData);

    render(<ArchivePage />);

    await waitFor(() => {
      expect(screen.getByTestId('archive-group-txt2img')).toBeInTheDocument();
      expect(screen.getByTestId('archive-group-img2img')).toBeInTheDocument();
      expect(screen.getByText('foo.png')).toBeInTheDocument();
      expect(screen.getByText('bar.png')).toBeInTheDocument();
    });
  });

  it('applies filters', async () => {
    (api.getArchive as any).mockResolvedValue({ txt2img: [], img2img: [] });
    render(<ArchivePage />);

    const fromInput = screen.getByTestId('archive-filter-from');
    const toInput = screen.getByTestId('archive-filter-to');
    const applyBtn = screen.getByTestId('archive-filter-apply');

    fireEvent.change(fromInput, { target: { value: '2023-01-01T10:00' } });
    fireEvent.change(toInput, { target: { value: '2023-01-02T10:00' } });
    fireEvent.click(applyBtn);

    await waitFor(() => {
      expect(api.getArchive).toHaveBeenLastCalledWith({
        from: new Date('2023-01-01T10:00').toISOString(),
        to: new Date('2023-01-02T10:00').toISOString()
      });
    });
  });

  it('validates date range (from > to)', async () => {
    render(<ArchivePage />);

    const fromInput = screen.getByTestId('archive-filter-from');
    const toInput = screen.getByTestId('archive-filter-to');
    const applyBtn = screen.getByTestId('archive-filter-apply');

    fireEvent.change(fromInput, { target: { value: '2023-02-01T10:00' } });
    fireEvent.change(toInput, { target: { value: '2023-01-01T10:00' } });
    fireEvent.click(applyBtn);

    await waitFor(() => {
        expect(screen.getByText('开始时间不能晚于结束时间')).toBeInTheDocument();
    });
    
    expect(api.getArchive).toHaveBeenCalledTimes(1);
  });
});
