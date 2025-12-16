package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	drive "google.golang.org/api/drive/v3"
)

func main() {
	ctx := context.Background()

	clientID := mustEnv("GDRIVE_CLIENT_ID")
	clientSecret := mustEnv("GDRIVE_CLIENT_SECRET")

	// 1) Levanta un callback local en un puerto libre
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	redirectURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	conf := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{drive.DriveFileScope}, // <-- SOLO lo que necesitamos
		RedirectURL:  redirectURL,
	}

	state := randomState()

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != state {
			http.Error(w, "invalid state", http.StatusBadRequest)
			errCh <- fmt.Errorf("invalid state")
			return
		}
		if e := q.Get("error"); e != "" {
			http.Error(w, "auth error: "+e, http.StatusBadRequest)
			errCh <- fmt.Errorf("auth error: %s", e)
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			errCh <- fmt.Errorf("missing code")
			return
		}

		fmt.Fprintln(w, "OK. Ya puedes cerrar esta ventana y volver a la terminal.")
		codeCh <- code
	})

	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go func() {
		_ = srv.Serve(ln)
	}()

	// 2) Genera URL de autorización (offline => refresh token)
	authURL := conf.AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	)

	fmt.Println("\nAbre esta URL en tu navegador:\n")
	fmt.Println(authURL)
	fmt.Println("\nEsperando autorización en:", redirectURL)

	// 3) Espera code o error
	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		_ = srv.Close()
		log.Fatal(err)
	case <-time.After(3 * time.Minute):
		_ = srv.Close()
		log.Fatal("timeout esperando autorización")
	}

	_ = srv.Close()

	// 4) Intercambia code por tokens
	tok, err := conf.Exchange(ctx, code)
	if err != nil {
		panic(err)
	}

	// Nota: refresh token puede venir vacío si ya autorizaste antes sin prompt=consent.
	// Con prompt=consent normalmente se fuerza a entregarlo.
	if strings.TrimSpace(tok.RefreshToken) == "" {
		fmt.Println("\n⚠️ No llegó refresh_token.")
		fmt.Println("Solución: revoca acceso previo de la app en tu Google Account y vuelve a correr este comando.")
		fmt.Println("https://myaccount.google.com/permissions")
		return
	}

	fmt.Println("\n✅ REFRESH TOKEN:\n")
	fmt.Println(tok.RefreshToken)
}

func mustEnv(k string) string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		panic("missing env: " + k)
	}
	return v
}

func randomState() string {
	b := make([]byte, 18)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// (opcional) por si quieres depurar URLs
var _ = url.URL{}
