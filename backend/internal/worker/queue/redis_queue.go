package queue

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type RedisQueue struct {
	rdb       *redis.Client
	queueName string
}

func NewRedisQueue(rdb *redis.Client, queueName string) *RedisQueue {
	return &RedisQueue{rdb: rdb, queueName: queueName}
}

// Pop bloquea hasta que exista un elemento (BRPOP)
func (q *RedisQueue) Pop(ctx context.Context) (string, error) {
	res, err := q.rdb.BRPop(ctx, 0, q.queueName).Result()
	if err != nil {
		return "", err
	}
	if len(res) < 2 {
		return "", nil
	}
	return res[1], nil
}
