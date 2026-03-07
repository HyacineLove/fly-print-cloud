import React, { useState, useEffect, useRef } from 'react';
import { Upload, message, Button, Card, Typography, Alert, Spin, Result } from 'antd';
import { InboxOutlined, FileOutlined, CheckCircleOutlined } from '@ant-design/icons';
import { useSearchParams } from 'react-router-dom';
import type { RcFile } from 'antd/es/upload';
import { apiService } from '../../services/api';

const { Title, Text, Paragraph } = Typography;

// 错误码映射
const ERROR_MESSAGES: Record<string, string> = {
  // Token 相关错误
  invalid_format: '上传凭证格式无效',
  token_expired: '上传凭证已过期，请重新扫码',
  invalid_signature: '上传凭证签名无效',
  token_already_used: '上传凭证已被使用',
  missing_token: '缺少上传凭证',
  // 文件相关错误
  file_type_not_allowed: '不支持该文件类型，仅支持：PNG、JPG、BMP、GIF、TIFF、WEBP、PDF、DOC、DOCX',
  FILE_TOO_MANY_PAGES: 'PDF文档页数超过限制（最多5页）',
  FILE_TOO_LARGE: '文件大小超过限制',
  FILE_INVALID_TYPE: '不支持的文件类型',
  // 节点相关错误
  node_not_found: '打印节点已被删除，请重新扫码',
  node_disabled: '打印节点已被禁用',
  // 打印机相关错误
  printer_not_found: '打印机已被删除，请重新扫码',
  printer_disabled: '打印机已被禁用',
  printer_not_belong_to_node: '打印机不属于该节点',
  // 通用错误
  unauthorized: '认证失败，请重新扫码',
};

// 文件类型配置
// 前端不做严格限制，让用户可以选择任何文件，由后端验证
const ACCEPTED_FILE_TYPES = '*';  // 接受所有文件类型
const FILE_TYPE_LABEL = 'PNG, JPG, BMP, GIF, TIFF, WEBP, PDF, DOC, DOCX';

// 文件图标映射
const getFileIcon = (fileName: string) => {
  return <FileOutlined style={{ fontSize: 48, color: '#1890ff' }} />;
};

// 格式化文件大小
const formatFileSize = (bytes: number): string => {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
};

const PublicUpload: React.FC = () => {
  const [searchParams] = useSearchParams();
  const [token, setToken] = useState<string | null>(null);
  const [nodeId, setNodeId] = useState<string | null>(null);
  
  // 页面状态：verifying | ready | uploading | success | error
  const [pageState, setPageState] = useState<'verifying' | 'ready' | 'uploading' | 'success' | 'error'>('verifying');
  const [errorMessage, setErrorMessage] = useState<string>('');
  
  // 文件选择
  const [selectedFile, setSelectedFile] = useState<RcFile | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // 页面加载时验证token
  useEffect(() => {
    const tokenParam = searchParams.get('token');
    const nodeIdParam = searchParams.get('node_id');
    const printerIdParam = searchParams.get('printer_id');
    
    if (!tokenParam) {
      setErrorMessage('缺少上传凭证，请确保您使用了正确的链接');
      setPageState('error');
      return;
    }

    // 轻量验证token（不消耗一次性token）
    verifyToken(tokenParam, nodeIdParam, printerIdParam);
  }, [searchParams]);



  const verifyToken = async (tokenParam: string, nodeIdParam: string | null, printerIdParam: string | null) => {
    try {
      const response = await fetch(`/api/v1/files/verify-upload-token?token=${encodeURIComponent(tokenParam)}`);
      const result = await response.json();
      
      if (result.code === 200 && result.valid) {
        setToken(tokenParam);
        setNodeId(nodeIdParam || result.data.node_id);
        setPageState('ready');
      } else {
        const errorMsg = ERROR_MESSAGES[result.error] || result.message || '上传凭证验证失败';
        setErrorMessage(errorMsg);
        setPageState('error');
      }
    } catch (err: any) {
      setErrorMessage('网络错误，请检查您的网络连接');
      setPageState('error');
    }
  };

  // 处理文件选择
  const handleFileSelect = (file: RcFile) => {
    setSelectedFile(file);
    return false; // 阻止自动上传
  };

  // 开始上传
  const handleUpload = async () => {
    if (!selectedFile || !token) {
      message.error('请先选择文件');
      return;
    }

    setPageState('uploading');
    
    try {
      const response = await apiService.uploadFile(selectedFile, token, nodeId || undefined);
      
      if (response.code === 200) {
        setPageState('success');
      } else {
        // 后端返回的错误信息
        const errorMsg = response.message || '上传失败';
        message.error(errorMsg);
        setErrorMessage(errorMsg);
        setPageState('error');
      }
    } catch (err: any) {
      // 错误处理：优先使用后端返回的 message，然后尝试从 details 中获取
      let errorMsg = '上传失败';
      
      // 1. 尝试从 details.message 获取后端返回的错误消息
      if (err.details?.message) {
        errorMsg = err.details.message;
      }
      // 2. 尝试从 details.error 获取错误码，并查找映射
      else if (err.details?.error) {
        const errorCode = err.details.error;
        errorMsg = ERROR_MESSAGES[errorCode] || errorCode;
      }
      // 3. 使用 err.message 作为后备
      else if (err.message) {
        errorMsg = err.message;
      }
      
      message.error(errorMsg);
      setErrorMessage(errorMsg);
      setPageState('error');
    }
  };

  // 重新选择文件
  const handleReselect = () => {
    fileInputRef.current?.click();
  };

  // 处理文件输入变化
  const handleFileInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      setSelectedFile(file as RcFile);
      // 清空 input value，允许重新选择相同文件
      e.target.value = '';
    }
  };

  // 验证中页面
  if (pageState === 'verifying') {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh', background: '#f0f2f5' }}>
        <Spin size="large" tip="正在验证上传凭证..." />
      </div>
    );
  }

  // 错误页面
  if (pageState === 'error') {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh', background: '#f0f2f5' }}>
        <Alert
          message="访问失败"
          description={errorMessage}
          type="error"
          showIcon
          style={{ maxWidth: 500, padding: '24px' }}
        />
      </div>
    );
  }

  // 上传成功页面
  if (pageState === 'success') {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh', background: '#f0f2f5' }}>
        <Result
          status="success"
          title="上传成功！"
          subTitle="文件已成功上传"
          icon={<CheckCircleOutlined style={{ color: '#52c41a' }} />}
          style={{ maxWidth: 500 }}
        />
      </div>
    );
  }

  // 主上传页面
  return (
    <div style={{ minHeight: '100vh', background: '#f0f2f5', padding: '24px', display: 'flex', justifyContent: 'center', alignItems: 'center' }}>
      <div style={{ maxWidth: 600, width: '100%' }}>
        <Card style={{ textAlign: 'center' }}>
          <Title level={2}>文件上传</Title>
          <Paragraph type="secondary">
            请选择要打印的文件
          </Paragraph>
          <Paragraph type="secondary" style={{ fontSize: '12px' }}>
            支持格式: {FILE_TYPE_LABEL}
          </Paragraph>

          {/* 隐藏的文件输入 */}
          <input
            ref={fileInputRef}
            type="file"
            accept={ACCEPTED_FILE_TYPES}
            style={{ display: 'none' }}
            onChange={handleFileInputChange}
          />

          {!selectedFile ? (
            // 未选择文件 - 显示选择区域
            <Upload.Dragger
              accept={ACCEPTED_FILE_TYPES}
              beforeUpload={handleFileSelect}
              showUploadList={false}
              disabled={pageState === 'uploading'}
              style={{ marginTop: 24 }}
            >
              <p className="ant-upload-drag-icon">
                <InboxOutlined />
              </p>
              <p className="ant-upload-text">点击或拖拽文件到此区域</p>
              <p className="ant-upload-hint">支持 {FILE_TYPE_LABEL}</p>
            </Upload.Dragger>
          ) : (
            // 已选择文件 - 显示文件预览
            <div style={{ marginTop: 24 }}>
              <div
                style={{
                  border: '1px dashed #d9d9d9',
                  borderRadius: '8px',
                  padding: '32px',
                  background: '#fafafa',
                  cursor: 'pointer',
                  transition: 'all 0.3s',
                }}
                onClick={handleReselect}
                onMouseEnter={(e) => {
                  e.currentTarget.style.borderColor = '#1890ff';
                  e.currentTarget.style.background = '#e6f7ff';
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.borderColor = '#d9d9d9';
                  e.currentTarget.style.background = '#fafafa';
                }}
              >
                <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 12 }}>
                  {getFileIcon(selectedFile.name)}
                  <Text strong style={{ fontSize: 16 }}>{selectedFile.name}</Text>
                  <Text type="secondary">{formatFileSize(selectedFile.size)}</Text>
                  <Text type="secondary" style={{ fontSize: 12 }}>点击可重新选择文件</Text>
                </div>
              </div>
              
              <Button
                type="primary"
                size="large"
                block
                loading={pageState === 'uploading'}
                onClick={handleUpload}
                style={{ marginTop: 24, height: 48 }}
              >
                {pageState === 'uploading' ? '上传中...' : '开始上传'}
              </Button>
            </div>
          )}
        </Card>
      </div>
    </div>
  );
};

export default PublicUpload;
