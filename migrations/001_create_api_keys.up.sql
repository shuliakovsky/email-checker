CREATE TABLE api_keys (
                          api_key TEXT PRIMARY KEY,
                          created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                          key_type TEXT NOT NULL CHECK (key_type IN ('pay_as_you_go', 'monthly')),
                          initial_checks INT NOT NULL CHECK (initial_checks > 0),
                          used_checks INT NOT NULL DEFAULT 0 CHECK (used_checks >= 0),
                          remaining_checks INT NOT NULL CHECK (remaining_checks >= 0),
                          expires_at TIMESTAMPTZ NOT NULL,
                          last_topup TIMESTAMPTZ,
                          CHECK (remaining_checks = initial_checks - used_checks)
);

CREATE INDEX api_keys_expires_at_idx ON api_keys (expires_at);
CREATE INDEX api_keys_key_type_idx ON api_keys (key_type);
CREATE INDEX api_keys_remaining_checks_idx ON api_keys (remaining_checks);