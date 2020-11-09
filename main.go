package main

import  (
	"flag"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	pdtclientset "podbuggertool/pkg/generated/clientset/versioned"
	pdtinformers "podbuggertool/pkg/generated/informers/externalversions"

	kubeinformers "k8s.io/client-go/informers"

	"podbuggertool/pkg/signals"
)

var (
	masterURL string
	kubeconfig string
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	stopCh := signals.SetupSignalHandler()

	// Here we start to play
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	// K8s API client
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building Kubernetes clientset: %s", err.Error())
	}

	pbtClient, err := pdtclientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building PodBuggerTool clientset: %s", err.Error())
	}

	/*kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	pbtInformerFactory := pdtinformers.NewSharedInformerFactory(pbtClient, time.Second*30)*/
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*5)
	pbtInformerFactory := pdtinformers.NewSharedInformerFactory(pbtClient, time.Second*5)

	controller := NewController(
		kubeClient, 
		pbtClient,
		kubeInformerFactory.Apps().V1().Deployments(),
		kubeInformerFactory.Core().V1().Pods(),
		pbtInformerFactory.Ualter().V1beta1().Podbuggertools())
	
	kubeInformerFactory.Start(stopCh)
	pbtInformerFactory.Start(stopCh)

	if err = controller.Run(2, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}