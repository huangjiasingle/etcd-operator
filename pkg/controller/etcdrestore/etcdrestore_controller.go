package etcdrestore

import (
	"context"

	appv1alpha1 "github.com/huangjiasingle/etcd-operator/pkg/apis/app/v1alpha1"
	"github.com/huangjiasingle/etcd-operator/pkg/resources/statefulset"
	"github.com/huangjiasingle/etcd-operator/pkg/tools/log"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Add creates a new EtcdRestore Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileEtcdRestore{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("etcdrestore-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource EtcdRestore
	err = c.Watch(&source.Kind{Type: &appv1alpha1.EtcdRestore{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner EtcdRestore
	err = c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appv1alpha1.EtcdRestore{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileEtcdRestore{}

// ReconcileEtcdRestore reconciles a EtcdRestore object
type ReconcileEtcdRestore struct {
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a EtcdRestore object and makes changes based on the state read
// and what is in the EtcdRestore.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileEtcdRestore) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Info("Reconciling EtcdRestore %v", request.String())
	// Fetch the EtcdRestore instance
	restore := &appv1alpha1.EtcdRestore{}
	err := r.client.Get(context.TODO(), request.NamespacedName, restore)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if restore.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}

	// before do restore operate
	condition := appv1alpha1.EtcdRestoreCondition{Ready: true, LastedTranslationTime: metav1.Now(), Reason: "begin etcd cluster restore", Message: err.Error()}
	if err := r.updateStatus(restore, condition, appv1alpha1.EtcdRestoreRunning); err != nil {
		log.Error(err)
		return reconcile.Result{}, err
	}

	// restoring
	ss := &appsv1.StatefulSet{}
	if err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: restore.Name, Name: restore.Spec.ClusterReference}, ss); err != nil {
		condition := appv1alpha1.EtcdRestoreCondition{Ready: false, LastedTranslationTime: metav1.Now(), Reason: "get reference statefulset failed", Message: err.Error()}
		if err := r.updateStatus(restore, condition, appv1alpha1.EtcdRestoreFailed); err != nil {
			log.Error(err)
			return reconcile.Result{}, err
		}
	}

	pods, err := r.listPod(ss)
	if err != nil {
		condition := appv1alpha1.EtcdRestoreCondition{Ready: false, LastedTranslationTime: metav1.Now(), Reason: "get reference pod failed", Message: err.Error()}
		if udpateErr := r.updateStatus(restore, condition, appv1alpha1.EtcdRestoreFailed); udpateErr != nil {
			log.Error(udpateErr)
			return reconcile.Result{}, udpateErr
		}
	}

	for _, pod := range pods {
		for _, v := range pod.Spec.Volumes {
			if v.VolumeSource.PersistentVolumeClaim != nil {
				if err := r.client.Delete(context.TODO(), &corev1.PersistentVolumeClaim{
					TypeMeta: metav1.TypeMeta{
						Kind:       "PersistentVolumeClaim",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      v.VolumeSource.PersistentVolumeClaim.ClaimName,
						Namespace: ss.Namespace,
					},
				}); err != nil {
					condition := appv1alpha1.EtcdRestoreCondition{Ready: false, LastedTranslationTime: metav1.Now(), Reason: "delete reference pvc failed", Message: err.Error()}
					if udpateErr := r.updateStatus(restore, condition, appv1alpha1.EtcdRestoreFailed); udpateErr != nil {
						log.Error(udpateErr)
						return reconcile.Result{}, udpateErr
					}
					return reconcile.Result{}, err
				}
			}
		}
	}
	ss.Spec.Template.Spec.InitContainers = statefulset.NewEtcdClusterInitContainers(ss, restore)

	if err := r.client.Update(context.TODO(), ss); err != nil {
		condition := appv1alpha1.EtcdRestoreCondition{Ready: false, LastedTranslationTime: metav1.Now(), Reason: "delete reference pvc failed", Message: err.Error()}
		if udpateErr := r.updateStatus(restore, condition, appv1alpha1.EtcdRestoreFailed); udpateErr != nil {
			log.Error(udpateErr)
			return reconcile.Result{}, udpateErr
		}
		return reconcile.Result{}, err
	}

	// store suuccess
	condition = appv1alpha1.EtcdRestoreCondition{Ready: true, LastedTranslationTime: metav1.Now(), Reason: "delete reference pvc failed", Message: err.Error()}
	if udpateErr := r.updateStatus(restore, condition, appv1alpha1.EtcdRestoreComplated); udpateErr != nil {
		log.Error(udpateErr)
		return reconcile.Result{}, udpateErr
	}
	return reconcile.Result{}, nil
}

func (r *ReconcileEtcdRestore) updateStatus(restore *appv1alpha1.EtcdRestore, condition appv1alpha1.EtcdRestoreCondition, phase appv1alpha1.EtcdRestorePhase) error {
	restore.Status.Phase = phase
	restore.Status.Conditions = append(restore.Status.Conditions, condition)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.client.Status().Update(context.TODO(), restore)
	})
}

func (r *ReconcileEtcdRestore) listPod(ss *appsv1.StatefulSet) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	if err := r.client.List(context.TODO(), &client.ListOptions{Namespace: ss.Namespace, LabelSelector: labels.SelectorFromSet(labels.Set(ss.Spec.Selector.MatchLabels))}, podList); err != nil {
		return nil, err
	}
	return podList.Items, nil
}
