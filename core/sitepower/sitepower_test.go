package sitepower

import (
	"testing"
	"time"

	serverdb "github.com/evcc-io/evcc/server/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSitePowerStorage(t *testing.T) {
	// 创建内存数据库
	db, err := serverdb.New("sqlite", ":memory:")
	require.NoError(t, err)

	// 创建存储实例
	store, err := NewStore(db)
	require.NoError(t, err)

	// 测试保存记录
	siteTitle := "Test Site"
	powerKW := 5.5
	err = store.Save(siteTitle, powerKW)
	require.NoError(t, err)

	// 测试获取最新记录
	record, err := store.GetLatestRecord(siteTitle)
	require.NoError(t, err)
	assert.Equal(t, siteTitle, record.SiteTitle)
	assert.Equal(t, powerKW, record.PowerKW)
	assert.WithinDuration(t, time.Now(), record.CreatedAt, time.Second)

	// 测试获取时间范围内的记录
	from := time.Now().Add(-time.Hour)
	to := time.Now().Add(time.Hour)
	records, err := store.GetRecords(siteTitle, from, to)
	require.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, siteTitle, records[0].SiteTitle)
	assert.Equal(t, powerKW, records[0].PowerKW)
}

func TestScheduler(t *testing.T) {
	// 创建内存数据库
	db, err := serverdb.New("sqlite", ":memory:")
	require.NoError(t, err)

	// 创建存储实例
	store, err := NewStore(db)
	require.NoError(t, err)

	// 创建调度器（使用较短的间隔进行测试）
	scheduler := NewScheduler(store, 100*time.Millisecond)

	// 测试调度器状态
	status := scheduler.GetStatus()
	assert.False(t, status["running"].(bool))

	// 启动调度器
	scheduler.Start()
	defer scheduler.Stop()

	// 验证调度器已启动
	status = scheduler.GetStatus()
	assert.True(t, status["running"].(bool))

	// 更新功率数据
	siteTitle := "Test Site"
	powerKW := 3.2
	scheduler.UpdatePower(siteTitle, powerKW)

	// 等待调度器执行保存
	time.Sleep(150 * time.Millisecond)

	// 验证数据已保存
	record, err := store.GetLatestRecord(siteTitle)
	require.NoError(t, err)
	assert.Equal(t, siteTitle, record.SiteTitle)
	assert.Equal(t, powerKW, record.PowerKW)

	// 测试停止调度器
	scheduler.Stop()
	status = scheduler.GetStatus()
	assert.False(t, status["running"].(bool))
}

func TestDeleteOldRecords(t *testing.T) {
	// 创建内存数据库
	db, err := serverdb.New("sqlite", ":memory:")
	require.NoError(t, err)

	// 创建存储实例
	store, err := NewStore(db)
	require.NoError(t, err)

	// 创建一些测试记录
	siteTitle := "Test Site"
	
	// 保存旧记录
	oldRecord := SitePowerRecord{
		CreatedAt: time.Now().Add(-2 * time.Hour),
		SiteTitle: siteTitle,
		PowerKW:   1.0,
	}
	err = db.Create(&oldRecord).Error
	require.NoError(t, err)

	// 保存新记录
	err = store.Save(siteTitle, 2.0)
	require.NoError(t, err)

	// 验证有2条记录
	records, err := store.GetRecords(siteTitle, time.Now().Add(-3*time.Hour), time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.Len(t, records, 2)

	// 删除1小时前的记录
	err = store.DeleteOldRecords(time.Now().Add(-time.Hour))
	require.NoError(t, err)

	// 验证只剩1条记录
	records, err = store.GetRecords(siteTitle, time.Now().Add(-3*time.Hour), time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, 2.0, records[0].PowerKW)
}