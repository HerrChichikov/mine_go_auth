package yggdrasil

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
)

var (
	profileID = regexp.MustCompile(`^[0-9a-f]{32}$`)
	nickname  = regexp.MustCompile(`^[A-Za-z0-9_]{3,16}$`)
	skinFile  = regexp.MustCompile(`^[0-9a-f]{64}(\.png)?$`)
	dnsName   = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)
)

type Profile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (p Profile) valid() bool {
	return validProfileID(p.ID) && nickname.MatchString(p.Name)
}

func validProfileID(id string) bool {
	return profileID.MatchString(id) && id != "00000000000000000000000000000000"
}

type Skin struct {
	URL  string
	Slim bool
}

type ProfileDetails struct {
	Profile
	Skin *Skin
}

type Property struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Signature string `json:"signature,omitempty"`
}

type DetailedProfile struct {
	Profile
	Properties []Property `json:"properties"`
}

type Metadata struct {
	Meta               map[string]any `json:"meta"`
	SkinDomains        []string       `json:"skinDomains"`
	SignaturePublicKey string         `json:"signaturePublickey"`
}

type Signer struct {
	key     *rsa.PrivateKey
	public  string
	domains []string
}

// NewSigner loads a persistent RSA key from the host service's secret configuration.
func NewSigner(keyPEM []byte, skinDomains []string) (*Signer, error) {
	block, rest := pem.Decode(keyPEM)
	if block == nil || len(bytes.TrimSpace(rest)) != 0 {
		return nil, errors.New("expected one RSA private key PEM block")
	}
	var key *rsa.PrivateKey
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, _ = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		parsed, _ := x509.ParsePKCS8PrivateKey(block.Bytes)
		key, _ = parsed.(*rsa.PrivateKey)
	}
	if key == nil || key.N.BitLen() < 2048 || key.Validate() != nil {
		return nil, errors.New("expected a valid RSA private key of at least 2048 bits")
	}
	public, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, err
	}
	domains := make([]string, 0, len(skinDomains))
	for _, domain := range skinDomains {
		domain = strings.ToLower(domain)
		if len(domain) > 253 || !dnsName.MatchString(domain) {
			return nil, errors.New("skin domains must be exact DNS host names")
		}
		if !slices.Contains(domains, domain) {
			domains = append(domains, domain)
		}
	}
	return &Signer{key, string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: public})), domains}, nil
}

func (s *Signer) Metadata(name string) Metadata {
	return Metadata{
		Meta: map[string]any{
			"serverName":                  name,
			"implementationName":          "minecraft-auth",
			"feature.non_email_login":     true,
			"feature.no_mojang_namespace": true,
			"feature.enable_profile_key":  false,
		},
		SkinDomains:        append([]string{}, s.domains...),
		SignaturePublicKey: s.public,
	}
}

func (s *Signer) AllowsSkin(value string) bool {
	u, err := url.Parse(value)
	return err == nil && len(value) <= 2048 && u.Scheme == "https" && u.User == nil && u.Fragment == "" &&
		u.RawQuery == "" && !u.ForceQuery && u.RawPath == "" && !strings.HasSuffix(u.Path, "/") && skinFile.MatchString(path.Base(u.Path)) &&
		(u.Port() == "" || u.Port() == "443") && slices.Contains(s.domains, strings.ToLower(u.Hostname()))
}

func (s *Signer) Details(profile ProfileDetails, signed bool, now time.Time) (DetailedProfile, error) {
	if !profile.Profile.valid() {
		return DetailedProfile{}, errors.New("invalid stored profile")
	}
	type texture struct {
		URL      string            `json:"url"`
		Metadata map[string]string `json:"metadata,omitempty"`
	}
	textures := make(map[string]texture, 1)
	if profile.Skin != nil {
		if !s.AllowsSkin(profile.Skin.URL) {
			return DetailedProfile{}, errors.New("skin URL is outside the configured HTTPS content-addressed storage")
		}
		skin := texture{URL: profile.Skin.URL}
		if profile.Skin.Slim {
			skin.Metadata = map[string]string{"model": "slim"}
		}
		textures["SKIN"] = skin
	}
	payload := struct {
		Timestamp         int64              `json:"timestamp"`
		ProfileID         string             `json:"profileId"`
		ProfileName       string             `json:"profileName"`
		SignatureRequired bool               `json:"signatureRequired,omitempty"`
		Textures          map[string]texture `json:"textures"`
	}{now.UnixMilli(), profile.ID, profile.Name, signed, textures}
	data, err := json.Marshal(payload)
	if err != nil {
		return DetailedProfile{}, err
	}
	property := Property{Name: "textures", Value: base64.StdEncoding.EncodeToString(data)}
	if signed {
		// Yggdrasil requires SHA1withRSA over the base64 value.
		digest := sha1.Sum([]byte(property.Value))
		signature, err := rsa.SignPKCS1v15(nil, s.key, crypto.SHA1, digest[:])
		if err != nil {
			return DetailedProfile{}, err
		}
		property.Signature = base64.StdEncoding.EncodeToString(signature)
	}
	return DetailedProfile{profile.Profile, []Property{property}}, nil
}
