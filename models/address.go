package models

type SavedAddress struct {
	BaseModel
	UserID     uint   `json:"user_id" gorm:"index;not null"`
	User       User   `json:"-" gorm:"foreignKey:UserID"`
	Label      string `json:"label"`
	FullName   string `json:"full_name"`
	Line1      string `json:"line1" gorm:"not null"`
	Line2      string `json:"line2"`
	City       string `json:"city" gorm:"not null"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code" gorm:"not null"`
	Country    string `json:"country" gorm:"size:2;not null"`
	Phone      string `json:"phone"`
	IsDefault  bool   `json:"is_default" gorm:"default:false"`
}
