CREATE TABLE categories (
    category_name text PRIMARY KEY,
    slug text NOT NULL UNIQUE,
    image text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    show_on_home boolean NOT NULL DEFAULT false,
    sort_order integer NOT NULL DEFAULT 0,
    CHECK (category_name <> '' AND slug <> '')
);

WITH mapped_categories AS (
    SELECT
        mapping.category_name,
        mapping.slug,
        COALESCE(feature.image, '') AS image,
        COALESCE(feature.description, '') AS description,
        feature.link IS NOT NULL AND feature.enabled AS show_on_home,
        feature.sort_order AS featured_order,
        mapping.sort_order AS fallback_order
    FROM category_mappings AS mapping
    LEFT JOIN LATERAL (
        SELECT link, image, description, enabled, sort_order
        FROM featured_categories
        WHERE link = mapping.slug OR label = mapping.category_name
        ORDER BY (link = mapping.slug) DESC, sort_order, link
        LIMIT 1
    ) AS feature ON true
), unmatched_featured_categories AS (
    SELECT
        feature.label AS category_name,
        feature.link AS slug,
        feature.image,
        feature.description,
        feature.enabled AS show_on_home,
        feature.sort_order AS featured_order,
        2147483647 AS fallback_order
    FROM featured_categories AS feature
    WHERE NOT EXISTS (
        SELECT 1
        FROM category_mappings AS mapping
        WHERE mapping.slug = feature.link OR mapping.category_name = feature.label
    )
), merged_categories AS (
    SELECT * FROM mapped_categories
    UNION ALL
    SELECT * FROM unmatched_featured_categories
), ordered_categories AS (
    SELECT
        category_name,
        slug,
        image,
        description,
        show_on_home,
        (row_number() OVER (
            ORDER BY show_on_home DESC, featured_order NULLS LAST, fallback_order, category_name
        ) - 1)::integer AS sort_order
    FROM merged_categories
)
INSERT INTO categories (category_name, slug, image, description, show_on_home, sort_order)
SELECT category_name, slug, image, description, show_on_home, sort_order
FROM ordered_categories;

UPDATE content_translations AS category
SET
    label = COALESCE(NULLIF(featured.label, ''), category.label),
    description = COALESCE(NULLIF(featured.description, ''), category.description)
FROM content_translations AS featured
WHERE category.locale = featured.locale
  AND category.entity_type = 'categories'
  AND featured.entity_type = 'featuredCategories'
  AND category.entity_key = featured.entity_key;

DELETE FROM content_translations AS featured
WHERE featured.entity_type = 'featuredCategories'
  AND EXISTS (
      SELECT 1
      FROM content_translations AS category
      WHERE category.locale = featured.locale
        AND category.entity_type = 'categories'
        AND category.entity_key = featured.entity_key
  );

UPDATE content_translations
SET entity_type = 'categories'
WHERE entity_type = 'featuredCategories';

DROP TABLE featured_categories;
DROP TABLE category_mappings;
