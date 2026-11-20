-- +goose Up
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;

ALTER TABLE sessions ADD COLUMN access_hash TEXT UNIQUE,
    ADD COLUMN access_expires_at TIMESTAMPTZ,
    ADD COLUMN refresh_count INTEGER NOT NULL DEFAULT 0 CHECK (refresh_count >= 0);
CREATE UNIQUE INDEX sessions_refresh_hash_unique ON sessions(token_hash) WHERE access_hash IS NOT NULL;
CREATE TABLE spent_refresh_tokens (
    token_hash TEXT PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE
);
CREATE INDEX spent_refresh_tokens_session_idx ON spent_refresh_tokens(session_id);

CREATE TABLE game_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    realm TEXT NOT NULL CHECK (realm ~ '^[A-Za-z0-9_-]{1,64}$'),
    nickname TEXT NOT NULL CHECK (nickname ~ '^[A-Za-z0-9_]{3,16}$'),
    skin_url TEXT,
    skin_slim BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE (user_id, realm)
);
CREATE UNIQUE INDEX game_profiles_realm_nickname ON game_profiles(realm, lower(nickname));

CREATE TABLE game_tokens (
    token_hash TEXT PRIMARY KEY,
    profile_id UUID NOT NULL REFERENCES game_profiles(id) ON DELETE CASCADE,
    session_id UUID REFERENCES sessions(id) ON DELETE CASCADE,
    client_token TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX game_tokens_profile_idx ON game_tokens(profile_id);
CREATE INDEX game_tokens_session_idx ON game_tokens(session_id);
CREATE INDEX game_tokens_expiry_idx ON game_tokens(expires_at);

CREATE TABLE game_joins (
    profile_id UUID NOT NULL REFERENCES game_profiles(id) ON DELETE CASCADE,
    challenge TEXT NOT NULL,
    token_hash TEXT NOT NULL REFERENCES game_tokens(token_hash) ON DELETE CASCADE ON UPDATE CASCADE,
    client_ip INET,
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (profile_id, challenge)
);
CREATE INDEX game_joins_token_idx ON game_joins(token_hash);
CREATE INDEX game_joins_expiry_idx ON game_joins(expires_at);

CREATE TABLE auth_attempts (
    bucket TEXT PRIMARY KEY,
    expires_at TIMESTAMPTZ NOT NULL,
    attempts INTEGER NOT NULL
);
CREATE INDEX auth_attempts_expiry_idx ON auth_attempts(expires_at);
