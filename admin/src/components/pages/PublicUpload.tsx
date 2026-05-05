import React, { useState, useEffect, useRef } from 'react';
import { Upload, message, Button, Card, Typography, Alert, Spin, Result } from 'antd';
import { InboxOutlined, FileOutlined, CheckCircleOutlined } from '@ant-design/icons';
import { useSearchParams } from 'react-router-dom';
import type { RcFile } from 'antd/es/upload';
import { apiService } from '../../services/api';
import { buildApiUrl } from '../../config';

const { Title, Text, Paragraph } = Typography;

// 错误码映射
const ERROR_MESSAGES: Record<string, string> = {
  // Token 相关错误
  invalid_format: '二维码格式无效，请使用打印机页面生成的上传二维码',
  token_expired: '二维码已过期，请重新扫码获取新的上传链接',
  invalid_signature: '二维码签名校验失败，请重新扫码获取新的上传链接',
  token_already_used: '该二维码已使用过，不能重复上传，请重新扫码',
  missing_token: '缺少二维码参数，请重新扫码进入上传页面',
  // 文件相关错误
  file_type_not_allowed: '文件类型不支持，请上传 PNG、JPG、BMP、GIF、TIFF、WEBP、PDF、DOC 或 DOCX 文件',
  FILE_TOO_MANY_PAGES: '文档页数超过限制（最多 5 页），请拆分文件后再上传',
  FILE_TOO_LARGE: '文件大小超过限制（最多 10MB），请压缩或拆分后再上传',
  FILE_INVALID_TYPE: '文件类型不支持，请上传图片、PDF 或 Word 文档',
  '6003': '文件类型不支持，请上传图片、PDF 或 Word 文档',
  '6004': '文件大小超过 10MB，请压缩文件后再上传',
  '6005': '文档页数超过 5 页，请拆分文件后再上传',
  // 节点相关错误
  node_not_found: '打印节点已被删除，请重新扫码',
  node_disabled: '打印节点已被禁用',
  // 打印机相关错误
  printer_not_found: '打印机已被删除，请重新扫码',
  printer_disabled: '打印机已被禁用',
  printer_not_belong_to_node: '打印机不属于该节点',
  // 通用错误
  unauthorized: '认证失败，请重新扫码进入上传页面',
};

const ENGLISH_ERROR_MESSAGES: Record<string, string> = {
  'Upload token is required': '缺少二维码参数，请重新扫码后再上传文件',
  'Upload token or OAuth2 authentication required': '二维码认证信息缺失，请重新扫码后再试',
  'No file uploaded': '未检测到上传文件，请先选择要打印的文件',
  'Failed to open uploaded file': '文件读取失败，请重新选择文件后重试',
  'Failed to validate file': '文件校验失败，请确认文件格式、大小和页数后重试',
  'Failed to save file': '文件保存失败，请稍后重试',
  'Failed to save file metadata': '文件信息保存失败，请稍后重试',
  'Invalid token format': '二维码格式无效，请重新扫码获取新的上传链接',
  'Token has expired': '二维码已过期，请重新扫码获取新的上传链接',
  'Invalid token signature': '二维码签名校验失败，请重新扫码获取新的上传链接',
  'Invalid token type': '二维码类型无效，请使用正确的上传二维码',
  'Token context mismatch': '二维码与当前打印任务不匹配，请重新扫码',
  'Token has already been used': '该二维码已使用过，请重新扫码获取新的上传链接',
  'Token has been revoked': '二维码已失效，请重新扫码获取新的上传链接',
  'Failed to verify token usage': '二维码校验失败，请稍后重试',
  'Failed to fetch': '网络连接失败，请检查网络后重试',
  'Network Error': '网络连接失败，请检查网络后重试',
};

const getFriendlyErrorMessage = (
  rawMessage?: string,
  errorCode?: string | number,
  fallback = '操作失败，请稍后重试'
): string => {
  const codeKey = errorCode !== undefined && errorCode !== null ? String(errorCode) : '';
  if (codeKey && ERROR_MESSAGES[codeKey]) {
    return ERROR_MESSAGES[codeKey];
  }

  const messageText = (rawMessage || '').trim();
  if (!messageText) {
    return fallback;
  }

  if (ERROR_MESSAGES[messageText]) {
    return ERROR_MESSAGES[messageText];
  }

  if (ENGLISH_ERROR_MESSAGES[messageText]) {
    return ENGLISH_ERROR_MESSAGES[messageText];
  }

  if (messageText.split('').every((char) => char.charCodeAt(0) <= 127)) {
    return fallback;
  }

  return messageText;
};

// 文件类型配置
// 前端不做严格限制，让用户可以选择任何文件，由后端验证
const ACCEPTED_FILE_TYPES = '*';  // 接受所有文件类型
const FILE_TYPE_LABEL = '常见图片格式/.docx/.doc/.pdf';

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
      const response = await fetch(
        buildApiUrl(`/files/verify-upload-token?token=${encodeURIComponent(tokenParam)}`)
      );
      const result = await response.json();
      
      if (result.code === 200 && result.valid) {
        setToken(tokenParam);
        setNodeId(nodeIdParam || result.data.node_id);
        setPageState('ready');
      } else {
        const errorMsg = getFriendlyErrorMessage(result.message, result.error, '上传凭证验证失败，请重新扫码后重试');
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
      const preflightResult = await apiService.preflightUpload(selectedFile, token);
      if (preflightResult.code !== 200 || (preflightResult as any).valid === false) {
        throw new Error(preflightResult.message || '预检失败');
      }

      const response = await apiService.uploadFile(selectedFile, token, nodeId || undefined);
      
      if (response.code === 200) {
        setPageState('success');
      } else {
        const errorMsg = getFriendlyErrorMessage(response.message, (response as any).error, '上传失败，请稍后重试');
        message.error(errorMsg);
        setErrorMessage(errorMsg);
        setPageState('error');
      }
    } catch (err: any) {
      let rawMessage = '';
      let errorCode: string | number | undefined;
      if (err.details?.message) {
        rawMessage = err.details.message;
      } else if (err.message) {
        rawMessage = err.message;
      }
      if (err.details?.error !== undefined && err.details?.error !== null) {
        errorCode = err.details.error;
      } else if (err.details?.code !== undefined && err.details?.code !== null) {
        errorCode = String(err.details.code);
      }
      const errorMsg = getFriendlyErrorMessage(rawMessage, errorCode, '上传失败，请稍后重试');
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
          <Paragraph type="secondary" style={{ whiteSpace: 'nowrap', marginBottom: 8 }}>
            上传文件后将自动进入打印流程
          </Paragraph>
          <Paragraph type="secondary" style={{ fontSize: '12px', whiteSpace: 'nowrap', margin: '0 0 6px 0' }}>
            支持格式：{FILE_TYPE_LABEL}
          </Paragraph>
          <Paragraph style={{ fontSize: '14px', whiteSpace: 'nowrap', margin: '0 0 6px 0' }}>
            上传限制：<Text strong style={{ color: '#cf1322' }}>≤ 10MB</Text> ｜ <Text strong style={{ color: '#cf1322' }}>≤ 5页</Text>
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
              <p className="ant-upload-text" style={{ whiteSpace: 'nowrap' }}>点击或拖拽文件到此处上传</p>
              <p className="ant-upload-hint" style={{ whiteSpace: 'nowrap' }}>
                可上传图片、PDF、Word 文档
              </p>
              <p className="ant-upload-hint" style={{ whiteSpace: 'nowrap' }}>
                上传限制：<Text strong style={{ color: '#cf1322' }}>≤ 10MB</Text> ｜ <Text strong style={{ color: '#cf1322' }}>≤ 5页</Text>
              </p>
              <p className="ant-upload-hint" style={{ whiteSpace: 'nowrap' }}>仅支持 A4 简历打印</p>
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
                  <Text type="secondary" style={{ fontSize: 12 }}>点击可重新选择文件，请确认内容清晰且方向正确</Text>
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
