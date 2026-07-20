import React, { useCallback, useEffect, useState } from 'react';
import { Card, Table, Tag, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { buildApiUrl, buildAuthUrl } from '../../config';
import { DateTimeValue, TwoLineValue } from '../DisplayValue';

interface PrintJob {
  id: string; name: string; initiator_name?: string; edge_node_id?: string; node_name?: string;
  printer_id?: string; printer_name?: string; copies?: number; created_at: string; end_time?: string;
  status: string; error_code?: string; error_message?: string;
}

async function listJobs(page: number) {
  const me = await fetch(buildAuthUrl('me')); const token = (await me.json())?.data?.access_token;
  const response = await fetch(buildApiUrl(`/admin/print-jobs?page=${page}&pageSize=20`), { headers: token ? { Authorization: `Bearer ${token}` } : {} });
  if (!response.ok) throw new Error(`HTTP ${response.status}`);
  return response.json();
}

const result = (job: PrintJob) => {
  const labels: Record<string, [string, string]> = {
    completed: ['success', '完成'], failed: ['error', '失败'], canceled: ['default', '已取消'], cancelled: ['default', '已取消'],
    unconfirmed: ['warning', '结果未确认'], pending: ['default', '等待中'], dispatched: ['processing', '已投递'], processing: ['processing', '打印中'],
  };
  const [color, text] = labels[job.status] || ['default', job.status];
  return <span><Tag color={color}>{text}</Tag>{job.error_message && <div style={{ color: '#8c8c8c', fontSize: 12 }}>{job.error_message}</div>}</span>;
};

const PrintJobs: React.FC = () => {
  const [jobs, setJobs] = useState<PrintJob[]>([]); const [total, setTotal] = useState(0); const [page, setPage] = useState(1); const [loading, setLoading] = useState(true);
  const load = useCallback(async (nextPage = page) => { try { setLoading(true); const data = await listJobs(nextPage); setJobs(data.jobs || []); setTotal(data.pagination?.total || 0); setPage(nextPage); } catch { message.error('打印任务加载失败'); } finally { setLoading(false); } }, [page]);
  useEffect(() => { load(1); }, []); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { const timer = window.setInterval(() => { if (!document.hidden) load(); }, 30000); return () => window.clearInterval(timer); }, [load]);
  const terminal = (status: string) => ['completed', 'failed', 'cancelled', 'canceled', 'unconfirmed'].includes(status);

  const columns: ColumnsType<PrintJob> = [
    { title: '任务 ID', width: 250, render: (_, job) => <TwoLineValue id={job.id} name={job.name || '-'} /> },
    { title: '任务来源', dataIndex: 'initiator_name', width: 110, render: value => value || '主系统' },
    { title: '节点 ID', width: 220, render: (_, job) => <TwoLineValue id={job.edge_node_id} name={job.node_name} /> },
    { title: '打印机 ID', width: 250, render: (_, job) => <TwoLineValue id={job.printer_id} name={job.printer_name} /> },
    { title: '任务创建时间', dataIndex: 'created_at', width: 150, render: value => <DateTimeValue value={value} /> },
    { title: '任务终态时间', width: 150, render: (_, job) => terminal(job.status) && job.end_time ? <DateTimeValue value={job.end_time} /> : '-' },
    { title: '任务结果', width: 120, render: (_, job) => result(job) },
  ];

  return <Card><Table rowKey="id" loading={loading} dataSource={jobs} columns={columns} pagination={{ current: page, total, pageSize: 20, showSizeChanger: false, onChange: next => load(next) }} /></Card>;
};

export default PrintJobs;
