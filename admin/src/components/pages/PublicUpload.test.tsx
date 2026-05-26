import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { message } from 'antd';
import PublicUpload from './PublicUpload';
import { apiService } from '../../services/api';

jest.mock('../../services/api', () => ({
  apiService: {
    preflightUpload: jest.fn(),
    uploadFile: jest.fn(),
  },
}));

const mockedApiService = apiService as jest.Mocked<typeof apiService>;

describe('PublicUpload', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    jest.spyOn(message, 'open').mockImplementation(() => ({}) as any);
    global.fetch = jest.fn().mockResolvedValue({
      json: async () => ({
        code: 200,
        valid: true,
        data: {
          node_id: 'node-1',
        },
      }),
    }) as jest.Mock;
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('keeps the user on the upload page and shows a top toast when upload fails', async () => {
    mockedApiService.preflightUpload.mockRejectedValue({
      details: {
        message: 'Failed to save file',
      },
    });

    render(
      <MemoryRouter initialEntries={['/upload?token=test-token&node_id=node-1']}>
        <Routes>
          <Route path="/upload" element={<PublicUpload />} />
        </Routes>
      </MemoryRouter>
    );

    await screen.findByText('文件上传');

    const fileInput = document.querySelector('#public-upload-input') as HTMLInputElement;
    const file = new File(['resume'], 'resume.pdf', { type: 'application/pdf' });
    fireEvent.change(fileInput, { target: { files: [file] } });

    fireEvent.click(screen.getByRole('button', { name: '开始上传' }));

    await waitFor(() => {
      expect(message.open).toHaveBeenCalledWith(
        expect.objectContaining({
          key: 'public-upload-error',
          type: 'error',
          content: '文件保存失败，请稍后重试',
        })
      );
    });

    expect(screen.queryByText('上传未完成')).toBeNull();
    expect(screen.queryByRole('button', { name: '重新选择文件' })).toBeNull();
    expect(screen.queryByRole('button', { name: '清除当前文件' })).toBeNull();
    expect(screen.queryByRole('button', { name: '选择文件' })).toBeNull();
    expect(screen.queryByText('访问失败')).toBeNull();
  });

  it('renders the simplified wechat upload layout with minimal external guidance', async () => {
    render(
      <MemoryRouter initialEntries={['/upload?token=test-token&node_id=node-1']}>
        <Routes>
          <Route path="/upload" element={<PublicUpload />} />
        </Routes>
      </MemoryRouter>
    );

    await screen.findByText('文件上传');

    const fileInput = document.querySelector('#public-upload-input') as HTMLInputElement;
    expect(fileInput.accept).toBe('');
    expect(screen.getByText('类型限制：常用图片、PDF、DOC/DOCX')).toBeTruthy();
    expect(screen.getByText('上传限制：<=10MB，文档类额外限制<=5页')).toBeTruthy();
    expect(screen.queryByText('上传文件后将自动进入打印流程')).toBeNull();
    expect(screen.queryByText(/支持格式：/)).toBeNull();
    expect(screen.queryByRole('button', { name: '选择文件' })).toBeNull();
  });
});
