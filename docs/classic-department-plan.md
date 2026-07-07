# Classic 前端 — 部门化改造文档（v2：多级部门）

## 一、现状

当前 classic 前端（React 18 + Vite + Semi Design）没有部门概念。backend 已完成多级部门 v2 改造：

- `Department` 表新增 `parent_id` 字段，支持树形结构
- 消费链路：叶子 → 父 → 祖父 → ... → 根，逐级 fallback
- API：列表、树、创建（支持 parent_id）、编辑（防循环引用）、充值、成员、统计、名称下拉

## 二、改造目标

1. **部门管理页面**：树形 Table 展示，支持展开/折叠；创建/编辑支持选择上级部门
2. **用户管理页面**：部门列渲染为路径（"总公司 / 研发部"），编辑时用 Cascader 选择器
3. **侧边栏**：admin/root 可见的「部门管理」入口
4. **路由**：新增 `/console/department` 路由

## 三、分步改动

### 3.1 新建：部门管理页面 `pages/Department/index.jsx`

参照 `pages/User/index.jsx` 的 table + modal 模式。

#### 3.1.1 树形列表

- 调用 `GET /api/department/tree` 获取树结构
- 使用 Semi `Table` + `children` 属性实现可展开行
- 每行展示：ID、名称、总额度、已用额度、超卖上限、倍率、状态、操作
- 支持实时展开/折叠（默认展开第一层）

#### 3.1.2 弹窗组件

**CreateDepartmentModal.jsx**
- 字段：名称、上级部门（Select/TreeSelect，可选）、超卖上限、倍率、状态
- 使用 `GET /api/department/names` 获取下拉选项（展平列表，按缩进或路径显示）
- 提交：POST `/api/department/`

**EditDepartmentModal.jsx**
- 同创建，但多了不可编辑的额度信息
- 上级部门不允许选择自己或自己的后代（backend 已有校验）

**RechargeDepartmentModal.jsx**
- 显示当前部门名称 + 剩余额度
- InputNumber 输入充值额度
- POST `/api/department/:id/recharge`

**ViewMembersModal.jsx** 或 **DepartmentMembersSideSheet.jsx**
- GET `/api/department/:id/members`
- 成员列表：ID、用户名、显示名称、角色

### 3.2 修改：用户管理页面

#### 3.2.1 `UsersColumnDefs.jsx`

「部门」列改为路径渲染：
```jsx
{
  title: t('部门'),
  dataIndex: 'department_id',
  render: (text, record) => {
    if (!record.department_id) return <Tag color='white' shape='circle'>{t('无')}</Tag>;
    const path = getDepartmentPath(record.department_id, departmentMap);
    // path 如 "总公司 / 研发部 / 后端组"
    const shortPath = path.length > 18 ? path.slice(0, 17) + '…' : path;
    return <Tooltip content={path}><Tag color='blue' shape='circle'>{shortPath}</Tag></Tooltip>;
  },
},
```

其中 `getDepartmentPath(id, map)` 需要沿 `parent_id` 递归向上拼接路径，通过 `departmentPathMap`（id→path 预计算）获取。

#### 3.2.2 `EditUserModal.jsx` / `AddUserModal.jsx`

部门选择从 `Select` 改为**级联选择**（可以用两层 Select：先选一级，再选二级；或用自定义组件）：
- 调用 `GET /api/department/tree` 展示树形结构
- 选中叶子部门发 `department_id`

Semi Design 没有现成 Cascader，需要自行实现或用 `TreeSelect` 替代（Semi 有 `TreeSelect` 组件）。

#### 3.2.3 `User/index.jsx`

`useEffect` 中新增：
- `fetchDepartments()` 调用 `/api/department/tree`，构建 `departmentPathMap`
- 构建部门下拉选项（展平树为带缩进前缀的列表）

### 3.3 修改：侧边栏 `SiderBar.jsx`

在 `adminItems` 新增：
```jsx
{
  text: t('部门管理'),
  itemKey: 'department',
  to: '/department',
  className: isAdmin() ? '' : 'tableHiddle',
},
```

`routerMap` 新增 `department: '/console/department'`

`getLucideIcon` 新增 `case 'department': return <Building2 .../>`

### 3.4 修改：路由 `App.jsx`

```jsx
import DepartmentPage from './pages/Department';

<Route
  path='/console/department'
  element={
    <AdminRoute>
      <Suspense fallback={<Loading />}>
        <DepartmentPage />
      </Suspense>
    </AdminRoute>
  }
/>
```

### 3.5 国际化

在 `web/classic/src/locales/zh.json` 和 `en.json` 中新增：

| key | zh | en |
|---|---|---|
| 部门管理 | 部门管理 | Department |
| 部门 | 部门 | Department |
| 上级部门 | 上级部门 | Parent Dept |
| 选择部门 | 选择部门 | Select Department |
| 无 | 无 | None |
| 总额度 | 总额度 | Total Quota |
| 已用额度 | 已用额度 | Used Quota |
| 超卖上限 | 超卖上限 | Oversell Limit |
| 倍率 | 倍率 | Ratio |
| 部门名称 | 部门名称 | Name |
| 充值 | 充值 | Recharge |
| 充值额度 | 充值额度 | Recharge Amount |
| 成员列表 | 成员列表 | Members |
| 创建部门 | 创建部门 | Create Department |
| 编辑部门 | 编辑部门 | Edit Department |
| 删除部门 | 删除部门 | Delete Department |
| 子树额度 | 子树额度 | Tree Quota |
| 子树已用 | 子树已用 | Tree Used |
| 后代部门数 | 后代部门数 | Descendants |
| 部门创建成功 | 部门创建成功 | Created |
| 部门更新成功 | 部门更新成功 | Updated |
| 部门已删除 | 部门已删除 | Deleted |
| 充值成功 | 充值成功 | Recharged |
| 该部门下还有子部门 | 该部门下还有子部门 | Dept has children |
| 不能将自身设为父部门 | 不能将自身设为父部门 | Cannot self-parent |
| 不能将后代部门设为父部门 | 不能将后代部门设为父部门 | Circular ref |

## 四、涉及文件

| 类型 | 文件 |
|---|---|
| **新建** | `pages/Department/index.jsx`（树形 Table + modals） |
| **新建** | `pages/Department/components/CreateDeptModal.jsx` |
| **新建** | `pages/Department/components/EditDeptModal.jsx` |
| **新建** | `pages/Department/components/RechargeDeptModal.jsx` |
| **新建** | `pages/Department/components/DeptMembersModal.jsx` |
| **新建** | `hooks/departments/useDepartmentsData.jsx` |
| **修改** | `components/table/users/UsersColumnDefs.jsx` |
| **修改** | `components/table/users/UsersFilters.jsx` |
| **修改** | `components/table/users/modals/EditUserModal.jsx` |
| **修改** | `components/table/users/modals/AddUserModal.jsx` |
| **修改** | `pages/User/index.jsx` |
| **修改** | `components/layout/SiderBar.jsx` |
| **修改** | `helpers/render.jsx` |
| **修改** | `App.jsx` |
| **修改** | `locales/zh.json`, `locales/en.json` |
