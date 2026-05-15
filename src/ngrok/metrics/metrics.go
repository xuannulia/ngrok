package metrics

import (
	"sync"
	"time"
)

type Counter interface {
	Inc(int64)
	Count() int64
}

type Gauge interface {
	Update(int64)
	Value() int64
}

type Meter interface {
	Mark(int64)
	Count() int64
	Rate1() float64
}

type Timer interface {
	Update(time.Duration)
	Time(func())
	Mean() float64
	Count() int64
}

type Histogram interface {
	Update(int64)
	Count() int64
	Mean() float64
}

type counter struct {
	mu    sync.Mutex
	count int64
}

func NewCounter() Counter { return &counter{} }

func (c *counter) Inc(delta int64) {
	c.mu.Lock()
	c.count += delta
	c.mu.Unlock()
}

func (c *counter) Count() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}

type gauge struct {
	mu    sync.Mutex
	value int64
}

func NewGauge() Gauge { return &gauge{} }

func (g *gauge) Update(value int64) {
	g.mu.Lock()
	g.value = value
	g.mu.Unlock()
}

func (g *gauge) Value() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.value
}

type meter struct {
	mu    sync.Mutex
	count int64
	start time.Time
}

func NewMeter() Meter {
	return &meter{start: time.Now()}
}

func (m *meter) Mark(delta int64) {
	m.mu.Lock()
	m.count += delta
	m.mu.Unlock()
}

func (m *meter) Count() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.count
}

func (m *meter) Rate1() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	elapsed := time.Since(m.start).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return float64(m.count) / elapsed
}

type timer struct {
	mu    sync.Mutex
	count int64
	total time.Duration
}

func NewTimer() Timer { return &timer{} }

func (t *timer) Update(duration time.Duration) {
	t.mu.Lock()
	t.count++
	t.total += duration
	t.mu.Unlock()
}

func (t *timer) Time(fn func()) {
	start := time.Now()
	defer func() { t.Update(time.Since(start)) }()
	fn()
}

func (t *timer) Mean() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.count == 0 {
		return 0
	}
	return float64(t.total) / float64(t.count)
}

func (t *timer) Count() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.count
}

type histogram struct {
	mu    sync.Mutex
	count int64
	total int64
}

func NewHistogram() Histogram { return &histogram{} }

func (h *histogram) Update(value int64) {
	h.mu.Lock()
	h.count++
	h.total += value
	h.mu.Unlock()
}

func (h *histogram) Count() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.count
}

func (h *histogram) Mean() float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.count == 0 {
		return 0
	}
	return float64(h.total) / float64(h.count)
}
