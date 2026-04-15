import React, { Component, ErrorInfo, ReactNode } from 'react';
import { Result, Button } from 'antd';

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
}

interface State {
  hasError: boolean;
  error?: Error;
  errorInfo?: ErrorInfo;
}

/**
 * 全局错误边界组件
 * 捕获React组件树中的JavaScript错误，记录错误并显示降级UI
 */
class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = {
      hasError: false,
    };
  }

  static getDerivedStateFromError(error: Error): State {
    // 更新 state 使下一次渲染能够显示降级后的 UI
    return {
      hasError: true,
      error,
    };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    // 记录错误到错误报告服务
    console.error('Error Boundary caught an error:', error, errorInfo);
    
    this.setState({
      error,
      errorInfo,
    });

    // TODO: 可以在这里发送错误日志到服务器
    // logErrorToService(error, errorInfo);
  }

  handleReset = (): void => {
    this.setState({
      hasError: false,
      error: undefined,
      errorInfo: undefined,
    });
  };

  render(): ReactNode {
    if (this.state.hasError) {
      // 如果提供了自定义降级UI
      if (this.props.fallback) {
        return this.props.fallback;
      }

      // 默认降级UI
      return (
        <div style={{ 
          display: 'flex', 
          justifyContent: 'center', 
          alignItems: 'center', 
          minHeight: '100vh',
          padding: '20px'
        }}>
          <Result
            status="error"
            title="应用程序错误"
            subTitle="抱歉，应用程序遇到了一个错误。请尝试刷新页面或联系管理员。"
            extra={[
              <Button type="primary" key="refresh" onClick={() => window.location.reload()}>
                刷新页面
              </Button>,
              <Button key="reset" onClick={this.handleReset}>
                重试
              </Button>,
            ]}
          >
            {process.env.NODE_ENV === 'development' && this.state.error && (
              <div style={{ 
                textAlign: 'left', 
                background: '#f5f5f5', 
                padding: '16px', 
                borderRadius: '4px',
                maxWidth: '800px',
                margin: '20px auto'
              }}>
                <h4 style={{ marginBottom: '8px' }}>错误详情（开发模式）：</h4>
                <pre style={{ 
                  whiteSpace: 'pre-wrap', 
                  wordBreak: 'break-word',
                  fontSize: '12px',
                  color: '#d32f2f'
                }}>
                  {this.state.error.toString()}
                  {this.state.errorInfo?.componentStack}
                </pre>
              </div>
            )}
          </Result>
        </div>
      );
    }

    return this.props.children;
  }
}

export default ErrorBoundary;
