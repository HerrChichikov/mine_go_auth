package yggdrasil

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

const profileUUID = "299c27ff25c74c4db47b2e46845b1034"

type fixture struct {
	profile ProfileDetails
	joined  atomic.Bool
}

func (f *fixture) Authenticate(_ context.Context, scope RequestScope, request AuthenticateRequest) (Authentication, error) {
	if scope.Realm != "survival" || request.Username != "Alice" || request.Password != "fixture-password" {
		return Authentication{}, ErrInvalidCredentials
	}
	return Authentication{
		Session: Session{
			AccessToken: "fixture-game-token", ClientToken: "fixture-client",
			SelectedProfile: &f.profile.Profile, User: &User{ID: "996e2eaf32ac4cd484962ab2667fd65a", Properties: []Property{}},
		},
		AvailableProfiles: []Profile{f.profile.Profile},
	}, nil
}

func (f *fixture) Refresh(ctx context.Context, scope RequestScope, request RefreshRequest) (Session, error) {
	if err := f.Validate(ctx, scope, request.TokenRequest); err != nil {
		return Session{}, err
	}
	if request.SelectedProfile != nil {
		return Session{}, ErrProfileBound
	}
	return Session{AccessToken: "fixture-rotated-token", ClientToken: "fixture-client", SelectedProfile: &f.profile.Profile}, nil
}

func (f *fixture) Validate(_ context.Context, scope RequestScope, request TokenRequest) error {
	if scope.Realm != "survival" || request.AccessToken != "fixture-game-token" ||
		(request.ClientToken != nil && *request.ClientToken != "fixture-client") {
		return ErrInvalidToken
	}
	return nil
}

func (f *fixture) Invalidate(ctx context.Context, scope RequestScope, token string) error {
	return f.Validate(ctx, scope, TokenRequest{AccessToken: token})
}

func (f *fixture) Signout(ctx context.Context, scope RequestScope, request Credentials) error {
	_, err := f.Authenticate(ctx, scope, AuthenticateRequest{Credentials: request})
	return err
}

func (f *fixture) Join(ctx context.Context, scope RequestScope, request JoinRequest) error {
	if err := f.Validate(ctx, scope, TokenRequest{AccessToken: request.AccessToken}); err != nil {
		return err
	}
	if request.SelectedProfile != f.profile.ID || request.ServerID != "-abc012" {
		return ErrInvalidToken
	}
	f.joined.Store(true)
	return nil
}

func (f *fixture) HasJoined(_ context.Context, scope RequestScope, request HasJoinedRequest) (ProfileDetails, error) {
	if !f.joined.Load() || scope.Realm != "survival" || request.ServerID != "-abc012" ||
		!strings.EqualFold(request.Username, f.profile.Name) ||
		(request.ClientIP.IsValid() && request.ClientIP != netip.MustParseAddr("203.0.113.1")) {
		return ProfileDetails{}, ErrNotFound
	}
	return f.profile, nil
}

func (f *fixture) Profile(_ context.Context, scope RequestScope, id string) (ProfileDetails, error) {
	if scope.Realm != "survival" || id != f.profile.ID {
		return ProfileDetails{}, ErrNotFound
	}
	return f.profile, nil
}

func (f *fixture) ProfilesByName(_ context.Context, scope RequestScope, names []string) ([]Profile, error) {
	if scope.Realm == "unavailable" {
		return nil, errors.New("private database diagnostics")
	}
	for _, name := range names {
		if scope.Realm == "survival" && strings.EqualFold(name, f.profile.Name) {
			return []Profile{f.profile.Profile}, nil
		}
	}
	return nil, nil
}

func testSigner(t *testing.T) *Signer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	signer, err := NewSigner(keyPEM, []string{"skins.example.test"})
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func TestYggdrasilHTTP(t *testing.T) {
	app := fiber.New()
	signer := testSigner(t)
	profile := ProfileDetails{Profile: Profile{profileUUID, "Alice"}, Skin: &Skin{URL: "https://skins.example.test/4d186321c1a7f0f354b297e8914ab2403e3f7abfe2d57c1a787fdc9dbb0bd029", Slim: true}}
	Mount(app, &fixture{profile: profile}, signer, "Survival")
	const root = "/yggdrasil/survival"
	call := func(method, path, body string, status int) []byte {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if method == "POST" {
			req.Header.Set("Content-Type", "application/json; charset=utf-8")
		}
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != status || res.Header.Get("Cache-Control") != "no-store" || status == 204 && len(data) != 0 {
			t.Fatalf("%s %s returned %d %s, expected %d", method, path, res.StatusCode, data, status)
		}
		return data
	}
	var meta Metadata
	decode(t, call("GET", root+"/", "", 200), &meta)
	var auth Authentication
	decode(t, call("POST", root+"/authserver/authenticate", `{"username":"Alice","password":"fixture-password","agent":{"name":"Minecraft","version":1}}`, 200), &auth)
	if auth.AccessToken != "fixture-game-token" || auth.User != nil || auth.SelectedProfile == nil || *auth.SelectedProfile != profile.Profile {
		t.Fatal("unexpected authenticate response")
	}
	const token = `{"accessToken":"fixture-game-token","clientToken":"fixture-client"}`
	var refreshed Session
	decode(t, call("POST", root+"/authserver/refresh", token, 200), &refreshed)
	if refreshed.AccessToken != "fixture-rotated-token" || refreshed.ClientToken != "fixture-client" {
		t.Fatal("unexpected refresh response")
	}
	call("POST", root+"/authserver/validate", token, 204)
	call("POST", "/yggdrasil/creative/authserver/validate", token, 403)
	call("POST", root+"/authserver/validate", `{"accessToken":"fixture-game-token","clientToken":"wrong"}`, 403)
	call("POST", root+"/authserver/invalidate", `{"accessToken":"unknown","clientToken":"mismatch"}`, 204)
	call("POST", root+"/authserver/signout", `{"username":"Alice","password":"fixture-password"}`, 204)
	call("POST", root+"/authserver/signout", `{"username":"Alice","password":"wrong"}`, 403)
	call("POST", root+"/authserver/authenticate", `{"username":"Alice","password":"`+strings.Repeat("x", maxBodyBytes)+`"}`, 400)

	const session = "/sessionserver/session/minecraft"
	const joined = session + "/hasJoined?username=Alice&serverId=-abc012"
	call("GET", root+joined, "", 204)
	call("POST", root+session+"/join", `{"accessToken":"fixture-game-token","selectedProfile":"f45e283b12ae4b3e84e1872ecab820af","serverId":"-abc012"}`, 403)
	call("POST", root+session+"/join", `{"accessToken":"fixture-game-token","selectedProfile":"`+profileUUID+`","serverId":"-abc012"}`, 204)
	call("GET", "/yggdrasil/creative"+joined, "", 204)
	call("GET", root+joined+"&ip=203.0.113.2", "", 204)
	var signed, unsigned DetailedProfile
	decode(t, call("GET", root+joined+"&ip=203.0.113.1", "", 200), &signed)
	decode(t, call("GET", root+session+"/profile/"+profileUUID, "", 200), &unsigned)
	verifyTextures(t, meta.SignaturePublicKey, signed)
	if len(unsigned.Properties) != 1 || unsigned.Properties[0].Signature != "" {
		t.Fatal("unsigned profile included a signature")
	}
	var single DetailedProfile
	decode(t, call("GET", root+session+"/profile/"+profileUUID+"?unsigned=false", "", 200), &single)
	verifyTextures(t, meta.SignaturePublicKey, single)
	if string(call("POST", root+"/api/profiles/minecraft", `["OtherPlayer"]`, 200)) != "[]" {
		t.Fatal("no match must return an empty array")
	}
	if strings.Contains(string(call("POST", "/yggdrasil/unavailable/api/profiles/minecraft", `[]`, 500)), "private database") {
		t.Fatal("backend error leaked to client")
	}
}

func TestSkinURLRequiresImmutableFilename(t *testing.T) {
	signer := testSigner(t)
	const skin = "https://skins.example.test/4d186321c1a7f0f354b297e8914ab2403e3f7abfe2d57c1a787fdc9dbb0bd029"
	if !signer.AllowsSkin(skin) {
		t.Fatal("valid immutable skin rejected")
	}
	for _, candidate := range []string{
		"https://skins.example.test/mutable.png",
		skin + "?signature=example",
		skin + "/",
	} {
		if signer.AllowsSkin(candidate) {
			t.Fatalf("accepted mutable or ambiguous skin URL %q", candidate)
		}
	}
}

func TestSignerRejectsUnknownSkinHost(t *testing.T) {
	signer := testSigner(t)
	profile := ProfileDetails{Profile: Profile{profileUUID, "Alice"}, Skin: &Skin{URL: "https://skins.example.test.other.test/skin.png"}}
	if _, err := signer.Details(profile, true, time.Unix(0, 0)); err == nil {
		t.Fatal("accepted an unlisted skin domain")
	}
}

func decode(t *testing.T, body []byte, dst any) {
	t.Helper()
	if err := json.Unmarshal(body, dst); err != nil {
		t.Fatal(err)
	}
}

func verifyTextures(t *testing.T, publicPEM string, profile DetailedProfile) {
	t.Helper()
	if profile.ID != profileUUID || len(profile.Properties) != 1 || profile.Properties[0].Name != "textures" {
		t.Fatal("missing verified profile and textures")
	}
	block, _ := pem.Decode([]byte(publicPEM))
	if block == nil {
		t.Fatal("missing public key")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, ok := key.(*rsa.PublicKey)
	if !ok {
		t.Fatal("expected RSA public key")
	}
	property := profile.Properties[0]
	signature, err := base64.StdEncoding.DecodeString(property.Signature)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha1.Sum([]byte(property.Value))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA1, digest[:], signature); err != nil {
		t.Fatal(err)
	}
	value, err := base64.StdEncoding.DecodeString(property.Value)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		ProfileID string `json:"profileId"`
		Textures  map[string]struct {
			URL      string            `json:"url"`
			Metadata map[string]string `json:"metadata"`
		} `json:"textures"`
	}
	decode(t, value, &payload)
	skin := payload.Textures["SKIN"]
	if payload.ProfileID != profileUUID || skin.URL != "https://skins.example.test/4d186321c1a7f0f354b297e8914ab2403e3f7abfe2d57c1a787fdc9dbb0bd029" || skin.Metadata["model"] != "slim" {
		t.Fatal("texture payload is not bound to the expected skin and profile")
	}
}
