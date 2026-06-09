package handlers

import (
	"github.com/zhiyungezhu/urldb-novel-upload/db/repo"
	"github.com/zhiyungezhu/urldb-novel-upload/services"
)

var repoManager *repo.RepositoryManager
var meilisearchManager *services.MeilisearchManager

// SetRepositoryManager 设置Repository管理器
func SetRepositoryManager(manager *repo.RepositoryManager) {
	repoManager = manager
}

// SetMeilisearchManager 设置Meilisearch管理器
func SetMeilisearchManager(manager *services.MeilisearchManager) {
	meilisearchManager = manager
}
