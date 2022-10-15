package discovery

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/servicediscovery"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
)

var (
	lblCloudMapPrefix = model.MetaLabelPrefix + "cloudmap_"
	lblNamespaceName  = model.LabelName(lblCloudMapPrefix + "namespace_name")
	lblServiceName    = model.LabelName(lblCloudMapPrefix + "service_name")
)

type targetSourceSpec struct {
	namespace string
	service   string
}

func (spec *targetSourceSpec) String() string {
	return fmt.Sprintf("%s/%s", spec.namespace, spec.service)
}

type SDConfig struct {
	Region            string
	RefreshInterval   int
	CloudmapNamespace *string
}

type discovery struct {
	logger            log.Logger
	aws               *aws.Config
	refreshInterval   time.Duration
	cloudmapNamespace *string
	oldSources        map[targetSourceSpec]bool
	newSources        map[targetSourceSpec]bool
	failedSources     map[targetSourceSpec]bool
	failedNamespaces  map[string]bool
}

func NewDiscovery(logger log.Logger, conf SDConfig) (*discovery, error) {
	d := &discovery{
		logger: logger,
		aws: &aws.Config{
			Region: &conf.Region,
		},
		refreshInterval:   time.Duration(conf.RefreshInterval) * time.Second,
		cloudmapNamespace: conf.CloudmapNamespace,
	}

	return d, nil
}

// Note: you must implement this function for your discovery implementation as part of the
// Discoverer interface. Here you should query your SD for it's list of known targets, determine
// which of those targets you care about and then send those targets as a target.TargetGroup to the ch channel.
func (d *discovery) Run(ctx context.Context, ch chan<- []*targetgroup.Group) {
	level.Info(d.logger).Log("msg", "Inside Run", "refreshInterval", d.refreshInterval)
	for c := time.Tick(d.refreshInterval); ; {
		tgs, err := d.refresh(ctx)
		if err != nil {
			level.Error(d.logger).Log("msg", "Error in refresh loop", "err", err)
		} else {
			ch <- tgs
		}
		// Wait for ticker or exit when ctx is closed.
		select {
		case <-c:
			continue
		case <-ctx.Done():
			return
		}
	}
}

func (d *discovery) refresh(ctx context.Context) ([]*targetgroup.Group, error) {
	level.Info(d.logger).Log("msg", "Inside refresh")
	sess, err := session.NewSessionWithOptions(session.Options{
		Config: *d.aws,
	})
	if err != nil {
		return nil, errors.Wrap(err, "could not create aws session")
	}

	cloudmap := servicediscovery.New(sess)
	tgs := []*targetgroup.Group{}
	d.newSources = make(map[targetSourceSpec]bool)

	lni := &servicediscovery.ListNamespacesInput{}

	if err = cloudmap.ListNamespacesPagesWithContext(ctx, lni, func(lno *servicediscovery.ListNamespacesOutput, lastPage bool) bool {
		for _, ns := range lno.Namespaces {
			if d.cloudmapNamespace != nil && aws.StringValue(ns.Name) != *d.cloudmapNamespace {
				continue
			}
			tgs = append(tgs, d.processNamespace(ctx, cloudmap, ns)...)
		}
		return false
	}); err != nil {
		return nil, errors.Wrap(err, "could not list namespaces")
	}

	tgs = append(tgs, d.cleanDeletedTargets()...)

	level.Info(d.logger).Log("msg", "Refresh done", "target-group-count", len(tgs))
	return tgs, nil
}

func (d *discovery) cleanDeletedTargets() []*targetgroup.Group {
	tgs := []*targetgroup.Group{}
	backfillSources := make(map[targetSourceSpec]bool)
	for k, _ := range d.oldSources {
		if _, ok := d.newSources[k]; !ok {
			//First check if there is failure syncing service or namespace corresponding to this source
			if _, ok := d.failedNamespaces[k.namespace]; ok {
				level.Info(d.logger).Log("msg", "Skipping cleanup because failure syncing namespace", "namespace", k.String())
				backfillSources[k] = true
				continue
			}
			if _, ok := d.failedSources[k]; ok {
				level.Info(d.logger).Log("msg", "Skipping cleanup because failure syncing source", "source", k.String())
				backfillSources[k] = true
				continue
			}
			level.Info(d.logger).Log("msg", "Cleaning up empty source", "source", k.String())
			tgs = append(tgs, &targetgroup.Group{Source: k.String()})
		}
	}
	d.oldSources = d.newSources
	//backfill oldSources with the failed ones
	for k, v := range backfillSources {
		d.oldSources[k] = v
	}

	return tgs
}

func (d *discovery) processNamespace(ctx context.Context, cloudmap *servicediscovery.ServiceDiscovery, ns *servicediscovery.NamespaceSummary) []*targetgroup.Group {
	tgs := []*targetgroup.Group{}
	lsi := &servicediscovery.ListServicesInput{
		Filters: []*servicediscovery.ServiceFilter{
			&servicediscovery.ServiceFilter{
				Name: aws.String(servicediscovery.ServiceFilterNameNamespaceId),
				Values: []*string{
					ns.Id,
				},
			},
		},
	}

	if err := cloudmap.ListServicesPagesWithContext(ctx, lsi, func(lso *servicediscovery.ListServicesOutput, lastPage bool) bool {
		for _, s := range lso.Services {
			level.Info(d.logger).Log("msg", "Processing", "service", s.Name, "namespace", ns.Name)
			tg := d.processService(ctx, cloudmap, ns, s)
			if tg != nil {
				tgs = append(tgs, tg)
			}
			level.Info(d.logger).Log("msg", "Processed", "service", s.Name, "namespace", ns.Name)
		}
		return false
	}); err != nil {
		level.Error(d.logger).Log("msg", "Error listing services", "namespace", ns.Name)
		d.failedNamespaces[aws.StringValue(ns.Name)] = true
	}

	level.Info(d.logger).Log("namespace", ns.Name, "target-group-count", len(tgs))
	return tgs
}

func (d *discovery) processService(ctx context.Context, cloudmap *servicediscovery.ServiceDiscovery, ns *servicediscovery.NamespaceSummary, s *servicediscovery.ServiceSummary) *targetgroup.Group {
	tgSourceSpec := targetSourceSpec{
		namespace: aws.StringValue(ns.Name),
		service:   aws.StringValue(s.Name),
	}
	discoveryInput := &servicediscovery.DiscoverInstancesInput{
		NamespaceName: ns.Name,
		ServiceName:   s.Name,
	}
	dio, err := cloudmap.DiscoverInstancesWithContext(ctx, discoveryInput)
	if err != nil {
		level.Error(d.logger).Log("msg", "Error calling DiscoverInstances",
			"service", s.Name, "namespace", ns.Name)

		d.failedSources[tgSourceSpec] = true
		return nil
	}

	return d.processServiceInstances(tgSourceSpec, dio)
}

func (d *discovery) processServiceInstances(tgSourceSpec targetSourceSpec, dio *servicediscovery.DiscoverInstancesOutput) *targetgroup.Group {
	d.newSources[tgSourceSpec] = true
	tg := &targetgroup.Group{
		Source: tgSourceSpec.String(),
		Labels: model.LabelSet{
			lblNamespaceName: model.LabelValue(tgSourceSpec.namespace),
			lblServiceName:   model.LabelValue(tgSourceSpec.service),
		},
		Targets: make([]model.LabelSet, 0, len(dio.Instances)),
	}
	for _, inst := range dio.Instances {
		instanceID := aws.StringValue(inst.InstanceId)
		ipv4 := aws.StringValue(inst.Attributes["AWS_INSTANCE_IPV4"])
		ipv6 := aws.StringValue(inst.Attributes["AWS_INSTANCE_IPV6"])
		var ipAddr string
		if ipv4 != "" {
			ipAddr = ipv4
		} else if ipv6 != "" {
			ipAddr = ipv6
		} else {
			level.Info(d.logger).Log("msg", "skipping instance with no ip", "instance", instanceID)
			continue
		}

		port := aws.StringValue(inst.Attributes["AWS_INSTANCE_PORT"])
		if port != "" {
			ipAddr = net.JoinHostPort(ipAddr, port)
		}
		labels := model.LabelSet{
			model.AddressLabel: model.LabelValue(ipAddr),
		}
		tg.Targets = append(tg.Targets, labels)
	}
	return tg
}
