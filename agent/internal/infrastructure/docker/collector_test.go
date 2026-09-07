package docker_test

import (
	"testing"

	"github.com/alvaroriverac/server_tracker_agent/internal/infrastructure/docker"
	dockertypes "github.com/docker/docker/api/types/container"
)

func TestCalculateCPUPercent(t *testing.T) {
	tests := []struct {
		name        string
		stats       *dockertypes.StatsResponse
		expected    float64
		expectDelta bool
	}{
		{
			name: "Muestreo válido con 2 CPUs al 50%",
			stats: &dockertypes.StatsResponse{
				CPUStats: dockertypes.CPUStats{
					CPUUsage: dockertypes.CPUUsage{
						TotalUsage: 200000000,
					},
					SystemUsage: 200000000,
					OnlineCPUs:  2,
				},
				PreCPUStats: dockertypes.CPUStats{
					CPUUsage: dockertypes.CPUUsage{
						TotalUsage: 100000000,
					},
					SystemUsage: 100000000,
				},
			},
			expected: 200.0, // (100M / 100M) * 2 * 100 = 200%
		},
		{
			name: "Delta de sistema <= 0 (descarte sin pánico / NaN)",
			stats: &dockertypes.StatsResponse{
				CPUStats: dockertypes.CPUStats{
					CPUUsage: dockertypes.CPUUsage{
						TotalUsage: 150000000,
					},
					SystemUsage: 100000000,
					OnlineCPUs:  2,
				},
				PreCPUStats: dockertypes.CPUStats{
					CPUUsage: dockertypes.CPUUsage{
						TotalUsage: 100000000,
					},
					SystemUsage: 100000000, // System delta = 0
				},
			},
			expected: 0.0,
		},
		{
			name: "Reinicio de contenedor con CPU acumulado menor (delta negativo)",
			stats: &dockertypes.StatsResponse{
				CPUStats: dockertypes.CPUStats{
					CPUUsage: dockertypes.CPUUsage{
						TotalUsage: 10000, // Se reinició el contador a un valor menor
					},
					SystemUsage: 200000000,
					OnlineCPUs:  1,
				},
				PreCPUStats: dockertypes.CPUStats{
					CPUUsage: dockertypes.CPUUsage{
						TotalUsage: 99999999,
					},
					SystemUsage: 100000000,
				},
			},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := docker.CalculateCPUPercent(tt.stats)
			if got != tt.expected {
				t.Errorf("CalculateCPUPercent() = %v, esperado %v", got, tt.expected)
			}
		})
	}
}

func TestCalculateRealRAM(t *testing.T) {
	stats := &dockertypes.StatsResponse{
		MemoryStats: dockertypes.MemoryStats{
			Usage: 200 * 1024 * 1024, // 200 MB
			Stats: map[string]uint64{
				"inactive_file": 80 * 1024 * 1024, // 80 MB
			},
			Limit: 1024 * 1024 * 1024, // 1 GB
		},
	}

	realRAM, limit := docker.CalculateRealRAM(stats)
	expectedRAM := uint64(120 * 1024 * 1024)

	if realRAM != expectedRAM {
		t.Errorf("CalculateRealRAM() real = %v, esperado %v", realRAM, expectedRAM)
	}
	if limit != 1024*1024*1024 {
		t.Errorf("CalculateRealRAM() limit = %v, esperado 1GB", limit)
	}
}
