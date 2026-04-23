package main

import (
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	webui "banana-lab"
	"banana-lab/internal/api"
	"banana-lab/internal/config"
	"banana-lab/internal/model"
	"banana-lab/internal/service"
	"banana-lab/internal/storage"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	storageRoot := strings.TrimSpace(cfg.StorageRoot)
	if storageRoot == "" {
		storageRoot = "./storage"
	}
	if err := os.MkdirAll(storageRoot, 0o755); err != nil {
		log.Fatalf("创建存储目录失败: %v", err)
	}

	dbPath := strings.TrimSpace(cfg.DBPath)
	if dbPath == "" {
		dbPath = filepath.Join(storageRoot, "banana-lab.db")
	}

	db, err := model.Open(dbPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	forceResetDB := model.IsTruthyFlag(os.Getenv(model.ForceResetDBEnv))
	if err := model.EnsureLegacySchemaGuard(db, forceResetDB); err != nil {
		log.Fatalf("数据库安全闸阻断启动: %v", err)
	}
	if err := model.Migrate(db); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}
	promptsHandler := &api.PromptsHandler{DB: db}
	promptSnippetsHandler := &api.PromptSnippetsHandler{DB: db}
	nodesHandler := &api.NodesHandler{DB: db}
	resultsHandler := &api.ResultsHandler{DB: db}
	archiveHandler := &api.ArchiveHandler{DB: db}
	libraryHandler := &api.LibraryHandler{DB: db}

	store := storage.Storage{Root: storageRoot}
	proxy := &service.ProxyClient{
		BaseURL:         cfg.ProxyBaseURL,
		AuthHeaderName:  cfg.ProxyAuthHeaderName,
		AuthHeaderValue: cfg.ProxyAuthHeaderValue,
		HTTP:            &http.Client{Timeout: cfg.GenerateTimeout},
	}
	limiter := service.NewConcurrencyLimiter(cfg.MaxConcurrent)
	genSvc := &service.GenerateService{
		DB:      db,
		Storage: store,
		Proxy:   proxy,
		Limiter: limiter,
	}
	registry := config.NewRegistry(cfg.Models)
	generateHandler := &api.GenerateHandler{Service: genSvc, Registry: registry}
	modelsHandler := &api.ModelsHandler{Registry: registry}

	uiFS, _ := webui.FS()

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), gin.ErrorLogger())
	router.Static("/assets/images", filepath.Join(storageRoot, "images"))
	router.Static("/assets/uploads", filepath.Join(storageRoot, "uploads"))

	api.RegisterRoutes(router)
	apiGroup := router.Group("/api")
	apiGroup.GET("/prompts", promptsHandler.List)
	apiGroup.PATCH("/prompts/:id", promptsHandler.Update)
	apiGroup.POST("/prompt-snippets", promptSnippetsHandler.Create)
	apiGroup.GET("/prompt-snippets", promptSnippetsHandler.List)
	apiGroup.PUT("/prompt-snippets", promptSnippetsHandler.ReplaceAll)
	apiGroup.PATCH("/prompt-snippets/:id", promptSnippetsHandler.Update)
	apiGroup.DELETE("/prompt-snippets/:id", promptSnippetsHandler.Delete)
	apiGroup.GET("/prompts/:id/tree", (&api.PromptTreeHandler{DB: db}).Get)
	apiGroup.DELETE("/prompts/:id", nodesHandler.Delete)
	apiGroup.POST("/prompts/:id/reroll", generateHandler.Reroll)
	apiGroup.GET("/nodes/:id", nodesHandler.Get)
	apiGroup.DELETE("/nodes/:id", nodesHandler.Delete)
	apiGroup.DELETE("/results/:id", resultsHandler.Delete)
	apiGroup.GET("/archive", archiveHandler.List)
	apiGroup.GET("/library", libraryHandler.List)
	apiGroup.GET("/models", modelsHandler.List)
	apiGroup.POST("/upload", (&api.UploadHandler{DB: db, Storage: store}).Handle)
	apiGroup.POST("/generate", generateHandler.Handle)

	router.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusNotFound)
			return
		}
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Status(http.StatusNotFound)
			return
		}

		filePath := strings.TrimPrefix(c.Request.URL.Path, "/")
		if filePath == "" {
			filePath = "index.html"
		}

		data, err := fs.ReadFile(uiFS, filePath)
		if err != nil {
			filePath = "index.html"
			data, err = fs.ReadFile(uiFS, filePath)
			if err != nil {
				c.Status(http.StatusInternalServerError)
				return
			}
		}

		contentType := mime.TypeByExtension(filepath.Ext(filePath))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		c.Data(http.StatusOK, contentType, data)
	})

	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	if err := router.Run(addr); err != nil {
		log.Fatalf("启动服务失败: %v", err)
	}
}
