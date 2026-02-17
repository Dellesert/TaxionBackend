-- Search documents unified index table
CREATE TABLE IF NOT EXISTS search_documents (
    id              BIGSERIAL PRIMARY KEY,

    -- Document identity
    entity_type     VARCHAR(30)  NOT NULL,
    entity_id       BIGINT       NOT NULL,

    -- Searchable text fields
    title           TEXT         NOT NULL DEFAULT '',
    content         TEXT         NOT NULL DEFAULT '',

    -- Full-text search vector (combined title + content, managed by trigger)
    search_vector   tsvector     NOT NULL DEFAULT '',

    -- Metadata for filtering (stored as JSONB for flexibility)
    metadata        JSONB        NOT NULL DEFAULT '{}',

    -- Permission fields (denormalized for fast filtering)
    accessible_by   BIGINT[]     NOT NULL DEFAULT '{}',
    is_public       BOOLEAN      NOT NULL DEFAULT false,
    creator_id      BIGINT       NOT NULL DEFAULT 0,

    -- Timestamps
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    -- Unique constraint: one document per entity
    CONSTRAINT uq_search_documents_entity UNIQUE (entity_type, entity_id)
);

-- GIN index on tsvector for full-text search
CREATE INDEX IF NOT EXISTS idx_search_documents_search_vector
    ON search_documents USING GIN (search_vector);

-- GIN index on trigram for fuzzy/typo-tolerant search
CREATE INDEX IF NOT EXISTS idx_search_documents_title_trgm
    ON search_documents USING GIN (title gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_search_documents_content_trgm
    ON search_documents USING GIN (content gin_trgm_ops);

-- GIN index on accessible_by array for permission filtering
CREATE INDEX IF NOT EXISTS idx_search_documents_accessible_by
    ON search_documents USING GIN (accessible_by);

-- B-tree index on entity_type for category filtering
CREATE INDEX IF NOT EXISTS idx_search_documents_entity_type
    ON search_documents (entity_type);

-- JSONB index on metadata for filter queries
CREATE INDEX IF NOT EXISTS idx_search_documents_metadata
    ON search_documents USING GIN (metadata jsonb_path_ops);

-- Index on creator_id for fast permission checks
CREATE INDEX IF NOT EXISTS idx_search_documents_creator_id
    ON search_documents (creator_id);

-- Partial index on is_public
CREATE INDEX IF NOT EXISTS idx_search_documents_is_public
    ON search_documents (is_public) WHERE is_public = true;

-- Composite index for common queries
CREATE INDEX IF NOT EXISTS idx_search_documents_type_updated
    ON search_documents (entity_type, updated_at DESC);

-- Helper: lowercase Cyrillic + Latin (lc_ctype=C does not handle Cyrillic)
CREATE OR REPLACE FUNCTION cyrillic_lower(t text) RETURNS text AS $$
    SELECT translate(lower(t),
        'АБВГДЕЁЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯ',
        'абвгдеёжзийклмнопрстуфхцчшщъыьэюя')
$$ LANGUAGE SQL IMMUTABLE STRICT;

-- Function to auto-update search_vector on INSERT/UPDATE
CREATE OR REPLACE FUNCTION search_documents_update_vector()
RETURNS TRIGGER AS $$
BEGIN
    NEW.search_vector :=
        setweight(to_tsvector('russian', cyrillic_lower(COALESCE(NEW.title, ''))), 'A') ||
        setweight(to_tsvector('english', cyrillic_lower(COALESCE(NEW.title, ''))), 'A') ||
        setweight(to_tsvector('russian', cyrillic_lower(COALESCE(NEW.content, ''))), 'B') ||
        setweight(to_tsvector('english', cyrillic_lower(COALESCE(NEW.content, ''))), 'B');
    NEW.updated_at := NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to auto-update search_vector
DROP TRIGGER IF EXISTS trg_search_documents_vector ON search_documents;
CREATE TRIGGER trg_search_documents_vector
    BEFORE INSERT OR UPDATE OF title, content
    ON search_documents
    FOR EACH ROW
    EXECUTE FUNCTION search_documents_update_vector();
