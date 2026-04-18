package handlers

import (
	"errors"
	"net/http"
	"time"

	"ecommerce/internal/media"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const mediaReadyTimeout = 2 * time.Second

type StatusError struct {
	StatusCode int
	Message    string
	Err        error
}

func (e *StatusError) Error() string {
	return e.Message
}

func (e *StatusError) Unwrap() error {
	return e.Err
}

func writeStatusError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}

	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		return false
	}

	c.JSON(statusErr.StatusCode, gin.H{"error": statusErr.Message})
	return true
}

func mediaLookupStatusError(err error, notFoundMessage string, loadFailureMessage string, failedMessage string, processingMessage string) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, media.ErrMediaNotFound):
		return &StatusError{StatusCode: http.StatusNotFound, Message: notFoundMessage, Err: err}
	case errors.Is(err, media.ErrMediaProcessingFailed):
		return &StatusError{StatusCode: http.StatusUnprocessableEntity, Message: failedMessage, Err: err}
	case errors.Is(err, media.ErrMediaStillProcessing):
		return &StatusError{StatusCode: http.StatusConflict, Message: processingMessage, Err: err}
	default:
		return &StatusError{StatusCode: http.StatusInternalServerError, Message: loadFailureMessage, Err: err}
	}
}

func AttachProductMediaToDraft(db *gorm.DB, mediaService *media.Service, product *models.Product, mediaIDs []string) error {
	if db == nil {
		return errors.New("db is required")
	}
	if mediaService == nil {
		return errors.New("media service is required")
	}
	if product == nil {
		return errors.New("product is required")
	}
	if len(mediaIDs) == 0 {
		return &StatusError{StatusCode: http.StatusBadRequest, Message: "Media IDs required"}
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		_, err := ensureProductCatalogDraft(tx, product)
		return err
	}); err != nil {
		return &StatusError{StatusCode: http.StatusInternalServerError, Message: "Failed to initialize product draft", Err: err}
	}

	var maxPosition int
	if err := db.Model(&models.MediaReference{}).
		Where("owner_type = ? AND owner_id = ? AND role = ?",
			media.OwnerTypeProduct, product.ID, media.RoleProductDraftImage).
		Select("COALESCE(MAX(position), 0)").
		Scan(&maxPosition).Error; err != nil {
		return &StatusError{StatusCode: http.StatusInternalServerError, Message: "Failed to load media order", Err: err}
	}

	for _, mediaID := range mediaIDs {
		if _, err := mediaService.WaitUntilReady(mediaID, mediaReadyTimeout); err != nil {
			return mediaLookupStatusError(
				err,
				"Media not found: "+mediaID,
				"Failed to load media: "+mediaID,
				"Media processing failed: "+mediaID,
				"Media is still processing: "+mediaID,
			)
		}

		maxPosition++
		ref := models.MediaReference{
			MediaID:   mediaID,
			OwnerType: media.OwnerTypeProduct,
			OwnerID:   product.ID,
			Role:      media.RoleProductDraftImage,
			Position:  maxPosition,
		}
		if err := db.Where("media_id = ? AND owner_type = ? AND owner_id = ? AND role = ?",
			ref.MediaID, ref.OwnerType, ref.OwnerID, ref.Role).FirstOrCreate(&ref).Error; err != nil {
			return &StatusError{StatusCode: http.StatusInternalServerError, Message: "Failed to attach media", Err: err}
		}
	}

	return nil
}
