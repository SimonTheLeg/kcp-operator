/*
Copyright 2024 The KCP Authors.

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

package controller

import (
	"context"
	"fmt"

	k8creconciling "k8c.io/reconciler/pkg/reconciling"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kcp-dev/kcp-operator/api/v1alpha1"
	operatorkcpiov1alpha1 "github.com/kcp-dev/kcp-operator/api/v1alpha1"
	"github.com/kcp-dev/kcp-operator/internal/reconciling"
	"github.com/kcp-dev/kcp-operator/internal/resources/rootshard"
)

// RootShardReconciler reconciles a RootShard object
type RootShardReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=operator.kcp.io,resources=rootshards,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=operator.kcp.io,resources=rootshards/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.kcp.io,resources=rootshards/finalizers,verbs=update
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=issuers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RootShard object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *RootShardReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling RootShard object")

	var rootShard v1alpha1.RootShard
	if err := r.Client.Get(ctx, req.NamespacedName, &rootShard); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, fmt.Errorf("failed to find %s/%s: %w", req.Namespace, req.Name, err)
		}

		// Object has apparently been deleted already.
		return ctrl.Result{}, nil
	}

	if rootShard.DeletionTimestamp != nil {
		rootShard.Status.Phase = operatorkcpiov1alpha1.RootShardPhaseDeleting
		if err := r.Client.Status().Update(ctx, &rootShard); err != nil {
			return ctrl.Result{}, err
		}
	}

	if rootShard.Status.Phase == "" {
		rootShard.Status.Phase = operatorkcpiov1alpha1.RootShardPhaseProvisioning
		if err := r.Client.Status().Update(ctx, &rootShard); err != nil {
			return ctrl.Result{}, err
		}
	}

	ownerRefWrapper := k8creconciling.OwnerRefWrapper(*metav1.NewControllerRef(&rootShard, operatorkcpiov1alpha1.GroupVersion.WithKind("RootShard")))

	// Intermediate CAs that we need to generate a certificate and an issuer for.
	intermediateCAs := []v1alpha1.CA{
		v1alpha1.ServerCA,
		v1alpha1.RequestHeaderClientCA,
		v1alpha1.ClientCA,
		v1alpha1.ServiceAccountCA,
	}

	issuerReconcilers := []reconciling.NamedIssuerReconcilerFactory{
		rootshard.RootCAIssuerReconciler(&rootShard),
	}

	certReconcilers := []reconciling.NamedCertificateReconcilerFactory{
		rootshard.ServerCertificateReconciler(&rootShard),
		rootshard.ServiceAccountCertificateReconciler(&rootShard),
		rootshard.VirtualWorkspacesCertificateReconciler(&rootShard),
	}

	for _, ca := range intermediateCAs {
		certReconcilers = append(certReconcilers, rootshard.CACertificateReconciler(&rootShard, ca))
		issuerReconcilers = append(issuerReconcilers, rootshard.CAIssuerReconciler(&rootShard, ca))
	}
	if rootShard.Spec.Certificates.IssuerRef != nil {
		certReconcilers = append(certReconcilers, rootshard.RootCACertificateReconciler(&rootShard))
	}

	if err := reconciling.ReconcileCertificates(ctx, certReconcilers, req.Namespace, r.Client, ownerRefWrapper); err != nil {
		return ctrl.Result{}, err
	}

	if err := reconciling.ReconcileIssuers(ctx, issuerReconcilers, req.Namespace, r.Client, ownerRefWrapper); err != nil {
		return ctrl.Result{}, err
	}

	if err := k8creconciling.ReconcileDeployments(ctx, []k8creconciling.NamedDeploymentReconcilerFactory{
		rootshard.DeploymentReconciler(&rootShard),
	}, req.Namespace, r.Client, ownerRefWrapper); err != nil {
		return ctrl.Result{}, err
	}

	if err := k8creconciling.ReconcileServices(ctx, []k8creconciling.NamedServiceReconcilerFactory{
		rootshard.ServiceReconciler(&rootShard),
	}, req.Namespace, r.Client, ownerRefWrapper); err != nil {
		return ctrl.Result{}, err
	}

	// check for Deployment health and update the rootShard phase if necessary.
	var dep appsv1.Deployment
	err := r.Client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: fmt.Sprintf("%s-kcp", rootShard.Name)}, &dep)
	if client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	}
	if err == nil {
		if rootShard.Status.Phase == operatorkcpiov1alpha1.RootShardPhaseProvisioning && dep.Status.ReadyReplicas == ptr.Deref(dep.Spec.Replicas, 0) {
			rootShard.Status.Phase = operatorkcpiov1alpha1.RootShardPhaseRunning
			if err := r.Client.Status().Update(ctx, &rootShard); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RootShardReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorkcpiov1alpha1.RootShard{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
