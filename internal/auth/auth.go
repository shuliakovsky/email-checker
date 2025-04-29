package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
	"github.com/shuliakovsky/email-checker/internal/lock"
	"github.com/shuliakovsky/email-checker/internal/logger"
)

// KeyType represents different types of API keys
type KeyType string

const (
	KeyTypePayAsYouGo KeyType = "pay_as_you_go" // Pay-per-use API key type
	KeyTypeMonthly    KeyType = "monthly"       // Monthly subscription API key type
	cacheTTL                  = 5 * time.Minute // TTL for Redis cache entries
)

// APIKey contains authentication details and usage metrics
type APIKey struct {
	Key           string    // Secret API key value
	Type          KeyType   // Type of key (pay_as_you_go/monthly)
	UsedChecks    int       // Number of checks consumed
	Remaining     int       // Remaining available checks
	ExpiresAt     time.Time // Key expiration timestamp
	InitialChecks int       // Original check quota when created
}

// AuthService handles API key authentication and quota management
type AuthService struct {
	db          *sqlx.DB              // PostgreSQL database connection
	redis       redis.UniversalClient // Redis client for caching/locking
	clusterMode bool                  // Flag for distributed system operation
}

// NewAuthService creates a new authentication service instance
func NewAuthService(db *sqlx.DB, redis redis.UniversalClient, clusterMode bool) *AuthService {
	return &AuthService{
		db:          db,
		redis:       redis,
		clusterMode: clusterMode,
	}
}

// ValidateKey checks API key validity and returns key details
func (s *AuthService) ValidateKey(ctx context.Context, apiKey string) (*APIKey, error) {
	// Check Redis cache first
	cachedKey, err := s.getFromCache(ctx, apiKey)
	if err == nil && cachedKey != nil {
		return cachedKey, nil
	}

	// Cache miss - query database
	key, err := s.getFromDB(ctx, apiKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("invalid api key")
		}
		return nil, fmt.Errorf("database error: %v", err)
	}

	// Validate key state
	if time.Now().After(key.ExpiresAt) {
		return nil, fmt.Errorf("api key expired")
	}
	if key.Remaining <= 0 {
		return nil, fmt.Errorf("quota exhausted")
	}

	// Update cache with fresh data
	if err := s.cacheKey(ctx, key); err != nil {
		logger.Log(fmt.Sprintf("Failed to cache key: %v", err))
	}

	return key, nil
}

// DecrementQuota reduces available checks count using appropriate concurrency control
func (s *AuthService) DecrementQuota(ctx context.Context, apiKey string, count int) error {
	if s.clusterMode {
		return s.decrementWithLock(ctx, apiKey, count) // Distributed lock for clusters
	}
	return s.decrementInTransaction(ctx, apiKey, count) // Local transaction for single instance
}

// decrementInTransaction updates quota using database transaction
func (s *AuthService) decrementInTransaction(ctx context.Context, apiKey string, count int) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Atomic update with returning new remaining value
	var newRemaining int
	err = tx.QueryRowContext(ctx, `
        UPDATE api_keys 
        SET used_checks = used_checks + $1,
            remaining_checks = remaining_checks - $1
        WHERE api_key = $2
        RETURNING remaining_checks`,
		count, apiKey,
	).Scan(&newRemaining)

	if err != nil {
		return fmt.Errorf("update failed: %v", err)
	}

	if newRemaining < 0 {
		return fmt.Errorf("quota exceeded")
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %v", err)
	}

	// Refresh cache with updated values
	key, err := s.getFromDB(ctx, apiKey)
	if err == nil {
		s.cacheKey(ctx, key)
	}

	return nil
}

// getFromCache retrieves API key details from Redis
func (s *AuthService) getFromCache(ctx context.Context, key string) (*APIKey, error) {
	data, err := s.redis.HGetAll(ctx, "apikey:"+key).Result()
	if err != nil || len(data) == 0 {
		return nil, err
	}

	expiresAt, _ := time.Parse(time.RFC3339, data["expires_at"])
	return &APIKey{
		Key:           key,
		Type:          KeyType(data["type"]),
		UsedChecks:    parseInt(data["used_checks"]),
		Remaining:     parseInt(data["remaining"]),
		ExpiresAt:     expiresAt,
		InitialChecks: parseInt(data["initial_checks"]),
	}, nil
}

// cacheKey stores API key details in Redis with TTL
func (s *AuthService) cacheKey(ctx context.Context, key *APIKey) error {
	fields := map[string]interface{}{
		"type":           key.Type,
		"used_checks":    key.UsedChecks,
		"remaining":      key.Remaining,
		"expires_at":     key.ExpiresAt.Format(time.RFC3339),
		"initial_checks": key.InitialChecks,
	}
	return s.redis.HSet(ctx, "apikey:"+key.Key, fields).Err()
}

// getFromDB retrieves API key details from PostgreSQL
func (s *AuthService) getFromDB(ctx context.Context, apiKey string) (*APIKey, error) {
	var key struct {
		Key           string    `db:"api_key"`
		Type          string    `db:"key_type"`
		UsedChecks    int       `db:"used_checks"`
		Remaining     int       `db:"remaining_checks"`
		ExpiresAt     time.Time `db:"expires_at"`
		InitialChecks int       `db:"initial_checks"`
	}

	err := s.db.GetContext(ctx, &key, `
		SELECT api_key, key_type, used_checks, remaining_checks, expires_at, initial_checks
		FROM api_keys
		WHERE api_key = $1`, apiKey)

	if err != nil {
		return nil, err
	}

	return &APIKey{
		Key:           key.Key,
		Type:          KeyType(key.Type),
		UsedChecks:    key.UsedChecks,
		Remaining:     key.Remaining,
		ExpiresAt:     key.ExpiresAt,
		InitialChecks: key.InitialChecks,
	}, nil
}

// decrementWithLock uses distributed lock and atomic Redis operations
func (s *AuthService) decrementWithLock(ctx context.Context, apiKey string, count int) error {
	lockKey := "lock:apikey:" + apiKey
	lock := lock.NewLock(s.redis, lockKey, 10*time.Second, true)

	if !lock.Acquire(ctx) {
		return fmt.Errorf("failed to acquire lock")
	}
	defer lock.Release(ctx)

	// Lua script for atomic check-and-update in Redis
	script := `
        local key = KEYS[1]
        local count = tonumber(ARGV[1])
        local remaining = tonumber(redis.call('HGET', key, 'remaining'))
        
        if not remaining or remaining < count then
            return {err='not enough quota'}
        end
        
        redis.call('HINCRBY', key, 'used_checks', count)
        redis.call('HINCRBY', key, 'remaining', -count)
        redis.call('EXPIRE', key, ARGV[2])
        return redis.call('HGETALL', key)
    `

	_, err := s.redis.Eval(ctx, script, []string{"apikey:" + apiKey}, count, cacheTTL.Seconds()).Result()
	if err != nil {
		return err
	}

	// Synchronize with PostgreSQL database
	_, err = s.db.ExecContext(ctx, `
        UPDATE api_keys 
        SET used_checks = used_checks + $1,
            remaining_checks = remaining_checks - $1
        WHERE api_key = $2`,
		count, apiKey,
	)

	return err
}

// parseInt converts string to integer with error suppression
func parseInt(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}
