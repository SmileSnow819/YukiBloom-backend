CREATE TABLE footprint_locations (
    id text PRIMARY KEY,
    name text NOT NULL,
    latitude double precision NOT NULL CHECK (latitude BETWEEN -90 AND 90),
    longitude double precision NOT NULL CHECK (longitude BETWEEN -180 AND 180),
    location_type text NOT NULL,
    icon text NOT NULL DEFAULT '',
    sort_order integer NOT NULL DEFAULT 0,
    CHECK (id <> '' AND name <> '')
);

CREATE TABLE footprint_stays (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    start_date text NOT NULL,
    end_date text NOT NULL DEFAULT '',
    is_present boolean NOT NULL DEFAULT false,
    location_id text NOT NULL REFERENCES footprint_locations(id) ON DELETE RESTRICT,
    title text NOT NULL,
    stay_type text NOT NULL,
    description text NOT NULL DEFAULT '',
    sort_order integer NOT NULL DEFAULT 0,
    CHECK (start_date <> '' AND title <> ''),
    CHECK (NOT is_present OR end_date = '')
);

CREATE TABLE footprint_routes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    from_location_id text NOT NULL REFERENCES footprint_locations(id) ON DELETE RESTRICT,
    to_location_id text NOT NULL REFERENCES footprint_locations(id) ON DELETE RESTRICT,
    route_date text NOT NULL,
    transport text NOT NULL DEFAULT '',
    label text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    images text[] NOT NULL DEFAULT '{}',
    sort_order integer NOT NULL DEFAULT 0,
    CHECK (route_date <> '')
);

CREATE TABLE internship_experiences (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    start_date text NOT NULL,
    end_date text NOT NULL DEFAULT '',
    is_present boolean NOT NULL DEFAULT false,
    company text NOT NULL,
    icon text NOT NULL DEFAULT '',
    icon_color text NOT NULL DEFAULT '',
    position text NOT NULL,
    description text NOT NULL DEFAULT '',
    sort_order integer NOT NULL DEFAULT 0,
    CHECK (start_date <> '' AND company <> '' AND position <> ''),
    CHECK (NOT is_present OR end_date = '')
);
