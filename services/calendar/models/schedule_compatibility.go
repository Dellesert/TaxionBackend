package models

// ScheduleTypeCompatibility defines which schedule types can coexist
// If a record (work, on_duty) exists, it means a user can be in both
// a work schedule and an on_duty schedule at the same time
type ScheduleTypeCompatibility struct {
	ID             uint         `gorm:"primaryKey" json:"id"`
	ScheduleType   ScheduleType `gorm:"not null;index;size:30" json:"schedule_type" validate:"required,oneof=work paid_services on_duty vk trips"`
	CompatibleWith ScheduleType `gorm:"not null;size:30" json:"compatible_with" validate:"required,oneof=work paid_services on_duty vk trips"`
}

// TableName returns the table name for ScheduleTypeCompatibility model
func (ScheduleTypeCompatibility) TableName() string {
	return "schedule_type_compatibilities"
}
