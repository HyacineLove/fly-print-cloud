import React, { useEffect, useRef, useState } from 'react';
import { Alert, Button, Card, Result, Spin, Typography, message } from 'antd';
import { CheckCircleOutlined, FileOutlined } from '@ant-design/icons';
import { useSearchParams } from 'react-router-dom';
import { apiService } from '../../services/api';
import { buildApiUrl } from '../../config';

const { Title, Text, Paragraph } = Typography;

const ERROR_MESSAGES: Record<string, string> = {
  invalid_format: '二维码格式无效，请使用打印机页面生成的上传二维码',
  token_expired: '二维码已过期，请重新扫码获取新的上传链接',
  invalid_signature: '二维码签名校验失败，请重新扫码获取新的上传链接',
  token_already_used: '该二维码已使用过，不能重复上传，请重新扫码',
  missing_token: '缺少二维码参数，请重新扫码进入上传页面',
  file_type_not_allowed: '文件类型不支持，请上传常用图片、PDF、DOC 或 DOCX 文件',
  FILE_TOO_MANY_PAGES: '文档页数超过限制（最多 5 页），请拆分文件后再上传',
  FILE_TOO_LARGE: '文件大小超过限制（最多 10MB），请压缩或拆分后再上传',
  FILE_INVALID_TYPE: '文件类型不支持，请上传图片、PDF 或 Word 文档',
  '6003': '文件类型不支持，请上传图片、PDF 或 Word 文档',
  '6004': '文件大小超过 10MB，请压缩文件后再上传',
  '6005': '文档页数超过 5 页，请拆分文件后再上传',
  node_not_found: '打印节点已被删除，请重新扫码',
  node_disabled: '打印节点已被禁用',
  printer_not_found: '打印机已被删除，请重新扫码',
  printer_disabled: '打印机已被禁用',
  printer_not_belong_to_node: '打印机不属于该节点',
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

const TYPE_LIMIT_LABEL = '类型限制：常用图片、PDF、DOC/DOCX';
const UPLOAD_LIMIT_LABEL = '上传限制：<=10MB，文档类额外限制<=5页';

type PageState = 'verifying' | 'ready' | 'success' | 'invalid';
type UploadPhase = 'idle' | 'validating' | 'uploading';

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

const formatFileSize = (bytes: number): string => {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
};

const extractErrorDetails = (err: any) => {
  let rawMessage = '';
  let errorCode: string | number | undefined;

  if (err?.details?.message) {
    rawMessage = err.details.message;
  } else if (err?.message) {
    rawMessage = err.message;
  }

  if (err?.details?.error !== undefined && err.details.error !== null) {
    errorCode = err.details.error;
  } else if (err?.details?.code !== undefined && err.details.code !== null) {
    errorCode = String(err.details.code);
  }

  return { rawMessage, errorCode };
};

const PublicUpload: React.FC = () => {
  const [searchParams] = useSearchParams();
  const [token, setToken] = useState<string | null>(null);
  const [nodeId, setNodeId] = useState<string | null>(null);
  const [pageState, setPageState] = useState<PageState>('verifying');
  const [pageErrorMessage, setPageErrorMessage] = useState('');
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [uploadPhase, setUploadPhase] = useState<UploadPhase>('idle');
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const tokenParam = searchParams.get('token');
    const nodeIdParam = searchParams.get('node_id');

    if (!tokenParam) {
      setPageErrorMessage('缺少上传凭证，请确认您使用了正确的链接');
      setPageState('invalid');
      return;
    }

    void verifyToken(tokenParam, nodeIdParam);
  }, [searchParams]);

  const verifyToken = async (tokenParam: string, nodeIdParam: string | null) => {
    try {
      const response = await fetch(
        buildApiUrl(`/files/verify-upload-token?token=${encodeURIComponent(tokenParam)}`)
      );
      const result = await response.json();

      if (result.code === 200 && result.valid) {
        setToken(tokenParam);
        setNodeId(nodeIdParam || result.data.node_id);
        setPageState('ready');
        return;
      }

      const errorMsg = getFriendlyErrorMessage(
        result.message,
        result.error,
        '上传凭证校验失败，请重新扫码后重试'
      );
      setPageErrorMessage(errorMsg);
      setPageState('invalid');
    } catch {
      setPageErrorMessage('网络错误，请检查您的网络连接');
      setPageState('invalid');
    }
  };

  const openFilePicker = () => {
    if (uploadPhase !== 'idle') {
      return;
    }
    fileInputRef.current?.click();
  };

  const showUploadError = (errorMsg: string) => {
    message.open({
      key: 'public-upload-error',
      type: 'error',
      content: errorMsg,
      duration: 4,
      style: {
        marginTop: 16,
      },
    });
  };

  const handleUpload = async () => {
    if (!selectedFile || !token) {
      showUploadError('请先选择文件');
      return;
    }

    setUploadPhase('validating');

    try {
      const preflightResult = await apiService.preflightUpload(selectedFile, token);
      if (preflightResult.code !== 200 || (preflightResult as any).valid === false) {
        throw new Error(preflightResult.message || '预检失败');
      }

      setUploadPhase('uploading');
      const response = await apiService.uploadFile(selectedFile, token, nodeId || undefined);

      if (response.code === 200) {
        setPageState('success');
        setUploadPhase('idle');
        return;
      }

      const errorMsg = getFriendlyErrorMessage(
        response.message,
        (response as any).error,
        '上传失败，请稍后重试'
      );
      showUploadError(errorMsg);
    } catch (err: any) {
      const { rawMessage, errorCode } = extractErrorDetails(err);
      const errorMsg = getFriendlyErrorMessage(rawMessage, errorCode, '上传失败，请稍后重试');
      showUploadError(errorMsg);
    } finally {
      setUploadPhase('idle');
    }
  };

  const handleFileInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      setSelectedFile(file);
      e.target.value = '';
    }
  };

  const uploadButtonLabel =
    uploadPhase === 'validating'
      ? '校验文件中...'
      : uploadPhase === 'uploading'
        ? '上传文件中...'
        : '开始上传';

  if (pageState === 'verifying') {
    return <Spin fullscreen size="large" tip="正在验证上传凭证..." />;
  }

  if (pageState === 'invalid') {
    return (
      <div
        style={{
          display: 'flex',
          justifyContent: 'center',
          alignItems: 'center',
          minHeight: '100vh',
          background: '#f0f2f5',
          padding: 24,
        }}
      >
        <Alert
          message="访问失败"
          description={pageErrorMessage}
          type="error"
          showIcon
          style={{ maxWidth: 520, padding: 24 }}
        />
      </div>
    );
  }

  if (pageState === 'success') {
    return (
      <div
        style={{
          display: 'flex',
          justifyContent: 'center',
          alignItems: 'center',
          minHeight: '100vh',
          background: '#f0f2f5',
          padding: 24,
        }}
      >
        <Result
          status="success"
          title="上传成功"
          subTitle="文件已成功上传"
          icon={<CheckCircleOutlined style={{ color: '#52c41a' }} />}
          style={{ maxWidth: 520 }}
        />
      </div>
    );
  }

  return (
    <div
      style={{
        minHeight: '100vh',
        background: '#f0f2f5',
        padding: 24,
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
      }}
    >
      <div style={{ maxWidth: 640, width: '100%' }}>
        <Card style={{ textAlign: 'center' }}>
          <Title level={2}>文件上传</Title>
          <Paragraph type="secondary" style={{ fontSize: 12, margin: '0 0 16px 0' }}>
            {TYPE_LIMIT_LABEL}
          </Paragraph>
          <Paragraph style={{ fontSize: 14, margin: '0 0 8px 0' }}>
            <Text strong style={{ color: '#cf1322' }}>{UPLOAD_LIMIT_LABEL}</Text>
          </Paragraph>

          <input
            id="public-upload-input"
            ref={fileInputRef}
            type="file"
            style={{ display: 'none' }}
            onChange={handleFileInputChange}
          />

          <button
            type="button"
            onClick={openFilePicker}
            disabled={uploadPhase !== 'idle'}
            style={{
              width: '100%',
              marginTop: 24,
              border: '1px dashed #d9d9d9',
              borderRadius: 8,
              padding: selectedFile ? '28px 24px' : '40px 24px',
              background: uploadPhase === 'idle' ? '#fafafa' : '#f5f5f5',
              cursor: uploadPhase === 'idle' ? 'pointer' : 'not-allowed',
              textAlign: 'center',
              transition: 'border-color 0.2s ease, background 0.2s ease',
            }}
          >
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 12 }}>
              <FileOutlined style={{ fontSize: 48, color: '#1890ff' }} />
              {selectedFile ? (
                <>
                  <Text strong style={{ fontSize: 16, wordBreak: 'break-word' }}>
                    {selectedFile.name}
                  </Text>
                  <Text type="secondary">{formatFileSize(selectedFile.size)}</Text>
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    再次点击可重新选择文件
                  </Text>
                </>
              ) : (
                <>
                  <Text strong style={{ fontSize: 16 }}>点击选择文件</Text>
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    请选择需要打印的文件
                  </Text>
                </>
              )}
            </div>
          </button>

          {selectedFile ? (
            <Button
              type="primary"
              size="large"
              block
              loading={uploadPhase !== 'idle'}
              onClick={handleUpload}
              style={{ marginTop: 24, height: 48 }}
            >
              {uploadButtonLabel}
            </Button>
          ) : null}
        </Card>
      </div>
    </div>
  );
};

export default PublicUpload;
