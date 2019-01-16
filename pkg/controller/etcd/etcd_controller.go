package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"k8s.io/client-go/util/retry"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	appv1alpha1 "github.com/huangjiasingle/etcd-operator/pkg/apis/app/v1alpha1"
	"github.com/huangjiasingle/etcd-operator/pkg/resources/service"
	"github.com/huangjiasingle/etcd-operator/pkg/resources/statefulset"
)

var log = logf.Log.WithName("controller_etcd")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Etcd Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileEtcd{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	c, err := controller.New("etcd-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &appv1alpha1.Etcd{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appv1alpha1.Etcd{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileEtcd{}

// ReconcileEtcd reconciles a Etcd object
type ReconcileEtcd struct {
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Etcd object and makes changes based on the state read
// and what is in the Etcd.Spec
func (r *ReconcileEtcd) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Etcd instance
	etcd := &appv1alpha1.Etcd{}
	err := r.client.Get(context.TODO(), request.NamespacedName, etcd)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if etcd.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}

	// Check if this etcd association resource already exists
	found := &appsv1.StatefulSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: etcd.Name, Namespace: etcd.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		//create association resources
		headlessService := service.New(etcd)
		err = r.client.Create(context.TODO(), headlessService)
		if err != nil {
			return reconcile.Result{}, err
		}

		ss := statefulset.New(etcd)
		err = r.client.Create(context.TODO(), ss)
		if err != nil {
			go r.client.Delete(context.TODO(), headlessService)
			return reconcile.Result{}, err
		}

		etcd.Annotations = map[string]string{"app.example.com/spec": toString(etcd)}
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			return r.client.Update(context.TODO(), etcd)
		})
		if retryErr != nil {
			fmt.Println(retryErr.Error())
		}
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	if !reflect.DeepEqual(etcd.Spec, toSpec(etcd.Annotations["app.example.com/spec"])) {
		//update association resources
		ss := statefulset.New(etcd)
		found.Spec = ss.Spec
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			return r.client.Update(context.TODO(), ss)
		})
		if retryErr != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func toString(etcd *appv1alpha1.Etcd) string {
	data, _ := json.Marshal(etcd.Spec)
	return string(data)
}

func toSpec(data string) appv1alpha1.EtcdSpec {
	spec := appv1alpha1.EtcdSpec{}
	json.Unmarshal([]byte(data), &spec)
	return spec
}
