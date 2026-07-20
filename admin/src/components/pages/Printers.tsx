import React, { useState, useEffect } from 'react';
import { Card, Table, Tag, Space, message, Select, Button, Popconfirm, Modal, Form, Input } from 'antd';
import { 
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  StopOutlined,
  PlayCircleOutlined,
  DeleteOutlined,
  EditOutlined
} from '@ant-design/icons';
import { buildApiUrl, buildAuthUrl } from '../../config';

// 打印机接口（适配后端数据模型）
interface PrinterStatus {
  id: string;
  name: string;
  display_name?: string;
  model: string;
  printer_status: string;
  status_received_at?: string;
  status_stale?: boolean;
  enabled: boolean;
  edge_node_id: string;
  edge_node_name?: string; // Edge Node 名称
  key?: string;
}

// Edge Node 接口
interface EdgeNode {
  id: string;
  name: string;
}

// Printers 服务类
class PrintersService {
  private async getToken(): Promise<string | null> {
    try {
      const response = await fetch(buildAuthUrl('me'));
      const result = await response.json();
      
      if (result.code === 200 && result.data.access_token) {
        return result.data.access_token;
      }
    } catch (error) {
      console.error('获取 token 失败:', error);
    }
    
    return null;
  }

  async getEdgeNodes(): Promise<EdgeNode[]> {
    try {
      const token = await this.getToken();
      const response = await fetch(buildApiUrl('/admin/edge-nodes'), {
        headers: {
          ...(token && { 'Authorization': `Bearer ${token}` }),
        },
      });
      
      if (response.ok) {
        const result = await response.json();
        return result.data.items || [];
      }
    } catch (error) {
      console.error('获取边缘节点列表失败:', error);
    }
    
    return [];
  }

  async getPrintersWithEdgeNodes(): Promise<{ printers: PrinterStatus[], edgeNodes: EdgeNode[] }> {
    try {
      const token = await this.getToken();
      
      // 同时获取打印机和边缘节点数据
      const [printersResponse, edgeNodesResponse] = await Promise.all([
        fetch(buildApiUrl('/admin/printers'), {
          headers: { ...(token && { 'Authorization': `Bearer ${token}` }) },
        }),
        fetch(buildApiUrl('/admin/edge-nodes'), {
          headers: { ...(token && { 'Authorization': `Bearer ${token}` }) },
        })
      ]);
      
      const printers = printersResponse.ok ? (await printersResponse.json()).data.items || [] : [];
      const edgeNodes = edgeNodesResponse.ok ? (await edgeNodesResponse.json()).data.items || [] : [];
      
      // 创建 Edge Node 映射
      const edgeNodeMap: { [key: string]: string } = {};
      edgeNodes.forEach((node: EdgeNode) => {
        edgeNodeMap[node.id] = node.name;
      });
      
      // 合并数据
      const printersWithEdgeNode = printers.map((printer: any) => ({
        ...printer,
        edge_node_name: edgeNodeMap[printer.edge_node_id] || printer.edge_node_id
      }));
      
      return { printers: printersWithEdgeNode, edgeNodes };
    } catch (error) {
      console.error('获取数据失败:', error);
      throw error;
    }
  }

  async getPrinters(): Promise<PrinterStatus[]> {
    try {
      const token = await this.getToken();
      const response = await fetch(buildApiUrl('/admin/printers'), {
        headers: {
          ...(token && { 'Authorization': `Bearer ${token}` }),
        },
      });
      
      if (response.ok) {
        const result = await response.json();
        return result.data.items || [];
      }
      } catch (error) {
        console.error('获取打印机列表失败:', error);
        throw error;
      }
      
      return [];
  }
}

const printersService = new PrintersService();

// Printers 组件
const Printers: React.FC = () => {
  const [printers, setPrinters] = useState<PrinterStatus[]>([]);
  const [edgeNodes, setEdgeNodes] = useState<EdgeNode[]>([]);
  const [filteredPrinters, setFilteredPrinters] = useState<PrinterStatus[]>([]);
  const [selectedEdgeNode, setSelectedEdgeNode] = useState<string>('');
  const [loading, setLoading] = useState(true);
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  
  // 搜索和筛选状态
  const [searchName, setSearchName] = useState<string>('');
  const [selectedStatus, setSelectedStatus] = useState<string>('');
  
  // 别名编辑相关状态
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [editingPrinter, setEditingPrinter] = useState<PrinterStatus | null>(null);
  const [form] = Form.useForm();

  // 获取token
  const getToken = async (): Promise<string | null> => {
    try {
      const response = await fetch(buildAuthUrl('me'));
      const result = await response.json();
      
      if (result.code === 200 && result.data.access_token) {
        return result.data.access_token;
      }
      return null;
    } catch (error) {
      console.error('获取token失败:', error);
      return null;
    }
  };

  // 删除打印机
  const handleDeletePrinter = async (printerId: string, printerName: string) => {
    try {
      const token = await getToken();
      if (!token) {
        message.error('未找到认证令牌，请重新登录');
        return;
      }

      const response = await fetch(buildApiUrl(`/admin/printers/${printerId}`), {
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
      });

      if (response.ok) {
        message.success(`打印机 ${printerName} 删除成功`);
        loadData(); // 重新加载数据
      } else {
        const error = await response.json();
        message.error(`删除失败: ${error.message || '未知错误'}`);
      }
    } catch (error) {
      console.error('删除打印机失败:', error);
      message.error('删除失败，请稍后重试');
    }
  };

  // 编辑打印机别名
  const handleEditPrinter = (printer: PrinterStatus) => {
    setEditingPrinter(printer);
    form.setFieldsValue({
      display_name: printer.display_name || printer.name
    });
    setEditModalVisible(true);
  };

  // 提交别名修改
  const handleEditSubmit = async (values: { display_name: string }) => {
    if (!editingPrinter) return;

    try {
      const token = await getToken();
      if (!token) {
        message.error('未找到认证令牌，请重新登录');
        return;
      }

      const response = await fetch(buildApiUrl(`/admin/printers/${editingPrinter.id}`), {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({
          display_name: values.display_name.trim()
        }),
      });

      if (response.ok) {
        message.success('打印机别名修改成功');
        setEditModalVisible(false);
        setEditingPrinter(null);
        form.resetFields();
        loadData(); // 重新加载数据
      } else {
        const error = await response.json();
        message.error(`修改失败: ${error.message || '未知错误'}`);
      }
    } catch (error) {
      console.error('修改打印机别名失败:', error);
      message.error('修改失败，请稍后重试');
    }
  };

  // 切换打印机启用/禁用状态
  const handleToggleEnabled = async (printer: PrinterStatus) => {
    try {
      const token = await getToken();
      if (!token) {
        message.error('未找到认证令牌，请重新登录');
        return;
      }

      const newEnabled = !printer.enabled;
      const response = await fetch(buildApiUrl(`/admin/printers/${printer.id}`), {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({
          enabled: newEnabled
        }),
      });

      if (response.ok) {
        message.success(`打印机已${newEnabled ? '启用' : '禁用'}`);
        loadData(); // 重新加载数据
      } else {
        const error = await response.json();
        message.error(`操作失败: ${error.message || '未知错误'}`);
      }
    } catch (error) {
      console.error('切换打印机状态失败:', error);
      message.error('操作失败，请稍后重试');
    }
  };

  // 加载数据函数
  const loadData = async () => {
    try {
      setLoading(true);
      const { printers: printerList, edgeNodes: edgeNodeList } = await printersService.getPrintersWithEdgeNodes();
      
      const printersWithKey = printerList.map(printer => ({ ...printer, key: printer.id }));
      setPrinters(printersWithKey);
      setEdgeNodes(edgeNodeList);
      setFilteredPrinters(printersWithKey);
    } catch (error) {
      console.error('加载数据失败:', error);
      message.error('加载打印机数据失败，请稍后重试');
      // 设置为空数组
      setPrinters([]);
      setEdgeNodes([]);
      setFilteredPrinters([]);
    } finally {
      setLoading(false);
    }
  };

  // 初始加载数据
  useEffect(() => {
    loadData();
	const timer = window.setInterval(() => { if (!document.hidden) loadData(); }, 30000);
	return () => window.clearInterval(timer);
  }, []);

  // 综合筛选逻辑：Edge Node + 状态 + 名称搜索
  useEffect(() => {
    let result = printers;
    
    // 按Edge Node筛选
    if (selectedEdgeNode) {
      result = result.filter(printer => printer.edge_node_id === selectedEdgeNode);
    }
    
    // 按状态筛选
    if (selectedStatus) {
      result = result.filter(printer => printer.printer_status === selectedStatus);
    }
    
    // 按名称搜索（不区分大小写，匹配名称、别名和ID）
    if (searchName) {
      const keyword = searchName.toLowerCase();
      result = result.filter(printer => 
        (printer.display_name || '').toLowerCase().includes(keyword) ||
        printer.name.toLowerCase().includes(keyword) ||
        printer.id.toLowerCase().includes(keyword)
      );
    }
    
    setFilteredPrinters(result);
  }, [selectedEdgeNode, selectedStatus, searchName, printers]);

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'idle': return 'success';
      case 'printing': return 'processing';
      case 'printer_state_unknown': return 'default';
      case 'printer_unconfirmed_lock': return 'warning';
      case 'printer_out_of_paper':
      case 'printer_jammed':
      case 'printer_out_of_toner':
      case 'printer_cover_open':
      case 'printer_offline':
      case 'ipp_unreachable':
      case 'printer_stopped':
      case 'printer_not_accepting_jobs':
      case 'printer_user_intervention':
      case 'printer_other_fault': return 'error';
      default: return 'default';
    }
  };

  const getStatusText = (status: string) => {
    switch (status) {
      case 'idle': return '空闲';
      case 'printing': return '打印中';
      case 'printer_out_of_paper': return '缺纸';
      case 'printer_jammed': return '卡纸';
      case 'printer_out_of_toner': return '耗材耗尽';
      case 'printer_cover_open': return '机盖打开';
      case 'printer_offline': return '离线';
      case 'ipp_unreachable': return '无法连接';
      case 'printer_stopped': return '已停止';
      case 'printer_not_accepting_jobs': return '拒绝任务';
      case 'printer_user_intervention': return '需要人工处理';
      case 'printer_other_fault': return '设备故障';
      case 'printer_unconfirmed_lock': return '结果待确认';
      case 'printer_state_unknown': return '状态未知';
      default: return status || '状态未知';
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'idle': return <CheckCircleOutlined />;
      case 'printing': return <PlayCircleOutlined />;
      case 'printer_state_unknown': return <StopOutlined />;
      case 'printer_unconfirmed_lock': return <ExclamationCircleOutlined />;
      case 'printer_out_of_paper':
      case 'printer_jammed':
      case 'printer_out_of_toner':
      case 'printer_cover_open':
      case 'printer_offline':
      case 'ipp_unreachable':
      case 'printer_stopped':
      case 'printer_not_accepting_jobs':
      case 'printer_user_intervention':
      case 'printer_other_fault': return <ExclamationCircleOutlined />;
      default: return <StopOutlined />;
    }
  };


  const columns = [
    {
      title: '打印机ID',
      dataIndex: 'id',
      key: 'id',
      render: (text: string) => text || '-',
      width: 220,
    },
    {
      title: '打印机名称',
      dataIndex: 'name',
      key: 'name',
      render: (text: string, record: PrinterStatus) => (
        <div>
          <strong>{record.display_name && record.display_name.trim() !== '' ? record.display_name : text}</strong>
          {record.display_name && record.display_name.trim() !== '' && (
            <div style={{ fontSize: '12px', color: '#666' }}>
              {text}
            </div>
          )}
        </div>
      ),
      width: 200,
    },
    {
      title: '型号',
      dataIndex: 'model',
      key: 'model',
      width: 200,
    },
    {
      title: '所属边缘节点',
      dataIndex: 'edge_node_name',
      key: 'edge_node_name',
      width: 180,
      render: (text: string) => text || '未知',
    },
    {
      title: '当前状态',
      dataIndex: 'printer_status',
      key: 'printer_status',
      width: 120,
      render: (status: string, record: PrinterStatus) => (
        <Tag color={getStatusColor(status)} icon={getStatusIcon(status)} title={record.status_stale ? '状态超过90秒未更新' : record.status_received_at ? `状态更新时间：${record.status_received_at}` : undefined}>
          {getStatusText(status)}
          {record.status_stale ? '（未更新）' : ''}
        </Tag>
      ),
    },
    {
      title: '启用状态',
      key: 'enabled_status',
      width: 120,
      render: (_, record: PrinterStatus) => {
		return (
		  <div>
			<Tag color={record.enabled ? 'green' : 'red'}>
			  {record.enabled ? '已启用' : '已禁用'}
			</Tag>
		  </div>
        );
      },
    },
    {
      title: '操作',
      key: 'action',
      width: 100,
      render: (_, record: PrinterStatus) => (
        <Space size="small">
          <Button 
            type="text" 
            size="small"
            icon={<EditOutlined />}
            onClick={() => handleEditPrinter(record)}
          >
            编辑
          </Button>
          <Button 
            type="text" 
            size="small"
            onClick={() => handleToggleEnabled(record)}
			style={{
			  color: record.enabled ? '#ff4d4f' : '#52c41a'
			}}
		  >
			{record.enabled ? '禁用' : '启用'}
          </Button>
          <Popconfirm
            title="确认删除"
            description={`确定要删除打印机 "${record.name}" 吗？`}
            onConfirm={() => handleDeletePrinter(record.id, record.name)}
            okText="确认"
            cancelText="取消"
          >
            <Button 
              type="text" 
              danger 
              size="small"
              icon={<DeleteOutlined />}
            >
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
        <h2 style={{ margin: 0 }}>打印机管理</h2>
        <Space>
          <a onClick={() => window.location.reload()}>刷新</a>
        </Space>
      </div>

      {/* 筛选器 */}
      <div style={{ marginBottom: 16 }}>
        <Space wrap>
          <span>边缘节点：</span>
          <Select 
            value={selectedEdgeNode} 
            onChange={setSelectedEdgeNode}
            style={{ width: 200 }}
            placeholder="选择边缘节点"
          >
            <Select.Option value="">全部</Select.Option>
            {edgeNodes.map(node => (
              <Select.Option key={node.id} value={node.id}>{node.name}</Select.Option>
            ))}
          </Select>
          
          <span style={{ marginLeft: 16 }}>当前状态：</span>
          <Select
            value={selectedStatus}
            onChange={setSelectedStatus}
            style={{ width: 120 }}
            placeholder="选择状态"
          >
            <Select.Option value="">全部</Select.Option>
            {Array.from(new Set(printers.map(printer => printer.printer_status).filter(Boolean))).map(status => (
              <Select.Option key={status} value={status}>{getStatusText(status)}</Select.Option>
            ))}
          </Select>
          
          <span style={{ marginLeft: 16 }}>搜索：</span>
          <Input.Search
            placeholder="搜索打印机名称/ID"
            value={searchName}
            onChange={(e) => setSearchName(e.target.value)}
            onSearch={setSearchName}
            style={{ width: 200 }}
            allowClear
          />
        </Space>
      </div>

      <Card>
        <Table
          columns={columns}
          dataSource={filteredPrinters}
          loading={loading}
          pagination={{
            current: currentPage,
            pageSize: pageSize,
            total: filteredPrinters.length,
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total) => `共 ${total} 台打印机`,
            onChange: (page, size) => {
              setCurrentPage(page);
              setPageSize(size || 10);
            },
            onShowSizeChange: (current, size) => {
              setCurrentPage(1);
              setPageSize(size);
            },
            pageSizeOptions: ['10', '20', '50', '100'],
          }}
          scroll={{ x: 900 }}
          locale={{
            emptyText: '暂无打印机数据'
          }}
        />
      </Card>

      {/* 统计信息已移动到顶部 */}

      {/* 编辑打印机别名模态框 */}
      <Modal
        title="编辑打印机名称"
        open={editModalVisible}
        onCancel={() => {
          setEditModalVisible(false);
          setEditingPrinter(null);
          form.resetFields();
        }}
        footer={null}
        width={500}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleEditSubmit}
        >
          <Form.Item
            name="display_name"
            label="打印机名称"
            rules={[
              { required: true, message: '请输入打印机名称' },
              { max: 100, message: '名称不能超过100个字符' }
            ]}
          >
            <Input placeholder="输入打印机名称" />
          </Form.Item>

          <Form.Item style={{ marginBottom: 0, textAlign: 'right' }}>
            <Space>
              <Button onClick={() => {
                setEditModalVisible(false);
                setEditingPrinter(null);
                form.resetFields();
              }}>
                取消
              </Button>
              <Button type="primary" htmlType="submit">
                保存
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default Printers;
