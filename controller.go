package main

import (

	"context"
	"fmt"
	"time"
	"strings"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gouuid "github.com/nu7hatch/gouuid"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	pbtv1beta1   "podbuggertool/pkg/apis/podbuggertool/v1beta1"
	pbtclientset "podbuggertool/pkg/generated/clientset/versioned"
	pbtscheme    "podbuggertool/pkg/generated/clientset/versioned/scheme"
	pbtinformers "podbuggertool/pkg/generated/informers/externalversions/podbuggertool/v1beta1"
	pbtlisters   "podbuggertool/pkg/generated/listers/podbuggertool/v1beta1"

	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
)


type MapPodBuggerTool struct {
	podbuggertool *pbtv1beta1.Podbuggertool
	labelKey      string
	labelValue    string
}

// Controller is the controller :-)
type Controller struct {
	// k8s is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// myapiclientset is a clientset for our own API group
	pbtclientset      pbtclientset.Interface
	deploymentsLister appslisters.DeploymentLister
	deploymentsSynced cache.InformerSynced
	podsLister        corelisters.PodLister
	podsSynced        cache.InformerSynced
	pbtLister         pbtlisters.PodbuggertoolLister
	pbtSynced         cache.InformerSynced
	workqueue         workqueue.RateLimitingInterface
	workqueuePods     workqueue.RateLimitingInterface
	recorder          record.EventRecorder
	mapPbtool         map[string]MapPodBuggerTool
}

const controllerAgentName = "podbuggertool"

const (
	SuccessSynced = "Synced"
	ErrResourceExists = "ErrResourceExists"
	MessageResourceExists = "Resource %q already exists and is not managed by PodBuggerTool"
	MessageResourceSynced = "PodBuggerTool synced successfully"
)

func NewController(kubeclientset      kubernetes.Interface,
	               pbtclientset       pbtclientset.Interface,
				   deploymentInformer appsinformers.DeploymentInformer,
				   podInformer        coreinformers.PodInformer,
	               pbtInformer        pbtinformers.PodbuggertoolInformer) *Controller {

	// Create event broadcaster
	// Add podbuggertool types to the default Kubernetes Scheme so Events can be logged for my-controller types.
	utilruntime.Must(pbtscheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:     kubeclientset,
		pbtclientset:      pbtclientset,
		deploymentsLister: deploymentInformer.Lister(),
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
		podsLister:        podInformer.Lister(),
		podsSynced:        podInformer.Informer().HasSynced,
		pbtLister:         pbtInformer.Lister(),
		pbtSynced:         pbtInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "podbuggertool"),
		workqueuePods:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "podbuggertool-pods"),
		recorder:          recorder,
	}

	controller.mapPbtool = make(map[string]MapPodBuggerTool)

	klog.Info("Setting up event handlers")
	// PodBuggerTool
	pbtInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueuePodbuggertool,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueuePodbuggertool(new)
		},
	})
	klog.Info("PodBuggerTool eventhandler set")

	// Pods
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func (obj interface{}) {
			newPod := obj.(*corev1.Pod)
			fmt.Printf(" ********* \033[1;94m Pod added to K8s: %s \n\033[0;0m", newPod.Name)
			controller.enqueuePod(newPod)
		},
		UpdateFunc: func (oldObj, newObj interface{}) {
			newPod := newObj.(*corev1.Pod)
			oldPod := oldObj.(*corev1.Pod)
			_, _ = newPod, oldPod
			//controller.enqueuePod(newPod)
			//fmt.Printf("    ---->> \033[1;94mAdded %s, Removed %s \033[0;0m\n", newPod.Name, oldPod.Name)
		},
		DeleteFunc: func (obj interface{}) {
			delPod := obj.(*corev1.Pod)
			_ = delPod
			//fmt.Printf("Deleted Pod: %s \n", delPod.Name)
		},
	})
	klog.Info("Pod eventhandler set")

	// Deployment
	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*appsv1.Deployment)
			oldDepl := old.(*appsv1.Deployment)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})
	klog.Info("Deployment eventhandler set")

	return controller
}

// It will block until stopCh is closed
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	klog.Info("Starting PodBuggerTool controller")

	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.pbtSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process PodBuggertool resources
	for i := 0; i < threadiness; i++ {
		go utilwait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// Continually run
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	var result bool
	result = c.processNextWorkItemPodbuggertool()
	result = c.processNextWorkItemPod()
	return result
}

// read a single work item off the workqueue and attempt to process it with syncHandler
func (c *Controller) processNextWorkItemPodbuggertool() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)
		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}

		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) processNextWorkItemPod() bool {
	fmt.Printf(" \n\nCALLED \033[43mprocessNextWorkItemPod\033[0m\n")
	obj, shutdown := c.workqueuePods.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueuePods.Done.
	err := func(obj interface{}) error {
		defer c.workqueuePods.Done(obj)
		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.workqueuePods.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueuePods but got %#v", obj))
			return nil
		}
		
		if err := c.handlePod(key); err != nil {
			// Put the item back on the workqueuePods to handle any transient errors.
			c.workqueuePods.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing in workqueuePods", key, err.Error())
		}

		// get queued again until another change happens.
		c.workqueuePods.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) handlePod(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	pod, err := c.podsLister.Pods(namespace).Get(name)
	if err != nil {
	   if errors.IsNotFound(err) {
	  	  utilruntime.HandleError(fmt.Errorf("Pod '%s' in queue no longer exists", key))
		  return nil
	   }
  	   return err
	}
	fmt.Printf(" \033[0;33m ----> POD: %s\033[0;0m\n", pod.Name)

	// Update Pod if it's in Running/Ready State
	// It will call updatePod immediatly if the Pod it's in the state Running/Ready
	//  OTHERWISE...
	// I will try every 3 seconds, until it returns true, an error, or the timeout(120 seconds) is reached
	utilwait.PollImmediate(3*time.Second, 120*time.Second, func() (bool, error) {
		podState, err := c.kubeclientset.CoreV1().Pods(namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
		if err != nil || c.isPodReady(podState) {
			if err != nil {
			   klog.Errorf("Error %s while waiting for Pod %s to be in Ready State", err.Error(), pod.Name)
			} else {
			   klog.Infof("Pod %s is READY to be udpated", pod.Name)
			   c.updatePod(namespace, pod)
			}
			return true, nil
		}
		klog.Infof("Pod %s IS NOT in Ready yet...", pod.Name)
		return false, nil
	})

	return nil
}

func (c *Controller) isPodReady(pod *corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// syncHandler compares the actual state with the desired, and attempts to converge the two. 
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the PodBuggertool resource with this namespace/name
	podbuggertool, err := c.pbtLister.Podbuggertools(namespace).Get(name)
	if err != nil {
		// The PodBuggerTool resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("podbuggertool '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	deploymentName := podbuggertool.Name
	if deploymentName == "" {
		utilruntime.HandleError(fmt.Errorf("%s: deployment name must be specified", key))
		return nil
	}

	// Get the deployment with the name specified in Podbuggertool.spec
	deployment, err := c.deploymentsLister.Deployments(podbuggertool.Namespace).Get(deploymentName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		deployment, err = c.kubeclientset.AppsV1().Deployments(podbuggertool.Namespace).Create(context.TODO(), newDeployment(podbuggertool), metav1.CreateOptions{})
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// If the Deployment is not controlled by this Podbuggertool resource, we should log
	// a warning to the event recorder and return error msg.
	if !metav1.IsControlledBy(deployment, podbuggertool) {
		msg := fmt.Sprintf(MessageResourceExists, deployment.Name)
		c.recorder.Event(podbuggertool, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
	}

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// Finally, we update the status block of the Podbuggertool resource to reflect the
	// current state of the world
	err = c.updatePodbuggertoolStatus(podbuggertool, deployment)
	if err != nil {
		return err
	}

	c.recorder.Event(podbuggertool, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)

	return nil
}

func (c *Controller) updatePodbuggertoolStatus(podbuggertool *pbtv1beta1.Podbuggertool, deployment *appsv1.Deployment) error {
	podbuggertoolCopy := podbuggertool.DeepCopy()
	podbuggertoolCopy.Status.Installed = 1
	_, err := c.pbtclientset.UalterV1beta1().Podbuggertools(podbuggertool.Namespace).Update(context.TODO(), podbuggertoolCopy, metav1.UpdateOptions{})
	return err
}

func (c *Controller) enqueuePodbuggertool(obj interface{}) {
	// Add Label to Map Pool
	pdt := obj.(*pbtv1beta1.Podbuggertool)
	if strings.Contains(pdt.Spec.Label,"=") {

	   if _, ok := c.mapPbtool[pdt.Spec.Label]; !ok {
			splitLbl := strings.Split(pdt.Spec.Label,"=")

			mpbt := MapPodBuggerTool{
				podbuggertool: pdt,
				labelKey:      splitLbl[0],
				labelValue:    splitLbl[1],
			}
			c.mapPbtool[pdt.Spec.Label] = mpbt
	
			msg := fmt.Sprintf("Looking for labels: %s",pdt.Spec.Label)
			c.recorder.Event(pdt, corev1.EventTypeNormal, SuccessSynced, msg)
	   }
	   
	} else {
	   msg := fmt.Sprintf("The label %s of the Podbuggertool %s is in incorrect format\n",pdt.Spec.Label,pdt)	
	   c.recorder.Event(pdt, corev1.EventTypeNormal, "Warning", msg)	
	   klog.Warningf(msg)
	}

	// Add Podbuggertool to Queue
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

func(c *Controller) checkLabelPod(pod *corev1.Pod) bool {
	for k, v := range pod.Labels {
		label := k + "=" + v
		fmt.Printf("  \033[0;36m      --> Label: %s\033[0;0m\n", label)

		// Check Label is to be considered by the Podbuggertools deployed
		if mapPodBuggerTool, ok := c.mapPbtool[label]; ok {
			fmt.Printf("  \033[0;93m     --> Found Pod to add to Queue: %s\033[0;0m\n", mapPodBuggerTool.podbuggertool.Spec.Label)
			msg := fmt.Sprintf("Found Pod %s with label %s to be handled", pod.Name, label)
			klog.Info(msg)
			c.recorder.Event(mapPodBuggerTool.podbuggertool, corev1.EventTypeNormal, "CheckLabelPod", msg)
			return true
		}
	}
	return false
}

func(c *Controller) updatePod(namespace string, pod *corev1.Pod) error {
	corev1Pod, err := c.kubeclientset.CoreV1().Pods(namespace).Get(context.TODO(),pod.Name, metav1.GetOptions{})

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Something went very badly wrong, Pod '%s' wasn't found", pod.Name))
		return nil
	}

	// Get the PodBuggerTool associated with the label (key=value) in this Pod
	var mapPodBuggerTool MapPodBuggerTool
	for k, v := range pod.Labels {
		label := k + "=" + v
		var ok bool
	    if mapPodBuggerTool, ok = c.mapPbtool[label]; ok {
			break;
		}
	}

	containers :=  []corev1.EphemeralContainer{
		{
			EphemeralContainerCommon: corev1.EphemeralContainerCommon{
				Name:                     "busybox",
				Image:                    "busybox",
				ImagePullPolicy:          corev1.PullIfNotPresent,
				Command:  []string{"sh"},
				Stdin: true,
				TTY: true,
				TerminationMessagePolicy: "File",
			},
		},
	}
	var msg string
	corev1Pod.Spec.EphemeralContainers = containers
	if _, err := c.kubeclientset.CoreV1().Pods(namespace).Update(context.TODO(), corev1Pod, metav1.UpdateOptions{}); err == nil {
	   msg := fmt.Sprintf("Error attempting add ephemeral container to Pod '%s'", corev1Pod.Name)
	   utilruntime.HandleError(fmt.Errorf(msg))
	   c.recorder.Event(mapPodBuggerTool.podbuggertool, corev1.EventTypeNormal, "UpdatePod", msg)
	   return nil
	}
	ec, err := c.kubeclientset.CoreV1().Pods(namespace).GetEphemeralContainers(context.TODO(), corev1Pod.Name, metav1.GetOptions{})
	if err != nil {
	   msg := fmt.Sprintf("Error attempting add ephemeral container to Pod '%s'", corev1Pod.Name)
	   utilruntime.HandleError(fmt.Errorf(msg))
	   c.recorder.Event(mapPodBuggerTool.podbuggertool, corev1.EventTypeNormal, "UpdatePod", msg)
	   return nil
	}
	ec.EphemeralContainers = containers
	if _, err = c.kubeclientset.CoreV1().Pods(namespace).UpdateEphemeralContainers(context.TODO(), corev1Pod.Name, ec, metav1.UpdateOptions{}); err != nil {
		msg := fmt.Sprintf("Error attempting add ephemeral container to Pod '%s'", corev1Pod.Name)
		utilruntime.HandleError(fmt.Errorf(msg))
		c.recorder.Event(mapPodBuggerTool.podbuggertool, corev1.EventTypeNormal, "UpdatePod", msg)
		return nil
	}
	
	msg = fmt.Sprintf("Successfully added the Ephemeral Container to Pod '%s'", corev1Pod.Name)
	c.recorder.Event(mapPodBuggerTool.podbuggertool, corev1.EventTypeNormal, "UpdatePod", msg)

	return nil
}

func (c *Controller) enqueuePod(pod *corev1.Pod) {
	if considered := c.checkLabelPod(pod); considered {
		var key string
		var err error
		if key, err = cache.MetaNamespaceKeyFunc(pod); err != nil {
		   utilruntime.HandleError(err)
		   return
		}
		klog.Infof("Pod Successfully queued '%s'", key)
		c.workqueuePods.Add(key)
	}
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Podbuggertool resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Podbuggertool resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	klog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a Podbuggertool, we should not do anything more
		// with it.
		if ownerRef.Kind != "Podbuggertool" {
			return
		}

		podbuggertool, err := c.pbtLister.Podbuggertools(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.V(4).Infof("ignoring orphaned object '%s' of podbuggertool '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueuePodbuggertool (podbuggertool)
		return
	}
}

func generateRandomName() string {
	u4, err := gouuid.NewV4()
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Error generating Podbuggertool Name"))
		return ""
	}
	return u4.String()
}

// newDeployment creates a new Deployment for a Podbuggertool resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Podbuggertool resource that 'owns' it.
func newDeployment(podbuggertool *pbtv1beta1.Podbuggertool) *appsv1.Deployment {
	labels := map[string]string{
		"app":        "podbuggertool",
		"deployment": podbuggertool.Name,
	}
	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podbuggertool.Name,
			Namespace: podbuggertool.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(podbuggertool, pbtv1beta1.SchemeGroupVersion.WithKind("Podbuggertool")),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}
}

