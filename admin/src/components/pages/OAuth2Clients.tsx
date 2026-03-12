import React, { useState, useEffect } from 'react';
import { Table, Button, Modal, Form, Input, message, Space, Tag, Popconfirm, Typography, Card } from 'antd';
import { PlusOutlined, CopyOutlined, ReloadOutlined, DeleteOutlined, EditOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { buildApiUrl, buildAuthUrl } from '../../config';

const { Paragraph } = Typography;

interface OAuth2Client {
  id: string;
  client_id: string;
  client_secret?: string;
  name: string;
  description: string;
  scopes: string[];
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

const OAuth2Clients: React.FC = () => {
  const [clients, setClients] = useState<OAuth2Client[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingClient, setEditingClient] = useState<OAuth2Client | null>(null);
  const [newSecret, setNewSecret] = useState<string | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [form] = Form.useForm();

  // 获取 access token 并加载数据
  useEffect(() => {
    const init = async () => {
      try {
        const response = await fetch(buildAuthUrl('me'));
        const result = await response.json();
        if (result.code === 200 && result.data.access_token) {
          const accessToken = result.data.access_token;
          setToken(accessToken);
          
          // 获取客户端列表
          const clientsResponse = await fetch(buildApiUrl('/admin/oauth2-clients'), {
            headers: { 'Authorization': `Bearer ${accessToken}` },
          });
          const clientsResult = await clientsResponse.json();
          if (clientsResult.code === 200) {
            setClients(clientsResult.data?.items || []);
          }
        }
      } catch (error) {
        console.error('初始化失败:', error);
      } finally {
        setLoading(false);
      }
    };
    init();
  }, []);

  const fetchClients = async () => {
    if (!token) return;
    setLoading(true);
    try {
      const response = await fetch(buildApiUrl('/admin/oauth2-clients'), {
        headers: { 'Authorization': `Bearer ${token}` },
      });
      const result = await response.json();
      if (result.code === 200) {
        setClients(result.data?.items || []);
      } else {
        message.error(result.message || '获取客户端列表失败');
      }
    } catch (error) {
      message.error('网络错误');
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async (values: any) => {
    if (!token) return;
    try {
      const response = await fetch(buildApiUrl('/admin/oauth2-clients'), {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({
          client_id: values.client_id,
          name: values.name,
          description: values.description,
          scopes: values.scopes?.split(',').map((s: string) => s.trim()).filter(Boolean) || [],
        }),
      });
      const result = await response.json();
      if (result.code === 200 || result.code === 201) {
        message.success('创建成功');
        setNewSecret(result.data?.client_secret);
        setModalVisible(false);
        form.resetFields();
        fetchClients();
      } else {
        message.error(result.message || '创建失败');
      }
    } catch (error) {
      message.error('网络错误');
    }
  };

  const handleUpdate = async (values: any) => {
    if (!editingClient || !token) return;
    try {
      const response = await fetch(buildApiUrl(`/admin/oauth2-clients/${editingClient.id}`), {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({
          name: values.name,
          description: values.description,
          scopes: values.scopes?.split(',').map((s: string) => s.trim()).filter(Boolean) || [],
          enabled: true,
        }),
      });
      const result = await response.json();
      if (result.code === 200) {
        message.success('更新成功');
        setModalVisible(false);
        setEditingClient(null);
        form.resetFields();
        fetchClients();
      } else {
        message.error(result.message || '更新失败');
      }
    } catch (error) {
      message.error('网络错误');
    }
  };

  const handleResetSecret = async (client: OAuth2Client) => {
    if (!token) return;
    try {
      const response = await fetch(buildApiUrl(`/admin/oauth2-clients/${client.id}/secret`), {
        method: 'PUT',
        headers: {
          'Authorization': `Bearer ${token}`,
        },
      });
      const result = await response.json();
      if (result.code === 200) {
        message.success('密钥已重置');
        setNewSecret(result.data?.client_secret);
      } else {
        message.error(result.message || '重置失败');
      }
    } catch (error) {
      message.error('网络错误');
    }
  };

  const handleDelete = async (client: OAuth2Client) => {
    if (!token) return;
    try {
      const response = await fetch(buildApiUrl(`/admin/oauth2-clients/${client.id}`), {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${token}`,
        },
      });
      const result = await response.json();
      if (result.code === 200) {
        message.success('删除成功');
        fetchClients();
      } else {
        message.error(result.message || '删除失败');
      }
    } catch (error) {
      message.error('网络错误');
    }
  };

  const openEditModal = (client: OAuth2Client) => {
    setEditingClient(client);
    // 处理 allowed_scopes 字段（API 返回的是空格分隔的字符串）
    const scopes = (client as any).allowed_scopes || '';
    form.setFieldsValue({
      client_id: client.client_id,
      name: client.name || '',
      description: client.description,
      scopes: typeof scopes === 'string' ? scopes.replace(/ /g, ', ') : scopes,
    });
    setModalVisible(true);
  };

  const openCreateModal = () => {
    setEditingClient(null);
    form.resetFields();
    setModalVisible(true);
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    message.success('已复制到剪贴板');
  };

  const columns: ColumnsType<OAuth2Client> = [
    {
      title: 'Client ID',
      dataIndex: 'client_id',
      key: 'client_id',
      width: 180,
      render: (text) => (
        <Space>
          <code>{text}</code>
          <CopyOutlined onClick={() => copyToClipboard(text)} style={{ cursor: 'pointer', color: '#1890ff' }} />
        </Space>
      ),
    },
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      width: 120,
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: 'Scopes',
      dataIndex: 'allowed_scopes',
      key: 'allowed_scopes',
      width: 200,
      render: (scopes: string[] | string) => {
        // 处理 scopes 可能是字符串或数组的情况
        const scopeList = typeof scopes === 'string' ? scopes.split(' ').filter(Boolean) : (scopes || []);
        return (
          <>
            {scopeList.slice(0, 2).map((scope) => (
              <Tag key={scope} color="blue">{scope}</Tag>
            ))}
            {scopeList.length > 2 && <Tag>+{scopeList.length - 2}</Tag>}
          </>
        );
      },
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      render: (enabled) => (
        <Tag color={enabled ? 'green' : 'red'}>{enabled ? '启用' : '禁用'}</Tag>
      ),
    },
    {
      title: '操作',
      key: 'action',
      width: 200,
      fixed: 'right',
      render: (_, record) => (
        <Space size="small">
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEditModal(record)}>
            编辑
          </Button>
          <Popconfirm
            title="重置密钥"
            description="重置后将显示新密钥（仅一次），旧密钥立即失效"
            onConfirm={() => handleResetSecret(record)}
            okText="确定"
            cancelText="取消"
          >
            <Button type="link" size="small" icon={<ReloadOutlined />}>
              重置密钥
            </Button>
          </Popconfirm>
          <Popconfirm
            title="删除客户端"
            description="删除后无法恢复"
            onConfirm={() => handleDelete(record)}
            okText="确定"
            cancelText="取消"
          >
            <Button type="link" size="small" danger icon={<DeleteOutlined />}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2 style={{ margin: 0 }}>OAuth2 客户端管理</h2>
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreateModal}>
          创建客户端
        </Button>
      </div>

      {newSecret && (
        <Card style={{ marginBottom: 16, backgroundColor: '#fffbe6', borderColor: '#ffe58f' }}>
          <div style={{ marginBottom: 8 }}>
            <strong>新生成的 Client Secret（仅显示一次，请妥善保存）：</strong>
          </div>
          <Space>
            <Paragraph copyable={{ text: newSecret }} style={{ margin: 0 }}>
              <code style={{ fontSize: 14, padding: '4px 8px', backgroundColor: '#f5f5f5' }}>{newSecret}</code>
            </Paragraph>
            <Button size="small" onClick={() => setNewSecret(null)}>关闭</Button>
          </Space>
        </Card>
      )}

      <Table
        columns={columns}
        dataSource={clients}
        rowKey="id"
        loading={loading}
        pagination={{ pageSize: 10 }}
        scroll={{ x: 1000 }}
      />

      <Modal
        title={editingClient ? '编辑客户端' : '创建客户端'}
        open={modalVisible}
        onCancel={() => {
          setModalVisible(false);
          setEditingClient(null);
          form.resetFields();
        }}
        footer={null}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={editingClient ? handleUpdate : handleCreate}
        >
          <Form.Item
            name="client_id"
            label="Client ID"
            rules={[
              { required: true, message: '请输入 Client ID' },
              { pattern: /^[a-z0-9-]+$/, message: '只能包含小写字母、数字和横线' },
            ]}
          >
            <Input placeholder="例如: fly-print-edge" disabled={!!editingClient} />
          </Form.Item>
          <Form.Item
            name="name"
            label="名称"
            rules={[{ required: true, message: '请输入名称' }]}
          >
            <Input placeholder="例如: Edge 节点客户端" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea placeholder="可选" rows={2} />
          </Form.Item>
          <Form.Item
            name="scopes"
            label="Scopes"
            extra="多个 scope 用逗号分隔"
          >
            <Input placeholder="例如: edge:register, edge:heartbeat, edge:printer" />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit">
                {editingClient ? '保存' : '创建'}
              </Button>
              <Button onClick={() => {
                setModalVisible(false);
                setEditingClient(null);
              }}>
                取消
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default OAuth2Clients;
