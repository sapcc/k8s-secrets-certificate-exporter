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
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	v1 "k8s.io/api/core/v1"
	v1Informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type exporter struct {
	opts           Options
	secretInformer cache.SharedIndexInformer
	queue          workqueue.RateLimitingInterface

	notBefore,
	notAfter *prometheus.Desc
}

func New(opts Options) *exporter {
	e := &exporter{
		opts:  opts,
		queue: workqueue.NewRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(30*time.Second, 600*time.Second)),

		notBefore: prometheus.NewDesc(
			"secrets_exporter_certificate_not_before",
			"Certificate is not valid before.",
			[]string{"host", "secret", "key"},
			prometheus.Labels{},
		),
		notAfter: prometheus.NewDesc(
			"secrets_exporter_certificate_not_after",
			"Certificate is not valid after.",
			[]string{"host", "secret", "key"},
			prometheus.Labels{},
		),
	}

	clientset, err := newKubernetesClientSet(opts)
	if err != nil {
		log.Fatalf("error creating kubernetes clientset: %v", err)
	}

	secretInformer := v1Informer.NewSecretInformer(clientset, opts.Namespace, opts.ResyncInterval, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    e.secretAdd,
		UpdateFunc: e.secretUpdate,
	})
	e.secretInformer = secretInformer

	return e
}

// Describe implements the Prometheus Describe interface.
func (e *exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.notBefore
	ch <- e.notAfter
}

// Collect implements the Prometheus Collect interface.
func (e *exporter) Collect(ch chan<- prometheus.Metric) {
	for _, secretKey := range e.secretInformer.GetStore().ListKeys() {
		o, exists, err := e.secretInformer.GetStore().GetByKey(secretKey)
		if err != nil {
			log.Printf("error getting secret %s: %v", secretKey, err)
			continue
		}
		if !exists {
			log.Printf("secret %s does not exist", secretKey)
			continue
		}

		secret := o.(*v1.Secret)
		for keyData, v := range secret.Data {
			cert, err := loadCertificate(v)
			if err != nil {
				// This might just not be a certificate.
				// log.Printf("error loading data from secret %s, key %s: %v", key, k, err)
				continue
			}

			sans := strings.Join(cert.DNSNames, ",")

			ch <- prometheus.MustNewConstMetric(e.notBefore, prometheus.GaugeValue, float64(cert.NotBefore.UTC().Unix()), sans, secretKey, keyData)
			ch <- prometheus.MustNewConstMetric(e.notAfter, prometheus.GaugeValue, float64(cert.NotAfter.UTC().Unix()), sans, secretKey, keyData)
		}
	}
}

func (e *exporter) Run(threadiness int, stopCh <-chan struct{}, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()

	log.Println("starting new secrets exporter")

	log.Println("waiting for cache to sync secrets")
	go e.secretInformer.Run(stopCh)
	if !cache.WaitForCacheSync(stopCh, e.secretInformer.HasSynced) {
		log.Fatalln("timeout while waiting for cache to sync")
	}

	<-stopCh
}

func (e *exporter) Serve(opts Options, stopCh <-chan struct{}, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()

	ln, err := net.Listen("tcp", fmt.Sprintf("%v:%v", opts.Host, opts.MetricPort))
	if err != nil {
		log.Printf("error exposing metrics: %v", err)
		return
	}

	log.Printf("exposing metrics on %s:%v", opts.Host, opts.MetricPort)
	go http.Serve(ln, promhttp.Handler())
	<-stopCh
}

func (e *exporter) secretAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Printf("error adding secret: %v", err)
		return
	}
	e.queue.AddRateLimited(key)
}

func (e *exporter) secretUpdate(oldObj, newObj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(newObj)
	if err != nil {
		log.Printf("error adding secret: %v", err)
		return
	}
	e.queue.AddRateLimited(key)
}

func loadCertificate(data []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("failed to parse certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse certificate")
	}
	return cert, nil
}
