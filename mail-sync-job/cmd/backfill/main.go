package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"time"

	"com.lwc.palie/internal/db"
	"com.lwc.palie/internal/job"
	"com.lwc.palie/internal/logger"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Kill, os.Interrupt)
	defer cancelFunc()

	logger.SetTextAsDefault(slog.LevelDebug)

	ch := make(chan *job.Job, 1000)
	pgPool := getPgxConn(ctx)

	var wg sync.WaitGroup
	wg.Add(6)

	for idx := range 5 {
		jpdb := &JobProducerDB{
			Pool: pgPool,
		}
		id := fmt.Sprintf("Producer #%d", idx)
		go job.Produce(ctx, job.ProduceOptions{
			Ch:         ch,
			JobStorage: jpdb,
			Logger:     logger.GetLoggerWithTID(id),
			Timer: Timer{
				Duration: 2 * time.Second,
			},
			Id:              id,
			AvailablePeriod: 10 * time.Second,
			Wg:              &wg,
		})
	}
	go func() {
		defer wg.Done()
		l := logger.GetLoggerWithTID("Consumer")
	Outer:
		for {
			select {
			case <-ctx.Done():
				l.Info("Ctx done")
				break Outer
			case id := <-ch:
				l.Info("Got a job", "WhoClaimed", id.WhoClaimed, "WhatGotClaimed", id.GmailBackfillJobId)
			}
		}
	}()

	<-ctx.Done()

	wg.Wait()
	println("bye")
}

func getPgxConn(ctx context.Context) *pgxpool.Pool {
	url := os.Getenv("DB_URL")
	if url == "" {
		panic(errors.New("DB Url env var not set"))
	}

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		panic(err)
	}

	return pool
}

type Timer struct {
	Duration time.Duration
}

func (t Timer) C() <-chan time.Time {
	return time.After(t.Duration)
}

func (t Timer) Now() time.Time {
	return time.Now()
}

type JobProducerDB struct {
	mu sync.Mutex

	Pool    *pgxpool.Pool
	queries *db.Queries
	tx      pgx.Tx
}

func (gpdb *JobProducerDB) StartTx(ctx context.Context) error {
	gpdb.mu.Lock()
	defer gpdb.mu.Unlock()

	if gpdb.tx != nil {
		return errors.New("Already in TX")
	}

	tx, err := gpdb.Pool.Begin(ctx)
	if err != nil {
		return err
	}

	gpdb.tx = tx
	gpdb.queries = db.New(tx)

	return nil
}

func (gpdb *JobProducerDB) CommitTx(ctx context.Context) error {
	gpdb.mu.Lock()
	defer gpdb.mu.Unlock()

	if gpdb.tx == nil {
		return errors.New("Not in TX")
	}

	err := gpdb.tx.Commit(ctx)
	if err != nil {
		return err
	}
	gpdb.tx = nil
	gpdb.queries = nil

	return nil
}

func (gpdb *JobProducerDB) RollbackTx(ctx context.Context) error {
	gpdb.mu.Lock()
	defer gpdb.mu.Unlock()

	if gpdb.tx == nil {
		return nil
	}

	err := gpdb.tx.Rollback(ctx)
	if err != nil {
		return err
	}
	gpdb.tx = nil
	gpdb.queries = nil

	return nil
}

func (gpdb *JobProducerDB) GetAvailJobForUpdate(ctx context.Context) (db.GmailBackfillJob, error) {
	gpdb.mu.Lock()
	defer gpdb.mu.Unlock()

	if gpdb.queries == nil {
		return db.New(gpdb.Pool).GetAvailJobForUpdate(ctx)
	}

	return gpdb.queries.GetAvailJobForUpdate(ctx)
}

func (gpdb *JobProducerDB) HoldJob(ctx context.Context, arg db.HoldJobParams) (db.GmailBackfillJob, error) {
	gpdb.mu.Lock()
	defer gpdb.mu.Unlock()

	if gpdb.queries == nil {
		return db.New(gpdb.Pool).HoldJob(ctx, arg)
	}

	return gpdb.queries.HoldJob(ctx, arg)
}
