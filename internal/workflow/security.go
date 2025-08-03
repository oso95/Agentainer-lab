package workflow

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"golang.org/x/crypto/bcrypt"
)

// SecurityManager handles authentication and authorization
type SecurityManager struct {
	redisClient *redis.Client
	jwtSecret   []byte
}

// NewSecurityManager creates a new security manager
func NewSecurityManager(redisClient *redis.Client) (*SecurityManager, error) {
	// Generate JWT secret if not exists
	secret, err := getOrCreateJWTSecret(redisClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get JWT secret: %w", err)
	}

	return &SecurityManager{
		redisClient: redisClient,
		jwtSecret:   secret,
	}, nil
}

// Tenant represents an isolated namespace for workflows
type Tenant struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Status      TenantStatus      `json:"status"`
	Config      TenantConfig      `json:"config"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// TenantStatus represents the status of a tenant
type TenantStatus string

const (
	TenantStatusActive    TenantStatus = "active"
	TenantStatusSuspended TenantStatus = "suspended"
	TenantStatusDeleted   TenantStatus = "deleted"
)

// TenantConfig holds tenant-specific configuration
type TenantConfig struct {
	MaxWorkflows      int               `json:"max_workflows"`
	MaxAgents         int               `json:"max_agents"`
	MaxParallelSteps  int               `json:"max_parallel_steps"`
	ResourceQuotas    ResourceQuotas    `json:"resource_quotas"`
	AllowedImages     []string          `json:"allowed_images,omitempty"`
	ForbiddenImages   []string          `json:"forbidden_images,omitempty"`
	NetworkPolicies   []NetworkPolicy   `json:"network_policies,omitempty"`
	RetentionDays     int               `json:"retention_days"`
}

// ResourceQuotas defines resource limits for a tenant
type ResourceQuotas struct {
	MaxCPU        float64 `json:"max_cpu"`         // Total CPU cores
	MaxMemoryGB   float64 `json:"max_memory_gb"`   // Total memory in GB
	MaxStorageGB  float64 `json:"max_storage_gb"`  // Total storage in GB
	MaxBandwidthMB float64 `json:"max_bandwidth_mb"` // Network bandwidth in MB/s
}

// NetworkPolicy defines network access rules
type NetworkPolicy struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"` // "allow" or "deny"
	Endpoints   []string `json:"endpoints"`
	Ports       []int    `json:"ports,omitempty"`
	Protocols   []string `json:"protocols,omitempty"`
}

// User represents a system user
type User struct {
	ID          string            `json:"id"`
	Username    string            `json:"username"`
	Email       string            `json:"email"`
	TenantID    string            `json:"tenant_id"`
	Roles       []string          `json:"roles"`
	Permissions []string          `json:"permissions"`
	Status      UserStatus        `json:"status"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	LastLoginAt *time.Time        `json:"last_login_at,omitempty"`
}

// UserStatus represents the status of a user
type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusInactive UserStatus = "inactive"
	UserStatusLocked   UserStatus = "locked"
)

// Role represents a security role
type Role struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Permissions []string   `json:"permissions"`
	IsSystem    bool       `json:"is_system"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Permission represents a specific permission
type Permission string

const (
	// Workflow permissions
	PermissionWorkflowCreate  Permission = "workflow:create"
	PermissionWorkflowRead    Permission = "workflow:read"
	PermissionWorkflowUpdate  Permission = "workflow:update"
	PermissionWorkflowDelete  Permission = "workflow:delete"
	PermissionWorkflowExecute Permission = "workflow:execute"
	
	// Agent permissions
	PermissionAgentCreate     Permission = "agent:create"
	PermissionAgentRead       Permission = "agent:read"
	PermissionAgentUpdate     Permission = "agent:update"
	PermissionAgentDelete     Permission = "agent:delete"
	
	// Tenant permissions
	PermissionTenantAdmin     Permission = "tenant:admin"
	PermissionTenantRead      Permission = "tenant:read"
	
	// System permissions
	PermissionSystemAdmin     Permission = "system:admin"
)

// APIKey represents an API key for programmatic access
type APIKey struct {
	ID          string            `json:"id"`
	Key         string            `json:"key,omitempty"` // Only shown once
	Name        string            `json:"name"`
	UserID      string            `json:"user_id"`
	TenantID    string            `json:"tenant_id"`
	Permissions []string          `json:"permissions"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time        `json:"last_used_at,omitempty"`
	Status      APIKeyStatus      `json:"status"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
}

// APIKeyStatus represents the status of an API key
type APIKeyStatus string

const (
	APIKeyStatusActive   APIKeyStatus = "active"
	APIKeyStatusRevoked  APIKeyStatus = "revoked"
	APIKeyStatusExpired  APIKeyStatus = "expired"
)

// AuditLog represents a security audit log entry
type AuditLog struct {
	ID         string                 `json:"id"`
	Timestamp  time.Time              `json:"timestamp"`
	TenantID   string                 `json:"tenant_id"`
	UserID     string                 `json:"user_id"`
	Action     string                 `json:"action"`
	Resource   string                 `json:"resource"`
	ResourceID string                 `json:"resource_id"`
	Result     string                 `json:"result"` // "success" or "failure"
	Details    map[string]interface{} `json:"details,omitempty"`
	IPAddress  string                 `json:"ip_address,omitempty"`
	UserAgent  string                 `json:"user_agent,omitempty"`
}

// SecurityContext represents the security context for a request
type SecurityContext struct {
	UserID      string
	TenantID    string
	Roles       []string
	Permissions []string
	APIKeyID    string
	IPAddress   string
}

// CreateTenant creates a new tenant
func (sm *SecurityManager) CreateTenant(ctx context.Context, tenant *Tenant) error {
	if tenant.ID == "" {
		tenant.ID = generateID("tenant")
	}
	
	tenant.CreatedAt = time.Now()
	tenant.UpdatedAt = time.Now()
	
	data, err := json.Marshal(tenant)
	if err != nil {
		return fmt.Errorf("failed to marshal tenant: %w", err)
	}
	
	key := fmt.Sprintf("tenant:%s", tenant.ID)
	if err := sm.redisClient.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to save tenant: %w", err)
	}
	
	// Add to tenant index
	if err := sm.redisClient.SAdd(ctx, "tenants", tenant.ID).Err(); err != nil {
		return fmt.Errorf("failed to index tenant: %w", err)
	}
	
	// Create default roles for tenant
	if err := sm.createDefaultRoles(ctx, tenant.ID); err != nil {
		return fmt.Errorf("failed to create default roles: %w", err)
	}
	
	return nil
}

// GetTenant retrieves a tenant by ID
func (sm *SecurityManager) GetTenant(ctx context.Context, tenantID string) (*Tenant, error) {
	key := fmt.Sprintf("tenant:%s", tenantID)
	data, err := sm.redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("tenant not found")
		}
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}
	
	var tenant Tenant
	if err := json.Unmarshal([]byte(data), &tenant); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tenant: %w", err)
	}
	
	return &tenant, nil
}

// CreateUser creates a new user
func (sm *SecurityManager) CreateUser(ctx context.Context, user *User, password string) error {
	if user.ID == "" {
		user.ID = generateID("user")
	}
	
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	
	// Save user
	userData, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user: %w", err)
	}
	
	userKey := fmt.Sprintf("user:%s", user.ID)
	if err := sm.redisClient.Set(ctx, userKey, userData, 0).Err(); err != nil {
		return fmt.Errorf("failed to save user: %w", err)
	}
	
	// Save password
	passwordKey := fmt.Sprintf("user:%s:password", user.ID)
	if err := sm.redisClient.Set(ctx, passwordKey, hashedPassword, 0).Err(); err != nil {
		return fmt.Errorf("failed to save password: %w", err)
	}
	
	// Index by username
	usernameKey := fmt.Sprintf("username:%s", user.Username)
	if err := sm.redisClient.Set(ctx, usernameKey, user.ID, 0).Err(); err != nil {
		return fmt.Errorf("failed to index username: %w", err)
	}
	
	// Add to tenant users
	tenantUsersKey := fmt.Sprintf("tenant:%s:users", user.TenantID)
	if err := sm.redisClient.SAdd(ctx, tenantUsersKey, user.ID).Err(); err != nil {
		return fmt.Errorf("failed to add user to tenant: %w", err)
	}
	
	// Audit log
	sm.logAudit(ctx, &AuditLog{
		TenantID:   user.TenantID,
		UserID:     user.ID,
		Action:     "user.create",
		Resource:   "user",
		ResourceID: user.ID,
		Result:     "success",
	})
	
	return nil
}

// AuthenticateUser authenticates a user with username and password
func (sm *SecurityManager) AuthenticateUser(ctx context.Context, username, password string) (*User, error) {
	// Get user ID by username
	usernameKey := fmt.Sprintf("username:%s", username)
	userID, err := sm.redisClient.Get(ctx, usernameKey).Result()
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	
	// Get user
	user, err := sm.GetUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	
	// Check user status
	if user.Status != UserStatusActive {
		return nil, fmt.Errorf("user account is %s", user.Status)
	}
	
	// Verify password
	passwordKey := fmt.Sprintf("user:%s:password", user.ID)
	hashedPassword, err := sm.redisClient.Get(ctx, passwordKey).Result()
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	
	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	
	// Update last login
	now := time.Now()
	user.LastLoginAt = &now
	sm.UpdateUser(ctx, user)
	
	// Audit log
	sm.logAudit(ctx, &AuditLog{
		TenantID:   user.TenantID,
		UserID:     user.ID,
		Action:     "user.login",
		Resource:   "user",
		ResourceID: user.ID,
		Result:     "success",
	})
	
	return user, nil
}

// GetUser retrieves a user by ID
func (sm *SecurityManager) GetUser(ctx context.Context, userID string) (*User, error) {
	key := fmt.Sprintf("user:%s", userID)
	data, err := sm.redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	
	var user User
	if err := json.Unmarshal([]byte(data), &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user: %w", err)
	}
	
	return &user, nil
}

// UpdateUser updates a user
func (sm *SecurityManager) UpdateUser(ctx context.Context, user *User) error {
	user.UpdatedAt = time.Now()
	
	data, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user: %w", err)
	}
	
	key := fmt.Sprintf("user:%s", user.ID)
	return sm.redisClient.Set(ctx, key, data, 0).Err()
}

// CreateAPIKey creates a new API key
func (sm *SecurityManager) CreateAPIKey(ctx context.Context, apiKey *APIKey) (string, error) {
	if apiKey.ID == "" {
		apiKey.ID = generateID("key")
	}
	
	// Generate key
	key := generateAPIKey()
	apiKey.Key = "" // Don't store the actual key
	apiKey.CreatedAt = time.Now()
	apiKey.Status = APIKeyStatusActive
	
	// Hash the key for storage
	hashedKey, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash API key: %w", err)
	}
	
	// Save API key
	data, err := json.Marshal(apiKey)
	if err != nil {
		return "", fmt.Errorf("failed to marshal API key: %w", err)
	}
	
	keyID := fmt.Sprintf("apikey:%s", apiKey.ID)
	if err := sm.redisClient.Set(ctx, keyID, data, 0).Err(); err != nil {
		return "", fmt.Errorf("failed to save API key: %w", err)
	}
	
	// Save hashed key for verification
	hashKey := fmt.Sprintf("apikey:%s:hash", apiKey.ID)
	if err := sm.redisClient.Set(ctx, hashKey, hashedKey, 0).Err(); err != nil {
		return "", fmt.Errorf("failed to save API key hash: %w", err)
	}
	
	// Index by prefix for lookup
	prefix := key[:12] // Use first 12 chars as prefix
	prefixKey := fmt.Sprintf("apikey:prefix:%s", prefix)
	if err := sm.redisClient.Set(ctx, prefixKey, apiKey.ID, 0).Err(); err != nil {
		return "", fmt.Errorf("failed to index API key: %w", err)
	}
	
	// Add to user's API keys
	userKeysKey := fmt.Sprintf("user:%s:apikeys", apiKey.UserID)
	if err := sm.redisClient.SAdd(ctx, userKeysKey, apiKey.ID).Err(); err != nil {
		return "", fmt.Errorf("failed to add API key to user: %w", err)
	}
	
	return key, nil
}

// ValidateAPIKey validates an API key and returns the associated context
func (sm *SecurityManager) ValidateAPIKey(ctx context.Context, key string) (*SecurityContext, error) {
	if len(key) < 12 {
		return nil, fmt.Errorf("invalid API key")
	}
	
	// Look up by prefix
	prefix := key[:12]
	prefixKey := fmt.Sprintf("apikey:prefix:%s", prefix)
	apiKeyID, err := sm.redisClient.Get(ctx, prefixKey).Result()
	if err != nil {
		return nil, fmt.Errorf("invalid API key")
	}
	
	// Get API key
	keyID := fmt.Sprintf("apikey:%s", apiKeyID)
	data, err := sm.redisClient.Get(ctx, keyID).Result()
	if err != nil {
		return nil, fmt.Errorf("invalid API key")
	}
	
	var apiKey APIKey
	if err := json.Unmarshal([]byte(data), &apiKey); err != nil {
		return nil, fmt.Errorf("invalid API key")
	}
	
	// Check status
	if apiKey.Status != APIKeyStatusActive {
		return nil, fmt.Errorf("API key is %s", apiKey.Status)
	}
	
	// Check expiration
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("API key expired")
	}
	
	// Verify key
	hashKey := fmt.Sprintf("apikey:%s:hash", apiKeyID)
	hashedKey, err := sm.redisClient.Get(ctx, hashKey).Result()
	if err != nil {
		return nil, fmt.Errorf("invalid API key")
	}
	
	if err := bcrypt.CompareHashAndPassword([]byte(hashedKey), []byte(key)); err != nil {
		return nil, fmt.Errorf("invalid API key")
	}
	
	// Update last used
	now := time.Now()
	apiKey.LastUsedAt = &now
	sm.updateAPIKey(ctx, &apiKey)
	
	// Get user
	user, err := sm.GetUser(ctx, apiKey.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}
	
	return &SecurityContext{
		UserID:      user.ID,
		TenantID:    user.TenantID,
		Roles:       user.Roles,
		Permissions: apiKey.Permissions,
		APIKeyID:    apiKey.ID,
	}, nil
}

// CheckPermission checks if a security context has a specific permission
func (sm *SecurityManager) CheckPermission(ctx context.Context, secCtx *SecurityContext, permission Permission) bool {
	// System admin has all permissions
	for _, perm := range secCtx.Permissions {
		if perm == string(PermissionSystemAdmin) {
			return true
		}
	}
	
	// Check specific permission
	for _, perm := range secCtx.Permissions {
		if perm == string(permission) {
			return true
		}
		// Check wildcard permissions
		if strings.HasSuffix(perm, ":*") {
			prefix := strings.TrimSuffix(perm, "*")
			if strings.HasPrefix(string(permission), prefix) {
				return true
			}
		}
	}
	
	return false
}

// EnforceTenantIsolation ensures resources are properly isolated by tenant
func (sm *SecurityManager) EnforceTenantIsolation(ctx context.Context, secCtx *SecurityContext, resourceTenantID string) error {
	// System admin can access any tenant
	if sm.CheckPermission(ctx, secCtx, PermissionSystemAdmin) {
		return nil
	}
	
	// Otherwise, must match tenant
	if secCtx.TenantID != resourceTenantID {
		return fmt.Errorf("access denied: resource belongs to different tenant")
	}
	
	return nil
}

// createDefaultRoles creates default roles for a tenant
func (sm *SecurityManager) createDefaultRoles(ctx context.Context, tenantID string) error {
	roles := []Role{
		{
			ID:          generateID("role"),
			Name:        "admin",
			Description: "Tenant administrator",
			Permissions: []string{
				string(PermissionWorkflowCreate),
				string(PermissionWorkflowRead),
				string(PermissionWorkflowUpdate),
				string(PermissionWorkflowDelete),
				string(PermissionWorkflowExecute),
				string(PermissionAgentCreate),
				string(PermissionAgentRead),
				string(PermissionAgentUpdate),
				string(PermissionAgentDelete),
				string(PermissionTenantAdmin),
			},
			IsSystem: true,
		},
		{
			ID:          generateID("role"),
			Name:        "developer",
			Description: "Workflow developer",
			Permissions: []string{
				string(PermissionWorkflowCreate),
				string(PermissionWorkflowRead),
				string(PermissionWorkflowUpdate),
				string(PermissionWorkflowExecute),
				string(PermissionAgentRead),
			},
			IsSystem: true,
		},
		{
			ID:          generateID("role"),
			Name:        "viewer",
			Description: "Read-only access",
			Permissions: []string{
				string(PermissionWorkflowRead),
				string(PermissionAgentRead),
				string(PermissionTenantRead),
			},
			IsSystem: true,
		},
	}
	
	for _, role := range roles {
		role.CreatedAt = time.Now()
		role.UpdatedAt = time.Now()
		
		data, err := json.Marshal(role)
		if err != nil {
			return err
		}
		
		key := fmt.Sprintf("tenant:%s:role:%s", tenantID, role.ID)
		if err := sm.redisClient.Set(ctx, key, data, 0).Err(); err != nil {
			return err
		}
		
		// Index by name
		nameKey := fmt.Sprintf("tenant:%s:role:name:%s", tenantID, role.Name)
		if err := sm.redisClient.Set(ctx, nameKey, role.ID, 0).Err(); err != nil {
			return err
		}
	}
	
	return nil
}

// updateAPIKey updates an API key
func (sm *SecurityManager) updateAPIKey(ctx context.Context, apiKey *APIKey) error {
	data, err := json.Marshal(apiKey)
	if err != nil {
		return err
	}
	
	key := fmt.Sprintf("apikey:%s", apiKey.ID)
	return sm.redisClient.Set(ctx, key, data, 0).Err()
}

// logAudit logs an audit event
func (sm *SecurityManager) logAudit(ctx context.Context, log *AuditLog) {
	log.ID = generateID("audit")
	log.Timestamp = time.Now()
	
	data, err := json.Marshal(log)
	if err != nil {
		return
	}
	
	// Store in time-series format
	key := fmt.Sprintf("audit:%s:%d", log.TenantID, log.Timestamp.Unix())
	sm.redisClient.Set(ctx, key, data, 30*24*time.Hour) // Keep for 30 days
	
	// Add to tenant audit index
	indexKey := fmt.Sprintf("tenant:%s:audit", log.TenantID)
	sm.redisClient.ZAdd(ctx, indexKey, &redis.Z{
		Score:  float64(log.Timestamp.Unix()),
		Member: log.ID,
	})
}

// Helper functions

func generateID(prefix string) string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, base64.RawURLEncoding.EncodeToString(b))
}

func generateAPIKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func getOrCreateJWTSecret(redisClient *redis.Client) ([]byte, error) {
	ctx := context.Background()
	key := "security:jwt:secret"
	
	secret, err := redisClient.Get(ctx, key).Result()
	if err == nil {
		return []byte(secret), nil
	}
	
	if err != redis.Nil {
		return nil, err
	}
	
	// Generate new secret
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	
	secretStr := base64.StdEncoding.EncodeToString(b)
	if err := redisClient.Set(ctx, key, secretStr, 0).Err(); err != nil {
		return nil, err
	}
	
	return []byte(secretStr), nil
}