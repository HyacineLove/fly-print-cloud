import React, { useState, useEffect } from 'react';
import { Form, Input, Button, Card, message, Typography, Spin } from 'antd';
import { UserOutlined, LockOutlined } from '@ant-design/icons';

const { Title, Text } = Typography;

interface LoginForm {
  username: string;
  password: string;
}

import { buildAuthUrl } from '../../config';

const Login: React.FC = () => {
  const [loading, setLoading] = useState(false);
  const [checkingMode, setCheckingMode] = useState(true);

  // 检查认证模式，keycloak 模式直接跳转到 OAuth2 登录
  useEffect(() => {
    fetch(buildAuthUrl('mode'))
      .then(r => r.json())
      .then(data => {
        if (data.mode === 'keycloak') {
          window.location.href = buildAuthUrl('login');
        } else {
          setCheckingMode(false);
        }
      })
      .catch(() => setCheckingMode(false));
  }, []);

  const onFinish = async (values: LoginForm) => {
    setLoading(true);
    try {
      const formData = new URLSearchParams();
      formData.append('grant_type', 'password');
      formData.append('username', values.username);
      formData.append('password', values.password);

      const response = await fetch(buildAuthUrl('token'), {
        method: 'POST',
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: formData.toString(),
      });

      const result = await response.json();

      if (response.ok && result.access_token) {
        // Token is set via Set-Cookie header by backend, or we store it
        // For builtin mode, we need to set the cookie manually
        const expiresDate = new Date(Date.now() + (result.expires_in || 3600) * 1000);
        document.cookie = `access_token=${result.access_token}; path=/; expires=${expiresDate.toUTCString()}`;
        
        message.success('登录成功');
        // Redirect to dashboard
        window.location.href = '/';
      } else {
        message.error(result.error_description || result.error || '登录失败');
      }
    } catch (error) {
      console.error('登录错误:', error);
      message.error('网络错误，请稍后重试');
    } finally {
      setLoading(false);
    }
  };

  // 检查模式中，显示加载状态
  if (checkingMode) {
    return (
      <div style={{
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        minHeight: '100vh',
        background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
      }}>
        <Spin size="large" />
      </div>
    );
  }

  return (
    <div style={{
      display: 'flex',
      justifyContent: 'center',
      alignItems: 'center',
      minHeight: '100vh',
      background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
    }}>
      <Card
        style={{
          width: 400,
          boxShadow: '0 4px 12px rgba(0, 0, 0, 0.15)',
          borderRadius: 8,
        }}
      >
        <div style={{ textAlign: 'center', marginBottom: 24 }}>
          <Title level={2} style={{ marginBottom: 8 }}>FlyPrint</Title>
          <Text type="secondary">云端智能打印管理系统</Text>
        </div>

        <Form
          name="login"
          onFinish={onFinish}
          size="large"
          autoComplete="off"
        >
          <Form.Item
            name="username"
            rules={[{ required: true, message: '请输入用户名' }]}
          >
            <Input
              prefix={<UserOutlined />}
              placeholder="用户名"
            />
          </Form.Item>

          <Form.Item
            name="password"
            rules={[{ required: true, message: '请输入密码' }]}
          >
            <Input.Password
              prefix={<LockOutlined />}
              placeholder="密码"
            />
          </Form.Item>

          <Form.Item>
            <Button
              type="primary"
              htmlType="submit"
              loading={loading}
              block
            >
              登录
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
};

export default Login;
