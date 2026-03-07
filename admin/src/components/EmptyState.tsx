import React from 'react';
import { Empty, Button, EmptyProps } from 'antd';

interface EmptyStateProps extends EmptyProps {
  title?: string;
  description?: string;
  actionText?: string;
  onAction?: () => void;
  fullHeight?: boolean;
}

/**
 * 通用空状态组件
 */
const EmptyState: React.FC<EmptyStateProps> = ({
  title,
  description,
  actionText,
  onAction,
  fullHeight = false,
  image = Empty.PRESENTED_IMAGE_SIMPLE,
  ...restProps
}) => {
  return (
    <div
      style={{
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        padding: '50px 20px',
        minHeight: fullHeight ? '100vh' : 'auto',
      }}
    >
      <Empty
        image={image}
        description={
          <div>
            {title && <div style={{ fontSize: '16px', fontWeight: 500, marginBottom: '8px' }}>{title}</div>}
            {description && <div style={{ color: '#999' }}>{description}</div>}
          </div>
        }
        {...restProps}
      >
        {actionText && onAction && (
          <Button type="primary" onClick={onAction}>
            {actionText}
          </Button>
        )}
      </Empty>
    </div>
  );
};

export default EmptyState;
