package etcddump

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/robfig/cron"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"os/exec"
	"reflect"
	"time"

	appv1alpha1 "github.com/gok8s/etcd-operator/pkg/apis/app/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	log                     = logf.Log.WithName("controller_etcddump")
	dumpCron                = cron.New()
	DefaultDumpFileTemplate = "root/%v_%v_%v.db"
	location                = "storageUrl/fileName"
	//todo make it configurable
	LocalFileDir = "/tmp/etcd.db"
)

func init() {
	dumpCron.Start()
}

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

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

// blank assignment to verify that ReconcileEtcdDump implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileEtcdDump{}

// ReconcileEtcdDump reconciles a EtcdDump object
type ReconcileEtcdDump struct {
	// This client, initialized using mgr.Client() above, is a split client
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
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling EtcdDump")

	// Fetch the EtcdDump instance
	instance := &appv1alpha1.EtcdDump{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if instance.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}
	//
	/*
		// don't need to create other resource here,just execute proceeeEtcdDump() is fine.

		// Check if this EtcdDump already exists
		found := &corev1.Pod{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Info("Creating a new Pod", "Pod.Namespace", instance.Namespace, "Pod.Name", instance.Name)

			err = r.client.Create(context.TODO(), pod)
			if err != nil {
				return reconcile.Result{}, err
			}

			// Pod created successfully - don't requeue
			return reconcile.Result{}, nil
		} else if err != nil {
			return reconcile.Result{}, err
		}*/

	//check update
	if !reflect.DeepEqual(instance.Spec, toSpec(instance.Annotations["etcddump.app.example.com/spec"])) {
		reqLogger.Info(fmt.Sprintf("reflect.deepEqual updated,instance.Spec:%+v, annotaion:%s", instance.Spec, instance.Annotations["etcddump.app.example.com/spec"]))
		if err := r.ProcessEtcdDump(instance); err != nil {
			// todo remove crontab task
			reqLogger.Error(err, "processEtcdDump failed", err.Error())
			return reconcile.Result{}, nil
		}
		// update status
		if instance.Annotations != nil {
			instance.Annotations["etcddump.app.example.com/spec"] = toString(instance.Spec)
		} else {
			instance.Annotations = map[string]string{"etcddump.app.example.com/spec": toString(instance.Spec)}
		}
		retryErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			return r.client.Update(context.TODO(), instance)
		})
		if retryErr != nil {
			reqLogger.Error(retryErr, fmt.Sprintf("update annotations failed:%+v", instance.Annotations))
		}
	} else {
		reqLogger.Info(fmt.Sprintf("nothing updated,instance.Spec:%+v, annotaion:%s", instance.Spec, instance.Annotations["etcddump.app.example.com/spec"]))
		return reconcile.Result{}, nil
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileEtcdDump) ProcessEtcdDump(etcddump *appv1alpha1.EtcdDump) error {
	if etcddump.Spec.Scheduler != "" {
		dumpCron.AddFunc(etcddump.Spec.Scheduler, func() {
			if err := r.CreateManulDump(etcddump); err != nil {
				log.Error(err, fmt.Sprintf("cron dump etcd cluster:%v/%v failed", etcddump.Namespace, etcddump.Name))
				return
			}
			log.Info(fmt.Sprintf("cron dump etcd cluster:%v/%v successed", etcddump.Namespace, etcddump.Name))
		})
	} else {
		if err := r.CreateManulDump(etcddump); err != nil {
			log.Error(err, fmt.Sprintf("manul dump etcd cluster:%v/%v failed", etcddump.Namespace, etcddump.Name))
			return err
		}
		log.Info(fmt.Sprintf("manul dump etcd cluster:%v/%v successed", etcddump.Namespace, etcddump.Name))
		return nil
	}
	return nil
}

func (r *ReconcileEtcdDump) CreateManulDump(etcddump *appv1alpha1.EtcdDump) error {
	log.Info("CreateManulDump")

	running := appv1alpha1.EtcdDumpCondition{
		Ready:                 true,
		Reason:                "begin dump",
		Message:               "",
		LastedTranslationTime: metav1.Now(),
		Location:              "",
	}

	etcddump.Status = appv1alpha1.EtcdDumpStatus{}
	if err := r.updateStatus(etcddump, running, appv1alpha1.EtcdDumpRunning); err != nil {
		log.Error(err, fmt.Sprintf("updateStatus failed:%s", err.Error()))
		return err
	}
	dumpFileName := fmt.Sprintf(DefaultDumpFileTemplate, etcddump.Namespace, etcddump.Spec.ClusterReference, time.Now().Format("2006150405"))

	dumpArgs := fmt.Sprintf(
		"kubectl -n %v exec %v-0 -- sh -c 'ETCDCTL_API=3 etcdctl snapshot save %v'",
		etcddump.Namespace, etcddump.Spec.ClusterReference, dumpFileName)

	log.Info(dumpArgs)
	dumpCmd := exec.Command("/bin/sh", "-c", dumpArgs)
	dumpOut, err := dumpCmd.CombinedOutput()
	if err != nil {
		log.Error(err, "snapshot failed", err.Error())
		execDump := appv1alpha1.EtcdDumpCondition{Ready: true, Location: "", LastedTranslationTime: metav1.Now(), Reason: "snapshot cmd exec failed", Message: fmt.Sprintf("exec cmd : %v, cmd response : %v", dumpArgs, string(dumpOut))}
		if updateErr := r.updateStatus(etcddump, execDump, appv1alpha1.EtcdDumpFailed); updateErr != nil {
			log.Error(err, "update status failed:snapshot failed")
			return updateErr
		}
	}
	log.Info("etcd-snapshot successed")

	execDump := appv1alpha1.EtcdDumpCondition{Ready: true, Location: "", LastedTranslationTime: metav1.Now(), Reason: "snapshot cmd exec successed", Message: ""}
	if updateErr := r.updateStatus(etcddump, execDump, appv1alpha1.EtcdDumpRunning); updateErr != nil {
		log.Error(err, fmt.Sprintf("updateStatus failed:%s", err.Error()))
		return updateErr
	}

	// get dump file to the operator container
	cpArgs := fmt.Sprintf("kubectl -n %v cp %v-0:%v %v",
		etcddump.Namespace,
		etcddump.Spec.ClusterReference,
		dumpFileName,
		LocalFileDir)
	log.Info(cpArgs)

	cpCmd := exec.Command("/bin/sh", "-c", cpArgs)
	cpOut, err := cpCmd.CombinedOutput()
	if err != nil {
		log.Error(err, fmt.Sprintf("cp failed err:%s", err.Error()))
		cpDump := appv1alpha1.EtcdDumpCondition{Ready: true, Location: "", LastedTranslationTime: metav1.Now(), Reason: "cp cmd exec failed", Message: fmt.Sprintf("exec cmd : %v, cp response : %v", cpArgs, string(cpOut))}
		if updateErr := r.updateStatus(etcddump, cpDump, appv1alpha1.EtcdDumpFailed); updateErr != nil {
			log.Error(err, fmt.Sprintf("updateStatus failed:%s", err.Error()))
			return updateErr
		}
		return err
	}
	log.Info("cp successed")

	cpDump := appv1alpha1.EtcdDumpCondition{Ready: true, Location: "", LastedTranslationTime: metav1.Now(), Reason: "cp cmd exec success", Message: ""}
	if updateErr := r.updateStatus(etcddump, cpDump, appv1alpha1.EtcdDumpFailed); updateErr != nil {
		log.Error(err, "update status:cp successed failed.")
		return updateErr
	}

	//rm container file
	rmArgs := fmt.Sprintf("kubectl -n %v exec  %v-0 -- sh -c 'rm -f %v'", etcddump.Namespace, etcddump.Spec.ClusterReference, dumpFileName)
	log.Info(rmArgs)
	rmCmd := exec.Command("/bin/sh", "-c", rmArgs)
	rmOut, err := rmCmd.CombinedOutput()
	if err != nil {
		log.Error(err, fmt.Sprintf("rm failed err:%s", err.Error()))
		cpDump := appv1alpha1.EtcdDumpCondition{Ready: true, Location: "", LastedTranslationTime: metav1.Now(), Reason: "rm cmd exec failed", Message: fmt.Sprintf("exec cmd : %v, cp response : %v", rmArgs, string(rmOut))}
		if updateErr := r.updateStatus(etcddump, cpDump, appv1alpha1.EtcdDumpFailed); updateErr != nil {
			log.Error(err, fmt.Sprintf("updateStatus failed:%s", err.Error()))
			return updateErr
		}
		return err
	}
	log.Info("rm successed")
	rmPhase := appv1alpha1.EtcdDumpCondition{Ready: true, Location: "", LastedTranslationTime: metav1.Now(), Reason: "rm cmd exec successed", Message: ""}
	if updateErr := r.updateStatus(etcddump, rmPhase, appv1alpha1.EtcdDumpRunning); updateErr != nil {
		log.Error(err, "update status failed:rm successed")
		return updateErr
	}

	//upload file to storage
	// 调用存储的接口实现上传处理
	// err:=storageProvider.Store()
	// if err!=nil{
	// 	upload := appv1alpha1.EtcdDumpCondition{Ready: false, Location: location, LastedTranslationTime: metav1.Now(), Reason: "upload dump data to store failed", Message: err.Error()}
	// if updateErr := r.updateStatus(dump, upload); err != nil {
	// 	return updateErr
	// }
	// return fmt.Errorf("upload backup file to storage err: %v",err)
	// }
	log.Info("upload success")
	upload := appv1alpha1.EtcdDumpCondition{Ready: true, Location: location, LastedTranslationTime: metav1.Now(), Reason: "upload dump data to store sunccess", Message: ""}
	if updateErr := r.updateStatus(etcddump, upload, appv1alpha1.EtcdDumpCompleted); updateErr != nil {
		log.Error(err, fmt.Sprintf("updateStatus failed:%s", err.Error()))
		return updateErr
	}

	log.Info("finished")
	return nil
}

func (r *ReconcileEtcdDump) updateStatus(etcddump *appv1alpha1.EtcdDump, condition appv1alpha1.EtcdDumpCondition, phase appv1alpha1.EtcdDumpPhase) error {
	etcddump.Status.Phase = phase
	etcddump.Status.Conditions = append(etcddump.Status.Conditions, condition)
	log.Info(fmt.Sprintf("conditions:%+v", condition))
	//etcddump.Status.Conditions = []appv1alpha1.EtcdDumpCondition{condition}

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return r.client.Status().Update(context.TODO(), etcddump)
	})
}

func toString(spec appv1alpha1.EtcdDumpSpec) string {
	bytes, err := json.Marshal(spec)
	if err != nil {
		log.Error(err, "toString:json.Marshal failed")
	}
	return string(bytes)
}

func toSpec(data string) appv1alpha1.EtcdDumpSpec {
	spec := appv1alpha1.EtcdDumpSpec{}
	err := json.Unmarshal([]byte(data), &spec)
	if err != nil {
		if data == "" {
			log.Info("empty spec")
		} else {
			log.Error(err, "toSpec:json.Unmarshal failed:data:%s", data)
		}
	}
	return spec
}
