package gmailx

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"

	"com.lwc.palie/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/people/v1"
)

type gmailStorage interface {
	CreateGmailBackfillJob(ctx context.Context, accountID pgtype.UUID) (db.GmailBackfillJob, error)
	CreateGoogleAccount(ctx context.Context, arg db.CreateGoogleAccountParams) (db.GoogleAccount, error)
	CreateGoogleOauthAccess(ctx context.Context, arg db.CreateGoogleOauthAccessParams) (db.GoogleOauthAccess, error)
}

type Me struct {
	Email       string
	DisplayName string
	Id          string
}

type InitGmailDataOptions struct {
	GmailStorage gmailStorage
	Me           *Me
	Token        *oauth2.Token
	Salt         string
}

func InitGmailData(ctx context.Context, ops InitGmailDataOptions) (db.GoogleAccount, error) {
	ga, err := ops.GmailStorage.CreateGoogleAccount(ctx, db.CreateGoogleAccountParams{
		GoogleID: ops.Me.Id,
		UserID: pgtype.UUID{
			Bytes: uuid.New(),
			Valid: true,
		},
	})
	if err != nil {
		return db.GoogleAccount{}, err
	}

	eak, err := encryptAK(ops.Token.AccessToken, ops.Salt)
	if err != nil {
		return db.GoogleAccount{}, err
	}

	erk, err := encryptAK(ops.Token.RefreshToken, ops.Salt)
	if err != nil {
		return db.GoogleAccount{}, err
	}

	_, err = ops.GmailStorage.CreateGoogleOauthAccess(ctx, db.CreateGoogleOauthAccessParams{
		AccessToken:  eak,
		RefreshToken: erk,
		AccessTokenExpiredAt: pgtype.Timestamptz{
			Time:  ops.Token.Expiry,
			Valid: true,
		},
		AccountID: ga.ID,
	})
	if err != nil {
		return db.GoogleAccount{}, err
	}

	_, err = ops.GmailStorage.CreateGmailBackfillJob(ctx, ga.ID)
	if err != nil {
		return db.GoogleAccount{}, err
	}

	return ga, nil
}

func GetMe(ctx context.Context, token *oauth2.Token, cfg *oauth2.Config) (*Me, error) {
	client := cfg.Client(ctx, token)
	peopleSrv, err := people.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	me, err := peopleSrv.People.Get("people/me").PersonFields("names,emailAddresses").Do()
	if err != nil {
		return nil, err
	}
	if me.EmailAddresses == nil || len(me.EmailAddresses) == 0 {
		return nil, errors.New("Invalid email addresses response from google")
	}
	if me.Names == nil || len(me.Names) == 0 {
		return nil, errors.New("Invalid names response from google")
	}

	var primaryEmail string
	var primaryId string
	for _, email := range me.EmailAddresses {
		if email.Metadata.Primary {
			primaryEmail = email.Value
			primaryId = email.Metadata.Source.Id
		}
	}

	var primaryName string
	for _, name := range me.Names {
		if name.Metadata.Primary {
			primaryName = name.DisplayName
		}
	}

	return &Me{
		DisplayName: primaryName,
		Email:       primaryEmail,
		Id:          primaryId,
	}, nil
}

func encryptAK(ak string, saltStr string) ([]byte, error) {
	salt := []byte(saltStr)
	if len(salt) != 32 {
		return nil, errors.New("Salt must be exactly 32 byte")
	}
	block, _ := aes.NewCipher(salt)
	gcm, _ := cipher.NewGCM(block)

	// Create unique nonce for every encryption
	nonce := make([]byte, gcm.NonceSize())
	io.ReadFull(rand.Reader, nonce)

	// Seal encrypts, authenticates, and prepends nonce
	return gcm.Seal(nonce, nonce, []byte(ak), nil), nil
}
