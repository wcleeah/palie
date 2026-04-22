package job

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"com.lwc.palie/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

type consumeStorage interface {
	GetHeldJob(ctx context.Context, ops db.GetHeldJobParams) (db.GmailBackfillJob, error)
	ClaimJob(ctx context.Context, ops db.ClaimJobParams) (db.GmailBackfillJob, error)
	GetGoogleOauthAcessByAcctId(ctx context.Context, accountID pgtype.UUID) (db.GoogleOauthAccess, error)
	StartTx(ctx context.Context) error
	CommitTx(ctx context.Context) error
	RollbackTx(ctx context.Context) error
}

type Job struct {
	GmailBackfillJobId [16]byte
	WhoClaimed         string
}

type ConsumeOption struct {
	Wg              *sync.WaitGroup
	Ch              <-chan *Job
	Logger          *slog.Logger
	AvailablePeriod time.Duration
	ConsumeStorage  consumeStorage
	ID              string
}

func Consume(ctx context.Context, ops ConsumeOption) {
	defer ops.Wg.Done()

Outer:
	for {
		var job *Job
		select {
		case <-ctx.Done():
			ops.Logger.Info("Ctx done")
			break Outer
		case job = <-ops.Ch:
			ops.Logger.Info("Got a job")
		}

		// check is job really available
		// claim the job
		// extend available at to a longer period
		gbj, err := claimJob(ctx, job, ops)
		if err != nil {
			ops.Logger.Error("Claim Job error", "err", err)
			continue
		}

		_, err = ops.ConsumeStorage.GetGoogleOauthAcessByAcctId(ctx, gbj.ID)

		// get labels
		// get threads -> all child messages
		// insert
	}
}

func claimJob(ctx context.Context, job *Job, ops ConsumeOption) (db.GmailBackfillJob, error) {
	err := ops.ConsumeStorage.StartTx(ctx)
	if err != nil {
		return db.GmailBackfillJob{}, err
	}

	defer ops.ConsumeStorage.RollbackTx(ctx)

	gbj, err := ops.ConsumeStorage.GetHeldJob(ctx, db.GetHeldJobParams{
		ClaimedBy: pgtype.Text{
			String: job.WhoClaimed,
			Valid:  true,
		},
		ID: pgtype.UUID{
			Bytes: job.GmailBackfillJobId,
			Valid: true,
		},
	})
	if err != nil {
		return db.GmailBackfillJob{}, err
	}

	updatedGbj, err := ops.ConsumeStorage.ClaimJob(ctx, db.ClaimJobParams{
		ClaimedBy: pgtype.Text{
			String: ops.ID,
			Valid:  true,
		},
		ID: gbj.ID,
		AvailableAt: pgtype.Timestamptz{
			Time:  time.Now().Add(ops.AvailablePeriod),
			Valid: true,
		},
	})

	ops.ConsumeStorage.CommitTx(ctx)
	return updatedGbj, nil
}
