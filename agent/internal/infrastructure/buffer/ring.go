package buffer

import (
	"sync"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/ports"
)

// RingBuffer implementa ports.BufferPort de forma concurrente con descarte FIFO.
type RingBuffer struct {
	mu       sync.RWMutex
	capacity int
	items    []domain.HostTelemetry
	head     int
	tail     int
	size     int
}

// NewRingBuffer crea un nuevo buffer circular con la capacidad especificada.
func NewRingBuffer(capacity int) ports.BufferPort {
	if capacity <= 0 {
		capacity = 1000
	}
	return &RingBuffer{
		capacity: capacity,
		items:    make([]domain.HostTelemetry, capacity),
	}
}

// Push inserta un lote de telemetría. Si el buffer está lleno, sobreescribe el más antiguo (FIFO).
func (r *RingBuffer) Push(telemetry domain.HostTelemetry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.items[r.tail] = telemetry
	r.tail = (r.tail + 1) % r.capacity

	if r.size < r.capacity {
		r.size++
	} else {
		// Buffer lleno: avanzamos la cabeza para descartar el elemento más antiguo
		r.head = (r.head + 1) % r.capacity
	}
	return nil
}

// Pop extrae el lote más antiguo (FIFO).
func (r *RingBuffer) Pop() (domain.HostTelemetry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.size == 0 {
		return domain.HostTelemetry{}, false
	}

	item := r.items[r.head]
	r.head = (r.head + 1) % r.capacity
	r.size--
	return item, true
}

// Drain extrae todos los elementos acumulados manteniendo el orden cronológico.
func (r *RingBuffer) Drain() []domain.HostTelemetry {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.size == 0 {
		return nil
	}

	result := make([]domain.HostTelemetry, r.size)
	for i := 0; i < r.size; i++ {
		idx := (r.head + i) % r.capacity
		result[i] = r.items[idx]
	}

	r.head = 0
	r.tail = 0
	r.size = 0
	return result
}

// Len retorna el número actual de elementos encolados.
func (r *RingBuffer) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.size
}
