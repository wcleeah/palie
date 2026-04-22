package job

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"com.lwc.palie/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type jobStorage interface {
	StartTx(ctx context.Context) error
	CommitTx(ctx context.Context) error
	RollbackTx(ctx context.Context) error
	GetAvailJobForUpdate(ctx context.Context) (db.GmailBackfillJob, error)
	HoldJob(ctx context.Context, arg db.HoldJobParams) (db.GmailBackfillJob, error)
}

type timer interface {
	C() <-chan time.Time
	Now() time.Time
}

type ProduceOptions struct {
	Ch              chan<- *Job
	JobStorage      jobStorage
	Timer           timer
	AvailablePeriod time.Duration
	Logger          *slog.Logger
	Id              string
	Wg              *sync.WaitGroup
}

func Produce(ctx context.Context, ops ProduceOptions) {
	defer ops.Wg.Done()
	ops.Logger.Info("Started")
Outer:
	for {
		select {
		case <-ctx.Done():
			ops.Logger.Info("Ctx done")
			break Outer
		case <-ops.Timer.C():
			ops.Logger.Info("Times up")
		}
		err := _produce(ctx, ops)
		if err != nil {
			ops.Logger.Error(err.Error())
		}
	}
}

func _produce(ctx context.Context, ops ProduceOptions) error {
	err := ops.JobStorage.StartTx(ctx)
	defer ops.JobStorage.RollbackTx(ctx)
	if err != nil {
		return err
	}

	job, err := ops.JobStorage.GetAvailJobForUpdate(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ops.Logger.Info("No job available")
			return nil
		}

		return err
	}
	ops.Logger.Info("Got a job")

	grabbedJob, err := ops.JobStorage.HoldJob(ctx, db.HoldJobParams{
		ID: job.ID,
		AvailableAt: pgtype.Timestamptz{
			Time:  time.Now().Add(ops.AvailablePeriod),
			Valid: true,
		},
	})
	if err != nil {
		return err
	}

	ops.Logger.Info("Grab a job")
	ops.JobStorage.CommitTx(ctx)
	ops.Logger.Info("Committed")

	select {
	case ops.Ch <- &Job{GmailBackfillJobId: grabbedJob.ID.Bytes, WhoClaimed: ops.Id}:
		ops.Logger.Info("Passed to channel")
		return nil
	case <-ctx.Done():
		ops.Logger.Info("Ctx done")
		return ctx.Err()
	}
}
