/*******************************************************************************
*
* Copyright 2019 SAP SE
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You should have received a copy of the License along with this
* program. If not, you may obtain a copy of the License at
*
*     http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*
*******************************************************************************/

package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sapcc/k8s-secrets-certificate-exporter/pkg/exporter"
	"github.com/spf13/pflag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var opts exporter.Options

func init() {
	pflag.IntVar(&opts.MetricPort, "metric-port", 9091, "Port for Prometheus metrics.")
	pflag.IntVar(&opts.Threadiness, "threadiness", 1, "Exporter threadiness.")
	pflag.DurationVar(&opts.ResyncInterval, "resync-interval", 15*time.Minute, "Interval to resync secrets.")
	pflag.DurationVar(&opts.RecheckInterval, "recheck-interval", 30*time.Minute, "Interval to check secrets.")
	pflag.StringVar(&opts.Namespace, "namespace", "", "Limit exporter to this namespace.")
	pflag.StringVar(&opts.KubeConfigPath, "kubeconfig", "", "Path to kube config (optional).")
	pflag.StringVar(&opts.Host, "host", "0.0.0.0", "Host to bind to.")
}

func main() {
	pflag.Parse()
	log.SetOutput(os.Stdout)

	sigs := make(chan os.Signal, 1)
	stop := make(chan struct{})
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM) // Push signals into channel
	wg := &sync.WaitGroup{}                            // Goroutines can add themselves to this to be waited on

	e := exporter.New(opts)

	go e.Run(opts.Threadiness, stop, wg)
	go e.Serve(opts, stop, wg)

	prometheus.MustRegister(e)

	<-sigs
	close(stop)
	wg.Wait()
}
