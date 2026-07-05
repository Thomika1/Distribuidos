package models

import (
	"strings"

	"github.com/shopspring/decimal"
)

type ChargebackInput struct {
	TransactionDate  string          `json:"transaction_date" binding:"required"`
	TransactionValue decimal.Decimal `json:"transaction_value" binding:"required"`
	CardBrand        string          `json:"card_brand" binding:"required"`
	ReasonCode       string          `json:"reason_code" binding:"required"`
}

func IsFraud(cardBrand, reasonCode string) bool {
	brand := strings.ToLower(strings.TrimSpace(cardBrand))
	code := strings.TrimSpace(reasonCode)

	switch brand {
	case "visa":
		return strings.HasPrefix(code, "10.")
	case "mastercard":
		return strings.HasPrefix(code, "48")
	default:
		return false
	}
}

func DetermineDecision(cardBrand, reasonCode string) string {
	if IsFraud(cardBrand, reasonCode) {
		return "reject"
	}
	return ""
}