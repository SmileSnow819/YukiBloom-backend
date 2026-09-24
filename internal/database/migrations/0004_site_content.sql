CREATE TABLE content_pages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    locale text NOT NULL,
    slug text NOT NULL,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    body_markdown text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published')),
    published_at timestamptz,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (locale, slug),
    CHECK (locale <> '' AND slug <> '' AND title <> ''),
    CHECK (status <> 'published' OR published_at IS NOT NULL)
);

CREATE TABLE site_profile (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    title text NOT NULL,
    alternate text NOT NULL DEFAULT '',
    subtitle text NOT NULL DEFAULT '',
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    avatar_url text NOT NULL DEFAULT '',
    show_logo boolean NOT NULL DEFAULT true,
    author text NOT NULL DEFAULT '',
    site_url text NOT NULL,
    default_og_image text NOT NULL DEFAULT '',
    start_year integer NOT NULL CHECK (start_year BETWEEN 1900 AND 2200),
    timezone text NOT NULL DEFAULT 'Asia/Shanghai',
    keywords text[] NOT NULL DEFAULT '{}',
    CHECK (title <> '' AND name <> '' AND site_url <> '')
);

CREATE TABLE social_links (
    platform text PRIMARY KEY,
    url text NOT NULL,
    icon text NOT NULL DEFAULT '',
    color text NOT NULL DEFAULT '',
    sort_order integer NOT NULL DEFAULT 0,
    enabled boolean NOT NULL DEFAULT true,
    CHECK (platform <> '' AND url <> '')
);

CREATE TABLE category_mappings (
    category_name text PRIMARY KEY,
    slug text NOT NULL UNIQUE,
    sort_order integer NOT NULL DEFAULT 0,
    CHECK (category_name <> '' AND slug <> '')
);

CREATE TABLE featured_categories (
    link text PRIMARY KEY,
    label text NOT NULL,
    image text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    sort_order integer NOT NULL DEFAULT 0,
    enabled boolean NOT NULL DEFAULT true,
    CHECK (link <> '' AND label <> '')
);

CREATE TABLE featured_series (
    slug text PRIMARY KEY,
    category_name text NOT NULL,
    label text NOT NULL DEFAULT '',
    full_name text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    cover text NOT NULL DEFAULT '',
    enabled boolean NOT NULL DEFAULT true,
    icon text NOT NULL DEFAULT '',
    highlight_on_home boolean NOT NULL DEFAULT true,
    links jsonb NOT NULL DEFAULT '{}'::jsonb,
    sort_order integer NOT NULL DEFAULT 0,
    CHECK (slug <> '' AND category_name <> '')
);

CREATE TABLE site_navigation (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_id uuid REFERENCES site_navigation(id) ON DELETE CASCADE,
    name text NOT NULL,
    name_key text NOT NULL DEFAULT '',
    path text NOT NULL DEFAULT '',
    icon text NOT NULL DEFAULT '',
    sort_order integer NOT NULL DEFAULT 0,
    CHECK (name <> '')
);

CREATE TABLE site_announcements (
    id text PRIMARY KEY,
    title text NOT NULL,
    content text NOT NULL,
    announcement_type text NOT NULL DEFAULT 'info' CHECK (announcement_type IN ('info','success','warning','error')),
    priority integer NOT NULL DEFAULT 0,
    color text NOT NULL DEFAULT '',
    publish_date date,
    starts_at timestamptz,
    ends_at timestamptz,
    link_url text NOT NULL DEFAULT '',
    link_text text NOT NULL DEFAULT '',
    link_external boolean NOT NULL DEFAULT false,
    enabled boolean NOT NULL DEFAULT true,
    CHECK (id <> '' AND title <> '')
);

CREATE TABLE friend_settings (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    title text NOT NULL DEFAULT '',
    subtitle text NOT NULL DEFAULT '',
    apply_title text NOT NULL DEFAULT '',
    apply_description text NOT NULL DEFAULT '',
    example_yaml text NOT NULL DEFAULT ''
);

CREATE TABLE friend_links (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    site text NOT NULL,
    url text NOT NULL,
    owner text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    image text NOT NULL DEFAULT '',
    color text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'approved' CHECK (status IN ('pending','approved','rejected')),
    sort_order integer NOT NULL DEFAULT 0,
    CHECK (site <> '' AND url <> '')
);

CREATE TABLE content_translations (
    locale text NOT NULL,
    entity_type text NOT NULL,
    entity_key text NOT NULL,
    label text NOT NULL DEFAULT '',
    full_name text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    PRIMARY KEY (locale, entity_type, entity_key)
);

CREATE TABLE music_groups (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title text NOT NULL,
    sort_order integer NOT NULL DEFAULT 0,
    enabled boolean NOT NULL DEFAULT true,
    CHECK (title <> '')
);

CREATE TABLE music_links (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id uuid NOT NULL REFERENCES music_groups(id) ON DELETE CASCADE,
    title text NOT NULL DEFAULT '',
    url text NOT NULL,
    sort_order integer NOT NULL DEFAULT 0,
    CHECK (url <> '')
);

CREATE TABLE background_music_tracks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title text NOT NULL,
    url text NOT NULL,
    sort_order integer NOT NULL DEFAULT 0,
    enabled boolean NOT NULL DEFAULT true,
    CHECK (title <> '' AND url <> '')
);
