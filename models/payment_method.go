package models

type SavedPaymentMethod struct {
	BaseModel
	UserID         uint   `json:"user_id" gorm:"index;not null"`
	User           User   `json:"-" gorm:"foreignKey:UserID"`
	Type           string `json:"type" gorm:"size:20;default:card"` // card
	Brand          string `json:"brand" gorm:"size:30"`
	Last4          string `json:"last4" gorm:"size:4"`
	ExpMonth       int    `json:"exp_month"`
	ExpYear        int    `json:"exp_year"`
	CardholderName string `json:"cardholder_name"`
	Nickname       string `json:"nickname"`
	IsDefault      bool   `json:"is_default" gorm:"default:false"`
}
