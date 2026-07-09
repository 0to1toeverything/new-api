// scripts/init_test_db.go
//
// 初始化本地测试数据库 (SQLite)
// 用法: cd new-api && go run scripts/init_test_db.go
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func main() {
	dbPath := "one-api.db"
	fmt.Printf("==> 初始化数据库: %s\n", dbPath)

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{PrepareStmt: true})
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}

	fmt.Println("==> 重建表结构...")
	db.Migrator().DropTable("abilities", "log", "token_cache", "tokens", "channels", "users", "options", "redemptions", "webauthn_credentials")

	err = db.AutoMigrate(&model.User{}, &model.Token{}, &model.Channel{}, &model.Ability{}, &model.Option{}, &model.Log{}, &model.Redemption{})
	if err != nil {
		log.Fatalf("AutoMigrate 失败: %v", err)
	}
	fmt.Println("   表结构创建完成")

	p1 := "root1234"
	admin := model.User{
		Username: "root", Password: func() string { h, _ := common.Password2Hash(p1); return h }(), DisplayName: "管理员",
		Role: common.RoleRootUser, Status: common.UserStatusEnabled, Quota: 999999999,
		AccessToken: common.GetPointer("adminrootaccesstoken000000000000000000"),
	}
	tx := db.Create(&admin)
	if tx.Error != nil {
		log.Fatalf("创建管理员失败: %v", tx.Error)
	}
	fmt.Printf("   管理员用户: root / %s (id=%d)\n", p1, admin.Id)

	p2 := "test1234"
	testUser := model.User{
		Username: "testuser", Password: func() string { h, _ := common.Password2Hash(p2); return h }(), DisplayName: "测试用户",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
		Quota: 99999999, AffCode: "aff123",
	}
	tx = db.Create(&testUser)
	if tx.Error != nil {
		log.Fatalf("创建测试用户失败: %v", tx.Error)
	}
	fmt.Printf("   测试用户: testuser / %s (id=%d)\n", p2, testUser.Id)

	// 固定令牌从环境变量读取，默认使用本地测试 key
	tk := os.Getenv("TEST_TOKEN_KEY")
	if tk == "" {
		tk = "concur00key00xxyyzz0011223344556677aabbccdd"
	}
	testToken := model.Token{
		UserId: testUser.Id, Key: tk, Status: 1, Name: "并发测试令牌",
		CreatedTime: time.Now().Unix(), ExpiredTime: -1,
		RemainQuota: 999999999, UnlimitedQuota: true, Group: "default",
	}
	tx = db.Create(&testToken)
	if tx.Error != nil {
		log.Fatalf("创建令牌失败: %v", tx.Error)
	}
	fmt.Printf("   测试令牌: %s\n", tk)

	dk := os.Getenv("DUMMY_KEY")
	if dk == "" {
		dk = "dummy-key-for-test"
	}
	dummyChannel := model.Channel{
		Type: constant.ChannelTypeDummy, Key: dk, Name: "Dummy 并发测试",
		Status: 1, Models: "test-gpt-concurrency", Group: "default",
		CreatedTime: time.Now().Unix(),
	}
	tx = db.Create(&dummyChannel)
	if tx.Error != nil {
		log.Fatalf("创建 Dummy 渠道失败: %v", tx.Error)
	}
	fmt.Printf("   Dummy 渠道: %s (id=%d, type=%d)\n", dummyChannel.Name, dummyChannel.Id, constant.ChannelTypeDummy)

	ability := model.Ability{
		Group: "default", Model: "test-gpt-concurrency",
		ChannelId: dummyChannel.Id, Enabled: true,
	}
	tx = db.Create(&ability)
	if tx.Error != nil {
		log.Fatalf("创建 Ability 失败: %v", tx.Error)
	}
	fmt.Println("   模型映射: default / test-gpt-concurrency -> Dummy 渠道")

	options := []model.Option{
		{Key: "PasswordLoginEnabled", Value: "true"},
		{Key: "RegisterEnabled", Value: "false"},
		{Key: "LogConsumeEnabled", Value: "true"},
		{Key: "MemoryCacheEnabled", Value: "true"},
		{Key: "BatchUpdateEnabled", Value: "true"},
		{Key: "GroupRatio", Value: "{\"default\": 1}"},
	}
	for _, opt := range options {
		db.Where("key = ?", opt.Key).Save(&opt)
	}
	model.DB = db
	model.InitOptionMap()
	fmt.Println("   系统配置初始化完成")

	// 标记 setup 表已初始化，避免"系统未初始化"提示
	db.Exec("INSERT OR REPLACE INTO setups (id, version, initialized_at) VALUES (1, 'dev-init', ?)", time.Now().Unix())
	fmt.Println("   系统初始化标记完成")

	fmt.Println("\n=============================================")
	fmt.Println("  ✅ 测试数据库初始化完成！")
	fmt.Println("=============================================\n")
	fmt.Println("启动服务:  go run main.go")
	fmt.Println()
	fmt.Println("并发测试:")
	fmt.Printf("  go run scripts/concurrency.go -url http://localhost:3000 -key %s -model test-gpt-concurrency -c 20 -n 200\n", tk)
	fmt.Println("\n管理后台: http://localhost:3000  (root / root1234)")
}
