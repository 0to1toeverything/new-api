package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"gorm.io/gorm"
)

// Department 部门实体（v2：多级树形结构）
type Department struct {
	Id            int            `json:"id"`
	Name          string         `json:"name" gorm:"uniqueIndex;size:64"`
	ParentId      *int           `json:"parent_id" gorm:"index;default:null"` // 父部门ID，nil=顶级
	Quota         int            `json:"quota" gorm:"default:0"`              // 共享额度池余额（累计充值）
	UsedQuota     int            `json:"used_quota" gorm:"default:0"`         // 已消费额度
	OversellLimit int            `json:"oversell_limit" gorm:"default:0"`     // 超卖上限
	Ratio         float64        `json:"ratio" gorm:"default:1"`              // 定价倍率
	Status        int            `json:"status" gorm:"default:1"`             // 1启用 0停用
	MonthlyQuota  int            `json:"monthly_quota" gorm:"default:0"`      // 月度刷新额度
	CreatedAt     int64          `json:"created_at"`
	UpdatedAt     int64          `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
}

// DepartmentTreeNode 树节点（JSON 序列化用）
type DepartmentTreeNode struct {
	Department
	Children []*DepartmentTreeNode `json:"children,omitempty" gorm:"-:all"`
}

// DepartmentQuotaLog 部门额度变动审计日志
type DepartmentQuotaLog struct {
	Id           int    `json:"id"`
	DepartmentId int    `json:"department_id" gorm:"index"`
	UserId       int    `json:"user_id"`
	Delta        int    `json:"delta"` // 变动量，正=增加，负=消费
	Reason       string `json:"reason"`
	CreatedAt    int64  `json:"created_at"`
}

// InsertDepartment 创建部门
func InsertDepartment(name string, parentId *int, quota int, oversellLimit int, ratio float64, status int, monthlyQuota int) (*Department, error) {
	dept := &Department{
		Name:          name,
		ParentId:      parentId,
		Quota:         quota,
		OversellLimit: oversellLimit,
		Ratio:         ratio,
		Status:        status,
		MonthlyQuota:  monthlyQuota,
		CreatedAt:     common.GetTimestamp(),
		UpdatedAt:     common.GetTimestamp(),
	}
	if err := DB.Create(dept).Error; err != nil {
		return nil, err
	}
	return dept, nil
}

// Update 更新部门信息
func (d *Department) Update() error {
	d.UpdatedAt = common.GetTimestamp()
	return DB.Model(d).Updates(map[string]interface{}{
		"name":           d.Name,
		"parent_id":      d.ParentId,
		"oversell_limit": d.OversellLimit,
		"ratio":          d.Ratio,
		"monthly_quota":  d.MonthlyQuota,
		"status":         d.Status,
		"updated_at":     d.UpdatedAt,
	}).Error
}

// Delete 软删除部门
func (d *Department) Delete() error {
	return DB.Delete(d).Error
}

// GetDepartmentByID 按 ID 查询部门
func GetDepartmentByID(id int) (*Department, error) {
	var dept Department
	err := DB.First(&dept, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &dept, nil
}

// GetDepartmentByName 按名称查询部门
func GetDepartmentByName(name string) (*Department, error) {
	var dept Department
	err := DB.Where("name = ?", name).First(&dept).Error
	if err != nil {
		return nil, err
	}
	return &dept, nil
}

// GetAllDepartments 获取所有部门列表，支持分页和搜索
func GetAllDepartments(pageInfo *common.PageInfo, keyword string, parentId *int) ([]Department, int64, error) {
	var depts []Department
	var total int64

	query := DB.Unscoped().Model(&Department{})
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	if parentId != nil {
		query = query.Where("parent_id = ?", *parentId)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&depts).Error; err != nil {
		return nil, 0, err
	}
	return depts, total, nil
}

// GetAllDepartmentNames 获取所有启用的部门名称（用于下拉）
func GetAllDepartmentNames() ([]Department, error) {
	var depts []Department
	err := DB.Where("status = ?", 1).Select("id, name, parent_id").Find(&depts).Error
	return depts, err
}

// GetDirectChildren 获取直接子部门
func GetDirectChildren(parentId int) ([]Department, error) {
	var depts []Department
	err := DB.Where("parent_id = ? AND status = 1", parentId).Find(&depts).Error
	return depts, err
}

// GetDepartmentChain 获取从指定部门到根的完整链路（叶子→根）
func GetDepartmentChain(departmentId int) ([]Department, error) {
	var chain []Department
	currentId := departmentId
	// 安全上限：最多 20 级，防止死循环
	for i := 0; i < 20; i++ {
		var dept Department
		err := DB.First(&dept, "id = ?", currentId).Error
		if err != nil {
			return nil, err
		}
		chain = append(chain, dept)
		if dept.ParentId == nil || *dept.ParentId == 0 {
			break
		}
		currentId = *dept.ParentId
	}
	return chain, nil
}

// GetDepartmentTree 获取完整部门树
func GetDepartmentTree() ([]*DepartmentTreeNode, error) {
	var allDepts []Department
	if err := DB.Order("id ASC").Find(&allDepts).Error; err != nil {
		return nil, err
	}

	// 构建 id → node 映射
	nodeMap := make(map[int]*DepartmentTreeNode, len(allDepts))
	for i := range allDepts {
		nodeMap[allDepts[i].Id] = &DepartmentTreeNode{Department: allDepts[i]}
	}

	// 组装树
	var roots []*DepartmentTreeNode
	for i := range allDepts {
		dept := allDepts[i]
		node := nodeMap[dept.Id]
		if dept.ParentId == nil || *dept.ParentId == 0 {
			roots = append(roots, node)
		} else if parent, ok := nodeMap[*dept.ParentId]; ok {
			parent.Children = append(parent.Children, node)
		} else {
			// 父级被删除或不存在的孤立节点，作为顶级
			roots = append(roots, node)
		}
	}
	return roots, nil
}

// GetDepartmentDescendantIds 获取部门的所有后代 ID（含自身），用于统计
func GetDepartmentDescendantIds(departmentId int) ([]int, error) {
	var allDepts []Department
	if err := DB.Where("status = 1").Select("id, parent_id").Find(&allDepts).Error; err != nil {
		return nil, err
	}

	// 构建 parent → children 映射
	childrenMap := make(map[int][]int)
	for _, d := range allDepts {
		if d.ParentId != nil && *d.ParentId != 0 {
			childrenMap[*d.ParentId] = append(childrenMap[*d.ParentId], d.Id)
		}
	}

	// BFS 收集所有后代
	ids := []int{departmentId}
	queue := []int{departmentId}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, child := range childrenMap[cur] {
			ids = append(ids, child)
			queue = append(queue, child)
		}
	}
	return ids, nil
}

// RechargeDepartment 为部门充值额度
func RechargeDepartment(departmentId int, delta int) error {
	if delta <= 0 {
		return errors.New("充值额度必须大于 0")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Department{}).Where("id = ?", departmentId).
			Update("quota", gorm.Expr("quota + ?", delta)).Error; err != nil {
			return err
		}
		log := &DepartmentQuotaLog{
			DepartmentId: departmentId,
			Delta:        delta,
			Reason:       "管理员充值",
			CreatedAt:    common.GetTimestamp(),
		}
		return tx.Create(log).Error
	})
}

// ConsumeDepartmentQuota 原子扣减部门共享额度（事务内 FOR UPDATE，单级）
func ConsumeDepartmentQuota(tx *gorm.DB, departmentId int, amount int, userId int, reason string) error {
	if amount <= 0 {
		return nil
	}

	var dept Department
	if err := tx.Set("gorm:query_option", "FOR UPDATE").
		First(&dept, "id = ?", departmentId).Error; err != nil {
		return err
	}

	if dept.Status != 1 {
		return errors.New("部门已停用")
	}

	newUsed := dept.UsedQuota + amount
	overshoot := newUsed - dept.Quota
	if overshoot > dept.OversellLimit {
		remaining := dept.Quota - dept.UsedQuota
		return fmt.Errorf("部门 %s 额度不足，池剩余: %s，超卖上限: %s，需要: %s",
			dept.Name,
			logger.LogQuota(remaining),
			logger.LogQuota(dept.OversellLimit),
			logger.LogQuota(amount))
	}

	if err := tx.Model(&dept).Update("used_quota", newUsed).Error; err != nil {
		return err
	}

	log := &DepartmentQuotaLog{
		DepartmentId: departmentId,
		UserId:       userId,
		Delta:        -amount,
		Reason:       reason,
		CreatedAt:    common.GetTimestamp(),
	}
	return tx.Create(log).Error
}

// IncreaseDepartmentQuota 退还部门已用额度（事务内）
func IncreaseDepartmentQuota(tx *gorm.DB, departmentId int, delta int, userId int, reason string) error {
	if delta <= 0 {
		return nil
	}
	if err := tx.Model(&Department{}).Where("id = ?", departmentId).
		Update("used_quota", gorm.Expr("used_quota - ?", delta)).Error; err != nil {
		return err
	}
	log := &DepartmentQuotaLog{
		DepartmentId: departmentId,
		UserId:       userId,
		Delta:        delta,
		Reason:       reason,
		CreatedAt:    common.GetTimestamp(),
	}
	return tx.Create(log).Error
}

// GetDepartmentMembers 获取部门成员列表
func GetDepartmentMembers(departmentId int, pageInfo *common.PageInfo) ([]User, int64, error) {
	var users []User
	var total int64

	query := DB.Unscoped().Model(&User{}).Where("department_id = ?", departmentId)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).
		Omit("password").Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}
