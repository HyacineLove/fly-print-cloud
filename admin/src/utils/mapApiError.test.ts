import { mapApiError } from './mapApiError';

describe('mapApiError', () => {
  it('maps OAuth invalid_grant jargon', () => {
    expect(
      mapApiError({
        error: 'invalid_grant',
        error_description: 'invalid_grant: invalid username or password',
      }),
    ).toBe('用户名或密码错误');
  });

  it('maps business settings validation phrases', () => {
    expect(mapApiError({ message: 'upload_max_size_bytes must be greater than 0' })).toBe(
      '上传大小上限必须大于 0',
    );
  });

  it('keeps already-Chinese messages', () => {
    expect(mapApiError({ message: '打印机不存在' })).toBe('打印机不存在');
  });

  it('falls back for unknown technical strings', () => {
    expect(mapApiError({ message: 'some_unknown_snake_case' }, '操作失败')).toBe('操作失败');
  });
});
