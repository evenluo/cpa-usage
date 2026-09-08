package dto

// StorageCleanupResult 是仓储层每日清理的结果。
type StorageCleanupResult struct {
	RedisInbox RedisUsageInboxCleanupResult
	Vacuum     StorageVacuumResult
}

// StorageVacuumResult records the reclaimable-page check that decides whether
// the daily cleanup needs a full SQLite rewrite.
type StorageVacuumResult struct {
	DatabasePages    int64
	ReclaimablePages int64
	Vacuumed         bool
}
