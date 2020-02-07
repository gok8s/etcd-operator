package etcdrestore

import (
	"context"
	appv1alpha1 "github.com/gok8s/etcd-operator/pkg/apis/app/v1alpha1"
	"github.com/gok8s/etcd-operator/pkg/resources/statefulset"
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
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_etcdrestore")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

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

	err = c.Watch(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, &handler.EnqueueRequestForOwner{})
	if err != nil {
		return err
	}
	return nil
}

// blank assignment to verify that ReconcileEtcdRestore implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileEtcdRestore{}

// ReconcileEtcdRestore reconciles a EtcdRestore object
type ReconcileEtcdRestore struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
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
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling EtcdRestore")

	// Fetch the EtcdRestore instance
	instance := &appv1alpha1.EtcdRestore{}
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

	//before restore
	condition := appv1alpha1.EtcdRestoreCondition{
		Ready:                 true,
		Reason:                "before restore",
		Message:               "",
		LastedTranslationTime: metav1.Now(),
	}

	if err := r.updateStatus(instance, condition, appv1alpha1.EtcdRestoreRunning); err != nil {
		log.Error(err, "updatestatus failed,before restore")
		return reconcile.Result{}, err
	}

	// restoring
	ss := &appsv1.StatefulSet{}
	if err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: instance.Namespace, Name: instance.Spec.ClusterReference}, ss); err != nil {
		condition := appv1alpha1.EtcdRestoreCondition{
			Ready:                 false,
			Reason:                "get reference statefulset failed",
			Message:               err.Error(),
			LastedTranslationTime: metav1.Now(),
		}
		if updateErr := r.updateStatus(instance, condition, appv1alpha1.EtcdRestoreFailed); updateErr != nil {
			log.Error(err, "updatestatus failed,get reference ss failed")
			return reconcile.Result{}, updateErr
		}
	}
	podList, err := r.listPod(ss)
	if err != nil {
		condition := appv1alpha1.EtcdRestoreCondition{
			Ready:                 false,
			Reason:                "get reference podlist failed",
			Message:               err.Error(),
			LastedTranslationTime: metav1.Now(),
		}
		if updateErr := r.updateStatus(instance, condition, appv1alpha1.EtcdRestoreFailed); updateErr != nil {
			log.Error(err, "updatestatus failed,get reference podlist failed")
			return reconcile.Result{}, updateErr
		}
		return reconcile.Result{}, err
	}
	// 清空pod对应的pvc
	for _, pod := range podList {
		for _, v := range pod.Spec.Volumes {
			if v.VolumeSource.PersistentVolumeClaim != nil {
				//delete
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
					// update condition
					condition := appv1alpha1.EtcdRestoreCondition{
						Ready:                 false,
						Reason:                "delete pvc failed",
						Message:               err.Error(),
						LastedTranslationTime: metav1.Now(),
					}

					if updateErr := r.updateStatus(instance, condition, appv1alpha1.EtcdRestoreFailed); updateErr != nil {
						reqLogger.Error(err, "updatestatus failed")
						return reconcile.Result{}, updateErr
					}
					return reconcile.Result{}, err
				}
			}
		}
	}
	// prepare to create statefulset
	// create init container
	ss.Spec.Template.Spec.InitContainers = statefulset.NewInitContainer(ss, instance)
	if err := r.client.Update(context.TODO(), ss); err != nil {
		condition := appv1alpha1.EtcdRestoreCondition{
			Ready:                 false,
			Reason:                "create init container failed",
			Message:               err.Error(),
			LastedTranslationTime: metav1.Now(),
		}
		if updateErr := r.updateStatus(instance, condition, appv1alpha1.EtcdRestoreFailed); updateErr != nil {
			reqLogger.Error(err, "update status failed,initcontainer failed")
		}
	}
	// success update
	condition = appv1alpha1.EtcdRestoreCondition{
		Ready:                 true,
		Reason:                "create init container successed",
		Message:               err.Error(),
		LastedTranslationTime: metav1.Now(),
	}
	if updateErr := r.updateStatus(instance, condition, appv1alpha1.EtcdRestoreCompleted); updateErr != nil {
		reqLogger.Error(err, "update status failed,initcontainer successed")
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileEtcdRestore) updateStatus(restore *appv1alpha1.EtcdRestore, condition appv1alpha1.EtcdRestoreCondition, phase appv1alpha1.EtcdRestorePhase) error {
	restore.Status.Phase = phase
	restore.Status.Conditions = append(restore.Status.Conditions, condition)

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return r.client.Update(context.TODO(), restore)
	})
}

/*func (r *ReconcileEtcdRestore) listPods(ss *appsv1.StatefulSet) []corev1.Pod {
	var pods []corev1.Pod
		if err := r.client.List(context.TODO(),
		client.ListOptions{
			Namespace:ss.Namespace,
			LabelSelector: labels.SelectorFromSet(
			labels.Set(ss.Spec.Selector.MatchLabels))
	    }, pods); err != nil {

		}
}
*/

func (r *ReconcileEtcdRestore) listPod(ss *appsv1.StatefulSet) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	listOpts := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set(ss.Spec.Selector.MatchLabels)),
		Namespace:     ss.Namespace,
	}

	if err := r.client.List(context.TODO(), podList, listOpts); err != nil {
		return nil, err
	}
	return podList.Items, nil
}
