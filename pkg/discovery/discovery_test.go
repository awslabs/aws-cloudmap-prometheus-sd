package discovery

import (
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/servicediscovery"
	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
)

var (
	targetSource1 = targetSourceSpec{
		namespace: "ns1",
		service:   "svc1",
	}
	targetSource2 = targetSourceSpec{
		namespace: "ns2",
		service:   "svc2",
	}
)

func TestProcessServiceInstances(t *testing.T) {
	d := sampleDiscovery()
	dio := &servicediscovery.DiscoverInstancesOutput{
		Instances: []*servicediscovery.HttpInstanceSummary{
			&servicediscovery.HttpInstanceSummary{
				InstanceId:    aws.String("192.168.0.1"),
				NamespaceName: aws.String(targetSource1.namespace),
				ServiceName:   aws.String(targetSource1.service),
				Attributes: map[string]*string{
					"AWS_INSTANCE_IPV4": aws.String("192.168.0.1"),
					"AWS_INSTANCE_PORT": aws.String("8080"),
					"version":           aws.String("v1"),
				},
			},
		},
	}
	tg := d.processServiceInstances(targetSource1, dio)
	assert.Equal(t, "192.168.0.1:8080", string(tg.Targets[0][model.AddressLabel]))
	assert.Equal(t, targetSource1.namespace, string(tg.Labels[lblNamespaceName]))
	assert.Equal(t, targetSource1.service, string(tg.Labels[lblServiceName]))
}

func TestProcessServiceInstances_WithEmptyList(t *testing.T) {
	d := sampleDiscovery()
	dio := &servicediscovery.DiscoverInstancesOutput{}
	tg := d.processServiceInstances(targetSource1, dio)
	assert.Empty(t, tg.Targets)
}

func TestCleanDeletedTargets_WithNoFailures(t *testing.T) {
	d := sampleDiscovery()
	d.oldSources[targetSource1] = true
	d.oldSources[targetSource2] = true
	d.newSources[targetSource1] = true
	d.newSources[targetSource2] = true
	tgs := d.cleanDeletedTargets()
	assert.Equal(t, d.newSources, d.oldSources)
	assert.Equal(t, 0, len(tgs))
}

func TestCleanDeletedTargets_WithFailedSources(t *testing.T) {
	d := sampleDiscovery()
	d.oldSources[targetSource1] = true
	d.oldSources[targetSource2] = true
	d.newSources[targetSource1] = true
	d.failedSources[targetSource2] = true
	tgs := d.cleanDeletedTargets()
	//oldSources is preserved due to failure
	assert.Equal(t, 2, len(d.oldSources))
	assert.Equal(t, 0, len(tgs))
}

func TestCleanDeletedTargets_WithFailedNamespaces(t *testing.T) {
	d := sampleDiscovery()
	d.oldSources[targetSource1] = true
	d.oldSources[targetSource2] = true
	d.newSources[targetSource1] = true
	d.failedNamespaces[targetSource2.namespace] = true
	tgs := d.cleanDeletedTargets()
	//oldSources is preserved due to failures
	assert.Equal(t, 2, len(d.oldSources))
	assert.Equal(t, 0, len(tgs))
}

func TestCleanDeletedTargets_WithDeletedTargets(t *testing.T) {
	d := sampleDiscovery()
	d.oldSources[targetSource1] = true
	d.oldSources[targetSource2] = true
	d.newSources[targetSource1] = true
	tgs := d.cleanDeletedTargets()
	//deleted targets are dropped from oldSources for subsequent runs
	assert.Equal(t, 1, len(d.oldSources))
	//deleted targets should trigger empty targets
	assert.Equal(t, 1, len(tgs))
}

func sampleDiscovery() *discovery {
	logger := log.NewSyncLogger(log.NewLogfmtLogger(os.Stdout))
	logger = log.With(logger, "ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller)
	return &discovery{
		logger:           logger,
		oldSources:       make(map[targetSourceSpec]bool),
		newSources:       make(map[targetSourceSpec]bool),
		failedSources:    make(map[targetSourceSpec]bool),
		failedNamespaces: make(map[string]bool),
	}
}
