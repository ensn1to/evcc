package sitepower

import (
	"context"
	"sync"
	"time"

	"github.com/evcc-io/evcc/util"
)

// Scheduler 管理sitePower数据的定时存储
type Scheduler struct {
	mu       sync.RWMutex
	log      *util.Logger
	db       *DB
	ticker   *time.Ticker
	ctx      context.Context
	cancel   context.CancelFunc
	interval time.Duration

	// 当前数据缓存
	siteTitle string
	lastPower float64
	lastUpdate time.Time
}

// NewScheduler 创建新的调度器
func NewScheduler(db *DB, interval time.Duration) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		log:      util.NewLogger("sitepower-scheduler"),
		db:       db,
		ctx:      ctx,
		cancel:   cancel,
		interval: interval,
	}
}

// Start 启动定时存储任务
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ticker != nil {
		s.log.WARN.Println("scheduler already started")
		return
	}

	s.ticker = time.NewTicker(s.interval)
	s.log.INFO.Printf("starting sitePower scheduler with interval: %v", s.interval)

	go s.run()
}

// Stop 停止定时存储任务
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ticker == nil {
		return
	}

	s.log.INFO.Println("stopping sitePower scheduler")
	s.ticker.Stop()
	s.ticker = nil
	s.cancel()
}

// UpdatePower 更新当前功率数据
func (s *Scheduler) UpdatePower(siteTitle string, powerKW float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.siteTitle = siteTitle
	s.lastPower = powerKW
	s.lastUpdate = time.Now()
}

// run 运行定时存储循环
func (s *Scheduler) run() {
	for {
		select {
		case <-s.ctx.Done():
			s.log.DEBUG.Println("scheduler context cancelled")
			return
		case <-s.ticker.C:
			s.saveCurrent()
		}
	}
}

// saveCurrent 保存当前缓存的数据
func (s *Scheduler) saveCurrent() {
	s.mu.RLock()
	siteTitle := s.siteTitle
	powerKW := s.lastPower
	lastUpdate := s.lastUpdate
	s.mu.RUnlock()

	// 检查是否有有效数据
	if siteTitle == "" {
		s.log.DEBUG.Println("no site title available, skipping save")
		return
	}

	// 检查数据是否过期（超过2个间隔周期）
	if time.Since(lastUpdate) > s.interval*2 {
		s.log.WARN.Printf("sitePower data is stale (last update: %v), skipping save", lastUpdate)
		return
	}

	// 保存到数据库
	if err := s.db.Save(siteTitle, powerKW); err != nil {
		s.log.ERROR.Printf("failed to save sitePower: %v", err)
	} else {
		s.log.DEBUG.Printf("saved sitePower: site=%s, power=%.3fkW", siteTitle, powerKW)
	}
}

// GetStatus 获取调度器状态
func (s *Scheduler) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"running":     s.ticker != nil,
		"interval":    s.interval.String(),
		"siteTitle":   s.siteTitle,
		"lastPower":   s.lastPower,
		"lastUpdate":  s.lastUpdate,
		"dataAge":     time.Since(s.lastUpdate).String(),
	}
}