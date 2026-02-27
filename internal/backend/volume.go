package backend

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	"github.com/joyrex2001/kubedock/internal/config"
	"github.com/joyrex2001/kubedock/internal/model/types"
)

// CreateVolume will create a PersistentVolumeClaim in the kubernetes
// namespace for the given volume. On OCP 4.18, the storage class,
// volume size, and access mode are configurable to match the cluster's
// storage provisioner (e.g. gp3-csi, ocs-storagecluster-cephfs).
func (in *instance) CreateVolume(vol *types.Volume) error {
	labels := map[string]string{}
	for k, v := range config.SystemLabels {
		labels[k] = v
	}
	for k, v := range config.DefaultLabels {
		labels[k] = v
	}
	labels["kubedock.volumeid"] = vol.ShortID

	annotations := map[string]string{}
	for k, v := range config.DefaultAnnotations {
		annotations[k] = v
	}
	annotations["kubedock.volumename"] = vol.Name

	pvcName := in.getVolumePVCName(vol)

	accessMode := in.parseAccessMode(in.volumeAccessMode)

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        pvcName,
			Namespace:   in.namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{accessMode},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(in.volumeSize),
				},
			},
		},
	}

	// Set explicit storage class if configured (required on many OCP clusters)
	if in.storageClass != "" {
		pvc.Spec.StorageClassName = &in.storageClass
	}

	_, err := in.cli.CoreV1().PersistentVolumeClaims(in.namespace).Create(
		context.Background(), pvc, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create PVC for volume %s: %w", vol.Name, err)
	}

	vol.Mountpoint = "/var/lib/kubedock/volumes/" + vol.Name

	klog.Infof("created PVC %s for volume %s in namespace %s (storageClass=%s, size=%s, accessMode=%s)",
		pvcName, vol.Name, in.namespace, in.storageClass, in.volumeSize, accessMode)
	return nil
}

// parseAccessMode converts a string access mode to the k8s PersistentVolumeAccessMode.
// Supported: ReadWriteOnce (default), ReadWriteMany, ReadOnlyMany.
func (in *instance) parseAccessMode(mode string) corev1.PersistentVolumeAccessMode {
	switch mode {
	case "ReadWriteMany", "RWX":
		return corev1.ReadWriteMany
	case "ReadOnlyMany", "ROX":
		return corev1.ReadOnlyMany
	default:
		return corev1.ReadWriteOnce
	}
}

// DeleteVolume will delete the PersistentVolumeClaim associated with
// the given volume from the kubernetes namespace.
func (in *instance) DeleteVolume(vol *types.Volume) error {
	pvcName := in.getVolumePVCName(vol)
	err := in.cli.CoreV1().PersistentVolumeClaims(in.namespace).Delete(
		context.Background(), pvcName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete PVC for volume %s: %w", vol.Name, err)
	}
	klog.Infof("deleted PVC %s for volume %s", pvcName, vol.Name)
	return nil
}

// DeleteVolumes will delete all kubedock-owned PVCs in the namespace.
func (in *instance) DeleteVolumes(selector string) error {
	pvcs, err := in.cli.CoreV1().PersistentVolumeClaims(in.namespace).List(
		context.Background(), metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return err
	}
	for _, pvc := range pvcs.Items {
		klog.V(3).Infof("deleting PVC: %s", pvc.Name)
		if err := in.cli.CoreV1().PersistentVolumeClaims(pvc.Namespace).Delete(
			context.Background(), pvc.Name, metav1.DeleteOptions{}); err != nil {
			klog.Errorf("error deleting PVC %s: %s", pvc.Name, err)
		}
	}
	return nil
}

// getVolumePVCName returns a deterministic PVC name for the given volume.
func (in *instance) getVolumePVCName(vol *types.Volume) string {
	name := "kubedock-vol-" + in.toKubernetesName(vol.Name)
	if len(name) > 63 {
		name = name[:63]
	}
	return name
}

// addNamedVolumes adds PVC-backed volume mounts to the pod spec for
// containers that reference named (non-bind) volumes. This is called
// during startContainer to wire up compose named volumes.
func (in *instance) addNamedVolumes(tainr *types.Container, pod *corev1.Pod, namedVolumes map[string]*types.Volume) {
	for mountPath, vol := range namedVolumes {
		volName := "nv-" + in.toKubernetesName(vol.Name)
		pvcName := in.getVolumePVCName(vol)

		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: volName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		})

		mount := corev1.VolumeMount{
			Name:      volName,
			MountPath: mountPath,
		}
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, mount)
	}
}
