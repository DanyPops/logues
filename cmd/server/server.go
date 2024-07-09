package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/DanyPops/logues/domain/auth"
	"github.com/DanyPops/logues/domain/channel"
	"github.com/DanyPops/logues/domain/client"
	"github.com/DanyPops/logues/domain/connection"
)

type Server struct {
	http.Handler
	clientServer       *client.ClientServer
	authenticator      auth.Authenticator
	connectionUpgrader connection.ConnectionUpgrader
	channel            *channel.Channel
}

func New() *Server {
	l := new(Server)
	l.clientServer = client.NewClientServer()
	l.authenticator = auth.Authenticator{
		UserAuthenticator:  auth.EchoUserAuth{},
		TokenAuthenticator: auth.NewOTPRetentionMap(time.Second * 5),
	}
  l.connectionUpgrader = connection.NewGorillaUpgrader()
  l.channel = channel.NewDefaultChannel()
  go l.channel.Start()

	m := http.NewServeMux()
	m.HandleFunc("GET /", l.homeHandler)
	m.HandleFunc("POST /auth", l.authHandler)
	m.HandleFunc("GET /ws", l.wsHandler)
	l.Handler = m

	return l
}

func (s *Server) Serve() error {
  slog.Info("starting logues")
  return http.ListenAndServe(":7331", s.Handler)
}

func (s *Server) homeHandler(w http.ResponseWriter, r *http.Request) {
  http.ServeFile(w, r, "./../../home.html")
}

// func (s *Server) registrationHandler(w http.ResponseWriter, r *http.Request) {}

func (s *Server) authHandler(w http.ResponseWriter, r *http.Request) {
	var cred auth.Credentials
	json.NewDecoder(r.Body).Decode(&cred)

	user, err := s.authenticator.AuthenticateCredentials(cred)
	if err != nil {
		slog.Error("authentication failed: %s", err)
		return
	}
  
  slog.Info("requesting token for user","user", user)
	token, err := s.authenticator.NewToken(user)
	if err != nil {
		slog.Error("token generation failed: %s", err)
		return
	}
  
	if err := json.NewEncoder(w).Encode(token); err != nil {
		slog.Error("token encoding failed: %s", err)
		return
	}
  
	return
}

func (s *Server) wsHandler(w http.ResponseWriter, r *http.Request) {
	// TODO change AuthenticateToken to Authorize Middleware
	var token auth.Token
	if err := s.authenticator.NewDecoder(r).Decode(&token); err != nil {
		slog.Error("token decoding failed: %s", err)
		return
	}

	user, err := s.authenticator.AuthenticateToken(token)
	if err != nil {
		slog.Error("authentication failed: %s", err)
		return
	}
	// ODOT

	conn, err := s.connectionUpgrader.Upgrade(w, r)
	if err != nil {
		slog.Error("connection upgrade failed: %s", err)
		return
	}

	s.clientServer.ServeClient(conn, user, s.channel)
}

func main() {
  // ctx := context.Background()
	l := New()
  if err := l.Serve(); err != nil {
    slog.Error("failed to start server", err)
  }
	// http.ListenAndServe(":5000", l))
}