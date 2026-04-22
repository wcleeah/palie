package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"com.lwc.palie/internal/db"
	"com.lwc.palie/internal/gmailx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/people/v1"
)

type RanStrFactory struct {
}

func (r RanStrFactory) Ran() string {
	return uuid.NewString()
}

func main() {
	oauthCfg := getGoogleCfg()
	conn := getPgxConn(context.Background())

	pgQueries := db.New(conn)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /oauth", func(rw http.ResponseWriter, req *http.Request) {
		salt := os.Getenv("STATE_SALT")
		if salt == "" {
			rw.Write([]byte("Salt is not set"))
		}

		goa := gmailx.GoogleOauthOptions{
			Cfg:           oauthCfg,
			RanStrFactory: RanStrFactory{},
			AuthStorage:   pgQueries,
			Salt:          salt,
		}

		url, err := gmailx.GetGoogleOauthUrl(req.Context(), goa)
		if err != nil {
			rw.Write([]byte(err.Error()))
			rw.WriteHeader(422)
			return
		}

		rw.Header().Add("Location", url)
		rw.WriteHeader(302)
	})

	mux.HandleFunc("GET /oauth/callback", func(rw http.ResponseWriter, req *http.Request) {
		queries := req.URL.Query()

		stateSalt := os.Getenv("STATE_SALT")
		if stateSalt == "" {
			rw.Write([]byte("Salt is not set"))
			rw.WriteHeader(422)
			return
		}

		accessKeySalt := os.Getenv("ACCESS_KEY_SALT")
		if accessKeySalt == "" {
			rw.Write([]byte("Access Key Salt is not set"))
			rw.WriteHeader(422)
			return
		}

		goa := gmailx.GoogleOauthOptions{
			Cfg:           oauthCfg,
			RanStrFactory: RanStrFactory{},
			AuthStorage:   pgQueries,
			Salt:          stateSalt,
		}

		token, err := gmailx.GoogleOauthExchange(req.Context(), queries, goa)
		if err != nil {
			rw.Write([]byte(err.Error()))
			rw.WriteHeader(422)
			return
		}

		me, err := gmailx.GetMe(req.Context(), token, oauthCfg)
		if err != nil {
			println(err.Error())
			rw.Write([]byte(err.Error()))
			rw.WriteHeader(422)
			return
		}

		ga, err := gmailx.InitGmailData(req.Context(), gmailx.InitGmailDataOptions{
			GmailStorage: pgQueries,
			Token:        token,
			Me:           me,
			Salt:         accessKeySalt,
		})
		if err != nil {
			println(err.Error())
			rw.Write([]byte(err.Error()))
			rw.WriteHeader(422)
			return
		}

		rw.Write([]byte(fmt.Sprintf("%s: success", ga.DisplayName)))
		rw.WriteHeader(200)
	})

	err := http.ListenAndServe(":3000", mux)
	println(err.Error())
}

func getGoogleCfg() *oauth2.Config {
	path := os.Getenv("GOOGLE_CREDENTIAL_JSON_PATH")
	if path == "" {
		panic(errors.New("Credential Json Path env var not set"))
	}

	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	cfg, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope, people.UserinfoEmailScope, people.UserinfoProfileScope)
	if err != nil {
		panic(err)
	}

	return cfg
}

func getPgxConn(ctx context.Context) *pgx.Conn {
	url := os.Getenv("DB_URL")
	if url == "" {
		panic(errors.New("DB Url env var not set"))
	}

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		panic(err)
	}

	return conn
}
