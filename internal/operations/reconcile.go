package operations

import (
	"context"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func StopReconciliation() (ctrl.Result, error) {
	logger := log.FromContext(context.TODO())
	logger.Info("reconcile stopped")
	return ctrl.Result{}, nil
}

func RetryFailedReconciliation() (ctrl.Result, error) {
	logger := log.FromContext(context.TODO())
	logger.Info("reconcile failed, see you in a minute")
	return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
}
