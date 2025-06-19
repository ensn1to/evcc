package core

import (
	"time"

	"github.com/evcc-io/evcc/api"
)

// smartCostActive 检查当前电价是否低于或等于智能成本限制
// 
// 参数:
//   rates api.Rates - 电价数据，包含多个时间点的电价信息
// 
// 返回值:
//   bool - 如果当前时间有电价且智能成本限制存在，并且当前电价小于等于限制，则返回true
func (lp *Loadpoint) smartCostActive(rates api.Rates) bool {
	rate, err := rates.At(time.Now())
	limit := lp.GetSmartCostLimit()
	return err == nil && limit != nil && rate.Value <= *limit
}

// smartCostNextStart returns the next start time for a smart cost rate below the limit
func (lp *Loadpoint) smartCostNextStart(rates api.Rates) time.Time {
	limit := lp.GetSmartCostLimit()
	if limit == nil || rates == nil {
		return time.Time{}
	}

	now := time.Now()
	for _, slot := range rates {
		if slot.Start.After(now) && slot.Value <= *limit {
			return slot.Start
		}
	}

	return time.Time{}
}
