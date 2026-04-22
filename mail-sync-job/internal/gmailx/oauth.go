package gmailx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"time"

	"com.lwc.palie/internal/assert"
	"com.lwc.palie/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/oauth2"
)

type oauthOpts struct {
	State string
}

type authStorage interface {
	CreateOauthRecord(context.Context, db.CreateOauthRecordParams) (db.OauthRecord, error)
	GetOauthRecordByStateHash(context.Context, db.GetOauthRecordByStateHashParams) (db.OauthRecord, error)
	CompleteOauthRecord(context.Context, db.CompleteOauthRecordParams) (db.OauthRecord, error)
}

type stateFactory interface {
	Ran() string
}

type GoogleOauthOptions struct {
	Cfg           *oauth2.Config
	AuthStorage   authStorage
	RanStrFactory stateFactory
	Salt          string
}

func GetGoogleOauthUrl(ctx context.Context, goa GoogleOauthOptions) (string, error) {
	assert.AssertNotNil(goa.Cfg, "Oauth Config is nil")
	assert.AssertNotNil(goa.AuthStorage, "Auth Storage is nil")
	assert.AssertNotNil(goa.RanStrFactory, "State Factory is nil")

	state := goa.RanStrFactory.Ran()
	verivier := goa.RanStrFactory.Ran()

	url := goa.Cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verivier))

	_, err := goa.AuthStorage.CreateOauthRecord(ctx, db.CreateOauthRecordParams{
		Provider:     "google",
		State:        hashState(state, goa.Salt),
		PkceVerifier: verivier,
		Scopes:       goa.Cfg.Scopes,
		RedirectUrl:  goa.Cfg.RedirectURL,
	})
	if err != nil {
		return "", err
	}

	return url, nil
}

func GoogleOauthExchange(ctx context.Context, queries url.Values, goa GoogleOauthOptions) (*oauth2.Token, error) {
	assert.AssertNotNil(goa.Cfg, "Oauth Config is nil")
	assert.AssertNotNil(goa.AuthStorage, "Auth Storage is nil")
	assert.AssertNotNil(goa.RanStrFactory, "State Factory is nil")

	if !queries.Has("state") {
		return nil, errors.New("State is missing")
	}
	if !queries.Has("code") {
		return nil, errors.New("Code is missing")
	}

	state := queries.Get("state")
	code := queries.Get("code")

	auth, err := goa.AuthStorage.GetOauthRecordByStateHash(ctx, db.GetOauthRecordByStateHashParams{
		Provider: "google",
		State:    hashState(state, goa.Salt),
	})
	if err != nil {
		return nil, errors.Join(errors.New("State invalid"), err)
	}

	token, err := goa.Cfg.Exchange(ctx, code, oauth2.VerifierOption(auth.PkceVerifier))
	if err != nil {
		return nil, err
	}

	_, err = goa.AuthStorage.CompleteOauthRecord(ctx, db.CompleteOauthRecordParams{
		CompletedAt: pgtype.Timestamptz{
			Time: time.Now().UTC(),
			Valid: true,
		},
		ID: auth.ID,
	})
	if err != nil {
		return nil, err
	}

	return token, nil
}

func hashState(state string, salt string) string {
	assert.AssertNotEmptyStr(salt, "State salt is empty str")

	hasher := sha256.New()
	hasher.Write([]byte(state + salt))
	return hex.EncodeToString(hasher.Sum(nil))
}
