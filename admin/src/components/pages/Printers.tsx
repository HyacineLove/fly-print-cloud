import React, { useCallback, useEffect, useState } from 'react';
import { Button, Card, Input, Modal, Space, Switch, Table, Tag, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { DeleteOutlined } from '@ant-design/icons';
import { buildApiUrl, buildAuthUrl } from '../../config';
import { TwoLineValue } from '../DisplayValue';

interface Node { id: string; name: string; alias?: string; connection_status: string; }
interface Printer { id: string; name: string; display_name?: string; model?: string; printer_status: string; enabled: boolean; edge_node_id: string; }

async function request(path: string, init?: RequestInit) {
  const me = await fetch(buildAuthUrl('me')); const token = (await me.json())?.data?.access_token;
  const response = await fetch(buildApiUrl(path), { ...init, headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}), ...(init?.headers || {}) } });
  if (!response.ok) throw new Error(`HTTP ${response.status}`);
  return (await response.json())?.data;
}

const stateTag = (value: string) => {
  const states: Record<string, { color: string; label: string }> = {
    idle: { color: 'success', label: '就绪' }, printing: { color: 'processing', label: '打印中' },
    printer_stopped: { color: 'error', label: '已停止' }, error: { color: 'error', label: '异常' },
    offline: { color: 'default', label: '离线' }, printer_state_unknown: { color: 'default', label: '未知' },
  };
  const state = states[value] || { color: 'default', label: value || '-' };
  return <Tag color={state.color}>{state.label}</Tag>;
};

export const effectivePrinterStatus = (printerStatus: string, nodeStatus?: string) =>
  nodeStatus === 'offline' ? 'offline' : printerStatus;

const Printers: React.FC = () => {
  const [printers, setPrinters] = useState<Printer[]>([]);
  const [nodes, setNodes] = useState<Record<string, Node>>({});
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState<string | null>(null);
  const [name, setName] = useState('');

  const load = useCallback(async () => {
    try {
      const [printerData, nodeData] = await Promise.all([request('/admin/printers?page=1&page_size=100'), request('/admin/edge-nodes?page=1&page_size=100')]);
      setPrinters(printerData?.items || []); setNodes(Object.fromEntries((nodeData?.items || []).map((node: Node) => [node.id, node])));
    } catch { message.error('打印机信息加载失败'); } finally { setLoading(false); }
  }, []);

  useEffect(() => { load(); const timer = window.setInterval(load, 30000); return () => window.clearInterval(timer); }, [load]);
  useEffect(() => {
    if (!editing) return undefined;
    const closeEditor = (event: MouseEvent) => { if (!(event.target as HTMLElement).closest('.inline-name-editor')) setEditing(null); };
    document.addEventListener('mousedown', closeEditor);
    return () => document.removeEventListener('mousedown', closeEditor);
  }, [editing]);

  const update = async (printer: Printer, payload: object) => { try { await request(`/admin/printers/${printer.id}`, { method: 'PUT', body: JSON.stringify(payload) }); load(); } catch { message.error('打印机更新失败'); } };
  const saveName = (printer: Printer) => { update(printer, { display_name: name.trim() }); setEditing(null); };
  const remove = (printer: Printer) => Modal.confirm({ title: '删除打印机？', content: `${printer.display_name || printer.name}\n${printer.id}`, okText: '删除', okType: 'danger', cancelText: '取消', onOk: async () => { try { await request(`/admin/printers/${printer.id}`, { method: 'DELETE' }); message.success('打印机已删除'); load(); } catch { message.error('删除失败'); } } });

  const columns: ColumnsType<Printer> = [
    { title: '打印机 ID', dataIndex: 'id', width: 270, render: (_, printer) => <TwoLineValue id={printer.id} name={printer.display_name || printer.name} /> },
    { title: '打印机名称', width: 250, render: (_, printer) => editing === printer.id ? <Space.Compact className="inline-name-editor"><Input autoFocus value={name} onChange={event => setName(event.target.value)} onPressEnter={() => saveName(printer)} placeholder="留空以清除别名" /><Button type="primary" onClick={() => saveName(printer)}>保存</Button></Space.Compact> : <span style={{ cursor: 'pointer' }} onClick={() => { setEditing(printer.id); setName(printer.display_name || printer.name || ''); }}><div>{printer.display_name || printer.name}</div>{printer.display_name && <div style={{ color: '#8c8c8c', fontSize: 12 }}>（{printer.name}）</div>}</span> },
    { title: '打印机型号', dataIndex: 'model', render: value => value || '-' },
    { title: '打印机所属节点', width: 250, render: (_, printer) => <TwoLineValue id={printer.edge_node_id} name={nodes[printer.edge_node_id]?.alias || nodes[printer.edge_node_id]?.name} /> },
    { title: '打印机当前状态', render: (_, printer) => stateTag(effectivePrinterStatus(printer.printer_status, nodes[printer.edge_node_id]?.connection_status)) },
    { title: '打印机启用状态', width: 110, render: (_, printer) => <Switch checked={printer.enabled} onChange={enabled => update(printer, { enabled })} /> },
    { title: '', width: 70, render: (_, printer) => <Button danger type="primary" icon={<DeleteOutlined />} onClick={() => remove(printer)} /> },
  ];

  return <Card><Table rowKey="id" loading={loading} dataSource={printers} columns={columns} scroll={{ x: 1200 }} pagination={{ pageSize: 20, showSizeChanger: false }} /></Card>;
};

export default Printers;
