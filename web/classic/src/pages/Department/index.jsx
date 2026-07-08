import React, { useState, useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess, renderQuota, isRoot } from '../../helpers';
import {
  Table, Button, Space, Tag, Modal, Form,
  Input, InputNumber, Popconfirm, SideSheet,
  Typography, Card, Avatar, Row, Col, Select
} from '@douyinfe/semi-ui';
import { IconPlus, IconEdit, IconDelete, IconUserGroup } from '@douyinfe/semi-icons';

const { Text, Title } = Typography;

const Department = ({ noWrapper = false }) => {
  const { t } = useTranslation();
  const [treeData, setTreeData] = useState([]);
  const [loading, setLoading] = useState(false);
  const [createVisible, setCreateVisible] = useState(false);
  const [editVisible, setEditVisible] = useState(false);
  const [rechargeVisible, setRechargeVisible] = useState(false);
  const [membersVisible, setMembersVisible] = useState(false);
  const [selectedDept, setSelectedDept] = useState(null);
  const [members, setMembers] = useState([]);
  const [membersTotal, setMembersTotal] = useState(0);
  const [membersLoading, setMembersLoading] = useState(false);
  const [departmentOptions, setDepartmentOptions] = useState([]);

  const loadTree = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/department/tree');
      if (res.data.success) setTreeData(res.data.data || []);
    } catch (e) { showError(e.message); }
    setLoading(false);
  };

  const loadDepartmentOptions = async () => {
    try {
      const res = await API.get('/api/department/names');
      if (res.data.success) {
        const opts = flattenDeptOptions(res.data.data || []);
        setDepartmentOptions(opts);
      }
    } catch (e) { /* ignore */ }
  };

  // Flatten tree into indented options for parent selector
  const flattenDeptOptions = (depts, level = 0) => {
    let result = [];
    for (const d of depts) {
      const prefix = level > 0 ? '  '.repeat(level) + '└ ' : '';
      result.push({ label: prefix + d.name, value: d.id });
      if (d.children) result = result.concat(flattenDeptOptions(d.children, level + 1));
    }
    return result;
  };

  useEffect(() => { loadTree(); loadDepartmentOptions(); }, []);

  const columns = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    { title: t('部门名称'), dataIndex: 'name' },
    {
      title: t('总额度'), dataIndex: 'quota',
      render: (v) => renderQuota(v || 0),
    },
    {
      title: t('已用额度'), dataIndex: 'used_quota',
      render: (v) => renderQuota(v || 0),
    },
    { title: t('超卖上限'), dataIndex: 'oversell_limit', render: (v) => renderQuota(v || 0) },
    { title: t('月度额度'), dataIndex: 'monthly_quota', render: (v) => renderQuota(v || 0) },
    { title: t('倍率'), dataIndex: 'ratio' },
    {
      title: t('状态'), dataIndex: 'status',
      render: (v) => v === 1 ? <Tag color='green' shape='circle'>{t('已启用')}</Tag> : <Tag color='red' shape='circle'>{t('已停用')}</Tag>,
    },
    {
      title: '', width: 200,
      render: (_, record) => (
        <Space>
          <Button size='small' icon={<IconUserGroup />} onClick={() => showMembers(record)}>{t('成员')}</Button>
          <Button size='small' icon={<IconEdit />} onClick={() => { setSelectedDept(record); setEditVisible(true); }}>{t('编辑')}</Button>
          {isRoot() && (
            <>
              <Button size='small' type='warning' onClick={() => { setSelectedDept(record); setRechargeVisible(true); }}>{t('充值')}</Button>
              <Popconfirm title={t('确定删除该部门？')} onConfirm={() => deleteDept(record.id)}>
                <Button size='small' type='danger' icon={<IconDelete />} />
              </Popconfirm>
            </>
          )}
        </Space>
      ),
    },
  ];

  const showMembers = async (dept) => {
    setSelectedDept(dept);
    setMembersVisible(true);
    setMembersLoading(true);
    try {
      const res = await API.get(`/api/department/${dept.id}/members?p=0&page_size=100`);
      if (res.data.success) { setMembers(res.data.data || []); setMembersTotal(res.data.total || 0); }
    } catch (e) { showError(e.message); }
    setMembersLoading(false);
  };

  const deleteDept = async (id) => {
    try {
      const res = await API.delete(`/api/department/${id}`);
      if (res.data.success) { showSuccess(t('部门已删除')); loadTree(); }
      else showError(res.data.message);
    } catch (e) { showError(e.message); }
  };

  const inner = (
    <>
      <Card className='!rounded-2xl shadow-sm border-0'>
        <div className='flex items-center justify-between mb-4'>
          <Title heading={4} className='m-0'>{t('部门管理')}</Title>
          {isRoot() && (
            <Button icon={<IconPlus />} theme='solid' onClick={() => { setSelectedDept(null); setCreateVisible(true); }}>
              {t('创建部门')}
            </Button>
          )}
        </div>
        <Table
          columns={columns}
          dataSource={treeData}
          loading={loading}
          childrenColumnName='children'
          defaultExpandAllRows
          pagination={false}
          rowKey='id'
        />
      </Card>

      {createVisible && (
        <DeptFormModal
          visible={createVisible}
          onClose={() => setCreateVisible(false)}
          onSuccess={() => { setCreateVisible(false); loadTree(); loadDepartmentOptions(); }}
          departmentOptions={departmentOptions}
          mode='create'
          t={t}
        />
      )}
      {editVisible && selectedDept && (
        <DeptFormModal
          visible={editVisible}
          onClose={() => setEditVisible(false)}
          onSuccess={() => { setEditVisible(false); loadTree(); loadDepartmentOptions(); }}
          departmentOptions={departmentOptions}
          mode='edit'
          initialValues={selectedDept}
          t={t}
        />
      )}
      {rechargeVisible && selectedDept && (
        <RechargeModal
          visible={rechargeVisible}
          onClose={() => setRechargeVisible(false)}
          onSuccess={() => { setRechargeVisible(false); loadTree(); }}
          department={selectedDept}
          t={t}
        />
      )}
      {membersVisible && selectedDept && (
        <SideSheet
          title={`${t('成员列表')} — ${selectedDept.name}`}
          visible={membersVisible}
          onCancel={() => setMembersVisible(false)}
          width={600}
        >
          <Table
            columns={[
              { title: 'ID', dataIndex: 'id' },
              { title: t('用户名'), dataIndex: 'username' },
              { title: t('显示名称'), dataIndex: 'display_name' },
              {
                title: t('角色'), dataIndex: 'role',
                render: (v) => v >= 100 ? <Tag color='orange'>{t('超级管理员')}</Tag> : v >= 10 ? <Tag color='yellow'>{t('管理员')}</Tag> : <Tag color='blue'>{t('普通用户')}</Tag>,
              },
            ]}
            dataSource={members}
            loading={membersLoading}
            rowKey='id'
            pagination={false}
          />
        </SideSheet>
      )}
    </>
  );

  if (noWrapper) {
    return inner;
  }

  return (
    <div className='mt-[60px] px-2'>
      {inner}
    </div>
  );
};

const DeptFormModal = ({ visible, onClose, onSuccess, departmentOptions, mode, initialValues, t }) => {
  const formRef = useRef(null);
  const quotaPerUnit = parseFloat(localStorage.getItem('quota_per_unit') || '500000');
  const [loading, setLoading] = useState(false);

  const submit = async (values) => {
    setLoading(true);
    try {
      let res;
      const payload = {
        name: values.name,
        parent_id: values.parent_id || null,
        quota: Math.round((Number(values.quota) || 0) * quotaPerUnit),
        oversell_limit: Math.round((Number(values.oversell_limit) || 0) * quotaPerUnit),
        monthly_quota: Math.round((Number(values.monthly_quota) || 0) * quotaPerUnit),
        ratio: values.ratio || 1,
        status: values.status ?? 1,
      };
      if (mode === 'create') {
        res = await API.post('/api/department/', payload);
      } else {
        res = await API.put(`/api/department/${initialValues.id}`, payload);
      }
      if (res.data.success) {
        showSuccess(mode === 'create' ? t('部门创建成功') : t('部门更新成功'));
        onSuccess();
      } else showError(res.data.message);
    } catch (e) { showError(e.message); }
    setLoading(false);
  };

  const initValues = mode === 'edit' ? {
    name: initialValues?.name || '',
    quota: initialValues?.quota != null ? Math.round(initialValues.quota / quotaPerUnit * 100) / 100 : 0,
    parent_id: initialValues?.parent_id || undefined,
    oversell_limit: initialValues?.oversell_limit != null ? Math.round(initialValues.oversell_limit / quotaPerUnit * 100) / 100 : 0,
    monthly_quota: initialValues?.monthly_quota != null ? Math.round(initialValues.monthly_quota / quotaPerUnit * 100) / 100 : 0,
    ratio: initialValues?.ratio || 1,
    status: initialValues?.status ?? 1,
  } : { name: '', ratio: 1, quota: 0, oversell_limit: 0, monthly_quota: 0, status: 1 };

  return (
    <Modal
      centered
      title={mode === 'create' ? t('创建部门') : t('编辑部门')}
      visible={visible}
      onCancel={onClose}
      onOk={() => formRef.current?.submitForm()}
      confirmLoading={loading}
    >
      <Form getFormApi={(api) => formRef.current = api} onSubmit={submit} initValues={initValues}>
        <Form.Input field='name' label={t('部门名称')} rules={[{ required: true, message: t('请输入部门名称') }]} />
        <Form.Select field='parent_id' label={t('上级部门')} optionList={departmentOptions} showClear placeholder={t('无（顶级部门）')} />
        <Form.InputNumber field='quota' label={t('总额度')} min={0} step={0.01} prefix='$' />
        <Form.InputNumber field='oversell_limit' label={t('超卖上限')} min={0} step={0.01} prefix='$' />
        <Form.InputNumber field='monthly_quota' label={t('月度额度')} min={0} step={0.01} prefix='$' />
        <Form.InputNumber field='ratio' label={t('倍率')} min={0.1} step={0.1} />
        <Form.Select field='status' label={t('状态')} optionList={[{ label: t('已启用'), value: 1 }, { label: t('已停用'), value: 0 }]} />
      </Form>
    </Modal>
  );
};

const RechargeModal = ({ visible, onClose, onSuccess, department, t }) => {
  const formRef = useRef(null);
  const [loading, setLoading] = useState(false);

  const quotaPerUnit = parseFloat(localStorage.getItem('quota_per_unit') || '500000');

  const submit = async (values) => {
    if (!values.amount || values.amount <= 0) return;
    setLoading(true);
    try {
      const res = await API.post(`/api/department/${department.id}/recharge`, { amount: Math.round(values.amount * quotaPerUnit) });
      if (res.data.success) { showSuccess(t('充值成功')); onSuccess(); }
      else showError(res.data.message);
    } catch (e) { showError(e.message); }
    setLoading(false);
  };

  return (
    <Modal
      centered
      title={`${t('充值')} — ${department.name}`}
      visible={visible}
      onCancel={onClose}
      onOk={() => formRef.current?.submitForm()}
      confirmLoading={loading}
    >
      <Text type='secondary' className='block mb-3'>
        {t('总额度')}: {renderQuota(department.quota || 0)} | {t('已用额度')}: {renderQuota(department.used_quota || 0)} | {t('剩余')}: {renderQuota((department.quota || 0) - (department.used_quota || 0))}
      </Text>
      <Form getFormApi={(api) => formRef.current = api} onSubmit={submit}>
        <Form.InputNumber field='amount' label={t('充值额度')} min={0.01} step={0.01} prefix='$' placeholder={t('请输入充值额度（美元）')} rules={[{ required: true, message: t('请输入充值额度') }, { validator: (_, value) => value > 0, message: t('充值额度必须大于0') }]} style={{ width: '100%' }} />
      </Form>
    </Modal>
  );
};

export default Department;
