package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func ListDepartments(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	keyword := c.Query("keyword")
	var parentId *int
	if pidStr := c.Query("parent_id"); pidStr != "" {
		if pid, err := strconv.Atoi(pidStr); err == nil {
			parentId = &pid
		}
	}
	depts, total, err := model.GetAllDepartments(pageInfo, keyword, parentId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": depts, "total": total})
}

func GetDepartmentTree(c *gin.Context) {
	tree, err := model.GetDepartmentTree()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": tree})
}

func GetDepartment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, errors.New("无效的部门ID"))
		return
	}
	dept, err := model.GetDepartmentByID(id)
	if err != nil {
		common.ApiError(c, errors.New("部门不存在"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": dept})
}

func CreateDepartment(c *gin.Context) {
	var req struct {
		Name          string  `json:"name"`
		ParentId      *int    `json:"parent_id"`
		Quota         int     `json:"quota"`
		OversellLimit int     `json:"oversell_limit"`
		MonthlyQuota  int     `json:"monthly_quota"`
		Ratio         float64 `json:"ratio"`
		Status        int     `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Name == "" {
		common.ApiError(c, errors.New("部门名称不能为空"))
		return
	}
	if req.Ratio == 0 {
		req.Ratio = 1
	}
	if req.Status == 0 {
		req.Status = 1
	}
	if req.ParentId != nil && *req.ParentId != 0 {
		if _, err := model.GetDepartmentByID(*req.ParentId); err != nil {
			common.ApiError(c, errors.New("父部门不存在"))
			return
		}
	}
	dept, err := model.InsertDepartment(req.Name, req.ParentId, req.Quota, req.OversellLimit, req.Ratio, req.Status, req.MonthlyQuota)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "部门创建成功", "data": dept})
}

func UpdateDepartment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, errors.New("无效的部门ID"))
		return
	}
	dept, err := model.GetDepartmentByID(id)
	if err != nil {
		common.ApiError(c, errors.New("部门不存在"))
		return
	}
	var req struct {
		Name          string  `json:"name"`
		ParentId      *int    `json:"parent_id"`
		OversellLimit int     `json:"oversell_limit"`
		MonthlyQuota  int     `json:"monthly_quota"`
		Ratio         float64 `json:"ratio"`
		Status        int     `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Name != "" {
		dept.Name = req.Name
	}
	if req.ParentId != nil && *req.ParentId == dept.Id {
		common.ApiError(c, errors.New("不能将自身设为父部门"))
		return
	}
	if req.ParentId != nil && *req.ParentId != 0 {
		if isCircularRef(dept.Id, *req.ParentId) {
			common.ApiError(c, errors.New("不能将后代部门设为父部门（循环引用）"))
			return
		}
	}
	dept.ParentId = req.ParentId
	dept.OversellLimit = req.OversellLimit
	dept.Ratio = req.Ratio
	dept.Status = req.Status
	if err := dept.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "部门更新成功", "data": dept})
}

func isCircularRef(departmentId, targetParentId int) bool {
	chain, err := model.GetDepartmentChain(targetParentId)
	if err != nil {
		return true
	}
	for _, d := range chain {
		if d.Id == departmentId {
			return true
		}
	}
	return false
}

func DeleteDepartment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, errors.New("无效的部门ID"))
		return
	}
	children, err := model.GetDirectChildren(id)
	if err == nil && len(children) > 0 {
		common.ApiError(c, errors.New("该部门下还有子部门，请先删除或移动子部门"))
		return
	}
	dept, err := model.GetDepartmentByID(id)
	if err != nil {
		common.ApiError(c, errors.New("部门不存在"))
		return
	}
	if err := dept.Delete(); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "部门已删除"})
}

func RechargeDepartment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, errors.New("无效的部门ID"))
		return
	}
	var req struct {
		Amount int `json:"amount"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Amount <= 0 {
		common.ApiError(c, errors.New("充值额度必须大于0"))
		return
	}
	if err := model.RechargeDepartment(id, req.Amount); err != nil {
		common.ApiError(c, err)
		return
	}
	logger.LogInfo(c, "部门充值成功, departmentId="+strconv.Itoa(id)+", amount="+strconv.Itoa(req.Amount))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "充值成功"})
}

func GetDepartmentMembers(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, errors.New("无效的部门ID"))
		return
	}
	pageInfo := common.GetPageQuery(c)
	users, total, err := model.GetDepartmentMembers(id, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": users, "total": total})
}

func GetDepartmentNames(c *gin.Context) {
	depts, err := model.GetAllDepartmentNames()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": depts})
}

func GetDepartmentUsage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, errors.New("无效的部门ID"))
		return
	}
	dept, err := model.GetDepartmentByID(id)
	if err != nil {
		common.ApiError(c, errors.New("部门不存在"))
		return
	}
	descendantIds, err := model.GetDepartmentDescendantIds(id)
	if err != nil {
		descendantIds = []int{id}
	}
	var treeQuota, treeUsedQuota int64
	for _, did := range descendantIds {
		d, err := model.GetDepartmentByID(did)
		if err != nil {
			continue
		}
		treeQuota += int64(d.Quota)
		treeUsedQuota += int64(d.UsedQuota)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"department":       dept,
			"tree_quota":       treeQuota,
			"tree_used_quota":  treeUsedQuota,
			"descendant_count": len(descendantIds) - 1,
		},
	})
}
