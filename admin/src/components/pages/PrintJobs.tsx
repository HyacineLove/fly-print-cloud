import React, { useState, useEffect } from 'react';
import { Card, Table, Tag, Space, Button, Select, message, DatePicker, Popconfirm } from 'antd';
import { 
  ReloadOutlined,
  FileTextOutlined,
  UserOutlined,
  PrinterOutlined,
  ClusterOutlined
} from '@ant-design/icons';
import type { Dayjs } from 'dayjs';

const { RangePicker } = DatePicker;

// 打印任务接口定义
interface PrintJob {
  id: string;
  name: string;
  user_name: string;
  printer_id: string;
  edge_node_id: string;  // 新增：节点ID
  status: 'pending' | 'dispatched' | 'downloading' | 'printing' | 'completed' | 'failed' | 'cancelled';
  created_at: string;
  updated_at: string;
  page_count: number;
  file_path: string;
  file_url: string;
  file_size: number;
  copies: number;
  paper_size: string;
  color_mode: string;
  duplex_mode: string;
  start_time: string;
  end_time: string;
  error_message: string;
  retry_count: number;
  max_retries: number;
  key?: string;
}

// Edge Node 接口
interface EdgeNode {
  id: string;
  name: string;
  display_name: string;
  status: string;
}

// Print Jobs 服务类
class PrintJobsService {
  async getToken(): Promise<string | null> {
    try {
      const response = await fetch('/auth/me');
      const result = await response.json();
      
      if (result.code === 200 && result.data.access_token) {
        return result.data.access_token;
      }
    } catch (error) {
      console.error('获取 token 失败:', error);
    }
    
    return null;
  }

  async getPrintJobs(
    page = 1, 
    pageSize = 10, 
    status = '', 
    edgeNodeId = '',
    startTime = '',
    endTime = ''
  ): Promise<{ jobs: PrintJob[]; total: number; page: number; pageSize: number }> {
    try {
      const token = await this.getToken();
      let url = `/api/v1/admin/print-jobs?page=${page}&pageSize=${pageSize}`;
      if (status) {
        url += `&status=${status}`;
      }
      if (edgeNodeId) {
        url += `&edge_node_id=${edgeNodeId}`;
      }
      if (startTime) {
        url += `&start_time=${encodeURIComponent(startTime)}`;
      }
      if (endTime) {
        url += `&end_time=${encodeURIComponent(endTime)}`;
      }
      
      const response = await fetch(url, {
        headers: {
          ...(token && { 'Authorization': `Bearer ${token}` }),
        },
      });
      
      if (response.ok) {
        const result = await response.json();
        return {
          jobs: result?.jobs || [],
          total: result?.pagination?.total || result?.jobs?.length || 0,
          page: page,
          pageSize: pageSize
        };
      }
    } catch (error) {
      console.error('获取打印任务列表失败:', error);
    }
    
    // API调用失败时返回空数据
    return {
      jobs: [],
      total: 0,
      page: page,
      pageSize: pageSize
    };
  }

  // 获取 Edge Nodes 列表
  async getEdgeNodes(): Promise<EdgeNode[]> {
    try {
      const token = await this.getToken();
      const response = await fetch('/api/v1/admin/edge-nodes', {
        headers: {
          ...(token && { 'Authorization': `Bearer ${token}` }),
        },
      });
      
      if (response.ok) {
        const result = await response.json();
        return result.data?.items || [];
      }
    } catch (error) {
      console.error('获取Edge Nodes失败:', error);
    }
    return [];
  }

  async cancelPrintJob(jobId: string): Promise<void> {
    const token = await this.getToken();
    const response = await fetch(`/api/v1/admin/print-jobs/${jobId}/cancel`, {
      method: 'POST',
      headers: {
        ...(token && { 'Authorization': `Bearer ${token}` }),
      },
    });

    if (!response.ok) {
      let errorMessage = '取消打印任务失败';
      try {
        const result = await response.json();
        errorMessage = result?.message || result?.error || errorMessage;
      } catch {
        // keep default message when response body is not json
      }
      throw new Error(errorMessage);
    }
  }
}

const printJobsService = new PrintJobsService();

// Print Jobs 组件
const PrintJobs: React.FC = () => {
  const [printJobs, setPrintJobs] = useState<PrintJob[]>([]);
  const [loading, setLoading] = useState(true);
  const [statusFilter, setStatusFilter] = useState<string>('');
  const [edgeNodeFilter, setEdgeNodeFilter] = useState<string>('');
  const [dateRange, setDateRange] = useState<[Dayjs | null, Dayjs | null] | null>(null);
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [total, setTotal] = useState(0);
  const [edgeNodes, setEdgeNodes] = useState<EdgeNode[]>([]);
  const [cancellingJobId, setCancellingJobId] = useState<string | null>(null);

  // 加载打印任务数据
  const loadPrintJobs = async (
    page = 1, 
    size = 10, 
    status = '', 
    edgeNodeId = '',
    startTime = '',
    endTime = ''
  ) => {
    try {
      setLoading(true);
      const result = await printJobsService.getPrintJobs(page, size, status, edgeNodeId, startTime, endTime);
      setPrintJobs(result.jobs.map(job => ({ ...job, key: job.id })));
      setTotal(result.total);
      setCurrentPage(page);
      setPageSize(size);
    } catch (error) {
      console.error('加载打印任务失败:', error);
      message.error('加载打印任务失败，请稍后重试');
    } finally {
      setLoading(false);
    }
  };

  // 加载Edge Nodes
  const loadEdgeNodes = async () => {
    const nodes = await printJobsService.getEdgeNodes();
    setEdgeNodes(nodes);
  };

  useEffect(() => {
    loadPrintJobs(currentPage, pageSize, statusFilter, edgeNodeFilter);
    loadEdgeNodes();
  }, []);

  // 状态标签映射
  const getStatusTag = (status: string) => {
    switch (status) {
      case 'pending':
        return <Tag color="default">等待中</Tag>;
      case 'dispatched':
        return <Tag color="blue">已分发</Tag>;
      case 'downloading':
        return <Tag color="cyan">下载中</Tag>;
      case 'printing':
        return <Tag color="processing">打印中</Tag>;
      case 'completed':
        return <Tag color="success">已完成</Tag>;
      case 'failed':
        return <Tag color="error">失败</Tag>;
      case 'cancelled':
        return <Tag color="default">已取消</Tag>;
      default:
        return <Tag color="default">{status}</Tag>;
    }
  };

  // 处理状态筛选
  const handleStatusChange = (value: string) => {
    setStatusFilter(value);
    setCurrentPage(1);
    const [startTime, endTime] = getDateRangeStrings();
    loadPrintJobs(1, pageSize, value, edgeNodeFilter, startTime, endTime);
  };

  // 处理节点筛选
  const handleEdgeNodeChange = (value: string) => {
    setEdgeNodeFilter(value);
    setCurrentPage(1);
    const [startTime, endTime] = getDateRangeStrings();
    loadPrintJobs(1, pageSize, statusFilter, value, startTime, endTime);
  };

  // 处理日期范围变化
  const handleDateRangeChange = (dates: [Dayjs | null, Dayjs | null] | null) => {
    setDateRange(dates);
    setCurrentPage(1);
    let startTime = '';
    let endTime = '';
    if (dates && dates[0] && dates[1]) {
      startTime = dates[0].format('YYYY-MM-DD HH:mm');
      endTime = dates[1].format('YYYY-MM-DD HH:mm');
    }
    loadPrintJobs(1, pageSize, statusFilter, edgeNodeFilter, startTime, endTime);
  };

  // 获取日期范围字符串
  const getDateRangeStrings = (): [string, string] => {
    if (dateRange && dateRange[0] && dateRange[1]) {
      return [
        dateRange[0].format('YYYY-MM-DD HH:mm'),
        dateRange[1].format('YYYY-MM-DD HH:mm')
      ];
    }
    return ['', ''];
  };

  // 处理分页变化
  const handleTableChange = (page: number, size?: number) => {
    const newSize = size || pageSize;
    setCurrentPage(page);
    setPageSize(newSize);
    const [startTime, endTime] = getDateRangeStrings();
    loadPrintJobs(page, newSize, statusFilter, edgeNodeFilter, startTime, endTime);
  };

  // 刷新数据
  const handleRefresh = () => {
    const [startTime, endTime] = getDateRangeStrings();
    loadPrintJobs(currentPage, pageSize, statusFilter, edgeNodeFilter, startTime, endTime);
  };

  const canCancel = (status: string) => {
    return status === 'pending' || status === 'dispatched' || status === 'printing';
  };

  const handleCancelJob = async (job: PrintJob) => {
    try {
      setCancellingJobId(job.id);
      await printJobsService.cancelPrintJob(job.id);
      message.success('任务已取消');
      const [startTime, endTime] = getDateRangeStrings();
      await loadPrintJobs(currentPage, pageSize, statusFilter, edgeNodeFilter, startTime, endTime);
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : '取消打印任务失败';
      message.error(errorMessage);
    } finally {
      setCancellingJobId(null);
    }
  };

  // 表格列定义
  const columns = [
    {
      title: '任务ID',
      dataIndex: 'id',
      key: 'id',
      width: 120,
      render: (text: string) => (
        <span title={text}>
          {text ? text.substring(0, 8) + '...' : '-'}
        </span>
      ),
    },
    {
      title: '任务名称',
      dataIndex: 'name',
      key: 'name',
      width: 200,
      render: (text: string) => (
        <Space>
          <FileTextOutlined />
          <span title={text}>
            {text && text.length > 30 ? `${text.substring(0, 30)}...` : text}
          </span>
        </Space>
      ),
    },
    {
      title: '用户',
      dataIndex: 'user_name',
      key: 'user_name',
      render: (text: string) => (
        <Space>
          <UserOutlined />
          {text || '-'}
        </Space>
      ),
    },
    {
      title: '节点ID',
      dataIndex: 'edge_node_id',
      key: 'edge_node_id',
      width: 120,
      render: (text: string) => (
        <Space>
          <ClusterOutlined />
          <span title={text}>
            {text ? text.substring(0, 8) + '...' : '-'}
          </span>
        </Space>
      ),
    },
    {
      title: '打印机ID',
      dataIndex: 'printer_id',
      key: 'printer_id',
      width: 120,
      render: (text: string) => (
        <Space>
          <PrinterOutlined />
          <span title={text}>
            {text ? text.substring(0, 8) + '...' : '-'}
          </span>
        </Space>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => getStatusTag(status),
    },
    {
      title: '页数',
      dataIndex: 'page_count',
      key: 'page_count',
      render: (count: number) => count || '-',
    },
    {
      title: '份数',
      dataIndex: 'copies',
      key: 'copies',
      render: (copies: number) => copies || 1,
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 170,
      render: (time: string) => {
        if (!time) return '-';
        const date = new Date(time);
        return date.toLocaleString('zh-CN');
      },
    },
    {
      title: '文件大小',
      dataIndex: 'file_size',
      key: 'file_size',
      render: (size: number) => {
        if (!size) return '-';
        if (size < 1024) return `${size} B`;
        if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
        return `${(size / (1024 * 1024)).toFixed(1)} MB`;
      },
    },
    {
      title: '操作',
      key: 'actions',
      width: 120,
      render: (_: unknown, record: PrintJob) => {
        if (!canCancel(record.status)) {
          return '-';
        }

        return (
          <Popconfirm
            title="确认取消该打印任务？"
            description="取消后任务将在云端立即结束，不会再参与重发。"
            okText="确认"
            cancelText="取消"
            onConfirm={() => handleCancelJob(record)}
          >
            <Button
              type="link"
              danger
              loading={cancellingJobId === record.id}
            >
              取消任务
            </Button>
          </Popconfirm>
        );
      },
    },
  ];

  return (
    <div style={{ padding: '24px' }}>
      <h2>打印任务管理</h2>
      
      <Card>
        {/* 筛选和操作栏 */}
        <div style={{ marginBottom: '16px', display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '8px' }}>
          <Space wrap>
            <Select
              placeholder="选择状态"
              allowClear
              style={{ width: 130 }}
              value={statusFilter || undefined}
              onChange={handleStatusChange}
            >
              <Select.Option value="pending">等待中</Select.Option>
              <Select.Option value="dispatched">已分发</Select.Option>
              <Select.Option value="printing">打印中</Select.Option>
              <Select.Option value="completed">已完成</Select.Option>
              <Select.Option value="failed">失败</Select.Option>
            </Select>
            
            <Select
              placeholder="选择节点"
              allowClear
              style={{ width: 180 }}
              value={edgeNodeFilter || undefined}
              onChange={handleEdgeNodeChange}
              showSearch
              optionFilterProp="children"
            >
              {edgeNodes.map(node => (
                <Select.Option key={node.id} value={node.id}>
                  {node.display_name || node.name}
                </Select.Option>
              ))}
            </Select>
            
            <RangePicker
              showTime={{ format: 'HH:mm' }}
              format="YYYY-MM-DD HH:mm"
              placeholder={['开始时间', '结束时间']}
              value={dateRange}
              onChange={handleDateRangeChange}
            />
          </Space>
          
          <Space>
            <Button 
              icon={<ReloadOutlined />} 
              onClick={handleRefresh}
              loading={loading}
            >
              刷新
            </Button>
          </Space>
        </div>

        {/* 打印任务表格 */}
        <Table
          columns={columns}
          dataSource={printJobs}
          loading={loading}
          pagination={{
            current: currentPage,
            pageSize: pageSize,
            total: total,
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total, range) =>
              `第 ${range[0]}-${range[1]} 条，共 ${total} 条`,
            onChange: handleTableChange,
            onShowSizeChange: handleTableChange,
            pageSizeOptions: ['10', '20', '50', '100'],
          }}
          size="middle"
          scroll={{ x: 1400 }}
        />
      </Card>
    </div>
  );
};

export default PrintJobs;
