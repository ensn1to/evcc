package sitepower

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

// API 提供sitePower数据的HTTP API接口
type API struct {
	db *DB
}

// NewAPI 创建新的API实例
func NewAPI(db *DB) *API {
	return &API{db: db}
}

// RegisterRoutes 注册API路由
func (api *API) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/api/sitepower/records", api.getRecords).Methods("GET")
	router.HandleFunc("/api/sitepower/latest", api.getLatest).Methods("GET")
	router.HandleFunc("/api/sitepower/cleanup", api.cleanup).Methods("POST")
}

// RecordsResponse API响应结构
type RecordsResponse struct {
	Records []SitePowerRecord `json:"records"`
	Count   int               `json:"count"`
}

// getRecords 获取指定时间范围内的记录
// GET /api/sitepower/records?site=<siteTitle>&from=<timestamp>&to=<timestamp>&limit=<limit>
func (api *API) getRecords(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 解析查询参数
	siteTitle := r.URL.Query().Get("site")
	if siteTitle == "" {
		http.Error(w, "site parameter is required", http.StatusBadRequest)
		return
	}

	// 解析时间范围
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	var from, to time.Time
	var err error

	if fromStr != "" {
		if fromTimestamp, parseErr := strconv.ParseInt(fromStr, 10, 64); parseErr == nil {
			from = time.Unix(fromTimestamp, 0)
		} else {
			if from, err = time.Parse(time.RFC3339, fromStr); err != nil {
				http.Error(w, "invalid from time format", http.StatusBadRequest)
				return
			}
		}
	} else {
		// 默认查询最近24小时
		from = time.Now().Add(-24 * time.Hour)
	}

	if toStr != "" {
		if toTimestamp, parseErr := strconv.ParseInt(toStr, 10, 64); parseErr == nil {
			to = time.Unix(toTimestamp, 0)
		} else {
			if to, err = time.Parse(time.RFC3339, toStr); err != nil {
				http.Error(w, "invalid to time format", http.StatusBadRequest)
				return
			}
		}
	} else {
		to = time.Now()
	}

	// 获取记录
	records, err := api.db.GetRecords(siteTitle, from, to)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get records: %v", err), http.StatusInternalServerError)
		return
	}

	// 应用限制
	limitStr := r.URL.Query().Get("limit")
	if limitStr != "" {
		if limit, parseErr := strconv.Atoi(limitStr); parseErr == nil && limit > 0 && limit < len(records) {
			records = records[len(records)-limit:] // 取最新的N条记录
		}
	}

	response := RecordsResponse{
		Records: records,
		Count:   len(records),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

// getLatest 获取最新记录
// GET /api/sitepower/latest?site=<siteTitle>
func (api *API) getLatest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	siteTitle := r.URL.Query().Get("site")
	if siteTitle == "" {
		http.Error(w, "site parameter is required", http.StatusBadRequest)
		return
	}

	record, err := api.db.GetLatestRecord(siteTitle)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get latest record: %v", err), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(record); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

// CleanupRequest 清理请求结构
type CleanupRequest struct {
	DaysToKeep int `json:"daysToKeep"`
}

// CleanupResponse 清理响应结构
type CleanupResponse struct {
	DeletedCount int64  `json:"deletedCount"`
	Message      string `json:"message"`
}

// cleanup 清理旧记录
// POST /api/sitepower/cleanup
// Body: {"daysToKeep": 30}
func (api *API) cleanup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req CleanupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.DaysToKeep <= 0 {
		req.DaysToKeep = 30 // 默认保留30天
	}

	before := time.Now().AddDate(0, 0, -req.DaysToKeep)
	err := api.db.DeleteOldRecords(before)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to cleanup records: %v", err), http.StatusInternalServerError)
		return
	}

	response := CleanupResponse{
		Message: fmt.Sprintf("Successfully cleaned up records older than %d days", req.DaysToKeep),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}