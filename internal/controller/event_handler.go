// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package controller

import (
	"reflect"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

/*
Event filter and handler for ConfigMap objects which are owned by AzureAppConfigurationProvider API
*/
type EnqueueRequestsFromWatchedObject struct {
}

// Update implements EventHandler.
func (e *EnqueueRequestsFromWatchedObject) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	ownerRefs := evt.ObjectNew.GetOwnerReferences()
	ownedByProvider := false
	ownerProvider := ""
	if len(ownerRefs) > 0 {
		for _, owner := range ownerRefs {
			if owner.Kind == ProviderName {
				ownedByProvider = true
				ownerProvider = owner.Name
				break
			}
		}
	}
	// Only queue new reconcile request when both conditions are true
	// 1. ConfigMap/Secret is owned by AzureAppConfigurationProvider
	// 2. ConfigMap/Secret is not updated by the reconcile controller
	if !ownedByProvider || evt.ObjectNew.GetAnnotations()[LastReconcileTimeAnnotation] != evt.ObjectOld.GetAnnotations()[LastReconcileTimeAnnotation] {
		return
	}

	q.Add(reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      ownerProvider,
			Namespace: evt.ObjectNew.GetNamespace(),
		},
	})
}

// Delete implements EventHandler.
func (e *EnqueueRequestsFromWatchedObject) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	ownerRefs := evt.Object.GetOwnerReferences()
	ownedByProvider := false
	ownerProvider := ""
	if len(ownerRefs) > 0 {
		for _, owner := range ownerRefs {
			if owner.Kind == ProviderName {
				ownedByProvider = true
				ownerProvider = owner.Name
				break
			}
		}
	}
	if !ownedByProvider {
		return
	}

	q.Add(reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      ownerProvider,
			Namespace: evt.Object.GetNamespace(),
		},
	})
}

// Create implements EventHandler.
func (e *EnqueueRequestsFromWatchedObject) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	// Do nothing
}

// Generic implements EventHandler.
func (e *EnqueueRequestsFromWatchedObject) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	// Do nothing
}

type WatchedObjectPredicate struct {
	predicate.Funcs
}

func (WatchedObjectPredicate) Delete(e event.DeleteEvent) bool {
	return true
}
func (WatchedObjectPredicate) Update(e event.UpdateEvent) bool {
	return true
}
func (WatchedObjectPredicate) Create(event.CreateEvent) bool {
	//Ignore the create event
	return false
}
func (WatchedObjectPredicate) Generic(event.GenericEvent) bool {
	//Ignore the generic event
	return false
}

/*
Event Filter for AzureAppConfigurationProvider objects
*/
func newEventFilter() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Only handle the update event when Annotation, Label or Spec is update.
			return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() ||
				!reflect.DeepEqual(e.ObjectOld.GetAnnotations(), e.ObjectNew.GetAnnotations()) ||
				!reflect.DeepEqual(e.ObjectOld.GetLabels(), e.ObjectNew.GetLabels())
		},
	}
}
