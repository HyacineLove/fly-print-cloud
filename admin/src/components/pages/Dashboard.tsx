import React, { useCallback, useEffect, useState } from 'react';
import { Card, Col, Collapse, Empty, Row, Select, Space, Statistic, Table, Typography, message } from 'antd';
import type { ColumnsType, TablePaginationConfig } from 'antd/es/table';
import { CloudServerOutlined, PrinterOutlined, WarningOutlined } from '@ant-design/icons';
import { Link } from 'react-router-dom';
import { buildApiUrl, buildAuthUrl } from '../../config';

interface AlertRecord {
  id: string;
  resource_type: 'node' | 'printer' | 'job';
  resource_id: string;
  node_name?: string;
  printer_name?: string;
  job_id?: string;
  title: string;
  status: 'open' | 'resolved';
  first_seen_at: string;
  last_seen_at: string;
  resolved_at?: string;
  duration_seconds?: number;
}

interface Summary { high: number; offline_nodes: number; unavailable_printers: number; }
interface AlertPage { items: AlertRecord[]; total: number; page: number; page_size: number; }
interface MaintenanceData extends AlertPage { summary: Summary; }

const emptySummary: Summary = { high: 0, offline_nodes: 0, unavailable_printers: 0 };

async function api(path: string) {
  const authResponse = await fetch(buildAuthUrl('me'));
  const authBody = await authResponse.json();
  const token = authBody?.data?.access_token;
  const response = await fetch(buildApiUrl(path), { headers: token ? { Authorization: `Bearer ${token}` } : {} });
  if (!response.ok) throw new Error(`HTTP ${response.status}`);
  const body = await response.json();
  return body?.data || {};
}

const resourceLink = (alert: AlertRecord) => {
  if (alert.resource_type === 'node') return <Link to="/edge-nodes">{alert.node_name || alert.resource_id}</Link>;
  if (alert.resource_type === 'printer') return <Link to="/printers">{alert.printer_name || alert.resource_id}</Link>;
  return <Link to="/print-jobs">{alert.job_id || alert.resource_id}</Link>;
};

const durationText = (seconds?: number) => {
  if (seconds == null) return '-';
  if (seconds < 60) return `${seconds} 秒`;
  if (seconds < 3600) return `${Math.round(seconds / 60)} 分钟`;
  return `${(seconds / 3600).toFixed(1)} 小时`;
};

const Dashboard: React.FC = () => {
  const [maintenance, setMaintenance] = useState<MaintenanceData>({ items: [], total: 0, page: 1, page_size: 20, summary: emptySummary });
  const [history, setHistory] = useState<AlertPage>({ items: [], total: 0, page: 1, page_size: 10 });
  const [resourceType, setResourceType] = useState('');
  const [historyOpen, setHistoryOpen] = useState(false);
  const [loading, setLoading] = useState(true);
  const [historyLoading, setHistoryLoading] = useState(false);

  const loadCurrent = useCallback(async (page = 1) => {
    const query = new URLSearchParams({ page: String(page), page_size: '20' });
    if (resourceType) query.set('resource_type', resourceType);
    const data = await api(`/admin/dashboard/maintenance?${query}`);
    setMaintenance({
      items: data.items || [], total: data.total || 0, page: data.page || page,
      page_size: data.page_size || 20, summary: data.summary || emptySummary,
    });
  }, [resourceType]);

  const loadHistory = useCallback(async (page = 1) => {
    setHistoryLoading(true);
    try {
      const data = await api(`/admin/alerts/history?status=resolved&page=${page}&page_size=10`);
      setHistory({ items: data.items || [], total: data.total || 0, page: data.page || page, page_size: data.page_size || 10 });
    } finally { setHistoryLoading(false); }
  }, []);

  const refresh = useCallback(async () => {
    try {
      await loadCurrent(maintenance.page || 1);
      if (historyOpen) await loadHistory(history.page || 1);
    } catch (error) {
      console.error(error);
      message.error('运维状态加载失败，请稍后重试');
    } finally { setLoading(false); }
  }, [history.page, historyOpen, loadCurrent, loadHistory, maintenance.page]);

  useEffect(() => {
    setLoading(true);
    loadCurrent(1).catch(error => console.error('运维状态加载失败', error)).finally(() => setLoading(false));
  }, [loadCurrent]);

  useEffect(() => {
    const timer = window.setInterval(() => { if (!document.hidden) refresh(); }, 30000);
    const onVisible = () => { if (!document.hidden) refresh(); };
    document.addEventListener('visibilitychange', onVisible);
    return () => { window.clearInterval(timer); document.removeEventListener('visibilitychange', onVisible); };
  }, [refresh]);

  const currentColumns: ColumnsType<AlertRecord> = [
    { title: '问题', dataIndex: 'title', width: 220 },
    { title: '节点', dataIndex: 'node_name', width: 180, render: value => value || '-' },
    { title: '打印机/任务', key: 'resource', width: 220, render: (_, row) => resourceLink(row) },
    { title: '首次发生', dataIndex: 'first_seen_at', width: 180, render: value => new Date(value).toLocaleString() },
    { title: '最后确认', dataIndex: 'last_seen_at', width: 180, render: value => new Date(value).toLocaleString() },
  ];
  const historyColumns: ColumnsType<AlertRecord> = [
    { title: '问题', dataIndex: 'title' },
    { title: '资源', key: 'resource', render: (_, row) => resourceLink(row) },
    { title: '发生时间', dataIndex: 'first_seen_at', render: value => new Date(value).toLocaleString() },
    { title: '恢复时间', dataIndex: 'resolved_at', render: value => value ? new Date(value).toLocaleString() : '-' },
    { title: '持续时长', dataIndex: 'duration_seconds', render: durationText },
  ];
  const pagination = (data: AlertPage, onChange: (page: number) => void): TablePaginationConfig => ({
    current: data.page, pageSize: data.page_size, total: data.total, showSizeChanger: false, onChange,
  });

  return <Space direction="vertical" size={16} style={{ width: '100%' }}>
    <Typography.Title level={3} style={{ margin: 0 }}>运维状态</Typography.Title>
    <Row gutter={[16, 16]}>
      <Col xs={24} lg={8}><Card><Statistic title="高优先级" value={maintenance.summary.high} valueStyle={{ color: maintenance.summary.high ? '#cf1322' : undefined }} prefix={<WarningOutlined />} /></Card></Col>
      <Col xs={24} lg={8}><Card><Statistic title="离线节点" value={maintenance.summary.offline_nodes} prefix={<CloudServerOutlined />} /></Card></Col>
      <Col xs={24} lg={8}><Card><Statistic title="不可用打印机" value={maintenance.summary.unavailable_printers} prefix={<PrinterOutlined />} /></Card></Col>
    </Row>
    <Card title="需要立即处理" extra={<Select value={resourceType} style={{ width: 120 }} onChange={setResourceType}
      options={[{ value: '', label: '全部资源' }, { value: 'node', label: '节点' }, { value: 'printer', label: '打印机' }, { value: 'job', label: '任务' }]} />}>
      <Table rowKey="id" loading={loading} dataSource={maintenance.items} columns={currentColumns}
        pagination={pagination(maintenance, page => loadCurrent(page).catch(() => message.error('告警加载失败')))}
        locale={{ emptyText: <Empty description="当前没有需要立即处理的问题" /> }} />
    </Card>
    <Collapse onChange={keys => {
      const open = (Array.isArray(keys) ? keys : [keys]).includes('history');
      setHistoryOpen(open);
      if (open && !historyOpen) loadHistory(1).catch(() => message.error('告警历史加载失败'));
    }} items={[{ key: 'history', label: '告警历史', children:
      <Table rowKey="id" size="small" loading={historyLoading} dataSource={history.items} columns={historyColumns}
        pagination={pagination(history, page => loadHistory(page).catch(() => message.error('告警历史加载失败')))}
        locale={{ emptyText: '暂无恢复记录' }} /> }]} />
  </Space>;
};

export default Dashboard;
