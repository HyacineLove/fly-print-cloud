# 前端错误处理使用指南

## 概述

本项目已集成完整的错误处理机制，包括全局错误边界、统一的错误处理工具和便捷的 Hooks。

## 组件说明

### 1. ErrorBoundary - 全局错误边界

捕获 React 组件树中的 JavaScript 错误，显示降级 UI。

```tsx
import ErrorBoundary from './components/ErrorBoundary';

<ErrorBoundary>
  <YourApp />
</ErrorBoundary>
```

### 2. ErrorHandler - 错误处理工具类

统一的错误处理方法。

```tsx
import { handleError, showSuccess, confirmAction } from './utils/errorHandler';

// 处理 API 错误
try {
  await apiService.post('/endpoint', data);
  showSuccess('操作成功！');
} catch (error) {
  handleError(error); // 自动显示友好的错误消息
}

// 确认操作
const confirmed = await confirmAction(
  '确认删除',
  '删除后无法恢复，是否继续？',
  async () => {
    await apiService.delete(`/items/${id}`);
  }
);
```

### 3. useAsync Hook - 异步操作管理

简化异步操作和状态管理。

```tsx
import { useAsync } from './hooks/useAsync';

function MyComponent() {
  const { data, loading, error, execute } = useAsync(
    async (id: string) => {
      const response = await apiService.get(`/items/${id}`);
      return response.data;
    }
  );

  useEffect(() => {
    execute('123'); // 执行异步操作
  }, []);

  if (loading) return <Loading />;
  if (error) return <div>加载失败</div>;
  
  return <div>{JSON.stringify(data)}</div>;
}
```

### 4. useApiRequest Hook - API 请求封装

基于 useAsync 的 API 请求专用 Hook。

```tsx
import { useApiRequest } from './hooks/useAsync';

function UserList() {
  const { data: users, loading, execute } = useApiRequest(
    () => apiService.get('/users'),
    true // 立即执行
  );

  if (loading) return <Loading />;
  
  return (
    <div>
      {users?.map(user => <div key={user.id}>{user.name}</div>)}
    </div>
  );
}
```

### 5. Loading 组件 - 加载状态

```tsx
import Loading from './components/Loading';

// 普通加载
<Loading tip="加载中..." />

// 全屏加载
<Loading fullscreen tip="初始化应用..." />
```

### 6. EmptyState 组件 - 空状态

```tsx
import EmptyState from './components/EmptyState';

<EmptyState
  title="暂无数据"
  description="还没有任何记录"
  actionText="创建新记录"
  onAction={() => navigate('/create')}
/>
```

## 最佳实践

### 1. API 请求错误处理

```tsx
// ✅ 推荐：使用 try-catch + handleError
try {
  const response = await apiService.post('/users', userData);
  showSuccess('用户创建成功');
  return response.data;
} catch (error) {
  handleError(error); // 自动处理并显示错误
  return null;
}

// ✅ 推荐：使用 useAsync Hook
const { execute, loading } = useAsync(
  async () => apiService.post('/users', userData)
);

const handleSubmit = async () => {
  const result = await execute();
  if (result) {
    showSuccess('用户创建成功');
  }
};
```

### 2. 表单验证错误

```tsx
import { handleValidationError } from './utils/errorHandler';

try {
  await apiService.post('/users', formData);
} catch (error) {
  // 特殊处理表单验证错误
  handleValidationError(error);
}
```

### 3. 删除确认

```tsx
const handleDelete = async (id: string) => {
  const confirmed = await confirmAction(
    '确认删除',
    '此操作无法撤销',
    async () => {
      await apiService.delete(`/users/${id}`);
      showSuccess('删除成功');
      // 刷新列表
      loadUsers();
    }
  );
};
```

### 4. 页面级错误边界

```tsx
function MyPage() {
  return (
    <ErrorBoundary fallback={<div>页面加载失败</div>}>
      <PageContent />
    </ErrorBoundary>
  );
}
```

## 错误码映射

系统自动将 HTTP 状态码映射为友好的中文消息：

- 400: 请求参数错误
- 401: 未授权，请重新登录
- 403: 没有权限访问
- 404: 请求的资源不存在
- 429: 请求过于频繁
- 500: 服务器内部错误
- 503: 服务暂时不可用

## 自定义错误消息

```tsx
// 使用自定义消息覆盖默认错误消息
try {
  await apiService.delete(`/users/${id}`);
} catch (error) {
  handleError(error, '删除用户失败，请稍后重试');
}
```

## 注意事项

1. **ErrorBoundary** 只能捕获组件渲染期间的错误，不能捕获：
   - 事件处理器中的错误（需要手动 try-catch）
   - 异步代码中的错误
   - 服务器端渲染的错误

2. **401 错误** 会自动重定向到登录页面

3. 所有错误都会在控制台输出详细信息，便于调试

4. 生产环境不会显示错误堆栈信息
