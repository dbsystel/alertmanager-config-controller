package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/dbsystel/alertmanager-config-controller/alertmanager"
	"github.com/dbsystel/alertmanager-config-controller/controller"
	"github.com/dbsystel/kube-controller-dbsystel-go-common/controller/configmap"
	"github.com/dbsystel/kube-controller-dbsystel-go-common/kubernetes"
	k8sflag "github.com/dbsystel/kube-controller-dbsystel-go-common/kubernetes/flag"
	opslog "github.com/dbsystel/kube-controller-dbsystel-go-common/log"
	logflag "github.com/dbsystel/kube-controller-dbsystel-go-common/log/flag"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	pclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	app = kingpin.New(filepath.Base(os.Args[0]), "Alertmanager Controller")
	//Here you can define more flags for your application
	configPath     = app.Flag("config-path", "The location to save rule and config files to").Required().String()
	configTemplate = app.Flag("config-template", "The template of alertmanager.yml").Required().String()
	id             = app.Flag("id", "The id of Alertmanager").Default("0").Int()
	key            = app.Flag("key", "The unique key for alertmanager config").String()
	reloadURL      = app.Flag("reload-url", "The url to issue requests to reload Alertmanager to").Required().String()
	addr           = app.Flag("listen-address", "The address to listen on for HTTP requests.").Default(":8080").String()
	namespace      = app.Flag("namespace", "The namespace to watching.").Default("").String()
)

var (
	configErrors = pclient.NewCounter(
		pclient.CounterOpts{
			Name: "alertmanager_controller_config_errors_total",
			Help: "Alertmanager Controller Errors Config Total",
		},
	)
)

func main() {
	//Define config for logging
	var logcfg opslog.Config
	//Definie if controller runs outside of k8s
	var runOutsideCluster bool
	//Add two additional flags to application for logging and decision if inside or outside k8s
	logflag.AddFlags(app, &logcfg)
	k8sflag.AddFlags(app, &runOutsideCluster)
	//Parse all arguments
	_, err := app.Parse(os.Args[1:])
	if err != nil {
		//Received error while parsing arguments from function app.Parse
		fmt.Fprintln(os.Stderr, "Catched the following error while parsing arguments: ", err)
		app.Usage(os.Args[1:])
		os.Exit(2)
	}
	//Initialize new logger from opslog
	logger, err := opslog.New(logcfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		app.Usage(os.Args[1:])
		os.Exit(2)
	}
	//First usage of initialized logger for testing
	//nolint:errcheck
	level.Debug(logger).Log("msg", "Logging initiated...")
	//Initialize new k8s client from common k8s package
	k8sClient, err := kubernetes.NewClientSet(runOutsideCluster)
	if err != nil {
		//nolint:errcheck
		level.Error(logger).Log("msg", err.Error())
		app.Usage(os.Args[1:])
		os.Exit(2)
	}

	URL, err := url.Parse(*reloadURL)
	if err != nil {
		//nolint:errcheck
		level.Error(logger).Log("msg", "Alertmanager reload URL could not be parsed: "+*reloadURL)
		os.Exit(2)
	}

	pclient.MustRegister(configErrors)
	a := alertmanager.New(URL, *configPath, *configTemplate, *id, *key, configErrors, logger)

	//nolint:errcheck
	level.Info(logger).Log("msg", "Starting Alertmanager Controller...")
	sigs := make(chan os.Signal, 1) // Create channel to receive OS signals
	stop := make(chan struct{})     // Create channel to receive stop signal

	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM, syscall.SIGINT) // Register the sigs channel to receieve SIGTERM

	wg := &sync.WaitGroup{} // Goroutines can add themselves to this to be waited on so that they finish

	//Initialize new k8s configmap-controller from common k8s package
	configMapController := &configmap.ConfigMapController{}
	configMapController.Controller = controller.New(*a, logger)
	configMapController.Initialize(k8sClient, *namespace)

	go startMetricsServer(logger)

	//Run initiated configmap-controller as go routine
	go configMapController.Run(stop, wg)

	<-sigs // Wait for signals (this hangs until a signal arrives)

	//nolint:errcheck
	level.Info(logger).Log("msg", "Shutting down...")

	close(stop) // Tell goroutines to stop themselves
	wg.Wait()   // Wait for all to be stopped
}

func startMetricsServer(logger log.Logger) {
	http.Handle("/metrics", promhttp.Handler())
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		level.Error(logger).Log("err", err.Error)
	}
}
