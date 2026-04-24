package deployment

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNextMemoryAfterOOM(t *testing.T) {
	tests := []struct {
		name       string
		current    string
		baseline   string
		ceiling    string
		multiplier float64
		want       string
		capped     bool
	}{
		{
			name:       "straight 2x bump well under ceiling",
			current:    "256Mi",
			baseline:   "256Mi",
			ceiling:    "4Gi",
			multiplier: 2.0,
			want:       "512Mi",
		},
		{
			name:       "empty current falls back to baseline",
			current:    "",
			baseline:   "128Mi",
			ceiling:    "4Gi",
			multiplier: 2.0,
			want:       "256Mi",
		},
		{
			name:       "one bump clamps to ceiling instead of overshooting",
			current:    "3Gi",
			baseline:   "256Mi",
			ceiling:    "4Gi",
			multiplier: 2.0,
			want:       "4Gi",
		},
		{
			name:       "at ceiling means capped",
			current:    "4Gi",
			baseline:   "256Mi",
			ceiling:    "4Gi",
			multiplier: 2.0,
			capped:     true,
		},
		{
			name:       "over ceiling means capped",
			current:    "8Gi",
			baseline:   "256Mi",
			ceiling:    "4Gi",
			multiplier: 2.0,
			capped:     true,
		},
		{
			name:       "1.5x multiplier is honored",
			current:    "200Mi",
			baseline:   "200Mi",
			ceiling:    "4Gi",
			multiplier: 1.5,
			want:       "300Mi",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, capped, err := nextMemoryAfterOOM(tc.current, tc.baseline, tc.ceiling, tc.multiplier)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if capped != tc.capped {
				t.Fatalf("capped = %v, want %v", capped, tc.capped)
			}
			if tc.capped {
				return
			}
			gotQ, err := resource.ParseQuantity(got)
			if err != nil {
				t.Fatalf("returned quantity %q doesn't parse: %v", got, err)
			}
			wantQ, err := resource.ParseQuantity(tc.want)
			if err != nil {
				t.Fatalf("want %q doesn't parse: %v", tc.want, err)
			}
			if gotQ.Cmp(wantQ) != 0 {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestOOMFromContainerStatus(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "target-pod"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "mcp",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
				},
			}},
		},
	}

	t.Run("last termination OOMKilled", func(t *testing.T) {
		cs := &corev1.ContainerStatus{
			Name: "mcp",
			LastTerminationState: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{
					Reason:   "OOMKilled",
					ExitCode: 137,
				},
			},
		}
		info, ok := oomFromContainerStatus(pod, cs)
		if !ok {
			t.Fatal("expected OOM to be detected")
		}
		if info.MemoryLimit != "256Mi" {
			t.Fatalf("MemoryLimit = %q, want 256Mi", info.MemoryLimit)
		}
		if info.PodName != "target-pod" {
			t.Fatalf("PodName = %q", info.PodName)
		}
	})

	t.Run("exit code 137 without explicit reason still counts", func(t *testing.T) {
		cs := &corev1.ContainerStatus{
			Name: "mcp",
			LastTerminationState: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 137,
				},
			},
		}
		if _, ok := oomFromContainerStatus(pod, cs); !ok {
			t.Fatal("expected exit code 137 to be treated as OOM")
		}
	})

	t.Run("unrelated termination reason is not an OOM", func(t *testing.T) {
		cs := &corev1.ContainerStatus{
			Name: "mcp",
			LastTerminationState: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{
					Reason:   "Error",
					ExitCode: 1,
				},
			},
		}
		if _, ok := oomFromContainerStatus(pod, cs); ok {
			t.Fatal("non-OOM termination should not trigger auto-scale")
		}
	})

	t.Run("current state terminated also counts", func(t *testing.T) {
		cs := &corev1.ContainerStatus{
			Name: "mcp",
			State: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{
					Reason:   "OOMKilled",
					ExitCode: 137,
				},
			},
		}
		if _, ok := oomFromContainerStatus(pod, cs); !ok {
			t.Fatal("current state OOM should be detected")
		}
	})
}

func TestOOMAutoscaleConfig_WithDefaults(t *testing.T) {
	got := (OOMAutoscaleConfig{}).withDefaults()
	if got.Multiplier != 2.0 {
		t.Fatalf("Multiplier = %v, want 2.0", got.Multiplier)
	}
	if got.MaxAttempts != 4 {
		t.Fatalf("MaxAttempts = %d, want 4", got.MaxAttempts)
	}
	if got.MaxMemory != "4Gi" {
		t.Fatalf("MaxMemory = %q, want 4Gi", got.MaxMemory)
	}
	if got.PollInterval == 0 {
		t.Fatal("PollInterval default should be set")
	}
}
