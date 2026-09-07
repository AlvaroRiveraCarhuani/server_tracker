package buffer_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/infrastructure/buffer"
)

func TestRingBuffer_PushAndPop(t *testing.T) {
	rb := buffer.NewRingBuffer(3)

	if rb.Len() != 0 {
		t.Fatalf("esperado Len 0, obtenido %d", rb.Len())
	}

	item1 := domain.HostTelemetry{HostID: "h1", Timestamp: 100}
	item2 := domain.HostTelemetry{HostID: "h1", Timestamp: 200}

	_ = rb.Push(item1)
	_ = rb.Push(item2)

	if rb.Len() != 2 {
		t.Fatalf("esperado Len 2, obtenido %d", rb.Len())
	}

	popped, ok := rb.Pop()
	if !ok || popped.Timestamp != 100 {
		t.Fatalf("esperado timestamp 100 en orden FIFO, obtenido %+v", popped)
	}

	if rb.Len() != 1 {
		t.Fatalf("esperado Len 1 tras Pop, obtenido %d", rb.Len())
	}
}

func TestRingBuffer_FIFO_Overflow(t *testing.T) {
	capacity := 3
	rb := buffer.NewRingBuffer(capacity)

	// Llenamos el buffer con 3 elementos (10, 20, 30)
	for i := 1; i <= 3; i++ {
		_ = rb.Push(domain.HostTelemetry{HostID: "h1", Timestamp: int64(i * 10)})
	}

	if rb.Len() != 3 {
		t.Fatalf("esperado buffer lleno con 3 elementos, obtenido %d", rb.Len())
	}

	// Insertamos un 4to elemento (40) -> debe descartar el más viejo (10)
	_ = rb.Push(domain.HostTelemetry{HostID: "h1", Timestamp: 40})

	if rb.Len() != 3 {
		t.Fatalf("capacidad no debe exceder 3, obtenido %d", rb.Len())
	}

	// El primer elemento al desapilar debe ser 20 (el 10 fue descartado)
	first, ok := rb.Pop()
	if !ok || first.Timestamp != 20 {
		t.Fatalf("esperado timestamp 20 tras descarte FIFO del 10, obtenido %+v", first)
	}

	second, ok := rb.Pop()
	if !ok || second.Timestamp != 30 {
		t.Fatalf("esperado timestamp 30, obtenido %+v", second)
	}

	third, ok := rb.Pop()
	if !ok || third.Timestamp != 40 {
		t.Fatalf("esperado timestamp 40, obtenido %+v", third)
	}

	_, ok = rb.Pop()
	if ok {
		t.Fatalf("buffer debería estar vacío")
	}
}

func TestRingBuffer_Drain(t *testing.T) {
	rb := buffer.NewRingBuffer(5)

	for i := 1; i <= 3; i++ {
		_ = rb.Push(domain.HostTelemetry{HostID: "h1", Timestamp: int64(i)})
	}

	items := rb.Drain()
	if len(items) != 3 {
		t.Fatalf("esperado 3 elementos en Drain, obtenido %d", len(items))
	}

	if rb.Len() != 0 {
		t.Fatalf("buffer debe quedar vacío tras Drain, obtenido %d", rb.Len())
	}

	if items[0].Timestamp != 1 || items[2].Timestamp != 3 {
		t.Fatalf("Drain debe preservar orden cronológico FIFO")
	}
}

func TestRingBuffer_ThreadSafety(t *testing.T) {
	rb := buffer.NewRingBuffer(50)

	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			_ = rb.Push(domain.HostTelemetry{HostID: fmt.Sprintf("h-%d", i), Timestamp: time.Now().UnixNano()})
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			rb.Pop()
		}
		done <- true
	}()

	<-done
	<-done
}
