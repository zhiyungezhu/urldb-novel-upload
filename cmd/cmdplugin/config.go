package cmdplugin

import (
	"fmt"
	"os"
	"strconv"

	"github.com/zhiyungezhu/urldb-novel-upload/utils"
)

// PluginConfig 插件系统配置
type PluginConfig struct {
	// 是否启用插件系统
	Enabled bool `json:"enabled" env:"PLUGIN_ENABLED"`

	// 是否启用热重载
	HotReload bool `json:"hot_reload" env:"PLUGIN_HOT_RELOAD"`

	// 钩子目录
	HooksDir string `json:"hooks_dir" env:"PLUGIN_HOOKS_DIR"`

	// 迁移目录
	MigrationsDir string `json:"migrations_dir" env:"PLUGIN_MIGRATIONS_DIR"`

	// 类型定义目录
	TypesDir string `json:"types_dir" env:"PLUGIN_TYPES_DIR"`

	// VM 池大小
	VMPoolSize int `json:"vm_pool_size" env:"PLUGIN_VM_POOL_SIZE"`

	// 是否启用调试模式
	Debug bool `json:"debug" env:"PLUGIN_DEBUG"`
}

// LoadPluginConfig 加载插件配置
func LoadPluginConfig() *PluginConfig {
	config := &PluginConfig{
		Enabled:      getEnvBool("PLUGIN_ENABLED", true),
		HotReload:    getEnvBool("PLUGIN_HOT_RELOAD", true),
		HooksDir:     getEnvString("PLUGIN_HOOKS_DIR", "./plugin-system/hooks"),
		MigrationsDir: getEnvString("PLUGIN_MIGRATIONS_DIR", "./migrations"),
		TypesDir:      getEnvString("PLUGIN_TYPES_DIR", "./plugin-system/types"),
		VMPoolSize:    getEnvInt("PLUGIN_VM_POOL_SIZE", 10),
		Debug:        getEnvBool("PLUGIN_DEBUG", false),
	}

	utils.Info("Plugin configuration loaded:")
	utils.Info("  - Enabled: %v", config.Enabled)
	utils.Info("  - Hot Reload: %v", config.HotReload)
	utils.Info("  - Hooks Dir: %s", config.HooksDir)
	utils.Info("  - Migrations Dir: %s", config.MigrationsDir)
	utils.Info("  - Types Dir: %s", config.TypesDir)
	utils.Info("  - VM Pool Size: %d", config.VMPoolSize)
	utils.Info("  - Debug: %v", config.Debug)

	return config
}

// getEnvString 获取环境变量字符串值
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvBool 获取环境变量布尔值
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// getEnvInt 获取环境变量整数值
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// ValidatePluginConfig 验证插件配置
func ValidatePluginConfig(config *PluginConfig) error {
	// 检查必需的目录
	if config.HooksDir == "" {
		return fmt.Errorf("Hooks directory cannot be empty")
	}

	if config.MigrationsDir == "" {
		return fmt.Errorf("Migrations directory cannot be empty")
	}

	if config.TypesDir == "" {
		return fmt.Errorf("Types directory cannot be empty")
	}

	// 检查 VM 池大小
	if config.VMPoolSize <= 0 {
		return fmt.Errorf("VM pool size must be greater than 0")
	}

	if config.VMPoolSize > 100 {
		utils.Warn("VM pool size is very large (%d), consider reducing it", config.VMPoolSize)
	}

	utils.Info("Plugin configuration validation passed")
	return nil
}

// EnsureDirectories 确保插件系统所需的目录存在
func EnsureDirectories(config *PluginConfig) error {
	dirs := []string{
		config.HooksDir,
		config.MigrationsDir,
		config.TypesDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("Failed to create directory %s: %v", dir, err)
		}
		utils.Info("Directory ensured: %s", dir)
	}

	return nil
}