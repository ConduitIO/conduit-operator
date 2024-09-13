package main

import (
	"flag"
	"os"

	"gopkg.in/yaml.v3"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	v1 "github.com/conduitio/conduit-operator/api/v1"
	"github.com/conduitio/conduit-operator/controllers"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var (
		metricsAddr, probeAddr string
		enableLeaderElection   bool
		metadataFile           string
	)
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&metadataFile, "instance-metadata", "", "Additional metadata for the conduit instances.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(
		zap.UseFlagOptions(
			timestampEncoderConfig(&opts),
		)),
	)

	meta, err := readMetadata(metadataFile)
	if err != nil {
		fatal(err, "failed to load conduit instance metadata", "file", metadataFile)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "61673ae0.conduit.io",
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: 9443,
		}),
	})
	if err != nil {
		fatal(err, "unable to start manager")
	}

	if err = (&controllers.ConduitReconciler{
		Metadata:      meta,
		Client:        mgr.GetClient(),
		Logger:        ctrl.Log.WithName("controller").WithName("conduit"),
		EventRecorder: mgr.GetEventRecorderFor("conduit-controller"),
	}).SetupWithManager(mgr); err != nil {
		fatal(err, "unable to create controller", "controller", "Conduit")
	}
	if err = (&v1.Conduit{}).SetupWebhookWithManager(mgr); err != nil {
		fatal(err, "unable to create webhook", "webhook", "Conduit")
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		fatal(err, "unable to set up health check")
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		fatal(err, "unable to set up ready check")
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		fatal(err, "problem running manager")
	}
}

func fatal(err error, msg string, kv ...any) {
	setupLog.Error(err, msg, kv...)
	os.Exit(1)
}

func readMetadata(file string) (*v1.ConduitInstanceMetadata, error) {
	c := v1.ConduitInstanceMetadata{
		PodAnnotations: make(map[string]string),
		Labels:         make(map[string]string),
	}

	_, err := os.Stat(file)
	switch {
	case os.IsNotExist(err):
		setupLog.Info("conduit instance metadata file missing, skipping.")
		return &c, nil
	case err != nil:
		return nil, err
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, err
	}

	setupLog.Info("conduit instance metadata loaded.")
	return &c, nil
}

// Changes the encoder time stamp key from the degault `ts` to `timestamp`
func timestampEncoderConfig(opts *zap.Options) *zap.Options {
	opts.EncoderConfigOptions = append(opts.EncoderConfigOptions, func(c *zapcore.EncoderConfig) {
		c.TimeKey = "timestamp"
	})
	return opts
}
