CREATE TABLE admins (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (username <> '')
);

CREATE TABLE sessions (
    token_hash bytea PRIMARY KEY,
    admin_id uuid NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    csrf_hash bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (octet_length(token_hash) = 32),
    CHECK (octet_length(csrf_hash) = 32)
);

CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

CREATE TABLE media (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    storage_key text NOT NULL UNIQUE,
    mime_type text NOT NULL,
    size_bytes bigint NOT NULL CHECK (size_bytes > 0),
    width integer CHECK (width > 0),
    height integer CHECK (height > 0),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE posts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    locale text NOT NULL,
    slug text NOT NULL,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    body_markdown text NOT NULL,
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    display_date timestamptz,
    published_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    categories text[] NOT NULL DEFAULT '{}',
    tags text[] NOT NULL DEFAULT '{}',
    extra jsonb NOT NULL DEFAULT '{}'::jsonb,
    cover_media_id uuid REFERENCES media(id) ON DELETE RESTRICT,
    UNIQUE (locale, slug),
    CHECK (locale <> '' AND slug <> '' AND title <> ''),
    CHECK (status <> 'published' OR published_at IS NOT NULL)
);

CREATE INDEX posts_published_list_idx ON posts (locale, published_at DESC) WHERE status = 'published';
