package service

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"

	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const (
	BillingSourceWallet       = "wallet"
	BillingSourceSubscription = "subscription"
	BillingSourceDepartment  = "department"
)

// PreConsumeBilling 根据用户计费偏好创建 BillingSession 并执行预扣费。
// 会话存储在 relayInfo.Billing 上，供后续 Settle / Refund 使用。
func PreConsumeBilling(c *gin.Context, preConsumedQuota int, relayInfo *relaycommon.RelayInfo) *types.NewAPIError {
	session, apiErr := NewBillingSession(c, relayInfo, preConsumedQuota)
	if apiErr != nil {
		return apiErr
	}
	relayInfo.Billing = session
	return nil
}

// ---------------------------------------------------------------------------
// SettleBilling — 后结算辅助函数
// ---------------------------------------------------------------------------

// SettleBilling 执行计费结算。如果 RelayInfo 上有 BillingSession 则通过 session 结算，
// 否则回退到旧的 PostConsumeQuota 路径（兼容按次计费等场景）。
func SettleBilling(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, actualQuota int) error {
	if relayInfo.Billing != nil {
		preConsumed := relayInfo.Billing.GetPreConsumedQuota()
		delta := actualQuota - preConsumed

		if delta > 0 {
			logger.LogInfo(ctx, fmt.Sprintf("预扣费后补扣费：%s（实际消耗：%s，预扣费：%s）",
				logger.FormatQuota(delta),
				logger.FormatQuota(actualQuota),
				logger.FormatQuota(preConsumed),
			))
		} else if delta < 0 {
			logger.LogInfo(ctx, fmt.Sprintf("预扣费后返还扣费：%s（实际消耗：%s，预扣费：%s）",
				logger.FormatQuota(-delta),
				logger.FormatQuota(actualQuota),
				logger.FormatQuota(preConsumed),
			))
		} else {
			logger.LogInfo(ctx, fmt.Sprintf("预扣费与实际消耗一致，无需调整：%s（按次计费）",
				logger.FormatQuota(actualQuota),
			))
		}

		if err := relayInfo.Billing.Settle(actualQuota); err != nil {
			return err
		}

		// 发送额度通知（订阅计费使用订阅剩余额度）
		if actualQuota != 0 {
			if relayInfo.BillingSource == BillingSourceSubscription {
				checkAndSendSubscriptionQuotaNotify(relayInfo)
			} else {
				checkAndSendQuotaNotify(relayInfo, actualQuota-preConsumed, preConsumed)
			}
		}
		return nil
	}

	// 回退：无 BillingSession 时使用旧路径
	quotaDelta := actualQuota - relayInfo.FinalPreConsumedQuota
	if quotaDelta != 0 {
		return PostConsumeQuota(relayInfo, quotaDelta, relayInfo.FinalPreConsumedQuota, true)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Department quota billing
// ---------------------------------------------------------------------------

// PreConsumeDepartmentQuota checks department chain quota before billing.
// For users assigned to a department, it walks the department chain (leaf→root)
// and verifies each level has sufficient quota (including oversell_limit).
// The chain IDs are stored on relayInfo for use in Settle/Refund.
func PreConsumeDepartmentQuota(c *gin.Context, preConsumedQuota int, relayInfo *relaycommon.RelayInfo) *types.NewAPIError {
	// Query user's department_id from DB since it's not pre-loaded in relayInfo
	user, err := model.GetUserById(relayInfo.UserId, false)
	if err != nil {
		return nil // user not found, skip department check
	}
	if user.DepartmentId == nil || *user.DepartmentId == 0 {
		return nil // user not in any department, skip
	}
	relayInfo.DepartmentId = user.DepartmentId

	chain, err := model.GetDepartmentChain(*relayInfo.DepartmentId)
	if err != nil {
		return types.NewError(
			fmt.Errorf("获取部门链路失败: %w", err),
			types.ErrorCodeQueryDataError,
			types.ErrOptionWithSkipRetry(),
		)
	}

	ids := make([]int, 0, len(chain))
	for _, dept := range chain {
		if dept.Status != 1 {
			return types.NewErrorWithStatusCode(
				fmt.Errorf("部门 %s 已停用", dept.Name),
				types.ErrorCodeInsufficientUserQuota, 403,
				types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog(),
			)
		}
		available := dept.Quota + dept.OversellLimit - dept.UsedQuota
		if available < preConsumedQuota {
			return types.NewErrorWithStatusCode(
				fmt.Errorf("部门 %s 额度不足, 剩余: %s, 需要: %s",
					dept.Name, logger.FormatQuota(available), logger.FormatQuota(preConsumedQuota)),
				types.ErrorCodeInsufficientUserQuota, 403,
				types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog(),
			)
		}
		ids = append(ids, dept.Id)
	}
	relayInfo.DepartmentChainIds = ids
	return nil
}


// SettleDepartmentQuota deducts quota from the department chain.
// Must be called AFTER the billing session has settled.
func SettleDepartmentQuota(c *gin.Context, relayInfo *relaycommon.RelayInfo, actualQuota int) error {
	if len(relayInfo.DepartmentChainIds) == 0 || actualQuota <= 0 {
		return nil
	}

	db := model.DB
	return db.Transaction(func(tx *gorm.DB) error {
		for _, deptId := range relayInfo.DepartmentChainIds {
			if err := model.ConsumeDepartmentQuota(tx, deptId, actualQuota, relayInfo.UserId, "API调用消费"); err != nil {
				return fmt.Errorf("扣减部门 %d 额度失败: %w", deptId, err)
			}
		}
		return nil
	})
}

// RefundDepartmentQuota restores department quota on relay failure.
func RefundDepartmentQuota(c *gin.Context, relayInfo *relaycommon.RelayInfo, preConsumedQuota int) {
	if len(relayInfo.DepartmentChainIds) == 0 || preConsumedQuota <= 0 {
		return
	}

	db := model.DB
	for _, deptId := range relayInfo.DepartmentChainIds {
		if err := model.IncreaseDepartmentQuota(db, deptId, preConsumedQuota, relayInfo.UserId, "API调用退款"); err != nil {
			common.SysLog(fmt.Sprintf("退还部门 %d 额度失败: %s", deptId, err.Error()))
		}
	}
}
