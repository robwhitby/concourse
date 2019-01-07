package worker

import (
	"context"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
)

//go:generate counterfeiter . Client

type Client interface {
	FindOrCreateContainer(
		context.Context,
		lager.Logger,
		ImageFetchingDelegate,
		db.ContainerOwner,
		db.ContainerMetadata,
		ContainerSpec,
		WorkerSpec,
		creds.VersionedResourceTypes,
	) (Container, error)

	FindContainerByHandle(lager.Logger, int, string) (Container, bool, error)

	CreateVolume(lager.Logger, VolumeSpec, int, db.VolumeType) (Volume, error)
	LookupVolume(lager.Logger, string) (Volume, bool, error)

	FindResourceTypeByPath(path string) (atc.WorkerResourceType, bool)

	Satisfying(lager.Logger, WorkerSpec) (Worker, error)
}

//go:generate counterfeiter . InputSource

type InputSource interface {
	Source() ArtifactSource
	DestinationPath() string
}

//go:generate counterfeiter . BindMountSource

type BindMountSource interface {
	VolumeOn(Worker) (garden.BindMount, bool, error)
}

type VolumeSpec struct {
	Strategy   baggageclaim.Strategy
	Properties VolumeProperties
	Privileged bool
	TTL        time.Duration
}

func (spec VolumeSpec) baggageclaimVolumeSpec() baggageclaim.VolumeSpec {
	return baggageclaim.VolumeSpec{
		Strategy:   spec.Strategy,
		Privileged: spec.Privileged,
		Properties: baggageclaim.VolumeProperties(spec.Properties),
	}
}

//go:generate counterfeiter . Container

type Container interface {
	garden.Container

	Destroy() error

	VolumeMounts() []VolumeMount

	WorkerName() string

	MarkAsHijacked() error
}

type VolumeProperties map[string]string
