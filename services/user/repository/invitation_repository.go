package repository

import (
	"fmt"
	"time"

	"tachyon-messenger/services/user/models"

	"gorm.io/gorm"
)

// InvitationRepository defines the interface for invitation data operations
type InvitationRepository interface {
	Create(invitation *models.Invitation) error
	GetByID(id uint) (*models.Invitation, error)
	GetByToken(token string) (*models.Invitation, error)
	GetByEmail(email string) (*models.Invitation, error)
	GetWithRelations(id uint) (*models.Invitation, error)
	GetByTokenWithRelations(token string) (*models.Invitation, error)
	Update(invitation *models.Invitation) error
	Delete(id uint) error
	List(filters map[string]interface{}, page, pageSize int) ([]*models.Invitation, int64, error)
	GetStats() (*models.InvitationStatsResponse, error)
	ExpireOldInvitations() (int64, error)
	HasPendingInvitation(email string) (bool, error)
	CancelPendingInvitationsByEmail(email string) error
}

// invitationRepository implements InvitationRepository interface
type invitationRepository struct {
	db *gorm.DB
}

// NewInvitationRepository creates a new invitation repository
func NewInvitationRepository(db *gorm.DB) InvitationRepository {
	return &invitationRepository{
		db: db,
	}
}

// Create creates a new invitation
func (r *invitationRepository) Create(invitation *models.Invitation) error {
	return r.db.Create(invitation).Error
}

// GetByID retrieves an invitation by ID
func (r *invitationRepository) GetByID(id uint) (*models.Invitation, error) {
	var invitation models.Invitation
	err := r.db.Where("id = ?", id).First(&invitation).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("invitation not found")
		}
		return nil, err
	}
	return &invitation, nil
}

// GetByToken retrieves an invitation by token
func (r *invitationRepository) GetByToken(token string) (*models.Invitation, error) {
	var invitation models.Invitation
	err := r.db.Where("token = ?", token).First(&invitation).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("invitation not found")
		}
		return nil, err
	}
	return &invitation, nil
}

// GetByEmail retrieves an invitation by email (latest one)
func (r *invitationRepository) GetByEmail(email string) (*models.Invitation, error) {
	var invitation models.Invitation
	err := r.db.Where("email = ?", email).Order("created_at DESC").First(&invitation).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("invitation not found")
		}
		return nil, err
	}
	return &invitation, nil
}

// GetWithRelations retrieves an invitation with all relations (department, created by, user)
func (r *invitationRepository) GetWithRelations(id uint) (*models.Invitation, error) {
	var invitation models.Invitation
	err := r.db.
		Preload("Department").
		Preload("CreatedBy").
		Preload("User").
		Where("id = ?", id).
		First(&invitation).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("invitation not found")
		}
		return nil, err
	}
	return &invitation, nil
}

// GetByTokenWithRelations retrieves an invitation by token with all relations
func (r *invitationRepository) GetByTokenWithRelations(token string) (*models.Invitation, error) {
	var invitation models.Invitation
	err := r.db.
		Preload("Department").
		Preload("CreatedBy").
		Preload("User").
		Where("token = ?", token).
		First(&invitation).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("invitation not found")
		}
		return nil, err
	}
	return &invitation, nil
}

// Update updates an invitation
func (r *invitationRepository) Update(invitation *models.Invitation) error {
	return r.db.Save(invitation).Error
}

// Delete deletes an invitation
func (r *invitationRepository) Delete(id uint) error {
	return r.db.Delete(&models.Invitation{}, id).Error
}

// List retrieves a paginated list of invitations with optional filters
func (r *invitationRepository) List(filters map[string]interface{}, page, pageSize int) ([]*models.Invitation, int64, error) {
	var invitations []*models.Invitation
	var total int64

	query := r.db.Model(&models.Invitation{}).
		Preload("Department").
		Preload("CreatedBy").
		Preload("User")

	// Apply filters
	if status, ok := filters["status"].(string); ok && status != "" {
		query = query.Where("status = ?", status)
	}

	if email, ok := filters["email"].(string); ok && email != "" {
		query = query.Where("email LIKE ?", "%"+email+"%")
	}

	if createdByID, ok := filters["created_by_id"].(uint); ok && createdByID > 0 {
		query = query.Where("created_by_id = ?", createdByID)
	}

	if role, ok := filters["role"].(string); ok && role != "" {
		query = query.Where("role = ?", role)
	}

	if departmentID, ok := filters["department_id"].(uint); ok && departmentID > 0 {
		query = query.Where("department_id = ?", departmentID)
	}

	// Filter by validity
	if isValid, ok := filters["is_valid"].(bool); ok {
		if isValid {
			query = query.Where("status = ? AND expires_at > ?", models.InvitationStatusPending, time.Now())
		}
	}

	// Count total records
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	offset := (page - 1) * pageSize
	if err := query.
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&invitations).Error; err != nil {
		return nil, 0, err
	}

	return invitations, total, nil
}

// GetStats retrieves invitation statistics
func (r *invitationRepository) GetStats() (*models.InvitationStatsResponse, error) {
	stats := &models.InvitationStatsResponse{}

	// Total invitations
	var totalInvitations int64
	if err := r.db.Model(&models.Invitation{}).Count(&totalInvitations).Error; err != nil {
		return nil, err
	}
	stats.TotalInvitations = int(totalInvitations)

	// Pending invitations (valid and not expired)
	var pendingInvitations int64
	if err := r.db.Model(&models.Invitation{}).
		Where("status = ? AND expires_at > ?", models.InvitationStatusPending, time.Now()).
		Count(&pendingInvitations).Error; err != nil {
		return nil, err
	}
	stats.PendingInvitations = int(pendingInvitations)

	// Accepted invitations
	var acceptedInvitations int64
	if err := r.db.Model(&models.Invitation{}).
		Where("status = ?", models.InvitationStatusAccepted).
		Count(&acceptedInvitations).Error; err != nil {
		return nil, err
	}
	stats.AcceptedInvitations = int(acceptedInvitations)

	// Expired invitations
	var expiredInvitations int64
	if err := r.db.Model(&models.Invitation{}).
		Where("status = ?", models.InvitationStatusExpired).
		Count(&expiredInvitations).Error; err != nil {
		return nil, err
	}
	stats.ExpiredInvitations = int(expiredInvitations)

	// Cancelled invitations
	var cancelledInvitations int64
	if err := r.db.Model(&models.Invitation{}).
		Where("status = ?", models.InvitationStatusCancelled).
		Count(&cancelledInvitations).Error; err != nil {
		return nil, err
	}
	stats.CancelledInvitations = int(cancelledInvitations)

	return stats, nil
}

// ExpireOldInvitations marks expired invitations as expired
func (r *invitationRepository) ExpireOldInvitations() (int64, error) {
	result := r.db.Model(&models.Invitation{}).
		Where("status = ? AND expires_at <= ?", models.InvitationStatusPending, time.Now()).
		Update("status", models.InvitationStatusExpired)

	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}

// HasPendingInvitation checks if there's a pending invitation for the given email
func (r *invitationRepository) HasPendingInvitation(email string) (bool, error) {
	var count int64
	err := r.db.Model(&models.Invitation{}).
		Where("email = ? AND status = ? AND expires_at > ?", email, models.InvitationStatusPending, time.Now()).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CancelPendingInvitationsByEmail cancels all pending invitations for the given email
func (r *invitationRepository) CancelPendingInvitationsByEmail(email string) error {
	return r.db.Model(&models.Invitation{}).
		Where("email = ? AND status = ?", email, models.InvitationStatusPending).
		Update("status", models.InvitationStatusCancelled).Error
}
