package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/zhiyungezhu/urldb-novel-upload/cmd/cmdplugin"
	"github.com/zhiyungezhu/urldb-novel-upload/config"
	"github.com/zhiyungezhu/urldb-novel-upload/db"
	"github.com/zhiyungezhu/urldb-novel-upload/db/entity"
	"github.com/zhiyungezhu/urldb-novel-upload/db/repo"
	"github.com/zhiyungezhu/urldb-novel-upload/handlers"
	"github.com/zhiyungezhu/urldb-novel-upload/middleware"
	"github.com/zhiyungezhu/urldb-novel-upload/monitor"
	"github.com/zhiyungezhu/urldb-novel-upload/plugin-system/manager/plugin"
	"github.com/zhiyungezhu/urldb-novel-upload/routes"
	"github.com/zhiyungezhu/urldb-novel-upload/scheduler"
	"github.com/zhiyungezhu/urldb-novel-upload/services"
	"github.com/zhiyungezhu/urldb-novel-upload/task"
	"github.com/zhiyungezhu/urldb-novel-upload/utils"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

func main() {
	// ��������в���
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version":
			versionInfo := utils.GetVersionInfo()
			fmt.Printf("�汾: v%s\n", versionInfo.Version)
			fmt.Printf("����ʱ��: %s\n", versionInfo.BuildTime.Format("2006-01-02 15:04:05"))
			fmt.Printf("Git�ύ: %s\n", versionInfo.GitCommit)
			fmt.Printf("Git��֧: %s\n", versionInfo.GitBranch)
			fmt.Printf("Go�汾: %s\n", versionInfo.GoVersion)
			fmt.Printf("ƽ̨: %s/%s\n", versionInfo.Platform, versionInfo.Arch)
			return
		case "plugin":
			// �����������
			cmdplugin.InitPluginCommands()
			rootCmd := &cobra.Command{Use: "urldb"}
			rootCmd.AddCommand(cmdplugin.GetPluginCmd())
			if err := rootCmd.Execute(); err != nil {
				utils.Error("�������ִ��ʧ��: %v", err)
				os.Exit(1)
			}
			return
		}
	}

	// ��ʼ����־ϵͳ
	if err := utils.InitLogger(); err != nil {
		log.Fatal("��ʼ����־ϵͳʧ��:", err)
	}

	// ���ػ�������
	if err := godotenv.Load(); err != nil {
		utils.Info("δ�ҵ�.env�ļ���ʹ��Ĭ������")
	}

	// ��ʼ��ʱ������
	utils.InitTimezone()

	// ����Gin����ģʽ
	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "" {
		// ���û������GIN_MODE�����ݻ����ж�
		if os.Getenv("ENV") == "production" {
			gin.SetMode(gin.ReleaseMode)
			utils.Info("����GinΪReleaseģʽ")
		} else {
			gin.SetMode(gin.DebugMode)
			utils.Info("����GinΪDebugģʽ")
		}
	} else {
		// ����Ѿ�������GIN_MODE������ֵ����ģʽ
		switch ginMode {
		case "release":
			gin.SetMode(gin.ReleaseMode)
			utils.Info("����GinΪReleaseģʽ (���Ի�������)")
		case "debug":
			gin.SetMode(gin.DebugMode)
			utils.Info("����GinΪDebugģʽ (���Ի�������)")
		case "test":
			gin.SetMode(gin.TestMode)
			utils.Info("����GinΪTestģʽ (���Ի�������)")
		default:
			gin.SetMode(gin.DebugMode)
			utils.Info("δ֪��GIN_MODEֵ: %s��ʹ��Debugģʽ", ginMode)
		}
	}

	// ��ʼ�����ݿ�
	if err := db.InitDB(); err != nil {
		utils.Fatal("���ݿ�����ʧ��: %v", err)
	}

	// ��־ϵͳ�Ѽ򻯣���������ʼ��

	// ����Repository������
	repoManager := repo.NewRepositoryManager(db.DB)

	// �������ù�����
	configManager := config.NewConfigManager(repoManager)

	// ����ȫ�����ù�����
	config.SetGlobalConfigManager(configManager)

	// �����������õ�����
	if err := configManager.LoadAllConfigs(); err != nil {
		utils.Error("�������û���ʧ��: %v", err)
	}

	// �������������
	taskManager := task.NewTaskManager(repoManager)

	// ע��ת����������
	transferProcessor := task.NewTransferProcessor(repoManager)
	taskManager.RegisterProcessor(transferProcessor)

	// ��ʼ�����ϵͳ
	if err := cmdplugin.InitializePluginSystem(repoManager); err != nil {
		utils.Error("���ϵͳ��ʼ��ʧ��: %v", err)
	} else {
		utils.Info("���ϵͳ�����ɹ�")
	}

	// �������������
	var pluginManager *plugin.Manager
	globalPluginIntegration := cmdplugin.GetGlobalPluginIntegration()
	if globalPluginIntegration != nil {
		pluginManager = globalPluginIntegration.GetPluginManager()
	} else {
		pluginManager = plugin.NewManager(nil)
		pluginManager.SetRepoManager(repoManager)
	}

	// ע��������������
	expansionProcessor := task.NewExpansionProcessor(repoManager)
	taskManager.RegisterProcessor(expansionProcessor)

	// ע���ϴ���������
	uploadProcessor := task.NewUploadProcessor(repoManager)
	taskManager.RegisterProcessor(uploadProcessor)

	// ע��С˵�ϴ���������
	novelUploadProcessor := task.NewNovelUploadProcessor(repoManager)
	taskManager.RegisterProcessor(novelUploadProcessor)

	// ��ʼ��Meilisearch������
	meilisearchManager := services.NewMeilisearchManager(repoManager)
	if err := meilisearchManager.Initialize(); err != nil {
		utils.Error("��ʼ��Meilisearch������ʧ��: %v", err)
	}

	// �ָ������е����񣨷�����������
	if err := taskManager.RecoverRunningTasks(); err != nil {
		utils.Error("�ָ�����������ʧ��: %v", err)
	} else {
		utils.Info("����������ָ����")
	}

	utils.Info("�����������ʼ�����")

	// ����Ginʵ��
	r := gin.New()

	// ������غʹ�������
	metrics := monitor.GetGlobalMetrics()
	errorHandler := monitor.GetGlobalErrorHandler()
	if errorHandler == nil {
		errorHandler = monitor.NewErrorHandler(1000, 24*time.Hour)
		monitor.SetGlobalErrorHandler(errorHandler)
	}

	// �����м��
	r.Use(gin.Logger())                     // Gin��־�м��
	r.Use(errorHandler.RecoverMiddleware()) // Panic�ָ��м��
	r.Use(errorHandler.ErrorMiddleware())   // �������м��
	r.Use(metrics.MetricsMiddleware())      // ����м��
	r.Use(gin.Recovery())                   // Gin�ָ��м��

	// ����CORS
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"*"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	r.Use(cors.New(config))

	// ��Repository������ע�뵽handlers��
	handlers.SetRepositoryManager(repoManager)

	// ���ز�����ò���ʼ�����ϵͳ
	pluginConfig := cmdplugin.LoadPluginConfig()

	// ��������˲��ϵͳ��������·����
	if pluginConfig.Enabled && globalPluginIntegration != nil {
		globalPluginIntegration.SetRouter(r)
		utils.Info("���·����������")
	}

	// ��Repository������ע�뵽services��
	services.SetRepositoryManager(repoManager)

	// ����Sitemap����������
	handlers.SetSitemapDependencies(
		repoManager.ResourceRepository,
		repoManager.SystemConfigRepository,
		repoManager.HotDramaRepository,
		repoManager.ReadyResourceRepository,
		repoManager.PanRepository,
		repoManager.CksRepository,
		repoManager.TagRepository,
		repoManager.CategoryRepository,
	)

	// ����Meilisearch��������handlers��
	handlers.SetMeilisearchManager(meilisearchManager)

	// ����Meilisearch��������services��
	services.SetMeilisearchManager(meilisearchManager)

	// ����ȫ�ֵ�������Meilisearch������
	scheduler.SetGlobalMeilisearchManager(meilisearchManager)

	// ��ʼ��������������
	globalScheduler := scheduler.GetGlobalScheduler(
		repoManager.HotDramaRepository,
		repoManager.ReadyResourceRepository,
		repoManager.ResourceRepository,
		repoManager.SystemConfigRepository,
		repoManager.PanRepository,
		repoManager.CksRepository,
		repoManager.TagRepository,
		repoManager.CategoryRepository,
		repoManager.TaskItemRepository,
		repoManager.TaskRepository,
	)

	// ����ϵͳ����������Ӧ�ĵ�������
	autoFetchHotDrama, _ := repoManager.SystemConfigRepository.GetConfigBool(entity.ConfigKeyAutoFetchHotDramaEnabled)
	autoProcessReadyResources, _ := repoManager.SystemConfigRepository.GetConfigBool(entity.ConfigKeyAutoProcessReadyResources)
	autoTransferEnabled, _ := repoManager.SystemConfigRepository.GetConfigBool(entity.ConfigKeyAutoTransferEnabled)
	autoSitemapEnabled, _ := repoManager.SystemConfigRepository.GetConfigBool(entity.ConfigKeySitemapAutoGenerateEnabled)
	autoGoogleIndexEnabled, _ := repoManager.SystemConfigRepository.GetConfigBool(entity.GoogleIndexConfigKeyEnabled)

	globalScheduler.UpdateSchedulerStatusWithAutoTransfer(
		autoFetchHotDrama,
		autoProcessReadyResources,
		autoTransferEnabled,
	)

	// ����ϵͳ��������Sitemap������
	if autoSitemapEnabled {
		globalScheduler.StartSitemapScheduler()
		utils.Info("ϵͳ��������Sitemap�Զ����ɹ��ܣ�������ʱ����")
	} else {
		utils.Info("ϵͳ���ý���Sitemap�Զ����ɹ���")
	}

	// Google����������������Sitemap���������������ٶ�������
	if autoGoogleIndexEnabled {
		utils.Info("ϵͳ��������Google�����Զ��ύ���ܣ�����Sitemap����������")
	} else {
		utils.Info("ϵͳ���ý���Google�����Զ��ύ����")
	}

	utils.Info("��������ʼ�����")

	// ���ù���API�м����Repository������
	middleware.SetRepositoryManager(repoManager)

	// ��������API������
	publicAPIHandler := handlers.NewPublicAPIHandler()

	// ������������
	taskHandler := handlers.NewTaskHandler(repoManager, taskManager)

	// �����ļ�������
	fileHandler := handlers.NewFileHandler(repoManager.FileRepository, repoManager.SystemConfigRepository, repoManager.UserRepository)

	// ����Meilisearch������
	meilisearchHandler := handlers.NewMeilisearchHandler(meilisearchManager)

	// ����OGͼƬ������
	ogImageHandler := handlers.NewOGImageHandler()

	// �����ٱ��Ͱ�Ȩ����������
	reportHandler := handlers.NewReportHandler(repoManager.ReportRepository, repoManager.ResourceRepository)
	copyrightClaimHandler := handlers.NewCopyrightClaimHandler(repoManager.CopyrightClaimRepository, repoManager.ResourceRepository)

	// ����Google������������
	googleIndexProcessor := task.NewGoogleIndexProcessor(repoManager)

	// ����Google����������
	googleIndexHandler := handlers.NewGoogleIndexHandler(repoManager, taskManager)

	// ����Bing������
	siteURL, _ := repoManager.SystemConfigRepository.GetConfigValue(entity.ConfigKeyWebsiteURL)
	bingHandler := handlers.NewBingHandler(siteURL, repoManager)

	// ע��Google���������������������
	taskManager.RegisterProcessor(googleIndexProcessor)

	utils.Info("Google�������������ã�ע�ᵽ���������")
	if bingHandler != nil {
		utils.Info("Bing�ύ����������")
	} else {
		utils.Warn("Bing��������ʼ��ʧ�ܣ���������Ϊվ��URLδ����")
	}

	// API·��
	api := r.Group("/api")
	{
		// ����API·�ɣ���ҪAPI Token��֤��
		publicAPI := api.Group("/public")
		publicAPI.Use(middleware.PublicAPIAuth())
		{
			// ����������Դ
			publicAPI.POST("/resources/batch-add", publicAPIHandler.AddBatchResources)
			// ��Դ����
			publicAPI.GET("/resources/search", publicAPIHandler.SearchResources)
			// ���ž�
			publicAPI.GET("/hot-dramas", publicAPIHandler.GetHotDramas)
		}

		// ��֤·��
		api.POST("/auth/login", handlers.Login)
		api.POST("/auth/register", handlers.Register)
		api.GET("/auth/profile", middleware.AuthMiddleware(), handlers.GetProfile)

		// ��Դ����
		api.GET("/resources", handlers.GetResources)
		api.GET("/resources/hot", handlers.GetHotResources)
		api.POST("/resources", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.CreateResource)
		api.PUT("/resources/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.UpdateResource)
		api.DELETE("/resources/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.DeleteResource)
		api.GET("/resources/:id", handlers.GetResourceByID)
		api.GET("/resources/key/:key", handlers.GetResourcesByKey)
		api.GET("/resources/check-exists", handlers.CheckResourceExists)
		api.GET("/resources/related", handlers.GetRelatedResources)
		api.POST("/resources/:id/view", handlers.IncrementResourceViewCount)
		api.GET("/resources/:id/link", handlers.GetResourceLink)
		api.GET("/resources/:id/validity", handlers.CheckResourceValidity)
		api.POST("/resources/validity/batch", handlers.BatchCheckResourceValidity)
		api.DELETE("/resources/batch", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.BatchDeleteResources)

		// �������
		api.GET("/categories", handlers.GetCategories)
		api.POST("/categories", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.CreateCategory)
		api.PUT("/categories/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.UpdateCategory)
		api.DELETE("/categories/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.DeleteCategory)

		// ����
		api.GET("/search", handlers.SearchResources)

		// ͳ��
		api.GET("/stats", handlers.GetStats)
		api.GET("/performance", handlers.GetPerformanceStats)
		api.GET("/stats/views-trend", handlers.GetViewsTrend)
		api.GET("/stats/searches-trend", handlers.GetSearchesTrend)
		api.GET("/system/info", handlers.GetSystemInfo)

		// ƽ̨����
		api.GET("/pans", handlers.GetPans)
		api.POST("/pans", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.CreatePan)
		api.PUT("/pans/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.UpdatePan)
		api.DELETE("/pans/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.DeletePan)
		api.GET("/pans/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.GetPan)

		// Cookie����
		api.GET("/cks", handlers.GetCks)
		api.POST("/cks", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.CreateCks)
		api.PUT("/cks/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.UpdateCks)
		api.DELETE("/cks/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.DeleteCks)
		api.GET("/cks/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.GetCksByID)
		api.POST("/cks/:id/refresh-capacity", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.RefreshCapacity)
		api.POST("/cks/:id/delete-related-resources", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.DeleteRelatedResources)

		// ��ǩ����
		api.GET("/tags", handlers.GetTags)
		api.POST("/tags", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.CreateTag)
		api.PUT("/tags/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.UpdateTag)
		api.DELETE("/tags/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.DeleteTag)
		api.GET("/tags/:id", handlers.GetTagByID)
		api.GET("/categories/:categoryId/tags", handlers.GetTagsByCategory)

		// ��������Դ����
		api.GET("/ready-resources", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.GetReadyResources)
		api.POST("/ready-resources/batch", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.BatchCreateReadyResources)
		api.POST("/ready-resources/text", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.CreateReadyResourcesFromText)
		api.DELETE("/ready-resources/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.DeleteReadyResource)
		api.DELETE("/ready-resources", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.ClearReadyResources)
		api.GET("/ready-resources/key/:key", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.GetReadyResourcesByKey)
		api.DELETE("/ready-resources/key/:key", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.DeleteReadyResourcesByKey)
		api.GET("/ready-resources/errors", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.GetReadyResourcesWithErrors)
		api.POST("/ready-resources/:id/clear-error", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.ClearErrorMsg)
		api.POST("/ready-resources/retry-failed", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.RetryFailedResources)
		api.POST("/ready-resources/batch-restore", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.BatchRestoreToReadyPool)
		api.POST("/ready-resources/batch-restore-by-query", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.BatchRestoreToReadyPoolByQuery)
		api.POST("/ready-resources/clear-all-errors-by-query", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.ClearAllErrorsByQuery)

		// �û�������������Ա��
		api.GET("/users", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.GetUsers)
		api.POST("/users", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.CreateUser)
		api.PUT("/users/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.UpdateUser)
		api.PUT("/users/:id/password", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.ChangePassword)
		api.DELETE("/users/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.DeleteUser)

		// ����ͳ��·��
		api.GET("/search-stats", handlers.GetSearchStats)
		api.GET("/search-stats/hot-keywords", handlers.GetHotKeywords)
		api.GET("/search-stats/daily", handlers.GetDailyStats)
		api.GET("/search-stats/trend", handlers.GetSearchTrend)
		api.GET("/search-stats/keyword/:keyword/trend", handlers.GetKeywordTrend)
		api.POST("/search-stats", handlers.RecordSearch)
		api.POST("/search-stats/record", handlers.RecordSearch)
		api.GET("/search-stats/summary", handlers.GetSearchStatsSummary)

		// API������־·��
		api.GET("/api-access-logs", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.GetAPIAccessLogs)
		api.GET("/api-access-logs/summary", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.GetAPIAccessLogSummary)
		api.GET("/api-access-logs/stats", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.GetAPIAccessLogStats)
		api.DELETE("/api-access-logs", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.ClearAPIAccessLogs)

		// ϵͳ��־·��
		api.GET("/system-logs", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.GetSystemLogs)
		api.GET("/system-logs/files", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.GetSystemLogFiles)
		api.GET("/system-logs/summary", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.GetSystemLogSummary)
		api.DELETE("/system-logs", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.ClearSystemLogs)

		// ϵͳ����·��
		api.GET("/system/config", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.GetSystemConfig)
		api.POST("/system/config", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.UpdateSystemConfig)
		api.GET("/system/config/status", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.GetConfigStatus)
		api.POST("/system/config/toggle-auto-process", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.ToggleAutoProcess)
		api.GET("/public/system-config", handlers.GetPublicSystemConfig)
api.GET("/public/site-verification", handlers.GetPublicSiteVerificationCode)  // ��վ��֤���루�������ʣ�

		// �Ȳ������·�ɣ���ѯ�ӿ�������֤��
		api.GET("/hot-dramas", handlers.GetHotDramaList)
		api.GET("/hot-dramas/:id", handlers.GetHotDramaByID)
		api.POST("/hot-dramas", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.CreateHotDrama)
		api.PUT("/hot-dramas/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.UpdateHotDrama)
		api.DELETE("/hot-dramas/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.DeleteHotDrama)
		api.GET("/hot-dramas/poster", handlers.GetPosterImage)

		// �������·��
		api.POST("/tasks/transfer", middleware.AuthMiddleware(), middleware.AdminMiddleware(), taskHandler.CreateBatchTransferTask)
		api.POST("/tasks/expansion", middleware.AuthMiddleware(), middleware.AdminMiddleware(), taskHandler.CreateExpansionTask)
		api.POST("/tasks/upload", middleware.AuthMiddleware(), middleware.AdminMiddleware(), taskHandler.CreateUploadTask)
	api.POST("/tasks/upload-novel", middleware.AuthMiddleware(), middleware.AdminMiddleware(), taskHandler.CreateNovelUploadTask)
		api.GET("/tasks/expansion/accounts", middleware.AuthMiddleware(), middleware.AdminMiddleware(), taskHandler.GetExpansionAccounts)
		api.GET("/tasks", middleware.AuthMiddleware(), middleware.AdminMiddleware(), taskHandler.GetTasks)
		api.GET("/tasks/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), taskHandler.GetTaskStatus)
		api.POST("/tasks/:id/start", middleware.AuthMiddleware(), middleware.AdminMiddleware(), taskHandler.StartTask)
		api.POST("/tasks/:id/stop", middleware.AuthMiddleware(), middleware.AdminMiddleware(), taskHandler.StopTask)
		api.POST("/tasks/:id/pause", middleware.AuthMiddleware(), middleware.AdminMiddleware(), taskHandler.PauseTask)
		api.DELETE("/tasks/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), taskHandler.DeleteTask)
		api.GET("/tasks/:id/items", middleware.AuthMiddleware(), middleware.AdminMiddleware(), taskHandler.GetTaskItems)

		// �ϴ�Ŀ¼��ص�����·��
	api.POST("/scheduler/upload-watcher/start", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.StartUploadWatcher)
	api.POST("/scheduler/upload-watcher/stop", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.StopUploadWatcher)
	// С˵�ϴ�Ŀ¼��ص�����·��
	api.POST("/scheduler/novel-upload-watcher/start", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.StartNovelUploadWatcher)
	api.POST("/scheduler/novel-upload-watcher/stop", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.StopNovelUploadWatcher)

		// ���̲���·��
		api.GET("/pan/quark/folders", middleware.AuthMiddleware(), handlers.BrowsePanFolders)
		api.GET("/pan/quark/check-cookie", middleware.AuthMiddleware(), handlers.CheckPanCookie)

		// �汾����·��
		api.GET("/version", handlers.GetVersion)
		api.GET("/version/string", handlers.GetVersionString)
		api.GET("/version/full", handlers.GetFullVersionInfo)
		api.GET("/version/check-update", handlers.CheckUpdate)

		// Meilisearch����·��
		api.GET("/meilisearch/status", middleware.AuthMiddleware(), middleware.AdminMiddleware(), meilisearchHandler.GetStatus)
		api.GET("/meilisearch/unsynced-count", middleware.AuthMiddleware(), middleware.AdminMiddleware(), meilisearchHandler.GetUnsyncedCount)
		api.GET("/meilisearch/unsynced", middleware.AuthMiddleware(), middleware.AdminMiddleware(), meilisearchHandler.GetUnsyncedResources)
		api.GET("/meilisearch/synced", middleware.AuthMiddleware(), middleware.AdminMiddleware(), meilisearchHandler.GetSyncedResources)
		api.GET("/meilisearch/resources", middleware.AuthMiddleware(), middleware.AdminMiddleware(), meilisearchHandler.GetAllResources)
		api.POST("/meilisearch/sync-all", middleware.AuthMiddleware(), middleware.AdminMiddleware(), meilisearchHandler.SyncAllResources)
		api.GET("/meilisearch/sync-progress", middleware.AuthMiddleware(), middleware.AdminMiddleware(), meilisearchHandler.GetSyncProgress)
		api.POST("/meilisearch/stop-sync", middleware.AuthMiddleware(), middleware.AdminMiddleware(), meilisearchHandler.StopSync)
		api.POST("/meilisearch/clear-index", middleware.AuthMiddleware(), middleware.AdminMiddleware(), meilisearchHandler.ClearIndex)
		api.POST("/meilisearch/test-connection", middleware.AuthMiddleware(), middleware.AdminMiddleware(), meilisearchHandler.TestConnection)
		api.POST("/meilisearch/update-settings", middleware.AuthMiddleware(), middleware.AdminMiddleware(), meilisearchHandler.UpdateIndexSettings)

		// �ļ��ϴ����·��
		api.POST("/files/upload", middleware.AuthMiddleware(), fileHandler.UploadFile)
		api.GET("/files", middleware.AuthMiddleware(), fileHandler.GetFileList)
		api.DELETE("/files", middleware.AuthMiddleware(), fileHandler.DeleteFiles)
		api.PUT("/files", middleware.AuthMiddleware(), fileHandler.UpdateFile)
		// ΢�Ź��ں���֤�ļ��ϴ���������֤����֧��TXT�ļ���
		api.POST("/wechat/verify-file", fileHandler.UploadWechatVerifyFile)

		// ����Telegram Bot����
		telegramBotService := services.NewTelegramBotService(
			repoManager.SystemConfigRepository,
			repoManager.TelegramChannelRepository,
			repoManager.ResourceRepository,
			repoManager.ReadyResourceRepository,
		)

		// ����Telegram Bot����
		if err := telegramBotService.Start(); err != nil {
			utils.Error("����Telegram Bot����ʧ��: %v", err)
		}

		// ����΢�Ź��ںŻ����˷���
		wechatBotService := services.NewWechatBotService(
			repoManager.SystemConfigRepository,
			repoManager.ResourceRepository,
			repoManager.ReadyResourceRepository,
		)

		// ����΢�Ź��ںŻ����˷���
		if err := wechatBotService.Start(); err != nil {
			utils.Error("����΢�Ź��ںŻ����˷���ʧ��: %v", err)
		}

		// Telegram���·��
		telegramHandler := handlers.NewTelegramHandler(
			repoManager.TelegramChannelRepository,
			repoManager.SystemConfigRepository,
			telegramBotService,
		)
		api.GET("/telegram/bot-config", middleware.AuthMiddleware(), middleware.AdminMiddleware(), telegramHandler.GetBotConfig)
		api.PUT("/telegram/bot-config", middleware.AuthMiddleware(), middleware.AdminMiddleware(), telegramHandler.UpdateBotConfig)
		api.POST("/telegram/validate-api-key", middleware.AuthMiddleware(), middleware.AdminMiddleware(), telegramHandler.ValidateApiKey)
		api.GET("/telegram/bot-status", middleware.AuthMiddleware(), middleware.AdminMiddleware(), telegramHandler.GetBotStatus)
		api.POST("/telegram/reload-config", middleware.AuthMiddleware(), middleware.AdminMiddleware(), telegramHandler.ReloadBotConfig)
		api.POST("/telegram/test-message", middleware.AuthMiddleware(), middleware.AdminMiddleware(), telegramHandler.TestBotMessage)
		api.GET("/telegram/debug-connection", middleware.AuthMiddleware(), middleware.AdminMiddleware(), telegramHandler.DebugBotConnection)
		api.GET("/telegram/channels", middleware.AuthMiddleware(), middleware.AdminMiddleware(), telegramHandler.GetChannels)
		api.POST("/telegram/channels", middleware.AuthMiddleware(), middleware.AdminMiddleware(), telegramHandler.CreateChannel)
		api.PUT("/telegram/channels/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), telegramHandler.UpdateChannel)
		api.DELETE("/telegram/channels/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), telegramHandler.DeleteChannel)
		api.GET("/telegram/logs", middleware.AuthMiddleware(), middleware.AdminMiddleware(), telegramHandler.GetTelegramLogs)
		api.GET("/telegram/logs/stats", middleware.AuthMiddleware(), middleware.AdminMiddleware(), telegramHandler.GetTelegramLogStats)
		api.POST("/telegram/logs/clear", middleware.AuthMiddleware(), middleware.AdminMiddleware(), telegramHandler.ClearTelegramLogs)
		api.POST("/telegram/webhook", telegramHandler.HandleWebhook)
		api.POST("/telegram/manual-push/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), telegramHandler.ManualPushToChannel)

		// ΢�Ź��ں����·��
		wechatHandler := handlers.NewWechatHandler(
			wechatBotService,
			repoManager.SystemConfigRepository,
		)
		api.GET("/wechat/bot-config", middleware.AuthMiddleware(), middleware.AdminMiddleware(), wechatHandler.GetBotConfig)
		api.PUT("/wechat/bot-config", middleware.AuthMiddleware(), middleware.AdminMiddleware(), wechatHandler.UpdateBotConfig)
		api.GET("/wechat/bot-status", middleware.AuthMiddleware(), middleware.AdminMiddleware(), wechatHandler.GetBotStatus)
		api.POST("/wechat/callback", wechatHandler.HandleWechatMessage)
		api.GET("/wechat/callback", wechatHandler.HandleWechatMessage)

		// OGͼƬ����·��
		api.GET("/og-image", ogImageHandler.GenerateOGImage)

		// �ٱ��Ͱ�Ȩ����·��
		api.POST("/reports", reportHandler.CreateReport)
		api.GET("/reports/:id", reportHandler.GetReport)
		api.GET("/reports", middleware.AuthMiddleware(), middleware.AdminMiddleware(), reportHandler.ListReports)
		api.PUT("/reports/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), reportHandler.UpdateReport)
		api.DELETE("/reports/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), reportHandler.DeleteReport)
		api.GET("/reports/resource/:resource_key", reportHandler.GetReportByResource)

		api.POST("/copyright-claims", copyrightClaimHandler.CreateCopyrightClaim)
		api.GET("/copyright-claims/:id", copyrightClaimHandler.GetCopyrightClaim)
		api.GET("/copyright-claims", middleware.AuthMiddleware(), middleware.AdminMiddleware(), copyrightClaimHandler.ListCopyrightClaims)
		api.PUT("/copyright-claims/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), copyrightClaimHandler.UpdateCopyrightClaim)
		api.DELETE("/copyright-claims/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), copyrightClaimHandler.DeleteCopyrightClaim)
		api.GET("/copyright-claims/resource/:resource_key", copyrightClaimHandler.GetCopyrightClaimByResource)

		// Sitemap��̬�ļ�����������API·�ɣ�
		// �ṩ���ɵ�sitemap.xml�����ļ�
		r.StaticFile("/sitemap.xml", "./data/sitemap/sitemap.xml")
		// �ṩ���ɵ�sitemap��ҳ�ļ���ʹ��ͨ���·��
		r.GET("/sitemap-:page", func(c *gin.Context) {
			page := c.Param("page")
			if !strings.HasSuffix(page, ".xml") {
				c.JSON(http.StatusNotFound, gin.H{"error": "�ļ�������"})
				return
			}
			c.File("./data/sitemap/sitemap-" + page)
		})

		// Sitemap��̬�ļ�API·�ɣ�API���ݣ�
		api.GET("/sitemap.xml", func(c *gin.Context) {
			c.File("./data/sitemap/sitemap.xml")
		})
		// �ṩ���ɵ�sitemap��ҳ�ļ���ʹ��API·��
		api.GET("/sitemap-:page", func(c *gin.Context) {
			page := c.Param("page")
			if !strings.HasSuffix(page, ".xml") {
				c.JSON(http.StatusNotFound, gin.H{"error": "�ļ�������"})
				return
			}
			c.File("./data/sitemap/sitemap-" + page)
		})

		// Sitemap����API��ͨ������Ա�ӿڽ��й�����
		api.GET("/sitemap/config", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.GetSitemapConfig)
		api.POST("/sitemap/config", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.UpdateSitemapConfig)
		api.POST("/sitemap/generate", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.GenerateSitemap)
		api.GET("/sitemap/status", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.GetSitemapStatus)
		api.POST("/sitemap/full-generate", middleware.AuthMiddleware(), middleware.AdminMiddleware(), handlers.GenerateFullSitemap)

		// Google��������API
		api.GET("/google-index/config", middleware.AuthMiddleware(), middleware.AdminMiddleware(), googleIndexHandler.GetConfig)
		api.GET("/google-index/config-all", middleware.AuthMiddleware(), middleware.AdminMiddleware(), googleIndexHandler.GetAllConfig)  // ��ȡ��������
		api.POST("/google-index/config", middleware.AuthMiddleware(), middleware.AdminMiddleware(), googleIndexHandler.UpdateConfig)
		api.POST("/google-index/config/update", middleware.AuthMiddleware(), middleware.AdminMiddleware(), googleIndexHandler.UpdateGoogleIndexConfig)  // �������ø���
		api.GET("/google-index/status", middleware.AuthMiddleware(), middleware.AdminMiddleware(), googleIndexHandler.GetStatus)  // ��ȡ״̬
		api.POST("/google-index/tasks", middleware.AuthMiddleware(), middleware.AdminMiddleware(), googleIndexHandler.CreateTask)
		api.GET("/google-index/tasks", middleware.AuthMiddleware(), middleware.AdminMiddleware(), googleIndexHandler.GetTasks)
		api.GET("/google-index/tasks/:id", middleware.AuthMiddleware(), middleware.AdminMiddleware(), googleIndexHandler.GetTaskStatus)
		api.POST("/google-index/tasks/:id/start", middleware.AuthMiddleware(), middleware.AdminMiddleware(), googleIndexHandler.StartTask)
		api.GET("/google-index/tasks/:id/items", middleware.AuthMiddleware(), middleware.AdminMiddleware(), googleIndexHandler.GetTaskItems)

		// Google����ƾ���ϴ�����֤API
		api.POST("/google-index/upload-credentials", middleware.AuthMiddleware(), middleware.AdminMiddleware(), googleIndexHandler.UploadCredentials)
		api.POST("/google-index/validate-credentials", middleware.AuthMiddleware(), middleware.AdminMiddleware(), googleIndexHandler.ValidateCredentials)
		api.POST("/google-index/diagnose-permissions", middleware.AuthMiddleware(), middleware.AdminMiddleware(), googleIndexHandler.DiagnosePermissions)
		api.POST("/google-index/urls/submit-to-index", middleware.AuthMiddleware(), middleware.AdminMiddleware(), googleIndexHandler.SubmitURLsToIndex)

		// Bing�ύAPI
		if bingHandler != nil {
			// Bing����API
			api.GET("/bing/config", middleware.AuthMiddleware(), middleware.AdminMiddleware(), bingHandler.GetBingIndexConfig)
			api.POST("/bing/config", middleware.AuthMiddleware(), middleware.AdminMiddleware(), bingHandler.UpdateBingIndexConfig)
		}

		// �������API
		routes.SetupPluginRoutes(r, repoManager, pluginManager)
	}

	// ���ü��ϵͳ
	monitor.SetupMonitoring(r)

	// ������ط�����
	metricsConfig := &monitor.MetricsConfig{
		Enabled:       true,
		ListenAddress: ":9090",
		MetricsPath:   "/metrics",
		Namespace:     "urldb",
		Subsystem:     "api",
	}
	metrics.StartMetricsServer(metricsConfig)

	// ��̬�ļ�����
	r.Static("/uploads", "./uploads")
	r.Static("/data", "./data")

	// ����CORSͷ����̬�ļ�
	r.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/uploads/") || strings.HasPrefix(c.Request.URL.Path, "/data/") {
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept")
		}
		c.Next()
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// �������Źر�
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// ��goroutine������������
	go func() {
		utils.Info("�����������ڶ˿� %s", port)
		if err := r.Run(":" + port); err != nil && err.Error() != "http: Server closed" {
			utils.Fatal("����������ʧ��: %v", err)
		}
	}()

	// �ȴ��ź�
	<-quit
	utils.Info("�յ��ر��źţ���ʼ���Źر�...")

	utils.Info("�����������Źر�")
}
