CREATE TABLE personal_content_revision (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    footprints_version bigint NOT NULL DEFAULT 1 CHECK (footprints_version > 0),
    timeline_version bigint NOT NULL DEFAULT 1 CHECK (timeline_version > 0)
);

INSERT INTO personal_content_revision (singleton) VALUES (true);
