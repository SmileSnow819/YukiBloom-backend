CREATE TABLE site_content_revision (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    version bigint NOT NULL DEFAULT 0 CHECK (version >= 0)
);
INSERT INTO site_content_revision (singleton, version) VALUES (true, 0);
