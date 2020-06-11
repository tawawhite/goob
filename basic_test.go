package goob_test

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/ysmood/goob"
)

type null struct{}

func ExampleNew() {
	ob := goob.New()

	ctx, unsubscribe := context.WithCancel(context.Background())
	defer unsubscribe()
	s := ob.Subscribe(ctx)

	go func() {
		ob.Publish(1)
		ob.Publish(2)
		ob.Publish(3)
	}()

	for e := range s {
		fmt.Println(e)

		if e.(int) == 3 {
			break
		}
	}

	// Output:
	// 1
	// 2
	// 3
}

func TestNew(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ob := goob.New()
	s := ob.Subscribe(ctx)
	size := 10000

	expected := []int{}
	go func() {
		for i := range make([]null, size) {
			expected = append(expected, i)
			ob.Publish(i)
		}
	}()

	result := []int{}
	for msg := range s {
		result = append(result, msg.(int))
		if len(result) == size {
			cancel()
		}
	}

	assert.Equal(t, expected, result)
}

func TestUnsubscribe(t *testing.T) {
	ob := goob.New()

	ctx, cancel := context.WithCancel(context.Background())
	ob.Subscribe(ctx)
	cancel()

	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, runtime.NumGoroutine(), 2)
	assert.Equal(t, ob.Count(), 0)
}

func TestMultipleConsumers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ob := goob.New()
	s1 := ob.Subscribe(ctx)
	s2 := ob.Subscribe(ctx)
	s3 := ob.Subscribe(ctx)
	size := 10000

	expected := []int{}
	go func() {
		for i := range make([]null, size) {
			expected = append(expected, i)
			time.Sleep(time.Duration(rand.Intn(100)) * time.Nanosecond)
			ob.Publish(i)
		}
	}()

	wg := sync.WaitGroup{}
	wg.Add(2)

	r1 := []int{}
	go func() {
		for e := range s1 {
			r1 = append(r1, e.(int))
			if len(r1) == size {
				wg.Done()
			}
		}
	}()

	r2 := []int{}
	go func() {
		for e := range s2 {
			r2 = append(r2, e.(int))
			if len(r2) == size {
				wg.Done()
			}
		}
	}()

	go func() {
		<-s3 // simulate slow consumer
	}()

	wg.Wait()

	assert.Equal(t, expected, r1)
	assert.Equal(t, expected, r2)
}

func TestEach(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ob := goob.New()
	s := ob.Subscribe(ctx)
	size := 100

	expected := []int{}
	go func() {
		for i := range make([]null, size) {
			expected = append(expected, i)
			ob.Publish(i)
		}
	}()

	result := []int{}
	goob.Each(s, func(e int) bool {
		result = append(result, e)
		return len(result) == size
	})

	assert.Equal(t, expected, result)
}

func TestMap(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ob := goob.New()
	s := ob.Map(ctx, func(e int) int {
		return e * 2
	}).Subscribe(ctx)

	go func() {
		ob.Publish(1)
		ob.Publish(2)
		ob.Publish(3)
	}()

	result := []int{}
	goob.Each(s, func(e int) bool {
		result = append(result, e)
		return len(result) == 3
	})

	assert.Equal(t, []int{2, 4, 6}, result)
}

func TestFilter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ob := goob.New()
	s := ob.Filter(ctx, func(e int) bool {
		return e%2 == 0
	}).Subscribe(ctx)

	go func() {
		ob.Publish(1)
		ob.Publish(2)
		ob.Publish(3)
		ob.Publish(4)
	}()

	result := []int{}
	goob.Each(s, func(e int) bool {
		result = append(result, e)
		return len(result) == 2
	})

	assert.Equal(t, []int{2, 4}, result)
}

func TestMonkey(t *testing.T) {
	wg := sync.WaitGroup{}
	count := int32(0)
	roundSize := 10000
	size := 100

	run := func() {
		wg.Add(1)
		defer wg.Done()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ob := goob.New()
		s := ob.Subscribe(ctx)

		go func() {
			for range make([]null, size) {
				time.Sleep(time.Duration(rand.Intn(100)) * time.Nanosecond)
				ob.Publish(nil)
			}
		}()

		wait := make(chan null)
		go func() {
			for i := range make([]null, size) {
				time.Sleep(time.Duration(rand.Intn(100)) * time.Nanosecond)

				<-s

				atomic.AddInt32(&count, 1)

				if i == size-1 {
					wait <- null{}
				}
			}
		}()
		<-wait
	}

	for range make([]null, roundSize) {
		go run()
	}

	wg.Wait()

	assert.EqualValues(t, roundSize*size, count)
}

func BenchmarkPublish(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ob := goob.New()
	s := ob.Subscribe(ctx)

	go func() {
		for range s {
		}
	}()

	for i := 0; i < b.N; i++ {
		ob.Publish(nil)
	}
}

func BenchmarkConsume(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ob := goob.New()
	s := ob.Subscribe(ctx)

	for i := 0; i < b.N; i++ {
		ob.Publish(nil)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		<-s
	}
}
