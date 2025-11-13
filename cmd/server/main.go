package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mikeysoft/flotilla/internal/server/api"
	"github.com/mikeysoft/flotilla/internal/server/auth"
	"github.com/mikeysoft/flotilla/internal/server/config"
	"github.com/mikeysoft/flotilla/internal/server/dashboard"
	"github.com/mikeysoft/flotilla/internal/server/database"
	appLogs "github.com/mikeysoft/flotilla/internal/server/logs"
	"github.com/mikeysoft/flotilla/internal/server/metrics"
	"github.com/mikeysoft/flotilla/internal/server/middleware"
	"github.com/mikeysoft/flotilla/internal/server/topology"
	"github.com/mikeysoft/flotilla/internal/server/websocket"
	"github.com/sirupsen/logrus"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Setup logging
	setupLogging(cfg.LogLevel, cfg.LogFormat)

	logrus.Info("Starting Flotilla Management Server...")

	// Connect to database
	if err := database.Connect(cfg.DatabaseURL, cfg.Mode); err != nil {
		logrus.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Run database migrations
	if err := database.Migrate(); err != nil {
		logrus.Fatalf("Failed to migrate database: %v", err)
	}

	// Initialize InfluxDB metrics client
	metricsClient, err := metrics.NewClient(
		cfg.InfluxDBURL,
		cfg.InfluxDBToken,
		cfg.InfluxDBOrg,
		cfg.InfluxDBBucket,
		cfg.InfluxDBEnabled,
	)
	if err != nil {
		logrus.Errorf("Failed to initialize InfluxDB client: %v", err)
	}
	defer metricsClient.Close()

	// Create WebSocket hub
	hub := websocket.NewHub()
	hub.SetMetricsClient(metricsClient)
	hub.Mode = cfg.Mode

	// Start WebSocket hub in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Application log manager
	logManager := appLogs.NewManager(1000)

	// Topology manager
	topologyManager := topology.NewManager(hub, database.DB, cfg.TopologyRefreshInterval, cfg.TopologyStaleAfter, cfg.TopologyBatchSize)
	topologyManager.StartBackgroundRefresh(ctx)

	dashboardManager := dashboard.NewManager(database.DB)
	if err := dashboardManager.RefreshSummary(context.Background()); err != nil {
		logrus.WithError(err).Warn("failed to prime dashboard summary")
	}

	dashboardScanner := dashboard.NewScanner(database.DB, hub, dashboardManager, topologyManager, metricsClient, nil)
	dashboardScanner.Start(ctx)

	// Setup Gin router
	router := setupRouter(cfg, hub, logManager, topologyManager, dashboardManager)

	// Start server
	serverAddr := cfg.GetServerAddress()
	logrus.Infof("Server starting on %s", serverAddr)

	// Start server in a goroutine
	go func() {
		var err error
		if cfg.TLSEnabled {
			if cfg.TLSCertFile == "" || cfg.TLSKeyFile == "" {
				logrus.Fatalf("TLS enabled but TLS_CERT_FILE or TLS_KEY_FILE not provided")
			}
			logrus.Infof("Starting server with TLS on %s", serverAddr)
			err = router.RunTLS(serverAddr, cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			logrus.Infof("Starting server without TLS on %s", serverAddr)
			err = router.Run(serverAddr)
		}
		if err != nil {
			logrus.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logrus.Info("Shutting down server...")
}

func setupLogging(level, format string) {
	// Set log level
	switch level {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	default:
		logrus.SetLevel(logrus.InfoLevel)
	}

	// Set log format
	if format == "json" {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}
}

// errorOnlyLogger logs requests only when response status >= 400
func errorOnlyLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		status := c.Writer.Status()
		if status >= 400 {
			logrus.WithFields(logrus.Fields{
				"status":  status,
				"method":  c.Request.Method,
				"path":    c.Request.URL.Path,
				"latency": time.Since(start),
				"client":  c.ClientIP(),
			}).Warn("http error")
		}
	}
}

func setupRouter(cfg *config.Config, hub *websocket.Hub, logManager *appLogs.Manager, topologyManager *topology.Manager, dashboardManager *dashboard.Manager) *gin.Engine {
	// Set Gin mode based on MODE
	if strings.EqualFold(cfg.Mode, "DEV") {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Middleware: full logs in DEV, errors-only in PROD
	if strings.EqualFold(cfg.Mode, "DEV") {
		router.Use(gin.Logger(), gin.Recovery())
	} else {
		router.Use(errorOnlyLogger(), gin.Recovery())
	}

	// Security middleware
	router.Use(middleware.SecurityHeadersMiddleware())
	router.Use(middleware.CORSMiddleware())

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "flotilla-server",
		})
	})

	// Create API handlers
	hostsHandler := api.NewHostsHandler(hub, logManager, topologyManager)
	containersHandler := api.NewContainersHandler(hub, logManager, topologyManager)
	metricsHandler := api.NewMetricsHandler(hub)
	apiKeysHandler := api.NewAPIKeysHandler()
	authHandler := api.NewAuthHandler()
	usersHandler := api.NewUsersHandler()
	logsHandler := api.NewLogsHandler(logManager)
	dashboardHandler := api.NewDashboardHandler(dashboardManager, logManager)

	// API routes
	apiGroup := router.Group("/api/v1")
	{
		// Auth routes with rate limiting
		apiGroup.GET("/auth/setup", authHandler.GetSetupStatus)
		apiGroup.POST("/auth/setup", middleware.RateLimitMiddleware(5, time.Minute), authHandler.Setup)
		apiGroup.POST("/auth/login", middleware.RateLimitMiddleware(10, time.Minute), authHandler.Login)
		apiGroup.POST("/auth/refresh", middleware.RateLimitMiddleware(20, time.Minute), authHandler.Refresh)
		apiGroup.POST("/auth/logout", authHandler.Logout)

		// Auth middleware
		authRequired := func(c *gin.Context) {
			header := c.GetHeader("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
				return
			}
			tok := strings.TrimPrefix(header, "Bearer ")
			claims, err := auth.ParseAccessToken(tok)
			if err != nil {
				c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
				return
			}
			c.Set("user_id", claims.RegisteredClaims.Subject)
			c.Set("role", claims.Role)
			c.Next()
		}

		adminRequired := func(c *gin.Context) {
			roleValue, exists := c.Get("role")
			if !exists {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
			if roleStr, ok := roleValue.(string); !ok || !strings.EqualFold(roleStr, "admin") {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
			c.Next()
		}

		// Host routes
		apiGroup.GET("/hosts", authRequired, hostsHandler.ListHosts)
		apiGroup.GET("/hosts/:id", authRequired, hostsHandler.GetHost)
		apiGroup.DELETE("/hosts/:id", authRequired, hostsHandler.DeleteHost)
		apiGroup.GET("/hosts/:id/info", authRequired, hostsHandler.GetHostInfo)
		apiGroup.GET("/hosts/:id/containers", authRequired, hostsHandler.ListContainers)
		apiGroup.GET("/hosts/:id/stacks", authRequired, hostsHandler.ListStacks)
		apiGroup.POST("/hosts/:id/stacks", authRequired, hostsHandler.DeployStack)
		apiGroup.POST("/hosts/:id/stacks/import", authRequired, hostsHandler.ImportStack)
		apiGroup.GET("/hosts/:id/stacks/:stack_name/containers", authRequired, hostsHandler.GetStackContainers)
		apiGroup.POST("/hosts/:id/stacks/:stack_name/containers/:container_id/:action", authRequired, hostsHandler.StackContainerAction)
		apiGroup.POST("/hosts/:id/stacks/:stack_name/:action", authRequired, hostsHandler.StackAction)
		apiGroup.POST("/hosts/:id/containers", authRequired, hostsHandler.CreateContainer)
		apiGroup.POST("/hosts/:id/containers/:container_id/:action", authRequired, hostsHandler.ContainerAction)

		// Container routes
		apiGroup.GET("/containers", authRequired, hostsHandler.ListAllContainers)
		apiGroup.GET("/stacks", authRequired, hostsHandler.ListAllStacks)
		apiGroup.GET("/hosts/:id/containers/:container_id", authRequired, containersHandler.GetContainer)
		apiGroup.GET("/hosts/:id/containers/:container_id/logs", authRequired, containersHandler.GetContainerLogs)
		apiGroup.GET("/hosts/:id/containers/:container_id/stats", authRequired, containersHandler.GetContainerStats)
		apiGroup.GET("/hosts/:id/images", authRequired, containersHandler.ListImages)
		apiGroup.POST("/hosts/:id/images/remove", authRequired, containersHandler.RemoveImages)
		apiGroup.POST("/hosts/:id/images/prune", authRequired, containersHandler.PruneDanglingImages)
		apiGroup.GET("/hosts/:id/networks", authRequired, containersHandler.ListNetworks)
		apiGroup.GET("/hosts/:id/networks/:network_id", authRequired, containersHandler.InspectNetwork)
		apiGroup.DELETE("/hosts/:id/networks/:network_id", authRequired, containersHandler.RemoveNetwork)
		apiGroup.POST("/hosts/:id/networks/refresh", authRequired, containersHandler.RefreshNetworks)
		apiGroup.GET("/hosts/:id/volumes", authRequired, containersHandler.ListVolumes)
		apiGroup.GET("/hosts/:id/volumes/:volume_name", authRequired, containersHandler.InspectVolume)
		apiGroup.DELETE("/hosts/:id/volumes/:volume_name", authRequired, containersHandler.RemoveVolume)
		apiGroup.POST("/hosts/:id/volumes/refresh", authRequired, containersHandler.RefreshVolumes)
		apiGroup.GET("/logs", authRequired, logsHandler.ListLogs)

		// Dashboard routes
		apiGroup.GET("/dashboard/summary", authRequired, dashboardHandler.GetSummary)
		apiGroup.GET("/dashboard/tasks", authRequired, dashboardHandler.ListTasks)
		apiGroup.POST("/dashboard/tasks", authRequired, dashboardHandler.CreateTask)
		apiGroup.PATCH("/dashboard/tasks/:id", authRequired, dashboardHandler.UpdateTask)
		apiGroup.POST("/dashboard/tasks/:id/status", authRequired, dashboardHandler.UpdateTaskStatus)

		// Metrics routes
		apiGroup.GET("/hosts/:id/metrics", authRequired, metricsHandler.GetHostMetrics)
		apiGroup.GET("/hosts/:id/containers/:container_id/metrics", authRequired, metricsHandler.GetContainerMetrics)

		// API Key routes
		apiGroup.POST("/api-keys", authRequired, adminRequired, apiKeysHandler.CreateAPIKey)
		apiGroup.GET("/api-keys", authRequired, adminRequired, apiKeysHandler.ListAPIKeys)
		apiGroup.DELETE("/api-keys/:id", authRequired, adminRequired, apiKeysHandler.RevokeAPIKey)
		apiGroup.DELETE("/api-keys/:id/permanent", authRequired, adminRequired, apiKeysHandler.DeleteAPIKeyPermanently)

		// Users (admin-only; minimal check)
		apiGroup.GET("/users", authRequired, adminRequired, usersHandler.List)
		apiGroup.POST("/users", authRequired, adminRequired, usersHandler.Create)
		apiGroup.PUT("/users/:id", authRequired, adminRequired, usersHandler.Update)
		apiGroup.POST("/users/:id/reset-password", authRequired, adminRequired, usersHandler.ResetPassword)
		apiGroup.DELETE("/users/:id/permanent", authRequired, adminRequired, usersHandler.DeleteUserPermanently)
	}

	// WebSocket routes
	ws := router.Group("/ws")
	{
		ws.GET("/agent", hub.AgentWebSocketHandler)
		ws.GET("/ui", hub.UIWebSocketHandler)
		ws.GET("/logs/:host_id/:container_id", hub.LogStreamHandler)
		ws.GET("/logs", logsHandler.StreamLogs)
	}

	// Serve static files (for frontend) - only if they exist
	if _, err := os.Stat("./web/dist"); err == nil {
		// Add no-cache middleware for assets BEFORE serving static files
		assetsGroup := router.Group("/assets")
		assetsGroup.Use(func(c *gin.Context) {
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
			c.Next()
		})
		assetsGroup.Static("/", "./web/dist/assets")

		// Serve other static files (favicon, etc.)
		router.StaticFile("/favicon.ico", "./web/dist/favicon.ico")
		router.StaticFile("/flotilla.png", "./web/dist/flotilla.png")
		router.StaticFile("/flotilla.svg", "./web/dist/flotilla.svg")
		router.StaticFile("/flotilla.webp", "./web/dist/flotilla.webp")
		router.StaticFile("/vite.svg", "./web/dist/vite.svg")
		// Load HTML templates
		router.LoadHTMLGlob("web/dist/*.html")
		// Serve the main page
		router.GET("/", func(c *gin.Context) {
			c.HTML(200, "index.html", nil)
		})
		// Catch-all route for SPA routing - must be last
		router.NoRoute(func(c *gin.Context) {
			// Only serve SPA for non-API routes
			if !strings.HasPrefix(c.Request.URL.Path, "/api/") && !strings.HasPrefix(c.Request.URL.Path, "/ws/") {
				c.HTML(200, "index.html", nil)
			} else {
				c.JSON(404, gin.H{"error": "Not found"})
			}
		})
	} else {
		// Fallback for development when frontend isn't built
		router.GET("/", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"message": "Flotilla Management Server",
				"status":  "running",
				"version": "1.0.0",
			})
		})
	}

	return router
}
