package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/config"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(ctx context.Context, cfg config.Config, log *slog.Logger) (*gorm.DB, *redis.Client, error) {
	var dialector gorm.Dialector
	switch cfg.DatabaseDriver {
	case "postgres":
		dialector = postgres.Open(cfg.DatabaseDSN)
	case "mysql":
		dialector = mysql.Open(cfg.DatabaseDSN)
	case "sqlite":
		dialector = sqlite.Open(cfg.DatabaseDSN)
	default:
		return nil, nil, fmt.Errorf("unsupported database driver %q", cfg.DatabaseDriver)
	}
	logLevel := logger.Warn
	if cfg.Environment == "development" {
		logLevel = logger.Info
	}
	var db *gorm.DB
	var err error
	for attempt := 1; attempt <= 20; attempt++ {
		db, err = gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logLevel)})
		if err == nil {
			sqlDB, dbErr := db.DB()
			if dbErr == nil && sqlDB.PingContext(ctx) == nil {
				break
			}
			if dbErr != nil {
				err = dbErr
			} else {
				err = sqlDB.PingContext(ctx)
			}
		}
		log.Warn("database not ready", "attempt", attempt, "error", err)
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	if err != nil {
		return nil, nil, fmt.Errorf("connect database: %w", err)
	}
	if err := migrate(db); err != nil {
		return nil, nil, err
	}
	if err := Seed(ctx, db); err != nil {
		return nil, nil, err
	}
	var redisClient *redis.Client
	if cfg.RedisAddr != "" {
		redisClient = redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword})
		if err := redisClient.Ping(ctx).Err(); err != nil {
			return nil, nil, fmt.Errorf("connect redis: %w", err)
		}
	}
	return db, redisClient, nil
}

func migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{}, &model.AuditLog{},
		&model.CaptureUnit{},
		&model.PermitRule{},
		&model.EmissionSample{},
		&model.ComplianceDecision{},
		&model.DecisionRevision{},
	)
}

func Seed(ctx context.Context, db *gorm.DB) error {
	var users int64
	if err := db.WithContext(ctx).Model(&model.User{}).Count(&users).Error; err != nil {
		return err
	}
	if users == 0 {
		password, err := bcrypt.GenerateFromPassword([]byte("Admin123!"), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		seedUsers := []model.User{
			{Username: "admin", DisplayName: "系统管理员", PasswordHash: string(password), Role: model.RoleAdmin, Active: true},
			{Username: "reviewer", DisplayName: "质量复核员", PasswordHash: string(password), Role: model.RoleReviewer, Active: true},
			{Username: "operator", DisplayName: "现场操作员", PasswordHash: string(password), Role: model.RoleOperator, Active: true},
			{Username: "viewer", DisplayName: "只读观察员", PasswordHash: string(password), Role: model.RoleViewer, Active: true},
		}
		if err := db.WithContext(ctx).Create(&seedUsers).Error; err != nil {
			return err
		}
	}

	if err := seedCaptureUnit(ctx, db); err != nil {
		return err
	}

	if err := seedPermitRule(ctx, db); err != nil {
		return err
	}

	if err := seedEmissionSample(ctx, db); err != nil {
		return err
	}

	if err := seedComplianceDecision(ctx, db); err != nil {
		return err
	}

	return nil
}

func seedCaptureUnit(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.CaptureUnit{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.CaptureUnit{

		{BaseModel: model.BaseModel{Code: "CU-001", Name: "捕集装置示例一", Status: "standby", Version: 1,
			Description: "用于启动验证和主要流程演示的捕集装置记录"}, Facility: "碳捕集装置合规运行区域1", Owner: "运行一组",
			Category: "常规", RiskLevel: "low", MetricValue: 12.5, MetricUnit: "unit",
			EffectiveAt: now.Add(0 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-514-01"},

		{BaseModel: model.BaseModel{Code: "CU-002", Name: "捕集装置示例二", Status: "running", Version: 1,
			Description: "用于启动验证和主要流程演示的捕集装置记录"}, Facility: "碳捕集装置合规运行区域2", Owner: "质量复核组",
			Category: "重点", RiskLevel: "medium", MetricValue: 25.0, MetricUnit: "%",
			EffectiveAt: now.Add(3 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-514-02"},

		{BaseModel: model.BaseModel{Code: "CU-003", Name: "捕集装置示例三", Status: "limited", Version: 1,
			Description: "用于启动验证和主要流程演示的捕集装置记录"}, Facility: "碳捕集装置合规运行区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 37.5, MetricUnit: "score",
			EffectiveAt: now.Add(6 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-514-03"},
	}
	return db.WithContext(ctx).Create(&items).Error
}

func seedPermitRule(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.PermitRule{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.PermitRule{

		{BaseModel: model.BaseModel{Code: "PR-001", Name: "许可规则示例一", Status: "draft", Version: 1,
			Description: "用于启动验证和主要流程演示的许可规则记录"}, Facility: "碳捕集装置合规运行区域1", Owner: "运行一组",
			Category: "常规", RiskLevel: "low", MetricValue: 12.5, MetricUnit: "unit",
			EffectiveAt: now.Add(0 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-514-01"},

		{BaseModel: model.BaseModel{Code: "PR-002", Name: "许可规则示例二", Status: "active", Version: 1,
			Description: "用于启动验证和主要流程演示的许可规则记录"}, Facility: "碳捕集装置合规运行区域2", Owner: "质量复核组",
			Category: "重点", RiskLevel: "medium", MetricValue: 25.0, MetricUnit: "%",
			EffectiveAt: now.Add(3 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-514-02"},

		{BaseModel: model.BaseModel{Code: "PR-003", Name: "许可规则示例三", Status: "superseded", Version: 1,
			Description: "用于启动验证和主要流程演示的许可规则记录"}, Facility: "碳捕集装置合规运行区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 37.5, MetricUnit: "score",
			EffectiveAt: now.Add(6 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-514-03"},
	}
	return db.WithContext(ctx).Create(&items).Error
}

func seedEmissionSample(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.EmissionSample{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.EmissionSample{

		{BaseModel: model.BaseModel{Code: "ES-001", Name: "排放样本示例一", Status: "collected", Version: 1,
			Description: "用于启动验证和主要流程演示的排放样本记录"}, Facility: "碳捕集装置合规运行区域1", Owner: "运行一组",
			Category: "常规", RiskLevel: "low", MetricValue: 12.5, MetricUnit: "unit",
			EffectiveAt: now.Add(0 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-514-01"},

		{BaseModel: model.BaseModel{Code: "ES-002", Name: "排放样本示例二", Status: "testing", Version: 1,
			Description: "用于启动验证和主要流程演示的排放样本记录"}, Facility: "碳捕集装置合规运行区域2", Owner: "质量复核组",
			Category: "重点", RiskLevel: "medium", MetricValue: 25.0, MetricUnit: "%",
			EffectiveAt: now.Add(3 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-514-02"},

		{BaseModel: model.BaseModel{Code: "ES-003", Name: "排放样本示例三", Status: "verified", Version: 1,
			Description: "用于启动验证和主要流程演示的排放样本记录"}, Facility: "碳捕集装置合规运行区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 37.5, MetricUnit: "score",
			EffectiveAt: now.Add(6 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-514-03"},
	}
	return db.WithContext(ctx).Create(&items).Error
}

func seedComplianceDecision(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.ComplianceDecision{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.ComplianceDecision{

		{BaseModel: model.BaseModel{Code: "CD-001", Name: "合规决定示例一", Status: "draft", Version: 1,
			Description: "用于启动验证和主要流程演示的合规决定记录"}, Facility: "碳捕集装置合规运行区域1", Owner: "运行一组",
			Category: "常规", RiskLevel: "low", MetricValue: 12.5, MetricUnit: "unit",
			EffectiveAt: now.Add(0 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-514-01"},

		{BaseModel: model.BaseModel{Code: "CD-002", Name: "合规决定示例二", Status: "review", Version: 1,
			Description: "用于启动验证和主要流程演示的合规决定记录"}, Facility: "碳捕集装置合规运行区域2", Owner: "质量复核组",
			Category: "重点", RiskLevel: "medium", MetricValue: 25.0, MetricUnit: "%",
			EffectiveAt: now.Add(3 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-514-02"},

		{BaseModel: model.BaseModel{Code: "CD-003", Name: "合规决定示例三", Status: "accepted", Version: 1,
			Description: "用于启动验证和主要流程演示的合规决定记录"}, Facility: "碳捕集装置合规运行区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 37.5, MetricUnit: "score",
			EffectiveAt: now.Add(6 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-514-03"},
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Revisions").Create(&items).Error; err != nil {
			return err
		}
		revisions := make([]model.DecisionRevision, 0, len(items))
		for _, item := range items {
			revisions = append(revisions, model.DecisionRevision{
				ComplianceDecisionID: item.ID, Version: item.Version, State: item.Status,
				Evidence: item.Evidence, Reason: "seeded demonstration decision",
				Actor: "system", RequestID: "seed-" + item.Code, CreatedAt: now,
			})
		}
		return tx.Create(&revisions).Error
	})
}
