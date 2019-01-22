package etcddump

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"reflect"
	"time"

	cron "gopkg.in/robfig/cron.v2"

	"k8s.io/client-go/util/retry"

	appv1alpha1 "github.com/huangjiasingle/etcd-operator/pkg/apis/app/v1alpha1"
	"github.com/huangjiasingle/etcd-operator/pkg/tools/log"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	dumpCron                = cron.New()
	DefaultDumpFileTemplate = "root/%v_%v_%v.db"
	location                = "storageUrl/fileName"

	LocalFileDir = "/Users/apple/workspace/go/src/github.com/huangjiasingle/etcd-operator/etcd.db"
)

func init() {
	dumpCron.Start()
}

// Add creates a new EtcdDump Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileEtcdDump{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("etcddump-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource EtcdDump
	err = c.Watch(&source.Kind{Type: &appv1alpha1.EtcdDump{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileEtcdDump{}

// ReconcileEtcdDump reconciles a EtcdDump object
type ReconcileEtcdDump struct {
	// This client, ini    tialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a EtcdDump object and makes changes based on the state read
// and what is in the EtcdDump.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileEtcdDump) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Infof("reconciling EtcdDump %v", request.String())
	// Fetch the EtcdDump instance
	dump := &appv1alpha1.EtcdDump{}
	err := r.client.Get(context.TODO(), request.NamespacedName, dump)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if dump.DeletionTimestamp != nil {
		// remove cron dump
		return reconcile.Result{}, nil
	}

	if !reflect.DeepEqual(dump.Spec, toSpec(dump.Annotations["etcddump.app.example.com/spec"])) {
		if err := r.ProcessDumpItem(dump); err != nil {
			return reconcile.Result{}, err
		}

		if dump.Annotations != nil {
			dump.Annotations["etcddump.app.example.com/spec"] = toString(dump.Spec)
		} else {
			dump.Annotations = map[string]string{"etcddump.app.example.com/spec": toString(dump.Spec)}
		}
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			return r.client.Update(context.TODO(), dump)
		})

		if retryErr != nil {
			log.Errorf("when dump etcd success, update etcddump %v err: %v", request.String(), err)
		}
		return reconcile.Result{}, nil
	}
	log.Infof("but EtcdDump spec no change, so it's ignore")
	return reconcile.Result{}, nil
}

func (r *ReconcileEtcdDump) ProcessDumpItem(dump *appv1alpha1.EtcdDump) error {
	if dump.Spec.Scheduler != "" {
		dumpCron.AddFunc(dump.Spec.Scheduler, func() {
			if err := r.CreateManulDump(dump); err != nil {
				log.Errorf("cron dump etcd cluster %v/%v err: %v", dump.Namespace, dump.Spec.ClusterReference, err)
				return
			}
			log.Infof("cron dump etcd cluster %v/%v success", dump.Namespace, dump.Spec.ClusterReference)
		})
	} else {
		if err := r.CreateManulDump(dump); err != nil {
			log.Errorf("manul dump etcd cluster %v/%v err: %v", dump.Namespace, dump.Spec.ClusterReference, err)
			return err
		}
		log.Infof("manul dump etcd cluster %v/%v success", dump.Namespace, dump.Spec.ClusterReference)
	}
	return nil
}

// 1. etcd pod  exec etcd snapshot save xx.db (inside conatienr). > update etcdDump crd status.
// 2. get xx.db operator conatiner  and then remve etcd container xx.db. > update etcdDump crd status.
// 3. upload xx.db to storage, rm xx.db in operator container. > update etcdDump crd status.
func (r *ReconcileEtcdDump) CreateManulDump(dump *appv1alpha1.EtcdDump) error {
	// dump before
	dump.Status = appv1alpha1.EtcdDumpStatus{}
	running := appv1alpha1.EtcdDumpCondition{Ready: false, Location: "", LastedTranslationTime: metav1.Now(), Reason: "begin dump", Message: ""}
	if err := r.updateStatus(dump, running, appv1alpha1.EtcdDumpRunning); err != nil {
		return err
	}
	// exec dump cmd
	dumpFileName := fmt.Sprintf(DefaultDumpFileTemplate, dump.Namespace, dump.Spec.ClusterReference, time.Now().Format("2006150405"))
	dumpArgs := fmt.Sprintf("kubectl -n %v exec %v-0 -- sh -c 'ETCDCTL_API=3 etcdctl snapshot save %v'", dump.Namespace, dump.Spec.ClusterReference, dumpFileName)
	dumpCmd := exec.Command("/bin/sh", "-c", dumpArgs)
	dumpOut, err := dumpCmd.CombinedOutput()
	if err != nil {
		execDump := appv1alpha1.EtcdDumpCondition{Ready: false, Location: "", LastedTranslationTime: metav1.Now(), Reason: "dump cmd exec failed", Message: fmt.Sprintf("exec cmd : %v, cmd response : %v", dumpArgs, string(dumpOut))}
		if updateErr := r.updateStatus(dump, execDump, appv1alpha1.EtcdDumpRunning); updateErr != nil {
			return updateErr
		}
		return fmt.Errorf("exec cmd : %v, cmd response : %v", dumpArgs, string(dumpOut))
	}
	execDump := appv1alpha1.EtcdDumpCondition{Ready: true, Location: "", LastedTranslationTime: metav1.Now(), Reason: "dump cmd exec success", Message: ""}
	if updateErr := r.updateStatus(dump, execDump, appv1alpha1.EtcdDumpRunning); updateErr != nil {
		return updateErr
	}

	// get dump file to operator container
	cpArgs := fmt.Sprintf("kubectl cp %v/%v-0:%v %v", dump.Namespace, dump.Spec.ClusterReference, dumpFileName, LocalFileDir)
	cpCmd := exec.Command("/bin/sh", "-c", cpArgs)
	cpOut, err := cpCmd.CombinedOutput()
	if err != nil {
		execDump := appv1alpha1.EtcdDumpCondition{Ready: false, Location: "", LastedTranslationTime: metav1.Now(), Reason: "cp cmd exec failed", Message: fmt.Sprintf("exec cmd : %v, cmd response : %v", cpArgs, string(cpOut))}
		if updateErr := r.updateStatus(dump, execDump, appv1alpha1.EtcdDumpRunning); updateErr != nil {
			return updateErr
		}
		return fmt.Errorf("exec cmd : %v, cmd response : %v", cpArgs, string(cpOut))
	}
	cpDump := appv1alpha1.EtcdDumpCondition{Ready: true, Location: "", LastedTranslationTime: metav1.Now(), Reason: "cp cmd exec success", Message: ""}
	if updateErr := r.updateStatus(dump, cpDump, appv1alpha1.EtcdDumpRunning); updateErr != nil {
		return updateErr
	}

	rmCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("kubectl -n %v exec  %v-0 -- rm -f %v", dump.Namespace, dump.Spec.ClusterReference, dumpFileName))
	rmOut, err := rmCmd.CombinedOutput()
	if err != nil {
		log.Errorf("exec cmd : %v, cmd response : %v", fmt.Sprintf("kubectl -n %v exec  %v-0 -- rm -f %v", dump.Namespace, dump.Spec.ClusterReference, dumpFileName), string(rmOut))
	}

	// 调用存储的接口实现上传处理
	// err:=storageProvider.Store()
	// if err!=nil{
	// 	upload := appv1alpha1.EtcdDumpCondition{Ready: false, Location: location, LastedTranslationTime: metav1.Now(), Reason: "upload dump data to store failed", Message: err.Error()}
	// if updateErr := r.updateStatus(dump, upload); err != nil {
	// 	return updateErr
	// }
	// return fmt.Errorf("upload backup file to storage err: %v",err)
	// }
	upload := appv1alpha1.EtcdDumpCondition{Ready: true, Location: location, LastedTranslationTime: metav1.Now(), Reason: "upload dump data to store sunccess", Message: ""}
	if updateErr := r.updateStatus(dump, upload, appv1alpha1.EtcdDumpComplated); updateErr != nil {
		return updateErr
	}

	return nil
}

func (r *ReconcileEtcdDump) updateStatus(dump *appv1alpha1.EtcdDump, condition appv1alpha1.EtcdDumpCondition, phase appv1alpha1.EtcdDumpPhase) error {
	dump.Status.Phase = phase
	dump.Status.Conditions = append(dump.Status.Conditions, condition)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.client.Status().Update(context.TODO(), dump)
	})
}

func toString(dumpspec appv1alpha1.EtcdDumpSpec) string {
	data, _ := json.Marshal(dumpspec)
	return string(data)
}

func toSpec(data string) appv1alpha1.EtcdDumpSpec {
	spec := appv1alpha1.EtcdDumpSpec{}
	json.Unmarshal([]byte(data), &spec)
	return spec
}
