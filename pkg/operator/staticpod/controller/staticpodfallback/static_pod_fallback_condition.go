package staticpodfallback

import (
	"context"
	"fmt"
	"time"

	operatorv1 "github.com/openshift/api/operator/v1"
	applyoperatorv1 "github.com/openshift/client-go/operator/applyconfigurations/operator/v1"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/staticpod/startupmonitor/annotations"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"

	"k8s.io/apimachinery/pkg/labels"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// staticPodFallbackConditionController knows how to detect and report that a static pod was rolled back to a previous revision
type staticPodFallbackConditionController struct {
	controllerInstanceName string
	operatorClient         operatorv1helpers.OperatorClient

	podLabelSelector labels.Selector
	podLister        corev1listers.PodNamespaceLister

	startupMonitorEnabledFn func() (bool, error)
}

// New creates a controller that detects and report roll back of a static pod
func New(
	instanceName, targetNamespace string,
	podLabelSelector labels.Selector,
	operatorClient operatorv1helpers.OperatorClient,
	kubeInformersForNamespaces operatorv1helpers.KubeInformersForNamespaces,
	startupMonitorEnabledFn func() (bool, error),
	eventRecorder events.Recorder) (factory.Controller, error) {
	if podLabelSelector == nil {
		return nil, fmt.Errorf("StaticPodFallbackConditionController: missing required podLabelSelector")
	}
	if podLabelSelector.Empty() {
		return nil, fmt.Errorf("StaticPodFallbackConditionController: podLabelSelector cannot be empty")
	}
	fd := &staticPodFallbackConditionController{
		controllerInstanceName:  factory.ControllerInstanceName(instanceName, "StaticPodStateFallback"),
		operatorClient:          operatorClient,
		podLabelSelector:        podLabelSelector,
		podLister:               kubeInformersForNamespaces.InformersFor(targetNamespace).Core().V1().Pods().Lister().Pods(targetNamespace),
		startupMonitorEnabledFn: startupMonitorEnabledFn,
	}
	return factory.New().
		WithSync(fd.sync).
		ResyncEvery(6*time.Minute).
		WithInformers(kubeInformersForNamespaces.InformersFor(targetNamespace).Core().V1().Pods().Informer()).
		ToController(
			fd.controllerInstanceName,
			eventRecorder,
		), nil
}

// sync sets/unsets a StaticPodFallbackRevisionDegraded condition if a pod that matches the given label selector is annotated with FallbackForRevision
func (fd *staticPodFallbackConditionController) sync(ctx context.Context, _ factory.SyncContext) (err error) {
	degradedCondition := applyoperatorv1.OperatorCondition().WithType("StaticPodFallbackRevisionDegraded")
	status := applyoperatorv1.OperatorStatus()
	defer func() {
		if err == nil {
			status = status.WithConditions(degradedCondition)
			if applyError := fd.operatorClient.ApplyOperatorStatus(ctx, fd.controllerInstanceName, status); applyError != nil {
				err = applyError
			}
		}
	}()

	// we rely on operators to provide
	// a condition for checking we are running on a single node cluster
	if enabled, err := fd.startupMonitorEnabledFn(); err != nil {
		return err
	} else if !enabled {
		degradedCondition = degradedCondition.WithStatus(operatorv1.ConditionFalse)
		return nil
	}

	kasPods, err := fd.podLister.List(fd.podLabelSelector)
	if err != nil {
		return err
	}

	var conditionReason string
	var conditionMessage string
	for _, kasPod := range kasPods {
		if fallbackFor, ok := kasPod.Annotations[annotations.FallbackForRevision]; ok {
			reason := "Unknown"
			message := "unknown"
			if s, ok := kasPod.Annotations[annotations.FallbackReason]; ok {
				reason = s
			}
			if s, ok := kasPod.Annotations[annotations.FallbackMessage]; ok {
				message = s
			}

			message = fmt.Sprintf("a static pod %v was rolled back to revision %v due to %v", kasPod.Name, fallbackFor, message)
			if len(conditionMessage) > 0 {
				conditionMessage = fmt.Sprintf("%s\n%s", conditionMessage, message)
			} else {
				conditionMessage = message
			}
			if len(conditionReason) == 0 {
				conditionReason = reason
			}
		}
	}

	// by default, the condition is in a non-degraded state
	degradedCondition = degradedCondition.WithStatus(operatorv1.ConditionFalse)
	if len(conditionReason) > 0 || len(conditionMessage) > 0 {
		degradedCondition = degradedCondition.
			WithMessage(conditionMessage).
			WithReason(conditionReason).
			WithStatus(operatorv1.ConditionTrue)
	}
	return nil
}
