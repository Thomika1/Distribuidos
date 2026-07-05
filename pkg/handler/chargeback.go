package handler

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Thomika1/Distribuidos/pkg/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ChargebackHandler struct {
	db *gorm.DB
}

func NewChargebackHandler(db *gorm.DB) *ChargebackHandler {
	return &ChargebackHandler{db: db}
}

func (h *ChargebackHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":     "ok",
		"middleware": "none",
	})
}

func (h *ChargebackHandler) Create(c *gin.Context) {
	var input models.ChargebackInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input", "detail": err.Error()})
		return
	}

	transactionDate, err := time.Parse(time.RFC3339, input.TransactionDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "transaction_date must be in RFC3339 format"})
		return
	}

	decision := models.DetermineDecision(input.CardBrand, input.ReasonCode)

	chargeback := models.Chargeback{
		TransactionDate:  transactionDate,
		TransactionValue: input.TransactionValue,
		CardBrand:       input.CardBrand,
		ReasonCode:      input.ReasonCode,
		Decision:        decision,
	}

	processingDelay := getProcessingDelay()
	if processingDelay > 0 {
		time.Sleep(processingDelay)
	}

	if err := h.db.Create(&chargeback).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create chargeback", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "chargeback created",
		"data":    chargeback,
	})
}

func (h *ChargebackHandler) Get(c *gin.Context) {
	id := c.Param("id")
	var chargeback models.Chargeback

	if err := h.db.First(&chargeback, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "chargeback not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": chargeback})
}

func (h *ChargebackHandler) List(c *gin.Context) {
	var chargebacks []models.Chargeback

	if err := h.db.Find(&chargebacks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list chargebacks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": chargebacks, "count": len(chargebacks)})
}

func (h *ChargebackHandler) Validate(c *gin.Context) {
	var input models.ChargebackInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input", "detail": err.Error()})
		return
	}

	isFraud := models.IsFraud(input.CardBrand, input.ReasonCode)
	decision := models.DetermineDecision(input.CardBrand, input.ReasonCode)

	c.JSON(http.StatusOK, gin.H{
		"is_fraud":    isFraud,
		"reason_code": input.ReasonCode,
		"card_brand":  input.CardBrand,
		"decision":    decision,
		"message":     getValidationMessage(isFraud, decision),
	})
}

func getValidationMessage(isFraud bool, decision string) string {
	if isFraud {
		return "fraud detected: chargeback dispute rejected - reason code indicates high acquirer win probability"
	}
	return "no fraud indicator: no automatic decision applied"
}

func getProcessingDelay() time.Duration {
	if v := os.Getenv("PROCESSING_DELAY_MS"); v != "" {
		if ms, err := strconv.Atoi(v); err == nil {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return 0
}