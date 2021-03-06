/*
Copyright 2017 Google Inc. All Rights Reserved.
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
	"log"
	"time"

	"go.uber.org/zap"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/golang/glog"
	"github.com/knative/build/pkg"
	"github.com/knative/build/pkg/builder"
	onclusterbuilder "github.com/knative/build/pkg/builder/cluster"
	gcb "github.com/knative/build/pkg/builder/google"
	buildclientset "github.com/knative/build/pkg/client/clientset/versioned"
	"github.com/knative/build/pkg/logging"
	"github.com/knative/build/pkg/signals"
	"github.com/knative/build/pkg/webhook"
)

var (
	builderName = flag.String("builder", "", "The builder implementation to use to execute builds (supports: cluster, google).")
)

func main() {
	flag.Parse()
	logger := logging.NewLoggerFromDefaultConfigMap("loglevel.webhook").Named("webhook")
	defer logger.Sync()

	logger.Info("Starting the Configuration Webhook")

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := rest.InClusterConfig()
	if err != nil {
		logger.Fatal("Failed to get in cluster config", zap.Error(err))
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logger.Fatal("Failed to get the client set", zap.Error(err))
	}

	buildClient, err := buildclientset.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("Error building Build clientset: %s", err.Error())
	}

	var bldr builder.Interface
	switch *builderName {
	case "cluster":
		kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
		bldr = onclusterbuilder.NewBuilder(kubeClient, kubeInformerFactory)
	case "google":
		bldr = gcb.NewBuilder(nil, "")
	default:
		glog.Fatalf("Unrecognized builder: %v (supported: google, cluster)", builderName)
	}

	options := webhook.ControllerOptions{
		ServiceName:      "build-webhook",
		ServiceNamespace: pkg.GetBuildSystemNamespace(),
		Port:             443,
		SecretName:       "build-webhook-certs",
		WebhookName:      "webhook.build.knative.dev",
	}
	webhook.NewAdmissionController(kubeClient, buildClient, bldr, options, logger).Run(stopCh)
}
