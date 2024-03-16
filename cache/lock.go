package cache

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	luaRefresh = redis.NewScript(
		`if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("pexpire", KEY[1], ARGV[2]) else return 0 end`,
	)
	luaRelease = redis.NewScript(
		`if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEY[1]) else return 0 end`,
	)

	ErrFailedToRefresh = errors.New("refresh failed")
	ErrFailedToRelease = errors.New("release failed")
)

type LockClient struct {
	client *redis.Client
}

func (c *LockClient) Lock(
	ctx context.Context,
	key string,
	expiration time.Duration,
) (*Lock, error) {
	token := uuid.New().String()
	ok, err := c.client.SetNX(ctx, key, token, expiration).Result()
	if err != nil {
		return nil, err
	}
	if ok {
		return &Lock{
			token:  token,
			key:    key,
			client: c.client,
		}, nil
	}
	return nil, errors.New("can't acquire the lock")
}

func (l *Lock) Release(
	ctx context.Context,
	expiration time.Duration,
) error {
	// val, err := l.client.Get(ctx, key).Result()
	// if err != nil {
	// 	return err
	// }
	//
	// if val == l.token {
	// 	_, err = l.client.Del(ctx, key).Result()
	// }
	defer func() {
		l.close <- struct{}{}
	}()

	status, err := luaRelease.Run(ctx, l.client, []string{l.key}, l.token).Result()
	if err != nil {
		return err
	}
	if status == int64(1) {
		return nil
	}
	return ErrFailedToRelease
}

type Lock struct {
	client *redis.Client
	close  chan struct{}
	token  string
	key    string
}

// AutoRefresh, interval -> the period for AutoRefresh, newExpire -> the new expire time
func (l *Lock) AutoRefresh(
	timeout time.Duration,
	interval time.Duration,
	newExpire time.Duration,
) <-chan error {
	ch := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(interval)
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				status, err := luaRefresh.Run(ctx, l.client, []string{l.key}, l.token, newExpire.Microseconds()).
					Result()
				cancel()
				if err != nil {
					// Retry here
					ch <- err
					close(ch)
					return
				}
				if status != int64(1) {
					ch <- ErrFailedToRefresh
					close(ch)
				}
			case <-l.close:
				close(ch)
				close(l.close)
				return
			}
		}
	}()
	return ch
}

// type Lock struct {
// 	client *redis.Client
// 	expire time.Duration
// }
//
// func NewLockClient(c *redis.Client) (*LockClient, error) {
// 	return &LockClient{
// 		client: c,
// 	}, nil
// }
