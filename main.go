/*
Copyright 2022.

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

package main

import (
	"flag"
	"os"

	"go.uber.org/zap/zapcore"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	configv1 "github.com/openshift/api/config/v1"
	aaov1alpha1 "github.com/openshift/aws-account-operator/api/v1alpha1"

	avov1alpha1 "github.com/openshift/aws-vpce-operator/api/v1alpha1"
	avov1alpha2 "github.com/openshift/aws-vpce-operator/api/v1alpha2"
	"github.com/openshift/aws-vpce-operator/controllers/util"
	"github.com/openshift/aws-vpce-operator/controllers/vpcendpoint"
	"github.com/openshift/aws-vpce-operator/controllers/vpcendpointacceptance"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	// Add config.openshift.io/v1 for the infrastructures CR
	utilruntime.Must(configv1.Install(scheme))

	// Add aws.managed.openshift.io/v1alpha1 for the AccountList CR
	utilruntime.Must(aaov1alpha1.AddToScheme(scheme))

	utilruntime.Must(avov1alpha1.AddToScheme(scheme))
	utilruntime.Must(avov1alpha2.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var (
		configFile string
		err        error
		trueBool   = true
		falseBool  = false
	)

	flag.StringVar(&configFile, "config", "",
		"The controller will load its initial configuration from this file. "+
			"Omit this flag to use the default configuration values.")

	opts := zap.Options{
		Development: false,
		TimeEncoder: zapcore.RFC3339TimeEncoder,
		// Remove misleading controller-runtime stack traces https://github.com/kubernetes-sigs/kubebuilder/issues/1593
		StacktraceLevel: zapcore.DPanicLevel,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Setup default options and override with values from the AVO ComponentConfig
	ctrlConfig := &avov1alpha1.AvoConfig{}
	options := ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     ":8080",
		HealthProbeBindAddress: ":8081",
		LeaderElection:         false,
	}

	if configFile != "" {
		options, err = options.AndFrom(ctrl.ConfigFile().AtPath(configFile).OfKind(ctrlConfig))
		if err != nil {
			setupLog.Error(err, "unable to load config file, continuing with defaults", "file", configFile)
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if ctrlConfig.EnableVpcEndpointController == nil {
		ctrlConfig.EnableVpcEndpointController = &trueBool
	}

	if *ctrlConfig.EnableVpcEndpointController {
		setupLog.Info("starting controller", "controller", vpcendpoint.ControllerName)
		if err = (&vpcendpoint.VpcEndpointReconciler{
			Client:   mgr.GetClient(),
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor(vpcendpoint.ControllerName),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", vpcendpoint.ControllerName)
			os.Exit(1)
		}
	}

	if ctrlConfig.EnableVpcEndpointAcceptanceController == nil {
		ctrlConfig.EnableVpcEndpointAcceptanceController = &falseBool
	}

	if *ctrlConfig.EnableVpcEndpointAcceptanceController {
		setupLog.Info("starting controller", "controller", "VpcEndpointAcceptance")
		if err = (&vpcendpointacceptance.VpcEndpointAcceptanceReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "VpcEndpointAcceptance")
			os.Exit(1)
		}
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", util.AWSEnvVarReadyzChecker); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
