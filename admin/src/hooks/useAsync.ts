import { useState, useCallback, useEffect } from 'react';
import { handleError } from '../utils/errorHandler';

interface UseAsyncState<T> {
  data: T | null;
  loading: boolean;
  error: Error | null;
}

interface UseAsyncReturn<T, Args extends any[]> extends UseAsyncState<T> {
  execute: (...args: Args) => Promise<T | null>;
  reset: () => void;
  setData: (data: T | null) => void;
}

/**
 * 通用异步操作 Hook
 * 自动处理加载状态、错误和数据
 */
export function useAsync<T, Args extends any[] = []>(
  asyncFunction: (...args: Args) => Promise<T>,
  immediate = false,
  onError?: (error: any) => void
): UseAsyncReturn<T, Args> {
  const [state, setState] = useState<UseAsyncState<T>>({
    data: null,
    loading: immediate,
    error: null,
  });

  const execute = useCallback(
    async (...args: Args): Promise<T | null> => {
      setState({ data: null, loading: true, error: null });

      try {
        const response = await asyncFunction(...args);
        setState({ data: response, loading: false, error: null });
        return response;
      } catch (error: any) {
        setState({ data: null, loading: false, error });
        
        // 调用自定义错误处理或使用默认错误处理
        if (onError) {
          onError(error);
        } else {
          handleError(error);
        }
        
        return null;
      }
    },
    [asyncFunction, onError]
  );

  const reset = useCallback(() => {
    setState({ data: null, loading: false, error: null });
  }, []);

  const setData = useCallback((data: T | null) => {
    setState(prev => ({ ...prev, data }));
  }, []);

  useEffect(() => {
    if (immediate) {
      execute(...([] as unknown as Args));
    }
  }, []);

  return {
    ...state,
    execute,
    reset,
    setData,
  };
}

/**
 * API 请求 Hook（基于 useAsync 的封装）
 */
export function useApiRequest<T, Args extends any[] = []>(
  apiFunction: (...args: Args) => Promise<{ data: T }>,
  immediate = false,
  onError?: (error: any) => void
) {
  return useAsync(
    async (...args: Args) => {
      const response = await apiFunction(...args);
      return response.data;
    },
    immediate,
    onError
  );
}

export default useAsync;
