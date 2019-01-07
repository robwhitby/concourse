package worker

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
)

//go:generate counterfeiter . WorkerProvider

type WorkerProvider interface {
	RunningWorkers(lager.Logger) ([]Worker, error)

	FindWorkerForContainer(
		logger lager.Logger,
		teamID int,
		handle string,
	) (Worker, bool, error)

	FindWorkerForContainerByOwner(
		logger lager.Logger,
		teamID int,
		owner db.ContainerOwner,
	) (Worker, bool, error)

	NewGardenWorker(
		logger lager.Logger,
		tikTok clock.Clock,
		savedWorker db.Worker,
		numBuildWorkers int,
	) Worker
}

var (
	ErrNoWorkers       = errors.New("no workers")
	ErrNoGlobalWorkers = errors.New("no global workers available")
)

type NoCompatibleWorkersError struct {
	Spec WorkerSpec
}

func (err NoCompatibleWorkersError) Error() string {
	return fmt.Sprintf("no workers satisfying: %s", err.Spec.Description())
}

type pool struct {
	provider WorkerProvider

	rand     *rand.Rand
	strategy ContainerPlacementStrategy
}

func NewPool(provider WorkerProvider, strategy ContainerPlacementStrategy) Client {
	return &pool{
		provider: provider,
		rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
		strategy: strategy,
	}
}

func (pool *pool) allSatisfying(logger lager.Logger, spec WorkerSpec) ([]Worker, error) {
	workers, err := pool.provider.RunningWorkers(logger)
	if err != nil {
		return nil, err
	}

	if len(workers) == 0 {
		return nil, ErrNoWorkers
	}

	compatibleTeamWorkers := []Worker{}
	compatibleGeneralWorkers := []Worker{}
	for _, worker := range workers {
		satisfyingWorker, err := worker.Satisfying(logger, spec)
		if err == nil {
			if worker.IsOwnedByTeam() {
				compatibleTeamWorkers = append(compatibleTeamWorkers, satisfyingWorker)
			} else {
				compatibleGeneralWorkers = append(compatibleGeneralWorkers, satisfyingWorker)
			}
		}
	}

	if len(compatibleTeamWorkers) != 0 {
		return compatibleTeamWorkers, nil
	}

	if len(compatibleGeneralWorkers) != 0 {
		return compatibleGeneralWorkers, nil
	}

	if spec.TeamID == 0 {
		return nil, ErrNoGlobalWorkers
	}

	return nil, NoCompatibleWorkersError{
		Spec: spec,
	}
}

func (pool *pool) Satisfying(logger lager.Logger, spec WorkerSpec) (Worker, error) {
	compatibleWorkers, err := pool.allSatisfying(logger, spec)
	if err != nil {
		return nil, err
	}
	randomWorker := compatibleWorkers[pool.rand.Intn(len(compatibleWorkers))]
	return randomWorker, nil
}

func (pool *pool) FindOrCreateContainer(
	ctx context.Context,
	logger lager.Logger,
	delegate ImageFetchingDelegate,
	owner db.ContainerOwner,
	metadata db.ContainerMetadata,
	containerSpec ContainerSpec,
	workerSpec WorkerSpec,
	resourceTypes creds.VersionedResourceTypes,
) (Container, error) {
	worker, found, err := pool.provider.FindWorkerForContainerByOwner(
		logger.Session("find-worker"),
		workerSpec.TeamID,
		owner,
	)
	if err != nil {
		return nil, err
	}

	if !found {
		compatibleWorkers, err := pool.allSatisfying(logger, workerSpec)
		if err != nil {
			return nil, err
		}

		worker, err = pool.strategy.Choose(logger, compatibleWorkers, containerSpec)
		if err != nil {
			return nil, err
		}
	}

	return worker.FindOrCreateContainer(
		ctx,
		logger,
		delegate,
		owner,
		metadata,
		containerSpec,
		workerSpec,
		resourceTypes,
	)
}

func (pool *pool) FindContainerByHandle(logger lager.Logger, teamID int, handle string) (Container, bool, error) {
	worker, found, err := pool.provider.FindWorkerForContainer(
		logger.Session("find-worker"),
		teamID,
		handle,
	)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	return worker.FindContainerByHandle(logger, teamID, handle)
}

func (*pool) FindResourceTypeByPath(string) (atc.WorkerResourceType, bool) {
	return atc.WorkerResourceType{}, false
}

func (pool *pool) CreateVolume(logger lager.Logger, spec VolumeSpec, teamID int, volumeType db.VolumeType) (Volume, error) {
	worker, err := pool.Satisfying(logger, WorkerSpec{TeamID: teamID})
	if err != nil {
		return nil, err
	}

	return worker.CreateVolume(logger, spec, teamID, volumeType)
}

func (pool *pool) LookupVolume(logger lager.Logger, handle string) (Volume, bool, error) {
	workers, err := pool.allSatisfying(logger, WorkerSpec{})
	if err != nil {
		return nil, false, err
	}

	for _, worker := range workers {
		volume, found, err := worker.LookupVolume(logger, handle)
		if found {
			return volume, found, err
		}
	}

	return nil, false, nil
}
