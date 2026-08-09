package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/media"
	"ecommerce/internal/requestctx"
	"ecommerce/models"

	"gorm.io/gorm"
)

func (e *CmsMediaEndpoints) SetProfilePhoto(ctx context.Context, request apicontract.SetProfilePhotoRequestObject) (apicontract.SetProfilePhotoResponseObject, error) {
	if e.media == nil {
		return nil, errors.New("media service is required")
	}
	if request.Body == nil || strings.TrimSpace(request.Body.MediaId) == "" {
		return nil, problemError(http.StatusBadRequest, "invalid_request", "A media ID is required.", nil)
	}
	user, err := e.profileUser(ctx)
	if err != nil {
		return nil, err
	}
	object, err := e.media.WaitUntilReady(ctx, request.Body.MediaId, 0)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(object.MimeType, "image/") {
		return nil, problemError(http.StatusBadRequest, "invalid_media", "Profile photo must be an image.", nil)
	}
	if object.SizeBytes > media.DefaultProfilePhotoMaxBytes {
		return nil, problemError(http.StatusRequestEntityTooLarge, "payload_too_large", "Profile photo is too large.", nil)
	}

	db := e.db.WithContext(ctx)
	var existing []models.MediaReference
	if err := db.Where("owner_type = ? AND owner_id = ? AND role = ?", media.OwnerTypeUser, user.ID, media.RoleProfilePhoto).Find(&existing).Error; err != nil {
		return nil, err
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("owner_type = ? AND owner_id = ? AND role = ?", media.OwnerTypeUser, user.ID, media.RoleProfilePhoto).Delete(&models.MediaReference{}).Error; err != nil {
			return err
		}
		return tx.Create(&models.MediaReference{MediaID: request.Body.MediaId, OwnerType: media.OwnerTypeUser, OwnerID: user.ID, Role: media.RoleProfilePhoto}).Error
	}); err != nil {
		return nil, err
	}
	for _, reference := range existing {
		if reference.MediaID != request.Body.MediaId {
			_ = e.media.DeleteIfOrphan(reference.MediaID)
		}
	}
	if profileURL, err := e.media.UserProfilePhotoURL(user.ID); err == nil {
		user.ProfilePhoto = profileURL
	}
	return apicontract.SetProfilePhoto200JSONResponse(modelUser(user)), nil
}

func (e *CmsMediaEndpoints) DeleteProfilePhoto(ctx context.Context, _ apicontract.DeleteProfilePhotoRequestObject) (apicontract.DeleteProfilePhotoResponseObject, error) {
	if e.media == nil {
		return nil, errors.New("media service is required")
	}
	user, err := e.profileUser(ctx)
	if err != nil {
		return nil, err
	}
	db := e.db.WithContext(ctx)
	var references []models.MediaReference
	if err := db.Where("owner_type = ? AND owner_id = ? AND role = ?", media.OwnerTypeUser, user.ID, media.RoleProfilePhoto).Find(&references).Error; err != nil {
		return nil, err
	}
	if err := db.Where("owner_type = ? AND owner_id = ? AND role = ?", media.OwnerTypeUser, user.ID, media.RoleProfilePhoto).Delete(&models.MediaReference{}).Error; err != nil {
		return nil, err
	}
	for _, reference := range references {
		_ = e.media.DeleteIfOrphan(reference.MediaID)
	}
	user.ProfilePhoto = ""
	return apicontract.DeleteProfilePhoto200JSONResponse(modelUser(user)), nil
}

func (e *CmsMediaEndpoints) profileUser(ctx context.Context) (models.User, error) {
	principal, err := requestctx.RequirePrincipal(ctx)
	if err != nil {
		return models.User{}, problemError(http.StatusUnauthorized, "authentication_required", "Authentication is required.", err)
	}
	var user models.User
	if err := e.db.WithContext(ctx).Where("subject = ?", principal.Subject).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.User{}, problemError(http.StatusUnauthorized, "authentication_required", "The authenticated account is unavailable.", err)
		}
		return models.User{}, err
	}
	return user, nil
}
