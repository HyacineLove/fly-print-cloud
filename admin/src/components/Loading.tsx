import React from 'react';
import { Spin, SpinProps } from 'antd';

interface LoadingProps extends SpinProps {
  fullscreen?: boolean;
  tip?: string;
}

/**
 * 通用加载组件
 */
const Loading: React.FC<LoadingProps> = ({ 
  fullscreen = false, 
  tip = '加载中...',
  size = 'large',
  ...restProps 
}) => {
  if (fullscreen) {
    return (
      <div
        style={{
          display: 'flex',
          justifyContent: 'center',
          alignItems: 'center',
          minHeight: '100vh',
          background: 'rgba(255, 255, 255, 0.9)',
        }}
      >
        <Spin size={size} tip={tip} {...restProps} />
      </div>
    );
  }

  return (
    <div
      style={{
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        padding: '50px 20px',
      }}
    >
      <Spin size={size} tip={tip} {...restProps} />
    </div>
  );
};

export default Loading;
