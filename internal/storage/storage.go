package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

type Storage struct {
	redisClient *redis.Client
}

func NewStorage(redisClient *redis.Client) *Storage {
	return &Storage{
		redisClient: redisClient,
	}
}

func (s *Storage) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return s.redisClient.Set(ctx, key, value, expiration).Err()
}

func (s *Storage) Get(ctx context.Context, key string) (string, error) {
	return s.redisClient.Get(ctx, key).Result()
}

func (s *Storage) Delete(ctx context.Context, key string) error {
	return s.redisClient.Del(ctx, key).Err()
}

func (s *Storage) Exists(ctx context.Context, key string) (bool, error) {
	result, err := s.redisClient.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

func (s *Storage) Keys(ctx context.Context, pattern string) ([]string, error) {
	return s.redisClient.Keys(ctx, pattern).Result()
}

func (s *Storage) SetAgentStatus(ctx context.Context, agentID, status string) error {
	key := fmt.Sprintf("agent:%s:status", agentID)
	return s.Set(ctx, key, status, 0)
}

func (s *Storage) GetAgentStatus(ctx context.Context, agentID string) (string, error) {
	key := fmt.Sprintf("agent:%s:status", agentID)
	return s.Get(ctx, key)
}

func (s *Storage) SetAgentMetrics(ctx context.Context, agentID string, metrics map[string]interface{}) error {
	key := fmt.Sprintf("agent:%s:metrics", agentID)
	return s.redisClient.HMSet(ctx, key, metrics).Err()
}

func (s *Storage) GetAgentMetrics(ctx context.Context, agentID string) (map[string]string, error) {
	key := fmt.Sprintf("agent:%s:metrics", agentID)
	return s.redisClient.HGetAll(ctx, key).Result()
}

func (s *Storage) IncrementCounter(ctx context.Context, key string) error {
	return s.redisClient.Incr(ctx, key).Err()
}

func (s *Storage) GetCounter(ctx context.Context, key string) (int64, error) {
	return s.redisClient.Get(ctx, key).Int64()
}