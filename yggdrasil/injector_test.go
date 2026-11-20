package yggdrasil

import (
	"net"
	"os"
	"testing"
	"time"

	"alpha/internal/testsupport"

	"github.com/gofiber/fiber/v2"
)

func TestPinnedInjectorSession(t *testing.T) {
	if os.Getenv("MINECRAFT_AUTH_SMOKE_JAVA") == "" && os.Getenv("MINECRAFT_AUTH_SMOKE_AGENT") == "" &&
		os.Getenv("MINECRAFT_AUTH_SMOKE_CLASSPATH") == "" {
		t.Skip("injector smoke fixture paths are not configured")
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true, BodyLimit: maxBodyBytes})
	profile := ProfileDetails{Profile: Profile{profileUUID, "Alice"}, Skin: &Skin{URL: "https://skins.example.test/4d186321c1a7f0f354b297e8914ab2403e3f7abfe2d57c1a787fdc9dbb0bd029"}}
	Mount(app, &fixture{profile: profile}, testSigner(t), "Survival")
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

	root := "http://" + listener.Addr().String() + "/yggdrasil/survival/"
	testsupport.VerifyInjector(t, root, "testdata/SessionSmoke.java", "299c27ff-25c7-4c4d-b47b-2e46845b1034", "Alice", "fixture-game-token")
}
