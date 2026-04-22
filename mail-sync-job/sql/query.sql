--oauth_record 

-- name: GetOauthRecordByStateHash :one
SELECT *
FROM oauth_record
WHERE state = $1 AND provider = $2 AND completed_at IS NULL
LIMIT 1
;

-- name: CreateOauthRecord :one
INSERT INTO oauth_record (
    provider, state, pkce_verifier, scopes, redirect_url
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

-- name: CompleteOauthRecord :one
UPDATE oauth_record SET completed_at = $1 WHERE id = $2 RETURNING *;

-- googla_account

-- name: CreateGoogleAccount :one
INSERT INTO google_account (
    google_id, user_id, email, display_name
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- google_oauth_access

-- name: CreateGoogleOauthAccess :one
INSERT INTO google_oauth_access (
    access_token, refresh_token, access_token_expired_at, account_id
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: GetGoogleOauthAcessByAcctId :one
SELECT * FROM google_oauth_access WHERE account_id = $1;

-- gmail_backfill_job

-- name: GetGmailBackfillJobById :one
SELECT * FROM gmail_backfill_job WHERE id = $1 LIMIT 1;

-- name: CreateGmailBackfillJob :one
INSERT INTO gmail_backfill_job (
    account_id
) VALUES (
    $1
) RETURNING *;

-- name: GetAvailJobForUpdate :one
SELECT *
FROM gmail_backfill_job
WHERE status = 'queued' OR (status = 'grabbed' AND available_at <= now())
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE SKIP LOCKED;

-- name: GetHeldJob :one
SELECT *
FROM gmail_backfill_job
WHERE status = 'grabbed' AND claimed_by = $1 AND id = $2
LIMIT 1
FOR UPDATE NOWAIT;

-- name: HoldJob :one
UPDATE gmail_backfill_job SET
    status = 'grabbed', available_at = $1, claimed_by = $2 
WHERE id = $3 RETURNING *
;

-- name: ClaimJob :one
UPDATE gmail_backfill_job SET
    available_at = $1, claimed_by = $2, started_at = now()
WHERE id = $3 RETURNING *
;
