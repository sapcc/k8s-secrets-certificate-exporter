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
	"log"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	v1Informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type exporter struct {
	opts           Options
	secretInformer cache.SharedIndexInformer
	queue          workqueue.RateLimitingInterface
}

func New(opts Options) *exporter {
	e := &exporter{
		opts:  opts,
		queue: workqueue.NewRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(30*time.Second, 600*time.Second)),
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

func (e *exporter) Run(threadiness int, stopCh <-chan struct{}, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()

	log.Println("starting new secrets exporter")

	log.Println("waiting for cache to sync..")
	go e.secretInformer.Run(stopCh)
	cache.WaitForCacheSync(stopCh, e.secretInformer.HasSynced)

	for i := 0; i < threadiness; i++ {
		go wait.Until(e.runWorker, time.Second, stopCh)
	}

	ticker := time.NewTicker(e.opts.RecheckInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				log.Printf("checking every %s", e.opts.RecheckInterval.String())
				e.requeueAllSecrets()
			case <-stopCh:
				ticker.Stop()
				return
			}
		}
	}()

	<-stopCh
}

func (e *exporter) runWorker() {
	for e.processNextWorkItem() {
	}
}

func (e *exporter) processNextWorkItem() bool {
	key, quit := e.queue.Get()
	if quit {
		return false
	}
	defer e.queue.Done(key)

	if err := e.syncHandler(key.(string)); err != nil {
		e.queue.Forget(key)
		return true
	}

	e.queue.Add(key)
	return true
}

func (e *exporter) requeueAllSecrets() {
	log.Printf("found %v secrets",len(e.secretInformer.GetStore().ListKeys()))

	for _, o := range e.secretInformer.GetStore().List() {
		key, err := cache.MetaNamespaceKeyFunc(o)
		if err != nil {
			log.Printf("error adding secret: %v", err)
			return
		}
		e.queue.Add(key)
	}
}

func (e *exporter) syncHandler(key string) error {
	o, exists, err := e.secretInformer.GetStore().GetByKey(key)
	if err != nil {
		return err
	}

	if !exists {
		return errors.Errorf("secrets does not exist. key=%s", key)
	}

	secret := o.(*v1.Secret)

	for secretKey, secretData := range secret.Data {
		cert, err := loadCertificate(secretData)
		if err != nil {
			// This might just not be a certificate.
			// log.Printf("unable to load certificate from secret %s, key %s", key, secretKey)
			continue
		}

		labels := prometheus.Labels{
			"host":   strings.Join(cert.DNSNames, ", "),
			"secret": key,
			"name":   secretKey,
		}

		CertificateNotBefore.With(labels).Set(float64(cert.NotBefore.Unix()))
		CertificateNotAfter.With(labels).Set(float64(cert.NotAfter.Unix()))
	}
	return nil
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
