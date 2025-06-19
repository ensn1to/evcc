package core

import (
	"maps"
	"math"
	"slices"
	"time"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/core/keys"
	"github.com/evcc-io/evcc/server/db/settings"
	"github.com/evcc-io/evcc/tariff"
	"github.com/samber/lo"
)

type solarDetails struct {
	Scale            *float64     `json:"scale,omitempty"`            // scale factor yield/forecasted today
	Today            dailyDetails `json:"today,omitempty"`            // tomorrow
	Tomorrow         dailyDetails `json:"tomorrow,omitempty"`         // tomorrow
	DayAfterTomorrow dailyDetails `json:"dayAfterTomorrow,omitempty"` // day after tomorrow
	Timeseries       timeseries   `json:"timeseries,omitempty"`       // timeseries of forecasted energy
}

type dailyDetails struct {
	Yield    float64 `json:"energy"`
	Complete bool    `json:"complete"`
}

// greenShare returns
//   - the current green share, calculated for the part of the consumption between powerFrom and powerTo
//     the consumption below powerFrom will get the available green power first
func (site *Site) greenShare(powerFrom float64, powerTo float64) float64 {
	// 计算当前站点可用绿电
	greenPower := math.Max(0, site.pvPower) + math.Max(0, site.batteryPower)
	greenPowerAvailable := math.Max(0, greenPower-powerFrom)

	power := powerTo - powerFrom
	share := math.Min(greenPowerAvailable, power) / power

	if math.IsNaN(share) {
		if greenPowerAvailable > 0 {
			share = 1
		} else {
			share = 0
		}
	}

	return share
}

// effectivePrice calculates the real energy price based on self-produced and grid-imported energy.
func (site *Site) effectivePrice(greenShare float64) *float64 {
	if grid, err := tariff.Now(site.GetTariff(api.TariffUsageGrid)); err == nil {
		feedin, err := tariff.Now(site.GetTariff(api.TariffUsageFeedIn))
		if err != nil {
			feedin = 0
		}
		effPrice := grid*(1-greenShare) + feedin*greenShare
		return &effPrice
	}
	return nil
}

// effectiveCo2 calculates the amount of emitted co2 based on self-produced and grid-imported energy.
func (site *Site) effectiveCo2(greenShare float64) *float64 {
	if co2, err := tariff.Now(site.GetTariff(api.TariffUsageCo2)); err == nil {
		effCo2 := co2 * (1 - greenShare)
		return &effCo2
	}
	return nil
}

// publishTariffs 发布各种电价信息到站点
// 参数:
//
//	greenShareHome: 家庭绿色能源占比
//	greenShareLoadpoints: 充电点绿色能源占比
func (site *Site) publishTariffs(greenShareHome float64, greenShareLoadpoints float64) {
	// 发布家庭和充电点的绿色能源占比
	site.publish(keys.GreenShareHome, greenShareHome)
	site.publish(keys.GreenShareLoadpoints, greenShareLoadpoints)

	// 获取并发布当前各类电价
	if v, err := tariff.Now(site.GetTariff(api.TariffUsageGrid)); err == nil {
		site.publish(keys.TariffGrid, v)
	}
	if v, err := tariff.Now(site.GetTariff(api.TariffUsageFeedIn)); err == nil {
		site.publish(keys.TariffFeedIn, v)
	}
	if v, err := tariff.Now(site.GetTariff(api.TariffUsageCo2)); err == nil {
		site.publish(keys.TariffCo2, v)
	}
	if v, err := tariff.Now(site.GetTariff(api.TariffUsageSolar)); err == nil {
		site.publish(keys.TariffSolar, v)
	}

	// 计算并发布家庭的有效价格和CO2
	if v := site.effectivePrice(greenShareHome); v != nil {
		site.publish(keys.TariffPriceHome, v)
	}
	if v := site.effectiveCo2(greenShareHome); v != nil {
		site.publish(keys.TariffCo2Home, v)
	}

	// 计算并发布充电点的有效价格和CO2
	if v := site.effectivePrice(greenShareLoadpoints); v != nil {
		site.publish(keys.TariffPriceLoadpoints, v)
	}
	if v := site.effectiveCo2(greenShareLoadpoints); v != nil {
		site.publish(keys.TariffCo2Loadpoints, v)
	}

	// 构建未来电价预测结构体
	fc := struct {
		Co2     api.Rates     `json:"co2,omitempty"`
		FeedIn  api.Rates     `json:"feedin,omitempty"`
		Grid    api.Rates     `json:"grid,omitempty"`
		Planner api.Rates     `json:"planner,omitempty"`
		Solar   *solarDetails `json:"solar,omitempty"`
	}{
		Co2:     tariff.Forecast(site.GetTariff(api.TariffUsageCo2)),
		FeedIn:  tariff.Forecast(site.GetTariff(api.TariffUsageFeedIn)),
		Planner: tariff.Forecast(site.GetTariff(api.TariffUsagePlanner)),
		Grid:    tariff.Forecast(site.GetTariff(api.TariffUsageGrid)),
	}

	// 计算调整后的太阳能预测
	if solar := timestampSeries(tariff.Forecast(site.GetTariff(api.TariffUsageSolar))); len(solar) > 0 {
		fc.Solar = lo.ToPtr(site.solarDetails(solar))
	}

	// 发布预测信息
	site.publish(keys.Forecast, fc)
}

func (site *Site) solarDetails(solar timeseries) solarDetails {
	res := solarDetails{
		Timeseries: solar,
	}

	last := solar[len(solar)-1].Timestamp

	bod := beginningOfDay(time.Now())
	eod := bod.AddDate(0, 0, 1)
	eot := eod.AddDate(0, 0, 1)

	remainingToday := solar.energy(time.Now(), eod)
	tomorrow := solar.energy(eod, eot)
	dayAfterTomorrow := solar.energy(eot, eot.AddDate(0, 0, 1))

	res.Today = dailyDetails{
		Yield:    remainingToday,
		Complete: !last.Before(eod),
	}
	res.Tomorrow = dailyDetails{
		Yield:    tomorrow,
		Complete: !last.Before(eot),
	}
	res.DayAfterTomorrow = dailyDetails{
		Yield:    dayAfterTomorrow,
		Complete: !last.Before(eot.AddDate(0, 0, 1)),
	}

	// accumulate forecasted energy since last update
	site.fcstEnergy.AddEnergy(solar.energy(site.fcstEnergy.updated, time.Now()) / 1e3)
	settings.SetFloat(keys.SolarAccForecast, site.fcstEnergy.Accumulated)

	produced := lo.SumBy(slices.Collect(maps.Values(site.pvEnergy)), func(v *meterEnergy) float64 {
		return v.AccumulatedEnergy()
	})

	if fcst := site.fcstEnergy.AccumulatedEnergy(); fcst > 0 {
		scale := produced / fcst
		site.log.DEBUG.Printf("solar forecast: accumulated %.3fkWh, produced %.3fkWh, scale %.3f", fcst, produced, scale)

		const minEnergy = 0.5 // kWh
		if produced+fcst > minEnergy {
			res.Scale = lo.ToPtr(scale)
		}
	}

	return res
}
