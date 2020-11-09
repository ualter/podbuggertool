package main

import (

	"context"
	"fmt"
	"time"

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
	"k8s.io/apimachinery/pkg/util/wait"
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
	recorder          record.EventRecorder
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
		recorder:          recorder,
	}

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
			//fmt.Printf("Added Pod: %s \n", newPod.Name)
			controller.handlePod(newPod)
		},
		UpdateFunc: func (oldObj, newObj interface{}) {
			newPod := newObj.(*corev1.Pod)
			oldPod := oldObj.(*corev1.Pod)
			//fmt.Printf("Added %s, Removed %s \n", newPod.Name, oldPod.Name)
		},
		DeleteFunc: func (obj interface{}) {
			delPod := obj.(*corev1.Pod)
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
		go wait.Until(c.runWorker, time.Second, stopCh)
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


// read a single work item off the workqueue and attempt to process it with syncHandler
func (c *Controller) processNextWorkItem() bool {
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

	deploymentName := podbuggertool.Spec.Image
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

	// If this number of the replicas on the Podbuggertool resource is specified, and the
	// number does not equal the current desired replicas on the Deployment, we
	// should update the Deployment resource.
	/*if podbuggertool.Spec.Replicas != nil && *podbuggertool.Spec.Replicas != *deployment.Spec.Replicas {
		klog.V(4).Infof("Podbuggertool %s replicas: %d, deployment replicas: %d", name, *podbuggertool.Spec.Replicas, *deployment.Spec.Replicas)
		deployment, err = c.kubeclientset.AppsV1().Deployments(podbuggertool.Namespace).Update(context.TODO(), newDeployment(podbuggertool), metav1.UpdateOptions{})
	}*/

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
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

func (c *Controller) handlePod(pod *corev1.Pod) {
	fmt.Printf("Pod to be handled %s \n", pod.Name)
	for k, v := range pod.Labels {
		label := k + "=" + v
		fmt.Printf("  --> Label: %s\n", label)
		// Check LABEL is present on one of the deployed Podbuggertool
		// IF YES:
		//    enqueue Pod to Add the EphimeralContainer with correlated Image at the Podbuggertool
		// OTHERWISE
		//    do nothing!
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
		"controller": podbuggertool.Name,
	}
	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podbuggertool.Spec.Image,
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

