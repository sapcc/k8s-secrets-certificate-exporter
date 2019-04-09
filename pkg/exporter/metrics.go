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

package exporter

import (
	"fmt"
	"github.com/prometheus/common/log"
	"net"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	MetricNamespace      = "secrets_exporter"
	SubsystemCertificate = "certificate"
)

var (
	CertificateNotBefore = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: MetricNamespace,
			Subsystem: SubsystemCertificate,
			Name:      "not_before",
			Help:      "How long the certificate is valid.",
		},
		[]string{"host", "secret", "name"},
	)

	CertificateNotAfter = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: MetricNamespace,
			Subsystem: SubsystemCertificate,
			Name:      "not_after",
			Help:      "How long the certificate is valid.",
		},
		[]string{"host", "secret", "name"},
	)
)

func registerCollectors() {
	prometheus.MustRegister(
		CertificateNotBefore,
		CertificateNotAfter,
	)
}

func ExposeMetrics(host string, metricPort int, stopCh <-chan struct{}, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()

	registerCollectors()
	ln, err := net.Listen("tcp", fmt.Sprintf("%v:%v", host, metricPort))
	if err != nil {
		log.Infof("error exposing metrics: %v", err)
		return
	}

	log.Infof("exposing metrics on %s:%v", host, metricPort)
	go http.Serve(ln, promhttp.Handler())
	<-stopCh
}
