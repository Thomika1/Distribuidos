package models

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type Chargeback struct {
	ID              uint            `gorm:"primaryKey" json:"id"`
	TransactionDate time.Time       `gorm:"not null" json:"transaction_date" time_format:"rfc3339"`
	TransactionValue decimal.Decimal `gorm:"type:decimal(20,2);not null" json:"transaction_value"`
	CardBrand       string          `gorm:"size:50;not null" json:"card_brand"`
	ReasonCode      string          `gorm:"size:20;not null" json:"reason_code"`
	Decision        string          `gorm:"size:20" json:"decision"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

func (Chargeback) TableName() string {
	return "chargebacks"
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&Chargeback{})
}