package deployment

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// oomProbe walks pods matching the workload's labels and reports whether
// any recent container termination was an OOMKill. It populates the OOM
// block of a StatusReport: whether OOM was observed, which pod, which
// exit code, and the memory limit the pod was running with at the time
// of death.
//
// Rules:
//
//   - Scans ContainerStatus.LastTerminationState for all containers in
//     every pod with the workload label selector. LastTerminationState
//     persists across CrashLoopBackoff restarts, so a pod that OOMed then
//     was quietly restarted by K8s still shows up here — which is exactly
//     what the auto-scaler wants to react to.
//   - Also checks the current State in case a pod is mid-termination.
//   - Uses the first OOMKilled container it finds. If multiple pods OOM
//     (scaled workload), the caller's reconcile loop still fires once;
//     subsequent OOMs land on a later tick.
//   - Returns ok=false if no OOM is found. All other error conditions
//     (list failure, decode issue) surface as a non-nil error — the
//     caller treats that the same as "no OOM" and moves on rather than
//     blocking Status().
func oomProbe(ctx context.Context, client kubernetes.Interface, namespace, workloadName string) (OOMInfo, error) {
	selector := fmt.Sprintf("app.kubernetes.io/name=%s", workloadName)
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return OOMInfo{}, fmt.Errorf("list pods: %w", err)
	}
	for i := range pods.Items {
		pod := &pods.Items[i]
		for _, cs := range pod.Status.ContainerStatuses {
			if info, ok := oomFromContainerStatus(pod, &cs); ok {
				return info, nil
			}
		}
	}
	return OOMInfo{}, nil
}

// OOMInfo is the structured result of oomProbe. Empty OK=false.
type OOMInfo struct {
	OK           bool
	PodName      string
	ContainerID  string
	ExitCode     int32
	MemoryLimit  string // limit active on the pod at the time of death
}

// oomFromContainerStatus returns true iff the container either recently
// terminated OR is currently terminating because of OOMKill. "OOMKilled"
// is the authoritative reason string in K8s; exit code 137 is a
// fallback for runtimes that set the code but not the reason.
func oomFromContainerStatus(pod *corev1.Pod, cs *corev1.ContainerStatus) (OOMInfo, bool) {
	check := func(term *corev1.ContainerStateTerminated) (OOMInfo, bool) {
		if term == nil {
			return OOMInfo{}, false
		}
		if term.Reason != "OOMKilled" && term.ExitCode != 137 {
			return OOMInfo{}, false
		}
		limit := containerMemoryLimit(pod, cs.Name)
		return OOMInfo{
			OK:          true,
			PodName:     pod.Name,
			ContainerID: cs.ContainerID,
			ExitCode:    term.ExitCode,
			MemoryLimit: limit,
		}, true
	}
	if info, ok := check(cs.LastTerminationState.Terminated); ok {
		return info, true
	}
	if info, ok := check(cs.State.Terminated); ok {
		return info, true
	}
	return OOMInfo{}, false
}

// containerMemoryLimit returns the memory limit string (e.g. "512Mi")
// that was set on the named container in the pod spec, or "" if none.
func containerMemoryLimit(pod *corev1.Pod, containerName string) string {
	for _, c := range pod.Spec.Containers {
		if c.Name != containerName {
			continue
		}
		if q, ok := c.Resources.Limits[corev1.ResourceMemory]; ok {
			return q.String()
		}
	}
	return ""
}
