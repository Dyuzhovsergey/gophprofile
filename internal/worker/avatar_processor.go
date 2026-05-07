package worker

import (
	"context"

	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	"go.uber.org/zap"
)

// AvatarProcessor обрабатывает события, связанные с аватарками.
type AvatarProcessor struct {
	log *zap.Logger
}

// NewAvatarProcessor создаёт обработчик событий аватарок.
func NewAvatarProcessor(log *zap.Logger) *AvatarProcessor {
	return &AvatarProcessor{
		log: log,
	}
}

// HandleAvatarUploaded обрабатывает событие загрузки аватарки.
func (p *AvatarProcessor) HandleAvatarUploaded(ctx context.Context, event domain.AvatarUploadEvent) error {
	_ = ctx

	p.log.Info(
		"avatar uploaded event received",
		zap.String("avatar_id", event.AvatarID),
		zap.String("user_id", event.UserID),
		zap.String("s3_key", event.S3Key),
	)

	return nil
}
