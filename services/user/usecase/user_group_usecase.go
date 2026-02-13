package usecase

import (
	"fmt"
	"strings"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/repository"
	sharedmodels "tachyon-messenger/shared/models"
)

// UserGroupUsecase defines the interface for user group business logic
type UserGroupUsecase interface {
	CreateGroup(req *models.CreateUserGroupRequest, creatorID uint) (*models.UserGroupResponse, error)
	GetGroup(id uint) (*models.UserGroupWithMembersResponse, error)
	GetAllGroups() ([]*models.UserGroupResponse, error)
	UpdateGroup(id uint, req *models.UpdateUserGroupRequest, userID uint, userRole string) (*models.UserGroupResponse, error)
	DeleteGroup(id uint, userID uint, userRole string) error
	SetMembers(groupID uint, req *models.UpdateUserGroupMembersRequest, userID uint, userRole string) (*models.UserGroupWithMembersResponse, error)
	AddMembers(groupID uint, req *models.AddRemoveUserGroupMembersRequest, userID uint, userRole string) error
	RemoveMembers(groupID uint, req *models.AddRemoveUserGroupMembersRequest, userID uint, userRole string) error
	GetAllGroupsWithMembers() ([]*models.UserGroupWithMembersResponse, error)
}

// userGroupUsecase implements UserGroupUsecase interface
type userGroupUsecase struct {
	groupRepo repository.UserGroupRepository
	userRepo  repository.UserRepository
}

// NewUserGroupUsecase creates a new user group usecase
func NewUserGroupUsecase(groupRepo repository.UserGroupRepository, userRepo repository.UserRepository) UserGroupUsecase {
	return &userGroupUsecase{
		groupRepo: groupRepo,
		userRepo:  userRepo,
	}
}

// canManageGroup checks if a user can manage (edit/delete) a specific group
func (uc *userGroupUsecase) canManageGroup(group *models.UserGroup, userID uint, userRole string) error {
	role := sharedmodels.Role(userRole)
	// Admins and super admins can manage any group
	if role == sharedmodels.RoleAdmin || role == sharedmodels.RoleSuperAdmin {
		return nil
	}
	// Department heads can only manage their own groups
	if role == sharedmodels.RoleDepartmentHead && group.CreatorID == userID {
		return nil
	}
	return fmt.Errorf("insufficient permissions to manage this group")
}

// CreateGroup creates a new user group
func (uc *userGroupUsecase) CreateGroup(req *models.CreateUserGroupRequest, creatorID uint) (*models.UserGroupResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("group name is required")
	}
	if len(name) < 2 {
		return nil, fmt.Errorf("group name must be at least 2 characters long")
	}
	if len(name) > 100 {
		return nil, fmt.Errorf("group name must be less than 100 characters")
	}

	group := &models.UserGroup{
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		CreatorID:   creatorID,
	}

	if err := uc.groupRepo.Create(group); err != nil {
		return nil, fmt.Errorf("failed to create user group: %w", err)
	}

	// Add initial members if provided
	if len(req.UserIDs) > 0 {
		if err := uc.groupRepo.AddMembers(group.ID, req.UserIDs); err != nil {
			return nil, fmt.Errorf("failed to add initial members: %w", err)
		}
	}

	memberCount, _ := uc.groupRepo.CountMembers(group.ID)

	return group.ToResponseWithMemberCount(int(memberCount)), nil
}

// GetGroup retrieves a user group by ID with its members
func (uc *userGroupUsecase) GetGroup(id uint) (*models.UserGroupWithMembersResponse, error) {
	group, err := uc.groupRepo.GetByID(id)
	if err != nil {
		return nil, err
	}

	members, err := uc.groupRepo.GetMembers(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get group members: %w", err)
	}

	memberResponses := make([]*models.UserResponse, len(members))
	for i, member := range members {
		memberResponses[i] = member.ToResponse()
	}

	return &models.UserGroupWithMembersResponse{
		ID:          group.ID,
		Name:        group.Name,
		Description: group.Description,
		CreatorID:   group.CreatorID,
		Members:     memberResponses,
		MemberCount: len(memberResponses),
		CreatedAt:   group.CreatedAt,
		UpdatedAt:   group.UpdatedAt,
	}, nil
}

// GetAllGroups retrieves all user groups with member counts
func (uc *userGroupUsecase) GetAllGroups() ([]*models.UserGroupResponse, error) {
	groups, counts, err := uc.groupRepo.GetAllWithMemberCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get user groups: %w", err)
	}

	responses := make([]*models.UserGroupResponse, len(groups))
	for i, group := range groups {
		responses[i] = group.ToResponseWithMemberCount(int(counts[i]))
	}

	return responses, nil
}

// UpdateGroup updates an existing user group
func (uc *userGroupUsecase) UpdateGroup(id uint, req *models.UpdateUserGroupRequest, userID uint, userRole string) (*models.UserGroupResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	group, err := uc.groupRepo.GetByID(id)
	if err != nil {
		return nil, err
	}

	// Check permissions
	if err := uc.canManageGroup(group, userID, userRole); err != nil {
		return nil, err
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("group name cannot be empty")
		}
		if len(name) < 2 {
			return nil, fmt.Errorf("group name must be at least 2 characters long")
		}
		if len(name) > 100 {
			return nil, fmt.Errorf("group name must be less than 100 characters")
		}
		group.Name = name
	}

	if req.Description != nil {
		group.Description = strings.TrimSpace(*req.Description)
	}

	if err := uc.groupRepo.Update(group); err != nil {
		return nil, fmt.Errorf("failed to update user group: %w", err)
	}

	memberCount, _ := uc.groupRepo.CountMembers(group.ID)

	return group.ToResponseWithMemberCount(int(memberCount)), nil
}

// DeleteGroup deletes a user group
func (uc *userGroupUsecase) DeleteGroup(id uint, userID uint, userRole string) error {
	group, err := uc.groupRepo.GetByID(id)
	if err != nil {
		return err
	}

	// Check permissions
	if err := uc.canManageGroup(group, userID, userRole); err != nil {
		return err
	}

	if err := uc.groupRepo.Delete(id); err != nil {
		return fmt.Errorf("failed to delete user group: %w", err)
	}

	return nil
}

// SetMembers replaces all members of a group
func (uc *userGroupUsecase) SetMembers(groupID uint, req *models.UpdateUserGroupMembersRequest, userID uint, userRole string) (*models.UserGroupWithMembersResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	group, err := uc.groupRepo.GetByID(groupID)
	if err != nil {
		return nil, err
	}

	// Check permissions
	if err := uc.canManageGroup(group, userID, userRole); err != nil {
		return nil, err
	}

	if err := uc.groupRepo.SetMembers(groupID, req.UserIDs); err != nil {
		return nil, fmt.Errorf("failed to set group members: %w", err)
	}

	// Return updated group with members
	return uc.GetGroup(groupID)
}

// AddMembers adds users to a group
func (uc *userGroupUsecase) AddMembers(groupID uint, req *models.AddRemoveUserGroupMembersRequest, userID uint, userRole string) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	group, err := uc.groupRepo.GetByID(groupID)
	if err != nil {
		return err
	}

	// Check permissions
	if err := uc.canManageGroup(group, userID, userRole); err != nil {
		return err
	}

	if err := uc.groupRepo.AddMembers(groupID, req.UserIDs); err != nil {
		return fmt.Errorf("failed to add members to group: %w", err)
	}

	return nil
}

// RemoveMembers removes users from a group
func (uc *userGroupUsecase) RemoveMembers(groupID uint, req *models.AddRemoveUserGroupMembersRequest, userID uint, userRole string) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	group, err := uc.groupRepo.GetByID(groupID)
	if err != nil {
		return err
	}

	// Check permissions
	if err := uc.canManageGroup(group, userID, userRole); err != nil {
		return err
	}

	if err := uc.groupRepo.RemoveMembers(groupID, req.UserIDs); err != nil {
		return fmt.Errorf("failed to remove members from group: %w", err)
	}

	return nil
}

// GetAllGroupsWithMembers retrieves all groups with their full member lists (for UserSelectorModal)
func (uc *userGroupUsecase) GetAllGroupsWithMembers() ([]*models.UserGroupWithMembersResponse, error) {
	groups, err := uc.groupRepo.GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get user groups: %w", err)
	}

	responses := make([]*models.UserGroupWithMembersResponse, 0, len(groups))
	for _, group := range groups {
		members, err := uc.groupRepo.GetMembers(group.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get members for group %d: %w", group.ID, err)
		}

		memberResponses := make([]*models.UserResponse, len(members))
		for i, member := range members {
			memberResponses[i] = member.ToResponse()
		}

		responses = append(responses, &models.UserGroupWithMembersResponse{
			ID:          group.ID,
			Name:        group.Name,
			Description: group.Description,
			CreatorID:   group.CreatorID,
			Members:     memberResponses,
			MemberCount: len(memberResponses),
			CreatedAt:   group.CreatedAt,
			UpdatedAt:   group.UpdatedAt,
		})
	}

	return responses, nil
}
