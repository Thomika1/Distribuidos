package middleware

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Thomika1/Distribuidos/pkg/lock"
	"github.com/gin-gonic/gin"
)

var LockManager = lock.NewLockManager()

func ConcurrencyMiddleware() gin.HandlerFunc {
	timeout := getLockTimeout()

	return func(c *gin.Context) {
		txID := generateTxID(c)
		tx := LockManager.Begin(txID)

		resource := resourceFromPath(c)

		var lockType lock.LockType
		var lockTypeStr string
		if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut || c.Request.Method == http.MethodDelete {
			lockType = lock.Exclusive
			lockTypeStr = "exclusive"
		} else {
			lockType = lock.Shared
			lockTypeStr = "shared"
		}

		waitStart := time.Now()
		err := LockManager.Lock(tx, resource, lockType, timeout)
		waitDuration := time.Since(waitStart)

		if err != nil {
			LockManager.Abort(tx)
			c.Header("X-Lock-Acquired", "false")
			c.Header("X-Lock-Wait-Ms", formatMs(waitDuration))
			c.Header("X-Lock-Resource", resource)
			c.Header("X-Lock-Type", lockTypeStr)
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"error":          "concurrency conflict",
				"detail":         err.Error(),
				"resource":       resource,
				"lock_type":      lockTypeStr,
				"wait_ms":        waitDuration.Milliseconds(),
			})
			return
		}

		c.Header("X-Lock-Acquired", "true")
		c.Header("X-Lock-Wait-Ms", formatMs(waitDuration))
		c.Header("X-Lock-Resource", resource)
		c.Header("X-Lock-Type", lockTypeStr)
		c.Header("X-Lock-TxID", txID)

		c.Set("tx", tx)
		c.Set("resource", resource)

		c.Next()

		if t, exists := c.Get("tx"); exists {
			if transaction, ok := t.(*lock.Transaction); ok {
				LockManager.Commit(transaction)
			}
		}
	}
}

func getLockTimeout() time.Duration {
	if v := os.Getenv("LOCK_TIMEOUT_MS"); v != "" {
		if ms, err := time.ParseDuration(v + "ms"); err == nil {
			return ms
		}
	}
	return 5 * time.Second
}

func formatMs(d time.Duration) string {
	return strconv.FormatInt(d.Milliseconds(), 10)
}

func resourceFromPath(c *gin.Context) string {
	path := c.FullPath()
	if path == "" {
		path = c.Request.URL.Path
	}
	if id := c.Param("id"); id != "" {
		return "chargeback:" + id
	}
	return strings.TrimSuffix(path, "/")
}

func generateTxID(c *gin.Context) string {
	return strings.ReplaceAll(time.Now().Format("20060102150405.000000")+c.ClientIP(), ":", "-")
}