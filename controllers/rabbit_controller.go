/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	farmv1 "rabbitco.io/api/v1"
)

// RabbitReconciler reconciles a Rabbit object
type RabbitReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=farm.rabbitco.io,resources=rabbits,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=farm.rabbitco.io,resources=rabbits/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=farm.rabbitco.io,resources=rabbits/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Rabbit object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.2/pkg/reconcile
func (r *RabbitReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("rabbit", req.NamespacedName)

	var rabbit farmv1.Rabbit
	if err := r.Get(ctx, req.NamespacedName, &rabbit); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		err = client.IgnoreNotFound(err)
		if err != nil {
			r.Log.Error(err, "unable to fetch rabbit")
		}
		return ctrl.Result{}, err
	}
	r.Log.Info("received update event", "name", rabbit.Name, "namespace", rabbit.Namespace)
	resourceFuncs := []func(ctx context.Context, rabbit *farmv1.Rabbit) error{
		r.createRabbits,
	}
	for _, resourceFunc := range resourceFuncs {
		if err := resourceFunc(ctx, &rabbit); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RabbitReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&farmv1.Rabbit{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

func (r *RabbitReconciler) createRabbits(ctx context.Context, rabbit *farmv1.Rabbit) error {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rabbit.Name,
			Namespace: rabbit.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		labels := map[string]string{
			"RabbitFarm": rabbit.Name,
		}
		deployment.Spec = appsv1.DeploymentSpec{
			Replicas: &rabbit.Spec.StartingPopulation,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:    "base",
							Image:   "busybox",
							Command: []string{"tail", "-f", "/dev/null"},
						},
					},
				},
			},
		}
		if err := ctrl.SetControllerReference(rabbit, deployment, r.Scheme); err != nil {
			return err
		}
		return nil
	})
	return err
}
