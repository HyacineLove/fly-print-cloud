import React, { useCallback, useEffect, useState } from 'react';
import { Button, Card, Input, Modal, Space, Switch, Table, Tag, Tooltip, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { buildApiUrl, buildAuthUrl } from '../../config';
import { DateTimeValue, TwoLineValue } from '../DisplayValue';

interface EdgeNode {
  id: string; name: string; alias?: string; location?: string; connection_status: string;
  health_status: string; health_message?: string; enabled: boolean; last_heartbeat?: string;
  version?: string; registration_state: string;
}

async function request(path: string, init?: RequestInit) {
  const me = await fetch(buildAuthUrl('me'));
  const token = (await me.json())?.data?.access_token;
  const response = await fetch(buildApiUrl(path), {
    ...init,
    headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}), ...(init?.headers || {}) },
  });
  if (!response.ok) throw new Error(`HTTP ${response.status}`);
  return (await response.json())?.data;
}

const statusTag = (status: string) => <Tag color={status === 'online' ? 'success' : status === 'unstable' ? 'warning' : 'default'}>{status === 'online' ? '在线' : status === 'unstable' ? '连接不稳定' : '离线'}</Tag>;
const healthTag = (status: string) => <Tag color={status === 'healthy' ? 'success' : status === 'critical' ? 'error' : status === 'degraded' ? 'warning' : 'default'}>{status === 'healthy' ? '健康' : status === 'critical' ? '严重' : status === 'degraded' ? '降级' : '未知'}</Tag>;

const EdgeNodes: React.FC = () => {
  const [nodes, setNodes] = useState<EdgeNode[]>([]);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState<string | null>(null);
  const [alias, setAlias] = useState('');
  const [activation, setActivation] = useState<{ code: string; expiresAt: string } | null>(null);

  const load = useCallback(async () => {
    try { const data = await request('/admin/edge-nodes?page=1&page_size=100'); setNodes(data?.items || []); }
    catch { message.error('节点信息加载失败'); }
    finally { setLoading(false); }
  }, []);

  useEffect(() => { load(); const timer = window.setInterval(load, 30000); return () => window.clearInterval(timer); }, [load]);
  useEffect(() => {
    if (!editing) return undefined;
    const closeEditor = (event: MouseEvent) => { if (!(event.target as HTMLElement).closest('.inline-name-editor')) setEditing(null); };
    document.addEventListener('mousedown', closeEditor);
    return () => document.removeEventListener('mousedown', closeEditor);
  }, [editing]);

  const saveAlias = async (node: EdgeNode) => {
    try { await request(`/admin/edge-nodes/${node.id}/alias`, { method: 'PATCH', body: JSON.stringify({ alias: alias.trim() }) }); message.success('名称已保存'); setEditing(null); load(); }
    catch { message.error('名称保存失败'); }
  };
  const createActivation = async () => {
    try { const data = await request('/admin/edge-nodes/activations', { method: 'POST', body: '{}' }); setActivation({ code: data.activation_code, expiresAt: data.expires_at }); load(); }
    catch { message.error('创建待激活终端失败'); }
  };
  const toggle = async (node: EdgeNode, enabled: boolean) => {
    try { await request(`/admin/edge-nodes/${node.id}/enabled`, { method: 'PATCH', body: JSON.stringify({ enabled }) }); load(); }
    catch { message.error('状态更新失败'); }
  };
  const remove = (node: EdgeNode) => Modal.confirm({
    title: '删除节点？', content: `删除后该节点的专属凭据将失效，节点需要重新激活。\n${node.id}`,
    okText: '删除', okType: 'danger', cancelText: '取消',
    onOk: async () => { try { await request(`/admin/edge-nodes/${node.id}`, { method: 'DELETE' }); message.success('节点已删除'); load(); } catch { message.error('删除失败'); } },
  });

  const columns: ColumnsType<EdgeNode> = [
    { title: '节点 ID', dataIndex: 'id', width: 245, render: (_, node) => <TwoLineValue id={node.id} name={node.alias || node.name} /> },
    { title: '节点名称', width: 260, render: (_, node) => editing === node.id ? <Space.Compact className="inline-name-editor"><Input autoFocus value={alias} onChange={event => setAlias(event.target.value)} onPressEnter={() => saveAlias(node)} placeholder="留空以清除别名" /><Button type="primary" onClick={() => saveAlias(node)}>保存</Button></Space.Compact> : <span onClick={() => { setEditing(node.id); setAlias(node.alias || node.name || ''); }} style={{ cursor: 'pointer' }}><div>{node.alias || node.name || '待激活终端'}</div>{node.alias && <div style={{ color: '#8c8c8c', fontSize: 12 }}>（{node.name || '待上报'}）</div>}</span> },
    { title: '节点位置', dataIndex: 'location', render: value => value || '-' },
    { title: '节点状态', dataIndex: 'connection_status', render: statusTag },
    { title: '节点健康状态', render: (_, node) => <Tooltip title={node.health_message}>{healthTag(node.health_status)}</Tooltip> },
    { title: '节点最后心跳', dataIndex: 'last_heartbeat', width: 175, render: value => <DateTimeValue value={value} /> },
    { title: '节点版本', dataIndex: 'version', render: value => value || '-' },
    { title: '节点启用状态', width: 105, render: (_, node) => <Switch checked={node.enabled} disabled={node.registration_state === 'pending_activation'} onChange={value => toggle(node, value)} /> },
    { title: '', width: 70, render: (_, node) => <Button danger type="primary" icon={<DeleteOutlined />} onClick={() => remove(node)} /> },
  ];

  return <div>
    <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 16 }}><Button type="primary" icon={<PlusOutlined />} onClick={createActivation}>创建待激活终端</Button></div>
    <Card><Table rowKey="id" loading={loading} dataSource={nodes} columns={columns} scroll={{ x: 1320 }} pagination={{ pageSize: 20, showSizeChanger: false }} /></Card>
    <Modal open={!!activation} footer={<Button type="primary" onClick={() => setActivation(null)}>我已保存</Button>} closable={false} title="一次性激活码">
      <Typography.Paragraph>请在 Edge 的初始激活界面填写 Cloud URL 和以下激活码。激活码仅显示一次，10 分钟后失效。</Typography.Paragraph>
      <Typography.Title level={3} copyable={{ text: activation?.code }}>{activation?.code}</Typography.Title>
      <Space align="start"><Typography.Text type="secondary">失效时间：</Typography.Text><DateTimeValue value={activation?.expiresAt} /></Space>
    </Modal>
  </div>;
};

export default EdgeNodes;
