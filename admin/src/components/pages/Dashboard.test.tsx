import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { message } from 'antd';
import Dashboard from './Dashboard';

describe('Dashboard empty state', () => {
  beforeAll(() => {
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: (query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: jest.fn(),
        removeListener: jest.fn(),
        addEventListener: jest.fn(),
        removeEventListener: jest.fn(),
        dispatchEvent: jest.fn(),
      }),
    });
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('shows a normal empty state without an error toast', async () => {
    const errorToast = jest.spyOn(message, 'error').mockImplementation(() => undefined as any);
    global.fetch = jest.fn()
      .mockResolvedValueOnce({ ok: true, json: async () => ({ code: 200, data: { access_token: 'token' } }) })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          code: 200,
          data: {
            items: [], total: 0, page: 1, page_size: 20,
            summary: {
              fault_nodes: 0, online_nodes: 0, total_nodes: 0,
              fault_printers: 0, online_printers: 0, total_printers: 0,
            },
          },
        }),
      }) as jest.Mock;

    render(<MemoryRouter><Dashboard /></MemoryRouter>);

    expect(await screen.findByText('暂无当前告警')).toBeTruthy();
    await waitFor(() => expect(errorToast).not.toHaveBeenCalled());
  });
});
