package service

import (
	"context"

	"cpa-usage/internal/repository"
	repodto "cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

type RollupBackfillStatusProvider interface {
	GetRollupBackfillStatus(context.Context) (repodto.RollupBackfillStatus, error)
}

type rollupBackfillService struct {
	db *gorm.DB
}

func NewRollupBackfillService(db *gorm.DB) RollupBackfillStatusProvider {
	return &rollupBackfillService{db: db}
}

func (s *rollupBackfillService) GetRollupBackfillStatus(context.Context) (repodto.RollupBackfillStatus, error) {
	return repository.GetUsageRollupBackfillStatus(s.db)
}
