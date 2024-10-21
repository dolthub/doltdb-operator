package builder

import (
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	corev1 "k8s.io/api/core/v1"
)

const (
	DoltContainerName = "dolt"

	DoltMySQLPort     = 3306
	DoltRemoteAPIPort = 50051

	DoltDataMountPath   = "/db"
	DoltConfigMountPath = "/etc/doltdb"
)

func doltVolumeMounts(doltcluster *doltv1alpha.DoltCluster) []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      doltcluster.Name,
			MountPath: DoltDataMountPath,
		},
		{
			Name:      doltcluster.Name,
			MountPath: DoltConfigMountPath,
		},
	}
}

func doltContainerCommand() []string {
	return []string{
		"/usr/local/bin/dolt",
		"sql-server",
		"--config",
		"config.yaml",
		"--data-dir",
		".",
	}
}

func doltEnv(doltcluster *doltv1alpha.DoltCluster) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "DOLT_ROOT_PATH",
			Value: "/db",
		},
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "DOLT_USERNAME",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: doltcluster.UserSecretKeyRef(),
			},
		},
		{
			Name: "DOLT_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: doltcluster.PasswordSecretKeyRef(),
			},
		},
	}
}

func doltContainers(doltcluster *doltv1alpha.DoltCluster) []corev1.Container {
	containers := []corev1.Container{
		{
			Name:            DoltContainerName,
			Image:           fmt.Sprintf("%s:%s", doltcluster.Spec.Image, doltcluster.Spec.EngineVersion),
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         doltContainerCommand(),
			WorkingDir:      DoltDataMountPath,
			Env:             doltEnv(doltcluster),
			Ports: []corev1.ContainerPort{
				{
					ContainerPort: DoltMySQLPort,
					Name:          "mysql",
				},
				{
					ContainerPort: DoltRemoteAPIPort,
					Name:          "grpc",
				},
			},
			VolumeMounts: doltVolumeMounts(doltcluster),
		},
	}

	return containers
}
