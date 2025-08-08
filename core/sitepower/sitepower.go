package sitepower

import (
	"time"

	"github.com/evcc-io/evcc/util"
	"gorm.io/gorm"
)

// SitePowerRecord 存储站点功率数据的记录
type SitePowerRecord struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CreatedAt time.Time `json:"createdAt" gorm:"column:created_at"`
	SiteTitle string    `json:"siteTitle" gorm:"column:site_title"`
	PowerKW   float64   `json:"powerKW" gorm:"column:power_kw"` // 功率，单位kW
}

// SitePowerRecords 是记录列表
type SitePowerRecords []SitePowerRecord

// DB 是SQLite数据库存储服务
type DB struct {
	log *util.Logger
	db  *gorm.DB
}

// NewStore 创建一个sitePower存储实例
func NewStore(db *gorm.DB) (*DB, error) {
	err := db.AutoMigrate(new(SitePowerRecord))

	sitePowerDB := &DB{
		log: util.NewLogger("sitepower"),
		db:  db,
	}

	return sitePowerDB, err
}

// Save 保存sitePower记录到数据库
func (s *DB) Save(siteTitle string, powerKW float64) error {
	record := SitePowerRecord{
		CreatedAt: time.Now(),
		SiteTitle: siteTitle,
		PowerKW:   powerKW,
	}

	if err := s.db.Create(&record).Error; err != nil {
		s.log.ERROR.Printf("save sitePower record failed: %v", err)
		return err
	}

	s.log.DEBUG.Printf("saved sitePower record: site=%s, power=%.3fkW", siteTitle, powerKW)
	return nil
}

// GetRecords 获取指定时间范围内的记录
func (s *DB) GetRecords(siteTitle string, from, to time.Time) (SitePowerRecords, error) {
	var records SitePowerRecords
	tx := s.db.Where("site_title = ? AND created_at BETWEEN ? AND ?", siteTitle, from, to).Order("created_at").Find(&records)
	return records, tx.Error
}

// GetLatestRecord 获取最新的记录
func (s *DB) GetLatestRecord(siteTitle string) (*SitePowerRecord, error) {
	var record SitePowerRecord
	tx := s.db.Where("site_title = ?", siteTitle).Order("created_at DESC").First(&record)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &record, nil
}

// DeleteOldRecords 删除指定时间之前的旧记录
func (s *DB) DeleteOldRecords(before time.Time) error {
	tx := s.db.Where("created_at < ?", before).Delete(&SitePowerRecord{})
	if tx.Error != nil {
		s.log.ERROR.Printf("delete old sitePower records failed: %v", tx.Error)
		return tx.Error
	}
	s.log.INFO.Printf("deleted %d old sitePower records before %s", tx.RowsAffected, before.Format(time.RFC3339))
	return nil
}