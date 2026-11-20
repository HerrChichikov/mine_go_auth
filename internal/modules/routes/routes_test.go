package routes_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"alpha/constants"
	"alpha/internal/handlers"
	"alpha/internal/modules"
	"alpha/internal/modules/logger"
	"alpha/internal/modules/routes"
	"alpha/internal/repository"
	"alpha/internal/testsupport"
	"alpha/internal/usecase"
	"alpha/yggdrasil"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresAccountAndGameSessions(t *testing.T) {
	dsn := os.Getenv("MINECRAFT_AUTH_TEST_DATABASE")
	if dsn == "" {
		t.Skip("MINECRAFT_AUTH_TEST_DATABASE points to a disposable PostgreSQL database")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin := stdlib.OpenDB(*config)
	defer admin.Close()
	schema := "auth_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec("DROP SCHEMA " + schema + " CASCADE")
	config.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*config)
	defer db.Close()
	logger.LoggerInit("test")
	if err := modules.ApplyMigrations(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	seed(t, db)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := yggdrasil.NewSigner(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), []string{"skins.example.test"})
	if err != nil {
		t.Fatal(err)
	}
	attempts := repository.NewAttemptRepository(db)
	users, err := usecase.NewUserUseCase(repository.NewUserRepository(db), repository.NewSessionRepository(db), attempts)
	if err != nil {
		t.Fatal(err)
	}
	game := usecase.NewGameUseCase(users, repository.NewGameRepository(db), attempts, signer)
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	const provision = "test-provision-key-for-isolated-fixture"
	routes.SetupRoutes(app, handlers.NewAuthHandler(users, game, attempts), users, game, signer, provision)
	call := func(method, path, bearer string, body any, status int) []byte {
		t.Helper()
		var data []byte
		if body != nil {
			var err error
			data, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		res, err := app.Test(req, int((15 * time.Second).Milliseconds()))
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		result, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != status {
			t.Fatalf("%s %s: got HTTP %d, want %d; %s", method, path, res.StatusCode, status, result)
		}
		return result
	}
	read := func(data []byte, v any) {
		t.Helper()
		if err := json.Unmarshal(data, v); err != nil {
			t.Fatal(err)
		}
	}
	const password = "example-password"
	const api = "/api/auth"
	const root = "/yggdrasil/survival"
	const mc = root + "/sessionserver/session/minecraft"
	body := func(pairs ...string) map[string]string {
		m := make(map[string]string)
		for i := 0; i < len(pairs); i += 2 {
			m[pairs[i]] = pairs[i+1]
		}
		return m
	}
	register := body("nickname", "Alice", "password", password, "invitation_code", "INVITE1")
	var user struct {
		User struct {
			ID       uuid.UUID
			Nickname string
			Email    string
		}
	}
	read(call("POST", api+"/register", "", register, 201), &user)
	if user.User.ID == uuid.Nil || user.User.Nickname != "Alice" || user.User.Email != "" {
		t.Fatal("unexpected registered user")
	}
	call("POST", api+"/register", "", body("nickname", "Alice", "password", password, "invitation_code", "INVITE2"), 400)
	call("POST", api+"/register", "", body("nickname", "Bob", "password", "short", "invitation_code", "INVITE2"), 400)
	var bob struct{ User struct{ ID uuid.UUID } }
	read(call("POST", api+"/register", "", body("nickname", "Bob", "password", password, "invitation_code", "INVITE2"), 201), &bob)
	call("POST", api+"/login", "", body("login", "Alice", "password", "wrong", "client", "launcher"), 401)
	login := func(name string) usecase.UserTokens {
		var pair usecase.UserTokens
		read(call("POST", api+"/login", "", body("login", name, "password", password, "client", "launcher"), 200), &pair)
		return pair
	}
	pair := login("Alice")
	call("GET", api+"/profile", pair.Access, nil, 200)
	call("GET", api+"/profile", "bogus", nil, 401)
	profileReq := body("userId", user.User.ID.String(), "realm", "survival", "nickname", "AliceGame")
	call("POST", "/internal/game-profiles", pair.Access, profileReq, 401)
	var profile, retry, other repository.GameProfile
	read(call("POST", "/internal/game-profiles", provision, profileReq, 200), &profile)
	profileReq["nickname"] = "ChangedNick"
	read(call("POST", "/internal/game-profiles", provision, profileReq, 200), &retry)
	if retry.ID != profile.ID || retry.Nickname != "AliceGame" {
		t.Fatal("retry changed permanent nickname")
	}
	read(call("POST", "/internal/game-profiles", provision, body("userId", bob.User.ID.String(), "realm", "survival", "nickname", "BobGame"), 200), &other)
	call("POST", "/internal/game-profiles", provision, body("userId", user.User.ID.String(), "realm", "creative", "nickname", "AliceGame"), 200)
	call("GET", api+"/game-profiles", pair.Access, nil, 200)
	call("PUT", api+"/game-profiles/"+profile.ID.String()+"/skin", pair.Access, map[string]any{"url": "https://skins.example.test/4d186321c1a7f0f354b297e8914ab2403e3f7abfe2d57c1a787fdc9dbb0bd029", "slim": true}, 200)
	call("PUT", api+"/game-profiles/"+other.ID.String()+"/skin", pair.Access, body("url", "https://skins.example.test/4d186321c1a7f0f354b297e8914ab2403e3f7abfe2d57c1a787fdc9dbb0bd029"), 404)
	call("POST", api+"/game-token", pair.Access, body("profileId", other.ID.String()), 404)
	var launch usecase.LaunchSession
	read(call("POST", api+"/game-token", pair.Access, body("profileId", profile.ID.String()), 201), &launch)
	if launch.InjectorPath != "/yggdrasil/survival/" || launch.Profile.ID != profile.ID {
		t.Fatal("wrong launch identity")
	}
	call("GET", api+"/profile", launch.AccessToken, nil, 401)
	id := profile.Wire().ID
	join := body("accessToken", launch.AccessToken, "selectedProfile", id, "serverId", "-fixture")
	call("POST", mc+"/join", "", body("accessToken", pair.Access, "selectedProfile", id, "serverId", "-fixture"), 403)
	call("POST", mc+"/join", "", body("accessToken", launch.AccessToken, "selectedProfile", other.Wire().ID, "serverId", "-fixture"), 403)
	call("POST", strings.Replace(mc, "survival", "creative", 1)+"/join", "", join, 403)
	call("POST", mc+"/join", "", join, 204)
	path := mc + "/hasJoined?username=AliceGame&serverId=-fixture"
	var signed yggdrasil.DetailedProfile
	read(call("GET", path, "", nil, 200), &signed)
	if signed.ID != id || len(signed.Properties) != 1 || signed.Properties[0].Signature == "" {
		t.Fatal("missing signed profile")
	}
	call("GET", mc+"/profile/"+id+"?unsigned=false", "", nil, 200)
	call("GET", strings.Replace(path, "survival", "creative", 1), "", nil, 204)
	call("GET", path+"&ip=192.0.2.1", "", nil, 204)
	call("GET", strings.Replace(path, "-fixture", "-unrelated", 1), "", nil, 204)
	var names []yggdrasil.Profile
	read(call("POST", root+"/api/profiles/minecraft", "", []string{"AliceGame"}, 200), &names)
	if len(names) != 1 || names[0].ID != id {
		t.Fatal("unexpected profile lookup")
	}

	t.Run("pinned Injector with Postgres", func(t *testing.T) {
		if os.Getenv("MINECRAFT_AUTH_SMOKE_JAVA") == "" && os.Getenv("MINECRAFT_AUTH_SMOKE_AGENT") == "" &&
			os.Getenv("MINECRAFT_AUTH_SMOKE_CLASSPATH") == "" {
			t.Skip("injector smoke fixture paths are not configured")
		}
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		served := make(chan error, 1)
		go func() { served <- app.Listener(listener) }()
		t.Cleanup(func() {
			if err := app.ShutdownWithTimeout(3 * time.Second); err != nil {
				t.Error(err)
			}
			if err := <-served; err != nil {
				t.Error(err)
			}
		})
		testsupport.VerifyInjector(t, "http://"+listener.Addr().String()+root+"/", "../../../yggdrasil/testdata/SessionSmoke.java",
			profile.ID.String(), profile.Nickname, launch.AccessToken)
	})

	var gameLogin yggdrasil.Authentication
	read(call("POST", root+"/authserver/authenticate", "", body("username", "AliceGame", "password", password, "clientToken", "fixture-binding"), 200), &gameLogin)
	var creativeLogin yggdrasil.Authentication
	read(call("POST", "/yggdrasil/creative/authserver/authenticate", "", body("username", "AliceGame", "password", password), 200), &creativeLogin)
	if creativeLogin.SelectedProfile == nil || creativeLogin.SelectedProfile.ID == id {
		t.Fatal("realm profile did not differ")
	}
	call("POST", root+"/authserver/validate", "", body("accessToken", creativeLogin.AccessToken), 403)

	call("POST", root+"/authserver/validate", "", body("accessToken", gameLogin.AccessToken, "clientToken", "wrong"), 403)
	call("POST", root+"/authserver/validate", "", body("accessToken", gameLogin.AccessToken), 204)
	var rotated yggdrasil.Session
	read(call("POST", root+"/authserver/refresh", "", body("accessToken", gameLogin.AccessToken), 200), &rotated)
	if rotated.ClientToken != "fixture-binding" || rotated.AccessToken == gameLogin.AccessToken {
		t.Fatal("unexpected game rotation")
	}
	call("POST", root+"/authserver/validate", "", body("accessToken", gameLogin.AccessToken), 403)
	call("POST", root+"/authserver/invalidate", "", body("accessToken", rotated.AccessToken), 204)
	if _, err := db.Exec(`UPDATE game_joins SET expires_at=now() WHERE profile_id=$1`, profile.ID); err != nil {
		t.Fatal(err)
	}
	call("GET", path, "", nil, 204)

	call("POST", root+"/authserver/validate", "", body("accessToken", rotated.AccessToken), 403)
	call("POST", root+"/authserver/signout", "", body("username", "AliceGame", "password", password), 204)
	call("GET", path, "", nil, 204)

	refresh := body("client", "launcher", "refreshToken", pair.Refresh)
	var fresh usecase.UserTokens
	read(call("POST", api+"/refresh", "", refresh, 200), &fresh)
	if fresh.SessionID != pair.SessionID || fresh.Refresh == pair.Refresh || fresh.Access == pair.Access {
		t.Fatal("unexpected account rotation")
	}
	call("GET", api+"/profile", pair.Access, nil, 401)
	call("GET", api+"/profile", fresh.Access, nil, 200)
	call("POST", api+"/refresh", "", refresh, 401)
	call("GET", api+"/profile", fresh.Access, nil, 401)
	call("POST", api+"/refresh", "", body("client", "launcher", "refreshToken", fresh.Refresh), 401)
	pair = login("Alice")
	call("POST", api+"/logout", "", body("client", "launcher", "refreshToken", pair.Refresh), 204)
	call("GET", api+"/profile", pair.Access, nil, 401)
	// Exactly one concurrent rotation can succeed, and the replay revokes its result.
	concurrent := login("Alice")
	payload, err := json.Marshal(body("client", "launcher", "refreshToken", concurrent.Refresh))
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	var ready sync.WaitGroup
	ready.Add(2)
	type refreshResult struct {
		status int
		pair   usecase.UserTokens
		err    error
	}
	results := make(chan refreshResult, 2)
	for range 2 {
		go func() {
			req := httptest.NewRequest("POST", api+"/refresh", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			ready.Done()
			<-start
			res, err := app.Test(req, 15000)
			if err != nil {
				results <- refreshResult{err: err}
				return
			}
			defer res.Body.Close()
			result := refreshResult{status: res.StatusCode}
			if result.status == 200 {
				result.err = json.NewDecoder(res.Body).Decode(&result.pair)
			}
			results <- result
		}()
	}
	ready.Wait()
	close(start)
	successes, replays := 0, 0
	first, second := <-results, <-results
	for _, result := range []refreshResult{first, second} {
		if result.err != nil {
			t.Fatal(result.err)
		}
		switch result.status {
		case 200:
			successes++
			call("GET", api+"/profile", result.pair.Access, nil, 401)
		case 401:
			replays++
		default:
			t.Fatalf("unexpected HTTP %d from concurrent refresh", result.status)
		}
	}
	if successes != 1 || replays != 1 {
		t.Fatalf("concurrent rotation: %d successes, %d replays", successes, replays)
	}
	var remaining int
	if err := db.QueryRow(`SELECT count(*) FROM sessions WHERE id=$1`, concurrent.SessionID).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("replayed family remains: count=%d, err=%v", remaining, err)
	}

	limited := login("Alice")
	if _, err := db.Exec(`UPDATE sessions SET refresh_count=$1 WHERE id=$2`, constants.MaxSessionRotations, limited.SessionID); err != nil {
		t.Fatal(err)
	}
	call("POST", api+"/refresh", "", body("client", "launcher", "refreshToken", limited.Refresh), 429)
	call("GET", api+"/profile", limited.Access, nil, 200)
	if _, err := db.Exec(`INSERT INTO game_tokens(token_hash,profile_id,client_token,expires_at)
        SELECT 'limit-fixture-' || n::text,$1,'fixture',now() + interval '5 minutes'
        FROM generate_series(1,$2::int) n`, profile.ID, constants.GameTokenLimit); err != nil {
		t.Fatal(err)
	}
	call("POST", api+"/game-token", limited.Access, body("profileId", profile.ID.String()), 429)

	one, two := login("Alice"), login("Alice")
	call("POST", api+"/logout-all", one.Access, nil, 204)
	call("GET", api+"/profile", two.Access, nil, 401)

	// The website consumes the same credentials using HTTP-only secure cookies.
	web := func(path string, input string, cookies []*http.Cookie, status int) []*http.Cookie {
		t.Helper()
		req := httptest.NewRequest("POST", api+path, strings.NewReader(input))
		req.Header.Set("Content-Type", "application/json")
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		res, err := app.Test(req, 15000)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode != status {
			t.Fatalf("website %s: HTTP %d, want %d", path, res.StatusCode, status)
		}
		var active []*http.Cookie
		for _, cookie := range res.Cookies() {
			if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.Domain != "" {
				t.Fatal("cookie missing secure attributes")
			}
			if cookie.Value != "" {
				active = append(active, cookie)
			}
		}
		return active
	}
	cookies := web("/login", `{"login":"Bob","password":"example-password"}`, nil, 200)
	if len(cookies) != 2 {
		t.Fatalf("expected two website session cookies, got %d", len(cookies))
	}
	renewed := web("/refresh", "", cookies, 204)
	if len(renewed) != 2 {
		t.Fatalf("missing rotated cookies, got %d", len(renewed))
	}
	web("/logout", "", renewed, 204)
	web("/refresh", "", renewed, 401)

	t.Log("login limit follows an account across aliases")
	var carol struct{ User struct{ ID uuid.UUID } }
	read(call("POST", api+"/register", "", body(
		"nickname", "Carol", "email", "carol@example.test", "password", password, "invitation_code", "INVITE3",
	), 201), &carol)
	call("POST", "/internal/game-profiles", provision, body(
		"userId", carol.User.ID.String(), "realm", "survival", "nickname", "CarolGame",
	), 200)
	for range constants.IdentityAttemptLimit {
		call("POST", root+"/authserver/authenticate", "", body(
			"username", "CarolGame", "password", "wrong-password",
		), 403)
	}
	call("POST", api+"/login", "", body("login", "Carol", "password", password), 401)
	call("POST", api+"/login", "", body("login", "carol@example.test", "password", password), 401)

}

func seed(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`
		WITH admin AS (INSERT INTO users(nickname,password_hash,role) VALUES ('fixture_admin','unused','admin') RETURNING id)
		INSERT INTO invitations(code,created_by) SELECT code,id FROM admin CROSS JOIN (VALUES ('INVITE1'),('INVITE2'),('INVITE3')) as x(code)
	`)
	if err != nil {
		t.Fatal(err)
	}
}
