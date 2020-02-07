package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	appv1alpha1 "github.com/gok8s/etcd-operator/pkg/apis/app/v1alpha1"
	"github.com/gok8s/etcd-operator/pkg/resources/service"
	"github.com/gok8s/etcd-operator/pkg/resources/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_etcd")
var etcdSpec = "etcd.app.example.com/spec"

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
	// Create a new controller
	c, err := controller.New("etcd-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Etcd
	err = c.Watch(&source.Kind{Type: &appv1alpha1.Etcd{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Etcd
	err = c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appv1alpha1.Etcd{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileEtcd implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileEtcd{}

// ReconcileEtcd reconciles a Etcd object
type ReconcileEtcd struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Etcd object and makes changes based on the state read
// and what is in the Etcd.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileEtcd) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Etcd")

	// Fetch the Etcd instance
	instance := &appv1alpha1.Etcd{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Error(err, "Deleted... return")

			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if instance.DeletionTimestamp != nil {
		reqLogger.Error(err, "DeletionTimestamp!=nil,return")
		return reconcile.Result{}, nil
	}

	/*	// Define a new Pod object
		pod := newPodForCR(instance)

		// Set Etcd instance as the owner and controller
		if err := controllerutil.SetControllerReference(instance, pod, r.scheme); err != nil {
			return reconcile.Result{}, err
		}*/

	// Check if the instance's reference resource already exists
	found := &appsv1.StatefulSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Etcd", "Namespace", instance.Namespace, "Name", instance.Name)
		//create etcd: statefulset & svc

		svc := service.New(instance)
		if err := r.client.Create(context.TODO(), svc); err != nil {
			return reconcile.Result{}, err
		}

		ss := statefulset.New(instance)
		if err := r.client.Create(context.TODO(), ss); err != nil {
			go r.client.Delete(context.TODO(), svc)
			return reconcile.Result{}, err
		}

		//update status
		instance.Annotations = map[string]string{etcdSpec: toString(instance)}

		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			return r.client.Update(context.TODO(), instance)
		})
		if err != nil {
			reqLogger.Error(err, "retry update etcd annotations failed")
		}

		// Pod created successfully - don't requeue
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// exist then check update
	if !reflect.DeepEqual(instance.Spec, toSpec(instance.Annotations[etcdSpec])) {
		//update ,
		//get old ,get new,update
		newSS := statefulset.New(instance)
		reqLogger.Info(fmt.Sprintf("Updating,newSpec:%+v oldSpec:%+v", *newSS.Spec.Replicas, *found.Spec.Replicas))

		// old ss = found
		// update newSS's spec to found
		found.Spec = newSS.Spec

		//TODO don't update newSS directly
		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			return r.client.Update(context.TODO(), found)
		})
		if err != nil {
			reqLogger.Error(err, "retry update etcd resource failed")
			return reconcile.Result{}, err
		}

		if instance.Annotations != nil {
			instance.Annotations[etcdSpec] = toString(instance)
		} else {
			instance.Annotations = map[string]string{etcdSpec: toString(instance)}
		}

		retryErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			return r.client.Update(context.TODO(), instance)
		})
		if retryErr != nil {
			reqLogger.Error(err, "retry update etcd annotations failed")
			return reconcile.Result{}, err
		}

	} else {
		reqLogger.Info("nothing Updated", "instance.spec", instance.Spec, "annotation", toSpec(instance.Annotations["app.example.com/spec"]))
	}

	// statefulset already exists - don't requeue
	reqLogger.Info("Skip reconcile: statefulset already exists", "Namespace", found.Namespace, "Name", found.Name, "replicas", instance.Spec.Replicas)
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

// newPodForCR returns a busybox pod with the same name/namespace as the cr
func newPodForCR(cr *appv1alpha1.Etcd) *corev1.Pod {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-pod",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"sleep", "3600"},
				},
			},
		},
	}
}
