# 分组 → 部门化改造方案（v2：多级部门）

## 一、现状

当前「分组」本质是倍率系统——一个 JSON 字典 `{"default": 1.0, "vip": 0.8}`。用户表里 `Group` 字符串字段标识归属。调用 API 时：

> 消耗额度 = tokens × 模型倍率 × 分组倍率

所有额度扣的是 `User.Quota` 个人钱包。没有共享池、没有超卖、没有部门实体。

---

## 二、目标

引入 Department 作为独立实体，核心特征：

1. **多级树形结构**：部门支持 `ParentId`，形成树。消费从叶子向上逐级 fallback
2. **每级独立额度池**：每级 Department 都有 quota/used_quota/oversell_limit，独立核算
3. **向上追溯消费**：叶子不够 → 父级 → 祖父级 → ... → 根。任意一级成功即放行
4. **倍率保留**：用户所在叶子部门的 Ratio 作为计费倍率
5. **兼容已有逻辑**：未分配部门的用户走个人钱包，不破坏现有流程

---

## 三、数据模型

### 3.1 Department 表（v2）

```go
type Department struct {
    Id            int            `json:"id"`
    Name          string         `json:"name" gorm:"uniqueIndex;size:64"`
    ParentId      *int           `json:"parent_id" gorm:"index;default:null"` // 父部门ID，nil=顶级
    Quota         int            `json:"quota" gorm:"default:0"`
    UsedQuota     int            `json:"used_quota" gorm:"default:0"`
    OversellLimit int            `json:"oversell_limit" gorm:"default:0"`
    Ratio         float64        `json:"ratio" gorm:"default:1"`
    Status        int            `json:"status" gorm:"default:1"`
    CreatedAt     int64          `json:"created_at"`
    UpdatedAt     int64          `json:"updated_at"`
    DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
}
```

### 3.2 DepartmentQuotaLog 表（不变）

```go
type DepartmentQuotaLog struct {
    Id           int   `json:"id"`
    DepartmentId int   `json:"department_id" gorm:"index"`
    UserId       int   `json:"user_id"`
    Delta        int   `json:"delta"`
    Reason       string `json:"reason"`
    CreatedAt    int64  `json:"created_at"`
}
```

### 3.3 User 表（不变）

```go
DepartmentId *int `json:"department_id" gorm:"index;default:null"`
```

---

## 四、消费流程改造（多级 fallback）

### 4.1 核心算法

```
用户挂载在部门 D（任意层级，通常是叶子）

PreConsume(amount):
  chain = [D, D.parent, D.parent.parent, ...]   // 叶子到根
  remaining = amount

  for each dept in chain:
    if remaining <= 0: break
    tx.Begin()
    SELECT ... FOR UPDATE
    consumable = min(remaining, dept.quota - dept.used_quota + dept.oversell_limit)
    if consumable > 0:
      扣减 dept.used_quota += consumable
      记录 DepartmentQuotaLog(dept.id, -consumable)
      remaining -= consumable
    tx.Commit()

  if remaining > 0:
    回滚所有已扣减的事务 → 返回 "insufficient department quota"
  else:
    记录每个 dept 的扣减量 (consumedByDept map[deptId]amount)
    d.consumedByDept = consumedByDept
```

**关键点：**
- 每个部门的扣减在独立事务中执行（或同一大事务中逐级 FOR UPDATE）
- 超卖判断在每级独立计算：`used_quota + amount - quota <= oversell_limit`
- `PreConsume` 的返回值包含 `consumedByDept` map，供 `Settle` / `Refund` 使用

### 4.2 Settle(delta)

```
Settle(delta):
  // delta > 0: 额外扣减 → 从叶子部门扣
  // delta < 0: 退还 → 按 consumedByDept 逆序退还（优先还原叶子）
  if delta > 0:
    ConsumeDepartmentQuota(user所在叶子部门ID, delta)
  else:
    remaining = -delta
    // 逆序遍历 consumedByDept（根→叶子），逐级退还
    for deptId in reverse(chain):
      refundable = min(remaining, consumedByDept[deptId])
      退还 dept.used_quota -= refundable
      remaining -= refundable
```

### 4.3 Refund()

```
Refund():
  // 全额退还 consumedByDept 中所有部门的扣减
  for deptId, amount in consumedByDept:
    IncreaseDepartmentQuota(deptId, amount)
```

---

## 五、部门管理 API 补充

在原有 CRUD 基础上新增：

| 方法 | 路径 | 说明 | 权限 |
|---|---|---|---|
| GET | `/api/department/tree` | 获取完整部门树 | admin |

`GET /api/department/tree` 返回格式：
```json
[
  {
    "id": 1,
    "name": "总公司",
    "parent_id": null,
    "quota": 1000000,
    "used_quota": 500000,
    "oversell_limit": 200000,
    "ratio": 1.0,
    "status": 1,
    "children": [
      {
        "id": 2,
        "name": "研发部",
        "parent_id": 1,
        ...
        "children": [...]
      }
    ]
  }
]
```

原有 `GET /api/department/` 列表接口保持不变（平铺分页），新增 `parent_id` 参数支持按父级筛选。

---

## 六、关键查询方法

### model/department.go

```go
// GetDepartmentChain 获取从指定部门到根的完整链路（叶子→根）
func GetDepartmentChain(departmentId int) ([]Department, error)

// GetDepartmentTree 获取完整部门树
func GetDepartmentTree() ([]*DepartmentTreeNode, error)

// GetDirectChildren 获取直接子部门
func GetDirectChildren(parentId int) ([]Department, error)

// GetDepartmentStats 获取部门子树统计（递归汇总所有后代的 used_quota）
func GetDepartmentStats(departmentId int) (quota int, usedQuota int, memberCount int, err error)
```

---

## 七、前端改动（classic）

### 7.1 部门管理页面

- 部门列表改为**树形 Table**（Semi Design 的 Table 支持 `children` 属性实现可展开行）
- 创建/编辑时新增「上级部门」下拉（可选，null = 顶级）
- 充值可针对任意层级

### 7.2 用户管理页面

- 部门选择改为**级联选择器**（如 `Cascader`），展示完整路径（如 "总公司 / 研发部 / 后端组"）
- 部门列展示完整路径（缩短显示，hover tooltip 显示全路径）

### 7.3 侧边栏

- 不变，仍然是「部门管理」入口

---

## 八、涉及文件总结

| 层 | 新增文件 | 修改文件 |
|---|---|---|
| model | — | `department.go`（加 ParentId、树查询、链路查询） |
| service | — | `funding_source.go`（DepartmentFunding 改为多级 fallback） |
| controller | — | `department.go`（加 tree 接口、创建/编辑支持 parent_id） |
| router | — | `api-router.go`（加 tree 路由） |
| classic 前端 | `pages/Department/index.jsx` 及子组件 | `UsersColumnDefs`, `EditUserModal`, `SiderBar`, `App.jsx`, i18n |

---

## 九、向后兼容

- 已有单级部门数据：`parent_id` 全部为 null，行为与 v1 完全一致
- `GetDepartmentChain` 对顶级部门返回 `[self]`
- 用户没有 `department_id` 时，完全不变
