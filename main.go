package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

//go:embed frontend/dist/*
var frontendFS embed.FS

// Process 进程信息结构
type Process struct {
	PID  uint32 `json:"pid"`
	Name string `json:"name"`
}

// MemorySearchRequest 内存搜索请求
type MemorySearchRequest struct {
	PID             uint32         `json:"pid"`
	Value           interface{}    `json:"value"`
	DataType        string         `json:"dataType"`  // int32, int64, float32, float64
	Operation       string         `json:"operation"` // equal, greater, less, increased, decreased
	PreviousResults []MemoryResult `json:"previousResults"`
}

// MemoryModifyRequest 内存修改请求
type MemoryModifyRequest struct {
	PID      uint32      `json:"pid"`
	Address  string      `json:"address"`
	Value    interface{} `json:"value"`
	DataType string      `json:"dataType"`
}

func main() {
	r := gin.Default()

	// API路由
	api := r.Group("/api")
	{
		api.GET("/processes", getProcessList)
		api.POST("/search", searchMemory)
		api.POST("/modify", modifyMemory)
	}

	// 静态文件服务
	// 获取前端文件系统
	frontend, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		panic(err)
	}

	// 设置静态文件服务
	r.StaticFS("/static", http.FS(frontend))
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/static/index.html")
	})

	fmt.Println("Starting server at http://localhost:8080")
	r.Run(":8080")
}

// getProcessList 获取进程列表
func getProcessList(c *gin.Context) {
	processes, err := GetProcessList()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, processes)
}

// searchMemory 搜索内存
func searchMemory(c *gin.Context) {
	var req MemorySearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 将字符串值转换为数值
	if strValue, ok := req.Value.(string); ok {
		if floatValue, err := strconv.ParseFloat(strValue, 64); err == nil {
			req.Value = floatValue
		}
	}

	results, err := ScanMemory(req.PID, req.Value, req.DataType, req.Operation, req.PreviousResults)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, results)
}

// modifyMemory 修改内存
func modifyMemory(c *gin.Context) {
	var req MemoryModifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 将字符串值转换为数值
	if strValue, ok := req.Value.(string); ok {
		if floatValue, err := strconv.ParseFloat(strValue, 64); err == nil {
			req.Value = floatValue
		}
	}

	if err := ModifyMemory(req.PID, req.Address, req.Value, req.DataType); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
