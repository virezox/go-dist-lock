package cache

import (
	"context"
	"testing"
	"time"
)

func TestLockClient_Release(t *testing.T) {
	c := &LockClient{}
	l, _ := c.Lock(context.Background(), "key1", time.Second*15)
  ch := l.AutoRefresh(time.Second, time.Second*10, time.Second*15)

  for {
  select {
  case <- ch:
    // rollback
  default:
    // do your work

  }
  }
  l.Release(context.Background()Background, expiration time.Duration)
  
}
