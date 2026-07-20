import React, { useEffect, useState } from 'react';
import { Alert, Button, Form, Input, InputNumber, message, Modal, Popconfirm, Space, Switch, Table, Tooltip } from 'antd';
import { EditOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { buildApiUrl, buildAuthUrl } from '../../config';

interface Provider {
  id: string;
  code: string;
  display_name: string;
  entry_url: string;
  callback_base_url: string;
  entry_visible: boolean;
  enabled: boolean;
  allowed_ip_cidrs: string;
  allowed_file_hosts: string;
  max_file_size: number;
  allowed_mime_types: string;
}

type ProviderForm = Omit<Provider, 'id' | 'enabled' | 'entry_visible'>;
interface OneTimeSecrets { inbound_hmac_secret: string; outbound_hmac_secret: string; }

const IntegrationProviders: React.FC = () => {
  const [providers, setProviders] = useState<Provider[]>([]);
  const [token, setToken] = useState<string>();
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState<Provider>();
  const [formVisible, setFormVisible] = useState(false);
  const [secrets, setSecrets] = useState<OneTimeSecrets>();
  const [form] = Form.useForm<ProviderForm>();

  const loadProviders = async (accessToken: string) => {
    setLoading(true);
    try {
      const response = await fetch(buildApiUrl('/admin/integration-providers'), { headers: { Authorization: `Bearer ${accessToken}` } });
      const result = await response.json();
      if (!response.ok || result.code !== 200) throw new Error(result.message || '加载第三方接入配置失败');
      setProviders(result.data || []);
    } catch (error) {
      message.error(error instanceof Error ? error.message : '加载第三方接入配置失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void (async () => {
      try {
        const response = await fetch(buildAuthUrl('me'));
        const result = await response.json();
        const accessToken = result.data?.access_token;
        if (response.ok && result.code === 200 && accessToken) {
          setToken(accessToken);
          await loadProviders(accessToken);
        }
      } finally { setLoading(false); }
    })();
  }, []);

  const openCreate = () => {
    setEditing(undefined);
    form.setFieldsValue({ max_file_size: 10 * 1024 * 1024, allowed_mime_types: 'application/pdf' } as ProviderForm);
    setFormVisible(true);
  };

  const openEdit = (provider: Provider) => {
    setEditing(provider);
    form.setFieldsValue(provider);
    setFormVisible(true);
  };

  const save = async (values: ProviderForm) => {
    if (!token) return;
    const path = editing ? `/admin/integration-providers/${editing.code}` : '/admin/integration-providers';
    const payload = { ...values, entry_visible: editing?.entry_visible ?? true, enabled: editing?.enabled ?? false };
    const response = await fetch(buildApiUrl(path), {
      method: editing ? 'PUT' : 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: JSON.stringify(payload),
    });
    const result = await response.json();
    if (!response.ok || (result.code !== 200 && result.code !== 201)) throw new Error(result.message || '保存失败');
    setFormVisible(false);
    setEditing(undefined);
    await loadProviders(token);
    if (!editing) {
      setSecrets({ inbound_hmac_secret: result.data.inbound_hmac_secret, outbound_hmac_secret: result.data.outbound_hmac_secret });
    }
  };

  const updateSwitch = async (provider: Provider, field: 'enabled' | 'entry_visible', value: boolean) => {
    if (!token) return;
    const path = field === 'enabled' ? 'enabled' : 'entry-visible';
    try {
      const response = await fetch(buildApiUrl(`/admin/integration-providers/${provider.code}/${path}`), {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ [field]: value }),
      });
      const result = await response.json();
      if (!response.ok || result.code !== 200) throw new Error(result.message || '状态更新失败');
      const updated = result.data as Provider;
      setProviders(current => current.map(item => item.code === provider.code ? { ...item, [field]: updated[field] } : item));
    } catch (error) {
      message.error(error instanceof Error ? error.message : '状态更新失败');
    }
  };

  const rotateSecrets = async (provider: Provider) => {
    if (!token) return;
    const response = await fetch(buildApiUrl(`/admin/integration-providers/${provider.code}/rotate-secret`), {
      method: 'POST', headers: { Authorization: `Bearer ${token}` }, cache: 'no-store',
    });
    const result = await response.json();
    if (!response.ok || result.code !== 200) throw new Error(result.message || '轮换密钥失败');
    setSecrets(result.data);
  };

  const columns: ColumnsType<Provider> = [
    { title: '三方', width: 180, render: (_, provider) => <span><div>{provider.display_name}</div><code style={{ fontSize: 12 }}>{provider.code}</code></span> },
    { title: '入口地址', dataIndex: 'entry_url', width: 220, ellipsis: true, render: value => <Tooltip title={value}>{value}</Tooltip> },
    { title: '回调地址', dataIndex: 'callback_base_url', width: 200, ellipsis: true, render: value => <Tooltip title={value}>{value}</Tooltip> },
    { title: '入口显示', width: 95, render: (_, provider) => <Switch checked={provider.entry_visible} onChange={value => void updateSwitch(provider, 'entry_visible', value)} /> },
    { title: '启用状态', width: 95, render: (_, provider) => <Switch checked={provider.enabled} onChange={value => void updateSwitch(provider, 'enabled', value)} /> },
    { title: '操作', width: 210, render: (_, provider) => <Space>
      <Button type="link" icon={<EditOutlined />} onClick={() => openEdit(provider)}>编辑</Button>
      <Popconfirm title="轮换双向 HMAC 密钥" description="旧密钥会立即失效。请先停用 provider 并完成双方替换。" onConfirm={() => void rotateSecrets(provider)}>
        <Button type="link" icon={<ReloadOutlined />}>轮换密钥</Button>
      </Popconfirm>
    </Space> },
  ];

  return <div>
    <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 16 }}><Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>创建三方</Button></div>
    <Table rowKey="id" columns={columns} dataSource={providers} loading={loading} pagination={{ pageSize: 10 }} />
    <Modal title={editing ? `编辑 ${editing.code}` : '创建三方'} open={formVisible} onCancel={() => setFormVisible(false)} footer={null} destroyOnClose>
      <Form form={form} layout="vertical" onFinish={(values) => void save(values).catch((error) => message.error(error.message))}>
        <Form.Item name="code" label="三方代码" rules={[{ required: true }, { pattern: /^[a-z][a-z0-9-]{1,62}$/, message: '使用小写字母、数字和连字符；创建后不可修改' }]}><Input disabled={Boolean(editing)} /></Form.Item>
        <Form.Item name="display_name" label="显示名称" rules={[{ required: true }]}><Input /></Form.Item>
        <Form.Item name="entry_url" label="第三方入口 URL" rules={[{ required: true, type: 'url' }]}><Input placeholder="http://provider.example.com/print" /></Form.Item>
        <Form.Item name="callback_base_url" label="回调基础 URL" rules={[{ required: true, type: 'url' }]}><Input placeholder="http://provider.example.com" /></Form.Item>
        <Form.Item name="allowed_ip_cidrs" label="出口 CIDR（逗号分隔）" rules={[{ required: true }]}><Input placeholder="203.0.113.0/24" /></Form.Item>
        <Form.Item name="allowed_file_hosts" label="文件主机（逗号分隔）" rules={[{ required: true }]}><Input placeholder="files.provider.example.com" /></Form.Item>
        <Form.Item name="max_file_size" label="最大文件字节数" rules={[{ required: true }]}><InputNumber min={1} style={{ width: '100%' }} /></Form.Item>
        <Form.Item name="allowed_mime_types" label="允许 MIME（逗号分隔）" rules={[{ required: true }]}><Input /></Form.Item>
        <div style={{ marginTop: 20 }}><Space><Button type="primary" htmlType="submit">保存</Button><Button onClick={() => setFormVisible(false)}>取消</Button></Space></div>
      </Form>
    </Modal>
    <Modal title="请立即安全保存双向 HMAC 密钥" open={Boolean(secrets)} onOk={() => setSecrets(undefined)} onCancel={() => setSecrets(undefined)} okText="已安全保存" cancelText="关闭">
      <Alert type="error" showIcon message="此处是唯一明文展示机会，关闭后无法再次读取。" style={{ marginBottom: 12 }} />
      <p>入站密钥（Provider → FlyPrint）</p><Input.TextArea readOnly value={secrets?.inbound_hmac_secret} rows={2} />
      <p>出站密钥（FlyPrint → Provider）</p><Input.TextArea readOnly value={secrets?.outbound_hmac_secret} rows={2} />
    </Modal>
  </div>;
};

export default IntegrationProviders;
