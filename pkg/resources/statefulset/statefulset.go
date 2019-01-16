package statefulset

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/huangjiasingle/etcd-operator/pkg/apis/app/v1alpha1"
)

func New(etcd *v1alpha1.Etcd) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Statefulset",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      etcd.Name,
			Namespace: etcd.Namespace,
			Labels:    map[string]string{"app.example.com/v1alpha1": etcd.Name},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(etcd, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "Etcd",
				}),
			},
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: etcd.Name,
			Replicas:    etcd.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app.example.com/v1alpha1": etcd.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:   etcd.Name,
					Labels: map[string]string{"app.example.com/v1alpha1": etcd.Name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						corev1.Container{
							Name:      "etcd",
							Image:     etcd.Spec.Image,
							Resources: etcd.Spec.Resources,
							Ports: []corev1.ContainerPort{
								corev1.ContainerPort{Name: "peer", ContainerPort: 2379},
								corev1.ContainerPort{Name: "client", ContainerPort: 2380},
							},
							Env: []corev1.EnvVar{
								corev1.EnvVar{Name: "INITIAL_CLUSTER_SIZE", Value: fmt.Sprintf("%v", *etcd.Spec.Replicas)},
								corev1.EnvVar{Name: "SET_NAME", Value: etcd.Name},
								corev1.EnvVar{Name: "NAMESPACE", ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
								}},
							},
							Command: []string{
								"/bin/sh",
								"-ec",
								"HOSTNAME=$(hostname)\n\n# store member id into PVC for later member replacement\ncollect_member() {\n    while ! etcdctl member list \u0026\u003e/dev/null; do sleep 1; done\n    etcdctl member list | grep http://${HOSTNAME}.${SET_NAME}:2380 | cut -d':' -f1 | cut -d'[' -f1 \u003e /var/run/etcd/member_id\n    exit 0\n}\n\neps() {\n    EPS=\"\"\n    for i in $(seq 0 $((${INITIAL_CLUSTER_SIZE} - 1))); do\n        EPS=\"${EPS}${EPS:+,}http://${SET_NAME}-${i}.${SET_NAME}:2379\"\n    done\n    echo ${EPS}\n}\n\nmember_hash() {\n    etcdctl member list | grep http://${HOSTNAME}.${SET_NAME}:2380 | cut -d':' -f1 | cut -d'[' -f1\n}\n\n# re-joining after failure?\nif [ -e /var/run/etcd/default.etcd ]; then\n    echo \"Re-joining etcd member\"\n    member_id=$(cat /var/run/etcd/member_id)\n\n    # re-join member\n    POD_IP=$(hostname -i)\n    ETCDCTL_ENDPOINT=$(eps) etcdctl member update ${member_id} http://${HOSTNAME}.${SET_NAME}:2380\n    exec etcd --name ${HOSTNAME} \\\n        --listen-peer-urls http://${POD_IP}:2380 \\\n        --listen-client-urls http://${POD_IP}:2379,http://127.0.0.1:2379 \\\n        --advertise-client-urls http://${POD_IP}:2379 \\\n        --data-dir /var/run/etcd/default.etcd\nfi\n\n# etcd-SET_ID\n\necho ${HOSTNAME:5:${#HOSTNAME}} \"09090\"\nSET_ID=${HOSTNAME:5:${#HOSTNAME}}\n\n# adding a new member to existing cluster (assuming all initial pods are available)\nif [ \"${SET_ID}\" -ge ${INITIAL_CLUSTER_SIZE} ]; then\n    export ETCDCTL_ENDPOINT=$(eps)\n\n    # member already added?\n    MEMBER_HASH=$(member_hash)\n\n    echo \"00000001212\"$MEMBER_HASH\n\n    if [ -n \"${MEMBER_HASH}\" ]; then\n        # the member hash exists but for some reason etcd failed\n        # as the datadir has not be created, we can remove the member\n        # and retrieve new hash\n        etcdctl member remove ${MEMBER_HASH}\n    fi\n\n    echo \"Adding new member\"\n    etcdctl member add ${HOSTNAME} http://${HOSTNAME}.${SET_NAME}:2380 | grep \"^ETCD_\" \u003e /var/run/etcd/new_member_envs\n\n    if [ $? -ne 0 ]; then\n        echo\"Exiting\"\n        rm -f /var/run/etcd/new_member_envs\n        exit 1\n    fi\n\n    cat /var/run/etcd/new_member_envs\n    source /var/run/etcd/new_member_envs\n\n    collect_member \u0026\n\n    POD_IP=$(hostname -i)\n    exec etcd --name ${HOSTNAME} \\\n        --listen-peer-urls http://${POD_IP}:2380 \\\n        --listen-client-urls http://${POD_IP}:2379,http://127.0.0.1:2379 \\\n        --advertise-client-urls http://${POD_IP}:2379 \\\n        --data-dir /var/run/etcd/default.etcd \\\n        --initial-advertise-peer-urls http://${HOSTNAME}.${SET_NAME}:2380 \\\n        --initial-cluster ${ETCD_INITIAL_CLUSTER} \\\n        --initial-cluster-state ${ETCD_INITIAL_CLUSTER_STATE}\nfi\n\nfor i in $(seq 0 $((${INITIAL_CLUSTER_SIZE} - 1))); do\n    while true; do\n        echo \"Waiting for ${SET_NAME}-${i}.${SET_NAME} to come up\"\n        ping -W 1 -c 1 ${SET_NAME}-${i}.${SET_NAME}.${NAMESPACE}.svc.cluster.local \u003e /dev/null \u0026\u0026 break\n        sleep 1s\n    done\ndone\n\nPEERS=\"\"\nfor i in $(seq 0 $((${INITIAL_CLUSTER_SIZE} - 1))); do\n    PEERS=\"${PEERS}${PEERS:+,}${SET_NAME}-${i}=http://${SET_NAME}-${i}.${SET_NAME}:2380\"\ndone\n\ncollect_member \u0026\n\necho ${PEERS}\n# join member\nPOD_IP=$(hostname -i)\nexec etcd --name ${HOSTNAME} \\\n    --initial-advertise-peer-urls http://${POD_IP}:2380 \\\n    --listen-peer-urls http://${POD_IP}:2380 \\\n    --listen-client-urls http://${POD_IP}:2379,http://127.0.0.1:2379 \\\n    --advertise-client-urls http://${POD_IP}:2379 \\\n    --initial-cluster-token etcd-cluster-1 \\\n    --initial-cluster ${PEERS} \\\n    --initial-cluster-state new \\\n    --data-dir /var/run/etcd/default.etcd\n",
							},
							Lifecycle: &corev1.Lifecycle{
								PreStop: &corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"/bin/sh",
											"-ec",
											"EPS=\"\"\nfor i in $(seq 0 $((${INITIAL_CLUSTER_SIZE} - 1))); do\n    EPS=\"${EPS}${EPS:+,}http://${SET_NAME}-${i}.${SET_NAME}:2379\"\ndone\n\nHOSTNAME=$(hostname)\n\nmember_hash() {\n    etcdctl member list | grep http://${HOSTNAME}.${SET_NAME}:2380 | cut -d':' -f1 | cut -d'[' -f1\n}\n\n# Remove everything otherwise the cluster will no longer scale-up\nSET_ID=${HOSTNAME:5:${#HOSTNAME}}\n# adding a new member to existing cluster (assuming all initial pods are available)\nif [ \"${SET_ID}\" -ge ${INITIAL_CLUSTER_SIZE} ]; then\n  echo \"Removing ${HOSTNAME} from etcd cluster\"\n  ETCDCTL_ENDPOINT=${EPS} etcdctl member remove $(member_hash)\n  if [ $? -eq 0 ]; then\n    rm -rf /var/run/etcd/*\n  fi\nfi\n",
										},
									},
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						corev1.Volume{
							Name: "datadir",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
			// local test ,use emptyDir to replace it
			// VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
			// 	corev1.PersistentVolumeClaim{
			// 		ObjectMeta: metav1.ObjectMeta{
			// 			Name: "datadir",
			// 		},
			// 		Spec: corev1.PersistentVolumeClaimSpec{
			// 			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			// 			Resources: corev1.ResourceRequirements{
			// 				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(fmt.Sprintf("%vGi", etcd.Spec.Storage))},
			// 			},
			// 		},
			// 	},
			// },
		},
	}
}
