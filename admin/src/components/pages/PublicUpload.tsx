import React, { useState, useEffect } from 'react';
import { Upload, message, Button, Card, Progress, Typography, Alert } from 'antd';
import { InboxOutlined, DownloadOutlined, FileOutlined } from '@ant-design/icons';
import { useSearchParams } from 'react-router-dom';
import type { UploadProps } from 'antd';
import { apiService } from '../../services/api';

const { Dragger } = Upload;
const { Title, Text, Paragraph } = Typography;

interface UploadedFile {
  id: string;
  original_name: string;
  url: string;
  size: number;
  mime_type: string;
}

const PublicUpload: React.FC = () => {
  const [searchParams] = useSearchParams();
  const [token, setToken] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadedFiles, setUploadedFiles] = useState<UploadedFile[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const tokenParam = searchParams.get('token');
    if (tokenParam) {
      setToken(tokenParam);
      // Optional: Verify token or just use it
    } else {
      setError('Missing access token. Please ensure you have the correct link.');
    }
  }, [searchParams]);

  const props: UploadProps = {
    name: 'file',
    multiple: true,
    showUploadList: false,
    customRequest: async (options) => {
      const { onSuccess, onError, file } = options;
      if (!token) {
        message.error('Authentication token missing');
        return;
      }

      setUploading(true);
      try {
        // Use the token for upload
        const response = await apiService.uploadFile(file as File, token);
        
        if (response.code === 200) {
          message.success(`${(file as File).name} uploaded successfully.`);
          const newFile: UploadedFile = {
            ...response.data,
            url: response.data.url.startsWith('/') ? response.data.url : `/api/v1/files/${response.data.id}`
          };
          setUploadedFiles(prev => [newFile, ...prev]);
          onSuccess && onSuccess(response.data);
        } else {
          message.error(`${(file as File).name} upload failed: ${response.message}`);
          onError && onError(new Error(response.message));
        }
      } catch (err: any) {
        message.error(`${(file as File).name} upload failed.`);
        onError && onError(err);
      } finally {
        setUploading(false);
      }
    },
    onDrop(e) {
      console.log('Dropped files', e.dataTransfer.files);
    },
  };

  const handleDownload = async (file: UploadedFile) => {
    try {
      const blob = await apiService.downloadFile(file.url, token || undefined);
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = file.original_name;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
    } catch (err: any) {
      message.error(`Download failed: ${err.message}`);
    }
  };

  if (error) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh', background: '#f0f2f5' }}>
        <Alert
          message="Access Denied"
          description={error}
          type="error"
          showIcon
          style={{ maxWidth: 400 }}
        />
      </div>
    );
  }

  return (
    <div style={{ minHeight: '100vh', background: '#f0f2f5', padding: '24px' }}>
      <div style={{ maxWidth: 800, margin: '0 auto' }}>
        <Card style={{ marginBottom: 24, textAlign: 'center' }}>
          <Title level={2}>Fly Print Upload</Title>
          <Paragraph type="secondary">
            Upload your documents for printing. Supported formats: PDF, DOCX, JPG, PNG.
          </Paragraph>
        </Card>

        <Card title="Upload Files">
          <Dragger {...props} disabled={!token || uploading}>
            <p className="ant-upload-drag-icon">
              <InboxOutlined />
            </p>
            <p className="ant-upload-text">Click or drag file to this area to upload</p>
            <p className="ant-upload-hint">
              Support for a single or bulk upload. Strictly prohibited from uploading banned files.
            </p>
          </Dragger>
        </Card>

        {uploadedFiles.length > 0 && (
          <Card title="Uploaded Files" style={{ marginTop: 24 }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
              {uploadedFiles.map(file => (
                <div key={file.id} style={{ 
                  display: 'flex', 
                  justifyContent: 'space-between', 
                  alignItems: 'center',
                  padding: '12px',
                  border: '1px solid #f0f0f0',
                  borderRadius: '4px',
                  background: '#fafafa'
                }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                    <FileOutlined style={{ fontSize: 24, color: '#1890ff' }} />
                    <div>
                      <Text strong>{file.original_name}</Text>
                      <br />
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        {(file.size / 1024).toFixed(1)} KB
                      </Text>
                    </div>
                  </div>
                  <Button 
                    icon={<DownloadOutlined />} 
                    onClick={() => handleDownload(file)}
                  >
                    Download
                  </Button>
                </div>
              ))}
            </div>
          </Card>
        )}
      </div>
    </div>
  );
};

export default PublicUpload;
