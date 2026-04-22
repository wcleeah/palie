--oauth_record 

-- name: GetOauthRecordByStateHash :one
SELECT * FROM oauth_record WHERE state = $1 AND provider = $2 AND completed_at IS NULL LIMIT 1;

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

-- gmail_backfill_job

-- name: CreateGmailBackfillJob :one
INSERT INTO gmail_backfill_job (
    account_id
) VALUES (
    $1
) RETURNING *;

